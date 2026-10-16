# Feature Specification: Web Catalog

**Feature Branch**: `003-web-catalog`

**Created**: 2026-10-16

**Status**: Draft

**Input**: User description: "Web catalog: list templates and open a template's creation surface — the content region lists the templates the service exposes, and activating one opens a creation surface that loads the template detail, offers a sheet-name field with required/trimmed validation, and issues no mutation."

## Clarifications

### Session 2026-10-16

- Q: The plan's F3 row lists no catalog refresh, while the web `AGENTS.md` documents F5 refresh as a V1 default and the desktop catalog phase shipped it (CAT-18/19). How should the catalog refresh behave in F3? → A: Deferred — the catalog loads once per app load and caches in memory; the F5 shortcut and the central shortcut registry land with the sheet-cycle feature (F5-T005), which the plan already owns.
- Q: The plan says F3 adds catalog + detail queries and that F4 is the UI engine that renders the field tree. Beyond the sheet-name field, what should the creation surface render from the template detail in F3? → A: The template's display name only; schema-driven fields, grouping and exclusive options belong to the UI-engine feature (F4), so no grouping logic is built here.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse the template catalog (Priority: P1)

A person opens the product and the content region lists the templates the service exposes. While the
request is in flight they see a loading state; if there are no templates they see one plain line; if
the request fails they see the reason and a way to try again; and when it succeeds they see the
templates in the order the service returned them. A response the client cannot read becomes the same
clear error state rather than a blank or half-filled screen.

**Why this priority**: it is the first real content in the shell — every later screen (the creation
surface, the sheet cycle) starts from a template chosen here, and the shell's designed empty state
exists only until this list lands.

**Independent Test**: with a scripted transport, drive the catalog through a slow response, a list of
two entries, an empty list, a failure response, and a malformed payload, and confirm five distinct
render situations plus a call counter proving exactly one request per activation.

**Acceptance Scenarios**:

1. **Given** the app has loaded, **When** the catalog is shown, **Then** the content region renders
   the catalog list in place of the designed empty-workspace state. (WCT-01)
2. **Given** the catalog screen is first shown, **When** it renders, **Then** it requests the template
   catalog exactly once and shows the loading state until the response settles. (WCT-02)
3. **Given** the first catalog request is in flight, **When** the screen renders, **Then** it shows
   the loading state and does not show the empty state. (WCT-03)
4. **Given** the catalog request succeeds with N entries, **When** the list renders, **Then** it shows
   N entries in the order the service returned them, each with the template name. (WCT-04)
5. **Given** the catalog request succeeds with no entries, **When** the list renders, **Then** it shows
   a single line of plain localized text and no decorative art. (WCT-05)
6. **Given** the first catalog request fails, **When** the screen renders, **Then** it shows the mapped
   failure message together with a Retry action. (WCT-06)
7. **Given** the catalog is in its error state, **When** Retry is activated, **Then** the catalog
   re-requests exactly once. (WCT-07)
8. **Given** a catalog request is in flight, **When** Retry is activated again, **Then** no second
   concurrent request starts. (WCT-08)
9. **Given** the catalog response cannot be decoded — a missing payload key or a malformed entry —
   **When** the screen renders, **Then** it shows the defensive parse failure as the error state with
   Retry, never a blank screen and never a partial list. (WCT-09)
10. **Given** a catalog entry whose name is an empty string, **When** the list renders, **Then** the
    entry still renders without throwing and the layout stays intact. (WCT-10)

---

### User Story 2 - Open a template's creation surface (Priority: P1)

A person activates a template in the catalog and a creation surface for that template opens. It loads
the template's detail, shows a loading state while that request is in flight, shows the template's name
when it succeeds, and — on a failure or a template that no longer exists — shows the mapped reason
inside the surface with a way to try again, without closing. Dismissing the surface returns to the
catalog exactly as it was.

**Why this priority**: choosing a template only matters if it opens the surface where the sheet is
named; it is the second half of the phase's user story ("list **and open** templates").

**Independent Test**: activate two different catalog entries in turn and confirm the surface opens for
the right identifier and loads its own detail; then drive a slow response, a failure, a missing
template and a malformed payload through the surface and confirm the inline states; then dismiss with
Esc and confirm the catalog list is unchanged and no catalog request was repeated.

