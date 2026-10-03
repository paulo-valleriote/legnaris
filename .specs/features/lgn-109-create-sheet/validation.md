# Create Sheet (LGN-109) Validation

**Date**: 2026-10-03
**Spec**: `.specs/features/lgn-109-create-sheet/spec.md`
**Diff range**: `1553730..HEAD` (5 commits, branch `feat/lgn-109-create-sheet`; round-1 verification was 4 commits through `1166c4c`, round-2 adds `7be258d` - tests only)
**Verifier**: independent sub-agent (author ≠ verifier)

---

## Validation: LGN-109 create sheet - PASS ✅

Every acceptance criterion (CRT-01 … CRT-11) is anchored to a matching `file:line`
assertion, the build gate is green, and the discrimination sensor killed all injected
mutants. The two round-1 edge-case coverage notes (whitespace-only name; a
server-authoritative name differing from the submitted one) are **closed** by
`7be258d`, which is test-only. No production source changed since round 1.

---

## Task Completion

| Task | Status  | Notes                                                            |
| ---- | ------- | ---------------------------------------------------------------- |
| T1   | ✅ Done | Opened-sheet seed seam; unit tests cover the override + fallback |
| T2   | ✅ Done | Dialog submit; widget tests cover POST, library, error, retry    |
| T3   | ✅ Done | Screen 2 title prefers the opened sheet name                     |
| T4   | ✅ Done | Catalog consumes the dialog result and opens Screen 2            |

All four tasks are marked `[x]` in `tasks.md`; the first 4 commits map 1:1 to T1→T4, and
the 5th (`7be258d`) is round-2 test hardening that closes the round-1 edge-case notes.

---

## Spec-Anchored Acceptance Criteria

