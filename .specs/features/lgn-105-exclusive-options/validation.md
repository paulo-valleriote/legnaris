# LGN-105 Exclusive Options Validation

**Date**: 2026-10-02
**Spec**: `.specs/features/lgn-105-exclusive-options/spec.md`
**Increment 3 diff range**: `85e7f7b..HEAD` (commits `ba8e1f3`, `cb3ec3f`, `646aa34`, `0535d62`, `6fc989c`, `b53af81`, `819b451`)
**Full feature range**: `6ab54f9..HEAD`
**Verifier**: independent sub-agent (author ≠ verifier)

## Validation

**Result**: PASS

The increment standardizes the Config controls exactly as the revised spec defines: every exclusive group renders as one dropdown (no segmented/radio anywhere), every 0/1 flag as a checkbox (no switch), and the new `option_label` drives short dropdown option text with a schema-label fallback. All 30 ACs (EXC-01…EXC-30) are covered with `file:line` evidence, all eight increment-3 discrimination-sensor mutants are killed, and both full suites are green. No gaps remain.

---

## Task Completion

| Task | Status  | Notes |
| ---- | ------- | ----- |
| T1–T15 | ✅ Done | previously verified (contract, DAG, service, desktop grouping/control/state/screen, spellcasting group) |
| T16  | ✅ Done | `option_label` on `FieldSchema` + DTO, `toDTO` copy |
| T17  | ✅ Done | short option labels seeded on ac pair + six `spellUses*` |
| T18  | ✅ Done | Dart decode of `option_label` |
| T19  | ✅ Done | `SheetExclusiveControl` replaced by a single dropdown (+ `None`, fallback, read-only) |
| T20  | ✅ Done | `Switch` → `Checkbox`, type-preserving emission, disabled read-only |
| T21  | ✅ Done | fixtures + screen/preview coverage for dropdown + checkbox |

All task boxes in `tasks.md` are checked; commits are one-per-task and Conventional-Commits shaped.

---

## Spec-Anchored Acceptance Criteria

