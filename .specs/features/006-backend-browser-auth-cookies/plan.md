# Implementation Plan: Backend Browser Auth Cookies

**Branch**: `006-backend-browser-auth-cookies` | **Date**: 2026-10-16 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/006-backend-browser-auth-cookies/spec.md`

**Program plan**: `docs/plan-web-pwa.md` (Parte B, feature F6 — `backend-browser-auth-cookies`) — the
umbrella plan this feature implements. It is the **blocking prerequisite** of F7 (`web-auth-session`).

## Summary

Move the refresh credential out of the response body and into an `HttpOnly` `__Host-legnaris_refresh`
cookie, so the browser client can hold a session no script can read; retire the native
`X-Refresh-Token` header transport the deprecated desktop used; issue a server-signed, session-bound
CSRF token mirrored to a script-readable cookie and echoed in `X-CSRF-Token`; refuse a
cookie-authenticated state-changing auth call whose `Origin`/`Referer` is missing or outside the
configured allowlist; answer the configured frontend origin with credentialed, exact-origin CORS; and
rate limit the auth routes per client address and, where an account identity is available, per
account. Registration now signs the browser in — the created-user body is replaced by the session
body (access token + cookie). The refresh credential and the CSRF secret never appear in a body, a
log, or a URL.

The change is confined to the Go backend: `internal/auth` (session material, stateless CSRF token,
cookie descriptors), a new `internal/platform/ratelimit` package, `internal/app` (controller, cookie
wiring, origin guard, rate-limit wiring, error mapping), `internal/platform/cors` (credentialed CORS,
header allowlist), `shared/apperr` (one new code), `shared/env` (two new documented defaults), and the
`shared/logger` redaction list. **No new third-party dependency**: the CSRF token is HMAC-SHA256 over
the standard library, the limiter is an in-process mutex-guarded counter, and cookies use `net/http`.

## Technical Context

**Language/Version**: Go 1.26.4 (`go.mod` `go 1.26.4`; `go version` confirms 1.26.4 linux/amd64) on
Echo v5.3.1 (`github.com/labstack/echo/v5 v5.3.1`), the pins the repository already carries. No
toolchain change.

**Primary Dependencies**: the existing stack only — Echo v5 (`net/http` cookies via
`echo.Context.SetCookie`/`Cookie`), the Echo CORS middleware (`middleware.CORSConfig`, which already
echoes the exact origin and always adds `Vary: Origin`), `golang-jwt/jwt/v5` (unchanged access-token
contract), `prometheus/client_golang` (one new counter), `go.mongodb.org/mongo-driver/v2` (unchanged),
`testify` and mockery-generated mocks. The CSRF token uses `crypto/hmac`, `crypto/sha256`,
`crypto/rand`, `encoding/base64`; the limiter uses `sync`, `time` and `net`. **No new dependency** is
declared, and `golang.org/x/time` (indirect) is not adopted.

**Storage**: unchanged — the existing `refresh_tokens` collection (family id, credential hash,
expiry, revoked flag, `replaced_by`) is the session; the CSRF token is **stateless** and adds no
collection or field (spec Clarifications, 2026-10-16), and the rate-limit window lives in process
memory. No credential is ever placed in a URL or a response body's field.

**Testing**: `go test ./...` and `go test -race ./...` (the constitution requires the race detector to
stay clean). Table-driven suites with `t.Run` and `Happy path:` / `Edge:` / `Error:` prefixes, using
`require` for preconditions and `assert` otherwise; mockery-generated mocks (`NewMockX(t)`, `make
mocks` after the interface change). Unit suites for the CSRF token and the limiter; controller/HTTP
suites in `internal/app` asserting cookie attributes, body shape, CORS, origin and 429 at the
boundary; rewritten integration suites in `internal/app/auth_integration_test.go` (self-skip without
MongoDB) that drive login/register/refresh/logout through cookies; a `shared/apperr` mapping test and a
`shared/logger` redaction test. The Go source scan (`scripts/check_requirement_ids.py`) already covers
`*.go`, so every `AUTH-*` citation resolves without a lint change.

**Target Platform**: the Go HTTP API behind Traefik at `api.legnaris.com.br`; the browser client is
served at `app.legnaris.com.br`, so requests are cross-origin but same-site (shared registrable domain
`legnaris.com.br`), which is why `SameSite=Strict` still sends the cookie (spec Assumptions). Every
non-local environment is HTTPS; local development uses `http://localhost`, which browsers treat as a
secure context, and the cookie attributes are **never** weakened for development.

