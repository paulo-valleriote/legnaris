# Create Sheet Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/lgn-109-create-sheet/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: `AGENTS.md` (edge cases first + happy path, typed errors), `desktop/AGENTS.md` (English only, reuse app infrastructure, async feedback), `.github/workflows/ci-flutter.yml` (`flutter analyze`, `flutter test`, `flutter build windows --release`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Edit controller (`SheetEditController` opened-sheet seeding) | unit | Opened-seed branch (baseline + data + `isPending == false`) and the unchanged no-override branch; 1:1 to CRT-08/CRT-10 | `desktop/test/sheet/sheet_edit_provider_test.dart` | `flutter test test/sheet/sheet_edit_provider_test.dart` |
| Creation dialog (`CreateSheetDialog`) | widget | Empty-name inert + tooltip; trimmed exactly-one POST; in-flight guard; mapped inline error with no library entry; retry = one further POST; store-failure still opens; every listed edge case | `desktop/test/catalog/create_sheet_dialog_test.dart` | `flutter test test/catalog/create_sheet_dialog_test.dart` |
| Screen 2 header (`SheetScreen`) | widget | Title prefers the opened sheet name; falls back to the template name when no sheet is opened | `desktop/test/sheet/sheet_screen_test.dart` | `flutter test test/sheet/sheet_screen_test.dart` |
| Navigation glue (`CatalogScreen`) | widget (integration) | Full flow: `201` → library entry persisted → dialog closed → Screen 2 mounted with the created name and value; preview path pushes Screen 2 with no sheet | `desktop/test/catalog/create_sheet_navigation_test.dart` | `flutter test test/catalog/create_sheet_navigation_test.dart` |

**Note on `Tests: none`:** no task here is `none`; every task creates or modifies a tested layer.

## Gate Check Commands

> Generated from codebase - confirm before Execute. All commands run from `desktop/` (the CI works in `desktop` per `.github/workflows/ci-flutter.yml`).

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | After tasks with unit/widget tests on one file | `flutter test test/<area>/<file>.dart` |
| Full | After tasks that regenerate shared l10n/codegen or wire cross-file behavior | `flutter test` |
| Build | At phase completion / final validation | `flutter analyze && flutter test` |

---

## Execution Plan

Phases are ordered and run sequentially - each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Foundation

The seed seam and the dialog submit are independent leaves, ordered for readability.

```
T1
T2
```

### Phase 2: Screen 2 title

```
T3
```

### Phase 3: Navigation integration

```
T4
```

---

## Task Breakdown

### T1: Opened-sheet seed seam

