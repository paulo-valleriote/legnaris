import { afterEach, describe, expect, it } from 'vitest';
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { LONG_PRESS_MS } from '../sheet/dice_row';
import {
  FANTASY_SHEET_ID,
  fantasySheetData,
  fantasySheetPayload,
  fantasyTemplatePayload,
} from './fantasy_fixtures';
import { initiativeRollName, rollName } from './dice_names';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';
import { arrangeTemplatePayload, createArrangeStub } from './arrange_stub';
import { SHEET_ID } from './sheet_fixtures';

const BASE_URL = 'http://api.test';

interface NotD20 {
  readonly section?: string[];
  readonly row?: string[];
}

function diceWith(notD20: NotD20): Record<string, unknown> {
  const base = fantasyTemplatePayload()['dice'] as Record<string, unknown>;
  return { ...base, not_d20: { section: notD20.section ?? [], row: notD20.row ?? [] } };
}

async function openPlay(
  notD20: NotD20,
  sheetDice?: Record<string, unknown>,
): Promise<ReturnType<typeof userEvent.setup>> {
  const fake = createFakeTransport(
    successOutcome(
      fantasySheetPayload({
        data: fantasySheetData(),
        ...(sheetDice === undefined ? {} : { dice: sheetDice }),
      }),
    ),
    successOutcome(fantasyTemplatePayload({ dice: diceWith(notD20) })),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FANTASY_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return user;
}

const EDIT_DICE = {
  section: {},
  row: { 'alpha.one': '1d20+alpha.two', 'alpha.two': '1d20+alpha.one' },
};

function editTemplate(notD20: NotD20) {
  return {
    ...arrangeTemplatePayload(),
    dice: { ...EDIT_DICE, not_d20: { section: notD20.section ?? [], row: notD20.row ?? [] } },
  };
}

async function openEdit(notD20: NotD20): Promise<ReturnType<typeof userEvent.setup>> {
  const stub = createArrangeStub({
    template: editTemplate(notD20),
    data: { alpha: { one: 5, two: 3, three: 0 } },
  });
  const user = userEvent.setup();
  renderApp(`/sheets/${SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
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

function initiativeButton(): HTMLElement {
  return within(screen.getByTestId('sheet-initiative')).getByRole('button', {
    name: initiativeRollName('+3'),
  });
}

function editRollButton(label: string): HTMLElement {
  return screen.getByRole('button', { name: rollName(label) });
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

async function longPress(element: HTMLElement) {
  fireEvent.pointerDown(element);
  await act(() => new Promise((resolve) => setTimeout(resolve, LONG_PRESS_MS + 50)));
  fireEvent.pointerUp(element);
  fireEvent.click(element);
}

afterEach(cleanup);

describe('a Template marking a Row as not a d20 test in Play', () => {
  it('Happy path: Shift-click on a marked d20 throws one die and offers no mode', async () => {
    const user = await openPlay({ row: ['stats.wisdom'] });

    expect(wisdomButton()).not.toHaveAttribute('aria-haspopup', 'dialog');
    await clickWith(user, wisdomButton(), 'Shift');
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
    expect(total()).toBeGreaterThanOrEqual(0);
    expect(total()).toBeLessThanOrEqual(19);
    expect(announcement()).toBe(
      messageWith('dice.rolled', { label: 'Wisdom modifier', total: total() }),
    );
    expect(panel()?.textContent).not.toContain(message('dice.mode.advantage'));
  }, 15000);

  it('Happy path: Alt-click on a marked d20 throws one die and offers no mode', async () => {
    const user = await openPlay({ row: ['stats.wisdom'] });

    await clickWith(user, wisdomButton(), 'Alt');
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
    expect(announcement()).toBe(
      messageWith('dice.rolled', { label: 'Wisdom modifier', total: total() }),
    );
    expect(panel()?.textContent).not.toContain(message('dice.mode.disadvantage'));
  }, 15000);

  it('Happy path: the Context Menu key opens no choice and throws one die', async () => {
    const user = await openPlay({ row: ['stats.wisdom'] });
    wisdomButton().focus();

    await user.keyboard('{Shift>}{F10}{/Shift}');

    expect(screen.queryByRole('dialog')).toBeNull();
    await waitFor(() => expect(panel()).not.toBeNull());
    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
  }, 15000);

  it('Happy path: a long press opens no choice and the release throws one die', async () => {
    await openPlay({ row: ['stats.wisdom'] });

    await longPress(wisdomButton());

    await waitFor(() => expect(panel()).not.toBeNull());
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
  }, 15000);

  it('Happy path: the Context Menu event on a marked d20 throws one die and opens no choice', async () => {
    await openPlay({ row: ['stats.wisdom'] });

    fireEvent.contextMenu(wisdomButton());

    await waitFor(() => expect(panel()).not.toBeNull());
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
  }, 15000);

  it('Edge: an unmarked neighbour keeps advantage', async () => {
    const user = await openPlay({ row: ['stats.wisdom'] });

    expect(initiativeButton()).toHaveAttribute('aria-haspopup', 'dialog');
    await clickWith(user, initiativeButton(), 'Shift');
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(panelDice()).toHaveLength(2);
    expect(droppedDice()).toHaveLength(1);
    expect(announcement()).toContain(`with ${message('dice.mode.advantage').toLowerCase()}`);
  }, 15000);

  it('Happy path: a marked Section covers its Rows', async () => {
    const user = await openPlay({ section: ['stats'] });

    expect(wisdomButton()).not.toHaveAttribute('aria-haspopup', 'dialog');
    await clickWith(user, wisdomButton(), 'Shift');
    await waitFor(() => expect(panel()).not.toBeNull());
    expect(panelDice()).toHaveLength(0);

    expect(initiativeButton()).toHaveAttribute('aria-haspopup', 'dialog');
  }, 15000);

  it('Edge: a player override on a marked Row still offers no mode', async () => {
    const user = await openPlay(
      { row: ['stats.wisdom'] },
      { section: {}, row: { 'stats.wisdom': '1d20 + 5' } },
    );

    expect(wisdomButton()).not.toHaveAttribute('aria-haspopup', 'dialog');
    await clickWith(user, wisdomButton(), 'Shift');
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
    expect(panel()?.textContent).toContain('1d20');
    expect(total()).toBeGreaterThanOrEqual(6);
    expect(total()).toBeLessThanOrEqual(25);
  }, 15000);
});

describe('a Template marking a Row as not a d20 test in Edit', () => {
  it('Happy path: Shift-click on a marked d20 throws one die and offers no mode', async () => {
    const user = await openEdit({ row: ['alpha.one'] });

    const button = editRollButton('One');
    expect(button).not.toHaveAttribute('aria-haspopup', 'dialog');
    await clickWith(user, button, 'Shift');
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
    expect(announcement()).toBe(messageWith('dice.rolled', { label: 'One', total: total() }));
  }, 15000);

  it('Edge: an unmarked neighbour keeps advantage', async () => {
    const user = await openEdit({ row: ['alpha.one'] });

    const neighbour = editRollButton('Two');
    expect(neighbour).toHaveAttribute('aria-haspopup', 'dialog');
    await clickWith(user, neighbour, 'Shift');
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(panelDice()).toHaveLength(2);
    expect(droppedDice()).toHaveLength(1);
  }, 15000);

  it('Edge: a marked d20 keyboard activation throws one die without a choice', async () => {
    const user = await openEdit({ row: ['alpha.one'] });
    const button = editRollButton('One');
    button.focus();

    await user.keyboard('{Shift>}{F10}{/Shift}');

    expect(screen.queryByRole('dialog')).toBeNull();
    await waitFor(() => expect(panel()).not.toBeNull());
    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
  }, 15000);

  it('Happy path: a long press on a marked d20 throws one die without an editor', async () => {
    await openEdit({ row: ['alpha.one'] });

    await longPress(editRollButton('One'));

    await waitFor(() => expect(panel()).not.toBeNull());
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
  }, 15000);

  it('Happy path: the Context Menu event on a marked d20 throws one die without a choice', async () => {
    await openEdit({ row: ['alpha.one'] });

    fireEvent.contextMenu(editRollButton('One'));

    await waitFor(() => expect(panel()).not.toBeNull());
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
  }, 15000);
});
