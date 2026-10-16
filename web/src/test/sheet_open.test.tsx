import { describe, expect, it, vi } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { createApiClient } from '../api/client';
import type { Transport } from '../api/transport';
import { failureMessage, parseFailureMessage } from '../api/error_mapper';
import { message } from '../l10n/strings';
import { SCHEMA_VERSION, createMemorySheetStore } from '../library/sheet_store';
import { seedCreatedSheet, sheetKey } from '../catalog/queries';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import { renderApp } from './render';
import {
  SHEET_ID,
  SHEET_MODEL,
  TEMPLATE_ID,
  emptyTemplateDetailPayload,
  sheetPayload,
  templateDetailPayload,
} from './sheet_fixtures';

const BASE_URL = 'http://api.test';

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function sheetOutcome() {
  return successOutcome(sheetPayload());
}

function templateOutcome() {
  return successOutcome(templateDetailPayload());
}

function countRequests(
  requests: readonly { method: string; url: string }[],
  method: string,
  suffix: string,
): number {
  return requests.filter((request) => request.method === method && request.url.endsWith(suffix))
    .length;
}

const SHEET_URL_SUFFIX = `/sheets/${SHEET_ID}`;

describe('sheet open situations (WSC-05 … WSC-11)', () => {
  it('Happy path: opening a sheet issues one GET for the sheet and one for its template, then renders the name and field tree (WSC-05, WSC-08, WSC-09)', async () => {
    const fake = createFakeTransport(sheetOutcome(), templateOutcome());
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport) });

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
    await waitFor(() => expect(screen.getByRole('status', { name: 'Total' })).toBeInTheDocument());

    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveValue('Aragorn');
    expect(screen.getByRole('textbox', { name: 'Level' })).toHaveValue('3');
    expect(screen.getByRole('combobox', { name: 'Ability' })).toBeInTheDocument();
    expect(countRequests(fake.requests, 'GET', SHEET_URL_SUFFIX)).toBe(1);
    expect(countRequests(fake.requests, 'GET', `/templates/${TEMPLATE_ID}`)).toBe(1);
  });

  it('Edge: a slow response shows the open-loading copy until it settles, issuing exactly one sheet GET (WSC-05)', async () => {
    const deferred = createDeferredTransport();
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(deferred.transport) });

    expect(screen.getByText(message('sheet.openLoading'))).toBeInTheDocument();
    expect(deferred.attempts).toBe(1);

    deferred.resolve(sheetOutcome());
    expect(await screen.findByText(message('sheet.templateLoading'))).toBeInTheDocument();

    deferred.resolve(templateOutcome());
    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
    expect(countRequests(deferred.requests, 'GET', SHEET_URL_SUFFIX)).toBe(1);
  });

  it('Edge: a missing sheet shows the missing copy with no Retry and evicts the sheet and its draft (WSC-06)', async () => {
    const fake = createFakeTransport(errorOutcome('NOT_FOUND', 'gone', 404));
    const store = createMemorySheetStore();
    const deleteSheet = vi.spyOn(store, 'deleteSheet');
    const deleteDraft = vi.spyOn(store, 'deleteDraft');
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport), sheetStore: store });

    expect(await screen.findByTestId('sheet-missing')).toHaveTextContent(message('sheet.missing'));
    await waitFor(() => expect(deleteSheet).toHaveBeenCalledWith(SHEET_ID));
    expect(deleteDraft).toHaveBeenCalledWith(SHEET_ID);
    expect(
      screen.queryByRole('button', { name: message('sheet.retryOpen') }),
    ).not.toBeInTheDocument();
  });

  it('Edge: a failing local eviction still shows the missing copy (WSC-06)', async () => {
    const fake = createFakeTransport(errorOutcome('NOT_FOUND', 'gone', 404));
    const store = createMemorySheetStore();
    store.deleteSheet = () => Promise.reject(new Error('eviction failed'));
    store.deleteDraft = () => Promise.reject(new Error('eviction failed'));
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport), sheetStore: store });

    expect(await screen.findByTestId('sheet-missing')).toHaveTextContent(message('sheet.missing'));
  });

  it('Edge: any other failure shows the mapped copy with a Retry that re-requests exactly once (WSC-07)', async () => {
    const fake = createFakeTransport(
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
      sheetOutcome(),
      templateOutcome(),
    );
    const user = userEvent.setup();
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport) });

    const retry = await screen.findByRole('button', { name: message('sheet.retryOpen') });
    expect(screen.getByText(failureMessage('serverError'))).toBeInTheDocument();
    expect(countRequests(fake.requests, 'GET', SHEET_URL_SUFFIX)).toBe(1);

    await user.click(retry);

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
    expect(countRequests(fake.requests, 'GET', SHEET_URL_SUFFIX)).toBe(2);
  });

  it('Edge: a malformed sheet payload shows the parse-failure copy with a Retry, never a blank screen (WSC-07)', async () => {
    const fake = createFakeTransport(
      successOutcome({ id: SHEET_ID }),
      sheetOutcome(),
      templateOutcome(),
    );
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport) });

    expect(await screen.findByText(parseFailureMessage())).toBeInTheDocument();
    expect(screen.getByRole('button', { name: message('sheet.retryOpen') })).toBeInTheDocument();
    expect(screen.queryByTestId('sheet-name')).not.toBeInTheDocument();
  });

  it('Edge: a template failure shows the mapped copy with a Retry that re-requests exactly once (WSC-09)', async () => {
    const fake = createFakeTransport(
      sheetOutcome(),
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
      templateOutcome(),
    );
    const user = userEvent.setup();
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport) });

    const retry = await screen.findByRole('button', { name: message('sheet.retryOpen') });
    expect(screen.getByText(failureMessage('serverError'))).toBeInTheDocument();
    expect(countRequests(fake.requests, 'GET', `/templates/${TEMPLATE_ID}`)).toBe(1);

    await user.click(retry);

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
    expect(countRequests(fake.requests, 'GET', `/templates/${TEMPLATE_ID}`)).toBe(2);
  });

  it('Edge: a template with no fields shows the empty-fields line (WSC-08)', async () => {
    const fake = createFakeTransport(sheetOutcome(), successOutcome(emptyTemplateDetailPayload()));
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport) });

    expect(await screen.findByTestId('sheet-empty-fields')).toHaveTextContent(
      message('sheet.emptyFields'),
    );
  });

  it('Happy path: a deep link resolves the same screen a navigation produces (WSC-10)', async () => {
    const fake = createFakeTransport(sheetOutcome(), templateOutcome());
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport) });

    expect(await screen.findByTestId('sheet-screen')).toBeInTheDocument();
    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
  });

  it('Happy path: after a successful open the store holds the sheet under schemaVersion 1 and no credential (WSC-11, WSC-31)', async () => {
    const fake = createFakeTransport(sheetOutcome(), templateOutcome());
    const { sheetStore } = renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport) });

    await screen.findByTestId('sheet-name');

    await waitFor(() =>
      expect(sheetStore.records()).toEqual([
        { schemaVersion: SCHEMA_VERSION, id: SHEET_ID, sheet: SHEET_MODEL },
      ]),
    );
    expect(JSON.stringify(sheetStore.records())).not.toMatch(
      /token|password|secret|authorization|cookie|credential/i,
    );
  });

  it('Happy path: a created sheet seeded into the cache renders without a second sheet GET (WSC-02)', async () => {
    const fake = createFakeTransport(templateOutcome());
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    seedCreatedSheet(queryClient, SHEET_MODEL);
    renderApp(SHEET_URL_SUFFIX, { client: clientWith(fake.transport), queryClient });

    expect(await screen.findByTestId('sheet-name')).toHaveTextContent('Aragorn');
    expect(countRequests(fake.requests, 'GET', SHEET_URL_SUFFIX)).toBe(0);
    expect(queryClient.getQueryData(sheetKey(SHEET_ID))).toEqual(SHEET_MODEL);
  });
});
