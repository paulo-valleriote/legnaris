import type { FieldNode, FieldSchema } from '../api/models/template';
import type { SheetSection } from '../engine/field_grouping';
import { buildSections } from '../engine/field_grouping';

function schema(overrides: Partial<FieldSchema> = {}): FieldSchema {
  return { type: 'number', label: '', isEditable: true, defaultValue: 0, ...overrides };
}

function leaf(overrides: Partial<FieldSchema> = {}): FieldNode {
  return { kind: 'leaf', schema: schema(overrides) };
}

function group(
  children: Record<string, FieldNode>,
  metadata: { label?: string; columns?: readonly string[] } = {},
): FieldNode {
  return { kind: 'group', children, ...metadata };
}

export const ARRANGE_FIELDS: Record<string, FieldNode> = {
  alpha: group(
    {
      one: leaf({ label: 'One' }),
      two: leaf({ label: 'Two' }),
      three: leaf({ label: 'Three' }),
    },
    { label: 'Alpha' },
  ),
  beta: group(
    {
      note: leaf({ type: 'text', label: 'Note' }),
      stats: group(
        { str: leaf({ label: 'Strength' }), dex: leaf({ label: 'Dexterity' }) },
        { label: 'Stats', columns: ['str', 'dex'] },
      ),
      pickA: leaf({ label: 'Pick A', exclusiveGroup: 'pick' }),
      pickB: leaf({ label: 'Pick B', exclusiveGroup: 'pick' }),
    },
    { label: 'Beta' },
  ),
  gamma: group({ solo: leaf({ label: 'Solo' }) }, { label: 'Gamma' }),
};

export function arrangeSections(): SheetSection[] {
  return buildSections(ARRANGE_FIELDS);
}

export const ARRANGE_ROW_KEYS = {
  alpha: ['alpha.one', 'alpha.two', 'alpha.three'],
  beta: ['beta.note', 'beta.stats', 'beta#pick'],
  gamma: ['gamma.solo'],
} as const;
