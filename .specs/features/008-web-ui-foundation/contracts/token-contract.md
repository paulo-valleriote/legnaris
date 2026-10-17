# Contract: Design Tokens

**Applies to**: every client screen rendered under `web/src/`.
**Requirements**: UIF-01 … UIF-13, UIF-28 … UIF-31.
**Supersedes**: the tokens section of `001-web-pwa-foundation/contracts/shell-layout.contract.md`
(the 12/13/15/17 scale with a 17 px cap, the 2/4 px radii and the "elevation 0" rule). The panel
clause of that contract is superseded by `shell-two-region.contract.md`.

## Source of truth

- Literal values live **only** in `web/src/theme/tokens.css`; `web/src/theme/tokens.ts` mirrors them
  for the tests.
- A component MUST NOT contain a literal colour or pixel value. Enforced by
  `src/test/tokens_used.test.tsx`, which scans `src/shell/`, `src/catalog/`, `src/engine/`,
  `src/sheet/`, `src/library/` and `src/auth/` and treats `src/theme/` as the documented exception.
- Every `var(--lg-*)` a scanned component references MUST resolve in `tokens.css`.
- Adding a token means: declare it in **both** themes, mirror it in `tokens.ts`, and declare its
  contrast pair when it is a foreground — then run `npm test`.

## Layers

| Layer | Prefix | Rule |
|---|---|---|
| Primitive | `--lg-ink-*`, `--lg-amber-*`, `--lg-danger-*`, `--lg-space-*`, `--lg-radius-*`, `--lg-font-*`, `--lg-motion-*`, `--lg-ease-*` | raw scales, no role |
| Semantic | `--lg-color-*`, `--lg-shadow-*`, `--lg-focus-*` | the roles components consume, declared per theme |
| Component | any other `--lg-*` | only where no semantic covers the need |

## Declared scales

| Group | Values |
|---|---|
| Type | `--lg-type-12/13/15/16/18/20/24/30` — no upper cap below the largest step |
| Weight | `--lg-font-weight-regular/medium/semibold/bold` (400/500/600/700) |
| Line height | `--lg-line-tight` (1.2) · `--lg-line-normal` (1.5) |
| Space | `--lg-space-2/4/8/12/16/24/32/48` |
| Radius | `--lg-radius-sm` (4) · `md` (8) · `lg` (12) · `pill` (999) |
| Border | `--lg-size-hairline` (1px) + `--lg-color-border` / `--lg-color-border-strong` |
| Elevation | `--lg-shadow-1` (lines/cards) · `--lg-shadow-2` (popover) · `--lg-shadow-3` (modal) |
| Motion | `--lg-motion-120/180/240` · `--lg-ease-standard` · `--lg-ease-emphasized` |
| Layout | `--lg-size-row` · `--lg-size-control` · `--lg-size-nav-min/max` · `--lg-size-content-min/max` |

## Themes

| Theme | Selector | Colour scheme |
|---|---|---|
| Light (parchment) | `:root` | `light` |
| Dark (charcoal + amber) | `[data-theme='dark']` | `dark` |

- Every semantic token MUST resolve in both themes; a token declared in one theme only is a defect
  and MUST fail the both-theme assertion.
- Exactly one accent per theme, reserved to action / active state / selection; an accent fill always
  pairs with `--lg-color-accent-text`.
- The layered model is `bg → surface → raised → overlay`; no more than two levels are visible at
  once.

## Motion

- Transitions animate only `background`, `color`, `border-color`, `box-shadow`, `transform`,
  `opacity`, using a declared duration and easing.
- Under `prefers-reduced-motion: reduce`, non-essential animation is zeroed.

## Accessibility

- Every foreground/background pair the interface uses declares its minimum ratio per theme
  (`contrastPairs` in `tokens.ts`); body text 4.5:1, the focus ring 3:1.
- `:focus-visible` uses `--lg-focus-width`/`--lg-focus-offset`/`--lg-color-focus` and is never
  removed without a replacement.

## Verification

| Check | Method |
|---|---|
| No literal values in components | `tokens_used.test.tsx` source scan |
| Every referenced token resolves | `tokens_used.test.tsx` / `tokens_contrast.test.ts` token map |
| Every semantic resolves in both themes | `tokens_files.ts` per-theme parser + `tokens_contrast.test.ts` |
| Declared pairs meet AA per theme | `tokens_contrast.test.ts` over `contrastPairs` |
| Reduced motion | `base.css` media block; rendering assertion in the theme suite |
| Font is self-hosted and preloaded | `self_hosted_assets.test.ts` |
