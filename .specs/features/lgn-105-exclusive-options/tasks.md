# LGN-105 Exclusive Options Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Spec**: `spec.md` · **Context**: `context.md` · **Design**: `design.md`
**Status**: Approved

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: root `AGENTS.md` (Testing section: edge cases first, `Happy path:`/`Edge:`/`Error:` case prefixes, never ship behavior without tests, `make fmt && make lint && make vet`, `go test -race`), `internal/sheet/template/AGENTS.md` (formulas and `DependsOn` must match; editability is decided only in `ValidateUpdateKeys`), `desktop/AGENTS.md` (widgets from tokens, Material primitives only, l10n strings), `.github/workflows/ci-flutter.yml` (`flutter analyze` + `flutter test`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Template contract (FieldSchema/DTO) | unit | Include when set, omit when empty; `toDTO` copy | `internal/sheet/template/*_test.go` | `go test ./internal/sheet/template/...` |
| Seeder contract | unit | ac pair tagged; every other config leaf untagged; formula/deps invariant unchanged | `internal/sheet/template/*_test.go` | `go test ./internal/sheet/template/...` |
| DAG normalization | unit | Every spec edge + conflict/off/single-member/derived-member/unrelated-group case | `internal/sheet/template/dag_test.go` | `go test ./internal/sheet/template/...` |
| Sheet service (PATCH) | unit + integration | Sibling zeroed, conflict 400, cascade recompute, no-op group untouched | `internal/sheet/*_test.go` | `go test ./internal/sheet/...` |
| Desktop model (FieldSchema) | unit | Decode `exclusive_group`, absent → null | `desktop/test/api/*_test.dart` | `cd desktop && flutter test test/api` |
| Desktop grouping | unit | Collapse runs, single-member degrade, order preserved | `desktop/test/engine/field_grouping_test.dart` | `cd desktop && flutter test test/engine` |
| Desktop exclusive control | widget | Segmented vs radio threshold, disabled read-only, keys | `desktop/test/engine/exclusive_control_test.dart` | `cd desktop && flutter test test/engine` |
| Desktop form | widget | Exclusive row rendering, builder injection invariant, data mode | `desktop/test/engine/sheet_form_test.dart` | `cd desktop && flutter test test/engine` |
| Desktop edit state | unit | Batch set, single emission, already-active no-op | `desktop/test/sheet/sheet_edit_provider_test.dart` | `cd desktop && flutter test test/sheet` |
| Desktop screen | widget | Group control wired, GET-only, default selection, preview read-only | `desktop/test/sheet/*_test.dart`, `desktop/test/catalog/create_sheet_preview_test.dart` | `cd desktop && flutter test` |
| Contract fixtures | unit | Fixtures carry `exclusive_group`; live contract stays green/skipped | `desktop/test/contract/*_test.dart` | `cd desktop && flutter test test/contract` |

## Gate Check Commands

> Generated from codebase - confirm before Execute.

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick (backend) | After backend tasks with unit tests only | `go test ./internal/sheet/...` |
| Build (backend) | Last task of each backend phase | `make fmt && make lint && make vet && go test ./...` |
| Quick (desktop) | After desktop tasks with unit/widget tests only | `cd desktop && flutter test` |
| Build (desktop) | Last task of each desktop phase | `cd desktop && flutter analyze && flutter test` |

---

## Execution Plan

Phases are ordered and run sequentially - each phase completes before the next begins, and tasks within a phase execute in order. Backend phases (1-3) and desktop phases (4-6) are two sequential task-budgeted batches.

### Phase 1: Backend contract

The metadata key and the seeded pair.

```
T1 → T2
```

### Phase 2: Backend DAG enforcement

Exclusive-group index and update normalization.

```
T3
```

### Phase 3: Backend service wiring

PATCH calls normalization and maps the conflict error.

```
T4
```

### Phase 4: Desktop model and grouping

Parse the key and collapse exclusive members into one row.

```
T5 → T6
```

### Phase 5: Desktop control and form

The segmented/radio control and the form row.

```
T7 → T8
```

### Phase 6: Desktop edit state and screen

Batch selection and screen wiring, with refreshed fixtures.

```
T9 → T10
```

### Phase 7: Verifier fix

Close the whole-tree contract-coverage gap the discrimination sensor found
(a mutant tagging `stats.*` survived).

```
T2 → T11
```

### Phase 8: Spellcasting group (backend)

Tag the six spellcasting flags exclusive and drop the redundant ability leaf.

```
T12 → T13
```

### Phase 9: Spellcasting group (desktop)

Mirror the contract in fixtures and cover the group on screen.

```
T14 → T15
```

### Phase 10: Option labels (backend)

Expose and seed the short option labels.

```
T16 → T17
```

### Phase 11: Standardized controls (desktop)

Unified dropdown for choices, checkbox for flags.

```
T18 → T19
T19 → T21
T20 → T21
```

---

## Task Breakdown

### T1: Add the exclusive_group metadata to the schema contract

**What**: add `ExclusiveGroup string` to `FieldSchema` (`bson:"exclusive_group,omitempty"`) and mirror it in `FieldSchemaDTO` (`json:"exclusive_group,omitempty"`); copy it in `FieldSchema.toDTO`.
**Where**: `internal/sheet/template/template.go`
**Depends on**: None
**Reuses**: existing `dto.go` clone pattern (`toDTO`), `dto_test.go` table harness
**Requirement**: EXC-01, EXC-02

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] A schema with a group serializes `"exclusive_group":"<id>"`; a schema without one omits the key
- [x] `ToDTO` copies the value and shares no memory with the source
- [x] Gate check passes: `go test ./internal/sheet/template/...`
- [x] Test count: new table cases in the DTO test (include/omit/copy), no silent deletions