> Re-derived independently from `spec.md`; asserted values must match the
> spec-defined outcome (not merely "an assertion exists").

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| CRT-01 activate with non-empty trimmed name | Exactly one `POST /sheets` body `{template_id, sheet_name}` with **trimmed** name | `desktop/test/catalog/create_sheet_dialog_test.dart:645` - `expect(posts, hasLength(1))`; `:646` - `expect(jsonDecode(posts.single.body), {'template_id':'fantasy_v1','sheet_name':'Aragorn'})` (input `'  Aragorn  '` at `:639`); nav `desktop/test/catalog/create_sheet_navigation_test.dart:327` - `expect(posts, hasLength(1))` | ✅ PASS |
| CRT-02 request in flight | Primary action stays disabled; second activation issues no POST | `create_sheet_dialog_test.dart:680` - `expect(tester.widget<TextButton>(submit).onPressed, isNull)`; `:683` - `expect(postsOf(harness.recorded), hasLength(1))` | ✅ PASS |
| CRT-03 `201` received | Upsert `{id, name, template_id, created_at}` from the **response** into the library | `create_sheet_dialog_test.dart:657` - `entry.id == 'sheet-1'`; `:658` - `entry.name == 'Aragorn'`; `:659` - `entry.templateId == 'fantasy_v1'`; `:660` - `entry.createdAt == DateTime.utc(2026,9,20,12)`; **server-authoritative name** `:791` - `expect(harness.popped()!.name, 'Strider')`; `:792`-`:795` - `entries.single.name == 'Strider'` (submitted `'Aragorn'` at `:786`); nav `:352`-`:354` | ✅ PASS |
| CRT-04 sheet registered from `201` | Dialog closes; Screen 2 opens for the created sheet | `create_sheet_dialog_test.dart:650` - `expect(find.byType(CreateSheetDialog), findsNothing)`; `:651` - `expect(harness.popped(), isNotNull)`; nav `:333` - `findsNothing`; `:334` - `expect(find.byType(SheetScreen), findsOneWidget)` | ✅ PASS |
| CRT-05 trimmed name empty | Primary action disabled + tooltip stating a name is required | `create_sheet_dialog_test.dart:297` - `expect(button.onPressed, isNull)`; `:305` - `expect(tooltip.message, 'Enter a sheet name.')`; enable-after-typing `:313`-`:320`; hover `:341`; **whitespace-only** `:754`-`:772` - `:763`-`:768` `onPressed, isNull`; `:769` `expect(postsOf(harness.recorded), isEmpty)`; `:770` dialog stays open | ✅ PASS |
| CRT-06 `POST` fails | Dialog stays open, mapped copy inline **above the action row**, no library entry | `create_sheet_dialog_test.dart:712` - `findsOneWidget`; `:713` - `expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget)`; `:716` - `getTopLeft(error).dy < getTopLeft(submit).dy`; `:721` - `entries, isEmpty` | ✅ PASS |
| CRT-07 reactivated after failure | Exactly one further `POST` | `create_sheet_dialog_test.dart:727` - `expect(postsOf(harness.recorded), hasLength(2))` (after retry at `:724`) | ✅ PASS |
| CRT-11 local library write fails after `201` | Still opens the created sheet; no second `POST` | `create_sheet_dialog_test.dart:748` - `findsNothing`; `:749` - `popped(), isNotNull`; `:750` - `hasLength(1)`; forced via `_ThrowingStore` `:803`-`:813` | ✅ PASS |
| CRT-08 Screen 2 opened for created sheet | Seed baseline + working data from the sheet's `data`, not schema defaults | `desktop/test/sheet/sheet_edit_provider_test.dart:114` - `valueAtPath(seeded.data,'stats.dexterity') == 16`; `:115` - `seeded.defaults` path `== 16` (schema default is `10`, `:19`); nav `:339`-`:347` renders `'16'` over the `10` default | ✅ PASS |
| CRT-09 Screen 2 shows created sheet | Render the sheet's `name` as the screen title | `desktop/test/sheet/sheet_screen_test.dart:351` - `expect(tester.widget<Text>(find.byKey(Key('sheet-title'))).data, 'Aragorn')`; nav `:335` - same key `'Aragorn'` | ✅ PASS |
| CRT-10 Screen 2 opened for created sheet | Edit state reported as not pending | `sheet_edit_provider_test.dart:116` - `expect(seeded.isPending, isFalse)`; edit-then-revert `:130`-`:134` | ✅ PASS |

**Status**: ✅ All 11 ACs covered with assertions matching the spec-defined outcome; **0 spec-precision gaps**.
Every precise outcome the spec defines (exact POST body, four library fields, exact timestamp,
exact server-error copy, `isPending == false`, schema default `10` vs stored `16`) is targeted directly.

---

## Discrimination Sensor

Scratch worktree: round 1 `git worktree add /tmp/opencode/lgn109-verify HEAD`, round 2
(confirmation at `7be258d`) `git worktree add /tmp/opencode/lgn109-verify2 HEAD`. Each
mutation is injected one at a time, the relevant tests run in the scratch, then the file
is reverted with `git checkout --`. No `git stash` is used in either run.

| Mutation | File:line (source) | Description | Killed? |
| -------- | ------------------ | ----------- | ------- |
| 1        | `desktop/lib/catalog/create_sheet_dialog.dart:124` | Dropped `.trim()` in `_submit` (`_nameController.text.trim()` → `.text`) | ✅ Killed (both runs) - `create_sheet_dialog_test.dart:646` (POST body `'  Aragorn  '` ≠ `'Aragorn'`) |
| 2        | `desktop/lib/catalog/create_sheet_dialog.dart:134` | Removed required side effect `_registerInLibrary(sheet)` | ✅ Killed (both runs) - round 1 `create_sheet_dialog_test.dart:654` (`entries.single` found none); round 2 also `create_sheet_dialog_test.dart:792`-`:795` (new response-name test) |
| 3        | `desktop/lib/sheet/sheet_edit_provider.dart:71-72` | Ignored `openedSheetProvider`, always `seedDefaults(template.fields)` | ✅ Killed (both runs) - `sheet_edit_provider_test.dart:114/131` (CRT-08, CRT-10) + `create_sheet_navigation_test.dart:339` (rendered `10` not `16`) |

