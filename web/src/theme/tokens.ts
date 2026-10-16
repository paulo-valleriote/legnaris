/**
 * Typed mirror of `tokens.css`. Tests read the CSS file and assert that these
 * declarations stay in sync, so the visual contract has a single source of
 * truth (FND-02, FND-05).
 */

/** The width below which the contextual panel collapses first (FND-03). */
export const SHELL_NARROW_QUERY = '(max-width: 900px)';

export interface ContrastPair {
  readonly description: string;
  /** CSS custom property holding the foreground color. */
  readonly foregroundVar: string;
  /** CSS custom property holding the background the foreground sits on. */
  readonly backgroundVar: string;
  /** Minimum WCAG 2 contrast ratio the pair must meet. */
  readonly minRatio: number;
}

/**
 * Every color token used as a foreground on a background declares its pair
 * here; `tokens_contrast.test.ts` asserts each pair meets `minRatio` (FND-05).
 */
export const contrastPairs: readonly ContrastPair[] = [
  {
    description: 'body text on background',
    foregroundVar: '--lg-color-text',
    backgroundVar: '--lg-color-bg',
    minRatio: 4.5,
  },
  {
    description: 'body text on surface',
    foregroundVar: '--lg-color-text',
    backgroundVar: '--lg-color-surface',
    minRatio: 4.5,
  },
  {
    description: 'muted text on background',
    foregroundVar: '--lg-color-text-muted',
    backgroundVar: '--lg-color-bg',
    minRatio: 4.5,
  },
  {
    description: 'muted text on surface',
    foregroundVar: '--lg-color-text-muted',
    backgroundVar: '--lg-color-surface',
    minRatio: 4.5,
  },
  {
    description: 'accent label on accent fill',
    foregroundVar: '--lg-color-accent-text',
    backgroundVar: '--lg-color-accent',
    minRatio: 4.5,
  },
  {
    description: 'accent label on accent hover fill',
    foregroundVar: '--lg-color-accent-text',
    backgroundVar: '--lg-color-accent-hover',
    minRatio: 4.5,
  },
  {
    description: 'danger label on danger fill',
    foregroundVar: '--lg-color-danger-text',
    backgroundVar: '--lg-color-danger',
    minRatio: 4.5,
  },
  {
    description: 'focus ring on background',
    foregroundVar: '--lg-color-focus',
    backgroundVar: '--lg-color-bg',
    minRatio: 3,
  },
  {
    description: 'focus ring on surface',
    foregroundVar: '--lg-color-focus',
    backgroundVar: '--lg-color-surface',
    minRatio: 3,
  },
];

/** Category of a design token; a closed set (data-model.md). */
export type TokenCategory = 'type' | 'space' | 'radius' | 'size' | 'color' | 'focus';

export interface DesignToken {
  /** CSS custom property name, e.g. `--lg-space-8`. */
  readonly name: string;
  readonly category: TokenCategory;
  readonly purpose: string;
}

export const designTokens: readonly DesignToken[] = [
  { name: '--lg-type-12', category: 'type', purpose: 'smallest supporting text' },
  { name: '--lg-type-13', category: 'type', purpose: 'secondary label / dense rows' },
  { name: '--lg-type-15', category: 'type', purpose: 'default body and control text' },
  { name: '--lg-type-17', category: 'type', purpose: 'section title, hard cap' },
  { name: '--lg-space-4', category: 'space', purpose: 'tight inline gap' },
  { name: '--lg-space-8', category: 'space', purpose: 'control padding and row gap' },
  { name: '--lg-space-12', category: 'space', purpose: 'group separation' },
  { name: '--lg-space-16', category: 'space', purpose: 'region padding' },
  { name: '--lg-space-24', category: 'space', purpose: 'section separation' },
  { name: '--lg-radius-2', category: 'radius', purpose: 'inputs and buttons' },
  { name: '--lg-radius-4', category: 'radius', purpose: 'panels and dialogs' },
  { name: '--lg-size-hairline', category: 'size', purpose: 'hairline border width' },
  { name: '--lg-size-row', category: 'size', purpose: 'dense list row height' },
  { name: '--lg-size-control', category: 'size', purpose: 'control height' },
  { name: '--lg-size-nav-min', category: 'size', purpose: 'navigation minimum width' },
  { name: '--lg-size-nav-max', category: 'size', purpose: 'navigation maximum width' },
  { name: '--lg-size-content-max', category: 'size', purpose: 'readable content measure' },
  { name: '--lg-size-panel-min', category: 'size', purpose: 'panel minimum width' },
  { name: '--lg-size-panel-max', category: 'size', purpose: 'panel maximum width' },
  { name: '--lg-focus-width', category: 'focus', purpose: 'focus ring outline width' },
  { name: '--lg-focus-offset', category: 'focus', purpose: 'focus ring offset' },
];
