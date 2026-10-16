import { render } from '@testing-library/react';
import { QueryClient } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import type { ApiClient } from '../api/client';
import { createApiClient } from '../api/client';
import { AppRoutes } from '../app/router';
import { AppProviders } from '../app/providers';
import { createFakeTransport, successOutcome } from './fake_transport';

const TEST_BASE_URL = 'http://api.test';

export interface RenderAppOptions {
  /** The client the screens read; tests inject one built over a fake transport. */
  readonly client?: ApiClient;
  /** The query client to mount with; tests reuse one to prove cache behaviour. */
  readonly queryClient?: QueryClient;
}

/** A client whose catalog resolves with no entries, so a plain render stays meaningful. */
function defaultClient(): ApiClient {
  const fake = createFakeTransport(successOutcome([]));
  return createApiClient({ baseUrl: TEST_BASE_URL, transport: fake.transport });
}

/**
 * Renders the app at a given path inside a memory router (FND-08), with the
 * shared providers mounted over an injected client and a fresh query client per
 * call so a case's cache never leaks into the next (WCT-02).
 */
export function renderApp(path = '/', options: RenderAppOptions = {}) {
  const queryClient =
    options.queryClient ??
    new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });

  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppProviders client={options.client ?? defaultClient()} queryClient={queryClient}>
        <AppRoutes />
      </AppProviders>
    </MemoryRouter>,
  );
}
