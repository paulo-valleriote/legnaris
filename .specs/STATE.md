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

## Handoff

- **Feature**: `lgn-110-save-delta` (.specs/features/lgn-110-save-delta)
- **Phase / Task**: Execute — COMPLETE (T1–T7 committed; independent Verifier PASS 27/27, 7/7 mutants killed on round 2; `validate_state.py` exit 0)
- **Completed**: T1 creation-dialog bounded content, T2 public `mergeCascade`, T3 save on `SheetEditController` (delta + baseline recompute), T4 Screen 2 footer + l10n, T5 `Ctrl+S` binding + `sheetSaveTickProvider`, T6 save-in-flight discard guards, T7 Verifier gap fixes. Commits `b35c6d8`, `904d6ad`, `07c8830`, `a8802cf`, `bcdfb7c`, `1a76241`, `cdfb359` (docs `d6fadda`, chore `71d4298`). Report: `.specs/features/lgn-110-save-delta/validation.md`.
- **In-progress** (file:line): none
- **Next step**: review and merge PR #57 (https://github.com/paulo-valleriote/legnaris/pull/57).
- **Blockers**: none. Full suite `flutter analyze` clean + 486 passed / 4 skipped (pre-existing live/contract skips; `api_client_retry_test` is a known wall-clock flake that passes in isolation); Verifier round 1 gaps (SAV-02 tooltip, SAV-15 merge granularity, surviving Ctrl+S-guard mutant) closed in `cdfb359` and re-verified PASS.
- **Uncommitted files**: none; branch pushed, PR #57 opened.
- **Branch**: `feat/lgn-110-save-delta` (pushed; PR #57 → main, based on `6eab233` = origin/main, includes LGN-108)
- **Prior feature**: `lgn-108-open-sheet` — COMPLETE (PR #56, merged); `lgn-109-create-sheet` — COMPLETE (PR #55, merged)
