import { isPlainObject } from '../parse';

export type DiceOverrides = Readonly<Record<string, string | null>>;

export interface DiceBlock {
  readonly section: DiceOverrides;
  readonly row: DiceOverrides;
}

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

export function parseDiceBlock(payload: unknown): DiceBlock {
  if (!isPlainObject(payload)) {
    return { section: {}, row: {} };
  }

  return {
    section: parseOverrides(payload['section']),
    row: parseOverrides(payload['row']),
  };
}

export function parseDiceDefaults(payload: unknown): DiceDefaults | undefined {
  if (!isPlainObject(payload)) {
    return undefined;
  }

  return {
    section: parseDefaults(payload['section']),
    row: parseDefaults(payload['row']),
  };
}

export function diceBlockToWire(block: DiceBlock): {
  readonly dice: { readonly section: DiceOverrides; readonly row: DiceOverrides };
} {
  return { dice: { section: { ...block.section }, row: { ...block.row } } };
}
