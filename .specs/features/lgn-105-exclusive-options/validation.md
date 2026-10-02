# LGN-105 Exclusive Options Validation

**Date**: 2026-10-02
**Spec**: `.specs/features/lgn-105-exclusive-options/spec.md`
**Increment diff range**: `a5d35ec..HEAD` (T12–T15: `da42ee3`, `f3bafbe`, `eb8923c`, `a51fd29`, plus the review-hardening commits `52c0bff`…`1016d81`)
**Full feature range**: `6ab54f9..HEAD` (commits `f86d5bb` … `a51fd29`)
**Verifier**: independent sub-agent (author ≠ verifier)

## Validation

**Result**: PASS

The increment matches the spec-defined outcome for every acceptance criterion (EXC-18…EXC-22). All 22 ACs (EXC-01…EXC-22) are covered with `file:line` evidence, the six increment discrimination-sensor mutants are all killed, and the full Go + Flutter suites are green. No gaps remain.

---

## Task Completion

| Task | Status  | Notes |
| ---- | ------- | ----- |
| T1   | ✅ Done | contract field + DTO mirror |
| T2   | ✅ Done | ac pair tagged |
| T3   | ✅ Done | DAG index + normalization |
| T4   | ✅ Done | service wiring + 400 mapping |
| T5   | ✅ Done | Dart decode |
| T6   | ✅ Done | grouping `rows` |
| T7   | ✅ Done | segmented/radio control |
| T8   | ✅ Done | form rendering (provisional screen wiring) |
| T9   | ✅ Done | batch `setExclusive` |
| T10  | ✅ Done | screen wiring (replaced T8 provisional wiring), fixtures, layout fix |
| T11  | ✅ Done | whole-tree `exclusive_group` assertion |
| T12  | ✅ Done | six `config.spellUses*` tagged `spellcastingAbility` |
| T13  | ✅ Done | `spells.ability` leaf removed (leaf count 131 → 130) |
| T14  | ✅ Done | desktop fixtures mirror both groups, drop `spells.ability` |
| T15  | ✅ Done | screen + preview coverage for the 6-option radio; `Flexible` label wrap fix |

All task boxes in `tasks.md` are checked; commits are one-per-task and Conventional-Commits shaped.

---

## Spec-Anchored Acceptance Criteria

