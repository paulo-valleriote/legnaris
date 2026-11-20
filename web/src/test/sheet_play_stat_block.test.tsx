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
import { rollName, initiativeRollName } from './dice_names';
import { SHEET_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';
const here = path.dirname(fileURLToPath(import.meta.url));
const stylesheet = path.resolve(here, '../sheet/preset/fantasy/stat_block.css');
const spellsStylesheet = path.resolve(here, '../sheet/preset/fantasy/spells.css');

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
  const { spells, attacks, ...rest } = data;
  return {
    ...fantasySheetData(rest),
    attacks: {
      attack1: { name: 'Longsword', damage: '1d8 + 3', bonus: 5 },
      attack2: { name: '', damage: '', bonus: 0 },
      ...(attacks as Record<string, unknown> | undefined),
    },
    spells: {
      attackBonus: 6,
      saveDC: 14,
      slot1: { total: 2, expended: 1, remaining: 1 },
      ...(spells as Record<string, unknown> | undefined),
    },
  };
}

function spellAbilityGroup(): Record<string, unknown> {
  return Object.fromEntries(
    ['Wisdom', 'Intelligence'].map((ability) => [
      `spellUses${ability}`,
      {
        schema: {
          type: 'number',
          label: `Spellcasting uses ${ability.toLowerCase()}`,
          is_editable: true,
          default: 0,
          exclusive_group: 'spellcastingAbility',
          option_label: ability,
        },
      },
    ]),
  );
}

function statBlockSpellAbilityFieldTree(): Record<string, unknown> {
  const tree = statBlockFieldTree();
  const config = tree.config as { label: string; children: Record<string, unknown> };
  return {
    ...tree,
    config: { ...config, children: { ...config.children, ...spellAbilityGroup() } },
  };
}

function statBlockTwoSlotFieldTree(): Record<string, unknown> {
  const tree = statBlockFieldTree();
  const spells = tree.spells as { label: string; children: Record<string, unknown> };
  return {
    ...tree,
    spells: { ...spells, children: { ...spells.children, slot2: spellSlotNode(2) } },
  };
}

function spellTextNode(level: { readonly key: 'atWill' | 'list'; readonly label: string }) {
  return [
    level.key,
    {
      schema: {
        type: 'text',
        label: level.label,
        is_editable: true,
        default: 0,
        max_length: 200,
      },
    },
  ];
}

