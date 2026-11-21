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

## Colour (Colors)

Two themes, one accent each.

- **Light — parchment.** Page `#EFE6D4`, surface `#FBF6EA`, inset `#E4D8BF`; text `#2A2118`,
  muted `#6B5B47`, subtle `#726046`; border `#D6C9AF`, border-strong `#B9A886`; accent burnt amber
  `#B45309` (hover `#92400E`), on-accent `#FFFFFF`; danger brick `#A83232`.
- **Dark — charcoal plus amber.** Page `#17181B`, surface `#212327`, inset `#141518`; text
  `#ECE9E4`, muted `#A8A29B`, subtle `#948D85`; border `#33363B`, border-strong `#4A4E54`; accent
  `#F59E0B` (hover `#FBBF24`), on-accent `#201A10`; danger `#F87171`.

Rules:

- **One accent per theme.** The accent marks an action, an active state, a selection or a Preview mark — never
  decoration, and never a derived value. Whenever the accent fills a surface, the text on it comes from
  `--lg-color-accent-text`, never a bare white or black.
- Amber text on parchment must use the darker amber steps, not the accent fill, so it clears AA.
  Accent-coloured text and links read `--lg-color-link` (the darker amber on parchment, a bright
  amber on charcoal) with `--lg-color-link-hover` for hover; both are asserted at the body-text
  ratio on the page and on the surface layer in both themes and are remapped to the stage accent
  inside the shell. The accent fill itself is never used for text.
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
- **Scale:** `11 / 12 / 13 / 15 / 16 / 18 / 20 / 24 / 30` px (`--lg-type-*`). There is no upper
  cap below the largest step; headings are allowed to exceed body size. Eleven is the desktop floor
  for a dense label; a phone label starts at twelve.
- **Weights:** 400 regular, 500 medium, 600 semibold, 700 bold.
- **Line height:** `--lg-line-tight` (1.2) for headings, `--lg-line-normal` (1.5) for body.
- **Tabular figures** (`font-variant-numeric: tabular-nums`) on every numeric or derived column so
  values align vertically.
- **Tracking.** `--lg-tracking-tight` for compact headings that need a slightly closed fit,
  `--lg-tracking-display` for display and page titles, `--lg-tracking-caps` (uppercase codes),
  `--lg-tracking-wide` (uppercase group labels) and `--lg-tracking-widest` (uppercase wordmarks);
  text links use `--lg-underline-offset`. A component never spells an `em` letter-spacing literal.

### Heading scale

One scale, one weight each, used by every screen and by the shared primitives:

- **Page title** — `--lg-type-30` semibold with `--lg-line-display` (`PageHeader`).
- **Section heading (h2)** — `--lg-type-20` semibold.
- **List row or card item name** — `--lg-type-18` semibold.
- **Dialog title** — `--lg-type-20` semibold.
- **Field label** — `--lg-type-13` medium.

No screen invents its own heading size; a smaller voice is a step of this list, not a new size.

---

## Spacing and layout

- **Space scale:** `2 / 4 / 8 / 12 / 16 / 24 / 32 / 48` px (`--lg-space-*`). Every gap, pad and
  margin is a step of this scale.
- **The shell is two regions: navigation then content.** There is no contextual panel. On every
  route the shell exposes exactly a named `nav` and a `main`, in that order.
- **Navigation is persistent from 640 px:** at 640 px (`40rem`) and wider it is a left column fixed
  between `--lg-size-nav-min` and `--lg-size-nav-max` (200–240 px). The desktop column never becomes
  a drawer or an overlay. Its surface and trailing hairline paint the full page height, while a
  viewport-high sticky rail keeps the links and Sign out in place on a page taller than the viewport.
- **Below 640 px the navigation is a compact top bar.** The same `nav` becomes a single row above
  the content: the brand mark, then a `--lg-size-control` menu button at the trailing edge
  (`aria-expanded`, `aria-controls`). Its destinations (Catalog, Your sheet, Plans, Campaigns,
  Account and Sign out) open in one floating panel on the surface layer, `--lg-space-16` in from
  each edge, just under the bar, with `--lg-radius-md`, `--lg-shadow-2` and a `70dvh` scroll cap.
  The panel is the only overlay the shell draws; the bar stays a grid row above the content, so
  nothing overlaps until the panel opens. The current item keeps the raised layer and an accent
  underline, labels are text only, and every control keeps the `--lg-size-control` height. Opening
  moves focus into the panel; Escape or a tap outside closes it and returns focus to the button.
  The breakpoint is `40rem` because a media query cannot read a token and the literal scan forbids
  pixel values.
- **Content keeps a minimum width** (`--lg-size-content-min`, 320 px) and a readable measure
  (`--lg-size-content-max`, 960 px, centred). From 640 px up, as the viewport narrows, the
  navigation stays put and the content reaches its minimum without ever overlapping the navigation
  — it scrolls instead of squeezing below the minimum. Below 640 px the minimum no longer applies:
  the content takes the full width on `--lg-space-16` gutters, the sheet editor and its sticky
  footer included, and the page never scrolls horizontally down to a 320 px viewport.
- **Density:** rows are `--lg-size-row` high and controls `--lg-size-control` high; prefer aligned
  rows and compact controls over whitespace-driven layouts. A pointer-width action that must stay
  denser without losing its touch target elsewhere uses `--lg-size-control-compact` (40 px) and
  returns to `--lg-size-control` (44 px) at phone widths.
- **Narrow single-column screens.** Join and privacy cap their column at `--lg-size-narrow-max`
  rather than a one-off width, so the two flows keep one measure.

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
  (240 ms); **easing** `--lg-ease-standard`. The landing's one entrance
  adds `--lg-motion-600` and `--lg-motion-1200` with `--lg-ease-out` (a steep exponential
  ease-out); `--lg-ease-out` stays declared for that surface and is not a second default.
- **Only these properties may transition:** `background`, `color`, `border-color`, `box-shadow`,
  `transform`, `opacity`.
- A hover state changes background or tint only — never layout, size or position. No shared or
  screen control moves, scales or translates on hover.
