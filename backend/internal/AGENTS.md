# AGENTS.md — backend/internal/ (Application Layer)

## Architecture Overview

| Layer | Package | Responsibility |
|-------|---------|----------------|
| API | `backend/internal/app` | Echo controllers, routing, error handling |
| Auth | `backend/internal/auth` (planned) | JWT, bcrypt, refresh rotation |
| Templates | `backend/internal/sheet/template` | DAG build, formula compile, recalculation, immutable registry |
| Sheets | `backend/internal/sheet` | CRUD, partial update with cascading recalc |
| Engine | `backend/internal/engine` | expr-lang wrapper (floor, max functions) |
| Dice | `backend/internal/dice` | Dice expression grammar: parse, fit a Row, evaluate (no `apperr`, no `template` import) |
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

## Concurrent PATCH (optimistic lock)

- **Contract** — `Sheet.Version` counts sheet saves (data, dice or both); `Repository.PartialUpdate` is a `FindOneAndUpdate` filtered on `_id` + expected version that `$set`s `data` (when the save carries a delta), `dice` (when it carries a block) and `updated_at`, and `$inc`s `version`, returning the stored version. A missing field matches expected 0 (pre-versioning sheets)
- **Conflict** — a lost race is `ErrVersionConflict`; the service does **not** retry internally and answers 409 `CONFLICT` with `Retry-After: 1`, so the client resend is a new request under the rate limiter. A missing sheet stays `ErrSheetNotFound` (404, never a conflict)
- **Version is internal** — never exposed to clients. `UpdateLayout` does not touch it, so `PUT /layout` never conflicts with PATCH
- **Verification** — `loadtest/k6/concurrency.js` (`VARIANT=distinct|same`) must report zero lost writes; it resends on 409

## Sheet Layout (`PUT /sheets/:id/layout`)

- **Contract** — body `{"layout": {"section_order": [...], "pinned": [...]}}`; 200 `{"data":{"layout": <stored|null>}}`. `GET /sheets/:id` returns `layout` only when set. Full replacement and idempotent; absent/`null`/empty layout = reset to default (stored as none). An order equal to the Template's is stored as given. Rows are never reordered: a pinned Row rises to the top of its own Section in pin order. The old `row_order` field no longer exists and is rejected like any unknown field (400 `INVALID_JSON`)
- **User data, not Template data** — `Sheet.Layout` is written by `Repository.UpdateLayout`, a `$set` on the `layout` field alone (no `updated_at`, no read-modify-write), so a concurrent `PATCH` of `data` is unaffected. `SetLayout` never calls the DAG or recalculates
- **Strict on write, tolerant on read** — keys are validated by `template.ValidateLayout` against the Rows of the sheet's Template; partial lists are valid, but an unknown section, a duplicate (per list) or a pin that is no Row of any Section are 400 `INVALID_REQUEST` (translated by `translateLayoutErr`; no dedicated apperr code). Readers must ignore stale keys and fall back to Template order for missing ones
- **CORS** — `PUT` is in the allowed methods of `platform/cors`; a new verb needs adding there or browsers block the preflight

## Sheet Dice (saved by `PATCH /sheets/:id`)

- **Contract** — the `PATCH` body is `{"data"?: <delta>, "dice"?: {"section": {name: expr|null}, "row": {rowKey: expr|null}}}` (ADR 0006). A present `dice` replaces the whole block (idempotent; an empty block, `{}` or null maps, clears everything and is stored as none); an absent or `null` `dice` leaves the stored block untouched. A body with neither a non-empty `data` nor a `dice` is 400 `INVALID_REQUEST` (`MsgNothingToUpdate`). A dice-only save skips the DAG and answers an empty cascade `{"data":{}}`. There is no `PUT /sheets/:id/dice` any more. `GET /sheets/:id` always returns `dice`: a Sheet that never had one reads as `{"section":{},"row":{}}`. A `null` value is an explicit "no dice" and is distinct from an absent key ("use the Template"). See ADR 0005 and ADR 0006
- **One atomic save** — `PartialUpdate` validates the delta (DAG) and the block (Template) before the single compare-and-swap write, so a refused block writes no data and a refused delta writes no block; both go out under one `version` and share the 409 resend. `Preview` reads `data` only
- **Strict on write, tolerant on read** — `template.ValidateSheetDice` checks the block against the Sheet's Template: cap of `MaxSheetDiceOverrides` (64) entries across both maps, null included; every key must be a Section (Row) key, so null entries need a known key too; every non-null expression must pass the `dice` grammar, a Row override must fit its Row and a Section override at least one Row. Failures are 400 `INVALID_REQUEST` (translated by `translateDiceErr`; no dedicated apperr code). Malformed JSON, wrong value types and unknown keys are 400 `INVALID_JSON`. `GET` returns the stored block untouched: readers must ignore an override whose Row or Field the Template no longer has
- **Ownership** — owner and sheet id are checked before the cap and the Template, so a foreign Sheet is 404 whatever the body, and a malformed id is 400

## Rate Limiting

- **Scopes** — `/auth/*` per address (`AUTH_RATE_LIMIT_*`, 10/1m, plus per account on login/register); `/csp-report` per address (`CSP_REPORT_RATE_LIMIT_*`); `/templates`, `/sheets` and `GET /auth/me` per user id (`USER_RATE_LIMIT_*`, 80/1m, one shared budget); `/health`, `/ready` and `/metrics` are exempt (orchestrator probes must never get 429)
- **Refusal** — always `rateLimited`: 429 `RATE_LIMITED` with `Retry-After`. Only the auth guard feeds the `RateLimitBlocked` metric
- **Order** — `userRateLimit` must follow `auth.Middleware`; with no resolved user id it passes through so the 401 stays the auth layer's call
- **Replicas** — guards take a `ratelimit.Allower`. `REDIS_URL` set → `ratelimit.Shared` over `RedisStore` (atomic Lua sliding window, Redis clock, namespaces `auth`/`user`/`public`); unset → in-process `Limiter` (cap per replica). A store error fails open to the per-process limiter (throttled warning, never a 5xx). Build limiters only through `newLimiter`; see `docs/adr/0003-shared-rate-limit-store.md`
- **Route-level middleware only** — attach limiters per route, never through `router.Group("", mw)`: a middleware on an empty-prefix group registers a catch-all that turns 405 into another status
