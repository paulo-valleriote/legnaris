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

## Handoff

- **Feature**: `lgn-107-local-sheet-library` (.specs/features/lgn-107-local-sheet-library)
- **Phase / Task**: Execute — COMPLETE (T1–T7 committed; Verifier PASS on round 2, `validate_state.py` exit 0)
- **Completed**: T1 entry model, T2 typed error, T3 `store.save`, T4 `store.load` + version recovery, T5 store providers + directory seam, T6 `SheetLibrary` notifier, T7 bootstrap. A round-1 Verifier FAIL on three test-coverage gaps was fixed in `3b5a4f9` (full upsert replacement, real-store durability, non-UTC serialization). Report: `.specs/features/lgn-107-local-sheet-library/validation.md` (23/23 ACs, 8/8 mutants killed, 417 tests green).
- **In-progress** (file:line): none
- **Next step**: open the PR for `feat/lgn-107-local-sheet-library` (base `feat/lgn-105-exclusive-options`, or `main` once PR #53 merges). Push is not yet authorized.
- **Blockers**: none
- **Uncommitted files**: none after the final report/lessons commit; untracked skill dirs (`.claude/`, `.cursor/`, `.windsurf/` — never commit)
- **Branch**: feat/lgn-107-local-sheet-library (based on feat/lgn-105-exclusive-options / PR #53)
- **Prior feature**: `lgn-105-exclusive-options` — COMPLETE (Verifier PASS, report in `.specs/features/lgn-105-exclusive-options/validation.md`)
