# Contract: Overlay and Feedback

**Applies to**: the dialog primitive, its focus-confinement module and the non-blocking notice
primitive under `web/src/ui/`.
**Requirements**: UIP-18 … UIP-26, SC-001, SC-003.
**Source of truth for appearance**: `web/DESIGN.md` → "Component specifications" (Dialog / overlay,
Feedback), "Accessibility" and "Per-screen patterns" (Overlays).

## Dialog

| Field | Rule |
|---|---|
| Scrim | the overlay colour, behind the panel, covering the viewport (`UIP-18`) |
| Panel | modal elevation (`--lg-shadow-3`), large radius, hairline border, on the page background (`UIP-18`) |
| Semantics | `role="dialog"` with modal semantics; never dependent on the platform's native modal element (`UIP-18`) |
| Name | from the dialog's title; the caller's description is exposed alongside it (`UIP-19`) |
| Open | focus moves inside the dialog (`UIP-19`) |
| Keyboard | Tab at the last control wraps to the first; Shift+Tab at the first wraps to the last; Escape dismisses the topmost dismissible surface only (`UIP-20`) |
| Close | focus returns to the control that opened the dialog; the caller is told exactly once (`UIP-20`, `UIP-21`) |
| Scrim activation | dismisses; an activation inside the panel does not (`UIP-21`) |
| Destructive action | never the control that carries focus on open, so Enter cannot destroy by default (`UIP-19`, `UIP-20`) |

## Focus confinement (standalone module)

| Situation | Rule |
|---|---|
| No focusable control | the container stays focusable; focus is never moved outside the dialog (`UIP-22`) |
| One control | forward and backward both land on it (`UIP-22`) |
| Many controls | cycles forward and backward, wrapping at both ends, never escaping the container (`UIP-22`) |

The module is behaviour, not layout: it is driven directly by `ui_focus_trap.test.ts` with zero, one
and many controls and with the focus forced outside.

## Non-blocking notice

| Field | Rule |
|---|---|
| Role | a polite live region; no focus theft, no scrim, no modal semantics (`UIP-23`) |
| Message | rendered as text; an empty message renders no notice (`UIP-24`) |
| Dismiss | a control whose accessible name resolves through the catalog (`UIP-24`) |
| Variant | `info`, `success`, `warning`, `danger`; a stripe/tint only, with the meaning carried by the
text, so nothing depends on colour alone (`UIP-25`) |
| Schedule | none — it stays until the caller removes it or the person dismisses it (`UIP-26`) |

## Verification

| Check | Method |
|---|---|
| Role, modal semantics, name and description | `ui_dialog.test.tsx` |
| Focus moves in, is confined, and returns on close | `ui_dialog.test.tsx`, `ui_focus_trap.test.ts` |
| Escape, scrim activation and inner activation resolve correctly, exactly one dismissal per close | `ui_dialog.test.tsx` |
| Live region, dismiss name, text rendering, four variants, no self-dismissal | `ui_toast.test.tsx` |
