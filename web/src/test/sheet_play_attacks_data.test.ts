import { describe, expect, it } from 'vitest';
import {
  ATTACK_BONUS_MAX,
  ATTACK_BONUS_MIN,
  ATTACK_SLOTS,
  attackFilled,
  attackSlot,
  parseAttackBonus,
  parseAttackDamage,
} from '../sheet/preset/fantasy/attacks_data';

describe('attack slots', () => {
  it('Edge: a slot exposes the row plus its three column paths', () => {
    const slot = attackSlot(3);

    expect(slot.rowKey).toBe('attacks.attack3');
    expect(slot.namePath).toBe('attacks.attack3.name');
    expect(slot.bonusPath).toBe('attacks.attack3.bonus');
    expect(slot.damagePath).toBe('attacks.attack3.damage');
  });

  it('Edge: the template offers four ordered slots', () => {
    expect(ATTACK_SLOTS).toHaveLength(4);
    expect(ATTACK_SLOTS.map((slot) => slot.slot)).toEqual([1, 2, 3, 4]);
  });
});

describe('attack filled state', () => {
  it.each([
    { name: 'a name', expected: true },
    { name: '  padded  ', expected: true },
  ])('Happy path: "$name" is filled: $expected', ({ name, expected }) => {
    expect(attackFilled(name)).toBe(expected);
  });

  it.each([
    { name: '', expected: false },
    { name: '   ', expected: false },
    { name: undefined, expected: false },
    { name: null, expected: false },
    { name: 3, expected: false },
  ])('Edge: "$name" is not filled: $expected', ({ name, expected }) => {
    expect(attackFilled(name)).toBe(expected);
  });
});

describe('attack bonus parsing', () => {
  it.each([
    { text: '', expected: undefined },
    { text: '   ', expected: undefined },
    { text: 'two', expected: undefined },
  ])('Edge: "$text" parses as $expected', ({ text, expected }) => {
    expect(parseAttackBonus(text)).toBe(expected);
  });

  it.each([
    { text: '+4', expected: 4 },
    { text: ' -2 ', expected: -2 },
  ])('Happy path: "$text" parses as $expected', ({ text, expected }) => {
    expect(parseAttackBonus(text)).toBe(expected);
  });

  it.each([
    { text: '3.6', expected: 4 },
    { text: `${ATTACK_BONUS_MAX + 5}`, expected: ATTACK_BONUS_MAX },
    { text: `${ATTACK_BONUS_MIN - 5}`, expected: ATTACK_BONUS_MIN },
  ])('Edge: "$text" clamps or rounds to $expected', ({ text, expected }) => {
    expect(parseAttackBonus(text)).toBe(expected);
  });
});

describe('attack damage parsing', () => {
  it('Edge: an empty expression offers no roll', () => {
    expect(parseAttackDamage('')).toBeUndefined();
    expect(parseAttackDamage('   ')).toBeUndefined();
  });

  it('Edge: text that is not a dice expression offers no roll', () => {
    expect(parseAttackDamage('banana')).toBeUndefined();
    expect(parseAttackDamage('1d8 + banana')).toBeUndefined();
  });

  it('Edge: an expression without dice or with a field reference offers no roll', () => {
    expect(parseAttackDamage('5')).toBeUndefined();
    expect(parseAttackDamage('1d8 + @.bonus')).toBeUndefined();
    expect(parseAttackDamage('attacks.attack1.bonus')).toBeUndefined();
  });

  it('Happy path: a damage expression parses into dice and a flat bonus', () => {
    expect(parseAttackDamage('1d8 + 3')?.terms).toEqual([
      { kind: 'dice', sign: 1, count: 1, faces: 8 },
      { kind: 'number', sign: 1, value: 3 },
    ]);
  });
});