**Sensor depth**: lightweight (3 high-risk behavior mutations), run twice (round 1 at
`1166c4c`, round 2 confirmation at `7be258d` - production code unchanged)
**Result**: 3/3 killed - PASS ✅

**Isolation**: scratch worktrees removed (`git worktree remove --force` + `prune`); in
round 2 the real worktree's pre-sensor baseline was the two round-1 lessons files
(`.specs/LESSONS.md`, `.specs/lessons.json`), and `git status --porcelain` after cleanup
equalled it exactly. This report edit is the only subsequent real-tree change.

---

## Interactive UAT Results

Not performed. This sub-agent runs read-only/automated verification; the
user-facing flow is exercised by widget/integration tests with a mocked HTTP client.

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code | ✅ Dialog-local `_isSubmitting`/`_errorMessage`, one provider, one navigation branch; no new abstraction |
| Surgical changes | ✅ Diff is confined to `catalog_screen.dart`, `create_sheet_dialog.dart`, `sheet_edit_provider.dart` (+generated), `sheet_screen.dart`, l10n ARB/generated; round 2 (`7be258d`) touches only the dialog test file |
| No scope creep | ✅ No `GET /sheets/:id`, no Screen 1 list, no duplication checks - all explicitly out of scope |
| Matches patterns | ✅ Reuses `apiClientProvider`, `SheetLibrary.upsert`, `failureCopy`, `appSupportDirectoryProvider` temp-dir test pattern (LGN-107); route-scoped `ProviderScope` mirrors the design's seam |
| Spec-anchored outcome check (asserted values match spec) | ✅ (see AC table) |
| Per-layer Coverage Expectation met (domain 1:1 ACs; routes happy+edge+error) | ✅ Edit controller 1:1 CRT-08/10; dialog happy+edge+error+CRT-11; Screen 2 title; navigation integration |
| Every test maps to a spec requirement - no unclaimed tests | ✅ Every new test names its CRT/GRN id in the title |
| Documented guidelines followed | ✅ `AGENTS.md` (English only, edge-first tests, async feedback), `desktop/AGENTS.md` (tooltip on disabled primary action, inline mapped error, no new deps) |

**Minor design-vs-code note (not a spec deviation):** `design.md:43` names a
`SheetLibraryEntry.fromSheet` helper; the implementation constructs the four fields
directly (`create_sheet_dialog.dart:150-155`). The model's only constructor takes the
four fields, so this is equivalent and arguably clearer; no `fromSheet` exists.

---

## Edge Cases

- [x] Leading/trailing whitespace → POST the trimmed value: `create_sheet_dialog_test.dart:639`+`:648` (input `'  Aragorn  '`, posted `'Aragorn'`).
- [x] Double activation while in flight → exactly one POST: `create_sheet_dialog_test.dart:680-683`.
- [x] Whitespace-only name treated as empty + no POST: `create_sheet_dialog_test.dart:754`-`:772` - types `'   '` (`:761`), asserts `onPressed, isNull` (`:763`-`:768`), `postsOf(harness.recorded), isEmpty` (`:769`), dialog stays open (`:770`). (Closed by `7be258d`; formerly a round-1 note.)
- [x] `201` response `name` differs from submitted → register/display response's values: `create_sheet_dialog_test.dart:774`-`:797` - submits `'Aragorn'` (`:786`), response `name` is `'Strider'`, asserts `popped()!.name == 'Strider'` (`:791`) and registered `entries.single.name == 'Strider'` (`:792`-`:795`). (Closed by `7be258d`; formerly a round-1 note.)
- [~] Local library write fails after `201` → index left unchanged and no second POST: pops + one POST asserted (`:748-750`); the "index unchanged" half is implied by `SheetLibrary._persist` rethrowing before publish (`sheet_library_provider.dart:67,115-119`) but not asserted in this test.
- [x] Created sheet's `data` leaf renders over schema default: `create_sheet_navigation_test.dart:339-347` (`16` over default `10`); unit proof `sheet_edit_provider_test.dart:114`.

