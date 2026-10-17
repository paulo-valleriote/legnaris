# Phase 1 — Data Model: Backend Browser Auth Cookies

The feature persists nothing new: the session is the existing `refresh_tokens` record, the CSRF token is
stateless, the rate-limit window is in process memory, and the cookie contract is a set of fixed
constants. Every element below traces to a functional requirement in [spec.md](./spec.md); the
transport and verification rules are pinned in the four documents under [contracts/](./contracts/).

Naming: Go types use `PascalCase`, exported constants use the package's own prefix, JSON keys stay
`snake_case` on the wire. No record, field, log line or URL carries a refresh credential or the CSRF
secret.

---

## RefreshSession (existing record, reused)

The rotated chain of refresh credentials for one sign-in — the session the CSRF token is bound to and
the unit revoked on logout or reuse. Unchanged from `internal/auth/refresh.go`.

| Field | Type | Rules |
|---|---|---|
| `ID` | `string` (`bson:"_id"`) | UUID v7 via `idgen.New`; the credential row, not the family |
| `TokenHash` | `string` | SHA-256 hex of the raw credential; the raw value is never stored (`AUTH-08`) |
| `UserID` | `string` | the authenticated user |
| `FamilyID` | `string` | the sign-in identity; shared by every rotation and the CSRF binding key (`AUTH-11`) |
| `ExpiresAt` | `time.Time` | expiry; an expired credential is 401 (`AUTH-07`) |
| `Revoked` | `bool` | a revoked credential presented again means reuse and revokes the family (`AUTH-07`) |
| `ReplacedBy` | `string` | the successor's id, set on rotation |
| `CreatedAt` | `time.Time` | creation instant |

**No schema change**: no CSRF field, no rate-limit field. `db.EnsureIndexes` is untouched.

---

## Session (service return value — NEW)

The material `Register`, `Login` and `Refresh` hand to the controller. Only the controller decides what
becomes a cookie and what becomes a body (`R1`).

| Field | Type | Rules |
|---|---|---|
| `AccessToken` | `string` | the unchanged JWT; serialised into the body (`AUTH-26`) |
| `ExpiresIn` | `int` | access-token lifetime in seconds; serialised into the body |
| `RefreshToken` | `string` | the raw credential — **cookie only**, never serialised (`AUTH-01`, `AUTH-02`, `AUTH-03`) |
| `RefreshMaxAge` | `int` | `AuthRefreshTTL` in whole seconds; the refresh and CSRF `Max-Age` (`AUTH-01`, `AUTH-24`) |
| `CSRFToken` | `string` | the freshly minted, session-bound token (`AUTH-09`, `AUTH-12`) |
| `UserID` | `string` | the authenticated user, for logging only; never serialised |

**Invariants**: the raw credential and the CSRF secret never reach a body, a URL or a log
(`AUTH-08`, `AUTH-25`); a successful rotation issues a **new** `RefreshToken` **and** a new `CSRFToken`
together (`AUTH-03`, `AUTH-12`).

---

## SessionResponse (HTTP body — NEW)

The login, registration and refresh body. It has **no** refresh field, which is what `SC-001` asserts.

| Field | JSON | Type | Rules |
|---|---|---|---|
| `AccessToken` | `access_token` | `string` | present on every success |
| `TokenType` | `token_type` | `string` | `"Bearer"`, unchanged |
| `ExpiresIn` | `expires_in` | `int` | seconds |

**Invariants**: identical shape for login, registration and refresh (`AUTH-01`, `AUTH-02`, `AUTH-03`);
the refresh credential appears **only** in `Set-Cookie`, never in the body (`SC-001`).

---

## Cookie descriptors (contract-fixed — NEW)

Defined in `internal/auth/cookie.go`. Names and attributes are constants, not configuration
(`AUTH-24`); only the `Max-Age` derives from `AuthRefreshTTL`.

| Cookie | Name constant | `HttpOnly` | `Secure` | `SameSite` | `Path` | `Domain` | `Max-Age` |
|---|---|---|---|---|---|---|---|
| refresh | `__Host-legnaris_refresh` | `true` | `true` | `Strict` | `/` | none | `AuthRefreshTTL` s |
| CSRF | `__Host-legnaris_csrf` | `false` | `true` | `Strict` | `/` | none | `AuthRefreshTTL` s |

**Clearing rule**: logout (and any error path that must drop the session) sets the same names with an
empty value and `Max-Age=-1`; the attribute set is otherwise identical, so the browser removes exactly
the cookie that was set (`AUTH-04`, `AUTH-12`).

**Invariants**: the `__Host-` prefix requires `Secure` + `Path=/` + no `Domain` and is never weakened for
local development (spec Assumptions); dropping any attribute fails the suite (`SC-001`).

---

## CSRFToken (stateless — NEW)

Minted and verified by `internal/auth/csrf.go` (`R2`); never stored.

