# Data Model — The Code Step in the Client

**Feature**: `.specs/features/019-web-auth-code-step/` | **Requirements**: `WCS-01` … `WCS-18`

The feature defines no persisted data. Everything below is either a wire shape the client decodes or an
in-memory shape the session holds for the life of the tab. No store, index, collection or migration is
involved — the server's collections belong to features `015`–`018`.

## 1. The wire shapes

### Sign-in answer (registration `201` / sign-in `200`/`202`)

One of exactly two bodies, discriminated by the keys present (`R1`):

```ts
// A session body — the gate-off answer (already consumed today).
{ access_token: string; token_type: string; expires_in: number }

// A code step — the gate-on answer (new to the client).
{ challenge_id: string; expires_in: number; resend_after: number }
```

| Operation | Gate off | Gate on |
|---|---|---|
| `POST /auth/register` | `201` + session body + both cookies | `201` + code step, **no cookie** |
| `POST /auth/login` | `200` + session body + both cookies | `202` + code step, **no cookie** |

Registration answers `201` in both states, so the client cannot use the status to tell them apart
(`WCS-01`, `R1`).

### Verification answer (`POST /auth/challenge/verify`, `200`)

```ts
{ access_token: string; token_type: string; expires_in: number }   // plus the refresh and CSRF cookies
```

Refused uniformly with `OTP_INVALID` / `401` for every failure kind — unknown, malformed, expired,
consumed, wrong or exhausted (`OTP-08`, `OTP-04`) — or `RATE_LIMITED` / `429`, `EMAIL_UNAVAILABLE` /
`503` (reserved, not produced here), `INTERNAL_ERROR` / `500`.

### Resend answer (`POST /auth/challenge/resend`)

```ts
{ resend_after: number }    // 202, identical whether a send was recorded, suppressed, or the handle unknown
```

`503` + `EMAIL_UNAVAILABLE` when the daily allowance is spent; `429`, `500` as above. The handle and the
code are never rotated by a resend (`OTP-10`, `OTP-11`).

### Failure envelope (unchanged)

```ts
{ error: { code: string; message: string } }
```

## 2. The decoded client types

### `SessionBody` (existing, unchanged)

```ts
interface SessionBody { readonly accessToken: string; readonly tokenType: string; readonly expiresIn: number }
```

### `ChallengeStep` (new)

```ts
interface ChallengeStep {
  readonly challengeId: string;   // the opaque handle the client presents back; never parsed
  readonly expiresIn: number;     // seconds the code is claimed valid for
  readonly resendAfter: number;   // seconds until a resend records a new send
}
```

### `SignInOutcome` (new, the discriminated answer)

```ts
type SignInOutcome =
  | { readonly kind: 'session';   readonly session: SessionBody }
  | { readonly kind: 'challenge'; readonly challenge: ChallengeStep };
```

**Invariants**

- Exactly one member is decoded. A payload carrying `access_token` **and** `challenge_id`, or neither,
  is a `DefensiveParseFailure` — never a silent pick (`WCS-01`, `R1`).
- The decoder reads the payload only; the HTTP status is not consulted for the discrimination.
- A `'challenge'` outcome carries no credential: decoding one writes nothing and holds nothing.

### `ResendResult` (new)

```ts
interface ResendResult { readonly resendAfter: number }
```

## 3. The session state

### `SessionState` (extended)

```ts
type SessionState =
  | 'restoring' | 'unauthenticated' | 'authenticating'
  | 'pendingCode'          // NEW: a code step is pending; no credential is held
  | 'authenticated';
```

### `PendingCodeStep` (new) — the snapshot projection of a pending step

```ts
interface PendingCodeStep {
  readonly origin: 'register' | 'login';   // which operation started the step; drives the copy
  readonly resendAfter: number;            // seeds the surface's local window
}
```

