# STATE

## Decisions

### AD-001
- **Decision**: Sheet edit state lives in an autoDispose Riverpod family keyed by template id (`sheetEditProvider`); each field control subscribes to its own dotted path via `select`, and the screen never watches the edit state directly (dirty reads happen at tap time; the marker is its own `Consumer`).
- **Reason**: Riverpod `select` only rebuilds listeners whose selected value changed, so one edit rebuilds exactly one control; a screen-level watch would rebuild the whole form and defeat the LGN-102 granularity requirement.
- **Trade-off**: `SheetForm` grew a `controlBuilder` injection point (the engine stays Riverpod-free but has two control sources), and dirty-aware UI must live in isolated `Consumer` widgets rather than the screen build.
- **Scope**: desktop/ — Screen 2 and any future surface rendering schema-driven fields with local edits (Phase 4's dirty + Save builds on this same state).
- **Date**: 2026-10-01
- **Status**: active

### AD-002
- **Decision**: Mutually exclusive schema options are expressed as an opt-in scalar `exclusive_group` on the leaf `FieldSchema` (json/bson `exclusive_group`, omitted when empty), not as a `select`/enum field. Leaves sharing a non-empty group under the same parent may have at most one non-zero value; the DAG normalizes PATCH updates (turns one member on → zeroes its siblings; two members on in one payload → typed 400).
- **Reason**: Keeping the 0/1 proxy leaves preserves the anti-copyright multiplier strategy and every existing sheet; a select field would rewrite the numeric formula environment, `SeedValue`/`default`, and require a data migration. `one_of` was rejected as a name because it collides with JSON Schema's array-of-schemas keyword and promises "exactly one" while the contract is "at most one".
- **Trade-off**: Exclusivity is a per-leaf tag rather than a structural group node, so it cannot carry its own display label; group/label humanization stays a follow-up. The desktop must collapse tagged siblings into one control, and any client that ignores the tag still renders independent toggles (safe but non-ideal).
- **Scope**: backend `internal/sheet/template` + `internal/sheet` (contract, DAG, PATCH); desktop `lib/engine` + `lib/sheet` (grouping, exclusive control, edit state, screen). Groups: `armorClassAbility` (ac pair) and `spellcastingAbility` (six `spellUses*`, unified dropdown since increment 3; the redundant `spells.ability` leaf was removed — increment decided 2026-10-02).
- **Date**: 2026-10-01
- **Status**: active

### AD-003
- **Decision**: Local persistence lives in `desktop/lib/library/` as a versioned JSON file (`sheet_library.json`) in the OS application-support directory, written atomically via temp-file + rename, behind an injectable `appSupportDirectoryProvider` (`path_provider`, resolved in `main()`); the store API is synchronous and the `SheetLibrary` Riverpod notifier owns the in-memory index with write-through saves.
- **Reason**: The local sheet library must survive restarts, migrate by `schema_version`, and quarantine a corrupt/unsupported file without losing its bytes — file-level atomic replace and byte-preserving quarantine require owning the file, which `shared_preferences` hides. Sync I/O is safe on Dart's single isolate for a tiny index and keeps providers synchronous.
- **Trade-off**: Adds the first-party `path_provider` dependency and a manual migration/versioning path; tests must override the directory (the platform channel is unavailable otherwise). `shared_preferences` would be fewer lines but cannot satisfy the quarantine/atomicity contract.
- **Scope**: desktop/ local persistence — the sheet library now; the dev base-URL override and UI preferences follow the same convention later.
- **Date**: 2026-10-02
- **Status**: active

### AD-004
- **Decision**: Screen 2 receives an opened sheet only through an overridable `openedSheetProvider` (`Sheet?`, default null); the creation flow wraps the pushed route in a `ProviderScope` that overrides both `openedSheetProvider` and the `sheetEditControllerProvider` family, so the edit state seeds from the `201` payload's `data` and the header shows its `name`.
- **Reason**: The card's "abrir a ficha criada" arrives before LGN-108 (the `GET /sheets/:id` open), and the `201` already carries the full sheet, so a route-scoped override avoids a second GET and gives LGN-108 one seam to add the `GET`-based open. The controller-family override is required because a nested `ProviderScope` shares non-overridden providers with its parent — a parent-created controller would not observe the seed.
- **Trade-off**: Screen 2's open now depends on a route-scoped provider seam instead of a widget parameter, and the controller family must be re-declared in that scope; the preview path (no sheet) keeps schema-default seeding. The sheet `data` is still never persisted (LIB-08) — the seam carries the in-memory `201` payload only.
- **Scope**: desktop/ — Screen 2 (`lib/sheet/`) and catalog navigation (`lib/catalog/catalog_screen.dart`); LGN-108's library-open flow builds on the same seam.
- **Date**: 2026-10-03
- **Status**: active

### AD-005
- **Decision**: Screen 2 always renders a stored sheet: `SheetScreen({required sheetId})`, the edit-state family `sheetEditControllerProvider` is keyed by the **sheet id** (not the template id), and a new autoDispose `SheetOpenController(sheetId)` owns the single `GET /sheets/:id` with loading/error state and evicts the local entry on `NOT_FOUND`. The creation flow keeps injecting the `201` payload through `openedSheetProvider`, so no second GET is issued. The temporary "Preview sheet" action (Phase 3) is removed.
- **Reason**: The card places the fetch on Screen 2 ("GET /sheets/:id na Tela 2") and retargets the edit state from template id to sheet id; seeding overlays `Sheet.data` on the schema defaults per key. User decisions (2026-10-03): on `NOT_FOUND` stay on Screen 2 with a mapped message and no Retry; remove Preview now that the real open flow exists.
- **Trade-off**: The edit family key and Screen 2's identity change together, so call sites and their tests must move in one commit. Screen 2 fetches the template detail only after the loaded sheet reveals `template_id` (two sequential GETs). The library write failure on `NOT_FOUND` eviction is swallowed (best-effort) while the message still renders.
- **Scope**: desktop/ — `lib/sheet/` (open + edit + screen), `lib/catalog/` (library section + catalog embed), `lib/engine/sheet_defaults.dart` (`seedFromSheet`), l10n. Reuses AD-004's route-scoped override seam.
- **Date**: 2026-10-03
- **Status**: active

### AD-006
- **Decision**: The save cycle lives on `SheetEditController`, not a second provider family: `SheetEditState` gains `isSaving`/`saveError`/`pendingDelta`, and `save()` sends the changed editable delta with one `PATCH /sheets/:id` and, on success, advances the baseline to `mergeCascade(mergeCascade(defaults, delta), cascade)` while `data = mergeCascade(data, cascade)`. `mergeCascade` is the promoted public deep-merge reused by `seedFromSheet`. `Ctrl+S` mirrors the F5 tick seam (`sheetSaveTickProvider`); back/F5 are no-ops while a save is in flight.
- **Reason**: The edit family is overridden in the creation route scope (AD-004), so a separate save family would resolve in the parent container and read an **unseeded** edit state; keeping `save()` on the scoped controller makes scope consistency structural. Recomputing the baseline (instead of `defaults = data`) keeps edits made during the in-flight request pending rather than silently marking them saved.
- **Trade-off**: Save status is part of the edit state (field controls `select` scalars, so they never rebuild on a status change); the creation-dialog fix relies on the `AlertDialog` content being height-bounded (`Flexible` content + flexing preview scroll) rather than a magic height.
- **Scope**: desktop/ — `lib/sheet/` (edit/save, screen footer + guards, refresh/save tick), `lib/shell/shortcut_registry.dart`, `lib/engine/sheet_defaults.dart`, `lib/l10n/`, `lib/catalog/create_sheet_dialog.dart`. Backend unchanged (PATCH already returns only the derived delta).
- **Date**: 2026-10-03
- **Status**: active

### AD-007
- **Decision**: The stateless sheet/template preview (`POST /sheets/:id/preview`, `POST /templates/:id/preview`) reuses the exact PATCH recalculation path through one unexported `recalculate` helper in `internal/sheet/service.go`; persistence (`data` + `updated_at`) stays only in `PartialUpdate`, and the preview returns the same derived-cascade body the PATCH returns with the same `apperr` mapping.
- **Reason**: LGN-103 requires a dry-run whose cascade is identical to an equivalent PATCH without duplicating the DAG logic; one helper guarantees the two cannot drift and lets tests assert zero repository writes.
- **Trade-off**: `PartialUpdate` now depends on a returned struct holding the merged data the preview does not need, and any future change to the recalc sequence is shared by both paths — deliberate, since divergence is the risk.
- **Scope**: backend `internal/sheet` (service) and `internal/app` (routes); future preview-like evaluations must go through the same helper rather than re-implementing DAG traversal.
- **Date**: 2026-10-04
- **Status**: active

### AD-008
- **Decision**: Unsaved sheet drafts are backed up as one atomic JSON file per sheet id under the injected app-support directory (`<appSupport>/drafts/<sheet_id>.json`, versioned envelope holding only the pending editable delta), written every 60 seconds while dirty through an injectable `DraftBackupScheduler` and deleted on save success, confirmed discard, `NOT_FOUND` eviction and library removal; restore overlays the stored sheet and is filtered to editable leaves of the loaded template.
- **Reason**: LGN-104 requires periodic draft backup with recovery; a per-sheet delta survives a crash, reuses AD-003's injected directory, and never persists derived values. The injected scheduler/clock keep the 60-second cadence testable without waiting or a new package.
- **Trade-off**: A best-effort backup can silently miss a write (no error surfaces), and restore depends on the template still exposing the same editable leaves — stale keys are ignored rather than migrated.
- **Scope**: desktop/ local persistence — drafts now; the same injected-directory + injected-scheduler convention applies to future periodic writes.
- **Date**: 2026-10-04
- **Status**: active

### AD-009
- **Decision**: The dry-run result is identified on Screen 2 by additive preview marks, not a second preview surface: `SheetEditState.previewMarks` publishes the previewed delta's editable leaves (`modified`) and the derived leaves the cascade recalculated (`recalculated`); each row shows a "Modified"/"Recalculated" label (through a `SheetForm.rowMarkBuilder` seam) and its control gets a change tint (`SheetMark` + `sheetMarkTint`). The base "Calculated" (RND-10) stays on derived fields the cascade did not touch, and the marks clear on the next edit/save/discard and never publish on a failed preview.
- **Reason**: The preview showed the recalculated form without saying which values the user changed and which were computed from them; the backend cascade already carries only the changed derived leaves, so the marks need no extra request or client-side comparison. Keeping the marking additive preserves RND-10 and the read-only surfaces that never preview (creation dialog).
- **Trade-off**: Each row gains a mark `Consumer` on top of the existing per-path control `select` (two subscriptions per row), and the label lives in the row area while the tint lives in the control, so `SheetFieldControl`/`SheetExclusiveControl` each gained a `mark` input and `SheetForm` a `rowMarkBuilder` seam.
- **Scope**: desktop/ — `lib/engine/` (sheet_mark, field_control, exclusive_control, sheet_form), `lib/sheet/sheet_edit_provider.dart` (state + preview), `lib/sheet/sheet_screen.dart`, l10n. Backend unchanged.
- **Date**: 2026-10-05
- **Status**: active

### AD-010
- **Decision**: The session is the desktop app's boundary for navigation, local data, and in-flight auth work: the `AuthGate` authenticated branch mounts its own `Navigator` (every authenticated route and dialog — `showDialog(useRootNavigator: false)` — lives inside it), local session data (sheet-library index + draft backups) is swept synchronously right after the state flip in `logout()` and `onUnauthorized()` only, and `AuthSession` exposes a session-version counter (bumped at login/register/logout/forced logout, never on refresh apply) that `AuthenticatedClient` checks before refreshing a `401` or forcing a logout.
- **Reason**: routes pushed on the app-root navigator and modal dialogs hosted there survived account switches (bugs 1/2 of LGN-88 manual QA), a state-transition listener cannot tell a definitive session end from the transient restore fallback, and token comparison breaks concurrent-`401` refresh sharing after a rotation while a boundary-only version does not.
- **Trade-off**: future authenticated-surface code must opt into the branch (`useRootNavigator: false`, push via the nearest navigator) instead of framework defaults; `AuthSession` now knows about the library and draft stores; the transport contract grows one method (`sessionVersion()`).
- **Scope**: desktop/ — `lib/auth/` (gate, session), `lib/api/authenticated_client.dart`, `lib/shell/`, `lib/catalog/`, `lib/sheet/`, `lib/library/`. Any future desktop screen, dialog, or session-local store must respect the boundary.
- **Date**: 2026-10-08
- **Status**: active

### AD-011
- **Decision**: The web client ships the Lighthouse budget configuration (`web/lighthouserc.json`, F1-T025) but not the Lighthouse runner: `@lhci/cli` is not a dependency of `web/`. The runner lands with the Lighthouse gate in F8 (`F8-T005`).
- **Reason**: Nothing consumed `@lhci/cli` — no npm script and no CI step — while its dev-only transitive tree was the sole source of the advisories the `sca` gate reported (17 findings, 11 high; `extract-zip` and `sprintf-js` have no patched release), which made SEC-02 unsatisfiable without suppressing findings. After removal, `npm audit --audit-level=high` and `osv-scanner` both report nothing, so the gate stays unscoped and unsuppressed.
- **Trade-off**: The installability/performance budgets in `lighthouserc.json` cannot run from a locked toolchain until F8 adds the runner; until then they are covered by the F1 quickstart's manual audit and the Playwright suites.
- **Scope**: web/ — `package.json` and `package-lock.json` only; `lighthouserc.json`, the pipelines and the F8 gate's contract are unchanged.
- **Date**: 2026-10-16
- **Status**: active

### AD-012
- **Decision**: The `sast` gate runs Semgrep Community Edition only, invoked as `pipx install semgrep` + `semgrep scan --config p/owasp-top-ten --config p/typescript --config p/react --error`; CodeQL is removed from `ci-web.yml` until the repository has GitHub Code Security. The deprecated `semgrep/semgrep-action` wrapper is replaced by the CLI.
- **Reason**: GitHub's availability table lists both code scanning and the CodeQL CLI as unavailable on a private repository without GitHub Code Security, and Code Security can only be purchased on GitHub Team or Enterprise plans — this repository is private under a personal account, so `github/codeql-action/analyze` failed with `Resource not accessible by integration`. Semgrep CE keeps SEC-02's blocking SAST enforcement at no licence cost, and `--error` makes a finding fail the job instead of only being reported (verified: exit 1 on a finding).
- **Trade-off**: CodeQL's `security-extended` analysis is lost meanwhile, and the three configured packs are thin for this stack — a seeded `eval`/`innerHTML`/`document.write` file produces no finding, while `p/default` catches it (broadening the ruleset is F9-T009's scope).
- **Scope**: `.github/workflows/ci-web.yml` (`sast` job), `contracts/ci-gates.contract.md`. Reinstating CodeQL requires enabling Code Security (org plan) or making the repository public; nothing else in the gate changes.
- **Date**: 2026-10-16
- **Status**: active

