# DESIGN.md — the visual contract for the web client

This document is **the sole written source of the visual values and rules** for every screen under
`web/src/`. Where `web/AGENTS.md` and this document could disagree, this one wins and the
disagreement is a defect. `web/AGENTS.md` keeps the development, security and architecture rules
and points here for anything visual.

The rules are enforced by tests, not by memory: the token source is mirrored in
`src/theme/tokens.ts`, the contrast pairs are asserted per theme, and a source scan keeps literal
colours and pixel values out of every component directory (`src/theme/` is the one sanctioned
exception, because it is where the literals are declared).

---

## Principles

- **Usability first; hierarchy guides the eye.** Size, weight, colour and elevation say "read this
  first". If a screen is only legible because everything is emphasised, nothing is.
- **Comfortable density, not sparse and not cramped.** Group and align so the eye scans; never go
  below the token sizes to gain density, and never pad a screen out to fill space.
- **Accessible by default.** AA contrast in **both** themes, a visible focus ring on every
  focusable control, and a keyboard-only path through every surface.
- **Token-only, zero dependencies.** There is no UI library, theme framework or font package: the
  design system is the tokens plus plain CSS plus convention.
- **Restraint.** One accent per theme, subtle depth, no decoration that does not carry data or
  hierarchy.

---

## Token architecture

Three layers live in `src/theme/tokens.css` (mirrored in `src/theme/tokens.ts`):

1. **Primitives** — raw scales with no role attached: the parchment, ink, charcoal, amber, danger
   and system ramps (`--lg-parchment-*`, `--lg-ink-*`, `--lg-charcoal-*`, `--lg-amber-*`,
   `--lg-danger-*`, `--lg-info-*`, `--lg-success-*`, `--lg-warning-*`), plus the space, radius, font,
   size, focus, motion and easing scales.
2. **Semantics** — the roles components consume, declared **per theme**: `--lg-color-bg`,
   `--lg-color-surface`, `--lg-color-surface-hover`, `--lg-color-raised`, `--lg-color-inset`,
   `--lg-color-overlay`, `--lg-color-text` / `-muted` / `-subtle`, `--lg-color-border` /
   `-strong`, `--lg-color-accent` / `-hover` / `-text`, `--lg-color-danger` / `-hover` / `-text`,
   the system states, `--lg-color-focus`, and `--lg-shadow-1` … `--lg-shadow-3`.
3. **Component** — added only where no semantic covers a component's need (for example a future
   `--lg-color-nav-bg`). None is required today.

Naming is `--lg-kebab-case`, and a primitive is never consumed directly by a component: components
read semantics.

**Adding a token — the mandatory flow**

1. Declare it in **both** themes in `tokens.css` (a semantic declared in one theme only is a defect).
2. Mirror it in `tokens.ts` (`designTokens`, and `contrastPairs` when it is a foreground).
3. Run `npm test`: the both-theme assertion resolves every declared pair and every documented token.

The light theme is declared on `:root`; the dark theme overrides the semantic layer under the
`data-theme="dark"` attribute selector on the document element.

---

## Colour

Two themes, one accent each.

- **Light — parchment.** Page `#EFE6D4`, surface `#FBF6EA`, inset `#E4D8BF`; text `#2A2118`,
  muted `#6B5B47`, subtle `#726046`; border `#D6C9AF`, border-strong `#B9A886`; accent burnt amber
  `#B45309` (hover `#92400E`), on-accent `#FFFFFF`; danger brick `#A83232`.
- **Dark — charcoal plus amber.** Page `#17181B`, surface `#212327`, inset `#141518`; text
  `#ECE9E4`, muted `#A8A29B`, subtle `#948D85`; border `#33363B`, border-strong `#4A4E54`; accent
  `#F59E0B` (hover `#FBBF24`), on-accent `#201A10`; danger `#F87171`.

Rules:

- **One accent per theme.** The accent marks an action, an active state or a selection — never
  decoration. Whenever the accent fills a surface, the text on it comes from
  `--lg-color-accent-text`, never a bare white or black.
- Amber text on parchment must use the darker amber steps, not the accent fill, so it clears AA.
- **System states** are info, success and warning, each with a light and a dark value, alongside
  danger; they are roles, not decoration.
- **Contrast is the authority.** Every foreground/background pair declares its minimum ratio per
  theme and is asserted: body text 4.5:1, the focus ring 3:1. The concrete hexes above are the
  starting point; the both-theme assertion is what rules, so a value may be adjusted to pass it.
- Each theme sets `color-scheme` so native controls and scrollbars match (`light` / `dark`).

---

## Typography

- **One self-hosted variable family: Geist** (OFL-1.1), vendored under `public/fonts/geist/` with
  its license. No second family and no monospace — tabular figures cover numeric alignment. No
  font, style or script is requested from a third-party origin.
- `--lg-font-family` leads with `'Geist'` and falls back to the system stack, so text renders
  without the file and never breaks layout.
