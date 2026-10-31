# Feature Specification: Web UI Engine

**Feature Branch**: `004-web-ui-engine`

**Created**: 2026-10-16

**Status**: Draft

**Input**: User description: "Web UI engine: render any template's field tree as grouped sections with a schema-driven control per leaf (number/text/config toggle/derived read-only), collapse mutually exclusive groups into one control, derive and deep-merge draft values, and mark previewed changes — the core port of `desktop/lib/engine/`."

## Clarifications

### Session 2026-10-16

- Q: The plan's F4 tasks name only engine modules under `web/src/engine/` plus tests, but the F3 clarification said rendering the field tree "belongs to the UI-engine feature (F4)". Should F4 mount the engine on a live surface, or ship the engine with rendering tests only? → A: Engine + rendering tests only (user decision, 2026-10-16): the feature delivers the engine modules and proves each by rendering tests of the components; no screen is wired. The editable sheet screen is the sheet-cycle feature (`web/src/sheet/sheet_screen.tsx`) and the creation surface's read-only preview is wired by whatever consumer needs it, so the engine stays free of screen and edit-state concerns.
- Q: How faithful should the web port's numeric input be to the desktop's whole-value guard? → A: Simpler finite-number guard only (user decision, 2026-10-16): the numeric input resolves the typed text to a finite number; empty input clears the field and a text that does not resolve to a finite number (`NaN`, an infinity, an overflow) leaves the stored value unchanged. The desktop's partial-token behavior (`-`/`.` kept as a value) and restore-on-blur are dropped, and a form the platform resolves to a finite number is accepted. Recorded in `ENG-12`. *Amended 2026-10-30: a sign or point typed so far (`-`, `.`, `-.`) is now kept as display text, without becoming a value, so a negative number can be typed; see `ENG-12`.*

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Read a template's field tree as labeled sections (Priority: P1)

A screen is handed a template's decoded field tree and the current draft values. Without any
knowledge of the specific template, it renders one labeled section per top-level key, in the order the
service returned the keys, with every leaf beneath it shown on its own labeled row. A group that
declares table columns is drawn as a single table row — the group's label plus one cell per declared
column — instead of one stacked row per leaf; a group with no renderable leaf is dropped rather than
drawn empty. The whole composition is exposed twice: once fed by data, and once with the per-field
control handed to the caller, so a later editing screen can subscribe to a single field without
rebuilding the whole form.

**Why this priority**: this is the screen every sheet surface is built on — the promise that any
template renders from its own schema, with no code and no label written for one RPG system. Without
the section derivation and the composition, a `FieldSchema` is only a data transfer object.

**Independent Test**: render a synthetic template whose groups are known and assert the section order,
the row kinds (stacked leaf, table row, exclusive row), the derived labels, and that an injected
per-field control builder is consulted once per leaf and the data-driven control is not rendered.

**Acceptance Scenarios**:

1. **Given** a field tree with top-level keys, **When** it is rendered, **Then** it shows one labeled
   section per key, in the order the keys arrived, and a key whose group holds no leaf is omitted.
   (ENG-01, ENG-02)
2. **Given** a top-level leaf, **When** it is rendered, **Then** it becomes its own single-field
   section. (ENG-02)
3. **Given** a leaf whose schema label is non-empty, **When** it is rendered, **Then** the label is
   the schema label; **Given** a leaf or group whose label is empty, **Then** the label is a
   mechanical humanization of its key, never a per-system literal. (ENG-03)
4. **Given** a group that declares table columns, **When** it is rendered, **Then** it draws one table
   row: the group's label and one cell per declared column, in the declared order, with a shared
   header; an unknown column is skipped and, when no declared column resolves, the group falls back to
   stacked rows. (ENG-04, ENG-06)
5. **Given** the composition is fed by data, **When** it renders, **Then** each row's value is resolved
   by its dotted path and an edit is reported as that path with its new value. (ENG-24)
