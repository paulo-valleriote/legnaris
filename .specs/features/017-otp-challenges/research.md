# Phase 0 Research: Email Code Challenges — Six-digit Codes, Atomic One-shot Consumption and the Unified Verify/Resend Endpoints

**Feature**: `.specs/features/017-otp-challenges/` | **Date**: 2026-10-20

Every decision below answers an open question in the plan's Technical Context or makes explicit a
choice the program plan (`docs/plan-auth-otp-2fa-email.md` §3/§4/§9) and the spec leave to the
design. No `NEEDS CLARIFICATION` remains; the two product-level choices were closed in the spec's
`## Clarifications` (2026-10-20).

---

## R1. The challenge domain lives in `internal/auth`, in four new files plus the service

**Decision**: the record (`challenge.go`), the Mongo store (`challenge_repository.go`), the code
toolkit (`code.go`), the port and policy (`challenge_mailer.go`, `challenge_policy.go`), the
observer seam (`challenge_observer.go`) and the policy-bearing service (`challenge_service.go`) are
new files in `internal/auth`; `service.go` gains the challenge dependencies and three methods
(`IssueChallenge`, `VerifyChallenge`, `ResendChallenge`).

**Rationale**: verification succeeds by issuing exactly the session `signIn` already issues — the
same token manager, refresh family, CSRF binding, user store and injected clock. A separate
`internal/challenge` package would either duplicate that machinery or reach upward into
`internal/auth`, and the record is as much a part of auth as `refresh_tokens`, whose repository
already lives here as its own file.

**Alternatives considered**:

- *A new `internal/challenge` domain package* — rejected: it would need the session issuer, so
  either the session machinery moves (churning every existing auth caller) or the package imports
  `internal/auth` upward.
- *A second service wired with its own collaborators* — rejected: two homes for "establish a
  session" is exactly the drift the constitution's layering rule exists to prevent.
- *Putting the endpoints in `internal/sheet`-style controllers with the logic in `internal/app`* —
  rejected: controllers stay thin; the consumption decision, the limits and the disclosure rules are
  policy and belong to the service.

## R2. The challenge record, its one-active invariant and its expiry hygiene

**Decision**: `auth_challenges` holds `_id` (canonical UUID v7 — the challenge identity the client
carries), `user_id`, `purpose`, `code_hash`, `expires_at`, `ceiling_at`, `consumed_at`,
`attempts`, `created_at`, `last_sent_at`, `last_intent_id`. Indexes: a **unique partial** index on
`{user_id, purpose}` filtered to `consumed_at = null`, and a TTL on `expires_at`.

**Rationale**: the one-active-challenge rule (`OTP-01`) is enforced by the store, not by a
read-then-write pre-check, so two concurrent issuances cannot both win — the loser takes a duplicate
key error. The TTL is hygiene only (LGPD retention); every decision compares `expires_at` with the
injected clock, because Mongo's TTL monitor runs about once a minute (F-13, `OTP-06`).
`ceiling_at` is stored rather than recomputed so the absolute ceiling (`OTP-10`) survives without
re-deriving the creation-origin formula in two places, and `last_intent_id` is what makes a resend
able to re-record the same code (`R3`).

**Alternatives considered**:

- *A partial unique index on `{user_id, purpose, consumed_at}`* — rejected: a plain unique index over
  a nullable field would treat two `null`s as distinct only under a partial filter, which is exactly
  the filter chosen; indexing the nullable field itself adds nothing.
- *Enforcing one-active in the service (close, then read, then insert)* — rejected: racy, and the
  whole point is a durable invariant.
- *No TTL, retiring only on consumption* — rejected: LGPD asks for retention on expiry too, and a TTL
  costs one index declaration.

## R3. The code has exactly one durable home: the send intention

