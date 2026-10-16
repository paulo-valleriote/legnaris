import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { act, renderHook, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient } from '@tanstack/react-query';
import { AppProviders } from '../app/providers';
import { createApiClient } from '../api/client';
import type { Transport } from '../api/transport';
import type { TemplateSummary } from '../api/models/template';
import { message } from '../l10n/strings';
import { useCatalog } from '../catalog/queries';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';

const TWO_TEMPLATES: readonly TemplateSummary[] = [
  { id: 'dnd5e', name: 'Dungeons & Dragons 5e', default: true, fields: {} },
  { id: 'pbta', name: 'Powered by the Apocalypse', default: false, fields: {} },
];

function clientWith(transport: Transport) {
  return createApiClient({ baseUrl: BASE_URL, transport });
}

function freshQueryClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
}

describe('catalog query policy (WCT-02, WCT-07, WCT-08)', () => {
  it('Happy path: the first mount issues exactly one request (WCT-02)', async () => {
    const fake = createFakeTransport(successOutcome(TWO_TEMPLATES));
    renderApp('/', { client: clientWith(fake.transport) });

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
    expect(fake.attempts).toBe(1);
  });

  it('Edge: remounting the route and firing focus, visibility and reconnect issue no further request (WCT-02)', async () => {
    const fake = createFakeTransport(successOutcome(TWO_TEMPLATES));
    const client = clientWith(fake.transport);
    const queryClient = freshQueryClient();

    const first = renderApp('/', { client, queryClient });
    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
    expect(fake.attempts).toBe(1);

    first.unmount();
    renderApp('/', { client, queryClient });

    act(() => {
      window.dispatchEvent(new Event('focus'));
      window.dispatchEvent(new Event('visibilitychange'));
      window.dispatchEvent(new Event('online'));
    });

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
    expect(fake.attempts).toBe(1);
  });

  it('Happy path: Retry from the error state issues exactly one further request (WCT-07)', async () => {
    const fake = createFakeTransport(
      errorOutcome('INTERNAL_ERROR', 'boom', 500),
      successOutcome(TWO_TEMPLATES),
    );
    const user = userEvent.setup();
    renderApp('/', { client: clientWith(fake.transport) });

    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('catalog.retry') })).toBeInTheDocument(),
    );
    expect(fake.attempts).toBe(1);

    await user.click(screen.getByRole('button', { name: message('catalog.retry') }));

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(2));
    expect(fake.attempts).toBe(2);
  });

  it('Edge: a second trigger while a request is in flight is joined, not duplicated (WCT-08)', async () => {
    const deferred = createDeferredTransport();
    const client = clientWith(deferred.transport);
    const queryClient = freshQueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <AppProviders client={client} queryClient={queryClient}>
        {children}
      </AppProviders>
    );

    const { result } = renderHook(() => useCatalog(), { wrapper });
    await waitFor(() => expect(result.current.isPending).toBe(true));
    expect(deferred.attempts).toBe(1);

    await act(async () => {
      void result.current.refetch();
      void result.current.refetch();
    });

    expect(deferred.attempts).toBe(1);

    deferred.resolve(successOutcome(TWO_TEMPLATES));
    await act(async () => {
      await Promise.resolve();
    });
  });
});
