/**
 * Typed mirror of `tokens.css`. Tests read the CSS file and assert that these
 * declarations stay in sync, so the visual contract has a single source of
 * truth (UIF-01, UIF-02, UIF-28, UIF-31).
 */

/** The two declared themes; the closed set (`UIF-17`, `UIF-19`, `UIF-22`). */
export type ThemeName = 'light' | 'dark';

export interface ContrastPair {
  readonly description: string;
  /** The theme the pair is measured in (`R9`). */
  readonly theme: ThemeName;
  /** CSS custom property holding the foreground color. */
  readonly foregroundVar: string;
  /** CSS custom property holding the background the foreground sits on. */
  readonly backgroundVar: string;
  /** Minimum WCAG 2 contrast ratio the pair must meet. */
  readonly minRatio: number;
}

interface PairSpec {
  readonly description: string;
  readonly foregroundVar: string;
  readonly backgroundVar: string;
  readonly minRatio: number;
}

const pairSpecs: readonly PairSpec[] = [
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
    description: 'subtle text on background',
    foregroundVar: '--lg-color-text-subtle',
    backgroundVar: '--lg-color-bg',
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
    description: 'stage text on the stage ground',
    foregroundVar: '--lg-color-stage-text',
    backgroundVar: '--lg-color-stage',
    minRatio: 4.5,
  },
  {
    description: 'stage muted text on the stage ground',
    foregroundVar: '--lg-color-stage-text-muted',
    backgroundVar: '--lg-color-stage',
    minRatio: 4.5,
  },
  {
    description: 'stage text on the stage band',
    foregroundVar: '--lg-color-stage-text',
    backgroundVar: '--lg-color-stage-band',
    minRatio: 4.5,
  },
  {
    description: 'stage muted text on the stage band',
    foregroundVar: '--lg-color-stage-text-muted',
    backgroundVar: '--lg-color-stage-band',
    minRatio: 4.5,
  },
  {
    description: 'stage accent on the stage band',
    foregroundVar: '--lg-color-stage-accent',
    backgroundVar: '--lg-color-stage-band',
    minRatio: 4.5,
  },
  {
    description: 'stage accent on the stage raised layer',
    foregroundVar: '--lg-color-stage-accent',
    backgroundVar: '--lg-color-stage-raised',
    minRatio: 4.5,
  },
  {
    description: 'stage text on the stage raised layer',
    foregroundVar: '--lg-color-stage-text',
    backgroundVar: '--lg-color-stage-raised',
    minRatio: 4.5,
  },
  {
    description: 'stage muted text on the stage raised layer',
    foregroundVar: '--lg-color-stage-text-muted',
    backgroundVar: '--lg-color-stage-raised',
    minRatio: 4.5,
  },
  {
    description: 'stage accent label on the stage accent fill',
    foregroundVar: '--lg-color-stage-accent-text',
    backgroundVar: '--lg-color-stage-accent',
    minRatio: 4.5,
  },
  {
    description: 'stage accent label on the stage accent hover fill',
    foregroundVar: '--lg-color-stage-accent-text',
    backgroundVar: '--lg-color-stage-accent-hover',
    minRatio: 4.5,
  },
  {
    description: 'brand mark accent on the stage ground',
    foregroundVar: '--lg-color-stage-brand',
    backgroundVar: '--lg-color-stage',
    minRatio: 3,
  },
  {
    description: 'brand mark accent on the stage band',
    foregroundVar: '--lg-color-stage-brand',
    backgroundVar: '--lg-color-stage-band',
    minRatio: 3,
  },
  {
    description: 'stage focus ring on the stage ground',
    foregroundVar: '--lg-color-stage-focus',
    backgroundVar: '--lg-color-stage',
    minRatio: 3,
  },
  {
    description: 'stage danger on the stage band',
    foregroundVar: '--lg-color-stage-danger',
    backgroundVar: '--lg-color-stage-band',
    minRatio: 4.5,
  },
  {
    description: 'stage danger on the stage ground',
    foregroundVar: '--lg-color-stage-danger',
    backgroundVar: '--lg-color-stage',
    minRatio: 4.5,
  },
  {
    description: 'stage focus ring on the stage band',
    foregroundVar: '--lg-color-stage-focus',
    backgroundVar: '--lg-color-stage-band',
    minRatio: 3,
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

/**
 * Every color token used as a foreground on a background declares its pair here, once per
 * theme; `tokens_contrast.test.ts` asserts each pair meets `minRatio` in its own theme
 * (UIF-28, UIF-31).
 */
export const contrastPairs: readonly ContrastPair[] = (['light', 'dark'] as const).flatMap(
  (theme) => pairSpecs.map((spec) => ({ ...spec, theme })),
);

/** The layer a design token belongs to (data-model.md §1). */
export type TokenLayer = 'primitive' | 'semantic' | 'component';

/** Category of a design token; a closed set (data-model.md). */
export type TokenCategory =
  | 'type'
  | 'weight'
  | 'line'
  | 'font'
  | 'space'
  | 'radius'
  | 'size'
  | 'color'
  | 'shadow'
  | 'motion'
  | 'focus';

export interface DesignToken {
  /** CSS custom property name, e.g. `--lg-space-8`. */
  readonly name: string;
  readonly layer: TokenLayer;
  readonly category: TokenCategory;
  readonly purpose: string;
}

function group(
  layer: TokenLayer,
  category: TokenCategory,
  entries: readonly (readonly [string, string])[],
): DesignToken[] {
  return entries.map(([name, purpose]) => ({ name, layer, category, purpose }));
}

export const designTokens: readonly DesignToken[] = [
  ...group('primitive', 'font', [
    ['--lg-font-family', 'self-hosted family with a system fallback'],
  ]),
  ...group('primitive', 'type', [
    ['--lg-type-12', 'smallest supporting text'],
    ['--lg-type-13', 'secondary label / dense rows'],
    ['--lg-type-15', 'default body and control text'],
    ['--lg-type-16', 'emphasized body'],
    ['--lg-type-18', 'section subtitle'],
    ['--lg-type-20', 'section title'],
    ['--lg-type-24', 'page title'],
    ['--lg-type-30', 'display title, no upper cap'],
    ['--lg-type-display-md', 'landing section title, fluid'],
    ['--lg-type-display-lg', 'landing hero title, fluid'],
  ]),
  ...group('primitive', 'weight', [
    ['--lg-font-weight-regular', 'body text'],
    ['--lg-font-weight-medium', 'controls and active labels'],
    ['--lg-font-weight-semibold', 'section titles'],
    ['--lg-font-weight-bold', 'maximal emphasis'],
  ]),
  ...group('primitive', 'line', [
    ['--lg-line-tight', 'headings'],
    ['--lg-line-normal', 'body'],
    ['--lg-line-display', 'landing display titles'],
    ['--lg-tracking-display', 'landing display titles letter spacing'],
  ]),
  ...group('primitive', 'space', [
    ['--lg-space-2', 'hairline gap'],
    ['--lg-space-4', 'tight inline gap'],
    ['--lg-space-8', 'control padding and row gap'],
    ['--lg-space-12', 'group separation'],
    ['--lg-space-16', 'region padding'],
    ['--lg-space-24', 'section separation'],
    ['--lg-space-32', 'block separation'],
    ['--lg-space-48', 'page separation'],
    ['--lg-space-64', 'landing block separation'],
    ['--lg-space-96', 'landing section padding'],
    ['--lg-space-128', 'landing section padding, wide viewports'],
  ]),
  ...group('primitive', 'radius', [
    ['--lg-radius-sm', 'inputs and small controls'],
    ['--lg-radius-md', 'buttons and cards'],
    ['--lg-radius-lg', 'panels and overlays'],
    ['--lg-radius-pill', 'pills and switches'],
  ]),
  ...group('primitive', 'size', [
    ['--lg-size-hairline', 'hairline border width'],
    ['--lg-size-row', 'dense list row height'],
    ['--lg-size-control', 'control height'],
    ['--lg-size-nav-min', 'navigation minimum width'],
    ['--lg-size-nav-max', 'navigation maximum width'],
    ['--lg-size-content-min', 'content minimum width'],
    ['--lg-size-content-max', 'readable content measure'],
    ['--lg-size-dialog-max', 'dialog maximum width'],
    ['--lg-size-auth-max', 'auth form maximum width'],
    ['--lg-size-auth-panel', 'auth panel column width beside the photograph'],
    ['--lg-size-landing-max', 'landing content maximum width'],
    ['--lg-size-landing-hero-min', 'landing hero minimum height'],
    ['--lg-size-landing-hero-max', 'landing hero maximum height'],
  ]),
  ...group('primitive', 'color', [
    ['--lg-brand-orange-500', 'brand orange ramp: the mark accent and the app icon frame'],
  ]),
  ...group('primitive', 'focus', [
    ['--lg-focus-width', 'focus ring outline width'],
    ['--lg-focus-offset', 'focus ring offset'],
  ]),
  ...group('primitive', 'motion', [
    ['--lg-motion-120', 'fast transition'],
    ['--lg-motion-180', 'default transition'],
    ['--lg-motion-240', 'slow transition'],
    ['--lg-motion-600', 'landing entrance'],
    ['--lg-motion-1200', 'landing hero settle'],
    ['--lg-ease-standard', 'default easing'],
    ['--lg-ease-emphasized', 'overshooting easing'],
    ['--lg-ease-out', 'landing exponential ease-out'],
  ]),
  ...group('semantic', 'color', [
    ['--lg-color-bg', 'page background'],
    ['--lg-color-surface', 'card and region surface'],
    ['--lg-color-surface-hover', 'surface hover tint'],
    ['--lg-color-raised', 'raised layer above the surface'],
    ['--lg-color-inset', 'recessed well'],
    ['--lg-color-overlay', 'modal backdrop scrim'],
    ['--lg-color-text', 'body text'],
    ['--lg-color-text-muted', 'secondary text'],
    ['--lg-color-text-subtle', 'tertiary text'],
    ['--lg-color-border', 'default hairline border'],
    ['--lg-color-border-strong', 'emphasized border'],
    ['--lg-color-accent', 'accent fill'],
    ['--lg-color-accent-hover', 'accent fill hover'],
    ['--lg-color-accent-text', 'text on an accent fill'],
    ['--lg-color-danger', 'danger fill'],
    ['--lg-color-danger-hover', 'danger fill hover'],
    ['--lg-color-danger-text', 'text on a danger fill'],
    ['--lg-color-info', 'informational state'],
    ['--lg-color-success', 'success state'],
    ['--lg-color-warning', 'warning state'],
    ['--lg-color-focus', 'focus ring'],
    ['--lg-color-ink', 'ink surface (toasts)'],
    ['--lg-color-stage', 'landing teal-black ground, identical in both themes'],
    ['--lg-color-stage-brand', 'brand orange on the stage ground: the BrandMark accent core'],
    ['--lg-color-stage-band', 'landing alternate band on the dark ground'],
    ['--lg-color-stage-raised', 'landing raised layer on the dark ground'],
    ['--lg-color-stage-scrim', 'landing scrim over the hero photograph'],
    ['--lg-color-stage-text', 'text on the landing dark ground'],
    ['--lg-color-stage-text-muted', 'secondary text on the landing dark ground'],
    ['--lg-color-stage-line', 'hairline on the landing dark ground'],
    ['--lg-color-stage-accent', 'accent fill on the landing dark ground'],
    ['--lg-color-stage-accent-hover', 'accent fill hover on the landing dark ground'],
    ['--lg-color-stage-accent-text', 'text on the landing accent fill'],
    ['--lg-color-stage-focus', 'focus ring on the landing dark ground'],
    ['--lg-color-stage-danger', 'error text and border on the landing dark ground'],
  ]),
  ...group('semantic', 'shadow', [
    ['--lg-shadow-1', 'elevation for rows and cards'],
    ['--lg-shadow-2', 'elevation for popovers'],
    ['--lg-shadow-3', 'elevation for modals'],
  ]),
];
