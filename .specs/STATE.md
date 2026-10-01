# STATE

## Decisions

### AD-001
- **Decision**: Sheet edit state lives in an autoDispose Riverpod family keyed by template id (`sheetEditProvider`); each field control subscribes to its own dotted path via `select`, and the screen never watches the edit state directly (dirty reads happen at tap time; the marker is its own `Consumer`).
- **Reason**: Riverpod `select` only rebuilds listeners whose selected value changed, so one edit rebuilds exactly one control; a screen-level watch would rebuild the whole form and defeat the LGN-102 granularity requirement.
- **Trade-off**: `SheetForm` grew a `controlBuilder` injection point (the engine stays Riverpod-free but has two control sources), and dirty-aware UI must live in isolated `Consumer` widgets rather than the screen build.
- **Scope**: desktop/ — Screen 2 and any future surface rendering schema-driven fields with local edits (Phase 4's dirty + Save builds on this same state).
- **Date**: 2026-10-01
- **Status**: active

## Handoff

- **Feature**: `lgn-102-granular-rebuild` (.specs/features/lgn-102-granular-rebuild)
- **Phase / Task**: Execute — T4 complete, start of T5 (refresh dispatch providers)
- **Completed**: T1 (edit-state provider), T2 (control-builder injection), T3 (screen wiring), T4 (marker + leave guard)
- **In-progress** (file:line): none — T4 committed
- **Next step**: implement T5 — `desktop/lib/sheet/sheet_refresh.dart` (active-sheet id + refresh tick providers) + build_runner + 4 tests, gate `cd desktop && flutter test`, commit `feat(sheet): add active-sheet refresh dispatch providers`
- **Blockers**: none
- **Uncommitted files**: `.specs/STATE.md`, `.specs/features/lgn-102-granular-rebuild/context.md` (feature artifacts), untracked skill dirs (`.claude/`, `.cursor/`, `.windsurf/` — never commit)
- **Branch**: feat/LGN-100-front-end-phase-3-sheet-preview
