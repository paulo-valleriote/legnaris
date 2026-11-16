import type { AuthApi, LoginInput, RegisterInput, SignInOutcome } from '../api/auth_api';
import { DomainFailure } from '../api/errors';
import type { SessionDelegate } from '../api/authenticated_transport';

export type SessionState =
  'restoring' | 'unauthenticated' | 'authenticating' | 'pendingCode' | 'authenticated';

export interface PendingCodeStep {
  readonly origin: 'register' | 'login';
  readonly email: string;
  readonly username: string | null;
  readonly expiresIn: number;
  readonly resendAfter: number;
}

export interface RestoredForm {
  readonly origin: 'register' | 'login';
  readonly email: string;
  readonly username: string | null;
}

export type SessionNotice = 'deferred' | 'verified';

export interface SessionSnapshot {
  readonly accessToken: string | null;
  readonly version: number;
  readonly state: SessionState;
  readonly pending: PendingCodeStep | null;
  readonly restored: RestoredForm | null;
  readonly notice: SessionNotice | null;
}

export interface AuthSessionOptions {
  readonly authApi: AuthApi;
  readonly clearSessionData?: () => Promise<void>;
  readonly initialState?: SessionState;
  readonly initialAccessToken?: string | null;
}

export interface AuthSession extends SessionDelegate {
  getSnapshot(): SessionSnapshot;
  subscribe(listener: () => void): () => void;
  restore(): Promise<void>;
  login(input: LoginInput): Promise<void>;
  register(input: RegisterInput): Promise<void>;
  submitCode(code: string): Promise<void>;
  resendCode(): Promise<void>;
  cancelCodeStep(): void;
  dismissNotice(): void;
  logout(): Promise<void>;
}

function isDefinitiveRejection(error: unknown): boolean {
  return error instanceof DomainFailure && error.kind === 'unauthorized';
}

