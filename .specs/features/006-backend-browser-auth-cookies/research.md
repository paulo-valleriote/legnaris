# Phase 0 Research: Backend Browser Auth Cookies

**Feature**: `.specs/features/006-backend-browser-auth-cookies/` | **Date**: 2026-10-16

Every decision below answers an open question in the feature's Technical Context or resolves a choice
the program plan's F6 task list (`docs/plan-web-pwa.md`, Parte B F6, tasks `F6B-T001`…`F6B-T006`)
leaves implicit. No `NEEDS CLARIFICATION` remains. The three decisions the plan left silent or
self-contradictory were confirmed with the user and are recorded in the spec's `## Clarifications`
(registration signs in; the CSRF token is stateless and session-bound; a missing `Origin`/`Referer`
fails closed).

---

## R1. Session material and the transport boundary

**Decision**: `internal/auth` gains a `Session` value returned by `Register`, `Login` and `Refresh`:
the access token and its lifetime (the unchanged JWT contract), the raw refresh credential, the
session family id, the minted CSRF token, and the refresh lifetime in seconds. The **controller**
(`internal/app/auth.go`) owns transport: it reads the refresh cookie on `/auth/refresh` and
`/auth/logout`, fills it into the command, and on success sets the two cookies and serialises a
**session body** (`access_token`, `token_type`, `expires_in`) that carries no refresh credential.

`RefreshCommand`/`LogoutCommand` keep their shape (a credential plus, now, the CSRF header value), but
the credential is filled from the cookie, never from a header. `auth.Service`'s signatures change to
return `*Session`; the mockery mocks are regenerated (`make mocks`).

**Rationale**: the service is the only layer that knows the family id, so it is the only layer that can
bind the CSRF token to the session and rotate both together (`AUTH-12`); the controller is the only
layer that knows HTTP, so it is where `Set-Cookie`/`http.Cookie` belong. Keeping the raw refresh
credential out of any response type (`SessionResponse` has no refresh field) is what makes `AUTH-01`,
`AUTH-02`, `AUTH-03` and `SC-001` structural rather than a serialisation accident.

**Alternatives considered**:

- *Keep `TokenPair` and add `json:"-"` on the refresh field* — rejected: the same struct would be one
  keystroke from leaking the credential, and the type name would still promise a body pair.
- *Mint the CSRF token in the controller* — rejected: the controller never sees the family id, so it
  could not bind the token to the session (`AUTH-11`) and could not rotate it with the family
  (`AUTH-12`).

## R2. The CSRF token: stateless HMAC bound to the session, key derived from the JWT secret

**Decision**: introduce `internal/auth/csrf.go` with a `CSRFTokenManager` interface
(`Issue(familyID string) (string, error)`, `Verify(token, familyID string) error`). The token is

```
token = base64url( nonce || HMAC-SHA256(key, "csrf/v1" || 0x00 || familyID || 0x00 || nonce) )
```

where `nonce` is 16 bytes from `crypto/rand`. `Verify` recomputes the MAC over the *presented* family
id and compares with `hmac.Equal`; a wrong length, a malformed base64, a tampered MAC or a token
minted for another family all fail. The signing `key` is **derived from the JWT secret**:
`key = HMAC-SHA256([]byte(jwtSecret), "legnaris/csrf/v1")`, so the CSRF key is domain-separated from
the JWT signing key and needs no new required environment variable.

Rotation is a fresh nonce: every `Issue` call produces a new token, and login, registration and every
successful refresh issue one (`AUTH-12`). No storage, no collection, no field — consistent with the
spec's Clarifications (2026-10-16) and `SC-005`'s "no new store".

**Rationale**: binding the MAC to the family id satisfies `AUTH-11` — an attacker who learns another
session's CSRF cookie still cannot mint a token the server accepts for the victim's family, because
the family id never leaves the server and the MAC cannot be forged. Domain separation means a leaked
JWT signing key and the CSRF key are still distinct derivations, and a CSRF key compromise cannot
forge access tokens. A stateless token keeps the feature's storage footprint at zero and the
verification free of a database round-trip.

