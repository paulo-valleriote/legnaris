# Feature Specification: Web UI Primitives

**Feature Branch**: `009-web-ui-primitives`

**Created**: 2026-10-17

**Status**: Draft

**Input**: User description: "Establish the shared web UI primitives layer (button, field, checkbox, dialog, toast) with token-only styles, rendering tests and the extended literal/copy scans"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One action primitive, and one sanctioned home for it (Priority: P1)

Every screen that offers an action — create a sheet, save a draft, retry a failed load, confirm a
destructive change — draws the same button, with the same variants, the same two sizes and the same
states, so an action looks and behaves the same wherever it appears and nobody re-invents its
styling. The primitives live in one shared layer with a declared name and a declared class
convention, so "where does a shared component go?" has one answer instead of a per-feature debate.

**Why this priority**: buttons are the widest surface in the product and the most duplicated today;
the layer itself is what every other primitive (and every later screen) is built on, so it is the
minimum viable slice of this feature.

**Independent Test**: render the action primitive in every variant, size and state and assert the
documented appearance contract (variant and size classes, accessible name, disabled/busy
behaviour); assert the shared layer exists with its declared modules, that it imports nothing from a
feature directory, and that the developer-rules document sanctions it while keeping the
"three real usages" rule for every other extraction.

**Acceptance Scenarios**:

1. **Given** the shared layer, **When** it is inspected, **Then** it holds the five primitives — the
   action, the field, the checkbox, the dialog and the toast — each in its own module, and it
   imports no feature directory. (UIP-01, UIP-02)
2. **Given** the developer-rules document, **When** it is read, **Then** it sanctions the shared
   layer as the one place a base component may live and keeps the "three real usages" rule for every
   other extraction; the design document names the layer, its primitive set and its class
   convention. (UIP-03)
3. **Given** an action, **When** it renders in each declared variant, **Then** the primary variant
   fills with the accent and uses the on-accent text, the secondary, ghost and outline variants read
   as progressively lighter surfaces, and the dangerous variant is outlined with the danger border
   and text rather than filled. (UIP-07)
4. **Given** an action, **When** it renders in the small and the medium size, **Then** each size uses
   a declared radius step and neither falls below the declared control height. (UIP-08)
5. **Given** an action, **When** it is hovered, pressed, focused, disabled or loading, **Then** each
   state is reachable and distinct: hover and press change only background or tint, focus shows the
   tokenised ring, disabled is announced as disabled and cannot run its action, and loading is
   announced as busy, cannot run its action, and shows the caller's in-flight label. (UIP-09)
6. **Given** an action inside a form, **When** it is activated, **Then** it never submits the form
   implicitly — its default type is the plain button — and its accessible name comes from its
   content or an explicit name. (UIP-10)

---

### User Story 2 - Inputs that are labelled and errors that are announced (Priority: P1)

A person filling in a name, a numeric stat or a password sees a permanent label above the control,
not a placeholder that disappears as they type; when the value is refused, the message appears next
to the control and is announced to a screen reader, and the control itself is marked invalid;
a disabled control is visibly and programmatically disabled rather than silently inert. The same
field contract covers the text input, the select and the multi-line text area, and a checkbox's
label stays next to its box.

**Why this priority**: the field and checkbox primitives are what the catalog dialog, the auth
surface and the schema-driven form all depend on; the label/error contract is the accessibility
guarantee the design document makes for every form in the product.

**Independent Test**: render each field control with and without an error, disabled, invalid and
numeric, and assert the label association, the error wiring, the disabled semantics and the numeric
presentation; render the checkbox checked, unchecked, disabled and invalid and assert its label
association and boolean reporting.

**Acceptance Scenarios**:

1. **Given** a form field, **When** it renders, **Then** a permanent label is associated with its
   control and a placeholder is never the only label. (UIP-11)
2. **Given** a text input, a select and a text area, **When** each renders inside the field, **Then**
   they share the declared height, border, radius and focus values. (UIP-12)
3. **Given** a field with an error, **When** it renders, **Then** the message renders inline and the
   control is marked invalid and associated with that message; **given** no error, **Then** no error
   association is emitted. (UIP-13)
4. **Given** a disabled field, **When** it renders, **Then** it cannot be edited and is announced as
   disabled. (UIP-14)
5. **Given** a numeric field, **When** it renders, **Then** its value is right-aligned with tabular
   figures. (UIP-15)
6. **Given** a checkbox, **When** it renders in each state, **Then** its label is permanently visible
   and associated with the box and its checked, disabled and invalid states are exposed. (UIP-16)
