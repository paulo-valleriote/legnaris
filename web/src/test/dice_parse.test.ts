import { describe, expect, it } from 'vitest';
import { LIMITS, canonicalForm, parseDice } from '../dice';
import { diceVectors } from './dice_vectors';

describe('parseDice against the shared vectors', () => {
  it('Happy path: the limits match the vectors file', () => {
    expect(LIMITS).toEqual(diceVectors.limits);
  });

  it.each(diceVectors.valid)('Happy path: $name ($expr)', ({ expr, canonical }) => {
    const parsed = parseDice(expr);
    expect(parsed.ok).toBe(true);
    if (parsed.ok) {
      expect(canonicalForm(parsed.value)).toBe(canonical);
    }
  });

  it.each(diceVectors.invalid)('Error: $name is refused as $class', ({ expr, class: reason }) => {
    const parsed = parseDice(expr);
    expect(parsed.ok).toBe(false);
    if (!parsed.ok) {
      expect(parsed.error.class).toBe(reason);
    }
  });
});

describe('parseDice terms', () => {
  it('Happy path: reads dice, numbers and Field references with their signs', () => {
    const parsed = parseDice('2d6 - 3 + stats.strength - @.total');
    expect(parsed.ok && parsed.value.terms).toEqual([
      { kind: 'dice', sign: 1, count: 2, faces: 6 },
      { kind: 'number', sign: -1, value: 3 },
      { kind: 'field', sign: 1, relative: false, path: 'stats.strength' },
      { kind: 'field', sign: -1, relative: true, column: 'total' },
    ]);
  });

  it('Happy path: count and faces may each be a Field reference', () => {
    const parsed = parseDice('{stats.count}d8 + 1d{stats.size} + {@.bonus}d6');
    expect(parsed.ok && parsed.value.terms).toEqual([
      {
        kind: 'dice',
        sign: 1,
        count: 0,
        faces: 8,
        countRef: { relative: false, path: 'stats.count' },
      },
      {
        kind: 'dice',
        sign: 1,
        count: 1,
        faces: 0,
        facesRef: { relative: false, path: 'stats.size' },
      },
      {
        kind: 'dice',
        sign: 1,
        count: 0,
        faces: 6,
        countRef: { relative: true, column: 'bonus' },
      },
    ]);
  });

  it('Happy path: the minimum is part of the parsed expression', () => {
    const parsed = parseDice('1d20 + 2 min 5');
    expect(parsed.ok && parsed.value.minimum).toBe(5);
    expect(parsed.ok && canonicalForm(parsed.value)).toBe('1d20 + 2 min 5');
  });

  it('Edge: a parser-built expression without a minimum carries no key', () => {
    const parsed = parseDice('1d20');
    expect(parsed.ok && parsed.value).toEqual({
      terms: [{ kind: 'dice', sign: 1, count: 1, faces: 20 }],
    });
  });

  it('Edge: a bare @ is the Row itself and carries no column', () => {
    const parsed = parseDice('@');
    expect(parsed.ok && parsed.value.terms).toEqual([
      { kind: 'field', sign: 1, relative: true, column: '' },
    ]);
  });

  it('Edge: the length is counted in code points, not UTF-16 units', () => {
    const atLimit = parseDice('🎲'.repeat(LIMITS.maxLength));
    expect(!atLimit.ok && atLimit.error.class).toBe('unreadable_term');
    const over = parseDice('🎲'.repeat(LIMITS.maxLength + 1));
    expect(!over.ok && over.error.class).toBe('too_long');
  });

  it.each(['stats.kl', 'kh.total', '@.kh1', 'khazad', 'kl2x', 'dexterity', 'd1'])(
    'Edge: %s keeps its meaning (a Field path or a die), not a bare d or keep token',
    (expr) => {
      expect(parseDice(`1d20 + ${expr}`).ok).toBe(true);
    },
  );

  it('Edge: a non-string input is refused instead of throwing', () => {
    const parsed = parseDice(undefined as unknown as string);
    expect(!parsed.ok && parsed.error.class).toBe('empty');
  });

  it('Edge: parsing is idempotent on the canonical form', () => {
    for (const { canonical } of diceVectors.valid) {
      const parsed = parseDice(canonical);
      expect(parsed.ok && canonicalForm(parsed.value)).toBe(canonical);
    }
  });
});
