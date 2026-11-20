import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import { arrangeTemplatePayload, createArrangeStub } from './arrange_stub';
import { rollName } from './dice_names';
import { renderApp } from './render';
import { SHEET_ID } from './sheet_fixtures';

const SHEET_URL = `/sheets/${SHEET_ID}`;
const BASE_URL = 'http://api.test';

const DICE_TEMPLATE = {
  ...arrangeTemplatePayload(),
  dice: {
    section: {},
    row: {
      'alpha.one': '1d20',
      'alpha.two': '1d20',
      'alpha.three': '1d20',
      'gamma.solo': '1d20',
    },
  },
};

async function open() {
  const stub = createArrangeStub({ template: DICE_TEMPLATE });
  const user = userEvent.setup();
  const view = renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
  });
  await screen.findByTestId('sheet-name');
  return { user, view };
}

const rollButton = (label: string) => screen.getByRole('button', { name: rollName(label) });
const labels = () =>
  [...document.querySelectorAll<HTMLElement>('.lg-dice-panel__total-label')].map(
    (label) => label.textContent,
  );

function diceStatuses(): HTMLElement[] {
  const panel = document.querySelector<HTMLElement>('.lg-dice-panel');
  return panel === null ? [] : within(panel).getAllByRole('status');
}

async function roll(user: ReturnType<typeof userEvent.setup>, label: string): Promise<void> {
  await user.click(rollButton(label));
  await waitFor(() => expect(labels()).toContain(label));
}

afterEach(cleanup);

describe('the recent rolls history', () => {
  it('Edge: the first roll ever is the only result and its announcement', async () => {
    const { user } = await open();

    await roll(user, 'One');

    expect(labels()).toEqual(['One']);
    const statuses = diceStatuses();
    expect(statuses).toHaveLength(1);
    expect(statuses[0].textContent).toMatch(
      new RegExp(`^${messageWith('dice.rolled', { label: 'One', total: '\\d+' })}$`),
    );
  });

  it('Happy path: four rolls keep the three newest, newest first', async () => {
    const { user } = await open();

    await roll(user, 'One');
    await roll(user, 'Two');
    await roll(user, 'Solo');
    await roll(user, 'Three');

    expect(labels()).toEqual(['Three', 'Solo', 'Two']);
    expect(diceStatuses()).toHaveLength(1);
    expect(diceStatuses()[0].textContent).toContain('Three');
    expect(diceStatuses()[0].textContent).not.toContain('Solo');
  });

  it('Happy path: dismissing the panel clears the history for the next roll', async () => {
    const { user } = await open();
    await roll(user, 'One');
    await roll(user, 'Two');

    await user.click(screen.getByRole('button', { name: message('common.close') }));

    expect(document.querySelector('.lg-dice-panel')).toBeNull();
    await roll(user, 'Solo');
    expect(labels()).toEqual(['Solo']);
  });

  it('Edge: a reload starts with no history', async () => {
    const first = await open();
    await roll(first.user, 'One');
    expect(labels()).toEqual(['One']);

    first.view.unmount();
    await open();

    expect(document.querySelector('.lg-dice-panel')).toBeNull();
    expect(labels()).toEqual([]);
  });
});
