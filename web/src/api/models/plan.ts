/**
 * Plan usage payload (GET /me/plan): the owner's plan and the Sheets held per
 * kind against the allowance the plan grants (LGN-183).
 */

import { drift, isPlainObject, optionalString, requireNumber, requireString } from '../parse';

/** The Sheet kinds the backend knows, in the order it reports them. */
export const SHEET_KINDS = ['character', 'npc', 'monster'] as const;

/** One Sheet kind. */
export type SheetKind = (typeof SHEET_KINDS)[number];

/** How many Sheets of one kind the owner holds, and what the plan allows. */
export interface SheetUsage {
  readonly used: number;
  readonly limit: number;
}

/** The owner's plan and the usage of every kind under it. */
export interface PlanUsage {
  /** The plan identifier, such as `free`. */
  readonly plan: string;
  /** Usage per kind; a kind the payload omits has no entry. */
  readonly sheets: Readonly<Partial<Record<SheetKind, SheetUsage>>>;
  /**
   * When the owner last granted LLM consent, as the server wrote it (RFC 3339), or
   * `null` when none is held or it was withdrawn (LGN-195).
   */
  readonly consentAt: string | null;
  /** The disclosure version the stored consent was given at, or `null` with no consent. */
  readonly consentVersion: string | null;
  /** The Free trial allowance left, or `undefined` when the payload carries no trial (LGN-196). */
  readonly trialRemaining: number | undefined;
  /** Whether generation is gated on consent the owner has not given at the current version. */
  readonly llmConsentRequired: boolean;
  /** The generation the owner may resume (queued, running or succeeded and not accepted), or `null`. */
  readonly activeGenerationId: string | null;
}

function decodeSheetUsage(value: unknown, kind: string): SheetUsage {
  if (!isPlainObject(value)) {
    throw drift(`plan usage for "${kind}" is not an object`);
  }
  return { used: requireNumber(value, 'used'), limit: requireNumber(value, 'limit') };
}

/**
 * Decodes the plan payload. An entry for a kind the client does not know is
 * dropped rather than rejected, so a newer backend kind never breaks the screen.
 */
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
    activeGenerationId: optionalString(data, 'active_generation_id') ?? null,
  };
}

/** Reads the Free trial's remaining allowance; an absent trial is `undefined`, a bad value drift. */
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

/** Reads a boolean flag; an absent key is `false`, a non-boolean value is drift. */
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

/**
 * Reads the stored consent date. An absent or null key is no consent; a value that
 * is not a parseable timestamp is drift, never shown as a date.
 */
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

/** Whether the owner can create no further Sheet of kind, by the usage read. */
export function isSheetLimitReached(usage: PlanUsage | undefined, kind: SheetKind): boolean {
  const entry = usage?.sheets[kind];
  return entry !== undefined && entry.used >= entry.limit;
}
