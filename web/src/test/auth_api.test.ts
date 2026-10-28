import { describe, expect, it } from 'vitest';
import { createAuthApi } from '../api/auth_api';
import type { AuthApiOptions } from '../api/auth_api';
import { DefensiveParseFailure } from '../api/errors';
import { createFakeTransport, errorOutcome, successOutcome } from './fake_transport';

const BASE_URL = 'http://api.test';

const SESSION_BODY = {
  access_token: 'access-1',
  token_type: 'Bearer',
  expires_in: 900,
};

const CHALLENGE_BODY = {
  challenge_id: '01a15c37-6f2d-7640-9dac-db9046d08d5c',
  expires_in: 600,
  resend_after: 60,
};

function authWith(
  fake: ReturnType<typeof createFakeTransport>,
  readCsrfToken: () => string | undefined = () => undefined,
) {
  const options: AuthApiOptions = {
    baseUrl: BASE_URL,
    transport: fake.transport,
    readCsrfToken,
  };
  return createAuthApi(options);
}

describe('auth client operations (WAS-01, WAS-02, WAS-06, WAS-18, WCS-01, WCS-15)', () => {
  it('Error: a malformed session body is a defensive parse failure (WAS-02)', async () => {
    const fake = createFakeTransport(successOutcome({ access_token: 'access-1' }));

    await expect(
      authWith(fake).login({ email: 'a@b.test', password: 'secret' }),
    ).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Error: a 401 refusal surfaces as a classified unauthorized domain failure (WAS-04, WAS-30)', async () => {
    const fake = createFakeTransport(errorOutcome('UNAUTHORIZED', 'no cookie', 401));

    await expect(authWith(fake).refresh()).rejects.toMatchObject({
      name: 'DomainFailure',
      kind: 'unauthorized',
      status: 401,
    });
  });

  it('Edge: register issues no follow-up login, so exactly one request is sent (WAS-02)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY, 201));

    await authWith(fake).register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });

    expect(fake.requests).toHaveLength(1);
    expect(fake.requests.some((request) => request.url.endsWith('/auth/login'))).toBe(false);
  });

  it('Edge: register and login send no CSRF header (WAS-18, WCS-15)', async () => {
    const registerFake = createFakeTransport(successOutcome(SESSION_BODY, 201));
    const loginFake = createFakeTransport(successOutcome(SESSION_BODY));

    await authWith(registerFake, () => 'csrf-value').register({
      email: 'a@b.test',
      username: 'aragorn',
      password: 'secret',
    });
    await authWith(loginFake, () => 'csrf-value').login({ email: 'a@b.test', password: 'secret' });

    expect(registerFake.lastRequest?.headers?.['X-CSRF-Token']).toBeUndefined();
    expect(loginFake.lastRequest?.headers?.['X-CSRF-Token']).toBeUndefined();
  });

  it('Edge: an absent CSRF token on refresh sends no header (WAS-18)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));

    await authWith(fake, () => undefined).refresh();

    expect(fake.lastRequest?.headers?.['X-CSRF-Token']).toBeUndefined();
  });

  it('Happy path: register posts one credentials-include request and decodes the session outcome (WAS-02, WAS-06, WCS-01)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY, 201));

    const outcome = await authWith(fake).register({
      email: 'a@b.test',
      username: 'aragorn',
      password: 'secret',
    });

    expect(outcome).toEqual({
      kind: 'session',
      session: { accessToken: 'access-1', tokenType: 'Bearer', expiresIn: 900 },
    });
    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/register`);
    expect(fake.lastRequest?.credentials).toBe('include');
    expect(fake.lastRequest?.body).toBe(
      JSON.stringify({ email: 'a@b.test', username: 'aragorn', password: 'secret' }),
    );
  });

  it('Happy path: login posts one credentials-include request and decodes the session outcome (WAS-01, WAS-06, WCS-01)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));

    const outcome = await authWith(fake).login({ email: 'a@b.test', password: 'secret' });

    expect(outcome.kind).toBe('session');
    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/login`);
    expect(fake.lastRequest?.credentials).toBe('include');
  });

  it('Happy path: refresh echoes the CSRF cookie when present (WAS-18)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));

    await authWith(fake, () => 'csrf-value').refresh();

    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/refresh`);
    expect(fake.lastRequest?.credentials).toBe('include');
    expect(fake.lastRequest?.headers?.['X-CSRF-Token']).toBe('csrf-value');
  });

  it('Happy path: logout accepts a 204 no-content response and echoes the CSRF token (WAS-18)', async () => {
    const fake = createFakeTransport({ status: 204, body: '' });

    await expect(authWith(fake, () => 'csrf-value').logout()).resolves.toBeUndefined();

    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/logout`);
    expect(fake.lastRequest?.credentials).toBe('include');
    expect(fake.lastRequest?.headers?.['X-CSRF-Token']).toBe('csrf-value');
  });
});

