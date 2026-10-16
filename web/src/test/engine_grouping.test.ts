import { describe, expect, it } from 'vitest';
import type { FieldNode, FieldSchema } from '../api/models/template';
import {
  buildSections,
  fieldColumnAlign,
  fieldColumnName,
  fieldDisplayName,
  groupDisplayName,
  lastSegment,
} from '../engine/field_grouping';

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

describe('section derivation (ENG-01 … ENG-06)', () => {
  it('Edge: an empty field tree yields no section and does not throw (ENG-02)', () => {
    expect(buildSections({})).toEqual([]);
  });

  it('Edge: a group with no leaf, and one whose only leaves sit under an empty sub-group, yield no section (ENG-02)', () => {
    expect(buildSections({ empty: group({}) })).toEqual([]);
    expect(buildSections({ outer: group({ inner: group({}) }) })).toEqual([]);
  });

  it('Edge: a declared-column group none of whose columns resolve falls back to stacked rows (ENG-04)', () => {
    const fields = {
      stats: group(
        { dexterity: leaf({ label: 'Dexterity' }), constitution: leaf({ label: 'Constitution' }) },
        { label: 'Stats', columns: ['missing', 'alsoMissing'] },
      ),
    };

    const rows = buildSections(fields)[0].rows;

    expect(rows.map((row) => row.kind)).toEqual(['normal', 'normal']);
  });

  it('Edge: an unknown column beside a known one is skipped (ENG-04)', () => {
    const fields = {
      stats: group(
        { dexterity: leaf({ label: 'Dexterity' }), constitution: leaf({ label: 'Constitution' }) },
        { label: 'Stats', columns: ['missing', 'dexterity'] },
      ),
    };

    const row = buildSections(fields)[0].rows[0];

    expect(row.kind).toBe('table');
    if (row.kind === 'table') {
      expect(row.fields.map((field) => field.path)).toEqual(['stats.dexterity']);
    }
  });

  it('Edge: a single-member exclusive group is not collapsed (ENG-05)', () => {
    const fields = { ability: group({ stealth: leaf({ exclusiveGroup: 'ability' }) }) };

    const rows = buildSections(fields)[0].rows;

    expect(rows).toHaveLength(1);
    expect(rows[0].kind).toBe('normal');
    if (rows[0].kind === 'normal') {
      expect(rows[0].field.path).toBe('ability.stealth');
    }
  });

  it('Edge: non-adjacent members of one exclusive group collapse at the first member position (ENG-05)', () => {
    const fields = {
      stats: group({
        stealth: leaf({ label: 'Stealth', exclusiveGroup: 'ability' }),
        level: leaf({ label: 'Level' }),
        athletics: leaf({ label: 'Athletics', exclusiveGroup: 'ability' }),
      }),
    };

    const rows = buildSections(fields)[0].rows;

    expect(rows.map((row) => row.kind)).toEqual(['exclusive', 'normal']);
    if (rows[0].kind === 'exclusive') {
      expect(rows[0].group.options.map((option) => option.path)).toEqual([
        'stats.stealth',
        'stats.athletics',
      ]);
    }
    if (rows[1].kind === 'normal') {
      expect(rows[1].field.path).toBe('stats.level');
    }
  });

  it('Happy path: one labeled section per top-level key, in arrival order, and a top-level leaf is its own section (ENG-01, ENG-02)', () => {
    const fields = {
      alpha: leaf({ label: 'Alpha' }),
      beta: group({ inner: leaf({ label: 'Inner' }) }, { label: 'Beta' }),
    };

    const sections = buildSections(fields);

    expect(sections.map((section) => section.key)).toEqual(['alpha', 'beta']);
    expect(sections.map((section) => section.label)).toEqual(['Alpha', 'Beta']);
    expect(sections[0].leaves.map((field) => field.path)).toEqual(['alpha']);
    expect(sections[1].leaves.map((field) => field.path)).toEqual(['beta.inner']);
  });

  it('Happy path: a label is the non-empty source label, else a mechanical humanization of the key (ENG-03)', () => {
    expect(fieldDisplayName('armorClassAbility', undefined)).toBe('Armor class ability');
    expect(fieldDisplayName('stats', schema({ label: 'Statistics' }))).toBe('Statistics');
    expect(fieldDisplayName('x', schema({ label: '   ' }))).toBe('X');
    expect(groupDisplayName('armorClassAbility', '  Armor Class  ')).toBe('Armor Class');
    expect(groupDisplayName('house_rules', undefined)).toBe('House rules');
    expect(lastSegment('stats.dexterity')).toBe('dexterity');
    expect(lastSegment('dexterity')).toBe('dexterity');
  });

  it('Happy path: a table row holds one field per declared column, in declared order (ENG-04, ENG-06)', () => {
    const fields = {
      stats: group(
        { dexterity: leaf({ label: 'Dexterity' }), constitution: leaf({ label: 'Constitution' }) },
        { label: 'Abilities', columns: ['constitution', 'dexterity'] },
      ),
    };

    const section = buildSections(fields)[0];

    expect(section.rows).toHaveLength(1);
    expect(section.rows[0].kind).toBe('table');
    if (section.rows[0].kind === 'table') {
      expect(section.rows[0].label).toBe('Abilities');
      expect(section.rows[0].fields.map((field) => field.path)).toEqual([
        'stats.constitution',
        'stats.dexterity',
      ]);
    }
    expect(section.leaves.map((field) => field.path)).toEqual([
      'stats.dexterity',
      'stats.constitution',
    ]);
  });

  it('Happy path: a column label is the columnLabel when non-empty, else the display name (ENG-06)', () => {
    expect(fieldColumnName('ac', schema({ label: 'Armor Class', columnLabel: 'AC' }))).toBe('AC');
    expect(fieldColumnName('armorClassAbility', schema({ label: '' }))).toBe('Armor class ability');
  });

  it('Happy path: a numeric column aligns right and every other column left (ENG-06)', () => {
    expect(fieldColumnAlign(schema({ type: 'number' }))).toBe('right');
    expect(fieldColumnAlign(schema({ type: 'text' }))).toBe('left');
    expect(fieldColumnAlign(schema({ type: 'bool' }))).toBe('left');
  });
});
