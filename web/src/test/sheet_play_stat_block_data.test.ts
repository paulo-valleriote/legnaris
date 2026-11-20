import { describe, expect, it } from 'vitest';
import {
  isStatBlockKind,
  statBlockAbilityWord,
  statBlockKindLabelKey,
  statBlockSubtitleParts,
  statMovementParts,
  statProseText,
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

describe('the stat block movement line', () => {
  const texts = {
    speed: 'Speed',
    passive: 'Passive',
    feet: (count: number) => `${count} ft`,
  };

  it('Edge: no speed and no passive awareness produces no line', () => {
    expect(statProseText(statMovementParts(undefined, undefined, texts))).toBeUndefined();
  });

  it('Edge: an empty part list produces no line', () => {
    expect(statProseText([])).toBeUndefined();
  });

  it('Edge: a missing speed keeps the passive value alone', () => {
    expect(statProseText(statMovementParts(undefined, 16, texts))).toBe('Passive 16');
  });

  it('Edge: a missing passive awareness keeps the speed alone', () => {
    expect(statProseText(statMovementParts(30, undefined, texts))).toBe('Speed 30 ft');
  });

  it('Edge: a zero speed and a zero passive still draw both parts', () => {
    expect(statProseText(statMovementParts(0, 0, texts))).toBe('Speed 0 ft · Passive 0');
  });

  it('Happy path: speed and passive awareness share one line', () => {
    expect(statProseText(statMovementParts(30, 16, texts))).toBe('Speed 30 ft · Passive 16');
  });
});

describe('the stat block ability word', () => {
  it('Edge: an empty abbreviation stays empty', () => {
    expect(statBlockAbilityWord('')).toBe('');
  });

  it('Happy path: a shouty abbreviation reads as a stat-block word', () => {
    expect(statBlockAbilityWord('DEX')).toBe('Dex');
    expect(statBlockAbilityWord('WIS')).toBe('Wis');
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
