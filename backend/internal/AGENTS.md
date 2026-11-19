# AGENTS.md — backend/internal/ (Application Layer)

Architecture, layering, shared primitives and cross-cutting middleware:
[`context/architecture.md`](../../context/architecture.md).

## Working rules

- Controllers in `backend/internal/app` call domain services; `bootstrap.go`
  is the API composition root; `email_intent.go` is the one controller-side
  write to the email outbox.
- Keep `app → domain service → template → engine`; domain packages may depend
  on lower domain packages, but no domain package imports `app` and no shared
  package imports a domain package. A domain package may declare an interface
  that `platform` implements, never the reverse.
- Generate every server ID with `idgen.New`; validate client ids with
  `idgen.Validate` before the first repository call; never call `uuid.New` or
  `uuid.NewString`.
- Domain errors are `apperr.NewError(code, message, cause)` at the service or
  controller boundary only. The cause is `nil`, one of the package's own typed
  sentinels, or a lower-layer typed error; never a bare `errors.New` created at
  the call site.
- Register routes through their controller and attach guards per route; never
  wrap an empty-prefix group with middleware.
- `make fmt && make lint && make vet` and `make test-race` must pass before a
  pull request.

## Concurrent PATCH (optimistic lock)

- **Contract** — `Sheet.Version` counts sheet saves (data, dice or both); `Repository.PartialUpdate` is a `FindOneAndUpdate` filtered on `_id` + expected version that `$set`s `data` (when the save carries a delta), `dice` (when it carries a block) and `updated_at`, and `$inc`s `version`, returning the stored version. A missing field matches expected 0 (pre-versioning sheets)
- **Conflict** — a lost race is `ErrVersionConflict`; the service does **not** retry internally and answers 409 `CONFLICT` with `Retry-After: 1`, so the client resend is a new request under the rate limiter. A missing sheet stays `ErrSheetNotFound` (404, never a conflict)
- **Version is internal** — never exposed to clients. `UpdateLayout` does not touch it, so `PUT /layout` never conflicts with PATCH
- **Verification** — `loadtest/k6/concurrency.js` (`VARIANT=distinct|same`) must report zero lost writes; it resends on 409

## Sheet Layout (`PUT /sheets/:id/layout`)

- **Contract** — body `{"layout": {"section_order": [...], "pinned": [...]}}`; 200 `{"data":{"layout": <stored|null>}}`. `GET /sheets/:id` returns `layout` only when set. Full replacement and idempotent; absent/`null`/empty layout = reset to default (stored as none). An order equal to the Template's is stored as given. Rows are never reordered: a pinned Row rises to the top of its own Section in pin order.
- **User data, not Template data** — `Sheet.Layout` is written by `Repository.UpdateLayout`, a `$set` on the `layout` field alone (no `updated_at`, no read-modify-write), so a concurrent `PATCH` of `data` is unaffected. `SetLayout` never calls the DAG or recalculates
- **Strict on write, tolerant on read** — keys are validated by `template.ValidateLayout` against the Rows of the sheet's Template; partial lists are valid, but an unknown section, a duplicate (per list) or a pin that is no Row of any Section are 400 `INVALID_REQUEST` (translated by `translateLayoutErr`; no dedicated apperr code). Readers must ignore stale keys and fall back to Template order for missing ones
- **CORS** — `PUT` is in the allowed methods of `platform/cors`; a new verb needs adding there or browsers block the preflight

## Sheet Dice (saved by `PATCH /sheets/:id`)

- **Contract** — the `PATCH` body is `{"data"?: <delta>, "dice"?: {"section": {name: expr|null}, "row": {rowKey: expr|null}}}` (ADR 0006). A present `dice` replaces the whole block (idempotent; an empty block, `{}` or null maps, clears everything and is stored as none); an absent or `null` `dice` leaves the stored block untouched. A body with neither a non-empty `data` nor a `dice` is 400 `INVALID_REQUEST` (`MsgNothingToUpdate`). A dice-only save skips the DAG and answers an empty cascade `{"data":{}}`. `GET /sheets/:id` always returns `dice`: a Sheet that never had one reads as `{"section":{},"row":{}}`. A `null` value is an explicit "no dice" and is distinct from an absent key ("use the Template"). See ADR 0005 and ADR 0006
- **One atomic save** — `PartialUpdate` validates the delta (DAG) and the block (Template) before the single compare-and-swap write, so a refused block writes no data and a refused delta writes no block; both go out under one `version` and share the 409 resend. `Preview` reads `data` only
- **Strict on write, tolerant on read** — `template.ValidateSheetDice` checks the block against the Sheet's Template: cap of `MaxSheetDiceOverrides` (64) entries across both maps, null included; every key must be a Section (Row) key, so null entries need a known key too; every non-null expression must pass the `dice` grammar, a Row override must fit its Row and a Section override at least one Row. Failures are 400 `INVALID_REQUEST` (translated by `translateDiceErr`; no dedicated apperr code). Malformed JSON, wrong value types and unknown keys are 400 `INVALID_JSON`. `GET` returns the stored block untouched: readers must ignore an override whose Row or Field the Template no longer has
- **Ownership** — owner and sheet id are checked before the cap and the Template, so a foreign Sheet is 404 whatever the body, and a malformed id is 400
