import { afterEach, describe, expect, it } from 'vitest';
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { LONG_PRESS_MS } from '../sheet/dice_row';
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

function fieldsWithAttacks(): Record<string, unknown> {
  return {
    ...fantasyFieldTree(),
    attacks: {
      label: 'Attacks',
      children: { attack1: attackNode(1) },
    },
  };
}

function dataWithAttacks(): Record<string, unknown> {
  return {
    ...fantasySheetData(),
    attacks: { attack1: { name: 'Longsword', damage: '1d8 + 3', bonus: 0 } },
  };
}

async function open() {
  const fake = createFakeTransport(
    successOutcome(fantasySheetPayload({ data: dataWithAttacks() })),
    successOutcome(fantasyTemplatePayload({ fields: fieldsWithAttacks() })),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return user;
}

function panel(): HTMLElement | null {
  return document.querySelector<HTMLElement>('.lg-dice-panel');
}

function panelDice(): number[] {
  return [...document.querySelectorAll<HTMLElement>('.lg-dice-panel__die')].map((die) =>
    Number(die.textContent),
  );
}

function keptDie(): number {
  return Number(document.querySelector<HTMLElement>('.lg-dice-panel__die--kept')?.textContent);
}

function droppedDice(): HTMLElement[] {
  return [...document.querySelectorAll<HTMLElement>('.lg-dice-panel__die--dropped')];
}

function total(): number {
  return Number(document.querySelector<HTMLElement>('.lg-dice-panel__total-value')?.textContent);
}

function announcement(): string {
  return within(panel() as HTMLElement).getByRole('status').textContent ?? '';
}

function wisdomButton(): HTMLElement {
  return within(screen.getByTestId('sheet-ability-wisdom')).getByRole('button', {
    name: rollName('Wisdom modifier'),
  });
}

function damageButton(): HTMLElement {
  return within(screen.getByTestId('sheet-attack-1')).getByRole('button', {
    name: /^Damage roll/,
  });
}

async function clickWith(
  user: ReturnType<typeof userEvent.setup>,
  element: HTMLElement,
  key: 'Shift' | 'Alt',
) {
  await user.keyboard(`{${key}>}`);
  await user.click(element);
  await user.keyboard(`{/${key}}`);
}

afterEach(cleanup);

describe('advantage and disadvantage from the keyboard and a mouse', () => {
  it('Happy path: Shift-click keeps the higher d20 and adds the negative modifier', async () => {
    const user = await open();

    await clickWith(user, wisdomButton(), 'Shift');
    await waitFor(() => expect(panel()).not.toBeNull());

    const dice = panelDice();
    expect(dice).toHaveLength(2);
    expect(keptDie()).toBe(Math.max(...dice));
    expect(droppedDice()).toHaveLength(1);
    expect(total()).toBe(Math.max(...dice) - 1);
    expect(announcement()).toBe(
      messageWith('dice.rolledAdvantage', { label: 'Wisdom modifier', total: total() }),
    );
    expect(panel()?.textContent).toContain(message('dice.mode.advantage'));
  }, 15000);

  it('Happy path: Alt-click keeps the lower d20 and adds the negative modifier', async () => {
    const user = await open();

    await clickWith(user, wisdomButton(), 'Alt');
    await waitFor(() => expect(panel()).not.toBeNull());

    const dice = panelDice();
    expect(dice).toHaveLength(2);
    expect(keptDie()).toBe(Math.min(...dice));
    expect(total()).toBe(Math.min(...dice) - 1);
    expect(announcement()).toBe(
      messageWith('dice.rolledDisadvantage', { label: 'Wisdom modifier', total: total() }),
    );
    expect(panel()?.textContent).toContain(message('dice.mode.disadvantage'));
  }, 15000);

  it('Edge: both keys held read as advantage', async () => {
    const user = await open();

    await user.keyboard('{Shift>}{Alt>}');
    await user.click(wisdomButton());
    await user.keyboard('{/Alt}{/Shift}');
    await waitFor(() => expect(panel()).not.toBeNull());

    const dice = panelDice();
    expect(keptDie()).toBe(Math.max(...dice));
    expect(announcement()).toContain(`with ${message('dice.mode.advantage').toLowerCase()}`);
  }, 15000);

  it('Edge: a plain click still rolls one d20 and names no mode', async () => {
    const user = await open();

    await user.click(wisdomButton());
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
    expect(announcement()).toBe(
      messageWith('dice.rolled', { label: 'Wisdom modifier', total: total() }),
    );
    expect(panel()?.textContent).not.toContain(message('dice.mode.advantage'));
  }, 15000);
});

describe('damage rolls ignore the roll modifiers', () => {
  it('Edge: Shift-click on a damage roll keeps every die of its own expression', async () => {
    const user = await open();

    await clickWith(user, damageButton(), 'Shift');
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
    expect(total() - 3).toBeGreaterThanOrEqual(1);
    expect(total() - 3).toBeLessThanOrEqual(8);
    expect(announcement()).toBe(
      messageWith('dice.rolled', { label: 'Attack 1 damage', total: total() }),
    );
    expect(panel()?.textContent).not.toContain(message('dice.mode.advantage'));
  }, 15000);
});

describe('the small roll choice', () => {
  it('Happy path: a long press offers Normal, Advantage and Disadvantage, then rolls the choice', async () => {
    await open();
    const button = wisdomButton();

    fireEvent.pointerDown(button);
    await act(() => new Promise((resolve) => setTimeout(resolve, LONG_PRESS_MS + 50)));

    const dialog = screen.getByRole('dialog');
    expect(
      within(dialog).getByText(messageWith('dice.modeTitle', { label: 'Wisdom modifier' })),
    ).toBeTruthy();
    expect(within(dialog).getByRole('button', { name: message('dice.mode.normal') })).toBeTruthy();
    expect(
      within(dialog).getByRole('button', { name: message('dice.mode.advantage') }),
    ).toBeTruthy();
    expect(
      within(dialog).getByRole('button', { name: message('dice.mode.disadvantage') }),
    ).toBeTruthy();

    fireEvent.pointerUp(button);
    fireEvent.click(within(dialog).getByRole('button', { name: message('dice.mode.advantage') }));
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(screen.queryByRole('dialog')).toBeNull();
    const dice = panelDice();
    expect(dice).toHaveLength(2);
    expect(keptDie()).toBe(Math.max(...dice));
  }, 15000);

  it('Edge: a short tap rolls at once and opens no choice', async () => {
    await open();
    const button = wisdomButton();

    fireEvent.pointerDown(button);
    fireEvent.pointerUp(button);
    fireEvent.click(button);

    await waitFor(() => expect(panel()).not.toBeNull());
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panelDice()).toHaveLength(0);
  }, 15000);

  it('Happy path: the context key opens the same choice for keyboard users', async () => {
    const user = await open();
    wisdomButton().focus();

    await user.keyboard('{Shift>}{F10}{/Shift}');

    const dialog = screen.getByRole('dialog');
    fireEvent.click(
      within(dialog).getByRole('button', { name: message('dice.mode.disadvantage') }),
    );
    await waitFor(() => expect(panel()).not.toBeNull());

    const dice = panelDice();
    expect(keptDie()).toBe(Math.min(...dice));
    expect(announcement()).toBe(
      messageWith('dice.rolledDisadvantage', { label: 'Wisdom modifier', total: total() }),
    );
  }, 15000);

  it('Edge: Escape closes the choice without rolling', async () => {
    const user = await open();
    wisdomButton().focus();

    await user.keyboard('{Shift>}{F10}{/Shift}');
    await user.keyboard('{Escape}');

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panel()).toBeNull();
  }, 15000);

  it('Edge: neither the choice nor a modifier changes a non-d20 rollable', async () => {
    const user = await open();
    const button = damageButton();
    button.focus();

    await user.keyboard('{Shift>}{F10}{/Shift}');

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panel()).toBeNull();
    expect(button).not.toHaveAttribute('aria-haspopup', 'dialog');
  }, 15000);
});
