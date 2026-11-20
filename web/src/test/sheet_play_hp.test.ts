import { describe, expect, it } from 'vitest';
import {
  applyDamage,
  applyHeal,
  applyTemp,
  formatHitPointsDelta,
  HIT_POINTS_CURRENT,
  HIT_POINTS_MAX,
  HIT_POINTS_TEMP,
  hitPointsBand,
  hitPointsFrom,
  hitPointsPercent,
  hitPointsPreview,
  parseHitPointsAmount,
} from '../sheet/preset/fantasy/hit_points';

describe('reading the hit points', () => {
  it('Happy path: the nested combat values read as current, maximum and temporary points', () => {
    expect(
      hitPointsFrom({ combat: { hitPoints: 10, hitPointsMax: 20, hitPointsTemp: 5 } }),
    ).toEqual({ current: 10, max: 20, temp: 5 });
  });

  it('Edge: a missing combat branch or key reads as zero', () => {
    expect(hitPointsFrom({})).toEqual({ current: 0, max: 0, temp: 0 });
    expect(hitPointsFrom({ combat: { hitPoints: 4 } })).toEqual({ current: 4, max: 0, temp: 0 });
    expect(hitPointsFrom({ combat: {} })).toEqual({ current: 0, max: 0, temp: 0 });
  });

  it('Edge: a value that is not a finite number reads as zero', () => {
    expect(
      hitPointsFrom({
        combat: { hitPoints: '10', hitPointsMax: null, hitPointsTemp: Number.NaN },
      }),
    ).toEqual({ current: 0, max: 0, temp: 0 });
    expect(
      hitPointsFrom({
        combat: {
          hitPoints: Number.POSITIVE_INFINITY,
          hitPointsMax: -20,
          hitPointsTemp: 2.5,
        },
      }),
    ).toEqual({ current: 0, max: -20, temp: 2.5 });
  });

  it('Edge: the exported paths name the combat values', () => {
    expect([HIT_POINTS_CURRENT, HIT_POINTS_MAX, HIT_POINTS_TEMP]).toEqual([
      'combat.hitPoints',
      'combat.hitPointsMax',
      'combat.hitPointsTemp',
    ]);
  });
});

describe('damage', () => {
  it('Edge: a zero, negative or non-finite amount is ignored', () => {
    const hitPoints = { current: 10, max: 20, temp: 5 };

    expect(applyDamage(hitPoints, 0)).toBeUndefined();
    expect(applyDamage(hitPoints, -3)).toBeUndefined();
    expect(applyDamage(hitPoints, Number.NaN)).toBeUndefined();
    expect(applyDamage(hitPoints, Number.POSITIVE_INFINITY)).toBeUndefined();
  });

  it('Edge: a stored current below zero never yields a negative current', () => {
    expect(applyDamage({ current: -4, max: 20, temp: -2 }, 3)).toEqual({ current: 0, temp: 0 });
  });

  it('Edge: damage at zero current and zero temp stays at zero', () => {
    expect(applyDamage({ current: 0, max: 20, temp: 0 }, 6)).toEqual({ current: 0, temp: 0 });
  });

  it('Edge: damage exactly equal to the temporary points leaves the current untouched', () => {
    expect(applyDamage({ current: 10, max: 20, temp: 5 }, 5)).toEqual({ current: 10, temp: 0 });
  });

  it('Edge: more damage than current and temp together floors both at zero', () => {
    expect(applyDamage({ current: 3, max: 20, temp: 2 }, 20)).toEqual({ current: 0, temp: 0 });
  });

  it('Edge: fractional temporary points absorb a fractional amount', () => {
    expect(applyDamage({ current: 10, max: 20, temp: 2.5 }, 1.25)).toEqual({
      current: 10,
      temp: 1.25,
    });
  });

  it('Happy path: damage with no temporary points comes off the current value', () => {
    expect(applyDamage({ current: 10, max: 20, temp: 0 }, 4)).toEqual({ current: 6, temp: 0 });
  });

  it('Happy path: damage spends the temporary points first and the rest off the current value', () => {
    expect(applyDamage({ current: 10, max: 20, temp: 5 }, 8)).toEqual({ current: 7, temp: 0 });
  });
});