function statBlockSpellProseFieldTree(): Record<string, unknown> {
  const tree = statBlockFieldTree();
  const spells = tree.spells as { label: string; children: Record<string, unknown> };
  return {
    ...tree,
    spells: {
      ...spells,
      children: {
        ...spells.children,
        ...Object.fromEntries([
          spellTextNode({ key: 'atWill', label: 'At-will spells' }),
          spellTextNode({ key: 'list', label: 'Spell list' }),
        ]),
      },
    },
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

async function editValue(
  user: User,
  scope: HTMLElement,
  label: string,
  text: string,
  trigger = label,
) {
  await user.click(within(scope).getByRole('button', { name: trigger }));
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
    expect(screen.getByRole('button', { name: initiativeRollName('+3') })).toHaveTextContent('+3');
    expect(screen.getByTestId('sheet-hp-value')).toHaveTextContent('10 / 20');
  });

  it('Happy path: the six abilities keep their medallions inside the block', async () => {
    await openFantasy({ kind: 'enemy' });

    expect(within(statBlock()).getAllByTestId(/^sheet-ability-/)).toHaveLength(6);
    expect(
      within(statBlock()).getByRole('button', { name: rollName('Strength modifier') }),
    ).toHaveTextContent('+3');
  });

  it('Happy path: attacks and spells lead the body while abilities and prose follow', async () => {
    await openFantasy({ kind: 'enemy' });

    const combat = screen.getByTestId('sheet-stat-column-combat');
    const traits = screen.getByTestId('sheet-stat-column-traits');
    expect(within(combat).getByTestId('sheet-attacks')).toBeInTheDocument();
    expect(within(combat).getByTestId('sheet-spells')).toBeInTheDocument();
    expect(within(traits).getAllByTestId(/^sheet-ability-/)).toHaveLength(6);
    expect(within(traits).getByTestId('sheet-stat-prose')).toBeInTheDocument();
    expect(within(traits).queryByTestId('sheet-attacks')).not.toBeInTheDocument();
    expect(within(traits).queryByTestId('sheet-spells')).not.toBeInTheDocument();
  });

  it('Edge: a sheet with no attacks or spells leaves the combat column empty', async () => {
    await openFantasy({ kind: 'npc', template: fantasyFieldTree() });

    expect(screen.getByTestId('sheet-stat-column-combat')).toBeEmptyDOMElement();
    expect(screen.getByTestId('sheet-stat-column-traits')).not.toBeEmptyDOMElement();
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

    await editValue(
      user,
      screen.getByTestId('sheet-stat-initiative'),
      'Initiative',
      '99',
      message('sheet.play.editInitiative'),
    );
    expect(screen.getByRole('button', { name: initiativeRollName('+20') })).toHaveTextContent(
      '+20',
    );

    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { combat: { initiative: 20 } } }));
  });

  it('Edge: a zero or negative stat block initiative names the signed roll', async () => {
    for (const [initiative, signed] of [
      [0, '+0'],
      [-1, '-1'],
    ] as const) {
      const { unmount } = renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
        client: createApiClient({
          baseUrl: BASE_URL,
          transport: createFakeTransport(
            successOutcome(
              fantasySheetPayload({
                kind: 'npc',
                data: fantasySheetData({ combat: { initiative } }),
              }),
            ),
            successOutcome(fantasyTemplatePayload()),
            successOutcome({}),
          ).transport,
        }),
      });
      await screen.findByTestId('sheet-name');

      const tile = screen.getByTestId('sheet-stat-initiative');
      expect(tile.querySelector('.lg-dice-icon')).toHaveAttribute('data-die-shape', 'd20');
      expect(
        within(tile).getByRole('button', { name: initiativeRollName(signed) }),
      ).toHaveTextContent(signed);

      unmount();
    }
  });

  it('Edge: Edit mode hides the stat block initiative die', async () => {
    const { user } = await openFantasy({ kind: 'npc' });
    expect(
      screen.getByTestId('sheet-stat-initiative').querySelector('.lg-dice-icon'),
    ).not.toBeNull();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));

    expect(screen.queryByTestId('sheet-stat-block')).toBeNull();
    expect(screen.getByRole('textbox', { name: 'Initiative' })).toBeInTheDocument();
  });

  it('Happy path: the stat block initiative pairs the die with an Edit initiative pencil', async () => {
    await openFantasy({ kind: 'npc' });

    const tile = screen.getByTestId('sheet-stat-initiative');
    expect(tile.querySelector('.lg-dice-icon')).toHaveAttribute('aria-hidden', 'true');
    expect(
      within(tile).getByRole('button', { name: message('sheet.play.editInitiative') }),
    ).toBeInTheDocument();
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

describe('the stat block prose lines', () => {
  it('Edge: no proficient saves or skills leaves their lines out but keeps the movement line', async () => {
    await openFantasy({
      kind: 'npc',
      data: { config: { saveDexterityProf: 0, skillAthleticsProf: 0 } },
    });

    expect(screen.queryByTestId('sheet-stat-saves')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-skills')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-stat-speed')).toHaveTextContent('Speed 30 ft · Passive 14');
  });

  it('Edge: no proficiency, no speed and no passive awareness drops the whole block', async () => {
    await openFantasy({
      kind: 'npc',
      data: {
        config: { saveDexterityProf: 0, skillAthleticsProf: 0 },
        combat: { speed: null },
        derived: { passiveAwareness: null },
      },
    });

    expect(screen.queryByTestId('sheet-stat-prose')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-speed')).not.toBeInTheDocument();
  });

  it('Edge: a missing speed leaves only the passive value on its line', async () => {
    await openFantasy({ kind: 'npc', data: { combat: { speed: null } } });

    const line = screen.getByTestId('sheet-stat-speed');
    expect(line).toHaveTextContent('Passive 14');
    expect(line).not.toHaveTextContent('ft');
    expect(line.querySelector('[data-separator]')).toBeNull();
  });

  it('Edge: a missing passive awareness leaves only the speed on its line', async () => {
    await openFantasy({ kind: 'npc', data: { derived: { passiveAwareness: null } } });

    const line = screen.getByTestId('sheet-stat-speed');
    expect(line).toHaveTextContent('Speed 30 ft');
    expect(line).not.toHaveTextContent('Passive');
  });

  it('Edge: negative totals keep their sign', async () => {
    await openFantasy({
      kind: 'enemy',
      data: { saves: { dexterity: -2 }, skills: { athletics: { bonus: 0, total: -3 } } },
    });

    expect(screen.getByTestId('sheet-stat-saves')).toHaveTextContent('Saves Dex -2');
    expect(screen.getByTestId('sheet-stat-skills')).toHaveTextContent('Skills Athletics -3');
  });

  it('Edge: only proficient saves and skills are drawn as inline text', async () => {
    await openFantasy({ kind: 'npc' });

    expect(screen.getByTestId('sheet-stat-save-dexterity')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-save-strength')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-stat-skill-athletics')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-stat-skill-stealth')).not.toBeInTheDocument();
  });

  it('Happy path: each save and skill shows exactly one number, the rollable total', async () => {
    await openFantasy({ kind: 'npc' });

    expect(screen.getByTestId('sheet-stat-saves')).toHaveTextContent('Saves Dex +4');
    expect(screen.getByTestId('sheet-stat-skills')).toHaveTextContent('Skills Athletics +5');
    expect(
      within(screen.getByTestId('sheet-stat-save-dexterity')).getByRole('button', {
        name: rollName('Dexterity save'),
      }),
    ).toHaveTextContent('+4');
    expect(
      within(screen.getByTestId('sheet-stat-skill-athletics')).getByRole('button', {
        name: rollName('Athletics'),
      }),
    ).toHaveTextContent('+5');
  });

  it('Happy path: several proficient skills share one line behind separators', async () => {
    await openFantasy({
      kind: 'npc',
      data: { config: { skillStealthProf: 1, skillPerceptionProf: 1, skillSurvivalProf: 1 } },
    });

    const skills = screen.getByTestId('sheet-stat-skills');
    expect(skills).toHaveTextContent(
      'Skills Athletics +5 · Stealth +3 · Perception +1 · Survival -1',
    );
    expect(skills.querySelectorAll('[data-separator]')).toHaveLength(3);
  });

  it('Edge: the base skill bonus draws no control in Play, so the line carries only the total', async () => {
    await openFantasy({ kind: 'npc' });

    const skill = screen.getByTestId('sheet-stat-skill-athletics');
    expect(within(skill).getByText('Athletics')).toBeInTheDocument();
    expect(skill).toHaveTextContent('Athletics +5');
    expect(
      within(skill).queryByRole('button', { name: 'Athletics bonus' }),
    ).not.toBeInTheDocument();
    expect(within(skill).queryByRole('textbox')).not.toBeInTheDocument();
  });

  it('Happy path: Edit mode still edits the base skill bonus', async () => {
    const { user } = await openFantasy({ kind: 'npc' });

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));

    expect(screen.getByRole('textbox', { name: 'Athletics bonus' })).toBeInTheDocument();
  });

  it('Happy path: speed and passive awareness share one line instead of the passive pill', async () => {
    await openFantasy({ kind: 'npc' });

    expect(screen.getByTestId('sheet-stat-speed')).toHaveTextContent('Speed 30 ft · Passive 14');
    expect(screen.queryByTestId('sheet-play-passive')).not.toBeInTheDocument();
  });
});