**What**: Add `openedSheetProvider` (`@Riverpod(keepAlive: true) Sheet? openedSheet(Ref ref) => null;`) and make `SheetEditController.build` seed the edit-state baseline and working data from `openedSheet.data` when the override is present, falling back to `seedDefaults(template.fields)`; regenerate `sheet_edit_provider.g.dart` with build_runner.
**Where**: `desktop/lib/sheet/sheet_edit_provider.dart`
**Depends on**: None
**Reuses**: `seedDefaults` (`desktop/lib/engine/sheet_defaults.dart`), `_copyTree`, `Sheet` (`desktop/lib/api/models/sheet.dart`)
**Requirement**: CRT-08, CRT-10

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Reading `openedSheetProvider` without an override returns null
- [x] With an override, the controller's `data` and `defaults` deep-equal the injected `Sheet.data` and `isPending` is false
- [x] A path present in the injected data renders its value over the template schema default
- [x] With no override, seeding is unchanged and the existing `sheet_edit_provider` tests stay green
- [x] Gate check passes: `flutter test test/sheet/sheet_edit_provider_test.dart`
- [x] Test count: 15 tests pass (file total; +3 new, no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): seed the sheet edit state from an opened sheet`

---

### T2: Creation dialog submit

**What**: Enable the dialog's primary action: disabled while the trimmed name is empty or a request is in flight, with a name-required tooltip; `_submit()` issues one `POST /sheets {template_id, sheet_name}` with the trimmed name, upserts the returned `{id,name,template_id,created_at}` into the local library, and pops with the created `Sheet`; a failure renders the mapped `failureCopy` inline above the action row and keeps the dialog open; a local `SheetLibraryException` after a `201` is swallowed so the sheet still opens with no second POST. Replace the obsolete `createSheetCreateTooltip` with `createSheetNameRequiredTooltip` and regenerate l10n.
**Where**: `desktop/lib/catalog/create_sheet_dialog.dart`
**Depends on**: None
**Reuses**: `apiClientProvider`, `SheetLibrary.upsert` (`desktop/lib/library/sheet_library_provider.dart`), `SheetLibraryEntry`, `failureCopy` (`desktop/lib/api/api_error.dart`)
**Requirement**: CRT-01, CRT-02, CRT-03, CRT-05, CRT-06, CRT-07, CRT-11

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Empty / whitespace-only name keeps the action disabled and shows the name-required tooltip; no POST
- [ ] A non-empty name enables the action; one activation issues exactly one POST whose body carries the trimmed `sheet_name` and the `template_id`
- [ ] A second activation while in flight issues no additional POST
- [ ] On `201` the library index gains the entry's four fields and the dialog pops with the created sheet
- [ ] On failure the dialog stays open, renders the mapped copy inline, and the index is unchanged; a retry issues exactly one further POST
- [ ] A forced library write failure after a `201` still pops with the sheet and issues no second POST
- [ ] Gate check passes: `flutter test`
- [ ] Test count: [N] tests pass (no silent deletions)

**Tests**: widget
**Gate**: full

**Commit**: `feat(desktop): submit sheet creation from the dialog`

---

### T3: Screen 2 title from the opened sheet

**What**: The Screen 2 header title prefers the opened sheet name (`openedSheetProvider`), falling back to the template name.
**Where**: `desktop/lib/sheet/sheet_screen.dart`
**Depends on**: T1
**Reuses**: `openedSheetProvider` (T1)
**Requirement**: CRT-09

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] With an opened-sheet override the header shows the sheet's `name`
- [ ] With no override the header still shows the template's `name`
- [ ] Gate check passes: `flutter test test/sheet/sheet_screen_test.dart`
- [ ] Test count: [N] tests pass (no silent deletions)

**Tests**: widget
**Gate**: quick

**Commit**: `feat(desktop): show the opened sheet name on screen 2`

---

### T4: Catalog opens the created sheet

**What**: `_openCreationDialog` consumes the dialog result via `showDialog<Sheet>` and, after restoring the catalog surface, pushes Screen 2 for the created sheet wrapped in a `ProviderScope` that overrides `openedSheetProvider`; `_openSheetScreen` takes an optional sheet and the preview path passes none.
**Where**: `desktop/lib/catalog/catalog_screen.dart`
**Depends on**: T1, T2, T3
**Reuses**: existing `_openSheetScreen` and `AppShell` route pattern
**Requirement**: CRT-03, CRT-04

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Submitting the dialog creates the sheet, persists the library entry, closes the dialog, and mounts Screen 2 showing the created name and the created value
- [ ] The preview action still opens Screen 2 with schema-default values and no library entry
- [ ] Gate check passes: `flutter test`
- [ ] Test count: [N] tests pass (no silent deletions)

**Tests**: widget (integration)
**Gate**: full

**Commit**: `feat(desktop): open the created sheet from the catalog`

---

## Phase Execution Map

Visual representation of task ordering. Phases run in sequence, and tasks within a phase run in order:

```
Phase 1 → Phase 2 → Phase 3

T1
T2
T1 -> T3
T1 -> T4
T2 -> T4
T3 -> T4
```

Execution is strictly sequential - there is no intra-phase parallelism. A single agent (or batch worker) works one task at a time, in order.

**Packing:** 4 tasks total. That fits one task-budgeted batch (≤ ~8), so Execute runs inline in the main window - no batch sub-agents. The always-on Verifier still runs after the last task.

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Opened-sheet seed seam | 1 provider + 1 notifier branch | ✅ Cohesive |
| T2: Creation dialog submit | 1 dialog widget (its local state + l10n string) | ✅ Cohesive |
| T3: Screen 2 title | 1 header line | ✅ Granular |
| T4: Catalog opens the created sheet | 1 navigation glue | ✅ Granular |

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
| T3 | T1 | T1 -> T3 | ✅ Match |
| T4 | T1, T2, T3 | T1 -> T4, T2 -> T4, T3 -> T4 | ✅ Match |

Every `Depends on` points backward or within the same phase; no forward-phase dependency exists.

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1: Opened-sheet seed seam | Edit controller | unit | unit | ✅ OK |
| T2: Creation dialog submit | Creation dialog | widget | widget | ✅ OK |
| T3: Screen 2 title | Screen 2 header | widget | widget | ✅ OK |
| T4: Catalog opens the created sheet | Navigation glue | widget (integration) | widget (integration) | ✅ OK |

No test is deferred to another task. Each task ships its tests in the same commit.

---

## Task Verification Standards

Every task MUST follow the `Done when` + `Tests` + `Gate` fields defined above. Each `Done when` entry is binary and references a gate command from the **Gate Check Commands** section. Include the expected test count to prevent silent deletions.
