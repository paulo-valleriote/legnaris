import { describe, expect, it, vi } from 'vitest';
import { createAuthApi } from '../api/auth_api';
import { DomainFailure } from '../api/errors';
import type { Transport } from '../api/transport';
import { createAuthSession } from '../auth/auth_session';
import {
  createFakeTransport,
  createDeferredTransport,
  errorOutcome,
  successOutcome,
} from './fake_transport';

const BASE_URL = 'http://api.test';

const SESSION_BODY = {
  access_token: 'access-1',
  token_type: 'Bearer',
  expires_in: 900,
};

function authApiWith(fake: { readonly transport: Transport }) {
  return createAuthApi({
    baseUrl: BASE_URL,
    transport: fake.transport,
    readCsrfToken: () => undefined,
  });
}

describe('in-memory session (WAS-01, WAS-02, WAS-07, WAS-32)', () => {
  it('Happy path: login holds the token in memory and flips to authenticated (WAS-01)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });

    expect(session.getSnapshot()).toMatchObject({
      state: 'authenticated',
      accessToken: 'access-1',
    });
    expect(session.currentAccessToken()).toBe('access-1');
  });

  it('Edge: login bumps the version exactly once, before applying the token (WAS-17)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    expect(session.getSnapshot().version).toBe(0);
    await session.login({ email: 'a@b.test', password: 'secret' });

    expect(session.getSnapshot().version).toBe(1);
  });

  it('Edge: registration bumps the version exactly once (WAS-02)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY, 201));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });

    expect(session.getSnapshot()).toMatchObject({ state: 'authenticated', version: 1 });
  });

  it('Edge: a rejected login leaves unauthenticated and rethrows the mapped error (WAS-07)', async () => {
    const fake = createFakeTransport(errorOutcome('UNAUTHORIZED', 'bad credentials', 401));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await expect(session.login({ email: 'a@b.test', password: 'wrong' })).rejects.toBeInstanceOf(
      DomainFailure,
    );

    expect(session.getSnapshot()).toMatchObject({ state: 'unauthenticated', accessToken: null });
    expect(session.getSnapshot().version).toBe(0);
  });

  it('Edge: a rejected registration leaves unauthenticated and rethrows (WAS-07)', async () => {
    const fake = createFakeTransport(errorOutcome('INVALID_REQUEST', 'bad input', 400));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await expect(
      session.register({ email: 'a@b.test', username: 'aragorn', password: 'x' }),
    ).rejects.toBeInstanceOf(DomainFailure);

    expect(session.getSnapshot().state).toBe('unauthenticated');
  });

  it('Edge: applying a refreshed token does not bump the version (WAS-17)', async () => {
    const fake = createFakeTransport(successOutcome({ ...SESSION_BODY, access_token: 'access-2' }));
    const session = createAuthSession({
      authApi: authApiWith(fake),
      initialState: 'authenticated',
      initialAccessToken: 'access-1',
    });

    const token = await session.refresh();

    expect(token).toBe('access-2');
    expect(session.getSnapshot()).toMatchObject({ accessToken: 'access-2', version: 0 });
  });

  it('Edge: a refresh rejected with 401 resolves null and keeps the version (WAS-04)', async () => {
    const fake = createFakeTransport(errorOutcome('UNAUTHORIZED', 'no cookie', 401));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await expect(session.refresh()).resolves.toBeNull();

    expect(session.getSnapshot().version).toBe(0);
  });

  it('Security: login never writes the token to a script-readable store (WAS-32)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });

    expect(window.localStorage.length).toBe(0);
    expect(window.sessionStorage.length).toBe(0);
    expect(JSON.stringify(window.localStorage)).not.toContain('access-1');
    expect(JSON.stringify(window.sessionStorage)).not.toContain('access-1');
  });

  it('Edge: restore on a valid cookie settles authenticated through exactly one silent refresh (WAS-03)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.restore();

    expect(session.getSnapshot().state).toBe('authenticated');
    expect(fake.requests.filter((request) => request.url.endsWith('/auth/refresh'))).toHaveLength(
      1,
    );
  });

  it('Edge: a transient restore failure settles unauthenticated and keeps the session-local data (WAS-05)', async () => {
    const fake = createFakeTransport({ failure: new Error('network') });
    const clearSessionData = vi.fn(() => Promise.resolve());
    const session = createAuthSession({
      authApi: authApiWith(fake),
      clearSessionData: () => clearSessionData(),
    });

    await session.restore();

    expect(session.getSnapshot().state).toBe('unauthenticated');
    expect(clearSessionData).not.toHaveBeenCalled();
  });

  it('Edge: concurrent refresh calls share a single in-flight rotation (WAS-13)', async () => {
    const deferred = createDeferredTransport();
    const session = createAuthSession({
      authApi: authApiWith(deferred),
      initialState: 'authenticated',
      initialAccessToken: 'access-1',
    });

    const first = session.refresh();
    const second = session.refresh();

    await vi.waitFor(() => expect(deferred.attempts).toBe(1));
    deferred.resolve(successOutcome(SESSION_BODY));

    await expect(Promise.all([first, second])).resolves.toEqual(['access-1', 'access-1']);
    expect(deferred.attempts).toBe(1);
  });
});