6. **Given** the composition is given an injected per-field control builder, **When** it renders,
   **Then** the builder owns each row's control and the composition subscribes to no value itself.
   (ENG-25, ENG-26)
7. **Given** both a data source and a builder, or neither, **When** the composition is constructed,
   **Then** it is a construction error rather than a half-rendered form. (ENG-25)

---

### User Story 2 - Each field renders the control its schema dictates (Priority: P1)

For every leaf, the control is chosen from the leaf's own schema: an editable numeric leaf is a
numeric input, an editable textual leaf is a text input, a boolean leaf or a `config.*` proxy leaf is
an on/off toggle, and a derived (non-editable) leaf is a read-only value marked "Calculated" with its
formula available on hover. A leaf of an unrecognized type degrades to the read-only value rather than
inventing an editor or crashing. A set of leaves that the contract tags as mutually exclusive is drawn
as one single-choice control, never as independent toggles for each member. A value that carries markup
— a character name of `<script>` — is rendered as text.

**Why this priority**: this is where "any template renders" becomes real for the person: the right
control per field and one control for one choice. A wrong control silently corrupts a sheet; a
per-system control set would break the promise of radical flexibility.

**Independent Test**: render a synthetic template holding one leaf of each type (number, text, bool,
`config.*` proxy, derived with a formula, unknown type) and one exclusive group, and assert the control
rendered per leaf, its interactivity, the "Calculated" indicator and formula tooltip, the single
control for the group, and that `<script>` in a value appears as literal text.

**Acceptance Scenarios**:

1. **Given** an editable numeric leaf, **When** it is rendered, **Then** it shows a numeric input whose
   value is the stored number and whose edit reports the parsed number. (ENG-07, ENG-12)
2. **Given** an editable textual leaf, **When** it is rendered, **Then** it shows a text input whose
   value is the stored text and whose edit reports the typed text. (ENG-07)
3. **Given** a boolean leaf or a leaf whose first path segment is `config`, **When** it is rendered,
   **Then** it shows an on/off toggle; a boolean leaf reports `true`/`false` and a `config.*` leaf
   reports `1`/`0`. (ENG-08)
4. **Given** a derived (non-editable) leaf, **When** it is rendered, **Then** it shows the formatted
   value read-only, marked "Calculated", and shows its formula on hover when the schema declares one.
   (ENG-09, ENG-10)
5. **Given** a leaf of an unknown type, **When** it is rendered, **Then** it renders the read-only
   value without an editor and without throwing. (ENG-11)
6. **Given** a numeric input holding a number, **When** the user types text that does not resolve to a
   finite number (`NaN`, an infinity, an overflowing exponent), **Then** the value does not change;
   **When** the field is emptied, **Then** the reported value is cleared. (ENG-12)
7. **Given** a leaf with no change handler, **When** it is rendered, **Then** it is inert — the value
   is shown and nothing is editable. (ENG-13)
8. **Given** leaves sharing a non-empty exclusive group, **When** the section renders, **Then** they
   appear as one single-choice control at the first member's position, never as independent toggles,
   and a group with a single member is not collapsed. (ENG-05, ENG-14)
9. **Given** an exclusive member with a non-empty option label, **When** the group renders, **Then**
   that member's option shows the option label, otherwise its display name. (ENG-15)
10. **Given** the group offers a "no choice" entry **only when** no member carries a non-zero default,
    **When** a default-active group has no active member, **Then** the control shows the "no choice"
    hint instead of rendering blank. (ENG-16)
11. **Given** an exclusive control with no selection handler, **When** it renders, **Then** it shows
    the active member (or the "no choice" hint) and nothing is selectable; with a handler, selecting
    an option reports that member's path, and selecting "no choice" reports a clear-group signal.
    (ENG-17, ENG-18)
12. **Given** a value from the server that contains markup, **When** it is rendered as a field value or
    label, **Then** it appears as literal text and is never interpreted as markup. (ENG-29)

