export const UNBOUNDED = Number.POSITIVE_INFINITY;

export function parseClampedInt(text: string, min: number, max: number): number | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return undefined;
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) {
    return undefined;
  }
  const value = Math.min(max, Math.max(min, Math.round(parsed)));
  return Object.is(value, -0) ? 0 : value;
}

export function clampPercent(value: number): number {
  return Math.min(100, Math.max(0, value));
}
