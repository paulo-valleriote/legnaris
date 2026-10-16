import { describe, expect, it } from 'vitest';
import type { FieldNode, FieldSchema } from '../api/models/template';
import {
  mergeCascade,
  seedDefaults,
  seedFromSheet,
  seedValue,
  setPathValue,
} from '../engine/sheet_defaults';

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
  identity: group(
    {
      characterName: leaf({ type: 'text', label: 'Name' }),
      level: leaf({ type: 'number', label: 'Level', defaultValue: 1 }),
      isNpc: leaf({ type: 'bool', label: 'NPC' }),
    },
    { label: 'Identity' },
  ),
  stats: group(
    {
      dexterity: leaf({ type: 'number', label: 'Dexterity', defaultValue: 8 }),
      strength: leaf({ type: 'number', label: 'Strength', defaultValue: 10 }),
    },
    { label: 'Stats', columns: ['dexterity', 'strength'] },
  ),
};

describe('sheet draft derivation (ENG-19 … ENG-21)', () => {
  it('Edge: mergeCascade with an empty override equals the base, is a fresh object and mutates neither input (ENG-21)', () => {
    const base = { stats: { dexterity: 8, strength: 10 }, name: 'x' };
    const overrides = {};
    const baseSnapshot = structuredClone(base);
    const overrideSnapshot = structuredClone(overrides);

    const result = mergeCascade(base, overrides);

    expect(result).toEqual(base);
    expect(result).not.toBe(base);
    expect(base).toEqual(baseSnapshot);
    expect(overrides).toEqual(overrideSnapshot);
  });

  it('Edge: a nested override changes only its leaf and preserves its siblings (ENG-21)', () => {
    const base = { stats: { dexterity: 8, strength: 10 }, name: 'x' };
    const baseSnapshot = structuredClone(base);

    const result = mergeCascade(base, { stats: { dexterity: 14 } });

    expect(result).toEqual({ stats: { dexterity: 14, strength: 10 }, name: 'x' });
    expect(base).toEqual(baseSnapshot);
  });

  it('Edge: setPathValue creates the intermediate objects a path needs and overwrites an existing leaf (ENG-19)', () => {
    const data: Record<string, unknown> = {};

    setPathValue(data, 'a.b.c', 5);
    expect(data).toEqual({ a: { b: { c: 5 } } });

    setPathValue(data, 'a.b.c', 9);
    expect(data).toEqual({ a: { b: { c: 9 } } });

    setPathValue(data, 'a.b', 'leaf');
    setPathValue(data, 'a.b.d', 1);
    expect(data).toEqual({ a: { b: { d: 1 } } });
  });

  it('Edge: seedValue seeds empty text, false boolean and the numeric default (ENG-19)', () => {
    expect(seedValue(schema({ type: 'text', defaultValue: 3 }))).toBe('');
    expect(seedValue(schema({ type: 'string', defaultValue: 3 }))).toBe('');
    expect(seedValue(schema({ type: 'bool', defaultValue: 3 }))).toBe(false);
    expect(seedValue(schema({ type: 'number', defaultValue: 7 }))).toBe(7);
  });

  it('Edge: an empty field tree seeds no leaf (ENG-19)', () => {
    expect(seedDefaults({})).toEqual({});
  });

  it('Happy path: seedDefaults yields one value per leaf under its dotted path in section order (ENG-19)', () => {
    const data = seedDefaults(FIELDS);

    expect(data).toEqual({
      identity: { characterName: '', level: 1, isNpc: false },
      stats: { dexterity: 8, strength: 10 },
    });
    expect(Object.keys(data)).toEqual(['identity', 'stats']);
    expect(Object.keys(data.identity as Record<string, unknown>)).toEqual([
      'characterName',
      'level',
      'isNpc',
    ]);
  });

  it('Happy path: seedFromSheet lets a stored value win, keeps an omitted leaf default and carries an unknown key (ENG-20)', () => {
    const data = seedFromSheet(FIELDS, {
      identity: { characterName: 'Aragorn' },
      stats: { dexterity: 12 },
      custom: { houseRule: true },
      extraLeaf: 5,
    });

    expect(data).toEqual({
      identity: { characterName: 'Aragorn', level: 1, isNpc: false },
      stats: { dexterity: 12, strength: 10 },
      custom: { houseRule: true },
      extraLeaf: 5,
    });
  });
});
