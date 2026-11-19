import { valueAtPath } from '../../../api/models/sheet';

export const HIT_POINTS_CURRENT = 'combat.hitPoints';
export const HIT_POINTS_MAX = 'combat.hitPointsMax';
export const HIT_POINTS_TEMP = 'combat.hitPointsTemp';

export const HIT_POINTS_CURRENT_MAX = 500;
export const HIT_POINTS_MAXIMUM_MAX = 500;
export const HIT_POINTS_TEMP_MAX = 100;

export interface HitPoints {
  readonly current: number;
  readonly max: number;
  readonly temp: number;
}

export interface HitPointsChange {
  readonly current: number;
  readonly temp: number;
}

export type HitPointsBand = 'high' | 'medium' | 'low';

const HIGH_MIN_PERCENT = 50;
const MEDIUM_MIN_PERCENT = 25;

function finiteOrZero(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function atLeastZero(value: number): number {
  return Math.max(0, finiteOrZero(value));
}

function isAmount(amount: number): boolean {
  return Number.isFinite(amount) && amount > 0;
}

export function hitPointsFrom(data: Record<string, unknown>): HitPoints {
  return {
    current: finiteOrZero(valueAtPath(data, HIT_POINTS_CURRENT)),
    max: finiteOrZero(valueAtPath(data, HIT_POINTS_MAX)),
    temp: finiteOrZero(valueAtPath(data, HIT_POINTS_TEMP)),
  };
}

export function applyDamage(hitPoints: HitPoints, amount: number): HitPointsChange | undefined {
  if (!isAmount(amount)) {
    return undefined;
  }
  const current = atLeastZero(hitPoints.current);
  const temp = atLeastZero(hitPoints.temp);
  const absorbed = Math.min(temp, amount);
  return {
    current: Math.max(0, current - (amount - absorbed)),
    temp: temp - absorbed,
  };
}

export function applyHeal(hitPoints: HitPoints, amount: number): HitPointsChange | undefined {
  if (!isAmount(amount)) {
    return undefined;
  }
  const max = atLeastZero(hitPoints.max);
  return {
    current: Math.min(max, atLeastZero(hitPoints.current) + amount),
    temp: atLeastZero(hitPoints.temp),
  };
}

export function hitPointsPercent(current: number, max: number): number {
  const safeMax = atLeastZero(max);
  if (safeMax === 0) {
    return 0;
  }
  return Math.min(100, Math.max(0, (atLeastZero(current) / safeMax) * 100));
}

export function hitPointsBand(percent: number): HitPointsBand {
  if (percent > HIGH_MIN_PERCENT) {
    return 'high';
  }
  if (percent >= MEDIUM_MIN_PERCENT) {
    return 'medium';
  }
  return 'low';
}

export function parseHitPointsAmount(text: string): number | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return undefined;
  }
  const amount = Number(trimmed);
  return isAmount(amount) ? amount : undefined;
}

function parseIntInRange(text: string, min: number, max: number): number | undefined {
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

export function parseHitPointsCurrent(text: string): number | undefined {
  return parseIntInRange(text, 0, HIT_POINTS_CURRENT_MAX);
}

export function parseHitPointsMaximum(text: string): number | undefined {
  return parseIntInRange(text, 1, HIT_POINTS_MAXIMUM_MAX);
}

export function parseHitPointsTemp(text: string): number | undefined {
  return parseIntInRange(text, 0, HIT_POINTS_TEMP_MAX);
}
