# front-end-phase-3-sheet-preview Validation

**Date**: 2026-10-01
**Spec**: `.specs/features/front-end-phase-3-sheet-preview/spec.md`
**Diff range**: `d68de2a..HEAD` (origin/main..HEAD; feature commits 282baf5 → 155ff3b)
**Verifier**: independent sub-agent (author ≠ verifier)

**Validation result**: PASS ✅ (fix iteration 3; the previously-surviving clamp mutation is now killed)

---

## Task Completion

| Task | Status     | Notes   |
| ---- | ---------- | ------- |
| T1   | ✅ Done    | `seedDefaults`/`setPathValue` in `desktop/lib/engine/sheet_defaults.dart`; shared with `sheet_screen.dart`. 8 unit tests. |
| T2   | ✅ Done    | `createSheetDialogWidth`/`previewCardMaxHeight` tokens (`desktop/lib/theme/tokens.dart:86,90`). |
| T3   | ✅ Done    | Read-only preview card + l10n header; reserved groups; loading/error/empty states. |
| T4   | ✅ Done    | Phase-2 `template_summary.dart` + 11 tests deleted; phase-2 spec amended. |
| T5   | ✅ Done    | Iteration-1 fixes (groups-only, offline, PRV-13 preview state). |
| T6   | ✅ Done    | Single 560px column; `nome | id` row + hairline divider; header "Read-only preview". |
| T7   | ✅ Done    | `Scrollbar(thumbVisibility: true)` + shared `ScrollController`. |
| T8   | ✅ Done    | 600px narrow test measures the content surface. |
| T9   | ✅ Done    | Rendered-width assertions: card `== 560` wide, card `== content < 560` narrow (`155ff3b`). |

---

## Spec-Anchored Acceptance Criteria

### P1: Card de preview (stacked) — amend UAT 2026-10-01

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| PRV-01: GET succeeds → preview card **below the sheet-name input**, one labeled section per top-level key in API order incl. reserved groups; identity renders `nome | id` on one line | card below name; name/id same row w/ divider; 5 sections in API order | below-name: `desktop/test/catalog/create_sheet_preview_test.dart:228-234` - `expect(cardTop, greaterThan(nameTop))`; identity row: `:244-253` - `getCenter(id).dy closeTo(getCenter(name).dy, 1)`, `name.dx < id.dx`, divider `findsOneWidget`; sections/order: `:115-139`; reserved groups: `desktop/test/catalog/create_sheet_dialog_test.dart:148-163` | ✅ PASS |
| (PRV-01 edge) empty template name → layout with just the id, **no dangling separator** | id present, divider absent | `desktop/test/catalog/create_sheet_dialog_test.dart:186-197` - id `findsOneWidget`, divider `findsNothing`, section `findsOneWidget` | ✅ PASS |
| (PRV-01 edge) group with no leaves → section omitted | absent section, present sibling | `desktop/test/catalog/create_sheet_preview_test.dart:176-183` | ✅ PASS |
| PRV-02: every leaf shows display name + base value from `FieldSchema.default` | `Dexterity score`→`10`, `Constitution`→`12` | `desktop/test/catalog/create_sheet_preview_test.dart:149-168` | ✅ PASS |
| PRV-03: derive only from `FieldSchema`; any template renders | synthetic prefixes/types render | `desktop/test/catalog/create_sheet_preview_test.dart:192-206` | ✅ PASS |
| PRV-04: WHILE in flight → loading, no stale fields | loading shown, no sections; clears on success | `desktop/test/catalog/create_sheet_preview_test.dart:298-303`; `desktop/test/catalog/create_sheet_dialog_test.dart:212-225` | ✅ PASS |
| PRV-05: fields listed only through preview; Phase-2 summary NOT rendered | preview card present, `Stats: ` absent | `desktop/test/catalog/create_sheet_dialog_test.dart:108-124` | ✅ PASS |
| PRV-06: no control accepts edit; **card header states read-only**; transport only GET | header "Read-only preview"; all `TextField.enabled == false`; all `Switch.onChanged == null`; zero POST | header: `desktop/test/catalog/create_sheet_preview_test.dart:218` - `expect(inPreview(find.text('Read-only preview')), findsOneWidget)`; controls: `:265-274`; GET-only: `:276-277`; `desktop/test/catalog/create_sheet_dialog_test.dart:326-334` | ✅ PASS |

