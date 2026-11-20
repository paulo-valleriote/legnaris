import { valueAtPath } from '../../../api/models/sheet';
import { clampPercent, parseClampedInt, UNBOUNDED } from './parse_number';

export const HIT_POINTS_CURRENT = 'combat.hitPoints';
export const HIT_POINTS_MAX = 'combat.hitPointsMax';
export const HIT_POINTS_TEMP = 'combat.hitPointsTemp';

export type HitPointsBand = 'high' | 'medium' | 'low';
export type HitPointsDirection = 'damage' | 'heal';

export interface HitPointsPreviewLine {
  readonly before: number;
  readonly after: number;
  readonly delta: number;
}

export interface HitPointsPreview {
  readonly direction: HitPointsDirection;
  readonly current: HitPointsPreviewLine | undefined;
  readonly temp: HitPointsPreviewLine | undefined;
}

export interface HitPoints {
  readonly current: number;
  readonly max: number;
  readonly temp: number;
}

export interface HitPointsChange {
  readonly current: number;
  readonly temp: number;
}

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

export function applyTemp(hitPoints: HitPoints, amount: number): HitPointsChange | undefined {
  if (!isAmount(amount)) {
    return undefined;
  }
  return {
    current: hitPoints.current,
    temp: Math.max(atLeastZero(hitPoints.temp), amount),
  };
}

function previewLine(before: number, after: number): HitPointsPreviewLine | undefined {
  const delta = after - before;
  return delta === 0 ? undefined : { before, after, delta };
}

export function hitPointsPreview(
  stored: HitPoints,
  preview: HitPoints,
): HitPointsPreview | undefined {
  const current = previewLine(stored.current, preview.current);
  const temp = previewLine(stored.temp, preview.temp);
  if (current === undefined && temp === undefined) {
    return undefined;
  }
  const direction: HitPointsDirection =
    (current?.delta ?? 0) < 0 || (current === undefined && (temp?.delta ?? 0) < 0)
      ? 'damage'
      : 'heal';
  return { direction, current, temp };
}

export function formatHitPointsDelta(delta: number): string {
  if (delta === 0) {
    return '0';
  }
  return `${delta > 0 ? '+' : '\u2212'}${Math.abs(delta)}`;
}

export function hitPointsPercent(current: number, max: number): number {
  const safeMax = atLeastZero(max);
  if (safeMax === 0) {
    return 0;
  }
  return clampPercent((atLeastZero(current) / safeMax) * 100);
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
  const amount = parseClampedInt(text, 0, UNBOUNDED);
  return amount === undefined || amount <= 0 ? undefined : amount;
}

export function parseHitPointsCurrent(text: string): number | undefined {
  return parseClampedInt(text, 0, UNBOUNDED);
}

export function parseHitPointsMaximum(text: string): number | undefined {
  return parseClampedInt(text, 1, UNBOUNDED);
}

export function parseHitPointsTemp(text: string): number | undefined {
  return parseClampedInt(text, 0, UNBOUNDED);
}
