# Save with Delta and Incremental Cascade Merge Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/lgn-110-save-delta/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: `AGENTS.md` (edge cases first + happy path, typed errors), `desktop/AGENTS.md` (English only, reuse app infrastructure, async feedback), `.github/workflows/ci-flutter.yml` (`flutter analyze`, `flutter test`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Cascade merge (`mergeCascade`) | unit | New key, overwritten leaf, deep nesting, empty delta, inputs unmutated; 1:1 to SAV-13 | `desktop/test/engine/sheet_defaults_test.dart` | `flutter test test/engine/sheet_defaults_test.dart` |
| Edit controller (`SheetEditController.save`) | unit | Delta excludes derived/reverted, empty delta = no request, merge + baseline recompute clears dirty, in-flight edits stay pending, failure keeps edits + mapped copy, no auto-retry, single-flight; 1:1 to SAV-05…23 | `desktop/test/sheet/sheet_edit_provider_test.dart` | `flutter test test/sheet/sheet_edit_provider_test.dart` |
| Screen 2 footer (`SheetScreen`) | widget | Footer + dirty indicator, Save disabled/enabled, inline error, one PATCH, back/F5 blocked while saving; 1:1 to SAV-01…04, 06, 18…23 | `desktop/test/sheet/sheet_screen_test.dart` | `flutter test test/sheet/sheet_screen_test.dart` |
| Shortcut registry (`Ctrl+S`) | widget | Ctrl+S issues one PATCH with a pending edit, no sheet = no request, in flight = one request; 1:1 to SAV-05…07 | `desktop/test/shell/shortcut_registry_test.dart` | `flutter test test/shell/shortcut_registry_test.dart` |
| Creation dialog layout (`CreateSheetDialog`) | widget | No overflow + non-overlapping action row at 1280×800, 1280×720, 640×480; preview still scrolls; 1:1 to DIA-01…04 | `desktop/test/catalog/create_sheet_layout_test.dart` | `flutter test test/catalog/create_sheet_layout_test.dart` |

**Note on `Tests: none`:** no task here is `none`; every task creates or modifies a tested layer.

## Gate Check Commands

> Generated from codebase - confirm before Execute. All commands run from `desktop/` (CI works in `desktop` per `.github/workflows/ci-flutter.yml`).

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | After tasks with unit/widget tests on one file | `flutter test test/<area>/<file>.dart` |
| Full | After tasks that regenerate shared l10n/codegen or wire cross-file behavior | `flutter test` |
| Build | At phase completion / final validation | `flutter analyze && flutter test` |

---

## Execution Plan

Phases are ordered and run sequentially - each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Independent foundations

The dialog layout fix and the merge helper share no code.

```
T1
T2
```

### Phase 2: Save controller

The controller computes the delta and merges the cascade using the helper.

```
T2 -> T3
```

### Phase 3: Save surface

The footer, the `Ctrl+S` dispatch and the in-flight guards all consume the controller.

```
T3 -> T4
T4 -> T5
T3 -> T6
```

---

## Task Breakdown

### T1: Creation dialog bounded content

**What**: Fix the dialog content overflow by making it flex: wrap the preview in `Flexible` inside the `AlertDialog` content column, and wrap the preview's scroll area in `Flexible` inside its card. The preview then shrinks to the available height and scrolls internally, keeping the action row separate. Add a widget regression test that mounts the dialog at 1280×800, 1280×720 and 640×480, asserts no layout exception, asserts the submit rect does not intersect the preview card rect, and keeps the existing scroll affordance assertions.
**Where**: `desktop/lib/catalog/create_sheet_dialog.dart`, `desktop/test/catalog/create_sheet_layout_test.dart`
**Depends on**: None
**Reuses**: the `create-sheet-preview` / `create-sheet-submit` keys, `previewCardMaxHeight`, the `create_sheet_preview_test.dart` pump harness
**Requirement**: DIA-01, DIA-02, DIA-03, DIA-04

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] At 1280×720 the content issues no `RenderFlex` overflow and the submit rect does not intersect the preview rect
- [x] At 640×480 the same holds and the Create action stays reachable
- [x] The preview card still scrolls internally and its scrollbar stays visible
- [x] At 1280×800 the layout is unchanged
- [x] Gate check passes: `flutter test test/catalog/create_sheet_layout_test.dart`
- [x] Test count: 3 tests pass (new file; all three sizes present)

**Tests**: widget
**Gate**: quick

**Commit**: `fix(desktop): keep the creation dialog action clear of the preview`

---

### T2: Public cascade merge helper

