import { describe, expect, it, vi } from 'vitest';
import { QueryClient } from '@tanstack/react-query';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { message } from '../l10n/strings';
import { seedCreatedSheet, sheetKey } from '../catalog/queries';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheet,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import { successOutcome, withPlanUsage } from './fake_transport';
import { renderApp } from './render';

vi.setConfig({ testTimeout: 15_000 });

const BASE_URL = 'http://api.test';
const SHEET_PATH = `/sheets/${FIFTH_CHARACTER_SHEET_ID}`;
const CAMPAIGN_ID = '01a21d57-9e79-7799-8572-a275ebc5ce18';

function routedTransport(): { readonly transport: Transport; readonly requests: HttpRequest[] } {
  const requests: HttpRequest[] = [];
  const transport: Transport = (request) => {
    requests.push(request);
    if (request.method === 'PATCH') {
      return Promise.resolve(successOutcome({}) satisfies HttpResponse);
    }
    if (request.url.includes('/templates/')) {
      return Promise.resolve(
        successOutcome(fifthCharacterTemplatePayload()) satisfies HttpResponse,
      );
    }
    return Promise.resolve(successOutcome({}) satisfies HttpResponse);
  };
  return { transport: withPlanUsage(transport), requests };
}

function appQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

async function renameSheet(user: ReturnType<typeof userEvent.setup>, name: string): Promise<void> {
  const edit = screen.queryByRole('button', { name: message('sheet.edit') });
  if (edit !== null) {
    await user.click(edit);
  }
  await user.click(
    within(screen.getByTestId('sheet-name')).getByRole('button', { name: /^Name(, .+)?$/ }),
  );
  const input = within(screen.getByTestId('sheet-name')).getByRole('textbox', { name: 'Name' });
  await user.clear(input);
  await user.type(input, `${name}{Enter}`);
}

async function save(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
}

function seedLists(queryClient: QueryClient): void {
  queryClient.setQueryData(['sheets', 'owned', 'character'], []);
  queryClient.setQueryData(['campaigns'], []);
  queryClient.setQueryData(['campaign', CAMPAIGN_ID], {});
  queryClient.setQueryData(['campaign', CAMPAIGN_ID, 'sheets'], []);
  queryClient.setQueryData(['campaign', CAMPAIGN_ID, 'npcs'], []);
}

describe('the top-level sheet name', () => {
  it('Happy path: a saved identity.name renames the sheet and refreshes the name lists', async () => {
    const fake = routedTransport();
    const queryClient = appQueryClient();
    seedCreatedSheet(queryClient, fifthCharacterSheet({ campaign_id: CAMPAIGN_ID }));
    seedLists(queryClient);
    const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
    const user = userEvent.setup();

    renderApp(SHEET_PATH, { client, queryClient });
    await screen.findByTestId('sheet-name');
    await renameSheet(user, 'Rune');
    await save(user);

    await waitFor(() =>
      expect(
        queryClient.getQueryData<{ name: string }>(sheetKey(FIFTH_CHARACTER_SHEET_ID)),
      ).toMatchObject({ name: 'Rune' }),
    );
    for (const key of [
      ['sheets', 'owned', 'character'],
      ['campaigns'],
      ['campaign', CAMPAIGN_ID],
      ['campaign', CAMPAIGN_ID, 'sheets'],
      ['campaign', CAMPAIGN_ID, 'npcs'],
    ] as const) {
      expect(queryClient.getQueryState(key)?.isInvalidated).toBe(true);
    }
  });

  it('Edge: clearing identity.name keeps the previous name', async () => {
    const fake = routedTransport();
    const queryClient = appQueryClient();
    seedCreatedSheet(queryClient, fifthCharacterSheet({ campaign_id: CAMPAIGN_ID }));
    const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
    const user = userEvent.setup();

    renderApp(SHEET_PATH, { client, queryClient });
    await screen.findByTestId('sheet-name');
    await renameSheet(user, 'Rune');
    await save(user);
    await waitFor(() =>
      expect(
        queryClient.getQueryData<{ name: string }>(sheetKey(FIFTH_CHARACTER_SHEET_ID)),
      ).toMatchObject({ name: 'Rune' }),
    );

    await renameSheet(user, '');
    await save(user);

    await waitFor(() =>
      expect(
        queryClient.getQueryData<{ name: string }>(sheetKey(FIFTH_CHARACTER_SHEET_ID)),
      ).toMatchObject({ name: 'Rune' }),
    );
    await waitFor(() => expect(screen.getByTestId('sheet-name')).toHaveTextContent('Rune'));
  });
});