- **The reduced-motion rule is declared once**, in `src/theme/base.css`: under
  `prefers-reduced-motion: reduce` every animation and transition is clamped to an instant state
  change (delays removed, a single iteration), scroll behaviour becomes instant, and the spinners
  in the shared layer stop turning and draw a static ring. Feature stylesheets do not add their own
  copy of the rule.

---

## Component specifications (Components)

- **Shared primitives layer (`web/src/ui/`).** The sanctioned base layer holds the five primitives —
  action (`button`), field (`field`), checkbox (`checkbox`), dialog (`dialog`) and notice (`toast`),
  now joined by the `Notice` strip, the `Select` control, the state primitives
  (`EmptyState`/`ErrorState`/`LoadingState`), `PageHeader` and the one `icons` module — plus the
  focus trap (`focus_trap`). Every visual value comes from `--lg-*` tokens; no feature directory is
  imported. The class convention is `lg-<block>__<element>--<modifier>`.
  **All buttons use `ui/Button`, all modals use `ui/Dialog`, and every icon comes from
  `ui/icons`.** A screen does not build its own button, overlay or glyph.
- **Button.** Variants `primary` (accent fill with `--lg-color-accent-text`), `secondary`
  (surface), `ghost`, `outline`, `danger` (danger border and text) and `row` (a full-width list-row
  entry: left-aligned content, surface layer, surface-hover tint, never a position change). Sizes
  `sm`/`md`, minimum height 36–40 px, radius `sm`/`md`. Props: `variant`, `size`, `type`
  (`button` by default, `submit` opt-in), `label`, `name`, `children` (rendered instead of `label`),
  `icon` (leading, `aria-hidden`), `trailingIcon` (after the content, `aria-hidden`, pushed to the
  trailing edge of a row entry), `className`, `fullWidth`, `id`, `title`, `ariaDescribedBy`,
  `ariaExpanded`, `ariaControls`, `ariaPressed`, `ariaHasPopup`, `disabled`, `loading`,
  `disabledReason` and `onClick`, which receives the React mouse event. The component is
  `forwardRef`, so a caller reaches the underlying element for focus. Link mode: `to` renders a
  router `Link` and `href` a plain anchor, with the identical classes, variants, sizes and aria
  state, so a middle click and "open in a new tab" work; `disabled`, `loading` and
  `disabledReason` are button-only and are ignored by a link. States: `hover`, `active`,
  `focus-visible`, `disabled`, `loading`. Loading draws a real border spinner
  (`.lg-button__spinner`, a declared motion token, a static ring under reduced motion) beside the
  label with `aria-busy`, never a glyph. A disabled action that needs a reason passes
  `disabledReason`: the button stays focusable with `aria-disabled`, suppresses `onClick`, and a
  small muted line is linked through `aria-describedby` instead of hiding the reason in a `title`.
  A destructive button is never a one-click destroy and never a confirm-by-Enter default.
- **Fields.** Input, select, textarea, checkbox, radio, switch. A permanent label (never a
  placeholder as label), an inline error, and the design-token border and radius. `Field` renders a
  `.lg-form-field` wrapper holding the `<label>` (its only accessible-name source), the control, the hint
  and the error as siblings, so no message inflates the name; the hint, any caller-supplied
  `describedBy` ids and the error are linked through `aria-describedby` in that order and the error
  is never inside the label. The error is a polite live region (`aria-live="polite"`), so a new
  message is announced without the double announcement a `role="alert"` would add when focus moves.
  Props: `label`, `id`, `type` (`text`/`password`/`email`/`number`), `name`, `value`, `placeholder`,
  `hint`, `error`, `describedBy`, `disabled`, `readOnly`, `numeric`, `required`, `multiline`, `rows`
  (the textarea form, same label/error contract and `.lg-form-field__textarea` styling), `autoComplete`,
  `inputMode`, `maxLength`, `pattern`, `autoFocus`, `inputRef`, `onChange`, `onBlur`, `trailing` (an
  inline control such as a reveal-password button). `Select` carries the same contract with
  `options`/`children` and `describedBy`. Controls sit on `--lg-color-raised` with a
  `--lg-color-border-strong` hairline and `--lg-radius-sm`; focus is the global focus ring, never
  only a border colour. Text and numeric inputs are right-aligned; numeric fields use tabular
  figures (via a class, never an inline style). An empty text field shows
  the muted "Not set" as its placeholder (`--lg-color-text-muted`, full opacity, so it clears AA): a
  hint about the value, never the label, which stays permanent and is the field's accessible name.
  A textarea reuses `.lg-form-field__input` (`.lg-form-field__textarea`) without a second contract.
- **Data.** Tables and aligned rows over cards; tabular figures; a header row in muted text; a
  hairline divider between groups. No zebra striping.
- **Navigation.** A persistent left region opening with a brand lockup (sigil, tracked wordmark
  and tagline); each item pairs an icon with its label and closes with a chevron, and the current
  item is marked with the raised layer and an accent icon; the footer hosts the icon-and-label
  session control. Below 640 px it is the compact top bar described under "Spacing and layout":
  the brand mark and one menu button, with the destinations and the session control in a floating
  panel on the surface layer. Its controls are `--lg-size-control` high at every width and pointer. There is no theme switch: the signed-in app is always
  dark.
- **Dialog / overlay.** `--lg-shadow-3` on the surface layer, `--lg-radius-lg`, the
  `--lg-color-overlay` scrim with an inset of `--lg-space-16`, so a full-height dialog never touches
  the viewport edge, a fade-and-small-rise entrance from the declared motion tokens (zeroed under
  reduced motion), `role="dialog"`, `aria-modal`, `aria-labelledby` pointing at the title element
  (never a duplicated `aria-label`), `Esc` cancels and focus is trapped within.
  `ui/Dialog` is **the single dialog**: props `open`, `title`, `titleId` (the id the title gets, and
  therefore the id a child region may name itself by; generated when absent), `description`,
  `onClose`, `children`, `tone` (`default`/`danger`), `actions` (a `.lg-dialog__actions` footer of
  secondary/primary buttons), `closeLabel` (a named ghost icon close button from `ui/icons`),
  `initialFocusRef`, `error` (a `role="alert"` danger line above the actions) and `className` (an
  extra class on the panel). Dialogs form one module-level stack: only the topmost open dialog reacts
  to `Esc`, so a document opened over a consent dialog closes alone. Focus returns to the element
  that opened the dialog when it closes or unmounts, and a parent that recreates `onClose` on every
  render never re-runs the focus effect or steals focus.
