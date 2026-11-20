import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
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
import { rollName } from './dice_names';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const presetDir = path.resolve(here, '../sheet/preset');
const readPresetCss = (name: string) => readFileSync(path.join(presetDir, name), 'utf8');

const rollableCss = readPresetCss('rollable_value.css');
const presetCss = readPresetCss('preset.css');
const abilitiesCss = readPresetCss('fantasy/abilities.css');
const attacksCss = readPresetCss('fantasy/attacks.css');
const combatCss = readPresetCss('fantasy/combat_strip.css');
const savesCss = readPresetCss('fantasy/saves_skills.css');
const spellsCss = readPresetCss('fantasy/spells.css');
const rulesCss = readPresetCss('fantasy/rules.css');
const playSectionCss = readPresetCss('fantasy/play_section.css');
const statBlockCss = readPresetCss('fantasy/stat_block.css');
const tokensCss = readFileSync(path.resolve(here, '../theme/tokens.css'), 'utf8');

const NARROW = '@media (max-width: 40rem)';

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function ruleBody(css: string, selector: string): string {
  const match = css.match(new RegExp(`${escapeRegExp(selector)}\\s*\\{([^}]*)\\}`));
  if (match === null) {
    throw new Error(`no rule for ${selector}`);
  }
  return match[1];
}

function narrowCss(css: string): string {
  const blocks: string[] = [];
  for (const match of css.matchAll(new RegExp(`${escapeRegExp(NARROW)}\\s*\\{`, 'g'))) {
    const open = (match.index ?? 0) + match[0].length - 1;
    let depth = 0;
    for (let index = open; index < css.length; index += 1) {
      if (css[index] === '{') {
        depth += 1;
      } else if (css[index] === '}') {
        depth -= 1;
        if (depth === 0) {
          blocks.push(css.slice(open + 1, index));
          break;
        }
      }
    }
  }
  return blocks.join('\n');
}

function tokenSizes(): Map<string, number> {
  const sizes = new Map<string, number>();
  for (const match of tokensCss.matchAll(/(--lg-type-\d+):\s*(\d+)px/g)) {
    sizes.set(match[1], Number(match[2]));
  }
  return sizes;
}

const sizes = tokenSizes();

function resolveFontSize(value: string): number {
  const trimmed = value.trim();
  const px = trimmed.match(/^(\d+)px$/);
  if (px !== null) {
    return Number(px[1]);
  }
  const token = trimmed.match(/^var\((--lg-type-\d+)\)$/);
  if (token !== null) {
    const size = sizes.get(token[1]);
    if (size !== undefined) {
      return size;
    }
  }
  throw new Error(`unresolved font size: ${value}`);
}

function fontSize(css: string, selector: string): number {
  const body = ruleBody(css, selector);
  const match = body.match(/font-size:\s*([^;]+);/);
  if (match === null) {
    throw new Error(`no font size for ${selector}`);
  }
  return resolveFontSize(match[1]);
}

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
    attacks: {
      label: 'Attacks',
      children: {
        attack1: attackNode(1),
        attack2: attackNode(2),
      },
    },
    spells: {
      label: 'Spells',
      children: {
        attackBonus: {
          schema: {
            type: 'number',
            label: 'Spell attack bonus',
            formula: '1 + 4',
            is_editable: false,
            default: 0,
          },
        },
        saveDC: {
          schema: {
            type: 'number',
            label: 'Spell save DC',
            formula: '8 + 5',
            is_editable: false,
            default: 0,
          },
        },
        atWill: {
          schema: {
            type: 'text',
            label: 'At-will spells',
            is_editable: true,
            default: 0,
            max_length: 200,
          },
        },
        list: {
          schema: {
            type: 'text',
            label: 'Spell list',
            is_editable: true,
            default: 0,
            max_length: 2000,
          },
        },
      },
    },
  };
}

function fullSheetData(): Record<string, unknown> {
  return {
    ...fantasySheetData({
      modifiers: { strengthMod: 0, wisdomMod: -3 },
      saves: { wisdom: -3 },
      skills: { insight: { bonus: -1, total: -2 } },
      combat: { initiative: -1 },
    }),
    attacks: {
      attack1: { name: 'Longsword', damage: '1d8 + 3', bonus: 0 },
      attack2: { name: 'Dagger', damage: '1d4 - 1', bonus: -2 },
    },
    spells: {
      attackBonus: 0,
      saveDC: 13,
      atWill: 'Light',
      list: 'Magic missile',
    },
  };
}

async function openSheet(fields: Record<string, unknown>, data: Record<string, unknown>) {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload({ data })),
    successOutcome(fantasyTemplatePayload({ fields })),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

