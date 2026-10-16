# Feature Specification: Web Sheet Cycle

**Feature Branch**: `005-web-sheet-cycle`

**Created**: 2026-10-16

**Status**: Draft

**Input**: User description: "Web sheet cycle: create a sheet from the catalog, open it on a sheet screen, edit fields through a cascade-aware draft, dry-run a preview that marks modified and recalculated values, and save only the changed delta"

## Clarifications

### Session 2026-10-16

- Q: The program plan's F5 task list (Parte A §7 / Parte B §6) names `sheet_open`, `sheet_edit`, the
  IndexedDB cache, the draft backup, the footer/Save and the keyboard shortcuts, but it omits the
  dry-run — while the delivered `web-ui-engine` spec (F4) states the sheet-cycle feature owns the
  dry-run and produces the preview marks, and the plan's Definition of Done lists "preview" under
  desktop parity. Is the dry-run in F5? → A: In scope (user decision, 2026-10-16): F5 extends the
  Web API Client with the preview operation, renders a Preview action beside Save, and publishes the
  `modified`/`recalculated` marks F4 can already render (`WSC-21`…`WSC-25`).
- Q: The plan's F5 section is titled "criar/abrir/editar com cascata" and F3's spec delegates making
  the creation surface's primary action functional to F5 (`WCT-25` is side-effect-free only "in F3"),
  but the F5 task list names only the sheet GET and PATCH. Does F5 wire the real sheet creation? →
  A: In scope (user decision, 2026-10-16): the creation surface issues `POST /sheets` and opens the
  created sheet on the sheet screen (`WSC-01`…`WSC-04`); F7 keeps the sheets list and deletion.
- Q: The spec's Key Entities says `F5` refreshes the sheet and Assumptions claims this feature owns the
  `F5` refresh dispatch, but no `WSC-*` requirement — only `WSC-20`'s `Ctrl+S` — defines it. What
  should `F5` do? → A: Port the desktop refresh flow (user decision, 2026-10-16): while a sheet screen
  is mounted `F5` re-fetches that sheet's template detail; a clean sheet refetches directly and a sheet
  with pending edits confirms a discard first (confirm discards the in-memory edits and deletes the
  draft, cancel issues no request); an in-flight preview or save, or no mounted sheet screen, makes it
  a no-op (`WSC-36`). This ports desktop `GRN-17`…`GRN-19`.
- Q: `WSC-29` deletes the draft when "a discard is confirmed" but no requirement defines where that
  confirmation comes from. Does abandoning the sheet screen with pending edits confirm first? → A:
  Yes (user decision, 2026-10-16): leaving the sheet screen with pending edits shows a confirmation;
  confirm discards the in-memory edits and deletes the draft, cancel stays on the sheet; a clean sheet
  leaves without a dialog, and an in-flight preview or save makes leaving a no-op (`WSC-37`). This
  ports desktop `GRN-13`…`GRN-16`.
- Q: The desktop throttles Save and Preview to three requests per rolling 60 seconds (`SVE-02`), but
  the plan's F5 task list omits it. Does this feature port the client-side throttle? → A: Yes (user
  decision, 2026-10-16): Save and Preview each keep a rolling 60-second window and issue no request
  once the window holds three, re-enabling when the window frees (`WSC-38`); this is distinct from the
  server-side rate limiting a later backend feature owns.
