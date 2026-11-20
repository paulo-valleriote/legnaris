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
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { rollName } from './dice_names';

const BASE_URL = 'http://api.test';

const ABILITIES = ['Strength', 'Dexterity', 'Constitution', 'Intelligence', 'Wisdom', 'Charisma'];

const RULES = [
  { key: 'modifierBase', label: 'Ability modifier base', value: 10 },
  { key: 'modifierStep', label: 'Ability modifier step', value: 2 },
  { key: 'passiveBase', label: 'Passive score base', value: 10 },
  { key: 'spellSaveBase', label: 'Spell save DC base', value: 8 },
];

function spellUsesKey(ability: string): string {
  return `spellUses${ability}`;
}

function slotTree(level: number): Record<string, unknown> {
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

function spellFieldTree(): Record<string, unknown> {
  const slots = Object.fromEntries(
    Array.from({ length: 9 }, (_, index) => [`slot${index + 1}`, slotTree(index + 1)]),
  );
  const spellUses = Object.fromEntries(
    ABILITIES.map((ability) => [
      spellUsesKey(ability),
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
  const rules = Object.fromEntries(
    RULES.map((rule) => [
      rule.key,
      {
        schema: {
          type: 'number',
          label: rule.label,
          is_editable: true,
          default: rule.value,
        },
      },
    ]),
  );

  return {
    ...fantasyFieldTree(),
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
        ...slots,
      },
    },
    config: { label: 'Configuration', children: spellUses },
    rules: { label: 'Rules', children: rules },
  };
}

function spellUses(selected?: string): Record<string, number> {
  return Object.fromEntries(
    ABILITIES.map((ability) => [spellUsesKey(ability), ability === selected ? 1 : 0]),
  );
}

function spellSlots(overrides: Record<number, Record<string, number>> = {}) {
  return Object.fromEntries(
    Array.from({ length: 9 }, (_, index) => {
      const level = index + 1;
      const slot = overrides[level] ?? { total: 0, expended: 0, remaining: 0 };
      return [`slot${level}`, slot];
    }),
  );
}

function spellData(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    ...fantasySheetData(),
    spells: {
      attackBonus: 5,
      saveDC: 13,
      atWill: 'Light',
      list: 'Magic missile',
      ...spellSlots({
        1: { total: 2, expended: 1, remaining: 1 },
        3: { total: 1, expended: 1, remaining: 0 },
      }),
    },
    config: spellUses('Wisdom'),
    rules: Object.fromEntries(RULES.map((rule) => [rule.key, rule.value])),
    ...overrides,
  };
}

interface OpenSpellsOptions {
  readonly data?: Record<string, unknown>;
  readonly save?: FakeOutcome;
}

async function openSpells(options: OpenSpellsOptions = {}) {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload({ data: options.data ?? spellData() })),
    successOutcome(fantasyTemplatePayload({ fields: spellFieldTree() })),
    options.save ?? successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

async function save(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
}

const spellPanel = () => screen.getByTestId('sheet-spells');

function abilityGroup() {
  return within(spellPanel()).getByRole('group', { name: 'Spellcasting ability' });
}

function totalButton(level: number): HTMLElement {
  return within(screen.getByTestId(`sheet-slot-${level}`)).getByRole('button', {
    name: `Level ${level} total`,
  });
}

describe('the spellcasting panel', () => {
  it('Edge: a template without spell fields draws neither the panel nor the table rules', async () => {
    const fake = createFakeTransport(
      successOutcome(fantasySheetPayload()),
      successOutcome(fantasyTemplatePayload()),
      successOutcome({}),
    );
    renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
      client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
    });
    await screen.findByTestId('sheet-name');

    expect(screen.queryByTestId('sheet-spells')).not.toBeInTheDocument();
    expect(screen.queryByTestId('sheet-table-rules')).not.toBeInTheDocument();
  });

  it('Edge: a level whose total is zero draws no slot row', async () => {
    await openSpells();

    expect(screen.getByTestId('sheet-slot-1')).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-slot-2')).not.toBeInTheDocument();
    expect(screen.getByTestId('sheet-slot-3')).toBeInTheDocument();
  });

  it('Edge: a level with every slot expended offers only the recover action', async () => {
    await openSpells();

    const slot = within(screen.getByTestId('sheet-slot-3'));
    expect(
      slot.queryByRole('button', {
        name: messageWith('sheet.play.slotSpend', { count: 3 }),
      }),
    ).not.toBeInTheDocument();
    expect(
      slot.getByRole('button', {
        name: messageWith('sheet.play.slotRecover', { count: 3 }),
      }),
    ).toBeInTheDocument();
  });

  it('Edge: a sheet with every spellcasting ability off draws no pressed segment', async () => {
    await openSpells({
      data: spellData({ config: spellUses() }),
    });

    expect(within(abilityGroup()).queryByRole('button', { pressed: true })).toBeNull();
  });

  it('Happy path: the panel shows the ability, attack, save DC, lists and slot rows', async () => {
    await openSpells();

    expect(spellPanel()).toHaveTextContent(message('sheet.play.spellcasting'));
    for (const ability of ABILITIES) {
      expect(within(abilityGroup()).getByRole('button', { name: ability })).toBeInTheDocument();
    }
    expect(screen.getByTestId('sheet-spell-attack')).toHaveTextContent('Spell attack bonus');
    expect(screen.getByTestId('sheet-spell-attack')).toHaveTextContent('+5');
    expect(screen.getByTestId('sheet-spell-save-dc')).toHaveTextContent('Spell save DC');
    expect(screen.getByTestId('sheet-spell-save-dc')).toHaveTextContent('13');
    expect(screen.getByRole('button', { name: 'At-will spells' })).toHaveTextContent('Light');
    expect(screen.getByRole('button', { name: 'Spell list' })).toHaveTextContent('Magic missile');
    expect(screen.getByTestId('sheet-slot-1')).toHaveTextContent(
      messageWith('sheet.play.slotsLeft', { remaining: 1, total: 2 }),
    );
  });

  it('Happy path: the spell attack rolls d20 plus the bonus through the dice panel', async () => {
    const { user } = await openSpells();

    const roll = within(screen.getByTestId('sheet-spell-attack')).getByRole('button', {
      name: rollName('Spell attack bonus'),
    });
    expect(roll).toHaveTextContent('+5');

    await user.click(roll);

    await waitFor(() => expect(document.querySelectorAll('.lg-dice-panel')).toHaveLength(1));
    const total = Number(document.querySelector('.lg-dice-panel__total-value')?.textContent);
    expect(document.querySelector('.lg-dice-panel__total-label')?.textContent).toBe(
      'Spell attack bonus',
    );
    expect(total).toBeGreaterThanOrEqual(6);
    expect(total).toBeLessThanOrEqual(25);
  });
});

