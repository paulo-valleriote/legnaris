# Tasks: Web UI Primitives

**Input**: Design documents from `.specs/features/009-web-ui-primitives/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: required — constitution III makes behavior ships with tests non-negotiable. Each story
opens with its suites (edge cases first) and they MUST fail before that story's implementation. The
primitives are pure rendering behavior, so the suites render each primitive directly; no suite needs
a backend, a route or the network.

**Organization**: tasks are grouped by user story so each story is independently implementable and
testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no incomplete dependency)
- **[Story]**: the user story the task belongs to
- Exact file paths are in every description

## Path Conventions

- Client layer sources: `web/src/ui/`; client tests: `web/src/test/` (flat, the client layout)
- Shared copy: `web/src/l10n/strings.ts`
- Documents: `web/DESIGN.md`, `web/AGENTS.md`, `web/README.md`
- Lint scripts: `scripts/`; decisions: `.specs/STATE.md`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: pin down the starting point so a later red is attributable to this feature.

- [x] T001 Run the baseline on the feature branch — `cd web && npm test && npm run lint && npm run typecheck` — and record the green result. No file changes; this is the reference every story checkpoint compares against.

**Checkpoint**: the client suites, lint and typecheck are green before any change.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the layer's skeleton and the three gates — the import rule, the two scans and the
citation lint — every story is measured by. No user story starts until this phase completes, and each
task here is green on its own.

- [X] T002 [P] `web/src/test/ui_layer.test.ts` — edge first: a module whose import specifier names a feature directory (for example `../catalog/…`) is reported, a barrel (`index.ts`) under `src/ui/` is reported, a missing `ui.css` is reported; happy path: `web/src/ui/` exists with its one stylesheet and no file under it names a feature directory (UIP-02). Fails until T006.
- [X] T003 [P] Extend `web/src/test/tokens_used.test.tsx` — add a `web/src/ui/` block: edge first, a literal hex colour or `NNpx` value under `src/ui/` is reported and a `var(--lg-*)` that no token declares is reported; happy path: the layer is clean and every token it references resolves (UIP-04, SC-002). Fails until T006.
- [x] T004 [P] Extend `web/src/test/l10n_strings.test.ts` — add a `web/src/ui/` block: edge first, a JSX text node that is a user-facing literal is reported and a `ui.*` key the catalog does not declare is reported; happy path: every key the layer renders resolves (UIP-05, SC-004). Fails until T006/T007.
- [X] T005 [P] Extend `scripts/check_requirement_ids.py` — add `.css` to the walked suffix set — and add its self-test case to `scripts/test_lints.py`: edge first, a `.css` file citing an unknown ID is flagged, a commented citation is still read and an import line is skipped; happy path, a `.css` file citing a known ID passes and the whole corpus stays clean (UIP-06, `R7`). Safe on the current corpus: every ID already cited in a client stylesheet resolves.
- [X] T006 [P] `web/src/ui/ui.css` — create the layer's single stylesheet with its header comment naming the class convention `lg-<block>__<element>--<modifier>`; token-only, no literal colour and no pixel value (UIP-01, UIP-04, UIP-06, `R1`).
- [X] T007 [P] `web/src/l10n/strings.ts` — add the layer's one chrome key `ui.dismiss` (UIP-24).

**Checkpoint**: `web/src/ui/` exists with its one stylesheet, both scans cover it, the requirement-ID
lint walks stylesheets, and the notice's one chrome key is declared.

---

## Phase 3: User Story 1 - One action primitive, and one sanctioned home for it (Priority: P1) 🎯 MVP

**Goal**: the action primitive in every variant, size and state, and the developer-rules document
that sanctions the layer while keeping the "three real usages" rule for every other extraction.

**Independent Test**: run `ui_button.test.tsx` and `design_guidelines.test.ts` — the action renders
its whole contract, a disabled or loading action cannot run, and both documents carry the rule change.

### Tests for User Story 1

- [x] T008 [P] [US1] `web/src/test/ui_button.test.tsx` — edge first: activating a disabled action does not run its handler, activating a loading one does not either, and an action with no explicit type inside a form is still a plain button; happy path: the five variants render their declared classes with danger outlined rather than filled, both sizes render at or above the declared control height, focus-visible is reachable, loading shows the caller's in-flight label and announces busy, and the accessible name comes from the content or an explicit name (UIP-07…UIP-10, SC-001, SC-003). Fails until T010/T011.
- [x] T009 [P] [US1] Extend `web/src/test/design_guidelines.test.ts` — edge first: a document that still forbids a base component layer is reported, a developer-rules document that drops the "three real usages" rule is reported; happy path: `web/DESIGN.md` names `web/src/ui/`, its five primitives and the class convention, and `web/AGENTS.md` sanctions the layer while keeping the ≥3-usages rule (UIP-03, SC-007). Fails until T012/T013.

### Implementation for User Story 1

- [X] T010 [US1] `web/src/ui/button.tsx` — the action primitive: five variants, two sizes, the declared states with the disabled/loading activation guard, a real button with a plain default type, and its label resolved through the props (UIP-07…UIP-10, `R9`).
- [X] T011 [US1] `web/src/ui/ui.css` — append the action block: variant surfaces, size steps and the hover/pressed/focus/disabled/loading states, token-only (UIP-07, UIP-08, UIP-09).
- [X] T012 [US1] `web/DESIGN.md` — add the layer to "Component specifications": its path, its primitive set, its class convention and the action's variants, sizes and states (UIP-03).
- [X] T013 [US1] `web/AGENTS.md` — replace the Componentization bullet `No "base component" layers, no wrapper-for-wrapper's-sake.` with the sanction of `web/src/ui/` as the one place a base component may live, and keep the "three real usages" rule for every other extraction (UIP-03, SC-007).
- [X] T014 [P] [US1] `.specs/STATE.md` — record `AD-019`: the base-layer rule change and why (one leaf layer over the tokens), with the rejected alternative (duplicating the action per screen) (UIP-03).

