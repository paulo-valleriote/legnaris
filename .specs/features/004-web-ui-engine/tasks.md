---
description: "Task list for the Web UI Engine feature"
---

# Tasks: Web UI Engine

**Input**: Design documents from `.specs/features/004-web-ui-engine/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED here. The constitution's "behavior ships with tests" principle, the feature's own
`ENG-01…ENG-29`, and the repo's edge-case-first rule all demand test tasks; they are written first and
must fail before the implementation they cover.

**Organization**: tasks are grouped by user story so each story is independently implementable,
testable, and demonstrable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks)
- **[Story]**: US1 … US3, per spec.md
- Every task names its exact file path

## Path Conventions

The engine lives under `web/src/engine/`, the copy catalog under `web/src/l10n/`, and every unit and
rendering test under `web/src/test/` with an `engine_*` prefix, matching the flat layout F1 established.
The engine mounts no surface: every case renders the engine's own components directly (Clarifications,
2026-10-16). No test-only route and no `web/e2e/` case are added.

The two P1 stories are ordered by the module graph the plan fixes (`field_grouping` → the two control
modules → `sheet_form`): the leaf controls (US2) land first so the composition (US1) can render data
mode against real controls, and US1's builder-mode cases remain independently testable on their own.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the stylesheet and copy keys every story consumes.

- [X] T001 [P] Create `web/src/engine/engine.css` with the token-based styles the engine renders —
      section header, row, row label, table header/row/cell, the control box, the numeric right
      alignment, and the marked-control tint class `lg-field--marked` expressed with
      `--lg-color-surface` — every visual value a `--lg-*` custom property, no literal hex or px
      (ENG-28, research R4)
- [X] T002 [P] Extend the translation catalog `web/src/l10n/strings.ts` with the four engine keys
      `engine.calculated`, `engine.none`, `engine.modified` and `engine.recalculated` (data-model
      `MessageKey additions`) and extend `web/src/test/l10n_strings.test.ts` so each new key is asserted
      to resolve to non-empty copy, distinct from its neighbours (ENG-27, SC-003, SC-004)

**Checkpoint**: the copy keys and the stylesheet exist; `cd web && npm test` still passes.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the mark contract every control and the composition consume.

**⚠️ CRITICAL**: US2's controls and US3's marks both import this module.

- [X] T003 [P] Implement `web/src/engine/sheet_mark.ts`: the `SheetMark` union
      (`'modified' | 'recalculated'`) and the marked-control class constant
      `FIELD_MARKED_CLASS = 'lg-field--marked'` (research R4). The module has no behavior of its own — it
      is a type and a class name — so it is asserted through the controls that consume it
      (`engine_marks.test.tsx`, T014)

**Checkpoint**: `sheet_mark.ts` compiles and the mark class is the single source the CSS uses.

---

## Phase 3: User Story 2 - Each field renders the control its schema dictates (Priority: P1)

**Goal**: one control per leaf chosen from its own schema (numeric/text input, `bool`/`config.*` toggle,
derived read-only "Calculated", unknown-type fallback), one single-choice control for an exclusive group,
and markup rendered as text.

**Independent Test**: render a synthetic template holding one leaf of each type and one exclusive group
and assert the control rendered per leaf, its interactivity, the "Calculated" indicator and formula
tooltip, the single control for the group with its "None" rules, and that `<script>` in a value appears
as literal text.

### Tests for User Story 2 ⚠️ (write first; they must fail before the implementation)

- [X] T004 [P] [US2] Field-control test `web/src/test/engine_field_control.test.tsx` (ENG-07…ENG-13),
      edge cases first: a numeric input holding a number dismisses `"abc"` / `"Infinity"` / `"1e999"`
      (the value does not change) and reports `null` on empty; with no change handler every control is
      inert; an unrecognized editable type renders no editor and does not throw; a `null`/absent stored
      value renders empty and a later edit reports a real value; a marked derived leaf drops the
      "Calculated" label and tints; then the happy paths — number, text, `bool` (`true`/`false`),
      `config.*` (`1`/`0`), derived with and without a formula (value read-only, "Calculated", formula
      on hover), each addressed by role and accessible name
- [X] T005 [P] [US2] Exclusive-control test `web/src/test/engine_exclusive_control.test.tsx`
      (ENG-14…ENG-18), edge cases first: a group whose members all default to `0` offers the "none"
      entry and one with a non-zero default does not; a default-active group with no active member shows
      the "none" hint, never a blank; a group with a single member is a stacked row, not this control;
      with no selection handler the control is disabled; `exclusiveValueIsOn` treats a non-zero number as
      on and `0`/`true`/`null`/`"1"` as off; then the happy paths — one `select` for three members (never
      three toggles), an option labelled by its `optionLabel` else its display name, selecting a member
      reports its dotted path and selecting "none" reports the clear-group signal
- [X] T006 [P] [US2] Markup-safety test `web/src/test/engine_xss.test.tsx` (ENG-29, SC-005): a field
      value and a label of `"<script>"` render as literal text and produce 0 `script` nodes; the engine
      sources contain no DOM-injection sink

### Implementation for User Story 2

- [X] T007 [US2] Implement `web/src/engine/field_control.tsx` per
      `contracts/field-control.contract.md`: `SheetFieldControl` choosing the control by schema
      (non-editable first, then `bool`/`config.*` toggle, then numeric text input with `inputMode`
      `decimal`, then text input, then read-only fallback), the finite-number guard on the numeric input,
      the "Calculated" indicator with the formula on hover (dropped while a mark is present), the inert
      mode with no change handler, an accessible role/name per control, and text-only rendering
      (ENG-07…ENG-13, ENG-29)
- [X] T008 [US2] Implement `web/src/engine/exclusive_control.tsx` per
      `contracts/exclusive-control.contract.md`: `SheetExclusiveControl` (one native `<select>` for the
      group's members), the `exclusiveValueIsOn` activity test, the `EXCLUSIVE_NONE_VALUE` sentinel, the
      "none" entry only when no member has a non-zero default, the "none" hint for a default-active group
      with no active member, option labels from `optionLabel` else the display name, the disabled
      (inert) mode with no selection handler, and an accessible role/name (ENG-14…ENG-18)

**Checkpoint**: every control branch and the exclusive single choice are proven by rendering tests, and
markup in data is proven inert.

---

## Phase 4: User Story 1 - Read a template's field tree as labeled sections (Priority: P1)

**Goal**: the pure derivation turns a decoded field tree into ordered, labeled sections and render rows,
and the composition renders them with exactly one control source (values or injected builders).

**Independent Test**: render a synthetic template whose groups are known (a stacked leaf, a
declared-column table, a non-adjacent exclusive group) and assert the section order, the row kinds, the
derived labels, the exclusive first-member position, that an injected per-field control builder is
consulted once per leaf and the data-driven control is not rendered, and that an invalid control-source
combination is a construction error.

### Tests for User Story 1 ⚠️ (write first; they must fail before the implementation)

- [X] T009 [P] [US1] Derivation test `web/src/test/engine_grouping.test.ts` (ENG-01…ENG-06), edge cases
      first: an empty tree yields `[]`; a group with no leaf (and one whose only leaves sit under an empty
      sub-group) yields no section; a declared-column group none of whose columns resolve falls back to
      stacked rows and an unknown column beside a known one is skipped; a single-member exclusive group
      is not collapsed; non-adjacent members of one group collapse at the first member's position; then
      the happy paths — one section per key in arrival order, a top-level leaf becomes a one-field
      section, a non-empty label wins and an empty one humanizes the key, a table row holds one field per
      declared column in declared order, `leaves` keeps the full flattened set, numeric right / other
      left alignment
- [X] T010 [P] [US1] Composition test `web/src/test/engine_form.test.tsx` (ENG-24, ENG-25, ENG-26,
      SC-002), edge cases first: passing both `data` and the builders, and passing neither, is a
      construction error rather than a half-rendered form; then the happy paths — a synthetic template
      exercising **every** field type (number, text, `bool`, `config.*`, derived, unknown) plus an
      exclusive group renders one complete form with 0 per-system literals (SC-002); data mode resolves
      each value by its dotted path and reports an edit as `(path, value)`; builder mode consults the
      injected `controlBuilder` once per leaf, renders no default control, and the composition reads no
      value itself

### Implementation for User Story 1

- [X] T011 [US1] Implement `web/src/engine/field_grouping.ts` per
      `contracts/section-derivation.contract.md`: `buildSections` (one section per top-level key, in
      arrival order; recursive flattening to dotted paths; empty group omitted; top-level leaf a
      one-field section; declared-column group a single table row with unknown columns skipped and a
      stacked-rows fallback; exclusive grouping by id at the first member's position, a single-member
      group not collapsed) plus the pure label helpers `fieldDisplayName`, `groupDisplayName`,
      `fieldColumnName`, `lastSegment` (ENG-01…ENG-06)
- [X] T012 [US1] Implement `web/src/engine/sheet_form.tsx` per
      `contracts/draft-and-marks.contract.md` and `contracts/section-derivation.contract.md`: render the
      derived sections and rows (section header, table header sharing, row label, cells) and take
      exactly one control source — data mode resolving each value with `valueAtPath` and reporting
      `(path, value)`, or builder mode handing each row to `controlBuilder`/`exclusiveControlBuilder` and
      reading no value — with any other combination a construction error (ENG-24, ENG-25, ENG-26). The
      `rowMarkBuilder` seam is added in T016 (US3). *Depends on T011, T007 and T008.*

**Checkpoint**: the field tree derives into ordered sections and rows and the composition is provably
fed by exactly one source; the marks do not exist yet.

---

## Phase 5: User Story 3 - Derive a draft from the template and mark previewed changes (Priority: P2)

**Goal**: the shared draft derivation (schema seeds, stored-sheet overlay, incremental cascade merge)
and the additive preview mark (Modified/Recalculated tinting a control and yielding the base
"Calculated"), exposed through a per-row mark seam.

**Independent Test**: seed a draft from a synthetic template with an empty stored object and assert one
value per leaf in section order; overlay a partial stored object and assert the per-leaf precedence; run
an empty cascade merge and assert the base is returned unchanged and that neither input was mutated;
render a row with each mark and assert the tint and label, including a marked row whose value is empty.

### Tests for User Story 3 ⚠️ (write first; they must fail before the implementation)

- [ ] T013 [P] [US3] Derivation test `web/src/test/engine_defaults.test.ts` (ENG-19…ENG-21), edge cases
      first: `mergeCascade` with an empty override returns a value equal to the base and mutates neither
      input; a nested override changes only its leaf and preserves its siblings; `setPathValue` creates
      missing intermediate objects and overwrites an existing leaf; then the happy paths — `seedDefaults`
      yields exactly one value per leaf in section order (empty text, `false` bool, numeric default) and
      `seedFromSheet` lets a stored value win, keeps an omitted schema leaf's default, and carries an
      unknown stored key over
- [ ] T014 [P] [US3] Mark test `web/src/test/engine_marks.test.tsx` (ENG-10, ENG-22, ENG-23), edge cases
      first: a row marked `modified` / `recalculated` whose value is empty still shows the tint and its
      label; a derived marked row drops the "Calculated" label; then the happy paths — a marked control
      carries `lg-field--marked`, the row shows `engine.modified` / `engine.recalculated`, and the
      `rowMarkBuilder` receives the single leaf path for a normal row and every member path for an
      exclusive row

### Implementation for User Story 3

- [ ] T015 [US3] Implement `web/src/engine/sheet_defaults.ts` per
      `contracts/draft-and-marks.contract.md`: `seedValue`, `seedDefaults` (walking `buildSections` and
      setting each leaf's seed under its dotted path in section order), `seedFromSheet`, the deep
      `mergeCascade` (recurse where both sides hold an object, replace otherwise; fresh result; no input
      mutated; empty cascade is the identity) and the internal `setPathValue` (ENG-19…ENG-21). *Depends
      on T011.*
- [ ] T016 [US3] Wire the mark and the row-mark seam into the engine per
      `contracts/draft-and-marks.contract.md`: pass the `mark` prop into the controls in
      `web/src/engine/field_control.tsx` and `web/src/engine/exclusive_control.tsx` (tint + "Calculated"
      yield), label a marked row with `message('engine.modified')` / `message('engine.recalculated')`,
      and add the optional `rowMarkBuilder(paths)` seam to `web/src/engine/sheet_form.tsx` in both modes
      (ENG-22, ENG-23). *Extends T012, T007 and T008.*

**Checkpoint**: the draft is derived, overlaid and merged without mutation, and a preview mark tints a
control and labels its row; all three stories are independently demonstrable.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T017 [P] Extend `web/src/test/tokens_used.test.tsx` to scan `web/src/engine/` for literal hex and
      px values (and that every `var(--lg-*)` it references is defined), so every visual value in the
      engine is proven to come from the shared tokens (ENG-28, SC-008)
- [ ] T018 [P] Extend `web/src/test/l10n_strings.test.ts` with an engine copy scan: no user-facing
      literal exists in `web/src/engine/`, every `message('…')` key referenced there resolves to a
      catalog entry (so removing a referenced key turns the suite red — SC-004), and the engine keys are
      asserted present (ENG-27, SC-003)
- [ ] T019 [P] Update `web/AGENTS.md` (the engine module under `web/src/engine/`, the two control-source
      modes and the two seams, the numeric finite-guard deviation from the desktop, the reused
      `--lg-color-surface` tint) and `web/README.md` (how to run the engine suites)
- [ ] T020 Run the full gate set from `web/` (`npm run format:check`, `npm run lint`, `npm run typecheck`,
      `npm test`, `npm run build`) and `make lint-specs` from the repo root, and confirm every `ENG-*`
      citation in the delivered sources resolves to `spec.md`
- [ ] T021 Run the nine `quickstart.md` scenarios and record the per-scenario outcome with the feature
- [ ] T022 Record any plan-versus-delivered deviation (path, naming or seam that changed) in
      `web/AGENTS.md` under the deviations section rather than weakening an assertion (constitution III)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — starts immediately (T001 and T002 are independent files).
- **Foundational (Phase 2)**: T003 has no dependency and blocks US2's and US3's mark work.
- **US2 (Phase 3)**: depends on T001 (css) and T003 (mark type); independent of the derivation.
- **US1 (Phase 4)**: `field_grouping.ts` (T011) starts after T009; `sheet_form.tsx` (T012) depends on
  T011 and on US2's control modules (T007, T008), which the phase order delivers first.
- **US3 (Phase 5)**: `sheet_defaults.ts` (T015) depends on `buildSections` (T011); the mark wiring (T016)
  extends US1's `sheet_form.tsx` and US2's controls.
- **Polish (Phase 6)**: depends on all desired stories.

### User Story Dependencies

- **US2 (P1)**: standalone leaf controls; testable once T001 and T003 land.
- **US1 (P1)**: the derivation (T011) is standalone; the composition (T012) renders data mode against
  US2's controls, and its builder-mode cases (ENG-25, ENG-26) are testable as soon as T011 and T012 land.
  The `SC-002` composite case (T010) needs both US2's controls and US1's composition.
- **US3 (P2)**: builds on US1's derivation and US2's controls.

### Within Each User Story

- Tests are written first and must fail before the implementation lands (constitution III).
- The control modules before the composition wires them; the pure derivation before the composition; the
  derivation before the draft seeds from it; the controls before the mark tints them.
- Story complete before moving to the next priority.

### Parallel Opportunities

- T001 and T002 (setup) — different files.
- T003 (mark type) is independent of both setup tasks.
- T004, T005 and T006 (US2 tests) — independent test files; T009 and T010 (US1 tests) likewise.
- T013 and T014 (US3 tests) — independent test files.
- T017, T018 and T019 (Polish) — independent files.

---

## Parallel Example: User Story 2

```bash
# Tests first (independent files, must fail before T007/T008):
Task: "Field-control test in web/src/test/engine_field_control.test.tsx"
Task: "Exclusive-control test in web/src/test/engine_exclusive_control.test.tsx"
Task: "Markup-safety test in web/src/test/engine_xss.test.tsx"

