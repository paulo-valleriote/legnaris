import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { Transport } from '../api/transport';
import type { TemplateSummary } from '../api/models/template';
import { failureMessage } from '../api/error_mapper';
import { message, messageWith } from '../l10n/strings';
import { createFakeTransport, errorOutcome, successOutcome, withPlanUsage } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';
const ENTRY_NAME = /Dungeons & Dragons 5e/;

const CATALOG: readonly TemplateSummary[] = [
  { id: 'dnd5e', name: 'Dungeons & Dragons 5e', default: true, fields: {} },
];

const DETAIL = { id: 'dnd5e', name: 'Dungeons & Dragons 5e', default: true, fields: {} };

const CHARACTER_FULL = {
  plan: 'free',
  sheets: {
    character: { used: 1, limit: 1 },
    npc: { used: 0, limit: 2 },
    monster: { used: 0, limit: 2 },
  },
};

const CHARACTER_ROOM = {
  plan: 'free',
  sheets: {
    character: { used: 0, limit: 1 },
    npc: { used: 0, limit: 2 },
    monster: { used: 0, limit: 2 },
  },
};

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

/** Opens the surface for the template and waits for its detail to settle. */
async function openSurface(transport: Transport) {
  const user = userEvent.setup();
  renderApp('/', { client: clientWith(transport) });
  await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
  await screen.findByTestId('detail-name');
  return user;
}

describe('creation surface plan allowance (LGN-183)', () => {
  it('Happy path: the surface shows the used and allowed Sheets of its kind once the usage loads', async () => {
    const fake = createFakeTransport(successOutcome(CATALOG), successOutcome(DETAIL));
    const user = await openSurface(withPlanUsage(fake.transport, successOutcome(CHARACTER_ROOM)));

    expect(
      await screen.findByText(messageWith('sheet.usage', { used: 0, limit: 1 })),
    ).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-limit-reached')).not.toBeInTheDocument();

    await user.type(screen.getByLabelText(message('sheet.nameLabel')), 'Aragorn');
    expect(screen.getByRole('button', { name: message('sheet.create') })).toBeEnabled();
  });

  it('Edge: at the allowance the primary action is disabled even with a valid name', async () => {
    const fake = createFakeTransport(successOutcome(CATALOG), successOutcome(DETAIL));
    const user = await openSurface(withPlanUsage(fake.transport, successOutcome(CHARACTER_FULL)));

    await user.type(screen.getByLabelText(message('sheet.nameLabel')), 'Aragorn');

    expect(await screen.findByTestId('sheet-limit-reached')).toHaveTextContent(
      message('sheet.limitReached'),
    );
    expect(screen.getByRole('button', { name: message('sheet.create') })).toBeDisabled();
    expect(fake.requests.filter((request) => request.method === 'POST')).toHaveLength(0);
  });

  it('Error: a refused creation shows the plan copy and keeps the surface open', async () => {
    const fake = createFakeTransport(
      successOutcome(CATALOG),
      successOutcome(DETAIL),
      errorOutcome('SHEET_LIMIT_REACHED', 'sheet limit reached for this kind', 409),
    );
    const user = await openSurface(withPlanUsage(fake.transport, successOutcome(CHARACTER_ROOM)));

    await user.type(screen.getByLabelText(message('sheet.nameLabel')), 'Second');
    await user.click(screen.getByRole('button', { name: message('sheet.create') }));

    expect(await screen.findByTestId('create-error')).toHaveTextContent(
      failureMessage('sheetLimitReached'),
    );
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('sheet.create') })).toBeEnabled(),
    );
  });
});