| Criterion | Spec-defined outcome | `file:line` + assertion | Result |
| --------- | -------------------- | ----------------------- | ------ |
| EXC-01 declared group is included | response carries `exclusive_group:"<id>"` verbatim | `internal/sheet/template/dto_test.go:130` - `assert.JSONEq(want, '…"exclusive_group":"armorClassAbility"')`; copy at `internal/sheet/template/dto_test.go:165` | ✅ |
| EXC-02 no group omits key | `exclusive_group` absent from payload | `internal/sheet/template/dto_test.go:135` - expected JSON has no `exclusive_group` key | ✅ |
| EXC-03 fantasy_v1 ac pair declares `armorClassAbility` | both ac leaves == `armorClassAbility`; other config leaves match their group (empty for save/skill) | `internal/sheet/template/seeders_test.go:221-231` - `wantGroups` map + `assert.Equal(…, config[name].Schema.ExclusiveGroup)`; whole-tree at `:358-366` | ✅ |
| EXC-04 one member on zeroes siblings | absent sibling written `0` | `internal/sheet/template/dag_test.go:1471-1475` (`{acUsesDex:1, acUsesCon:0}`); `internal/sheet/service_test.go:407-418` (stored pair `(1,0)`, cascade `12.0`); `internal/app/integration_test.go:376-393` (`acUsesDex==0`, stored `acUsesCon==1`) | ✅ |
| EXC-05 two members on → 400 INVALID_REQUEST | typed conflict, HTTP 400 | `internal/sheet/template/dag_test.go:1536-1544` (group + sorted keys); `internal/sheet/service_test.go:433-440,494-497` (`CodeInvalidRequest`, `HttpStatus()==400`); `internal/app/integration_test.go:396-401` (`StatusBadRequest`, `INVALID_REQUEST`) | ✅ |
| EXC-06 member off leaves siblings | input returned unchanged, no sibling added | `internal/sheet/template/dag_test.go:1487-1492` (`wantSameMap`); `internal/sheet/service_test.go:420-431` (stored `(0,0)`, cascade `armorClass 10.0`) | ✅ |
| EXC-07 recompute dependent derived | derived reflects only the active member | `internal/sheet/service_test.go:407-417` (cascade `derived.armorClass == 12.0` = 10 + 2·dex); `internal/app/integration_test.go:383` (`armorClass == 11.0` = 10 + 1·con) | ✅ |
| EXC-08 <2 editable members not exclusive | 1-member group dropped; derived never grouped | `internal/sheet/template/dag_test.go:1433-1439` (`NotContains(groups,"soloGroup")`, derived-only group absent); `:1524-1528` solo not enforced | ✅ |
| EXC-09 ≤3 options + labels ≤30 → one segmented | `SegmentedButton` keyed `sheet-exclusive-armorClassAbility` | `desktop/lib/engine/exclusive_control.dart:46-50,59-94`; `desktop/test/engine/exclusive_control_test.dart:66-67`; `:89-90` (30-char boundary stays segmented) | ✅ |
| EXC-10 >3 options or long label → radio | `Radio` group instead | `desktop/lib/engine/exclusive_control.dart:46-57,96-131`; `desktop/test/engine/exclusive_control_test.dart:107-108` (4 opts → 4 radios), `:131-132` (31 chars → radios) | ✅ |
| EXC-11 read-only disabled, active selected | `onSelectionChanged == null`, active selected | `desktop/test/engine/exclusive_control_test.dart:239` (segmented), `:258-259,282` (radio); `desktop/test/engine/sheet_form_test.dart:404-407` (data mode); `desktop/test/catalog/create_sheet_preview_test.dart:314-318` | ✅ |
| EXC-12 single-member group → normal control | no exclusive row; per-field control | `desktop/lib/engine/field_grouping.dart:83-86`; `desktop/test/engine/field_grouping_test.dart:152-166`; `desktop/test/engine/sheet_form_test.dart:439-455` | ✅ |
| EXC-13 select sets member 1 + siblings 0, one emission | one state emission, both paths edited | `desktop/lib/sheet/sheet_edit_provider.dart:83-99`; `desktop/test/sheet/sheet_edit_provider_test.dart:250-288` (values + `emissions hasLength(1)`); `desktop/test/sheet/sheet_screen_test.dart:1048-1092` (ac pair) and `:1229-1296` (spellcasting) | ✅ |
| EXC-14 re-select active → unchanged | same state instance, no emission | `desktop/lib/sheet/sheet_edit_provider.dart:85-89`; `desktop/test/sheet/sheet_edit_provider_test.dart:290-316` (`identical`, `emissions 0`); `desktop/test/sheet/sheet_screen_test.dart:1094-1120` | ✅ |
| EXC-15 default selection from schema | seeded default shown active | `desktop/test/sheet/sheet_screen_test.dart:1033` (`selected == {'config.acUsesDex'}`); `desktop/test/catalog/create_sheet_preview_test.dart:317`; `desktop/test/sheet/sheet_edit_provider_test.dart:80` | ✅ |
| EXC-16 editing issues no network request | request log GET-only | `desktop/test/sheet/sheet_edit_provider_test.dart:285-286`; `desktop/test/sheet/sheet_screen_test.dart:1043,1088-1089` (`hasLength(1)`, single `GET`) | ✅ |
| EXC-17 desktop parses `exclusive_group` | present → id; absent/null → null | `desktop/lib/api/models/template.dart:58,83`; `desktop/test/api/template_model_test.dart:158-168` (id), `:171-180,183-192` (absent/null), `:201` (untagged leaf null) | ✅ |
| EXC-18 six `spellUses*` tagged `spellcastingAbility`, nothing else | whole tree: exactly 2 groups (2 ac + 6 spell) and no other tagged leaf | `internal/sheet/template/seeders.go:45` (constant), `:361-364` (tag loop); `internal/sheet/template/seeders_test.go:221-231` (config pin) and `:347-366` (whole-tree `wantGrouped` equality); `desktop/test/fixtures/contract_fixtures_test.dart:104-123` (fixtures) | ✅ |
| EXC-19 no `spells.ability`, leaf count 130 | leaf absent; count decremented | `internal/sheet/template/seeders.go:319-343` (spellsGroup has no `ability`); `internal/sheet/template/seeders_test.go:280-281` (`NotContains`), `:320` (`require.Equal(130, len(schemas))`); `desktop/test/fixtures/contract_fixtures_test.dart:124-128` | ✅ |
| EXC-20 spellcasting renders a radio on screen | 6 radios, `sheet-exclusive-spellcastingAbility`, no member toggle, only ac pair segmented | `desktop/lib/engine/exclusive_control.dart:46-57` (`>3 options` → radio); `desktop/test/sheet/sheet_screen_test.dart:1173-1204` (`Radio<String> findsNWidgets(6)`, `SegmentedButton findsOneWidget`, per-ability keys) | ✅ |
| EXC-21 select sets member 1, other five 0, single emission, GET-only | one emission, siblings zeroed, no PATCH | `desktop/lib/sheet/sheet_edit_provider.dart:83-99`; `desktop/test/sheet/sheet_screen_test.dart:1229-1296` (`emissions==1`, all five siblings `0`, `editedPaths`, `recorded.single.method=='GET'`); `desktop/test/sheet/sheet_edit_provider_test.dart:250-288` | ✅ |
| EXC-22 preview renders radio disabled with active option | disabled radios, active option from seeded data | `desktop/lib/engine/exclusive_control.dart:96-131` (`enabled: onSelect != null`, `RadioGroup.groupValue`); `desktop/test/catalog/create_sheet_preview_test.dart:331-367` (6 radios `enabled==false`, `groupValue=='config.spellUsesIntelligence'`) | ✅ |