7. **Given** a checkbox whose stored value is not a boolean (for example a numeric proxy), **When**
   it is composed, **Then** the primitive itself still takes and reports a boolean and the
   conversion stays with the caller. (UIP-17)

---

### User Story 3 - One dialog pattern, with the keyboard inside it (Priority: P1)

Any overlay the product opens — a creation flow, a destructive confirmation, a blocking error — is
the same surface: a scrim behind a raised panel, a title that names it, the focus moved inside it
when it opens, Tab and Shift+Tab cycling within it, Escape closing it, and the focus handed back to
the control that opened it. Activating the scrim closes it; clicking inside it does not.

**Why this priority**: three surfaces already hand-roll this pattern, and every one of them owns its
own Escape handling and focus behaviour; a single trapped, restorable dialog is the accessibility
guarantee the design document names for overlays and the largest duplication this feature removes.

**Independent Test**: open the dialog and assert its role, modal semantics, accessible name and
description; walk Tab and Shift+Tab from the ends and assert the focus never leaves it; press Escape,
activate the scrim and activate a control inside it and assert which of them closes; assert the
focus returns to the invoking control. Drive the trap's own module directly with zero, one and
several controls.

**Acceptance Scenarios**:

1. **Given** a dialog, **When** it opens, **Then** it renders a scrim over the overlay colour behind
   a raised panel in the modal elevation and the large radius, and it exposes the dialog role with
   modal semantics. (UIP-18)
2. **Given** a dialog, **When** it opens, **Then** its accessible name comes from its title and its
   description from the caller's text. (UIP-19)
3. **Given an open dialog**, **When** the person presses Tab at the last control, **Then** focus
   moves to the first; **when** they press Shift+Tab at the first, **Then** focus moves to the last;
   **when** they press Escape, **Then** it closes and focus returns to the control that opened it.
   (UIP-20)
4. **Given an open dialog**, **When** the person activates the scrim, **Then** it closes; **when**
   the activation happens inside the panel, **Then** it stays open. (UIP-21)
5. **Given an open dialog**, **When** it closes, **Then** the caller is told exactly once. (UIP-21)
6. **Given** a container with zero, one or several focusable controls, **When** the trap moves focus
   forward and backward, **Then** it always lands on a control inside the container and never
   escapes it. (UIP-22)

---

### User Story 4 - A notice that does not interrupt (Priority: P2)

When something happens that the person does not have to act on — a background save finished, a
connection hiccup resolved — a small notice appears in a corner of the current screen, announced
politely to a screen reader, without stealing focus and without covering the work. It stays until the
caller takes it away or the person dismisses it, and it never carries its meaning in its colour
alone.

**Why this priority**: no surface in the product emits a toast today, so the primitive is the
lowest-risk slice of this feature; it exists so the screens that need non-blocking feedback (F3 and
later) inherit one pattern instead of inventing one.

**Independent Test**: render a notice in each declared system state and assert the polite live
region, the dismiss control's accessible name, the message rendered as text, the absence of any
modal semantics, that focus is untouched, and that it stays rendered until it is removed.

**Acceptance Scenarios**:

1. **Given** a notice, **When** it renders, **Then** it is a polite live region, it takes no focus
   and it blocks nothing — no scrim and no modal semantics. (UIP-23)
2. **Given** a notice, **When** it renders, **Then** its dismiss control has an accessible name
   resolved through the copy catalog and its message renders as text. (UIP-24)
3. **Given** the four declared system states, **When** a notice renders in each, **Then** the state
   is presented as a variant and no meaning is carried by colour alone. (UIP-25)
4. **Given a notice left alone**, **When** time passes, **Then** it is still rendered: it disappears
   only when the caller removes it or the person dismisses it. (UIP-26)

---

### Edge Cases

- **A literal colour or pixel value in the shared layer**: the literal scan fails, and a token the
  layer references but nobody declared fails the resolution check. (UIP-04)
- **A primitive authors a user-facing string**: the copy scan fails — a text node in JSX or a key the
  catalog does not declare is a violation. (UIP-05)
- **A primitive reaches into a feature directory**: the layer's import rule fails, so a primitive
  cannot depend on a screen. (UIP-02)
- **An action is disabled or loading while it is activated**: its action does not run, and the state
  is announced rather than only styled. (UIP-09)
- **A destructive action is the first control in a dialog**: it is never the control that carries
  focus on open, so Enter cannot destroy anything by default. (UIP-19, UIP-20)
- **A field has no error**: no invalid marking and no dangling error association is emitted.
  (UIP-13)
- **A container has no focusable control**: the dialog remains a valid, dismissible surface instead
  of trapping focus on nothing. (UIP-22)
- **Escape is pressed while a nested surface is open**: the topmost dismissible surface closes, and
  a single Escape never closes two surfaces. (UIP-20)
