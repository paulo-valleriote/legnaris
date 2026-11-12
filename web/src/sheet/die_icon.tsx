/**
 * The die icons of the Dice controls: one bare outline per common polyhedral die, drawn on the
 * same 24-unit grid and stroke as the rest of the Dice icon set so they read as one family.
 * Any die without its own silhouette, and any control that names no die, falls back to the d20.
 */

import type { Expression } from '../dice/parse';

/** The silhouettes the icon set draws. */
export type DieShape = 'd4' | 'd6' | 'd8' | 'd10' | 'd12' | 'd20';

/** The shape drawn when no die is known or the die has no silhouette of its own. */
export const DEFAULT_DIE_SHAPE: DieShape = 'd20';

/** A d100 is rolled as a pair of d10s at the table, so it borrows the d10 silhouette. */
const SHAPE_BY_FACES: ReadonlyMap<number, DieShape> = new Map([
  [4, 'd4'],
  [6, 'd6'],
  [8, 'd8'],
  [10, 'd10'],
  [100, 'd10'],
  [12, 'd12'],
  [20, 'd20'],
]);

/** The silhouette for a die with [faces] sides, or the d20 for an uncommon die. */
export function dieShapeForFaces(faces: number): DieShape {
  return SHAPE_BY_FACES.get(faces) ?? DEFAULT_DIE_SHAPE;
}

/**
 * The silhouette for an expression: its leading die, since that is the die the player reads
 * first in the button's summary; an expression with no dice draws the default.
 */
export function dieShapeFor(expression: Expression | undefined): DieShape {
  const leading = expression?.terms.find((term) => term.kind === 'dice');
  return leading === undefined ? DEFAULT_DIE_SHAPE : dieShapeForFaces(leading.faces);
}

function DieOutline({ shape }: { readonly shape: DieShape }) {
  switch (shape) {
    case 'd4':
      return <path d="M12 3.5 21 19.5H3z" />;
    case 'd6':
      return <rect x="3.5" y="3.5" width="17" height="17" rx="3.5" />;
    case 'd8':
      return <path d="M12 2.5 21 12l-9 9.5L3 12z" />;
    case 'd10':
      return <path d="M12 2.5 20.5 8.5 12 21.5 3.5 8.5z" />;
    case 'd12':
      return <path d="M12 3 21 9.6l-3.4 10.6H6.4L3 9.6z" />;
    case 'd20':
      return <path d="M12 2.5 20.2 7.25v9.5L12 21.5l-8.2-4.75v-9.5z" />;
  }
}

/**
 * A die drawn on `currentColor` in the silhouette of [shape]; `off` strikes it through for a
 * Row whose dice the player turned off.
 */
export function DieIcon({
  shape = DEFAULT_DIE_SHAPE,
  off = false,
}: {
  readonly shape?: DieShape;
  readonly off?: boolean;
}) {
  return (
    <svg
      className="lg-dice-icon"
      viewBox="0 0 24 24"
      aria-hidden="true"
      focusable="false"
      data-die-shape={shape}
    >
      <DieOutline shape={shape} />
      {off && <path d="M3 21 21 3" />}
    </svg>
  );
}
