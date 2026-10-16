import { describe, expect, it } from 'vitest';
import { parseSheet, valueAtPath } from '../api/models/sheet';
import { DefensiveParseFailure, PathFailure } from '../api/errors';

function sheetPayload(overrides: Record<string, unknown> = {}) {
  return {
    id: '01a0f967-f42b-710e-851f-8f293a634921',
    template_id: 'fantasy_v1',
    name: 'Aragorn',
    data: {
      stats: { dexterity: 16, strength: 15 },
      combat: { hitPoints: 10.5, speed: 30 },
    },
    created_at: '2026-10-01T21:38:54.635Z',
    updated_at: '2026-10-02T09:00:00.000Z',
    ...overrides,
  };
}

describe('sheet model', () => {
  it('Happy path: decodes identity, template, name, data and both timestamps (API-10)', () => {
    const decoded = parseSheet(sheetPayload());

    expect(decoded.id).toBe('01a0f967-f42b-710e-851f-8f293a634921');
    expect(decoded.templateId).toBe('fantasy_v1');
    expect(decoded.name).toBe('Aragorn');
    expect(decoded.createdAt).toBe('2026-10-01T21:38:54.635Z');
    expect(decoded.updatedAt).toBe('2026-10-02T09:00:00.000Z');
  });

  it('Happy path: keeps the nested dynamic shape of data as received (API-10)', () => {
    const decoded = parseSheet(sheetPayload());

    expect(decoded.data['stats']).toEqual({ dexterity: 16, strength: 15 });
    expect(Object.keys(decoded.data)).toEqual(['stats', 'combat']);
  });

  it('Edge: a whole number stays whole and a fractional number stays fractional, with no coercion (API-11)', () => {
    const decoded = parseSheet(sheetPayload());
    const dexterity = valueAtPath(decoded.data, 'stats.dexterity');
    const hitPoints = valueAtPath(decoded.data, 'combat.hitPoints');

    expect(dexterity).toBe(16);
    expect(Number.isInteger(dexterity)).toBe(true);
    expect(hitPoints).toBe(10.5);
    expect(Number.isInteger(hitPoints)).toBe(false);
    expect(String(hitPoints)).toBe('10.5');
  });

  it('Edge: a string that looks numeric is not coerced into a number (API-11)', () => {
    const decoded = parseSheet(sheetPayload({ data: { stats: { dexterity: '16' } } }));

    expect(valueAtPath(decoded.data, 'stats.dexterity')).toBe('16');
  });

  it('Edge: an empty data object is legitimate and decodes as an empty map (API-10)', () => {
    expect(parseSheet(sheetPayload({ data: {} })).data).toEqual({});
  });

  it('Error: a missing identifier, template, name or timestamp is rejected by name (API-10)', () => {
    for (const key of ['id', 'template_id', 'name', 'created_at', 'updated_at']) {
      expect(() => parseSheet(sheetPayload({ [key]: undefined }))).toThrow(DefensiveParseFailure);
    }
  });

  it('Error: a data value that is neither an object nor null is rejected (API-10)', () => {
    for (const data of ['stats', 42, [1, 2]]) {
      expect(() => parseSheet(sheetPayload({ data }))).toThrow(DefensiveParseFailure);
    }
  });

  it('Error: a non-object payload is rejected rather than coerced (API-10)', () => {
    for (const payload of [null, 'sheet', 7, []]) {
      expect(() => parseSheet(payload)).toThrow(DefensiveParseFailure);
    }
  });
});

describe('dotted path accessor', () => {
  const data = {
    stats: { dexterity: 16 },
    identity: { characterName: 'Aragorn', level: 1 },
    flags: { none: null },
  };

  it('Happy path: a resolving path returns the value it points at (API-12)', () => {
    expect(valueAtPath(data, 'stats.dexterity')).toBe(16);
    expect(valueAtPath(data, 'identity.characterName')).toBe('Aragorn');
  });

  it('Edge: a path whose segment is missing yields no value without raising (API-13)', () => {
    expect(valueAtPath(data, 'stats.charisma')).toBeNull();
    expect(valueAtPath(data, 'absent.deep.deeper')).toBeNull();
  });

  it('Edge: a path whose intermediate node is not an object yields no value without raising (API-13)', () => {
    expect(valueAtPath(data, 'identity.characterName.first')).toBeNull();
    expect(valueAtPath(data, 'identity.level.ten')).toBeNull();
  });

  it('Edge: an explicit null value is returned as null, like a missing one (API-13)', () => {
    expect(valueAtPath(data, 'flags.none')).toBeNull();
  });

  it('Edge: a path never resolves through the prototype chain (API-13)', () => {
    expect(valueAtPath(data, 'toString')).toBeNull();
    expect(valueAtPath(data, 'constructor')).toBeNull();
    expect(valueAtPath(data, '__proto__')).toBeNull();
    expect(valueAtPath(data, 'stats.constructor')).toBeNull();
    expect(valueAtPath(data, 'stats.hasOwnProperty')).toBeNull();
  });

  it('Edge: an own property shadowing a prototype member is still data (API-12)', () => {
    expect(valueAtPath({ constructor: 'rendered' }, 'constructor')).toBe('rendered');
    expect(valueAtPath({ stats: { toString: 12 } }, 'stats.toString')).toBe(12);
  });

  it('Edge: a single-segment path resolves against the top level (API-12)', () => {
    expect(valueAtPath(data, 'stats')).toEqual({ dexterity: 16 });
  });

  it('Edge: the accessor never mutates the data it reads (API-13)', () => {
    const snapshot = JSON.stringify(data);

    valueAtPath(data, 'stats.dexterity');
    valueAtPath(data, 'absent.deep');

    expect(JSON.stringify(data)).toBe(snapshot);
  });

  it('Error: an empty path is malformed, and says so (API-14)', () => {
    expect(() => valueAtPath(data, '')).toThrow(PathFailure);
  });

  it('Error: a path with an empty segment is malformed, and says so (API-14)', () => {
    for (const path of ['stats..dexterity', '.stats', 'stats.']) {
      expect(() => valueAtPath(data, path)).toThrow(PathFailure);
    }
  });

  it('Error: a malformed path is not confused with a missing one (API-13, API-14)', () => {
    let raised: unknown;
    try {
      valueAtPath(data, 'stats..dexterity');
    } catch (error) {
      raised = error;
    }

    expect(raised).toBeInstanceOf(PathFailure);
    expect(valueAtPath(data, 'stats.dexterity')).toBe(16);
  });
});