- **The activation inside the panel bubbles to the scrim's handler**: the dialog stays open, because
  only an activation whose target is the scrim itself dismisses. (UIP-21)
- **A checkbox receives a value that is not a boolean**: the primitive does not coerce it; the
  caller adapts a numeric proxy. (UIP-17)
- **A notice's message is empty or only whitespace**: the primitive renders nothing the live region
  would announce as a phantom notice; the caller supplies the copy. (UIP-23, UIP-24)

## Requirements *(mandatory)*

### Functional Requirements

**Shared layer and governance**

- **UIP-01**: The client MUST have one shared base component layer at `web/src/ui/` holding exactly
  the five primitives of this feature — the action, the field, the checkbox, the dialog and the
  toast — each in its own module, with the layer's styles in one token-only stylesheet.
- **UIP-02**: The shared layer MUST NOT import from a feature directory (the network, application,
  authentication, catalog, form-engine, library, installable-app, sheet or shell directories); it
  MAY resolve copy through the copy catalog and MUST take every visual value from the design tokens.
- **UIP-03**: The developer-rules document MUST sanction the shared layer as the one place a base
  component may live, MUST keep the "three real usages" rule for every other extraction, and the
  design document MUST name the layer, its primitive set and the component class convention. The
  "no base component layer" prohibition MUST be replaced, not restated.
- **UIP-04**: The layer MUST contain no literal colour or pixel value, every token it references MUST
  resolve to a declared token, and the literal-value scan MUST cover `web/src/ui/`.
- **UIP-05**: Every user-facing string the layer renders MUST resolve through the copy catalog, no
  primitive MAY author a user-facing literal, and the copy scan MUST cover `web/src/ui/`.
- **UIP-06**: Requirement-ID citations in the layer's stylesheet MUST be gated: the requirement-ID
  lint MUST walk `*.css` as well as the source suffixes it already walks.

**Action primitive**

- **UIP-07**: The action primitive MUST offer the five declared variants — primary (accent fill with
  the on-accent text), secondary, ghost, outline and danger (danger border and text) — and the danger
  variant MUST NOT be filled with its own colour.
- **UIP-08**: The action primitive MUST offer the two declared sizes, each with a declared radius
  step and a height that never falls below the declared control height.
- **UIP-09**: The action primitive MUST expose the declared states — hover, pressed, focus-visible,
  disabled and loading — where hover and press change only background or tint, focus-visible shows
  the tokenised ring, disabled is announced as disabled and cannot run the action, and loading is
  announced as busy, cannot run the action and renders the caller's in-flight label.
- **UIP-10**: The action primitive MUST render a real button whose default type never submits a form
  implicitly, MUST expose its accessible name from its content or an explicit name, and MUST NOT run
  its action while it is disabled or loading.

**Field and checkbox primitives**

- **UIP-11**: The field primitive MUST render a permanent label associated with its control; a
  placeholder MUST NOT be the only label.
- **UIP-12**: The field primitive MUST support a text input, a select and a multi-line text area that
  share the declared height, border, radius and focus values.
- **UIP-13**: The field primitive MUST render an inline error when one is given, MUST mark its
  control invalid and associate the control with that message, and MUST emit no invalid marking or
  error association when no error is given.
- **UIP-14**: The field primitive MUST support a disabled state in which the control cannot be
  edited and is announced as disabled.
- **UIP-15**: The field primitive MUST support a numeric presentation whose value is right-aligned
  and rendered with tabular figures.
- **UIP-16**: The checkbox primitive MUST render a permanently visible label associated with its
  box and MUST expose its checked, disabled and invalid states.
- **UIP-17**: The checkbox primitive MUST take and report a boolean only, and MUST NOT convert a
  schema value itself: adapting a stored value (for example a numeric proxy) remains the caller's
  responsibility.

**Dialog and focus primitives**

- **UIP-18**: The dialog primitive MUST render one overlay pattern — a scrim using the overlay token
  behind a panel with the modal elevation and the large radius — and MUST expose the dialog role with
  modal semantics, without depending on the platform's native modal method.
- **UIP-19**: The dialog primitive MUST take its accessible name from its title and expose the
  caller's description, and focus MUST land inside the dialog when it opens.
- **UIP-20**: While the dialog is open, Tab at the last control MUST move focus to the first and
  Shift+Tab at the first MUST move focus to the last; Escape MUST dismiss the topmost dismissible
  surface only; and on close focus MUST return to the control that opened the dialog.
- **UIP-21**: An activation whose target is the scrim MUST dismiss the dialog, an activation inside
  the panel MUST NOT, and the caller MUST be notified exactly once per close.