- **Select.** `ui/Select` renders a native `<select>` with the `Field` label/hint/error contract and
  the identical control styling (`.lg-form-field__input lg-form-field__select`); it is a field, not a menu.
- **Notice strip.** A standing, non-blocking condition the person should act on (a payment issue,
  an exhausted generation quota, a generation still running from an earlier visit) is one flat
  strip inline in the flow: the surface layer, a `--lg-size-hairline` leading edge in the state's
  system tone (`--lg-color-warning` for a problem, `--lg-color-info` for information, danger and
  success for their own conditions), one line of `--lg-type-15` text and at most one way forward, an
  accent link or a secondary button. `ui/Notice` takes `tone`, an optional `surface` (`app` by
  default, `stage` for the landing's world, where the strip sits on the stage band with the stage
  text roles and `--lg-color-stage-info` for its information edge), `children`, an optional `action`,
  `live` (`polite` renders `role="status"`), `id` and `className`. It is never a toast, never
  dismissible while the condition holds, and the payment strip above the signed-in content carries
  `role="status"`.
- **States.** `ui/EmptyState`, `ui/ErrorState` and `ui/LoadingState` are the designed states of every
  async region; each accepts `id` and `className` on its root. An empty state is **one line of muted
  text** plus, where it applies, one secondary action — never a dashed box, a card, a border or
  illustration. An error state is the mapped copy in the danger tone with `role="alert"` and one
  secondary Retry (`common.retry`, "Try again") only when retrying is offered. A loading state is
  muted text plus the shared spinner with `aria-busy`.
- **Page header.** `ui/PageHeader` is the one page title block: the `h1` at `--lg-type-30` semibold
  on `--lg-line-display`, an optional ghost icon back link on a `--lg-size-control` target named by
  `backLabel` (the `common.back` "Back" fallback), an optional meta row (pill or caption) and
  right-aligned actions that wrap under the title below `40rem`. It accepts `id` and `className` on
  the header and `titleId` on the `h1`, so a region outside the header can name itself by the page
  title. The back link is a router `Link`.
- **Icons.** One module, `ui/icons`, exports the named glyphs (`ChevronRight`, `ChevronLeft`/`Back`,
  `ChevronDown`, `Close`, `Plus`, `Minus`, `Pencil`, `ArrowRight`/`Open`, `Check`, `Dice`, `Menu`).
  Every glyph is a 24-unit viewBox, stroke `1.6`, `currentColor`, `aria-hidden` and sized in `em`
  (or a token), so colour and size come from the text it sits in. An icon-only control is always
  named by `aria-label`.
- **Feedback.** Every async action shows progress and an inline result (success or a mapped error).
  Loading, empty and error states are designed, never blank.
- **Badge.** A short status label on a bounded object (the current plan): `--lg-type-12` semibold
  on the accent fill with `--lg-color-accent-text`, `--lg-radius-pill`. A badge is never a control;
  the pill shape stays banned for buttons.
- **Document dialog.** Long reading content opened from a flow (the privacy policy) uses the one
  dialog pattern, built on `ui/Dialog`, at `--lg-size-dialog-max`: a hairline-divided header with the
  title and a ghost icon close button on a `--lg-size-control` target, and a body that scrolls on its
  own, contains its overscroll, is named by the dialog title through `aria-labelledby` and takes the
  focus ring when focused for keyboard scrolling. The panel's shape comes from the `className` the
  document passes to `ui/Dialog`, never from a selector over the dialog's descendants.
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

## Landing surface

The public page a visitor without a session sees on `/` is the one surface where the app's
restraint is relaxed, because its job is to persuade, not to operate. The exceptions below apply
to `src/landing/` and to the auth surface in `src/auth/`, which the landing's actions lead into and
which therefore keeps the landing's world. The signed-in app shares that world through the
semantic layer (see "Signed-in app" below) but keeps the rules above, so it takes no photograph and
no display-scale heading.

- **One photograph, in the hero and on the auth surface.** A self-hosted WebP under `public/images/` in two widths
  (`srcset`), decorative (`alt=""`), never decoding a third-party origin. A scrim built from
  `--lg-color-stage` and `--lg-color-stage-scrim` keeps the type legible over it; the scrim and
  the top-to-bottom fade are the only gradients the product allows.