**Checkpoint**: the action primitive is complete and asserted, one document sanctions the layer, and
the developer-rules document keeps the extraction rule — the layer is the MVP.

---

## Phase 4: User Story 2 - Inputs that are labelled and errors that are announced (Priority: P1)

**Goal**: the field primitive (text input, select, text area) with a permanent label, an announced
error, a disabled state and a numeric presentation, plus the labelled boolean checkbox.

**Independent Test**: run `ui_field.test.tsx` and `ui_checkbox.test.tsx` — every field control is
labelled and shares its box, the error association appears only with an error, and the checkbox reports
a boolean with its states exposed.

### Tests for User Story 2

- [X] T015 [P] [US2] `web/src/test/ui_field.test.tsx` — edge first: with no error there is no invalid marking and no described-by association, a disabled control cannot be edited and is announced as disabled, and a placeholder is never the only label; happy path: the label is associated with the control, the input, the select and the text area share height, border, radius and focus values, an error renders inline on an invalid control and is associated with it, and a numeric field right-aligns its value with tabular figures (UIP-11…UIP-15, SC-001). Fails until T017/T019.
- [X] T016 [P] [US2] `web/src/test/ui_checkbox.test.tsx` — edge first: an unchecked box reports false and a value that is not a boolean is not coerced (the caller adapts a numeric proxy); happy path: the label is permanently visible and associated with the box, and the checked, disabled and invalid states are each exposed (UIP-16, UIP-17). Fails until T018/T019.

### Implementation for User Story 2

- [X] T017 [US2] `web/src/ui/field.tsx` — the field primitive: a permanent label bound to the control, the three controls, the inline error with its invalid marking and association, the disabled state and the numeric presentation (UIP-11…UIP-15).
- [X] T018 [US2] `web/src/ui/checkbox.tsx` — the labelled boolean primitive with its checked, disabled and invalid states (UIP-16, UIP-17).
- [X] T019 [US2] `web/src/ui/ui.css` — append the field and checkbox blocks: the shared control box, the label, the inline error, the disabled and invalid states and the numeric alignment, token-only (UIP-11, UIP-12, UIP-13, UIP-14, UIP-15, UIP-16).

**Checkpoint**: every field control is labelled and shares its box, the error is announced only when
present, and the checkbox is a labelled boolean — the form contract the later screens consume.

---

## Phase 5: User Story 3 - One dialog pattern, with the keyboard inside it (Priority: P1)

**Goal**: one overlay-plus-panel dialog with a title that names it, the focus moved in and confined by
a standalone module, Escape and the scrim each closing the topmost surface once, and the focus handed
back to the opener.

**Independent Test**: run `ui_dialog.test.tsx` and `ui_focus_trap.test.ts` — the dialog's role, name,
focus move, confinement, dismissal paths and focus return all hold, and the trap holds directly with
zero, one and many controls.

### Tests for User Story 3