- **UIP-22**: The focus confinement MUST be a standalone module with its own tests, and it MUST keep
  focus inside its container for zero, one and many focusable controls, cycling in both directions,
  without ever moving focus outside.

**Notice primitive**

- **UIP-23**: The notice primitive MUST render a polite live region and MUST NOT take focus, MUST NOT
  render a scrim and MUST NOT expose modal semantics.
- **UIP-24**: The notice primitive MUST offer a dismiss control whose accessible name resolves
  through the copy catalog, MUST render its message as text, and MUST render no notice for an empty
  message.
- **UIP-25**: The notice primitive MUST support the four declared system states as a variant, and no
  notice MAY carry its meaning in colour alone.
- **UIP-26**: The notice primitive MUST own no timer: a notice stays rendered until the caller
  removes it or the person dismisses it.

### Key Entities

- **Component primitive**: one shared control of the layer (action, field, checkbox, dialog,
  notice), with its variants, sizes, states, accessibility contract and the tokens it consumes.
- **Shared layer**: the `web/src/ui/` directory, with its import rule, class convention, stylesheet
  and scan coverage; the sanctioned home for a base component.
- **Interactive state**: one condition a primitive can be in (hover, pressed, focus-visible,
  disabled, loading, invalid, checked), with what it changes visually and what it announces.
- **Focus confinement**: the standalone rule set that keeps keyboard focus inside an open dialog and
  hands it back on close.
- **Non-blocking notice**: a transient message rendered in a polite live region with a variant and a
  dismiss control; it owns no dismiss schedule.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the variants, sizes and states the design document specifies for the five
  primitives have at least one rendering assertion, listed in the coverage table.
- **SC-002**: 0 literal colour or pixel values remain in `web/src/ui/`, and 0 tokens it references
  are undeclared.
- **SC-003**: 0 keyboard-only dead ends: every focusable primitive is reached in tab order and shows
  the focus ring, and an open dialog confines focus and returns it on close.
- **SC-004**: 0 user-facing literals in `web/src/ui/`; every string a primitive renders resolves
  through the copy catalog.
- **SC-005**: 0 new runtime and 0 new dev dependencies.
- **SC-006**: 0 dangling requirement citations from the layer's sources and stylesheet: the spec lint
  reports clean with the stylesheet included.
- **SC-007**: 1 sanctioned base layer: the developer-rules document names it and keeps the
  "three real usages" rule for every other extraction, asserted by the document suite.

## Assumptions

- The plan names this phase's primitives exactly — the action, the field, the checkbox, the dialog
  and the notice — so the field family ships the text input, the select and the multi-line text area,
  and the checkbox ships the labelled boolean. The radio and the switch are specified in the design
  document but are built by the screen that first needs them (F3 or later), because no surface uses
  them today.
- No navigation primitive is built here: navigation stays in the shell directory from the previous
  phase and is retargeted by the screen-refactor phase.
- Screens are not migrated in this feature. The layer ships with its own rendering suites and no
  mounted route — the same shape the schema-driven engine took when it landed with no screen — and
  the existing feature styles for actions and overlays remain until the screen-refactor phase
  switches the catalog, the sheet, the auth surface and the state screens onto the primitives.
- The notice primitive owns no dismissal schedule: the design document asks for the notice as
  non-blocking feedback, and time-based behaviour in this client is owned by an injectable caller
  (the draft cadence is the precedent), so an auto-dismiss would have to be the caller's, not the
  primitive's.
- The declared semantic tokens cover every primitive state, so the feature is expected to add no
  token. If a state genuinely has no semantic, the token-addition workflow of the previous phase
  applies in the same commit: declare it in both themes, mirror it, and declare its contrast pair
  when it is a foreground.
- The stack is unchanged — React with Vite and TypeScript, token-driven CSS, Vitest with Testing
  Library — and no end-to-end scenario is added, because a layer that mounts no route cannot be
  driven in a browser; the browser-level visual regression belongs to the last phase of the redesign.
- The dialog keeps the overlay pattern rather than the platform's native modal element, matching the
  decision the catalog phase recorded: the platform's modal method is not implemented in the test
  DOM, so the native element could not be driven by the rendering tests the constitution requires.
- The requirement-ID lint already walks TypeScript sources; this feature extends it to stylesheets,
  because the layer's stylesheet is where its requirement citations live.
- Copy owned by the layer is limited to generic chrome — the notice's dismiss action; feature copy
  stays with the screens that render it.
- The developer-rules and design documents are the only places the base-layer rule and the
  "three real usages" rule are written; a restatement of a visual value in the developer-rules
  document remains a defect.
