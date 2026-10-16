# Contract: Application Shell Layout

**Applies to**: every client screen rendered under `web/src/`.
**Requirements**: FND-01, FND-02, FND-03, FND-04, FND-05.

## Regions

| Order | Region | Landmark | Width | Collapse priority |
|---|---|---|---|---|
| 1 | Navigation | `nav` | 200–240 px, fixed | 2 (never collapses before the panel) |
| 2 | Content | `main` | grows; readable measure ~960 px for forms | 3 (last to give up space) |
| 3 | Contextual panel | `complementary` | 280–320 px | 1 (hidden first) |

Rules:

- All three regions are present on every declared route; the shell never renders a route without
  navigation or without the content region.
- The content region renders the designed empty-workspace view until a feature supplies real
  content; no placeholder rows, lorem text, or demo data are allowed (FND-01).
- Navigation is persistent on desktop-class viewports and MUST NOT collapse into an overlay at
  large widths; below the shell's minimum width it may become a compact bar, but content never
  overlaps navigation.
- When the viewport narrows: hide the panel → let content reach its minimum → then reduce
  navigation. Order is asserted by test.

## Tokens

- Every visual value comes from `theme/tokens.css` / `theme/tokens.ts` (FND-02). Literal spacing,
  border radius, font size, and color values in a component are a defect.
- Scale: type 12/13/15/17 (17 is the hard cap), spacing 4/8/12/16/24, radius 2 (inputs, buttons) /
  4 (panels, dialogs), row height 32, control height 32–36.
- The visual direction follows `web/AGENTS.md` (ported from `desktop/AGENTS.md`): functional before
  decorative, dense yet legible, flat surfaces, hairline borders, elevation 0, no gradients,
  no glassmorphism, no radius above 4 px, no icon overload, no "AI SaaS" aesthetic.

## Interaction

- Keyboard: tab order follows visual order; no keyboard traps; `Esc` dismisses the contextual panel
  when it is dismissible (FND-04).
- Focus: every focusable control shows a visible focus ring from the focus token; a focused control
  is distinguishable from hover (FND-04).
- Hover: hover changes background/tint only — never layout, size, or position.
- Contrast: every foreground/background pair the shell uses meets AA (FND-05). A new color token
  without a declared contrast pair fails review.

## Verification

| Check | Method |
|---|---|
| Region order and presence | rendering test asserting landmark order |
| Empty workspace state | rendering test asserting the empty-state text and the absence of demo rows |
| Panel collapses first | viewport test at narrow widths |
| No literal visual values | test comparing computed styles against the token map |
| Focus and keyboard reachability | keyboard-only walkthrough test |
| Contrast | automated contrast assertion over declared pairs |
