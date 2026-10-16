import { describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { UserEvent } from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { createApiClient } from '../api/client';
import type { HttpRequest, Transport } from '../api/transport';
import { failureMessage } from '../api/error_mapper';
import { message } from '../l10n/strings';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import { renderApp } from './render';
import { SHEET_ID, TEMPLATE_ID, sheetPayload, templateDetailPayload } from './sheet_fixtures';

const BASE_URL = 'http://api.test';

const CATALOG = [{ id: TEMPLATE_ID, name: 'Fantasy', default: true, fields: {} }];

const ENTRY_NAME = /Fantasy/;

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

/** A query client with the production cache lifetime, so a seeded sheet survives the navigation. */
function appQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

function postRequests(requests: readonly HttpRequest[]) {
  return requests.filter((request) => request.method === 'POST' && request.url.endsWith('/sheets'));
}

/** Opens the catalog and its creation surface for the synthetic template. */
async function openSurface(user: UserEvent) {
  const fake = createFakeTransport(
    successOutcome(CATALOG),
    successOutcome(templateDetailPayload()),
  );
  renderApp('/', { client: clientWith(fake.transport), queryClient: appQueryClient() });
  await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
  await screen.findByTestId('detail-name');
  return fake;
}

describe('sheet creation flow (WSC-01 … WSC-04)', () => {
  it('Edge: a whitespace-only name keeps the primary action disabled and issues zero requests (WSC-01)', async () => {
    const user = userEvent.setup();
    const fake = await openSurface(user);

    await user.type(screen.getByLabelText(message('sheet.nameLabel')), '   ');

    expect(screen.getByRole('button', { name: message('sheet.create') })).toBeDisabled();
    expect(postRequests(fake.requests)).toHaveLength(0);
  });

  it('Edge: while the request is in flight the action is disabled, the surface shows progress and a repeat activation issues no second request (WSC-03)', async () => {
    const deferred = createDeferredTransport();
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(deferred.transport), queryClient: appQueryClient() });

    deferred.resolve(successOutcome(CATALOG));
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    deferred.resolve(successOutcome(templateDetailPayload()));
    await screen.findByTestId('detail-name');

    await user.type(screen.getByLabelText(message('sheet.nameLabel')), 'Aragorn');
    await user.click(screen.getByRole('button', { name: message('sheet.create') }));

    const busy = screen.getByRole('button', { name: message('sheet.creating') });
    expect(busy).toBeDisabled();
    expect(busy).toHaveAttribute('aria-busy', 'true');
    expect(postRequests(deferred.requests)).toHaveLength(1);

    fireEvent.click(busy);
    expect(postRequests(deferred.requests)).toHaveLength(1);

    deferred.resolve(successOutcome(sheetPayload(), 201));
    await waitFor(() =>
      expect(
        deferred.requests.filter((request) => request.url.endsWith(`/templates/${TEMPLATE_ID}`)),
      ).toHaveLength(2),
    );
    deferred.resolve(successOutcome(templateDetailPayload()));
    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
  });

  it('Edge: a failed creation keeps the surface open with the typed name intact and the mapped message inline (WSC-04)', async () => {
    const fake = createFakeTransport(
      successOutcome(CATALOG),
      successOutcome(templateDetailPayload()),
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
    );
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(fake.transport), queryClient: appQueryClient() });
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    await screen.findByTestId('detail-name');

    await user.type(screen.getByLabelText(message('sheet.nameLabel')), 'Aragorn');
    await user.click(screen.getByRole('button', { name: message('sheet.create') }));

    expect(await screen.findByText(failureMessage('serverError'))).toBeInTheDocument();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByLabelText(message('sheet.nameLabel'))).toHaveValue('Aragorn');
    expect(screen.queryByTestId('sheet-screen')).not.toBeInTheDocument();
  });

  it('Happy path: a non-empty trimmed name issues one POST carrying the template and name, then opens the created sheet without a sheet GET (WSC-01, WSC-02)', async () => {
    const fake = createFakeTransport(
      successOutcome(CATALOG),
      successOutcome(templateDetailPayload()),
      successOutcome(sheetPayload(), 201),
      successOutcome(templateDetailPayload()),
    );
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(fake.transport), queryClient: appQueryClient() });
    await user.click(await screen.findByRole('button', { name: ENTRY_NAME }));
    await screen.findByTestId('detail-name');

    await user.type(screen.getByLabelText(message('sheet.nameLabel')), '  Aragorn  ');
    await user.click(screen.getByRole('button', { name: message('sheet.create') }));

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');

    const posts = postRequests(fake.requests);
    expect(posts).toHaveLength(1);
    expect(posts[0].url).toBe(`${BASE_URL}/sheets`);
    expect(JSON.parse(posts[0].body ?? '')).toEqual({
      template_id: TEMPLATE_ID,
      sheet_name: 'Aragorn',
    });
    expect(
      fake.requests.filter(
        (request) => request.method === 'GET' && request.url.endsWith(`/sheets/${SHEET_ID}`),
      ),
    ).toHaveLength(0);
  });
});
