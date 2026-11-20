import { describe, expect, it } from 'vitest';
import type { SheetField } from '../engine/field_grouping';
import { hasSpellCast, SLOT_LEVELS, slotPath } from '../sheet/preset/fantasy/spells_data';

function totalField(level: number): SheetField {
  return {
    path: slotPath(level, 'total'),
    schema: { type: 'number', label: `Level ${level} total`, isEditable: true, defaultValue: 0 },
  };
}

function fieldsWithLevels(levels: readonly number[]): ReadonlyMap<string, SheetField> {
  return new Map(levels.map((level) => [slotPath(level, 'total'), totalField(level)]));
}

const edited = new Set<string>();

describe('spellcasting presence', () => {
  it('Edge: an empty sheet has no spellcasting', () => {
    expect(
      hasSpellCast({ data: {}, fields: fieldsWithLevels(SLOT_LEVELS), editedPaths: edited }),
    ).toBe(false);
  });

  it('Edge: blank at-will and list copy is not spellcasting', () => {
    expect(
      hasSpellCast({
        data: { spells: { atWill: '   ', list: '' } },
        fields: fieldsWithLevels(SLOT_LEVELS),
        editedPaths: edited,
      }),
    ).toBe(false);
  });

  it('Happy path: at-will copy alone counts as spellcasting', () => {
    expect(
      hasSpellCast({
        data: { spells: { atWill: 'Light' } },
        fields: fieldsWithLevels(SLOT_LEVELS),
        editedPaths: edited,
      }),
    ).toBe(true);
  });

  it('Happy path: list copy alone counts as spellcasting', () => {
    expect(
      hasSpellCast({
        data: { spells: { list: 'Magic missile' } },
        fields: fieldsWithLevels(SLOT_LEVELS),
        editedPaths: edited,
      }),
    ).toBe(true);
  });

  it('Happy path: a level with slots counts as spellcasting', () => {
    expect(
      hasSpellCast({
        data: { spells: { slot1: { total: 2, expended: 0, remaining: 2 } } },
        fields: fieldsWithLevels([1]),
        editedPaths: edited,
      }),
    ).toBe(true);
  });

  it('Edge: a zero or negative total is not a slot', () => {
    expect(
      hasSpellCast({
        data: { spells: { slot1: { total: 0 }, slot2: { total: -3 } } },
        fields: fieldsWithLevels([1, 2]),
        editedPaths: edited,
      }),
    ).toBe(false);
  });

  it('Edge: a fractional total below one truncates to no slot', () => {
    expect(
      hasSpellCast({
        data: { spells: { slot1: { total: 0.9 } } },
        fields: fieldsWithLevels([1]),
        editedPaths: edited,
      }),
    ).toBe(false);
  });

  it('Edge: a pending total edit keeps the level present at zero', () => {
    expect(
      hasSpellCast({
        data: { spells: { slot1: { total: 0 } } },
        fields: fieldsWithLevels([1]),
        editedPaths: new Set([slotPath(1, 'total')]),
      }),
    ).toBe(true);
  });

  it('Edge: a level the template lacks is ignored', () => {
    expect(
      hasSpellCast({
        data: { spells: { slot3: { total: 4 } } },
        fields: fieldsWithLevels([1]),
        editedPaths: edited,
      }),
    ).toBe(false);
  });

  it('Edge: derived spell attack and save DC alone are not spellcasting', () => {
    expect(
      hasSpellCast({
        data: { spells: { attackBonus: 6, saveDC: 14 } },
        fields: fieldsWithLevels(SLOT_LEVELS),
        editedPaths: edited,
      }),
    ).toBe(false);
  });
});
