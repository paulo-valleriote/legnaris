import { describe, expect, it } from 'vitest';
import { isStatBlockKind, statBlockKindLabelKey } from '../sheet/preset/fantasy/stat_block_data';

describe('stat block kinds', () => {
  it('Edge: only npc and enemy sheets count as stat block kinds', () => {
    expect(isStatBlockKind('npc')).toBe(true);
    expect(isStatBlockKind('enemy')).toBe(true);
    expect(isStatBlockKind('character')).toBe(false);
    expect(isStatBlockKind(undefined)).toBe(false);
  });

  it('Happy path: each stat block kind names itself', () => {
    expect(statBlockKindLabelKey('npc')).toBe('sheet.play.kind.npc');
    expect(statBlockKindLabelKey('enemy')).toBe('sheet.play.kind.enemy');
    expect(statBlockKindLabelKey('character')).toBeUndefined();
    expect(statBlockKindLabelKey(undefined)).toBeUndefined();
  });
});
