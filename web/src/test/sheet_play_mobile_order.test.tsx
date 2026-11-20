import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { createApiClient } from '../api/client';
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

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const presetCss = readFileSync(path.resolve(here, '../sheet/preset/preset.css'), 'utf8');
const sheetCss = readFileSync(path.resolve(here, '../sheet/sheet.css'), 'utf8');

function narrowSheetCss(): string {
  return sheetCss.slice(sheetCss.indexOf('@media (max-width: 39.99rem)'));
}

const LONG_PLAYER = 'Alexandria-Cassandra Montgomery-Worthington the Third';

function attackNode(slot: number): Record<string, unknown> {
  return {
    label: `Attack ${slot}`,
    columns: ['name', 'damage', 'bonus'],
    children: {
      name: {
        schema: {
          type: 'text',
          label: `Attack ${slot} name`,
          is_editable: true,
          default: 0,
          max_length: 40,
          column_label: 'Name',
        },
      },
      damage: {
        schema: {
          type: 'text',
          label: `Attack ${slot} damage`,
          is_editable: true,
          default: 0,
          max_length: 60,
          column_label: 'Damage',
        },
      },
      bonus: {
        schema: {
          type: 'number',
          label: `Attack ${slot} bonus`,
          is_editable: true,
          default: 0,
          min: -5,
          max: 20,
          column_label: 'Bonus',
        },
      },
    },
  };
}

function fullFieldTree(): Record<string, unknown> {
  return {
    ...fantasyFieldTree(),
    attacks: { label: 'Attacks', children: { attack1: attackNode(1) } },
    spells: {
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
    },
    rules: {
      label: 'Rules',
      children: {
        modifierBase: {
          schema: {
            type: 'number',
            label: 'Ability modifier base',
            is_editable: true,
            default: 10,
          },
        },
      },
    },
  };
}

function fullData(identity: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    ...fantasySheetData(identity),
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
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake };
}

function sectionOrder(): string[] {
  return [...document.querySelectorAll<HTMLElement>('[data-preset-section]')]
    .filter((element) => element.childElementCount > 0)
    .map((element) => element.dataset.presetSection ?? '');
}

describe('the mobile Play section order', () => {
  it('Happy path: phone DOM order is combat, attacks, spells, abilities, saves, skills, rules', async () => {
    await openFantasy(390);

    expect(sectionOrder()).toEqual([
      'combat',
      'attacks',
      'spells',
      'stats',
      'saves',
      'skills',
      'rules',
    ]);
  });

  it('Happy path: keyboard tab order follows the mobile visual order', async () => {
    await openFantasy(390);

    const sections = [...document.querySelectorAll<HTMLElement>('[data-preset-section]')];
    const position = new Map(sections.map((section, index) => [section, index]));
    const body = screen.getByTestId('sheet-play');
    const focusables = [
      ...body.querySelectorAll<HTMLElement>('button, input, select, textarea, a[href], [tabindex]'),
    ].filter(
      (element) =>
        !element.hasAttribute('disabled') &&
        element.getAttribute('tabindex') !== '-1' &&
        element.closest('[hidden]') === null,
    );

    const reached = focusables.map((element) => {
      const section = element.closest<HTMLElement>('[data-preset-section]');
      if (section !== null) {
        return position.get(section) ?? -1;
      }
      const strip = element.closest<HTMLElement>('[data-testid="sheet-section-nav"]');
      const previous = strip?.previousElementSibling;
      return previous instanceof HTMLElement ? (position.get(previous) ?? -1) : -1;
    });

    expect(focusables.length).toBeGreaterThan(0);
    expect(reached.filter((index) => index < 0)).toEqual([]);
    expect(reached).toEqual([...reached].sort((a, b) => a - b));
    expect(focusables.some((element) => Number(element.getAttribute('tabindex')) > 0)).toBe(false);
  });

  it('Edge: a sheet without spells hides the section and keeps the remaining order', async () => {
    const fields = fullFieldTree();
    delete fields.spells;
    await openFantasy(390, { fields });

    expect(screen.queryByTestId('sheet-spells')).not.toBeInTheDocument();
    expect(sectionOrder()).toEqual(['combat', 'attacks', 'stats', 'saves', 'skills', 'rules']);
    expect(presetCss).toMatch(/\.lg-preset__section:empty\s*\{[^}]*display:\s*none/);
  });

  it('Edge: 1440 keeps the three-column body without the mobile wrappers', async () => {
    await openFantasy(1440);

    expect(document.querySelector('[data-preset-section]')).toBeNull();
    const columns = [...document.querySelectorAll<HTMLElement>('.lg-preset__column')];
    expect(columns).toHaveLength(3);
    expect(columns[0].querySelector('.lg-preset-abilities')).not.toBeNull();
    expect(columns[1].querySelector('[data-testid="sheet-play-saves"]')).not.toBeNull();
    expect(columns[1].querySelector('[data-testid="sheet-play-skills"]')).not.toBeNull();
    expect(columns[2].querySelector('[data-testid="sheet-attacks"]')).not.toBeNull();
    expect(columns[2].querySelector('[data-testid="sheet-spells"]')).not.toBeNull();
  });

  it('Edge: 1100 keeps the two-column arrangement without the mobile wrappers', async () => {
    await openFantasy(1100);

    expect(document.querySelector('[data-preset-section]')).toBeNull();
    expect(document.querySelectorAll('.lg-preset__column')).toHaveLength(3);
    expect(presetCss.slice(presetCss.indexOf('@media (max-width: 68.75rem)'))).toMatch(
      /grid-template-columns:\s*180px minmax\(0, 1fr\)/,
    );
  });
});