| Criterion | Spec-defined outcome | `file:line` + assertion | Result |
| --------- | -------------------- | ----------------------- | ------ |
| EXC-01 declared group is included | response carries `exclusive_group:"<id>"` verbatim | `internal/sheet/template/dto.go:38` (JSON tag) + `:152` (`toDTO` copy); `internal/sheet/template/dto_test.go:122-152` (declared group serialized), `:157-170` (copy shares no memory) | ✅ |
| EXC-02 no group omits key | `exclusive_group` absent from payload | `dto.go:38` (`omitempty`); `dto_test.go:133-137` (empty group → `want` has no key) | ✅ |
| EXC-03 fantasy_v1 ac pair declares `armorClassAbility` | both ac leaves == `armorClassAbility`; other config leaves empty | `internal/sheet/template/seeders.go:352,355` (pair tagged); `internal/sheet/template/seeders_test.go:217-231` (config pin: pair tagged, save/skill empty) | ✅ |
| EXC-04 one member on zeroes siblings | absent sibling written `0` | `internal/sheet/template/dag_test.go:1470-1481` (`{acUsesDex:1}` → `acUsesCon:0`); `internal/sheet/service_test.go:406-417` (stored `(1,0)`, cascade `12.0` = 10+2·dex); `internal/app/integration_test.go:376-393` (`acUsesDex==0`, stored `acUsesCon==1`) | ✅ |
| EXC-05 two members on → 400 INVALID_REQUEST | typed conflict, HTTP 400 | `dag_test.go:1536-1544` (group + sorted keys); `service_test.go:433-497` (`CodeInvalidRequest`, `HttpStatus()==400`); `integration_test.go:396-401` (`StatusBadRequest`, `INVALID_REQUEST`) | ✅ |
| EXC-06 member off leaves siblings | input returned unchanged, no sibling added | `dag_test.go:1487-1492` (`wantSameMap`); `service_test.go:420-431` (stored `(0,0)`, cascade `armorClass 10.0`) | ✅ |
| EXC-07 recompute dependent derived | derived reflects only the active member | `service_test.go:406-417` (cascade `derived.armorClass == 12.0`); `integration_test.go:376-393` (`armorClass == 11.0` = 10+1·con) | ✅ |
| EXC-08 <2 editable members not exclusive | 1-member group dropped; derived never grouped | `dag_test.go:1414-1439` (`NotContains(groups,"soloGroup")`, derived-only group absent); `:1524-1528` solo not enforced | ✅ |
| EXC-09 every exclusive group = one dropdown | one `DropdownButton` per group, no segmented/radio | `desktop/lib/engine/exclusive_control.dart:79-114` (single `DropdownButton`); grep of `desktop/lib` shows zero `SegmentedButton`/`Radio`/`Switch`; `desktop/test/engine/exclusive_control_test.dart:141` (`DropdownButton` findsOneWidget, no `Radio`); `desktop/test/sheet/sheet_screen_test.dart:1215-1224` (both groups are dropdowns, no radio), `:1055-1058` | ✅ |
| EXC-10 `None` iff no active default | ac pair (default 1) → no `None`; spellcasting (all 0) → `None` | predicate `exclusive_control.dart:51-52` (`group.options.every((o) => o.schema.defaultValue == 0)`); no-`None` `exclusive_control_test.dart:104-109`; `None` `:144-149`; preview `desktop/test/catalog/create_sheet_preview_test.dart:328-332` (no None) and `:377-397` (`None`) | ✅ |
| EXC-11 read-only dropdown disabled, current value or `None` | disabled with value/`None` | `exclusive_control.dart:87-92` (`onChanged == null` when `onSelect == null`), value at `:56-62`; `desktop/lib/engine/sheet_form.dart:123-127` (data mode passes `onSelect: null`); `exclusive_control_test.dart:242-279`; `desktop/test/engine/sheet_form_test.dart:392-436`; `create_sheet_preview_test.dart:313-343` | ✅ |
| EXC-12 single-member group → normal control | no exclusive row; per-field control | `desktop/lib/engine/field_grouping.dart:81-88` (`options.length < 2` not collapsed); `desktop/test/engine/field_grouping_test.dart:152-166`; `sheet_form_test.dart:438-460`; `sheet_screen_test.dart:1150-1185` (refresh to single member → two checkboxes) | ✅ |
| EXC-13 select member or `None` → one emission | member→1/siblings→0; `None`→all 0, single emission | `desktop/lib/sheet/sheet_edit_provider.dart:84-102` (`paths = null ? siblings : [selected, ...siblings]`, one state assignment); wiring `desktop/lib/sheet/sheet_screen.dart:350-356` (None maps all members); `desktop/test/sheet/sheet_edit_provider_test.dart:250-288` (member, `emissions hasLength(1)`), `:343-381` (`setExclusive(null)` clears all, one emission); `sheet_screen_test.dart:1072-1117` (ac pair), `:1338-1391` (`None` clears six) | ✅ |
| EXC-14 re-select active → unchanged | same state instance, no emission | `sheet_edit_provider.dart:88-92` (early return when no path changed); `sheet_edit_provider_test.dart:290-315` (`identical`, `emissions 0`), `:383-409` (already-empty group); `sheet_screen_test.dart:1119-1148` | ✅ |
| EXC-15 default selection from schema | seeded default shown active | `sheet_screen.dart:339-349` (active resolved from data); `sheet_screen_test.dart:1046-1069` (`dropdown.value == 'config.acUsesDex'`); `create_sheet_preview_test.dart:313-343` | ✅ |
| EXC-16 editing issues no network request | request log GET-only | `sheet_edit_provider_test.dart:284-286` (`recorded.hasLength(1)`, all GET); `sheet_screen_test.dart:1114-1115`, `:1310-1311`, `:1381` (`single.method == 'GET'`) | ✅ |
| EXC-17 desktop parses `exclusive_group` | present → id; absent/null → null | `desktop/lib/api/models/template.dart:59` (decode), `:82-85` (field); `desktop/test/api/template_model_test.dart:157-203` (id/absent/null/untagged) | ✅ |
| EXC-18 six `spellUses*` tagged `spellcastingAbility`, nothing else | whole tree: exactly 2 groups (2 ac + 6 spell) | `internal/sheet/template/seeders.go:365` (tag loop); `seeders_test.go:224-231` (config pin) + `:365-378` (whole-tree exact `wantGrouped`); `desktop/test/fixtures/contract_fixtures_test.dart:129-147` | ✅ |
| EXC-19 no `spells.ability`, leaf count 130 | leaf absent; count decremented | `seeders.go:308-345` (`spellsGroup` has no `ability`); `seeders_test.go:295-296` (`NotContains`), `:335` (`require.Equal(130, len(schemas))`); `contract_fixtures_test.dart:148-152` | ✅ |
| EXC-20 spellcasting = same single dropdown with `None` | one dropdown, `None` + six ability items | `exclusive_control.dart:79-114` (one widget for every group); `sheet_screen_test.dart:1200-1242` (`sheet-exclusive-spellcastingAbility`, `value == None`, no radio, no member checkboxes); `exclusive_control_test.dart:117-160` | ✅ |
| EXC-21 select ability → 1, other five 0, single emission, GET-only | one emission, siblings zeroed, no PATCH | `sheet_edit_provider.dart:84-102`; `sheet_screen_test.dart:1266-1336` (`emissions == 1`, all five siblings `0`, `single.method == 'GET'`, unaffected controls identical) | ✅ |
| EXC-22 preview dropdown disabled with active option or `None` | disabled, active value shown | `create_sheet_preview_test.dart:345-374` (active `config.spellUsesIntelligence`, `onChanged == null`, no radio), `:376-397` (no active → `None` disabled); `exclusive_control_test.dart:260-278` | ✅ |
| EXC-23 `option_label` included when set | response carries `option_label` verbatim | `dto.go:39` (JSON tag) + `:153` (copy); `internal/sheet/template/dto_test.go:177-206` (declared label serialized), `:212-226` (copy) | ✅ |
| EXC-24 `option_label` omitted when empty | key absent | `dto.go:39` (`omitempty`); `dto_test.go:190-205` (empty label → `want` without key) | ✅ |
| EXC-25 seeded on exactly ac pair + six `spellUses` | labels on those 8 leaves, empty elsewhere | `seeders.go:353,356,366` (`Dexterity`, `Constitution`, `a.label`); `seeders_test.go:234-245` (config pin) + `:379-401` (whole-tree exact `wantLabeled`); `contract_fixtures_test.dart:157-182` | ✅ |
| EXC-26 Dart parses `option_label` | present → string; absent/null → null | `template.dart:60` (decode), `:87-90` (field); `template_model_test.dart:205-252` (present/absent/null/untagged) | ✅ |
| EXC-27 dropdown uses `option_label`, fallback to label | option text = option_label when non-empty else schema label | `exclusive_control.dart:64-70` (`_optionLabel`); `exclusive_control_test.dart:110-113` (`Dexterity`/`Constitution`), `:150-158` (six abilities), `:162-180` (missing/empty → schema label) | ✅ |
| EXC-28 editable flag renders a checkbox | `Checkbox`, never a switch | `field_control.dart:91` (`_isToggle`), `:102` (branch), `:146-169` (`Checkbox`); `field_control_test.dart:108-130` (config → checkbox, no `TextField`), `:155-174` (bool), `:176-195`; `sheet_screen_test.dart:1177-1184` (flags are checkboxes after a single-member refresh) | ✅ |
| EXC-29 checkbox emits value in field type | bool → true/false; numeric `config.*` → 1/0 | `field_control.dart:147` (`emitsBool = type == 'bool'`), `:162` (`emitsBool ? on : (on ? 1 : 0)`); `field_control_test.dart:108-130` (`[0]`), `:132-153` (`[1]`), `:155-174` (`[false]`), `:176-195` (numeric never bool) | ✅ |
| EXC-30 read-only checkbox disabled | `onChanged == null`, value shown | `field_control.dart:157-159` (`widget.onChanged == null ? null : ...`); `field_control_test.dart:284-298` (`value isTrue`, `onChanged isNull`); `create_sheet_preview_test.dart:301-305` (every preview checkbox disabled) | ✅ |