- [X] T020 [P] [US3] `web/src/test/ui_focus_trap.test.ts` — edge first: with zero focusable controls the container stays focusable and focus never moves outside; with exactly one control forward and backward both land on it; happy path: several controls cycle in both directions, wrapping at both ends, and focus forced outside the container is pulled back in (UIP-22). Fails until T022.
- [x] T021 [P] [US3] `web/src/test/ui_dialog.test.tsx` — edge first: a destructive action is never the control that carries focus on open, an activation inside the panel does not dismiss, and a container with no focusable control stays dismissible; happy path: the dialog exposes its role with modal semantics, its name comes from the title and its description is exposed, focus lands inside on open, Tab and Shift+Tab wrap at the ends, Escape dismisses the topmost surface only, a scrim activation dismisses, and each close notifies the caller once and returns focus to the opener (UIP-18…UIP-21, SC-001, SC-003). Fails until T023/T024.

### Implementation for User Story 3

- [X] T022 [US3] `web/src/ui/focus_trap.ts` — the standalone focus-confinement module exposing the container and moving focus forward and backward with the zero/one/many rules (UIP-22, `R5`).
- [X] T023 [US3] `web/src/ui/dialog.tsx` — the overlay-plus-panel primitive: the scrim, the raised panel, the dialog role with modal semantics, the title-and-description wiring, the trap from T022, the Escape and scrim dismissal of the topmost surface only, and the focus return on close (UIP-18, UIP-19, UIP-20, UIP-21).
- [X] T024 [US3] `web/src/ui/ui.css` — append the overlay/dialog block: the scrim on the overlay token, the panel elevation, the large radius and the hairline border, token-only (UIP-18).

**Checkpoint**: any overlay the product opens is the same surface, with the keyboard confined and
returned — the duplication the three hand-rolled overlays carry is removed.

---

## Phase 6: User Story 4 - A notice that does not interrupt (Priority: P2)

**Goal**: a polite, non-blocking notice with a catalog-resolved dismiss control, four system states
and no timer of its own.

**Independent Test**: run `ui_toast.test.tsx` — the live region, the dismiss name, the text rendering,
the four variants, the untouched focus and the absence of self-dismissal are all proven.

### Tests for User Story 4

- [X] T025 [P] [US4] `web/src/test/ui_toast.test.tsx` — edge first: an empty or whitespace-only message renders no notice, focus is untouched when a notice appears, and advancing the virtual clock does not remove a notice; happy path: it is a polite live region with no scrim and no modal semantics, its dismiss control's accessible name resolves through the catalog, its message renders as text, and the four declared variants are distinguished (UIP-23…UIP-26, SC-001). Fails until T026/T027.

### Implementation for User Story 4

- [X] T026 [US4] `web/src/ui/toast.tsx` — the non-blocking notice primitive: the polite live region, the text message with the empty-message guard, the catalog-resolved dismiss control, the four variants and no timer (UIP-23, UIP-24, UIP-25, UIP-26, `R6`).
- [X] T027 [US4] `web/src/ui/ui.css` — append the notice block: the four variant stripes and tints with the text carrying the meaning, token-only (UIP-25).

**Checkpoint**: the product has one non-blocking feedback pattern, and it is the lowest-risk slice —
it can be dropped without touching any other story.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: the layer-completeness assertion, the recorded deviations, the quickstart run and the
final gates.

- [X] T028 [P] Extend `web/src/test/ui_layer.test.ts` — assert the layer holds exactly the five primitive modules (`button.tsx`, `field.tsx`, `checkbox.tsx`, `dialog.tsx`, `toast.tsx`) plus the focus helper (`focus_trap.ts`) and no unexpected module (UIP-01).
- [X] T029 [P] Update `web/README.md` (the new layer, its primitives and the two extended scans) and add this feature's delivered deviations to the `web/AGENTS.md` log (the stylesheet-walker extension, the standalone focus module, the notice's no-timer boundary).
- [X] T030 Run the [quickstart.md](./quickstart.md) scenarios end to end (`cd web && npm test`, the individual suites, `npm run lint`, `npm run build`) and `make lint-specs`; confirm no dependency was added (SC-005, SC-006).
- [X] T031 Final gates: `make fmt && make lint && make vet && make lint-specs` from the repo root and `make lint-web && make test-web && make sec-scan` from the client, with the full suite green.

