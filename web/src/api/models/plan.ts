import { drift, isPlainObject, optionalString, requireNumber, requireString } from '../parse';

export const SHEET_KINDS = ['character', 'npc', 'monster'] as const;

export type SheetKind = (typeof SHEET_KINDS)[number];

export interface SheetUsage {
  readonly used: number;
  readonly limit: number;
}

export interface PlanUsage {
  readonly plan: string;
  readonly sheets: Readonly<Partial<Record<SheetKind, SheetUsage>>>;
  readonly consentAt: string | null;
  readonly consentVersion: string | null;
  readonly trialRemaining: number | undefined;
  readonly llmConsentRequired: boolean;
  readonly lastFreeGeneration: boolean;
  readonly activeGenerationId: string | null;
}

function decodeSheetUsage(value: unknown, kind: string): SheetUsage {
  if (!isPlainObject(value)) {
    throw drift(`plan usage for "${kind}" is not an object`);
  }
  return { used: requireNumber(value, 'used'), limit: requireNumber(value, 'limit') };
}

export function parsePlanUsage(data: unknown): PlanUsage {
  if (!isPlainObject(data)) {
    throw drift('plan payload is not an object');
  }
  const rawSheets = data['sheets'];
  if (!isPlainObject(rawSheets)) {
    throw drift('plan payload is missing a sheets object');
  }

  const sheets: Partial<Record<SheetKind, SheetUsage>> = {};
  for (const kind of SHEET_KINDS) {
    const entry = rawSheets[kind];
    if (entry !== undefined) {
      sheets[kind] = decodeSheetUsage(entry, kind);
    }
  }

  return {
    plan: requireString(data, 'plan'),
    sheets,
    consentAt: decodeConsentAt(data),
    consentVersion: optionalString(data, 'llm_consent_version') ?? null,
    trialRemaining: decodeTrialRemaining(data),
    llmConsentRequired: decodeFlag(data, 'llm_consent_required'),
    lastFreeGeneration: decodeFlag(data, 'last_free_generation'),
    activeGenerationId: optionalString(data, 'active_generation_id') ?? null,
  };
}

function decodeTrialRemaining(data: Record<string, unknown>): number | undefined {
  const trial = data['trial'];
  if (trial === undefined || trial === null) {
    return undefined;
  }
  if (!isPlainObject(trial)) {
    throw drift('plan trial is not an object');
  }
  return requireNumber(trial, 'remaining');
}

function decodeFlag(data: Record<string, unknown>, key: string): boolean {
  const value = data[key];
  if (value === undefined || value === null) {
    return false;
  }
  if (typeof value !== 'boolean') {
    throw drift(`plan flag "${key}" is not a boolean`);
  }
  return value;
}

function decodeConsentAt(data: Record<string, unknown>): string | null {
  const at = optionalString(data, 'llm_consent_at');
  if (at === undefined) {
    return null;
  }
  if (Number.isNaN(Date.parse(at))) {
    throw drift('plan consent date is not a timestamp');
  }
  return at;
}

export function isSheetLimitReached(usage: PlanUsage | undefined, kind: SheetKind): boolean {
  const entry = usage?.sheets[kind];
  return entry !== undefined && entry.used >= entry.limit;
}
