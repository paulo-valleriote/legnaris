# AGENTS.md — Legnaris Backend

## Quick Start

```bash
# Run API (templates served from the in-memory registry; MongoDB required)
go run ./cmd/api.go

# Run tests
go test ./...

# Generate mocks (after interface changes)
make mocks

# Format code
make fmt
```

## Architecture Overview

| Layer | Package | Responsibility |
|-------|---------|----------------|
| API | `internal/app` | Echo controllers, routing, error handling |
| Auth | `internal/auth` (planned) | JWT, bcrypt, refresh rotation |
| Templates | `internal/sheet/template` | DAG build, formula compile, recalculation, immutable registry |
| Sheets | `internal/sheet` | CRUD, partial update with cascading recalc |
| Engine | `internal/engine` | expr-lang wrapper (floor, max functions) |
| Shared | `shared/*` | env, logger, db, apperr, response, json |

**Entry point:** `cmd/api.go` → builds DAG registry from code-defined templates → starts Echo on the configured `PORT` (default `:8080`).

## Key Concepts

- **Template** — code-defined structure (fields, formulas, deps). Immutable by ID (`fantasy_v1`, `fantasy_v2`...).
- **DAG** — compiled dependency graph with topological order. Built once at boot per template and immutable afterwards: private (never embedded) mutex, a single read-lock acquisition per `Recalculate`, and `GetNode` returning value copies.
- **Sheet** — `map[string]any` document linked to a template. Base fields editable; derived fields computed.
- **Recalculation** — `DAG.Recalculate(original, updates)` returns only changed derived fields (cascading).

## Commands

| Task | Command |
|------|---------|
| Run server | `make run` (or `go run ./cmd/api.go`) |
| Build | `make build` |
| Run all tests | `make test` (or `go test ./...`) |
| Run tests (race) | `make test-race` |
| Run single test | `go test -run TestBuildDag ./internal/sheet/template` |
| Lint | `make lint` |
| Vet | `make vet` |
| Generate mocks | `make mocks` |
| Format | `make fmt` |
| Deploy (production) | `make deploy-up` |
| Deploy (local tunnel) | `make deploy-local` |
| Stop deploy | `make deploy-down` |

## Testing

- Unit tests use `testify` + Mockery mocks in `internal/*/mocks/`
- Integration tests self-skip when MongoDB is unavailable (see the `deploy/` stack)
- Key test files: `dag_test.go`, `dag_recalculate_test.go`, `path_test.go`

## Current State (per TDD)

**Phase 0 — Hygiene: ✅ complete**
- Removed PostgreSQL leftovers (README, root `docker-compose.yml`, and the `scripts/database/migrations/` SQL)
- Fixed sheet repository error messages ("sheet not found")
- Normalized `floor`/`max` int/float coercion in engine
- Eliminated template seed on boot; templates served from the in-memory registry
- PATCH body uses a nested map (`{"data":{"stats":{"dexterity":12}}}`), flattened internally to DAG dot-keys

**Next:** Phase 1 — auth (JWT, bcrypt, refresh rotation).

## Conventions

- **Env vars:** `PORT`, `APP_NAME`, `DATABASE_URI` (defaults in `shared/env/env.go`)
- **GitHub auth:** for `git push` and the GitHub REST API (PR creation), read the token from `OPENCODE_PR` in the repo-root `.env` (gitignored). Never print the token, never commit `.env`; use it only via the `Authorization` header / push URL.
- **Error handling — errors MUST always be typed:** no bare/untyped `errors.New` for domain errors anywhere. Each layer has its own typed errors:
  - **Services and controllers** (`internal/sheet/service.go`, `internal/app/*.go`) use **only `apperr`**: `apperr.NewError(code, message, err)` (not found → `CodeNotFound`/404, invalid or non-editable field → `CodeInvalidRequest`/`CodeUnknownField`/400).
    - The `err` cause argument must be `nil` or a lower-layer typed error (repository/DAG/json/maputils/…); never a freshly created `errors.New`, and no package-level `errors.New` sentinels in `internal/app`/`internal/sheet`.
    - Shared user-facing **message constants** (plain strings) are not errors — exporting them from the owning service package so controllers reuse the wording is allowed and encouraged.
  - **All other layers** (repository, template/DAG, engine, `shared/db`, …) **must NOT import `apperr`** — they declare their own typed errors (sentinels like `var ErrSheetNotFound = errors.New("sheet not found")` or custom error types) and wrap with `fmt.Errorf("...: %w", err)`.
  - **Services translate at the boundary:** map lower-layer errors to `apperr` with `errors.Is`/`errors.As` (e.g. `errors.Is(err, ErrSheetNotFound)` → `apperr.NewError(apperr.CodeNotFound, ...)`) so `HTTPErrorHandler` returns the correct status instead of the 500 `INTERNAL_ERROR` fallback.
