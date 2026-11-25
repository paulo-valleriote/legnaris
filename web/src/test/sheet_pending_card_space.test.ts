import { describe, expect, it } from 'vitest';
import { reservedBottomSpace } from '../sheet/preset/pending_card';

describe('the space reserved under the Play pending card', () => {
  it('Happy path: the card height and its inset above the viewport edge add up', () => {
    expect(reservedBottomSpace(120, 80)).toBe(212);
  });

  it('Edge: a card resting on the edge reserves only its own height', () => {
    expect(reservedBottomSpace(96, 0)).toBe(108);
  });

  it('Edge: a missing, negative or unbounded measurement reserves nothing', () => {
    expect(reservedBottomSpace(Number.NaN, Number.NaN)).toBe(12);
    expect(reservedBottomSpace(-10, -5)).toBe(12);
    expect(reservedBottomSpace(Number.POSITIVE_INFINITY, 80)).toBe(92);
  });
});