**Status**: ✅ All 22 ACs match the spec outcome.

**Regression spot-check (EXC-01…EXC-17)**: the increment is additive (one new group, one removed leaf). Re-verified:
- `spells.attackBonus` formula and `DependsOn` are byte-identical to the T11 state: `internal/sheet/template/seeders.go:309-323` builds the sum over the same six `spellUses*` leaves, and `TestFantasyV1_FormulaReferencesMatchDependsOn` (part of the green `go test ./internal/sheet/template/...`) still enforces both directions. No dep was added or removed by the `spells.ability` deletion; the `spells` section's other deps (`saveDC → spells.attackBonus`) are untouched.
- The armor-class pair behavior, DTO include/omit/copy, DAG index rules, service 400 mapping, segmented/radio threshold, read-only rendering, single-member degrade, batch `setExclusive`, already-active no-op, default selection, and GET-only edits all still pass unchanged (full suites below).
- The T11 whole-tree assertion was strengthened from "exactly the ac pair" to "exactly the ac pair + six spell flags" (`seeders_test.go:347-366`), so EXC-01/02/03 remain covered and are now stricter.

**Note on EXC-17 traceability**: the User Stories enumerate 16 EARS criteria (3+5+4+4); EXC-17 exists only in the traceability table and T5, and its outcome is defined by the edge case `spec.md:155`. Outcome verified; minor documentation inconsistency, not a precision gap.

---

## Discrimination Sensor

Isolation: `git worktree add --detach /tmp/opencode/lgn-105-verify-inc HEAD` (never `git stash`). Real-tree porcelain baseline captured before injecting: `?? .claude/skills/tlc-spec-driven/`, `?? .cursor/skills/tlc-spec-driven/`, `?? .windsurf/skills/tlc-spec-driven/`; re-checked after worktree removal and byte-identical.

Test commands used per mutant:
- Go: `go test -count=1 ./internal/sheet/template/...`
- Dart: `flutter test <file>` from `desktop/` (worktree had its own `flutter pub get`)

### Increment sensor (T12–T15)

| #  | File:line (base) | Injected change | Test run | Killed? |
| -- | ---------------- | --------------- | -------- | ------- |
| N1 | `internal/sheet/template/seeders.go:363` | Un-tag one flag (`spellUsesWisdom` only) | template pkg | ✅ Killed (`TestFantasyV1_Structure` config pin + `TestFantasyV1_LeafContracts` whole-tree) |
| N2 | `internal/sheet/template/seeders.go:319` | Re-introduce `spells.ability` leaf | template pkg | ✅ Killed (`TestFantasyV1_Structure` `NotContains`, `TestFantasyV1_LeafContracts` leaf count 130) |
| N3 | `desktop/lib/engine/exclusive_control.dart:46-50` | `_useSegmented => true` (always segmented) | `test/sheet/sheet_screen_test.dart` | ✅ Killed at `:1196` (`Radio<String> findsNWidgets(6)`) |
| N4 | `desktop/lib/sheet/sheet_edit_provider.dart:90-93` | `setExclusive` sets only the selected member (no sibling zeroing) | `test/sheet/sheet_edit_provider_test.dart` | ✅ Killed at `:273-274` and `:338` (sibling must become 0) |
| N5 | `desktop/lib/sheet/sheet_edit_provider.dart:84-98` | `setExclusive` emits per-path via `setPath` (6 emissions) | `test/sheet/sheet_screen_test.dart` | ✅ Killed at `:1269` (`emissions == 1`) |
| N6 | `desktop/lib/engine/exclusive_control.dart:120-125` | Revert the `Flexible` label wrap | `test/engine/exclusive_control_test.dart` | ✅ Killed at `:152` (`takeException isNull` → RenderFlex overflow) |