The **handle is not on the snapshot**. It stays in a module-private field beside the access token, and
the surface acts on the step only through the session's `submitCode` / `resendCode` / `cancelCodeStep`
methods (`WCS-02`, `R4`). There is therefore no second copy of the handle to keep in sync, and no
subscriber can read, persist or leak it.

### `SessionSnapshot` (extended)

```ts
interface SessionSnapshot {
  readonly accessToken: string | null;     // unchanged: memory-only
  readonly version: number;                // unchanged: bumps at session boundaries only
  readonly state: SessionState;
  readonly pending: PendingCodeStep | null; // NEW: non-null iff state === 'pendingCode'
}
```

**Invariants**

- `pending !== null` **iff** `state === 'pendingCode'`; both are cleared together.
- While `pending !== null`, `accessToken === null` and `version` is unchanged.
- The snapshot carries no handle, no code and no credential beyond the existing access token.
- A reload restores `restoring` and finds no pending step (`R2`, `WCS-02`).

## 4. State transitions

| From | Event | To | Side effects |
|---|---|---|---|
| `restoring` | silent refresh succeeds | `authenticated` | access token held (unchanged) |
| `restoring` | silent refresh refused / transient | `unauthenticated` | (unchanged) |
| `unauthenticated` | register / sign-in submitted | `authenticating` | (unchanged) |
| `authenticating` | answer is a **session** | `authenticated` | token held, `version + 1` (unchanged) |
| `authenticating` | answer is a **code step** | `pendingCode` | `pending = { origin, resendAfter }`, handle held privately, **no token, no version bump** |
| `authenticating` | refusal | `unauthenticated` | mapped copy shown by the surface (unchanged) |
| `pendingCode` | `submitCode` succeeds | `authenticated` | token held, `version + 1`, `pending = null` |
| `pendingCode` | `submitCode` refused | `pendingCode` | pending kept, copy shown, no token |
| `pendingCode` | `submitCode` answer for a **stale handle** | unchanged | answer discarded (`R10`) |
| `pendingCode` | `resendCode` answered | `pendingCode` | `pending.resendAfter` updated from the answer |
| `pendingCode` | `resendCode` refused (unavailable / rate limited) | `pendingCode` | pending kept, copy shown, action re-arms at the window's end (`R3`) |
| `pendingCode` | `cancelCodeStep` | `unauthenticated` | handle and entered code cleared, `pending = null`, no request |
| `pendingCode` | sign-out / forced logout | — | not reachable: no session is held while pending |

**Version semantics (unchanged definition)**: `version` increments only where a session is established
or ends — including the verification that establishes one — and never when a step is started, resends
or is abandoned. That keeps the existing `WAS-17` stale-result guard meaningful without redefining what
the counter counts (`R10`).

## 5. The failure taxonomy (extended)

```ts
type FailureKind =
  | 'clientFault' | 'invalidEdit' | 'renderMismatch' | 'notFound' | 'serverError' | 'offline'
  | 'unauthorized' | 'forbidden' | 'rateLimited' | 'unknown'
  | 'invalidCode'        // NEW: OTP_INVALID (401) — the uniform code refusal
  | 'emailUnavailable'   // NEW: EMAIL_UNAVAILABLE (503) — the send allowance is spent
  | 'conflict';          // NEW: CONFLICT (409) — an address that already has an account
```

| Code | Status | Kind | Emitted by |
|---|---|---|---|
| `OTP_INVALID` | 401 | `invalidCode` | `POST /auth/challenge/verify` |
| `EMAIL_UNAVAILABLE` | 503 | `emailUnavailable` | `POST /auth/register` (gate on), `POST /auth/challenge/resend` |
| `CONFLICT` | 409 | `conflict` | `POST /auth/register` — duplicate address only (`backend/internal/auth/service.go:207`) |
| any unmapped code | any | `unknown` | fallback, raw code preserved (`WCS-12`) |

