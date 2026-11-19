# AGENTS.md — backend/internal/ (Application Layer)

## Architecture Overview

| Layer | Package | Responsibility |
|-------|---------|----------------|
| API | `backend/internal/app` | Echo controllers, routing, error handling |
| Auth | `backend/internal/auth` | JWT, bcrypt, refresh rotation, email code challenge |
| Templates | `backend/internal/sheet/template` | DAG build, formula compile, recalculation, immutable registry |
| Sheets | `backend/internal/sheet` | CRUD, partial update with cascading recalc |
| Engine | `backend/internal/engine` | expr-lang wrapper (floor, max functions) |
| Dice | `backend/internal/dice` | Dice expression grammar: parse, fit a Row, evaluate (no `apperr`, no `template` import) |
| Platform | `backend/internal/platform` | cross-cutting middleware (cors, health, metrics) |
| Shared | `backend/shared/*` | env, logger, db, apperr, response, json, idgen, maputils, requestcache |

**Entry point:** `backend/cmd/api.go` → builds DAG registry from code-defined templates → starts Echo on `PORT` (default `:8080`).

## Layering Rules

- **app** → controllers call domain service packages (`sheet`, `auth`, `user`, `generation`, `plan`, `quota`, …); `bootstrap.go` is the composition root that builds repositories, and `EmailIntent` (`email_intent.go`) writes to the outbox repository directly
- **sheet** service → calls **template** (DAG/Registry) and **repository**
- **template** → calls **engine** for formula evaluation
- **engine**, **repository**, **backend/shared/*** → no upward imports

## Service Conventions

- Services in `backend/internal/sheet/service.go`, controllers in `backend/internal/app/*.go`
- Use **only `apperr`** for domain errors: `apperr.NewError(code, message, err)`
- The `err` cause must be `nil` or a lower-layer typed error; never a bare `errors.New`
- Export user-facing message constants (plain strings) from the service package for controller reuse
- Translate lower-layer errors at the boundary using `errors.Is`/`errors.As`

## Sheet Writes

- **One CAS, no retry** — `PATCH` validates the delta with `DAG.NormalizeAndRecalculate`, validates any dice block, then calls `Repository.PartialUpdate` once (`_id` + expected `version`, `$inc version`). The service never retries: a lost race is 409 `CONFLICT` with `Retry-After: 1`; a missing sheet stays 404. `loadtest/k6/concurrency.js` (`VARIANT=distinct|same`) must report zero lost writes.
- **Version is internal** — never expose `Sheet.Version` to clients. `UpdateLayout` must not read or write it, so `PUT /layout` never conflicts with a `PATCH`.
- **Layout is a single field write** — only `Repository.UpdateLayout` writes `layout` (`$set` alone; no `updated_at`, no DAG, no recalculation). Validate with `template.ValidateLayout`; readers must ignore stale keys. Add `PUT` to `platform/cors` or browsers block the preflight.
- **Dice is part of the same save** — validate with `template.ValidateSheetDice` before the CAS: a present block replaces the stored one (empty clears it), absent or `null` leaves it untouched. A dice-only `PATCH` skips the DAG and returns `{}`; a body with neither data nor dice is 400.
- **Ownership first** — check owner and sheet id before body validation or template lookups, so a foreign sheet is 404 whatever the payload.

## Rate Limiting

- **Scopes** — session endpoints `POST /auth/refresh`, `GET /auth/csrf` and `POST /auth/logout` per address (`SESSION_RATE_LIMIT_*`, 120/1m, namespace `session`); credential endpoints `POST /auth/login`, `POST /auth/challenge/verify` and `POST /auth/challenge/resend` per address (`AUTH_RATE_LIMIT_*`, 30/1m, namespace `auth`) plus, on login only, the canonical account (`AUTH_ACCOUNT_RATE_LIMIT_*`, 10/1m, namespace `auth_account`); registration per address and per canonical e-mail (`AUTH_REGISTER_*`, 5/1h and 3/1h); `/csp-report` per address (`CSP_REPORT_RATE_LIMIT_*`); `/templates`, `/sheets`, `GET /me/plan` and `GET /auth/me` per user id (`USER_RATE_LIMIT_*`, 80/1m, one shared budget); `/health`, `/ready` and `/metrics` are exempt (orchestrator probes must never get 429)
- **Refusal** — always `rateLimited`: 429 `RATE_LIMITED` with `Retry-After`. Only the auth and session guards feed the `RateLimitBlocked` metric
- **Order** — `userRateLimit` must follow `auth.Middleware`; with no resolved user id it passes through so the 401 stays the auth layer's call
- **Replicas** — guards take a `ratelimit.Allower`. `REDIS_URL` set → `ratelimit.Shared` over `RedisStore` (atomic Lua sliding window, Redis clock, namespaces `auth`/`auth_account`/`session`/`user`/`public`); unset → in-process `Limiter` (cap per replica). A store error fails open to the per-process limiter (throttled warning, never a 5xx). Build limiters only through `newLimiter`; see `docs/adr/0003-shared-rate-limit-store.md`
- **Route-level middleware only** — attach limiters per route, never through `router.Group("", mw)`: a middleware on an empty-prefix group registers a catch-all that turns 405 into another status