**What**: Promote the private `_overlay` deep-merge in `sheet_defaults.dart` to a public `mergeCascade(base, overrides)` (recurse where both sides are maps, replace the leaf otherwise; returns a fresh tree) and have `seedFromSheet` call it, so the cascade merge reuses the one merge implementation. Add unit tests for new key, overwritten leaf, deep nesting, empty delta and input immutability.
**Where**: `desktop/lib/engine/sheet_defaults.dart`
**Depends on**: None
**Reuses**: `seedFromSheet`'s existing merge semantics
**Requirement**: SAV-13

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] A new nested key is added at its path and an existing leaf is overwritten
- [ ] A deep tree merges recursively instead of replacing a sibling map
- [ ] An empty delta returns an equal tree
- [ ] The inputs are not mutated
- [ ] `seedFromSheet` still passes its existing tests
- [ ] Gate check passes: `flutter test test/engine/sheet_defaults_test.dart`
- [ ] Test count: recorded after the run (file total; +new, no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): expose the cascade deep-merge helper`

---

### T3: Save on the edit controller

**What**: Extend `SheetEditState` with `isSaving` and `saveError`, a `copyWith`, and a `pendingDelta` getter (edited paths whose value differs from the baseline, nested via `setPathValue`). Add `SheetEditController.save()`: return when saving or the delta is empty; mark `isSaving`; `ApiClient.patchSheet(sheetId, delta)`; on success recompute `data = mergeCascade(data, cascade)` and `defaults = mergeCascade(mergeCascade(defaults, delta), cascade)` preserving `editedPaths` and clearing `saveError`; on failure keep data/defaults/editedPaths, clear `isSaving` and set `saveError = failureCopy(error)`. Preserve `isSaving`/`saveError` across `setPath`/`setExclusive`. Add unit tests covering the delta rules, merge/baseline, in-flight edits, empty delta, failure, retry and single-flight.
**Where**: `desktop/lib/sheet/sheet_edit_provider.dart`
**Depends on**: T2
**Reuses**: `mergeCascade` (T2), `setPathValue`/`valueAtPath`, `apiClientProvider`, `failureCopy`, the `SheetOpenController` pattern
**Requirement**: SAV-08, SAV-09, SAV-10, SAV-11, SAV-12, SAV-13, SAV-14, SAV-15, SAV-16, SAV-17, SAV-18, SAV-19, SAV-20, SAV-21, SAV-22, SAV-23

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] The delta contains exactly the changed editable leaves, omits a reverted field and never a derived leaf
- [ ] An empty delta issues no PATCH and leaves `isSaving` false
- [ ] A `200` merges the cascade, recomputes the baseline and clears `isPending`
- [ ] An edit made during the request stays pending after the response
- [ ] An empty `{}` cascade clears `isPending` without changing values
- [ ] A failure keeps data/baseline/edits, sets `saveError` and adds no cascade
- [ ] Two rapid `save()` calls issue exactly one PATCH; a retry after failure issues one further PATCH
- [ ] Gate check passes: `flutter test test/sheet/sheet_edit_provider_test.dart`
- [ ] Test count: recorded after the run (file total; +new, no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): save the sheet delta and merge the cascade`

---

### T4: Screen 2 footer with the Save action

**What**: Add a fixed footer to Screen 2, rendered only once the sheet and template are loaded: an isolated `Consumer` selecting `isPending`/`isSaving`/`saveError` renders the dirty indicator, the inline mapped error above the action row, and the Save action (enabled only when `isPending && !isSaving`, with a no-changes tooltip otherwise) calling `save()`. Add the l10n keys `sheetSaveAction` and `sheetSaveNoChangesTooltip`, regenerate l10n, and add widget tests.
**Where**: `desktop/lib/sheet/sheet_screen.dart`, `desktop/lib/l10n/app_en.arb`
**Depends on**: T3
**Reuses**: `sheetEditControllerProvider` (T3), `AppSpacing`/`AppSizes`, the sheet l10n block, the existing `sheetLocalEdits` string
**Requirement**: SAV-01, SAV-02, SAV-03, SAV-04, SAV-06

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] The footer renders with the loaded form and is absent in loading/error/no-fields states
- [ ] Save is disabled with no pending edit and enabled once a field is edited
- [ ] Activating Save issues exactly one PATCH; a second activation while in flight issues none
- [ ] A failure renders the mapped copy inline above the action row and keeps the dirty indicator
- [ ] A success removes the dirty indicator and the inline error
- [ ] Gate check passes: `flutter test test/sheet/sheet_screen_test.dart`
- [ ] Test count: recorded after the run (file total; +new, no silent deletions)

**Tests**: widget
**Gate**: quick

**Commit**: `feat(desktop): add the sheet footer with the save action`

---

### T5: Ctrl+S saves the open sheet

**What**: Add `AppShortcut.saveSheet`/`SaveSheetIntent` bound to `Ctrl+S` and a `sheetSaveTickProvider` dispatch counter. `appActions` bumps the tick only while `activeSheetIdProvider != null`; the mounted Screen 2 listens to the tick and calls `save()`. Extend the shortcut registry tests to prove one PATCH with a pending edit, no request with no active sheet, and one request in flight.
**Where**: `desktop/lib/shell/shortcut_registry.dart`, `desktop/lib/sheet/sheet_refresh.dart`
**Depends on**: T4
**Reuses**: the F5 tick seam (`sheetRefreshTickProvider`), `activeSheetIdProvider`, the T4 `save()` call site
**Requirement**: SAV-05, SAV-06, SAV-07

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `Ctrl+S` with a pending edit issues exactly one PATCH
- [ ] `Ctrl+S` with no active sheet issues no request
- [ ] A second `Ctrl+S` while a save is in flight issues no second request
- [ ] The F5 behavior is unchanged
- [ ] Gate check passes: `flutter test test/shell/shortcut_registry_test.dart`
- [ ] Test count: recorded after the run (file total; +new, no silent deletions)