**Alternatives considered**:

- *A separate required `AUTH_CSRF_SECRET`* — rejected: it adds a required deployment secret to
  `RequiredKeys()` and doubles the rotation surface for no security gain over a derived key; the
  repository's posture is "one required JWT secret".
- *Reuse the raw JWT secret* — rejected: no domain separation between the two MAC uses.
- *Double-submit equality (header == cookie)* — rejected by `AUTH-11` ("equality between the header and
  the cookie alone MUST NOT satisfy the check"): a subdomain-controlled attacker can set a cookie, so
  equality is not authentication.
- *A stored, per-session CSRF row* — rejected by the user decision: no new store or schema.

## R3. The verification order on `/auth/refresh` and `/auth/logout`

**Decision**: a cookie-authenticated state-changing call is evaluated in this fixed order:

1. **Origin** (`internal/app`, middleware, before the handler) → absent or not allowlisted ⇒ 403
   (`AUTH-13`), before any credential work.
2. **Cookie presence** → `/auth/refresh` absent ⇒ 401 (`AUTH-06`); `/auth/logout` absent ⇒ 204 no-op.
3. **Credential resolution** by hash → unknown ⇒ refresh 401 (`AUTH-07`), logout 204 (idempotent: there
   is no session left to end); a session is now resolved.
4. **CSRF** (`internal/auth`) → the `X-CSRF-Token` header must verify against the **resolved** family ⇒
   missing, malformed or foreign ⇒ 403 with the session untouched (`AUTH-10`, `AUTH-11`).
5. **Session state** → revoked ⇒ 401 and the family is revoked (reuse, `AUTH-07`); expired ⇒ 401.
6. **Effect** → refresh rotates and issues a new refresh + CSRF + access (`AUTH-03`, `AUTH-12`); logout
   revokes the family, clears both cookies and answers 204 (`AUTH-04`, `AUTH-12`).

CSRF precedes the reuse/expiry branch because reuse detection **mutates** (revokes the family), and
`AUTH-10` requires a refused call to leave the session untouched.

**Rationale**: the order is the only one that satisfies every edge case at once — "cookie absent while
a CSRF header is present ⇒ 401, not 403" (presence before CSRF), "missing/disallowed origin ⇒ 403
before any credential work" (origin first), "CSRF invalid ⇒ 403 and the session is not modified"
(CSRF before the mutating reuse branch). Logout resolving an unknown credential to a 204 no-op
preserves today's idempotent logout and removes no state, so no CSRF check is owed for a request that
changes nothing.

**Alternatives considered**:

- *CSRF before the cookie-presence check* — rejected: it would answer 403 where the spec demands 401
  (or 204) and would reveal that no session exists behind a CSRF failure.
- *CSRF after the reuse branch* — rejected: an invalid CSRF request that presents a rotated token would
  still revoke the family, violating `AUTH-10`'s "session is left untouched".
- *Require CSRF on logout for an unknown credential* — rejected: nothing is revoked, so a 403 protects
  nothing and would make a stale client's logout fail loudly instead of idempotently.

## R4. Cookie attributes and clearing

**Decision**: two `__Host-`-prefixed cookies, contract-fixed constants in `internal/auth/cookie.go`:

| Cookie | Name | HttpOnly | Other attributes |
|---|---|---|---|
| refresh | `__Host-legnaris_refresh` | yes | `Secure`, `SameSite=Strict`, `Path=/`, no `Domain`, `Max-Age = AUTH_REFRESH_TTL` seconds |
| CSRF | `__Host-legnaris_csrf` | **no** (script-readable) | `Secure`, `SameSite=Strict`, `Path=/`, no `Domain`, `Max-Age = AUTH_REFRESH_TTL` seconds |

Clearing (logout) writes the same name and attributes with an empty value and `Max-Age=-1` (Go renders
`Max-Age=0`), so the browser drops the cookie; the attributes are never weakened for local development
(`Secure` is always set, and `http://localhost` is a secure context).

**Rationale**: `__Host-` requires exactly `Secure` + `Path=/` + no `Domain`, which is what makes the
cookie host-only for the API and maximises its protection; the attribute set is the behaviour under
test (`AUTH-01`, `AUTH-09`) and the edge case that must fail the suite when an attribute is dropped
(`SC-001`). The CSRF cookie must not be `HttpOnly` because the page has to read it and echo it in
`X-CSRF-Token` (`AUTH-09`).

**Alternatives considered**:

- *`__Secure-` prefix or no prefix* — rejected: `__Host-` is the strongest available prefix and the
  program plan names it explicitly (Parte 3.2).
- *Scoping by `Domain`* — rejected: `__Host-` forbids it, and a host-only cookie is the point.
- *A separate short `Max-Age` for the CSRF cookie* — rejected: the CSRF token is bound to the session and
  must live as long as it; rotating the session rotates it (`AUTH-12`).

## R5. Credentialed CORS reuses the existing middleware

**Decision**: extend `internal/platform/cors.Middleware` with `AllowCredentials: true` and change its
`AllowHeaders` to `Content-Type`, `Authorization`, `X-Request-ID`, `X-CSRF-Token` (dropping
`X-Refresh-Token`). No new middleware and no custom origin function: Echo v5's CORS middleware already
echoes the **exact** allowlisted origin in `Access-Control-Allow-Origin`, always adds `Vary: Origin`,
never emits `*` for the default origin function, and **refuses to construct** a config that combines
`*` with credentials.

**Rationale**: `AUTH-14`/`AUTH-15` are exactly the middleware's documented behaviour once credentials
are enabled, so the honest change is configuration, and re-using it keeps `Vary: Origin` (which a
hand-rolled middleware would have to remember) and the preflight/OPTIONS handling the router already
relies on. Advertising `X-CSRF-Token` is required or the browser never sends it (`AUTH-16`);
continuing to advertise the retired header would violate `AUTH-21`.

**Alternatives considered**:

- *Hand-rolled credentialed CORS* — rejected: it would re-implement origin matching, `Vary`, and
  preflight for no gain, and risks a wildcard regression.
- *Leaving `AllowHeaders` empty* — rejected: Echo then echoes the requested headers, which would
  advertise whatever the caller asks for, including the retired header.

## R6. The Origin/Referer guard

**Decision**: a new `internal/app/origin.go` provides a middleware, parameterised by
`CORS_ALLOWED_ORIGINS`, applied to `/auth/refresh` and `/auth/logout`. It compares the request
`Origin` (falling back to the **origin** of `Referer`: scheme + host + optional port) against the
allowlist by exact, case-insensitive match; absent or unmatched ⇒ `apperr.NewError(apperr.CodeForbidden,
MsgOriginNotAllowed, nil)` ⇒ 403, before the handler runs.

**Rationale**: `AUTH-13` requires a fail-closed decision before any credential work, so a middleware is
the right seam, and it belongs in `internal/app` because it returns `apperr` (the platform layer must
not import it). Matching exactly mirrors the CORS allowlist so one list governs both, and reading
`Referer`'s origin covers browsers that omit `Origin` while never trusting a URL's path.

**Alternatives considered**:

- *Rely on the CORS middleware alone* — rejected: CORS is a browser control that a non-browser client
  ignores; `AUTH-13` is a server-side check.
- *`sameSite`/`sec-fetch-site` header checks* — rejected: not universally sent and not the allowlist the
  spec names.
- *A platform-package middleware returning `echo.NewHTTPError(403)`* — rejected: it would lose the
  specific exported message constant `AUTH-23` requires.

## R7. Rate limiting: an in-process sliding-window limiter

**Decision**: a new leaf package `internal/platform/ratelimit` holds a `Limiter` — a mutex-guarded map
from key to the timestamps of recent attempts, with `Allow(key string, now time.Time) (allowed bool,
retryAfter time.Duration)`. An attempt older than the window is dropped on every check, and a key whose
list empties is deleted, so memory is bounded by the keys active in one window. The clock is injected
(`Now func() time.Time`) so the window boundary is testable without waiting. `internal/app/ratelimit.go`
wraps it: it builds the keys, and on a block sets `Retry-After` (whole seconds, rounded up) and returns
`apperr.NewError(apperr.CodeRateLimited, MsgRateLimited, nil)` (429), incrementing a counter.

Keys: `ip:<RealIP>` always, plus `account:<normalized email>` where an identity is available. For
`/auth/login` and `/auth/register` the email is read from the JSON body by the guard (the body is read
and restored so the handler still decodes it); for `/auth/refresh` only the IP key applies, because the
account identity is only knowable after the cookie resolves — which must not happen before a block
(`AUTH-17` says "where an account identity is available"). Threshold and window come from
`shared/env`: `AUTH_RATE_LIMIT_ATTEMPTS` (default 10) and `AUTH_RATE_LIMIT_WINDOW` (default `1m`).

**Rationale**: a sliding window answers the spec's "rolling counter window" and its "at the instant the
window frees, the key is allowed again" (the key frees when the oldest attempt ages out), and the two
prefixed key namespaces make `AUTH-19`'s independence structural. An injected clock and a pure
`Allow` keep the SUITE deterministic, and the standard library alone honours "no new dependency". The
package is deliberately domain-free so the future `/csp-report` route (F9-T011) reuses it.

**Alternatives considered**:

- *`golang.org/x/time/rate`* — rejected: it is an indirect dependency, would become direct, and a
  token bucket does not express "N attempts per window" as directly as a window counter.
- *A fixed window reset every `windowMs`* — rejected: it admits a double burst across a boundary and
  makes "the instant the window frees" a per-key moment rather than the oldest attempt aging out.
- *Rate limiting with a Redis/store backend* — rejected: the deployment is a single process and the spec
  names an in-process limiter; a store would add infrastructure.

## R8. Registration signs the browser in

**Decision**: `POST /auth/register` keeps its `201` status but the body becomes the **session body**
(access token, token type, expiry) and the response sets the refresh and CSRF cookies — exactly as
login does (`AUTH-02`). The created-user body is retired; the account is still created first and a
duplicate email still answers 409 (`CONFLICT`) under the limit.

**Rationale**: the user decision (2026-10-16) makes registration establish the session; keeping `201`
preserves the "a resource was created" contract and minimises churn for the client, which treats any
2xx as success. Creating the account before issuing the session keeps the 409 path intact.

**Alternatives considered**:

- *`200` on register* — rejected: it would drop the created-resource signal for no requirement, and the
  spec fixes neither status (`AUTH-02` speaks of body and cookie, not the code).
- *Register then require a separate login* — rejected explicitly by the clarification.

## R9. Retiring the native transport, and the artifacts that pinned it

**Decision**: delete `app.RefreshTokenHeader`, delete the CORS `refreshTokenHeader` constant and its
allowlist entry, and rewrite every Go test that pinned the native transport — in particular the
assertions that "native clients must not receive a refresh cookie" (`auth_integration_test.go:58`,
`auth_test.go:103`, `:139`, `:174`) and the preflight test that advertised the refresh header
(`cors_test.go`). The server must not read `X-Refresh-Token` on any route, so a request carrying it is
resolved **solely** by the cookie (`AUTH-21`, `AUTH-22`).

**Rationale**: `AUTH-21` requires the constant and the tests that pinned the transport to be removed or
rewritten, and the tests that assert the *absence* of a cookie now assert the wrong thing — leaving
them would fight the feature (`SC-005`).

**Out of scope**: the deprecated desktop client (`desktop/lib/api/auth_api_client.dart`) and its tests
still reference the header, and the sibling planning artifact
`.specs/features/front-end-phase-6-auth/` still describes the native transport. Neither is delivered
code for this backend feature; the desktop is retired by F10 (`web-polish-decommission`), which owns
updating or deleting those artifacts. The spec's assumption records that the desktop is not kept
working. `docs/TDD.md` is pt-BR and already carries a banner that the native path is superseded; F10
reconciles it.

**Alternatives considered**:

- *Keep the header as a fallback* — rejected: `AUTH-21` forbids reading it, and a second way to carry
  the credential would keep the old contract alive (US5's whole point).

## R10. Error taxonomy and message constants

**Decision**: `shared/apperr` gains `CodeRateLimited Code = "RATE_LIMITED"` mapped to **429** in
`NewError`. The existing `CodeUnauthorized` (401) and `CodeForbidden` (403) cover the credential and
CSRF/origin failures. New exported message constants: `auth.MsgCSRFInvalid` (403 on a missing,
malformed or foreign CSRF token), `app.MsgOriginNotAllowed` (403 on a missing/disallowed origin) and
`app.MsgRateLimited` (429). Existing `auth.MsgInvalidRefreshToken`/`MsgRefreshTokenRequired` keep the
401 vocabulary.

**Rationale**: `AUTH-23` requires every new failure mode to be a typed code with an exported message
and a documented status, and the rate-limit code to join the taxonomy. Reusing 401/403 keeps the client
error vocabulary stable; only 429 is genuinely new.

**Alternatives considered**:

- *A distinct code per CSRF/origin failure* — rejected: both are 403 "forbidden"; the message
  distinguishes them and the spec maps both to 403.
- *Returning `echo.NewHTTPError` and letting `asAppErr` map it* — rejected for the origin guard and the
  limiter: it would lose the exported message constant the principle asks for (the echoed-status path
  only has the generic HTTP text).

## R11. Configuration additions

**Decision**: `shared/env.Variables` gains `AuthRateLimitAttempts int` (`AUTH_RATE_LIMIT_ATTEMPTS`,
default `10`) and `AuthRateLimitWindow time.Duration` (`AUTH_RATE_LIMIT_WINDOW`, default `1m`), read
with the existing helpers (`lookupIntEnv`, `lookupDurationEnv`) so an absent/invalid value logs a
structured warning and falls back to the default. The cookie **names and attributes** stay
contract-fixed constants (not configuration); the refresh lifetime keeps coming from the existing
`AUTH_REFRESH_TTL` (default `720h`). `Makefile` exports the two new variables and `deploy/.env.example`
documents them.

**Rationale**: `AUTH-20` and `AUTH-24` require documented defaults with no magic numbers in the request
path, while the cookie contract must not be free configuration.

**Alternatives considered**:

- *Different thresholds for the IP key and the account key* — rejected: the spec fixes one configured
  limit per key and does not ask for per-scope tuning; one pair keeps the config small and testable.
- *Cookie names from the environment* — rejected: `AUTH-24` calls them contract-fixed, and a configurable
  cookie name would break the `__Host-` documentation and the client contract.

## R12. Logging and redaction

**Decision**: the refresh credential and the CSRF token are never placed in a response body, a URL, or
a log record. `shared/logger.redactedKeys` is extended with the two cookie names
(`__host-legnaris_refresh`, `__host-legnaris_csrf`) so a record whose key names either cookie is
redacted, and a test asserts it.

**Rationale**: `AUTH-08` and `AUTH-25` require the credential and the secret to stay out of bodies,
logs and URLs, and that redaction names the new cookies. Echo's `RequestLogger` logs the method, URI,
status and latency — not headers — and the credential now travels in a cookie header and a request
header, never in the URL, so the URL vector is closed by construction.

**Alternatives considered**:

- *Relying on the existing `_cookie`/`cookie_` substrings* — rejected: they match a key like
  `cookie_jar`, not a key named `__host-legnaris_refresh`; naming them explicitly is the requirement.

## R13. Testing strategy

**Decision**: unit suites for the pure pieces — `internal/auth/csrf_test.go` (issue/verify, foreign
family, tampered MAC, malformed/short token, constant-time compare) and
`internal/platform/ratelimit/limiter_test.go` (under limit, at limit, over limit, window frees,
independent keys, memory dropped), both with an injected clock/randomness where a value must be
controlled. Boundary suites in `internal/app` drive the wired router through `httptest`: cookie
attributes on set and clear, the session body without a refresh field, the CSRF and origin guards'
401/403/204 ordering, and 429 with `Retry-After`. `internal/auth/service_test.go` covers the session
material and the rotation order; `shared/apperr/err_test.go` and `shared/logger/logger_test.go` gain
the new mapping and redaction cases. The integration suite
(`internal/app/auth_integration_test.go`, self-skips without MongoDB) is rewritten to drive
login/register/refresh/logout through cookies and the CSRF header.

**Rationale**: constitution III and `AUTH-27` require edge cases first with the typed code and mapped
status asserted on every failure path, and the repository's shape is table-driven `testify` tests next
to the code. The three test seams the plan needs — an injectable libc clock, an injectable RNG, and
mockery mocks — already exist in the repository's conventions.

**Alternatives considered**:

- *Only integration tests* — rejected: they self-skip without MongoDB, so the CSRF and limiter rules must
  be unit-provable in CI's no-database state.
- *Only boundary tests* — rejected: the CSRF token format and the window arithmetic are cheaper and
  sharper as unit tests.

## R14. Route wiring and the Bearer routes

**Decision**: `internal/app/router.go` mounts `requireAllowedOrigin` on the refresh and logout routes
only, and the rate-limit guard on login, register and refresh only. `/sheets` and `/templates` keep
`auth.Middleware` (Bearer) and gain no cookie dependency; the access-token contract is unchanged
(`AUTH-26`).

**Rationale**: the guards protect the cookie-authenticated state-changing surface; adding them
elsewhere would either break bearer calls (which carry no cookie and no CSRF) or rate limit data
traffic the spec does not target.

**Alternatives considered**:

- *A global origin guard* — rejected: it would reject non-browser bearer calls that legitimately omit
  `Origin`.
- *Rate limiting the whole `/auth` group* — rejected: `AUTH-17` names login, registration and refresh;
  logout is a no-op for a blocked client and limiting it adds no protection.

## R15. The program plan's F6 task-to-requirement mapping

**Decision**: the plan's `F6B-T001`…`F6B-T006` map to the spec's requirement groups as follows, and the
tasks the next step generates follow this correspondence:

| Plan task | Requirements |
|---|---|
| `F6B-T001` Refresh in an HttpOnly cookie | AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06, AUTH-07, AUTH-08 |
| `F6B-T002` CSRF token | AUTH-09, AUTH-10, AUTH-11, AUTH-12 |
| `F6B-T003` Origin/Referer validation | AUTH-13 |
| `F6B-T004` Credentialed CORS | AUTH-14, AUTH-15, AUTH-16 |
| `F6B-T005` Remove the native path | AUTH-21, AUTH-22 |
| `F6B-T006` Rate limiting on `/auth/*` | AUTH-17, AUTH-18, AUTH-19, AUTH-20 |
| Cross-cutting | AUTH-23, AUTH-24, AUTH-25, AUTH-26, AUTH-27, AUTH-28 |

**Rationale**: the program plan is the feature's source of sequence and scope; stating the mapping keeps
the generated tasks grounded in the plan rather than invented, and makes a missing requirement visible.
