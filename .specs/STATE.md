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
- **Phase / Task**: Execute — **BLOCKED at T8** (200-field render budget, GRN-10)
- **Completed**: T1 (edit-state provider), T2 (control-builder injection), T3 (screen wiring), T4 (marker + leave guard), T5 (refresh dispatch providers), T6 (F5 routing), T7 (sheet F5 refresh flow) — all committed and green (325 passed + 4 skips, analyze clean)
- **In-progress** (file:line): T8 implemented but uncommitted — `desktop/test/sheet/sheet_screen_test.dart` (render-budget group, fixture + 1 test); fails ONLY the `<100 ms` assertion
- **Next step**: escalate GRN-10 to the spec owner — measured evidence: the frame that mounts 200 `SheetFieldControl`s takes ~913–1037 ms warm (~1443 ms cold) in `flutter test` on this machine; scaling probe shows a linear ~4.4 ms/control (0→35 ms, 10→242 ms, 50→351 ms, 100→595 ms, 200→~950 ms), i.e. structural debug-mode cost, not noise or JIT warm-up. Decision needed: amend the AC/threshold, change the gate environment (profile/DevTools, per spec decision line 46), or optimize rendering. Do NOT weaken the AC without that decision
- **Blockers**: GRN-10 `<100 ms` widget-test gate unachievable on this machine (10x over budget); execution contract says STOP and escalate rather than weaken the test
- **Uncommitted files**: `desktop/test/sheet/sheet_screen_test.dart` (T8 test — failing on timing by design of the escalation); untracked skill dirs (`.claude/`, `.cursor/`, `.windsurf/` — never commit)
- **Branch**: feat/LGN-100-front-end-phase-3-sheet-preview
