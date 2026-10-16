# Quickstart: validate the web UI engine

**Feature**: `.specs/features/004-web-ui-engine/` | **Date**: 2026-10-16

Runnable scenarios that prove the feature works end to end. Each lists what to run and the outcome that
means it passed. Prerequisites: the F1 client toolchain (`web/` with `npm install` already done). No
scenario needs a backend and none needs MongoDB: the engine performs no I/O and issues no request — it
derives and renders purely from data already in memory.

Paths referenced here are defined in [plan.md](./plan.md) (source layout),
[data-model.md](./data-model.md) (shapes) and the four documents under [contracts/](./contracts/).

---

## 1. The suite runs with no backend at all

```bash
cd web && npm test
```

**Outcome**: the whole suite passes with the API and MongoDB stopped. The engine suites never touch the
network — the engine renders from its inputs and holds no credential — so this is the state continuous
integration runs in.

---

## 2. Every control branch is proven by a rendering test

```bash
cd web && npx vitest run src/test/engine_field_control.test.tsx
```

**Outcome**: one case per type proves the control the schema dictates (SC-001): an editable number leaf
renders a numeric input whose edit reports the parsed number (ENG-07, ENG-12); an editable text leaf a
text input (ENG-07); a `bool` leaf and a `config.*` proxy leaf each render an on/off toggle reporting
`true`/`false` and `1`/`0` respectively (ENG-08); a derived leaf renders its value read-only, marked
"Calculated", with its formula on hover and no formula when none is declared (ENG-09, ENG-10); an
unrecognized type renders the read-only value with no editor and no throw (ENG-11); with no change
handler the control is inert (ENG-13); a marked derived leaf drops the "Calculated" label and tints the
control. The numeric guard is proven at the edge: `"abc"`, `"Infinity"` and `"1e999"` leave the value
unchanged, an empty field reports a cleared value (ENG-12).

---

## 3. One mutually exclusive group renders as one control

```bash
cd web && npx vitest run src/test/engine_exclusive_control.test.tsx
```

**Outcome**: a three-member group renders **one** single-choice control (by role and accessible name),
never three toggles (ENG-14); each option is labeled by its non-empty `optionLabel`, else its display
name (ENG-15); the "none" entry exists only when no member has a non-zero default, and a default-active
group with no active member shows the "none" hint instead of a blank (ENG-16); selecting a member
reports that member's dotted path and selecting "none" reports the clear-group signal (ENG-18); with no
selection handler the control is disabled and nothing is selectable (ENG-17).

---

## 4. The field tree derives into labeled sections with the right row kinds

```bash
cd web && npx vitest run src/test/engine_grouping.test.ts
```

**Outcome**: a synthetic tree derives one labeled section per top-level key in arrival order (ENG-01);
each leaf becomes a dotted path in tree order, an empty group is omitted and a top-level leaf becomes a
one-field section (ENG-02); a label is the non-empty schema/group label else a humanization of the key
(ENG-03); a declared-column group draws one table row — the group label plus one cell per declared
column in declared order — skipping an unknown column and falling back to stacked rows when none
resolves (ENG-04, ENG-06); a non-adjacent exclusive group collapses into one row at the first member's
position and a single-member group is not collapsed (ENG-05).

---

## 5. The draft is derived, overlaid and merged without mutation

```bash
cd web && npx vitest run src/test/engine_defaults.test.ts
```

**Outcome**: the empty draft holds exactly one seed per leaf under its dotted path in section order —
empty text, `false` bool, else the numeric default (ENG-19, SC-006); overlaying a stored sheet lets a
stored value win at its path, keeps a schema leaf the sheet omits and carries over a stored key absent
from the schema (ENG-20); merging an empty cascade returns the base unchanged and merging a nested
cascade changes only its leaf — neither input mutated in either case (ENG-21, SC-006).

---

## 6. The composition proves its seams and the marks

```bash
cd web && npx vitest run src/test/engine_form.test.tsx src/test/engine_marks.test.tsx
```

