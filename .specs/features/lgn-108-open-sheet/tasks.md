# Open Sheet from the Library Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/lgn-108-open-sheet/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: `AGENTS.md` (edge cases first + happy path, typed errors), `desktop/AGENTS.md` (English only, reuse app infrastructure, async feedback), `.github/workflows/ci-flutter.yml` (`flutter analyze`, `flutter test`, `flutter build windows --release`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Seed derivation (`seedFromSheet`) | unit | Stored value wins, missing key keeps schema default, stored key absent from schema carried; 1:1 to OPN-10 | `desktop/test/engine/sheet_defaults_test.dart` | `flutter test test/engine/sheet_defaults_test.dart` |
| Open controller (`SheetOpenController`) | unit | Injected short-circuit, one GET + loading, 404 removal + `isNotFound`, 5xx no removal, single-flight Retry; 1:1 to OPN-05…09 | `desktop/test/sheet/sheet_open_provider_test.dart` | `flutter test test/sheet/sheet_open_provider_test.dart` |
| Edit controller (`SheetEditController`) | unit | Family keyed by sheet id; seeding overlays sheet data over schema defaults; `isPending == false`; injection path; 1:1 to OPN-10/11 | `desktop/test/sheet/sheet_edit_provider_test.dart` | `flutter test test/sheet/sheet_edit_provider_test.dart` |
| Screen 2 (`SheetScreen`) | widget | Loading, stored values over defaults, title, 404 message without Retry, 5xx + Retry, back/F5 guards on a sheet id, GET-only; 1:1 to OPN-05…19 | `desktop/test/sheet/sheet_screen_test.dart` | `flutter test test/sheet/sheet_screen_test.dart` |
| Library section (`LibrarySection`) | widget | List order, empty state, remove dialog + zero API, cancel, local write failure; 1:1 to OPN-01…03, 12…15 | `desktop/test/catalog/library_section_test.dart` | `flutter test test/catalog/library_section_test.dart` |
| Catalog + navigation (`CatalogScreen`) | widget (integration) | No preview action; Open pushes Screen 2 with one sheet GET; create flow issues zero sheet GETs; 1:1 to OPN-04, 09, 20, 21 | `desktop/test/catalog/catalog_screen_test.dart`, `desktop/test/catalog/create_sheet_navigation_test.dart` | `flutter test test/catalog/` |

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

### Phase 1: Foundation

The seed derivation and the open controller are independent leaves.

```
T1
T2
```

### Phase 2: Screen 2 contract

Screen 2 opens a stored sheet by id; the preview path and the template-id family are removed.

```
T1 -> T3
T2 -> T3
```

### Phase 3: Library surface

```
T4
```

### Phase 4: Catalog integration

```
T3 -> T5
T4 -> T5
```

---

## Task Breakdown

### T1: Per-key sheet seeding

