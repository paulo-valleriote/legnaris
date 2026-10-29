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
import { createFakeTransport, successOutcome } from './fake_transport';

const TEST_BASE_URL = 'http://api.test';

/** The two session postures a suite selects by name; an object injects a scripted one (WAS-08). */
export type SessionOption = 'authenticated' | 'unauthenticated';

export interface RenderAppOptions {
  /** The client the screens read; tests inject one built over a fake transport. */
  readonly client?: ApiClient;
  /** The query client to mount with; tests reuse one to prove cache behaviour. */
  readonly queryClient?: QueryClient;
  /** The local store to mount with; a fresh in-memory double is used by default (WSC-11). */
  readonly sheetStore?: SheetStore;
  /** The session the gate reads; an authenticated session by default (WAS-03, WAS-08). */
  readonly session?: SessionOption | AuthSession;
  /** An auth client the default session is built over, so a suite can script the restore. */
  readonly authApi?: AuthApi;
}

/** A client whose catalog resolves with no entries, so a plain render stays meaningful. */
function defaultClient(): ApiClient {
  const fake = createFakeTransport(successOutcome([]));
  return createApiClient({ baseUrl: TEST_BASE_URL, transport: fake.transport });
}

/** An auth client no suite expects to call, so an unscripted operation fails loudly. */
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

/** Seeds the app's session from the two named postures, so the gate renders deterministically. */
function seedSession(posture: SessionOption, authApi: AuthApi): AuthSession {
  return createAuthSession({
    authApi,
    initialState: posture,
    initialAccessToken: posture === 'authenticated' ? 'test-access-token' : null,
  });
}

/**
 * Renders the app at a given path inside a memory router (FND-08), with the
 * shared providers mounted over an injected client, a fresh query client and a
 * fresh in-memory `SheetStore` per call, so a case's cache never leaks into the
 * next (WCT-02, WSC-11). The gate reads an authenticated session unless a suite
 * asks for the unauthenticated posture (WAS-03, WAS-08).
 */
export function renderApp(path = '/', options: RenderAppOptions = {}) {
  const queryClient =
    options.queryClient ??
    new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const sheetStore = options.sheetStore ?? createMemorySheetStore();
  const session =
    typeof options.session === 'object'
      ? options.session
      : seedSession(options.session ?? 'authenticated', options.authApi ?? unusedAuthApi());

  const result = render(
    <MemoryRouter initialEntries={[path]}>
      <AppProviders
        client={options.client ?? defaultClient()}
        queryClient={queryClient}
        sheetStore={sheetStore}
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
    session,
  };
}