**Increment sensor depth**: 6 behavior-level mutations (2 Go, 4 Dart) spanning the tagging, leaf removal, radio/segmented choice, single-emission semantics, sibling zeroing, and the overflow fix.
**Increment result**: 6/6 killed. 0 survived.

**Coverage observation (non-blocking)**: N4 (no sibling zeroing) was *not* killed by the screen-level EXC-21 test, because that fixture seeds all six `spellUses*` to `0`, so the assertion "the other five are 0" is vacuously satisfied. The mutant is killed by the edit-state unit test, which starts from `acUsesDex:1` and asserts the sibling reaches `0` (`sheet_edit_provider_test.dart:273-274,338`; also `sheet_screen_test.dart:1048-1092` for the ac pair, whose default is `1`). No surviving mutant; the behavior is covered at the layer that can observe it.

### Original sensor (T1–T11, retained)

| #    | File:line | Injected change | Test run | Killed? |
| ---- | --------- | --------------- | -------- | ------- |
| M1 | `internal/sheet/template/dag.go:610-622` | Remove sibling-zeroing in `NormalizeExclusiveUpdates` | template pkg | ✅ Killed |
| M2 | `internal/sheet/template/dag.go:623-624` | Accept two non-zero members instead of conflict | template pkg | ✅ Killed |
| M3 | `internal/sheet/template/dag.go:244-249` | Remove the `<2` group drop | template pkg | ✅ Killed |
| M4 | `internal/sheet/service.go:171-174` | Skip normalization in `PartialUpdate` | sheet pkg | ✅ Killed |
| M5 | `internal/sheet/template/seeders.go:348-349` | Untag `config.acUsesCon` | template pkg | ✅ Killed |
| M6 | `internal/sheet/template/dto.go:38` | Drop `omitempty` from the JSON tag | template pkg | ✅ Killed |
| M7 | `desktop/lib/engine/exclusive_control.dart:41` | Segmented threshold `3 → 2` | `exclusive_control_test.dart` | ✅ Killed |
| M8 | `desktop/lib/engine/exclusive_control.dart:44` | Label limit `30 → 29` | `exclusive_control_test.dart` | ✅ Killed |
| M9 | `desktop/lib/engine/exclusive_control.dart:86-92` | Read-only `onSelectionChanged` made non-null | `exclusive_control_test.dart` | ✅ Killed |
| M10 | `desktop/lib/sheet/sheet_edit_provider.dart:90-93` | `setExclusive` sets only the selected member | `sheet_edit_provider_test.dart` | ✅ Killed |
| M11 | `desktop/lib/sheet/sheet_edit_provider.dart:89` | `setExclusive` always emits even when active | `sheet_edit_provider_test.dart` | ✅ Killed |
| M12 | `desktop/lib/engine/field_grouping.dart:85` | Collapse a single-member group | `field_grouping_test.dart` | ✅ Killed |
| P1 | `internal/sheet/template/seeders.go:190-196` | Tag `stats.strength` + `stats.dexterity` with a second group | template pkg (post-`010895a`) | ✅ Killed by T11 (was survived) |

**Cumulative mutant totals**: original 13/13 killed + increment 6/6 killed = **19/19 killed**. No surviving mutant.

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code / no scope creep | ✅ (T15 `Flexible` wrap is a 5-line, presentational overflow fix; review-hardening commits are spec-governed) |
| Surgical changes | ✅ |
| Matches existing patterns/style | ✅ |
| Single control source invariant preserved | ✅ `desktop/lib/engine/sheet_form.dart:48-52` assert; `desktop/test/engine/sheet_form_test.dart:301-345` |
| Spec-anchored outcome check (asserted values match spec) | ✅ all 22 ACs |
| Per-layer coverage met (domain 1:1 ACs) | ✅ EXC-04…08 have unit + service + HTTP e2e |
| Every test maps to a spec AC / edge / done-when | ✅ |
| `make fmt && make lint && make vet` | ✅ 0 issues; `go vet` clean; `flutter analyze` clean |

**T15 `Flexible` fix scrutiny**: the diff wraps the radio option `Text` in `Flexible` only (`desktop/lib/engine/exclusive_control.dart:120-125`); it changes layout/constraints, not values, keys, `enabled`, `groupValue`, or `onSelect`. Read-only/disabled behavior (EXC-22) and single-emission semantics (EXC-21) are unaffected and independently covered; N6 proves a test fails without the wrap.

---

## Edge Cases

