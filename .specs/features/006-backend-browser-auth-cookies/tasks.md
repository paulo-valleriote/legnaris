---
description: "Task list for the Backend Browser Auth Cookies feature"
---

# Tasks: Backend Browser Auth Cookies

**Input**: Design documents from `.specs/features/006-backend-browser-auth-cookies/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED here. The constitution's "behavior ships with tests" principle (NON-NEGOTIABLE), the
feature's own `AUTH-01…AUTH-28` and `AUTH-27`'s "every new failure path asserts its typed code and mapped
status", and the repo's edge-case-first rule all demand test tasks; they are written first and must fail
before the implementation they cover.

**Organization**: tasks are grouped by user story so each story is independently implementable,
testable, and demonstrable. The two P1 stories (US1 cookie transport, US2 CSRF + Origin) are the MVP and
ship together, as the spec states; the two P2 stories (US3 credentialed CORS, US4 rate limiting) and the
P3 story (US5 retire the native transport) follow.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks)
- **[Story]**: US1 … US5, per spec.md
- Every task names its exact file path

## Path Conventions

The change is confined to the Go backend: the session material, the stateless CSRF manager and the cookie
descriptors under `internal/auth/`; the controllers, the two guards and the wiring under `internal/app/`;
the credentialed CORS in `internal/platform/cors/`; the reusable limiter engine in a new
`internal/platform/ratelimit/` leaf package; the block counter in `internal/platform/metrics/`; and the
error code, config defaults and log redaction under `shared/`. Every unit/boundary test sits next to its
code (`*_test.go`, `package X_test` at the boundary), and the cookie-driven integration suite lives in
`internal/app/auth_integration_test.go` (self-skips without MongoDB). `Makefile` and `deploy/.env.example`
carry the two new documented variables. No `web/` or `desktop/` file is touched.

## Traceability

| Story | Priority | Requirements |
|---|---|---|
| US1 A browser session whose refresh credential no script can read | P1 | AUTH-01…AUTH-08 |
| US2 Cookie-authenticated auth calls cannot be driven from another site | P1 | AUTH-09…AUTH-13 |
| US3 The browser origin is trusted with credentials; nothing else is | P2 | AUTH-14, AUTH-15, AUTH-16 |
| US4 Repeated auth attempts are rate limited | P2 | AUTH-17…AUTH-20 |
| US5 The native refresh transport is retired | P3 | AUTH-21, AUTH-22 |
| Cross-cutting | — | AUTH-23, AUTH-24, AUTH-25, AUTH-26, AUTH-27, AUTH-28 |

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the configuration, error-taxonomy and redaction primitives every story consumes.

- [ ] T001 [P] Extend `shared/env/env.go` with `AuthRateLimitAttempts int` (`AUTH_RATE_LIMIT_ATTEMPTS`,
      default `10`) and `AuthRateLimitWindow time.Duration` (`AUTH_RATE_LIMIT_WINDOW`, default `1m`), read
      through the existing `lookupIntEnv`/`lookupDurationEnv` helpers so an absent or invalid value warns
      and falls back to the default; extend `shared/env/env_test.go` with the default, the
      invalid/zero/negative fallback, and the override path; export the two optional variables in
      `Makefile` and document them in `deploy/.env.example` (AUTH-20, AUTH-24)
- [ ] T002 [P] Add `CodeRateLimited Code = "RATE_LIMITED"` mapped to 429 in `shared/apperr/err.go`, and
      add its row (`{CodeRateLimited, 429}`) to the status-mapping table in `shared/apperr/err_test.go` so
      the new code can never silently fall back to 500 (AUTH-23)
- [ ] T003 [P] Extend `shared/logger/logger.go` `redactedKeys` with the two cookie names
      (`__host-legnaris_refresh`, `__host-legnaris_csrf` — literal strings, since `shared` must not import
      `internal/auth`) and add the redaction cases in `shared/logger/logger_test.go` asserting a record
      whose key names either cookie is `[REDACTED]` (AUTH-25)

**Checkpoint**: `go test ./shared/...` passes with the new defaults, the 429 mapping and the new redaction
in place.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the two pure, dependency-free building blocks every story writes through — the cookie
descriptors and the stateless CSRF manager. Both are unit-provable with no database and no wired router.

**⚠️ CRITICAL**: no user story work can begin until this phase is complete — US1 sets the cookies these
builders describe and US2 verifies the tokens this manager mints.

### Tests for the foundation ⚠️ (write first; they must fail before the implementation)

- [ ] T004 [P] Cookie-descriptor test `internal/auth/cookie_test.go` (AUTH-01, AUTH-09, AUTH-24): the set
      builders produce exactly `__Host-legnaris_refresh` (`HttpOnly`, `Secure`, `SameSite=Strict`,
      `Path=/`, no `Domain`, `Max-Age` = the passed lifetime) and `__Host-legnaris_csrf` (same, **not**
      `HttpOnly`); the clear builders produce the same name/attributes with an empty value and
      `Max-Age=-1`; a dropped `HttpOnly`/`Secure`/`SameSite=Strict`/`Path=/`/`__Host-` attribute fails the
      assertion, not the suite silently
- [ ] T005 [P] CSRF-manager test `internal/auth/csrf_test.go` (AUTH-09, AUTH-10, AUTH-11, AUTH-12), edge
      cases first: an empty/`null` token, a non-base64 value, a truncated value and a value with a flipped
      MAC byte all fail `Verify`; a token issued for `familyA` fails `Verify` against `familyB`; every
      `Issue` yields a different value (fresh nonce, so rotation changes the token); then the happy path —
      `Issue(family)` then `Verify(token, family)` succeeds, and the same token verifies twice (stateless,
      not single-use)

### Implementation for the foundation

- [ ] T006 [P] Implement `internal/auth/cookie.go` per `contracts/auth-session.contract.md` and
      `data-model.md` `Cookie descriptors`: the exported name constants
      (`RefreshCookieName = "__Host-legnaris_refresh"`, `CSRFCookieName = "__Host-legnaris_csrf"`) and the
      builders `RefreshCookie(value, maxAge)`, `CSRFCookie(value, maxAge)`, `ClearRefreshCookie()`,
      `ClearCSRFCookie()` returning `*http.Cookie` with the contract-fixed attribute set; the names and
      attributes are constants, never configuration (AUTH-01, AUTH-09, AUTH-24)
- [ ] T007 [P] Implement `internal/auth/csrf.go` per `contracts/csrf.contract.md` and `research.md` R2: the
      `CSRFTokenManager` interface (`Issue(familyID) (string, error)`, `Verify(token, familyID) error`) and
      `NewCSRFTokenManager(secret []byte)`, minting `base64url(nonce ‖ HMAC-SHA256(key, "csrf/v1" ‖ 0x00 ‖
      familyID ‖ 0x00 ‖ nonce))` with a 16-byte `crypto/rand` nonce and `key = HMAC-SHA256(secret,
      "legnaris/csrf/v1")`; `Verify` recomputes over the presented family and compares with `hmac.Equal`;
      no storage, no schema, standard library only (AUTH-09, AUTH-10, AUTH-11, AUTH-12)

**Checkpoint**: the cookie contract and the CSRF manager exist and pass with no database; every later
story can set/clear the cookies and issue/verify tokens through them.

---

## Phase 3: User Story 1 - A browser session whose refresh credential no script can read (Priority: P1) 🎯 MVP

**Goal**: login and registration answer with the access token in a session body and put the refresh
credential in an `HttpOnly` `__Host-legnaris_refresh` cookie (never in a body); refresh consumes the
cookie, rotates the credential and returns a new access token; logout revokes the family, clears the
cookie and answers 204; the refresh and logout endpoints take the credential from the cookie only, so a
value supplied in a header or body is ignored; absent/unknown/expired/rotated credentials map to the
spec's 401 (and logout's no-op 204). The CSRF cookie is issued/rotated/cleared alongside the refresh
cookie (its *verification* lands in US2 — see the cross-reference note).

**Independent Test**: drive login, registration, refresh and logout against the wired router and assert,
for each success, the exact `Set-Cookie` attributes on the refresh cookie, that the body carries an access
token and no refresh token, that a changed cookie value proves rotation, and that logout clears the cookie
and revokes the family; then drive absent, expired, revoked and reused credentials and assert the mapped
401 responses.

### Tests for User Story 1 ⚠️ (write first; they must fail before the implementation)

- [ ] T008 [P] [US1] Service test `internal/auth/service_test.go` rewritten/extended (AUTH-01, AUTH-02,
      AUTH-03, AUTH-04, AUTH-05, AUTH-07, AUTH-08, AUTH-11, AUTH-12, AUTH-23, AUTH-27), edge cases first:
      registration still creates the account and a duplicate email is still `CONFLICT` before any session
      is issued; refresh with an empty credential is `UNAUTHORIZED`; an unknown, expired and
      already-rotated credential each map to `UNAUTHORIZED` with the typed code; a reused rotated
      credential revokes the family so its successor dies too; a refused refresh touches nothing; then the
      happy paths — register and login return a `Session` carrying an access token, a raw refresh
      credential, the family-scoped `RefreshMaxAge` and a fresh CSRF token; a refresh returns a `Session`
      whose refresh credential **and** CSRF token both differ from the presented ones; logout resolves the
      session, revokes the family and returns nil
- [ ] T009 [P] [US1] Controller test `internal/app/auth_test.go` rewritten (AUTH-01, AUTH-02, AUTH-03,
      AUTH-04, AUTH-05, AUTH-06, AUTH-08, AUTH-09, AUTH-12, AUTH-22, AUTH-24, AUTH-27, SC-001), edge cases
      first: a credential supplied in `X-Refresh-Token` or in a body is ignored (the command carries the
      cookie's value, or empty); refresh with no cookie is 401 `UNAUTHORIZED`; logout with no cookie is 204
      and sets no session; a failure body never contains a credential; then the happy paths — login,
      registration (still 201) and refresh each set the refresh cookie with the full attribute set and the
      CSRF cookie (not `HttpOnly`) and return `{access_token, token_type, expires_in}` with **no**
      `refresh_token` field; logout answers 204 and clears **both** cookies (same names, empty value,
      `Max-Age=0`)
- [ ] T010 [P] [US1] Integration test `internal/app/auth_integration_test.go` rewritten
      (AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-06, AUTH-07, AUTH-12, SC-001), edge cases first: refresh
      with no cookie is 401; reusing the already-rotated credential is 401 and revokes the whole family so
      the rotated successor is 401 too; logout with no cookie is 204 and changes nothing; then the happy
      paths — registration signs in (account created + session body + cookies), login issues a parseable
      access token and the cookies, refresh changes the cookie value and returns a new access token, and
      logout clears the cookies and revokes the family

### Implementation for User Story 1

- [ ] T011 [US1] Restructure `internal/auth/service.go` per `contracts/auth-session.contract.md` and
      `research.md` R1/R8: replace `TokenPair` with `Session` (access token, `ExpiresIn`, raw
      `RefreshToken`, `RefreshMaxAge` seconds, `CSRFToken`, `UserID`) and `SessionResponse` (no refresh
      field); extend `RefreshCommand`/`LogoutCommand` with `CSRFToken`; change the `Service` interface to
      `Register/Login/Refresh → *Session` and `Logout → error`; thread a `CSRFTokenManager` into
      `NewService`; make **registration sign in** (create the account, then issue the session exactly as
      login does, keeping 409 before issuance); have `Login` and every successful `Refresh` build a
      `Session` with a freshly minted CSRF token bound to the family and rotate both the refresh and CSRF
      values together; have `Logout` revoke the family; add `MsgCSRFInvalid` to `internal/auth/errors.go`
      (AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-07, AUTH-08, AUTH-09, AUTH-11, AUTH-12, AUTH-23)
- [ ] T012 [US1] Rewrite `internal/app/auth.go` per `contracts/auth-session.contract.md` and
      `research.md` R3/R4/R8: read the refresh credential from `c.Cookie(auth.RefreshCookieName)` on
      refresh/logout (never from a header or body — AUTH-05), set `auth.RefreshCookie(...)` and
      `auth.CSRFCookie(...)` on every success from the returned `Session`, serialise `auth.SessionResponse`
      (200 on login/refresh, 201 on registration), clear both cookies and answer 204 on logout; the
      `X-CSRF-Token` header is read into the command here (its verification is US2)
      (AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06, AUTH-09, AUTH-12, AUTH-22)
- [ ] T013 [US1] Wire the new seam in `internal/app/bootstrap.go`: build the CSRF manager from the
      configured JWT secret (`auth.NewCSRFTokenManager([]byte(cfg.Variables.JWTSecret))`) and pass it to
      `auth.NewService`, then run `make mocks` to regenerate `internal/auth/mocks/` for the changed
      `Service` interface (AUTH-11, AUTH-12)
- [ ] T014 [US1] Update `internal/app/integration_test.go` (the token-pair decode helper and
      `newAuthUser`) to the session body so the bearer-route suites keep compiling and authenticate with
      the access token as before (AUTH-01, AUTH-02, AUTH-26)

**Checkpoint**: login, registration, refresh and logout work end to end through cookies with the session
body and rotation, and every credential failure maps to its typed 401 (or logout's 204); the CSRF cookie
is issued and cleared but not yet *enforced*.

---

## Phase 4: User Story 2 - Cookie-authenticated auth calls cannot be driven from another site (Priority: P1)

**Goal**: refresh and logout refuse a call whose `X-CSRF-Token` is missing, malformed, or not bound to the
session presented by the refresh cookie (403, session untouched), and refuse a call whose
`Origin`/`Referer` is absent or outside the allowlist (403, before any credential work); a legitimate
browser call carrying both the session's CSRF token and the allowed origin is accepted. The CSRF and
origin checks follow the fixed order in `research.md` R3.

**Independent Test**: with a valid refresh cookie, drive a refresh/logout with the CSRF header absent,
wrong, and minted for another session, and with the `Origin` absent, disallowed, and allowed, asserting
403 for every failure and success only for the fully valid call; then replay a token across two sessions
and assert it is refused.

### Tests for User Story 2 ⚠️ (write first; they must fail before the implementation)

- [ ] T015 [P] [US2] Origin-guard test `internal/app/origin_test.go` (AUTH-13, AUTH-23, AUTH-27, SC-002),
      edge cases first: a refresh/logout with no `Origin` and no `Referer` is 403 `FORBIDDEN` with
      `app.MsgOriginNotAllowed`; an unlisted `Origin` is 403 and **no** `Access-Control-Allow-Origin` is
      returned; a `Referer` whose origin is unlisted is 403 while one whose origin is allowlisted passes;
      a bearer route with no `Origin` is **not** rejected (the guard is scoped); then the happy path — an
      allowlisted `Origin` lets the call reach the CSRF/credential checks
- [ ] T016 [P] [US2] CSRF boundary cases added to `internal/app/auth_test.go` (AUTH-06, AUTH-10, AUTH-11,
      AUTH-12, AUTH-23, AUTH-27, SC-002), edge cases first: refresh/logout with the cookie present but the
      `X-CSRF-Token` header absent, malformed, or minted for another family each answer 403 `FORBIDDEN`
      and neither rotate nor revoke (assert the mock records no mutation); refresh with a CSRF header but
      **no** cookie answers 401 (presence before CSRF), not 403; then the happy path — a valid
      session-bound token lets refresh rotate and logout revoke

### Implementation for User Story 2

- [ ] T017 [US2] Implement the origin guard `internal/app/origin.go` per
      `contracts/cors-origin.contract.md` and `research.md` R6: `requireAllowedOrigin(origins []string)
      echo.MiddlewareFunc` comparing the request `Origin` (falling back to the **origin** of `Referer`:
      scheme + host + optional port) against the allowlist by exact, case-insensitive match; absent or
      unmatched ⇒ `apperr.NewError(apperr.CodeForbidden, MsgOriginNotAllowed, nil)` (403) before the
      handler runs; export `MsgOriginNotAllowed` (AUTH-13, AUTH-23)
- [ ] T018 [US2] Add CSRF verification to `internal/auth/service.go` per `contracts/csrf.contract.md` and
      `research.md` R3: on refresh and logout, after the cookie-presence and credential-resolution steps
      and **before** the reuse/expiry branch, `Verify` the command's CSRF token against the **resolved**
      family and refuse with `apperr.NewError(apperr.CodeForbidden, MsgCSRFInvalid, nil)` when it is
      missing, malformed, or bound to another family — leaving the session untouched; skip the check for a
      logout that resolves to no session (a 204 no-op owes no CSRF) (AUTH-10, AUTH-11, AUTH-12, AUTH-23)
- [ ] T019 [US2] Extend `internal/app/auth.go` to fill `CSRFToken` from the `X-CSRF-Token` request header
      into `RefreshCommand`/`LogoutCommand` (extends T012) (AUTH-09, AUTH-10)
- [ ] T020 [US2] Mount the origin guard in `internal/app/router.go` on `/auth/refresh` and
      `/auth/logout` only, leaving `/auth/login`, `/auth/register`, `/sheets` and `/templates` unguarded
      (AUTH-13, AUTH-26). *Depends on T017.*

**Checkpoint**: a cookie-authenticated call with a missing/foreign CSRF token or a missing/disallowed
origin is refused 403 and changes nothing, while a legitimate browser call succeeds.

---

## Phase 5: User Story 3 - The browser origin is trusted with credentials; nothing else is (Priority: P2)

**Goal**: the API answers the configured frontend origin with credentials allowed and echoes that exact
origin (never a wildcard), always with `Vary: Origin`; the preflight for the auth routes advertises
`X-CSRF-Token` and no longer the retired native refresh header; an origin outside the allowlist gets no
allow-origin header and never `*` with credentials.

**Independent Test**: send a preflight and a real request with the allowlisted origin and assert
`Access-Control-Allow-Credentials: true`, the exact `Access-Control-Allow-Origin`, `Vary: Origin`, and the
advertised `X-CSRF-Token`; then send the same from an unlisted origin and assert no allow-origin header is
returned and no wildcard appears.

### Tests for User Story 3 ⚠️ (write first; they must fail before the implementation)

- [ ] T021 [P] [US3] Rewrite the preflight/credential boundary test `internal/app/cors_test.go`
      (AUTH-14, AUTH-15, AUTH-16, AUTH-21, SC-003), edge cases first: a preflight from an **unlisted**
      origin advertises no allow-origin and no allow-headers; a request from an unlisted origin receives no
      allow-origin header and no `*` with credentials; then the happy path — a preflight and a real request
      from the allowlisted origin answer with that exact `Access-Control-Allow-Origin`,
      `Access-Control-Allow-Credentials: true`, `Vary: Origin`, and `X-CSRF-Token` in the advertised
      headers, with the retired `X-Refresh-Token` **absent**
- [ ] T022 [P] [US3] Extend `internal/platform/cors/cors_test.go` (AUTH-14, AUTH-15): a preflight from an
      allowlisted origin echoes the exact origin with credentials and `Vary: Origin`; an unlisted origin
      receives no allow-origin; the retired header is never advertised

### Implementation for User Story 3

- [ ] T023 [US3] Extend `internal/platform/cors/cors.go` per `contracts/cors-origin.contract.md` and
      `research.md` R5: set `AllowCredentials: true`, change `AllowHeaders` to `Content-Type`,
      `Authorization`, `X-Request-ID`, `X-CSRF-Token`, and **delete** the `refreshTokenHeader` constant and
      its allowlist entry; rely on Echo v5's default origin function for the exact-origin echo and
      `Vary: Origin` (it refuses to build `*` + credentials) (AUTH-14, AUTH-15, AUTH-16, AUTH-21)

**Checkpoint**: the real client can make credentialed cross-origin auth calls with the CSRF header, and
no other origin can read a response.

---

## Phase 6: User Story 4 - Repeated auth attempts are rate limited (Priority: P2)

**Goal**: login, registration and refresh are limited per client address and, where an account identity is
available, per account; the request that exceeds a key's limit is answered 429 with `Retry-After`, with no
credential verified and no token issued; one key's exhaustion never blocks another, and a key recovers
once its window frees; the cap and window come from configuration with documented defaults and every block
increments a counter.

**Independent Test**: drive login past its configured threshold from one key and assert the next response
is 429 with `Retry-After` and no token issued; assert a second key is unaffected; advance the window and
assert the key is allowed again; drive register and refresh the same way.

### Tests for User Story 4 ⚠️ (write first; they must fail before the implementation)

- [ ] T024 [P] [US4] Limiter engine test `internal/platform/ratelimit/limiter_test.go` (AUTH-17, AUTH-18,
      AUTH-19, AUTH-20), edge cases first: a fresh key is admitted; the attempt that equals the cap is
      admitted and the next is refused with a positive `retryAfter`; exhausting one key leaves another
      admitted; with an injected clock, advancing past the window admits the key again; a key whose
      attempts age out is dropped so the map does not grow unboundedly; then the happy path — repeated
      `Allow` under the cap records and returns `true`
- [ ] T025 [P] [US4] Rate-limit boundary test `internal/app/ratelimit_test.go` (AUTH-17, AUTH-18,
      AUTH-19, AUTH-20, AUTH-23, AUTH-27, SC-004), edge cases first: a different address and a different
      account are each unaffected when one key is exhausted; once the injected window frees, the same key
      is admitted again; register and refresh behave exactly like login; then the happy paths — the request
      past the limit answers 429 `RATE_LIMITED` with a `Retry-After` header, the service mock records
      **zero** calls (no credential verified, no token issued), and the block increments the counter
      exactly once

### Implementation for User Story 4

- [ ] T026 [US4] Implement the reusable limiter engine `internal/platform/ratelimit/limiter.go` per
      `contracts/rate-limit.contract.md` and `research.md` R7: a mutex-guarded map from key to the
      timestamps of the attempts inside the window, `New(max int, window time.Duration, now func()
      time.Time) *Limiter` and `Allow(key string, now time.Time) (allowed bool, retryAfter time.Duration)`
      that drops attempts older than the window and deletes a key whose list empties; standard library
      only, no `apperr` import (AUTH-17, AUTH-18, AUTH-19, AUTH-20)
- [ ] T027 [US4] Add the block counter `legnaris_auth_rate_limited_total` to
      `internal/platform/metrics/metrics.go` with an exported `RateLimitBlocked()` method (the seam the
      guard calls) and extend `internal/platform/metrics/metrics_test.go` to assert it increments
      (AUTH-20)
- [ ] T028 [US4] Implement the rate-limit guard `internal/app/ratelimit.go` per
      `contracts/rate-limit.contract.md` and `research.md` R7: `rateLimit(limiter *ratelimit.Limiter,
      observer Observer, withAccount bool) echo.MiddlewareFunc` that always checks `ip:<c.RealIP()>` and,
      on login/registration, adds `account:<normalized email>` read from the JSON body (the body is read
      and restored so the handler still decodes it); on a block it sets `Retry-After` (whole seconds,
      rounded up), calls `observer.RateLimitBlocked()`, and returns `apperr.NewError(apperr.CodeRateLimited,
      MsgRateLimited, nil)` (429) without reaching the handler; export `MsgRateLimited`
      (AUTH-17, AUTH-18, AUTH-19, AUTH-20, AUTH-23)
- [ ] T029 [US4] Build the limiter in `internal/app/bootstrap.go` from
      `AuthRateLimitAttempts`/`AuthRateLimitWindow` and mount the guard in `internal/app/router.go` on
      `/auth/login`, `/auth/register` and `/auth/refresh` only (logout and the bearer routes are not
      limited) (AUTH-17, AUTH-20, AUTH-26). *Depends on T026, T028.*

**Checkpoint**: brute force on login, registration or refresh is bounded per address and per account, with
429 + `Retry-After` and no credential work past the limit, while other keys and the data routes are
unaffected.

---

## Phase 7: User Story 5 - The native refresh transport is retired (Priority: P3)

**Goal**: no server source reads the native `X-Refresh-Token` header, the cross-origin allow-list no
longer advertises it, the exported constant and the tests that pinned the native transport (including the
ones asserting that no refresh cookie is set) are removed or rewritten, and a request that sets the header
has its outcome determined solely by the cookie.

**Independent Test**: grep the sources for the native header name and assert it is gone; assert the auth
routes ignore a refresh value supplied in that header; assert no test still asserts the absence of a
refresh cookie.

### Tests for User Story 5 ⚠️ (write first; they must fail before the implementation)

- [ ] T030 [P] [US5] Inert-header case added to `internal/app/auth_test.go` (AUTH-21, AUTH-22, SC-005): a
      request that sets `X-Refresh-Token` is resolved solely by the cookie — refresh answers 401 when the
      cookie is absent even though the header is set, and logout answers 204 — exactly as if the header
      were absent; the constant `app.RefreshTokenHeader` no longer exists for the test to reference

### Implementation for User Story 5

- [ ] T031 [US5] Remove the exported `app.RefreshTokenHeader` constant and every remaining read of it, and
      confirm the CORS `refreshTokenHeader` constant/entry removed by T023 leaves no other reference, so no
      source or allow-list advertises or reads the native transport (AUTH-21, AUTH-22)

**Checkpoint**: the native refresh path no longer exists; the refresh credential travels only in the
cookie.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: the cross-cutting requirements and the gate sweep that close the feature.

- [ ] T032 [P] Add `internal/app/cookie_scope_test.go` (AUTH-26, SC-008): `/sheets` and `/templates`
      keep working with an `Authorization: Bearer` access token, never require a cookie, and are not
      rejected by the origin or rate-limit guards when `Origin` is absent; the access-token contract is
      unchanged
- [ ] T033 [P] Requirement/English audit (AUTH-27, AUTH-28, SC-007): confirm every `AUTH-01…AUTH-28`
      citation in the delivered sources resolves to `spec.md` via `make lint-specs`, that every new
      failure path asserts its typed code and mapped status (401/403/429), that no suite is vacuous, and
      that all identifiers, comments, log messages and user-facing strings are English
- [ ] T034 Run the backend gate set — `make fmt && make lint && make vet && make lint-specs` then
      `make test-race` — and confirm it stays clean with no new dependency declared (SC-001…SC-008)
- [ ] T035 Run the eleven `quickstart.md` scenarios and record the per-scenario outcome with the feature,
      filling the `Validation outcome` section (SC-001…SC-008)
- [ ] T036 Record any plan-versus-delivered deviation (a path, a default, or a seam that changed) in the
      feature's `quickstart.md`/`research.md` rather than weakening an assertion (constitution III,
      AUTH-27)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — T001, T002 and T003 are independent files.
- **Foundational (Phase 2)**: depends on Setup; T004 (cookie descriptors) and T005 (CSRF manager tests) +
  T006/T007 are independent. Blocks every story.
- **US1 (Phase 3)**: depends on Foundational; the service restructure (T011) precedes the controller
  (T012), the bootstrap/mocks (T013) and the integration-helper update (T014).
- **US2 (Phase 4)**: depends on US1's service and controller; the origin guard (T017) precedes its mount
  (T020); the CSRF verification (T018) extends T011; the header read (T019) extends T012.
- **US3 (Phase 5)**: depends only on Setup/Foundational for its assertions; the CORS config change (T023)
  is independent of the guard (it is a different package and a different concern).
- **US4 (Phase 6)**: depends on Setup (the config) and Foundational; the engine (T026) precedes the guard
  (T028), which precedes the mount (T029); the counter (T027) precedes the guard that calls it.
- **US5 (Phase 7)**: depends on US1/US2/US3 having rewritten the tests that pinned the native transport
  (T009, T010, T021) and on T023 having dropped the CORS entry; the removal (T031) follows the inert-header
  test (T030).
- **Polish (Phase 8)**: depends on all desired stories.

### User Story Dependencies

- **US1 (P1)**: needs only Foundational — the MVP core.
- **US2 (P1)**: builds on US1's service/controller; it is the security half and ships with US1 (the spec's
  "ships together with US1"). The CSRF *issuance* lands in US1's T011/T012; the *verification* lands here.
- **US3 (P2)**: independent of US1/US2 at the package level (it is the CORS middleware), but its
  `X-CSRF-Token` assertion refers to the header the US1/US2 controller reads.
- **US4 (P2)**: independent of US1/US2/US3 except that it limits the routes US1 delivers.
- **US5 (P3)**: cleanup that removes the constant the US1/US2/US3 rewrites left unused.

### Within Each User Story

- Tests are written first and must fail before the implementation lands (constitution III, AUTH-27).
- The descriptors/manager before the service; the service before the controller; the controller before its
  guard; the engine before the guard that wraps it.
- Story complete before moving to the next priority.

### Parallel Opportunities

- T001, T002 and T003 (Setup) — different files.
- T004 (cookie test) and T005 (CSRF test) — independent test files; T006 and T007 — the implementations.
- T008, T009 and T010 (US1 tests) — independent test files.
- T015 (origin test) and T016 (CSRF boundary test) — independent files.
- T021 (app CORS test) and T022 (platform CORS test) — independent files.
- T024 (limiter test) and T025 (boundary test) — independent files.
- T032 and T033 (Polish) — independent files.

---

## Parallel Example: User Story 1

```bash
# Tests first (independent files, must fail before T011–T014):
Task: "Service test in internal/auth/service_test.go"
Task: "Controller test in internal/app/auth_test.go"
Task: "Integration test in internal/app/auth_integration_test.go"

