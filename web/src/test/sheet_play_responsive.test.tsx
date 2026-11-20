import { readFileSync, readdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { parseTemplateDetail } from '../api/models/template';
import { message } from '../l10n/strings';
import { InlineEdit, NARROW_QUERY } from '../sheet/inline_edit';
import { editableLeafPaths } from '../sheet/sheet_edit';
import { parseInitiative } from '../sheet/preset/fantasy/armor_class';
import {
  parseHitPointsCurrent,
  parseHitPointsMaximum,
  parseHitPointsTemp,
} from '../sheet/preset/fantasy/hit_points';
import { parseProficiencyBonus } from '../sheet/preset/fantasy/identity';
import { parseXpTotal } from '../sheet/preset/fantasy/xp_progress';
import { FANTASY_SHEET_ID, fantasySheetPayload, fantasyTemplatePayload } from './fantasy_fixtures';
import { createFakeTransport, successOutcome } from './fake_transport';
import { setViewportWidth } from './match_media';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const presetCss = readFileSync(path.resolve(here, '../sheet/preset/preset.css'), 'utf8');
const diceCss = readFileSync(path.resolve(here, '../sheet/dice.css'), 'utf8');
const xpCss = readFileSync(path.resolve(here, '../sheet/preset/fantasy/xp_progress.css'), 'utf8');
const abilitiesCss = readFileSync(
  path.resolve(here, '../sheet/preset/fantasy/abilities.css'),
  'utf8',
);
const presetCssDir = path.resolve(here, '../sheet/preset');
const presetNarrowCss = readdirSync(path.join(presetCssDir, 'fantasy'))
  .filter((name) => name.endsWith('.css'))
  .map((name) => path.join(presetCssDir, 'fantasy', name));

async function openFantasy(width = 1280) {
  setViewportWidth(width);
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload()),
    successOutcome(fantasyTemplatePayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function editPath(path: string): HTMLElement {
  const element = document.querySelector<HTMLElement>(`[data-edit-path="${path}"]`);
  if (element === null) {
    throw new Error(`no editing control for ${path}`);
  }
  return element;
}

function mobileCss(css: string): string {
  return css.slice(css.indexOf('@media (max-width: 40rem)'));
}

function mediaBlock(css: string, query: string): string {
  const start = css.indexOf(query);
  const open = css.indexOf('{', start);
  let depth = 0;
  for (let index = open; index < css.length; index += 1) {
    if (css[index] === '{') {
      depth += 1;
    } else if (css[index] === '}') {
      depth -= 1;
      if (depth === 0) {
        return css.slice(start, index + 1);
      }
    }
  }
  throw new Error(`unclosed media block ${query}`);
}

function ruleBlock(css: string, selector: string): string {
  const start = css.indexOf(`${selector} {`);
  const end = css.indexOf('}', start);
  if (start === -1 || end === -1) {
    throw new Error(`no rule for ${selector}`);
  }
  return css.slice(start, end + 1);
}

describe('the Play editing surface', () => {
  it('Happy path: every editable leaf of the fantasy_v1 fixture has an editing control', async () => {
    const { user } = await openFantasy();
    const template = parseTemplateDetail(fantasyTemplatePayload());
    const paths = [...editableLeafPaths(template.fields).keys()];

    await user.click(screen.getByTestId('sheet-xp-trigger'));
    await user.click(screen.getByTestId('sheet-ac-shield'));

    const missing = paths.filter((leaf) => {
      const control = document.querySelector<HTMLElement>(`[data-edit-path="${leaf}"]`);
      return control === null || control.hasAttribute('disabled');
    });

    expect(paths.length).toBeGreaterThan(0);
    expect(missing).toEqual([]);
  });

  it('Happy path: identity, temporary hit points, maximum hit points and initiative are editable', async () => {
    const { user } = await openFantasy();

    await user.click(editPath('identity.proficiencyBonus'));
    await user.clear(screen.getByRole('textbox', { name: 'Proficiency bonus' }));
    await user.type(screen.getByRole('textbox', { name: 'Proficiency bonus' }), '3{Enter}');
    expect(editPath('identity.proficiencyBonus')).toHaveTextContent('+3Prof');

    await user.click(editPath('combat.hitPointsTemp'));
    const temp = screen.getByRole('textbox', { name: 'Temporary hit points' });
    await user.clear(temp);
    await user.type(temp, '5{Enter}');
    expect(screen.getByTestId('sheet-hp-temp-value')).toHaveTextContent('+5 temp');

    await user.click(editPath('combat.hitPointsMax'));
    const max = screen.getByRole('textbox', { name: 'Hit points (maximum)' });
    await user.clear(max);
    await user.type(max, '25{Enter}');
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 25');

    await user.click(editPath('combat.initiative'));
    const initiative = screen.getByRole('textbox', { name: 'Initiative' });
    await user.clear(initiative);
    await user.type(initiative, '7{Enter}');
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Initiative 3 → 7');
  });
});

describe('inline editing on a narrow screen', () => {
  it('Happy path: a text edit opens a bottom sheet and Enter commits the value', async () => {
    const { user } = await openFantasy(360);

    await user.click(screen.getByRole('button', { name: 'Character name' }));

    const dialog = screen.getByRole('dialog', { name: 'Character name' });
    const input = within(dialog).getByRole('textbox', { name: 'Character name' });
    expect(input).toHaveAttribute('type', 'text');
    expect(input).toHaveAttribute('inputmode', 'text');

    await user.clear(input);
    await user.type(input, 'Brenna{Enter}');

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Character name' })).toHaveTextContent('Brenna');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeEnabled();
  });

  it('Happy path: a numeric edit opens a bottom sheet with a numeric input and commits', async () => {
    const { user } = await openFantasy(360);

    await user.click(editPath('combat.speed'));

    const dialog = screen.getByRole('dialog', { name: 'Speed' });
    const input = within(dialog).getByRole('spinbutton', { name: 'Speed' });
    expect(input).toHaveAttribute('type', 'number');
    expect(input).toHaveAttribute('inputmode', 'numeric');

    await user.clear(input);
    await user.type(input, '35{Enter}');

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Speed' })).toHaveTextContent('35');
  });

  it('Edge: Escape on the bottom sheet cancels and keeps the stored value', async () => {
    const { user } = await openFantasy(360);

    await user.click(screen.getByRole('button', { name: 'Ancestry' }));
    const input = screen.getByRole('textbox', { name: 'Ancestry' });
    await user.clear(input);
    await user.type(input, 'Half-elf{Escape}');

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Ancestry' })).toHaveTextContent('Elf');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
  });

  it('Edge: the close button on the bottom sheet cancels the edit', async () => {
    const { user } = await openFantasy(360);

    await user.click(screen.getByRole('button', { name: 'Origin' }));
    const dialog = screen.getByRole('dialog', { name: 'Origin' });
    await user.click(within(dialog).getByRole('button', { name: message('common.close') }));

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Origin' })).toHaveTextContent('Wanderer');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
  });

  it('Edge: a numeric bottom sheet strips the display plus sign from its value', async () => {
    setViewportWidth(360);
    const user = userEvent.setup();
    const onCommit = vi.fn();
    render(<InlineEdit value="+2" label="Bonus" inputMode="numeric" onCommit={onCommit} />);

    await user.click(screen.getByRole('button', { name: 'Bonus' }));
    const input = screen.getByRole('spinbutton', { name: 'Bonus' });
    expect(input).toHaveValue(2);

    await user.clear(input);
    await user.type(input, '5{Enter}');
    expect(onCommit).toHaveBeenCalledWith('5');
  });

  it('Edge: a wide viewport keeps the input inline and opens no sheet', async () => {
    const { user } = await openFantasy(1280);

    await user.click(screen.getByRole('button', { name: 'Character name' }));

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Character name' })).toBeInTheDocument();
  });
});

describe('the responsive Play layout', () => {
  it('Happy path: the Play body groups abilities and saves with skills into columns', async () => {
    await openFantasy();

    const columns = document.querySelectorAll('.lg-preset__column');
    expect(columns).toHaveLength(3);
    expect(columns[0].querySelector('.lg-preset-abilities')).not.toBeNull();
    expect(columns[1].querySelector('[data-testid="sheet-play-saves"]')).not.toBeNull();
    expect(columns[1].querySelector('[data-testid="sheet-play-skills"]')).not.toBeNull();
    expect(screen.getByTestId('sheet-combat-strip').closest('.lg-preset__column')).toBeNull();
  });

  it('Edge: the body is three columns, two near 1100px and one on a phone', () => {
    expect(presetCss).toMatch(
      /\.lg-preset__columns\s*\{[^}]*grid-template-columns:\s*200px minmax\(0, 1fr\) minmax\(0, 1\.1fr\)/,
    );
    const twoColumn = presetCss.slice(presetCss.indexOf('@media (max-width: 68.75rem)'));
    expect(twoColumn).toMatch(/grid-template-columns:\s*180px minmax\(0, 1fr\)/);
    expect(mobileCss(presetCss)).toMatch(/grid-template-columns:\s*minmax\(0, 1fr\)/);
  });

  it('Edge: no Play column can force a horizontal scroll at 360px', () => {
    expect(presetCss).toMatch(/\.lg-preset\s*\{[^}]*max-width:\s*100%/);
    expect(presetCss).toMatch(/\.lg-preset__columns\s*\{[^}]*min-width:\s*0/);
    expect(presetCss).toMatch(/\.lg-preset__column[\s\S]*?min-width:\s*0/);
  });

  it('Edge: reduced motion switches off the hit point and experience bar animations', () => {
    const reduced = presetCss.slice(presetCss.indexOf('@media (prefers-reduced-motion: reduce)'));
    expect(reduced).toMatch(/\.lg-hp-card__fill\s*\{[^}]*transition:\s*none/);
    const xpReduced = xpCss.slice(xpCss.indexOf('@media (prefers-reduced-motion: reduce)'));
    expect(xpReduced).toMatch(/\.lg-xp__fill\s*\{[^}]*transition:\s*none/);
  });

  it('Edge: the roll toast sits on the bottom edge on a phone and keeps no motion', () => {
    expect(diceCss).toMatch(/\.lg-dice-panel\s*\{[^}]*inset-block-end:/);
    expect(mobileCss(diceCss)).toMatch(/\.lg-dice-panel\s*\{[^}]*inset-inline:\s*0/);
    expect(diceCss).toMatch(
      /@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.lg-dice-panel\s*\{[^}]*animation:\s*none/,
    );
  });
});

describe('ability names at every Play width', () => {
  const widths = [1440, 1100, 390, 360] as const;
  const names = ['Strength', 'Dexterity', 'Constitution', 'Intelligence', 'Wisdom', 'Charisma'];

  it.each(widths.map((width) => [width]))(
    'Happy path: the six ability names are drawn whole at %ipx',
    async (width) => {
      await openFantasy(width);

      const drawn = screen
        .getAllByTestId(/^sheet-ability-/)
        .map((item) => item.querySelector('.lg-ability__name')?.textContent);

      expect(drawn).toEqual(names);
    },
  );

  it('Edge: the 1440 layout keeps the name beside the medallion', () => {
    expect(ruleBlock(abilitiesCss, '.lg-ability')).toMatch(
      /grid-template-columns:\s*minmax\(0, 1fr\) auto/,
    );
    expect(ruleBlock(abilitiesCss, '.lg-ability__name')).toMatch(/white-space:\s*nowrap/);
  });

  it('Edge: at 1100, 390 and 360 the name moves above the medallion and never ellipsises', () => {
    for (const width of [1100, 390, 360]) {
      expect(width).toBeLessThanOrEqual(68.75 * 16);
    }

    const stacked = mediaBlock(abilitiesCss, '@media (max-width: 68.75rem)');
    expect(stacked).toMatch(/\.lg-ability\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\)/);
    const name = stacked.slice(stacked.indexOf('.lg-ability__name'));
    expect(name).toMatch(/white-space:\s*normal/);
    expect(name).toMatch(/overflow-wrap:\s*anywhere/);
    expect(name).not.toMatch(/text-overflow:\s*ellipsis/);
  });

  it('Edge: an ability card can never widen the column or the page at 360px', () => {
    expect(abilitiesCss).toMatch(/\.lg-ability\s*\{[^}]*min-width:\s*0/);
    expect(abilitiesCss).toMatch(/\.lg-ability__name\s*\{[^}]*min-width:\s*0/);
    expect(abilitiesCss).toMatch(/\.lg-ability__name\s*\{[^}]*white-space:\s*nowrap/);
  });
});

describe('the new editing value parsers', () => {
  it('Edge: the new editing parsers clamp to the template ranges but keep values uncapped where the schema has no maximum', () => {
    expect(parseProficiencyBonus('')).toBeUndefined();
    expect(parseProficiencyBonus('abc')).toBeUndefined();
    expect(parseProficiencyBonus('Infinity')).toBeUndefined();
    expect(parseInitiative('')).toBeUndefined();
    expect(parseInitiative('NaN')).toBeUndefined();
    expect(parseHitPointsCurrent('')).toBeUndefined();
    expect(parseHitPointsMaximum('1e999')).toBeUndefined();
    expect(parseHitPointsTemp('abc')).toBeUndefined();
    expect(parseXpTotal('')).toBeUndefined();
    expect(parseXpTotal('abc')).toBeUndefined();
    expect(parseXpTotal('Infinity')).toBeUndefined();
  });

  it('Happy path: parsers round and clamp to the template ranges', () => {
    expect(parseProficiencyBonus('3.6')).toBe(4);
    expect(parseProficiencyBonus('1')).toBe(2);
    expect(parseProficiencyBonus('9')).toBe(6);
    expect(parseInitiative('-99')).toBe(-5);
    expect(parseInitiative('99')).toBe(20);
    expect(parseInitiative('-0')).toBe(0);
    expect(parseHitPointsCurrent('-4')).toBe(0);
    expect(parseHitPointsMaximum('0')).toBe(1);
    expect(parseHitPointsTemp('-1')).toBe(0);
    expect(parseXpTotal('-1')).toBe(0);
    expect(parseXpTotal(' 250.4 ')).toBe(250);
  });

  it('Edge: hit points and experience keep a large typed value instead of an arbitrary cap', () => {
    expect(parseHitPointsCurrent('999')).toBe(999);
    expect(parseHitPointsMaximum('999')).toBe(999);
    expect(parseHitPointsTemp('999')).toBe(999);
    expect(parseXpTotal('400000')).toBe(400000);
    expect(parseXpTotal('355001')).toBe(355001);
  });
});

describe('the inline edit breakpoint', () => {
  it('Edge: the only preset breakpoints are the phone query and the 1100 layout', () => {
    const narrowRem = NARROW_QUERY.match(/max-width:\s*([\d.]+rem)/)?.[1];
    expect(narrowRem).toBe('40rem');
    expect(presetCss).toContain(`@media (max-width: ${narrowRem})`);

    const breakpoints = new Set<string>();
    for (const file of presetNarrowCss) {
      const css = readFileSync(file, 'utf8');
      for (const query of css.matchAll(/@media \(max-width: ([\d.]+rem)\)/g)) {
        breakpoints.add(query[1]);
      }
    }
    expect([...breakpoints].sort()).toEqual(['40rem', '68.75rem']);
  });
});
