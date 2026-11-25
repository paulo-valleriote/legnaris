import { describe, expect, it } from 'vitest';
import { parseTemplateDetail } from '../api/models/template';
import { fieldLabelFromFields, resolveRollPartLabel } from '../sheet/dice_label';
import { SYNTHETIC_RECORD_ID } from '../engine/row_identity';

function lookup(labels: Readonly<Record<string, string>>) {
  return (path: string) => labels[path];
}

const labelCases: readonly {
  readonly name: string;
  readonly path: string;
  readonly labels: Readonly<Record<string, string>>;
  readonly want: string;
}[] = [
  {
    name: 'Happy path: the schema label of the path wins',
    path: 'abilities.wisdom.mod',
    labels: { 'abilities.wisdom.mod': 'Wisdom modifier' },
    want: 'Wisdom modifier',
  },
  {
    name: 'Happy path: a generic leaf falls back to its parent label',
    path: 'abilities.wisdom.mod',
    labels: { 'abilities.wisdom': 'Wisdom' },
    want: 'Wisdom mod',
  },
  {
    name: 'Happy path: with no labels the last segments are humanized',
    path: 'abilities.wisdom.mod',
    labels: {},
    want: 'Wisdom mod',
  },
  {
    name: 'Edge: an unknown single segment is humanized, never dotted',
    path: 'mystery',
    labels: {},
    want: 'Mystery',
  },
  {
    name: 'Edge: a non-generic leaf never borrows its parent label',
    path: 'stats.wisdom',
    labels: { stats: 'Stats' },
    want: 'Wisdom',
  },
  {
    name: 'Edge: a blank schema label falls through to the fallback',
    path: 'exhaustion.penalty',
    labels: { 'exhaustion.penalty': '   ' },
    want: 'Exhaustion penalty',
  },
  {
    name: 'Edge: a generic root with no parent is humanized alone',
    path: 'bonus',
    labels: {},
    want: 'Bonus',
  },
];

describe('resolveRollPartLabel', () => {
  it.each(labelCases)('$name', ({ path, labels, want }) => {
    expect(resolveRollPartLabel(path, lookup(labels))).toBe(want);
  });

  it('Edge: no lookup still humanizes the last segments', () => {
    expect(resolveRollPartLabel('abilities.wisdom.mod')).toBe('Wisdom mod');
  });
});

const template = parseTemplateDetail({
  id: 'label_v1',
  name: 'Labels',
  default: true,
  kinds: [],
  fields: {
    abilities: {
      label: 'Abilities',
      children: {
        wisdom: {
          children: {
            mod: {
              schema: {
                type: 'number',
                label: 'Wisdom modifier',
                is_editable: false,
                default: 0,
              },
            },
          },
        },
      },
    },
    attacks: {
      list: {
        label: 'Attacks',
        capacity: 3,
        shape: {
          toHit: {
            schema: {
              type: 'number',
              label: 'To hit',
              is_editable: false,
              default: 0,
            },
          },
        },
      },
    },
  },
});

const fieldCases: readonly {
  readonly name: string;
  readonly path: string;
  readonly want: string | undefined;
}[] = [
  {
    name: 'Happy path: a leaf path resolves to its schema label',
    path: 'abilities.wisdom.mod',
    want: 'Wisdom modifier',
  },
  {
    name: 'Happy path: a group path resolves to its declared label',
    path: 'abilities',
    want: 'Abilities',
  },
  {
    name: 'Happy path: a Record shape field resolves to its schema label',
    path: `attacks.${SYNTHETIC_RECORD_ID}.toHit`,
    want: 'To hit',
  },
  {
    name: 'Edge: an unknown path resolves to nothing',
    path: 'abilities.wisdom.unknown',
    want: undefined,
  },
];

describe('fieldLabelFromFields', () => {
  it.each(fieldCases)('$name', ({ path, want }) => {
    expect(fieldLabelFromFields(template.fields)(path)).toBe(want);
  });
});