---

### User Story 3 - Derive a draft from the template and mark previewed changes (Priority: P2)

Before anything is stored, the engine derives the initial draft: every leaf seeded with its schema
default (text starts empty, a boolean starts off, everything else takes its numeric default), and an
opened sheet's stored values overlaid on those defaults. A cascade from the service is merged
incrementally so only the keys the service sent change. When a dry-run preview has run, the engine
tints the controls the preview touched and labels those rows "Modified" (a change the person made) or
"Recalculated" (a derived value the cascade computed), with the base "Calculated" label yielding to the
mark.

**Why this priority**: the derivation is what both the editable sheet and the read-only creation
preview read from, so it must be shared rather than duplicated; the marks are what make a dry-run
trustworthy by naming which values changed and which were computed. Both depend on the rendering from
US1 and US2.

**Independent Test**: seed a draft from a synthetic template with an empty stored object and assert one
value per leaf in section order; overlay a partial stored object and assert the per-leaf precedence; run
an empty cascade merge and assert the base is returned unchanged and that neither input was mutated;
render a row with each mark and assert the tint and label, including a marked row whose value is empty.

**Acceptance Scenarios**:

1. **Given** a field tree, **When** the empty draft is derived, **Then** every leaf holds its seed
   value under its dotted path: empty text, `false` boolean, else the numeric default. (ENG-19)
2. **Given** a field tree and a stored sheet object, **When** the draft is derived from the sheet,
   **Then** a stored value wins at its path, a schema leaf the sheet omits keeps its default, and a
   stored key absent from the schema is carried over unchanged. (ENG-20)
3. **Given** a base object and a cascade object, **When** they are merged, **Then** keys present in both
   as objects merge recursively and any other key replaces the leaf, returning a fresh object without
   mutating either input; an empty cascade returns the base unchanged. (ENG-21)
4. **Given** a row whose field was changed or recalculated by the last preview, **When** it renders,
   **Then** its control is tinted and its row is labeled "Modified" or "Recalculated", and the base
   "Calculated" label is dropped while the mark is shown — including when the marked value is empty.
   (ENG-10, ENG-22)
5. **Given** the composition is given a per-row mark source, **When** a row renders, **Then** that
   source receives every dotted path the row covers (a single leaf, or all members of an exclusive
   group) and the composition owns no mark state. (ENG-23)

---

### Edge Cases

- **Empty field tree**: rendering a template with no top-level key produces no section and no row, and
  does not throw.
- **Group with no leaves**: a group with no renderable leaf is omitted; a group whose only leaves are
  under an empty sub-group is omitted too (ENG-02).
- **Declared columns that resolve nothing**: a group declaring columns whose named leaves are all
  absent falls back to stacked rows rather than drawing an empty table (ENG-04).
- **Single-member exclusive group**: a group with one member is not collapsed into a single-choice
  control (ENG-05).
- **Non-adjacent exclusive members**: members of one group separated by other leaves still collapse
  into one control at the first member's position (ENG-05).
- **Default-active group with no active member**: the control shows the "no choice" hint instead of
  rendering blank (ENG-16).
- **Empty numeric input**: the reported value is cleared rather than treated as `0` (ENG-12).
- **Non-finite input** (`NaN`, `Infinity`, an overflowing exponent): the value does not change
  (ENG-12).
- **Pending sign or point** (`-`, `.`, `-.`): kept as display text, no value reported until the
  whole text resolves to a finite number; leaving the field restores the last reported value's
  text (ENG-12).
- **Unrecognized field type**: the read-only value fallback, never a fabricated editor and never a
  crash (ENG-11).
- **Null or absent stored value**: the control shows an empty display and an edit from it reports a
  real value (ENG-09, ENG-13).
- **Empty cascade / nested cascade**: an empty override leaves the base unchanged; a nested override
  merges only its leaf (ENG-21).
