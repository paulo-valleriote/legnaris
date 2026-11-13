/**
 * PROTOTYPE (LGN-177) — throwaway, never merged. A fake template, fake generated values and a
 * fake server recalculation, so the generation flow runs with no backend.
 */
import { parseTemplateDetail } from '../../api/models/template';
import type { TemplateDetail } from '../../api/models/template';

const leaf = (label: string, extra: Record<string, unknown> = {}) => ({
  schema: { type: 'number', label, is_editable: true, default: 0, ...extra },
});

export const FAKE_TEMPLATE: TemplateDetail = parseTemplateDetail({
  id: 'fantasy_v1',
  name: 'Fantasy',
  default: true,
  fields: {
    identity: {
      label: 'Identity',
      children: {
        characterName: { schema: { type: 'text', label: 'Name', is_editable: true, default: 0 } },
        level: leaf('Level', { default: 1 }),
      },
    },
    attributes: {
      label: 'Attributes',
      children: {
        strength: leaf('Strength'),
        agility: leaf('Agility'),
        mind: leaf('Mind'),
      },
    },
    derived: {
      label: 'Derived',
      children: {
        hp: {
          schema: {
            type: 'number',
            label: 'Hit points',
            formula: 'attributes.strength * 2 + identity.level',
            depends_on: [
              ['attributes', 'strength'],
              ['identity', 'level'],
            ],
            is_editable: false,
            default: 0,
          },
        },
        defense: {
          schema: {
            type: 'number',
            label: 'Defense',
            formula: '10 + attributes.agility',
            depends_on: [['attributes', 'agility']],
            is_editable: false,
            default: 0,
          },
        },
      },
    },
    calling: {
      label: 'Calling',
      children: {
        warrior: leaf('Warrior', { exclusive_group: 'calling', option_label: 'Warrior' }),
        rogue: leaf('Rogue', { exclusive_group: 'calling', option_label: 'Rogue' }),
        mage: leaf('Mage', { exclusive_group: 'calling', option_label: 'Mage' }),
      },
    },
  },
});

type Data = Record<string, Record<string, unknown>>;

/** Stands in for the server's NormalizeAndRecalculate. */
export function recalculate(data: Data): Data {
  const strength = Number(data.attributes?.strength ?? 0);
  const agility = Number(data.attributes?.agility ?? 0);
  const level = Number(data.identity?.level ?? 1);
  return { ...data, derived: { hp: strength * 2 + level, defense: 10 + agility } };
}

const NAMES = ['Brannoc Vell', 'Isra Duskmere', 'Tolven Ashgrave', 'Mirela Quain'];
let generationCount = 0;

/** One "LLM" result, different on each regenerate. */
export function generatedValues(): Data {
  generationCount += 1;
  const n = generationCount;
  const calling = ['warrior', 'rogue', 'mage'][n % 3] as string;
  return recalculate({
    identity: { characterName: NAMES[n % NAMES.length], level: 1 + (n % 3) },
    attributes: { strength: 2 + (n % 4), agility: 4 - (n % 3), mind: 1 + (n % 5) },
    calling: { warrior: 0, rogue: 0, mage: 0, [calling]: 1 },
  });
}

/** Sets a dotted-path value without mutating the input. */
export function withValue(data: Data, path: string, value: unknown): Data {
  const [group, key] = path.split('.') as [string, string];
  const next: Data = { ...data, [group]: { ...(data[group] ?? {}), [key]: value } };
  if (group === 'calling') {
    next.calling = { warrior: 0, rogue: 0, mage: 0, [key]: value };
  }
  return next;
}