**Tests**: unit
**Gate**: quick (backend)

**Commit**: `feat(template): add exclusive_group to the field schema contract`

---

### T2: Tag the armor-class proxy pair as one exclusive group

**What**: mark `config.acUsesDex` and `config.acUsesCon` with the shared group id `armorClassAbility` (a named constant) in `configGroup`; leave every other `config.*` flag untagged. Update the config pin test to assert the pair's group and the others' emptiness; the `armorClass` formula and its `DependsOn` stay unchanged.
**Where**: `internal/sheet/template/seeders.go`
**Depends on**: T1
**Reuses**: `numField`, the existing `seeders_test.go` config table test and `TestFantasyV1_FormulaReferencesMatchDependsOn`
**Requirement**: EXC-03

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `config.acUsesDex` and `config.acUsesCon` both carry `ExclusiveGroup == "armorClassAbility"`; every other config leaf carries `""`
- [x] The formula/depends-on invariant test is unchanged and green
- [x] Gate check passes: `make fmt && make lint && make vet && go test ./...`
- [x] Test count: config pin test extended (pair tagged, others untagged)

**Tests**: unit
**Gate**: build (backend)

**Commit**: `feat(template): tag the armor-class proxies as one exclusive group`

---

### T3: Normalize exclusive groups in the DAG

**What**: index exclusive groups at `BuildDAG` (editable leaves only, ≥2 members, sorted) and add `DAG.NormalizeExclusiveUpdates(updates)`: reject ≥2 non-zero members of one group with a new `ExclusiveGroupConflictError` (in `errors.go`), otherwise zero the siblings of the single active member; leave untouched groups unchanged. Non-numeric values count as off.
**Where**: `internal/sheet/template/dag.go`
**Depends on**: T1
**Reuses**: `collectLeaves`/`buildAdjacencyList` build pattern, single-`RLock` accessor discipline (`ValidateUpdateKeys`)
**Requirement**: EXC-04, EXC-05, EXC-06, EXC-08

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Turning one member on adds `0` for the absent siblings; a sibling already `0` is not duplicated
- [x] Two non-zero members of one group returns `*ExclusiveGroupConflictError` naming the group and keys
- [x] Setting a member to zero leaves siblings unchanged; a 1-member group is not enforced; a derived member is never grouped
- [x] An update touching no exclusive group returns the input unchanged and allocates no new map
- [x] Gate check passes: `make fmt && make lint && make vet && go test ./...`
- [x] Test count: new table-driven DAG tests for every listed edge plus the happy path

