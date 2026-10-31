# Phase 0 Research: Web UI Engine

**Feature**: `.specs/features/004-web-ui-engine/` | **Date**: 2026-10-16

Every decision below answers an open question in the feature's Technical Context or resolves a choice
the program plan's F4 task list leaves implicit. No `NEEDS CLARIFICATION` remains.

---

## R1. What the engine owns, and what shape it takes

**Decision**: six modules under `web/src/engine/`, a one-to-one port of `desktop/lib/engine/`:
`field_grouping.ts` (pure section/row derivation plus the label helpers), `field_control.tsx` (one
leaf's control), `exclusive_control.tsx` (one exclusive group's single control), `sheet_defaults.ts`
(the seeded draft, `seedFromSheet`, and the deep `mergeCascade`), `sheet_mark.ts` (the mark type and
its tint), and `sheet_form.tsx` (the composition, with `controlBuilder` / `exclusiveControlBuilder` /
`rowMarkBuilder` seams). Everything is state-free: the pure functions return fresh values, and the
components are controlled and report through callbacks.

**Rationale**: the program plan's F4 is explicitly "porta de `desktop/lib/engine/`", and the desktop
modules already separate derivation (pure) from rendering (controlled) from composition (seams) — that
separation is exactly what makes the same rules testable on the web with a different UI toolkit. A
line-by-line port of the *rules* (not the widgets) keeps the two implementations from drifting on the
things that matter — grouping by prefix, the control per schema rule, exclusivity, the seeded draft,
the incremental cascade — while the widgets themselves are re-expressed in React.

**Alternatives considered**:

- *Rewrite the engine from a blank slate* — rejected: the rules are the product (they encode the
  backend contract and `AD-001`…`AD-010`) and re-deriving them invites divergence; the plan says port.
- *A single monolithic `SheetForm` module* — rejected: the derivation is pure and I/O-free and must be
  unit-testable without rendering, and the exclusive control and the mark are separately testable
  units, matching the desktop split.

## R2. The controlled value model and the seams

**Decision**: a sheet draft is `Record<string, unknown>` — the same nested shape the F2 `Sheet.data`
carries — read through the F2 `valueAtPath` accessor and written through a local `setPathValue`.
`SheetForm` takes exactly one control source: either `data` plus `onChanged(path, value)`, or the
`controlBuilder`/`exclusiveControlBuilder` pair. With builders, the form does not read values at all;
the builder owns the value subscription and the edit reporting, which is the web seam that preserves
`AD-001` (one edit rebuilds one control, not the whole form). A `rowMarkBuilder(paths)` seam lets a
caller publish a mark per row without the form owning mark state. Passing both sources, or neither, is
a construction error.

**Rationale**: `AD-001` is a granularity requirement — a per-field subscription — and the desktop
expressed it through a Riverpod `select`. On the web the equivalent is a builder that a state
subscription mounts per row; keeping the form state-free is what lets the sheet-cycle feature choose
its own subscription mechanism without touching the engine. Making the one-source rule a construction
error (not a silent precedence) means a half-wired form fails loudly instead of rendering nothing
(ENG-25).

**Alternatives considered**:

- *A context/provider the engine mandates* — rejected: it would force a state library into the engine,
  which the plan assigns to the sheet-cycle feature.
- *A `renderRow` callback returning the whole row* — rejected: it moves the label, the section header
  and the table layout into every caller, duplicating the composition the engine owns.

## R3. The numeric input and its guard

**Decision**: the numeric leaf renders an `<input>` with `inputMode="decimal"` (a text input, **not**
`type="number"`). The display text is held locally; each keystroke is resolved with `Number(text)` and
kept only when the result is finite. Empty text reports a cleared value (`null`); text that resolves to
`NaN`, an infinity or an overflow leaves the stored value unchanged. The desktop's partial-token
retention and restore-on-blur are not ported (Clarifications, 2026-10-16). *Amended 2026-10-30:* a sign
or point typed so far (`-`, `.`, `-.`) is kept as display text without reporting, because rejecting
the bare `-` made a negative number impossible to type; a blur with only that pending text restores the
last reported value's text, and a focus-only sign toggle covers the iOS decimal pad, which has no minus.

**Rationale**: `type="number"` would let the browser silently *drop* the keystrokes that do not parse,
so the DOM value would become `""` and the guard would be unobservable — the tests could not prove "the
value does not change". A text input with a finite-number guard keeps the guard in our code, where it
is testable, and matches the clarified scope (finite-number guard only). Holding the display text
locally is what lets a half-typed value render without immediately overwriting the stored one, without
the restore-on-blur rule the clarification dropped.

