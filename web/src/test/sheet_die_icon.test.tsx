import { describe, expect, it } from 'vitest';
import { render } from '@testing-library/react';
import { parseDice } from '../dice/parse';
import { DEFAULT_DIE_SHAPE, DieIcon, dieShapeFor, dieShapeForFaces } from '../sheet/die_icon';

function parsed(text: string) {
  const result = parseDice(text);
  if (!result.ok) {
    throw new Error(`fixture "${text}" must parse`);
  }
  return result.value;
}

describe('dieShapeForFaces', () => {
  const tests: { name: string; faces: number; want: string }[] = [
    { name: 'Happy path: d4', faces: 4, want: 'd4' },
    { name: 'Happy path: d6', faces: 6, want: 'd6' },
    { name: 'Happy path: d8', faces: 8, want: 'd8' },
    { name: 'Happy path: d10', faces: 10, want: 'd10' },
    { name: 'Happy path: d12', faces: 12, want: 'd12' },
    { name: 'Happy path: d20', faces: 20, want: 'd20' },
    { name: 'Edge: d100 borrows the d10', faces: 100, want: 'd10' },
    { name: 'Edge: d2 falls back to the d20', faces: 2, want: 'd20' },
    { name: 'Edge: d7 falls back to the d20', faces: 7, want: 'd20' },
    { name: 'Edge: d1000 falls back to the d20', faces: 1000, want: 'd20' },
    { name: 'Edge: zero faces falls back to the d20', faces: 0, want: 'd20' },
  ];
  for (const tc of tests) {
    it(tc.name, () => {
      expect(dieShapeForFaces(tc.faces)).toBe(tc.want);
    });
  }
});

describe('dieShapeFor', () => {
  const tests: { name: string; text?: string; want: string }[] = [
    { name: 'Happy path: a single die', text: '1d12', want: 'd12' },
    { name: 'Happy path: count and modifier', text: '2d8+3', want: 'd8' },
    { name: 'Edge: the leading die wins over later dice', text: '1d4+1d20', want: 'd4' },
    { name: 'Edge: a leading number does not count as a die', text: '3+1d6', want: 'd6' },
    { name: 'Edge: a subtracted leading die still names the shape', text: '-1d10+5', want: 'd10' },
    { name: 'Edge: an expression with no dice draws the default', text: '5', want: 'd20' },
    { name: 'Edge: no expression draws the default', want: 'd20' },
  ];
  for (const tc of tests) {
    it(tc.name, () => {
      expect(dieShapeFor(tc.text === undefined ? undefined : parsed(tc.text))).toBe(tc.want);
    });
  }
});

describe('DieIcon', () => {
  it('Edge: draws the d20 when no shape is given', () => {
    const { container } = render(<DieIcon />);
    expect(container.querySelector('svg')?.dataset.dieShape).toBe(DEFAULT_DIE_SHAPE);
  });

  it('Happy path: draws the requested shape, hidden from assistive technology', () => {
    const { container } = render(<DieIcon shape="d8" />);
    const svg = container.querySelector('svg');
    expect(svg?.dataset.dieShape).toBe('d8');
    expect(svg?.getAttribute('aria-hidden')).toBe('true');
  });

  it('Edge: off adds the strike-through over the silhouette', () => {
    const on = render(<DieIcon shape="d12" />).container.querySelectorAll('path').length;
    const off = render(<DieIcon shape="d12" off />).container.querySelectorAll('path').length;
    expect(off).toBe(on + 1);
  });
});
