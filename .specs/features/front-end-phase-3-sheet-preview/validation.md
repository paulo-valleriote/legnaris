# front-end-phase-3-sheet-preview Validation

**Date**: 2026-10-01
**Spec**: `.specs/features/front-end-phase-3-sheet-preview/spec.md`
**Diff range**: `a540877..HEAD` (fc702b1, 3e51f28, aebd38a, 5c047dc, 50b9f2f)
**Verifier**: independent sub-agent (author ≠ verifier)

**Validation result**: PASS ✅ (re-verification after fix iteration 1)

---

## Task Completion

| Task | Status     | Notes   |
| ---- | ---------- | ------- |
| T1   | ✅ Done    | `seedDefaults`/`setPathValue` extracted to `desktop/lib/engine/sheet_defaults.dart`; `desktop/lib/sheet/sheet_screen.dart` consumes them (behavior-identical). 8 unit tests. |
| T2   | ✅ Done    | Tokens `createSheetDialogWidth=720`, `previewCardWidth=360`, `previewCardMaxHeight=420` (`desktop/lib/theme/tokens.dart:85-93`); 1 unit test. |
| T3   | ✅ Done    | Two-column dialog + read-only preview card (`desktop/lib/catalog/create_sheet_dialog.dart:40-215`); l10n `createSheetPreviewLabel`. |
| T4   | ✅ Done    | `template_summary.dart` + `template_summary_test.dart` (11 tests) deleted; phase-2 spec amended. |
| T5   | ✅ Done    | Fix task from iteration 1 (test-only, commit `50b9f2f`): closed PRV-07/PRV-09/PRV-10/PRV-13 coverage gaps. No production code changed. |

---

## Spec-Anchored Acceptance Criteria

### P1: Card de preview ao lado da informação do diálogo

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| PRV-01: GET succeeds → preview card with one labeled section per top-level key, in API order, incl. `config`/`derived`/`modifiers` | 5 sections, order = API insertion order | `desktop/test/catalog/create_sheet_preview_test.dart:115-127` - `expect(inPreview(find.byKey(Key('sheet-section-$key'))), findsOneWidget)` for baseArmorClass/config/derived/modifiers/stats; `:129-139` - `expect(order, orderedEquals([...order]..sort()))`; `desktop/test/catalog/create_sheet_dialog_test.dart:148-163` - reserved sections `findsOneWidget` | ✅ PASS |
| (PRV-01 edge) WHEN a top-level group has no leaves THEN omit that section | `sheet-section-empty` absent, `stats` present | `desktop/test/catalog/create_sheet_preview_test.dart:176-183` - `findsNothing` for `sheet-section-empty`, `findsOneWidget` for `sheet-section-stats` | ✅ PASS |
| PRV-02: every leaf shows display name (schema label else humanized key) + base value from `FieldSchema.default` | `Dexterity score`→`10`, `Constitution`→`12` | `desktop/test/catalog/create_sheet_preview_test.dart:149-150` - `find.text('Dexterity score')`/`find.text('Constitution')`; `:151-168` - `TextField.controller?.text == '10'` / `'12'` | ✅ PASS |
| PRV-03: derive sections/labels/values only from `FieldSchema`; any template renders | synthetic unknown prefixes/types render with humanized names + value | `desktop/test/catalog/create_sheet_preview_test.dart:192-206` - `Weird prefix z`, `Custom thing`, `Another one`, `controller?.text == '3'` | ✅ PASS |
| PRV-04: WHILE in flight → loading, no stale fields | loading shown, no sections; loading clears on success | `desktop/test/catalog/create_sheet_preview_test.dart:251-256` - `find.text('Loading template...')` `findsOneWidget`/`findsNothing`, `sheet-section-stats` `findsNothing`; `desktop/test/catalog/create_sheet_dialog_test.dart:212-221` - same | ✅ PASS |
| PRV-05: dialog lists fields only through preview; Phase-2 text summary NOT rendered | preview card present, `Stats: ` absent | `desktop/test/catalog/create_sheet_dialog_test.dart:112-124` - `create-sheet-preview` `findsOneWidget`, `sheet-section-stats` `findsOneWidget`, `find.textContaining('Stats: ')` `findsNothing` | ✅ PASS |
| PRV-06: no control accepts edit; no mutation; transport only GET | all preview `TextField.enabled == false`, all `Switch.onChanged == null`, zero POST | `desktop/test/catalog/create_sheet_preview_test.dart:218-230` - `expect(textFields.every((f) => f.enabled == false), isTrue)`, `expect(switches.every((s) => s.onChanged == null), isTrue)`, `expect(recorded.every((r) => r.method == 'GET'), isTrue)`, `expect(recorded.any((r) => r.method == 'POST'), isFalse)` | ✅ PASS |