**Project Type**: backend web service inside the existing Go monorepo; the `web/` client is not built
here (F7 consumes this contract), and the deprecated `desktop/` is not modified.

**Performance Goals**: not throughput; the limiter bounds brute force. Two keys per request where an
account identity exists (client address and account), an in-memory sliding window per key, and a
constant-time CSRF verification (`hmac.Equal`). The limiter must not leak memory: a key whose window
has emptied is dropped.

**Constraints**: cookie names and attributes are **contract-fixed** constants while the refresh
lifetime and the rate-limit thresholds/window come from configuration with documented defaults
(`shared/env`); every new failure mode is a typed `apperr` with an exported message constant and a
documented status (401/403/429) (AUTH-23, AUTH-24); the refresh credential and CSRF secret are never
logged and the redaction list covers the new cookie names (AUTH-08, AUTH-25); `/sheets` and
`/templates` stay Bearer-only (AUTH-26); all identifiers, comments, log messages and user-facing
strings are English (AUTH-28); layering stays `app → auth → shared` with `platform/*` middleware not
importing `apperr` (the two new guards live at the `internal/app` boundary, where `apperr` is allowed).
No new CI gate is introduced: `ci-lint` (`golangci-lint` + `go test -race`) and `ci-spec-lint` cover
the change as they stand.

