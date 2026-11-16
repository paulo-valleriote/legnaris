import { createCaller } from './request';
import type { CallerOptions } from './request';
import { drift, isPlainObject, requireNumber, requireString } from './parse';
import type { Transport } from './transport';

export interface SessionBody {
  readonly accessToken: string;
  readonly tokenType: string;
  readonly expiresIn: number;
}

export interface ChallengeStep {
  readonly challengeId: string;
  readonly expiresIn: number;
  readonly resendAfter: number;
}

export type SignInOutcome =
  | { readonly kind: 'session'; readonly session: SessionBody }
  | { readonly kind: 'challenge'; readonly challenge: ChallengeStep }
  | { readonly kind: 'deferred' };

const DELIVERY_DEFERRED = 'deferred';

export interface ResendResult {
  readonly resendAfter: number;
}

export interface RegisterInput {
  readonly email: string;
  readonly username: string;
  readonly password: string;
}

export interface LoginInput {
  readonly email: string;
  readonly password: string;
}

export interface VerifyChallengeInput {
  readonly challengeId: string;
  readonly code: string;
}

export interface ResendChallengeInput {
  readonly challengeId: string;
}

export interface AuthApi {
  register(input: RegisterInput): Promise<SignInOutcome>;
  login(input: LoginInput): Promise<SignInOutcome>;
  refresh(): Promise<SessionBody>;
  logout(): Promise<void>;
  verifyChallenge(input: VerifyChallengeInput): Promise<SessionBody>;
  resendChallenge(input: ResendChallengeInput): Promise<ResendResult>;
}

export interface AuthApiOptions extends CallerOptions {
  readonly readCsrfToken: () => string | undefined;
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

function decodeCsrf(data: unknown): string {
  if (!isPlainObject(data)) {
    throw drift('csrf payload is not an object');
  }

  return requireString(data, 'csrf_token');
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

export function createAuthApi(options: AuthApiOptions): AuthApi {
  const { readCsrfToken } = options;
  const caller = createCaller(options);

  const bootstrapCsrf = async (): Promise<string | undefined> => {
    try {
      return await caller.call('GET', '/auth/csrf', undefined, decodeCsrf, {
        credentials: 'include',
      });
    } catch {
      return undefined;
    }
  };

  const toHeaders = (token: string | undefined): Readonly<Record<string, string>> =>
    token === undefined ? {} : { 'X-CSRF-Token': token };

  const withCsrf = <T>(
    send: (headers: Readonly<Record<string, string>>) => Promise<T>,
  ): Promise<T> => {
    const token = readCsrfToken();
    return token === undefined
      ? bootstrapCsrf().then((boot) => send(toHeaders(boot)))
      : send(toHeaders(token));
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
      withCsrf((headers) =>
        caller.call('POST', '/auth/refresh', undefined, decodeSession, {
          credentials: 'include',
          headers,
        }),
      ),
    logout: () =>
      withCsrf((headers) =>
        caller.callNoContent('POST', '/auth/logout', undefined, {
          credentials: 'include',
          headers,
        }),
      ),
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