describe('the stat block attacks', () => {
  it('Happy path: an attack keeps its rollable bonus and damage', async () => {
    await openFantasy({ kind: 'enemy' });

    const card = screen.getByTestId('sheet-attack-1');
    expect(
      within(card).getByRole('button', {
        name: messageWith('sheet.play.attackRoll', { name: 'Longsword', value: '+5' }),
      }),
    ).toHaveTextContent('+5');
    expect(
      within(card).getByRole('button', {
        name: messageWith('sheet.play.damageRoll', { name: 'Longsword', value: '1d8 + 3' }),
      }),
    ).toHaveTextContent('1d8 + 3');
  });

  it('Edge: no filled attack hides the section and offers no add control in Play', async () => {
    await openFantasy({
      kind: 'npc',
      data: {
        attacks: {
          attack1: { name: '', damage: '', bonus: 0 },
          attack2: { name: '   ', damage: '', bonus: 0 },
        },
      },
    });

    expect(screen.queryByTestId('sheet-attacks')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Add attack/ })).not.toBeInTheDocument();
  });

  it('Edge: Play draws no add control while Edit still edits the empty attack slot', async () => {
    const { user } = await openFantasy({
      kind: 'npc',
      data: {
        attacks: {
          attack1: { name: '', damage: '', bonus: 0 },
          attack2: { name: '', damage: '', bonus: 0 },
        },
      },
    });

    expect(screen.queryByRole('button', { name: /Add attack/ })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('sheet.mode.edit') }));

    expect(screen.getByRole('textbox', { name: 'Attack 2 name' })).toBeInTheDocument();
  });

  it('Edge: a template without attacks or spells still draws the rest of the block', async () => {
    await openFantasy({ kind: 'npc', template: fantasyFieldTree() });

    expect(statBlock()).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-attacks')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-spells')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-ac-value')).toBeInTheDocument();
  });
});

