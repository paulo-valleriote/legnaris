# Phase 0 Research: Web UI Primitives

**Feature**: `009-web-ui-primitives` | **Date**: 2026-10-17

Each decision resolves a choice the spec left to design and records the alternatives that were
rejected. Every environmental fact was verified in the repository, not assumed: the client's
directory layout, the tokens the previous phase declared, the two existing scans, the lint script's
suffix list and the ID corpus of the existing stylesheets.

---

## R1 — One layer, one stylesheet, no barrel

**Decision**: the primitives live in `web/src/ui/` with one module per primitive
(`button.tsx`, `field.tsx`, `checkbox.tsx`, `dialog.tsx`, `toast.tsx`) plus the focus-confinement
helper `focus_trap.ts`, and **one** stylesheet, `ui.css`, imported by the modules that need it — the
shape the engine already uses (`engine.css` imported by three modules). Class names follow the
program plan's convention `lg-<block>__<element>--<modifier>` (`lg-button--primary`,
`lg-dialog__title`, `lg-toast--danger`). There is no `index.ts` barrel: a caller imports the module
it uses.

**Rationale**: the plan fixes the home and the convention (§3.8) and asks for the scan to grow to
`src/ui/` (§3.12); one stylesheet keeps the layer a single unit for the literal scan, Prettier and
review, and matches how every existing feature directory holds its styles. A barrel adds an indirection
layer the client does not use anywhere else and would hide which primitive a module actually needs.

**Rejected alternatives**: (a) one stylesheet per primitive — five files, five import sites and no
gate improvement; (b) `src/ui/index.ts` re-exporting everything — a convenience the client's
conventions do not have, and one that makes it easy to import a primitive that pulls in styles the
importer does not need.

## R2 — The layer is a leaf: no feature import, one catalog import

**Decision**: `web/src/ui/` MUST NOT import from `api/`, `app/`, `auth/`, `catalog/`, `engine/`,
`library/`, `pwa/`, `sheet/` or `shell/`. Its only allowed client import is `../l10n/strings` for the
chrome copy it owns, and its only visual input is the `--lg-*` custom properties the browser resolves
from `src/theme/tokens.css`. `ui_layer.test.ts` asserts both halves: the expected modules exist, and
no module's import specifier names a feature directory.

**Rationale**: the whole point of the layer is that a primitive cannot depend on a screen. The
previous phase established the same shape one level down (`theme/` is a leaf the shell reads), and
constitution V's directionality requirement is the backend expression of the same instinct. Keeping
copy in the catalog is required by the client's one-copy-source rule, so `l10n/` is the single
permitted dependency.

**Rejected alternatives**: (a) letting a primitive import a feature hook "just for now" — that is how
a shared layer becomes a screen; (b) a `src/ui/tokens.ts` module re-exporting token names — the
tokens are CSS custom properties resolved by the browser, so a TypeScript mirror would exist only to
satisfy the tests, and `tokens.ts` already mirrors them for exactly that purpose.

## R3 — The base-layer rule changes in `web/AGENTS.md`, documented and asserted

**Decision**: the "no base component layer" bullet is replaced by a sanction of `web/src/ui/` (one
base layer, holding exactly this primitive set) while the "≥ 3 real usages" rule stays in force for
every other extraction; `web/DESIGN.md`'s component-specification section names the layer, its
primitive set and the class convention. `design_guidelines.test.ts` (the existing document suite)
gains the two assertions: `web/AGENTS.md` names `src/ui/` and still states the three-usages rule, and
`web/DESIGN.md` names the layer. The rule change is recorded as `AD-019` in `.specs/STATE.md`.

**Rationale**: the plan's risk list (§6) predicts exactly this edit, and a rule the code breaks is
worse than no rule. The constitution is not touched: its principles say nothing about the client's
component granularity, and the client's directionality (R2) is preserved rather than amended. Keeping
the three-usages rule for everything else is what stops the new layer from being used as a licence
for wrapper-for-wrapper's-sake.

**Rejected alternatives**: (a) amending the constitution — the constitution's Additional Constraints
point at `web/AGENTS.md` for client detail, and a PATCH/MINOR bump would buy nothing; (b) shipping the
layer while leaving the prohibition in place — the repository would contradict itself and every later
review would have to re-litigate it; (c) deleting the componentization section — it still governs
every non-primitive extraction.

## R4 — The field family stops at input, select and text area; the checkbox is its own primitive