### P1: Estados de erro e vazio na área do preview

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| PRV-07: detail fails (transport / 4xx / 5xx / undecodable) → mapped message + Retry inline, dialog open | mapped copy per kind + Retry + dialog open | 500: `desktop/test/catalog/create_sheet_dialog_test.dart:349-356` - `errorCopy(serverError)` + `OutlinedButton, 'Retry'` + dialog open; 404: `:367-374` - `errorCopy(notFound)` + Retry; undecodable (no data key): `:385-393` - `parseCopy()` + Retry `:387-390` + no section; undecodable (malformed): `:404-412` - `parseCopy()` + Retry `:406-409`; offline transport: `:433-445` - `errorCopy(ApiErrorKind.offline)` + Retry `:438-441` + no section, dialog open | ✅ PASS (all four failure kinds + transport failure now assert copy + Retry + open) |
| PRV-08: Retry → re-request exactly once per activation | 1 GET after open, 2 after Retry; preview swaps in; Retry gone | `desktop/test/catalog/create_sheet_dialog_test.dart:472` - `expect(detailRequestCount(recorded,'fantasy_v1'), 1)`; `:478` - `expect(..., 2)`; `:479-486` - `sheet-section-stats` `findsOneWidget`, `Retry` `findsNothing` | ✅ PASS |
| PRV-09: no renderable field (empty `fields`, or only groups without leaves) → localized "This template has no fields." | line shown for BOTH inputs, no field control | empty `fields`: `desktop/test/catalog/create_sheet_preview_test.dart:266-269`; groups-only: `:279-283` - `find.text('This template has no fields.')` `findsOneWidget` + `inPreview(find.byType(TextField))` `findsNothing`; `desktop/test/catalog/create_sheet_dialog_test.dart:173-176` | ✅ PASS |

### P2: Duas colunas sem quebrar o diálogo

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| PRV-10: card bounds its height and scrolls | `SingleChildScrollView` height ≤ `previewCardMaxHeight`; name field reachable | `desktop/test/catalog/create_sheet_preview_test.dart:296-305` - `expect(scroll, findsOneWidget)`, `expect(tester.getSize(scroll).height, lessThanOrEqualTo(AppSizes.previewCardMaxHeight))`, name-input present and center `dy < view height`; narrow window `:319-334` - `AlertDialog` width `lessThanOrEqualTo(700)`, preview card + name input present, scroll height ≤ `previewCardMaxHeight` | ✅ PASS |
| PRV-11: at most one `GET /templates/:id` per activation | exactly 1 GET | `desktop/test/catalog/create_sheet_dialog_test.dart:137-138` - `expect(detailRequests, hasLength(1))`, `expect(detailRequests.single.method, 'GET')` | ✅ PASS |
| PRV-12: reopened for same id → fresh GET, no cache | 2 GETs across open→close→reopen | `desktop/test/catalog/create_sheet_dialog_test.dart:501` - `expect(detailRequestCount(recorded,'fantasy_v1'), 2)` | ✅ PASS |
| PRV-13: typing name → preview unchanged AND no API request | preview identical + zero new requests | `desktop/test/catalog/create_sheet_dialog_test.dart:260` - `expect(recorded.length, before)`; `:261-264` - `sheet-section-stats` `findsOneWidget`; `:265-273` - `sheet-input-stats.dexterity` controller text still `'10'` | ✅ PASS |

