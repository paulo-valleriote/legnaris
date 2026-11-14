/**
 * Plan usage payload (GET /me/plan): the owner's plan and the Sheets held per
 * kind against the allowance the plan grants (LGN-183).
 */

import { drift, isPlainObject, requireNumber, requireString } from '../parse';

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

  return { plan: requireString(data, 'plan'), sheets };
}

/** Whether the owner can create no further Sheet of kind, by the usage read. */
export function isSheetLimitReached(usage: PlanUsage | undefined, kind: SheetKind): boolean {
  const entry = usage?.sheets[kind];
  return entry !== undefined && entry.used >= entry.limit;
}
