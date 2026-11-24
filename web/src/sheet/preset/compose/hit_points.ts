import { numberValue } from '../values';

export interface PresetHitPoints {
  readonly current: number;
  readonly max: number;
  readonly temp: number;
}

export interface PresetHitPointsChange {
  readonly current: number;
  readonly temp: number;
}

const MAX_PERCENT = 100;

function whole(value: number): number {
  return Math.trunc(value);
}

function atLeastZero(value: number): number {
  return Math.max(0, whole(value));
}

function isAmount(amount: number | undefined): amount is number {
  return amount !== undefined && Number.isFinite(amount) && amount > 0;
}

export function parseHitPointsAmount(text: string): number | undefined {
  const amount = Number(text.trim());
  if (text.trim() === '' || !Number.isFinite(amount) || !Number.isInteger(amount) || amount <= 0) {
    return undefined;
  }
  return amount;
}

export function parseHitPointsValue(text: string): number | undefined {
  const value = Number(text.trim());
  if (text.trim() === '' || !Number.isFinite(value) || !Number.isInteger(value) || value < 0) {
    return undefined;
  }
  return value;
}

export function hitPointValue(value: unknown): number {
  const number = numberValue(value);
  return number === undefined ? 0 : atLeastZero(number);
}

export function applyDamage(
  hitPoints: PresetHitPoints,
  amount: number | undefined,
): PresetHitPointsChange | undefined {
  if (!isAmount(amount)) {
    return undefined;
  }
  const absorbed = Math.min(hitPoints.temp, amount);
  return {
    current: Math.max(0, hitPoints.current - (amount - absorbed)),
    temp: hitPoints.temp - absorbed,
  };
}

export function applyHeal(
  hitPoints: PresetHitPoints,
  amount: number | undefined,
): PresetHitPointsChange | undefined {
  if (!isAmount(amount)) {
    return undefined;
  }
  return {
    current: Math.min(hitPoints.max, hitPoints.current + amount),
    temp: hitPoints.temp,
  };
}

export function hitPointsPercent(current: number, max: number): number {
  if (!Number.isFinite(max) || max <= 0) {
    return 0;
  }
  return Math.min(MAX_PERCENT, Math.max(0, (atLeastZero(current) / max) * MAX_PERCENT));
}
