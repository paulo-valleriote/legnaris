import { describe, expect, it } from 'vitest';
import { createAuthApi, decodeSignInOutcome } from '../api/auth_api';
import { DefensiveParseFailure } from '../api/errors';
import type { Transport } from '../api/transport';
import { createAuthSession } from '../auth/auth_session';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const BASE_URL = 'http://api.test';

const SESSION_BODY = { access_token: 'access-1', token_type: 'Bearer', expires_in: 900 };
const CHALLENGE_BODY = { challenge_id: 'challenge-1', expires_in: 600, resend_after: 60 };
const DEFERRED_BODY = { delivery: 'deferred' };

function authApiWith(transport: Transport) {
  return createAuthApi({ baseUrl: BASE_URL, transport, readCsrfToken: () => undefined });
}

describe('decodeSignInOutcome deferred answer (D1)', () => {
  it.each([
    {
      name: 'Happy path: bare deferred marker decodes as deferred',
      data: DEFERRED_BODY,
      kind: 'deferred',
    },
    {
      name: 'Edge: a challenge key wins over a stray deferred marker',
      data: { ...DEFERRED_BODY, ...CHALLENGE_BODY },
      kind: 'challenge',
    },
    {
      name: 'Edge: a session key wins over a stray deferred marker',
      data: { ...DEFERRED_BODY, ...SESSION_BODY },
      kind: 'session',
    },
    { name: 'Edge: unknown delivery value is a drift', data: { delivery: 'later' }, kind: 'drift' },
    { name: 'Edge: empty object is a drift', data: {}, kind: 'drift' },
  ])('$name', ({ data, kind }) => {
    if (kind === 'drift') {
      expect(() => decodeSignInOutcome(data)).toThrow(DefensiveParseFailure);
      return;
    }
    expect(decodeSignInOutcome(data).kind).toBe(kind);
  });
});

describe('auth session deferred registration (D1, D8)', () => {
  it('Happy path: a deferred register answer holds no session and shows the deferred notice', async () => {
    const fake = createFakeTransport(successOutcome(DEFERRED_BODY, 201));
    const session = createAuthSession({ authApi: authApiWith(fake.transport) });

    await session.register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });

    expect(session.getSnapshot()).toMatchObject({
      state: 'unauthenticated',
      accessToken: null,
      pending: null,
      notice: 'deferred',
      restored: { origin: 'register', email: 'a@b.test', username: 'aragorn' },
    });
    expect(session.getSnapshot().version).toBe(0);
  });

  it('Edge: a deferred answer starts no verification step, so a later submit is refused', async () => {
    const fake = createFakeTransport(successOutcome(DEFERRED_BODY, 201));
    const session = createAuthSession({ authApi: authApiWith(fake.transport) });

    await session.register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });

    await expect(session.submitCode('123456')).rejects.toThrow('no code step is pending');
  });

  it('Edge: the deferred notice is cleared when the next attempt starts', async () => {
    const fake = createFakeTransport(
      successOutcome(DEFERRED_BODY, 201),
      successOutcome(CHALLENGE_BODY),
    );
    const session = createAuthSession({ authApi: authApiWith(fake.transport) });

    await session.register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });
    expect(session.getSnapshot().notice).toBe('deferred');

    await session.login({ email: 'a@b.test', password: 'secret' });

    expect(session.getSnapshot()).toMatchObject({ state: 'pendingCode', notice: null });
  });

  it('Error: a refused register (rate limited) keeps the session unauthenticated with no notice', async () => {
    const fake = createFakeTransport(errorOutcome('RATE_LIMITED', 'slow down', 429));
    const session = createAuthSession({ authApi: authApiWith(fake.transport) });

    await expect(
      session.register({ email: 'a@b.test', username: 'aragorn', password: 'secret' }),
    ).rejects.toMatchObject({ kind: 'rateLimited' });

    expect(session.getSnapshot()).toMatchObject({ state: 'unauthenticated', notice: null });
  });
});

describe('auth session verified notice (D8)', () => {
  it('Happy path: a correct code establishes the session with the verified notice', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY), successOutcome(SESSION_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake.transport) });

    await session.login({ email: 'a@b.test', password: 'secret' });
    await session.submitCode('123456');

    expect(session.getSnapshot()).toMatchObject({
      state: 'authenticated',
      accessToken: 'access-1',
      notice: 'verified',
    });
  });

  it('Edge: dismissing the notice clears it without touching the session', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY), successOutcome(SESSION_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake.transport) });

    await session.login({ email: 'a@b.test', password: 'secret' });
    await session.submitCode('123456');
    session.dismissNotice();
    session.dismissNotice();

    expect(session.getSnapshot()).toMatchObject({ state: 'authenticated', notice: null });
  });

  it('Edge: a wrong code keeps the step and raises no verified notice', async () => {
    const fake = createFakeTransport(
      successOutcome(CHALLENGE_BODY),
      errorOutcome('OTP_INVALID', 'wrong code', 401),
    );
    const session = createAuthSession({ authApi: authApiWith(fake.transport) });

    await session.login({ email: 'a@b.test', password: 'secret' });
    await expect(session.submitCode('000000')).rejects.toMatchObject({ kind: 'invalidCode' });

    expect(session.getSnapshot()).toMatchObject({ state: 'pendingCode', notice: null });
  });

  it('Edge: logout clears a pending notice', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY), successOutcome(SESSION_BODY));
    const session = createAuthSession({ authApi: authApiWith(fake.transport) });

    await session.login({ email: 'a@b.test', password: 'secret' });
    await session.submitCode('123456');
    await session.logout();

    expect(session.getSnapshot()).toMatchObject({ state: 'unauthenticated', notice: null });
  });
});