### P1: Estados de erro e vazio na área do preview

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| PRV-07: detail fails (transport / 4xx / 5xx / undecodable) → mapped message + Retry inline, dialog open | mapped copy per kind + Retry + dialog open | 500 `desktop/test/catalog/create_sheet_dialog_test.dart:353-361`; 404 `:371-379`; no-data-key `:389-398`; malformed `:408-417`; offline `:437-449` | ✅ PASS |
| PRV-08: Retry → re-request exactly once | 1 GET after open, 2 after Retry; preview swaps; Retry gone | `desktop/test/catalog/create_sheet_dialog_test.dart:476,482-490` | ✅ PASS |
| PRV-09: no renderable field (empty `fields`, or only groups without leaves) → localized line | line for both inputs | `desktop/test/catalog/create_sheet_preview_test.dart:313-316` and `:326-330`; `desktop/test/catalog/create_sheet_dialog_test.dart:173-176` | ✅ PASS |

### P2: Coluna única sem quebrar o diálogo

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| PRV-10: card scrolls within a bounded height **and exposes an always-visible scrollbar**; name field reachable | scroll height ≤ `previewCardMaxHeight`; `Scrollbar.thumbVisibility == true`; drag changes offset | `desktop/test/catalog/create_sheet_preview_test.dart:343-347` (`height <= AppSizes.previewCardMaxHeight`), `:355-356` (`bar.thumbVisibility, isTrue`), `:358-360` (`maxScrollExtent > 0`), `:362-364` (after `drag(0,-200)`, `position.pixels > 0`), `:366-369` (name input present) | ✅ PASS |
| PRV-10 (rendered width): the preview card renders at the 560px design width on a wide window | `card.width == createSheetDialogWidth` (560) | `desktop/test/catalog/create_sheet_preview_test.dart:342-346` - `expect(tester.getSize(card).width, AppSizes.createSheetDialogWidth)`; literal 560 pinned by `desktop/test/theme/tokens_test.dart:61` | ✅ PASS |
| PRV-10 (edge): window narrower than 640px → content **clamped to the window**, preview stays below the name, card scrolls with its scrollbar, never wider than the window | content clamped to available width; card fills it; below name | `desktop/test/catalog/create_sheet_preview_test.dart:373-407` at `Size(600, 800)`: `:388` `expect(content.width, lessThan(AppSizes.createSheetDialogWidth))`, `:389` `expect(tester.getSize(card).width, content.width)`, below-name `:391-395`, body height ≤ 420 `:396-399`, scrollbar visible `:400-405` | ✅ PASS |
| PRV-11: at most one `GET /templates/:id` per activation | exactly 1 GET | `desktop/test/catalog/create_sheet_dialog_test.dart:137-138` | ✅ PASS |
| PRV-12: reopened for same id → fresh GET | 2 GETs across open→close→reopen | `desktop/test/catalog/create_sheet_dialog_test.dart:505` | ✅ PASS |
| PRV-13: typing name → preview unchanged AND no request | preview identical + zero requests | `desktop/test/catalog/create_sheet_dialog_test.dart:264,265-277` | ✅ PASS |

**Status**: ✅ All 13 ACs and the sub-640 edge assert the spec-defined outcomes.

---

## Spec-Anchored Edge Cases

| Edge case (spec.md) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------- | -------------------- | ----------------------- | ------ |
| Group with no leaves → omit section (`:125`) | section absent | `desktop/test/catalog/create_sheet_preview_test.dart:176-183` | ✅ PASS |
| Detail still failing when dialog opens → mapped error + Retry (`:126`) | copy + Retry + dialog open | `desktop/test/catalog/create_sheet_dialog_test.dart:347-381,421-451` | ✅ PASS |
| Empty template name → layout with just the id, no dangling separator (`:127`) | id present, divider absent | `desktop/test/catalog/create_sheet_dialog_test.dart:186-197` | ✅ PASS |
| Window narrower than the 640px minimum → content clamped to the window, preview below name, scrollbar, never wider (`:128`) | `content.width < 560` and `card.width == content.width` at 600px | `desktop/test/catalog/create_sheet_preview_test.dart:388-389` | ✅ PASS |
| Implicit-dimension scan (`:129`) | PRV-08/11/12/04 + TPL-14 | `desktop/test/catalog/create_sheet_dialog_test.dart:509-523` | ✅ PASS |

---

## Discrimination Sensor

Scratch worktree `/tmp/opencode/lgn100-sensor5` (HEAD = 155ff3b), removed with `git worktree remove --force`; real-tree `git status --porcelain` matched the pre-sensor baseline (only `.claude/`, `.cursor/`, `.windsurf/` untracked skill dirs).

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| 1        | `desktop/lib/catalog/create_sheet_dialog.dart:43-55` | Delete the tight `SizedBox(width: AppSizes.createSheetDialogWidth)` (the mutation that **survived** in the previous pass) | ✅ **Killed** — wide test `create_sheet_preview_test.dart:336` fails (`card.width` 331 ≠ 560); narrow test `:373` also fails |
| 2        | `desktop/lib/catalog/create_sheet_dialog.dart:181` | `thumbVisibility: true` → `false` | ✅ Killed — scrollbar test `:336` failed |
| 3        | `desktop/lib/catalog/create_sheet_dialog.dart:50-52` | Swap the card above the sheet-name input | ✅ Killed — below-name test `:222` failed |
| 4        | `desktop/lib/theme/tokens.dart:86` | `createSheetDialogWidth` 560 → 500 | ✅ Killed — `desktop/test/theme/tokens_test.dart:61` failed |

