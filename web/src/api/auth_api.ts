/**
 * The four browser session operations (WAS-01, WAS-02, WAS-06, WAS-18, WAS-30, R2).
 *
 * `register`/`login`/`refresh`/`logout` all go through the shared caller over the
 * **raw** transport with `credentials: 'include'`, so the browser attaches the
 * `HttpOnly` refresh cookie on the auth routes. `refresh` and `logout` echo the
 * script-readable CSRF cookie in `X-CSRF-Token`; `register` and `login` carry no
 * CSRF header because the server issues the token with the session. The caller
 * is the raw transport, so `/auth/*` can never carry a Bearer or trigger a
 * refresh (WAS-16).
 */

import { createCaller } from './request';
import type { CallerOptions } from './request';
import { drift, isPlainObject, requireNumber, requireString } from './parse';
import type { Transport } from './transport';

/** The decoded session body the three JSON operations return (WAS-01). */
export interface SessionBody {
  readonly accessToken: string;
  readonly tokenType: string;
  readonly expiresIn: number;
}

/** The registration input (WAS-02). */
export interface RegisterInput {
  readonly email: string;
  readonly username: string;
  readonly password: string;
}

/** The login input (WAS-01). */
export interface LoginInput {
  readonly email: string;
  readonly password: string;
}

/** The four consumed auth operations, one method each (R2). */
export interface AuthApi {
  register(input: RegisterInput): Promise<SessionBody>;
  login(input: LoginInput): Promise<SessionBody>;
  refresh(): Promise<SessionBody>;
  logout(): Promise<void>;
}

/** Construction options for {@link createAuthApi} (R2). */
export interface AuthApiOptions extends CallerOptions {
  /** Resolves the CSRF cookie value at request time; `undefined` sends no header (WAS-18). */
  readonly readCsrfToken: () => string | undefined;
  /** The raw transport the auth operations go through (never the authenticated one). */
  readonly transport: Transport;
}

function decodeSession(data: unknown): SessionBody {
  if (!isPlainObject(data)) {
    throw drift('session payload is not an object');
  }

  return {
    accessToken: requireString(data, 'access_token'),
    tokenType: requireString(data, 'token_type'),
    expiresIn: requireNumber(data, 'expires_in'),
  };
}

/** Builds the four session operations over the raw transport (R2). */
export function createAuthApi(options: AuthApiOptions): AuthApi {
  const { readCsrfToken } = options;
  const caller = createCaller(options);

  const csrfHeaders = (): Readonly<Record<string, string>> => {
    const token = readCsrfToken();
    return token === undefined ? {} : { 'X-CSRF-Token': token };
  };

  return {
    register: (input) =>
      caller.call(
        'POST',
        '/auth/register',
        JSON.stringify({ email: input.email, username: input.username, password: input.password }),
        decodeSession,
        { credentials: 'include' },
      ),
    login: (input) =>
      caller.call(
        'POST',
        '/auth/login',
        JSON.stringify({ email: input.email, password: input.password }),
        decodeSession,
        { credentials: 'include' },
      ),
    refresh: () =>
      caller.call('POST', '/auth/refresh', undefined, decodeSession, {
        credentials: 'include',
        headers: csrfHeaders(),
      }),
    logout: () =>
      caller.callNoContent('POST', '/auth/logout', undefined, {
        credentials: 'include',
        headers: csrfHeaders(),
      }),
  };
}