**Outcome**: in data mode the composition renders the derived sections and rows, resolves each value by
its dotted path and reports an edit as `(path, value)` (ENG-24); passing both control sources, or
neither, is a construction error rather than a half-rendered form (ENG-25); in builder mode the injected
`controlBuilder` is consulted once per leaf, the default control is not rendered and the composition
subscribes to no value itself (ENG-26); `rowMarkBuilder` receives the single leaf path for a normal row
and every member path for an exclusive row (ENG-23); a row marked `modified` or `recalculated` is tinted
and labeled even when its value is empty, and the base "Calculated" label yields (ENG-22).

---

## 7. Markup in data is rendered as text

```bash
cd web && npx vitest run src/test/engine_xss.test.tsx
```

**Outcome**: a value or label of `<script>` appears as literal text and produces 0 `script` nodes
(ENG-29, SC-005); the engine contains no DOM-injection sink.

---

## 8. No copy and no visual value is authored in the engine

```bash
cd web && npx vitest run src/test/l10n_strings.test.ts src/test/tokens_used.test.tsx && npm run lint
```

**Outcome**: every visible string resolves through `src/l10n/strings.ts` — the four engine keys
(`engine.calculated`, `engine.none`, `engine.modified`, `engine.recalculated`) exist and removing one
turns the suite red (ENG-27, SC-003, SC-004) — and the token scan finds no literal hex or px in
`web/src/engine/` (ENG-28, SC-008). The ESLint anti-sink rules cover the new `.tsx` sources.

---

## 9. The gates stay green

```bash
cd web && npm run format:check && npm run lint && npm run typecheck && npm test && npm run build
cd .. && make lint-specs
```

**Outcome**: all client gates pass and the spec lint reports clean, meaning every `ENG-*` citation in
the delivered sources resolves to `spec.md` and no assertion in the engine suite is vacuous. No new
dependency is added, so the lockfile and the security gates are unchanged.

---

## Traceability

| Scenario | Requirements |
|---|---|
| 1 | all `ENG-*` (no I/O) |
| 2 | ENG-07…ENG-13, ENG-22, ENG-27…ENG-29 |
| 3 | ENG-14…ENG-18, ENG-22 |
| 4 | ENG-01…ENG-06 |
| 5 | ENG-19…ENG-21 |
| 6 | ENG-22…ENG-26 |
| 7 | ENG-29 |
| 8 | ENG-27, ENG-28 |
| 9 | all `ENG-*` (lint) |

## Validation status

Implementation is deferred to the tasks step of this cycle (this cycle produces the spec, plan, tasks
and analysis only). The scenarios above are the acceptance guide the implementing commits must satisfy;
their per-scenario outcome table is appended when the implementation lands, as F3's quickstart records.

## Validation outcome (2026-10-16)

Run in the delivered tree; every scenario passed. The engine introduced no runtime dependency, so the
lockfile, the bundle and the security gates are unchanged.

| Scenario | Result |
|---|---|
| 1 — suite runs with no backend | PASS — `npm test`: 36 files, 286 passed, 5 live-suite skips |
| 2 — every control branch proven | PASS — `engine_field_control.test.tsx` |
| 3 — one exclusive group, one control | PASS — `engine_exclusive_control.test.tsx` |
| 4 — field tree derives into labeled sections | PASS — `engine_grouping.test.ts` |
| 5 — draft derived, overlaid, merged without mutation | PASS — `engine_defaults.test.ts` |
| 6 — composition seams and marks | PASS — `engine_form.test.tsx` + `engine_marks.test.tsx` |
| 7 — markup in data rendered as text | PASS — `engine_xss.test.tsx` |
| 8 — no copy or visual value authored in the engine | PASS — `l10n_strings.test.ts` + `tokens_used.test.tsx` + `npm run lint` |
| 9 — gates stay green | PASS — `format:check`, `lint`, `typecheck`, `npm test`, `npm run build`, `make lint-specs` |

No surface was wired: per the 2026-10-16 clarifications the feature proves itself through the engine's
own rendering tests, so no `web/e2e/` case and no test-only route were added.