- Loaded with `font-display: swap` and preloaded (`as="font"`, `crossorigin`).
- **Scale:** `12 / 13 / 15 / 16 / 18 / 20 / 24 / 30` px (`--lg-type-*`). There is no upper cap
  below the largest step; headings are allowed to exceed body size.
- **Weights:** 400 regular, 500 medium, 600 semibold, 700 bold.
- **Line height:** `--lg-line-tight` (1.2) for headings, `--lg-line-normal` (1.5) for body.
- **Tabular figures** (`font-variant-numeric: tabular-nums`) on every numeric or derived column so
  values align vertically.

---

## Spacing and layout

- **Space scale:** `2 / 4 / 8 / 12 / 16 / 24 / 32 / 48` px (`--lg-space-*`). Every gap, pad and
  margin is a step of this scale.
- **The shell is two regions: navigation then content.** There is no contextual panel. On every
  route the shell exposes exactly a named `nav` and a `main`, in that order.
- **Navigation is persistent:** fixed between `--lg-size-nav-min` and `--lg-size-nav-max`
  (200–240 px). It never becomes a hamburger or an overlay.
- **Content keeps a minimum width** (`--lg-size-content-min`, 320 px) and a readable measure
  (`--lg-size-content-max`, 960 px, centred). As the viewport narrows, the navigation stays put and
  the content reaches its minimum without ever overlapping the navigation — it scrolls instead of
  squeezing below the minimum.
- **Density:** rows are `--lg-size-row` high and controls `--lg-size-control` high; prefer aligned
  rows and compact controls over whitespace-driven layouts.

---

## Radius, border and elevation

- **Radius:** `--lg-radius-sm` (4, inputs and small controls), `--lg-radius-md` (8, buttons and
  cards), `--lg-radius-lg` (12, panels and overlays), `--lg-radius-pill` (999, pills and switches).
  There is no upper cap.
- **Borders:** a one-pixel hairline (`--lg-size-hairline`) with `--lg-color-border`, or
  `--lg-color-border-strong` for emphasis.
- **Elevation is three steps:** `--lg-shadow-1` (rows and cards), `--lg-shadow-2` (popovers),
  `--lg-shadow-3` (modals). In the dark theme the shadows are deeper and a border separates the
  layers.
- **The layered model is `background → surface → raised → overlay`**, and no more than two levels
  are visible at once.

---

## Motion

- **Durations:** `--lg-motion-120` (120 ms), `--lg-motion-180` (180 ms), `--lg-motion-240`
  (240 ms); **easings** `--lg-ease-standard` and `--lg-ease-emphasized`.
- **Only these properties may transition:** `background`, `color`, `border-color`, `box-shadow`,
  `transform`, `opacity`.
- A hover state changes background or tint only — never layout, size or position.
- **`prefers-reduced-motion: reduce` zeroes non-essential animation** (transitions and animations
  are clamped to near-zero duration, scroll behaviour becomes instant).

---

## Component specifications

- **Shared primitives layer (`web/src/ui/`).** The sanctioned base layer holds exactly the five
  primitives — action (`button`), field (`field`), checkbox (`checkbox`), dialog (`dialog`) and
  notice (`toast`) — plus the focus trap (`focus_trap`). Every visual value comes from
  `--lg-*` tokens; no feature directory is imported (`UIP-02`). The class convention is
  `lg-<block>__<element>--<modifier>`.
- **Button.** Variants `primary` (accent fill with `--lg-color-accent-text`), `secondary`
  (surface), `ghost`, `outline` and `danger` (danger border and text). Sizes `sm`/`md`, minimum
  height 36–40 px, radius `sm`/`md`. States: `hover`, `active`, `focus-visible`, `disabled`,
  `loading`. A destructive button is never a one-click destroy and never a confirm-by-Enter
  default.
- **Fields.** Input, select, textarea, checkbox, radio, switch. A permanent label (never a
  placeholder as label), an inline error, and the design-token border and radius. Text and
  numeric inputs are right-aligned; numeric fields use tabular figures.
- **Data.** Tables and aligned rows over cards; tabular figures; a header row in muted text; a
  hairline divider between groups. No zebra striping.
- **Navigation.** A persistent left region opening with a brand lockup (sigil, tracked wordmark
  and tagline); each item pairs an icon with its label and closes with a chevron, and the current
  item is marked with the accent fill and `--lg-color-accent-text`; the footer hosts the
  icon-and-label session control and the theme switch (two end icons over a sliding thumb).
- **Dialog / overlay.** `--lg-shadow-3`, `role="dialog"`, `aria-modal`, `Esc` cancels and focus is
  trapped within. The scrim uses `--lg-color-overlay`.
- **Feedback.** Every async action shows progress and an inline result (success or a mapped error).
  Loading, empty and error states are designed, never blank.
- **Code field.** One editable field holds the whole code, presented as six
  single-digit positions that follow the field's value. The field keeps a
  permanent label, a numeric input mode and the platform's one-time-code
  autofill, so a paste, an autofill and a code with leading zeros land whole.
  The positions are presentational and never a second copy of the value.
