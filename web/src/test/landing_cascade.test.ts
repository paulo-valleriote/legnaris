import { describe, expect, it } from 'vitest';
import {
  MIGHT_DEFAULT,
  MIGHT_MAX,
  MIGHT_MIN,
  clampMight,
  deriveCascade,
  formatSigned,
} from '../landing/cascade';

describe('landing cascade example', () => {
  const clamped = [
    { name: 'Edge: NaN falls back to the default', input: Number.NaN, want: MIGHT_DEFAULT },
    { name: 'Edge: Infinity falls back to the default', input: Infinity, want: MIGHT_DEFAULT },
    { name: 'Edge: below the minimum clamps up', input: -5, want: MIGHT_MIN },
    { name: 'Edge: zero clamps up to the minimum', input: 0, want: MIGHT_MIN },
    { name: 'Edge: above the maximum clamps down', input: 99, want: MIGHT_MAX },
    { name: 'Edge: a fraction rounds to the nearest whole number', input: 13.6, want: 14 },
    { name: 'Happy path: an in-range whole number is kept', input: 18, want: 18 },
  ];

  for (const tc of clamped) {
    it(`clampMight — ${tc.name}`, () => {
      expect(clampMight(tc.input)).toBe(tc.want);
    });
  }

  const derived = [
    {
      name: 'Happy path: the default derives its dependents',
      might: 14,
      modifier: 2,
      climbing: 5,
      carry: 98,
    },
    { name: 'Edge: ten is the neutral modifier', might: 10, modifier: 0, climbing: 3, carry: 70 },
    {
      name: 'Edge: an odd score rounds the modifier down',
      might: 11,
      modifier: 0,
      climbing: 3,
      carry: 77,
    },
    {
      name: 'Edge: the minimum yields a negative modifier',
      might: MIGHT_MIN,
      modifier: -5,
      climbing: -2,
      carry: 7,
    },
    { name: 'Edge: the maximum', might: MIGHT_MAX, modifier: 10, climbing: 13, carry: 210 },
    {
      name: 'Edge: an out-of-range input derives from its clamped value',
      might: 500,
      modifier: 10,
      climbing: 13,
      carry: 210,
    },
  ];

  for (const tc of derived) {
    it(`deriveCascade — ${tc.name}`, () => {
      expect(deriveCascade(tc.might)).toMatchObject({
        modifier: tc.modifier,
        climbing: tc.climbing,
        carry: tc.carry,
      });
    });
  }

  it('Edge: deriveCascade reports the clamped base score', () => {
    expect(deriveCascade(Number.NaN).might).toBe(MIGHT_DEFAULT);
  });

  it('formatSigned — Happy path: positive values carry a plus', () => {
    expect(formatSigned(4)).toBe('+4');
  });

  it('formatSigned — Edge: zero carries a plus', () => {
    expect(formatSigned(0)).toBe('+0');
  });

  it('formatSigned — Edge: negative values use a true minus sign', () => {
    expect(formatSigned(-3)).toBe('−3');
  });
});
