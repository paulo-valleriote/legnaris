import { describe, expect, it } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createApiClient } from '../api/client';
import { failureMessage, parseFailureMessage } from '../api/error_mapper';
import type { Transport } from '../api/transport';
import { createMemorySheetStore } from '../library/sheet_store';
import { message } from '../l10n/strings';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';

const SHEET_ID = '01a0f967-f42b-710e-851f-8f293a634921';

const SHEET_JSON = {
  id: SHEET_ID,
  template_id: 'fantasy_v1',
  name: 'Aragorn',
  data: {},
  created_at: '2026-10-01T21:38:54.635Z',
  updated_at: '2026-10-02T09:00:00.000Z',
};

const TEMPLATE_DETAIL = {
  id: 'fantasy_v1',
  name: 'Fantasy RPG',
  default: true,
  fields: {},
};

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function sheetRequests(fake: ReturnType<typeof createFakeTransport>): string[] {
  return fake.requests.filter((request) => request.url.endsWith('/sheets')).map((r) => r.url);
}

function deleteRequests(fake: ReturnType<typeof createFakeTransport>): string[] {
  return fake.requests
    .filter((request) => request.method === 'DELETE')
    .map((request) => request.url);
}

async function findName(): Promise<HTMLElement> {
  return screen.findByTestId('your-sheet-name');
}

describe('the "Your sheet" destination', () => {
  it('Happy path: a person with a sheet renders it from exactly one GET /sheets', async () => {
    const fake = createFakeTransport(successOutcome(SHEET_JSON));

    renderApp('/your-sheet', { client: clientWith(fake.transport) });

    await waitFor(() => expect(screen.getByTestId('your-sheet-name')).toHaveTextContent('Aragorn'));
    expect(
      screen.getByRole('heading', { level: 1, name: message('sheet.ownTitle') }),
    ).toBeInTheDocument();
    expect(sheetRequests(fake)).toHaveLength(1);
    expect(screen.queryByTestId('your-sheet-failure')).not.toBeInTheDocument();
  });

  it('Happy path: Open navigates to the sheet route', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      successOutcome(SHEET_JSON),
      successOutcome(SHEET_JSON),
      successOutcome(TEMPLATE_DETAIL),
    );

    renderApp('/your-sheet', { client: clientWith(fake.transport) });
    await findName();

    await user.click(screen.getByRole('button', { name: message('sheet.ownOpen') }));

    await waitFor(() => expect(screen.getByTestId('sheet-screen')).toBeInTheDocument());
  });

  it('Edge: no sheet renders the designed empty state, not a failure', async () => {
    const fake = createFakeTransport(errorOutcome('NOT_FOUND', 'no sheet', 404));

    renderApp('/your-sheet', { client: clientWith(fake.transport) });

    await waitFor(() =>
      expect(screen.getByTestId('your-sheet-empty')).toHaveTextContent(message('sheet.ownEmpty')),
    );
    expect(screen.queryByTestId('your-sheet-failure')).not.toBeInTheDocument();
    expect(screen.queryByTestId('your-sheet-name')).not.toBeInTheDocument();
  });

  it('Edge: any other outcome renders the mapped copy with a Retry that issues exactly one more request', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
      successOutcome(SHEET_JSON),
    );

    renderApp('/your-sheet', { client: clientWith(fake.transport) });

    await waitFor(() =>
      expect(screen.getByTestId('your-sheet-failure')).toHaveTextContent(
        failureMessage('serverError'),
      ),
    );
    expect(sheetRequests(fake)).toHaveLength(1);

    await user.click(screen.getByRole('button', { name: message('common.retry') }));

    await waitFor(() => expect(screen.getByTestId('your-sheet-name')).toHaveTextContent('Aragorn'));
    expect(sheetRequests(fake)).toHaveLength(2);
  });

  it('Edge: an undecodable payload is a failure, not a crash', async () => {
    const fake = createFakeTransport({
      status: 200,
      body: JSON.stringify({ data: { not: 'a sheet' } }),
    });

    renderApp('/your-sheet', { client: clientWith(fake.transport) });

    await waitFor(() =>
      expect(screen.getByTestId('your-sheet-failure')).toHaveTextContent(parseFailureMessage()),
    );
    expect(screen.getByTestId('your-sheet-screen')).toBeInTheDocument();
  });

  it('Edge: cancel issues zero DELETEs and keeps the sheet', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(successOutcome(SHEET_JSON));

    renderApp('/your-sheet', { client: clientWith(fake.transport) });
    await findName();

    await user.click(screen.getByRole('button', { name: message('sheet.ownDelete') }));
    expect(screen.getByRole('dialog')).toHaveTextContent(message('sheet.ownDeleteMessage'));

    await user.click(screen.getByRole('button', { name: message('sheet.ownCancel') }));

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(deleteRequests(fake)).toHaveLength(0);
    expect(screen.getByTestId('your-sheet-name')).toHaveTextContent('Aragorn');
  });

  it('Happy path: the delete confirmation is one danger modal that focuses the safe action', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(successOutcome(SHEET_JSON));

    renderApp('/your-sheet', { client: clientWith(fake.transport) });
    await findName();

    await user.click(screen.getByRole('button', { name: message('sheet.ownDelete') }));

    const dialog = screen.getByRole('dialog', { name: message('sheet.ownDeleteTitle') });
    expect(dialog).toHaveClass('lg-dialog__panel--danger');
    expect(screen.getByRole('button', { name: message('sheet.ownCancel') })).toHaveFocus();

    await user.keyboard('{Escape}');

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(deleteRequests(fake)).toHaveLength(0);
  });

  it('Happy path: confirm issues exactly one DELETE, evicts the sheet and its draft and shows empty', async () => {
    const user = userEvent.setup();
    const store = createMemorySheetStore();
    await store.putSheet({
      id: SHEET_ID,
      templateId: 'fantasy_v1',
      name: 'Aragorn',
      data: {},
      createdAt: SHEET_JSON.created_at,
      updatedAt: SHEET_JSON.updated_at,
    });
    await store.writeDraft(SHEET_ID, { stats: { dexterity: 18 } });
    const fake = createFakeTransport(successOutcome(SHEET_JSON), { status: 204, body: '' });

    renderApp('/your-sheet', { client: clientWith(fake.transport), sheetStore: store });
    await findName();

    await user.click(screen.getByRole('button', { name: message('sheet.ownDelete') }));
    await user.click(screen.getByRole('button', { name: message('sheet.ownDeleteConfirm') }));

    await waitFor(() =>
      expect(screen.getByTestId('your-sheet-empty')).toHaveTextContent(message('sheet.ownEmpty')),
    );
    expect(deleteRequests(fake)).toEqual([`${BASE_URL}/sheets/${SHEET_ID}`]);
    expect(store.records()).toEqual([]);
  });

  it('Edge: a delete failure keeps the sheet and shows the mapped copy', async () => {
    const user = userEvent.setup();
    const fake = createFakeTransport(
      successOutcome(SHEET_JSON),
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
    );

    renderApp('/your-sheet', { client: clientWith(fake.transport) });
    await findName();

    await user.click(screen.getByRole('button', { name: message('sheet.ownDelete') }));
    await user.click(screen.getByRole('button', { name: message('sheet.ownDeleteConfirm') }));

    await waitFor(() =>
      expect(screen.getByTestId('your-sheet-delete-error')).toHaveTextContent(
        failureMessage('serverError'),
      ),
    );
    expect(screen.getByTestId('your-sheet-name')).toHaveTextContent('Aragorn');
  });
});
