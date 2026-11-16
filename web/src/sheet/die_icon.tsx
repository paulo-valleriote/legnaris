import type { Expression } from '../dice/parse';

export type DieShape = 'd4' | 'd6' | 'd8' | 'd10' | 'd12' | 'd20';

export const DEFAULT_DIE_SHAPE: DieShape = 'd20';

const SHAPE_BY_FACES: ReadonlyMap<number, DieShape> = new Map([
  [4, 'd4'],
  [6, 'd6'],
  [8, 'd8'],
  [10, 'd10'],
  [100, 'd10'],
  [12, 'd12'],
  [20, 'd20'],
]);

export function dieShapeForFaces(faces: number): DieShape {
  return SHAPE_BY_FACES.get(faces) ?? DEFAULT_DIE_SHAPE;
}

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