- **Stored key absent from the schema**: carried over unchanged rather than dropped (ENG-20).
- **Marked row with an empty value**: the tint and the "Modified"/"Recalculated" label still show
  (ENG-22).
- **Server value containing markup** (`characterName = "<script>"`): rendered as literal text, never
  executed and never injected as markup (ENG-29).
- **Dimension sweep** (explicitly covered / not applicable here):
  - *Input validation and bounds* — the numeric guard (ENG-12) and the label/type fallbacks (ENG-03,
    ENG-11).
  - *Failure and partial failure* — a malformed or unrecognized field type degrades to read-only
    (ENG-11); no request is issued by the engine.
  - *Idempotency and retry* — not applicable: the engine issues no request and renders purely from its
    inputs; `mergeCascade` over an empty override is the identity (ENG-21).
  - *Concurrency and ordering* — sections, leaves and options preserve the order the service returned
    them; nesting and non-adjacent grouping preserve the first-member position (ENG-01, ENG-04,
    ENG-05).
  - *State-transition integrity* — the engine is stateless: no control owns domain state, and the
    composition surfaces per-field value and per-row mark through injected seams (ENG-13, ENG-25,
    ENG-26).
  - *Not applicable*: authentication, rate limits, persistence and expiry (the engine holds no
    credential and persists nothing), navigation (no route), observability (logging belongs to a later
    feature).

## Requirements *(mandatory)*

### Functional Requirements

**Deriving sections from the field tree**

- **ENG-01**: WHEN a template's field tree is rendered, the client MUST render one labeled section per
  top-level key, in the order the service returned the keys.
- **ENG-02**: The client MUST flatten every leaf beneath a top-level key into dotted paths in tree
  order; a group with no leaf MUST be omitted, and a top-level leaf MUST become its own single-field
  section.
- **ENG-03**: A field, group or section label MUST be the schema's label when it is non-empty and a
  mechanical humanization of the key otherwise; no per-system literal may decide a label.
- **ENG-04**: WHEN a group declares table columns, the client MUST render it as one table row — the
  group's label and one cell per declared column, in the declared order — skip an unknown or absent
  column, and fall back to stacked rows when no declared column resolves.
- **ENG-05**: A section MUST place every leaf that shares a non-empty exclusive group into one
  exclusive row at the first member's position, regardless of adjacency, and MUST NOT collapse a group
  with a single member.
- **ENG-06**: The client MUST label a table column with the leaf's column label when non-empty and its
  display name otherwise, and MUST align a numeric cell to the right and every other cell to the left.

**Rendering the control per field**

- **ENG-07**: An editable leaf MUST render an input chosen by its type: a numeric input for a number
  and a text input otherwise.
- **ENG-08**: A leaf whose type is boolean, or whose first path segment is `config`, MUST render an
  on/off toggle; the toggle MUST report `true`/`false` for a boolean leaf and `1`/`0` for a
  `config.*` proxy leaf.
- **ENG-09**: A non-editable leaf MUST render its formatted value read-only.
- **ENG-10**: A derived (non-editable) leaf MUST be marked "Calculated" and MUST expose its formula on
  hover when the schema declares one; the "Calculated" mark MUST be dropped while the leaf carries a
  preview mark.
- **ENG-11**: A leaf of an unrecognized type MUST fall back to the read-only value and MUST NOT render
  a fabricated editor or throw.
- **ENG-12**: The numeric input MUST resolve the typed text to a finite number: an empty value MUST
  clear the field, and a text that does not resolve to a finite number (`NaN`, an infinity, an
  overflowing exponent) MUST leave the stored value unchanged. A form the platform resolves to a
  finite number is accepted. A sign or point typed so far (`-`, `.`, `-.`) MUST be kept as display
  text without reporting a value, so a negative number can be typed key by key; leaving the field with
  only such a pending text MUST restore the text of the last reported value. The desktop's general
  partial-token retention is not ported (Clarifications, 2026-10-16, amended 2026-10-30). A sign
  toggle MUST be offered while the field has focus, for keyboards without a minus key.
