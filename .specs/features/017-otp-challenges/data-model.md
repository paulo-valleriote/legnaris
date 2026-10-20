# Phase 1 Data Model: Email Code Challenges

**Feature**: `.specs/features/017-otp-challenges/` | **Date**: 2026-10-20

One new persisted shape (the challenge), one value shape (the code and its digest), one policy shape
(the limits), one declarative shape (the configuration), and two additive methods on the send
intention the preceding feature owns. The existing `users`, `refresh_tokens`, `email_outbox`,
`sheets` and `template` shapes are unchanged apart from one new index on `email_outbox`.

---

## 1. Challenge — `auth_challenges` collection

The durable record of one pending code step. Owned by `internal/auth`.

| Field | Type | BSON | Rules |
|---|---|---|---|
| `ID` | string | `_id` | canonical lowercase UUID v7 from `shared/idgen.New`; the identity the client carries back (`challenge_id`); screened with `idgen.Validate` at the boundary (`OTP-01`) |
| `UserID` | string | `user_id` | the account the code belongs to; indexed with `purpose` (`OTP-01`) |
| `Purpose` | string | `purpose` | `verify_email` \| `login_otp`; resolved server-side and never accepted from the client (`OTP-01`, `OTP-09`) |
| `CodeHash` | string | `code_hash` | the keyed digest of the code, binding `user_id` and `purpose`; the code itself is never stored here (`OTP-02`, `OTP-03`) |
| `ExpiresAt` | time | `expires_at` | the current deadline; extended by a resend up to the ceiling; carries the TTL index (`OTP-06`, `OTP-10`) |
| `CeilingAt` | time | `ceiling_at` | `created_at + ceiling`; the absolute limit a resend can never pass (`OTP-10`) |
| `ConsumedAt` | time (nullable) | `consumed_at` | set exactly once, by the consuming claim; `null` while the challenge is active (`OTP-04`) |
| `Attempts` | int | `attempts` | incremented by every verification attempt, right or wrong; usable only while `< cap` (`OTP-05`) |
| `CreatedAt` | time | `created_at` | the issuance instant; the origin of the ceiling (`OTP-01`) |
| `LastSentAt` | time | `last_sent_at` | the last instant an email was recorded for this challenge; the cooldown origin (`OTP-10`) |
| `LastIntentID` | string | `last_intent_id` | the send intention carrying this challenge's code; what a resend clones (`OTP-10`, `OTP-12`) |

### Invariants

- **One active challenge per account and purpose.** Enforced by a unique partial index (below), not
  by a pre-check: issuing a new challenge closes the previous active row first, and a concurrent
  issuer loses on the duplicate key (`OTP-01`).
- **A challenge is consumable at most once**, even under concurrency: the consuming claim matches only
  while `consumed_at` is null, so exactly one of two racing correct codes wins (`OTP-04`).
- **A challenge is usable only while all three hold**: `consumed_at` is null, `expires_at` is after
  the injected clock, and `attempts` is below the cap. Expiry is never inferred from the row's
  absence (F-13, `OTP-06`).
- **The deadline never passes the ceiling**: a resend sets `expires_at = min(now + lifetime, ceiling_at)`
  (`OTP-10`).
- **No code is recoverable from this record**: the value stored is a keyed digest; the code exists in
  the clear only inside the send intention (`OTP-03`, `R3`).

### State transitions

```text
                    Issue (close previous active)
                              │
                              ▼
                       ┌───────────┐   verify: attempts+1, digest mismatch   ┌───────────┐
                       │  active   │ ──────────────────────────────────────▶ │  active   │
                       │ consumed=∅│                                          │ attempts+1│
                       └───────────┘ ◀────────────────────────────────────── └───────────┘
                         │      │                                                  │
        verify: digest    │      │  resend (cooldown elapsed, under ceiling)         │ attempts == cap
        matches → consume │      │  expires_at ← min(now+lifetime, ceiling_at)       │ or expires_at ≤ now
                         ▼      │  last_sent_at ← now, new intention cloned         ▼
                   ┌───────────┐│                                            ┌────────────────┐
                   │ consumed  │└───────────────────────────────────────────▶│   unusable     │
                   │ (terminal)│                                             │ consumed=false │
                   └───────────┘                                             └────────────────┘
```

`consumed` is terminal. `unusable` is a derived state, not a stored one: the row still exists (until
its TTL) but can never be consumed, and every attempt on it fails uniformly (`OTP-08`).

### Indexes (`shared/db/indexes.go`, idempotent)

| Name | Keys | Options | Backs |
|---|---|---|---|
| `auth_challenges_active_unique` | `{user_id: 1, purpose: 1}` | unique, `partialFilterExpression: {consumed_at: {$eq: null}}` | the one-active invariant (`OTP-01`) |
| `auth_challenges_expires_at_ttl` | `{expires_at: 1}` | `expireAfterSeconds: 0` | hygiene/retention only, never a decision (`OTP-06`, LGPD) |
| `email_outbox_created_at` (existing collection) | `{created_at: 1}` | non-unique | the daily send-cap count (`OTP-13`) |