## Handoff

- **Feature**: `desktop-session-boundary` (desktop logout/login hygiene: navigation, session-local data, in-flight auth work) — COMPLETE, validation PASS ✅.
- **Phase / Task**: Specify + Design + Tasks + **Execute COMPLETE** (T1–T12, 3 batch sub-agents + 1 Verifier); Verifier (author ≠ verifier) returned PASS on the first iteration.
- **Completed**: T1–T12 across commits `b32cfd5..9faaad8` (one atomic commit per task). Session-mounted navigator in `AuthGate` (+ observers seam), three dialogs hosted on the session navigator, shared `pushSheetScreen()` route, post-login sheet above a mounted catalog, sheet-library/draft-backup sweep at session end, single-flight logout + logout confirmation, dead-session writer guards, session-version two-point check in `AuthenticatedClient.send()`.
- **Validation**: `.specs/features/desktop-session-boundary/validation.md` (PASS; 18/18 ACs matched spec outcome, 5/5 sensor mutations killed, 706 tests, +45 vs baseline `31a7b32`, 0 deleted/weakened). All NAV-01…08, DATA-01…08, SES-01…02 promoted to `Verified` in `spec.md`.
- **In-progress** (file:line): none.
- **Next step**: user reviews the 13 local commits on `main` (`31a7b32..9faaad8`), then pushes / opens a PR — remote operations need explicit go-ahead (blast radius: local commits only).
- **Blockers**: none. `flutter analyze` clean, `flutter test` 706 passed / 4 pre-existing skips, `make lint-specs` clean.
- **Uncommitted files**: none.
- **Branch**: `main` (feature commits local, not pushed).
- **Lesson recorded**: none (clean PASS — no surviving mutants, spec-precision gaps, or deviations).
- **Prior feature**: `lgn-7-ownership` — COMPLETE, PASS (PR #60); `front-end-phase-6-auth` merged as `41ff4c7`; earlier: `lgn-103-104-sheet-preview-dry-run`, `lgn-110-save-delta` (PR #57), `lgn-108-open-sheet` (PR #56), `lgn-109-create-sheet` (PR #55)
