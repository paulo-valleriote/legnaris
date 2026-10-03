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

## Handoff

- **Feature**: `lgn-109-create-sheet` (.specs/features/lgn-109-create-sheet)
- **Phase / Task**: Execute — COMPLETE (T1–T4 committed; Verifier PASS on round 2, `validate_state.py` exit 0)
- **Completed**: T1 opened-sheet seed seam, T2 dialog submit (name validation, single POST, library upsert, inline error + retry), T3 Screen 2 title, T4 catalog navigation. A test-only fix commit `7be258d` closed two Verifier edge-case notes; round-2 PASS (11/11 ACs, 3/3 mutants killed, 433 tests green). Report: `.specs/features/lgn-109-create-sheet/validation.md`.
- **In-progress** (file:line): none
- **Next step**: open the PR for `feat/lgn-109-create-sheet` (base `main`). Push is not yet authorized.
- **Blockers**: none. LGN-108 ("Abrir ficha": library list + `GET /sheets/:id`) is still To Do; this card intentionally opens the created sheet from the `201` payload (AD-004).
- **Uncommitted files**: none; untracked skill dirs (`.claude/`, `.cursor/`, `.windsurf/` — never commit)
- **Branch**: feat/lgn-109-create-sheet (based on main @ 1553730)
- **Prior feature**: `lgn-107-local-sheet-library` — COMPLETE (merged #54)