**Tests**: widget
**Gate**: quick

**Commit**: `feat(desktop): bind Ctrl+S to save the open sheet`

---

### T6: Save-in-flight guards

**What**: While a save is in flight, make the back action and F5 no-ops so an in-flight commit is never abandoned by a discard. Add widget tests that edit a field, start a save with a deferred response, activate back and F5, and assert no request is issued and the edits are kept; then let the response settle and assert the normal flow.
**Where**: `desktop/lib/sheet/sheet_screen.dart`
**Depends on**: T3
**Reuses**: `_leave`, `_refreshTemplate`, `isSaving` (T3), the deferred-response test pattern
**Requirement**: SAV-06

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Back while saving issues no request and does not discard the edits
- [ ] F5 while saving issues no template request and does not discard the edits
- [ ] After the save settles the guards behave as before
- [ ] Gate check passes: `flutter test test/sheet/sheet_screen_test.dart`
- [ ] Test count: recorded after the run (file total; +new, no silent deletions)

**Tests**: widget
**Gate**: quick

**Commit**: `fix(desktop): block discard while a sheet save is in flight`

---

## Phase Execution Map

```
Phase 1 → Phase 2 → Phase 3

T1
T2 -> T3 -> T4 -> T5
T3 -> T6
```

Execution is strictly sequential - there is no intra-phase parallelism. A single agent works one task at a time, in order.

**Packing:** 6 tasks total. That fits one task-budgeted batch (≤ ~8), so Execute runs inline in the main window - no batch sub-agents. The always-on Verifier still runs after the last task.

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Creation dialog bounded content | 1 layout fix + its regression test | ✅ Granular |
| T2: Public cascade merge helper | 1 engine function | ✅ Granular |
| T3: Save on the edit controller | 1 controller method + its state | ✅ Cohesive |
| T4: Screen 2 footer with the Save action | 1 footer widget + its strings | ✅ Cohesive |
| T5: Ctrl+S saves the open sheet | 1 binding + 1 dispatch seam | ✅ Cohesive |
| T6: Save-in-flight guards | 1 guard behavior in one screen | ✅ Granular |

**Granularity check**:

- ✅ 1 component / 1 function / 1 endpoint = Good
- ⚠️ 2-3 related things in same file = OK if cohesive
- ❌ Multiple components or files = MUST split

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | no arrow | ✅ Match |
| T2 | None | no arrow | ✅ Match |
| T3 | T2 | T2 -> T3 | ✅ Match |
| T4 | T3 | T3 -> T4 | ✅ Match |
| T5 | T4 | T4 -> T5 | ✅ Match |
| T6 | T3 | T3 -> T6 | ✅ Match |

Every `Depends on` points backward or within the same phase; no forward-phase dependency exists.

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1: Creation dialog bounded content | Creation dialog layout | widget | widget | ✅ OK |
| T2: Public cascade merge helper | Cascade merge | unit | unit | ✅ OK |
| T3: Save on the edit controller | Edit controller | unit | unit | ✅ OK |
| T4: Screen 2 footer with the Save action | Screen 2 footer | widget | widget | ✅ OK |
| T5: Ctrl+S saves the open sheet | Shortcut registry | widget | widget | ✅ OK |
| T6: Save-in-flight guards | Screen 2 guards | widget | widget | ✅ OK |

No test is deferred to another task. Each task ships its tests in the same commit.

---

## Task Verification Standards

Every task MUST follow the `Done when` + `Tests` + `Gate` fields defined above. Each `Done when` entry is binary and references a gate command from the **Gate Check Commands** section.

---

## Requirement Traceability

| Requirement ID | Task(s) | Status |
| -------------- | ------- | ------ |
| SAV-01 | T4 | Pending |
| SAV-02 | T4 | Pending |
| SAV-03 | T4 | Pending |
| SAV-04 | T4 | Pending |
| SAV-05 | T5 | Pending |
| SAV-06 | T4, T5, T6 | Pending |
| SAV-07 | T5 | Pending |
| SAV-08 | T3 | Pending |
| SAV-09 | T3 | Pending |
| SAV-10 | T3 | Pending |
| SAV-11 | T3 | Pending |
| SAV-12 | T3 | Pending |
| SAV-13 | T2, T3 | Pending |
| SAV-14 | T3 | Pending |
| SAV-15 | T3 | Pending |
| SAV-16 | T3 | Pending |
| SAV-17 | T3 | Pending |
| SAV-18 | T3 | Pending |
| SAV-19 | T3 | Pending |
| SAV-20 | T3 | Pending |
| SAV-21 | T3 | Pending |
| SAV-22 | T3 | Pending |
| SAV-23 | T3 | Pending |
| DIA-01 | T1 | Done |
| DIA-02 | T1 | Done |
| DIA-03 | T1 | Done |
| DIA-04 | T1 | Done |

**Coverage:** 27 total, 27 mapped to tasks, 0 unmapped.
