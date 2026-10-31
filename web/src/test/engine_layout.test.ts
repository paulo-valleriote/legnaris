import { describe, expect, it } from 'vitest';
import type { Layout } from '../api/models/layout';
import type { Arrangement } from '../engine/layout';
import {
  arrangementToLayout,
  arrangementView,
  arrangementsEqual,
  resolveArrangement,
} from '../engine/layout';
import { ARRANGE_ROW_KEYS, arrangeSections } from './layout_fixtures';

const sections = arrangeSections();

function layout(partial: Partial<Layout> = {}): Layout {
  return { sectionOrder: [], pinned: [], ...partial };
}

const viewRows = (arrangement: Arrangement) =>
  Object.fromEntries(
    arrangementView(sections, arrangement).map((entry) => [
      entry.section.key,
      entry.rows.map((row) => row.key),
    ]),
  );

describe('resolveArrangement', () => {
  it('Edge: an absent layout is the Template order with nothing pinned', () => {
    expect(resolveArrangement(sections, undefined)).toEqual({
      sectionOrder: ['alpha', 'beta', 'gamma'],
      pinned: [],
    });
  });

  it('Edge: an empty layout equals an absent one', () => {
    expect(
      arrangementsEqual(
        resolveArrangement(sections, layout()),
        resolveArrangement(sections, undefined),
      ),
    ).toBe(true);
  });

  it('Edge: no sections resolves to an empty arrangement', () => {
    expect(resolveArrangement([], layout({ sectionOrder: ['alpha'], pinned: ['x'] }))).toEqual({
      sectionOrder: [],
      pinned: [],
    });
  });

  it('Happy path: a complete layout is honoured', () => {
    expect(
      resolveArrangement(
        sections,
        layout({ sectionOrder: ['gamma', 'alpha', 'beta'], pinned: ['beta.stats', 'alpha.two'] }),
      ),
    ).toEqual({ sectionOrder: ['gamma', 'alpha', 'beta'], pinned: ['beta.stats', 'alpha.two'] });
  });

  it('Edge: a partial section list appends the missing sections in Template order', () => {
    expect(resolveArrangement(sections, layout({ sectionOrder: ['gamma'] })).sectionOrder).toEqual([
      'gamma',
      'alpha',
      'beta',
    ]);
  });

  it('Edge: a partial pinned list is valid as it stands', () => {
    expect(resolveArrangement(sections, layout({ pinned: ['beta#pick'] })).pinned).toEqual([
      'beta#pick',
    ]);
  });

  it('Edge: stale section and pin keys are ignored', () => {
    expect(
      resolveArrangement(
        sections,
        layout({ sectionOrder: ['ghost', 'beta'], pinned: ['nope', 'alpha.ghost', 'gamma.solo'] }),
      ),
    ).toEqual({ sectionOrder: ['beta', 'alpha', 'gamma'], pinned: ['gamma.solo'] });
  });

  it('Edge: a pin naming a Section instead of a Row is ignored', () => {
    expect(resolveArrangement(sections, layout({ pinned: ['alpha', 'beta'] })).pinned).toEqual([]);
  });

  it('Edge: duplicates keep their first occurrence in every list', () => {
    expect(
      resolveArrangement(
        sections,
        layout({
          sectionOrder: ['gamma', 'alpha', 'gamma'],
          pinned: ['alpha.one', 'gamma.solo', 'alpha.one'],
        }),
      ),
    ).toEqual({ sectionOrder: ['gamma', 'alpha', 'beta'], pinned: ['alpha.one', 'gamma.solo'] });
  });

  it('Edge: keys named like Object prototype members resolve as ordinary absent keys', () => {
    expect(
      resolveArrangement(
        sections,
        layout({ sectionOrder: ['__proto__', 'constructor', 'toString'], pinned: ['__proto__'] }),
      ),
    ).toEqual({ sectionOrder: ['alpha', 'beta', 'gamma'], pinned: [] });
  });

  it('Edge: a retired row_order smuggled onto the object is not read', () => {
    const legacy = { sectionOrder: [], pinned: [], rowOrder: { alpha: ['alpha.three'] } };

    expect(viewRows(resolveArrangement(sections, legacy)).alpha).toEqual(ARRANGE_ROW_KEYS.alpha);
  });
});

