import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
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
import type { FakeOutcome } from './fake_transport';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { rollName } from './dice_names';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const stylesheet = path.resolve(here, '../sheet/preset/fantasy/stat_block.css');

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

function spellSlotNode(level: number): Record<string, unknown> {
  return {
    label: `Level ${level}`,
    columns: ['total', 'expended', 'remaining'],
    children: {
      total: {
        schema: {
          type: 'number',
          label: `Level ${level} total`,
          is_editable: true,
          default: 0,
          column_label: 'Total',
        },
      },
      expended: {
        schema: {
          type: 'number',
          label: `Level ${level} expended`,
          is_editable: true,
          default: 0,
          column_label: 'Expended',
        },
      },
      remaining: {
        schema: {
          type: 'number',
          label: `Level ${level} remaining`,
          formula: 'spells.slot1.total - spells.slot1.expended',
          is_editable: false,
          default: 0,
          column_label: 'Remaining',
        },
      },
    },
  };
}

function statBlockFieldTree(): Record<string, unknown> {
  return {
    ...fantasyFieldTree(),
    attacks: { label: 'Attacks', children: { attack1: attackNode(1), attack2: attackNode(2) } },
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
        slot1: spellSlotNode(1),
      },
    },
  };
}

function statBlockData(data: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    ...fantasySheetData(data),
    attacks: {
      attack1: { name: 'Longsword', damage: '1d8 + 3', bonus: 5 },
      attack2: { name: '', damage: '', bonus: 0 },
    },
    spells: { attackBonus: 6, saveDC: 14, slot1: { total: 2, expended: 1, remaining: 1 } },
  };
}

interface OpenOptions {
  readonly kind?: 'character' | 'npc' | 'enemy';
  readonly data?: Record<string, unknown>;
  readonly template?: Record<string, unknown>;
  readonly save?: FakeOutcome;
}