**Status**: ✅ All 30 ACs match the spec outcome.

**Regression spot-check (EXC-01…EXC-22)**: the increment is additive (two `OptionLabel` assignments + one `omitempty` DTO field + desktop control swaps). Re-verified:
- `spells.attackBonus` formula and `DependsOn` are byte-identical to the `85e7f7b` state: `git diff 85e7f7b..HEAD -- internal/sheet/template/seeders.go` touches only the three `OptionLabel` lines (`:353,356,366`); the `attackBonus` sum loop (`seeders.go:308-321`) and its six `config.spellUses*` deps are untouched, and `TestFantasyV1_FormulaReferencesMatchDependsOn` (green) enforces both directions.
- Leaf count is still 130 (`seeders_test.go:335`), and `spells.ability` is still absent (`:295-296`).
- Backend DTO include/omit/copy, DAG index rules, normalization/conflict/type errors, service 400 mapping and cascade recompute all pass unchanged; the increment does not touch `dag.go`/`service.go`.
- The desktop now substitutes the single dropdown for the previous segmented/radio control and the checkbox for the switch; grouping, single-member degrade, batch `setExclusive`, already-active no-op, GET-only edits and the single-control-source invariant still pass.

**Note on EXC-17 traceability**: the User Stories enumerate 29 EARS criteria; EXC-17 exists only in the traceability table (Story 1) and T5, and its outcome is defined by the edge case `spec.md:176`. Outcome verified; minor documentation inconsistency, not a precision gap.