**Decision**: the challenge stores only the keyed digest of the code. Issuance records the send
intention through the `CodeMailer` port, generating the intention identity in the service and passing
it to `outbox.NewIntent`, so the challenge can store `last_intent_id` **before** the intention is
written (the plan's ordering: account, challenge, intention last). A resend does **not** rotate the
code: it extends the deadline, then calls `CodeMailer.Requeue`, which asks the outbox to `Clone` the
original intention — a new row, with a new identity, carrying the same recipient, template and
payload.

**Rationale**: `OTP-10` requires the same code to be re-sent, and a digest cannot be reversed, so a
resend must be able to reach the code somewhere. The outbox is that somewhere: the message contract
already carries the code (`EML-11`, §5.5), it is the only durable copy, and it is retained far longer
than the challenge's ceiling. Cloning rather than re-queuing the same row is what makes the resend
actually deliver: the intention identity is the provider idempotency key (`EML-18`), so re-queuing
the same identity would be silently de-duplicated by the provider and the user would receive nothing.

**Alternatives considered**:

- *Store the code in the clear on the challenge too* — rejected: it duplicates the credential in a
  second collection and makes the digest decorative; the plan's §3.4 requires the digest precisely so
  the challenge holds no recoverable code.
- *Store the code encrypted on the challenge (AES-GCM with the same secret)* — rejected: no security
  gain over the outbox copy that already exists, plus nonce management and a second decryption path.
- *Re-queue the existing intention (reset it to pending)* — rejected: same idempotency key, so the
  provider suppresses the second send; the user waits forever.
- *Rotate the code on resend and update the digest* — rejected by the plan (§3.3): an older email
  arriving after the resend would show a code that no longer works.

## R4. Consumption is the plan's three-write sequence, with a diagnostic read for metrics only

**Decision**: verifying a code is (1) one `findOneAndUpdate` filtered by
`{_id, consumed_at: null, expires_at: {$gt: now}, attempts: {$lt: cap}}` applying `$inc: {attempts: 1}`
and returning the post-image, (2) a constant-time comparison of the submitted code's digest against
`code_hash`, and (3) on a match, a second `findOneAndUpdate` filtered by
`{_id, consumed_at: null, code_hash}` setting `consumed_at`. Step (1) matching nothing means the
challenge is unusable; a separate read then classifies *why* (unknown, expired, consumed, exhausted)
**for metrics and logs only**.

**Rationale**: the attempt must be counted even when the code is wrong, which a single
code-in-the-filter update cannot do (no match means no write). Incrementing first also gives the cap
its meaning (five attempts total, one of which may succeed) and makes the exhausted case fail before
any comparison. Step (3) is the claim that makes consumption one-shot under concurrency: two correct
codes racing both see `consumed_at: null` in step (1), but only one matches in step (3). The
diagnostic read exists because `OTP-16` wants expirations observable while `OTP-08` forbids the
class from changing the response.

**Alternatives considered**:

- *One update filtered by the digest, then a separate attempt-increment update* — rejected: the
  stored digest is compared twice (in the filter and again to classify), and the increment would have
  to be conditional on "still usable", reintroducing a read-then-write window.
- *Comparing the digest in the database (aggregation)* — rejected: non-constant-time, and it would
  move the secret into the query path.
- *Deleting the challenge on consumption* — rejected: a deleted row cannot distinguish "consumed"
  from "never existed", which is needed for the metric, and it would break the concurrent-race
  detection in step (3).

## R5. One uniform failure, produced in one place

**Decision**: every verification failure returns the same typed outcome — `apperr.CodeOTPInvalid`,
401, with the exported message `invalid or expired code` — from a single service path that all five
cases call. A malformed or non-canonical challenge identity takes the same path; only a malformed
*body* (missing field, invalid JSON, unknown field) is a 400.

**Rationale**: `OTP-08` asks for indistinguishability, and one producer is the only way to guarantee
it as the code evolves. Keeping body-shape errors at 400 is not a state leak: it says nothing about
any challenge or account. The diagnostic classification of `R4` stays internal.

**Alternatives considered**:

- *Distinct codes for expired/consumed/exhausted* — rejected: reveals challenge state and gives the
  client a reason to build divergent copy; the spec's Clarifications fixed one code and one status.
- *A 404 for an unknown challenge and a 401 for a wrong code* — rejected: the two are
  indistinguishable to a client that must not be able to probe challenge identities.

## R6. The daily cap counts what was actually recorded, in the outbox

**Decision**: the cap is checked by counting the send intentions recorded since the start of the
current day (`outbox.CountSince`, backed by a new `{created_at: 1}` index) against
`AUTH_OTP_DAILY_SEND_CAP` (default 80, below the provider's free-tier 100). The window and the value
are decided in the auth service; only the count comes from the store. Reaching the cap means no new
intention is written and the request answers `apperr.CodeEmailUnavailable` (503) with `Retry-After`.

**Rationale**: the intentions *are* the sends this system initiates, so counting them is drift-free
and needs no second source of truth — the plan's own alternative (§4.2). Counting a separate quota
document would be exactly as racy (check-then-write in both cases) while introducing a collection
that can silently disagree with the outbox. The free-tier ceiling has deliberate headroom, so a
brief concurrent overshoot bounded by in-flight requests cannot cross the provider's limit; the
spirit of `SC-004` (no intention is recorded once the cap is reached) holds deterministically and
approximately under concurrency, and that is documented behavior, not a hope.

**Alternatives considered**:

- *A dedicated `email_quota` collection with an atomic increment-and-check* — rejected: the exact
  guarantee it buys is bounded by the same check-then-write window in the caller (the increment
  cannot be rolled back if the intention write then fails), so it leaks quota on every store failure
  and can disagree with the outbox.
- *Counting published intentions only* — rejected: a stalled relay would then never reach the cap and
  the API would keep queueing work it cannot deliver within the day.
- *Enforcing the cap in the relay instead* — rejected: the cap exists to answer the user honestly;
  a relay-side refusal would accept the request and silently send nothing.

## R7. Per-account and per-mailbox collapse into one durable bound

**Decision**: the per-account bound on the new routes is a durable window count over the account's
challenges (`challenge_repository.CountForUserSince`), refusing with the existing `RATE_LIMITED`
(429) and `Retry-After` when exceeded; this one bound serves both the "per user" and the "per email"
limit the plan lists, because one canonical mailbox is one account (`PRQ-10`). Per-challenge bounds
are the attempt cap (verify) and the cooldown plus ceiling (resend); the per-address bound is the
existing limiter middleware.

**Rationale**: the in-memory limiter is per process and reset by a deploy (F-07), so it cannot be
the per-account bound for a step where each attempt is worth something; the challenge rows are
already the account's history and are indexed by `user_id`. Merging user and mailbox is not a
shortcut — `PRQ-10` makes two spellings of one mailbox one account, so two keys would be the same
key with extra machinery.

**Alternatives considered**:

- *A counting limiter keyed by the account in the in-memory limiter* — rejected: F-07; a restart or a
  second instance resets the budget.
- *Counting the account's send intentions instead of its challenges* — rejected: the window's purpose
  is to bound how many codes one account can be issued, which is a fact about challenges.
- *No per-account bound, relying on the issuance route's limiter (stage 4)* — rejected: the bound
  must exist where the codes are minted and verified, not depend on a caller that does not exist yet.

## R8. The auth↔email seam is a consumer-declared port adapted at the composition root

**Decision**: `internal/auth` declares

```go
type CodeMailer interface {
    Record(ctx context.Context, id, to string, purpose Purpose, code string, expiresInMinutes int, now time.Time) error
    Requeue(ctx context.Context, id string, now time.Time) (string, error)
    SentSince(ctx context.Context, since time.Time) (int, error)
}

var ErrSendBudgetExhausted = errors.New("daily email send budget exhausted")
```

and `internal/app/email_intent.go` implements it over `internal/email/outbox`
(`NewIntent` + `Create`, `Clone`, `CountSince`). `internal/auth` never imports the email domain.

**Rationale**: the ports-in-the-consumer shape is already the repository's convention
(`auth.Observer` ← `internal/platform/metrics`; `app.Observer` ← the same package), it keeps the
endpoints testable with a plain fake, and it keeps the cap policy on the consumer's side where the
configuration lives. Generating the intention identity in the service is what lets the challenge
store `last_intent_id` before the intention row exists, preserving the plan's write order without a
third update.

**Alternatives considered**:

- *`internal/auth` importing `internal/email/outbox` directly* — rejected: it couples the auth domain
  to a store it does not own and makes every auth test link the Mongo-backed package.
- *A `Mailer` interface in `internal/email` with the auth domain importing it* — rejected: the email
  domain should not know that a challenge exists; its store's vocabulary is intents, not challenges.
- *Having the adapter return the intention identity and updating the challenge afterwards* —
  rejected: it reintroduces a write after the send intention, and a crash between them would leave a
  challenge unable to re-send its own code.

## R9. `EMAIL_CODE_SECRET` is required and validated at boot; everything else is optional

**Decision**: `EMAIL_CODE_SECRET` joins `env.RequiredKeys()`, is read as a required variable, and is
validated by the code toolkit at boot (`NewCodeDigest`), which fails with a typed error when it is
absent, blank, or shorter than 32 bytes; `internal/app.NewModules` propagates that failure so
`cmd/api.go` never starts with a weak key. The challenge settings — lifetime (10m), ceiling (30m),
resend cooldown (60s), attempt cap (5), daily send cap (80), per-account window (10 per hour) — are
optional with those documented defaults, following the existing lookups.

**Rationale**: this is the first setting in the program the *API itself* cannot function safely
without, so the "no new required variable" rule of the two preceding features (which existed because
the API never needed their settings) does not transfer — the spec's Clarifications session (user
decision, 2026-10-20) settled this. A weak or reused key would silently weaken every code (F-09);
failing fast is the honest behavior, and the deployment change is one variable in
`deploy/.env.example`, the compose file and their test.

**Alternatives considered**:

- *Optional with a default derived from `JWT_SECRET`* — rejected: exactly the secret reuse F-09 warns
  about; a session-signing compromise would then also forge codes.
- *Optional, with the feature refusing to serve at runtime* — rejected: a misconfigured deploy would
  surface as a 503 on a security-critical path after boot instead of a boot failure.
- *A generated-and-persisted secret* — rejected: new state, new failure mode, and it would still need
  a place to live that is not the database.

## R10. Observability: a second small observer interface, not a widened `auth.Observer`

**Decision**: `internal/auth` gains `ChallengeObserver` (`ChallengeIssued(purpose)`,
`ChallengeVerified(result)`, `ResendRequested(result)`, `SendCapBlocked()`) with a nop
implementation; `internal/platform/metrics` implements it with
`legnaris_auth_otp_issued_total{purpose}`, `legnaris_auth_otp_verify_total{result}`,
`legnaris_auth_otp_resend_total{result}` and `legnaris_auth_otp_cap_blocked_total`. `result` values
are `verified`, `invalid`, `expired`, `consumed`, `exhausted`, `unknown` for verification and
`sent`, `suppressed`, `capped` for resend.

**Rationale**: widening `auth.Observer` would force every existing implementer and its generated
mock to move for signals they do not care about, and inspection of a challenge is a distinct concern
from credential failures. `expired` is observable because `R4`'s diagnostic read classifies the
failure; the response stays uniform, so the metric is an operator signal, not a client one. No series
carries a code, a challenge identity or an unmasked address (`OTP-15`, `OTP-16`).

**Alternatives considered**:

- *Widening `auth.Observer`* — rejected: churns unrelated implementers and their mocks.
- *Emitting the classification into the HTTP response's `Retry-After` or body* — rejected by `OTP-08`.
- *Logging at info level per attempt* — rejected: the volume is attacker-controlled; counters are the
  right instrument, and logs stay at warn for genuinely notable events (cap reached, store failure).

## R11. The resend answer is uniform, and the email copy states the configured lifetime

**Decision**: `POST /auth/challenge/resend` answers 202 with a body carrying only the configured
retry window (`resend_after`) in every case — re-sent, inside the cool-down, unknown, consumed, or
expired — and `EMAIL_UNAVAILABLE` (503) only when the daily cap is reached. The rendered message
always states the configured lifetime; the server's actual deadline may extend beyond it (up to the
ceiling) but never falls below it.

