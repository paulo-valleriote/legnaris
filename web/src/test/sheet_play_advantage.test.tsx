import { afterEach, describe, expect, it } from 'vitest';
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { LONG_PRESS_MS } from '../sheet/dice_row';
import {
  FIFTH_CHARACTER_SHEET_ID,
  FIFTH_STAT_BLOCK_SHEET_ID,
  fifthCharacterSheetData,
  fifthCharacterSheetPayload,
  fifthCharacterTemplatePayload,
  fifthStatBlockSheetData,
  fifthStatBlockSheetPayload,
  fifthStatBlockTemplatePayload,
} from './fifth_fixtures';
import { rollName } from './dice_names';
import { createFakeTransport, successOutcome } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';

function characterData(): Record<string, unknown> {
  const base = fifthCharacterSheetData();
  const abilities = base.abilities as Record<string, Record<string, number>>;
  return {
    ...base,
    abilities: { ...abilities, wisdom: { score: 8, mod: -1 } },
  };
}

async function openCharacter() {
  const fake = createFakeTransport(
    successOutcome(fifthCharacterSheetPayload({ data: characterData() })),
    successOutcome(fifthCharacterTemplatePayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FIFTH_CHARACTER_SHEET_ID}`, {
    client: createApiClient({ baseUrl: BASE_URL, transport: fake.transport }),
  });
  await screen.findByTestId('sheet-name');
  return user;
}

function statBlockData(): Record<string, unknown> {
  const base = fifthStatBlockSheetData();
  const hitPoints = base.hitPoints as Record<string, number>;
  return {
    ...base,
    hitPoints: { ...hitPoints, count: 2, size: 8, conBonus: 0, maximum: 9 },
  };
}

async function openStatBlock() {
  const fake = createFakeTransport(
    successOutcome(fifthStatBlockSheetPayload({ data: statBlockData() })),
    successOutcome(fifthStatBlockTemplatePayload()),
    successOutcome({}),
  );
  const user = userEvent.setup();
  renderApp(`/sheets/${FIFTH_STAT_BLOCK_SHEET_ID}`, {
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
  return within(screen.getByTestId('compose-ability-abilities.wisdom.score')).getByRole('button', {
    name: rollName('Wisdom modifier'),
  });
}

function hitPointMaximumButton(): HTMLElement {
  return within(screen.getByTestId('compose-hit-points')).getByRole('button', {
    name: /^Roll Hit points/,
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
    const user = await openCharacter();

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
    const user = await openCharacter();

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
    const user = await openCharacter();

    await user.keyboard('{Shift>}{Alt>}');
    await user.click(wisdomButton());
    await user.keyboard('{/Alt}{/Shift}');
    await waitFor(() => expect(panel()).not.toBeNull());

    const dice = panelDice();
    expect(keptDie()).toBe(Math.max(...dice));
    expect(announcement()).toContain(`with ${message('dice.mode.advantage').toLowerCase()}`);
  }, 15000);

  it('Edge: a plain click still rolls one d20 and names no mode', async () => {
    const user = await openCharacter();

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
  it('Edge: Shift-click on a non-d20 roll keeps every die of its own expression', async () => {
    const user = await openStatBlock();

    await clickWith(user, hitPointMaximumButton(), 'Shift');
    await waitFor(() => expect(panel()).not.toBeNull());

    expect(panelDice()).toHaveLength(0);
    expect(droppedDice()).toHaveLength(0);
    expect(total()).toBeGreaterThanOrEqual(2);
    expect(total()).toBeLessThanOrEqual(16);
    expect(announcement()).toBe(
      messageWith('dice.rolled', { label: 'Hit points', total: total() }),
    );
    expect(panel()?.textContent).not.toContain(message('dice.mode.advantage'));
  }, 15000);
});

describe('the small roll choice', () => {
  it('Happy path: a long press offers Normal, Advantage and Disadvantage, then rolls the choice', async () => {
    await openCharacter();
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
    await openCharacter();
    const button = wisdomButton();

    fireEvent.pointerDown(button);
    fireEvent.pointerUp(button);
    fireEvent.click(button);

    await waitFor(() => expect(panel()).not.toBeNull());
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panelDice()).toHaveLength(0);
  }, 15000);

  it('Happy path: the context key opens the same choice for keyboard users', async () => {
    const user = await openCharacter();
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
    const user = await openCharacter();
    wisdomButton().focus();

    await user.keyboard('{Shift>}{F10}{/Shift}');
    await user.keyboard('{Escape}');

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panel()).toBeNull();
  }, 15000);

  it('Edge: neither the choice nor a modifier changes a non-d20 rollable', async () => {
    const user = await openStatBlock();
    const button = hitPointMaximumButton();
    button.focus();

    await user.keyboard('{Shift>}{F10}{/Shift}');

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(panel()).toBeNull();
    expect(button).not.toHaveAttribute('aria-haspopup', 'dialog');
  }, 15000);
});
