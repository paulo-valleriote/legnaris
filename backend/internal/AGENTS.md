# AGENTS.md — backend/internal/ (Application Layer)

## Architecture Overview

| Layer | Package | Responsibility |
|-------|---------|----------------|
| API | `backend/internal/app` | Echo controllers, routing, error handling |
| Auth | `backend/internal/auth` (planned) | JWT, bcrypt, refresh rotation |
| Templates | `backend/internal/sheet/template` | DAG build, formula compile, recalculation, immutable registry |
| Sheets | `backend/internal/sheet` | CRUD, partial update with cascading recalc |
| Engine | `backend/internal/engine` | expr-lang wrapper (floor, max functions) |
| Platform | `backend/internal/platform` | cross-cutting middleware (cors, health, metrics) |
| Shared | `backend/shared/*` | env, logger, db, apperr, response, json |

**Entry point:** `backend/cmd/api.go` → builds DAG registry from code-defined templates → starts Echo on `PORT` (default `:8080`).

## Layering Rules

- **app** → calls **sheet** service only
- **sheet** service → calls **template** (DAG/Registry) and **repository**
- **template** → calls **engine** for formula evaluation
- **engine**, **repository**, **backend/shared/*** → no upward imports

## Service Conventions

- Services in `backend/internal/sheet/service.go`, controllers in `backend/internal/app/*.go`
- Use **only `apperr`** for domain errors: `apperr.NewError(code, message, err)`
- The `err` cause must be `nil` or a lower-layer typed error; never a bare `errors.New`
- Export user-facing message constants (plain strings) from the service package for controller reuse
- Translate lower-layer errors at the boundary using `errors.Is`/`errors.As`

## Sheet Layout (`PUT /sheets/:id/layout`)

- **Contract** — body `{"layout": {"section_order": [...], "pinned": [...]}}`; 200 `{"data":{"layout": <stored|null>}}`. `GET /sheets/:id` returns `layout` only when set. Full replacement and idempotent; absent/`null`/empty layout = reset to default (stored as none). An order equal to the Template's is stored as given. Rows are never reordered: a pinned Row rises to the top of its own Section in pin order. The old `row_order` field no longer exists and is rejected like any unknown field (400 `INVALID_JSON`)
- **User data, not Template data** — `Sheet.Layout` is written by `Repository.UpdateLayout`, a `$set` on the `layout` field alone (no `updated_at`, no read-modify-write), so a concurrent `PATCH` of `data` is unaffected. `SetLayout` never calls the DAG or recalculates
- **Strict on write, tolerant on read** — keys are validated by `template.ValidateLayout` against the Rows of the sheet's Template; partial lists are valid, but an unknown section, a duplicate (per list) or a pin that is no Row of any Section are 400 `INVALID_REQUEST` (translated by `translateLayoutErr`; no dedicated apperr code). Readers must ignore stale keys and fall back to Template order for missing ones
- **CORS** — `PUT` is in the allowed methods of `platform/cors`; a new verb needs adding there or browsers block the preflight

## Rate Limiting

- **Scopes** — `/auth/*` per address (`AUTH_RATE_LIMIT_*`, 10/1m, plus per account on login/register); `/csp-report` per address (`CSP_REPORT_RATE_LIMIT_*`); `/templates`, `/sheets` and `GET /auth/me` per user id (`USER_RATE_LIMIT_*`, 80/1m, one shared budget); `/health` and `/metrics` per address and route (`PUBLIC_RATE_LIMIT_*`, 80/1m)
- **Refusal** — always `rateLimited`: 429 `RATE_LIMITED` with `Retry-After`. Only the auth guard feeds the `RateLimitBlocked` metric
- **Order** — `userRateLimit` must follow `auth.Middleware`; with no resolved user id it passes through so the 401 stays the auth layer's call
- **Route-level middleware only** — attach limiters per route, never through `router.Group("", mw)`: a middleware on an empty-prefix group registers a catch-all that turns 405 into another status
