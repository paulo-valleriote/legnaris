import { describe, expect, it } from 'vitest';
import { clampPercent, parseClampedInt, UNBOUNDED } from '../sheet/preset/fantasy/parse_number';

describe('the shared clamped integer parser', () => {
  it('Edge: empty and blank text never reports a value', () => {
    expect(parseClampedInt('', 0, 10)).toBeUndefined();
    expect(parseClampedInt('   ', 0, 10)).toBeUndefined();
  });

  it('Edge: non-finite and unreadable text never reports a value', () => {
    expect(parseClampedInt('abc', 0, 10)).toBeUndefined();
    expect(parseClampedInt('NaN', 0, 10)).toBeUndefined();
    expect(parseClampedInt('Infinity', 0, 10)).toBeUndefined();
    expect(parseClampedInt('-Infinity', 0, 10)).toBeUndefined();
    expect(parseClampedInt('1e999', 0, 10)).toBeUndefined();
  });

  it('Edge: decimals round before the clamp', () => {
    expect(parseClampedInt('3.4', 0, 10)).toBe(3);
    expect(parseClampedInt('3.6', 0, 10)).toBe(4);
    expect(parseClampedInt('-2.5', -10, 0)).toBe(-2);
  });

  it('Edge: values below the minimum and above the maximum clamp to the bounds', () => {
    expect(parseClampedInt('-99', -5, 20)).toBe(-5);
    expect(parseClampedInt('99', -5, 20)).toBe(20);
  });

  it('Edge: an unbounded maximum keeps a large typed value intact', () => {
    expect(parseClampedInt('400000', 0, UNBOUNDED)).toBe(400000);
    expect(parseClampedInt('999', 1, UNBOUNDED)).toBe(999);
  });

  it('Edge: a negative zero is normalised to zero', () => {
    expect(Object.is(parseClampedInt('-0', -5, 20), 0)).toBe(true);
    expect(Object.is(parseClampedInt('-0.2', -5, 20), 0)).toBe(true);
  });

  it('Happy path: a signed value is read and trimmed as written', () => {
    expect(parseClampedInt(' 7 ', -5, 20)).toBe(7);
    expect(parseClampedInt('+3', -5, 20)).toBe(3);
    expect(parseClampedInt('-4', -5, 20)).toBe(-4);
    expect(parseClampedInt('0', 0, 10)).toBe(0);
  });
});

describe('the shared percent clamp', () => {
  it('Edge: negative, zero and above-full values clamp to the 0..100 range', () => {
    expect(clampPercent(-1)).toBe(0);
    expect(clampPercent(-0.5)).toBe(0);
    expect(clampPercent(0)).toBe(0);
    expect(clampPercent(100)).toBe(100);
    expect(clampPercent(100.5)).toBe(100);
    expect(clampPercent(500)).toBe(100);
  });

  it('Happy path: a value inside the range is returned untouched', () => {
    expect(clampPercent(42.5)).toBe(42.5);
    expect(clampPercent(0.1)).toBe(0.1);
    expect(clampPercent(99.9)).toBe(99.9);
  });
});
