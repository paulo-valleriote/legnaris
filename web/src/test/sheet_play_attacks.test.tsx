import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message } from '../l10n/strings';
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

const BASE_URL = 'http://api.test';

const SLOTS = [1, 2, 3, 4];

interface AttackValues {
  readonly name?: string;
  readonly damage?: string;
  readonly bonus?: number;
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

function attackFieldTree(): Record<string, unknown> {
  return {
    ...fantasyFieldTree(),
    attacks: {
      label: 'Attacks',
      children: Object.fromEntries(SLOTS.map((slot) => [`attack${slot}`, attackNode(slot)])),
    },
  };
}

function attackFieldTreeWithoutFirstSlot(): Record<string, unknown> {
  const fields = attackFieldTree();
  delete (fields.attacks as { children: Record<string, unknown> }).children.attack1;
  return fields;
}

function attackSlotData(values: AttackValues): Record<string, unknown> {
  return {
    name: values.name ?? '',
    damage: values.damage ?? '',
    bonus: values.bonus ?? 0,
  };
}

function attackData(overrides: Record<number, AttackValues> = {}): Record<string, unknown> {
  return {
    ...fantasySheetData(),
    attacks: Object.fromEntries(
      SLOTS.map((slot) => [`attack${slot}`, attackSlotData(overrides[slot] ?? {})]),
    ),
  };
}

interface OpenAttacksOptions {
  readonly data?: Record<string, unknown>;
  readonly template?: Record<string, unknown>;
  readonly dice?: Record<string, unknown>;
  readonly save?: FakeOutcome;
}

async function openAttacks(options: OpenAttacksOptions = {}) {
  const sheet = fantasySheetPayload({
    data: options.data ?? attackData(),
    ...(options.dice === undefined ? {} : { dice: options.dice }),
  });
  const fake = createFakeTransport(
    successOutcome(sheet),
    successOutcome(fantasyTemplatePayload({ fields: attackFieldTree(), ...options.template })),
    options.save ?? successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { fake, user };
}

function attackCard(slot: number) {
  return screen.getByTestId(`sheet-attack-${slot}`);
}

function addButton() {
  return screen.queryByRole('button', { name: /^\+ Add attack/ });
}

function savedData(fake: ReturnType<typeof createFakeTransport>) {
  const patch = fake.requests.find((request) => request.method === 'PATCH');
  return patch === undefined ? undefined : JSON.parse(patch.body ?? '{}');
}

async function editValue(
  user: ReturnType<typeof userEvent.setup>,
  card: HTMLElement,
  label: string,
  text: string,
) {
  await user.click(within(card).getByRole('button', { name: label }));
  const input = within(card).getByRole('textbox', { name: label });
  await user.clear(input);
  if (text !== '') {
    await user.type(input, `${text}{Enter}`);
  } else {
    await user.keyboard('{Enter}');
  }
}

async function save(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole('button', { name: message('sheet.save') }));
}

describe('attack cards in play mode', () => {
  it('Edge: no attacks field in the template renders no attacks section', async () => {
    const { fake } = await openAttacks({ template: { fields: fantasyFieldTree() } });

    expect(screen.queryByTestId('sheet-attacks')).toBeNull();
    expect(fake.requests).toHaveLength(2);
  });

  it('Edge: empty slots are hidden and counted in the add button', async () => {
    await openAttacks();

    expect(screen.getByTestId('sheet-attacks')).toHaveTextContent('Attacks');
    expect(screen.queryAllByTestId(/^sheet-attack-\d+$/)).toHaveLength(0);
    expect(addButton()).toHaveTextContent('+ Add attack (4 free)');
  });

  it('Edge: an empty name hides the slot even when damage and bonus are set', async () => {
    await openAttacks({
      data: attackData({
        1: { name: '', damage: '1d8 + 3', bonus: 5 },
        2: { name: 'Dagger', damage: '1d4 + 2', bonus: 4 },
      }),
    });

    expect(screen.queryByTestId('sheet-attack-1')).toBeNull();
    expect(screen.getByTestId('sheet-attack-2')).toHaveTextContent('Dagger');
    expect(addButton()).toHaveTextContent('+ Add attack (3 free)');
  });

  it('Edge: every filled slot hides the add button', async () => {
    await openAttacks({
      data: attackData({
        1: { name: 'Longsword', damage: '1d8 + 3', bonus: 5 },
        2: { name: 'Dagger', damage: '1d4 + 2', bonus: 4 },
        3: { name: 'Shortbow', damage: '1d6 + 2', bonus: 4 },
        4: { name: 'Firebolt', damage: '1d10', bonus: 5 },
      }),
    });

    expect(screen.getAllByTestId(/^sheet-attack-\d+$/)).toHaveLength(4);
    expect(addButton()).toBeNull();
  });

  it('Edge: adding reveals one empty card at a time and counts the hidden ones', async () => {
    const { user } = await openAttacks();

    await user.click(addButton() as HTMLElement);

    expect(screen.getByTestId('sheet-attack-1')).toBeInTheDocument();
    expect(addButton()).toHaveTextContent('+ Add attack (3 free)');

    await user.click(addButton() as HTMLElement);

    expect(screen.getByTestId('sheet-attack-2')).toBeInTheDocument();
    expect(addButton()).toHaveTextContent('+ Add attack (2 free)');
  });

  it('Edge: a template without attack1 reveals the next present slot when adding', async () => {
    const { user } = await openAttacks({
      template: { fields: attackFieldTreeWithoutFirstSlot() },
    });

    expect(screen.queryByTestId('sheet-attack-1')).toBeNull();
    expect(addButton()).toHaveTextContent('+ Add attack (3 free)');

    await user.click(addButton() as HTMLElement);

    expect(screen.queryByTestId('sheet-attack-1')).toBeNull();
    expect(screen.getByTestId('sheet-attack-2')).toBeInTheDocument();
    expect(addButton()).toHaveTextContent('+ Add attack (2 free)');
  });

  it('Edge: revealing all four slots hides the add button', async () => {
    const { user } = await openAttacks();

    for (let click = 0; click < 4; click += 1) {
      await user.click(addButton() as HTMLElement);
    }

    expect(screen.getAllByTestId(/^sheet-attack-\d+$/)).toHaveLength(4);
    expect(addButton()).toBeNull();
  });
});

describe('attack editing', () => {
  it('Happy path: name, bonus and damage edit into the pending delta and save as one attack', async () => {
    const { fake, user } = await openAttacks();

    await user.click(addButton() as HTMLElement);
    const card = attackCard(1);
    await editValue(user, card, 'Attack 1 name', 'Sting');
    await editValue(user, card, 'Attack 1 bonus', '7');
    await editValue(user, card, 'Attack 1 damage', '1d6 + 2');

    expect(within(card).getByRole('button', { name: 'Attack 1 name' })).toHaveTextContent('Sting');
    expect(within(card).getByRole('button', { name: 'Attack 1 bonus' })).toBeInTheDocument();
    expect(within(card).getByRole('button', { name: 'Attack 1 damage' })).toBeInTheDocument();
    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent(
      'Attack 1 name empty → Sting',
    );

    await save(user);

    await waitFor(() =>
      expect(savedData(fake)).toEqual({
        data: {
          attacks: {
            attack1: { name: 'Sting', bonus: 7, damage: '1d6 + 2' },
          },
        },
      }),
    );
  });

  it('Edge: a non-numeric bonus is ignored and leaves no pending change', async () => {
    const { user } = await openAttacks({
      data: attackData({ 1: { name: 'Longsword', damage: '1d8 + 3', bonus: 5 } }),
    });
    const card = attackCard(1);

    await editValue(user, card, 'Attack 1 bonus', 'many');

    expect(within(card).getByRole('button', { name: 'Attack 1 bonus' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
  });

  it('Edge: a bonus above the template range is clamped', async () => {
    const { user } = await openAttacks({
      data: attackData({ 1: { name: 'Longsword', damage: '1d8 + 3', bonus: 5 } }),
    });

    await editValue(user, attackCard(1), 'Attack 1 bonus', '99');

    expect(screen.getByTestId('sheet-pending-changes')).toHaveTextContent('Attack 1 bonus 5 → 20');
  });

  it('Error: a non-editable attack name keeps the stored value and writes nothing', async () => {
    const fields = attackFieldTree();
    const attacks = fields.attacks as {
      children: Record<string, { children: Record<string, { schema: Record<string, unknown> }> }>;
    };
    attacks.children.attack1.children.name.schema = {
      ...attacks.children.attack1.children.name.schema,
      is_editable: false,
    };
    const { fake, user } = await openAttacks({
      data: attackData({ 1: { name: 'Longsword', damage: '1d8 + 3', bonus: 5 } }),
      template: { fields },
    });

    await editValue(user, attackCard(1), 'Attack 1 name', 'Sting');

    expect(screen.getByRole('button', { name: 'Attack 1 name' })).toHaveTextContent('Longsword');
    expect(screen.getByRole('button', { name: message('sheet.save') })).toBeDisabled();
    expect(savedData(fake)).toBeUndefined();
  });
});

describe('attack rolling', () => {
  it('Happy path: the bonus rolls d20 plus the stored bonus through the dice store', async () => {
    const { user } = await openAttacks({
      data: attackData({ 1: { name: 'Longsword', damage: '1d8 + 3', bonus: 5 } }),
    });

    await user.click(
      within(attackCard(1)).getByRole('button', { name: rollName('Attack 1 bonus') }),
    );

    await waitFor(() => expect(document.querySelector('.lg-dice-panel')).not.toBeNull());
    expect(document.querySelector('.lg-dice-panel__total-label')).toHaveTextContent(
      'Attack 1 bonus',
    );
    const total = Number(document.querySelector('.lg-dice-panel__total-value')?.textContent);
    expect(total).toBeGreaterThanOrEqual(6);
    expect(total).toBeLessThanOrEqual(25);
    expect(document.querySelector('.lg-dice-panel__parts')).toHaveTextContent('1d20');
  });

  it('Happy path: the damage rolls the stored damage expression', async () => {
    const { user } = await openAttacks({
      data: attackData({ 1: { name: 'Longsword', damage: '1d8 + 3', bonus: 5 } }),
    });

    await user.click(
      within(attackCard(1)).getByRole('button', { name: rollName('Attack 1 damage') }),
    );

    await waitFor(() => expect(document.querySelector('.lg-dice-panel')).not.toBeNull());
    expect(document.querySelector('.lg-dice-panel__total-label')).toHaveTextContent(
      'Attack 1 damage',
    );
    const total = Number(document.querySelector('.lg-dice-panel__total-value')?.textContent);
    expect(total).toBeGreaterThanOrEqual(4);
    expect(total).toBeLessThanOrEqual(11);
    expect(document.querySelector('.lg-dice-panel__parts')).toHaveTextContent('1d8');
  });

  it('Edge: damage without dice is shown but offers no roll', async () => {
    await openAttacks({
      data: attackData({ 1: { name: 'Longsword', damage: 'three', bonus: 5 } }),
    });
    const card = attackCard(1);

    expect(within(card).queryByRole('button', { name: rollName('Attack 1 damage') })).toBeNull();
    expect(within(card).getByText('three')).toBeInTheDocument();
    expect(
      within(card).getByRole('button', { name: rollName('Attack 1 bonus') }),
    ).toBeInTheDocument();
  });
});
