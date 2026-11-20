import { createContext, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { AdsConfig } from '../ads/ads_config';
import { AdsProvider } from '../ads/ads_provider';
import type { AuthApi } from '../api/auth_api';
import { createAuthApi } from '../api/auth_api';
import { createAuthenticatedTransport } from '../api/authenticated_transport';
import type { ApiClient } from '../api/client';
import { createApiClient } from '../api/client';
import { resolveApiConfigFromBuild } from '../api/config';
import { readCsrfToken } from '../api/csrf';
import type { Transport } from '../api/transport';
import { browserFetch, createFetchTransport } from '../api/transport';
import type { AuthSession } from '../auth/auth_session';
import { createAuthSession } from '../auth/auth_session';
import { createIdbSheetStore } from '../library/idb_sheet_store';
import { clearSessionData } from '../library/session_data';
import type { SheetStore } from '../library/sheet_store';
import { createLazySheetStore } from '../library/sheet_store';
import type { ViewStorage } from '../sheet/view_store';
import { browserViewStorage } from '../sheet/view_store';

const ApiClientContext = createContext<ApiClient | undefined>(undefined);
const SheetStoreContext = createContext<SheetStore | undefined>(undefined);
const AuthSessionContext = createContext<AuthSession | undefined>(undefined);
const ViewStorageContext = createContext<ViewStorage | undefined>(undefined);

export interface AppProvidersProps {
  readonly children: ReactNode;
  readonly apiBaseUrl?: string;
  readonly client?: ApiClient;
  readonly transport?: Transport;
  readonly authApi?: AuthApi;
  readonly session?: AuthSession;
  readonly queryClient?: QueryClient;
  readonly sheetStore?: SheetStore;
  readonly viewStorage?: ViewStorage;
  readonly adsConfig?: AdsConfig | null;
  readonly adsDocument?: Document;
  readonly adsWindow?: Window;
}

function createDefaultQueryClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

export function useApiClient(): ApiClient {
  const client = useContext(ApiClientContext);
  if (client === undefined) {
    throw new Error('useApiClient must be used within <AppProviders>.');
  }
  return client;
}

export function useSheetStore(): SheetStore {
  const store = useContext(SheetStoreContext);
  if (store === undefined) {
    throw new Error('useSheetStore must be used within <AppProviders>.');
  }
  return store;
}

export function useViewStorage(): ViewStorage | undefined {
  return useContext(ViewStorageContext);
}

export function useAuthSession(): AuthSession {
  const session = useContext(AuthSessionContext);
  if (session === undefined) {
    throw new Error('useAuthSession must be used within <AppProviders>.');
  }
  return session;
}

export function AppProviders({
  children,
  apiBaseUrl,
  client,
  transport,
  authApi,
  session,
  queryClient,
  sheetStore,
  viewStorage,
  adsConfig,
  adsDocument,
  adsWindow,
}: AppProvidersProps) {
  const [baseUrl] = useState(() => apiBaseUrl ?? resolveApiConfigFromBuild().baseUrl);
  const [fallbackTransport] = useState(() => createFetchTransport(browserFetch));
  const [fallbackQueryClient] = useState(createDefaultQueryClient);
  const [fallbackSheetStore] = useState(() => createLazySheetStore(() => createIdbSheetStore()));
  const [fallbackViewStorage] = useState(browserViewStorage);
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
          <ViewStorageContext.Provider value={viewStorage ?? fallbackViewStorage}>
            <QueryClientProvider client={resolvedQueryClient}>
              <AdsProvider
                config={adsConfig ?? null}
                {...(adsDocument === undefined ? {} : { document: adsDocument })}
                {...(adsWindow === undefined ? {} : { window: adsWindow })}
              >
                {children}
              </AdsProvider>
            </QueryClientProvider>
          </ViewStorageContext.Provider>
        </SheetStoreContext.Provider>
      </AuthSessionContext.Provider>
    </ApiClientContext.Provider>
  );
}