**Status**: ✅ All 13 ACs covered; asserted values match the spec-defined outcomes.

---

## Discrimination Sensor

Scratch worktree `/tmp/opencode/lgn100-sensor2` (HEAD = 50b9f2f), removed with `git worktree remove --force`; real-tree `git status --porcelain` matched the pre-sensor baseline (only `.claude/`, `.cursor/`, `.windsurf/` untracked skill dirs).

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| 1        | `desktop/lib/catalog/create_sheet_dialog.dart:213` | `onChanged: null` → `onChanged: (path, value) {}` (re-enable preview controls) | ✅ Killed — PRV-06 test `create_sheet_preview_test.dart:213` failed |
| 2        | `desktop/lib/catalog/create_sheet_dialog.dart:206-208` | Remove the `buildSections(...).isEmpty` → no-fields branch | ✅ Killed — 3 PRV-09 tests failed (empty `fields` x2, groups-only x1) |
| 3        | `desktop/lib/catalog/create_sheet_dialog.dart:212` | `data: seedDefaults(template.fields)` → `data: const {}` (starve the preview) | ✅ Killed — PRV-02 test `create_sheet_preview_test.dart:144` and the new PRV-13 assertion `create_sheet_dialog_test.dart:248` failed |

**Sensor depth**: lightweight (3 behavior-level mutations, targeting the new/critical preview code)
**Result**: 3/3 mutants killed ✅

---

## Interactive UAT Results (if performed)

Not performed: read-only, widget-testable surface; automated checks treated as sufficient per `validate.md` §3. No human-judgment-only flow requested.

---

## Code Quality

| Principle        | Status |
| ---------------- | ------ |
| Minimum code     | ✅ |
| Surgical changes | ✅ (`50b9f2f` is tests-only + `tasks.md`; `git show --stat 50b9f2f` = 2 test files + tasks.md; no production file touched) |
| No scope creep   | ✅ |
| Matches patterns | ✅ (same `MockClient` + Riverpod override harness; `Happy path:`/`Edge:`/`Error:` prefixes; added `viewSize` parameter mirrors existing `catalog_screen_test.dart` `viewport`) |
| Spec-anchored outcome check (asserted values match spec) | ✅ (all 13 ACs cite a `file:line` asserting the precise outcome) |
| Per-layer Coverage Expectation met (domain 1:1 ACs; routes happy+edge+error) | ✅ domain/widget |
| Every test maps to a spec requirement - no unclaimed tests | ✅ (new tests: groups-only → PRV-09, narrow-window → PRV-10 + min-width edge, offline → PRV-07) |
| Documented guidelines followed: `desktop/AGENTS.md`, `.claude/skills/tlc-spec-driven/references/coding-principles.md` | ✅ (schema-driven via `SheetForm`, no per-system literals, tokens not magic numbers, English l10n, no new dependencies — `desktop/pubspec.yaml` unchanged) |

New tests assessed for quality (no new gap):
- **Narrow-window** (`create_sheet_preview_test.dart:309-336`): deterministic (fixed 700×800 viewport, fake time); asserts dialog width ≤ window, both columns present, card bounded. It does not assert the horizontal ordering (x-name < x-preview), only co-presence — acceptable since the dialog content is a `Row` and the width clamp is proven.
- **Offline** (`create_sheet_dialog_test.dart:417-447`): pumps 350 ms, ahead of the fixed 300 ms `retryDelay` (`desktop/lib/api/api_client.dart:29`), so the single retry is deterministic under fake time; asserts the mapped offline copy, Retry, dialog open, and no preview section. No stale-loading state can co-render (error branch is exclusive of `isLoading`).
- **Groups-only** and **PRV-13**: both assert widget state/values, not call counts.

Minor documentation note (non-blocking): `spec.md:125` still does not name a numeric "minimum supported width"; the test pins 700 and the design clamps the fixed 720 token to the parent width, so the behavior is verified even though the spec leaves the number implicit.

---

## Edge Cases

