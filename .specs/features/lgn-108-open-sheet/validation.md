# Open Sheet from the Library Validation

**Date**: 2026-10-03
**Spec**: `.specs/features/lgn-108-open-sheet/spec.md`
**Diff range**: `7508d4d..HEAD` (`f8a740c` docs + `1229ce4`, `2b9cf1a`, `cdd0b02`, `0b2e3e4`, `d9a8119`)
**Verifier**: independent sub-agent (author ≠ verifier)

---

## Task Completion

| Task | Status  | Notes |
| ---- | ------- | ----- |
| T1: Per-key sheet seeding | ✅ Done | `seedFromSheet` implemented; 9→14 tests in `sheet_defaults_test.dart` (+5). |
| T2: Sheet open controller | ✅ Done | New `sheet_open_provider.dart` + 9 unit tests; single-flight, 404 eviction, mapped errors. |
| T3: Screen 2 opens a stored sheet | ✅ Done | Edit family retargeted to sheet id; screen/widget tests 34→37; no assertions weakened (family key `fantasy_v1`→`sheet-1` only). |
| T4: "My sheets" library section | ✅ Done | New `library_section.dart` + 6 widget tests; dialog, zero-API removal, inline failure. |
| T5: Catalog embeds My sheets and opens from it | ✅ Done | `LibrarySection` embedded in the shared scroll region; new `library_open_navigation_test.dart` (2 tests). |

---