**Tests**: unit
**Gate**: build (backend)

**Commit**: `feat(template): normalize exclusive groups in the DAG`

---

### T4: Enforce exclusivity on PATCH

**What**: call `dag.NormalizeExclusiveUpdates` in `ServiceImpl.PartialUpdate` after `ValidateUpdateKeys` and use the normalized map for `Recalculate`/`ApplyUpdates`; add an `ExclusiveGroupConflictError` case to `translateDAGErr` mapping to `apperr.CodeInvalidRequest`.
**Where**: `internal/sheet/service.go`
**Depends on**: T3
**Reuses**: existing `translateDAGErr` table, `service_test.go` PATCH harness, `app/integration_test.go` PATCH flow
**Requirement**: EXC-04, EXC-05, EXC-07

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] PATCH turning `acUsesDex` on while `acUsesCon` is 1 stores `(1,0)` and the response cascades `derived.armorClass` from dexterity only
- [x] PATCH setting both to 1 returns `400 INVALID_REQUEST` and stores nothing
- [x] PATCH zeroing a member leaves its sibling unchanged
- [x] An integration test covers the sibling-zero + cascade path end to end
- [x] Gate check passes: `make fmt && make lint && make vet && go test ./...`
- [x] Test count: service table cases + one integration test, no silent deletions

**Tests**: unit + integration
**Gate**: build (backend)

**Commit**: `feat(sheet): enforce exclusive options on partial update`

---

### T5: Parse exclusive_group in the desktop schema model

**What**: add `String? exclusiveGroup` to `FieldSchema`, decoded from `exclusive_group`; absent or null → `null`.
**Where**: `desktop/lib/api/models/template.dart`
**Depends on**: None
**Reuses**: existing `FieldSchema.fromJson` optional-field pattern (`formula`, `depends_on`)
**Requirement**: EXC-17

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] A schema JSON with `exclusive_group` parses the string; without it parses `null`
- [x] Existing decode tests stay green; fixtures with no key still parse
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test test/api`
- [x] Test count: new decode cases in `template_model_test.dart`

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(engine): parse exclusive_group in the field schema model`

---

### T6: Collapse exclusive members into one grouped row

**What**: after flattening a section's leaves, collapse runs of leaves sharing a non-empty `exclusiveGroup` into one `SheetRow` carrying `SheetExclusiveGroup(id, options)` at the first member's position; a group with a single member is not collapsed.
**Where**: `desktop/lib/engine/field_grouping.dart`
**Depends on**: T5
**Reuses**: `_flatten`/`buildSections` tree walk, `SheetField`/`SheetSection` typedefs
**Requirement**: EXC-12

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] A two-member group becomes one exclusive row with both options in API order; the surrounding rows keep their order
- [x] A single-member group stays a normal row; untagged leaves are unaffected
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test test/engine`
- [x] Test count: grouping tests for collapse, degrade, and order

**Tests**: unit
**Gate**: build (desktop)

**Commit**: `feat(engine): group exclusive members into one row`

---

### T7: Add the exclusive segmented/radio control

**What**: new `SheetExclusiveControl(group, selectedPath, onSelect)` rendering a `SegmentedButton` when the group has ≤3 options and every label is ≤30 chars, otherwise a vertical `Radio` group; `onSelect == null` renders the same control disabled. Keys `sheet-exclusive-<id>` and `sheet-exclusive-option-<path>`; sizes/type from tokens.
**Where**: `desktop/lib/engine/exclusive_control.dart`
**Depends on**: None
**Reuses**: `lib/theme/tokens.dart`, Material `SegmentedButton`/`Radio` primitives, the disabled pattern from `SheetFieldControl`
**Requirement**: EXC-09, EXC-10, EXC-11

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] ≤3 short options → segmented; >3 options or a label longer than 30 → radio; exactly 30 chars stays segmented
- [x] Read-only (`onSelect == null`) renders the control disabled with the active option selected; no active option renders none selected
- [x] Selecting an option reports its path; keys are stable
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test test/engine`
- [x] Test count: widget tests for segmented/radio boundary, disabled, none-selected, callback

