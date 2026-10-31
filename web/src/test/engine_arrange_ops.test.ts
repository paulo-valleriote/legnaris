import { describe, expect, it } from 'vitest';
import { moveSection, pinRow, resetArrangement, unpinRow } from '../engine/arrange_ops';
import { arrangementToLayout, arrangementView, resolveArrangement } from '../engine/layout';
import { arrangeSections } from './layout_fixtures';

const sections = arrangeSections();
const base = resolveArrangement(sections, undefined);

const rowKeys = (arrangement: typeof base, sectionKey: string) =>
  arrangementView(sections, arrangement)
    .find((entry) => entry.section.key === sectionKey)
    ?.rows.map((row) => row.key);

describe('moveSection', () => {
  it('Happy path: moves a Section to an index', () => {
    expect(moveSection(base, 'gamma', 0).sectionOrder).toEqual(['gamma', 'alpha', 'beta']);
    expect(moveSection(base, 'alpha', 2).sectionOrder).toEqual(['beta', 'gamma', 'alpha']);
  });

  it('Edge: the place it already holds is the same arrangement', () => {
    expect(moveSection(base, 'alpha', 0)).toBe(base);
  });

  it('Edge: an index past either end is clamped', () => {
    expect(moveSection(base, 'alpha', 99).sectionOrder).toEqual(['beta', 'gamma', 'alpha']);
    expect(moveSection(base, 'gamma', -5).sectionOrder).toEqual(['gamma', 'alpha', 'beta']);
  });

  it('Edge: an unknown Section, NaN or Infinity changes nothing', () => {
    expect(moveSection(base, 'ghost', 0)).toBe(base);
    expect(moveSection(base, 'alpha', Number.NaN)).toBe(base);
    expect(moveSection(base, 'alpha', Number.POSITIVE_INFINITY)).toBe(base);
  });

  it('Edge: a fractional index is truncated', () => {
    expect(moveSection(base, 'gamma', 0.9).sectionOrder).toEqual(['gamma', 'alpha', 'beta']);
  });

  it('Edge: the input arrangement is never mutated', () => {
    const before = [...base.sectionOrder];
    moveSection(base, 'gamma', 0);
    expect(base.sectionOrder).toEqual(before);
  });

  it('Edge: moving a Section leaves the pins untouched', () => {
    const pinned = pinRow(base, 'alpha.two');

    expect(moveSection(pinned, 'alpha', 2).pinned).toEqual(['alpha.two']);
  });
});

describe('pinRow and unpinRow', () => {
  it('Happy path: pinning appends in pin order', () => {
    const pinned = pinRow(pinRow(base, 'gamma.solo'), 'alpha.two');

    expect(pinned.pinned).toEqual(['gamma.solo', 'alpha.two']);
  });

  it('Edge: pinning an already pinned Row is the same arrangement (idempotent)', () => {
    const pinned = pinRow(base, 'alpha.two');

    expect(pinRow(pinned, 'alpha.two')).toBe(pinned);
  });

  it('Edge: unpinning a Row that is not pinned is the same arrangement', () => {
    expect(unpinRow(base, 'alpha.two')).toBe(base);
    expect(unpinRow(base, 'ghost')).toBe(base);
  });

  it('Happy path: unpinning returns the Row to its Template position', () => {
    const pinned = pinRow(base, 'alpha.three');
    expect(rowKeys(pinned, 'alpha')).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);

    const unpinned = unpinRow(pinned, 'alpha.three');

    expect(unpinned.pinned).toEqual([]);
    expect(rowKeys(unpinned, 'alpha')).toEqual(['alpha.one', 'alpha.two', 'alpha.three']);
  });

  it('Edge: unpinning one of several keeps the others in pin order', () => {
    const pinned = pinRow(pinRow(pinRow(base, 'alpha.three'), 'alpha.one'), 'gamma.solo');

    expect(unpinRow(pinned, 'alpha.one').pinned).toEqual(['alpha.three', 'gamma.solo']);
  });

  it('Edge: pinning, unpinning and re-pinning moves the Row to the end of the pin order', () => {
    const pinned = pinRow(pinRow(base, 'alpha.one'), 'alpha.two');
    const repinned = pinRow(unpinRow(pinned, 'alpha.one'), 'alpha.one');

    expect(rowKeys(repinned, 'alpha')).toEqual(['alpha.two', 'alpha.one', 'alpha.three']);
  });

  it('Edge: the input arrangement is never mutated', () => {
    const pinned = pinRow(base, 'alpha.one');
    unpinRow(pinned, 'alpha.one');

    expect(base.pinned).toEqual([]);
    expect(pinned.pinned).toEqual(['alpha.one']);
  });
});

describe('resetArrangement', () => {
  it('Happy path: Template order with nothing pinned', () => {
    expect(resetArrangement(sections)).toEqual({
      sectionOrder: ['alpha', 'beta', 'gamma'],
      pinned: [],
    });
  });

  it('Edge: a reset serializes to null, which is what Done sends', () => {
    expect(arrangementToLayout(sections, resetArrangement(sections))).toBeNull();
  });

  it('Edge: no Sections resets to an empty arrangement', () => {
    expect(resetArrangement([])).toEqual({ sectionOrder: [], pinned: [] });
  });
});