**Rationale**: an identical body for every case is what `OTP-11` requires, and the only way to make
it identical is to derive it from configuration rather than from the row. The copy rule keeps the
user looking at a code that works at least as long as it claims: extending the deadline after the
email was written is the safe direction, and rewriting the payload would need an outbox update path
the delivery contract does not offer.

**Alternatives considered**:

- *Returning the row's remaining lifetime* — rejected: the body would differ between a live challenge
  and an unknown identity, which is the state disclosure `OTP-11` forbids.
- *Returning 204 with no body* — rejected: the client needs the retry window to render the countdown
  the user story asks for.
- *Updating the payload's `expires_in_minutes` when the deadline moves* — rejected: an outbox update
  path for a cosmetic string, and it would need its own tests and retention rules.

## R12. Verification issues a session through the existing sign-in path, whatever the purpose

**Decision**: on a successful consumption the service records the account's verification timestamp
(idempotently, through `MarkEmailVerified`) and then establishes the session through the existing
`signIn` and controller cookie code. Both purposes take the same path: `verify_email` and
`login_otp` differ only in what triggered them, and `login_otp` recording the timestamp when it was
absent *is* decision D4.

**Rationale**: the purpose is a server-side attribute (O3) that must not fork the session contract;
one path is one set of tests. Recording the timestamp before issuing the session means a session is
only ever minted for a verified account once this feature is in play — the property stage 4's
renewal gate (`Refresh` checking `email_verified_at`) will rely on.