**Tests**: widget
**Gate**: quick (desktop)

**Commit**: `feat(engine): add the exclusive option control`

---

### T8: Render exclusive rows in SheetForm

**What**: `SheetForm` renders each exclusive `SheetRow` through an injected `exclusiveControlBuilder` in edit mode, or a disabled `SheetExclusiveControl` (active option resolved from `data`) in data mode; the constructor asserts both builders or neither, preserving the single control source invariant.
**Where**: `desktop/lib/engine/sheet_form.dart`
**Depends on**: T7, T6
**Reuses**: existing `controlBuilder` injection seam and row/section layout, `valueAtPath`
**Requirement**: EXC-09, EXC-11

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Edit mode renders the injected exclusive builder for the group and never two switches for its members
- [x] Data mode renders the group disabled with the active option; a group with no active member renders none selected
- [x] Providing one builder without the other asserts at construction
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test test/engine`
- [x] Test count: widget tests for injected/data/read-only and the assertion

**Tests**: widget
**Gate**: build (desktop)

**Commit**: `feat(engine): render exclusive groups in the sheet form`

---

### T9: Batch exclusive selection in the edit state

**What**: add `SheetEditController.setExclusive(selectedPath, siblings)` that sets the selected member to 1 and every sibling to 0 in one copy-on-write state emission; if no affected path changes (option already active) it returns without emitting, keeping the state instance identical.
**Where**: `desktop/lib/sheet/sheet_edit_provider.dart`
**Depends on**: None
**Reuses**: existing `setPath`/`_copyTree`/`setPathValue` helpers and `editedPaths`
**Requirement**: EXC-13, EXC-14

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] One call yields the selected member `1` and every sibling `0` in a single new state; both paths are recorded as edited
- [x] Selecting the already-active option returns the same state instance (no emission)
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test test/sheet`
- [x] Test count: unit tests for batch set, single emission, and the no-op

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(sheet): batch exclusive selection in the edit state`

---

### T10: Wire exclusive groups into the screen and refresh fixtures

**What**: `SheetScreen` passes an `exclusiveControlBuilder` that watches each option path via `provider.select` and calls `setExclusive(path, siblings)`; refresh `template_detail.json`/`templates.json` with `"exclusive_group":"armorClassAbility"` on the two ac schemas; confirm the preview renders the group disabled; assert the flow issues GET-only requests and reflects the schema default.
**Where**: `desktop/lib/sheet/sheet_screen.dart`
**Depends on**: T8, T9
**Reuses**: the screen's per-path `Consumer`/`select` pattern, `create_sheet_preview_test.dart` read-only loop, `contract_fixtures_test.dart`
**Requirement**: EXC-13, EXC-15, EXC-16

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] The edit screen renders one segmented control for the pair; selecting the constitution option flips the stored pair to `(0,1)` with a GET-only request log
- [x] The default option (`acUsesDex`) is selected on first render
- [x] The preview renders the group disabled with the default selected
- [x] Fixtures carry the key and the contract tests are green (live contract still skips without a server)
- [x] Gate check passes: `make fmt && make lint && make vet && go test ./...` and `cd desktop && flutter analyze && flutter test`
- [x] Test count: screen + preview + fixture tests, no silent deletions

**Tests**: widget
**Gate**: build (desktop)

**Commit**: `feat(sheet): wire exclusive groups into the screen`

---

### T11: Assert the exclusive group set covers exactly the armor-class pair

**What**: extend `TestFantasyV1_LeafContracts` to walk the whole leaf tree and assert the set of leaves with a non-empty `ExclusiveGroup` is exactly `config.acUsesDex` and `config.acUsesCon`, both carrying `armorClassAbility`. Closes the Verifier's surviving mutant (tagging `stats.*` was not caught).
**Where**: `internal/sheet/template/seeders_test.go`
**Depends on**: T2
**Reuses**: `collectLeaves` + the existing `TestFantasyV1_LeafContracts` walk
**Requirement**: EXC-01, EXC-02, EXC-03

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] The whole-tree assertion passes on `fantasy_v1` and fails if any other leaf is tagged (kills the P1 mutant)
- [x] Gate check passes: `make fmt && make lint && make vet && go test ./...`
- [x] Test count: 1 extended test, no silent deletions

**Tests**: unit
**Gate**: build (backend)

**Commit**: `test(template): assert exclusive groups cover exactly the armor-class pair`

---

### T12: Make the spellcasting ability flags exclusive

**What**: tag the six `config.spellUses*` leaves with the shared group id `spellcastingAbility` (named constant) in `configGroup`; extend the config pin test and the T11 whole-tree assertion to require exactly two groups (`armorClassAbility` with 2 members, `spellcastingAbility` with 6). The `spells.attackBonus` formula and its `DependsOn` stay unchanged; all six defaults stay `0`.
**Where**: `internal/sheet/template/seeders.go`
**Depends on**: T2, T11
**Reuses**: `spellUsesKey`, the T11 whole-tree assertion, the config pin test
**Requirement**: EXC-18

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] All six `config.spellUses*` carry `ExclusiveGroup == "spellcastingAbility"`; `save*Prof`/`skill*Prof` stay untagged
- [x] The whole-tree assertion names both groups with their exact member sets
- [x] Formula/depends-on invariant test unchanged and green
- [x] Gate check passes: `make fmt && make lint && make vet && go test ./...`
- [x] Test count: config pin + whole-tree tests extended, no silent deletions

**Tests**: unit
**Gate**: build (backend)

**Commit**: `feat(template): make the spellcasting ability flags exclusive`

---

### T13: Remove the redundant spells.ability leaf

**What**: remove the `spells.ability` text leaf from `spellsGroup`; update the spells structure test (no longer expects an `ability` text leaf) and the whole-tree leaf-count assertion (131 → 130); the formula/dep graph is unaffected.
**Where**: `internal/sheet/template/seeders.go`
**Depends on**: T12
**Reuses**: `TestFantasyV1_Structure` and `TestFantasyV1_LeafContracts`
**Requirement**: EXC-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `fantasy_v1` no longer exposes a `spells.ability` leaf; the remaining spells leaves render in order
- [x] Leaf count is 130 and the structure test passes without the removed leaf
- [x] Gate check passes: `make fmt && make lint && make vet && go test ./...`
- [x] Test count: structure + leaf-contract tests updated, no silent deletions

**Tests**: unit
**Gate**: build (backend)

**Commit**: `feat(template): remove the redundant spells.ability leaf`

---

### T14: Mirror the spellcasting group in the desktop fixtures

**What**: update `template_detail.json` and `templates.json` so the six `spellUses*` carry `"exclusive_group":"spellcastingAbility"` and the `spells.ability` schema is gone; drop `spells.ability` from `sheet_get.json`/`sheet_create.json`; update `contract_fixtures_test.dart` to require exactly the two groups (ac pair + six spellcasting) and the absence of `spells.ability`.
**Where**: `desktop/test/fixtures/contract_fixtures_test.dart`
**Depends on**: T12, T13
**Reuses**: existing contract-fixture group assertions
**Requirement**: EXC-18, EXC-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Fixtures tag exactly the ac pair and the six `spellUses*`; no other leaf is tagged
- [x] No fixture exposes `spells.ability`
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [x] Test count: contract-fixture test extended to both groups, no silent deletions

**Tests**: unit
**Gate**: build (desktop)

**Commit**: `test(desktop): mirror the spellcasting group contract fixtures`

---

### T15: Cover the exclusive spellcasting radio on screen

**What**: extend the sheet screen/preview tests: the six-option `spellcastingAbility` group renders a radio control (`sheet-exclusive-spellcastingAbility`), selecting one member sets it to 1 and the other five to 0 in a single emission with GET-only requests, none-selected renders with nothing checked, and the preview renders it disabled with the active option.
**Where**: `desktop/test/sheet/sheet_screen_test.dart`
**Depends on**: T14
**Reuses**: the existing exclusive-group screen test harness and `pumpScreen`
**Requirement**: EXC-20, EXC-21, EXC-22

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] The spellcasting group renders a radio (not segmented) and selecting one member zeroes the other five in one emission
- [x] The request log stays GET-only; the preview shows the group disabled with the active option
- [x] Gate check passes: `make fmt && make lint && make vet && go test ./...` and `cd desktop && flutter analyze && flutter test`
- [x] Test count: new screen/preview cases, no silent deletions

**Tests**: widget
**Gate**: build (desktop)

**Commit**: `test(sheet): cover the exclusive spellcasting radio group`

---

### T16: Add the option_label metadata to the schema contract

**What**: add `OptionLabel string` to `FieldSchema` (`bson:"option_label,omitempty"`) and mirror it in `FieldSchemaDTO` (`json:"option_label,omitempty"`); copy it in `FieldSchema.toDTO`.
**Where**: `internal/sheet/template/template.go`
**Depends on**: T1
**Reuses**: the `ExclusiveGroup` DTO pattern and `dto_test.go` harness
**Requirement**: EXC-23, EXC-24

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] A schema with an option label serializes `"option_label":"<label>"`; a schema without one omits the key
- [x] `ToDTO` copies the value and shares no memory with the source
- [x] Gate check passes: `go test ./internal/sheet/template/...`
- [x] Test count: DTO include/omit/copy cases, no silent deletions

**Tests**: unit
**Gate**: quick (backend)

**Commit**: `feat(template): add option_label to the field schema contract`

---

### T17: Seed short option labels for the exclusive groups

**What**: set `OptionLabel` on the exclusive members in `configGroup`: the ac pair (`Dexterity`, `Constitution`) and each `spellUses*` (`Strength`, `Dexterity`, `Constitution`, `Intelligence`, `Wisdom`, `Charisma`); leave every other leaf empty. Extend the config pin / whole-tree tests to assert the labels and their emptiness elsewhere.
**Where**: `internal/sheet/template/seeders.go`
**Depends on**: T16
**Reuses**: the whole-tree group assertion (T11/T12) and the ability catalog
**Requirement**: EXC-25

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Exactly the ac pair + six spellUses carry the expected `option_label`; every other leaf's is empty
- [x] Formula/depends-on invariant unchanged and green
- [x] Gate check passes: `make fmt && make lint && make vet && go test ./...`
- [x] Test count: seed label assertions added

**Tests**: unit
**Gate**: build (backend)

**Commit**: `feat(template): seed short option labels for the exclusive groups`

---

### T18: Parse option_label in the desktop schema model

**What**: add `String? optionLabel` to `FieldSchema`, decoded from `option_label`; absent/null → null.
**Where**: `desktop/lib/api/models/template.dart`
**Depends on**: T16
**Reuses**: the optional-field decode pattern (`exclusiveGroup`)
**Requirement**: EXC-26

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] A schema with `option_label` parses the string; without it parses null
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test test/api`
- [x] Test count: decode cases for present/absent/null

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(engine): parse option_label in the field schema model`

---

### T19: Render every exclusive group as one dropdown

**What**: replace the segmented/radio branches in `SheetExclusiveControl` with a single dense `DropdownButton<String>` (key `sheet-exclusive-<id>`, items keyed `sheet-exclusive-option-<path>`). Value = active path or `None`; include a `None` item iff the group has no active default (every member default `0`); option text = `optionLabel` when non-empty else the member label; `None` text via l10n; `onSelect == null` renders the dropdown disabled. `setExclusive` accepts null (None) and clears the whole group in one emission.
**Where**: `desktop/lib/engine/exclusive_control.dart`
**Depends on**: T18
**Reuses**: the existing group/model types, `lib/theme/tokens.dart`, the l10n pattern
**Requirement**: EXC-09, EXC-10, EXC-11, EXC-13, EXC-27

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] AC (default active) shows the default and no `None`; spellcasting shows `None` + six ability labels
- [ ] Selecting a member zeroes the siblings in one emission; selecting `None` zeroes all members in one emission
- [ ] Read-only renders the dropdown disabled with the current value (or `None`); a missing `option_label` falls back to the member label
- [ ] Gate check passes: `cd desktop && flutter analyze && flutter test test/engine`
- [ ] Test count: dropdown happy + edge (None/no-None/missing label/read-only/no-rebuild) cases

**Tests**: widget
**Gate**: quick (desktop)

**Commit**: `feat(engine): render exclusive groups as a unified dropdown`

---

### T20: Render 0/1 flags as checkboxes

**What**: in `SheetFieldControl`, replace the `Switch` in the flag branch with a compact `Checkbox` (key `sheet-checkbox-<path>`); `bool` emits `true`/`false`, a numeric `config.*` proxy emits `1`/`0`; a null/absent value renders unchecked; read-only renders the checkbox disabled. Keep numeric/text inputs unchanged.
**Where**: `desktop/lib/engine/field_control.dart`
**Depends on**: None
**Reuses**: `_isToggle` predicate, tokens, existing numeric/text branches
**Requirement**: EXC-28, EXC-29, EXC-30

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Editable flags render a checkbox; numeric proxy emits `1`/`0`, bool emits `true`/`false`
- [ ] Null value renders unchecked; read-only renders a disabled checkbox; numeric/text fields are unaffected
- [ ] Gate check passes: `cd desktop && flutter analyze && flutter test test/engine`
- [ ] Test count: checkbox happy + edge (numeric/bool/null/read-only) cases

**Tests**: widget
**Gate**: quick (desktop)

**Commit**: `feat(engine): render 0/1 flags as checkboxes`

---

### T21: Update fixtures and screen coverage for the standardized controls

**What**: add `option_label` to the ac pair and the six `spellUses*` in `template_detail.json`/`templates.json`; update `contract_fixtures_test.dart` to assert the labels present/absent; update the sheet screen and preview tests to the dropdown + checkbox selectors (member + `None` selection, single emission, GET-only, read-only disabled); add the `None` l10n string.
**Where**: `desktop/test/sheet/sheet_screen_test.dart`
**Depends on**: T17, T19, T20
**Reuses**: the existing exclusive-group screen harness, `contract_fixtures_test.dart`, `create_sheet_preview_test.dart`
**Requirement**: EXC-09, EXC-25, EXC-27, EXC-28

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Fixtures carry `option_label` on exactly the ac pair + six spellUses
- [ ] The edit screen renders the dropdown (default selected, `None` for spellcasting) and checkboxes for flags; selecting `None` clears the group in one emission with GET-only requests
- [ ] The preview renders the dropdown and checkboxes disabled
- [ ] Gate check passes: `make fmt && make lint && make vet && go test ./...` and `cd desktop && flutter analyze && flutter test`
- [ ] Test count: screen/preview/contract cases updated + new, no silent deletions

**Tests**: widget
**Gate**: build (desktop)

**Commit**: `test(sheet): cover the standardized dropdown and checkbox controls`

---

## Phase Execution Map

Phases run in sequence; tasks within a phase run in order:

```
Phase 1:  T1 → T2
Phase 2:  T1 → T3
Phase 3:  T3 → T4
Phase 4:  T5 → T6
Phase 5:  T7 → T8
          T6 → T8