- [x] Group with no leaves → section omitted: `desktop/test/catalog/create_sheet_preview_test.dart:172-184`.
- [x] Detail still failing when the dialog opens → mapped error + Retry: `desktop/test/catalog/create_sheet_dialog_test.dart:343-377` (500/404) and `:417-447` (offline).
- [x] Empty template name → layout renders (no throw): `desktop/test/catalog/create_sheet_dialog_test.dart:180-195`.
- [x] Minimum supported width → two columns side by side, card scrolls, dialog ≤ window: `desktop/test/catalog/create_sheet_preview_test.dart:309-336`. ⚠️ Spec-precision note: the spec does not name the numeric minimum.
- [x] Implicit dimension scan: Retry-per-activation (PRV-08), one request per activation (PRV-11), fresh reopen (PRV-12), no preview in flight (PRV-04), TPL-14 Esc-close refetches nothing (`desktop/test/catalog/create_sheet_dialog_test.dart:505-519`). N/A items correctly out of scope.

---

## Gate Check

- **Gate command**: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && flutter gen-l10n && flutter analyze && flutter test`
- **Result**: 255 passed, 0 failed, 4 skipped
- **`flutter analyze`**: `No issues found! (ran in 2.9s)`
- **Test count before feature**: 246 passed / 4 skipped
- **Test count after feature**: 255 passed / 4 skipped
- **Delta**: +9 net (20 added, 11 deleted). Added: `create_sheet_preview_test.dart` 10 (8 at T3 + groups-only and narrow-window at T5) + `sheet_defaults_test.dart` 8 + `tokens_test.dart` +1 + `create_sheet_dialog_test.dart` net +1 (offline at T5; the rest were rewrites of existing cases) = 20. Deleted: `template_summary_test.dart` 11. 246 + 20 − 11 = 255. No other test file was deleted (`git diff a540877..HEAD --diff-filter=D --name-only` returns only `template_summary.dart` and `template_summary_test.dart`).
- **Skipped tests**: 4, all in `desktop/test/contract/live_contract_test.dart:120,149,175,194`, guarded by `skip: contractSkip(base)` — pre-existing live-contract tests that self-skip without a running API. Justified, unchanged by this feature.
- **Failures**: none.

---

## Fix Plans (if issues found)

None — all four iteration-1 gaps are closed and no new gap was introduced.

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status   |
| ----------- | --------------- | ------------ |
| PRV-01 | Done | ✅ Verified |
| PRV-02 | Done | ✅ Verified |
| PRV-03 | Done | ✅ Verified |
| PRV-04 | Done | ✅ Verified |
| PRV-05 | Done | ✅ Verified |
| PRV-06 | Done | ✅ Verified |
| PRV-07 | Done | ✅ Verified |
| PRV-08 | Done | ✅ Verified |
| PRV-09 | Done | ✅ Verified |
| PRV-10 | Done | ✅ Verified |
| PRV-11 | Done | ✅ Verified |
| PRV-12 | Done | ✅ Verified |
| PRV-13 | Done | ✅ Verified |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 13/13 ACs matched the spec outcome (0 gaps; 1 non-blocking spec-precision note on the unnamed minimum window width).

**Sensor**: 3/3 mutations killed.

**Gate**: 255 passed, 0 failed, 4 skipped; `flutter analyze` clean.

**What works**: Two-column create dialog with a schema-driven, read-only preview seeded from `FieldSchema.defaultValue` via the shared `seedDefaults`; reserved groups render; loading/error/empty states live in the preview card; Retry is single-flight; the card scrolls within a bounded height and the dialog stays within a narrow window; the Phase-2 text summary is removed and the phase-2 spec is amended; no new dependencies.

**Issues found**: none. Iteration-1 gaps closed: PRV-13 now asserts preview state after typing (`create_sheet_dialog_test.dart:261-273`); PRV-09 drives a groups-only tree (`create_sheet_preview_test.dart:273-285`); the min-width edge has a 700×800 test (`:309-336`); PRV-07 asserts Retry on both undecodable payloads and adds a dialog-level offline test (`:379-447`).

**Next steps**: None. Feature verified; `validate_state.py` exits 0.
