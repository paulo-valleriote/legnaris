# Contract: Auth rate limiting

**Requirements**: AUTH-17, AUTH-18, AUTH-19, AUTH-20 | **Backing document**:
[data-model.md `RateLimitKey and window`](../data-model.md) and [research.md `R7` / `R11`](../research.md)

**Modules**: `internal/platform/ratelimit/limiter.go` (the reusable engine),
`internal/app/ratelimit.go` (the guard: keys, 429, `Retry-After`, counter),
`internal/app/router.go` (route mounting), `shared/env/env.go` (threshold/window),
`internal/platform/metrics/metrics.go` (the block counter).

## Interface

```go
// internal/platform/ratelimit — leaf package, standard library only
type Limiter struct{ /* mu, entries, max, window, now */ }
func New(max int, window time.Duration, now func() time.Time) *Limiter
func (l *Limiter) Allow(key string, now time.Time) (allowed bool, retryAfter time.Duration)

// internal/app/ratelimit.go
func rateLimit(limiter *ratelimit.Limiter, observer Observer, withAccount bool) echo.MiddlewareFunc
```

## Rules

1. **Per address and per account** (AUTH-17). Every limited route is checked under `ip:<RealIP>`; on
   login and registration an `account:<normalized email>` key is added when the body yields an email
   (the body is read and restored so the handler still decodes it). Refresh is checked per address
   only, because the account identity is not available before the cookie resolves. The two keys are
   independent.
2. **Over the limit ⇒ 429, nothing done** (AUTH-18). The request that exceeds a key's limit is answered
   `429` (`RATE_LIMITED`) with a `Retry-After` header in whole seconds; **no credential is verified and
   no token is issued** for it, and it never reaches the handler.
3. **Independent keys and a freeing window** (AUTH-19). Exhausting one key never blocks another
   (different address, different account); a key is allowed again once its oldest attempt ages out of
   the rolling window.
4. **Configured, documented defaults** (AUTH-20). The cap and the window come from
   `AUTH_RATE_LIMIT_ATTEMPTS` (default 10) and `AUTH_RATE_LIMIT_WINDOW` (default `1m`); there is no
   magic number in the request path. Every block increments `legnaris_auth_rate_limited_total`.
5. **Bounded memory** (R7). Attempts older than the window are dropped on every check and a key whose
   list empties is deleted, so the map never grows beyond the keys active in one window. The clock is
   injected, so a suite drives the window without waiting.
6. **Scope** (AUTH-17). The guard covers login, registration and refresh; logout and the Bearer routes
   are not limited by it.

## Obligations

1. **AUTH-17** — login, registration and refresh are limited per address and (where available) per
   account.
2. **AUTH-18** — over-limit ⇒ 429 with `Retry-After`, no credential work, no token.
3. **AUTH-19** — keys are independent and a key recovers when its window frees.
4. **AUTH-20** — thresholds and window are configured with documented defaults; each block counts.

## Test obligations

- **Happy path (unit)**: under the cap `Allow` admits and records; the block counter is observable at the
  boundary; with an injected clock, advancing past the window admits again.
- **Edge (unit)**: a fresh key is admitted; exhausting one key leaves another admitted; the attempt that
  equals the cap is admitted; the next is refused with a positive `retryAfter`; the key entry is dropped
  once its attempts age out (no unbounded growth).
- **Error (boundary)**: the request past the limit answers 429 with `Retry-After` and a `RATE_LIMITED`
  body, and the service mock records **zero** calls (no credential verified, no token issued); a second
  address or account is unaffected; once the window frees the same key is admitted; the block increments
  the counter exactly once.
- **Config**: an unset or invalid `AUTH_RATE_LIMIT_ATTEMPTS`/`AUTH_RATE_LIMIT_WINDOW` falls back to the
  documented default and logs a warning.

## Traceability

- Keys and scope: AUTH-17, AUTH-19.
- Blocking: AUTH-18.
- Configuration and metric: AUTH-20.
