# LGN-102 — Granular Per-Field Rebuild (Riverpod select/family) Specification

Source: [LGN-102](https://paulovalleriotedev.atlassian.net/browse/LGN-102) — "[Front End] Spec Fase 3 — Rebuild granular por campo (Riverpod select/family)".

## Problem Statement

Screen 2 renders the sheet but holds edit state in the screen widget, so any edit would re-render the whole form. For templates of up to ~200 fields that violates the TDD performance requirement. This feature isolates edit state per field with Riverpod select/family so one edit rebuilds only the affected field, keeping edits in memory (commit is Phase 4). The discussion pulled the local-edits marker and the leave/refresh discard guards forward from Phase 4 — recorded as user decisions in `context.md`.

## Goals

- [ ] One edit rebuilds only the edited field's control; no sibling control rebuilds.
- [ ] Edits are stored in an in-memory, per-sheet Riverpod state (family keyed by template id), seeded from schema defaults.
- [ ] A template of ≤200 fields renders its field controls in <100 ms after the API response arrives.
- [ ] No network request is ever issued by an edit (read-only stays read-only; no PATCH).
- [ ] Pending local edits are visible in the header, and both back and refresh warn before discarding them.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| `PATCH /sheets/:id`, delta, cascade merge | Phase 4 (card: "commit é Fase 4") |
| Save button, footer dirty bar, clearing dirty on successful save | Phase 4 — the card scopes the commit cycle there; only the header marker and discard guards moved forward |
| Reapplying pending edits across a reload (merge onto a fresh schema) | Merge logic is Phase 4; a confirmed reload discards and reseeds |
| Guarding app/window close | Neither the card nor the discussion covers leaving the app; only in-screen navigation is guarded |
| Rendering itself (grouping, control choice, labels) | Screen 2 spec (LGN-100/LGN-101), already shipped |
| Persisting edits (disk, local library, sessions) | Card: "sem persistência" |
| Backend/API changes | Frontend-only feature |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | --------------------------- | --------- | ---------- |
| Leave semantics: what happens to edits when leaving the sheet | Back with pending edits shows a destructive-confirmation dialog; confirming discards and navigates, cancelling stays with edits intact; back without pending edits navigates directly | User decision (discuss, Area 1): warn, then discard on proceed | y |
| Refresh semantics | New F5 refresh on the sheet screen refetches the template detail; pending edits trigger the same discard-confirmation first; confirming reseeds from the fresh schema | User decision (discuss, Area 2): warn before any reload that would discard | y |
| Visible signal of local-only edits | Small "local edits" caption marker in the sheet header; no Save affordance | User decision (discuss, Area 3): minimal visible marker, card boundary amended | y |
| What counts as pending | Edit state differs from the seeded defaults; reverting a field to its default removes its contribution (marker hides when everything matches) | Mirrors Phase 4's dirty definition ("differs from the source of truth") without merge logic | y |
| Seed source for edit state | `seedDefaults` (every leaf gets its schema default), the same derivation the read-only preview uses | One mechanism; the screen already seeds this way today | y |
| Reload failure after confirm | Edits are already discarded when the refetch starts (the loading state unmounts the form); a failure shows the mapped error + Retry and reseeds on success | The user confirmed the discard before the request ran | y |
| Marker placement vs footer preference | Header caption in Phase 3; `desktop/AGENTS.md`'s footer-bar preference applies to Phase 4's dirty + Save footer | No footer exists without Save; a lone footer indicator would look orphaned | y |
| How the <100 ms AC is gated | An automated widget test measures the real build time of the form frame after the API response; the TDD's DevTools/profile measurement remains the release check | Keeps the acceptance criterion enforced by the test runner, not by memory | y |
| Derived/`is_editable: false` controls on edit | Stay non-editable (existing Screen 2 behavior is preserved, not changed) | Card does not change control semantics | y |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Isolated in-memory edit state ⭐ MVP

**User Story**: As a user editing a sheet, I want each edit stored in a per-sheet in-memory state keyed by field path so that my edits stay local until I explicitly save (Phase 4).

**Why P1**: The card's Objective — state isolated per field, no global re-render, no persistence — is the foundation Phase 4's save cycle commits from.

**Acceptance Criteria** (each line is one EARS pattern):

1. WHEN the loaded template's edit state initializes, the system SHALL seed every leaf path with its schema default  <!-- event-driven -->
2. WHEN the user edits an editable field control, the system SHALL store the entered value at that field's dotted path in the in-memory edit state  <!-- event-driven -->
3. WHEN the user edits any field, the system SHALL issue no network request  <!-- event-driven -->
4. WHILE the sheet screen is mounted, the edit state SHALL exist only in memory and SHALL NOT be persisted or sent to the API  <!-- state-driven -->
5. WHILE a field schema has `is_editable: false`, its control SHALL remain non-interactive  <!-- state-driven -->
6. WHEN the user leaves the sheet screen, the system SHALL discard that sheet's edit state; reopening the same template SHALL reseed from defaults  <!-- event-driven -->

**Independent Test**: Open Screen 2, type into a field, toggle another, assert the recorded requests are GET-only, navigate away, reopen, and see defaults again.

---

### P1: Granular rebuild per field ⭐ MVP

**User Story**: As a user editing a large sheet, I want only the edited field's control to re-render so that edits stay instant on templates of up to ~200 fields.

**Why P1**: Second acceptance criterion of the card; without it the first edit re-renders the whole ficha.

**Acceptance Criteria**:

1. WHEN one field's stored value changes, the system SHALL rebuild that field's control  <!-- event-driven -->
2. WHEN one field's stored value changes, every other field's control SHALL keep its existing widget instance (no rebuild)  <!-- event-driven -->
3. WHEN the sheet screen rebuilds for a non-edit reason (template detail arrival), the field controls SHALL reflect the seeded values  <!-- event-driven -->

**Independent Test**: Snapshot the `SheetFieldControl` widget instances, edit one field, and assert every other instance is identical (`identical()`), while the edited field's control shows the new value.

---

### P1: Render performance on a full-size template

**User Story**: As a user opening a large template, I want the ficha to render within the TDD budget so that the screen feels instant after the API responds.

**Why P1**: First acceptance criterion of the card and a TDD performance requirement (`docs/TDD-desktop-mvp.md`, § Requisitos de Performance).

**Acceptance Criteria**:

1. WHEN the template detail response for a template of at most 200 fields arrives, the system SHALL render all field controls within 100 ms  <!-- event-driven -->

**Independent Test**: Widget test with a generated 200-field template; a `Stopwatch` around the frame that mounts the form asserts <100 ms.

---

### P1: Local-edit awareness and discard guards

**User Story**: As a user editing a sheet that cannot save yet, I want to see that my edits are local-only and be warned before any action discards them.

**Why P1**: User decision in the spec discussion (Areas 1–3); the card scoped dirty UI to Phase 4, and the user pulled the marker plus the leave/refresh guards forward into this ticket.

**Acceptance Criteria**:

1. WHEN the edit state differs from the seeded defaults, the system SHALL show a local-edits marker in the sheet header  <!-- event-driven -->
2. WHEN the edit state equals the seeded defaults, the system SHALL hide the local-edits marker  <!-- event-driven -->
3. WHEN the user activates the sheet's back action while the edit state differs from the seeded defaults, the system SHALL show a confirmation dialog stating that the local edits will be discarded  <!-- event-driven -->
4. IF the user confirms the leave dialog, then the system SHALL discard the edit state and navigate back  <!-- unwanted-behavior -->
5. IF the user cancels the leave dialog, then the system SHALL keep the sheet screen and its edit state  <!-- unwanted-behavior -->
6. WHEN the user activates the back action while the edit state equals the seeded defaults, the system SHALL navigate back without a dialog  <!-- event-driven -->
7. WHEN the user requests a template refresh (F5) on the sheet screen while no local edits are pending, the system SHALL refetch the template detail and reseed the edit state from the fresh schema  <!-- event-driven -->
8. IF local edits are pending when the user requests a refresh, then the system SHALL show the discard-confirmation dialog before refetching  <!-- unwanted-behavior -->
9. IF the user cancels the refresh dialog, then the system SHALL issue no template detail request  <!-- unwanted-behavior -->

**Independent Test**: Type a value, see the marker; press back, cancel the dialog, and stay; press back, confirm, and land on the catalog with the state gone; revert the value, press back, and leave without a dialog; press F5 clean vs pending and assert the request count.

---

## Edge Cases

- IF a numeric input holds an unparseable draft (`-`, `.`) when focus leaves, THEN the control SHALL restore the stored value (existing Screen 2 behavior must not regress).
- IF the template detail request fails, THEN the screen SHALL show the mapped error with Retry and no edit state SHALL be created.
- WHEN the template has no fields, THEN the screen SHALL show the empty-state line and create no edit state.
- WHEN the user reopens a different template, THEN its edit state SHALL be independent of the previous one (family isolation).
- WHEN a toggle (`config.*` proxy) is flipped, THEN the control SHALL display the new state without any other control rebuilding.
- WHEN the edit state reverts to the seeded defaults field by field, THEN the marker SHALL hide once the last pending edit is reverted.
- IF a confirmed refresh fails, THEN the screen SHALL show the mapped error with Retry and the edit state SHALL remain discarded, reseeding from defaults after a successful reload.
- WHEN a confirmation dialog is open, THEN it SHALL block further back/refresh actions until dismissed (modal barrier).

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| GRN-01 | P1: Isolated in-memory edit state | Execute | Verified |
| GRN-02 | P1: Isolated in-memory edit state | Execute | Implementing |
| GRN-03 | P1: Isolated in-memory edit state | Execute | Implementing |
| GRN-04 | P1: Isolated in-memory edit state | Execute | Verified |
| GRN-05 | P1: Isolated in-memory edit state | Design | Pending |
| GRN-06 | P1: Isolated in-memory edit state | Execute | Verified |
| GRN-07 | P1: Granular rebuild per field | Execute | Implementing |
| GRN-08 | P1: Granular rebuild per field | Execute | Implementing |
| GRN-09 | P1: Granular rebuild per field | Design | Pending |
| GRN-10 | P1: Render performance on a full-size template | Design | Pending |
| GRN-11 | P1: Local-edit awareness and discard guards | Design | Pending |
| GRN-12 | P1: Local-edit awareness and discard guards | Design | Pending |
| GRN-13 | P1: Local-edit awareness and discard guards | Design | Pending |
| GRN-14 | P1: Local-edit awareness and discard guards | Design | Pending |
| GRN-15 | P1: Local-edit awareness and discard guards | Design | Pending |
| GRN-16 | P1: Local-edit awareness and discard guards | Design | Pending |
| GRN-17 | P1: Local-edit awareness and discard guards | Design | Pending |
| GRN-18 | P1: Local-edit awareness and discard guards | Design | Pending |
| GRN-19 | P1: Local-edit awareness and discard guards | Design | Pending |

**ID format:** `GRN-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 19 total, 19 mapped to tasks, 0 unmapped

---

## Success Criteria

- [ ] `flutter test` green, including: sibling-identity granularity test, GET-only request assertion, 200-field <100 ms test.
- [ ] Editing a field in a 200-field sheet rebuilds exactly one control (identity check).
- [ ] Marker appears on first pending edit and hides when the state returns to defaults.
- [ ] Back with pending edits opens the confirmation; cancel keeps state, confirm discards; back clean skips the dialog.
- [ ] F5 without pending edits refetches once; F5 with pending edits confirms first and a cancel issues zero requests.
- [ ] Reopening a template after leaving shows schema defaults (state was discarded).
