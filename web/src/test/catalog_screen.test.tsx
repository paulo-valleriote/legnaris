import { describe, expect, it } from 'vitest';
import { screen, waitFor, within } from '@testing-library/react';
import { createApiClient } from '../api/client';
import type { Transport } from '../api/transport';
import type { TemplateSummary } from '../api/models/template';
import { failureMessage, parseFailureMessage } from '../api/error_mapper';
import { message } from '../l10n/strings';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
  withPlanUsage,
} from './fake_transport';
import { renderApp } from './render';
import { GAME_SYSTEM_CATALOG } from './template_game_system_fixture';

const BASE_URL = 'http://api.test';

const TWO_TEMPLATES: readonly TemplateSummary[] = [
  {
    id: 'dnd5e',
    name: 'Dungeons & Dragons 5e',
    default: true,
    kinds: ['character', 'npc', 'enemy'],
    fields: {},
  },
  {
    id: 'pbta',
    name: 'Powered by the Apocalypse',
    default: false,
    kinds: ['character', 'npc', 'enemy'],
    fields: {},
  },
];

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport: withPlanUsage(transport) });
}

describe('catalog screen', () => {
  it('Happy path: while the request is in flight it shows the loading copy and not the empty copy', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    expect(screen.getByText(message('catalog.loading'))).toBeInTheDocument();
    expect(screen.queryByText(message('catalog.empty'))).not.toBeInTheDocument();
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);

    deferred.resolve(successOutcome(TWO_TEMPLATES));
  });

  it('Happy path: a two-entry response renders two rows in the response order with their names', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome(TWO_TEMPLATES));

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
    const rows = screen.getAllByRole('listitem');
    expect(rows[0]).toHaveTextContent('Dungeons & Dragons 5e');
    expect(rows[1]).toHaveTextContent('Powered by the Apocalypse');
    expect(within(rows[0]).getByRole('button')).toHaveAccessibleName('Dungeons & Dragons 5e');
    expect(screen.queryByText(message('catalog.loading'))).not.toBeInTheDocument();
  });

  it('Happy path: the row arrow is the shared trailing icon, hidden from assistive technology', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome(TWO_TEMPLATES));

    const rows = await screen.findAllByRole('listitem');
    const trailing = rows[0].querySelector('.lg-button__icon--trailing');
    expect(trailing).not.toBeNull();
    expect(trailing).toHaveAttribute('aria-hidden', 'true');
    expect(trailing!.querySelector('svg')).not.toBeNull();
  });

  it('Happy path: an empty response renders exactly one line and no decorative row', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome([]));

    await waitFor(() => expect(screen.getByText(message('catalog.empty'))).toBeInTheDocument());
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
    expect(screen.getByRole('button', { name: message('common.retry') })).toBeInTheDocument();
    expect(screen.queryByText(message('catalog.loading'))).not.toBeInTheDocument();
  });

  it('Edge: a failed response renders the mapped copy plus a Retry action', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(errorOutcome('INTERNAL_ERROR', 'boom', 500));

    await waitFor(() =>
      expect(screen.getByText(failureMessage('serverError'))).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: message('common.retry') })).toBeInTheDocument();
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
  });

  it('Edge: a malformed payload renders the parse-failure copy with Retry, no blank screen and no partial list', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve({ status: 200, body: JSON.stringify({ data: { not: 'a list' } }) });

    await waitFor(() => expect(screen.getByText(parseFailureMessage())).toBeInTheDocument());
    expect(screen.getByRole('button', { name: message('common.retry') })).toBeInTheDocument();
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
    expect(screen.getByTestId('catalog')).toBeInTheDocument();
  });

  it('Edge: an entry whose name is empty renders without throwing and keeps the layout intact', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(
      successOutcome([
        { id: 'blank', name: '', default: false, kinds: ['character', 'npc', 'enemy'], fields: {} },
      ]),
    );

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(1));
    const main = screen.getByRole('main');
    expect(within(main).getByRole('button', { name: 'blank' })).toBeInTheDocument();
  });

  it('Happy path: an entry with a game system shows its compatibility label and no attribution link', async () => {
    const fake = createFakeTransport(successOutcome(GAME_SYSTEM_CATALOG));
    renderApp('/', { client: clientWith(fake.transport) });

    const row = await screen.findByRole('listitem');

    expect(within(row).getByTestId('catalog-compatibility')).toHaveTextContent('5E compatible');
    expect(within(row).queryByRole('link')).not.toBeInTheDocument();
    expect(row).not.toHaveTextContent('SRD 5.2.1');
  });

  it('Edge: an entry without a game system draws no compatibility label', async () => {
    const deferred = createDeferredTransport();
    renderApp('/', { client: clientWith(deferred.transport) });

    deferred.resolve(successOutcome(TWO_TEMPLATES));

    const rows = await screen.findAllByRole('listitem');
    for (const row of rows) {
      expect(within(row).queryByTestId('catalog-compatibility')).not.toBeInTheDocument();
    }
    expect(rows[0]).toHaveTextContent('Dungeons & Dragons 5e');
  });
});
