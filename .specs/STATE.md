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

## Handoff

- **Feature**: `lgn-103-104-sheet-preview-dry-run` (.specs/features/lgn-103-104-sheet-preview-dry-run)
- **Phase / Task**: Execute — COMPLETE. T1–T15 committed plus hardening; independent Verifier (round 2) PASS 49/49 requirements, 2 flagged gaps closed, 11/11 mutants killed; `validate_state.py` exit 0. Report: `.specs/features/lgn-103-104-sheet-preview-dry-run/validation.md`.
- **Completed**: backend dry-run T1–T5 (`35f25d8`, `9cc082f`, `b320a6c`, `1a22bf7`, `b7367ac`); desktop plumbing T6–T11 (`006aed2`, `da2a87c`, `6867a91`, `247112e`, `be3f1cc`, `82a057a`); surface/cleanup T12–T15 (`f3b812e`, `a0073ee`, `371a4cf`, `02815aa`); spec commit `5db3539`; verification hardening `ed7900c`.
- **In-progress** (file:line): none.
- **Next step**: push the branch and open the PR — a remote operation that needs explicit go-ahead.
- **Blockers**: none.
- **Uncommitted files**: none after the closing docs commit (validation report, lessons, this handoff).
- **Branch**: `feat/lgn-103-104-sheet-preview-dry-run` (from `origin/main` @ `8ae4252`, includes the merged LGN-110 save cycle); not pushed.
- **Prior feature**: `lgn-110-save-delta` — COMPLETE (PR #57 merged); `lgn-108-open-sheet` — COMPLETE (PR #56, merged); `lgn-109-create-sheet` — COMPLETE (PR #55, merged)
