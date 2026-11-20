import { describe, expect, it } from 'vitest';
import { formatLevelRanges } from '../sheet/preset/fantasy/hidden_levels';
import {
  recoverSlot,
  slotPips,
  slotPool,
  spendSlot,
  type SlotPool,
} from '../sheet/preset/fantasy/slot_pips';

describe('the slot pool', () => {
  it('Edge: a missing total reads as a pool of no slots', () => {
    expect(slotPool(undefined, undefined)).toEqual({ total: 0, expended: 0, remaining: 0 });
  });

  it('Edge: a non-numeric total and expended read as zero', () => {
    expect(slotPool('3', null)).toEqual({ total: 0, expended: 0, remaining: 0 });
    expect(slotPool(Number.NaN, Number.POSITIVE_INFINITY)).toEqual({
      total: 0,
      expended: 0,
      remaining: 0,
    });
  });

  it('Edge: negative counts never leave the pool below zero', () => {
    expect(slotPool(-3, -2)).toEqual({ total: 0, expended: 0, remaining: 0 });
  });

  it('Edge: fractional counts truncate to whole slots', () => {
    expect(slotPool(2.9, 0.7)).toEqual({ total: 2, expended: 0, remaining: 2 });
  });

  it('Edge: an expended count above the total clamps to the total', () => {
    expect(slotPool(2, 5)).toEqual({ total: 2, expended: 2, remaining: 0 });
  });

  it('Happy path: the remaining count is the total minus the expended slots', () => {
    expect(slotPool(3, 1)).toEqual({ total: 3, expended: 1, remaining: 2 });
  });
});

describe('spending a slot', () => {
  it('Edge: a pool of no slots spends nothing and returns the same pool', () => {
    const pool = slotPool(0, 0);

    expect(spendSlot(pool)).toBe(pool);
  });

  it('Edge: a pool with every slot expended spends nothing and returns the same pool', () => {
    const pool = slotPool(2, 2);

    expect(spendSlot(pool)).toBe(pool);
  });

  it('Edge: spending never raises the expended count above the total', () => {
    let pool = slotPool(2, 1);

    pool = spendSlot(pool);
    pool = spendSlot(pool);
    pool = spendSlot(pool);

    expect(pool).toEqual({ total: 2, expended: 2, remaining: 0 });
  });

  it('Happy path: spending one slot lowers the remaining count', () => {
    expect(spendSlot(slotPool(3, 1))).toEqual({ total: 3, expended: 2, remaining: 1 });
  });
});

describe('recovering a slot', () => {
  it('Edge: a pool with nothing expended recovers nothing and returns the same pool', () => {
    const pool = slotPool(3, 0);

    expect(recoverSlot(pool)).toBe(pool);
  });

  it('Edge: recovering never lowers the expended count below zero', () => {
    let pool = slotPool(2, 1);

    pool = recoverSlot(pool);
    pool = recoverSlot(pool);
    pool = recoverSlot(pool);

    expect(pool).toEqual({ total: 2, expended: 0, remaining: 2 });
  });

  it('Happy path: recovering one slot raises the remaining count', () => {
    expect(recoverSlot(slotPool(3, 2))).toEqual({ total: 3, expended: 1, remaining: 2 });
  });

  it('Happy path: a spend followed by a recover returns the first pool', () => {
    const first: SlotPool = slotPool(2, 0);

    expect(recoverSlot(spendSlot(first))).toEqual(first);
  });
});

describe('the hidden level ranges', () => {
  it('Edge: no levels read as an empty label', () => {
    expect(formatLevelRanges([])).toBe('');
  });

  it('Edge: unsorted and repeated levels collapse into one ordered list', () => {
    expect(formatLevelRanges([9, 4, 5, 4])).toBe('4–5, 9');
  });

  it('Edge: a gap splits the levels into a list', () => {
    expect(formatLevelRanges([2, 4, 5, 6])).toBe('2, 4–6');
  });

  it('Happy path: one level draws itself', () => {
    expect(formatLevelRanges([5])).toBe('5');
  });

  it('Happy path: consecutive levels draw one range', () => {
    expect(formatLevelRanges([4, 5, 6, 7, 8, 9])).toBe('4–9');
  });
});

describe('the slot pips', () => {
  it('Edge: a pool of no slots draws no pips', () => {
    expect(slotPips(slotPool(0, 0))).toEqual([]);
  });

  it('Edge: a pool with every slot expended draws every pip as spent', () => {
    expect(slotPips(slotPool(2, 2))).toEqual([
      { index: 0, spent: true },
      { index: 1, spent: true },
    ]);
  });

  it('Happy path: the remaining pips come before the spent ones', () => {
    expect(slotPips(slotPool(3, 1))).toEqual([
      { index: 0, spent: false },
      { index: 1, spent: false },
      { index: 2, spent: true },
    ]);
  });
});
