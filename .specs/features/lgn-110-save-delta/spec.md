# Save with Delta and Incremental Cascade Merge Specification

Source: [LGN-110](https://paulovalleriotedev.atlassian.net/browse/LGN-110) — "[Front End] Spec Fase 4 — Salvar com delta e merge incremental da cascata", under epic [LGN-93](https://paulovalleriotedev.atlassian.net/browse/LGN-93). Depends on LGN-108 (open sheet). Also bundles a creation-dialog layout fix (LGN-109/LGN-100 follow-up) into the same PR.

## Problem Statement

Screen 2 edits a stored sheet in memory and signals "Local edits", but there is no way to commit those edits: the API already recalculates on `PATCH /sheets/:id` and returns only the derived cascade delta, yet the desktop never sends it. Every edit is lost on pop. Two gaps remain: (a) the save cycle — a fixed footer with a Save action, a payload carrying only the changed editable delta, and an incremental merge of the returned cascade; and (b) the creation dialog's content overflows short windows, so the preview card and the Create/Close action row overlap. This feature ships the save cycle and the dialog layout fix in one pull request.

## Goals

- [ ] Add a fixed footer on Screen 2 with the dirty indicator and a Save action, disabled with no changes and with a `Ctrl+S` binding.
- [ ] Send exactly one `PATCH /sheets/:id` per commit, carrying only the changed editable delta.
- [ ] Merge the returned derived cascade into the in-memory `data` incrementally and clear the dirty state only on success.
- [ ] Keep local edits and render the mapped error copy inline on any failure; never auto-retry a PATCH.
- [ ] Keep the creation dialog's action row and the preview card non-overlapping at every supported window size.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Auto-save / debounce / save-on-keystroke | Card and TDD: commit is explicit via the Save action |
| Backend changes to `PATCH` / delta shape / DAG recalc | Already implemented and stable; the card consumes the existing contract |
| Per-field revert on `INVALID_REQUEST` / `UNKNOWN_FIELD` | The card supersedes the TDD's revert mapping: edits stay and the mapped message is shown |
| Optimistic merge, offline queue, conflict resolution, retry policy for PATCH | Card: no phantom merge offline; one request per commit, no auto-retry |
| Delete / import / export | Out of the epic scope |
| Multi-sheet / tabs / more than one active sheet | V2 (TDD Questão #5) |
| Dry-run preview, rate-limit, draft backup (LGN-103 / LGN-104) | Separate cards in the same phase |
| Humanized labels / group labels (LGN-99 follow-up) | Separate card |
| Redesigning the creation dialog beyond the overlap fix | The fix is local, per `desktop/AGENTS.md` "Critical Layout Review" |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | -------------- | --------- | ---------- |
| Where the dirty indicator and Save live | A fixed footer bar on Screen 2 replaces the header dirty marker; the header keeps only back + title + template id | Card scope ("Rodapé fixo da Tela 2: indicador dirty + botão Salvar") and the TDD Tela 2 wireframe | n |
| Save control copy and disabled affordance | Label `Save`; disabled while not pending with a tooltip `There are no changes to save.` (new l10n keys) | `desktop/AGENTS.md`: disabled primary actions explain themselves via tooltip | n |
| Inline error placement on failure | Inside the footer, directly above the action row | Pins the placement so the rendered-copy assertion is anchored (lesson L-004) | n |
| What happens to edits when a save fails | Every local edit stays; the dirty indicator stays; the mapped copy renders | Card AC ("edições locais permanecem + mensagem mapeada"); overrides the TDD's older revert-on-`INVALID_REQUEST` mapping | y |
| `Ctrl+S` scope | Registered in the central registry; dispatches only to the mounted sheet; inert with no active sheet and while a save is in flight | `desktop/AGENTS.md` keyboard registry; mirrors the F5 dispatch seam (GRN-17) | n |
| Baseline after a successful save | The seed baseline (`defaults`) becomes the merged `data`; the pending set clears | Card scope ("limpa o dirty state"); the next delta must be relative to the last successful save | n |
| Delta source | The pending paths whose current value differs from the baseline; every such path is editable because read-only controls never write, so derived leaves never enter the payload | Card AC ("campo derivado nunca incluído") plus the read-only rendering rule | n |
| Merge semantics | A recursive deep merge of the response delta onto `data`; a key the response does not carry is left untouched and the response is authoritative for the keys it carries | TDD §Fluxo de Dados ("merge incremental"); the response holds only derived effects | n |
| Save in flight vs. discard guards | While a save is in flight the back action and F5 do nothing, so an in-flight commit is never abandoned by a discard | The API may have committed; discarding would hide the response and desync the screen | n |
| Footer during load / error / empty states | The footer renders only once the sheet and its template are loaded; it is absent in loading, error and no-fields states | There is nothing to save before the form exists | n |
| Creation dialog overflow fix | Bound the dialog content to the available viewport so the preview card flexes, scrolls internally and the action row stays separate | Confirmed by reproduction: at 1280×720 the content `RenderFlex` overflows 39px and the submit rect intersects the preview rect (159px at 600px height); unbounded dialog content is the root cause and the fix is local per `desktop/AGENTS.md` | y |
| Backend impact | None — the existing `PATCH /sheets/:id` already validates editability, normalizes exclusive groups and returns only the derived delta | Card: consume the existing contract | y |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Commit local edits from a fixed footer ⭐ MVP

**User Story**: As the app user, I want a Save action in a fixed footer, enabled only when I have unsaved edits, so that I can commit my changes with one request.

**Why P1**: Without it nothing on Screen 2 can be persisted; this is the vertical slice the card asks for.

**Acceptance Criteria** (each line is one EARS pattern):

1. WHEN Screen 2 has loaded the stored sheet and its template, the system SHALL render a fixed footer bar containing the dirty indicator and the Save action.  <!-- event-driven -->
2. WHILE the edit state reports no pending change, the system SHALL keep the Save action disabled and SHALL expose a tooltip stating there are no changes to save.  <!-- state-driven -->
3. WHILE the edit state reports a pending change, the system SHALL enable the Save action.  <!-- state-driven -->
4. WHEN the Save action is activated while a change is pending, the system SHALL issue exactly one `PATCH /sheets/:id`.  <!-- event-driven -->
5. WHEN `Ctrl+S` is pressed while a change is pending and a sheet is active, the system SHALL issue exactly one `PATCH /sheets/:id`.  <!-- event-driven -->
6. WHILE a save request is in flight, the system SHALL keep the Save action disabled so a further activation or `Ctrl+S` issues no second `PATCH /sheets/:id`.  <!-- state-driven -->
7. WHILE no sheet is active, the system SHALL issue no `PATCH /sheets/:id` on `Ctrl+S`.  <!-- state-driven -->

**Independent Test**: With a mocked transport, open a sheet, prove the Save action is inert with no edits, edit a field, activate Save once and assert exactly one PATCH; repeat through `Ctrl+S` and assert exactly one PATCH; press `Ctrl+S` twice quickly and assert one PATCH; press `Ctrl+S` with no sheet open and assert zero PATCH requests.

---

### P1: The PATCH carries only the changed editable delta ⭐ MVP

**User Story**: As the owner of the sheet data, I want the commit to send only what I changed so that the API recomputes the rest and no derived value is written by the client.

**Why P1**: The card's core rule; a wrong payload corrupts derived data or sends noise.

**Acceptance Criteria**:

1. WHEN a save is issued, the system SHALL send a `data` object that contains exactly the editable leaves whose current value differs from the baseline, nested by their dotted path.  <!-- event-driven -->
2. WHEN an edited field is reverted to its baseline value, the system SHALL omit that field from the `data` object.  <!-- event-driven -->
3. The system SHALL never include a derived (non-editable) leaf in the `data` object.  <!-- ubiquitous -->
4. WHEN a save is issued after an exclusive-group selection, the system SHALL include the selected member and every sibling whose value differs from the baseline, so at most one member can be non-zero.  <!-- event-driven -->
5. WHILE no field differs from the baseline, the system SHALL issue no `PATCH /sheets/:id`.  <!-- state-driven -->

**Independent Test**: Load a sheet, edit two editable fields and revert one, activate Save and assert the body's `data` has exactly the still-changed field nested by path and no derived key; assert a reverted-only state issues no request; select an exclusive option and assert the delta carries the selected member.

---

### P1: Merge the cascade incrementally and clear dirty on success ⭐ MVP

**User Story**: As the app user, I want the recalculated values to appear as soon as the save succeeds so that the sheet reflects the API without a full reload.

**Why P1**: The card's "merge incremental da cascata"; a full reload would contradict the delta contract and lose the editing position.

**Acceptance Criteria**:

1. WHEN `PATCH /sheets/:id` responds `200`, the system SHALL deep-merge the returned nested delta into the in-memory `data` and SHALL leave every key the response does not carry unchanged.  <!-- event-driven -->
2. WHEN the merge completes, the system SHALL set the baseline to the merged data and SHALL clear the pending state so the dirty indicator disappears.  <!-- event-driven -->
3. WHEN the merge completes, the system SHALL rebuild only the controls whose value changed.  <!-- event-driven -->
4. WHEN the response delta is empty, the system SHALL update the baseline and clear the pending state without changing any rendered value.  <!-- event-driven -->
5. WHEN the merge completes, the system SHALL clear any previously rendered inline error copy.  <!-- event-driven -->

**Independent Test**: Respond to the PATCH with a new nested derived key and an overwritten one; assert both are merged, an untouched derived key keeps its stored value, the dirty indicator is gone, and a control watching a merged path shows the new value; respond with `{}` and assert the dirty state clears with unchanged values.

---

### P1: A failed save keeps the edits and reports the mapped error

**User Story**: As the app user, I want my edits to survive a failed save and see why it failed, so that I can correct and retry without losing work.

**Why P1**: The card requires no phantom merge and no data loss; silent failure would be worse than the bug.

**Acceptance Criteria**:

1. IF `PATCH /sheets/:id` fails for any reason (`INVALID_REQUEST`, `UNKNOWN_FIELD`, network/timeout, `5xx`), THEN the system SHALL keep every local edit and keep the dirty indicator visible.  <!-- unwanted-behavior -->
2. IF `PATCH /sheets/:id` fails, THEN the system SHALL render the mapped error copy inline in the footer above the action row.  <!-- unwanted-behavior -->
3. IF `PATCH /sheets/:id` fails, THEN the system SHALL leave the in-memory `data` unchanged, adding no cascade.  <!-- unwanted-behavior -->
4. WHEN Save is activated again after a failure, the system SHALL issue exactly one further `PATCH /sheets/:id`.  <!-- event-driven -->
5. The system SHALL NOT retry a `PATCH /sheets/:id` automatically.  <!-- ubiquitous -->
6. WHEN a save succeeds after an earlier failure, the system SHALL clear the inline error copy.  <!-- event-driven -->

**Independent Test**: Fail a save with `INVALID_REQUEST` and assert the edit is still rendered, the dirty indicator is present, the mapped copy is inline in the footer and the recorded PATCH count is one; activate Save again and assert one further PATCH; assert the client never re-issues a PATCH without a user action; then succeed and assert the copy is gone.

---

### P1: The creation dialog action never overlaps the preview ⭐ MVP

**User Story**: As the app user, I want the Create action and the preview card to stay visually separate so that I can read the preview and activate Create without content being covered.

**Why P1**: The overlap is a visible layout defect in the current creation flow that makes the primary action unreadable on short windows.

**Acceptance Criteria**:

1. WHEN the creation dialog is rendered, the system SHALL render the primary action in the dialog action row, visually separated from the preview card.  <!-- event-driven -->
2. WHEN the window height is shorter than the dialog's full content, the system SHALL constrain the dialog to the viewport so the preview scrolls internally and neither the dialog nor the action row overflows the viewport.  <!-- event-driven -->
3. The system SHALL keep the action row and the preview card non-overlapping at the minimum supported window size.  <!-- ubiquitous -->
4. WHEN the window is resized to the minimum supported size, the system SHALL keep the Create action reachable and the preview readable without clipping the action.  <!-- event-driven -->

**Independent Test**: Mount the creation dialog at the default size and at the minimum supported size, assert no `RenderFlex`/viewport overflow, and assert the submit button's rectangle does not intersect the preview card's rectangle; assert the preview still scrolls internally.

---

## Edge Cases

Edge cases are usually unwanted-behavior (IF/THEN) or boundary (WHEN) criteria:

- WHEN the primary action is activated twice while a save is in flight, the system SHALL issue exactly one `PATCH /sheets/:id`.
- WHEN `Ctrl+S` and the Save action are activated in the same frame, the system SHALL issue exactly one `PATCH /sheets/:id`.
- IF the back action is activated with a save in flight, THEN the system SHALL NOT discard the edits and SHALL issue no request.
- IF `F5` is pressed with a save in flight, THEN the system SHALL issue no template request and SHALL NOT discard the edits.
- WHEN the response delta nests deeper than one level, the system SHALL merge every nested key without flattening it.
- WHEN the response delta changes a derived value back to the baseline value, the system SHALL still clear the pending state.
- IF `PATCH /sheets/:id` responds with an error envelope instead of `data`, THEN the system SHALL keep the edits and render the mapped copy.
- WHEN the sheet template fails to load, the system SHALL NOT render the footer and SHALL NOT offer a Save action.
- WHEN the creation dialog is shown at the minimum supported width, the system SHALL keep the preview card bounded to the dialog content and SHALL NOT overflow horizontally.

---

## Requirement Traceability

Each requirement gets a unique ID for tracking across design, tasks, and validation.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| SAV-01 | P1: Commit local edits from a fixed footer | Design | Pending |
| SAV-02 | P1: Commit local edits from a fixed footer | Design | Pending |
| SAV-03 | P1: Commit local edits from a fixed footer | Design | Pending |
| SAV-04 | P1: Commit local edits from a fixed footer | Design | Pending |
| SAV-05 | P1: Commit local edits from a fixed footer | Design | Pending |
| SAV-06 | P1: Commit local edits from a fixed footer | Design | Pending |
| SAV-07 | P1: Commit local edits from a fixed footer | Design | Pending |
| SAV-08 | P1: The PATCH carries only the changed editable delta | Design | Pending |
| SAV-09 | P1: The PATCH carries only the changed editable delta | Design | Pending |
| SAV-10 | P1: The PATCH carries only the changed editable delta | Design | Pending |
| SAV-11 | P1: The PATCH carries only the changed editable delta | Design | Pending |
| SAV-12 | P1: The PATCH carries only the changed editable delta | Design | Pending |
| SAV-13 | P1: Merge the cascade incrementally and clear dirty on success | Design | Pending |
| SAV-14 | P1: Merge the cascade incrementally and clear dirty on success | Design | Pending |
| SAV-15 | P1: Merge the cascade incrementally and clear dirty on success | Design | Pending |
| SAV-16 | P1: Merge the cascade incrementally and clear dirty on success | Design | Pending |
| SAV-17 | P1: Merge the cascade incrementally and clear dirty on success | Design | Pending |
| SAV-18 | P1: A failed save keeps the edits and reports the mapped error | Design | Pending |
| SAV-19 | P1: A failed save keeps the edits and reports the mapped error | Design | Pending |
| SAV-20 | P1: A failed save keeps the edits and reports the mapped error | Design | Pending |
| SAV-21 | P1: A failed save keeps the edits and reports the mapped error | Design | Pending |
| SAV-22 | P1: A failed save keeps the edits and reports the mapped error | Design | Pending |
| SAV-23 | P1: A failed save keeps the edits and reports the mapped error | Design | Pending |
| DIA-01 | P1: The creation dialog action never overlaps the preview | Design | Pending |
| DIA-02 | P1: The creation dialog action never overlaps the preview | Design | Pending |
| DIA-03 | P1: The creation dialog action never overlaps the preview | Design | Pending |
| DIA-04 | P1: The creation dialog action never overlaps the preview | Design | Pending |

**ID format:** `SAV-[NUMBER]` for the save cycle, `DIA-[NUMBER]` for the dialog layout.

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 27 total, mapped to tasks at Tasks.

---

## Success Criteria

- [ ] `flutter test` green, including: Save inert with no edits, exactly-one-PATCH per commit (button and `Ctrl+S`), delta excludes derived and reverted fields, cascade merge (new key, overwritten key, deep nesting, empty delta), dirty cleared on success, edits kept and mapped copy rendered on failure, and single-flight.
- [ ] `flutter analyze` clean.
- [ ] At the default and minimum supported window sizes the creation dialog shows no overflow and the Create action does not intersect the preview card.
- [ ] Manual check: edit a field on an open sheet, Save, and see the recalculated cascade appear without a reload.
