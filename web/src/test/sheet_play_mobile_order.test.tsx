import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { createApiClient } from '../api/client';
import { fifthCharacterConfig } from '../sheet/preset/compose/configs/fifth_character';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { setViewportWidth } from './match_media';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const presetCss = readFileSync(path.resolve(here, '../sheet/preset/preset.css'), 'utf8');

function narrowSections(): string[] {
  return (fifthCharacterConfig.narrowLayout ?? []).flatMap((block) =>
    'columns' in block ? block.columns.flat() : [...block.sections],
  );
}

async function openFifth(width: number) {
  setViewportWidth(width);
  const fake = createFakeTransport(
    successOutcome(fifthCharacterSheetPayload()),
    successOutcome(fifthCharacterTemplatePayload()),
    successOutcome({}),
  );
  renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
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

describe('the mobile preset section order', () => {
  it('Happy path: phone DOM order follows the narrow layout', async () => {
    await openFifth(390);

    expect(sectionOrder()).toEqual(narrowSections());
  });

  it('Happy path: keyboard tab order follows the mobile visual order', async () => {
    await openFifth(390);

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

  it('Edge: 1440 draws the combat columns above the ability and skill columns', async () => {
    await openFifth(1440);

    expect(document.querySelectorAll('[data-preset-section]').length).toBeGreaterThan(0);
    const columns = [...document.querySelectorAll<HTMLElement>('.lg-preset__column')];
    expect(columns).toHaveLength(4);
    expect(sectionOrder().slice(0, 3)).toEqual(['identity', 'vitals', 'hitPoints']);
    expect(sectionOrder().indexOf('hitPointsSetup')).toBeGreaterThan(
      sectionOrder().indexOf('conditions'),
    );
    expect(sectionOrder().indexOf('conditions')).toBeLessThan(sectionOrder().indexOf('armorSetup'));
    expect(document.querySelector('[data-preset-section="abilities"]')).not.toBeNull();
    expect(document.querySelector('[data-preset-section="skills"]')).not.toBeNull();
    expect(document.querySelector('[data-preset-section="spells"]')).not.toBeNull();
  });

  it('Edge: 1100 keeps the column blocks and their narrower track rule', async () => {
    await openFifth(1100);

    expect(document.querySelectorAll('[data-preset-section]').length).toBeGreaterThan(0);
    expect(document.querySelectorAll('.lg-preset__column')).toHaveLength(4);
    expect(presetCss.slice(presetCss.indexOf('@media (max-width: 68.75rem)'))).toMatch(
      /grid-template-columns:\s*180px minmax\(0, 1fr\)/,
    );
  });
});
