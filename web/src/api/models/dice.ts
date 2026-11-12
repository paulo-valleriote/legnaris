/**
 * The Dice models (ADR 0005): the player's expression block that lives on the
 * Sheet, and the Template's defaults.
 *
 * The block is tolerant on read: an entry that is neither a string nor `null`
 * is dropped rather than failing the whole Sheet, and an absent block reads as
 * empty. Whether an expression still applies is the resolver's job, not the
 * decoder's, so a stale override is kept here and ignored later.
 */

import { isPlainObject } from '../parse';

/** A player's choice per key: a string replaces or adds, `null` is "no dice". */
export type DiceOverrides = Readonly<Record<string, string | null>>;

/** The Sheet's dice block (ADR 0005); both maps are always present, possibly empty. */
export interface DiceBlock {
  readonly section: DiceOverrides;
  readonly row: DiceOverrides;
}

/** The Template's dice defaults, keyed by Section key and Row key. */
export interface DiceDefaults {
  readonly section: Readonly<Record<string, string>>;
  readonly row: Readonly<Record<string, string>>;
}

function parseOverrides(value: unknown): Record<string, string | null> {
  if (!isPlainObject(value)) {
    return {};
  }

  const overrides: Record<string, string | null> = {};
  for (const [key, entry] of Object.entries(value)) {
    if (typeof entry === 'string' || entry === null) {
      overrides[key] = entry;
    }
  }
  return overrides;
}

function parseDefaults(value: unknown): Record<string, string> {
  return Object.fromEntries(
    Object.entries(parseOverrides(value)).filter(
      (entry): entry is [string, string] => typeof entry[1] === 'string',
    ),
  );
}

/** Parses the `dice` block of a sheet payload; absent or malformed reads as empty. */
export function parseDiceBlock(payload: unknown): DiceBlock {
  if (!isPlainObject(payload)) {
    return { section: {}, row: {} };
  }

  return {
    section: parseOverrides(payload['section']),
    row: parseOverrides(payload['row']),
  };
}

/** Parses the Template's dice defaults; `undefined` when the Template declares none. */
export function parseDiceDefaults(payload: unknown): DiceDefaults | undefined {
  if (!isPlainObject(payload)) {
    return undefined;
  }

  return {
    section: parseDefaults(payload['section']),
    row: parseDefaults(payload['row']),
  };
}

/** The wire shape of a dice block: the `dice` member of the `PATCH /sheets/:id` body (ADR 0006). */
export function diceBlockToWire(block: DiceBlock): {
  readonly dice: { readonly section: DiceOverrides; readonly row: DiceOverrides };
} {
  return { dice: { section: { ...block.section }, row: { ...block.row } } };
}