**Scale/Scope**: ~10 changed/added Go modules across `internal/auth`, `internal/app`,
`internal/platform/{cors,ratelimit}`, `shared/{apperr,env,logger}`; 5 user stories (2 P1, 2 P2, 1 P3)
and 28 requirements; roughly 8 new or rewritten test suites plus the rewritten integration suite; 4
contract documents; small edits to `Makefile` (env export) and `deploy/.env.example` (documented
defaults).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | Every new failure mode is a typed `apperr` — 401 unknown/expired/rotated credential, 403 missing/invalid/unbound CSRF or missing/disallowed origin, 429 rate limited (the new `apperr.CodeRateLimited`) — with an exported message constant in the owning package and the mapped status asserted at the HTTP boundary. Reuse detection keeps its existing typed path; the two guards return `apperr` from `internal/app` (allowed) and the CSRF check stays in `internal/auth` (allowed). | PASS |
| II. Canonical Identifiers | No new entity id; the CSRF nonce is random bytes, not an entity id, and the session family id keeps `idgen.New`. Inbound ids are untouched. | PASS (not exercised) |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | Every `AUTH-*` requirement lands with a test in the same commit, edge cases first (absent/unknown/expired/rotated cookie, missing/malformed/foreign CSRF, missing/disallowed origin, over-limit key, window boundary, dropped cookie attribute, concurrent rotation) then a representative happy path; failure paths assert the typed code and the mapped status; `go test -race` stays clean. | PASS |
| IV. English-Only Source | All identifiers, comments, log messages and user-facing strings are English; no pt-BR is introduced. | PASS |
| V. Layered Architecture | `app → auth → shared`; the new `internal/platform/ratelimit` is a leaf middleware package importing only the standard library, and `internal/platform/cors` keeps importing Echo/middleware only. JSON decoding stays in `shared/json`; logging stays in `shared/logger`. No upward import. | PASS |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | This principle mandates exactly this feature: the refresh credential becomes an `HttpOnly` `Secure` `SameSite=Strict` cookie, the access token stays in memory (client-side, F7), and every state-changing cookie-authenticated route is CSRF-protected. No gate is changed or weakened; no `continue-on-error`; no secret is committed. | PASS |
| VII. Installable, Offline-Capable Web Client | The feature is the backend precondition of the client principle: it makes the browser the only client by retiring the native transport (AUTH-21) and enabling credentialed cross-origin calls (AUTH-14…AUTH-16). It adds no service-worker or offline surface (none exists server-side). | PASS (enabling) |
| Additional Constraints — Stack | Go on Echo with MongoDB, entry point `cmd/api.go`, configuration from `shared/env`, `testify` + mockery tests — all unchanged; the feature adds no dependency. | PASS |
| Additional Constraints — Spec corpus and traceability | `AUTH-*` is defined in `spec.md`; `scripts/check_requirement_ids.py` already scans `*.go`, so citations in the delivered sources are gated without further work. | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card. Recorded for the PR; this planning artifact is not itself a card claim. | PASS (noted) |
| Development Workflow | The documented backend PR sequence (`make fmt && make lint && make vet && make lint-specs`, then `make test-race`) covers the change; no Makefile target is added (the env-export line gains the two new optional variables). | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps each concern in one place: the
session material and the CSRF token in `internal/auth`, the cookie descriptors with the credential
that owns them, the credentialed CORS in the existing `internal/platform/cors`, the limiter engine in
a leaf `internal/platform/ratelimit` package, and the origin and rate-limit *guards* — which need
`apperr` — in `internal/app` at the boundary. The CSRF token is stateless (no storage, no schema), the
limiter is in-process with a bounded key set, and the access-token contract is untouched. The two
choices that could have drifted were resolved in research: the CSRF signing key is derived from the
JWT secret with domain separation (`R2`), so no new required secret is introduced, and registration
keeps its 201 status while swapping the body for the session payload (`R8`).

## Project Structure

### Documentation (this feature)

