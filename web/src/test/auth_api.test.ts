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

describe('auth client operations (WAS-01, WAS-02, WAS-06, WAS-18)', () => {
  it('Happy path: register posts one credentials-include request and decodes the session body (WAS-02, WAS-06)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY, 201));

    const body = await authWith(fake).register({
      email: 'a@b.test',
      username: 'aragorn',
      password: 'secret',
    });

    expect(body).toEqual({ accessToken: 'access-1', tokenType: 'Bearer', expiresIn: 900 });
    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/register`);
    expect(fake.lastRequest?.credentials).toBe('include');
    expect(fake.lastRequest?.body).toBe(
      JSON.stringify({ email: 'a@b.test', username: 'aragorn', password: 'secret' }),
    );
  });

  it('Edge: register issues no follow-up login, so exactly one request is sent (WAS-02)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY, 201));

    await authWith(fake).register({ email: 'a@b.test', username: 'aragorn', password: 'secret' });

    expect(fake.requests).toHaveLength(1);
    expect(fake.requests.some((request) => request.url.endsWith('/auth/login'))).toBe(false);
  });

  it('Happy path: login posts one credentials-include request and decodes the session body (WAS-01, WAS-06)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));

    const body = await authWith(fake).login({ email: 'a@b.test', password: 'secret' });

    expect(body.accessToken).toBe('access-1');
    expect(fake.attempts).toBe(1);
    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/login`);
    expect(fake.lastRequest?.credentials).toBe('include');
  });

  it('Edge: register and login send no CSRF header (WAS-18)', async () => {
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

  it('Happy path: refresh echoes the CSRF cookie when present (WAS-18)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));

    await authWith(fake, () => 'csrf-value').refresh();

    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/refresh`);
    expect(fake.lastRequest?.credentials).toBe('include');
    expect(fake.lastRequest?.headers?.['X-CSRF-Token']).toBe('csrf-value');
  });

  it('Edge: an absent CSRF token on refresh sends no header (WAS-18)', async () => {
    const fake = createFakeTransport(successOutcome(SESSION_BODY));

    await authWith(fake, () => undefined).refresh();

    expect(fake.lastRequest?.headers?.['X-CSRF-Token']).toBeUndefined();
  });

  it('Happy path: logout accepts a 204 no-content response and echoes the CSRF token (WAS-18)', async () => {
    const fake = createFakeTransport({ status: 204, body: '' });

    await expect(authWith(fake, () => 'csrf-value').logout()).resolves.toBeUndefined();

    expect(fake.lastRequest?.method).toBe('POST');
    expect(fake.lastRequest?.url).toBe(`${BASE_URL}/auth/logout`);
    expect(fake.lastRequest?.credentials).toBe('include');
    expect(fake.lastRequest?.headers?.['X-CSRF-Token']).toBe('csrf-value');
  });

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
});
