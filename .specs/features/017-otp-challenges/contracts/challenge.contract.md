# Contract: Challenge record and its store

**Feature**: `.specs/features/017-otp-challenges/` | **Requirement IDs**: `OTP-01`, `OTP-04`,
`OTP-05`, `OTP-06`, `OTP-14` | **Owner**: `internal/auth` (`challenge.go`,
`challenge_repository.go`)

## Purpose

Give the code step one durable home whose consumption decision is a single-document write, so a
replay, a race, a restart or a second instance can never consume one challenge twice, and make the
one-active-challenge rule an invariant the store enforces rather than a pre-check the service hopes
to win.

## Shape

```go
type Purpose string

const (
    PurposeVerifyEmail Purpose = "verify_email"
    PurposeLoginOTP    Purpose = "login_otp"
)

// Challenge is one pending code step (data-model.md §1).
type Challenge struct {
    ID           string     // canonical UUID v7; the client-facing challenge_id
    UserID       string
    Purpose      Purpose
    CodeHash     string     // keyed digest; never the code
    ExpiresAt    time.Time  // movable up to CeilingAt
    CeilingAt    time.Time  // created_at + ceiling; never passed
    ConsumedAt   *time.Time // set exactly once, by Consume
    Attempts     int        // verification attempts, right or wrong
    CreatedAt    time.Time
    LastSentAt   time.Time
    LastIntentID string     // the send intention a resend clones
}

type ChallengeRepository interface {
    // CloseActive invalidates the account's active challenge for the purpose.
    CloseActive(ctx context.Context, userID string, purpose Purpose, at time.Time) error
    // Insert writes a new active challenge; a concurrent active row for the same
    // pair is rejected with ErrChallengeActive.
    Insert(ctx context.Context, challenge Challenge) error
    // ClaimAttempt increments the attempt count and returns the row only while it
    // is usable (unconsumed, unexpired, under maxAttempts).
    ClaimAttempt(ctx context.Context, id string, now time.Time, maxAttempts int) (*Challenge, error)
    // Consume claims the row by its digest and reports whether this caller won.
    Consume(ctx context.Context, id, codeHash string, at time.Time) (bool, error)
    // Extend stamps a resend: it moves the deadline to newExpiry and updates
    // LastSentAt, only while the row is usable and its cooldown has elapsed. It
    // reports false when the resend must be suppressed.
    Extend(ctx context.Context, id string, now time.Time, cooldown time.Duration, newExpiry time.Time) (bool, error)
    // Fetch reads one row without mutating it.
    Fetch(ctx context.Context, id string) (*Challenge, error)
    // CountForUserSince counts the account's challenges created since an instant.
    CountForUserSince(ctx context.Context, userID string, since time.Time) (int, error)
}

var (
    ErrChallengeNotFound = errors.New("challenge not found")
    ErrChallengeUnusable = errors.New("challenge is not usable")
    ErrChallengeActive   = errors.New("account already has an active challenge for this purpose")
)
```

The sentinels are plain `errors.New`, never `apperr`: this is a store, not an HTTP boundary, exactly
like `internal/user.ErrUserNotFound` and `shared/db`'s own typed errors.

## Rules

- `CloseActive` is one `UpdateMany` filtered `{user_id, purpose, consumed_at: null}` setting
  `consumed_at = at`; it never touches an already-consumed row (`OTP-01`).
- `Insert` is one `InsertOne`; the unique partial index rejects a second active row for the pair, and
  the duplicate-key error is translated to `ErrChallengeActive` (`OTP-01`).
- `ClaimAttempt` is one `findOneAndUpdate` filtered
  `{_id, consumed_at: null, expires_at: {$gt: now}, attempts: {$lt: maxAttempts}}` applying
  `$inc: {attempts: 1}` and returning the **post-image**; no match is `ErrChallengeUnusable`
  (`OTP-04`, `OTP-05`, `OTP-06`). It does not compare the digest — the service does.
