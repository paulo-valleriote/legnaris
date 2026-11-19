import { clampPercent, parseClampedInt, UNBOUNDED } from './parse_number';

export const XP_PATH = 'identity.xp';

export const LEVEL_PATH = 'identity.level';

export const XP_LEVEL_MIN = 1;

export const XP_LEVEL_MAX = 20;

export const XP_THRESHOLDS: readonly number[] = [
  0, 300, 900, 2700, 6500, 14000, 23000, 34000, 48000, 64000, 85000, 100000, 120000, 140000, 165000,
  195000, 225000, 265000, 305000, 355000,
];

export interface XpProgress {
  readonly level: number;
  readonly xp: number;
  readonly floor: number;
  readonly nextThreshold: number | null;
  readonly nextLevel: number | null;
  readonly value: number;
  readonly percent: number;
  readonly canLevelUp: boolean;
}

function finiteNumber(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function levelNumber(level: unknown): number {
  const finite = finiteNumber(level);
  if (finite === undefined) {
    return XP_LEVEL_MIN;
  }
  return Math.min(XP_LEVEL_MAX, Math.max(XP_LEVEL_MIN, Math.trunc(finite)));
}

export function levelThreshold(level: unknown): number {
  return XP_THRESHOLDS[levelNumber(level) - 1];
}

export function nextThreshold(level: unknown): number | null {
  const current = levelNumber(level);
  return current >= XP_LEVEL_MAX ? null : XP_THRESHOLDS[current];
}

export function nextLevel(level: unknown): number | null {
  return nextThreshold(level) === null ? null : levelNumber(level) + 1;
}

export function xpTotal(value: unknown): number {
  const finite = finiteNumber(value);
  return finite === undefined ? 0 : Math.max(0, finite);
}

export function xpProgress(level: unknown, xp: unknown): XpProgress {
  const shownLevel = levelNumber(level);
  const total = xpTotal(xp);
  const floor = levelThreshold(shownLevel);
  const threshold = nextThreshold(shownLevel);
  const value = threshold === null ? floor : Math.min(threshold, Math.max(floor, total));
  const percent =
    threshold === null || threshold <= floor
      ? 100
      : clampPercent(((value - floor) / (threshold - floor)) * 100);

  return {
    level: shownLevel,
    xp: total,
    floor,
    nextThreshold: threshold,
    nextLevel: nextLevel(shownLevel),
    value,
    percent,
    canLevelUp: threshold !== null && total >= threshold,
  };
}

export function parseXpGain(text: string): number | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return undefined;
  }
  const amount = Number(trimmed);
  return Number.isFinite(amount) && amount > 0 ? amount : undefined;
}

export function parseXpTotal(text: string): number | undefined {
  return parseClampedInt(text, 0, UNBOUNDED);
}