describe('the credential answer carries either a session or a code step (WCS-01, WCS-03)', () => {
  it('Error: an answer carrying both a session and a challenge is a defensive parse failure (WCS-01, R1)', async () => {
    const fake = createFakeTransport(successOutcome({ ...SESSION_BODY, ...CHALLENGE_BODY }, 201));

    await expect(
      authWith(fake).register({ email: 'a@b.test', username: 'aragorn', password: 'secret' }),
    ).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Error: an answer carrying neither a session nor a challenge is a defensive parse failure (WCS-01, R1)', async () => {
    const fake = createFakeTransport(successOutcome({ token_type: 'Bearer' }, 201));

    await expect(
      authWith(fake).register({ email: 'a@b.test', username: 'aragorn', password: 'secret' }),
    ).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Error: a challenge answer without its resend window is a defensive parse failure (WCS-01)', async () => {
    const fake = createFakeTransport(
      successOutcome({ challenge_id: CHALLENGE_BODY.challenge_id, expires_in: 600 }, 201),
    );

    await expect(
      authWith(fake).register({ email: 'a@b.test', username: 'aragorn', password: 'secret' }),
    ).rejects.toBeInstanceOf(DefensiveParseFailure);
  });

  it('Edge: the discrimination reads the payload shape, not the status, because registration answers 201 in both gate states (WCS-01, R1)', async () => {
    const sessionFake = createFakeTransport(successOutcome(SESSION_BODY, 201));
    const challengeFake = createFakeTransport(successOutcome(CHALLENGE_BODY, 201));

    const session = await authWith(sessionFake).login({ email: 'a@b.test', password: 'secret' });
    const challenge = await authWith(challengeFake).login({
      email: 'a@b.test',
      password: 'secret',
    });

    expect(sessionFake.lastRequest?.url).toBe(challengeFake.lastRequest?.url);
    expect(session.kind).toBe('session');
    expect(challenge.kind).toBe('challenge');
  });

  it('Happy path: a challenge answer decodes as a code step and carries no credential (WCS-01)', async () => {
    const fake = createFakeTransport(successOutcome(CHALLENGE_BODY, 201));

    const outcome = await authWith(fake).register({
      email: 'a@b.test',
      username: 'aragorn',
      password: 'secret',
    });

    expect(outcome).toEqual({
      kind: 'challenge',
      challenge: {
        challengeId: '01a15c37-6f2d-7640-9dac-db9046d08d5c',
        expiresIn: 600,
        resendAfter: 60,
      },
    });
    expect(fake.lastRequest?.body).not.toContain('access_token');
  });
});

describe('the challenge operations (WCS-03, WCS-08, WCS-15)', () => {
  it('Error: a refused code surfaces as the invalid-code kind (WCS-11)', async () => {
    const fake = createFakeTransport(errorOutcome('OTP_INVALID', 'invalid or expired code', 401));

    await expect(
      authWith(fake).verifyChallenge({ challengeId: 'challenge-1', code: '000000' }),
    ).rejects.toMatchObject({ name: 'DomainFailure', kind: 'invalidCode', status: 401 });
  });

  it('Error: an unavailable send surfaces as the email-unavailable kind (WCS-11)', async () => {
    const fake = createFakeTransport(
      errorOutcome('EMAIL_UNAVAILABLE', 'email delivery is temporarily unavailable', 503),
    );

    await expect(
      authWith(fake).resendChallenge({ challengeId: 'challenge-1' }),
    ).rejects.toMatchObject({ name: 'DomainFailure', kind: 'emailUnavailable', status: 503 });
  });

  it('Edge: a code with leading zeros reaches the server exactly as typed (WCS-03)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));

    await authWith(fake).verifyChallenge({ challengeId: 'challenge-1', code: '042917' });

    expect(JSON.parse(fake.lastRequest?.body ?? '{}')).toEqual({
      challenge_id: 'challenge-1',
      code: '042917',
    });
  });

  it('Edge: neither challenge operation sends a CSRF header or a Bearer (WCS-15, R7)', async () => {
    const verifyFake = createFakeTransport(successOutcome(SESSION_BODY));
    const resendFake = createFakeTransport(successOutcome({ resend_after: 60 }, 202));

    await authWith(verifyFake, () => 'csrf-value').verifyChallenge({
      challengeId: 'challenge-1',
      code: '042917',
    });
    await authWith(resendFake, () => 'csrf-value').resendChallenge({ challengeId: 'challenge-1' });

    for (const fake of [verifyFake, resendFake]) {
      expect(fake.lastRequest?.headers?.['X-CSRF-Token']).toBeUndefined();
      expect(fake.lastRequest?.headers?.['Authorization']).toBeUndefined();
    }
  });

  it('Edge: neither challenge operation triggers a refresh, so each issues exactly one request (WCS-15)', async () => {
    const verifyFake = createFakeTransport(successOutcome(SESSION_BODY));
    const resendFake = createFakeTransport(successOutcome({ resend_after: 60 }, 202));

    await authWith(verifyFake).verifyChallenge({ challengeId: 'challenge-1', code: '042917' });
    await authWith(resendFake).resendChallenge({ challengeId: 'challenge-1' });

    for (const fake of [verifyFake, resendFake]) {
      expect(fake.requests).toHaveLength(1);
      expect(fake.requests.some((request) => request.url.endsWith('/auth/refresh'))).toBe(false);
    }
  });

  it('Happy path: the verification posts the handle and the code and decodes the session body (WCS-03)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));

    const body = await authWith(fake).verifyChallenge({
      challengeId: 'challenge-1',
      code: '042917',
    });

    expect(body).toEqual({ accessToken: 'access-1', tokenType: 'Bearer', expiresIn: 900 });
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/challenge/verify`);
    expect(fake.lastRequest?.credentials).toBe('include');
    expect(fake.lastRequest?.body).toBe(
      JSON.stringify({ challenge_id: 'challenge-1', code: '042917' }),
    );
  });

  it('Happy path: the resend posts the handle alone and decodes the advertised window (WCS-08)', async () => {
    const fake = createFakeTransport(successOutcome({ resend_after: 60 }, 202));

    const result = await authWith(fake).resendChallenge({ challengeId: 'challenge-1' });

    expect(result).toEqual({ resendAfter: 60 });
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/challenge/resend`);
    expect(fake.lastRequest?.credentials).toBe('include');
    expect(fake.lastRequest?.body).toBe(JSON.stringify({ challenge_id: 'challenge-1' }));
  });
});