**Acceptance Scenarios**:

1. **Given** the catalog lists a template, **When** the entry is activated, **Then** the creation
   surface opens for that template identifier. (WCT-11)
2. **Given** the creation surface opens, **When** it renders, **Then** it requests the template detail
   for that identifier exactly once and shows a loading state in the detail area until the response
   settles. (WCT-12)
3. **Given** a detail request is in flight, **When** the surface renders, **Then** it shows the loading
   state and does not show a previously opened template's detail. (WCT-13)
4. **Given** the detail request succeeds, **When** the surface renders, **Then** it shows the
   template's display name from the detail response. (WCT-14)
5. **Given** the detail request fails, **When** the surface renders, **Then** it shows the mapped
   message inline in the surface and keeps the surface open. (WCT-15)
6. **Given** the detail request failed, **When** Retry is activated, **Then** the surface re-requests
   the detail exactly once. (WCT-16)
7. **Given** the requested template does not exist, **When** the surface renders, **Then** it shows the
   mapped missing-template message inline instead of closing or failing. (WCT-17)
8. **Given** the detail response cannot be decoded, **When** the surface renders, **Then** it shows the
   defensive parse failure inline and never a half-rendered detail. (WCT-18)
9. **Given** the creation surface is open, **When** it is dismissed with Esc or its close action,
   **Then** it returns to the catalog with the list intact and issues no catalog request. (WCT-19)
10. **Given** the creation surface is a catalog overlay, **When** it is opened and dismissed, **Then**
    the app keeps navigation shallow and returns to the catalog unchanged. (WCT-20)
11. **Given** a template was opened before, **When** the same template is opened again, **Then** the
    surface requests the detail again — one request per opening. (WCT-21)

---

### User Story 3 - Name the sheet in the creation surface (Priority: P1)

A person types a sheet name into the creation surface. The surface ships the name field and its
validation: the primary action stays disabled, with an explanation, until the name is not just
whitespace, and enables once it is a real name. This feature delivers the surface and the validation;
the action that actually creates the sheet arrives with the sheet-cycle feature.

**Why this priority**: the surface is only trustworthy if its one input is validated; shipping the field
without validation would let an empty name reach the creation step later.

**Independent Test**: open the surface, confirm the field starts empty with a localized label and
placeholder and the primary action is disabled with a tooltip; type spaces and confirm it stays
disabled; type a name and confirm it enables; activate the action and confirm zero requests were made.

**Acceptance Scenarios**:

1. **Given** the surface is open, **When** it renders, **Then** it shows a sheet-name text input, empty
   by default, with a localized label and example placeholder. (WCT-22)
2. **Given** the sheet-name value is empty or only whitespace, **When** the surface renders, **Then**
   the primary action is disabled and exposes a tooltip stating that a sheet name is required. (WCT-23)
3. **Given** the trimmed sheet-name value is non-empty, **When** the surface renders, **Then** the
   primary action is enabled. (WCT-24)
4. **Given** the surface is open, **When** the person types and activates the primary action, **Then**
   no mutating request is issued — the surface performs no creation in this feature. (WCT-25)

---

### Edge Cases

- **Whitespace-only name**: a name of spaces or tabs counts as empty; the primary action stays disabled
  (WCT-23).
- **Leading/trailing whitespace**: validation uses the trimmed value, so `"  Aragorn  "` enables the
  action (WCT-24).
- **Empty template name**: the surface renders with an empty title instead of throwing (WCT-14).
- **Empty catalog-entry name**: the entry renders with the layout intact (WCT-10).
- **Missing payload key or malformed entry**: the catalog takes the defensive parse failure path — the
  error state with Retry, never a blank screen and never a partial list (WCT-09).
- **Decodable-but-wrong detail shape**: the surface shows the defensive parse failure inline rather than
  a half-rendered detail (WCT-18).
- **Template that no longer exists**: the mapped missing-template message is shown inline; the surface
  stays open (WCT-17).
- **Reopening a template**: a fresh detail request is issued per opening — no stale reuse (WCT-21).
- **Trigger during flight**: a second Retry (catalog or detail) while a request is in flight starts no
  second concurrent request (WCT-08, WCT-16).