async function openFantasy(options: OpenOptions = {}) {
  const fake = createFakeTransport(
    successOutcome(
      fantasySheetPayload({
        data: statBlockData(options.data),
        ...(options.kind === undefined ? {} : { kind: options.kind }),
      }),
    ),
    successOutcome(fantasyTemplatePayload({ fields: options.template ?? statBlockFieldTree() })),
    options.save ?? successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

async function openPlainWithKind(kind: 'npc' | 'enemy') {
  const fake = createFakeTransport(
    successOutcome(sheetPayload({ kind })),
    successOutcome(templateDetailPayload()),
    successOutcome({}),
  );
  renderApp(`/sheets/${SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return fake;
}

type User = ReturnType<typeof userEvent.setup>;

function statBlock() {
  return screen.getByTestId('sheet-stat-block');
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

async function save(user: User) {
  await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
}

async function editValue(user: User, scope: HTMLElement, label: string, text: string) {
  await user.click(within(scope).getByRole('button', { name: label }));
  const input = within(scope).getByRole('textbox', { name: label });
  await user.clear(input);
  await user.type(input, `${text}{Enter}`);
}

describe('stat block selection', () => {
  it('Edge: the character kind keeps the full character layout', async () => {
    await openFantasy({ kind: 'character' });

    expect(screen.getByTestId('sheet-play-hero')).toBeInTheDocument();
    expect(screen.getByTestId('sheet-combat-strip')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-block')).not.toBeInTheDocument();
  });

  it('Edge: a sheet without a kind keeps the full character layout', async () => {
    await openFantasy();

    expect(screen.getByTestId('sheet-play-hero')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-block')).not.toBeInTheDocument();
  });

  it('Edge: a template without the fantasy preset keeps the generic form for an NPC', async () => {
    await openPlainWithKind('npc');

    expect(screen.getByRole('textbox', { name: 'Level' })).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-block')).not.toBeInTheDocument();
  });

  it('Edge: an NPC renders the stat block instead of the character sections', async () => {
    await openFantasy({ kind: 'npc' });

    expect(statBlock()).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-play-hero')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-combat-strip')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-play-saves')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-play-skills')).not.toBeInTheDocument();
  });

  it('Happy path: an NPC names its kind', async () => {
    await openFantasy({ kind: 'npc' });

    expect(screen.getByTestId('sheet-stat-kind')).toHaveTextContent(message('kind.npc'));
    expect(screen.getByTestId('sheet-stat-kind')).toHaveAttribute('data-kind', 'npc');
  });

  it('Happy path: an enemy names its kind', async () => {
    await openFantasy({ kind: 'enemy' });

    expect(screen.getByTestId('sheet-stat-kind')).toHaveTextContent(message('kind.enemy'));
    expect(screen.getByTestId('sheet-stat-kind')).toHaveAttribute('data-kind', 'enemy');
  });
});

describe('the stat block card', () => {
  it('Edge: empty identity hides the subtitle', async () => {
    await openFantasy({
      kind: 'npc',
      data: { ancestry: '', origin: '', level: null },
    });

    expect(screen.queryByTestId('sheet-stat-subtitle')).not.toBeInTheDocument();
  });

  it('Edge: only the level set draws a subtitle with no separator', async () => {
    await openFantasy({
      kind: 'npc',
      data: { ancestry: '', origin: '', level: 5 },
    });

    const subtitle = screen.getByTestId('sheet-stat-subtitle');
    expect(subtitle).toHaveTextContent(messageWith('sheet.play.level', { count: 5 }));
    expect(subtitle.querySelectorAll('[data-separator]')).toHaveLength(0);
  });

  it('Happy path: the subtitle joins ancestry, origin and level in order', async () => {
    await openFantasy({ kind: 'npc' });

    const subtitle = screen.getByTestId('sheet-stat-subtitle');
    expect(subtitle).toHaveTextContent('Elf · Wanderer · Level 3');
    expect(subtitle.querySelectorAll('[data-separator]')).toHaveLength(2);
  });

  it('Edge: an empty character name falls back to the sheet name inside the card', async () => {
    await openFantasy({ kind: 'npc', data: { characterName: '' } });

    expect(within(statBlock()).getByTestId('sheet-name')).toHaveTextContent('Aragorn');
  });

  it('Happy path: the header sits inside the stat block card, not in the sheet header', async () => {
    await openFantasy({ kind: 'enemy' });

    const header = screen.getByTestId('sheet-stat-block-header');
    expect(statBlock()).toContainElement(header);
    expect(screen.queryByTestId('sheet-eyebrow')).not.toBeInTheDocument();
  });

  it('Happy path: Duplicate and the Play/Edit toggle stay in the screen header', async () => {
    await openFantasy({ kind: 'enemy' });

    expect(screen.getByRole('button', { name: message('sheet.duplicate') })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.mode.edit') })).toBeInTheDocument();
  });
});

describe('the stat block body', () => {
  it('Happy path: name, armor class, initiative and hit points lead the block', async () => {
    await openFantasy({ kind: 'npc' });

    expect(screen.getByTestId('sheet-name')).toHaveTextContent('Aria');
    expect(screen.getByTestId('sheet-ac-value')).toHaveTextContent('12');
    expect(screen.getByTestId('sheet-stat-initiative')).toHaveTextContent('Initiative');
    expect(screen.getByRole('button', { name: rollName('Initiative') })).toHaveTextContent('+3');
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
  });

  it('Happy path: the six abilities keep their medallions inside the block', async () => {
    await openFantasy({ kind: 'enemy' });

    expect(within(statBlock()).getAllByTestId(/^sheet-ability-/)).toHaveLength(6);
    expect(
      within(statBlock()).getByRole('button', { name: rollName('Strength modifier') }),
    ).toHaveTextContent('+3');
  });

  it('Happy path: damage and healing move the mini hit point bar', async () => {
    const { user } = await openFantasy({ kind: 'npc' });

    await user.type(screen.getByRole('spinbutton', { name: message('sheet.play.amount') }), '3');
    await user.click(screen.getByRole('button', { name: message('sheet.play.damage') }));

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('7 / 20');
    expect(screen.getByTestId('sheet-hp-pending')).toBeInTheDocument();

    await user.type(screen.getByRole('spinbutton', { name: message('sheet.play.amount') }), '2');
    await user.click(screen.getByRole('button', { name: message('sheet.play.heal') }));

    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('9 / 20');
  });

  it('Happy path: an initiative edit clamps and reaches the pending delta', async () => {
    const { fake, user } = await openFantasy({ kind: 'npc' });

    await editValue(user, screen.getByTestId('sheet-stat-initiative'), 'Initiative', '99');
    expect(screen.getByRole('button', { name: rollName('Initiative') })).toHaveTextContent('+20');

    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { combat: { initiative: 20 } } }));
  });
});

describe('the stat block vitals band', () => {
  function vitalsBand() {
    return screen.getByTestId('sheet-stat-band');
  }

  function hitPointsMeter() {
    return within(vitalsBand()).getByRole('meter', { name: message('sheet.play.hitPoints') });
  }

  it('Happy path: armor class, initiative and hit points share one band under the header', async () => {
    await openFantasy({ kind: 'npc' });

    const band = vitalsBand();
    expect(statBlock()).toContainElement(band);
    expect(within(band).getByTestId('sheet-ac')).toBeInTheDocument();
    expect(within(band).getByTestId('sheet-stat-initiative')).toBeInTheDocument();
    expect(within(band).getByTestId('sheet-hp-card')).toBeInTheDocument();
    expect(within(band).getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
  });

  it('Edge: a zero maximum draws an empty band instead of a full bar', async () => {
    await openFantasy({
      kind: 'npc',
      data: { combat: { hitPoints: 0, hitPointsMax: 0, hitPointsTemp: 0 } },
    });

    expect(within(vitalsBand()).getByTestId('sheet-hp-value')).toHaveTextContent('0 / 0');
    expect(hitPointsMeter()).toHaveAttribute('aria-valuemax', '0');
    expect(hitPointsMeter()).toHaveAttribute('aria-valuenow', '0');
    expect(hitPointsMeter()).toHaveAttribute('data-band', 'low');
  });

  it('Edge: editing the current hit points above the maximum keeps them at the maximum', async () => {
    const { user } = await openFantasy({
      kind: 'npc',
      data: { combat: { hitPoints: 10, hitPointsMax: 20, hitPointsTemp: 0 } },
    });

    await user.click(within(vitalsBand()).getByRole('button', { name: 'Current hit points' }));
    const input = within(vitalsBand()).getByRole('textbox', { name: 'Current hit points' });
    await user.clear(input);
    await user.type(input, '40{Enter}');

    expect(within(vitalsBand()).getByTestId('sheet-hp-value')).toHaveTextContent('20 / 20');
    expect(hitPointsMeter()).toHaveAttribute('aria-valuenow', '20');
    expect(within(vitalsBand()).getByTestId('sheet-hp-pending')).toBeInTheDocument();
    expect(within(vitalsBand()).getByTestId('sheet-hp-change')).toHaveTextContent('10 → 20 (+10)');
  });

  it('Edge: an enemy at zero hit points draws no fill and a low band', async () => {
    await openFantasy({
      kind: 'enemy',
      data: { combat: { hitPoints: 0, hitPointsMax: 20, hitPointsTemp: 0 } },
    });

    expect(within(vitalsBand()).getByTestId('sheet-hp-value')).toHaveTextContent('0 / 20');
    expect(hitPointsMeter()).toHaveAttribute('aria-valuenow', '0');
    expect(hitPointsMeter()).toHaveAttribute('data-band', 'low');
    expect(vitalsBand().querySelector('.lg-hp-card__fill')).toHaveStyle({ width: '0%' });
  });

  it('Happy path: temporary hit points draw their value and segment inside the band', async () => {
    await openFantasy({
      kind: 'npc',
      data: { combat: { hitPoints: 10, hitPointsMax: 20, hitPointsTemp: 5 } },
    });

    expect(within(vitalsBand()).getByTestId('sheet-hp-temp-value')).toHaveTextContent('+5 temp');
    expect(within(vitalsBand()).getByTestId('sheet-hp-temp')).toBeInTheDocument();
  });

  it('Happy path: the quick actions work in the band too', async () => {
    const { user } = await openFantasy({ kind: 'npc' });

    await user.click(
      within(vitalsBand()).getByRole('button', {
        name: messageWith('sheet.play.healAmount', { count: 1 }),
      }),
    );

    expect(within(vitalsBand()).getByTestId('sheet-hp-value')).toHaveTextContent('11 / 20');
    expect(within(vitalsBand()).getByTestId('sheet-hp-change')).toHaveTextContent('10 → 11 (+1)');
  });

  it('Happy path: damage and healing move the band and draw the pending preview', async () => {
    const { user } = await openFantasy({ kind: 'npc' });

    await user.type(
      within(vitalsBand()).getByRole('spinbutton', { name: message('sheet.play.amount') }),
      '3',
    );
    await user.click(
      within(vitalsBand()).getByRole('button', { name: message('sheet.play.damage') }),
    );

    expect(within(vitalsBand()).getByTestId('sheet-hp-value')).toHaveTextContent('7 / 20');
    expect(within(vitalsBand()).getByTestId('sheet-hp-pending')).toBeInTheDocument();
    expect(within(vitalsBand()).getByTestId('sheet-hp-change')).toBeInTheDocument();
  });
});

describe('the stat block proficiencies', () => {
  it('Edge: no proficient saves or skills hides the proficiency block', async () => {
    await openFantasy({
      kind: 'npc',
      data: { config: { saveDexterityProf: 0, skillAthleticsProf: 0 } },
    });

    expect(screen.queryByTestId('sheet-stat-proficiencies')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-saves')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-skills')).not.toBeInTheDocument();
  });

  it('Edge: only proficient saves and skills are drawn as inline text', async () => {
    await openFantasy({ kind: 'npc' });

    expect(screen.getByTestId('sheet-stat-save-dexterity')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-save-strength')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-stat-skill-athletics')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-skill-stealth')).not.toBeInTheDocument();
  });

  it('Happy path: a save total rolls and a skill bonus is inline editable', async () => {
    const { fake, user } = await openFantasy({ kind: 'npc' });

    const saveRow = screen.getByTestId('sheet-stat-save-dexterity');
    expect(within(saveRow).getByText('DEX')).toBeInTheDocument();
    expect(
      within(saveRow).getByRole('button', { name: rollName('Dexterity save') }),
    ).toHaveTextContent('+4');

    const skill = screen.getByTestId('sheet-stat-skill-athletics');
    expect(within(skill).getByRole('button', { name: rollName('Athletics') })).toHaveTextContent(
      '+5',
    );

    await editValue(user, skill, 'Athletics bonus', '+3');
    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { skills: { athletics: { bonus: 3 } } } }),
    );
  });
});

describe('the stat block attacks and spells', () => {
  it('Happy path: an attack keeps its rollable bonus and damage', async () => {
    await openFantasy({ kind: 'enemy' });

    const card = screen.getByTestId('sheet-attack-1');
    expect(
      within(card).getByRole('button', { name: rollName('Attack 1 bonus') }),
    ).toHaveTextContent('+5');
    expect(
      within(card).getByRole('button', { name: rollName('Attack 1 damage') }),
    ).toHaveTextContent('1d8 + 3');
  });

  it('Happy path: spells keep their slot pips', async () => {
    await openFantasy({ kind: 'npc' });

    const slot = screen.getByTestId('sheet-slot-1');
    expect(within(slot).getByText('1 of 2 left')).toBeInTheDocument();
    expect(
      within(slot).getByRole('button', { name: messageWith('sheet.play.slotSpend', { count: 1 }) }),
    ).toBeInTheDocument();
    expect(
      within(slot).getByRole('button', {
        name: messageWith('sheet.play.slotRecover', { count: 1 }),
      }),
    ).toBeInTheDocument();
  });

  it('Edge: a template without attacks or spells still draws the rest of the block', async () => {
    await openFantasy({ kind: 'npc', template: fantasyFieldTree() });

    expect(statBlock()).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-attacks')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-spells')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-ac-value')).toBeInTheDocument();
  });
});

describe('the stat block styles', () => {
  const css = readFileSync(stylesheet, 'utf8');

  it('Edge: the block draws body text at 15px and every control on a 44px target', () => {
    expect(css).toMatch(/\.lg-stat-block\s*\{[^}]*font-size:\s*var\(--lg-type-15\)/);
    expect(css).toMatch(
      /\.lg-stat-block :is\(button, input\)\s*\{[^}]*min-block-size:\s*var\(--lg-size-control\)/,
    );
  });

  it('Edge: the phone combat strip folds to the shield beside initiative with a full-width hit points card', () => {
    const mobile = css.slice(css.indexOf('@media (max-width: 40rem)'));
    expect(mobile).toMatch(
      /\.lg-stat-block__combat\s*\{[^}]*grid-template-columns:\s*auto minmax\(0, 1fr\)/,
    );
    expect(mobile).toMatch(/\.lg-stat-block__hp\s*\{[^}]*grid-column:\s*1 \/ -1/);
  });

  it('Edge: the vitals band draws a compact shield beside an inline initiative', () => {
    expect(css).toMatch(
      /\.lg-stat-block__combat\s*\{[^}]*grid-template-columns:\s*auto auto minmax\(0, 1fr\)/,
    );
    expect(css).toMatch(/\.lg-stat-block \.lg-ac\s*\{[^}]*width:\s*3\.5rem/);
    expect(css).toMatch(/\.lg-stat-block \.lg-ac__shield\s*\{[^}]*min-height:\s*4rem/);
    expect(css).toMatch(/\.lg-stat-block__stat\s*\{[^}]*flex-direction:\s*row/);
  });

  it('Edge: the hit points card folds into one row with a 74px amount field', () => {
    expect(css).toMatch(
      /\.lg-stat-block \.lg-hp-card\s*\{[^}]*grid-template-areas:\s*'head bar actions'\s*'notes notes notes'/,
    );
    expect(css).toMatch(/\.lg-stat-block \.lg-hp-card__amount\s*\{[^}]*width:\s*4\.625rem/);
  });

  it('Edge: the phone band keeps AC and initiative together above the hit points', () => {
    const mobile = css.slice(css.indexOf('@media (max-width: 40rem)'));
    expect(mobile).toMatch(
      /\.lg-stat-block__combat\s*\{[^}]*grid-template-columns:\s*auto minmax\(0, 1fr\)/,
    );
    expect(mobile).toMatch(/\.lg-stat-block__hp\s*\{[^}]*grid-column:\s*1 \/ -1/);
    expect(mobile).toMatch(
      /\.lg-stat-block \.lg-hp-card__entry > \.lg-button\s*\{[^}]*min-block-size:\s*var\(--lg-size-control\)/,
    );
  });

  it('Happy path: one centred card holds the block, capped near 45rem', () => {
    expect(css).toMatch(/\.lg-stat-block\s*\{[^}]*max-width:\s*44rem[^}]*margin-inline:\s*auto/);
    expect(css).toMatch(
      /\.lg-stat-block\s*\{[^}]*border:\s*var\(--lg-size-hairline\) solid var\(--lg-color-border\)/,
    );
    expect(css).toMatch(/\.lg-stat-block\s*\{[^}]*background-color:\s*var\(--lg-color-surface\)/);
  });

  it('Edge: sections are hairline-divided inside the card and drop their own borders', () => {
    expect(css).toMatch(
      /\.lg-stat-block\s*>\s*\*\s*\+\s*\*\s*\{[^}]*border-block-start:\s*var\(--lg-size-hairline\) solid var\(--lg-color-border\)/,
    );
    expect(css).toMatch(/\.lg-stat-block \.lg-play-section\s*\{[^}]*border:\s*0/);
    expect(css).toMatch(/\.lg-stat-block \.lg-hp-card\s*\{[^}]*border:\s*0/);
    expect(css).toMatch(/\.lg-stat-block__profs\s*\{[^}]*border:\s*0/);
  });

  it('Edge: the enemy badge uses the danger role and the NPC badge a neutral role', () => {
    expect(css).toMatch(/\.lg-stat-block__kind\s*\{[^}]*color:\s*var\(--lg-color-text-muted\)/);
    expect(css).toMatch(
      /\.lg-stat-block__kind\[data-kind='enemy'\]\s*\{[^}]*color:\s*var\(--lg-color-danger\)/,
    );
  });

  it('Edge: the block keeps the damage and heal buttons at 40px on a desktop and 44px on a phone', () => {
    const mobile = css.slice(css.indexOf('@media (max-width: 40rem)'));
    expect(css).toMatch(
      /\.lg-stat-block \.lg-hp-card__entry > \.lg-button\s*\{[^}]*min-block-size:\s*var\(--lg-size-control-compact\)/,
    );
    expect(mobile).toMatch(
      /\.lg-stat-block \.lg-hp-card__entry > \.lg-button\s*\{[^}]*min-block-size:\s*var\(--lg-size-control\)/,
    );
    expect(mobile).toMatch(
      /\.lg-stat-block \.lg-hp-card__quick > \.lg-button\s*\{[^}]*min-block-size:\s*var\(--lg-size-control\)/,
    );
    expect(mobile).toMatch(
      /\.lg-stat-block \.lg-hp-card__quick > \.lg-button\s*\{[^}]*min-inline-size:\s*var\(--lg-size-control\)/,
    );
  });
});