describe('healing', () => {
  it('Edge: a zero, negative or non-finite amount is ignored', () => {
    const hitPoints = { current: 5, max: 20, temp: 5 };

    expect(applyHeal(hitPoints, 0)).toBeUndefined();
    expect(applyHeal(hitPoints, -3)).toBeUndefined();
    expect(applyHeal(hitPoints, Number.NaN)).toBeUndefined();
    expect(applyHeal(hitPoints, Number.NEGATIVE_INFINITY)).toBeUndefined();
  });

  it('Edge: healing at the maximum keeps the maximum and the temporary points', () => {
    expect(applyHeal({ current: 20, max: 20, temp: 5 }, 6)).toEqual({ current: 20, temp: 5 });
  });

  it('Edge: healing a current above the maximum caps it at the maximum', () => {
    expect(applyHeal({ current: 25, max: 20, temp: 5 }, 1)).toEqual({ current: 20, temp: 5 });
  });

  it('Edge: a zero or negative maximum caps healing at zero', () => {
    expect(applyHeal({ current: 0, max: 0, temp: 0 }, 4)).toEqual({ current: 0, temp: 0 });
    expect(applyHeal({ current: 2, max: -5, temp: 0 }, 4)).toEqual({ current: 0, temp: 0 });
  });

  it('Happy path: healing adds to the current value and leaves the temporary points alone', () => {
    expect(applyHeal({ current: 5, max: 20, temp: 5 }, 3)).toEqual({ current: 8, temp: 5 });
  });

  it('Happy path: healing past the maximum caps the current value', () => {
    expect(applyHeal({ current: 19, max: 20, temp: 0 }, 5)).toEqual({ current: 20, temp: 0 });
  });
});

describe('temporary points', () => {
  it('Edge: a zero, negative or non-finite amount is ignored', () => {
    const hitPoints = { current: 10, max: 20, temp: 2 };

    expect(applyTemp(hitPoints, 0)).toBeUndefined();
    expect(applyTemp(hitPoints, -3)).toBeUndefined();
    expect(applyTemp(hitPoints, Number.NaN)).toBeUndefined();
    expect(applyTemp(hitPoints, Number.POSITIVE_INFINITY)).toBeUndefined();
  });

  it('Edge: an amount below the current temporary points keeps the higher value', () => {
    expect(applyTemp({ current: 10, max: 20, temp: 5 }, 3)).toEqual({ current: 10, temp: 5 });
  });

  it('Edge: an amount equal to the current temporary points keeps them', () => {
    expect(applyTemp({ current: 10, max: 20, temp: 5 }, 5)).toEqual({ current: 10, temp: 5 });
  });

  it('Edge: a stored negative temporary value counts as zero', () => {
    expect(applyTemp({ current: 10, max: 20, temp: -4 }, 2)).toEqual({ current: 10, temp: 2 });
  });

  it('Happy path: a higher amount replaces the temporary points and leaves the current value', () => {
    expect(applyTemp({ current: 10, max: 20, temp: 2 }, 5)).toEqual({ current: 10, temp: 5 });
    expect(applyTemp({ current: 10, max: 20, temp: 0 }, 5)).toEqual({ current: 10, temp: 5 });
  });
});

describe('the health percentage and its band', () => {
  it('Edge: a missing or negative maximum reads as zero percent', () => {
    expect(hitPointsPercent(10, 0)).toBe(0);
    expect(hitPointsPercent(10, -20)).toBe(0);
  });

  it('Edge: a stored value outside the range reads as the nearest end', () => {
    expect(hitPointsPercent(-4, 20)).toBe(0);
    expect(hitPointsPercent(30, 20)).toBe(100);
  });

  it('Edge: the band boundaries are 50 and 25 inclusive', () => {
    expect(hitPointsBand(100)).toBe('high');
    expect(hitPointsBand(50.1)).toBe('high');
    expect(hitPointsBand(50)).toBe('medium');
    expect(hitPointsBand(25)).toBe('medium');
    expect(hitPointsBand(24.9)).toBe('low');
    expect(hitPointsBand(0)).toBe('low');
  });

  it('Happy path: ten of twenty is half health', () => {
    expect(hitPointsPercent(10, 20)).toBe(50);
    expect(hitPointsBand(hitPointsPercent(10, 20))).toBe('medium');
  });
});

