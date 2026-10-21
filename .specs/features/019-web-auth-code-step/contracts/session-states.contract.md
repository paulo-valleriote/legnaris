# Contract: The session states and the pending code step

**Feature**: `.specs/features/019-web-auth-code-step/` | **Requirement IDs**: `WCS-02`, `WCS-03`,
`WCS-04`, `WCS-06`, `WCS-08`, `WCS-10` | **Owner**: `web/src/auth/auth_session.ts`

## Purpose

Fix the state machine the gate renders and the three methods a surface uses to finish a step, so there
is exactly one answer to "is a step pending", the handle never leaves the session, and a stale answer
cannot establish a session for a step the person already left.

## Shape

```ts
type SessionState =
  | 'restoring' | 'unauthenticated' | 'authenticating'
  | 'pendingCode'
  | 'authenticated';

interface PendingCodeStep {
  readonly origin: 'register' | 'login';
  readonly resendAfter: number;
}

interface SessionSnapshot {
  readonly accessToken: string | null;
  readonly version: number;
  readonly state: SessionState;
  readonly pending: PendingCodeStep | null;
}

interface AuthSession extends SessionDelegate {
  getSnapshot(): SessionSnapshot;
  subscribe(listener: () => void): () => void;
  restore(): Promise<void>;
  login(input: LoginInput): Promise<void>;
  register(input: RegisterInput): Promise<void>;
  submitCode(code: string): Promise<void>;   // NEW
  resendCode(): Promise<void>;               // NEW
  cancelCodeStep(): void;                    // NEW
  logout(): Promise<void>;
}
```

The challenge handle is a **module-private** field beside the access token; it is never placed on the
snapshot and never returned by a method (`R4`).

## Rules

- `register`/`login` enter `authenticating`, call the operation, then branch on the outcome: a
  `'session'` outcome commits `authenticated` with the token and `version + 1` exactly as today; a
  `'challenge'` outcome commits `pendingCode` with `pending = { origin, resendAfter }`, holds the
  handle privately, and leaves the token `null` and `version` unchanged.
- `submitCode` calls `verifyChallenge` with the held handle; on success it commits `authenticated`
  (token, `version + 1`, `pending = null`, handle cleared); on failure it rethrows to the caller and
  leaves the pending state untouched so the surface can show the mapped copy.
- `submitCode` captures the handle it is verifying and applies the answer only while the session's
  pending handle is still that same handle; an answer for an abandoned or superseded step is dropped
  (`R10`).
- `resendCode` is single-flight: a call while one is in flight returns the same promise, so a double
  activation issues one request (`WCS-08`). On success it re-seeds `pending.resendAfter` from the
  answer; on failure it rethrows and leaves the pending state untouched, because a resend never rotates
  the handle or the code (`WCS-10`).
- `cancelCodeStep` clears the handle and the entered code (the latter lives in the surface), commits
  `unauthenticated`, and issues no request (`WCS-06`).
- `logout` and `onUnauthorized` are unchanged and unreachable while pending, because no session is held.

## Invariants

1. **One pending answer.** `pending !== null` iff `state === 'pendingCode'`; they are set and cleared
   together.
2. **No credential while pending.** While a step is pending the snapshot's `accessToken` is `null`, the
   `version` is unchanged and no cookie has been issued by the client.
3. **The handle is private.** No snapshot, method return or thrown value carries the handle.
4. **Version semantics unchanged.** `version` bumps only where a session is established or ends —
   including the verification that establishes one — and never when a step starts, resends or is
   abandoned; this keeps the delivered `WAS-17` guard meaningful.
5. **Abandonment is total.** After `cancelCodeStep`, no later answer — verification or resend — can
   move the state away from `unauthenticated` or write a token.
6. **Memory-only.** Nothing about the pending step survives a reload; `restore()` begins at
   `restoring` with `pending === null`.

## Boundary cases

| Case | Result |
|---|---|
| register answered with a code step | `pendingCode`, no token, `version` unchanged (`WCS-02`) |
| register answered with a session | `authenticated`, token held, `version + 1` (`WCS-01`) |
| verification succeeds | `authenticated`, token held, `version + 1`, pending cleared (`WCS-03`) |
| verification refused | `pendingCode` kept, failure rethrown, no token (`WCS-04`) |
| answer arrives after `cancelCodeStep` | dropped; state stays `unauthenticated`, no token (`WCS-06`) |
| a second step replaces the first | an answer for the first handle is dropped (`R10`) |
| `resendCode` called twice in flight | one request; the second caller shares the promise (`WCS-08`) |
| `resendCode` refused | pending kept, failure rethrown, handle and code unchanged (`WCS-10`) |
| reload while pending | `restoring`; no pending step (`WCS-02`) |
| `logout` while pending | not reachable; no session to end |

## Test evidence

- `web/src/test/auth_session.test.ts`: the outcome branch under both gate states; the pending snapshot
  invariant; submit success/failure; the stale-handle guard; resend single-flight and window re-seed;
  abandonment clearing; the version counter's unchanged semantics.
- `web/src/test/session_boundary.test.ts`: the boundary rules still hold with the code step in front of
  them (no credential written, no session-local data touched by a pending step).