**Alternatives considered**:

- *`<input type="number">`* — rejected: it hides the guard inside the browser and cannot be driven by
  the required test environment; partial input reads as `""`.
- *A fully controlled input with no local text* — rejected: the displayed text would have to equal the
  stored number on every keystroke, so an intermediate non-finite state could not be shown or
  asserted.

## R4. The mark tint without an ad-hoc visual value

**Decision**: a marked control gets a CSS class (`lg-field--marked`) whose tint is a background token
that already has a declared contrast pair with the control's foreground — `--lg-color-surface`, whose
pair with `--lg-color-text` is already asserted by `tokens_contrast.test.ts`. The row's label
("Modified" / "Recalculated") distinguishes the two marks; both share the one tint, as the desktop did.
If a new background token is ever introduced for the tint, its contrast pair MUST be declared in
`theme/tokens.ts`.

**Rationale**: `ENG-28` and `web/AGENTS.md` forbid ad-hoc visual values and require every foreground
token to declare its pair. Reusing an existing, already-asserted token keeps the tint inside the token
system and needs no new contrast assertion; the desktop used one tint for both marks, so the web keeps
the same and lets copy carry the distinction.

**Alternatives considered**:

- *An inline `style` with an rgba/hex tint computed from the accent token* — rejected: it authors a
  visual value in a component and defeats the token scan.
- *Two distinct tints (Modified vs Recalculated)* — rejected: it invents a color distinction the
  desktop did not make and would need its own contrast pair; the label already distinguishes them.

## R5. The exclusive group's single control and its "None" rule

**Decision**: an exclusive group renders as one native `<select>` (a single-choice control) whose
options are its members plus, only when no member carries a non-zero default, a "no choice" entry. A
member is labeled with its `optionLabel` when non-empty and its display name otherwise. The selection
control reports the chosen member's dotted path, or a sentinel (`__none__`) that a real path can never
collide with, meaning "clear the group". A member counts as active only when its value is a non-zero
number. With no selection handler the control renders disabled.

**Rationale**: the desktop used a `DropdownButton`; the native `<select>` is the platform single-choice
control a strict CSP and the "no design-system dependency" rule both prefer, and it is fully drivable by
the test environment. The "None only when no default" rule and the non-zero numeric test come straight
from the backend contract (`AD-002`, `EXC-11`), so they are ported verbatim rather than reinterpreted.

**Alternatives considered**:

- *A custom listbox* — rejected: it re-implements focus, keyboard and ARIA that `<select>` gives for
  free, against `web/AGENTS.md` ("no component library, no wrapper for its own sake").
- *Radio buttons per member* — rejected: it spreads one choice across N tab stops and loses the dense
  single row the desktop used (`EXC-09`).
- *Dropping the "None" entry* — rejected: a 0/1 proxy group with no active member would render blank
  (ENG-16 forbids that).

## R6. The derivation and the incremental merge

**Decision**: `seedDefaults(fields)` walks the derived sections and sets each leaf's seed under its
dotted path (text → empty string, boolean → `false`, everything else → the schema default).
`seedFromSheet(fields, data)` is `mergeCascade(seedDefaults(fields), data)`. `mergeCascade(base,
overrides)` copies the base, recurses where both sides hold a plain object at a key, replaces the leaf
otherwise, and never mutates either input; an empty override returns a copy equal to the base. A stored
key absent from the schema is carried over because the merge is key-driven, not schema-driven.

**Rationale**: the desktop's `seedDefaults`/`seedFromSheet`/`mergeCascade` are the shared derivation the
editable sheet and the read-only preview both read from; porting them verbatim keeps the two surfaces
from drifting, and the "carry an unknown key over" behavior is what makes a sheet survive a template
that gains and loses leaves. `mergeCascade` is also the incremental cascade merge the sheet-cycle
feature applies after `PATCH` (`AD-006`), so it must be the same function.

**Alternatives considered**:

- *A schema-driven projection that drops unknown keys* — rejected: it would silently delete a sheet's
  data on a template change; the desktop carries it over (ENG-20).
- *A shallow `Object.assign`* — rejected: it would replace a whole nested group when the cascade only
  changed one leaf, discarding the sibling values (`AD-006`).

## R7. Label derivation and the humanization helper