describe('the pending preview', () => {
  it('Edge: an unchanged sheet has no preview', () => {
    expect(
      hitPointsPreview({ current: 10, max: 20, temp: 0 }, { current: 10, max: 20, temp: 0 }),
    ).toBeUndefined();
  });

  it('Edge: healing at the maximum has no preview', () => {
    expect(
      hitPointsPreview({ current: 20, max: 20, temp: 0 }, { current: 20, max: 20, temp: 0 }),
    ).toBeUndefined();
  });

  it('Edge: damage fully absorbed by temporary points reads as damage with no current change', () => {
    expect(
      hitPointsPreview({ current: 10, max: 20, temp: 5 }, { current: 10, max: 20, temp: 2 }),
    ).toEqual({
      direction: 'damage',
      current: undefined,
      temp: { before: 5, after: 2, delta: -3 },
    });
  });

  it('Edge: damage below zero reports the floored current value', () => {
    expect(
      hitPointsPreview({ current: 2, max: 20, temp: 0 }, { current: 0, max: 20, temp: 0 }),
    ).toEqual({
      direction: 'damage',
      current: { before: 2, after: 0, delta: -2 },
      temp: undefined,
    });
  });

  it('Edge: damage spilling past the temporary points lists both lines', () => {
    expect(
      hitPointsPreview({ current: 10, max: 20, temp: 5 }, { current: 7, max: 20, temp: 0 }),
    ).toEqual({
      direction: 'damage',
      current: { before: 10, after: 7, delta: -3 },
      temp: { before: 5, after: 0, delta: -5 },
    });
  });

  it('Edge: temporary points gained without a current change read as healing', () => {
    expect(
      hitPointsPreview({ current: 10, max: 20, temp: 0 }, { current: 10, max: 20, temp: 4 }),
    ).toEqual({
      direction: 'heal',
      current: undefined,
      temp: { before: 0, after: 4, delta: 4 },
    });
  });

  it('Happy path: healing reports the gained current value', () => {
    expect(
      hitPointsPreview({ current: 19, max: 20, temp: 0 }, { current: 20, max: 20, temp: 0 }),
    ).toEqual({
      direction: 'heal',
      current: { before: 19, after: 20, delta: 1 },
      temp: undefined,
    });
  });
});

describe('the signed delta', () => {
  it('Edge: no change reads as a bare zero', () => {
    expect(formatHitPointsDelta(0)).toBe('0');
  });

  it('Happy path: a gain carries a plus and a loss a minus sign', () => {
    expect(formatHitPointsDelta(5)).toBe('+5');
    expect(formatHitPointsDelta(-7)).toBe('\u22127');
  });
});

describe('the amount parser', () => {
  it('Edge: empty, blank and unreadable amounts are ignored', () => {
    expect(parseHitPointsAmount('')).toBeUndefined();
    expect(parseHitPointsAmount('   ')).toBeUndefined();
    expect(parseHitPointsAmount('abc')).toBeUndefined();
    expect(parseHitPointsAmount('NaN')).toBeUndefined();
    expect(parseHitPointsAmount('Infinity')).toBeUndefined();
    expect(parseHitPointsAmount('1e999')).toBeUndefined();
  });

  it('Edge: zero and negative amounts are ignored', () => {
    expect(parseHitPointsAmount('0')).toBeUndefined();
    expect(parseHitPointsAmount('-0')).toBeUndefined();
    expect(parseHitPointsAmount('-3')).toBeUndefined();
  });

  it('Edge: a fractional amount rounds to the nearest whole number', () => {
    expect(parseHitPointsAmount('2.5')).toBe(3);
    expect(parseHitPointsAmount('2.4')).toBe(2);
    expect(parseHitPointsAmount('0.4')).toBeUndefined();
  });

  it('Happy path: a positive amount reads as a finite number, surrounding blanks trimmed', () => {
    expect(parseHitPointsAmount(' 4 ')).toBe(4);
    expect(parseHitPointsAmount('+3')).toBe(3);
  });
});
