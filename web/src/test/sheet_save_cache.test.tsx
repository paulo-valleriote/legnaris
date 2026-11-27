import { describe, expect, it, vi } from 'vitest';
import { QueryClient } from '@tanstack/react-query';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import type { HttpRequest, HttpResponse, Transport } from '../api/transport';
import { parseSheet } from '../api/models/sheet';
import { parseTemplateDetail } from '../api/models/template';
import { message } from '../l10n/strings';
import { seedCreatedSheet, sheetKey } from '../catalog/queries';
import { createMemorySheetStore } from '../library/sheet_store';
import { createSheetEditStore } from '../sheet/sheet_edit';
import {
  FIFTH_CHARACTER_SHEET_ID,
  fifthCharacterSheet,
  fifthCharacterTemplatePayload,
} from './fifth_fixtures';
import { createFakeTransport, errorOutcome, successOutcome, withPlanUsage } from './fake_transport';
import { renderApp } from './render';
import { sheetPayload, templateDetailPayload } from './sheet_fixtures';
import { startEditing } from './start_editing';

vi.setConfig({ testTimeout: 15_000 });

const BASE_URL = 'http://api.test';
const SHEET_PATH = `/sheets/${FIFTH_CHARACTER_SHEET_ID}`;

interface RoutedTransport {
  readonly transport: Transport;
  readonly requests: HttpRequest[];
}

function routedTransport(save: HttpResponse): RoutedTransport {
  const requests: HttpRequest[] = [];
  const transport: Transport = (request) => {
    requests.push(request);
    if (request.method === 'PATCH') {
      return Promise.resolve(save);
    }
    if (request.url.includes('/templates/')) {
      return Promise.resolve(successOutcome(fifthCharacterTemplatePayload()));
    }
    return Promise.resolve(successOutcome({}));
  };
  return { transport: withPlanUsage(transport), requests };
}

interface DeferredTransport {
  readonly transport: Transport;
  readonly requests: HttpRequest[];
  resolve(response: HttpResponse): void;
}

function deferredTransport(): DeferredTransport {
  const requests: HttpRequest[] = [];
  let settle: ((response: HttpResponse) => void) | undefined;
  const transport: Transport = (request) => {
    requests.push(request);
    if (request.method === 'PATCH') {
      return new Promise<HttpResponse>((resolve) => {
        settle = resolve;
      });
    }
    if (request.url.includes('/templates/')) {
      return Promise.resolve(successOutcome(fifthCharacterTemplatePayload()));
    }
    return Promise.resolve(successOutcome({}));
  };
  return {
    transport: withPlanUsage(transport),
    requests,
    resolve(response) {
      settle?.(response);
    },
  };
}

function appQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

async function renameSheet(user: ReturnType<typeof userEvent.setup>, name: string): Promise<void> {
  await startEditing();
  await user.click(
    within(screen.getByTestId('sheet-name')).getByRole('button', { name: /^Name(, .+)?$/ }),
  );
  const input = within(screen.getByTestId('sheet-name')).getByRole('textbox', { name: 'Name' });
  await user.clear(input);
  await user.type(input, `${name}{Enter}`);
}

async function editClass(user: ReturnType<typeof userEvent.setup>, value: string): Promise<void> {
  await user.click(screen.getByRole('button', { name: /^Class(, .+)?$/ }));
  const input = screen.getByRole('textbox', { name: 'Class' });
  await user.clear(input);
  await user.type(input, `${value}{Enter}`);
}

describe('the saved sheet in the query cache', () => {
  it('Happy path: a saved edit survives leaving and reopening the sheet', async () => {
    const fake = routedTransport(successOutcome({}));
    const queryClient = appQueryClient();
    seedCreatedSheet(queryClient, fifthCharacterSheet());
    const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
    const user = userEvent.setup();

    const first = renderApp(SHEET_PATH, { client, queryClient });
    await screen.findByTestId('sheet-name');
    await renameSheet(user, 'Brenna the Bold');
    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
    await screen.findByTestId('sheet-pending-saved');
    first.unmount();

    renderApp(SHEET_PATH, { client, queryClient });

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Brenna the Bold');
  });

  it('Happy path: an edit made during the save stays pending after the cache write', async () => {
    const fake = deferredTransport();
    const queryClient = appQueryClient();
    seedCreatedSheet(queryClient, fifthCharacterSheet());
    const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
    const user = userEvent.setup();

    renderApp(SHEET_PATH, { client, queryClient });
    await screen.findByTestId('sheet-name');
    await renameSheet(user, 'Brenna the Bold');
    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
    await editClass(user, 'Rogue');
    fake.resolve(successOutcome({}));

    await waitFor(() =>
      expect(
        queryClient.getQueryData<{ data: unknown }>(sheetKey(FIFTH_CHARACTER_SHEET_ID)),
      ).toMatchObject({ data: { identity: { name: 'Brenna the Bold' } } }),
    );
    expect(screen.getByRole('button', { name: /^Class(, .+)?$/ })).toHaveTextContent('Rogue');
    expect(screen.getByTestId('sheet-pending-card')).toHaveTextContent('1 change');
  });

  it('Happy path: an applied reset publishes the new baseline', () => {
    const onSaved = vi.fn();
    const store = createSheetEditStore(
      {
        template: parseTemplateDetail(templateDetailPayload()),
        sheet: parseSheet(sheetPayload()),
      },
      {
        client: createApiClient({
          baseUrl: BASE_URL,
          transport: createFakeTransport(successOutcome({})).transport,
        }),
        store: createMemorySheetStore(),
        onSaved,
      },
    );

    store.applyReset({ identity: { level: 7 } });

    expect(onSaved).toHaveBeenCalledWith({
      data: expect.objectContaining({ identity: expect.objectContaining({ level: 7 }) }),
      dice: { section: {}, row: {} },
    });
  });

  it('Error: a failed save leaves the cached sheet unchanged', async () => {
    const fake = routedTransport(errorOutcome('INTERNAL', 'save failed', 500));
    const queryClient = appQueryClient();
    seedCreatedSheet(queryClient, fifthCharacterSheet());
    const client = createApiClient({ baseUrl: BASE_URL, transport: fake.transport });
    const user = userEvent.setup();

    const first = renderApp(SHEET_PATH, { client, queryClient });
    await screen.findByTestId('sheet-name');
    await renameSheet(user, 'Brenna the Bold');
    await user.click(screen.getByRole('button', { name: message('sheet.pending.save') }));
    await waitFor(() => expect(screen.getByTestId('sheet-pending-error')).toBeInTheDocument());
    first.unmount();

    renderApp(SHEET_PATH, { client, queryClient });

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Brenna');
    expect(screen.getByTestId('sheet-name')).not.toHaveTextContent('Brenna the Bold');
  });
});