**Decision**: `field.tsx` renders a permanent label, an optional inline error and one of three
controls — a text input, a select or a multi-line text area — sharing the declared height, border,
radius and focus values, plus a numeric presentation (right-aligned, tabular figures). `checkbox.tsx`
is a separate primitive: a permanently labelled boolean box exposing checked, disabled and invalid
states and reporting a boolean. The radio and the switch are specified in the design document but are
not built here; they land with the screen that first needs them.

**Rationale**: the plan names this phase's primitives exactly (button, field, checkbox, dialog,
toast), and the client today uses exactly an input, a select, a checkbox and a button — verified in
the sources. Building a radio and a switch now would ship two primitives no surface can exercise, and
a primitive that no suite can drive against real content is exactly the kind of speculation the
plan's anti-pattern list warns about. Splitting the checkbox out of the field matches the plan's own
list and keeps the field's contract about label-plus-error rather than about every control shape.

**Rejected alternatives**: (a) one `Control` super-primitive with a `kind` switch — it would pull the
schema-shaped branching the engine already owns into the shared layer, which the componentization rule
forbids; (b) building all six controls now — two of them would be unverifiable against real usage;
(c) putting the numeric presentation in the engine only — the design document makes right-aligned
tabular numerics part of the *field* specification, so the shared layer must own it and the engine
consumes it.

## R5 — The dialog is the overlay pattern with a standalone focus-confinement module

**Decision**: `dialog.tsx` renders the scrim (`--lg-color-overlay`) behind a panel
(`--lg-shadow-3`, `--lg-radius-lg`, hairline border), exposes `role="dialog"` with `aria-modal`,
takes its accessible name from its title and its description from the caller, moves focus inside on
open, confines Tab/Shift+Tab, dismisses on Escape, dismisses only when the activation's target is the
scrim itself, and returns focus to the invoking control on close. The confinement lives in
`focus_trap.ts` — a small module the `ui_focus_trap.test.ts` suite drives directly with zero, one and
many focusable controls.

**Rationale**: three surfaces hand-roll this pattern today (the catalog's creation dialog, the sheet
guard, the "Your sheet" delete confirmation), each with its own Escape listener and focus handling —
that duplication is what this feature removes. The native `<dialog>` element is not an option: the
test DOM does not implement its modal method, so it could not be driven by the rendering tests the
constitution requires; the catalog phase recorded the same constraint for the same reason. Extracting
the confinement makes the edge cases (no focusable control, wrapping at both ends, never escaping)
testable without rendering an overlay.

**Rejected alternatives**: (a) a headless-UI dependency for the dialog — the "no new package without
strict necessity" rule, and a dependency for one component is not necessity; (b) keeping Escape and
focus in each caller — the defect this feature exists to fix; (c) relying on the platform's modal
element — verified unavailable in the project's test DOM; (d) a focus trap built from the `inert`
attribute — the overlay is a sibling of the app, so `inert` on the rest of the document would have to
be managed per-route, a much larger change than confining Tab inside the panel.

## R6 — The notice owns no timer

**Decision**: `toast.tsx` renders a polite live region with a message, a variant and a dismiss
control, and it never removes itself: a notice stays rendered until the caller drops it or the person
dismisses it (which the caller observes through the dismiss callback).

**Rationale**: the plan asks for the notice as the non-blocking feedback surface (§3.8, §3.13) and
for nothing else; no surface consumes it yet, so the dismissal schedule has no requirements to
satisfy. The client's precedent for time-based behaviour is an injected cadence owned by the caller
(the draft backup), so a built-in timer would have to invent its own injectable schedule, its own
pause-on-interaction rules and its own tests — all policy with no consumer.

**Rejected alternatives**: (a) a built-in auto-dismiss with an injectable duration — more surface to
specify, test and later change than the plan asks for; (b) auto-dismiss with a fixed duration — a
second, invisible source of truth for a timing rule nothing declares.

## R7 — The requirement-ID lint walks `.css` too

**Decision**: `scripts/check_requirement_ids.py` gains `.css` in its walked suffixes (joining `.go`,
`.dart`, `.ts`, `.tsx`), its docstring is updated, and `scripts/test_lints.py` gains a case that a
stylesheet carrying a dangling requirement ID is reported and one carrying a defined ID is not.

**Rationale**: the layer's stylesheet is where most of its requirement citations live, and today a
citation there has no gate — a dangling one would reach `main`. The extension is safe on the current
corpus: the only requirement-shaped tokens in the repository's non-generated stylesheets are
`UIF-01`, `UIF-02`, `UIF-03`, `UIF-14`, `WCT-28`, `ENG-28`, `WSC-33` and `WAS-31`, and every one
resolves to a spec (verified against `.specs/**/spec.md` before the change). `node_modules/` and
`dist/` are already excluded from the walk, so the extension adds no noise. HTML is deliberately not
added: `web/index.html` contains `UTF-8`, a charset token the rule would flag, and the layer's
citations do not live there.

