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
import { createIdbSheetStore } from '../library/idb_sheet_store';
import type { SheetStore } from '../library/sheet_store';
import { createLazySheetStore } from '../library/sheet_store';

const ApiClientContext = createContext<ApiClient | undefined>(undefined);
const SheetStoreContext = createContext<SheetStore | undefined>(undefined);

export interface AppProvidersProps {
  readonly children: ReactNode;
  /** Injected client; tests build one over a fake transport. */
  readonly client?: ApiClient;
  /** Injected transport; the client is built over it when no client is given. */
  readonly transport?: Transport;
  /** Injected query client; tests pass a fresh one per case. */
  readonly queryClient?: QueryClient;
  /** Injected local store; tests pass an in-memory double (WSC-11). */
  readonly sheetStore?: SheetStore;
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

/** Resolves the local sheet store from context (WSC-11). */
export function useSheetStore(): SheetStore {
  const store = useContext(SheetStoreContext);
  if (store === undefined) {
    throw new Error('useSheetStore must be used within <AppProviders>.');
  }
  return store;
}

/**
 * Provides the API client, the local sheet store and the query client.
 *
 * `retry` is disabled on the query client so the retry policy stays the F2
 * client's alone; the library's own default would otherwise repeat every failed
 * read three more times.
 */
export function AppProviders({
  children,
  client,
  transport,
  queryClient,
  sheetStore,
}: AppProvidersProps) {
  const [fallbackQueryClient] = useState(createDefaultQueryClient);
  const [fallbackSheetStore] = useState(() => createLazySheetStore(() => createIdbSheetStore()));
  const resolvedQueryClient = queryClient ?? fallbackQueryClient;
  const resolvedSheetStore = sheetStore ?? fallbackSheetStore;

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
      <SheetStoreContext.Provider value={resolvedSheetStore}>
        <QueryClientProvider client={resolvedQueryClient}>{children}</QueryClientProvider>
      </SheetStoreContext.Provider>
    </ApiClientContext.Provider>
  );
}
