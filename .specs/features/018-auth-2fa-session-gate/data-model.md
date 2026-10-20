# Phase 1 Data Model: Sign-in Hardening — The Code Step Replaces the Immediate Session, and Unverified Accounts Cannot Renew

**Feature**: `.specs/features/018-auth-2fa-session-gate/` | **Date**: 2026-10-20

This feature adds **no stored entity, no collection and no index**. Everything below is either a
boot-time setting, a value that travels through the Go service boundary, or an existing stored field
that becomes readable. The stored shapes of the preceding features (`users`, `auth_challenges`,
`refresh_tokens`, `email_outbox`) are unchanged.

---

## 1. Rollout gate (configuration, not persisted)

| Attribute | Value |
|---|---|
| Name | `AUTH_REQUIRE_EMAIL_VERIFICATION` |
| Type | boolean (`strconv.ParseBool`: `1/0/t/f/true/false/TRUE/FALSE/True/False`) |
| Default | `false` (disabled) |
| Required | **No** — absent from `env.RequiredKeys()` (`TFA-07`, `TFA-16`) |
| Reader | new `lookupBoolEnv(key, default)` in `shared/env/env.go` |
| Invalid value | structured warning + the default (mirrors `lookupDurationEnv`/`lookupIntEnv`) |
| Consumer | `auth.Config.RequireEmailVerification`, set at the composition root (`internal/app/bootstrap.go`) |

**Why it is not a stored entity**: the gate selects which contract the running process serves. It is
read once at boot, carried by the service, and never written to a document (`TFA-07`). Enabling and
disabling it therefore rewrites nothing (`SC-007`).

**Behaviour by state** (the whole table, exhaustive):

| Operation | Gate disabled | Gate enabled |
|---|---|---|
| `Register` (valid input) | account created + **session**, 201, both cookies | account created (no timestamp) + **`verify_email` challenge**, 201, no cookie |
| `Register` (duplicate address) | 409, no session | 409, no challenge, no send |
| `Register` (invalid input) | typed `INVALID_REQUEST`, no session | unchanged |
| `Login` (valid credentials) | **session**, 200, both cookies | **`login_otp` challenge**, 202, no cookie |
| `Login` (unknown address / wrong password) | uniform `UNAUTHORIZED`, 401 | unchanged, plus the equalized password work (`TFA-05`) |
| `Challenge/verify` | unchanged (`OTP-07`): consume, record timestamp, session | unchanged |
| `Challenge/resend` | unchanged | unchanged |
| `Refresh` (valid credential, account verified) | rotation, 200 | rotation, 200 |
| `Refresh` (valid credential, account unverified) | rotation, 200 | **`UNAUTHORIZED`, 401, family revoked** (`TFA-06`) |
| `Refresh` (missing account, reuse, expired) | unchanged | unchanged |
| `GET /auth/me` | available and truthful | available and truthful |

## 2. Start outcome (service boundary value)

The discriminated result `internal/auth.Register` and `internal/auth.Login` hand back. It exists so
the *service* decides whether a session was issued and the *controller* only projects the decision
(`R1`).

| Field | Type | Present when |
|---|---|---|
| `Session` | `*Session` (existing shape: access token, expiry, refresh credential, CSRF, user id) | the gate is disabled |
| `Challenge` | `*ChallengeIssued` (existing shape: challenge identity, lifetime, resend window) | the gate is enabled |

**Invariants** (each is an assertion in the suite):

1. Exactly one half is non-nil on success; never both, never neither.
2. `Challenge` is non-nil only if the credential proof for that request succeeded (`TFA-03`).
3. `Session` is non-nil only when the gate is disabled or when it comes from `VerifyChallenge`.
4. The legacy half is byte-identical to today's value when the gate is disabled (`TFA-12`).

**Lifecycle of a gated registration/sign-in**:

```text
request ──▶ validate + prove credential ──▶ gate?
   │                                          ├─ off ──▶ signIn ──▶ Session (cookies set)
   │                                          └─ on  ──▶ IssueChallenge ──▶ Challenge (no cookies)
   │                                                         │
   │                                                         └─ cap/store failure ──▶ typed error, no credential
   └─ proof failed ──▶ uniform 401, nothing started
```