Phase 6:  T9 → T10
          T8 → T10
Phase 7:  T2 → T11
Phase 8:  T2 → T12
          T11 → T12
          T12 → T13
Phase 9:  T12 → T14
          T13 → T14
          T14 → T15
Phase 10: T1 → T16
          T16 → T17
          T16 → T18
Phase 11: T18 → T19
          T19 → T21
          T20 → T21
          T17 → T21
```

Backend phases (T1-T4) and desktop phases (T5-T10) are each one task-budgeted batch; run backend first so the desktop fixtures mirror an already-shipped contract, then dispatch the Verifier after the final commit.

**Task Granularity Check**

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: contract field | 1 struct + DTO mirror | ✅ Granular |
| T2: seeder tag | 1 function | ✅ Granular |
| T3: DAG normalization | 1 file | ✅ Granular |
| T4: service wiring | 1 call site + error case | ✅ Granular |
| T5: model parse | 1 class | ✅ Granular |
| T6: grouping | 1 file | ✅ Granular |
| T7: exclusive control | 1 widget file | ✅ Granular |
| T8: form rendering | 1 file | ✅ Granular |
| T9: edit-state batch | 1 method | ✅ Granular |
| T10: screen + fixtures | 1 screen file (+ fixtures) | ✅ Granular |
| T11: whole-tree group assertion | 1 test file | ✅ Granular |
| T12: spellcasting tagging | 1 function | ✅ Granular |
| T13: remove spells.ability | 1 function | ✅ Granular |
| T14: contract fixtures | 1 test file (+ fixtures) | ✅ Granular |
| T15: spellcasting radio screen test | 1 test file | ✅ Granular |
| T16: option_label contract | 1 struct + DTO | ✅ Granular |
| T17: seed option labels | 1 function | ✅ Granular |
| T18: parse option_label | 1 class | ✅ Granular |
| T19: unified dropdown | 1 widget file | ✅ Granular |
| T20: checkbox flags | 1 file | ✅ Granular |
| T21: fixtures + screen coverage | 1 test file (+ fixtures) | ✅ Granular |

**Diagram-Definition Cross-Check**

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | — | ✅ Match |
| T2 | T1 | T1 → T2 | ✅ Match |
| T3 | T1 | T1 → T3 | ✅ Match |
| T4 | T3 | T3 → T4 | ✅ Match |
| T5 | None | — | ✅ Match |
| T6 | T5 | T5 → T6 | ✅ Match |
| T7 | None | — | ✅ Match |
| T8 | T7, T6 | T7 → T8, T6 → T8 | ✅ Match |
| T9 | None | — | ✅ Match |
| T10 | T8, T9 | T8 → T10, T9 → T10 | ✅ Match |
| T11 | T2 | T2 → T11 | ✅ Match |
| T12 | T2, T11 | T2 → T12, T11 → T12 | ✅ Match |
| T13 | T12 | T12 → T13 | ✅ Match |
| T14 | T12, T13 | T12 → T14, T13 → T14 | ✅ Match |
| T15 | T14 | T14 → T15 | ✅ Match |
| T16 | T1 | T1 → T16 | ✅ Match |
| T17 | T16 | T16 → T17 | ✅ Match |
| T18 | T16 | T16 → T18 | ✅ Match |
| T19 | T18 | T18 → T19 | ✅ Match |
| T20 | None | — | ✅ Match |
| T21 | T17, T19, T20 | T17 → T21, T19 → T21, T20 → T21 | ✅ Match |

**Test Co-location Validation**

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1 | Template contract | unit | unit | ✅ OK |
| T2 | Seeder contract | unit | unit | ✅ OK |
| T3 | DAG normalization | unit | unit | ✅ OK |
| T4 | Sheet service | unit + integration | unit + integration | ✅ OK |
| T5 | Desktop model | unit | unit | ✅ OK |
| T6 | Desktop grouping | unit | unit | ✅ OK |
| T7 | Desktop control | widget | widget | ✅ OK |
| T8 | Desktop form | widget | widget | ✅ OK |
| T9 | Desktop edit state | unit | unit | ✅ OK |
| T10 | Desktop screen | widget | widget | ✅ OK |
| T11 | Seeder contract | unit | unit | ✅ OK |
| T12 | Seeder contract | unit | unit | ✅ OK |
| T13 | Seeder contract | unit | unit | ✅ OK |
| T14 | Contract fixtures | unit | unit | ✅ OK |
| T15 | Desktop screen | widget | widget | ✅ OK |
| T16 | Template contract | unit | unit | ✅ OK |
| T17 | Seeder contract | unit | unit | ✅ OK |
| T18 | Desktop model | unit | unit | ✅ OK |
| T19 | Desktop control | widget | widget | ✅ OK |
| T20 | Desktop control | widget | widget | ✅ OK |
| T21 | Desktop screen | widget | widget | ✅ OK |
