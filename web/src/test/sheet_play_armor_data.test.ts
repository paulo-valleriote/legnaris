import { describe, expect, it } from 'vitest';
import {
  ARMOR_ABILITIES,
  ARMOR_ABILITY_KEYS,
  BASE_ARMOR_MAX,
  BASE_ARMOR_MIN,
  SPEED_MAX,
  SPEED_MIN,
  armorAbilityConfigPaths,
  armorAbilityFromValues,
  parseBaseArmor,
  parseSpeed,
} from '../sheet/preset/fantasy/armor_class';

describe('the armor class ability selection', () => {
  it.each([
    ['dexterity alone', { dex: 1, con: 0 }, 'dex'],
    ['constitution alone', { dex: 0, con: 1 }, 'con'],
    ['a non-zero member other than one', { dex: 0, con: 3 }, 'con'],
    ['the other leaf is missing', { dex: 1 }, 'dex'],
    ['the other leaf is null', { dex: null, con: 1 }, 'con'],
  ] as const)('Happy path: %s is the active member', (_case, values, expected) => {
    expect(armorAbilityFromValues(values)).toBe(expected);
  });

  it.each([
    ['both members are off', { dex: 0, con: 0 }],
    ['both members are on', { dex: 1, con: 1 }],
    ['both leaves are missing', {}],
    ['both leaves are null', { dex: null, con: null }],
    ['both leaves are text', { dex: '1', con: '0' }],
    ['both leaves are booleans', { dex: true, con: false }],
  ])('Edge: %s, so no member is active', (_case, values) => {
    expect(armorAbilityFromValues(values)).toBeUndefined();
  });

  it('Edge: the exclusive group offers the two config leaves in a stable order', () => {
    expect(ARMOR_ABILITY_KEYS).toEqual(['dex', 'con']);
    expect(armorAbilityConfigPaths()).toEqual(['config.acUsesDex', 'config.acUsesCon']);
  });

  it('Happy path: each member reads its own modifier path', () => {
    expect(ARMOR_ABILITIES.dex.modifierPath).toBe('modifiers.dexterityMod');
    expect(ARMOR_ABILITIES.con.modifierPath).toBe('modifiers.constitutionMod');
  });
});

describe('parsing the base armor class', () => {
  it.each([
    ['12', 12],
    [' 7 ', 7],
    ['1', BASE_ARMOR_MIN],
    ['30', BASE_ARMOR_MAX],
    ['0', BASE_ARMOR_MIN],
    ['-4', BASE_ARMOR_MIN],
    ['99', BASE_ARMOR_MAX],
    ['12.6', 13],
    ['12.4', 12],
  ] as const)('Happy path: "%s" reads as %i', (text, expected) => {
    expect(parseBaseArmor(text)).toBe(expected);
  });

  it.each(['', '   ', 'abc', '12abc', 'Infinity', 'NaN'])(
    'Edge: "%s" never reaches the store',
    (text) => {
      expect(parseBaseArmor(text)).toBeUndefined();
    },
  );
});

describe('parsing the speed', () => {
  it.each([
    ['30', 30],
    [' 0 ', 0],
    ['-2', SPEED_MIN],
    ['200', SPEED_MAX],
    ['119.5', 120],
  ] as const)('Happy path: "%s" reads as %i', (text, expected) => {
    expect(parseSpeed(text)).toBe(expected);
  });

  it.each(['', '  ', 'fast', '30ft'])('Edge: "%s" never reaches the store', (text) => {
    expect(parseSpeed(text)).toBeUndefined();
  });
});
