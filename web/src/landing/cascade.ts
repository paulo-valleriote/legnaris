export const MIGHT_MIN = 1;
export const MIGHT_MAX = 30;
export const MIGHT_DEFAULT = 14;

const CLIMBING_BONUS = 3;
const CARRY_PER_MIGHT = 7;

export interface Cascade {
  readonly might: number;
  readonly modifier: number;
  readonly climbing: number;
  readonly carry: number;
}

export function clampMight(value: number): number {
  if (!Number.isFinite(value)) {
    return MIGHT_DEFAULT;
  }
  return Math.min(MIGHT_MAX, Math.max(MIGHT_MIN, Math.round(value)));
}

export function deriveCascade(might: number): Cascade {
  const base = clampMight(might);
  const modifier = Math.floor((base - 10) / 2);
  return {
    might: base,
    modifier,
    climbing: modifier + CLIMBING_BONUS,
    carry: base * CARRY_PER_MIGHT,
  };
}

export function formatSigned(value: number): string {
  return value < 0 ? `−${Math.abs(value)}` : `+${value}`;
}