describe('arrangementsEqual', () => {
  it('Edge: differs on any section order or pin difference, pin order included', () => {
    const base = resolveArrangement(sections, undefined);
    const pinnedTwo = resolveArrangement(sections, layout({ pinned: ['alpha.one', 'alpha.two'] }));

    expect(arrangementsEqual(base, resolveArrangement(sections, undefined))).toBe(true);
    expect(
      arrangementsEqual(base, resolveArrangement(sections, layout({ sectionOrder: ['beta'] }))),
    ).toBe(false);
    expect(
      arrangementsEqual(base, resolveArrangement(sections, layout({ pinned: ['alpha.one'] }))),
    ).toBe(false);
    expect(
      arrangementsEqual(
        pinnedTwo,
        resolveArrangement(sections, layout({ pinned: ['alpha.two', 'alpha.one'] })),
      ),
    ).toBe(false);
  });
});

describe('arrangementToLayout', () => {
  it('Edge: the Template default serializes to null (a reset)', () => {
    expect(arrangementToLayout(sections, resolveArrangement(sections, undefined))).toBeNull();
  });

  it('Edge: a layout that only restates the Template order serializes to null', () => {
    const restated = resolveArrangement(
      sections,
      layout({ sectionOrder: ['alpha', 'beta', 'gamma'] }),
    );

    expect(arrangementToLayout(sections, restated)).toBeNull();
  });

  it('Happy path: only the lists that deviate are emitted', () => {
    const arranged = resolveArrangement(
      sections,
      layout({ sectionOrder: ['gamma', 'alpha', 'beta'], pinned: ['alpha.one'] }),
    );

    expect(arrangementToLayout(sections, arranged)).toEqual({
      sectionOrder: ['gamma', 'alpha', 'beta'],
      pinned: ['alpha.one'],
    });
  });

  it('Edge: a pin alone serializes without a section list', () => {
    expect(
      arrangementToLayout(
        sections,
        resolveArrangement(sections, layout({ pinned: ['beta#pick'] })),
      ),
    ).toEqual({ sectionOrder: [], pinned: ['beta#pick'] });
  });

  it('Edge: a section order alone serializes without pins', () => {
    expect(
      arrangementToLayout(
        sections,
        resolveArrangement(sections, layout({ sectionOrder: ['beta'] })),
      ),
    ).toEqual({ sectionOrder: ['beta', 'alpha', 'gamma'], pinned: [] });
  });

  it('Edge: a pin that names no Row never reaches the wire', () => {
    const stale: Arrangement = {
      sectionOrder: ['alpha', 'beta', 'gamma'],
      pinned: ['ghost', 'alpha.one'],
    };

    expect(arrangementToLayout(sections, stale)).toEqual({
      sectionOrder: [],
      pinned: ['alpha.one'],
    });
    expect(arrangementToLayout(sections, { ...stale, pinned: ['ghost'] })).toBeNull();
  });

  it('Edge: serialization round-trips through resolution, pin order included', () => {
    const arranged = resolveArrangement(
      sections,
      layout({ sectionOrder: ['beta'], pinned: ['gamma.solo', 'alpha.two'] }),
    );
    const wire = arrangementToLayout(sections, arranged);

    expect(wire).not.toBeNull();
    expect(arrangementsEqual(resolveArrangement(sections, wire ?? undefined), arranged)).toBe(true);
  });
});