**Total tasks**: 31.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001)**: no dependencies.
- **Foundational (T002–T007)**: depends on nothing but the baseline — **blocks all user stories**.
- **US1 (T008–T014)**: depends on Foundational (T006's stylesheet, T007's key).
- **US2 (T015–T019)**: depends on Foundational; T019 appends to the stylesheet T011 opened, so T011 precedes it.
- **US3 (T020–T024)**: depends on Foundational; T024 appends after T019, and T023 consumes T022.
- **US4 (T025–T027)**: depends on Foundational; T027 appends after T024, and T026 consumes T007's `ui.dismiss`.
- **Polish (T028–T031)**: depends on all desired stories; T028 is only green once US1–US4 exist.

### User Story Dependencies

- **US1 (P1)**: no dependency on other stories — it is the MVP and the layer's governance.
- **US2 (P1)**: independently testable through its two suites; it reuses the layer T006 created but does not touch the action.
- **US3 (P1)**: independently testable through its suites; it uses the layer T006 created and shares only the stylesheet with US1/US2 (append order, no logic coupling).
- **US4 (P2)**: independently testable through its suite; it consumes the chrome key T007 declared and shares only the stylesheet append.

### Within Each User Story

- Tests are written first and must fail before the story's implementation.
- The shared pieces come before their consumers (`focus_trap.ts` before `dialog.tsx`; the stylesheet
  blocks appended in story order).
- The stylesheet is one file, so its per-story appends (T011 → T019 → T024 → T027) are sequential.
- A story is complete before the next priority starts.

### Parallel Opportunities

- Foundational: `T002`–`T007` in parallel (different files).
- US1 tests `T008`/`T009` in parallel; then `T010` → `T011`, `T012`/`T014` in parallel, `T013` after `T012`.
- US2 tests `T015`/`T016` in parallel; then `T017`/`T018` in parallel, `T019` after both.
- US3 tests `T020`/`T021` in parallel; then `T022` → `T023` → the `T024` stylesheet append.
- US4: `T025` first, then `T026` → `T027`.
- Polish: `T028`/`T029` in parallel; `T030` after the stories; `T031` last.

---

## Parallel Example: User Story 2

```bash
# Launch both US2 suites together (they fail until T017–T019):
Task: "ui_field.test.tsx — label, error, disabled, numeric, three controls (UIP-11…UIP-15)"
Task: "ui_checkbox.test.tsx — label, states, boolean contract (UIP-16, UIP-17)"

# Then the two independent primitive modules together:
Task: "field.tsx — the field primitive (UIP-11…UIP-15)"
Task: "checkbox.tsx — the labelled boolean (UIP-16, UIP-17)"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Setup (T001) and Foundational (T002–T007).
2. Complete US1 (T008–T014): the layer, the action primitive and the rule change.
3. **STOP and VALIDATE** independently (quickstart scenarios 2, 3, 5, 11).
4. Demo: every screen's action can be drawn by one primitive, and the developer-rules document points
   at the layer.

### Incremental Delivery

1. Setup + Foundational → the layer skeleton, the scans and the citation gate.
2. US1 → the action primitive and the sanctioned home (MVP).
3. US2 → the labelled fields and the boolean checkbox.
4. US3 → the one trapped, restorable dialog.
5. US4 → the non-blocking notice.
6. Polish → the completeness assertion, the deviations, the quickstart and the gate run.

---

## Requirement Coverage

| Story | Requirements | Success criteria |
|---|---|---|
| Foundational | UIP-01, UIP-02, UIP-04, UIP-05, UIP-06 | SC-002, SC-004, SC-006 |
| US1 | UIP-01, UIP-03, UIP-07…UIP-10 | SC-001, SC-003, SC-007 |
| US2 | UIP-11…UIP-17 | SC-001, SC-003 |
| US3 | UIP-18…UIP-22 | SC-001, SC-003 |
| US4 | UIP-23…UIP-26 | SC-001 |
| Polish | UIP-01 | SC-005, SC-006 |

Every `UIP-*` requirement and every `SC-*` outcome maps to at least one task above; UIP-01 spans the
Foundational stylesheet (T006) and the Polish completeness assertion (T028), which is why it appears
in both, and the layer-wide scans (UIP-04, UIP-05, UIP-06) sit in Foundational because every story is
measured by them.

---

## Notes

- `[P]` tasks target different files with no incomplete dependency; the four stylesheet appends are
  deliberately sequential because they share `web/src/ui/ui.css`.
- `[Story]` labels trace a task to its user story.
- Commit after each task or logical group; every behavior change ships its tests in the same commit.
- No dependency is added, so the SCA/SAST/secret gates see nothing new to vet.
- Stop at any checkpoint to validate a story independently.