**Sensor depth**: lightweight (4 behavior-level mutations, including the exact mutation that survived iteration 2)
**Result**: 4/4 mutants killed ✅ — the rendered-width gap is closed.

---

## Interactive UAT Results (if performed)

Not performed by the Verifier; the UAT amend records the maintainer's visual review.

---

## Code Quality

| Principle        | Status |
| ---------------- | ------ |
| Minimum code     | ✅ |
| Surgical changes | ✅ (doc comments at `create_sheet_dialog.dart:10-15,136-139` describe the single-column layout; only `155ff3b` adds one `Key` + width assertions) |
| No scope creep   | ✅ |
| Matches patterns | ✅ (same harness; tokens not magic numbers; header via l10n `app_en.arb:121`; no new dependencies — `desktop/pubspec.yaml` unchanged) |
| Spec-anchored outcome check (asserted values match spec) | ✅ |
| Per-layer Coverage Expectation met (domain 1:1 ACs; routes happy+edge+error) | ✅ |
| Every test maps to a spec requirement - no unclaimed tests | ✅ |
| Documented guidelines followed: `desktop/AGENTS.md`, `.claude/skills/tlc-spec-driven/references/coding-principles.md` | ✅ |

New-change assessment (no new gap):
- **Key collision**: `Key('create-sheet-content')` is used only at `create_sheet_dialog.dart:44` and `create_sheet_preview_test.dart:386` — unique.
- **Equality flakiness**: `expect(tester.getSize(card).width, content.width)` (`:389`) is deterministic — the card is stretched to the content column, so both render boxes receive the same width constraint and compute the same value; `devicePixelRatio` is fixed at 1 in the harness and fake time is used. No rounding risk observed across repeated runs.
- **Self-referential wide assertion**: `:344-345` compares the rendered card to `AppSizes.createSheetDialogWidth`; the literal `560` is pinned independently by `tokens_test.dart:61`, and mutation 4 confirms a token change is caught.

---

## Gate Check

- **Gate command**: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && flutter gen-l10n && flutter analyze && flutter test`
- **Result**: 291 passed, 0 failed, 4 skipped
- **`flutter analyze`**: `No issues found! (ran in 2.7s)`
- **Test count**: 291 passed / 4 skipped (as expected); no test deleted.
- **Skipped tests**: 4, all in `desktop/test/contract/live_contract_test.dart:120,149,175,194` (`skip: contractSkip(base)`) — pre-existing live-contract tests, self-skip without a running API.
- **Failures**: none.

---

## Fix Plans (if issues found)

None — the iteration-2 gap is closed and no new gap was introduced.

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status   |
| ----------- | --------------- | ------------ |
| PRV-01 | Pending (amend UAT) | ✅ Verified |
| PRV-02 | Done | ✅ Verified |
| PRV-03 | Done | ✅ Verified |
| PRV-04 | Done | ✅ Verified |
| PRV-05 | Done | ✅ Verified |
| PRV-06 | Pending (amend UAT) | ✅ Verified |
| PRV-07 | Done | ✅ Verified |
| PRV-08 | Done | ✅ Verified |
| PRV-09 | Done | ✅ Verified |
| PRV-10 | Pending (amend UAT) | ✅ Verified |
| PRV-11 | Done | ✅ Verified |
| PRV-12 | Done | ✅ Verified |
| PRV-13 | Done | ✅ Verified |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 13/13 ACs matched the spec outcome (0 gaps; all edge cases covered).

**Sensor**: 4/4 mutations killed — including the tight-width deletion that survived iteration 2 (now caught by `create_sheet_preview_test.dart:342-346`, card 331 ≠ 560).

**Gate**: 291 passed, 0 failed, 4 skipped; `flutter analyze` clean; no test deleted.

**What works**: the single-column 560px dialog (card below the name; `nome | id` with a hairline divider omitted for an empty name; "Read-only preview" header); the schema-driven read-only preview; the always-visible scrollbar with a working controller-driven scroll; the rendered 560px design width pinned on a wide window and the clamp to the content width below 640px; the loading/error/empty states; single-flight Retry; the removed Phase-2 summary; corrected doc comments; no new dependencies.

**Issues found**: none. Iteration-2 gap closed — the clamp assertion now compares the card to the clamped content width and the rendered width to the design token, and the previously-surviving mutation is killed.

**Next steps**: None. Feature verified; `validate_state.py` exits 0.
