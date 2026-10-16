import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import { createApiClient } from '../api/client';
import type { Transport } from '../api/transport';
import type { TemplateSummary } from '../api/models/template';
import { failureMessage, parseFailureMessage } from '../api/error_mapper';
import { message } from '../l10n/strings';
import { createDeferredTransport, errorOutcome, successOutcome } from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';

const TWO_TEMPLATES: readonly TemplateSummary[] = [
  { id: 'dnd5e', name: 'Dungeons & Dragons 5e', default: true, fields: {} },
  { id: 'pbta', name: 'Powered by the Apocalypse', default: false, fields: {} },
];

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

describe('catalog screen (WCT-03 … WCT-10)', () => {
  it('Happy path: while the request is in flight it shows the loading copy and not the empty copy (WCT-03)', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    expect(screen.getByText(message('catalog.loading'))).toBeInTheDocument();
    expect(screen.queryByText(message('catalog.empty'))).not.toBeInTheDocument();
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);

    deferred.resolve(successOutcome(TWO_TEMPLATES));
  });

  it('Happy path: a two-entry response renders two rows in the response order with their names (WCT-04)', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome(TWO_TEMPLATES));

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
    const rows = screen.getAllByRole('listitem');
    expect(rows[0]).toHaveTextContent('Dungeons & Dragons 5e');
    expect(rows[1]).toHaveTextContent('Powered by the Apocalypse');
    expect(screen.queryByText(message('catalog.loading'))).not.toBeInTheDocument();
  });

  it('Happy path: an empty response renders exactly one line and no decorative row (WCT-05)', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome([]));

    await waitFor(() => expect(screen.getByText(message('catalog.empty'))).toBeInTheDocument());
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
    expect(screen.queryByText(message('catalog.loading'))).not.toBeInTheDocument();
  });

  it('Edge: a failed response renders the mapped copy plus a Retry action (WCT-06)', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(errorOutcome('INTERNAL_ERROR', 'boom', 500));

    await waitFor(() =>
      expect(screen.getByText(failureMessage('serverError'))).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: message('catalog.retry') })).toBeInTheDocument();
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
  });

  it('Edge: a malformed payload renders the parse-failure copy with Retry, no blank screen and no partial list (WCT-09)', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve({ status: 200, body: JSON.stringify({ data: { not: 'a list' } }) });

    await waitFor(() => expect(screen.getByText(parseFailureMessage())).toBeInTheDocument());
    expect(screen.getByRole('button', { name: message('catalog.retry') })).toBeInTheDocument();
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
    expect(screen.getByTestId('catalog')).toBeInTheDocument();
  });

  it('Edge: an entry whose name is empty renders without throwing and keeps the layout intact (WCT-10)', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome([{ id: 'blank', name: '', default: false, fields: {} }]));

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(1));
    const main = screen.getByRole('main');
    expect(within(main).getByRole('button', { name: 'blank' })).toBeInTheDocument();
  });
});