The mapping is total over the codes the auth routes can answer: `INVALID_JSON`, `INVALID_REQUEST`,
`UNKNOWN_FIELD`, `UNAUTHORIZED`, `FORBIDDEN`, `CONFLICT`, `RATE_LIMITED`, `EMAIL_UNAVAILABLE`,
`OTP_INVALID`, `INTERNAL_ERROR`. The `conflict` copy names the address and offers sign-in, which is
truthful **only** because the server emits `CONFLICT` in exactly one place today; if another route
starts emitting it, the copy must be revisited (`R6`).

## 6. Catalog keys (new)

| Key | Purpose |
|---|---|
| `auth.codeTitle` | the code surface's heading |
| `auth.codeSubtitleRegister` | "confirm the address you registered" copy |
| `auth.codeSubtitleLogin` | "sign in with the code we emailed" copy |
| `auth.codeLabel` | the code field's label |
| `auth.codeSubmit` | the submit action |
| `auth.codeVerifying` | the in-flight submit label |
| `auth.codeResend` | the resend action |
| `auth.codeResendIn` | the countdown, formatted with the remaining seconds |
| `auth.codeResendSent` | the confirmation after a resend is recorded |
| `auth.codeStartOver` | the way back to the credential form |
| `api.error.invalidCode` | the uniform code refusal copy |
| `api.error.emailUnavailable` | the unavailable-send copy |
| `api.error.conflict` | the duplicate-address copy, offering sign-in |

Every key resolves through `src/l10n/strings.ts`; no surface authors a literal (`WCS-13`, `WCS-16`).

## 7. The operations (extended `AuthApi`)

```ts
interface AuthApi {
  register(input: RegisterInput): Promise<SignInOutcome>;   // was Promise<SessionBody>
  login(input: LoginInput): Promise<SignInOutcome>;         // was Promise<SessionBody>
  refresh(): Promise<SessionBody>;                          // unchanged
  logout(): Promise<void>;                                  // unchanged
  verifyChallenge(input: { challengeId: string; code: string }): Promise<SessionBody>;  // NEW
  resendChallenge(input: { challengeId: string }): Promise<ResendResult>;               // NEW
}
```

All six go through the shared caller; every one passes `credentials: 'include'`. `refresh` and `logout`
additionally echo `X-CSRF-Token`; the four others carry no CSRF header. None carries a Bearer header
and none can trigger a refresh (`WCS-15`, `R7`).

## 8. Boundary cases

| Case | Result |
|---|---|
| registration `201` with `access_token` | a `'session'` outcome; session established, no code surface (`WCS-01`) |
| registration `201` with `challenge_id` | a `'challenge'` outcome; no token, code surface (`WCS-01`) |
| payload with both keys, or with neither | `DefensiveParseFailure` — no outcome is guessed (`R1`) |
| `challenge_id` present but `resend_after` missing | `DefensiveParseFailure`; the surface is never rendered half-formed |
| verification `200` | session established, `version + 1`, pending cleared (`WCS-03`) |
| verification `401 OTP_INVALID` | pending kept, `invalidCode` copy, no token (`WCS-04`) |
| verification answer for an abandoned handle | discarded; the credential form stays, no token (`WCS-06`, `R10`) |
| resend `202` | window re-seeded from `resend_after`; handle and code unchanged (`WCS-08`, `WCS-10`) |
| resend `503 EMAIL_UNAVAILABLE` | pending kept, `emailUnavailable` copy, action re-arms at the window's end (`WCS-09`, `R3`) |
| registration `409 CONFLICT` | `conflict` copy naming the address and offering sign-in; credential form keeps its values (`WCS-11`) |
| a code with leading zeros submitted | sent exactly as typed (`WCS-03`) |
| reload while pending | `restoring` → `unauthenticated`; the credential form returns (`WCS-02`, `R2`) |
| unknown backend code | `unknown` kind, raw code preserved (`WCS-12`) |