## Spec-Anchored Acceptance Criteria

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| OPN-01 WHEN Screen 1 shown THEN list every stored entry, newest first | rows in `created_at` descending | `desktop/test/catalog/library_section_test.dart:73` - `expect(find.text('My sheets'), findsOneWidget)`; `:78` - `expect(tester.getTopLeft(newer).dy, lessThan(tester.getTopLeft(older).dy), reason: 'entries must read newest-first')` | ✅ PASS |
| OPN-02 WHEN library empty THEN single localized line, no rows | one empty line, no entry rows | `desktop/test/catalog/library_section_test.dart:115` - `expect(find.text('No sheets created yet'), findsOneWidget)`; `:117` - `expect(find.byType(TextButton), findsNothing)` | ✅ PASS |
| OPN-03 WHEN entry listed THEN name + template_id + Open + Remove | both fields and both actions per row | `desktop/test/catalog/library_section_test.dart:83` - `expect(find.text('fantasy_v1'), findsNWidgets(2))`; `:84` - `expect(find.byKey(const Key('library-open-new')), findsOneWidget)`; `:85` - `expect(find.byKey(const Key('library-remove-new')), findsOneWidget)` | ✅ PASS |
| OPN-04 WHEN Open activated THEN navigate to Screen 2 for that id | one `GET /sheets/:id`, Screen 2 shows stored sheet | `desktop/test/catalog/library_open_navigation_test.dart:86` tap `library-open-sheet-1`; `:89` - `expect(find.byType(SheetScreen), findsOneWidget)`; `:90`-`:93` - `expect(recorded.where((r) => r.url.path == '/sheets/sheet-1'), hasLength(1))`; `:95`-`:97` title `'Aria'`; `:98`-`:106` stored `'16'`. Also `library_section_test.dart:106` - `expect(opened, ['id-1'])` | ✅ PASS |
| OPN-05 WHEN Screen 2 opens for a library sheet THEN one `GET /sheets/:id` + loading until settled | exactly one GET, `isLoading`/loading copy visible | `desktop/test/sheet/sheet_open_provider_test.dart:102` - `expect(stateOf(...).isLoading, isTrue)`; `:105` - `expect(recorded, hasLength(1))`; `:107` - `expect(recorded.single.url.path, '/sheets/sheet-1')`. `desktop/test/sheet/sheet_screen_test.dart:300`-`:301` - `LinearProgressIndicator` + `'Loading sheet...'`; `:313`-`:316` one GET | ✅ PASS |
| OPN-06 IF `GET /sheets/:id` NOT_FOUND THEN remove entry, mapped missing-sheet message, no Retry | `isNotFound` true, entry evicted, localized copy, retry null | `desktop/test/sheet/sheet_open_provider_test.dart:199` - `expect(state.isNotFound, isTrue)`; `:201`-`:204` - `errorMessage == lookupAppLocalizations(const Locale('en')).sheetMissingMessage`; `:205` - `expect(container.read(sheetLibraryProvider).entries, isEmpty)`. `desktop/test/sheet/sheet_screen_test.dart:352`-`:357` mapped copy; `:358` - `expect(find.widgetWithText(OutlinedButton, 'Retry'), findsNothing)` | ✅ PASS |
| OPN-07 IF `GET /sheets/:id` fails non-404 THEN keep entry, keep screen, mapped message + Retry | entry length 1, no eviction, mapped copy, Retry present | `desktop/test/sheet/sheet_open_provider_test.dart:246` - `expect(state.isNotFound, isFalse)`; `:248` - `expect(state.errorMessage, errorCopy(ApiErrorKind.serverError))`; `:249` - `expect(container.read(sheetLibraryProvider).entries, hasLength(1))`; `:250` - `expect(store.saveCalls, 0)`. `desktop/test/sheet/sheet_screen_test.dart:382`-`:383` mapped copy + Retry | ✅ PASS |
| OPN-08 WHEN Retry activated after non-404 THEN exactly one further GET (single-flight) | two rapid loads = 1 request; later Retry = 1 more | `desktop/test/sheet/sheet_open_provider_test.dart:297` - `expect(recorded, hasLength(1))`; `:306`-`:308` - after `load()` `expect(recorded, hasLength(2))`. `desktop/test/sheet/sheet_screen_test.dart:388`-`:391` - `hasLength(2)` after Retry | ✅ PASS |
| OPN-09 WHEN creation flow opens the created sheet THEN render from injected 201, no `GET /sheets/:id` | injected sheet used, zero requests | `desktop/test/sheet/sheet_open_provider_test.dart:158` - `expect(stateOf(...).sheet, same(injected))`; `:161` - `expect(recorded, isEmpty)`. Integration: `desktop/test/catalog/create_sheet_navigation_test.dart:320` one POST; `:327`-`:340` created title `'Aragorn'` + stored `'16'` (handler 404s any `/sheets/sheet-1` GET) | ✅ PASS |
| OPN-10 WHEN seeding opened sheet THEN stored value wins / missing key keeps schema default | per-leaf overlay | `desktop/test/engine/sheet_defaults_test.dart:92`-`:96` - `{stats: {dexterity: 16, constitution: 12.0}}`; `:100`-`:111` unknown key carried. `desktop/test/sheet/sheet_edit_provider_test.dart:188`-`:195` - `16` over default, `12.0` fallback, `1.0`/`0.0` defaults | ✅ PASS |
| OPN-11 WHEN library sheet response settles THEN edit state not pending | `isPending == false` | `desktop/test/sheet/sheet_edit_provider_test.dart:140` - `expect(seeded.isPending, isFalse)`; `:196` - `expect(seeded.isPending, isFalse)` | ✅ PASS |
| OPN-12 WHEN Remove activated THEN confirmation stating local-only, stays in API | dialog title + consequence copy | `desktop/test/catalog/library_section_test.dart:140` - `expect(find.text('Remove from local library?'), findsOneWidget)`; `:141`-`:144` - `expect(find.textContaining('stays in the API'), findsOneWidget)` | ✅ PASS |
| OPN-13 WHEN confirmed THEN entry removed, zero API requests | row gone, store save 1, no requests | `desktop/test/catalog/library_section_test.dart:154` - `expect(find.byKey(const Key('library-name-id-1')), findsNothing)`; `:156` - `expect(store.saveCalls, 1)`; `:157` - `expect(recorded, isEmpty, reason: 'removal is local-only (OPN-13)')` | ✅ PASS |
| OPN-14 IF confirmation dismissed THEN keep entry, zero API requests | row present, no save, no requests | `desktop/test/catalog/library_section_test.dart:177` - `expect(find.byKey(const Key('library-name-id-1')), findsOneWidget)`; `:178` - `expect(store.saveCalls, 0)`; `:179` - `expect(recorded, isEmpty)` | ✅ PASS |
| OPN-15 IF local write fails on removal THEN keep entry + inline library error | row present, error key, no requests | `desktop/test/catalog/library_section_test.dart:203` - `expect(find.byKey(const Key('library-name-id-1')), findsOneWidget)`; `:204` - `expect(find.byKey(const Key('library-remove-error')), findsOneWidget)`; `:205` - `expect(recorded, isEmpty)` | ✅ PASS |
| OPN-16 WHILE sheet open THEN exactly one active sheet, edit state discarded on pop | autoDispose state gone after pop; one active registration | `desktop/test/sheet/sheet_screen_test.dart:844`-`:848` - `expect(container.exists(sheetEditControllerProvider(sheetId)), isFalse, reason: 'popping must discard the autoDispose edit state')`; `:954`-`:955` active-sheet registration; `:977` cleared on unmount | ✅ PASS |
| OPN-17 WHEN back with pending edits on an opened sheet THEN existing discard confirmation | dialog + consequence, no navigation | `desktop/test/sheet/sheet_screen_test.dart:806` - `expect(find.byType(AlertDialog), findsOneWidget)`; `:808`-`:812` exact leave copy; `:813` - `expect(find.byType(SheetScreen), findsOneWidget)`; `:814` unchanged request count. **Library-fetch path (`injectSheet: false`, serves `GET /sheets/sheet-1` then `GET /templates/fantasy_v1`):** `:960`-`:981`; `:973` AlertDialog; `:974`-`:977` exact leave copy; `:980` screen stays | ✅ PASS |
| OPN-18 WHEN F5 with pending edits on an opened sheet THEN existing discard confirmation | dialog + consequence, no reload before confirm | `desktop/test/sheet/sheet_screen_test.dart:1044` - `expect(find.byType(AlertDialog), findsOneWidget)`; `:1046`-`:1050` exact refresh copy; `:1051`-`:1055` - `expect(detailGets, 1)` + `expect(recorded, hasLength(1))`. **Library-fetch path:** `:985`-`:1014`; `:1002` AlertDialog; `:1003`-`:1006` exact refresh copy; `:1009`-`:1013` - `expect(detailGets, 1, reason: 'no template reload until the user confirms')` | ✅ PASS |
| OPN-19 WHILE Screen 2 shows an opened library sheet THEN no mutation request | every recorded request is GET | `desktop/test/sheet/sheet_screen_test.dart:247` - `expect(recorded.where((request) => request.method != 'GET'), isEmpty)`; `:470`-`:471` - `hasLength(1)` + `single.method == 'GET'`; `:1264`, `:1437`, `:1506`, `:1577` GET-only. Unit: `desktop/test/sheet/sheet_edit_provider_test.dart:243`-`:244` | ✅ PASS |
| OPN-20 WHEN catalog card renders THEN no "Preview sheet" action | no preview text/key; only Create sheet | `desktop/test/catalog/create_sheet_navigation_test.dart:357` - `expect(find.text('Preview sheet'), findsNothing)`; `:358` - `expect(find.byKey(const Key('catalog-preview-fantasy_v1')), findsNothing)`; `:360`-`:362` two `Create sheet` actions | ✅ PASS |
| OPN-21 WHILE Screen 2 mounted THEN require stored sheet identity, no schema-only fake data | title from sheet, never template fallback | `desktop/test/sheet/sheet_screen_test.dart:515`-`:518` - `expect(tester.widget<Text>(find.byKey(const Key('sheet-title'))).data, '')` for an empty sheet name (not `'Fantasy Template'`); `create_sheet_navigation_test.dart:357`-`:364` no fake-data path, library stays empty | ✅ PASS |

