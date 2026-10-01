# LGN-102 Granular Rebuild Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: inline (Medium scope — no `design.md`); decisions recorded in `context.md`
**Status**: Approved

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: root `AGENTS.md` (Testing section: edge cases first, `Happy path:`/`Edge:`/`Error:` case prefixes, never ship behavior without tests), `desktop/AGENTS.md` (English strings, theme tokens, reuse existing infra), `.github/workflows/ci-flutter.yml` (`flutter analyze` + `flutter test` are the CI gates).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Riverpod state (edit state, refresh dispatch) | unit (ProviderContainer) | All branches; 1:1 to spec ACs; every listed edge case | `desktop/test/sheet/*_test.dart` | `cd desktop && flutter test test/sheet` |
| Engine widgets (SheetForm, controls) | widget | Happy + edge paths; keys-based finders; default and injected modes | `desktop/test/engine/*_test.dart` | `cd desktop && flutter test test/engine` |
| Screen surface (SheetScreen, marker, dialogs, refresh flow) | widget | Happy + every listed edge + error paths (GRN-02…19) | `desktop/test/sheet/*_test.dart` | `cd desktop && flutter test test/sheet` |
| Shell shortcuts (F5 routing) | widget | Routing happy path + guarded edges (sheet active, dialog open) | `desktop/test/shell/*_test.dart` | `cd desktop && flutter test test/shell` |
| Perf budget (GRN-10) | widget (wall-clock) | ≤200-field template mounts in <100 ms after API response | `desktop/test/sheet/sheet_screen_test.dart` | `cd desktop && flutter test test/sheet` |

## Gate Check Commands

> Generated from codebase - confirm before Execute.

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | After tasks with unit/widget tests only | `cd desktop && flutter test` |
| Build | Last task of each phase | `cd desktop && flutter analyze && flutter test` |

---

## Execution Plan

Phases are ordered and run sequentially - each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Foundation

State and rendering primitives, independently testable.

```
T1   T2
```

### Phase 2: Screen wiring

The screen switches from widget-local data to the edit provider with per-field select.

```
T3
```

### Phase 3: Awareness and guards

Marker, leave confirmation, and the F5 dispatch chain.

```
T4
T5 → T6 → T7
```

### Phase 4: Performance gate

```
T8
```

---

## Task Breakdown

### T1: Add the in-memory edit-state provider

**What**: `sheetEditProvider` — autoDispose family keyed by template id; seeds every leaf from the template detail via `seedDefaults`, stores edits immutably at dotted paths, exposes a pending flag (deep-compare against the seeded defaults), discards on dispose/reload.
**Where**: `desktop/lib/sheet/sheet_edit_provider.dart`
**Depends on**: None
**Reuses**: `desktop/lib/engine/sheet_defaults.dart` (`seedDefaults`, `setPathValue`), `desktop/lib/catalog/template_detail_provider.dart` (seed source), test pattern from `desktop/test/catalog/template_detail_provider_test.dart`
**Requirement**: GRN-01, GRN-02, GRN-03, GRN-04, GRN-06

**Tools**:

- MCP: `context7` (only if Riverpod 3 API needs confirming)
- Skill: NONE

**Done when**:

