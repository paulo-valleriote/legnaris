/**
 * The browser session operations and the code step (WAS-01, WAS-02, WAS-06,
 * WAS-18, WAS-30, R2, WCS-01, WCS-03, WCS-15).
 *
 * `register`/`login`/`refresh`/`logout` all go through the shared caller over
 * the **raw** transport with `credentials: 'include'`, so the browser attaches
 * the `HttpOnly` refresh cookie on the auth routes. `refresh` and `logout` echo
 * the script-readable CSRF cookie in `X-CSRF-Token`; `register` and `login`
 * carry no CSRF header because the server issues the token with the session.
 * The caller is the raw transport, so `/auth/*` can never carry a Bearer or
 * trigger a refresh (WAS-16).
 *
 * The two challenge operations join them: `verifyChallenge` establishes the
 * session the code step was waiting for and `resendChallenge` asks for the code
 * again. Both go through the same raw transport with the credentials included,
 * so the verification answer's cookies are accepted on the cross-origin API,
 * and neither carries a Bearer, a CSRF header or a refresh (WCS-15, R7).
 *
 * A credential operation answers either a session or a code step, and the
 * discrimination reads the payload's own shape: registration answers the same
 * status in both gate states, so the status cannot tell them apart (WCS-01,
 * R1).
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

/**
 * A started code step: the handle the client presents back, the lifetime the
 * step was claimed for and the window before a resend records a new send
 * (WCS-01).
 */
export interface ChallengeStep {
  readonly challengeId: string;
  readonly expiresIn: number;
  readonly resendAfter: number;
}

/**
 * The answer a credential operation gives: a session or a code step (WCS-01).
 *
 * The client tells them apart by the payload's shape, never by the status, and
 * never guesses: a payload carrying both keys or neither is a defensive parse
 * failure (R1).
 */
export type SignInOutcome =
  | { readonly kind: 'session'; readonly session: SessionBody }
  | { readonly kind: 'challenge'; readonly challenge: ChallengeStep }
  | { readonly kind: 'deferred' };

/** The delivery marker of a registration whose code could not be queued (D1). */
const DELIVERY_DEFERRED = 'deferred';

/** The answer a resend gives: the window the client counts down (WCS-08). */
export interface ResendResult {
  readonly resendAfter: number;
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

/** The verification input: the handle and the code, both verbatim (WCS-03). */
export interface VerifyChallengeInput {
  readonly challengeId: string;
  readonly code: string;
}

/** The resend input: the handle alone (WCS-08). */
export interface ResendChallengeInput {
  readonly challengeId: string;
}

/** The consumed auth operations, one method each (R2, WCS-03, WCS-08). */
export interface AuthApi {
  register(input: RegisterInput): Promise<SignInOutcome>;
  login(input: LoginInput): Promise<SignInOutcome>;
  refresh(): Promise<SessionBody>;
  logout(): Promise<void>;
  verifyChallenge(input: VerifyChallengeInput): Promise<SessionBody>;
  resendChallenge(input: ResendChallengeInput): Promise<ResendResult>;
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

function decodeChallenge(data: unknown): ChallengeStep {
  if (!isPlainObject(data)) {
    throw drift('challenge payload is not an object');
  }

  return {
    challengeId: requireString(data, 'challenge_id'),
    expiresIn: requireNumber(data, 'expires_in'),
    resendAfter: requireNumber(data, 'resend_after'),
  };
}

/**
 * Decodes the answer of a credential operation (WCS-01, R1).
 *
 * The payload's own keys decide which answer it is: an `access_token` is a
 * session, a `challenge_id` is a code step, and a bare `delivery: "deferred"`
 * is an account created with no code queued (D1). A payload carrying both keys,
 * or neither without the deferred marker, is a drift, never a silent pick — the
 * envelope decoder exists to keep a contract change visible.
 */
export function decodeSignInOutcome(data: unknown): SignInOutcome {
  if (!isPlainObject(data)) {
    throw drift('credential payload is not an object');
  }

  const carriesSession = 'access_token' in data;
  const carriesChallenge = 'challenge_id' in data;

  if (!carriesSession && !carriesChallenge && data.delivery === DELIVERY_DEFERRED) {
    return { kind: 'deferred' };
  }

  if (carriesSession === carriesChallenge) {
    throw drift('the credential answer carries both a session and a challenge, or neither');
  }

  return carriesSession
    ? { kind: 'session', session: decodeSession(data) }
    : { kind: 'challenge', challenge: decodeChallenge(data) };
}

function decodeResend(data: unknown): ResendResult {
  if (!isPlainObject(data)) {
    throw drift('resend payload is not an object');
  }

  return { resendAfter: requireNumber(data, 'resend_after') };
}

/** Builds the session and challenge operations over the raw transport (R2). */
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
        decodeSignInOutcome,
        { credentials: 'include' },
      ),
    login: (input) =>
      caller.call(
        'POST',
        '/auth/login',
        JSON.stringify({ email: input.email, password: input.password }),
        decodeSignInOutcome,
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
    verifyChallenge: (input) =>
      caller.call(
        'POST',
        '/auth/challenge/verify',
        JSON.stringify({ challenge_id: input.challengeId, code: input.code }),
        decodeSession,
        { credentials: 'include' },
      ),
    resendChallenge: (input) =>
      caller.call(
        'POST',
        '/auth/challenge/resend',
        JSON.stringify({ challenge_id: input.challengeId }),
        decodeResend,
        { credentials: 'include' },
      ),
  };
}
