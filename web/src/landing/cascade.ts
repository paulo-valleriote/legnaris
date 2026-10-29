/**
 * The worked example behind the landing's recalculating sheet. The names and
 * formulas are invented for the demonstration and belong to no game system.
 */

export const MIGHT_MIN = 1;
export const MIGHT_MAX = 30;
export const MIGHT_DEFAULT = 14;

const CLIMBING_BONUS = 3;
const CARRY_PER_MIGHT = 7;

/** The values derived from one base score. */
export interface Cascade {
  readonly might: number;
  readonly modifier: number;
  readonly climbing: number;
  readonly carry: number;
}

/**
 * Brings any input onto the demo's whole-number range: a non-finite value falls back to the
 * default, a fraction rounds to the nearest whole number, and the result stays within bounds.
 */
export function clampMight(value: number): number {
  if (!Number.isFinite(value)) {
    return MIGHT_DEFAULT;
  }
  return Math.min(MIGHT_MAX, Math.max(MIGHT_MIN, Math.round(value)));
}

/** Derives every dependent value from the base score, always through `clampMight`. */
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

/** Formats a bonus with an explicit sign, using a true minus so it aligns with the plus. */
export function formatSigned(value: number): string {
  return value < 0 ? `−${Math.abs(value)}` : `+${value}`;
}