## 3. Verification state (derived; read-only in this feature)

| Attribute | Source | Wire |
|---|---|---|
| Stored instant | `user.User.EmailVerifiedAt *time.Time` (`bson:"email_verified_at,omitempty"`) | **never** |
| Derived state | `EmailVerifiedAt != nil` | `user.UserDTO.EmailVerified bool` (`json:"email_verified"`) |

**Rules**:

- The instant is written only by the existing idempotent `user.Repository.MarkEmailVerified`
  (`PRQ-13`), which `VerifyChallenge` already calls; this feature adds no write path (`TFA-11`).
- The derived boolean is computed in `ToDTO` so one place defines it (`R4`, `TFA-08`).
- No response ever carries the instant (`TFA-08`, `SC-006`); `PRQ-12`/`PRQ-14` are superseded for the
  read path only (`TFA-09`).

## 4. Identity read (wire value)

`GET /auth/me` under a valid access credential answers `200` with the existing success envelope
carrying the user DTO:

| Field | Type | Notes |
|---|---|---|
| `id` | string | canonical UUID v7, unchanged |
| `email` | string | canonical lowercase address (`PRQ-08`) |
| `username` | string | unchanged |
| `email_verified` | boolean | **new** — derived as in §3 |
| `created_at` | timestamp | unchanged, already on the DTO |

**Failure paths** (typed, mapped, `TFA-10`):

| Case | Code | Status |
|---|---|---|
| No credential / malformed / expired / forged | `UNAUTHORIZED` | 401 (existing middleware outcome) |
| Credential valid, account no longer exists | `UNAUTHORIZED` | 401 (`R4`) |
| Account-store failure | `INTERNAL_ERROR` | 500 |

## 5. Renewal decision (state transition on the existing refresh session)

`Refresh` already loads the account (`PRQ-05`). The gate adds one predicate between the load and the
first issuance:

```text
FetchByHash(credential) ──▶ CSRF verify ──▶ revoked? ──▶ expired?
   └─▶ FetchByID(account)
          ├─ not found ──▶ revoke family ──▶ 401 (PRQ-05, unchanged)
          └─ found ──▶ gate on AND EmailVerifiedAt == nil ?
                 ├─ yes ──▶ revoke family ──▶ 401 UNAUTHORIZED, 0 credentials issued
                 └─ no  ──▶ issue access + rotate refresh ──▶ 200 (unchanged)
```

**Ordering rule**: the predicate runs **after** the account load and **after** the credential rules
(CSRF, revoked, expired) and **before** any issuance, so a refusal can never leave a minted successor
behind (`TFA-06`, `SC-003`). A missing account still decides first (`PRQ-05`).

## 6. Superseded rules (corpus bookkeeping)

| Superseded | In | Superseded by | Extent |
|---|---|---|---|
| `AUTH-01` (login establishes the session) | 006 | `TFA-02` | gate-enabled behaviour only |
| `AUTH-02` (registration establishes the session) | 006 | `TFA-01` | gate-enabled behaviour only |
| `WAS-02` (registration authenticates, client premise) | 007 | `TFA-02` | deferred to stage 5 for the client code |
| `PRQ-12` (no verification timestamp on the wire) | 015 | `TFA-09` | the read path only; the write path is unchanged |
| `PRQ-14` (the timestamp is not reachable over HTTP) | 015 | `TFA-08` | the read path only; still not a timestamp on the wire |

The IDs remain defined; each gets a one-line pointer in its own spec (`R7`, `TFA-14`). Nothing is
renumbered or deleted, so every existing citation still resolves.

## 7. What is deliberately absent

- **No `email_verified` on the challenge or refresh records** — the state is an account fact.
- **No cohort / percentage / per-account flag** — one global switch (spec Clarifications,
  2026-10-20); a staged activation is stage 7's concern.
- **No migration, backfill or cleanup command** — every existing account keeps `EmailVerifiedAt` absent
  (`TFA-07`, `SC-007`).
- **No new outbox, broker or provider field** — this feature never writes an intention itself; it calls
  the challenge domain, which does.
