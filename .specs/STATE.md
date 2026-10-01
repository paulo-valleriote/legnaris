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
- **Phase / Task**: Execute — **T8 complete; all 8 tasks done** (200-field render budget, GRN-10, under the user-approved amendment)
- **Completed**: T1 (edit-state provider), T2 (control-builder injection), T3 (screen wiring), T4 (marker + leave guard), T5 (refresh dispatch providers), T6 (F5 routing), T7 (sheet F5 refresh flow), T8 (200-field render budget) — T8 gate green: `flutter analyze && flutter test` → 326 passed + 4 skips
- **In-progress** (file:line): none — T8 uncommitted pending commit `test(sheet): assert the 200-field render budget`
- **Next step**: commit T8 (test + spec/tasks/context amendments + this STATE), then dispatch the independent Verifier (spec-anchored check + discrimination sensor) → `.specs/features/lgn-102-granular-rebuild/validation.md`, run `validate_state.py`, final Handoff update
- **Blockers**: none — GRN-10 escalation resolved by user decision (2026-10-01): debug-widget test holds a <4000 ms regression budget (measured 1800–2700 ms under full-suite parallel load, ~1000 ms file-alone); the <100 ms product AC stays in profile mode per spec line 46
- **Uncommitted files**: `desktop/test/sheet/sheet_screen_test.dart` (T8 test), `.specs/features/lgn-102-granular-rebuild/{spec.md,tasks.md,context.md}` (GRN-10 amendment wording), this STATE; untracked skill dirs (`.claude/`, `.cursor/`, `.windsurf/` — never commit)
- **Branch**: feat/LGN-100-front-end-phase-3-sheet-preview
