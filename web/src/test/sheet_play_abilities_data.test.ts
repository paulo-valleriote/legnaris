import { describe, expect, it } from 'vitest';
import {
  abilityModifierPath,
  abilityScorePath,
  formatModifier,
  parseAbilityScore,
} from '../sheet/preset/fantasy/abilities_data';

describe('formatModifier', () => {
  it.each([
    ['+0', 0],
    ['+1', 1],
    ['+12', 12],
    ['-1', -1],
    ['-12', -12],
    ['+0', -0],
  ])('Happy path: %s keeps its sign', (expected, modifier) => {
    expect(formatModifier(modifier)).toBe(expected);
  });

  it('Edge: a negative zero never shows as -0', () => {
    expect(formatModifier(-0)).toBe('+0');
  });
});

describe('parseAbilityScore', () => {
  it.each([
    ['12', 12],
    [' 14 ', 14],
    ['3', 3],
    ['20', 20],
  ])('Happy path: "%s" resolves to %d', (text, expected) => {
    expect(parseAbilityScore(text)).toBe(expected);
  });

  it.each([
    ['', 'an empty text'],
    ['   ', 'a blank text'],
    ['abc', 'letters'],
    ['12abc', 'trailing letters'],
    ['--3', 'a doubled sign'],
    ['3-', 'a trailing sign'],
    ['NaN', 'NaN'],
    ['Infinity', 'Infinity'],
    ['1e999', 'an overflowing number'],
  ])('Edge: %s is not a score (%s)', (text) => {
    expect(parseAbilityScore(text)).toBeUndefined();
  });

  it.each([
    ['2', 3],
    ['-5', 3],
    ['21', 20],
    ['99', 20],
  ])('Edge: "%s" clamps to the score range as %d', (text, expected) => {
    expect(parseAbilityScore(text)).toBe(expected);
  });

  it.each([
    ['12.4', 12],
    ['12.5', 13],
    ['3.2', 3],
    ['20.9', 20],
  ])('Edge: "%s" rounds to the whole score %d', (text, expected) => {
    expect(parseAbilityScore(text)).toBe(expected);
  });
});

describe('ability paths', () => {
  it('Happy path: each ability reads its score and its modifier by dotted path', () => {
    expect(abilityScorePath('strength')).toBe('stats.strength');
    expect(abilityModifierPath('strength')).toBe('modifiers.strengthMod');
    expect(abilityModifierPath('constitution')).toBe('modifiers.constitutionMod');
  });
});
