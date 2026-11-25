import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import {
  FANTASY_SHEET_ID,
  fantasyFieldTree,
  fantasySheetData,
  fantasySheetPayload,
  fantasyTemplatePayload,
} from './fantasy_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { setViewportWidth } from './match_media';
import { renderApp } from './render';
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

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const navCss = readFileSync(path.resolve(here, '../sheet/preset/section_nav.css'), 'utf8');

function attackNode(): Record<string, unknown> {
  return {
    label: 'Attacks',
    children: {
      attack1: {
        label: 'Attack 1',
        columns: ['name', 'damage', 'bonus'],
        children: {
          name: {
            schema: {
              type: 'text',
              label: 'Attack 1 name',
              is_editable: true,
              default: 0,
              max_length: 40,
              column_label: 'Name',
            },
          },
          damage: {
            schema: {
              type: 'text',
              label: 'Attack 1 damage',
              is_editable: true,
              default: 0,
              max_length: 60,
              column_label: 'Damage',
            },
          },
          bonus: {
            schema: {
              type: 'number',
              label: 'Attack 1 bonus',
              is_editable: true,
              default: 0,
              min: -5,
              max: 20,
              column_label: 'Bonus',
            },
          },
        },
      },
    },
  };
}

function spellsNode(): Record<string, unknown> {
  return {
    label: 'Spells',
    children: {
      atWill: {
        schema: {
          type: 'text',
          label: 'At-will spells',
          is_editable: true,
          default: 0,
          max_length: 200,
        },
      },
    },
  };
}

function rulesNode(): Record<string, unknown> {
  return {
    label: 'Rules',
    children: {
      modifierBase: {
        schema: { type: 'number', label: 'Ability modifier base', is_editable: true, default: 10 },
      },
    },
  };
}

function fullFieldTree(): Record<string, unknown> {
  return {
    ...fantasyFieldTree(),
    attacks: attackNode(),
    spells: spellsNode(),
    rules: rulesNode(),
  };
}

function fullData(): Record<string, unknown> {
  return {
    ...fantasySheetData(),
    attacks: { attack1: { name: 'Longsword', damage: '1d8', bonus: 5 } },
    spells: { atWill: 'Light' },
    rules: { modifierBase: 10 },
  };
}

async function openFantasy(
  width: number,
  options: { data?: Record<string, unknown>; fields?: Record<string, unknown> } = {},
) {
  setViewportWidth(width);
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload({ data: options.data ?? fullData() })),
    successOutcome(fantasyTemplatePayload({ fields: options.fields ?? fullFieldTree() })),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function nav() {
  return screen.queryByTestId('sheet-section-nav');
}

function chipLabels(): string[] {
  const surface = nav();
  if (surface === null) {
    return [];
  }
  return within(surface)
    .getAllByRole('button')
    .map((button) => button.textContent ?? '');
}

function chip(name: string) {
  return screen.getByRole('button', { name: messageWith('sheet.sectionLink', { label: name }) });
}

function currentChip(): string | undefined {
  const current = document.querySelector<HTMLElement>(
    '[data-testid="sheet-section-nav"] [aria-current="location"]',
  );
  return current?.textContent ?? undefined;
}

function sectionsRoot(): HTMLElement {
  const root = document.querySelector<HTMLElement>('.lg-preset');
  if (root === null) {
    throw new Error('no preset root');
  }
  return root;
}

function headingTitle(element: HTMLElement): string {
  const title = element.querySelector<HTMLElement>(':scope > span:first-child');
  return (title ?? element).textContent?.trim() ?? '';
}

function heading(label: string): HTMLElement {
  const found = [...sectionsRoot().querySelectorAll<HTMLElement>('h2')].find(
    (element) => headingTitle(element) === label,
  );
  if (found === undefined) {
    throw new Error(`no heading ${label}`);
  }
  return found;
}

function placeHeadings(tops: Readonly<Record<string, number>>): void {
  for (const [label, top] of Object.entries(tops)) {
    heading(label).getBoundingClientRect = () =>
      ({ top, bottom: top + 40, height: 40, left: 0, right: 360, width: 360 }) as DOMRect;
  }
}

function placePageAtEnd(scrollY: number, scrollHeight = 5000, clientHeight = 768): void {
  Object.defineProperty(document.documentElement, 'scrollHeight', {
    configurable: true,
    value: scrollHeight,
  });
  Object.defineProperty(document.documentElement, 'clientHeight', {
    configurable: true,
    value: clientHeight,
  });
  Object.defineProperty(window, 'scrollY', { configurable: true, value: scrollY });
}

afterEach(() => {
  Reflect.deleteProperty(Element.prototype, 'scrollIntoView');
  Reflect.deleteProperty(document.documentElement, 'scrollHeight');
  Reflect.deleteProperty(document.documentElement, 'clientHeight');
  Reflect.deleteProperty(window, 'scrollY');
});