The TTL monitor runs about once a minute, so a passed `expires_at` can still be present; the
consuming claim filters on it rather than trusting the monitor (F-13).

---

## 2. Code and digest — value shape (not persisted by this domain)

| Shape | Rules |
|---|---|
| The code | exactly six decimal digits from `crypto/rand`, uniform over `000000`…`999999`, leading zeros preserved as a string (`OTP-02`) |
| The digest | `HMAC-SHA256(EMAIL_CODE_SECRET, "legnaris-otp-v1" ‖ user_id ‖ purpose ‖ code)` rendered as lowercase hex; the version tag and the identity binding make a code from one account or purpose unusable for another (`OTP-03`) |
| Comparison | constant-time (`hmac.Equal`) on the digests, never on the codes (`OTP-03`) |
| Validation | the secret is validated once at boot: absent, blank, or shorter than 32 bytes is a typed boot failure (`OTP-18`, `SC-008`) |
| The clear copy | written once into the send intention's payload — the only place it is stored — and rendered by the worker from the preceding feature's template; never logged, echoed, or placed in a subject (`OTP-15`, `R3`) |

---

## 3. Challenge policy — value shape

Resolved from configuration at boot and carried by the service; never stored per challenge.

| Field | Default | Meaning |
|---|---|---|
| `Lifetime` | `10m` | the deadline set at issuance and at each resend (`OTP-10`) |
| `Ceiling` | `30m` | the absolute limit from `created_at` that a resend can never pass (`OTP-10`) |
| `ResendCooldown` | `60s` | the window in which a resend is answered but records no new send (`OTP-10`) |
| `MaxAttempts` | `5` | verification attempts per challenge before it becomes unusable (`OTP-05`) |
| `DailySendCap` | `80` | send intentions per day before the API answers unavailable (`OTP-13`) |
| `UserWindow` / `MaxPerUserWindow` | `1h` / `10` | the durable per-account bound on the challenge routes (`OTP-14`) |

---

## 4. Send intention — `email_outbox` (owned by the preceding feature; extended, not reshaped)

The challenge domain keeps **no** copy of these fields; it records through the `CodeMailer` port.

| Addition | Rules |
|---|---|
| `Repository.Clone(ctx, id, now)` | reads the identified intention and writes a **new** row with a fresh canonical UUID v7, the same `to`, `template` and `payload`, `status = pending`, `attempts = 0`, `next_attempt_at = now`; an unknown id is a typed not-found error (`OTP-10`). The new identity is what keeps the provider from de-duplicating the resend (`EML-18`, `R3`) |
| `Repository.CountSince(ctx, since)` | counts intentions with `created_at ≥ since`; backed by the `{created_at: 1}` index; the daily cap reads it once per issuance and once per resend (`OTP-13`) |

The intention identity is generated by the challenge service (`idgen.New`) and handed to
`outbox.NewIntent`, so the challenge stores `last_intent_id` before the intention row exists and the
plan's write order — account, challenge, intention last — is preserved without a third update (`R8`).

---

## 5. Configuration (declarative)

| Variable | Default | Required | Validated by |
|---|---|---|---|
| `EMAIL_CODE_SECRET` | — | **yes** | `auth.NewCodeDigest` at boot (absent/blank/< 32 bytes → typed error) (`OTP-18`) |
| `AUTH_OTP_TTL` | `10m` | no | the policy builder (positive duration, else default) |
| `AUTH_OTP_CEILING` | `30m` | no | the policy builder |
| `AUTH_OTP_RESEND_COOLDOWN` | `60s` | no | the policy builder |
| `AUTH_OTP_MAX_ATTEMPTS` | `5` | no | the policy builder (positive int) |
| `AUTH_OTP_DAILY_SEND_CAP` | `80` | no | the policy builder |
| `AUTH_OTP_USER_WINDOW` | `1h` | no | the policy builder |
| `AUTH_OTP_USER_LIMIT` | `10` | no | the policy builder |

`env.RequiredKeys()` becomes `{DATABASE_URI, JWT_SECRET, EMAIL_CODE_SECRET}`, and the deployment
description and its test follow (`OTP-18`).

---

## 6. Errors (the boundary contract)

| Condition | Code | Status | Body |
|---|---|---|---|
| any verification failure (unknown, expired, consumed, wrong, exhausted) | `OTP_INVALID` | 401 | one message: `invalid or expired code` (`OTP-08`) |
| daily send cap reached (issuance or resend) | `EMAIL_UNAVAILABLE` | 503 | one message plus `Retry-After` (`OTP-13`) |
| per-address or per-account bound exceeded | `RATE_LIMITED` (existing) | 429 | existing message plus `Retry-After` (`OTP-14`) |
| malformed body: invalid JSON, unknown field, missing field | `INVALID_JSON` / `UNKNOWN_FIELD` / `INVALID_REQUEST` (existing) | 400 | existing messages (`R5`) |
| challenge store or account store failure | `INTERNAL_ERROR` (existing) | 500 | never reported as a code failure (`OTP-17`) |