- **Dismissing mid-flight**: closing the surface while the detail request is in flight surfaces no error
  afterwards and issues no further request.
- **Dimension sweep** (explicitly covered / not applicable here):
  - *Input validation and bounds* — the sheet-name trim/required rule (WCT-23, WCT-24) and defensive
    payload parsing (WCT-09, WCT-18).
  - *Failure and partial failure* — first-load failure (WCT-06), detail failure (WCT-15), malformed
    payload (WCT-09, WCT-18), missing template (WCT-17).
  - *Idempotency and retry* — exactly one request per first load and per Retry (WCT-02, WCT-07, WCT-12,
    WCT-16), no concurrent duplicate (WCT-08).
  - *Concurrency and ordering* — no stale previous detail while a new one is in flight (WCT-13); the
    list preserves the service's order (WCT-04).
  - *State-transition integrity* — open/dismiss returns to the catalog unchanged (WCT-19, WCT-20).
  - *Not applicable*: authentication and rate limits (this feature attaches no credential; session
    handling is a later feature), data lifecycle and expiry (the catalog cache is in memory only and
    holds no credentials), observability (local logging belongs to a later feature).

## Requirements *(mandatory)*

### Functional Requirements

**The catalog list**

- **WCT-01**: The content region MUST render the template catalog in place of the designed
  empty-workspace state once this feature lands.
- **WCT-02**: WHEN the catalog screen is first shown, the client MUST request the template catalog
  exactly once and MUST show a loading state until the response settles.
- **WCT-03**: WHILE the first catalog request is in flight, the client MUST show the loading state and
  MUST NOT show the empty state.
- **WCT-04**: WHEN the catalog request succeeds with N entries, the client MUST render N entries in the
  order the service returned them, each showing the template name.
- **WCT-05**: IF the catalog request succeeds with an empty list, THEN the client MUST show a single
  line of plain localized text and MUST NOT show decorative art.
- **WCT-06**: IF the first catalog request fails, THEN the client MUST show the mapped failure message
  produced by the shared error mapper together with a Retry action.
- **WCT-07**: WHEN the Retry action is activated, the client MUST re-request the catalog exactly once.
- **WCT-08**: WHILE a catalog request is in flight, a further trigger MUST NOT start a second concurrent
  request.
- **WCT-09**: IF a catalog response cannot be decoded — a missing payload key or a malformed entry —
  THEN the client MUST show the defensive parse failure as the error state with Retry, and MUST NOT
  show a blank screen or a partial list.
- **WCT-10**: WHEN a catalog entry's name is an empty string, the entry MUST still render without
  throwing and the layout MUST stay intact.

**Opening a template's creation surface**

- **WCT-11**: WHEN a catalog entry is activated, the client MUST open the creation surface for that
  template identifier.
- **WCT-12**: WHEN the creation surface opens, the client MUST request the template detail for that
  identifier exactly once and MUST show a loading state in the detail area until the response settles.
- **WCT-13**: WHILE a detail request is in flight, the client MUST show the loading state and MUST NOT
  show a previously opened template's detail.
- **WCT-14**: WHEN the detail request succeeds, the client MUST show the template's display name from
  the detail response.
- **WCT-15**: IF the detail request fails, THEN the client MUST show the mapped message inline in the
  surface and MUST keep the surface open.
- **WCT-16**: WHEN the surface's Retry action is activated, the client MUST re-request the detail
  exactly once.
- **WCT-17**: IF the requested template does not exist, THEN the client MUST show the mapped
  missing-template message inline and MUST NOT close or crash.
- **WCT-18**: IF the detail response cannot be decoded, THEN the client MUST show the defensive parse
  failure inline and MUST NOT render a half-filled detail.
- **WCT-19**: WHEN the surface is dismissed with Esc or its close action, the client MUST return to the
  catalog with the list intact and MUST NOT request the catalog again.
- **WCT-20**: The client MUST keep navigation shallow: the creation surface is shown over the catalog,
  and dismissing it returns to the catalog unchanged.
- **WCT-21**: WHEN the same template is opened again, the client MUST request its detail again — one
  request per opening.

**Naming the sheet**

- **WCT-22**: WHEN the surface is open, it MUST show a sheet-name text input, empty by default, with a
  localized label and an example placeholder.
