import type { Sheet } from '../api/models/sheet';
import { valueAtPath } from '../api/models/sheet';
import type { TemplateDetail } from '../api/models/template';
import type { DemoDerivation } from './demo_store';
import { FIFTH_DEMO_SHEET, FIFTH_DEMO_TEMPLATE } from './fifth_demo_character';
import { templateDerivation } from './template_derivation';

const SLOT_LEVELS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;
const HIT_DICE_KEYS = ['primary', 'd6', 'd8', 'd10', 'd12'] as const;

export const MOMENT_TEMPLATE: TemplateDetail = FIFTH_DEMO_TEMPLATE;

export const MOMENT_SHEET: Sheet = FIFTH_DEMO_SHEET;

export const MOMENT_DERIVATION: DemoDerivation = templateDerivation(FIFTH_DEMO_TEMPLATE);

function readNumber(data: Record<string, unknown>, path: string): number {
  try {
    const value = valueAtPath(data, path);
    return typeof value === 'number' && Number.isFinite(value) ? value : 0;
  } catch {
    return 0;
  }
}

function readRecords(
  data: Record<string, unknown>,
  path: string,
): Record<string, unknown> | undefined {
  try {
    const value = valueAtPath(data, path);
    return typeof value === 'object' && value !== null && !Array.isArray(value)
      ? (value as Record<string, unknown>)
      : undefined;
  } catch {
    return undefined;
  }
}

function resourceReset(data: Record<string, unknown>): Record<string, unknown> | undefined {
  const records = readRecords(data, 'resources.list');
  if (records === undefined) {
    return undefined;
  }
  return Object.fromEntries(
    Object.keys(records).map((id) => [
      id,
      {
        used:
          readNumber(data, `resources.list.${id}.used`) *
          readNumber(data, `resources.list.${id}.rechargeNone`),
      },
    ]),
  );
}

export function longRestCascade(data: Record<string, unknown>): Record<string, unknown> {
  const slots = Object.fromEntries(SLOT_LEVELS.map((level) => [`slot${level}`, { spent: 0 }]));
  const hitDice = Object.fromEntries(HIT_DICE_KEYS.map((key) => [key, { spent: 0 }]));
  const resources = resourceReset(data);
  const cascade: Record<string, unknown> = {
    combat: { hp: { current: readNumber(data, 'combat.hp.max') } },
    exhaustion: { level: Math.max(0, readNumber(data, 'exhaustion.level') - 1) },
    status: { deathSaves: { successes: 0, failures: 0 } },
    hitDice,
    spellcasting: { ...slots, pact: { used: 0 } },
  };
  if (resources !== undefined) {
    cascade.resources = { list: resources };
  }
  return cascade;
}