- [x] Derived/non-editable leaf in a group is excluded — `dag_test.go:1433-1439`.
- [x] Single editable member not enforced — `dag_test.go:1524-1528`.
- [x] Sibling absent from payload written `0` only when a sibling is turned on — `dag_test.go:1470-1481`.
- [x] Member set to zero may leave the group with no active member — `dag_test.go:1487-1491`, `sheet_form_test.dart:419-436`.
- [x] Absent `exclusive_group` parses null and renders normal controls — `template_model_test.dart:171-180`, `field_grouping_test.dart:227-239`.
- [x] Read-only with no active member renders none selected + disabled — `exclusive_control_test.dart:267-282`.
- [x] Label exactly at the 30-char limit stays segmented — `exclusive_control_test.dart:82-90`.
- [x] Editing a group rebuilds only the group/consumers, not the whole form — `sheet_screen_test.dart:329-447` + `:1038-1082`.
- [x] Spellcasting group all six off renders none selected and stays selectable — `sheet_screen_test.dart:1206-1227`.
- [x] `spells.ability` removed leaves the remaining spells leaves in order — `seeders_test.go:278-303`.
- [x] Long radio label wraps in a narrow row without overflow — `exclusive_control_test.dart:136-153` (increment).
- [x] Preview spellcasting radio disabled with the seeded active option — `create_sheet_preview_test.dart:331-367`.

---

## Gate Check

- **Gate commands** (from `tasks.md`): `go test ./...` and `cd desktop && flutter analyze && flutter test`
- **Go**: `go test -count=1 ./...` green. 10 `TestIntegration_*` self-skip because no MongoDB is reachable at `localhost:27017` (the `legnaris-mongo` dev container is not running; the prod container publishes no host port). Non-integration packages all pass.
- **Flutter**: `flutter test` → 370 passed, 0 failed, 4 skipped (pre-existing `api_client_retry_test.dart` timing skips, unrelated to this feature).
- **Increment delta**: Go `seeders.go`/`seeders_test.go` (T12/T13); Dart `exclusive_control.dart` + 4 test files + 4 fixtures (T14/T15). No test deletions.

---

## Shortcuts and Deviations Found

1. **T8 provisional screen wiring corrected in T10** — T8 (`6c6650d`) wired `_exclusiveControl` with a `setPath` loop (two emissions); T10 (`fc775ea`) replaced it with the batch `setExclusive`. Final state satisfies EXC-13's single-emission rule. No violation.
2. **`SheetSection` gained `rows` while keeping `leaves`** — both live consumers (`sheet_defaults.dart:18` vs `sheet_form.dart:92`). Not a dead field; no violation.
3. **T10 layout fix (`Expanded`) in `sheet_form.dart`** — presentational only, tokens-based, preserves the single-control-source invariant and read-only behaviour. No spec violation.
4. **T15 `Flexible` wrap** — presentational overflow fix carried inside the test task (`a51fd29`); covered by `exclusive_control_test.dart:136-153` and killed as N6. Justified; no behavior change.
5. **T11/T12/T13 edited `spec.md` traceability** (`Pending → Implementing`). Acceptable; this Verifier now flips all rows to Verified.
6. **Review-hardening commits (`52c0bff`…`1016d81`) sit inside the increment range** — they were re-verified in the previous report's Follow-up section and remain covered by the current green suites.

No `// SPEC_DEVIATION` marker was found in the diff.

---

## Ranked Gaps

None. All 22 ACs are verified, and all 19 discrimination-sensor mutants (13 original + 6 increment) are killed. The one earlier gap (whole-tree `exclusive_group` assertion) was fixed by `010895a`; the increment's T11 extension keeps it exact for both groups.

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ---------- |
| EXC-01 … EXC-22 | Verified / Implementing | ✅ Verified |

All 22 traceability rows in `spec.md` are `Verified`.

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 22/22 ACs matched the spec outcome (EXC-18…EXC-22 new; EXC-01…EXC-17 regression spot-check clean; `spells.attackBonus` formula/`DependsOn` unchanged).
**Sensor**: 19/19 mutants killed (13 original + 6 increment).
**Gate**: Go `./...` green (integration self-skips without Mongo); Flutter 370 passed / 0 failed / 4 skipped; analyzer + lint + vet clean.
**`validate_state.py`**: exit code `0`.

**What works (increment)**: the six `config.spellUses*` are tagged `spellcastingAbility` and the whole tree has exactly two groups (`seeders_test.go:347-366`); `spells.ability` is gone and the leaf count is 130; the desktop decodes and groups the six options; the screen renders a radio (not segmented) with one key per ability; selecting one sets it to `1` and zeroes the other five in a single emission with GET-only traffic; the preview renders the same radio disabled with the seeded active option; the `Flexible` wrap stops the narrow-row overflow.
**Issues found**: none outstanding.
**Next steps**: none — feature is verified.
