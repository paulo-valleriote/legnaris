# Contract: Email verification timestamp (store-only)

**Feature**: `.specs/features/015-auth-prerequisites/` | **Requirement IDs**: `PRQ-12`…`PRQ-15` |
**Owner**: `internal/user` (record + repository)

## Purpose

Make the account ready for the verification gate without exposing anything: the record can hold the
first verification instant and the store can set it exactly once.

## Record

```go
type User struct {
    // …
    EmailVerifiedAt *time.Time `bson:"email_verified_at,omitempty" json:"email_verified_at,omitempty"`
}
```

- Absent for every existing account (no backfill) and for every account created by `Register`.
- Never included in `UserDTO`; no HTTP response exposes it (`PRQ-14`).
- Never cleared by this feature.

## Operation

```go
MarkEmailVerified(ctx context.Context, id string, at time.Time) error
```

Implementation contract:

1. One `UpdateOne` with filter `{_id: id, email_verified_at: nil}` and
   `$set: {email_verified_at: at, updated_at: at}`.
2. `MatchedCount == 1` → return `nil`.
3. `MatchedCount == 0` → `FetchByID`:
   - found → return `nil` (already verified; `updated_at` unchanged, first value preserved);
   - `ErrUserNotFound` → return `ErrUserNotFound` (never creates a document);
   - other error → wrapped store error.
4. Any driver error is wrapped with `fmt.Errorf("mark email verified: %w", err)`.

`{email_verified_at: nil}` matches both a missing field and an explicit `null`, so a legacy or
hand-written document with a null value is still "unverified".

## Invariants

| ID | Invariant | Requirement |
|---|---|---|
| V1 | The operation never creates an account | `PRQ-13` |
| V2 | Every field except `email_verified_at` and `updated_at` is untouched | `PRQ-13` |
| V3 | A second call is a successful no-op that preserves the first timestamp and `updated_at` | `PRQ-13` |
| V4 | Concurrent calls cannot overwrite the first value (conditional filter) | `PRQ-13` |
| V5 | The operation is not reachable from any HTTP route | `PRQ-14` |
| V6 | Boot and deployment never populate the field | `PRQ-12`, `PRQ-15` |

## Failure modes

| Condition | Result |
|---|---|
| Unknown account | `ErrUserNotFound` |
| Driver/store failure | wrapped error (the boundary would map it to `INTERNAL_ERROR` when a caller exists) |
| Already verified | `nil`, no write |

## Test evidence

- `internal/user/repository_test.go` (Mongo, self-skip): unknown id → `ErrUserNotFound`; first call
  sets only the two fields; second call preserves both values; a freshly created user has no
  `email_verified_at`; concurrent calls leave the first timestamp.
- `internal/user/user_test.go` (new): `ToDTO` carries no verification field.
- Absence of a route is asserted by the existing route inventory/HTTP suites (no new endpoint is
  registered).