**What**: Add `seedFromSheet(FieldGroup fields, Map<String, Object?> data)` to the engine: build `seedDefaults(fields)` and deep-overlay the sheet's stored values so a stored value wins, a key missing from `data` keeps the schema default, and a stored key absent from the schema is carried over.
**Where**: `desktop/lib/engine/sheet_defaults.dart`
**Depends on**: None
**Reuses**: `seedDefaults`, `setPathValue`, `FieldGroup`
**Requirement**: OPN-10

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] A stored value overrides the schema default at its dotted path
- [x] A schema leaf missing from `data` keeps the schema default
- [x] A stored key absent from the schema survives the merge with its nested shape
- [x] The result is a fresh tree; mutating it does not mutate the input `data`
- [x] Gate check passes: `flutter test test/engine/sheet_defaults_test.dart`
- [x] Test count: 14 tests pass (file total; +5 new, no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): seed a sheet draft from stored data over schema defaults`

---

### T2: Sheet open controller

**What**: Add `desktop/lib/sheet/sheet_open_provider.dart`: `SheetOpenState { sheet, isLoading, errorMessage, isNotFound }` and an autoDispose `SheetOpenController` family keyed by sheet id. `build` short-circuits to the injected `openedSheetProvider` sheet when its id matches the key (no GET), otherwise schedules a single-flight `load()` that calls `ApiClient.getSheet`; on `ApiErrorKind.notFound` it removes the entry from `sheetLibraryProvider` (swallowing `SheetLibraryException`) and publishes `isNotFound`; any other failure publishes the mapped `failureCopy`; `load()` is re-invoked by Retry with the single-flight guard. Regenerate `sheet_open_provider.g.dart`.
**Where**: `desktop/lib/sheet/sheet_open_provider.dart`
**Depends on**: None
**Reuses**: `apiClientProvider`, `openedSheetProvider`, `failureCopy`/`ApiErrorKind` (`desktop/lib/api/api_error.dart`, `error_mapper.dart`), `sheetLibraryProvider.notifier.remove` (LGN-107), the `TemplateDetailController` state shape
**Requirement**: OPN-05, OPN-06, OPN-07, OPN-08, OPN-09

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] With an injected `openedSheetProvider` whose id matches, no GET is issued and the state carries the sheet
- [x] Without injection, `build` starts loading and one `GET /sheets/:id` is issued
- [x] A `200` publishes the decoded sheet
- [x] A `404` removes the id from the library (zero further requests) and publishes `isNotFound` with the missing-sheet copy
- [x] A `500`/transport failure keeps the library entry and publishes the mapped message
- [x] Two rapid `load()` calls issue exactly one in-flight GET; a later Retry issues one further GET
- [x] Gate check passes: `flutter test test/sheet/sheet_open_provider_test.dart`
- [x] Test count: 9 tests pass (new file; every listed case present)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): load an opened sheet by id with not-found eviction`

---

### T3: Screen 2 opens a stored sheet

**What**: Retarget the edit state to the sheet id and make Screen 2 render a stored sheet. `SheetEditController.build(String sheetId)` watches `sheetOpenControllerProvider(sheetId)` and, once a sheet is present, `templateDetailControllerProvider(sheet.templateId)`, seeding `defaults`/`data` from `seedFromSheet(template.fields, sheet.data)`. `SheetScreen({required sheetId})` watches the open controller: loading while the sheet (and then the template) resolve; the missing-sheet copy with no Retry on `isNotFound`; `failureCopy` + Retry otherwise; the `SheetForm`, title, local-edits marker, back guard, F5 refresh and `activeSheetIdProvider` registration all key off the sheet id. `CatalogScreen` drops the "Preview sheet" action and its handler and passes a sheet id to `_openSheetScreen`; the creation flow pushes `SheetScreen(sheetId: created.id)` in the existing route-scoped `ProviderScope`. Update `openedSheetProvider`/`SheetEditController` doc comments, remove the obsolete `catalogPreviewAction` string, add the new sheet strings (`sheetLoading`, `sheetMissingMessage`), and regenerate l10n + codegen. Update the edit, screen and create-navigation tests to the new key and keep every existing assertion.
**Where**: `desktop/lib/sheet/sheet_edit_provider.dart`, `desktop/lib/sheet/sheet_screen.dart`, `desktop/lib/catalog/catalog_screen.dart`, `desktop/lib/sheet/sheet_refresh.dart`, `desktop/lib/l10n/app_en.arb`
**Depends on**: T1, T2
**Reuses**: `sheetOpenControllerProvider` (T2), `seedFromSheet` (T1), `SheetForm`, `errorCopy`, `AppShell` route pattern, AD-004 route-scoped override
**Requirement**: OPN-05, OPN-06, OPN-07, OPN-08, OPN-10, OPN-11, OPN-16, OPN-17, OPN-18, OPN-19, OPN-20, OPN-21

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] The edit family is keyed by the sheet id; opening a sheet seeds data from `Sheet.data` over the schema defaults and reports not pending
- [ ] Screen 2 shows a loading state, then the sheet's name and stored values
- [ ] A `404` renders the missing-sheet copy with no Retry; a `500` renders the mapped copy with Retry and keeps the entry
- [ ] The local-edits marker, back guard and F5 guard read the sheet-id provider and keep their existing behavior
- [ ] No "Preview sheet" action remains; a catalog card offers only Create sheet
- [ ] The creation flow mounts Screen 2 for the created sheet with zero `/sheets/:id` GETs
- [ ] No `POST`/`PATCH` is issued while a sheet is open
- [ ] Gate check passes: `flutter test` (whole suite; the signature change is cross-file)
- [ ] Test count: edit, screen and create-navigation files green with the existing assertions preserved (no silent deletions)