- **Code surface.** The auth surface's second state: the origin line replaces
  the mode switch, and the blocks follow a fixed order — the origin line, the
  heading, the instruction, the code field, the refusal, the primary action, the
  resend row and the way back. The swap between the two surfaces is a fade and a
  small movement, at most the longest declared motion duration, and zeroed under
  `prefers-reduced-motion`.

---

## Accessibility

- **AA in both themes** for every declared foreground/background pair (body text 4.5:1, focus ring
  3:1), asserted from the token values.
- **Visible focus** on every focusable control, from `--lg-focus-width`, `--lg-focus-offset` and
  `--lg-color-focus`; a focused control is distinguishable from a hovered one. The focus ring is
  never removed without a replacement.
- **Keyboard-first:** tab order follows visual order, there are no keyboard traps, and `Esc`
  dismisses the topmost dismissible surface.
- **Native controls match the theme** through `color-scheme`.
- **Reduced motion** is honoured, and the theme control exposes an accessible name and its pressed
  state.

---

## Security posture

- **No DOM injection sinks:** `dangerouslySetInnerHTML`, `innerHTML`, `eval`, `new Function` and
  `document.write` are forbidden (ESLint fails the build on them). Data is rendered as text.
- **No third-party scripts, styles or fonts.** Everything is self-hosted and same-origin, so a
  strict Content-Security-Policy can be enforced later; the theme boot script is same-origin and
  classic, so `script-src 'self'` already permits it with no inline script and no policy hash.
- **No credentials in JavaScript-readable storage.** The theme preference may live in
  `localStorage` because it is not a credential; the refresh credential stays an `HttpOnly` cookie
  and the access token stays in memory.
- Every user-facing string resolves through `src/l10n/strings.ts`; all code, comments and copy are
  English.

---

## Per-screen patterns

- **Catalog / list.** A page title, a hairline-divided list or table, an empty state in muted text,
  and an inline retry on failure.
- **Detail / form.** A header with the record's identity, aligned labelled fields, a sticky footer
  bar anchored to the page end carrying the dirty state and the primary action, and inline
  validation.
- **Auth.** A centred panel on the page background; the mode switch uses the accent for the active
  mode; errors render inline, never as a toast alone.
- **Auth — the code step.** The same panel in its second state, with no route of its own: it names
  the address the code was sent to and how long the code remains valid, takes the code over one
  field, keeps the refusal against that field, and offers the resend with its countdown and one way
  back to the credential form.
- **States (loading / empty / error / not found).** Each is designed: a short line of plain text
  and, where there is an action, one control. No illustration, no demo data.
- **Overlays.** Creation flows, destructive confirmations and blocking errors use the one dialog
  pattern; everything else stays inline.

---

## Enforcement

- `src/test/tokens_used.test.tsx` forbids hexadecimal colours and pixel values in the scanned
  component directories and asserts every referenced token resolves. `src/theme/` is the documented
  exception where the literals live.
- `src/test/tokens_contrast.test.ts` resolves both themes and asserts every declared pair meets its
  minimum.
- `src/test/self_hosted_assets.test.ts` asserts the font is declared, preloaded and same-origin with
  its license, and that no theme source or document head reaches a third-party origin.
- The shell, responsiveness and browser suites assert the two-region shell and the no-third-region
  rule; `src/test/design_guidelines.test.ts` asserts this document carries its mandatory sections
  and that `AGENTS.md` restates no visual value.
- `npm test`, `npm run lint` and `npm run typecheck` must stay green; a behaviour change ships its
  tests in the same commit.

---

## Anti-patterns

| Avoid                                                                                | Instead                                                                  |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------ |
| Cards for everything                                                                 | Dividers + section headers; cards only for genuinely bounded objects     |
| Gradients, glow, glassmorphism / backdrop blur                                       | Flat surfaces, opaque surfaces with hairline borders                     |
| Heavy or decorative shadows                                                          | One of the three elevation steps                                         |
| A second accent, or the accent used decoratively                                     | One accent per theme, reserved to action / active / selection            |
| Hardcoded hex, pixel or pixel-font values in a component                             | A `--lg-*` token; add one in both themes if it is genuinely missing      |
| Body text or an accent label that fails AA                                           | Adjust the token until the both-theme assertion passes                   |
| Animation longer than 240 ms, or that ignores reduced motion                         | A declared duration and easing, zeroed under `prefers-reduced-motion`    |
| Large decorative headings, hero text, pill buttons, emoji icons, "AI SaaS" aesthetic | The declared type scale and the restrained component set                 |
| Floating action buttons, bottom bars, swipe-only actions                             | A persistent navigation and a fixed footer/toolbar action                |
| Decorative illustrations, including empty-state art                                  | One line of plain text                                                   |
| Icon overload or icon-only controls without an accessible name                       | Text labels; icons only for universally recognised actions, always named |
| Placeholder rows, lorem text or demo data                                            | A designed empty state                                                   |