describe('the compact stat block spellcasting', () => {
  it('Edge: a creature with no spellcasting draws no spells section', async () => {
    await openFantasy({
      kind: 'npc',
      data: { spells: { slot1: { total: 0, expended: 0, remaining: 0 } } },
    });

    expect(screen.queryByTestId('sheet-spells')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-spell-summary')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-spell-hidden-levels')).not.toBeInTheDocument();
  });

  it('Happy path: one summary line carries the ability, rollable attack and save DC', async () => {
    await openFantasy({
      kind: 'npc',
      template: statBlockSpellAbilityFieldTree(),
      data: { config: { spellUsesWisdom: 1, spellUsesIntelligence: 0 } },
    });

    expect(screen.getByTestId('sheet-spells')).toHaveAttribute('data-variant', 'compact');

    const summary = screen.getByTestId('sheet-spell-summary');
    expect(within(summary).getByRole('combobox', { name: 'Spellcasting ability' })).toHaveValue(
      'config.spellUsesWisdom',
    );
    expect(within(summary).getByTestId('sheet-spell-attack')).toHaveTextContent('+6');
    expect(
      within(summary).getByRole('button', { name: rollName('Spell attack bonus') }),
    ).toHaveTextContent('+6');
    expect(within(summary).getByTestId('sheet-spell-save-dc')).toHaveTextContent('14');
  });

  it('Happy path: the spellcasting ability select still writes the exclusive group', async () => {
    const { fake, user } = await openFantasy({
      kind: 'npc',
      template: statBlockSpellAbilityFieldTree(),
      data: { config: { spellUsesWisdom: 1, spellUsesIntelligence: 0 } },
    });

    const select = within(statBlock()).getByRole('combobox', { name: 'Spellcasting ability' });
    expect(select).toHaveValue('config.spellUsesWisdom');

    await user.selectOptions(select, 'Intelligence');

    expect(select).toHaveValue('config.spellUsesIntelligence');
    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
    await waitFor(() => expect(savedData(fake)).toBeDefined());
    const config = (savedData(fake) as { data: { config: Record<string, number> } }).data.config;
    expect(config.spellUsesIntelligence).toBe(1);
    expect(config.spellUsesWisdom).toBe(0);
  });

  it('Edge: a level without slots is hidden beside a level that has them', async () => {
    await openFantasy({
      kind: 'npc',
      template: statBlockTwoSlotFieldTree(),
      data: {
        spells: {
          slot1: { total: 1, expended: 0, remaining: 1 },
          slot2: { total: 0, expended: 0, remaining: 0 },
        },
      },
    });

    expect(screen.getByTestId('sheet-slot-1')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-slot-2')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-spell-hidden-levels')).toHaveTextContent(
      'Level 2 hidden · no slots',
    );
  });

  it('Edge: every slot spent keeps one row with only the recover action', async () => {
    await openFantasy({
      kind: 'npc',
      data: { spells: { slot1: { total: 2, expended: 2, remaining: 0 } } },
    });

    const slot = screen.getByTestId('sheet-slot-1');
    expect(within(slot).getByTestId('sheet-slot-1-remaining')).toHaveTextContent('0 of 2 left');
    expect(
      within(slot).queryByRole('button', {
        name: messageWith('sheet.play.slotSpend', { count: 1 }),
      }),
    ).not.toBeInTheDocument();
    expect(
      within(slot).getAllByRole('button', {
        name: messageWith('sheet.play.slotRecover', { count: 1 }),
      }),
    ).toHaveLength(2);
  });

  it('Happy path: a pip spends a slot through the pending delta and saves it', async () => {
    const { fake, user } = await openFantasy({ kind: 'npc' });

    await user.click(
      within(screen.getByTestId('sheet-slot-1')).getByRole('button', {
        name: messageWith('sheet.play.slotSpend', { count: 1 }),
      }),
    );

    expect(screen.getByTestId('sheet-slot-1-remaining')).toHaveTextContent('0 of 2 left');
    expect(screen.getByTestId('sheet-pending-card')).toBeInTheDocument();

    await save(user);

    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { spells: { slot1: { expended: 2 } } } }),
    );
  });

  it('Happy path: at-will and spell list read as prose lines', async () => {
    await openFantasy({
      kind: 'npc',
      template: statBlockSpellProseFieldTree(),
      data: { spells: { atWill: 'Light', list: 'Magic missile' } },
    });

    expect(screen.getByTestId('sheet-spell-atWill')).toHaveTextContent('At-will spells Light');
    expect(screen.getByTestId('sheet-spell-list')).toHaveTextContent('Spell list Magic missile');
  });

  it('Edge: empty at-will and spell lists leave no prose line', async () => {
    await openFantasy({
      kind: 'npc',
      template: statBlockSpellProseFieldTree(),
      data: { spells: { atWill: '   ', list: '' } },
    });

    expect(screen.getByTestId('sheet-spells')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-spell-atWill')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-spell-list')).not.toBeInTheDocument();
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

  it('Edge: the initiative pencil waits for hover or focus on a full 44px target', () => {
    expect(css).toMatch(/\.lg-stat-block__edit\s*\{[^}]*opacity:\s*0/);
    expect(css).toMatch(
      /\.lg-stat-block__stat:hover \.lg-stat-block__edit,\s*\.lg-stat-block__edit:focus-visible\s*\{[^}]*opacity:\s*1/,
    );
    expect(css).toMatch(
      /\.lg-stat-block__edit\s*\{[^}]*min-inline-size:\s*var\(--lg-size-control\)/,
    );
    expect(css).toMatch(
      /\.lg-stat-block :is\(button, input\)\s*\{[^}]*min-block-size:\s*var\(--lg-size-control\)/,
    );
    const mobile = css.slice(css.indexOf('@media (max-width: 40rem)'));
    expect(mobile).toMatch(/\.lg-stat-block__edit\s*\{[^}]*opacity:\s*1/);
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

  it('Happy path: the body stacks below 1100 px and splits into two columns from it', () => {
    expect(css).toMatch(
      /\.lg-stat-block__body\s*\{[^}]*display:\s*flex[^}]*flex-direction:\s*column/,
    );
    expect(css).toMatch(/\.lg-stat-block__column\s*\{[^}]*display:\s*flex[^}]*min-width:\s*0/);

    const wide = css.slice(css.indexOf('@media (min-width: 68.75rem)'));
    expect(wide).toMatch(/\.lg-stat-block__body\s*\{[^}]*display:\s*grid/);
    expect(wide).toMatch(/grid-template-columns:\s*repeat\(2, minmax\(0, 1fr\)\)/);
    expect(wide).toMatch(
      /\.lg-stat-block__column \+ \.lg-stat-block__column\s*\{[^}]*border-inline-start:\s*var\(--lg-size-hairline\)/,
    );
    expect(wide).toMatch(/\.lg-stat-block\s*\{[^}]*max-width:\s*var\(--lg-size-content-max\)/);
  });

  it('Edge: the phone body pads tighter and keeps the ability score beside its medallion', () => {
    const mobile = css.slice(css.indexOf('@media (max-width: 40rem)'));
    expect(mobile).toMatch(/\.lg-stat-block__column > \*\s*\{[^}]*padding:\s*var\(--lg-space-12\)/);
    expect(mobile).toMatch(
      /\.lg-stat-block \.lg-ability-grid\s*\{[^}]*grid-template-columns:\s*repeat\(3, minmax\(0, 1fr\)\)/,
    );
    expect(mobile).toMatch(/\.lg-stat-block \.lg-ability__name\s*\{[^}]*var\(--lg-type-12\)/);
    expect(mobile).toMatch(
      /\.lg-stat-block \.lg-ability \.lg-ability__score\s*\{[^}]*position:\s*absolute/,
    );
    expect(mobile).toMatch(/\.lg-stat-block__line\s*\{[^}]*font-size:\s*var\(--lg-type-13\)/);
  });

  it('Edge: one centred card holds the block, capped near 45rem', () => {
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
    expect(css).toMatch(/\.lg-stat-block__prose\s*\{[^}]*min-width:\s*0/);
  });

  it('Edge: the prose lines wrap without splitting a name from its total, on 44px targets', () => {
    expect(css).toMatch(/\.lg-stat-block__line\s*\{[^}]*overflow-wrap:\s*anywhere/);
    expect(css).toMatch(/\.lg-stat-block__line-item\s*\{[^}]*white-space:\s*nowrap/);
    expect(css).toMatch(
      /\.lg-stat-block :is\(button, input\)\s*\{[^}]*min-block-size:\s*var\(--lg-size-control\)/,
    );
    expect(css).not.toMatch(/lg-stat-block__prof/);
    expect(css).not.toMatch(/lg-ability-passive/);
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

  it('Edge: the compact spell summary wraps the ability, attack and save DC on one line', () => {
    const spellsCss = readFileSync(spellsStylesheet, 'utf8');
    expect(spellsCss).toMatch(
      /\.lg-spells--compact \.lg-spells__top\s*\{[^}]*display:\s*flex[^}]*flex-wrap:\s*wrap/,
    );
    expect(spellsCss).toMatch(
      /\.lg-spells--compact \.lg-spells__stat\s*\{[^}]*flex-direction:\s*row/,
    );
    expect(spellsCss).toMatch(
      /\.lg-spells--compact \.lg-spells__ability\s*\{[^}]*flex-direction:\s*row/,
    );
  });

  it('Edge: the compact slot rows and the spell prose keep one line each', () => {
    const spellsCss = readFileSync(spellsStylesheet, 'utf8');
    expect(spellsCss).toMatch(
      /\.lg-spells--compact \.lg-spells__slot\s*\{[^}]*gap:\s*var\(--lg-space-8\)/,
    );
    expect(spellsCss).toMatch(/\.lg-spells__prose\s*\{[^}]*min-width:\s*0/);
    expect(spellsCss).toMatch(/\.lg-spells__prose-line\s*\{[^}]*margin:\s*0/);
  });
});
