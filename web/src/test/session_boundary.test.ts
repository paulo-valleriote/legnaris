import { describe, expect, it } from 'vitest';
import { act, screen, waitFor } from '@testing-library/react';
import { createAuthApi } from '../api/auth_api';
import type { Sheet } from '../api/models/sheet';
import type { Transport } from '../api/transport';
import { createAuthSession } from '../auth/auth_session';
import { clearSessionData } from '../library/session_data';
import { createMemorySheetStore } from '../library/sheet_store';
import { message } from '../l10n/strings';
import {
  createDeferredTransport,
  createFakeTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';
import { renderApp } from './render';

const BASE_URL = 'http://api.test';

const SESSION_BODY = { access_token: 'access-1', token_type: 'Bearer', expires_in: 900 };

const SHEET: Sheet = {
  id: '01a0f967-f42b-710e-851f-8f293a634921',
  templateId: 'fantasy_v1',
  name: 'Aragorn',
  data: {},
  createdAt: '2026-10-01T21:38:54.635Z',
  updatedAt: '2026-10-02T09:00:00.000Z',
};

function sessionOver(
  transport: Transport,
  clear: () => Promise<void>,
  state: 'restoring' | 'authenticated' = 'authenticated',
) {
  return createAuthSession({
    authApi: createAuthApi({ baseUrl: BASE_URL, transport, readCsrfToken: () => undefined }),
    clearSessionData: clear,
    initialState: state,
    initialAccessToken: state === 'authenticated' ? 'access-1' : null,
  });
}

describe('session boundary (WAS-19 … WAS-23, WAS-32)', () => {
  it('Happy path: sign-out empties the cache and the drafts, drops the token and shows the form (WAS-19)', async () => {
    const store = createMemorySheetStore();
    await store.putSheet(SHEET);
    await store.writeDraft(SHEET.id, { stats: { dexterity: 18 } });
    const fake = createFakeTransport({ status: 204, body: '' });
    const session = sessionOver(fake.transport, () => clearSessionData(store));

    renderApp('/', { session, sheetStore: store });

    await act(async () => {
      await session.logout();
    });

    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );
    expect(session.getSnapshot()).toMatchObject({ state: 'unauthenticated', accessToken: null });
    expect(store.records()).toEqual([]);
    await expect(store.readSheet(SHEET.id)).resolves.toBeNull();
    await expect(store.readDraft(SHEET.id)).resolves.toBeNull();
  });

  it('Happy path: a forced logout clears the store identically, so no record survives the boundary (WAS-20, WAS-22)', async () => {
    const store = createMemorySheetStore();
    await store.putSheet(SHEET);
    await store.writeDraft(SHEET.id, { stats: { dexterity: 18 } });
    const fake = createFakeTransport(successOutcome(SESSION_BODY));
    const session = sessionOver(fake.transport, () => clearSessionData(store));

    await act(async () => {
      await session.onUnauthorized();
    });

    expect(session.getSnapshot()).toMatchObject({ state: 'unauthenticated', version: 1 });
    expect(store.records()).toEqual([]);
  });

  it('Edge: a failing clearAll still ends the session and shows the form (WAS-21)', async () => {
    const store = createMemorySheetStore();
    store.clearAll = () => Promise.reject(new Error('idb closed'));
    const fake = createFakeTransport({ status: 204, body: '' });
    const session = sessionOver(fake.transport, () => clearSessionData(store));

    renderApp('/', { session, sheetStore: store });

    await act(async () => {
      await session.logout();
    });

    expect(session.getSnapshot().state).toBe('unauthenticated');
    await waitFor(() =>
      expect(screen.getByRole('button', { name: message('auth.submitLogin') })).toBeInTheDocument(),
    );
  });

  it('Edge: a transient refresh failure is not a boundary and leaves the cached sheet and the draft in place (WAS-05, WAS-15)', async () => {
    const store = createMemorySheetStore();
    await store.putSheet(SHEET);
    await store.writeDraft(SHEET.id, { stats: { dexterity: 18 } });
    const fake = createFakeTransport({ failure: new Error('network') });
    const session = sessionOver(fake.transport, () => clearSessionData(store), 'restoring');

    await session.restore();

    expect(session.getSnapshot().state).toBe('unauthenticated');
    expect(store.records()).toHaveLength(2);
  });

  it('Edge: a failed /auth/logout never blocks the local teardown (WAS-19)', async () => {
    const store = createMemorySheetStore();
    await store.putSheet(SHEET);
    const fake = createFakeTransport(errorOutcome('INTERNAL_ERROR', 'boom', 500));
    const session = sessionOver(fake.transport, () => clearSessionData(store));

    await session.logout();

    expect(session.getSnapshot()).toMatchObject({ state: 'unauthenticated', accessToken: null });
    expect(store.records()).toEqual([]);
  });

  it('Edge: activating sign-out twice in one frame issues exactly one /auth/logout (WAS-23)', async () => {
    const store = createMemorySheetStore();
    const deferred = createDeferredTransport();
    const session = sessionOver(deferred.transport, () => clearSessionData(store));

    const first = session.logout();
    const second = session.logout();

    expect(second).toBe(first);
    await waitFor(() => expect(deferred.attempts).toBe(1));

    deferred.resolve({ status: 204, body: '' });
    await first;

    expect(session.getSnapshot().state).toBe('unauthenticated');
    expect(
      deferred.requests.filter((request) => request.url.endsWith('/auth/logout')),
    ).toHaveLength(1);
  });
});