**Status**: ✅ All 21 ACs covered with spec-anchored `file:line` evidence. The two prior minor notes are closed in re-verification round 1 (`f610afa`): OPN-17/OPN-18 now also reach the guards through the non-injected library-fetch path (`sheet_screen_test.dart:960`-`:981` and `:985`-`:1014`), and the "keep entry after template-detail failure" half is directly asserted at `library_open_navigation_test.dart:154`-`:160`.

---

## Discrimination Sensor

Scratch: `git worktree add --detach /tmp/opencode/lgn108-sensor HEAD` (real tree untouched; no stash). Mutations applied one at a time, relevant test file(s) run, then reverted. Scratch removed with `git worktree remove --force`.

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| 1 | `desktop/lib/sheet/sheet_open_provider.dart:54` | Flipped injected-id condition `injected.id == sheetId` → `!=` | ✅ Killed — `sheet_open_provider_test.dart` failed 2 cases (matching-id used-without-request; non-matching ignored-and-fetched) |
| 2 | `desktop/lib/engine/sheet_defaults.dart:35` | Reversed overlay precedence `_overlay(seedDefaults(fields), data)` → `_overlay(data, seedDefaults(fields))` (default wins over stored) | ✅ Killed — `sheet_defaults_test.dart` failed 3 cases (stored wins, unknown key carried, deep merge) |
| 3 | `desktop/lib/sheet/sheet_open_provider.dart:77` | Removed the `_evict()` side effect in the `NOT_FOUND` branch | ✅ Killed — `sheet_open_provider_test.dart:205` failed (`entries` not empty) |
| 4 | `desktop/lib/catalog/library_section.dart:145` | Removed the `sheetLibraryProvider.notifier.remove(entry.id)` call on confirm | ✅ Killed — `library_section_test.dart` failed 2 cases (row not dropped; write-failure path inverted) |