---

## Discrimination Sensor

Isolation: `git worktree add --detach /tmp/opencode/lgn-105-v3 HEAD` (never `git stash`). Real-tree porcelain baseline captured before injecting: ` M .agents/.skill-lock.json`, ` M .agents/.skill-lock.json.backup`, `?? .claude/skills/{the-judge,tlc-spec-driven}/`, `?? .cursor/…`, `?? .windsurf/…`; re-checked after worktree removal and byte-identical (`diff` empty).

Test commands used per mutant (worktree had its own `flutter pub get`):
- Go: `go test -count=1 ./internal/sheet/template/...`
- Dart: `flutter test test/engine/exclusive_control_test.dart`, `test/engine/field_control_test.dart`, `test/sheet/sheet_edit_provider_test.dart`

### Increment 3 sensor (T16–T21, unified dropdown + checkbox + `option_label`)

| #  | File mutate | Injected change | Test run | Killed? |
| -- | ----------- | --------------- | -------- | ------- |
| V1 | `internal/sheet/template/dto.go:39` | Drop `omitempty` from `json:"option_label,omitempty"` | template pkg | ✅ Killed (`TestFieldSchemaDTO_MarshalJSON_OptionLabel/Edge:_an_empty_label_omits_the_key` — key leaks) |
| V2 | `desktop/lib/engine/exclusive_control.dart:51-52` | `_allowsNone => true` (always offer `None`) | `exclusive_control_test.dart` | ✅ Killed (`:104-109` pair must not contain `None`; `:194-199` active-default unselected must not) |
| V3 | `desktop/lib/engine/exclusive_control.dart:51-52` | `_allowsNone => false` (never offer `None`) | `exclusive_control_test.dart` | ✅ Killed (`:144-149` six-option group must be `None`; read-only no-active) |
| V4 | `desktop/lib/engine/exclusive_control.dart:66-70` | Ignore `option_label` (always fall back to schema label) | `exclusive_control_test.dart` | ✅ Killed (`:110-113`/`:150-158` expected short labels; `:175-178` fallback case) |
| V5 | `desktop/lib/sheet/sheet_edit_provider.dart:84` | `setExclusive(null)` early-returns (does not clear members) | `sheet_edit_provider_test.dart` | ✅ Killed (`:343-381` `None` must clear every member to 0) |
| V6 | `desktop/lib/engine/field_control.dart:162` | Numeric proxy emits a bool (`emitsBool ? on : …` → `on`) | `field_control_test.dart` | ✅ Killed (`:128`,`:151`,`:192` expect `[0]`/`[1]`, got `[false]`/`[true]`) |
| V7 | `desktop/lib/engine/field_control.dart:157-159` | Read-only checkbox left enabled (`onChanged` made non-null) | `field_control_test.dart` | ✅ Killed (`:284-298` read-only flag must be `onChanged == null`) |
| V8 | `desktop/lib/engine/exclusive_control.dart:88` | Read-only dropdown left enabled (`onChanged` made non-null) | `exclusive_control_test.dart` | ✅ Killed (`:242-258`,`:260-278` read-only `onChanged` must be null) |