**Tests**: unit + widget (integration)
**Gate**: full

**Commit**: `feat(desktop): open a stored sheet on screen 2 by sheet id`

---

### T4: "My sheets" library section

**What**: Add `desktop/lib/catalog/library_section.dart`: a `LibrarySection` widget that watches `sheetLibraryProvider` and renders the "My sheets" header plus one 32px row per entry (`name` + `template_id` + Open calling `onOpen(entry)` + destructive Remove). Empty library renders one localized line and no rows. Remove opens a destructive confirmation stating the removal is local and the sheet stays in the API; confirm calls `sheetLibraryProvider.notifier.remove(id)` (zero API requests), cancel keeps the row; a `SheetLibraryException` renders an inline `libraryRemoveFailed` and keeps the row. Add the library l10n strings and regenerate.
**Where**: `desktop/lib/catalog/library_section.dart`, `desktop/lib/l10n/app_en.arb`
**Depends on**: None
**Reuses**: `sheetLibraryProvider`, `SheetLibraryEntry`, `SheetLibraryException`, `AppSpacing`/`AppSizes`, the AlertDialog confirmation pattern from `sheet_screen.dart`
**Requirement**: OPN-01, OPN-02, OPN-03, OPN-12, OPN-13, OPN-14, OPN-15

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Two stored entries render two rows in `created_at` order with name, template id and both actions
- [ ] An empty library renders the single empty-state line and no rows
- [ ] Remove shows the confirmation with the local-only consequence; confirm drops the row with zero recorded requests; cancel keeps it with zero requests
- [ ] A forced store write failure keeps the row and renders the inline error
- [ ] Gate check passes: `flutter test test/catalog/library_section_test.dart`
- [ ] Test count: new file, every listed case present

**Tests**: widget
**Gate**: quick

**Commit**: `feat(desktop): list and remove library sheets on screen 1`

---

### T5: Catalog embeds My sheets and opens from it

**What**: Embed `LibrarySection` on Screen 1 above the catalog inside one shared scroll region (so a long library never pushes the catalog off screen), pass `onOpen` a handler that pushes `AppShell(content: SheetScreen(sheetId: entry.id))` with no override, and override `appSupportDirectoryProvider` in the catalog test helpers so the library section has a store.
**Where**: `desktop/lib/catalog/catalog_screen.dart`, `desktop/test/catalog/catalog_screen_test.dart`, `desktop/test/catalog/create_sheet_navigation_test.dart`
**Depends on**: T3, T4
**Reuses**: `LibrarySection` (T4), `SheetScreen(sheetId:)` (T3), `AppShell`, the LGN-107 temp-directory test pattern
**Requirement**: OPN-04, OPN-09

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Screen 1 renders the "My sheets" section above the catalog
- [ ] Activating Open on a row mounts Screen 2 and issues exactly one `GET /sheets/:id`, rendering the stored values
- [ ] The catalog still loads and refreshes with the library section present
- [ ] The create flow still opens the created sheet with zero `/sheets/:id` GETs
- [ ] Gate check passes: `flutter test` (whole suite; the catalog surface is cross-file)
- [ ] Test count: catalog and create-navigation files green with the existing assertions preserved (no silent deletions)

**Tests**: widget (integration)
**Gate**: full

**Commit**: `feat(desktop): open library sheets from the screen 1 list`

