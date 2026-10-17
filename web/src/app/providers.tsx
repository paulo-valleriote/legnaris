/**
 * The application's shared context (WCT-02, WCT-12, R11): one API client, one
 * session, one auth client and one query client, all provided to the tree.
 *
 * The construction graph is ordered so the credential is attached at exactly one
 * seam: the raw `Transport` feeds the `AuthApi`; the `AuthSession` is built over
 * that and the sheet store; the authenticated `Transport` decorates the raw one
 * with the session; and the domain `ApiClient` is built over the decorator. The
 * `document.cookie` reader and the real `fetch` are injected at the app entry, so
 * every suite can drive a fake transport, a memory store and injected session.
 */

import { createContext, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { AuthApi } from '../api/auth_api';
import { createAuthApi } from '../api/auth_api';
import { createAuthenticatedTransport } from '../api/authenticated_transport';
import type { ApiClient } from '../api/client';
import { createApiClient } from '../api/client';
import { resolveApiConfigFromBuild } from '../api/config';
import { readCsrfToken } from '../api/csrf';
import type { Transport } from '../api/transport';
import { createFetchTransport } from '../api/transport';
import type { AuthSession } from '../auth/auth_session';
import { createAuthSession } from '../auth/auth_session';
import { createIdbSheetStore } from '../library/idb_sheet_store';
import { clearSessionData } from '../library/session_data';
import type { SheetStore } from '../library/sheet_store';
import { createLazySheetStore } from '../library/sheet_store';

const ApiClientContext = createContext<ApiClient | undefined>(undefined);
const SheetStoreContext = createContext<SheetStore | undefined>(undefined);
const AuthSessionContext = createContext<AuthSession | undefined>(undefined);

export interface AppProvidersProps {
  readonly children: ReactNode;
  /** Injected client; tests build one over a fake transport. */
  readonly client?: ApiClient;
  /** Injected transport; the client is built over it when no client is given. */
  readonly transport?: Transport;
  /** Injected auth client; the session is built over it when no session is given. */
  readonly authApi?: AuthApi;
  /** Injected session; tests pass one in a known state. */
  readonly session?: AuthSession;
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

/** Resolves the session from context (R11). */
export function useAuthSession(): AuthSession {
  const session = useContext(AuthSessionContext);
  if (session === undefined) {
    throw new Error('useAuthSession must be used within <AppProviders>.');
  }
  return session;
}

/**
 * Provides the API client, the session, the local sheet store and the query
 * client.
 *
 * `retry` is disabled on the query client so the retry policy stays the F2
 * client's alone; the library's own default would otherwise repeat every failed
 * read three more times.
 */
export function AppProviders({
  children,
  client,
  transport,
  authApi,
  session,
  queryClient,
  sheetStore,
}: AppProvidersProps) {
  const [baseUrl] = useState(() => resolveApiConfigFromBuild().baseUrl);
  const [fallbackTransport] = useState(() => createFetchTransport(fetch));
  const [fallbackQueryClient] = useState(createDefaultQueryClient);
  const [fallbackSheetStore] = useState(() => createLazySheetStore(() => createIdbSheetStore()));
  const resolvedQueryClient = queryClient ?? fallbackQueryClient;
  const resolvedSheetStore = sheetStore ?? fallbackSheetStore;
  const rawTransport = transport ?? fallbackTransport;

  const resolvedAuthApi = useMemo(
    () =>
      authApi ??
      createAuthApi({
        baseUrl,
        transport: rawTransport,
        readCsrfToken: () => readCsrfToken(document.cookie),
      }),
    [authApi, baseUrl, rawTransport],
  );

  const resolvedSession = useMemo(
    () =>
      session ??
      createAuthSession({
        authApi: resolvedAuthApi,
        clearSessionData: () => clearSessionData(resolvedSheetStore),
      }),
    [session, resolvedAuthApi, resolvedSheetStore],
  );

  const authenticatedTransport = useMemo(
    () => createAuthenticatedTransport({ inner: rawTransport, session: resolvedSession }),
    [rawTransport, resolvedSession],
  );

  const resolvedClient = useMemo(
    () =>
      client ??
      createApiClient({
        baseUrl,
        transport: authenticatedTransport,
      }),
    [client, baseUrl, authenticatedTransport],
  );

  return (
    <ApiClientContext.Provider value={resolvedClient}>
      <AuthSessionContext.Provider value={resolvedSession}>
        <SheetStoreContext.Provider value={resolvedSheetStore}>
          <QueryClientProvider client={resolvedQueryClient}>{children}</QueryClientProvider>
        </SheetStoreContext.Provider>
      </AuthSessionContext.Provider>
    </ApiClientContext.Provider>
  );
}
