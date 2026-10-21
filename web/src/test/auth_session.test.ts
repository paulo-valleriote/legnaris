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

const CHALLENGE_BODY = {
  challenge_id: 'challenge-1',
  expires_in: 600,
  resend_after: 60,
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

describe('the pending code step (WCS-01, WCS-02, WCS-03, WCS-04, WCS-06)', () => {
  it('Happy path: a code step establishes no session and holds no credential (WCS-01, WCS-02)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY, 201));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });

    expect(session.getSnapshot()).toMatchObject({ state: 'pendingCode', accessToken: null });
    expect(session.currentAccessToken()).toBeNull();
    expect(session.getSnapshot().version).toBe(0);
  });

  it('Happy path: the pending projection is non-null exactly while the state is pendingCode (WCS-02)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY, 201));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });

    expect(session.getSnapshot().pending).not.toBeNull();
    expect(session.getSnapshot().state).toBe('pendingCode');

    session.cancelCodeStep();

    expect(session.getSnapshot().pending).toBeNull();
    expect(session.getSnapshot().state).toBe('unauthenticated');
  });

  it('Happy path: the pending step carries the address, the username and the advertised lifetime (WCS-19, WCS-20)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY, 201));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });

    expect(session.getSnapshot().pending).toEqual({
      origin: 'register',
      email: 'a@b.test',
      username: 'aragorn',
      expiresIn: 600,
      resendAfter: 60,
    });
  });

  it('Edge: a sign-in step carries no username, and no step carries the password (WCS-21)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });

    expect(session.getSnapshot().pending).toMatchObject({
      origin: 'login',
      email: 'a@b.test',
      username: null,
    });
    expect(JSON.stringify(session.getSnapshot())).not.toContain('secret');
    expect(JSON.stringify(session.getSnapshot())).not.toContain('challenge-1');
  });

  it('Happy path: the verified code establishes the session exactly as a session answer does (WCS-03)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY), successOutcome(SESSION_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });
    await session.submitCode('042917');

    expect(session.getSnapshot()).toMatchObject({
      state: 'authenticated',
      accessToken: 'access-1',
      version: 1,
      pending: null,
    });
    expect(fake.lastRequest?.body).toBe(
      JSON.stringify({ challenge_id: 'challenge-1', code: '042917' }),
    );
  });

  it('Edge: a refused code keeps the pending step and establishes no session (WCS-04)', async () => {
    const fake = createFakeTransport(
      successOutcome(CHALLENGE_BODY),
      errorOutcome('OTP_INVALID', 'invalid or expired code', 401),
    );
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });

    await expect(session.submitCode('000000')).rejects.toMatchObject({
      name: 'DomainFailure',
      kind: 'invalidCode',
    });

    expect(session.getSnapshot()).toMatchObject({
      state: 'pendingCode',
      accessToken: null,
      version: 0,
    });
    expect(session.getSnapshot().pending).not.toBeNull();
  });

  it('Edge: abandoning the step clears it, issues no request and moves no version (WCS-06)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });
    const before = fake.attempts;

    session.cancelCodeStep();

    expect(session.getSnapshot()).toMatchObject({
      state: 'unauthenticated',
      accessToken: null,
      version: 0,
      pending: null,
    });
    expect(fake.attempts).toBe(before);
  });

  it('Happy path: the values the person typed survive the abandonment, and the password does not (WCS-21)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });
    session.cancelCodeStep();

    expect(session.getSnapshot().restored).toEqual({
      origin: 'register',
      email: 'a@b.test',
      username: 'aragorn',
    });
    expect(JSON.stringify(session.getSnapshot())).not.toContain('secret');
    expect(JSON.stringify(session.getSnapshot())).not.toContain('challenge-1');
  });

  it('Edge: a verified step leaves nothing to restore (WCS-21)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY), successOutcome(SESSION_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });
    await session.submitCode('042917');

    expect(session.getSnapshot().restored).toBeNull();
  });

  it('Edge: an answer for an abandoned step is dropped and establishes no session (WCS-06, R10)', async () => {
    const deferred = createDeferredTransport();
    const session = createAuthSession({ authApi: authApiWith(deferred) });

    const pending = session.login({ email: 'a@b.test', password: 'secret' });
    await vi.waitFor(() => expect(deferred.attempts).toBe(1));
    deferred.resolve(successOutcome(CHALLENGE_BODY));
    await pending;

    const verification = session.submitCode('042917');
    await vi.waitFor(() => expect(deferred.attempts).toBe(2));

    session.cancelCodeStep();
    deferred.resolve(successOutcome(SESSION_BODY));
    await verification;

    expect(session.getSnapshot()).toMatchObject({
      state: 'unauthenticated',
      accessToken: null,
      version: 0,
    });
  });

  it('Edge: a step never survives a reload, because nothing about it is persisted (WCS-02, R2)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });

    expect(window.localStorage.length).toBe(0);
    expect(window.sessionStorage.length).toBe(0);
    expect(JSON.stringify(window.localStorage)).not.toContain('challenge-1');
    expect(JSON.stringify(window.sessionStorage)).not.toContain('challenge-1');
  });
});

describe('the resend (WCS-08, WCS-10)', () => {
  it('Happy path: concurrent resend calls share one request and re-seed the window (WCS-08, WCS-10)', async () => {
    const fake = createFakeTransport(
      successOutcome(CHALLENGE_BODY),
      successOutcome({ resend_after: 90 }, 202),
    );
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });

    const first = session.resendCode();
    const second = session.resendCode();

    expect(second).toBe(first);
    await first;

    expect(
      fake.requests.filter((request) => request.url.endsWith('/auth/challenge/resend')),
    ).toHaveLength(1);
    expect(session.getSnapshot().pending?.resendAfter).toBe(90);
  });

  it('Edge: a refused resend keeps the pending step, the handle and the window (WCS-09, WCS-10)', async () => {
    const fake = createFakeTransport(
      successOutcome(CHALLENGE_BODY),
      errorOutcome('EMAIL_UNAVAILABLE', 'email delivery is temporarily unavailable', 503),
    );
    const session = createAuthSession({ authApi: authApiWith(fake) });

    await session.login({ email: 'a@b.test', password: 'secret' });

    await expect(session.resendCode()).rejects.toMatchObject({
      name: 'DomainFailure',
      kind: 'emailUnavailable',
    });

    expect(session.getSnapshot()).toMatchObject({ state: 'pendingCode', accessToken: null });
    expect(session.getSnapshot().pending?.resendAfter).toBe(60);
    expect(JSON.stringify(session.getSnapshot())).not.toContain('challenge-1');
  });
});
