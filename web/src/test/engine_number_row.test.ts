import { describe, expect, it } from 'vitest';
import type { FieldNode, FieldSchema } from '../api/models/template';
import { buildSections } from '../engine/field_grouping';
import { arrangementView, isNumberRow } from '../engine/layout';

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

const FIELDS: Record<string, FieldNode> = {
  core: group(
    {
      hp: leaf({ label: 'Hit points' }),
      speed: leaf({ label: 'Speed' }),
      mod: leaf({ label: 'Modifier', isEditable: false, formula: 'hp + 1' }),
      label: leaf({ type: 'text', label: 'Epithet' }),
      ghostly: leaf({ type: 'text', label: 'Derived text', isEditable: false }),
      guarded: leaf({ type: 'bool', label: 'Guarded' }),
      stats: group(
        { str: leaf({ label: 'Str' }), dex: leaf({ label: 'Dex' }) },
        { columns: ['str', 'dex'] },
      ),
      pickA: leaf({ label: 'Pick A', exclusiveGroup: 'pick' }),
      pickB: leaf({ label: 'Pick B', exclusiveGroup: 'pick' }),
      lone: leaf({ label: 'Lone', exclusiveGroup: 'solo' }),
    },
    { label: 'Core' },
  ),
  more: group(
    {
      a: leaf({ label: 'A' }),
      b: leaf({ label: 'B' }),
      c: leaf({ label: 'C' }),
      d: leaf({ label: 'D' }),
      e: leaf({ label: 'E' }),
      f: leaf({ label: 'F' }),
      g: leaf({ label: 'G' }),
    },
    { label: 'More' },
  ),
  config: group(
    {
      switchOn: leaf({ label: 'Switch' }),
      readout: leaf({ label: 'Readout', isEditable: false }),
    },
    { label: 'Config' },
  ),
};

const sections = buildSections(FIELDS);

describe('which Rows are number Rows', () => {
  const rowOf = (key: string) =>
    arrangementView(sections, { sectionOrder: [], pinned: [] })
      .flatMap((entry) => entry.rows)
      .find((entry) => entry.key === key)!.row;

  it.each([
    { name: 'Happy path: an editable number leaf', key: 'core.hp', expected: true },
    { name: 'Happy path: a derived number leaf', key: 'core.mod', expected: true },
    {
      name: 'Edge: a derived leaf under config is a readout, not a switch',
      key: 'config.readout',
      expected: true,
    },
    {
      name: 'Edge: a single-member exclusive id is still a lone number Row',
      key: 'core.lone',
      expected: true,
    },
    {
      name: 'Edge: an editable config leaf draws a switch',
      key: 'config.switchOn',
      expected: false,
    },
    { name: 'Edge: a text leaf', key: 'core.label', expected: false },
    { name: 'Edge: a derived text leaf', key: 'core.ghostly', expected: false },
    { name: 'Edge: a bool leaf', key: 'core.guarded', expected: false },
    { name: 'Edge: a table Row', key: 'core.stats', expected: false },
    { name: 'Edge: an exclusive-group Row', key: 'core#pick', expected: false },
  ])('$name', ({ key, expected }) => {
    expect(isNumberRow(rowOf(key))).toBe(expected);
  });
});
