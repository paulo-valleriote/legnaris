import { render } from '@testing-library/react';
import { QueryClient } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import type { AuthApi } from '../api/auth_api';
import type { ApiClient } from '../api/client';
import { createApiClient } from '../api/client';
import { AppRoutes } from '../app/router';
import { AppProviders } from '../app/providers';
import type { AuthSession } from '../auth/auth_session';
import { createAuthSession } from '../auth/auth_session';
import type { MemorySheetStore, SheetStore } from '../library/sheet_store';
import { createMemorySheetStore } from '../library/sheet_store';
import type { ViewStorage } from '../sheet/view_store';
import { createFakeTransport, successOutcome } from './fake_transport';
import type { MemoryViewStorage } from './view_storage';
import { createMemoryViewStorage } from './view_storage';

const TEST_BASE_URL = 'http://api.test';

export type SessionOption = 'authenticated' | 'unauthenticated';

export interface RenderAppOptions {
  readonly client?: ApiClient;
  readonly queryClient?: QueryClient;
  readonly sheetStore?: SheetStore;
  readonly viewStorage?: ViewStorage;
  readonly session?: SessionOption | AuthSession;
  readonly authApi?: AuthApi;
  readonly routeState?: unknown;
}

function defaultClient(): ApiClient {
  const fake = createFakeTransport(successOutcome([]));
  return createApiClient({ baseUrl: TEST_BASE_URL, transport: fake.transport });
}

function unusedAuthApi(): AuthApi {
  const unused = () => Promise.reject(new Error('auth operation is not scripted for this suite'));
  return {
    register: unused,
    login: unused,
    refresh: unused,
    logout: () => Promise.resolve(),
    verifyChallenge: unused,
    resendChallenge: unused,
  };
}

function seedSession(posture: SessionOption, authApi: AuthApi): AuthSession {
  return createAuthSession({
    authApi,
    initialState: posture,
    initialAccessToken: posture === 'authenticated' ? 'test-access-token' : null,
  });
}

function initialEntry(path: string, state: unknown) {
  const queryAt = path.indexOf('?');
  return queryAt === -1
    ? { pathname: path, state }
    : { pathname: path.slice(0, queryAt), search: path.slice(queryAt), state };
}

export function renderApp(path = '/', options: RenderAppOptions = {}) {
  const queryClient =
    options.queryClient ??
    new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const sheetStore = options.sheetStore ?? createMemorySheetStore();
  const viewStorage = options.viewStorage ?? createMemoryViewStorage();
  const session =
    typeof options.session === 'object'
      ? options.session
      : seedSession(options.session ?? 'authenticated', options.authApi ?? unusedAuthApi());

  const result = render(
    <MemoryRouter initialEntries={[initialEntry(path, options.routeState)]}>
      <AppProviders
        client={options.client ?? defaultClient()}
        queryClient={queryClient}
        sheetStore={sheetStore}
        viewStorage={viewStorage}
        session={session}
      >
        <AppRoutes />
      </AppProviders>
    </MemoryRouter>,
  );

  return {
    ...result,
    queryClient,
    sheetStore: sheetStore as MemorySheetStore,
    viewStorage: viewStorage as MemoryViewStorage,
    session,
  };
}