describe('the Play section nav', { timeout: 15_000 }, () => {
  it('Edge: a sheet without spells draws no spells chip on a phone or a desktop', async () => {
    const fields = fullFieldTree();
    delete fields.spells;
    delete fields.attacks;
    delete fields.rules;
    await openFantasy(390, { fields });

    expect(nav()).not.toBeNull();
    expect(chipLabels().some((label) => label.includes('Spellcasting'))).toBe(false);
    expect(chipLabels()).not.toContain('Attacks');
    expect(chipLabels()).not.toContain('Table rules');

    setViewportWidth(1280);
    await waitFor(() => expect(nav()).not.toBeNull());
    expect(chipLabels().some((label) => label.includes('Spellcasting'))).toBe(false);
    expect(chipLabels()).not.toContain('Attacks');
    expect(chipLabels()).not.toContain('Table rules');
  });

  it('Happy path: the desktop nav is a sticky jump bar at the top of the body', async () => {
    await openFantasy(1280);

    const surface = nav();
    expect(surface).not.toBeNull();
    expect(surface?.parentElement?.classList.contains('lg-preset__body')).toBe(true);
    expect(surface?.parentElement?.firstElementChild).toBe(surface);
    expect(chipLabels()).toEqual([
      'Stats',
      'Saving throws',
      'Skills',
      'Attacks',
      'Spellcasting',
      'Table rules',
    ]);
  });

  it('Edge: the desktop jump bar is token-only, sticky and scrolls its chips horizontally', async () => {
    await openFantasy(1280);

    expect(navCss).toMatch(
      /@media \(min-width: 40\.01rem\)[\s\S]*?\.lg-preset-nav\s*\{[^}]*position:\s*sticky/,
    );
    expect(navCss).toMatch(/\.lg-preset-nav\s*\{[^}]*background-color:\s*var\(--lg-color-bg\)/);
    expect(navCss).toMatch(/\.lg-preset-nav__list\s*\{[^}]*overflow-x:\s*auto/);
    expect(navCss).toMatch(
      /\[data-preset-section\][\s\S]*?scroll-margin-block-start:\s*calc\(var\(--lg-size-control\)/,
    );
  });

  it('Happy path: the chips follow the layout order and sit right under the combat strip', async () => {
    await openFantasy(390);

    expect(chipLabels()).toEqual([
      'Attacks',
      'Spellcasting',
      'Stats',
      'Saving throws',
      'Skills',
      'Table rules',
    ]);
    const surface = nav();
    expect(surface?.closest('[data-preset-section]')).toBeNull();
    expect(surface?.previousElementSibling).toBe(
      document.querySelector('[data-preset-section="combat"]'),
    );
    expect(within(surface as HTMLElement).getByRole('navigation')).toHaveAttribute(
      'aria-label',
      message('sheet.sections'),
    );
  });

  it('Happy path: tapping a chip reveals its heading and moves focus there', async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView =
      scrollIntoView as unknown as typeof Element.prototype.scrollIntoView;
    const { user } = await openFantasy(390);

    await user.click(chip('Saving throws'));

    const target = heading('Saving throws');
    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'smooth' });
    expect(document.activeElement).toBe(target);
    expect(target).toHaveAttribute('tabindex', '-1');
  });

  it('Edge: reduced motion jumps instantly', async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView =
      scrollIntoView as unknown as typeof Element.prototype.scrollIntoView;
    const original = window.matchMedia;
    window.matchMedia = ((media: string) =>
      media.includes('prefers-reduced-motion')
        ? ({
            media,
            matches: true,
            onchange: null,
            addEventListener: () => undefined,
            removeEventListener: () => undefined,
            addListener: () => undefined,
            removeListener: () => undefined,
            dispatchEvent: () => true,
          } as unknown as MediaQueryList)
        : original(media)) as typeof window.matchMedia;
    try {
      const { user } = await openFantasy(390);
      await user.click(chip('Attacks'));
    } finally {
      window.matchMedia = original;
    }

    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'auto' });
  });

  it('Edge: scrolling marks the last heading above the line, not the one below it', async () => {
    await openFantasy(390);
    placeHeadings({ 'Saving throws': -80, Skills: 120, 'Table rules': 640 });
    placePageAtEnd(0, 5000, 768);

    fireEvent.scroll(window);

    await waitFor(() => expect(currentChip()).toBe('Saving throws'));
  });

  it('Edge: a short last section still marks itself once the page reaches the bottom', async () => {
    await openFantasy(390);
    placeHeadings({ Skills: -400, 'Table rules': 700 });
    placePageAtEnd(4232);

    fireEvent.scroll(window);

    await waitFor(() => expect(currentChip()).toBe('Table rules'));
  });

  it('Edge: the chips are 44px, scroll horizontally and keep every chip a tab stop', async () => {
    await openFantasy(390);

    expect(navCss).toMatch(
      /\.lg-preset-nav__chip\s*\{[^}]*min-block-size:\s*var\(--lg-size-control\)/,
    );
    expect(navCss).toMatch(/\.lg-preset-nav__list\s*\{[^}]*overflow-x:\s*auto/);
    expect(navCss).toMatch(
      /\.lg-preset__section h2\s*\{[^}]*scroll-margin-block-start:\s*calc\(var\(--lg-space-96\)/,
    );
    expect(
      within(nav() as HTMLElement)
        .getAllByRole('button')
        .every((button) => button.tabIndex === 0),
    ).toBe(true);
  });
});

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