**Sensor depth**: lightweight (4 behavior-level mutations across the 3 highest-risk new/changed units)
**Result**: 4/4 killed — PASS ✅
**Isolation**: real-tree `git status --porcelain` was empty before injection and empty after scratch removal (`git worktree list` back to the single worktree).

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code | ✅ `seedFromSheet` reuses `seedDefaults`/`setPathValue`; `SheetOpenController` mirrors `TemplateDetailState`. |
| Surgical changes | ✅ Screen-2 retarget is a mechanical `templateId`→sheet-id change; every pre-existing assertion kept (diff shows only family-key/seed-source edits). |
| No scope creep | ✅ No save/PATCH, no DELETE, no tabs, no caching — matches Out of Scope. |
| Matches patterns | ✅ Reuses `ApiClient`, `failureCopy`/`ApiErrorKind`, `sheetLibraryProvider`, AD-004 route-scoped override, `AppSpacing`/`AppSizes`; l10n through `.arb`. |
| Spec-anchored outcome check (asserted values match spec) | ✅ See AC table; exact copy/status/state asserted. |
| Per-layer Coverage Expectation met (domain 1:1 ACs; routes happy+edge+error) | ✅ Unit (seed/open), widget (section/screen), integration (catalog→screen) all present. |
| Every test maps to a spec requirement — no unclaimed tests | ✅ New tests map to OPN-01…21 / listed edge cases; no speculative tests found. |
| Documented guidelines followed | ✅ `desktop/AGENTS.md` (English, dialog states consequence, token sizes, no new deps), root `AGENTS.md` (edge-cases-first, typed errors). |
| `flutter analyze` | ✅ No issues found. |

**Tests deleted/weakened**: only `desktop/test/sheet/sheet_navigation_test.dart` (2 tests) was removed — its premise (the "Preview sheet" action, RND-14) was removed by design/spec. The re-homed RND-18 "back returns to the cached catalog without a new list request" now lives at `desktop/test/catalog/create_sheet_navigation_test.dart:368` (`expect(listRequestCount(recorded), 1)`). No assertions weakened elsewhere; family-key edits are like-for-like.

**Minor note (non-blocking)**: `sheet_open_provider.dart:100` keeps a one-line why-comment inside the catch (explaining the swallowed eviction failure); `library_section.dart:119` mirrors the pre-existing load-bearing barrier comment style in `sheet_screen.dart`. Consistent with the surrounding code, not a new violation.

---

## Edge Cases

