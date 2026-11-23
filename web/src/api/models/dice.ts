import { isPlainObject } from '../parse';

export type DiceOverrides = Readonly<Record<string, string | null>>;

export interface DiceBlock {
  readonly section: DiceOverrides;
  readonly row: DiceOverrides;
}

export interface DiceNotD20 {
  readonly section: readonly string[];
  readonly row: readonly string[];
}

export interface DiceDefaults {
  readonly section: Readonly<Record<string, string>>;
  readonly row: Readonly<Record<string, string>>;
  readonly notD20?: DiceNotD20;
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

  const notD20 = parseNotD20(payload['not_d20']);
  return {
    section: parseDefaults(payload['section']),
    row: parseDefaults(payload['row']),
    ...(notD20 === undefined ? {} : { notD20 }),
  };
}

function parseStringList(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter((entry): entry is string => typeof entry === 'string');
}

function parseNotD20(value: unknown): DiceNotD20 | undefined {
  if (!isPlainObject(value)) {
    return undefined;
  }

  const section = parseStringList(value['section']);
  const row = parseStringList(value['row']);
  if (section.length === 0 && row.length === 0) {
    return undefined;
  }
  return { section, row };
}

export function diceBlockToWire(block: DiceBlock): {
  readonly dice: { readonly section: DiceOverrides; readonly row: DiceOverrides };
} {
  return { dice: { section: { ...block.section }, row: { ...block.row } } };
}
