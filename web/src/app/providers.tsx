/**
 * The application's shared context (WCT-02, WCT-12): one API client and one
 * query client, both provided to the tree.
 *
 * The client is built over the F2 fetch transport and the resolved base address
 * unless one is injected; keeping it in React context (never a module-level
 * singleton) is what lets the screens run against a fake transport in tests and
 * read the real client in the app.
 */

import { createContext, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ApiClient } from '../api/client';
import { createApiClient } from '../api/client';
import { resolveApiConfigFromBuild } from '../api/config';
import type { Transport } from '../api/transport';
import { createFetchTransport } from '../api/transport';

const ApiClientContext = createContext<ApiClient | undefined>(undefined);

export interface AppProvidersProps {
  readonly children: ReactNode;
  /** Injected client; tests build one over a fake transport. */
  readonly client?: ApiClient;
  /** Injected transport; the client is built over it when no client is given. */
  readonly transport?: Transport;
  /** Injected query client; tests pass a fresh one per case. */
  readonly queryClient?: QueryClient;
}

function createDefaultQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

/** Resolves the API client from context (WCT-02). */
export function useApiClient(): ApiClient {
  const client = useContext(ApiClientContext);
  if (client === undefined) {
    throw new Error('useApiClient must be used within <AppProviders>.');
  }
  return client;
}

/**
 * Provides the API client and the query client to the subtree.
 *
 * `retry` is disabled on the query client so the retry policy stays the F2
 * client's alone; the library's own default would otherwise repeat every failed
 * read three more times.
 */
export function AppProviders({ children, client, transport, queryClient }: AppProvidersProps) {
  const [fallbackQueryClient] = useState(createDefaultQueryClient);
  const resolvedQueryClient = queryClient ?? fallbackQueryClient;

  const resolvedClient = useMemo(
    () =>
      client ??
      createApiClient({
        baseUrl: resolveApiConfigFromBuild().baseUrl,
        transport: transport ?? createFetchTransport(fetch),
      }),
    [client, transport],
  );

  return (
    <ApiClientContext.Provider value={resolvedClient}>
      <QueryClientProvider client={resolvedQueryClient}>{children}</QueryClientProvider>
    </ApiClientContext.Provider>
  );
}