**Rejected alternatives**: (a) leaving stylesheets ungated — the citations this feature introduces
would be the ones without a gate; (b) a second lint script for stylesheets — the same walker and the
same corpus, duplicated; (c) adding `.html` as well — it would flag `UTF-8` and force a fix unrelated
to this feature.

## R8 — The scans grow a `ui/` block; the layer's literal rule gets teeth

**Decision**: `tokens_used.test.tsx` gains a `web/src/ui/` describe block (no hexadecimal colour, no
pixel value, every referenced token resolves), and `l10n_strings.test.ts` gains a `web/src/ui/` block
that resolves every `message()` key the layer renders, asserts the `ui.*` keys resolve and are
referenced, and — new for this layer — fails on a JSX text node containing letters, which is the one
shape a stray user-facing literal takes in a primitive.

**Rationale**: the plan's §3.12 asks for exactly these two scans to cover `src/ui/`, and mirroring
the existing blocks keeps the suites' intent legible. The JSX text-node check is added because the
layer is the first place where a primitive would author chrome copy directly, and the existing scans
would pass vacuously on it (they check keys, not the absence of literals).

**Rejected alternatives**: (a) a new scan suite per rule — duplicated file walkers and a second place
to update when the layout changes; (b) relying on review for the no-literal rule — the repository's
pattern is that a rule earns a test; (c) a general AST-based literal detector — no parser is a client
dependency and the regex covers the shapes these small presentational modules can take.

## R9 — The primitive states map onto declared semantics, with no new token

**Decision**: every primitive state consumes a semantic the previous phase declared, and this feature
adds no token:

| Need | Declared token |
|---|---|
| Action primary fill / hover / on-accent text | `--lg-color-accent`, `--lg-color-accent-hover`, `--lg-color-accent-text` |
| Action secondary, ghost, outline surfaces | `--lg-color-surface`, `--lg-color-surface-hover`, `--lg-color-bg`, `--lg-color-border` |
| Action danger (outlined) | `--lg-color-danger` border and text, `--lg-color-surface-hover` on hover |
| Action disabled / loading | `--lg-color-text-muted`, `--lg-color-surface`, `--lg-color-border` |
| Action sizes | `--lg-size-control` (height floor), `--lg-radius-sm`/`--lg-radius-md`, `--lg-type-13`/`--lg-type-15` |
| Field control | `--lg-color-bg`, `--lg-color-border`, `--lg-radius-sm`, `--lg-size-control`, `--lg-type-15` |
| Field error / invalid | `--lg-color-danger` |
| Field label / help | `--lg-color-text`, `--lg-color-text-muted`, `--lg-type-13` |
| Checkbox accent | `--lg-color-accent` (native accent colour), `--lg-color-danger` when invalid |
| Dialog scrim / panel / title | `--lg-color-overlay`, `--lg-color-bg`, `--lg-color-raised`, `--lg-shadow-3`, `--lg-radius-lg`, hairline |
| Notice surface / variant | `--lg-color-raised`, `--lg-color-border`, `--lg-shadow-2`, `--lg-radius-md`, `--lg-color-info`/`--lg-color-success`/`--lg-color-warning`/`--lg-color-danger` |
| Focus ring (all) | `--lg-color-focus`, `--lg-focus-width`, `--lg-focus-offset` (from `base.css`) |

The notice's variant is a **state stripe**, not a fill: its message text stays on `--lg-color-text`,
so no new foreground/background pair (and therefore no new contrast pair) is introduced. Both action
sizes keep the declared control height as their floor, so no size token is added either.

**Rationale**: the previous phase's token contract is the authority and the plan locks "zero
dependencies, tokens plus plain CSS". Reusing the declared semantics keeps the both-theme contrast
assertion meaningful without re-deriving it, and it keeps this feature out of the token file — so the
phase cannot quietly change the palette while claiming to add components.

**Rejected alternatives**: (a) a filled danger button — it would need a `--lg-color-danger-text` pair
against the danger fill, and the previous phase already recorded the outlined form as the delivered
shape; (b) a filled notice per state — four new foreground pairs per theme for a component nothing
consumes; (c) a `--lg-size-control-sm` token for the small action — a token added for one component
that the design document does not ask for.