**Increment 3 sensor depth**: 8 behavior-level mutations (1 Go, 7 Dart) spanning DTO omission, the `None` predicate (both ways), the `option_label` fallback, `None` clearing, numeric-vs-bool checkbox emission, and read-only checkbox/dropdown disabling.
**Increment 3 result**: 8/8 killed. 0 survived.

### Previous sensors (retained)

**Increment 2 (T12–T15)** — 6/6 killed: N1 un-tag one flag, N2 re-introduce `spells.ability`, N3 always segmented, N4 no sibling zeroing, N5 per-path emission, N6 revert `Flexible` label wrap. (No surviving mutant; see the increment-2 report — the earlier coverage observation about the all-zero spellcasting fixture not killing N4 was covered by the `acUsesDex:1` edit-state unit test.)

**Original (T1–T11)** — 13/13 killed: M1 remove sibling-zeroing, M2 accept two members, M3 remove `<2` drop, M4 skip service normalization, M5 un-tag `acUsesCon`, M6 drop `omitempty`, M7 segmented ≤2, M8 label limit 29, M9 read-only radio enabled, M10 set only selected member, M11 always emit, M12 collapse single-member, P1 (was survived, killed by T11).

**Cumulative mutant totals**: 13 original + 6 increment-2 + 8 increment-3 = **27/27 killed**. No surviving mutant.

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code / no scope creep | ✅ (increment is two Go lines + one model field + control swaps; `None` l10n string is spec-required) |
| Surgical changes | ✅ |
| Matches existing patterns/style | ✅ |
| Single control source invariant preserved | ✅ `desktop/lib/engine/sheet_form.dart:48-52` assert; `desktop/test/engine/sheet_form_test.dart:301-348` |
| No legacy widgets left | ✅ `grep -rn "SegmentedButton\|Radio\|Switch" desktop/lib` returns nothing |
| Spec-anchored outcome check (asserted values match spec) | ✅ all 30 ACs |
| Per-layer coverage met | ✅ DTO (Go), seeder exact-set (Go), dropdown/checkbox widget, edit state unit, screen + preview widget |
| Every test maps to a spec AC / edge / done-when | ✅ |
| `make fmt && make lint && make vet` | ✅ clean; `flutter analyze` clean |

---

## Edge Cases

- [x] Derived/non-editable leaf in a group is excluded — `dag_test.go:1414-1439`.
- [x] Single editable member not enforced — `dag_test.go:1524-1528`.
- [x] Sibling absent from payload written `0` only when a member is turned on — `dag_test.go:1470-1481`.
- [x] Member set to zero may leave the group with no active member — `dag_test.go:1487-1491`; `sheet_form_test.dart:419-435`.
- [x] Absent `exclusive_group`/`option_label` parses null and renders normal controls — `template_model_test.dart:171-203`, `:220-251`; `field_grouping_test.dart:238-246`.
- [x] Read-only with no active member renders `None` disabled — `exclusive_control_test.dart:260-278`; `create_sheet_preview_test.dart:376-397`.
- [x] Group with a default active member offers no `None` — `exclusive_control_test.dart:104-109`; `create_sheet_preview_test.dart:328-332`.
- [x] Missing/empty `option_label` falls back to the member schema label — `exclusive_control_test.dart:162-180`.
- [x] Selecting a value rebuilds only the group/consumers, not the whole form — `sheet_screen_test.dart:1322-1334` + `:1018-1043`.
- [x] Spellcasting all six off shows `None` and stays selectable — `sheet_screen_test.dart:1245-1264`.
- [x] Numeric `config.*` checkbox emits `1`/`0`; bool flag emits `true`/`false` — `field_control_test.dart:108-195`.
- [x] Null/absent flag renders unchecked; read-only renders disabled — `field_control_test.dart:255-298`.
- [x] Selecting `None` sets every member to 0 in one emission — `sheet_edit_provider_test.dart:343-381`; `sheet_screen_test.dart:1338-1391`.
- [x] `spells.ability` removed; remaining spells leaves render in order — `seeders_test.go:290-303`.
- [x] Preview renders dropdown + checkboxes disabled — `create_sheet_preview_test.dart:288-310`, `:313-397`.

