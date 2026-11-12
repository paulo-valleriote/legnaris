import { describe, expect, it } from 'vitest';
import type { DiceBlock } from '../api/models/dice';
import { hasEntries } from '../api/parse';
import type { SheetDraft } from '../library/sheet_store';
import { hasPendingWork } from '../library/sheet_store';

const EMPTY_BLOCK: DiceBlock = { section: {}, row: {} };

describe('hasEntries', () => {
  it.each<[string, Record<string, unknown>, boolean]>([
    ['Edge: an empty record has no entries', {}, false],
    ['Edge: a key holding undefined still counts', { a: undefined }, true],
    ['Edge: a key holding null still counts', { a: null }, true],
    ['Happy path: a record with a key has entries', { 'stats.dexterity': 12 }, true],
  ])('%s', (_name, record, expected) => {
    expect(hasEntries(record)).toBe(expected);
  });
});

describe('hasPendingWork (ADR 0006)', () => {
  it.each<[string, SheetDraft, boolean]>([
    ['Edge: an empty delta with no Dice block is no pending work', { delta: {} }, false],
    [
      'Edge: an explicit undefined Dice block is no pending work',
      { delta: {}, dice: undefined },
      false,
    ],
    [
      'Edge: an empty Dice block alone is pending work (it clears the saved one)',
      { delta: {}, dice: EMPTY_BLOCK },
      true,
    ],
    [
      'Happy path: a non-empty delta alone is pending work',
      { delta: { 'stats.dexterity': 12 } },
      true,
    ],
    [
      'Happy path: a delta and a Dice block together are pending work',
      { delta: { 'stats.dexterity': 12 }, dice: { section: { attacks: '1d20+@' }, row: {} } },
      true,
    ],
  ])('%s', (_name, draft, expected) => {
    expect(hasPendingWork(draft)).toBe(expected);
  });
});
