import { describe, expect, it } from 'vitest';
import {
  INITIATIVE_MAX,
  INITIATIVE_MIN,
  isStatBlockKind,
  parseInitiative,
  statBlockKindLabelKey,
  usesStatBlock,
} from '../sheet/preset/fantasy/stat_block_data';

describe('stat block kinds', () => {
  it('Edge: only npc and enemy sheets count as stat block kinds', () => {
    expect(isStatBlockKind('npc')).toBe(true);
    expect(isStatBlockKind('enemy')).toBe(true);
    expect(isStatBlockKind('character')).toBe(false);
    expect(isStatBlockKind(undefined)).toBe(false);
  });

  it('Edge: another template never uses the fantasy stat block', () => {
    expect(usesStatBlock('fantasy_v1', 'npc')).toBe(true);
    expect(usesStatBlock('fantasy_v1', 'enemy')).toBe(true);
    expect(usesStatBlock('fantasy_v1', 'character')).toBe(false);
    expect(usesStatBlock('fantasy_v1', undefined)).toBe(false);
    expect(usesStatBlock('plain_v1', 'npc')).toBe(false);
  });

  it('Happy path: each stat block kind names itself', () => {
    expect(statBlockKindLabelKey('npc')).toBe('sheet.play.kind.npc');
    expect(statBlockKindLabelKey('enemy')).toBe('sheet.play.kind.enemy');
    expect(statBlockKindLabelKey('character')).toBeUndefined();
    expect(statBlockKindLabelKey(undefined)).toBeUndefined();
  });
});

describe('the initiative parser', () => {
  it('Edge: empty, blank and non-numeric text never reports a value', () => {
    expect(parseInitiative('')).toBeUndefined();
    expect(parseInitiative('   ')).toBeUndefined();
    expect(parseInitiative('abc')).toBeUndefined();
    expect(parseInitiative('Infinity')).toBeUndefined();
    expect(parseInitiative('1e999')).toBeUndefined();
  });

  it('Edge: values clamp to the schema range', () => {
    expect(parseInitiative('99')).toBe(INITIATIVE_MAX);
    expect(parseInitiative('-99')).toBe(INITIATIVE_MIN);
    expect(parseInitiative(String(INITIATIVE_MIN))).toBe(INITIATIVE_MIN);
    expect(parseInitiative(String(INITIATIVE_MAX))).toBe(INITIATIVE_MAX);
  });

  it('Edge: decimals round and a negative zero is normalised', () => {
    expect(parseInitiative('3.6')).toBe(4);
    expect(parseInitiative('3.4')).toBe(3);
    expect(parseInitiative('-2.5')).toBe(-2);
    expect(Object.is(parseInitiative('-0'), 0)).toBe(true);
  });

  it('Happy path: a signed value is read as written', () => {
    expect(parseInitiative(' 7 ')).toBe(7);
    expect(parseInitiative('+3')).toBe(3);
    expect(parseInitiative('-4')).toBe(-4);
  });
});
