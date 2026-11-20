import { describe, expect, it } from 'vitest';
import {
  isStatBlockKind,
  statBlockKindLabelKey,
  statBlockSubtitleParts,
} from '../sheet/preset/fantasy/stat_block_data';

const levelText = (level: number) => `Level ${level}`;

describe('stat block kinds', () => {
  it('Edge: only npc and enemy sheets count as stat block kinds', () => {
    expect(isStatBlockKind('npc')).toBe(true);
    expect(isStatBlockKind('enemy')).toBe(true);
    expect(isStatBlockKind('character')).toBe(false);
    expect(isStatBlockKind(undefined)).toBe(false);
  });

  it('Happy path: each stat block kind names itself', () => {
    expect(statBlockKindLabelKey('npc')).toBe('kind.npc');
    expect(statBlockKindLabelKey('enemy')).toBe('kind.enemy');
    expect(statBlockKindLabelKey('character')).toBeUndefined();
    expect(statBlockKindLabelKey(undefined)).toBeUndefined();
  });
});

describe('the stat block subtitle', () => {
  it('Edge: an empty identity produces no parts', () => {
    expect(
      statBlockSubtitleParts({ ancestry: '', origin: '', level: undefined }, levelText),
    ).toEqual([]);
  });

  it('Edge: blank parts are skipped and only the level remains', () => {
    expect(statBlockSubtitleParts({ ancestry: '  ', origin: '', level: 5 }, levelText)).toEqual([
      'Level 5',
    ]);
  });

  it('Edge: only the ancestry is set', () => {
    expect(
      statBlockSubtitleParts({ ancestry: 'Elf', origin: '', level: undefined }, levelText),
    ).toEqual(['Elf']);
  });

  it('Happy path: ancestry, origin and level keep their order', () => {
    expect(
      statBlockSubtitleParts({ ancestry: 'Elf', origin: 'Wanderer', level: 3 }, levelText),
    ).toEqual(['Elf', 'Wanderer', 'Level 3']);
  });
});
