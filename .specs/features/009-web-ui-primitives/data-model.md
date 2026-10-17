# Phase 1 Data Model: Web UI Primitives

**Feature**: `009-web-ui-primitives` | **Date**: 2026-10-17 | **Spec**: [spec.md](./spec.md)

This feature's "data" is the component contract: the layer, its five primitives, the interactive
states each owns, the focus confinement the dialog delegates to, the non-blocking notice and the one
copy key the layer adds. Each entity lists its fields, the rules that constrain it and the
requirements that make those rules testable. There is no server entity, no migration and no
persisted state.

---

## 1. Shared layer

| Field | Meaning | Rule |
|---|---|---|
| `path` | `web/src/ui/` | the one sanctioned base-component home (`UIP-01`, `AD-019`) |
| `primitives` | action, field, checkbox, dialog, notice | exactly these five, one module each (`UIP-01`) |
| `stylesheet` | `ui.css` | one token-only stylesheet for the layer (`UIP-01`, `UIP-04`) |
| `allowedImports` | React plus `../l10n/strings` | no feature directory may appear in an import specifier (`UIP-02`) |
| `classConvention` | `lg-<block>__<element>--<modifier>` | declared in the design document (`UIP-03`) |
| `scanCoverage` | literal scan + copy scan | both cover `web/src/ui/` (`UIP-04`, `UIP-05`) |
| `citationGate` | requirement-ID lint walks `.css` | the stylesheet's citations resolve (`UIP-06`) |

**Rule change** (`UIP-03`, `SC-007`): the developer-rules document states that `web/src/ui/` is the
one place a base component may live and that the "three real usages" rule still governs every other
extraction; the design document names the layer, its primitive set and the class convention. The
previous "no base component layer" prohibition is replaced, not restated. Asserted by the document
suite, recorded as `AD-019`.

**Removed-from-scope**: no screen is migrated by this feature; the existing feature styles for
actions and overlays (`lg-button`, `lg-overlay`, `lg-dialog*` in `catalog.css`) stay until the
screen-refactor phase switches their consumers onto the layer.

---

## 2. Primitive: the action (button)

| Field | Values | Rule |
|---|---|---|
| `variants` | `primary` \| `secondary` \| `ghost` \| `outline` \| `danger` | primary fills with the accent and pairs the on-accent text; danger is outlined with the danger border and text, never filled (`UIP-07`) |
| `sizes` | `sm` \| `md` | both keep the declared control height as their floor; `sm` uses the small type step and the small radius, `md` the body step and the medium radius (`UIP-08`) |
| `states` | `hover` \| `pressed` \| `focus-visible` \| `disabled` \| `loading` | see §3 (`UIP-09`) |
| `element` | a real `button` | `type` defaults to `button`, so an action inside a form never submits it implicitly (`UIP-10`) |
| `name` | from content or an explicit accessible name | never unnamed (`UIP-10`) |
| `loadingLabel` | caller copy | the in-flight label is the consumer's string, not the primitive's (`UIP-09`) |

**Activation rule** (`UIP-10`): an action that is disabled **or** loading MUST NOT run its handler.
The primitive owns the guard so no caller can forget it.

**Tokens** (`R9`): accent, accent-hover, accent-text, surface, surface-hover, bg, border, text,
text-muted, danger, shadow-free; sizes from `--lg-size-control`, `--lg-radius-sm`/`md`, `--lg-type-13`/`15`.

---

## 3. Interactive state

One condition a primitive can be in, with what it changes and what it announces.

| State | Visual change | Announced as | Activation effect |
|---|---|---|---|
| `hover` | background or tint only | — | runs the action |
| `pressed` | background or tint only | — | runs the action |
| `focus-visible` | the tokenised focus ring | focus | runs the action |
| `disabled` | muted surface and text | disabled | **does not run** |
| `loading` | caller's in-flight label | busy | **does not run** |
| `invalid` | danger border and inline error | invalid, with the error described | — |
| `checked` | accent-filled box | checked / not checked | reports the boolean |

Rules: hover and press MUST NOT change layout, size or position; `focus-visible` uses the declared
focus tokens and is never removed without a replacement; `disabled` and `loading` are announced, not
merely styled (`UIP-09`, `UIP-13`, `UIP-16`).

---

## 4. Primitive: the field

