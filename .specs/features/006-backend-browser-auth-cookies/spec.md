# Feature Specification: Backend Browser Auth Cookies

**Feature Branch**: `006-backend-browser-auth-cookies`

**Created**: 2026-10-16

**Status**: Draft

**Input**: User description: "Backend browser auth: move the refresh credential into an HttpOnly cookie, add CSRF and Origin protection, credentialed CORS, and rate limiting on the auth routes"

## Clarifications

### Session 2026-10-16

- Q: Should registering a new account also sign the browser in (issue the access token and set the
  refresh cookie), or stay a pure account-creation step? → A: Register signs the browser in (user
  decision, 2026-10-16): the registration response carries the access token in the body and the
  refresh cookie, replacing the created-user body (`AUTH-02`). The plan's `register` statement is
  honored over today's contract; the desktop's register-then-login composition is not preserved.
- Q: How should the server validate the CSRF token "against the server, not blind equality"? → A:
  Stateless signed token (user decision, 2026-10-16): the server mints an HMAC-signed token bound to
  the refresh session and a nonce and verifies it with the server secret — no new storage or schema
  change (`AUTH-09`…`AUTH-12`).
- Q: On cookie-authenticated state-changing auth requests, what should the server do when the
  `Origin`/`Referer` header is absent? → A: Reject (fail closed) (user decision, 2026-10-16): a
  missing `Origin`/`Referer` is answered 403, so only the configured frontend origin can exercise the
  cookie endpoints (`AUTH-13`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A browser session whose refresh credential no script can read (Priority: P1)

A person signs in from a browser. The server answers with a short-lived access token the page keeps
in memory and puts the long-lived refresh credential in a cookie the page's JavaScript cannot read;
the refresh credential never travels in a response body. When the access token lapses the page
silently refreshes: the browser sends the cookie, the server rotates the credential and returns a
fresh access token. Signing out revokes the session and removes the cookie. A person registering a
new account is signed in by the same act.

**Why this priority**: it is the whole reason this backend feature exists — the browser client (the
product's only client once the desktop is retired) cannot hold a session safely until the refresh
credential stops being JavaScript-readable and stops appearing in bodies.

**Independent Test**: drive login, registration, refresh and logout against the wired router and
assert, for each success, the exact `Set-Cookie` attributes on the refresh cookie, that the body
carries an access token and no refresh token, that a changed cookie value proves rotation, and that
logout clears the cookie and revokes the family; then drive absent, expired, revoked and reused
credentials and assert the mapped 401 responses.

**Acceptance Scenarios**:

1. **Given** valid credentials, **When** login succeeds, **Then** the response body carries the
   access token and no refresh token, and it sets a `HttpOnly`, `Secure`, `SameSite=Strict`,
   `__Host-`-prefixed, `Path=/`, `Domain`-less refresh cookie with the configured lifetime.
   (AUTH-01)
2. **Given** a valid new account, **When** registration succeeds, **Then** the account is created and
   the response establishes the session exactly as a login does, carrying the access token and
   setting the refresh cookie instead of the created-user body. (AUTH-02)
3. **Given** a valid refresh cookie, **When** refresh succeeds, **Then** the presented credential is
   consumed, a successor in the same family is minted, the new cookie is set, and a new access token
   is returned in the body with no refresh token. (AUTH-03)
4. **Given** a valid session, **When** logout succeeds, **Then** the session's token family is
   revoked, the refresh cookie is cleared, and the response is 204. (AUTH-04)
5. **Given** a request that supplies a refresh credential in a header or body instead of the cookie,
   **When** it reaches refresh or logout, **Then** that value is ignored. (AUTH-05)
6. **Given** no refresh cookie, **When** refresh is requested, **Then** it is answered 401; **When**
   logout is requested, **Then** it is answered 204 as a no-op. (AUTH-06)
7. **Given** a credential that was already rotated, **When** it is presented again, **Then** it is
   answered 401 and the whole family is revoked. (AUTH-07)

---

### User Story 2 - Cookie-authenticated auth calls cannot be driven from another site (Priority: P1)

Because the cookie is now an ambient credential the browser attaches automatically, the server no
longer accepts a refresh or logout just because the cookie is present. Alongside the refresh cookie
it issues a second token the page can read and echo back in a request header, cryptographically bound
to the session; a call that omits it, alters it, or presents one minted for a different session is
refused, as is a call whose `Origin`/`Referer` is missing or outside the configured frontend origin.

**Why this priority**: it is the security half of moving the credential into a cookie — without it,
turning the cookie on would introduce a CSRF hole. It ships together with US1.

**Independent Test**: with a valid refresh cookie, drive a refresh/logout with the CSRF header
absent, wrong, and minted for another session, and with the `Origin` absent, disallowed, and allowed,
asserting 403 for every failure and success only for the fully valid call; then replay a token across
two sessions and assert it is refused.

**Acceptance Scenarios**:

1. **Given** a session is established or rotated, **When** the response is produced, **Then** it also
   sets a script-readable (`non-HttpOnly`), `Secure`, `SameSite=Strict`, `__Host-`-prefixed CSRF
   cookie whose value the client echoes in an `X-CSRF-Token` header. (AUTH-09)
2. **Given** a valid refresh cookie, **When** refresh or logout is called with a missing, malformed,
   or not-session-bound `X-CSRF-Token`, **Then** it is refused with 403 and the session is left
   untouched. (AUTH-10)
3. **Given** a CSRF token minted for another session, **When** it is presented for this session,
   **Then** it is refused with 403. (AUTH-11)
4. **Given** a legitimate browser call carrying both the session's CSRF token and the allowed origin,
   **When** it reaches refresh or logout, **Then** it is accepted. (AUTH-10, AUTH-13)
5. **Given** a rotation, **When** it succeeds, **Then** the CSRF token is rotated with the refresh
   cookie; **Given** logout, **Then** the CSRF cookie is cleared. (AUTH-12)
6. **Given** a missing or disallowed `Origin`/`Referer`, **When** refresh or logout is called, **Then**
   it is refused with 403 before any credential work. (AUTH-13)

---

### User Story 3 - The browser origin is trusted with credentials; nothing else is (Priority: P2)

The frontend lives on one host and the API on another under the same registrable domain, so the
browser's requests are cross-origin. The API answers the configured frontend origin with credentials
allowed and echoes that exact origin — never a wildcard — so the browser sends and accepts the
session cookie, while a request from any other origin is left unreadable.

**Why this priority**: without credentialed, exact-origin CORS the cookie never reaches the server
from the real client; it is a precondition for US1 working end to end, but the cookie and CSRF
behaviour can be built and tested before CORS is finalized.

**Independent Test**: send a preflight and a real request with the allowlisted origin and assert
`Access-Control-Allow-Credentials: true`, the exact `Access-Control-Allow-Origin`, `Vary: Origin`,
and the advertised `X-CSRF-Token`; then send the same from an unlisted origin and assert no
allow-origin header is returned and no wildcard appears.

**Acceptance Scenarios**:

1. **Given** an allowlisted origin, **When** a cross-origin request is answered, **Then** the response
   echoes that exact origin in `Access-Control-Allow-Origin`, sends
   `Access-Control-Allow-Credentials: true`, and includes `Vary: Origin`. (AUTH-14)
2. **Given** an origin outside the allowlist, **When** the request is answered, **Then** no
   allow-origin header is returned and `*` is never sent with credentials. (AUTH-15)
3. **Given** the cookie-authenticated auth routes, **When** a preflight is answered, **Then** it
   advertises the `X-CSRF-Token` request header and no longer advertises the native refresh header.
   (AUTH-16)

---

### User Story 4 - Repeated auth attempts are rate limited (Priority: P2)

A person mistypes a password a few times and is told to wait; an attacker hammering login, register
or refresh from one address or against one account is stopped after a bounded number of tries in a
window, answered with a clear retry hint, and never gets a credential checked or a token issued while
blocked.

**Why this priority**: turning the cookie on makes the auth routes the primary remote target; the
limiter bounds brute force, but the feature is usable (and secure) without it, so it follows the
cookie and CSRF work.

**Independent Test**: drive login past its configured threshold from one key and assert the next
response is 429 with `Retry-After` and no token issued; assert a second key is unaffected; advance
the window and assert the key is allowed again; drive register and refresh the same way.

**Acceptance Scenarios**:

1. **Given** repeated login, registration, or refresh attempts, **When** a key exceeds its configured
   limit in the window, **Then** the next attempt is answered 429 with `Retry-After` and no
   credential is verified and no token is issued. (AUTH-17, AUTH-18)
2. **Given** one key at its limit, **When** another key makes a request, **Then** it is unaffected;
   **When** the window frees, the first key is allowed again. (AUTH-19)
3. **Given** the configured thresholds, **When** the server boots, **Then** the values come from
   configuration with documented defaults, and each block increments a counter. (AUTH-20)

---

### User Story 5 - The native refresh transport is retired (Priority: P3)

The desktop client is deprecated, so the header-based refresh transport that existed only for native
clients is removed: the refresh credential is read from the cookie only, the header is dropped from
the cross-origin allow-list, and the tests and constants that pinned the native path — including the
ones asserting a refresh cookie must not be set — are rewritten or deleted.

**Why this priority**: it is cleanup that removes a second, conflicting way to carry the refresh
credential; leaving it in place would keep the old contract alive and confuse the browser model, but
no user-facing journey depends on it.

**Independent Test**: grep the sources for the native header name and assert it is gone; assert the
auth routes ignore a refresh value supplied in that header; assert no test still asserts the absence
of a refresh cookie.

**Acceptance Scenarios**:

1. **Given** the delivered sources, **When** they are searched for the native refresh header
   transport, **Then** no source reads it, the cross-origin allow-list no longer advertises it, and
   the constant and the tests that pinned it are removed or rewritten. (AUTH-21)
2. **Given** a request that sets the native refresh header, **When** it reaches refresh or logout,
   **Then** the header has no effect on the outcome. (AUTH-22)

---

### Edge Cases

- **Refresh cookie absent on refresh**: answered 401, no cookie set; on logout it is a 204 no-op
  (AUTH-06).
- **Refresh cookie unknown, expired, revoked, or already rotated**: answered 401; a reused rotated
  credential additionally revokes the family, so its successor is dead too (AUTH-07).
- **Concurrent refresh with the same cookie**: exactly one rotation wins; the loser is answered 401
  and the family is not destroyed by the loser (existing behaviour preserved).
- **CSRF header absent / malformed / from another session**: answered 403; the session is not
  modified (AUTH-10, AUTH-11).
- **Refresh cookie absent while a CSRF header is present**: answered 401 (no session to act on)
  rather than 403.
- **Origin/Referer absent on a cookie-authenticated state-changing call**: answered 403 (fail
  closed), before any credential work (AUTH-13).
- **Origin present but unlisted**: answered 403, and no `Access-Control-Allow-Origin` is returned
  (AUTH-13, AUTH-15).
- **Preflight from an unlisted origin**: no allow-origin and no allow-headers are advertised
  (AUTH-15, AUTH-16).
- **Cookie attribute dropped**: a regression that drops `HttpOnly`, `Secure`, `SameSite=Strict`,
  `Path=/`, or the `__Host-` prefix MUST fail the suite, not pass silently (AUTH-01, SC-001).
- **Rotation**: every successful refresh MUST change both the refresh cookie value and the CSRF token
  (AUTH-03, AUTH-12).
- **Rate limit before credentials**: a blocked login MUST NOT compare the password, hit the user
  store, or mint a token; an invalid password under the limit is still answered 401 as today
  (AUTH-18).
- **Independent keys**: exhausting the per-IP key MUST NOT block a different IP; exhausting an
  account key MUST NOT block a different account from the same IP (AUTH-19).
- **Window boundary**: at the instant the window frees, the key is allowed again (AUTH-19).
- **Duplicate registration**: a duplicate email under the limit still answers 409 exactly as today;
  registration now also signs in when it succeeds (AUTH-02).
- **Bearer routes unaffected**: `/sheets` and `/templates` keep working with a Bearer access token
  and never require a cookie (AUTH-26).
- **Credential leakage**: neither the refresh credential nor the CSRF secret appears in a response
  body, a log line, or a URL (AUTH-08, AUTH-25).
- **Local development**: over `http://localhost` the browser treats the origin as secure and still
  sends `Secure`/`__Host-` cookies; the attributes are never weakened for development.

## Requirements *(mandatory)*

### Functional Requirements

**A browser session carried by an HttpOnly refresh cookie**

- **AUTH-01**: WHEN a login succeeds, the server MUST return the access token in the response body
  and MUST set the refresh credential as an `HttpOnly`, `Secure`, `SameSite=Strict`, `__Host-`
  prefixed cookie named on the `__Host-legnaris_` prefix with `Path=/`, no `Domain`, and a `Max-Age`
  equal to the configured refresh lifetime; the response body MUST NOT contain the refresh
  credential. **Superseded for the gate-enabled behavior by `TFA-02`** (`.specs/features/018-auth-2fa-session-gate/spec.md`): while the rollout gate is on, a login answers a code step instead of a session; with the gate off this requirement holds unchanged.
- **AUTH-02**: WHEN a registration succeeds, the server MUST create the account and establish the
  session exactly as a login does — access token in the body, refresh cookie set — in place of
  returning only the created account, and MUST NOT return the refresh credential in the body.
  **Superseded for the gate-enabled behavior by `TFA-01`** (`.specs/features/018-auth-2fa-session-gate/spec.md`): while the rollout gate is on, a registration creates the unverified account and answers a code step instead of a session; with the gate off this requirement holds unchanged.
- **AUTH-03**: WHEN a refresh succeeds, the server MUST consume the presented credential, mint a
  successor in the same family, set the new refresh cookie, and return a new access token in the
  body; the body MUST NOT contain the refresh credential.
- **AUTH-04**: WHEN logout is requested with a session, the server MUST revoke the session's token
  family, clear the refresh cookie, and answer 204.
- **AUTH-05**: The refresh and logout endpoints MUST take the refresh credential from the request
  cookie only; a credential supplied in a request header or body MUST be ignored.
- **AUTH-06**: IF the refresh cookie is absent, THEN refresh MUST be answered 401 and logout MUST be
  answered 204 as a no-op.
- **AUTH-07**: IF the refresh credential is unknown, expired, revoked, or has already been rotated,
  THEN refresh MUST be answered 401, and a reused rotated credential MUST additionally revoke the
  whole family.
- **AUTH-08**: The refresh credential MUST NOT appear in any response body, any log record, or any
  URL.

**CSRF and Origin protection for cookie-authenticated requests**

- **AUTH-09**: WHEN a session is established or rotated (login, registration, refresh), the server
  MUST also set a CSRF token as a script-readable (not `HttpOnly`), `Secure`, `SameSite=Strict`,
  `__Host-legnaris_`-prefixed cookie distinct from the refresh cookie, whose value the client echoes
  in an `X-CSRF-Token` request header.
- **AUTH-10**: A cookie-authenticated state-changing auth request (`POST /auth/refresh`,
  `POST /auth/logout`) MUST be refused with 403 unless the `X-CSRF-Token` header carries a token the
  server itself issued and bound to the session presented by the refresh cookie; a missing header, a
  malformed value, or a token not bound to that session MUST each be refused with 403.
- **AUTH-11**: The CSRF token MUST be unforgeable without the server secret and MUST be
  cryptographically bound to the session, so a token issued for one session MUST NOT be accepted for
  another and equality between the header and the cookie alone MUST NOT satisfy the check.
- **AUTH-12**: On every successful rotation the CSRF token MUST be rotated together with the refresh
  cookie, and logout MUST clear the CSRF cookie.
- **AUTH-13**: A cookie-authenticated state-changing auth request whose `Origin` (falling back to
  `Referer`) is absent or not in the configured allowlist MUST be refused with 403 before any
  credential work.

**Credentialed cross-origin access**

- **AUTH-14**: A cross-origin request from an allowlisted origin MUST be answered with that exact
  origin echoed in `Access-Control-Allow-Origin`, `Access-Control-Allow-Credentials: true`, and
  `Vary: Origin`.
- **AUTH-15**: A request from an origin outside the allowlist MUST NOT receive an
  `Access-Control-Allow-Origin` header, and `*` MUST NEVER be sent together with credentials.
- **AUTH-16**: The preflight for the cookie-authenticated auth routes MUST advertise the
  `X-CSRF-Token` request header and MUST NOT advertise the retired native refresh header.

**Rate limiting on the auth routes**

- **AUTH-17**: Login, registration, and refresh MUST be rate limited per client address and, where an
  account identity is available, per account, so that attempts on one key cannot exceed a configured
  number within a configured window.
- **AUTH-18**: WHEN a key exceeds its limit, the request MUST be answered 429 with a `Retry-After`
  header, and no credential MUST be verified and no token issued for that request.
- **AUTH-19**: Limits MUST be independent per key — one key's exhaustion MUST NOT block another key —
  and a key MUST be allowed again once its window frees.
- **AUTH-20**: The thresholds and window MUST come from configuration with documented defaults (no
  magic numbers in the request path), and each block MUST increment a counter.

**Retiring the native refresh transport**

- **AUTH-21**: The server MUST NOT read the native `X-Refresh-Token` header on any route, MUST remove
  it from the cross-origin allowed headers, and MUST remove or rewrite the exported constant and
  every **backend** test that pinned the native transport — including the tests asserting that no
  refresh cookie is set (the deprecated `desktop/` client and the superseded planning artifact are out
  of scope; see SC-005 and research R9).
- **AUTH-22**: A request that sets the retired native refresh header MUST have its outcome determined
  solely by the cookie, exactly as if the header were absent.

**Cross-cutting**

- **AUTH-23**: Every new failure mode MUST be a typed `apperr` with an exported message constant and
  a documented HTTP status — 401 (absent/invalid/expired/reused refresh credential), 403
  (missing/invalid/unbound CSRF token; missing or disallowed origin), 429 (rate limited) — and the
  rate-limit code MUST be added to the error taxonomy and mapped to 429.
- **AUTH-24**: Cookie names, cookie attributes, the refresh lifetime, and the rate-limit
  thresholds/window MUST derive from configuration (`shared/env`) with documented defaults; the
  cookie names and attributes are contract-fixed, not free configuration.
- **AUTH-25**: The refresh credential and the CSRF secret MUST never be logged, and the log redaction
  MUST cover the new cookie names.
- **AUTH-26**: The `/sheets` and `/templates` routes MUST remain Bearer-only and MUST NOT depend on
  the cookie, and the access-token contract MUST be unchanged.
- **AUTH-27**: Every requirement MUST ship tests — edge cases first, then a representative happy
  path — asserting the typed error code and the mapped HTTP status on every failure path.
- **AUTH-28**: All identifiers, comments, log messages, and user-facing strings MUST be written in
  English.

### Key Entities

- **Refresh session (token family)**: the rotated chain of refresh credentials for one sign-in; carries
  the family identity, the current credential's hash, its expiry, and the revoked state. The family is
  revoked on logout and on detected reuse.
- **Refresh cookie**: the `__Host-`-prefixed, `HttpOnly`, `Secure`, `SameSite=Strict`, host-only
  cookie that carries the session credential on the auth routes; set on login, registration and
  rotation, cleared on logout.
- **CSRF token**: a server-signed, session-bound value mirrored to a script-readable cookie and echoed
  in the `X-CSRF-Token` header; rotates with the session, cleared on logout.
- **Allowed origin set**: the configured frontend origins permitted to make credentialed cross-origin
  requests; matched exactly, never wildcarded.
- **Rate-limit key**: the per-address or per-account identity together with its rolling counter window;
  carries the current count, the configured limit, and the time the window frees.
- **Access token**: the unchanged short-lived bearer credential the client keeps in memory and sends on
  the data routes; out of scope for how it is stored client-side.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of successful login, registration, and refresh responses carry the refresh
  credential only in the HttpOnly cookie and 0 carry it in the body; a dropped cookie attribute fails
  the suite.
- **SC-002**: 0 requests with a missing, malformed, or unbound CSRF token and 0 requests with a
  missing or disallowed origin are accepted on refresh or logout — each is answered 403.
- **SC-003**: 0 responses to an allowlisted origin carry `Access-Control-Allow-Origin: *`, and 100% of
  credentialed cross-origin preflights for the auth routes succeed and advertise `X-CSRF-Token`.
- **SC-004**: In any configured window, the request that exceeds a key's limit is answered 429 with
  `Retry-After`, 0 tokens are issued for it, and 0 other keys are affected.
- **SC-005**: 0 occurrences of the native refresh-header transport remain in the delivered backend
  sources (`internal/`, `shared/`, `Makefile`, `deploy/`), and 0 tests still assert the absence of a
  refresh cookie. The deprecated `desktop/` client and the superseded
  `.specs/features/front-end-phase-6-auth/` artifacts still name the header; they are retired by F10
  (`web-polish-decommission`) and are out of scope here (research R9).
- **SC-006**: 0 refresh credentials and 0 CSRF secrets appear in logs, response bodies, or URLs.
- **SC-007**: 100% of new failure paths assert their typed code and mapped status, and 0 suites are
  non-vacuous.
- **SC-008**: The `/sheets` and `/templates` suites stay green with 0 dependency on the cookie.

## Assumptions

- The deployed hosts are the web client on `app.legnaris.com.br` and the API on
  `api.legnaris.com.br` (`deploy/.env.example`, and the corrected program plan); the allowlist is
  the configured `CORS_ALLOWED_ORIGINS`, matched as exact origins. Because both hosts share the
  registrable domain, the request is cross-origin but same-site, so `SameSite=Strict` still sends the
  cookie.
- Every non-local environment serves over HTTPS. The `__Host-` prefix requires `Secure`, `Path=/`, and
  no `Domain`, so the cookie is host-only for the API; local development uses `http://localhost`,
  which browsers treat as a secure context, and the attributes are never weakened.
- The desktop client is deprecated; this feature removes the native refresh transport and does not
  keep the desktop working. The plan's blocking prerequisite is satisfied by the web session feature
  (F7) building on this contract.
- Registration signs the browser in (user decision, 2026-10-16): the registration response body
  becomes the session payload carrying the access token, and the created-user body is retired.
- The CSRF token is stateless and server-signed, bound to the refresh session and a nonce; no new
  store, collection, or field is introduced (user decision, 2026-10-16).
- A missing `Origin`/`Referer` on a cookie-authenticated state-changing auth request fails closed
  (user decision, 2026-10-16).
- Rate limiting is provided by a reusable in-process limiter whose thresholds and window are
  configurable with documented defaults; the concrete numbers are an implementation default refined
  during planning, and the future CSP-report route (F9-T011) reuses the same limiter.
- `/sheets` and `/templates` keep Bearer-only auth; the cookie is never required there, so no CSRF
  concern is introduced outside the auth routes.
- No new third-party dependency is expected: the work uses the standard library and the existing Echo
  stack, and follows the repository's typed-error, config, and test conventions.
- The access token stays in memory in the client, and its storage is owned by the web session feature
  (F7); this feature only stops returning the refresh credential and stops reading it from storage.
- Continuous integration already runs the backend suite and the requirement-ID lint covers Go
  sources, so this feature introduces no new gate.
- All identifiers, comments, log messages, and user-facing strings remain English-only, per the
  project constitution.