```text
.specs/features/006-backend-browser-auth-cookies/
├── spec.md              # Requirements (AUTH-*)
├── plan.md              # This file
├── research.md          # Phase 0 — decisions and rejected alternatives
├── data-model.md        # Phase 1 — session material, cookies, CSRF token, limiter, config, messages
├── quickstart.md        # Phase 1 — runnable validation guide
├── contracts/           # Phase 1 — one contract per feature interface
│   ├── auth-session.contract.md
│   ├── csrf.contract.md
│   ├── cors-origin.contract.md
│   └── rate-limit.contract.md
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/auth/
├── service.go             # session material returned by Register/Login/Refresh; cookie value in
│                          # RefreshCommand/LogoutCommand + CSRF verification order (AUTH-01…AUTH-07,
│                          # AUTH-10, AUTH-11, AUTH-12)
├── csrf.go                # CSRFTokenManager: stateless HMAC token bound to the family id (NEW)
├── cookie.go              # cookie name constants + attribute builders (set and clear) (NEW)
├── errors.go              # new exported message constants (CSRF, session) (extended)
├── service_test.go        # rewritten/extended: session material + CSRF + rotation order
├── csrf_test.go           # token issue/verify, binding, tamper, malformed (NEW)
├── cookie_test.go         # the exact attribute set on set and clear (NEW)
└── mocks/                 # regenerated after the interface change (make mocks)

internal/app/
├── auth.go                # controller: read the cookie, set/clear both cookies, session body
│                          # (AUTH-01…AUTH-12, AUTH-21, AUTH-22)
├── origin.go              # requireAllowedOrigin middleware over CORS_ALLOWED_ORIGINS: Origin with a
│                          # Referer fallback, fail-closed 403 (NEW; AUTH-13)
├── ratelimit.go           # rate-limit guard over internal/platform/ratelimit: keys, 429 + Retry-After
│                          # (NEW; AUTH-17…AUTH-20)
├── router.go              # mount the two guards on the auth routes (extended; AUTH-13, AUTH-17)
├── errors.go              # no new sentinel; reuses the apperr translation (unchanged behaviour)
├── bootstrap.go           # build the CSRF manager and the limiter, pass them in (extended)
├── auth_test.go           # rewritten: cookie attributes, body shape, guard wiring (AUTH-01…AUTH-12)
├── auth_integration_test.go # rewritten: cookie-driven login/register/refresh/logout (AUTH-01…AUTH-07)
├── cors_test.go           # rewritten: X-CSRF-Token advertised, X-Refresh-Token gone, credentials
│                          # echoed (AUTH-14…AUTH-16, AUTH-21)
├── origin_test.go         # missing/allowed/disallowed Origin and Referer (NEW; AUTH-13)
├── ratelimit_test.go      # over-limit 429 + Retry-After, independent keys, window free (NEW;
│                          # AUTH-17…AUTH-20)
├── cookie_scope_test.go   # /sheets and /templates stay Bearer-only, never require a cookie (NEW;
│                          # AUTH-26)
└── integration_test.go    # token-pair decode replaced by the session body (extended)

internal/platform/
├── cors/cors.go           # AllowCredentials + X-CSRF-Token, drop X-Refresh-Token (extended;
│                          # AUTH-14…AUTH-16, AUTH-21)
├── cors/cors_test.go      # credentialed preflight and exact-origin echo (extended)
├── metrics/metrics.go     # the rate-limit block counter legnaris_auth_rate_limited_total (extended;
│                          # AUTH-20)
├── metrics/metrics_test.go # the block counter increments (extended; AUTH-20)
└── ratelimit/             # the reusable in-process sliding-window limiter (NEW; AUTH-17…AUTH-20)
    ├── limiter.go
    └── limiter_test.go

shared/
├── apperr/err.go          # CodeRateLimited → 429 (extended; AUTH-23)
├── apperr/err_test.go     # the 429 mapping (extended)
├── env/env.go             # AuthRateLimitAttempts, AuthRateLimitWindow defaults (extended; AUTH-20,
│                          # AUTH-24)
├── env/env_test.go        # the defaults and the invalid/absent fallbacks (extended)
├── logger/logger.go       # redaction covers the new cookie names (extended; AUTH-25)
└── logger/logger_test.go  # the new redaction cases (extended)

Makefile                   # export the two new optional variables (extended; AUTH-20)
deploy/.env.example        # document the two new defaults (extended; AUTH-20)
```

**Structure Decision**: the credential's *material* — the session payload, the CSRF token and the
cookie descriptors — lives in `internal/auth`, next to the service that mints the family, so the
service can bind the CSRF token to the session it just created and the controller stays a thin
translator (it reads the cookie, sets two cookies, and serialises the body). The two *guards* that
must return `apperr` live in `internal/app` (`origin.go`, `ratelimit.go`), because `apperr` is a
service/controller vocabulary and `internal/platform` must not import it; the reusable limiter
*engine* (`internal/platform/ratelimit`) is a leaf package with standard-library-only imports so the
later `/csp-report` route (F9) can reuse it without pulling in the auth domain. Credentialed CORS is a
configuration change to the existing `internal/platform/cors` middleware (Echo already echoes the
exact origin and sets `Vary: Origin`; it refuses `*` with credentials by construction), not a new
middleware. `internal/app/router.go` mounts the origin guard only on `/auth/refresh` and
`/auth/logout` and the rate-limit guard only on `/auth/login`, `/auth/register` and `/auth/refresh`,
leaving `/sheets` and `/templates` untouched (AUTH-26). Tests stay next to the code they cover
(`*_test.go`), matching the repository convention, with the HTTP-boundary assertions concentrated in
`internal/app`.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