describe('arrangementView', () => {
  it('Edge: the Template default draws every Row once, in Template order', () => {
    expect(viewRows(resolveArrangement(sections, undefined))).toEqual(ARRANGE_ROW_KEYS);
  });

  it('Happy path: a pinned Row rises to the top of its own Section', () => {
    const view = viewRows(resolveArrangement(sections, layout({ pinned: ['alpha.three'] })));

    expect(view.alpha).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);
    expect(view.beta).toEqual(ARRANGE_ROW_KEYS.beta);
  });

  it('Edge: pinned Rows are ordered by pin time, not by Template order', () => {
    const view = viewRows(
      resolveArrangement(sections, layout({ pinned: ['alpha.three', 'alpha.one'] })),
    );

    expect(view.alpha).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);
  });

  it('Edge: pins in several Sections apply independently and never cross Sections', () => {
    const view = viewRows(
      resolveArrangement(
        sections,
        layout({ pinned: ['beta#pick', 'alpha.two', 'beta.stats', 'gamma.solo'] }),
      ),
    );

    expect(view.alpha).toEqual(['alpha.two', 'alpha.one', 'alpha.three']);
    expect(view.beta).toEqual(['beta#pick', 'beta.stats', 'beta.note']);
    expect(view.gamma).toEqual(['gamma.solo']);
  });

  it('Edge: every Row of a Section pinned keeps the Section, ordered by pin time', () => {
    const view = viewRows(
      resolveArrangement(sections, layout({ pinned: ['alpha.three', 'alpha.one', 'alpha.two'] })),
    );

    expect(view.alpha).toEqual(['alpha.three', 'alpha.one', 'alpha.two']);
    expect(Object.keys(view)).toEqual(['alpha', 'beta', 'gamma']);
  });

  it('Edge: every Row of the sheet pinned still draws every Section and Row once', () => {
    const all = Object.values(ARRANGE_ROW_KEYS).flat();
    const view = viewRows(resolveArrangement(sections, layout({ pinned: [...all].reverse() })));

    expect(Object.values(view).flat().sort()).toEqual([...all].sort());
    expect(view.gamma).toEqual(['gamma.solo']);
  });

  it('Edge: unpinning returns the Row to its Template position', () => {
    const pinned = resolveArrangement(sections, layout({ pinned: ['alpha.three'] }));

    expect(viewRows({ ...pinned, pinned: [] }).alpha).toEqual(ARRANGE_ROW_KEYS.alpha);
  });

  it('Edge: a pinned key that names no Row is ignored on read', () => {
    const view = viewRows(
      resolveArrangement(sections, layout({ pinned: ['alpha.ghost', 'alpha.two'] })),
    );

    expect(view.alpha).toEqual(['alpha.two', 'alpha.one', 'alpha.three']);
  });

  it('Edge: the Row flags mark exactly the pinned ones', () => {
    const rows = arrangementView(
      sections,
      resolveArrangement(sections, layout({ pinned: ['alpha.two'] })),
    )[0].rows;

    expect(rows.map((row) => [row.key, row.pinned, row.sectionKey])).toEqual([
      ['alpha.two', true, 'alpha'],
      ['alpha.one', false, 'alpha'],
      ['alpha.three', false, 'alpha'],
    ]);
  });

  it('Happy path: Sections follow the section order', () => {
    const view = arrangementView(
      sections,
      resolveArrangement(sections, layout({ sectionOrder: ['gamma', 'beta'] })),
    );

    expect(view.map((entry) => entry.section.key)).toEqual(['gamma', 'beta', 'alpha']);
  });

  it('Edge: an arrangement that predates a Template change still draws every current Section and Row', () => {
    const stale: Arrangement = { sectionOrder: ['gamma', 'ghost'], pinned: ['ghost.row'] };
    const view = arrangementView(sections, stale);

    expect(view.map((entry) => entry.section.key)).toEqual(['gamma', 'alpha', 'beta']);
    expect(view.flatMap((entry) => entry.rows)).toHaveLength(7);
  });

  it('Edge: no Sections draws nothing', () => {
    expect(arrangementView([], { sectionOrder: ['alpha'], pinned: [] })).toEqual([]);
  });
});