- **ENG-13**: A leaf MUST be interactive only when it is given a change handler; without one it MUST
  render inert, showing the value and allowing no edit.

**Rendering mutually exclusive groups**

- **ENG-14**: Leaves sharing a non-empty exclusive group MUST render as one single-choice control
  listing the members; the members MUST NOT render as independent toggles.
- **ENG-15**: An option MUST be labeled with its option label when non-empty and its display name
  otherwise.
- **ENG-16**: The single-choice control MUST offer a "no choice" entry only when no member carries a
  non-zero default; WHEN a default-active group has no active member, the control MUST show the "no
  choice" hint rather than rendering blank.
- **ENG-17**: An exclusive control without a selection handler MUST render inert — the active member
  (or the "no choice" hint) is shown and nothing is selectable.
- **ENG-18**: WHEN a member is selected, the control MUST report that member's dotted path; WHEN "no
  choice" is selected, it MUST report a signal that clears the group.

**Deriving and merging draft values**

- **ENG-19**: WHEN the empty draft is derived from a field tree, every leaf MUST be seeded under its
  dotted path in section order: text starts empty, a boolean starts off, and every other type takes its
  numeric default.
- **ENG-20**: WHEN the draft is derived from a stored sheet, a stored value MUST win at its dotted
  path, a schema leaf the stored object omits MUST keep its default, and a stored key absent from the
  schema MUST be carried over unchanged.
- **ENG-21**: The merge of a cascade onto a base MUST recurse where both sides hold an object at a key
  and replace the leaf otherwise, MUST return a fresh object, MUST NOT mutate either input, and MUST
  return the base unchanged for an empty cascade.

**Composing the form and marking previews**

- **ENG-22**: A control MUST accept an optional preview mark (`modified` or `recalculated`) that tints
  the control; a row carrying a mark MUST be labeled "Modified" or "Recalculated" even when its value
  is empty.
- **ENG-23**: The composition MUST expose a per-row mark source that receives every dotted path the
  row covers (a single leaf, or all members of an exclusive group), so the caller publishes marks
  without the composition owning mark state.
- **ENG-24**: The data-driven composition MUST render the derived sections and rows with each row
  labeled, resolve each control's value by its dotted path, and report an edit as that path with its
  new value.
- **ENG-25**: The composition MUST take exactly one control source — either values plus a change
  handler, or the injected per-field control builders — and MUST treat any other combination as a
  construction error rather than rendering a half-filled form.
- **ENG-26**: WHEN the control builders are injected, the composition MUST hand each row's control to
  the builder and MUST NOT subscribe to any value itself.

**Cross-cutting**

- **ENG-27**: Every user-visible string the engine renders MUST resolve through the shared translation
  catalog; no user-facing literal may appear in an engine component.
- **ENG-28**: Every visual value the engine renders (spacing, type, radius, color, focus, hover) MUST
  come from the shared token set; no ad-hoc visual value may be introduced.
- **ENG-29**: A value or label that contains markup MUST render as literal text; the engine MUST NOT
  interpret it as markup and MUST NOT introduce a DOM-injection sink.

### Key Entities

- **Field tree**: the decoded template fields the Web API Client already models — a leaf carries the
  leaf contract, a group carries ordered children plus optional label and declared columns.
- **Section**: one top-level key rendered as a block; carries the key, its label, every leaf beneath it
  in tree order, and the rows derived from those leaves.
- **Row**: one render line; a normal row carries a single untagged leaf, an exclusive row carries the
  members of one group plus the group identity, and a table row carries the group label plus one leaf
  per declared column.
- **Exclusive group**: the group identity advertised by the leaf contract plus its member leaves, in
  the order the service returned them.
- **Preview mark**: whether the last dry-run changed an editable leaf (`modified`) or recalculated a
  derived leaf (`recalculated`); absent on surfaces that never preview.