# Then the service, the controller, the wiring and the helper:
Task: "Restructure internal/auth/service.go (Session, registration signs in, rotation)"
Task: "Rewrite internal/app/auth.go (cookie transport and session body)"
Task: "Wire the CSRF manager in internal/app/bootstrap.go and run make mocks"
Task: "Update internal/app/integration_test.go to the session body"
```

---

## Parallel Example: the two P2 stories and the P3 cleanup

```bash
# Once US1/US2 land, US3 and US4 share no file at the start:
Task: "Rewrite internal/app/cors_test.go (credentialed, exact-origin, X-CSRF-Token)"
Task: "Limiter engine test in internal/platform/ratelimit/limiter_test.go"

Task: "Extend internal/platform/cors/cors.go (AllowCredentials, X-CSRF-Token)"
Task: "Implement internal/platform/ratelimit/limiter.go"

# US5 closes the native path once the rewrites have landed:
Task: "Remove app.RefreshTokenHeader and every read in internal/app/auth.go"
```

---

## Implementation Strategy

### MVP First (US1 + US2, both P1)

1. Complete Phase 1 Setup → Phase 2 Foundational.
2. Complete Phase 3 US1: the refresh credential moves into the cookie, the session body replaces
   `TokenPair`, and registration signs in.
3. Complete Phase 4 US2: the CSRF and origin checks close the hole the cookie opens.
4. **STOP and VALIDATE** with `quickstart.md` scenarios 1–6 (the pure rules, the cookie/body contract, the
   rotation, the logout and the CSRF/origin refusals) and confirm `make test-race` is clean.
5. The browser can now hold a session no script can read and no other site can drive.

### Incremental Delivery

1. Setup + Foundational → the config, the 429 code, the redaction, the cookie descriptors and the CSRF
   manager exist.
2. US1 → the cookie transport and the session body (MVP core).
3. US2 → CSRF and origin protection (MVP security half).
4. US3 → the real client's credentialed cross-origin calls succeed.
5. US4 → the auth routes are brute-force bounded.
6. US5 → the native transport is gone.
7. Polish → the bearer-scope guard, the requirement/English audit, the gate sweep and the quickstart.

### Parallel Team Strategy

With two contributors: one takes the cookie/CSRF core (Foundational, then US1 then US2) while the other
takes the reusable limiter (Setup `T001`/`T002`, then the US4 engine and guard); US3 is a small, isolated
CORS configuration change that can land at any point, and US5 is the cleanup that follows the rewrites.
The Polish scans (T032, T033) share no file with the feature modules and can be done at any point.

---

## Notes

- [P] tasks touch different files and have no dependency on incomplete work.
- Requirement IDs cited in task descriptions (`AUTH-*`, `SC-*`) are defined in `spec.md`; the delivered
  source must cite the same IDs so `make lint-specs` can trace them (it already scans `*.go`).
- Write tests first (T004, T005, T008, T009, T010, T015, T016, T021, T022, T024, T025, T030) and confirm
  they fail before implementing.
- Commit per task or per logical group, with the tests in the same commit (constitution III). Run
  `make mocks` after the `auth.Service` interface change (T013).
- Do not weaken an assertion to make a gate pass; retarget it to the story that owns the subject, or amend
  the spec and record the decision instead. In particular, the tests that asserted "native clients must
  not receive a refresh cookie" now assert the opposite and are rewritten in T009/T010/T021 (SC-005).
- The retired native transport's remaining references in `desktop/` and
  `.specs/features/front-end-phase-6-auth/` are **out of scope** here (research R9; F10
  `web-polish-decommission` owns them); the Go sources and the CORS allow-list are the deliverable.
- Keep each concern in one place: the session material and the CSRF token in `internal/auth`, the cookie
  descriptors with the credential that owns them, the two guards that return `apperr` in `internal/app`,
  the credentialed CORS in the existing `internal/platform/cors`, and the limiter engine in the
  standard-library-only `internal/platform/ratelimit` leaf package.