describe('the one-line hero', () => {
  it('Edge: on a phone the hero comes before the sheet actions, so the title sits near the top', () => {
    expect(narrowSheetCss()).toMatch(
      /\.lg-sheet__heading:has\(\.lg-preset-hero\)\s*\{[^}]*flex-direction:\s*column;/,
    );
  });

  it('Happy path: the meta line and played by share one line container at 390', async () => {
    await openFantasy(390);

    const meta = document.querySelector('.lg-preset-hero__meta');
    const played = document.querySelector('.lg-preset-hero__played');

    expect(meta).not.toBeNull();
    expect(played).not.toBeNull();
    expect(meta?.parentElement).toBe(played?.parentElement);
    expect(meta?.parentElement).toHaveClass('lg-preset-hero__line');
  });

  it('Edge: empty ancestry and origin keep the line with their invitations', async () => {
    await openFantasy(390, { data: fullData({ ancestry: '', origin: '' }) });

    const line = document.querySelector('.lg-preset-hero__line');
    expect(line).not.toBeNull();
    expect(screen.getByRole('button', { name: 'Ancestry' })).toHaveTextContent('Add Ancestry');
    expect(screen.getByRole('button', { name: 'Origin' })).toHaveTextContent('Add Origin');
    expect(line?.querySelector('.lg-preset-hero__played')).not.toBeNull();
  });

  it('Edge: a long player name wraps without overflow', async () => {
    await openFantasy(390, { data: fullData({ playerName: LONG_PLAYER }) });

    expect(screen.getByRole('button', { name: 'Player name' })).toHaveTextContent(LONG_PLAYER);
    expect(presetCss).toMatch(
      /\.lg-preset-hero__played\s*\{[^}]*min-width:\s*0[^}]*overflow-wrap:\s*anywhere/,
    );
    expect(presetCss).toMatch(/\.lg-preset-hero__meta-item\s*\{[^}]*max-width:\s*100%/);
  });

  it('Edge: the hero line is a wrapping row that shrinks', () => {
    expect(presetCss).toMatch(
      /\.lg-preset-hero__line\s*\{[^}]*display:\s*flex[^}]*flex-wrap:\s*wrap/,
    );
    expect(presetCss).toMatch(/\.lg-preset-hero__line\s*\{[^}]*min-width:\s*0/);
  });
});