- `Consume` is one `findOneAndUpdate` filtered `{_id, consumed_at: null, code_hash: codeHash}`
  setting `consumed_at = at`; `MatchedCount == 1` means this caller consumed the challenge and
  `false` means another request won the race (`OTP-04`). It is idempotent-safe: a second call matches
  nothing and reports `false`.
- `Extend` is one `findOneAndUpdate` filtered
  `{_id, consumed_at: null, expires_at: {$gt: now}, last_sent_at: {$lte: now - cooldown}}` setting
  `{expires_at: newExpiry, last_sent_at: now}`; no match reports `false`, which is the cool-down
  suppression (and the concurrent-resend suppression) (`OTP-10`). The service computes `newExpiry` as
  `min(now + lifetime, CeilingAt)`, so the ceiling is never passed.
- `Fetch` is one `FindOne` that never mutates; the service uses it to classify a failure for metrics
  and to read `LastIntentID` and `CeilingAt` for a resend (`OTP-16`, `R4`).
- `CountForUserSince` is one indexed `CountDocuments` over `{user_id, created_at ≥ since}` and backs
  the durable per-account bound (`OTP-14`).
- Every identity the store writes has been validated with `idgen.Validate` by the service before it
  reaches the store (`OTP-01`).

## Invariants

- At most one row with `consumed_at == null` exists per `{user_id, purpose}` (unique partial index).
- `consumed_at` is written exactly once per row and never cleared.
- `attempts` only ever increases, and only `ClaimAttempt` and `Insert` write it.
- `expires_at ≤ ceiling_at` holds after every write.
- `ClaimAttempt` and `Consume` on an unusable row never mutate it.
- The TTL monitor deleting a row never changes a decision: the decision is the filtered write.

## Boundary cases

| Case | Result |
|---|---|
| two correct verifications race | `ClaimAttempt` admits both read-side, `Consume` admits exactly one; the loser gets `false` (`OTP-04`) |
| verification exactly at `expires_at` | filtered out (`$gt`): unusable; one nanosecond before, usable (`OTP-06`) |
| fifth wrong guess | `ClaimAttempt` succeeds (attempts 4 → 5); the sixth finds `attempts == cap` and is `ErrChallengeUnusable` (`OTP-05`) |
| two issuances for one pair concurrently | `CloseActive` runs for both, `Insert` succeeds for one, the other gets `ErrChallengeActive` (`OTP-01`) |
| two resends inside one cooldown window | `Extend` matches for exactly one; the other reports `false` and records no send (`OTP-10`) |
| resend with `now + lifetime` beyond the ceiling | `expires_at = ceiling_at`, never later (`OTP-10`) |
| row already expired, still present (TTL lag) | `ClaimAttempt` filters it out; the row's presence is irrelevant (`OTP-06`) |
| unknown identity on any operation | `ErrChallengeNotFound` for `Fetch`; `ErrChallengeUnusable` for the conditional writes (`OTP-08`) |
| `Insert` with a non-canonical `ID` | rejected by the service before the store; a stored malformed row is never produced (`OTP-01`) |
| store failure during `ClaimAttempt` | returned wrapped; the service maps it to `INTERNAL_ERROR`, never to a code failure (`OTP-17`) |

## Test evidence

- `internal/auth/challenge_repository_test.go` (Mongo, self-skip): the unique partial index rejects a
  second active row and admits one after consumption; two concurrent `ClaimAttempt`s; two concurrent
  `Consume`s (exactly one `true`); `Extend` cooldown suppression and the ceiling clamp; `Fetch`
  classification of unknown/expired/consumed/exhausted; `CountForUserSince` range; the TTL index and
  its `expireAfterSeconds`.
- `internal/auth/challenge_test.go`: the usability predicate and the derived states.
- `internal/auth/challenge_service_test.go`: the conditional-write outcomes drive the service's
  uniform failure without a database.
- `shared/db/indexes_test.go` (Mongo, self-skip): both new indexes exist with the documented options.