- [x] Unknown stored `Sheet.data` key is carried over: `desktop/test/engine/sheet_defaults_test.dart:100` (`'notInSchema': {'nested': 1}` survives); nested merge `:114`.
- [x] Template leaf missing from `Sheet.data` renders the schema default: `desktop/test/engine/sheet_defaults_test.dart:85`; `sheet_edit_provider_test.dart:189`-`:193`.
- [x] Same sheet opened → popped → reopened re-issues one GET (no cache): `desktop/test/sheet/sheet_open_provider_test.dart:120` (`hasLength(2)` after disposal + re-listen).
- [x] Library removal write fails → zero API requests: `desktop/test/catalog/library_section_test.dart:183` (`recorded, isEmpty`).
- [x] `NOT_FOUND` eviction write fails → mapped message still renders, no crash: `desktop/test/sheet/sheet_open_provider_test.dart:210` (`isNotFound` true, entry kept).
- [x] Template detail failure after sheet loaded → mapped message + Retry **and** the library entry stays: `desktop/test/catalog/library_open_navigation_test.dart:152` - `expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget)`; `:153` - Retry present; `:154`-`:160` - `expect(container.read(sheetLibraryProvider).entries.map((e) => e.id), contains('sheet-1'), reason: 'a template failure must not evict the local entry')` (non-injected library-fetch path). Existing mapped+Retry coverage also at `desktop/test/sheet/sheet_screen_test.dart:253` and `:1155`.
- [x] Opening retried while a request is in flight → exactly one outstanding: `desktop/test/sheet/sheet_open_provider_test.dart:272` (`hasLength(1)` for two rapid `load()`s).

---

## Gate Check

- **Gate command**: `flutter test` (Build gate per tasks.md; also ran `flutter analyze`)
- **Result**: `flutter test --concurrency=1` (re-verification round 1): **461 passed, 0 failed, 4 skipped** — `All tests passed!` (earlier parallel `flutter test`: 458 passed / 4 skipped / 0 failed — both green)
- **`flutter analyze`**: **No issues found** (2.9s)
- **Test count before feature** (`7508d4d`, static `test(`/`testWidgets(` declarations): 436
- **Test count after feature**: 460 static at `d9a8119` (+24); `f610afa` adds +3 test-only (2 in `sheet_screen_test.dart`, 1 in `library_open_navigation_test.dart`) → 463 static; interpreter total 461 passed + 4 skipped = 465
- **Delta**: +27 tests net of the 2 removed preview-navigation tests
- **Skipped tests**: `desktop/test/contract/live_contract_test.dart` ×4 via `contractSkip(base)` — pre-existing live/contract skips, unrelated to this feature.
- **Failures**: none. The known wall-clock `desktop/test/api/api_client_retry_test.dart` flake passed in both the parallel and the serial (`--concurrency=1`) runs.

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ---------- |
| OPN-01 … OPN-21 | Implementing | ✅ Verified |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 21/21 ACs matched spec outcome (0 spec-precision gaps; the 2 prior minor notes are closed in re-verification round 1 — see below)

**Sensor**: 4/4 mutations killed

**Gate**: 461 passed, 0 failed, 4 skipped (pre-existing) — serial re-run; also green in parallel

**What works**: Screen 1 "My sheets" list (newest-first, empty state, name+template id+Open/Remove); open flow with exactly one `GET /sheets/:id` and loading state; per-key seeding (stored wins, unknown key carried, missing keeps default); `NOT_FOUND` eviction with no-Retry mapped message; non-404 keeps entry with Retry and single-flight re-request; creation flow still injects the `201` with zero sheet GETs; back/F5 discard guards on the sheet id; no mutations; "Preview sheet" removed; analyze clean.

**Issues found**: none blocking.

**Re-verification round 1 (`f610afa`)**: The two prior minor notes are closed. (1) OPN-17 and OPN-18 are now also asserted through the **non-injected library-fetch path** — `desktop/test/sheet/sheet_screen_test.dart:960`-`:981` (back-with-pending opens the discard confirmation, exact leave copy, screen stays) and `:985`-`:1014` (F5-with-pending opens the confirmation and `detailGets == 1`, i.e. no reload before confirm). (2) The template-detail-failure retention half is now directly asserted at `desktop/test/catalog/library_open_navigation_test.dart:152`-`:160` (mapped `serverError` copy + Retry + `sheetLibraryProvider.entries` still `contains('sheet-1')`). The commit is test-only and purely additive (`git show f610afa --stat`: 121 insertions, 0 deletions across the 2 test files) — **no existing assertion was weakened**. Serial gate `flutter test --concurrency=1`: 461 passed / 0 failed / 4 skipped. Verdict remains **PASS**.

**Next steps**: None required; feature is verified and ready for distillation/orchestration.