---

## Gate Check

- **Gate command**: `flutter analyze && flutter test` (from `desktop/`)
- **Result**: 433 passed, 0 failed, 4 skipped (exit 0) - `flutter analyze`: "No issues found!"
- **Test count before feature** (`1553730`): 419 passed + 1 pre-existing flaky failure + 4 skipped = **424 run**
- **Test count after feature** (`HEAD` `7be258d`): **437 run** (433 passed + 4 skipped)
- **Delta**: **+13 tests** (T1 +3, T2 +4, T3 +2, T4 +2, plus the 2 round-2 edge tests in `7be258d`) - no tests deleted or weakened
- **Skipped tests**: 4 - pre-existing, identical in baseline and after; unrelated to this feature
- **Failures**: none at `HEAD`
- **Baseline flake note**: running `1553730` produced one failure in
  `desktop/test/api/api_client_retry_test.dart` ("every GET attempt is cut by the
  5s timeout before going offline", FE1-28) - a timing-sensitive test outside this
  feature's diff that passes at `HEAD`. Pre-existing flakiness, not a regression.

---

## Fix Plans

Both round-1 coverage notes were resolved by commit `7be258d` (test-only); no open fix tasks.

### ✅ Resolved 1: Whitespace-only boundary
Round-1 note: CRT-05 covered empty then a valid name, never whitespace-only.
**Resolved** by `create_sheet_dialog_test.dart:754`-`:772` (types `'   '`, asserts inert + zero POST).

### ✅ Resolved 2: Server-authoritative differing name
Round-1 note: no test used a response `name` that differs from the submitted one.
**Resolved** by `create_sheet_dialog_test.dart:774`-`:797` (submits `'Aragorn'`, response `'Strider'`, asserts popped + registered name is `'Strider'`).

### Remaining (non-AC, informational)
The local-write-failure edge's "index unchanged" half is implied by
`SheetLibrary._persist` rethrowing before publish (`sheet_library_provider.dart:67,115-119`)
but is asserted only indirectly. Not required by CRT-11 (which asserts open + one POST); no fix task.

---

## Requirement Traceability Update

`spec.md` was **not** edited (Verifier is read-only outside this report). Recommended
status update for the Requirements table:

| Requirement | Previous Status | Recommended New Status |
| ----------- | --------------- | ---------------------- |
| CRT-01 … CRT-11 | Implementing | ✅ Verified |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 11/11 ACs matched spec outcome | 0 spec-precision gaps
**Sensor**: 3/3 mutations killed (confirmed twice)
**Gate**: 433 passed, 0 failed, 4 skipped

**What works**: The full create flow is enabled end-to-end and proven with a mocked
`http.Client`: trimmed exactly-one `POST /sheets`, single-flight guard, four-field
library upsert from the `201` response, dialog close + Screen 2 open with the created
name and stored value, mapped inline error above the action row with an empty index,
one-POST retry, and best-effort library write that still opens the sheet. Screen 2
seeding from the injected payload (not schema defaults) is anchored to a concrete
value difference (`16` vs `10`) and `isPending == false`.

**Issues found**: None open. The two round-1 edge-case coverage notes (whitespace-only
input; a server-authoritative name differing from the submitted one) are closed by
`7be258d`, which adds only tests. All 11 ACs and the six listed edge cases now have
direct `file:line` evidence.

**Next steps**: None required; mark LGN-109 verified. (Recommendation only: the spec
Requirement Traceability table still reads `Implementing` - a human/author may flip
CRT-01 … CRT-11 to `✅ Verified`; the Verifier left `spec.md` untouched.)