# Then the two control modules:
Task: "Implement web/src/engine/field_control.tsx"
Task: "Implement web/src/engine/exclusive_control.tsx"
```

---

## Implementation Strategy

### MVP First (US2 + US1, both P1)

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 US2: the per-type control and the exclusive single choice.
3. Phase 4 US1: the field tree derives into ordered, labeled sections and rows and the composition proves
   its single control source.
4. **STOP and VALIDATE** with `quickstart.md` scenarios 1, 2, 3, 4 and 6 (all controls, the exclusive
   group, the derivation, the composition and its seams; scenario 6's mark half is US3).
5. The engine still holds no draft and no mark; that is the next increment.

### Incremental Delivery

1. Setup + Foundational → the copy keys, stylesheet and mark contract exist.
2. US2 → the per-type control and the exclusive single choice.
3. US1 → the derivation and the composition (completes the MVP).
4. US3 → the shared draft derivation and the preview marks.
5. Polish → the two scans, documentation and the gate sweep.

### Parallel Team Strategy

With two contributors: one takes US2 (the control modules) while the other takes US1's derivation and
tests; US1's composition (T012) lands once US2's controls are in. US3 then extends both with the draft,
the marks and the row-mark seam. The two scans (T017, T018) share no file with the engine modules and can
be done at any point in Polish.

---

## Notes

- [P] tasks touch different files and have no dependency on incomplete work.
- Requirement IDs cited in task descriptions (`ENG-*`) are defined in `spec.md`; the delivered source
  must cite the same IDs so `make lint-specs` can trace them.
- Write tests first (T004, T005, T006, T009, T010, T013, T014) and confirm they fail before implementing.
- Commit per task or per logical group, with the tests in the same commit (constitution III).
- Do not weaken an assertion to make a gate pass; retarget it to the feature that owns the subject, or
  amend the spec and record the decision instead.
- The engine must issue no request, own no domain state, author no user-facing literal outside
  `web/src/l10n/strings.ts`, derive every visual value from the shared tokens, and render data as text —
  check those rules on every task that touches `web/src/engine/`.
