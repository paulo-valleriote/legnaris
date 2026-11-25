function lastSegment(path: string): string {
  const index = path.lastIndexOf('.');
  return index < 0 ? path : path.slice(index + 1);
}

export function rowDisplayName(label: string, path: string): string {
  const trimmed = label.trim();
  const words = trimmed.split(/\s+/);
  if (words.length < 2) {
    return trimmed;
  }
  const suffix = lastSegment(path).toLowerCase();
  const last = words[words.length - 1].toLowerCase();
  return last === suffix ? words.slice(0, -1).join(' ') : trimmed;
}

export function formatSigned(value: number): string {
  if (!Number.isFinite(value)) {
    return '';
  }
  return value < 0 ? `\u2212${Math.abs(value)}` : `+${value}`;
}

export function parseScore(text: string): number | undefined {
  const trimmed = text.trim();
  if (trimmed === '') {
    return undefined;
  }
  const value = Number(trimmed);
  return Number.isFinite(value) && Number.isInteger(value) ? value : undefined;
}

export function isProficient(value: unknown): boolean {
  return typeof value === 'number' && Number.isFinite(value) && value !== 0;
}

export function nextProficiency(value: unknown): 0 | 1 {
  return isProficient(value) ? 0 : 1;
}

export type RankProficiency = 'none' | 'proficient' | 'expertise';

export function rankProficiency(value: unknown): RankProficiency {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 'none';
  }
  const rank = Math.trunc(value);
  if (rank >= 2) {
    return 'expertise';
  }
  if (rank >= 1) {
    return 'proficient';
  }
  return 'none';
}

export function rankLimit(max: number): number {
  return Number.isFinite(max) ? Math.max(0, Math.trunc(max)) : 0;
}

export function rankValue(value: unknown, limit: number): number {
  if (!Number.isFinite(limit) || limit <= 0) {
    return 0;
  }
  const rank = typeof value === 'number' && Number.isFinite(value) ? Math.trunc(value) : 0;
  return Math.min(limit, Math.max(0, rank));
}

export function nextRank(current: number, limit: number): number {
  if (!Number.isFinite(limit) || limit <= 0) {
    return 0;
  }
  const rank = Math.min(limit, Math.max(0, Math.trunc(current)));
  return rank >= limit ? 0 : rank + 1;
}