- **WCT-23**: WHILE the sheet-name value is empty or whitespace-only, the primary action MUST be
  disabled and MUST expose a tooltip stating that a sheet name is required.
- **WCT-24**: WHEN the trimmed sheet-name value is non-empty, the primary action MUST be enabled.
- **WCT-25**: The creation surface MUST be side-effect free in this feature: opening it, typing in it
  and activating the primary action MUST NOT issue a mutating request.

**Cross-cutting**

- **WCT-26**: Every user-visible string MUST resolve through the shared translation catalog; no
  user-facing literal may appear in a component.
- **WCT-27**: The catalog and the creation surface MUST be operable by keyboard alone: Esc dismisses the
  surface, focus moves into the surface on open and returns to the triggering entry on close, and every
  control exposes a visible focus state.
- **WCT-28**: The catalog and the creation surface MUST derive every visual value (spacing, type, radius,
  color, focus, hover) from the shared token set; no ad-hoc visual value may be introduced.

### Key Entities

- **Template catalog entry / template detail**: the decoded template — identifier, display name, default
  flag and field tree — as the Web API Client already models it. The catalog and the detail share one
  shape; the catalog renders the name and identifier, and the creation surface renders the detail's
  display name.
- **Creation surface**: the overlay for one template; carries the template identifier, the detail
  request's state (loading, loaded, failed), the sheet-name value, and the primary action's enabled
  state.
- **Catalog render state**: the list request's state (loading, loaded, empty, failed) plus the cached
  list that survives a failed re-request.
- **Sheet-name value**: the text the person typed; the validation input is its trimmed form, which must
  be non-empty for the primary action to enable.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All five catalog render situations — loading, list, empty, error-with-Retry and
  defensive-parse error — are each proven by an interface test; 0 of them rely on visual inspection.
- **SC-002**: 0 mutating requests are issued by the catalog or the creation surface (opening, typing,
  activating the primary action, dismissing).
- **SC-003**: 0 user-facing literals exist in the delivered components; 100% of the copy resolves
  through the translation catalog.
- **SC-004**: A key removed from a pinned catalog capture turns the catalog suite red on the first run
  (0 false negatives).
- **SC-005**: 0 critical automated accessibility violations on the catalog and the creation surface.
- **SC-006**: Opening the same template twice issues exactly two detail requests (one per opening) and
  0 additional catalog requests.
- **SC-007**: A malformed entry produces the error state with Retry; 0 blank screens and 0 partial
  lists.

## Assumptions

- The catalog and the template detail share one payload shape, and the service's catalog response
  already carries the field tree. The catalog list therefore renders names without a second request,
  but the creation surface still issues its own detail request on open (per the program plan's F3 and
  the desktop's on-demand detail), so the surface reflects the template as opened rather than the list's
  copy.
- The creation surface is an overlay over the catalog, not a deep-linkable route: the desktop kept V1
  navigation shallow, and the program plan's F3 names a dialog component. Deep-linking the surface is
  out of scope.
- Rendering the template's field tree (schema-driven controls, grouping, exclusive options) belongs to
  the UI-engine feature (F4); this feature renders the template's display name only and builds no
  field control.
- Creating a sheet, opening the created sheet on the sheet screen, the "My sheets" library, and drafts
  belong to the sheet-cycle feature (F5). This feature delivers the surface and its name validation with
  no mutation; the primary action becomes functional there.
- The catalog refresh shortcut (F5) and the central shortcut registry belong to the sheet-cycle/shortcut
  work (F5-T005). This feature loads the catalog once per app load and caches it in memory, and issues
  no refresh on its own.
- Authentication is a later feature; this feature consumes the state-free Web API Client seam as built,
  attaches no credential and persists nothing.
- Only the two operations the catalog consumes are in scope: the template catalog and the template
  detail. Health, sheet creation/retrieval/update, listing the caller's sheets, deletion and preview
  remain in their own features.
- The service's real catalog and detail responses are already pinned as contract captures by the Web API
  Client feature, so this feature reuses them and adds no new capture unless the catalog needs one.
- Continuous integration already runs the client suite and the requirement-ID lint covers TypeScript
  sources, so this feature introduces no new gate.
- All identifiers, comments and user-facing strings remain English-only, per the project constitution.