async function openFull() {
  return openSheet(fullFieldTree(), fullSheetData());
}

async function openDefault() {
  return openSheet(fantasyFieldTree(), fantasySheetData());
}

async function openWithSpells() {
  const spells = fullFieldTree().spells;
  return openSheet(
    { ...fantasyFieldTree(), spells },
    { ...fantasySheetData(), spells: fullSheetData().spells },
  );
}

function rollableTotals(): HTMLElement[] {
  return [...document.querySelectorAll<HTMLElement>('[data-rollable="true"]')];
}

describe('the one rollable affordance', () => {
  it('Edge: a zero or negative total keeps the rollable style', async () => {
    await openFull();

    const zeroModifier = within(screen.getByTestId('sheet-ability-strength')).getByRole('button', {
      name: rollName('Strength modifier'),
    });
    const negativeModifier = within(screen.getByTestId('sheet-ability-wisdom')).getByRole(
      'button',
      { name: rollName('Wisdom modifier') },
    );
    const negativeSave = within(screen.getByTestId('sheet-save-wisdom')).getByRole('button', {
      name: rollName('Wisdom save'),
    });
    const negativeSkill = within(screen.getByTestId('sheet-skill-insight')).getByRole('button', {
      name: rollName('Insight'),
    });
    const negativeInitiative = within(screen.getByTestId('sheet-initiative')).getByRole('button', {
      name: rollName('Initiative'),
    });
    const zeroAttack = within(screen.getByTestId('sheet-attack-1')).getByRole('button', {
      name: messageWith('sheet.play.attackRoll', { name: 'Longsword', value: '+0' }),
    });
    const zeroSpellAttack = within(screen.getByTestId('sheet-spell-attack')).getByRole('button', {
      name: rollName('Spell attack bonus'),
    });

    for (const total of [
      zeroModifier,
      negativeModifier,
      negativeSave,
      negativeSkill,
      negativeInitiative,
      zeroAttack,
      zeroSpellAttack,
    ]) {
      expect(total).toHaveClass('lg-rollable--button');
      expect(total).toHaveAttribute('data-rollable', 'true');
    }
    expect(zeroModifier).toHaveTextContent('+0');
    expect(negativeModifier).toHaveTextContent('-3');
    expect(negativeSave).toHaveTextContent('-3');
    expect(negativeSkill).toHaveTextContent('-2');
    expect(negativeInitiative).toHaveTextContent('-1');
    expect(zeroAttack).toHaveTextContent('+0');
    expect(zeroSpellAttack).toHaveTextContent('+0');
  }, 15000);

  it('Edge: toggling proficiency changes the dot only, never the total style', async () => {
    const { user } = await openFull();
    const row = screen.getByTestId('sheet-save-wisdom');
    const dot = within(row).getByRole('button', {
      name: messageWith('sheet.play.proficiencyFor', { label: 'Wisdom save' }),
    });
    const total = within(row).getByRole('button', { name: rollName('Wisdom save') });
    const before = total.className;
    const text = total.textContent;

    expect(dot).toHaveAttribute('aria-pressed', 'false');
    await user.click(dot);

    expect(dot).toHaveAttribute('aria-pressed', 'true');
    expect(total.className).toBe(before);
    expect(total).toHaveTextContent(text ?? '');
    expect(total).toHaveAttribute('data-rollable', 'true');
  }, 15000);

  it('Edge: switching to Edit removes every rollable total style', async () => {
    const { user } = await openDefault();
    expect(rollableTotals().length).toBeGreaterThan(0);

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));

    expect(rollableTotals()).toEqual([]);
    expect(screen.getByRole('textbox', { name: 'Strength' })).toBeInTheDocument();
  }, 15000);

  it('Happy path: every rollable total in Play shares the one button class', async () => {
    await openFull();

    const totals = rollableTotals();
    expect(totals.length).toBeGreaterThanOrEqual(20);
    expect(totals.every((total) => total.classList.contains('lg-rollable--button'))).toBe(true);
  }, 15000);
});

describe('the spell text edit affordance', () => {
  it('Edge: at-will and spell-list text draw no edit underline until hover or focus', async () => {
    await openWithSpells();

    for (const testId of ['sheet-spell-atWill', 'sheet-spell-list']) {
      const text = screen.getByTestId(testId).querySelector('.lg-inline-edit');
      expect(text).not.toBeNull();
      expect(text).toHaveClass('lg-inline-edit--quiet');
    }
  }, 15000);

  it('Edge: Edit mode keeps the plain input affordance for both spell text fields', async () => {
    const { user } = await openWithSpells();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));

    expect(document.querySelectorAll('.lg-inline-edit--quiet')).toHaveLength(0);
    expect(screen.getByRole('textbox', { name: 'At-will spells' })).toHaveValue('Light');
    expect(screen.getByRole('textbox', { name: 'Spell list' })).toHaveValue('Magic missile');
  }, 15000);
});