describe('the spellcasting ability segments', () => {
  it('Happy path: choosing an ability writes exactly one non-zero member of the group', async () => {
    const { fake, user } = await openSpells();

    await user.click(within(abilityGroup()).getByRole('button', { name: 'Intelligence' }));

    expect(within(abilityGroup()).getByRole('button', { name: 'Intelligence' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(within(abilityGroup()).getByRole('button', { name: 'Wisdom' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );

    await save(user);
    await waitFor(() => expect(savedData(fake)).toBeDefined());
    const config = (savedData(fake) as { data: { config: Record<string, number> } }).data.config;
    expect(Object.values(config).filter((value) => value !== 0)).toEqual([1]);
    expect(config[spellUsesKey('Intelligence')]).toBe(1);
  });

  it('Edge: choosing the ability already on writes nothing', async () => {
    const { fake, user } = await openSpells();

    await user.click(within(abilityGroup()).getByRole('button', { name: 'Wisdom' }));

    expect(
      screen.queryByRole('button', { name: message('sheet.pending.save') }),
    ).not.toBeInTheDocument();
    expect(savedData(fake)).toBeUndefined();
  });
});

describe('the spell slots', () => {
  it('Happy path: tapping a remaining pip spends one slot', async () => {
    const { fake, user } = await openSpells();

    const slot = within(screen.getByTestId('sheet-slot-1'));
    await user.click(
      slot.getByRole('button', { name: messageWith('sheet.play.slotSpend', { count: 1 }) }),
    );

    expect(screen.getByTestId('sheet-slot-1-remaining')).toHaveTextContent(
      messageWith('sheet.play.slotsLeft', { remaining: 0, total: 2 }),
    );
    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { spells: { slot1: { expended: 2 } } } }),
    );
  });

  it('Happy path: tapping a spent pip recovers one slot', async () => {
    const { fake, user } = await openSpells();

    const slot = within(screen.getByTestId('sheet-slot-1'));
    await user.click(
      slot.getByRole('button', { name: messageWith('sheet.play.slotRecover', { count: 1 }) }),
    );

    expect(screen.getByTestId('sheet-slot-1-remaining')).toHaveTextContent(
      messageWith('sheet.play.slotsLeft', { remaining: 2, total: 2 }),
    );
    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { spells: { slot1: { expended: 0 } } } }),
    );
  });

  it('Edge: a total set to zero keeps the level visible while the edit is pending', async () => {
    const { user } = await openSpells();

    await user.click(totalButton(1));
    const total = within(screen.getByTestId('sheet-slot-1')).getByRole('textbox', {
      name: 'Level 1 total',
    });
    await user.clear(total);
    await user.type(total, '0{Enter}');

    expect(screen.getByTestId('sheet-slot-1')).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: messageWith('sheet.play.slotSpend', { count: 1 }) }),
    ).not.toBeInTheDocument();
  });

  it('Happy path: the slot total is editable inline', async () => {
    const { fake, user } = await openSpells();

    await user.click(totalButton(1));
    const total = within(screen.getByTestId('sheet-slot-1')).getByRole('textbox', {
      name: 'Level 1 total',
    });
    await user.clear(total);
    await user.type(total, '4{Enter}');

    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { spells: { slot1: { total: 4 } } } }),
    );
  });

  it('Error: a total that is not a number is refused and leaves the stored value', async () => {
    const { fake, user } = await openSpells();

    await user.click(totalButton(1));
    const total = within(screen.getByTestId('sheet-slot-1')).getByRole('textbox', {
      name: 'Level 1 total',
    });
    await user.clear(total);
    await user.type(total, 'abc{Enter}');

    expect(totalButton(1)).toHaveTextContent('2');
    expect(
      screen.queryByRole('button', { name: message('sheet.pending.save') }),
    ).not.toBeInTheDocument();
    expect(savedData(fake)).toBeUndefined();
  });

  it('Error: a failed save keeps the spent slot pending and shows the mapped error', async () => {
    const { fake, user } = await openSpells({
      save: errorOutcome('INTERNAL_ERROR', 'boom', 500),
    });

    const slot = within(screen.getByTestId('sheet-slot-1'));
    await user.click(
      slot.getByRole('button', { name: messageWith('sheet.play.slotSpend', { count: 1 }) }),
    );
    await user.click(
      within(screen.getByTestId('sheet-pending-card')).getByRole('button', {
        name: message('sheet.pending.save'),
      }),
    );

    await waitFor(() =>
      expect(screen.getByTestId('sheet-pending-error')).toHaveTextContent(
        message('api.error.serverError'),
      ),
    );
    expect(screen.getByTestId('sheet-slot-1-remaining')).toHaveTextContent(
      messageWith('sheet.play.slotsLeft', { remaining: 0, total: 2 }),
    );
    expect(fake.requests.some((request) => request.method === 'PATCH')).toBe(true);
  });
});

