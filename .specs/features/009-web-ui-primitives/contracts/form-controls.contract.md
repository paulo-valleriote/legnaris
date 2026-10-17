# Contract: Form Controls

**Applies to**: the action, the field and the checkbox primitives under `web/src/ui/`.
**Requirements**: UIP-07 … UIP-17, SC-001, SC-003.
**Source of truth for appearance**: `web/DESIGN.md` → "Component specifications" (Button, Fields) and
the token tables of `008-web-ui-foundation/contracts/token-contract.md`.

## Action (button)

| Axis | Values | Rule |
|---|---|---|
| Variant | `primary` · `secondary` · `ghost` · `outline` · `danger` | primary = accent fill + on-accent text; secondary/ghost/outline read as progressively lighter surfaces; danger = danger border + danger text, **never filled** with the danger colour |
| Size | `sm` · `md` | neither falls below the declared control height; `sm` = small type step + small radius, `md` = body type step + medium radius |
| State | `hover` · `pressed` · `focus-visible` · `disabled` · `loading` | hover/press change background or tint only; focus-visible shows the tokenised ring; disabled and loading are announced and **cannot run the action**; loading shows the caller's in-flight label |

- Element rules: a real `button`; `type` defaults to `button` so an action never submits a form
  implicitly; the accessible name comes from the content or an explicit name (`UIP-10`).
- The danger variant is never the control that carries focus when a dialog opens (`UIP-19`).

## Field

| Aspect | Rule |
|---|---|
| Label | permanent, caller-provided, associated with the control; a placeholder is never the only label (`UIP-11`) |
| Control | `input`, `select` or `textarea`, sharing height, border, radius and focus values (`UIP-12`) |
| Error | optional inline message; the control is marked invalid and associated with it; with no error, no invalid marking and no association (`UIP-13`) |
| Disabled | cannot be edited, and is announced as disabled (`UIP-14`) |
| Numeric | right-aligned with tabular figures (`UIP-15`) |

## Checkbox

| Aspect | Rule |
|---|---|
| Label | permanently visible, associated with the box (`UIP-16`) |
| States | checked / unchecked, disabled, invalid — each exposed, not merely styled (`UIP-16`) |
| Value | a boolean in and a boolean out; the primitive never converts a schema value, so a numeric proxy is adapted by the caller (`UIP-17`) |

## Accessibility (all three)

- Keyboard-first: every focusable control is reached in visual tab order and shows the focus ring
  (`SC-003`).
- No state is carried by colour alone: the invalid state renders its message, the disabled state is
  programmatic, the busy state is announced (`UIP-09`, `UIP-13`).
- Every user-facing string resolves through `src/l10n/strings.ts`.

## Verification

| Check | Method |
|---|---|
| Every variant, size and state renders and is distinguishable | `ui_button.test.tsx` |
| A disabled or loading action does not run its handler; the default type never submits | `ui_button.test.tsx` |
| Label association, error association (present and absent), disabled and numeric presentation, and the three controls | `ui_field.test.tsx` |
| Checkbox label association, states and the boolean contract | `ui_checkbox.test.tsx` |
| No literal value, no unresolved token, no user-facing literal | `tokens_used.test.tsx`, `l10n_strings.test.ts` |