export function createAuthSession(options: AuthSessionOptions): AuthSession {
  const { authApi } = options;
  const clearSessionData = options.clearSessionData ?? (() => Promise.resolve());

  let state: SessionState = options.initialState ?? 'restoring';
  let accessToken: string | null = options.initialAccessToken ?? null;
  let version = 0;
  let refreshInFlight: Promise<string | null> | undefined;
  let logoutInFlight: Promise<void> | undefined;
  let resendInFlight: Promise<void> | undefined;
  let pendingStep: PendingCodeStep | null = null;
  let pendingHandle: string | null = null;
  let restoredForm: RestoredForm | null = null;
  let notice: SessionNotice | null = null;
  const listeners = new Set<() => void>();
  let snapshot: SessionSnapshot = Object.freeze({
    accessToken,
    version,
    state,
    pending: null,
    restored: null,
    notice: null,
  });

  function commit(next: {
    readonly state?: SessionState;
    readonly accessToken?: string | null;
    readonly version?: number;
    readonly pending?: PendingCodeStep | null;
    readonly restored?: RestoredForm | null;
    readonly notice?: SessionNotice | null;
  }): void {
    if (next.state !== undefined) {
      state = next.state;
    }
    if (next.accessToken !== undefined) {
      accessToken = next.accessToken;
    }
    if (next.version !== undefined) {
      version = next.version;
    }
    if (next.pending !== undefined) {
      pendingStep = next.pending;
    }
    if (next.restored !== undefined) {
      restoredForm = next.restored;
    }
    if (next.notice !== undefined) {
      notice = next.notice;
    }
    snapshot = Object.freeze({
      accessToken,
      version,
      state,
      pending: pendingStep,
      restored: restoredForm,
      notice,
    });
    for (const listener of listeners) {
      listener();
    }
  }

  function refresh(): Promise<string | null> {
    if (refreshInFlight !== undefined) {
      return refreshInFlight;
    }

    const pending = (async (): Promise<string | null> => {
      try {
        const body = await authApi.refresh();
        commit({ accessToken: body.accessToken });
        return body.accessToken;
      } catch (error) {
        if (isDefinitiveRejection(error)) {
          return null;
        }
        throw error;
      } finally {
        refreshInFlight = undefined;
      }
    })();

    refreshInFlight = pending;
    return pending;
  }

  async function restore(): Promise<void> {
    if (state === 'authenticated' && accessToken !== null) {
      return;
    }

    commit({ state: 'restoring' });

    try {
      const token = await refresh();
      commit({ state: token === null ? 'unauthenticated' : 'authenticated' });
    } catch {
      commit({ state: 'unauthenticated' });
    }
  }

  async function authenticate(
    origin: 'register' | 'login',
    input: { readonly email: string; readonly username?: string },
    apply: () => Promise<SignInOutcome>,
  ): Promise<void> {
    commit({ state: 'authenticating', notice: null });

    try {
      const outcome = await apply();

      if (outcome.kind === 'session') {
        pendingHandle = null;
        commit({
          version: version + 1,
          accessToken: outcome.session.accessToken,
          state: 'authenticated',
          pending: null,
          restored: null,
        });
        return;
      }

      if (outcome.kind === 'deferred') {
        pendingHandle = null;
        commit({
          state: 'unauthenticated',
          accessToken: null,
          pending: null,
          restored: { origin, email: input.email, username: input.username ?? null },
          notice: 'deferred',
        });
        return;
      }

      pendingHandle = outcome.challenge.challengeId;
      commit({
        state: 'pendingCode',
        accessToken: null,
        pending: {
          origin,
          email: input.email,
          username: input.username ?? null,
          expiresIn: outcome.challenge.expiresIn,
          resendAfter: outcome.challenge.resendAfter,
        },
        restored: { origin, email: input.email, username: input.username ?? null },
      });
    } catch (error) {
      pendingHandle = null;
      commit({ state: 'unauthenticated', pending: null });
      throw error;
    }
  }

  async function submitCode(code: string): Promise<void> {
    const handle = pendingHandle;

    if (handle === null) {
      throw new Error('no code step is pending');
    }

    const body = await authApi.verifyChallenge({ challengeId: handle, code });

    if (pendingHandle !== handle) {
      return;
    }

    pendingHandle = null;
    commit({
      version: version + 1,
      accessToken: body.accessToken,
      state: 'authenticated',
      pending: null,
      restored: null,
      notice: 'verified',
    });
  }

  function resendCode(): Promise<void> {
    if (resendInFlight !== undefined) {
      return resendInFlight;
    }

    const handle = pendingHandle;

    if (handle === null) {
      return Promise.reject(new Error('no code step is pending'));
    }

    const pending = (async (): Promise<void> => {
      try {
        const result = await authApi.resendChallenge({ challengeId: handle });

        if (pendingHandle !== handle || pendingStep === null) {
          return;
        }

        commit({ pending: { ...pendingStep, resendAfter: result.resendAfter } });
      } finally {
        resendInFlight = undefined;
      }
    })();

    resendInFlight = pending;
    return pending;
  }

  function dismissNotice(): void {
    commit({ notice: null });
  }

  function cancelCodeStep(): void {
    pendingHandle = null;
    commit({ state: 'unauthenticated', pending: null });
  }

  function logout(): Promise<void> {
    if (logoutInFlight !== undefined) {
      return logoutInFlight;
    }

    const pending = (async (): Promise<void> => {
      try {
        await authApi.logout();
      } catch {}
      commit({
        version: version + 1,
        accessToken: null,
        state: 'unauthenticated',
        restored: null,
        notice: null,
      });
      await clearSessionData();
    })();

    const guarded = pending.finally(() => {
      logoutInFlight = undefined;
    });
    logoutInFlight = guarded;
    return guarded;
  }

  async function onUnauthorized(): Promise<void> {
    commit({ version: version + 1, accessToken: null, state: 'unauthenticated', notice: null });
    await clearSessionData();
  }

  return {
    getSnapshot: () => snapshot,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    currentAccessToken: () => accessToken,
    sessionVersion: () => version,
    refresh,
    onUnauthorized,
    restore,
    login: (input) => authenticate('login', input, () => authApi.login(input)),
    register: (input) => authenticate('register', input, () => authApi.register(input)),
    submitCode,
    resendCode,
    cancelCodeStep,
    dismissNotice,
    logout,
  };
}