describe('the rollable style contract', () => {
  it('Edge: the base rule reads text colour with a dashed lava underline', () => {
    const body = ruleBody(rollableCss, '.lg-rollable--button');
    expect(body).toMatch(/color:\s*var\(--lg-color-text\)/);
    expect(body).toMatch(/text-decoration-style:\s*dashed/);
    expect(body).toMatch(/text-decoration-color:\s*var\(--lg-color-link\)/);
    expect(body).not.toMatch(/--lg-lava/);
  });

  it('Edge: hover and focus share the lava fill', () => {
    const body = ruleBody(
      rollableCss,
      '.lg-rollable--button:hover,\n.lg-rollable--button:focus-visible',
    );
    expect(body).toMatch(/background-color:\s*var\(--lg-color-accent\)/);
    expect(body).toMatch(/color:\s*var\(--lg-color-accent-text\)/);
  });

  it('Edge: the proficiency dot keeps the only filled-dot mark and the primary button stays the only other accent fill', () => {
    const dot = ruleBody(savesCss, ".lg-prof-dot[aria-pressed='true'] .lg-prof-dot__mark");
    expect(dot).toMatch(/background-color:\s*var\(--lg-color-accent\)/);
    expect(dot).not.toMatch(/--lg-lava/);
  });

  it('Happy path: the attack cards drop their separate lava pill so a to-hit uses the shared style', () => {
    expect(attacksCss).not.toMatch(/lg-attack-card__roll[^{]*\{[^}]*--lg-lava/);
  });

  it('Happy path: the quiet inline edit shows its dashed edge on hover and focus only', () => {
    expect(ruleBody(presetCss, '.lg-inline-edit--quiet')).toMatch(
      /border-bottom-color:\s*transparent/,
    );
    const hover = ruleBody(
      presetCss,
      '.lg-inline-edit--quiet:hover,\n.lg-inline-edit--quiet:focus-visible',
    );
    expect(hover).toMatch(/border-bottom-color:\s*var\(--lg-color-border-strong\)/);
  });
});

describe('the table rules chevron contract', () => {
  it('Edge: the chevron points right when closed and down when open', () => {
    expect(ruleBody(rulesCss, '.lg-rules__chevron')).toMatch(/transform:\s*rotate\(0deg\)/);
    expect(ruleBody(rulesCss, '.lg-rules__chevron--open')).toMatch(/transform:\s*rotate\(90deg\)/);
  });

  it('Edge: reduced motion removes the chevron transition', () => {
    const reduced = rulesCss.slice(rulesCss.indexOf('@media (prefers-reduced-motion: reduce)'));
    expect(reduced).not.toBe('');
    expect(ruleBody(reduced, '.lg-rules__chevron')).toMatch(/transition:\s*none/);
  });
});

describe('the Play label size floor', () => {
  const labels: readonly (readonly [string, string])[] = [
    ['hero proficiency badge label', presetCss],
    ['hit points chip label', presetCss],
    ['armor class label', combatCss],
    ['combat stat label', combatCss],
    ['spellcasting label', spellsCss],
    ['stat block ability label', statBlockCss],
    ['ability name label', abilitiesCss],
    ['skill ability label', savesCss],
    ['play section note', playSectionCss],
  ];

  const selectors: Record<string, string> = {
    'hero proficiency badge label': '.lg-preset-hero__badge-label',
    'hit points chip label': '.lg-hp-card__label',
    'armor class label': '.lg-ac__label',
    'combat stat label': '.lg-combat-stat__label',
    'spellcasting label': '.lg-spells__label',
    'stat block ability label': '.lg-stat-block .lg-ability__name',
    'ability name label': '.lg-ability__name',
    'skill ability label': '.lg-skill__ability',
    'play section note': '.lg-play-section__note',
  };

  it.each(labels)('Edge: the desktop %s clears the 11px floor', (name, css) => {
    expect(fontSize(css, selectors[name])).toBeGreaterThanOrEqual(11);
  });

  it.each(labels)('Edge: the phone %s clears the 12px floor', (name, css) => {
    expect(fontSize(narrowCss(css), selectors[name])).toBeGreaterThanOrEqual(12);
  });
});