- **Stage roles.** The whole page sits on the `--lg-color-stage-*` roles, declared in both themes
  with the same values: a teal-black ground (the colour of the hero photograph's rock), an
  alternate band for the game moments and the questions, a raised layer for the moment cards and
  the contextual action, a lava orange accent sampled from the photograph's lava, a danger tone
  for refusals
  (`--lg-color-stage-danger`) and an information tone for an informational notice edge
  (`--lg-color-stage-info`, asserted on the stage band). The landing is therefore always dark, whatever
  the theme, and carries no theme control. The accent is used for the primary action, the sigil's
  core, the step numerals, the derived values of the step illustrations and the flash a moment
  action replays.
- **Display type.** Geist 600 at `--lg-type-display-lg` (hero) and `--lg-type-display-md`
  (section titles), `--lg-line-display` and `--lg-tracking-display`.
- **Hero demo.** The static hero copy (one H1, the lead and the sign-up action) renders first;
  below it a `Suspense` fallback reserves the stage and the real Fantasy Play preset mounts as a
  lazy chunk over the same in-memory sample character. Its character name is the demo's heading at
  level 2, under the page's one H1. A first edit or roll reveals the contextual sign-up action,
  and the demo's own live regions announce the changed values and dice results. A sticky sign-up
  action sits at the viewport foot below `40rem` and is withdrawn above it.
- **Rhythm.** Section padding uses `--lg-space-96` and, from the wide breakpoint, `--lg-space-128`.
  Content is capped at `--lg-size-landing-max`. Rows and questions are hairline-divided, never
  cards; the bounded objects are the demo sheet's own cards, the three game-moment cards and the
  contextual action on `--lg-color-stage-raised`. Each step carries a small sheet-visual
  illustration: a template picker, an ability tile with its derived modifier, and a hit-point bar
  with slot pips.
- **Proof, not claims.** The hero demo and the game moments mount the product's real Play
  components over invented values, with no network request; the moments are exactly level-up,
  damage and a spent slot, each with one action and a polite result line. The page carries no
  testimonial, statistic, price or customer claim.
- **Motion.** One authored entrance: the photograph settles (`--lg-motion-1200`) and the hero
  content rises (`--lg-motion-600`, `--lg-ease-out`), staggered by `--lg-motion-120` steps.
  A game-moment action replays a short accent flash on the card and the real control's own value
  transition. The flash is not drawn when the browser prefers reduced motion. Everything is
  zeroed under `prefers-reduced-motion` and content is visible by default.
- **Actions.** "Create your character sheet" leads to the Fantasy start intent and "Sign in" to
  `/sign-in`; both are links. Every conversion action (hero, contextual, sticky, close) carries the
  same start intent. Buttons keep `--lg-radius-md`, never a pill.
- **Accessibility.** Landmarks are `header`, `main` and `footer`; the questions are native
  `details` disclosures; each game moment is a named article with one polite result line, and its
  controls are the real sheet controls or a labelled button; the step illustrations are
  decorative. The stage focus ring is `--lg-color-stage-focus`; the stage text pairs are
  asserted in both themes like every other pair. The landing mounts no ad slot.

---

## Signed-in app

While the shell or the landing is mounted, `:root:has(.lg-shell), :root:has(.lg-landing)` in
`tokens.css` remaps the semantic layer onto the stage roles, always dark whatever the stored or
system theme: page and inset are the stage ground, surface is the stage band (the navigation,
dialogs, the sheet footer, the landing demo's cards), raised and surface-hover are the stage raised
layer, text pairs are the stage text roles, border is the stage line, and the accent is the lava
orange with `--lg-color-stage-accent-text` on its fill. Components keep reading semantics, so no
route, dialog, portaled panel or control carries its own colour for this world.

- **Same ground, quieter voice.** The app is an operating surface: page titles use
  `--lg-type-30` with `--lg-tracking-display` and `--lg-line-display`, list-row and card names use
  `--lg-type-18` semibold per the heading scale; there is no photograph and no section-scale display
  type.
- **Rows, not boxes.** Catalog and own-sheet entries are hairline-divided rows; the trailing arrow
  and the accent mark hover and focus only.
- **Navigation.** The current item sits on the raised layer with the accent on its icon; the
  accent fill is reserved for actions.
- **Browser chrome.** The shell paints `theme-color` in the stage ground, like the landing. The web
  manifest's `theme_color` and `background_color` and both `<meta name="theme-color">` entries carry
  that same stage ground (`--lg-color-stage`, teal-950 `#061516`). The BrandMark core takes
  `--lg-color-stage-brand` (brand orange `#F2692A`, the app icon's frame), which is asserted at 3:1 on the
  stage ground and the stage band.
- **Contrast.** The remap uses only stage pairs already asserted in `tokens.ts`, plus
  `--lg-color-border-strong`, which carries no text.

### Sheet surface

The sheet is read at the table, so it is built from bounded units rather than a stack of labels
and inputs (`src/engine/engine.css`, `src/sheet/sheet.css`; tokens only, none added).

- **Section card.** A Section is the one bounded object: `--lg-color-surface` with a hairline
  border, `--lg-radius-lg` and `--lg-shadow-1`, opening with its title at `--lg-type-20`
  semibold and a hairline. Rows are never cards: they are hairline-divided lines inside it.
  In the play view the title row is a disclosure control that folds the Section's Rows away (see
  "Sheet navigation"); in Arrange mode it is plain text beside the handle.
- **Row.** Every Row (lone field, exclusive group, table row) is one full-width line of at least
  `--lg-size-control`, label left and control right on the shared 2:3 split. A table row stacks
  its label above its cells in a narrow container.
- **Editable versus derived.** An editable field is a raised, bordered control
  (`--lg-color-raised`, `--lg-color-border-strong`). A derived field is a recessed readout on
  `--lg-color-bg` with no border, its value in `--lg-color-text` and bold, so it reads as a result
  and never as an input. A derived value is never the accent: the accent is for actions, pins,
  selection and marks, and about forty orange results would out-shout Save.
  The recessed look carries that meaning alone: there is no per-Row
  "Calculated" caption, in a lone Row or a table cell alike (a table of readouts would
  repeat the word on every line: 12 times in Skills). The word stays as a visually hidden
  description (`aria-describedby`, unique id per readout), so assistive technology reads it
  unchanged, and the sheet's `Help` explains the recessed readout once. A marked readout carries the
  "Recalculated" pill instead and no description. A marked readout keeps the accent on its border and
  pill only; its value stays text colour. A derived row's label is muted.
- **Phone tap targets.** Play controls stay at least `--lg-size-control` (44 px) square at phone
  widths, however compact their drawing. A save or skill row still draws at 36 px tall: its
  proficiency dot, rollable total and base-bonus edit keep the full 44 px target on a 44 px row
  pitch (an 8 px row gap), so the list stays dense without shrinking what the finger hits and
  adjacent targets never overlap. Attack to-hit and damage pills and the attack edit pencils, the
  spell-slot pips (whose drawn diamond stays compact) and the armor-class and initiative edit
  targets are the same 44 px.
- **Rollable totals (Play).** A rollable total reads in `--lg-color-text` with a dashed
  `--lg-color-link` underline; hover and focus fill it with `--lg-color-accent` and
  `--lg-color-accent-text`. Filled accent stays for the proficiency dot and the primary action, so
  "proficient" and "rollable" never share a shape. A d20 rollable takes a roll mode: Shift-click is
  advantage, Alt-click is disadvantage, and a long press or the Context Menu key / Shift+F10 opens one
  small dialog with Normal, Advantage and Disadvantage as full-width secondary buttons (the first
  takes focus). A non-d20 roll ignores all of it. Read-only Play text (the at-will and prepared
  spell lists) draws no input edge until hover or focus, while Edit keeps the generic input
  affordance. Initiative reads as a roll before hover: a decorative d20 sits beside its signed total
  at all times, named `Roll initiative, <signed value>`, and its ghost pencil waits for hover or
  focus on a desktop, is a full `--lg-size-control` target on a phone and is named
  `Edit initiative`. No Play label renders below `--lg-type-11`, or below `--lg-type-12` on a phone.
- **Derived beside its input.** An editable number Row whose value directly feeds one or two derived
  leaves (`schema.dependsOn`, direct edges only; none or three and more draw nothing; a `config.*`
  switch, a text or a table cell never does) carries a second line inside the same Row, under the
  control column's right edge: each dependent's full label (`--lg-type-13`, `--lg-color-text-muted`,
  one line, ellipsis, full label as title) and its value (`--lg-type-15` semibold,
  `--lg-color-text`, tabular). No caption, no arrow and no border. While the input has been edited and
  no Preview has recalculated it the value is stale: `--lg-color-text-muted` and slanted (the weight
  is unchanged, so the line never changes width or height), with a visually hidden "Not recalculated
  yet". A Preview or a Save settles it with the merged cascade; the derived value keeps its own Row,
  marks and `data-row-key`. The input is described by the list (`aria-describedby`), the list is
  never a live region, and it is not drawn while arranging. No token is added: the dim state reuses
  `--lg-color-text-muted`.
- **Numbers lead.** Numeric values are `--lg-type-20` semibold (derived `--lg-type-24` bold),
  right-aligned with tabular figures.
- **Hit point quick actions.** The hit-points card puts a strip of common changes (`−5`, `−1`, `+1`,
  `+5`, `+ Temp`) above the amount field and Damage/Heal: ghost-weight `sm` buttons on a hairline
  border with tabular figures, the damage pair in `--lg-color-danger-400` and the heal pair in
  `--lg-success-400`. Each is named by its effect ("Damage 5"), and `+ Temp` stays disabled until the
  amount is positive. They sit on `--lg-size-control-compact` on a desktop and a
  `--lg-size-control` square on a phone, so a touch target never falls under 44 px.
- **Compact vitals bar.** On a phone, once the character combat strip scrolls out of view, a fixed
  bar about `--lg-space-48` tall under the top safe-area inset holds AC, the current and maximum hit
  points with a 4 px bar, and one `Damage or heal` control on a `--lg-size-control` target; it hides
  the moment the strip returns. Opening it reveals the amount field and the Damage/Heal actions on
  the same `--lg-size-control` targets and through the same pending flow as the card. A pending
  change adds one `--lg-type-12` line under the row (`10 → 6 (−4)`) in the danger or success tone,
  with its hatched segment on the bar. It slides in with `--lg-motion-180` and drops the motion
  under reduced motion. The page reserves its height with `scroll-padding-block-start`, so a focused
  field scrolls clear of it rather than under it. The stat block's vitals band is the same promise
  by other means: it is sticky under the safe-area inset at every width.
- **Booleans are switches.** A toggle is a pill track (`--lg-radius-pill`) with a knob drawn on
  the native checkbox: accent track and `--lg-color-accent-text` knob when on, the knob position
  carrying the state as well as the colour. The touch area is at least `--lg-size-control` high: the native input itself is that tall, its track
  drawn in the padding box inside a transparent border so the visible switch stays the same size.
- **Preview mark.** A marked control keeps the `--lg-color-surface` tint and gains an accent
  border; the row's "Modified" / "Recalculated" label is a small outlined pill. The pill sits on the
  Row's top hairline, out of the label's flow, so marking a Row never wraps its label or changes its
  height.
- **Heading.** Above the sheet's name a small muted line names the template ("Fantasy Template"),
  falling back to "Sheet" when the template has no name; a long name wraps. Preview (secondary) and
  Save (primary, the accent fill) are both `md` buttons in the sticky footer; `Arrange` stays a
  small outline button in the heading, so the footer carries the weight.
- **Readout names.** A derived readout in a lone Row is named by that Row's visible label through
  `aria-labelledby`, so the name has one source; a table cell, which has no Row label of its own,
  keeps its `aria-label`.
- **Footer and preview summary.** The sticky footer is at most two rows: one status line, then
  Preview and Save. The status line merges the unsaved state and the last Preview's cascade into one
  phrase in `--lg-color-text` (`--lg-type-15` medium): "Unsaved changes" before a Preview, then
  "Unsaved, 4 recalculated", "Unsaved, 1 recalculated", "Unsaved, none recalculated"; with nothing
  unsaved a leftover Preview reads "4 values recalculated" / "No values recalculated". After a Preview
  the Jump control sits on the same line at the trailing edge, shown only when something recalculated:
  a ghost `sm` button (text colour, underlined, `--lg-space-32` high, 24 px minimum target; a deliberate
  step down from the 44 px actions, since it is a text-weight shortcut and not an action of the
  footer). With nothing to say the status block is empty and takes no room, so an error slot (the
  preview and save alerts; an offline one is the muted `--lg-color-text-muted` tone, not danger, since the player did nothing wrong) appears under the line without moving the actions. On a
  phone the actions share the second row equally (`--lg-size-control` high, Preview the `secondary`
  surface button, Save the `primary` accent fill; a disabled Save keeps the accent, dimmed, instead of
  the flat global disabled style, so an idle pair is never two identical outlines); from `40rem` the
  status sits left and the actions right on one row. Measured: 61 px idle, 87.5 px with unsaved edits, 97 px
  after a Preview at 390 and 320 wide (budget 104; before, about 150), 69 px at 1280.
  The jump scrolls the first recalculated Row to the centre (instantly under `prefers-reduced-motion`),
  clear of the footer and top bar through `scroll-margin-block`, and focuses it with the shared focus
  ring. One visually hidden polite live region announces the cascade ("4 values recalculated"); the
  visible phrase is hidden from assistive technology while a Preview shows, so it is read once, and the
  Jump button stays outside that hidden phrase. An edit, a Save or a discard clears the summary exactly
  as it clears the marks.

- **Header.** The sheet name is the heading (`--lg-type-30` semibold) with the template name under it
  in `--lg-type-15` muted: no kicker above the heading. The actions sit at the trailing edge as one
  bounded group (hairline border, `--lg-radius-md`) of ghost text buttons divided by hairlines, each
  on a `--lg-size-control` target: `Help`, `Arrange` (pinning is part of Arrange mode). None is primary, because none is: Save stays the only one. Below
  `40rem` the group wraps under the name and takes the content width edge to edge, its segments
  sharing it equally with centred labels (two for a character's `Play mode` / `Edit mode`, three
  with `Duplicate` on an NPC or enemy); in Play the hero leads and the group follows it, so the
  sheet title starts within the first 80 px of a 390 px viewport. From `40rem` the group stays
  compact and trailing. `Collapse all` is not a header action: it sits at the trailing edge of the
  sticky section strip, next to the links it acts on. There is no vitals strip and no card of pinned
  numbers above the Sections: pinned Rows rise to the top of their own Section.

  **Help.** The ghost `Help` button in the header group (`aria-expanded`) opens a quiet note under the
  heading: a hairline-bordered `--lg-radius-md` box on `--lg-color-surface` with two `--lg-type-13`
  muted sentences and a close button (an × icon on a `--lg-size-control` target, named `Got it`) at
  its trailing edge: calculated numbers are flat and borderless and are worked out from the rows you
  can edit; Preview shows what edits change without saving, Save keeps them. No modal, tour,
  spotlight or colour. It opens by itself once, until it is closed (or `Help` is pressed again),
  which is remembered per viewer and sheet.

### Arrange mode

Arrange mode (`src/sheet/sheet.css`, `src/engine/engine.css`; tokens only, none added) is the one
mode where structure is edited; playing the sheet never shows any of it. It offers exactly two
controls: a handle on each Section and a star on each Row.

- **Entry.** A small outline `Arrange` button at the trailing edge of the sheet heading. In the
  mode it disappears, a short muted hint (two lines on a phone) takes focus under the title, and the sticky footer is
  replaced by `Reset to default` (outline), `Cancel` (secondary) and `Done` (the one primary
  action), with the unsaved-arrangement indicator and any commit failure in the status area
  (`--lg-color-text-muted` and `--lg-color-danger`, as for values).
- **Compact rows.** While arranging, a Row is a hairline-divided line of at least
  `--lg-size-control`: its label, and the star at the trailing edge. Controls and table headers are
  not drawn, so no value is edited by accident and a long sheet stays short. A Section keeps its
  card and opens with its handle and title.
- **Section handle.** A grip glyph in `--lg-color-text-muted` on a `--lg-size-control` target
  (`--lg-color-text` and `--lg-color-surface-hover` on hover, the shared focus ring on focus), with
  `touch-action: none` so a touch drag does not scroll. It is the only way to reorder Sections;
  Rows are never reordered and there are no Move up / Move down buttons.
- **Pin star.** An icon button on the same `--lg-size-control` target, drawn as an inline SVG on
  `currentColor`: an outline star in `--lg-color-text-muted` when the Row is not pinned, a filled
  star in `--lg-color-accent` when it is (the state is carried by shape as well as colour). It is a
  toggle named "Pin <label>" whose name never changes (`aria-pressed` carries the state), keeps focus
  after toggling, and uses the shared focus ring.
- **Pinned Rows.** A pinned Row rises to the top of its own Section, in the order the Rows were
  pinned, and the others keep their Template order below it. There is no page-level pinned zone, no
  caption and no extra Section, and no limit on how many Rows are pinned; a Section whose Rows are
  all pinned renders like any other. Unpinning returns the Row to its Template position. Outside Arrange mode
  the toggle is absent and a pinned Row carries a small filled `--lg-color-accent` star right after its label (a picture
  plus a visually hidden "Pinned"), before any preview mark, so a pinned Row reads as pinned beyond
  being first.
  An optional pinned filter can show only the pinned Rows (see "Sheet navigation"): it is a filter
  over the same Sections, each keeping its own card and title, never a zone, a caption or an extra
  Section, and a Row is still drawn once.
- **Dragging.** The dragged Section takes `--lg-color-raised` and `--lg-shadow-2`; the Sections it
  displaces slide with `--lg-motion-180` and `--lg-ease-standard` (transform only). Reduced motion
  removes the slide, the handle and star colour transitions and the keyboard drag's smooth
  scrolling.
- **Announcements.** The drag library's live region speaks every pick-up, move, drop and cancel
  ("Alpha, position 2 of 3"); a visually hidden polite live region speaks every pin and unpin.

### Sheet navigation

Getting around a long sheet while playing (`src/sheet/section_index.*`, `src/sheet/sheet_view.tsx`,
`src/engine/engine.css`; tokens only, none added). Nothing here edits the sheet: it is view state.

- **The strip.** One sticky row at the top of the scrolling content region, the same on a phone and
  on a desktop: `--lg-size-control` high on `--lg-color-surface` with a bottom hairline, its content
  at the readable measure and the page gutters. The narrow top bar is a grid row above the content
  region, never over it, so the strip sticks to the region's own top edge (offset zero); the editor
  carries the region's top padding so nothing slides above it. It is drawn for one or more Sections,
  not for none, and it stays during Arrange mode, following the staged order.
- **Pinned filter.** The leading control: a pinned-star glyph, a `aria-pressed` toggle on a
  `--lg-size-control` target, muted when off, on the raised layer with a filled accent star when on.
  Its name "Pinned only" is read at every width and drawn beside the star from `40rem`; below it the
  star stands alone.
- **Sheet name chip.** Right after the filter, the sheet's name in `--lg-type-13` semibold on
  `--lg-color-text`, ellipsised at `--lg-space-96` (`--lg-space-128` from `40rem`), a hairline on its
  trailing edge, named "<name>, back to top": a tap scrolls the content region to its start
  (instant under reduced motion) and leaves focus where it is, so identity stays in view once the
  heading has scrolled away. On, only the pinned Rows are drawn, in pin order, each Section that
  has any as its normal card and the others omitted; the links then list only the Sections drawn. An
  empty filter shows one line ("Nothing pinned yet. Pin the rows you check most and they will be
  here.") and an outline `Edit pins` button; with pins, `Edit pins` sits at the strip's trailing
  edge. Both enter Arrange mode. The filter is remembered per viewer and per sheet
  (see "Memory"), is disabled and reads off while arranging (leaving the mode returns to
  what the person left), and announces "Showing pinned rows, N" / "Showing all
  sections" through the sheet's one polite live region.
- **Section links.** After the chip, only with two or more Sections: text-only links in the
  arranged order, no fill and no pill, `--lg-color-text-muted` by default, the current one in
  `--lg-color-text` with a `--lg-space-2` accent underline and `aria-current="location"`. Labels
  ellipsise at `--lg-space-96`; the full label stays the accessible name. The row scrolls
  horizontally and keeps the current link in view. They are plain buttons in a `nav` named
  "Sections", never tabs: one tab stop, Left / Right / Home / End move between them, Enter and Space
  go. The current Section follows the scroll (an observer whose top margin is the strip's height; the
  last Section wins at the scroll end; with no observer none is marked). A link whose Section holds
  Rows marked by the last Preview carries a count dot: a `--lg-radius-pill` badge on
  `--lg-color-accent` with `--lg-color-accent-text` (the accent button's asserted pair),
  `--lg-type-12` semibold, tabular, the number of marked Rows drawn in that Section; the link's accessible name carries
  it ("Combat, 3 changed").
- **Going to a Section.** The Section lands under the strip with `--lg-space-8` of air (a scroll
  margin of the strip height plus that step), opens if it is collapsed or filtered out, and its title
  control takes focus. The scroll is smooth through the stylesheet and instant under
  `prefers-reduced-motion`.
- **Collapse.** In the play view the title row is `<h2><button aria-expanded aria-controls>` across
  the whole row at `--lg-size-control`: the label leading, a stroked chevron on `currentColor`
  trailing, turned with a `transform` over `--lg-motion-120` (no turn under reduced motion), with
  the shared focus ring and the `--lg-color-surface-hover` hover. A collapsed Section keeps its Rows
  mounted and hidden, so a typed value and every subscription survive. A collapsed Section holding
  preview marks shows an outlined "N changed" mark in its title row, the same mark as a Row's. One
  text button, "Collapse all" / "Expand all", sits beside `Arrange`. Everything is expanded by
  default. Collapse is unavailable while arranging and while the filter is on; its state is left
  untouched and comes back.
- **Memory.** Which Sections are collapsed, and whether the pinned filter is on, are remembered per viewer and per sheet, in the browser
  only. It is view state: it is never sent to the server, never part of the Layout or of an export,
  never unsaved work, and never triggers the leave guard; a failing or missing store falls back to
  everything expanded. The pinned filter is remembered the same way, in the same record (`onlyPinned`
  next to `collapsed`, record version 1; a record without it reads as off); a jump or reveal that
  has to turn the filter off turns it off for good. After a Preview the footer grows by the
  one status line, and Rows and Sections scroll clear of it and of the strip.

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
- **Reduced motion** is honoured.

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
- **Landing.** Public `/` for visitors; see the Landing surface section. Sign-in and registration
  live on `/sign-in` and `/sign-up`, and a signed-in person on either is sent to `/`.
- **Auth.** The landing's world, always dark: the photograph with the brand lockup on one side
  (a short band above the panel on a narrow viewport) and a flat panel of `--lg-size-auth-panel`
  beside it holding the title, the mode switch, the fields and the one primary action. The mode
  switch is two underlined tabs, and the active one carries the accent; the primary action is the
  stage accent fill; the lockup leads back to the landing. Errors render inline in the stage
  danger tone, never as a toast alone.
- **Auth — the code step.** The same panel in its second state, with no route of its own: it names
  the address the code was sent to and how long the code remains valid, takes the code over one
  field, keeps the refusal against that field, and offers the resend with its countdown and one way
  back to the credential form.
- **Settings rows (Account).** The page title, then hairline-divided rows of a muted label and its
  content (two columns from `40rem`, label `1fr` to content `3fr`; stacked below). A row leads with
  its value (`--lg-type-20` semibold for the plan name), may list counts in tabular figures, and
  holds its own actions and inline error. No cards.
- **Plans.** The one place cards are sanctioned in the signed-in app, because each plan is a
  genuinely bounded offer: surface layer, hairline border, `--lg-radius-lg`, `--lg-shadow-1`, one
  column, two from `60rem`. The current plan swaps its border to the accent and carries a Badge.
  The price is `--lg-type-24` bold in tabular figures with its period and note muted; the actions
  sit at the card's foot and fill the width below `40rem`. Refund lives below the cards behind a
  hairline, as plain text and one danger action, its outcome an inline status line. No comparison table, no "most popular" emphasis.
- **Generate.** A single-column flow (prompt, consent, review) with prose capped at `65ch`. The
  prompt fields sit on the raised layer with a `--lg-color-border-strong` hairline and
  `--lg-radius-sm`; a character counter in tabular figures trails the helper text and turns
  warning while the text is too short and danger once it is too long. A resumable generation is a
  Notice strip. Consent is a definition list of muted points under one intro, and the privacy
  policy opens in the Document dialog. On Free the AI entry carries a Pro badge and opens the
  upgrade dialog, which lists the paid limits as hairline-bordered rows with the AI allowance on
  the surface layer and an accent border.
- **Campaign and Join.** The Campaign page header pairs a ghost back arrow (a `--lg-size-control`
  target named "Back to your Campaigns") with the title and the role pill under it; the GM actions
  (`Invite players`, `Campaign settings`) sit at the trailing edge as the sheet header's hairline
  group of ghost buttons, the open one on the raised layer, and their panels open full width below.
  Below it the page is two columns from a `48rem` container (a single column under it, Members
  last): the Sheets lead, Members follow as a `15rem` sticky column of hairline-divided rows (name
  semibold, meta in `--lg-type-13` muted, actions wrapping under). Linked character sheets and
  NPCs/enemies are the second sanctioned card use, because each is a bounded Sheet: an auto-fill
  grid of `12rem` minimum, surface layer, hairline border, `--lg-radius-lg`, `--lg-shadow-1`; the
  name (`--lg-type-18` semibold) is the card's one link, stretched over the whole card, with the
  trailing accent arrow on hover and focus only; meta ("Played by …", kind and owner) is muted and
  the update date sits at the foot in `--lg-type-12` subtle tabular figures. An NPC card shows its
  visibility to players as a dot (success fill when visible, an outline when hidden) plus text,
  and its actions fill the card's foot. Section headings are `--lg-type-20`
  with a muted tabular count beside them (Members included); the NPC link form is one inline row capped at `32rem`.
  Join is the same page title over one form capped at `36rem`. Long names and invite codes wrap
  anywhere, never overflow.
- **Privacy (`/privacy`).** Public and outside the shell, so it follows the stored or system theme
  on the base semantic layer rather than the stage world: the page background, prose at
  `--lg-type-16` capped at `--lg-size-content-max`, accent links that wrap anywhere. Inside the
  app the same text opens in the Document dialog instead of navigating away.
- **States (loading / empty / error / not found).** Each is designed: a short line of plain text
  and, where there is an action, one control. No illustration, no demo data.
- **Overlays.** Creation flows, destructive confirmations, blocking errors, the dice expression
  editor and the roll-mode choice use the one dialog pattern; everything else stays inline.
- **Dice.** A Row's dice controls sit at the trailing edge of its label, in every Row kind, so they
  always sit right before the value; a label beside them keeps whole words. Roll is the action of the
  table and the only outlined control: raised fill, `--lg-color-border-strong` hairline (accent on
  hover and press), `--lg-radius-md`, an accent die drawn as a bare outline in the shape of the
  expression's leading die (d4, d6, d8, d10, d12 or d20; a d100 draws the d10; no leading die or
  an uncommon one draws the d20) and the dice it throws in semibold tabular
  figures (`d20`, `2d6+d4`; numbers and Fields are left to the result panel; the word `Roll` when it
  throws none). Its accessible name is the Row plus those dice (`Roll Initiative, d20`); a press scales
  it down slightly, zeroed under reduced motion. Edit is a ghost pencil icon on a `--lg-size-control`
  target, muted until hovered; on a Row whose dice are off it draws a struck-through die instead, the
  way back. A Section's dice button is the same ghost die icon at the title row's trailing edge,
  costing no line: muted while the Section has no expression of its own, text colour once it has one.
  Icon-only buttons carry their full name as `aria-label` and `title`. Below `40rem` the Row's
  controls drop under the label text, inside the label column, and the Roll drops its die, so a
  Row with dice keeps its value on the same guide as every other Row. The editor is the dialog: title, the Section note when editing a Section, the
  `Expression` field, the two quiet ghost alternatives (`Use the Template dice`, `No dice`), then a
  hairline and the commit pair (`Keep current dice` secondary, `Save dice` primary). The Roll result
  floats at the bottom of the viewport above the sticky footer or the Play pending bar
  (`--lg-shadow-2`, raised, a short rise-in zeroed under reduced motion), moves with that surface
  when it appears, grows or leaves, names the rolled Row beside its total and lists every die and
  number; it keeps the last three results of the session, newest first, separating them with a
  hairline (a fourth result drops off the oldest, and older totals step down to `--lg-type-18`).
  A roll made with advantage or disadvantage names the mode beside its total at
  `--lg-type-13` semibold, lists every thrown d20 in muted figures with the dropped one struck
  through and the kept one in text colour semibold, and reads the whole thing in `dice.keptRolls`.
  It never traps focus and closes on its × or Escape, which also clears the history and starts the
  next roll from an empty panel.

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

| Avoid                                                                                | Instead                                                                                                 |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------- |
| Cards for everything                                                                 | Dividers + section headers; cards only for genuinely bounded objects                                    |
| Gradients, glow, glassmorphism / backdrop blur                                       | Flat surfaces, opaque surfaces with hairline borders                                                    |
| Heavy or decorative shadows                                                          | One of the three elevation steps                                                                        |
| A second accent, or the accent used decoratively                                     | One accent per theme, reserved to action / active / selection                                           |
| Hardcoded hex, pixel or pixel-font values in a component                             | A `--lg-*` token; add one in both themes if it is genuinely missing                                     |
| Body text or an accent label that fails AA                                           | Adjust the token until the both-theme assertion passes                                                  |
| Animation longer than 240 ms, or that ignores reduced motion                         | A declared duration and easing, zeroed under `prefers-reduced-motion`                                   |
| Large decorative headings, hero text, pill buttons, emoji icons, "AI SaaS" aesthetic | The declared type scale and the restrained component set (the landing surface alone has display titles) |
| Floating action buttons, a bottom navigation bar, swipe-only actions                 | A persistent navigation (a top bar on a narrow viewport) and a fixed footer/toolbar action              |
| Decorative illustrations, including empty-state art                                  | One line of plain text                                                                                  |
| Icon overload or icon-only controls without an accessible name                       | Text labels; icons only for universally recognised actions, always named                                |
| Placeholder rows, lorem text or demo data                                            | A designed empty state                                                                                  |
