import { describe, expect, it } from 'vitest';
import {
  atScrollEnd,
  currentSectionKey,
  prefersReducedMotion,
  presentSections,
  presentTargets,
  sectionAnchor,
  type PresetSectionGroup,
  type PresetSectionTarget,
} from '../sheet/preset/section_nav_targets';

describe('the section nav helpers', () => {
  it('Edge: an empty target list marks nothing', () => {
    expect(currentSectionKey([], 8, false)).toBeUndefined();
    expect(currentSectionKey([], 8, true)).toBeUndefined();
  });

  it('Edge: a page that cannot scroll is never at its end', () => {
    expect(atScrollEnd({ scrollTop: 0, clientHeight: 768, scrollHeight: 768 })).toBe(false);
    expect(atScrollEnd({ scrollTop: 0, clientHeight: 900, scrollHeight: 768 })).toBe(false);
  });

  it('Happy path: the end marks the last section even when its heading sits below the line', () => {
    const positions = [
      { key: 'attacks', top: -400 },
      { key: 'rules', top: 700 },
    ];

    expect(currentSectionKey(positions, 8, true)).toBe('rules');
    expect(atScrollEnd({ scrollTop: 4232, clientHeight: 768, scrollHeight: 5000 })).toBe(true);
  });

  it('Edge: no heading above the line keeps the first section marked', () => {
    expect(currentSectionKey([{ key: 'attacks', top: 120 }], 8, false)).toBe('attacks');
  });

  it('Edge: an empty slot or a slot without a heading contributes no target', () => {
    const root = document.createElement('div');
    const empty = document.createElement('div');
    empty.setAttribute('data-preset-section', 'combat');
    const headingless = document.createElement('div');
    headingless.setAttribute('data-preset-section', 'rules');
    headingless.append(document.createElement('p'));
    const present = document.createElement('div');
    present.setAttribute('data-preset-section', 'attacks');
    const title = document.createElement('h2');
    title.textContent = 'Attacks';
    present.append(title);
    root.append(empty, headingless, present);

    expect(presentSections(root, 'combat')).toEqual([
      { key: 'attacks', label: 'Attacks', sectionKey: 'attacks' },
    ]);
  });

  it('Edge: a browser without matchMedia reads as no reduced-motion preference', () => {
    const original = window.matchMedia;
    Object.defineProperty(window, 'matchMedia', { configurable: true, value: undefined });
    try {
      expect(prefersReducedMotion()).toBe(false);
    } finally {
      Object.defineProperty(window, 'matchMedia', {
        configurable: true,
        writable: true,
        value: original,
      });
    }
  });

  function sectionWrappers(keys: readonly string[]): HTMLElement {
    const root = document.createElement('div');
    for (const key of keys) {
      const wrapper = document.createElement('div');
      wrapper.setAttribute('data-preset-section', key);
      wrapper.append(document.createElement('p'));
      root.append(wrapper);
    }
    return root;
  }

  function group(id: string, label: string, sections: readonly string[]): PresetSectionGroup {
    return { id, label, sections };
  }

  const groupCases: ReadonlyArray<{
    readonly name: string;
    readonly present: readonly string[];
    readonly groups: readonly PresetSectionGroup[];
    readonly skipKey?: string;
    readonly expected: readonly PresetSectionTarget[];
  }> = [
    {
      name: 'Happy path: each group resolves to its first present section in group order',
      present: ['abilities', 'hitPoints', 'inventory'],
      groups: [
        group('abilities', 'Abilities', ['abilities']),
        group('combat', 'Combat', ['hitPoints', 'attacks']),
        group('items', 'Items', ['inventory']),
      ],
      expected: [
        { key: 'abilities', label: 'Abilities', sectionKey: 'abilities' },
        { key: 'combat', label: 'Combat', sectionKey: 'hitPoints' },
        { key: 'items', label: 'Items', sectionKey: 'inventory' },
      ],
    },
    {
      name: 'Edge: a group with no present section is skipped',
      present: ['abilities'],
      groups: [
        group('abilities', 'Abilities', ['abilities']),
        group('combat', 'Combat', ['hitPoints', 'attacks']),
      ],
      expected: [{ key: 'abilities', label: 'Abilities', sectionKey: 'abilities' }],
    },
    {
      name: 'Edge: a section skipped above the bar falls through to the next present one',
      present: ['hitPoints', 'attacks'],
      skipKey: 'hitPoints',
      groups: [group('combat', 'Combat', ['hitPoints', 'attacks'])],
      expected: [{ key: 'combat', label: 'Combat', sectionKey: 'attacks' }],
    },
    {
      name: 'Edge: no present section yields no targets',
      present: [],
      groups: [group('abilities', 'Abilities', ['abilities'])],
      expected: [],
    },
  ];

  it.each(groupCases)('$name', ({ present, groups, skipKey, expected }) => {
    expect(presentTargets(sectionWrappers(present), groups, skipKey)).toEqual(expected);
  });

  it('Edge: a group whose only section renders nothing is skipped', () => {
    const root = sectionWrappers([]);
    const empty = document.createElement('div');
    empty.setAttribute('data-preset-section', 'combat');
    root.append(empty);

    expect(presentTargets(root, [group('combat', 'Combat', ['combat'])], undefined)).toEqual([]);
  });

  it('Happy path: an anchor prefers its section heading and falls back to its wrapper', () => {
    const root = document.createElement('div');
    const headingSection = document.createElement('div');
    headingSection.setAttribute('data-preset-section', 'attacks');
    const heading = document.createElement('h2');
    heading.textContent = 'Attacks';
    headingSection.append(heading);
    const plainSection = document.createElement('div');
    plainSection.setAttribute('data-preset-section', 'rules');
    plainSection.append(document.createElement('p'));
    root.append(headingSection, plainSection);

    expect(sectionAnchor(root, 'attacks')).toBe(heading);
    expect(sectionAnchor(root, 'rules')).toBe(plainSection);
    expect(sectionAnchor(root, 'missing')).toBeUndefined();
  });
});
