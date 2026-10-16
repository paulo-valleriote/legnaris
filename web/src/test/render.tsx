import { render } from '@testing-library/react';
import { QueryClient } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import type { ApiClient } from '../api/client';
import { createApiClient } from '../api/client';
import { AppRoutes } from '../app/router';
import { AppProviders } from '../app/providers';
import type { MemorySheetStore, SheetStore } from '../library/sheet_store';
import { createMemorySheetStore } from '../library/sheet_store';
import { createFakeTransport, successOutcome } from './fake_transport';

const TEST_BASE_URL = 'http://api.test';

export interface RenderAppOptions {
  /** The client the screens read; tests inject one built over a fake transport. */
  readonly client?: ApiClient;
  /** The query client to mount with; tests reuse one to prove cache behaviour. */
  readonly queryClient?: QueryClient;
  /** The local store to mount with; a fresh in-memory double is used by default (WSC-11). */
  readonly sheetStore?: SheetStore;
}

/** A client whose catalog resolves with no entries, so a plain render stays meaningful. */
function defaultClient(): ApiClient {
  const fake = createFakeTransport(successOutcome([]));
  return createApiClient({ baseUrl: TEST_BASE_URL, transport: fake.transport });
}

/**
 * Renders the app at a given path inside a memory router (FND-08), with the
 * shared providers mounted over an injected client, a fresh query client and a
 * fresh in-memory `SheetStore` per call, so a case's cache never leaks into the
 * next (WCT-02, WSC-11).
 */
export function renderApp(path = '/', options: RenderAppOptions = {}) {
  const queryClient =
    options.queryClient ??
    new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  const sheetStore = options.sheetStore ?? createMemorySheetStore();

  const result = render(
    <MemoryRouter initialEntries={[path]}>
      <AppProviders
        client={options.client ?? defaultClient()}
        queryClient={queryClient}
        sheetStore={sheetStore}
      >
        <AppRoutes />
      </AppProviders>
    </MemoryRouter>,
  );

  return {
    ...result,
    queryClient,
    sheetStore: sheetStore as MemorySheetStore,
  };
}