**Alternatives considered**:

- *A distinct session for `verify_email`* — rejected: the plan's §3.2 says both emit a session; two
  paths would double the contract and the tests for no product difference.
- *Recording the timestamp after the session is issued* — rejected: an issuance failure would leave a
  verified-looking session for an unverified account, the exact inversion of the rule stage 4 needs.
- *Leaving the timestamp to stage 4* — rejected: `OTP-07` is this endpoint's effect, and the endpoint
  is this feature's.

## R13. The new routes take the address limiter and no origin guard

**Decision**: both new routes sit behind the existing per-address rate limiter and inside
`registerAuthRoutes`; neither takes `requireAllowedOrigin`, matching the existing login and
registration routes.

**Rationale**: the origin guard exists for routes that authenticate with ambient cookie authority
(refresh, logout), which is the CSRF surface the constitution names. Verification and resend present
a capability (the challenge identity and its code) and no session credential, so an attacker's page
has no ambient authority to abuse — the same reasoning that leaves login and registration
unguarded. Piling the guard on would also break the non-browser callers the integration suite uses
and would imply a CSRF property that does not exist.

**Alternatives considered**:

- *The origin guard on both* — rejected: it protects nothing here and would be an inconsistent
  precedent against login/registration.
- *The origin guard on verify only because it issues cookies* — rejected: the session is issued in
  exchange for a capability the caller must already hold, which is precisely the case the guard does
  not cover.
