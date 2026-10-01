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
- **Phase / Task**: Execute — **COMPLETE** (T1–T8 done and committed; independent Verifier PASS)
- **Completed**: T1 (edit-state provider), T2 (control-builder injection), T3 (screen wiring), T4 (marker + leave guard), T5 (refresh dispatch providers), T6 (F5 routing), T7 (sheet F5 refresh flow), T8 (200-field render budget, commit `34df4d4`); Verifier: 19/19 ACs matched, 7/7 sensor mutants killed, gate 326 passed + 4 skips, `validate_state.py` exit 0 — report in `.specs/features/lgn-102-granular-rebuild/validation.md`
- **In-progress** (file:line): none
- **Next step**: commit the validation report + GRN-10 traceability flip; then interactive UAT (if requested) and lessons distillation (none required — clean PASS, no signal). Profile-mode <100 ms GRN-10 measurement remains a manual TDD release check per spec decision line 46
- **Blockers**: none
- **Uncommitted files**: `.specs/features/lgn-102-granular-rebuild/{spec.md,validation.md}`, this STATE; untracked skill dirs (`.claude/`, `.cursor/`, `.windsurf/` — never commit)
- **Branch**: feat/LGN-100-front-end-phase-3-sheet-preview
