/**
 * The in-memory session (WAS-01 … WAS-24, R3, WCS-01 … WCS-06, WCS-19 … WCS-21).
 *
 * The session holds the short-lived access token **in memory only** — it is
 * never persisted — plus a session-version counter and a `restoring |
 * unauthenticated | authenticating | pendingCode | authenticated` state. It
 * emits immutable snapshots for `useSyncExternalStore` (the F5 store pattern,
 * no state library) and implements the `SessionDelegate` the authenticated
 * transport drives.
 *
 * A credential operation answers either a session or a code step, and the
 * session follows the answer: a session establishes the session exactly as
 * before, a code step establishes nothing at all and holds the challenge handle
 * privately beside the access token (WCS-01, WCS-02, R4). The handle never
 * reaches the snapshot, so no subscriber can read, persist or leak it.
 *
 * `refresh()` is single-flight so every concurrent `401` shares one rotation, and
 * `logout()` is single-flight so a repeated sign-out asks the server once. The
 * session-local data is swept at a **definitive** boundary only — `logout()` and
 * `onUnauthorized()` — never on the transient restore fallback (WAS-05, WAS-21).
 */

import type { AuthApi, LoginInput, RegisterInput, SignInOutcome } from '../api/auth_api';
import { DomainFailure } from '../api/errors';
import type { SessionDelegate } from '../api/authenticated_transport';

/** The session states the gate renders (WAS-03, WAS-08, WCS-02). */
export type SessionState =
  'restoring' | 'unauthenticated' | 'authenticating' | 'pendingCode' | 'authenticated';

/**
 * The pending code step the snapshot projects (WCS-02, WCS-19, WCS-20).
 *
 * It carries the origin, the address the code was sent to, the username the
 * person typed and the advertised lifetime and window. It never carries the
 * password and never carries the handle.
 */
export interface PendingCodeStep {
  /** Which operation started the step; drives the copy and the restored form (WCS-21). */
  readonly origin: 'register' | 'login';
  /** The address the code was sent to; the surface names it (WCS-19). */
  readonly email: string;
  /** The registration's username, restored with the form (WCS-21). */
  readonly username: string | null;
  /** The advertised lifetime, in seconds; the surface states it (WCS-20). */
  readonly expiresIn: number;
  /** The advertised resend window, in seconds; the surface counts it down (WCS-07). */
  readonly resendAfter: number;
}

/**
 * The values the credential form comes back with after an abandoned step
 * (WCS-21).
 *
 * They are the identifiers the person typed — never the password — and they
 * live in memory for the life of the tab.
 */
export interface RestoredForm {
  /** The account mode the step started with. */
  readonly origin: 'register' | 'login';
  /** The address the person typed. */
  readonly email: string;
  /** The username the person typed, for a registration step. */
  readonly username: string | null;
}

/**
 * A one-shot notice the surface shows beside the form (D1, D8).
 *
 * `deferred` follows a registration whose account exists but whose code was not
 * queued; `verified` follows a correct code and is shown once as a toast.
 */
export type SessionNotice = 'deferred' | 'verified';

/** The immutable session snapshot a subscriber reads (R3, WCS-02). */
export interface SessionSnapshot {
  readonly accessToken: string | null;
  readonly version: number;
  readonly state: SessionState;
  /** The pending step; non-null exactly while the state is `pendingCode`. */
  readonly pending: PendingCodeStep | null;
  /** The values the form returns with after an abandoned step (WCS-21). */
  readonly restored: RestoredForm | null;
  /** The notice to show, or `null` when none applies. */
  readonly notice: SessionNotice | null;
}

/** Construction options for {@link createAuthSession}. */
export interface AuthSessionOptions {
  /** The session and challenge operations the session calls (R2). */
  readonly authApi: AuthApi;
  /** Sweeps the session-local data at a definitive boundary; best-effort (WAS-21). */
  readonly clearSessionData?: () => Promise<void>;
  /** Initial state, for an injected test session; defaults to `restoring` (WAS-03). */
  readonly initialState?: SessionState;
  /** Initial access token, for an injected test session; defaults to `null`. */
  readonly initialAccessToken?: string | null;
}

/** The session the gate, the screens and the transport read (R3, WCS-02). */
export interface AuthSession extends SessionDelegate {
  getSnapshot(): SessionSnapshot;
  subscribe(listener: () => void): () => void;
  /** Runs the one cold-start silent refresh before deciding the session state (WAS-03). */
  restore(): Promise<void>;
  login(input: LoginInput): Promise<void>;
  register(input: RegisterInput): Promise<void>;
  /** Verifies the entered code and establishes the session (WCS-03). */
  submitCode(code: string): Promise<void>;
  /** Asks for the code again, without rotating the handle or the code (WCS-08). */
  resendCode(): Promise<void>;
  /** Abandons the step; issues no request (WCS-06). */
  cancelCodeStep(): void;
  /** Clears the notice once the person has seen it. */
  dismissNotice(): void;
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
      // A transient failure keeps the session-local data and shows the form (WAS-05).
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
        // The account exists unverified and no code is in flight; the next
        // sign-in starts the verification step (D1, D2).
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

      // A code step establishes nothing: no token is held and the version does
      // not move, because no session began (WCS-01, WCS-02).
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

    // An answer for a step that was abandoned or superseded is dropped: it
    // establishes no session and overwrites nothing (WCS-06, R10).
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
    // The handle and the code go; the values the person typed stay, so the
    // credential form comes back with the address and the account mode
    // (WCS-06, WCS-21).
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
      } catch {
        // Sign-out is best-effort: a failed logout never blocks the teardown (WAS-19).
      }
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