- **Error handling — new codes require handler updates:** when adding a new `apperr.Code` you MUST (1) add its HTTP status mapping in `apperr.NewError` (`shared/apperr/err.go`) — unmapped codes fall back to 500 via `HttpStatus()`; (2) confirm `HTTPErrorHandler` (`internal/app/router.go`) serializes it correctly; (3) add a test asserting the status mapping for the new error
- **JSON decoding:** `shared/json/decode.go` helper
- **Logging:** `shared/logger` structured logger (zerolog-based)
- **Mocks:** generated via `mockery` (config in `go.mod` tool directive)

## PR Review Checklist

Automated reviewers (AI + linters) evaluate pull requests against these
criteria. AI findings are advisory; the `ci-lint` workflow (golangci-lint +
`go test -race`) is blocking on new issues.

- **Cyclomatic complexity** — keep functions small; `gocyclo` flags complexity
  above 15.
- **Code duplication** — extract shared logic; `dupl` flags near-duplicate
  blocks.
- **Conventions** — follow this document: typed errors via `shared/apperr`,
  JSON decoding via `shared/json`, structured logging via `shared/logger`, and
  the layering (app → sheet → template → engine → shared).
- **English-only** — all identifiers, comments, log messages, and user-facing
  strings must be in English. No Portuguese (pt-BR) code or strings.
  Exception: `docs/TDD.md` is a Portuguese design document and stays in pt-BR —
  only code and code-adjacent strings must be English.
- **Race conditions** — no unsynchronized access to shared state; prefer
  channels or mutexes; `go test -race` must stay clean.
- **Memory leaks** — close resources (files, DB cursors, response bodies),
  avoid unbounded goroutines, use context cancellation.

## API Endpoints (Current + Planned)

| Endpoint | Method | Status |
|----------|--------|--------|
| `/health` | GET | ✅ `internal/platform/http/health.go` |
| `/templates` | GET | ✅ lists catalog |
| `/sheets` | POST | ✅ create from template |
| `/sheets/:id` | GET | ✅ fetch |
| `/sheets/:id` | PATCH | ✅ partial update + recalc |
| `/auth/*` | — | 🔄 Phase 1 |

## Gotchas

- **Templates are code, not DB** — there is no `template` collection in Mongo; templates live in the in-memory registry
- **DAG keys are flat path keys** (`stats.dexterity`) while engine env uses underscores (`stats_dexterity`) — field names are validated at `BuildDAG` (LGN-52): every segment must match `^[A-Za-z][A-Za-z0-9]*$`, so neither `.` nor `_` can appear in a name and the two key encodings can never collide (regression tests: `TestBuildDAG_FieldNameValidation` and the LGN-41 case in `TestBuildDag_EdgeCases`)
- **PartialUpdate validates editability** against DAG before recalc — non-editable fields in payload = 400
- **Registry and DAGs are frozen after boot** — `Registry.List()`/`FetchByID()` return deep copies (the registry clones `DefaultTemplates` on ingest), DAGs are resolved per ID via `Registry.DAG(id)` through the `sheet.DAGProvider` interface (there is no `DAGs()` map accessor anymore), and `DAG` guards its state with an unexported mutex: never acquire it twice — read through `HasNode`/`GetNode`/`GetChildren`, or stay under the single lock `Recalculate` takes (recursive RLock deadlocks with a queued writer, regression test `TestRecalculate_ValidationRunsUnderSingleReadLock`)
- **No auth yet** — all endpoints open; ownership/authorization in Phase 2

## File Patterns

```
internal/
  app/           # HTTP layer (controllers, router)
  auth/          # (planned) JWT, bcrypt, middleware
  engine/        # formula engine (expr-lang wrapper)
  sheet/
    template/    # DAG, registry, clone, types, seeders, errors
    mocks/       # generated mocks
    *.go         # sheet service, repository
shared/
  apperr/        # typed app errors
  db/            # mongo init
  env/           # env var loading
  logger/        # structured logging
  response/      # HTTP response helpers
  json/          # decode helper
cmd/api.go       # main entry point
```