| Field | Meaning | Rule |
|---|---|---|
| `label` | caller's text, permanently rendered | MUST be associated with the control; a placeholder is never the only label (`UIP-11`) |
| `control` | `input` \| `select` \| `textarea` | the three share height, border, radius and focus values (`UIP-12`) |
| `error` | optional caller copy | rendered inline; the control is marked invalid and associated with the message; absent → no marking and no association (`UIP-13`) |
| `disabled` | boolean | the control cannot be edited and is announced as disabled (`UIP-14`) |
| `numeric` | boolean | the value is right-aligned with tabular figures (`UIP-15`) |

**Association rule**: the label binds to the control by id, and the error only appears in the
control's described-by list when an error exists — a dangling description is a defect.

---

## 5. Primitive: the checkbox

| Field | Meaning | Rule |
|---|---|---|
| `label` | caller's text | permanently visible and associated with the box (`UIP-16`) |
| `checked` | boolean | reported as a boolean; the primitive never converts a schema value (`UIP-17`) |
| `disabled` | boolean | announced as disabled (`UIP-16`) |
| `invalid` | boolean | announced as invalid (`UIP-16`) |

**Boundary rule** (`UIP-17`): adapting a stored value that is not a boolean — the engine's numeric
proxy, for example — stays with the consumer; the primitive's value is a boolean in both directions.

---

## 6. Primitive: the dialog, and the focus confinement it delegates to

| Field | Meaning | Rule |
|---|---|---|
| `title` | caller copy | the accessible name of the dialog (`UIP-19`) |
| `description` | optional caller copy | rendered and associated (`UIP-19`) |
| `onDismiss` | caller callback | invoked exactly once per close (`UIP-21`) |
| `scrim` | the overlay element | dismisses only an activation whose target is the scrim itself (`UIP-21`) |
| `panel` | the raised surface | `--lg-shadow-3`, the large radius, hairline border, no activation dismissal (`UIP-18`, `UIP-21`) |
| `role` | dialog + modal semantics | asserted by role and attribute (`UIP-18`) |

**Lifecycle**

| From | Event | To | Side effect |
|---|---|---|---|
| closed | open requested | open | focus moves inside; the name and description are wired |
| open | Tab at the last control | open | focus wraps to the first inside |
| open | Shift+Tab at the first control | open | focus wraps to the last inside |
| open | Escape | closed | `onDismiss` once; focus returns to the invoking control |
| open | activation on the scrim | closed | `onDismiss` once; focus returns |
| open | activation inside the panel | open | nothing |

**Focus confinement** (`UIP-22`, standalone module): exposes the container and moves focus forward
and backward.

| Situation | Rule |
|---|---|
| no focusable control | the container itself stays focusable, so the dialog remains dismissible and focus is never lost outside |
| exactly one control | forward and backward both land on it |
| several controls | forward from the last wraps to the first; backward from the first wraps to the last; focus never leaves the container |
| the first control is destructive | it is not the control that carries focus on open — Enter cannot destroy by default (`UIP-19`, `UIP-20`) |

---

## 7. Primitive: the non-blocking notice

| Field | Meaning | Rule |
|---|---|---|
| `message` | caller copy | rendered as **text**, never markup; an empty message renders no notice (`UIP-24`) |
| `variant` | `info` \| `success` \| `warning` \| `danger` | the state is a stripe/tint, and the text carries the meaning, so nothing depends on colour alone (`UIP-25`) |
| `onDismiss` | caller callback | the dismiss control's accessible name resolves through the catalog (`UIP-24`) |
| `role` | polite live region | no focus theft, no scrim, no modal semantics (`UIP-23`) |
| `schedule` | **none** | the primitive owns no timer; it stays until the caller removes it or the person dismisses it (`UIP-26`) |

**Lifecycle**: created by the caller → rendered and announced politely → dismissed by the person (the
caller is told, and the caller removes it) → gone. There is no implicit transition.

---

## 8. Copy key

| Key | Value | Rule |
|---|---|---|
| `ui.dismiss` | `'Dismiss'` | the notice's dismiss control's accessible name; declared in the catalog, referenced by the layer, and covered by the copy scan (`UIP-05`, `UIP-24`) |

The layer owns no other copy: every other string it renders arrives as a prop from the caller
(`UIP-05`).