- [x] Provider builds empty while the detail is loading, seeds from schema defaults once the detail arrives, reseeds after a detail reload (edits dropped)
- [x] `setPath` stores at the dotted path without mutating the previous state; pending flag flips on a differing edit and clears when the state equals the defaults again
- [x] Different template ids hold independent state; disposing the provider discards it (reopen reseeds)
- [x] No HTTP request is issued by seeding or editing (mock client records nothing beyond the detail GET)
- [x] `dart run build_runner build --delete-conflicting-outputs` regenerates `.g.dart`
- [x] Gate check passes: `cd desktop && flutter test`
- [x] Test count: 7 tests in `desktop/test/sheet/sheet_edit_provider_test.dart` (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(sheet): add in-memory edit-state provider`

---

### T2: Add control-builder injection to SheetForm

**What**: optional `controlBuilder` parameter on `SheetForm`; when present each row renders the builder's widget instead of the default `data`/`onChanged` control, keeping the engine Riverpod-free.
**Where**: `desktop/lib/engine/sheet_form.dart`
**Depends on**: None
**Reuses**: existing `SheetForm` row/section layout and `sheet_form_test.dart` harness
**Requirement**: GRN-07, GRN-08

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `controlBuilder: (context, field) => Widget` replaces the per-row control when provided; default path (`data` + `onChanged`) is byte-for-byte unchanged when absent
- [x] Constructor asserts exactly one control source (builder, or data)
- [x] Gate check passes: `cd desktop && flutter test`
- [x] Test count: 3 new tests in `desktop/test/engine/sheet_form_test.dart` (builder rows used, default rows unchanged, both-sources assert)

**Tests**: widget
**Gate**: build (last task of Phase 1)

**Commit**: `feat(engine): add control builder injection to sheet form`

---

### T3: Wire granular per-field edits into the screen

**What**: `SheetScreen` drops its local `_data`, renders `SheetForm` with a `controlBuilder` whose `Consumer` watches `sheetEditProvider.select` on the field's own path and writes through `setPath`; controls become editable, derived stay inert; library doc amended for LGN-102.
**Where**: `desktop/lib/sheet/sheet_screen.dart`
**Depends on**: T1, T2
**Reuses**: `valueAtPath` from `desktop/lib/api/models/sheet.dart`, `SheetFieldControl` keys (`sheet-input-*`, `sheet-toggle-*`)
**Requirement**: GRN-02, GRN-03, GRN-05, GRN-07, GRN-08, GRN-09

**Tools**:

- MCP: `context7` (Riverpod `select` inside `Consumer` semantics, if needed)
- Skill: NONE

**Done when**:

- [x] Editing an editable input stores the value and shows it; toggling a `config.*` proxy flips the switch display; only GET requests are recorded (GRN-02, GRN-03)
- [x] Sibling-identity test: snapshot all `SheetFieldControl` instances, edit one field, every other instance is `identical`, the edited one is not (GRN-07, GRN-08)
- [x] Derived (`is_editable: false`) controls stay non-interactive; the stale "no control accepts an edit" test is amended to the spec outcome: controls enabled, still zero mutations (GRN-05)
- [x] Controls reflect seeded values after the template arrives (GRN-09)
- [x] Gate check passes: `cd desktop && flutter test`
- [x] Test count: 5 new/amended tests in `desktop/test/sheet/sheet_screen_test.dart` (planned 6; the numeric-draft-restore edge is already asserted at the control layer — `field_control_test.dart:444` — and a screen-level copy would duplicate it)

**Tests**: widget
**Gate**: build (last task of Phase 2)

**Commit**: `feat(sheet): wire granular per-field edits into the screen`

---

### T4: Show the local-edits marker and guard leaving

**What**: header caption "Local edits" shown while the edit state differs from the seeded defaults, plus a destructive-confirmation dialog on back with pending edits (confirm discards + pops, cancel stays); clean back pops directly. New l10n strings in `desktop/lib/l10n/app_en.arb` (+ regenerated localizations).
**Where**: `desktop/lib/sheet/sheet_screen.dart`
**Depends on**: T3
**Reuses**: `desktop/AGENTS.md` destructive-confirmation pattern (explicit consequence, explicit buttons), existing `sheet-back` key
**Requirement**: GRN-11, GRN-12, GRN-13, GRN-14, GRN-15, GRN-16

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] First differing edit shows the marker; reverting every field to its default hides it (GRN-11, GRN-12)
- [x] Back with pending edits opens the dialog stating the local edits will be discarded; confirm discards and navigates back; cancel keeps the screen and state (GRN-13, GRN-14, GRN-15)
- [x] Back with no pending edits navigates with zero dialogs (GRN-16)
- [x] Marker and dialog copy go through `lib/l10n/*.arb`; regenerated localizations committed
- [x] Gate check passes: `cd desktop && flutter test`
- [x] Test count: 7 new tests in `desktop/test/sheet/sheet_screen_test.dart`

**Tests**: widget
**Gate**: quick

**Commit**: `feat(sheet): show local-edits marker and guard leaving`

---

### T5: Add active-sheet refresh dispatch providers

**What**: `sheet_refresh.dart` holding `activeSheetIdProvider` (set while a sheet screen is mounted) and `sheetRefreshTickProvider` (increments per refresh request) — the dispatch seam F5 uses to reach the active sheet.
**Where**: `desktop/lib/sheet/sheet_refresh.dart`
**Depends on**: None
**Reuses**: riverpod_annotation codegen pattern from `desktop/lib/shell/shell_state.dart`
**Requirement**: GRN-17, GRN-18, GRN-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Registering an id sets the active sheet; clearing it (dispose) returns null; a second registration replaces the first
- [ ] The tick increments monotonically and starts at 0
- [ ] `dart run build_runner build --delete-conflicting-outputs` regenerates `.g.dart`
- [ ] Gate check passes: `cd desktop && flutter test`
- [ ] Test count: 4 tests in `desktop/test/sheet/sheet_refresh_test.dart`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(sheet): add active-sheet refresh dispatch providers`

---

### T6: Route F5 to the active sheet

**What**: `appActions` dispatches F5 by surface — a mounted sheet bumps `sheetRefreshTickProvider` (no catalog request), otherwise the existing catalog guard applies unchanged.
**Where**: `desktop/lib/shell/shortcut_registry.dart`
**Depends on**: T5
**Reuses**: existing `RefreshCatalogIntent` action and its creation-dialog guard
**Requirement**: GRN-17, GRN-18, GRN-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] F5 with a registered sheet bumps the tick and issues no catalog request
- [ ] F5 without a sheet keeps today's behavior (catalog refresh; blocked while the creation dialog is open)
- [ ] Gate check passes: `cd desktop && flutter test`
- [ ] Test count: 2 new tests in `desktop/test/shell/shortcut_registry_test.dart` (existing guard tests stay green)

**Tests**: widget
**Gate**: quick

**Commit**: `feat(shell): route F5 to the active sheet`

---

### T7: Refresh the template behind a discard confirmation

**What**: `SheetScreen` registers itself as the active sheet, listens to the refresh tick, and on fire runs the guard: pending edits → confirmation dialog → confirm invalidates the edit state and refetches the template detail (reseed on success), cancel issues nothing; clean → refetch directly. New l10n strings for the refresh dialog.
**Where**: `desktop/lib/sheet/sheet_screen.dart`
**Depends on**: T4, T6
**Reuses**: `templateDetailControllerProvider.notifier.load()`, the T4 confirmation-dialog shape
**Requirement**: GRN-17, GRN-18, GRN-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] F5 with no pending edits issues exactly one new detail GET and the form reseeds from the fresh schema (GRN-17)
- [ ] F5 with pending edits shows the dialog before any request; the request count is unchanged until confirm (GRN-18)
- [ ] Cancelling the refresh dialog issues zero requests (GRN-19)
- [ ] A confirmed refresh that fails shows the mapped error + Retry; state reseeds after a successful retry (spec edge)
- [ ] Unmounting the screen clears the active-sheet registration
- [ ] Gate check passes: `cd desktop && flutter test`
- [ ] Test count: 7 new tests in `desktop/test/sheet/sheet_screen_test.dart`

**Tests**: widget
**Gate**: build (last task of Phase 3)

**Commit**: `feat(sheet): refresh the template behind a discard confirmation`

---

### T8: Assert the 200-field render budget

**What**: generated 200-field template fixture; `Stopwatch` around the frame that mounts the form after the API response asserts <100 ms with all controls present (GRN-10).
**Where**: `desktop/test/sheet/sheet_screen_test.dart`
**Depends on**: T3
**Reuses**: `pumpScreen` harness already in the file
**Requirement**: GRN-10

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] A 200-field template renders every field control and the measured build frame is <100 ms
- [ ] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: 1 new test; full suite green with no deletions

**Tests**: widget
**Gate**: build (last task of Phase 4)

**Commit**: `test(sheet): assert the 200-field render budget`

---

## Phase Execution Map

Phases run in sequence; tasks within a phase run in order:

```
Phase 1 → Phase 2 → Phase 3 → Phase 4

Phase 1:  T1 ----------------→ T3
          T2 ----------------→ T3
Phase 2:                       T3
Phase 3:                       T3 → T4 --------→ T7
          T5 → T6 ------------------→ T7
Phase 4:                       T3 → T8
```

Execution is strictly sequential — one task at a time, in order. 8 tasks fit a single task-budgeted batch, so Execute runs inline with no sub-agents.

**Task Granularity Check**

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: edit-state provider | 1 provider file | ✅ Granular |
| T2: control-builder injection | 1 widget file | ✅ Granular |
| T3: screen wiring | 1 screen file | ✅ Granular |
| T4: marker + leave guard | 1 screen file (+ l10n data) | ✅ Granular (cohesive, same file) |
| T5: dispatch providers | 1 provider file | ✅ Granular |
| T6: F5 routing | 1 registry file | ✅ Granular |
| T7: sheet refresh flow | 1 screen file (+ l10n data) | ✅ Granular |
| T8: perf test | 1 test file | ✅ Granular |

**Diagram-Definition Cross-Check**

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | — | ✅ Match |
| T2 | None | — | ✅ Match |
| T3 | T1, T2 | T1 → T3, T2 → T3 | ✅ Match |
| T4 | T3 | T3 → T4 | ✅ Match |
| T5 | None | — | ✅ Match |
| T6 | T5 | T5 → T6 | ✅ Match |
| T7 | T4, T6 | T4 → T7, T6 → T7 | ✅ Match |
| T8 | T3 | T3 → T8 | ✅ Match |

**Test Co-location Validation**

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1 | Riverpod state | unit | unit | ✅ OK |
| T2 | Engine widget | widget | widget | ✅ OK |
| T3 | Screen surface | widget | widget | ✅ OK |
| T4 | Screen surface | widget | widget | ✅ OK |
| T5 | Riverpod state | unit | unit | ✅ OK |
| T6 | Shell shortcuts | widget | widget | ✅ OK |
| T7 | Screen surface | widget | widget | ✅ OK |
| T8 | Perf budget | widget (timing) | widget | ✅ OK |