---

## Phase Execution Map

Visual representation of task ordering. Phases run in sequence, and tasks within a phase run in order:

```
Phase 1 → Phase 2 → Phase 3 → Phase 4

T1
T2
T1 -> T3
T2 -> T3
T3 -> T5
T4 -> T5
```

Execution is strictly sequential - there is no intra-phase parallelism. A single agent (or batch worker) works one task at a time, in order.

**Packing:** 5 tasks total. That fits one task-budgeted batch (≤ ~8), so Execute runs inline in the main window - no batch sub-agents. The always-on Verifier still runs after the last task.

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Per-key sheet seeding | 1 engine function | ✅ Granular |
| T2: Sheet open controller | 1 provider family | ✅ Cohesive |
| T3: Screen 2 opens a stored sheet | 1 contract change (family key + screen identity), applied to its call sites | ⚠️ Cross-cutting by nature; no sub-behavior can ship without the others (a half-retarget leaves the app semantically broken) |
| T4: "My sheets" library section | 1 widget + its state | ✅ Cohesive |
| T5: Catalog embeds My sheets and opens from it | 1 navigation glue | ✅ Granular |

**Granularity check**:

- ✅ 1 component / 1 function / 1 endpoint = Good
- ⚠️ 2-3 related things in same file = OK if cohesive
- ❌ Multiple components or files = MUST split

T3 is one cohesive contract change: the edit family key and Screen 2's identity must change together or the screen reads a state nobody seeds. Splitting it would produce a non-compiling or semantically broken intermediate commit, which the Execution Contract forbids.

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | no arrow | ✅ Match |
| T2 | None | no arrow | ✅ Match |
| T3 | T1, T2 | T1 -> T3, T2 -> T3 | ✅ Match |
| T4 | None | no arrow | ✅ Match |
| T5 | T3, T4 | T3 -> T5, T4 -> T5 | ✅ Match |

Every `Depends on` points backward or within the same phase; no forward-phase dependency exists.

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1: Per-key sheet seeding | Seed derivation | unit | unit | ✅ OK |
| T2: Sheet open controller | Open controller | unit | unit | ✅ OK |
| T3: Screen 2 opens a stored sheet | Edit controller + Screen 2 + navigation | unit + widget (integration) | unit + widget (integration) | ✅ OK |
| T4: "My sheets" library section | Library section | widget | widget | ✅ OK |
| T5: Catalog embeds My sheets and opens from it | Catalog + navigation | widget (integration) | widget (integration) | ✅ OK |

No test is deferred to another task. Each task ships its tests in the same commit.

---

## Task Verification Standards

Every task MUST follow the `Done when` + `Tests` + `Gate` fields defined above. Each `Done when` entry is binary and references a gate command from the **Gate Check Commands** section. Include the expected test count to prevent silent deletions.

---

## Requirement Traceability

| Requirement ID | Task(s) | Status |
| -------------- | ------- | ------ |
| OPN-01 | T4 | Pending |
| OPN-02 | T4 | Pending |
| OPN-03 | T4 | Pending |
| OPN-04 | T5 | Pending |
| OPN-05 | T2, T3 | Pending |
| OPN-06 | T2, T3 | Pending |
| OPN-07 | T2, T3 | Pending |
| OPN-08 | T2, T3 | Pending |
| OPN-09 | T2, T3, T5 | Pending |
| OPN-10 | T1, T3 | Pending |
| OPN-11 | T3 | Pending |
| OPN-12 | T4 | Pending |
| OPN-13 | T4 | Pending |
| OPN-14 | T4 | Pending |
| OPN-15 | T4 | Pending |
| OPN-16 | T3 | Pending |
| OPN-17 | T3 | Pending |
| OPN-18 | T3 | Pending |
| OPN-19 | T3 | Pending |
| OPN-20 | T3 | Pending |
| OPN-21 | T3 | Pending |

**Coverage:** 21 total, 21 mapped to tasks, 0 unmapped.