| Part | Size | Rules |
|---|---|---|
| nonce | 16 bytes | from `crypto/rand`; fresh on every issue, so rotation changes the token (`AUTH-12`) |
| MAC | 32 bytes | `HMAC-SHA256(key, "csrf/v1" ‖ 0x00 ‖ familyID ‖ 0x00 ‖ nonce)`; `key = HMAC-SHA256(jwtSecret, "legnaris/csrf/v1")` |

**Encoded form**: `base64url(nonce ‖ MAC)`, carried in the script-readable cookie and echoed in the
`X-CSRF-Token` header.

**Verification** (`Verify(token, familyID)`): decode; require the exact length; recompute the MAC over
the presented `familyID`; compare with `hmac.Equal`. A missing header, a malformed/short value, a
tampered MAC or a token minted for another family all fail (`AUTH-10`, `AUTH-11`).

**Invariants**: the server never compares the header to the cookie — it recomputes from its own secret
and the resolved session (`AUTH-11`); the token is unforgeable without the server secret; verifying for
one family cannot succeed for another.

---

## AllowedOriginSet (configured — reused)

The origins permitted to make credentialed cross-origin requests, from `Variables.CORSOrigins`
(`CORS_ALLOWED_ORIGINS`).

| Rule | Detail |
|---|---|
| Matching | exact, case-insensitive origin match; never a wildcard (`AUTH-14`, `AUTH-15`) |
| Source | `Origin`, falling back to the origin of `Referer` (`AUTH-13`) |
| Miss | absent or unmatched ⇒ 403 before credential work (`AUTH-13`) |

**Invariants**: one list governs both the CORS response headers and the server-side origin guard;
`*` is never sent with credentials (Echo refuses to build that config); `Vary: Origin` is always present.

---

## RateLimitKey and window (in-process — NEW)

The limiter in `internal/platform/ratelimit` (`R7`). One entry per key, holding the timestamps of the
attempts inside the window.

| Element | Value | Rules |
|---|---|---|
| key (address) | `ip:<RealIP>` | every limited route; never blocks another address (`AUTH-19`) |
| key (account) | `account:<normalized email>` | login and registration only, read from the JSON body; never blocks another account (`AUTH-19`) |
| `max` | `AuthRateLimitAttempts` (default 10) | the configured cap (`AUTH-20`) |
| `window` | `AuthRateLimitWindow` (default `1m`) | the rolling window length (`AUTH-20`) |

| Operation | Rule |
|---|---|
| `Allow(key, now)` | drops attempts older than `window`, then admits and records while under `max`; over `max` returns `false` and the time until the oldest attempt ages out |
| `Retry-After` | that time, in whole seconds rounded up (`AUTH-18`) |
| counter | every block increments `legnaris_auth_rate_limited_total` (`AUTH-20`) |

**Invariants**: no credential is verified and no token is issued for a blocked request (`AUTH-18`); the
window frees exactly when the oldest attempt ages out (`AUTH-19`); the clock is injected so no suite
waits; a key whose window empties is removed so memory stays bounded.

---

## Config additions (shared/env — extended)

Documented defaults, no magic numbers in the request path (`AUTH-20`, `AUTH-24`).

| Variable | Field | Default | Read with |
|---|---|---|---|
| `AUTH_RATE_LIMIT_ATTEMPTS` | `AuthRateLimitAttempts int` | `10` | `lookupIntEnv` |
| `AUTH_RATE_LIMIT_WINDOW` | `AuthRateLimitWindow time.Duration` | `1m` | `lookupDurationEnv` |
| `AUTH_REFRESH_TTL` | `AuthRefreshTTL` (existing) | `720h` | unchanged |

**Invariants**: an absent or invalid value logs a structured warning and falls back to the default; the
cookie names, cookie attributes and the CSRF binding shape are **not** configurable.

---

## Error codes and message constants (extended)

`shared/apperr` gains one code; the auth and app packages export the new messages (`AUTH-23`).

| Code | Status | Raised when | Message constant |
|---|---|---|---|
| `CodeUnauthorized` (existing) | 401 | cookie absent, credential unknown/expired/rotated (`AUTH-06`, `AUTH-07`) | `auth.MsgRefreshTokenRequired`, `auth.MsgInvalidRefreshToken` |
| `CodeForbidden` (existing) | 403 | CSRF missing/malformed/foreign | `auth.MsgCSRFInvalid` |
| `CodeForbidden` (existing) | 403 | origin absent/disallowed | `app.MsgOriginNotAllowed` |
| `CodeRateLimited` (**new**) | 429 | key over its limit | `app.MsgRateLimited` |
| `CodeConflict` (existing) | 409 | duplicate email on registration | `auth.MsgEmailAlreadyExists` |

**Invariants**: every failure path asserts the typed code and the mapped status (`AUTH-27`); 429 is the
only new status and it is added to the taxonomy's mapping (`AUTH-23`).
