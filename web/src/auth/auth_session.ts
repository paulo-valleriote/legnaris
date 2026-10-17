/**
 * The in-memory session (WAS-01 … WAS-24, R3).
 *
 * The session holds the short-lived access token **in memory only** — it is
 * never persisted — plus a session-version counter and a `restoring |
 * unauthenticated | authenticating | authenticated` state. It emits immutable
 * snapshots for `useSyncExternalStore` (the F5 store pattern, no state library)
 * and implements the `SessionDelegate` the authenticated transport drives.
 *
 * `refresh()` is single-flight so every concurrent `401` shares one rotation, and
 * `logout()` is single-flight so a repeated sign-out asks the server once. The
 * session-local data is swept at a **definitive** boundary only — `logout()` and
 * `onUnauthorized()` — never on the transient restore fallback (WAS-05, WAS-21).
 */

import type { AuthApi, LoginInput, RegisterInput } from '../api/auth_api';
import { DomainFailure } from '../api/errors';
import type { SessionDelegate } from '../api/authenticated_transport';

/** The four session states the gate renders (WAS-03, WAS-08). */
export type SessionState = 'restoring' | 'unauthenticated' | 'authenticating' | 'authenticated';

/** The immutable session snapshot a subscriber reads (R3). */
export interface SessionSnapshot {
  readonly accessToken: string | null;
  readonly version: number;
  readonly state: SessionState;
}

/** Construction options for {@link createAuthSession}. */
export interface AuthSessionOptions {
  /** The four session operations the session calls (R2). */
  readonly authApi: AuthApi;
  /** Sweeps the session-local data at a definitive boundary; best-effort (WAS-21). */
  readonly clearSessionData?: () => Promise<void>;
  /** Initial state, for an injected test session; defaults to `restoring` (WAS-03). */
  readonly initialState?: SessionState;
  /** Initial access token, for an injected test session; defaults to `null`. */
  readonly initialAccessToken?: string | null;
}

/** The session the gate, the screens and the transport read (R3). */
export interface AuthSession extends SessionDelegate {
  getSnapshot(): SessionSnapshot;
  subscribe(listener: () => void): () => void;
  /** Runs the one cold-start silent refresh before deciding the session state (WAS-03). */
  restore(): Promise<void>;
  login(input: LoginInput): Promise<void>;
  register(input: RegisterInput): Promise<void>;
  logout(): Promise<void>;
}

function isDefinitiveRejection(error: unknown): boolean {
  return error instanceof DomainFailure && error.kind === 'unauthorized';
}

/** Builds the session over the auth operations and the local-data sweep (R3). */
export function createAuthSession(options: AuthSessionOptions): AuthSession {
  const { authApi } = options;
  const clearSessionData = options.clearSessionData ?? (() => Promise.resolve());

  let state: SessionState = options.initialState ?? 'restoring';
  let accessToken: string | null = options.initialAccessToken ?? null;
  let version = 0;
  let refreshInFlight: Promise<string | null> | undefined;
  let logoutInFlight: Promise<void> | undefined;
  const listeners = new Set<() => void>();
  let snapshot: SessionSnapshot = Object.freeze({ accessToken, version, state });

  function commit(next: {
    readonly state?: SessionState;
    readonly accessToken?: string | null;
    readonly version?: number;
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
    snapshot = Object.freeze({ accessToken, version, state });
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
      // A transient failure keeps the session-local data and shows the form (WAS-05).
      commit({ state: 'unauthenticated' });
    }
  }

  async function authenticate(apply: () => Promise<{ accessToken: string }>): Promise<void> {
    commit({ state: 'authenticating' });

    try {
      const body = await apply();
      commit({ version: version + 1, accessToken: body.accessToken, state: 'authenticated' });
    } catch (error) {
      commit({ state: 'unauthenticated' });
      throw error;
    }
  }

  function logout(): Promise<void> {
    if (logoutInFlight !== undefined) {
      return logoutInFlight;
    }

    const pending = (async (): Promise<void> => {
      try {
        await authApi.logout();
      } catch {
        // Sign-out is best-effort: a failed logout never blocks the teardown (WAS-19).
      }
      commit({ version: version + 1, accessToken: null, state: 'unauthenticated' });
      await clearSessionData();
    })();

    const guarded = pending.finally(() => {
      logoutInFlight = undefined;
    });
    logoutInFlight = guarded;
    return guarded;
  }

  async function onUnauthorized(): Promise<void> {
    commit({ version: version + 1, accessToken: null, state: 'unauthenticated' });
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
    login: (input) => authenticate(() => authApi.login(input)),
    register: (input) => authenticate(() => authApi.register(input)),
    logout,
  };
}
