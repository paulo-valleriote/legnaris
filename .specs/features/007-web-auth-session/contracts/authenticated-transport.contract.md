# Contract: Authenticated transport (Bearer, silent refresh, one replay)

**Requirements**: WAS-11, WAS-12, WAS-13, WAS-14, WAS-15, WAS-16, WAS-17 | **Backing document**:
[data-model.md `AuthenticatedTransport` / `AuthSession`](../data-model.md) and
[research.md `R3` / `R4`](../research.md).

**Modules**: `web/src/api/authenticated_transport.ts` (the decorator), `web/src/auth/auth_session.ts`
(the session the decorator drives), `web/src/api/transport.ts` (the seam it wraps).

## Interface

```ts
export interface SessionDelegate {
  /** The in-memory access token, or null when no session is held (WAS-11). */
  currentAccessToken(): string | null;
  /** Exchanges the cookie for a fresh token; null on a definitive 401, throws on a transient failure (WAS-12, WAS-14, WAS-15). */
  refresh(): Promise<string | null>;
  /** Forced logout: drops the token, clears the session-local data, ends the session (WAS-14). */
  onUnauthorized(): Promise<void>;
  /** The session generation, bumped only at a boundary (WAS-17). */
  sessionVersion(): number;
}

export function createAuthenticatedTransport(input: {
  inner: Transport;
  session: SessionDelegate;
}): Transport;
```

## Rules

1. **Bearers, not cookies, on the domain routes** (WAS-11, WAS-16). A request gets `Authorization:
   Bearer <access>` when a token is held and the path is protected; `/health` and every `/auth/*`
   path get no Bearer and never trigger a refresh. The decorator is the one place the token is
   attached, so `/sheets` and `/templates` carry no cookie and have no CSRF concern.
2. **One refresh, one replay** (WAS-12). A protected `401` triggers exactly one silent refresh and,
   on success, replays that request exactly once with the refreshed Bearer. The replay is a shallow
   clone of the original request with the header replaced, because the body is a string.
3. **Single-flight** (WAS-13). Concurrent `401`s share one in-flight refresh future, set
   synchronously before the first `await`, so the family rotates once rather than once per refused
   call; the future clears on completion so a later lapse starts a fresh refresh.
4. **Definitive rejection ends the session** (WAS-14). A refresh that returns `null` (a `401`) and an
   unchanged version calls `onUnauthorized()` and returns the **original** `401` response, so the
   refused call's error still reaches its caller and the in-memory token is dropped.
5. **Transient failure survives** (WAS-15). A refresh that throws (offline, `5xx`, unreadable) is
   rethrown; the session and its session-local data are untouched and the failure surfaces as
   transient.
6. **Finished sessions are inert** (WAS-17). The version is captured before sending; a `401` or a
   refresh result whose version changed in between neither ends nor overwrites the current session —
   no refresh, no replay, no forced logout.

## Obligations

1. **WAS-11** — `Bearer` on `/sheets` and `/templates`; no cookie sent.
2. **WAS-12** — one refresh and one replay per refused call.
3. **WAS-13** — one refresh for a burst of concurrent `401`s.
4. **WAS-14** — a rejected refresh forces logout and surfaces the original refusal.
5. **WAS-15** — a transient refresh keeps the session.
6. **WAS-16** — `/health` and `/auth/*` are never authenticated or refreshed.
7. **WAS-17** — a late response from a finished session changes nothing.

## Test obligations

- **Happy path**: a protected call with a held token carries `Bearer`; a `401` produces one refresh
  and one replayed request that succeeds.
- **Edge**: two concurrent `401`s trigger one refresh; a replayed request that is again `401` is not
  replayed twice; an absent token sends no `Bearer`; a replay with no body works.
- **Error**: a rejected refresh calls `onUnauthorized()` once and returns the original `401`; a
  transient refresh rethrows and leaves the token and the session-local data intact; a `401` or
  refresh belonging to a superseded version is dropped.
- **Boundary**: `/health` and each `/auth/*` path carry no `Bearer` and start no refresh.

## Traceability

- Attachment: WAS-11, WAS-16.
- Refresh lifecycle: WAS-12, WAS-13, WAS-14, WAS-15.
- Version guard: WAS-17.