**Decision**: `fieldDisplayName(key, schema)` returns `schema.label` when it trims to non-empty and a
mechanical humanization of the key otherwise; `groupDisplayName(key, label)` and
`fieldColumnName(key, schema)` mirror it for groups and table columns. The humanization splits
`_`/`-` and camel-case boundaries, lowercases, and capitalizes the first letter — a pure function
ported from the desktop.

**Rationale**: `RND-05` fixed "schema label, else humanized key" as the one label rule, shared by the
engine and the template summary. Porting the exact helper keeps labels identical between the desktop
and the web for the same template, and it is the reason a template with no labels still renders
readably. It is pure, so it is unit-tested directly.

**Alternatives considered**:

- *Render the raw key when the label is empty* — rejected: keys are machine-shaped
  (`armorClassAbility`) and would look broken; `RND-05` mandates humanization.
- *Hardcode a label per known key* — rejected: it breaks the "any template renders" promise (ENG-03).

## R8. Table rows and column alignment

**Decision**: a group that declares `columns` renders as one table row — the group's label and one cell
per declared column, in the declared order — with a header naming the columns; an unknown or absent
column is skipped; when no declared column resolves, the group falls back to stacked rows. Consecutive
table rows with the same column identity share one header. A numeric cell is right-aligned through a
class, everything else left.

**Rationale**: this is the `RND-21` rule the desktop shipped (a declared-column group is a table, not a
stack of rows). Porting it keeps the reference template's stats tables rendering the same way, and the
fallback keeps a malformed column list from producing an empty table (ENG-04).

**Alternatives considered**:

- *Always stacked rows* — rejected: it regresses `RND-21` and the density `web/AGENTS.md` asks for.
- *A real `<table>` element* — left to implementation; the contract fixes the *rendering* (one row, one
  cell per column, aligned), not the element, so the suite asserts cells and alignment, not markup.

## R9. Where the new copy lives

**Decision**: four keys are added to the existing typed English catalog
(`web/src/l10n/strings.ts`): `engine.calculated` (the derived indicator), `engine.none` (the exclusive
"no choice" entry), `engine.modified` and `engine.recalculated` (the row marks). The F2 `api.error.*`
keys are untouched. No literal string is authored in `web/src/engine/`.

**Rationale**: `ENG-27` requires every string to resolve through the catalog. The engine is the first
place the widget-level copy ("Calculated", "None") appears, so it owns those keys; keeping them in the
one catalog keeps the later localization feature a matter of replacing the module.

**Alternatives considered**:

- *A second catalog for the engine* — rejected: two catalogs is two places to look for a string.
- *Reusing an existing key* — rejected: those strings are not errors and do not exist yet.

## R10. How the engine is tested

**Decision**: the components are rendered directly with Testing Library, by role and accessible name,
and the pure functions are unit-tested; request counting is irrelevant because the engine issues no
request. The F1 token-usage scan (`tokens_used.test.tsx`) and the F3 copy scan (`l10n_strings.test.ts`)
are extended to scan `web/src/engine/`, so the token and copy rules keep one gate. No surface and no
test-only route are added (Clarifications, 2026-10-16).

**Rationale**: an engine that mounts nowhere yet is proven by rendering its own components — that is
still an interface test (roles, names, control per type, marks), which is what constitution III and VII
require. Pushing a test-only route into production to satisfy an end-to-end sweep would add a surface
with no product value and is exactly what the clarification excluded.

**Alternatives considered**:

- *A Playwright case over a test route* — rejected: a test-only route is production code that only the
  test uses; the browser-level sweep belongs to the feature that mounts the engine (`F9-T007`,
  `AD-014`).
- *MSW / a network double* — rejected: the engine performs no I/O, so there is nothing to intercept.

## R11. The `<script>`-in-data guarantee

**Decision**: values and labels are rendered as React text children only; the engine contains no
`dangerouslySetInnerHTML`, no `innerHTML`, and no markup interpolation of server data. A dedicated case
renders `characterName = "<script>"` and asserts the visible text is the literal string and that no
`<script>` element was created.

**Rationale**: this is the one security-relevant surface the engine owns — it renders attacker-influenced
sheet data and template labels. React escapes text children by construction, and `ENG-29` plus the F1
anti-sink ESLint rules make the guarantee both structural and asserted (constitution VI, SC-005).

**Alternatives considered**:

- *Sanitize through DOMPurify* — rejected: there is no HTML to sanitize; the engine never renders
  markup, so a sanitizer would be a dependency guarding a path that does not exist.