- **Draft**: the values addressed by dotted path that a sheet surface holds before anything is stored;
  derived from the schema defaults and overlaid with the stored sheet, and updated by merging a service
  cascade.
- **Control source**: whether the composition reads and reports values itself or hands each row's
  control to an injected builder; exactly one is active.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every control branch — numeric input, text input, boolean toggle, `config.*` toggle,
  derived read-only, unknown-type fallback and exclusive single choice — is proven by a rendering test;
  0 of them rely on visual inspection.
- **SC-002**: A synthetic template exercising every field type and an exclusive group renders a
  complete form with 0 per-system literals in the engine.
- **SC-003**: 0 user-facing literals exist in the engine components; 100% of the copy resolves through
  the translation catalog.
- **SC-004**: Removing a catalog key the engine references turns the engine suite red on the first run
  (0 false negatives).
- **SC-005**: A field value or label containing `<script>` produces 0 markup nodes; the text is proven
  to appear as literal text.
- **SC-006**: Deriving an empty draft from a template yields exactly one value per schema leaf and 0
  missing leaves; merging an empty cascade returns the base unchanged and mutates 0 inputs.
- **SC-007**: Every control the engine renders exposes an accessible role and name; 0 controls are
  rendered without one, proven by the rendering suite. The browser-level automated accessibility sweep
  of the mounted surface belongs to the feature that mounts the engine (the engine is not a surface).
- **SC-008**: A source scan finds 0 literal hex or px values in the engine components; 100% of the
  visual values come from the shared tokens.

## Assumptions

- The Web API Client feature already models the field tree (`FieldSchema`, leaf and group nodes,
  declared columns, exclusive-group membership, option and column labels) and the dotted-path value
  accessor; this feature decodes nothing new and re-declares no payload shape.
- The desktop engine (`desktop/lib/engine/`) is the source of the rendering rules — grouping by prefix,
  the control per schema rule, the exclusive group as one control, the seeded draft and incremental
  cascade merge, and the preview marks. This feature ports those rules to the web stack; the wire-type
  contract is unchanged, so the port is a re-expression, not a redesign.
- The rendering rules are the contract's, not a per-system choice: `config.*` is a 0/1 proxy node,
  exclusivity is a per-leaf tag grouped by identity across the whole parent, and the "Calculated"
  indicator belongs to a derived leaf.
- The engine is stateless: no control owns domain state, and the composition exposes per-field values,
  edits and per-row marks through injected seams. The web editing-state library the program plan
  selects (for the per-field subscription that preserves the granularity decision `AD-001`) is
  introduced by the sheet-cycle feature that owns the edit state, not here.
- The preview marks are produced by the sheet-cycle feature (which owns the dry-run and the draft); this
  feature owns rendering a mark it is given, including the "Calculated" default. The mark therefore
  never causes a request.
- The engine is proven by rendering tests of its own components, not by mounting a screen: this feature
  wires no surface (Clarifications, 2026-10-16). The editable sheet screen belongs to the sheet-cycle
  feature (`web/src/sheet/sheet_screen.tsx`), and the creation surface's read-only preview is wired by
  whatever consumer needs it, so the engine stays free of screen and edit-state concerns.
- The numeric input ports only a finite-number guard (Clarifications, 2026-10-16): empty clears the
  field and a non-finite text leaves the stored value unchanged. The desktop's full decimal-shape guard
  and restore-on-blur are deliberately not ported; how the guard is implemented against the platform
  input is a plan decision.
- Only rendering and derivation are in scope: creating/opening/saving a sheet, the draft backup, the
  offline cache, the library and the refresh shortcut belong to the sheet-cycle and PWA features.
- Continuous integration already runs the client suite and the requirement-ID lint covers TypeScript
  sources, so this feature introduces no new gate. No new runtime dependency is added.
- All identifiers, comments and user-facing strings remain English-only, per the project constitution.