- Q: F4 deferred the mounted-surface accessibility sweep to "the feature that mounts the engine", and
  F1 (`SC-006`) and F3 (`SC-005`) each carry a zero-critical-a11y criterion; this feature had none.
  Does this feature add one? → A: Yes (user decision, 2026-10-16): the mounted sheet screen and the
  creation surface MUST report zero critical automated accessibility violations and every control MUST
  expose an accessible role and name (`WSC-39`, `SC-010`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create a sheet and open it (Priority: P1)

A person names a sheet in the creation surface and the sheet is created and opened. From the catalog
they open a template's creation surface, type a name, and activate the primary action; the client
creates the sheet and moves straight to the sheet screen showing that sheet's name and its fields.
The same screen is reachable directly for an existing sheet — by its address or a link — where it
loads the sheet once and renders the template's field tree. While a request is in flight the screen
shows a loading state; a failure shows the mapped reason with a way to retry; and a sheet the server
no longer has shows the mapped missing message with no Retry, having been dropped from the local
copy so it cannot come back from cache.

**Why this priority**: it is the doorway to the whole feature — nothing can be edited, previewed or
saved until a sheet is reached, and the plan's phase is "criar/abrir/editar".

**Independent Test**: drive the creation surface through a successful creation and a failed one and
confirm one request each and the transition onto the sheet screen; then load an existing sheet
through a slow response, a failure, a missing sheet and a decodable-but-wrong response and confirm
the four distinct screen states and the local eviction on the missing case.

**Acceptance Scenarios**:

1. **Given** the creation surface holds a non-empty trimmed name, **When** the primary action is
   activated, **Then** the client issues exactly one sheet-creation request for the chosen template
   and that name. (WSC-01)
2. **Given** the creation request is in flight, **When** the primary action is activated again,
   **Then** no second request is issued and the surface shows progress. (WSC-03)
3. **Given** the creation request succeeds, **When** the response settles, **Then** the client opens
   the created sheet on the sheet screen for the returned identifier and shows the sheet's name
   without fetching that sheet again. (WSC-02)
4. **Given** the creation request fails, **When** the response settles, **Then** the surface stays
   open, shows the mapped failure message inline, and keeps the typed name. (WSC-04)
5. **Given** a sheet is opened by identifier, **When** it renders, **Then** it requests the sheet
   exactly once and shows a loading state until the response settles. (WSC-05)
6. **Given** the requested sheet does not exist, **When** it renders, **Then** it shows the mapped
   missing-sheet message with no Retry and drops the sheet from the local cache. (WSC-06)
7. **Given** the sheet request fails for any other reason, **When** it renders, **Then** it shows the
   mapped message with a Retry that re-requests exactly once. (WSC-07)
8. **Given** the sheet is loaded, **When** the sheet screen renders, **Then** it shows the sheet's
   name and renders its template's field tree through the shared UI engine. (WSC-08)
9. **Given** a sheet's address, **When** it is navigated to directly, **Then** the same sheet screen
   renders for that identifier. (WSC-10)
10. **Given** a sheet was loaded or created, **When** the local store is inspected, **Then** the sheet
    is cached locally under a schema version. (WSC-11)

---

### User Story 2 - Edit fields and save only the changed delta (Priority: P1)

A person changes one or more fields on the open sheet. The client remembers exactly which fields were
changed; a field put back to the value it started with is no longer pending. A footer reports the
pending state and offers Save, which sends **only** the changed, editable fields — never a value the
person did not change, never a value the template computes — and merges the derived values the server
returns, advancing the point it compares against so those edits stop being pending. A failed save
keeps every edit and shows the mapped reason; it is never retried automatically. Activating Save again
while the first is in flight does nothing, and a clean sheet issues no request at all. `Ctrl+S` runs
the same save as the footer action.

**Why this priority**: editing is the point of opening a sheet, and the delta discipline is what makes
a save safe — an over-broad payload silently overwrites fields the person never touched.

**Independent Test**: load a sheet, change an editable field, revert it, change two fields (one an
exclusive-group member), and assert the pending state and the exact nested payload sent; then drive a
successful save and assert the derived values merge and the baseline advances, and a failed save and
assert the edits and the mapped message both survive.

**Acceptance Scenarios**:

1. **Given** an editable field, **When** its value changes, **Then** the client records that field as
   pending. (WSC-12)
2. **Given** an editable field was changed back to its seeded value, **When** the pending state is
   read, **Then** that field is no longer pending. (WSC-13)
3. **Given** pending edits, **When** Save runs, **Then** the payload carries only the changed editable
   fields, nested by their dotted paths, and carries no unchanged field and no derived value.
   (WSC-14)
4. **Given** an exclusive group, **When** a member is chosen, **Then** that member is set on and every
   sibling set off in one change; **When** "None" is chosen, **Then** every member is set off. (WSC-15)
5. **Given** the save request succeeds with derived values, **When** it settles, **Then** the returned
   values are merged into the sheet, the comparison baseline advances to what the server now stores,
   and the edits stop being pending. (WSC-16)
6. **Given** the save request fails, **When** it settles, **Then** every edit is kept, the mapped
   failure message is shown inline, and no retry is issued automatically. (WSC-17)
7. **Given** a save is in flight, **When** Save is activated again, **Then** no second request is
   issued. (WSC-18)
8. **Given** the sheet has no pending edit, **When** Save is activated, **Then** no request is issued.
   (WSC-19)
9. **Given** the sheet screen is open, **When** `Ctrl+S` is pressed, **Then** the same save runs as the
   footer action. (WSC-20)

---

### User Story 3 - Dry-run a preview that marks what changed (Priority: P2)

Before saving, a person dry-runs the pending edits. The client sends the same delta to a stateless
preview, merges the derived values the server computes **without** treating them as saved, and marks
the rows: the fields the person changed are labeled "Modified" and the derived fields the preview
recalculated are labeled "Recalculated", with the controls tinted. A new edit clears the marks and
makes the preview available again; a preview failure keeps the edits and shows the mapped reason.
Because the marks come from F4's rendering seam and the dry-run returns the same cascade a save
would, the preview can never disagree with the save.

**Why this priority**: the preview is what makes a large schema trustworthy — the person sees which
values their change would recompute before committing — but the feature is usable without it, so it
sits below create/open/save.

**Independent Test**: with pending edits, run the preview and assert one request carrying the delta,
the derived values merged without the baseline advancing, and the marks on the changed and
recalculated rows; then edit again and assert the marks clear, and drive a failing preview and assert
the edits and message survive.

**Acceptance Scenarios**:

1. **Given** pending edits, **When** Preview runs, **Then** the client issues exactly one preview
   request carrying the pending delta. (WSC-21)
2. **Given** the preview response carries a cascade, **When** it settles, **Then** the cascade is
   merged into the displayed values without advancing the save baseline — every edit is still
   pending. (WSC-22)
3. **Given** a successful preview, **When** the sheet renders, **Then** each changed editable row is
   labeled "Modified" and tinted, and each recalculated derived row is labeled "Recalculated" and
   tinted, including a marked row whose value is empty. (WSC-23)
4. **Given** a successful preview, **When** an editable field changes again, **Then** the marks clear
   and Preview is available again for the new edits. (WSC-24)
5. **Given** the preview request fails, **When** it settles, **Then** every edit is kept, the mapped
   failure message is shown inline, and Preview stays available. (WSC-25)

---

### User Story 4 - Guard unsaved edits and throttle requests (Priority: P2)

A person with unsaved edits presses `F5` or leaves the sheet. The client never discards edits
silently: `F5` re-fetches the sheet's template behind a confirmation, and leaving the screen asks
first; confirming discards the edits and drops the draft, and cancelling does nothing. While a preview
or save is in flight those actions are inert, so a response can never land on a screen that moved out
from under it. Save and Preview are each throttled to three requests in a rolling minute; a throttled
action is a no-op that re-enables when the window frees, so the client never leans on a server limit a
later backend feature owns.

**Why this priority**: it protects the person's uncommitted work and the server, but the feature is
fully usable without the guards, so it sits below create/open/save.

**Independent Test**: with pending edits press `F5` and assert the confirmation gates the template
re-fetch and that cancel issues zero requests, then confirm and assert the edits are discarded and the
draft deleted; leave with pending edits and assert the same; press `F5` on a clean sheet and assert one
template request; then drive four rapid saves and assert at most three requests inside the window and
the action re-enabling once it frees.

**Acceptance Scenarios**:

1. **Given** a sheet screen with pending edits, **When** `F5` is pressed, **Then** the client asks
   before discarding and, on confirm, discards the edits, deletes the draft and re-fetches the
   template detail once; on cancel it issues no request. (WSC-36)
2. **Given** a clean sheet screen, **When** `F5` is pressed, **Then** the template detail is
   re-fetched exactly once with no confirmation. (WSC-36)
3. **Given** a preview or save is in flight, **When** `F5` is pressed, **Then** no discard and no
   request occur. (WSC-36)
4. **Given** a sheet screen with pending edits, **When** the person leaves the screen, **Then** the
   client asks before discarding and, on confirm, discards the edits and deletes the draft; on cancel
   it stays. (WSC-37)
5. **Given** three Save (or Preview) requests are already in the rolling minute, **When** the action
   is activated again, **Then** no request is issued and the action re-enables when the window frees.
   (WSC-38)

---

### User Story 5 - Unsaved drafts survive a crash and are restored (Priority: P3)

A person edits a sheet and, without saving, the tab crashes or is closed. While edits are pending the
client periodically backs up the pending delta to local storage; reopening the same sheet restores
that draft, showing the backed-up values as pending again. Backing up only ever stores the editable
delta a person actually changed — never derived values, never credentials — and a draft is discarded
the moment it is no longer meaningful: after a successful save, after a confirmed discard, or when the
sheet turns out not to exist.

**Why this priority**: it protects work that has not been committed, but it depends on the edit state
(US2) and is invisible until a crash occurs, so it comes after the guards.

**Independent Test**: make the cadence controllable, edit a sheet, advance the cadence and assert one
backup holding only the editable delta; reopen the sheet and assert the draft is restored over the
stored values and the restored fields are pending; then save and assert the backup is gone, and write
a corrupt backup and assert the sheet seeds cleanly instead of throwing.

**Acceptance Scenarios**:

1. **Given** the sheet has pending edits, **When** the backup cadence elapses, **Then** the client
   writes the pending editable delta to local storage and stores no derived value. (WSC-26)
2. **Given** the sheet is clean, **When** the cadence elapses, **Then** no backup is written. (WSC-27)
3. **Given** a stored draft for a sheet, **When** that sheet is reopened, **Then** the draft values are
   overlaid on the stored sheet, only keys that are editable leaves of the template are applied, and
   the restored fields are pending. (WSC-28)
4. **Given** a successful save or a confirmed discard or a missing sheet, **When** the event occurs,
   **Then** the sheet's draft is deleted. (WSC-29)
5. **Given** a stored draft that is corrupt, of an unsupported schema version, or names another sheet,
   **When** the sheet is opened, **Then** the sheet seeds cleanly from the stored values without
   throwing. (WSC-30)
6. **Given** a stored draft, **When** its contents are inspected, **Then** it holds no credential.
   (WSC-31)

---

### Edge Cases

- **Whitespace-only sheet name**: the primary action stays disabled; no creation request is issued.
- **Creation failure**: the surface stays open with the typed name intact and the mapped message
  inline (WSC-04).
- **Repeated creation activation during flight**: no second request (WSC-03).
- **Missing sheet**: the mapped missing-sheet message with no Retry, and local eviction including its
  draft; a failing local eviction never hides the message (WSC-06, WSC-29).
- **Malformed sheet or template payload**: the defensive parse failure state, never a blank or
  half-rendered screen (WSC-07).
- **Template with no fields**: the sheet screen shows the empty-fields line rather than an empty form.
- **Reverted edit**: a field back at its seeded value is not pending and does not enter the delta
  (WSC-13, WSC-14).
- **Derived value edited attempt**: a non-editable field never enters the pending set or the payload
  (WSC-14).
- **Unchanged sibling of a changed nested group**: not included in the payload (WSC-14).
- **High-precision number**: a value the person typed as a decimal is sent unchanged, without
  coercion or rounding.
- **Empty delta**: Save and Preview each issue no request (WSC-19, WSC-21).
- **Concurrent activation**: a second Save or Preview while one is in flight is joined, not
  duplicated (WSC-18, WSC-21).
- **Save failure then retry**: the edits remain pending so a later Save resends them (WSC-17).
- **Preview then edit**: the stale marks clear and Preview re-enables (WSC-24).
- **Marked row with an empty value**: the mark still renders (WSC-23).
- **Draft with a stale key**: a key that is not an editable leaf of the loaded template is ignored
  (WSC-28).
- **Corrupt / unsupported / foreign draft**: seeds cleanly, never throws (WSC-30).
- **Markup in a value** (`characterName = "<script>"`): rendered as text (WSC-35).
- **Deep link to a sheet route**: resolves the same screen as a navigation (WSC-10).
- **Credential in local storage**: forbidden; the local stores hold drafts and cached sheets only
  (WSC-31).
- **Refresh with pending edits**: `F5` confirms before discarding; a cancel issues zero requests
  (WSC-36).
- **Refresh or exit during a request**: an in-flight preview or save makes `F5` and leaving the screen
  inert (WSC-36, WSC-37).
- **Throttled action**: a fourth Save or Preview inside the rolling minute issues no request and
  re-enables when the window frees (WSC-38).
- **Mounted surface accessibility**: the mounted sheet screen and the creation surface report zero
  critical automated accessibility violations (WSC-39).
- **Dimension sweep** (explicitly covered / not applicable here):
  - *Input validation and bounds* — the trim/required name (WSC-01) and defensive payload parsing
    (WSC-07).
  - *Failure and partial failure* — creation failure (WSC-04), open failure (WSC-07), missing sheet
    (WSC-06), save failure (WSC-17), preview failure (WSC-25), malformed payload (WSC-07).
  - *Idempotency and retry* — one request per creation, open and Retry (WSC-01, WSC-05, WSC-07), one
    save per activation (WSC-18), no request on a clean sheet (WSC-19), one preview per activation and
    none without new edits (WSC-21, WSC-24), a throttled action issues none (WSC-38).
  - *Concurrency and ordering* — a second Save/Preview while one is in flight is joined (WSC-18,
    WSC-21); the cascade merge returns a fresh value and never mutates its inputs (WSC-16, WSC-22,
    WSC-34).
  - *State-transition integrity* — a reverted edit un-pends (WSC-13); the baseline advances only on a
    successful save (WSC-16); a preview never advances it (WSC-22); a discard/eviction clears the draft
    (WSC-29, WSC-36, WSC-37).
  - *Not applicable*: authentication (this feature attaches no credential; session handling is a later
    feature), the **server-side** rate limit (a later backend feature — the client-side throttle of
    WSC-38 is in scope), and the offline write queue (Background Sync is out of scope per the program
    plan; a later feature owns offline reading).

## Requirements *(mandatory)*

### Functional Requirements

**Creating a sheet from the catalog**

- **WSC-01**: WHEN the creation surface's primary action is activated with a non-empty trimmed name,
  the client MUST issue exactly one sheet-creation request carrying that name and the surface's
  template identifier.
- **WSC-02**: WHEN the creation request succeeds, the client MUST open the created sheet on the sheet
  screen for the returned identifier and MUST NOT fetch that sheet again.
- **WSC-03**: WHILE the creation request is in flight, the primary action MUST be disabled, the
  surface MUST show progress, and a further activation MUST NOT start a second request.
- **WSC-04**: IF the creation request fails, THEN the surface MUST stay open, MUST show the mapped
  failure message inline, and MUST keep the typed name.

**Opening a sheet**

- **WSC-05**: WHEN a sheet is opened by identifier, the client MUST request it exactly once and MUST
  show a loading state until the response settles.
- **WSC-06**: IF the open resolves that the sheet does not exist, THEN the client MUST show the mapped
  missing-sheet message with no Retry and MUST drop the sheet (and its draft) from local storage,
  where a failing local eviction MUST NOT hide the message.
- **WSC-07**: IF the open fails for any other reason, THEN the client MUST show the mapped message
  with a Retry action that re-requests exactly once; a response that cannot be decoded MUST take this
  same error path rather than rendering a blank or partial screen.
- **WSC-08**: WHEN the sheet is loaded, the client MUST show the sheet's name and render its
  template's field tree through the shared UI engine.
- **WSC-09**: WHILE the sheet's template detail is loading, the client MUST show a loading state; IF
  it fails, THEN it MUST show the mapped message with a Retry.
- **WSC-10**: The sheet screen MUST be reachable by direct navigation of its own route, resolving the
  same screen a navigation would produce.
- **WSC-11**: WHEN a sheet is loaded or created, the client MUST cache it locally by identifier under
  a stored schema version, so a later offline read can render it.

**Editing and saving the delta**

- **WSC-12**: WHEN an editable field's value changes, the client MUST record that field's dotted path
  as pending.
- **WSC-13**: A field whose current value equals the value it was seeded with MUST NOT be pending.
- **WSC-14**: WHEN Save runs, the payload MUST carry only the changed editable fields, nested by their
  dotted paths, and MUST NOT carry an unchanged field, a non-editable (derived) field, or a value that
  the person did not change.
- **WSC-15**: WHEN a member of an exclusive group is chosen, the client MUST set that member on and
  every sibling off in one change; WHEN "None" is chosen, it MUST set every member off.
- **WSC-16**: WHEN the save response carries derived values, the client MUST merge them into the sheet,
  MUST advance the save baseline to what the server now stores (the seeded values overlaid with the
  sent delta and the returned values), and the sent edits MUST stop being pending.
- **WSC-17**: IF the save request fails, THEN the client MUST keep every edit, MUST show the mapped
  failure message inline, and MUST NOT retry automatically.
- **WSC-18**: WHILE a save is in flight, a further Save activation (including `Ctrl+S`) MUST NOT start
  a second request.
- **WSC-19**: WHEN the sheet has no pending edit, Save MUST issue no request.
- **WSC-20**: WHEN `Ctrl+S` is pressed while the sheet screen is open, the client MUST run the same
  save as the footer action and MUST be a no-op when no sheet screen is mounted.

**Dry-run preview**

- **WSC-21**: WHEN Preview runs with pending edits, the client MUST issue exactly one preview request
  carrying the pending delta; with no pending edit, or while a request is in flight, it MUST issue
  none.
- **WSC-22**: WHEN the preview response carries a cascade, the client MUST merge it into the displayed
  values and MUST NOT advance the save baseline.
- **WSC-23**: WHEN a preview succeeded, each changed editable row MUST be labeled "Modified" and
  tinted and each recalculated derived row MUST be labeled "Recalculated" and tinted, including when
  the marked value is empty; the marks MUST come from the shared engine's mark seam and MUST NOT be
  authored as a per-screen literal.
- **WSC-24**: WHEN an editable field changes after a successful preview, the marks MUST clear and
  Preview MUST become available again.
- **WSC-25**: IF the preview request fails, THEN the client MUST keep every edit, MUST show the mapped
  failure message inline, and Preview MUST remain available.

**Unsaved drafts**

- **WSC-26**: WHILE the sheet has pending edits, the client MUST periodically back up the pending
  editable delta to local storage on an injectable cadence, and MUST NOT store a derived value.
- **WSC-27**: WHILE the sheet is clean, the client MUST NOT write a draft.
- **WSC-28**: WHEN a sheet with a stored draft is opened, the client MUST overlay the draft on the
  stored values, MUST apply only keys that are editable leaves of the loaded template, and MUST mark
  the restored fields pending.
- **WSC-29**: WHEN a save succeeds, a discard is confirmed, or a sheet is found missing, the client
  MUST delete that sheet's draft.
- **WSC-30**: A draft that is corrupt, carries an unsupported schema version, or names a sheet other
  than the one opened MUST be ignored so the sheet seeds cleanly, and MUST NOT raise.
- **WSC-31**: Local storage managed by this feature MUST hold no credential; the cached sheet and the
  draft hold sheet values only.

**Guarding unsaved edits and throttling requests**

- **WSC-36**: WHILE a sheet screen is mounted, WHEN `F5` is pressed with a clean sheet, the client MUST
  re-fetch that sheet's template detail exactly once; WHEN edits are pending it MUST confirm before
  discarding and, on confirm, MUST discard the in-memory edits, delete the draft and re-fetch the
  template detail once, and on cancel MUST issue no request; WHILE a preview or save is in flight, or
  with no sheet screen mounted, it MUST be a no-op.
- **WSC-37**: WHEN the person leaves the sheet screen with pending edits, the client MUST confirm
  before discarding and, on confirm, MUST discard the in-memory edits and delete the draft; on cancel
  it MUST stay; a clean sheet MUST leave without a dialog, and an in-flight preview or save MUST make
  leaving a no-op.
- **WSC-38**: Save and Preview MUST each keep a rolling 60-second window and MUST issue no request once
  that window holds three requests, re-enabling when the window frees; the throttle MUST NOT replace
  the in-flight no-op rules (WSC-18, WSC-21).

**Cross-cutting**

- **WSC-32**: Every user-visible string the feature renders MUST resolve through the shared
  translation catalog; no user-facing literal may appear in a component.
- **WSC-33**: Every visual value the feature renders MUST come from the shared token set; no ad-hoc
  visual value may be introduced.
- **WSC-34**: The feature MUST reuse the shared API client and the shared UI-engine draft/merge
  derivation rather than re-implementing the cascade merge or the seed rules.
- **WSC-35**: A value containing markup MUST render as literal text; the feature MUST NOT introduce a
  DOM-injection sink.
- **WSC-39**: Every control the mounted sheet screen and the creation surface render MUST expose an
  accessible role and name, and both mounted surfaces MUST report no critical automated accessibility
  violation.

### Key Entities

- **Sheet edit state**: the values of one open sheet addressed by dotted path, the reference values
  they were seeded from, the set of pending paths, whether a request is in flight, the mapped message
  of the last failure, and the current preview records. Held in memory while the sheet screen is
  mounted; discarded when it unmounts.
- **Pending delta**: the nested object rebuilt from the pending paths whose current value differs from
  the reference — the exact body a save and a preview send.
- **Preview marks**: the pending-delta paths the last preview sent (`modified`) and the derived paths
  its cascade recalculated (`recalculated`); absent before a preview and after a save or a new edit.
- **Sheet open state**: the load situation of one sheet by identifier — loading, loaded (with the
  sheet), or a failure carrying either the missing-sheet situation (no Retry, evicted) or a retryable
  failure.
- **Local sheet cache**: one locally stored record per sheet identifier under a schema version,
  holding the sheet's identity and data so a later offline read can render it; never a credential.
- **Draft backup**: the pending editable delta of one sheet, stored under a schema version with the
  sheet identifier; overlaid on the stored sheet on open and deleted on save/discard/eviction.
- **Shortcut registry**: the single action → key map the shell dispatches through — `Ctrl+S` saves the
  mounted sheet, `F5` re-fetches the mounted sheet's template detail behind a discard confirmation.
- **Request throttle**: the per-action rolling 60-second window that counts Save and Preview requests
  and disables an action once it holds three, re-enabling when it frees.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every screen situation — creation in flight, creation failed, sheet loading, sheet
  loaded, missing sheet, retryable failure, template loading, template failed — is proven by an
  interface test; 0 rely on visual inspection.
- **SC-002**: A save and a preview each send a payload containing 0 unchanged fields and 0 derived
  fields; a clean sheet issues 0 requests.
- **SC-003**: A sheet with N pending editable paths produces exactly N leaf entries in the nested
  payload and 0 missing or extra entries.
- **SC-004**: 0 credentials appear in any local storage the feature writes; the refresh credential
  never becomes JavaScript-readable here.
- **SC-005**: A draft whose keys include a derived or non-editable key restores 0 of those keys; only
  editable leaves are applied.
- **SC-006**: 0 user-facing literals exist in the delivered components; 100% of the copy resolves
  through the translation catalog, and removing a referenced key turns the suite red on the first run.
- **SC-007**: A value or label containing `<script>` produces 0 markup nodes; the text appears as
  literal text.
- **SC-008**: A failed save and a failed preview each leave 100% of the pending edits intact and issue
  0 automatic retries.
- **SC-009**: A preview merges its cascade into the display and advances the baseline 0 times; a save
  advances it exactly once.
- **SC-010**: The mounted sheet screen and the creation surface report 0 critical automated
  accessibility violations, and every control exposes an accessible role and name.
- **SC-011**: In any rolling 60-second window, Save and Preview each issue at most 3 requests; a 4th
  activation inside the window issues 0 requests and the action re-enables within the window's
  remaining time.

## Assumptions

- The feature is the web port of the desktop sheet cycle (desktop `Fase 4`, `AD-005`, `AD-006`,
  `AD-008`, `AD-009`). The rules are the contract's, so the port is a re-expression, not a redesign.
- It consumes the delivered Web API Client (F2) and extends it with the preview operation the backend
  already exposes (`POST /sheets/:id/preview`, `AD-007`); it consumes the delivered UI engine (F4) for
  the field rendering, the seeded draft, the cascade merge and the preview-mark rendering, and reuses
  F4's `sheet_defaults` rather than re-deriving anything.
- The backend is not modified by this feature: the sheet endpoints already exist (`GET /sheets/:id`,
  `POST /sheets`, `PATCH /sheets/:id`, `POST /sheets/:id/preview`). The browser-auth cookie feature
  (F6) and the session feature (F7) are later; this feature uses the F2 client as built (no credential
  attached) and its tests inject a fake transport.
- The server is the source of truth for the set of a caller's sheets: listing the caller's sheets and
  deleting one (`GET /sheets`, `DELETE /sheets/:id`) belong to F7, so this feature builds no "My
  sheets" list. The local store is an offline cache and a draft store, not the list of record.
- Making the creation surface's primary action functional supersedes F3's `WCT-25` side-effect-free
  scope, exactly as F3's own Assumptions delegate ("the primary action becomes functional there").
- The keyboard registry (`web/src/shell/shortcut_registry.ts`) is introduced here because F5-T005 owns
  the `Ctrl+S` save and the `F5` refresh dispatch, which F1 documented as a V1 default but did not
  build. `F5` with no sheet screen mounted is a no-op here; refreshing the catalog surface is not part
  of this feature.
- The guard and throttle behavior ports the desktop's local-edit guards and request limiter: `F5`'s
  discard-confirmed template refetch (`GRN-17`…`GRN-19`), the leave-screen discard guard
  (`GRN-13`…`GRN-16`), and the three-per-rolling-60-second Save/Preview window (`SVE-02`). The
  client-side throttle is distinct from the server-side rate limiting a later backend feature owns.
- The mounted sheet screen and the creation surface carry the zero-critical-a11y criterion F4 deferred
  to the feature that mounts the engine (`F1` `SC-006`, `F3` `SC-005`).
- Persisting locally uses the browser's structured storage (IndexedDB) via the plan's chosen library
  (`idb`, Parte 3); it is the feature's one new runtime dependency, recorded as an inherited plan
  decision. A memory-only test double, or a fake IndexedDB, keeps the suites dependency-free at
  runtime.
- The offline **read** path (rendering a cached sheet with the network down) and the service-worker
  precache belong to the PWA feature (F8); this feature only writes the cache and the draft, and
  consumes them on open.
- An offline **write** queue (Background Sync of pending saves) is explicitly out of scope per the
  program plan; a save requires connectivity and a failure keeps the edits.
- Continuous integration already runs the client suite and the requirement-ID lint covers TypeScript
  sources, so this feature introduces no new gate.
- All identifiers, comments and user-facing strings remain English-only, per the project constitution.