describe('the at-will and prepared lists', () => {
  it('Happy path: the at-will list commits its text and writes one path', async () => {
    const { fake, user } = await openSpells();

    await user.click(screen.getByRole('button', { name: 'At-will spells' }));
    const input = screen.getByRole('textbox', { name: 'At-will spells' });
    await user.clear(input);
    await user.type(input, 'Light, Mage Hand{Enter}');

    await save(user);
    await waitFor(() =>
      expect(savedData(fake)).toEqual({ data: { spells: { atWill: 'Light, Mage Hand' } } }),
    );
  });
});

describe('the table rules', () => {
  it('Edge: the section is collapsed by default and its fields are not reachable', async () => {
    await openSpells();

    const rules = screen.getByTestId('sheet-table-rules');
    expect(
      within(rules).getByRole('button', { name: message('sheet.play.rules') }),
    ).toHaveAttribute('aria-expanded', 'false');
    expect(within(rules).queryByRole('textbox', { name: 'Ability modifier base' })).toBeNull();
  });

  it('Happy path: opening the section reveals the editable rules and writes one value', async () => {
    const { fake, user } = await openSpells();

    const rules = screen.getByTestId('sheet-table-rules');
    await user.click(within(rules).getByRole('button', { name: message('sheet.play.rules') }));

    const modifierBase = within(rules).getByRole('textbox', { name: 'Ability modifier base' });
    expect(modifierBase).toHaveValue('10');
    await user.clear(modifierBase);
    await user.type(modifierBase, '12');

    await save(user);
    await waitFor(() => expect(savedData(fake)).toEqual({ data: { rules: { modifierBase: 12 } } }));
  });
});