---

## Gate Check

- **Gate commands** (from `tasks.md`): `go test ./...` and `cd desktop && flutter analyze && flutter test`
- **Go**: `go test -count=1 ./...` green. `internal/app` integration tests self-skip without MongoDB (`localhost:27017` unreachable); every non-integration package passes.
- **Flutter**: `flutter analyze` → "No issues found"; `flutter test` → 378 passed, 0 failed, 4 skipped (pre-existing `api_client_retry_test.dart` timing skips, unrelated to this feature).
- **Increment delta**: Go `template.go`/`dto.go` + `dto_test.go`/`seeders_test.go`/`seeders.go`; Dart `template.dart`, `exclusive_control.dart`, `field_control.dart`, `sheet_edit_provider.dart`, `sheet_screen.dart`, l10n, 4 fixtures + 6 test files. No test deletions.

---

## Shortcuts and Deviations Found

1. **`field_grouping.dart:14` comment still cites EXC-09** for grouping — accurate (the row feeds the dropdown). No violation.
2. **`_value` returns null when a default-active group has nothing selected** (`exclusive_control.dart:56-62`) — the dropdown shows no value until the data carries the member; the seeded screen/preview always does (`sheet_screen_test.dart:1055-1058`, `create_sheet_preview_test.dart:321-326`). Consistent with the spec's "no active default" predicate. No violation.
3. **EXC-17 has no EARS bullet** (traceability-only, as in the previous report) — verified via the edge case; documentation inconsistency, not a behavior gap.

No `// SPEC_DEVIATION` marker was found in the increment diff.

---

## Ranked Gaps

None. All 30 ACs are verified with file:line evidence, and all 27 cumulative discrimination-sensor mutants (13 original + 6 increment-2 + 8 increment-3) are killed.

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ---------- |
| EXC-01 … EXC-08 | Verified | ✅ Verified |
| EXC-09, EXC-10, EXC-11, EXC-13 | Implementing | ✅ Verified |
| EXC-12, EXC-14 … EXC-22 | Verified | ✅ Verified |
| EXC-23 … EXC-30 | Implementing | ✅ Verified |

All 30 traceability rows in `spec.md` are `Verified`.

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 30/30 ACs matched the spec outcome (EXC-09/10/11/13 revised; EXC-23…30 new; EXC-01…22 regression spot-check clean; `spells.attackBonus` formula/`DependsOn` unchanged and leaf count still 130).
**Sensor**: 27/27 cumulative mutants killed (13 original + 6 increment-2 + 8 increment-3); 0 survived.
**Gate**: Go `./...` green (integration self-skips without Mongo); Flutter 378 passed / 0 failed / 4 skipped; analyzer + lint + vet clean.
**`validate_state.py`**: exit code `0`.

**What works (increment 3)**: every exclusive group (2-option ac pair and 6-option spellcasting) renders as one `DropdownButton` (`exclusive_control.dart:79-114`); no `SegmentedButton`/`Radio`/`Switch` remains in `desktop/lib`; `None` is offered exactly when no member carries a non-zero default; read-only surfaces render the dropdown and flags disabled; selecting a member or `None` writes 1/0 (or all 0) in a single edit-state emission with GET-only traffic; `option_label` is emitted/omitted by the DTO, seeded on exactly the ac pair + six `spellUses*`, parsed in Dart and used with a schema-label fallback; 0/1 flags render as checkboxes that emit `1`/`0` (numeric proxy) or `true`/`false` (bool).
**Issues found**: none outstanding.
**Next steps**: none — feature is verified.
