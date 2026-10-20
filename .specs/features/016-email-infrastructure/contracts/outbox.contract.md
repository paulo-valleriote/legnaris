# Contract: Durable outbox

**Feature**: `.specs/features/016-email-infrastructure/` | **Requirement IDs**: `EML-01`, `EML-03`,
`EML-04`, `EML-17`, `EML-18` | **Owner**: `internal/email/outbox`

## Purpose

Record a send intention durably before anyone is told the request was accepted, and move it through
its lifecycle with single-document atomic writes, so a crash, a concurrent publisher, or a broker
outage never loses the email.

## Interface

```go
type Status string
const (
    StatusPending   Status = "pending"
    StatusPublished Status = "published"
    StatusFailed    Status = "failed"
)

type Repository interface {
    Create(ctx context.Context, intent Intent) error
    Claim(ctx context.Context, claimer string, now time.Time, lease time.Duration, limit int) ([]Intent, error)
    MarkPublished(ctx context.Context, id string, at time.Time) error
    Reschedule(ctx context.Context, id string, now, nextAt time.Time, lastError string) error
    MarkTerminal(ctx context.Context, id string, now time.Time, lastError string) error
    Backlog(ctx context.Context) (int, error)
}
```

`NewIntent(id, to, template string, payload Payload, now time.Time) (Intent, error)` generates a
canonical UUID v7 identity (or validates a supplied one) and starts the row `pending` with
`next_attempt_at = now`, `attempts = 0` and no lease.

## Rules

- `Create` writes one document and never publishes; the request-serving caller returns only after it
  succeeds (EML-01).
- `Claim` runs one conditional `findOneAndUpdate` **per returned row**: filter
  `{status: pending, next_attempt_at: {$lte: now}, $or: [{locked_until: null}, {locked_until: {$lte: now}}]}`,
  sorted by `next_attempt_at`, setting `{locked_until: now+lease, locked_by: claimer}` and returning
  the post-image; it stops at `limit` rows. It does not change `status` or `attempts` (EML-03).
- `MarkPublished` sets `{status: published, published_at: at, locked_until: null, locked_by: ""}`; it
  is the only writer of `published_at`.
- `Reschedule` sets `{status: pending, attempts: $inc 1, next_attempt_at: nextAt, locked_until: null,
  locked_by: "", last_error: msg}` (EML-04).
- `MarkTerminal` sets `{status: failed, locked_until: null, locked_by: "", last_error: msg}` and the
  row is never claimed again (EML-04).
- Every write validates `id` with `shared/idgen.Validate`; a non-canonical identity is rejected
  (EML-18).
- `Backlog` counts `{status: pending}` (EML-23).

## Invariants

- A row is claimed by at most one publisher at a time (`locked_until` in the future excludes it).
- A terminal row (`published`/`failed`) is never returned by `Claim`.
- `attempts` equals the number of failed publications for the row.
- A crash after publish and before `MarkPublished` leaves the row claimable again after the lease,
  and republishing is harmless (identity = idempotency key).

## Boundary cases

| Case | Result |
|---|---|
| two publishers claim in the same window | the conditional write gives each row to exactly one |
| claim while a lease is still live | the row is not returned |
| claim exactly at lease expiry (`locked_until == now`) | the row is returned (`$lte`) |
| empty outbox | `Claim` returns no rows, no error |
| row with a non-canonical `_id` | rejected, not published (EML-18) |
| `MarkPublished` on an already-published row | no-op safe (idempotent write by `_id`) |
| retention: published row past the window | eligible for the TTL monitor; pending/failed rows (`published_at` null) are never expired |
| store failure during `Claim` | returned unwrapped for the relay to log; the relay stops the pass and tries the next tick |

## Test evidence

- `internal/email/outbox/intent_test.go`: identity generation/validation, initial lifecycle.
- `internal/email/outbox/repository_test.go` (Mongo, self-skip): unique claim under two callers,
  lease expiry re-claim, `attempts` bump on `Reschedule`, terminal immutability, backlog count, TTL
  index presence, canonical-identity rejection.
- `internal/email/relay/relay_test.go`: the relay consumes the contract (crash-between, cap).