import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { message, messageWith } from '../l10n/strings';
import {
  CAPABILITIES_ACTION_LABEL,
  CAPABILITIES_LIST_PATH,
  CAPABILITIES_RECORD_ACTIVE,
  CAPABILITIES_SHEET_ID,
  CAPABILITIES_TEMPLATE_ID,
  capabilitiesResetCascade,
  capabilitiesSheetPayload,
} from './capabilities_fixture';
import { createCapabilitiesStub } from './capabilities_stub';
import { rollName } from './dice_names';
import { successOutcome } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';
const SHEET_URL = `/sheets/${CAPABILITIES_SHEET_ID}`;

function recordRows(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>('[data-row-kind="record"]'));
}

function openSheet(options: Parameters<typeof createCapabilitiesStub>[0] = {}) {
  const stub = createCapabilitiesStub(options);
  const user = userEvent.setup();
  const view = renderApp(SHEET_URL, {
    client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
  });
  return { stub, user, view };
}

function panel(): HTMLElement | null {
  return document.querySelector<HTMLElement>('.lg-dice-panel');
}

async function dismissPanel(user: ReturnType<typeof userEvent.setup>) {
  await user.click(
    within(panel() as HTMLElement).getByRole('button', { name: message('common.close') }),
  );
  await waitFor(() => expect(panel()).toBeNull());
}

afterEach(cleanup);

describe('the six-capability fixture through the generic form', () => {
  it('Happy path: the fixture creates from the dialog with one of its accepted kinds', async () => {
    const stub = createCapabilitiesStub({
      createdSheet: capabilitiesSheetPayload({ kind: 'enemy' }),
    });
    const user = userEvent.setup();
    renderApp('/', {
      client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
    });

    await user.click(await screen.findByRole('button', { name: /Capabilities Test/ }));
    await screen.findByTestId('detail-name');

    const kind = screen.getByLabelText(message('catalog.kindLabel'));
    expect(kind).toHaveValue('npc');
    expect(screen.queryByRole('option', { name: message('catalog.kindCharacter') })).toBeNull();
    expect(screen.getByRole('option', { name: message('kind.npc') })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: message('kind.enemy') })).toBeInTheDocument();

    await user.selectOptions(kind, 'enemy');
    await user.type(screen.getByLabelText(message('sheet.nameLabel')), 'Ember Warden');
    await user.click(screen.getByRole('button', { name: message('sheet.create') }));

    await waitFor(() => expect(stub.createRequests()).toHaveLength(1));
    expect(JSON.parse(stub.createRequests()[0]?.body ?? '{}')).toEqual({
      template_id: CAPABILITIES_TEMPLATE_ID,
      kind: 'enemy',
      sheet_name: 'Ember Warden',
    });

    await screen.findByTestId('sheet-name');
    expect(screen.getByRole('button', { name: CAPABILITIES_ACTION_LABEL })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Inventory' })).toBeInTheDocument();
    expect(recordRows()).toHaveLength(2);
    expect(screen.getByRole('checkbox', { name: 'Inspired' })).toBeChecked();
  });

  it('Happy path: a flag and a Record edit leave the sheet unsaved and save in one delta', async () => {
    const { stub, user } = openSheet();
    await screen.findByTestId('sheet-name');

    await user.click(screen.getByRole('checkbox', { name: 'Inspired' }));
    const quantity = within(recordRows()[0] as HTMLElement).getByRole('textbox', {
      name: 'Quantity',
    });
    await user.clear(quantity);
    await user.type(quantity, '5');

    expect(screen.getByText(message('sheet.pendingEdits'))).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: message('sheet.save') }));

    await waitFor(() => expect(stub.patchRequests()).toHaveLength(1));
    expect(JSON.parse(stub.patchRequests()[0]?.body ?? '{}')).toEqual({
      data: {
        flags: { inspired: 0 },
        inventory: { items: { [CAPABILITIES_RECORD_ACTIVE]: { quantity: 5 } } },
      },
    });
  });

  it('Happy path: the Field-sized dice roll throws as many dice as the Fields say, raised to the minimum', async () => {
    const { user } = openSheet();
    await screen.findByTestId('sheet-name');

    await user.click(screen.getByRole('button', { name: rollName('Level') }));

    await waitFor(() => expect(panel()).not.toBeNull());
    expect(document.querySelector('.lg-dice-panel__total-value')).toHaveTextContent('50');
    expect(document.querySelector('.lg-dice-panel__minimum')).toHaveTextContent(
      messageWith('dice.minimum', { minimum: 50 }),
    );
    const rolls = document.querySelector('.lg-dice-panel__rolls')?.textContent ?? '';
    expect(rolls.split(',')).toHaveLength(3);
  });

  it('Happy path: the not-a-d20 Row offers no mode and throws one die', async () => {
    const { user } = openSheet();
    await screen.findByTestId('sheet-name');

    const roll = screen.getByRole('button', { name: rollName('Insight') });
    expect(roll).not.toHaveAttribute('aria-haspopup', 'dialog');
    await user.keyboard('{Shift>}');
    await user.click(roll);
    await user.keyboard('{/Shift}');

    await waitFor(() => expect(panel()).not.toBeNull());
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(document.querySelector('.lg-dice-panel__mode')).toBeNull();
    const rolls = document.querySelector('.lg-dice-panel__rolls')?.textContent ?? '';
    expect(rolls.split(',')).toHaveLength(1);
    expect(
      Number(document.querySelector('.lg-dice-panel__total-value')?.textContent),
    ).toBeGreaterThanOrEqual(4);
  });

  it('Happy path: the reset applies after the rolls and the sheet stays saved', async () => {
    const stub = createCapabilitiesStub({
      resets: [successOutcome(capabilitiesResetCascade())],
    });
    const user = userEvent.setup();
    renderApp(SHEET_URL, {
      client: createApiClient({ baseUrl: BASE_URL, transport: stub.transport }),
    });
    await screen.findByTestId('sheet-name');

    await user.click(screen.getByRole('button', { name: rollName('Level') }));
    await waitFor(() => expect(panel()).not.toBeNull());
    await dismissPanel(user);

    await user.click(screen.getByRole('button', { name: CAPABILITIES_ACTION_LABEL }));

    await waitFor(() => expect(stub.resetRequests()).toHaveLength(1));
    expect(screen.getByTestId('sheet-reset-notice')).toHaveTextContent(
      messageWith('sheet.reset.applied', { label: CAPABILITIES_ACTION_LABEL }),
    );
    expect(screen.queryByText(message('sheet.pendingEdits'))).toBeNull();
    expect(screen.getByRole('textbox', { name: 'Level' })).toHaveValue('2');
    expect(
      within(screen.getByRole('group', { name: /^Insight\b/ })).getByText('4'),
    ).toBeInTheDocument();
    expect(
      within(recordRows()[0] as HTMLElement).getByRole('textbox', { name: 'Quantity' }),
    ).toHaveValue('3');
    expect(recordRows()[0]?.dataset.rowKey).toBe(
      `${CAPABILITIES_LIST_PATH}.${CAPABILITIES_RECORD_ACTIVE}`,
    );
  });
});
