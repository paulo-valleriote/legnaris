# Contract: Auth clock

**Feature**: `.specs/features/015-auth-prerequisites/` | **Requirement IDs**: `PRQ-16`, `SC-007` |
**Owner**: `internal/auth` (`Clock`), `internal/app/bootstrap.go` (production value)

## Purpose

Every auth timestamp is written or compared through one injectable source, so boundary tests never
sleep and production behavior is unchanged.

## Shape

```go
// internal/auth
type Clock func() time.Time

// Config carries the clock; nil means time.Now.
type Config struct {
    RefreshTTL time.Duration
    Now        Clock
}

func NewTokenManager(secret string, ttl time.Duration, now Clock) (TokenManager, error)
```

- Production wiring: `time.Now` passed to both the service config and the token manager.
- Nil is accepted at construction and defaults to `time.Now` (the limiter's existing convention).
- The repository and the migration never read the clock; they receive `at` from their caller.

## Timestamp ownership

| Written / compared | Where | Clock |
|---|---|---|
| `created_at`, `updated_at` on registration | `auth.Register` | service clock |
| Refresh credential `created_at`, `expires_at` | `auth.issueRefreshToken` | service clock |
| Refresh expiry decision | `auth.Refresh` | service clock |
| Access-token `iat`, `exp` | `jwtTokenManager.Issue` | token-manager clock |
| Access-token expiry validation | `jwtTokenManager.Parse` (`jwt.WithTimeFunc`) | token-manager clock |
| `email_verified_at`, `updated_at` on verification | `user.MarkEmailVerified` | caller-supplied `at` |
| `email`, `updated_at` in the migration | `EmailMigrationPlan.Apply` | caller-supplied `now` |

CSRF tokens carry no timestamp; cookie `Max-Age` is configuration, not clock.

## Boundary semantics

- Refresh expiry: `now.Before(expires_at)` → valid; `now == expires_at` or later → expired. The
  exact instant is **expired** (spec edge case).
- Access-token expiry follows `jwt` v5 validation (`now >= exp` is expired) now driven by the same
  clock.
- All other observable production behavior is unchanged.

## Test evidence

- `internal/auth/service_test.go`: frozen clock asserts exact `created_at`/`updated_at`/
  `expires_at`; stepped clock asserts one nanosecond before expiry is valid and the exact instant is
  expired; refresh succeeds one nanosecond after issuance.
- `internal/auth/token_test.go`: issuance uses the injected clock; parse honours it.
- No test calls `time.Sleep`.
