# AGENTS.md — Legnaris Backend

## Quick Start

```bash
# Start MongoDB
docker compose up -d

# Run API (loads templates from code, seeds fantasy_v1 on boot)
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
| Templates | `internal/sheet/template` | DAG build, formula compile, recalculation |
| Sheets | `internal/sheet` | CRUD, partial update with cascading recalc |
| Engine | `internal/engine` | expr-lang wrapper (floor, max functions) |
| Shared | `shared/*` | env, logger, db, apperr, response, json |

**Entry point:** `cmd/api.go` → builds DAG registry from code-defined templates → starts Echo on `:8080`.

## Key Concepts

- **Template** — code-defined structure (fields, formulas, deps). Immutable by ID (`fantasy_v1`, `fantasy_v2`...).
- **DAG** — compiled dependency graph with topological order. Built once at boot per template.
- **Sheet** — `map[string]any` document linked to a template. Base fields editable; derived fields computed.
- **Recalculation** — `DAG.Recalculate(original, updates)` returns only changed derived fields (cascading).

## Commands

| Task | Command |
|------|---------|
| Start dev DB | `docker compose up -d` |
| Run server | `go run ./cmd/api.go` |
| Run all tests | `go test ./...` |
| Run single test | `go test -run TestBuildDag ./internal/sheet/template` |
| Generate mocks | `make mocks` |
| Format | `make fmt` |

## Testing

- Unit tests use `testify` + Mockery mocks in `internal/*/mocks/`
- Integration tests need MongoDB running (`docker compose up -d`)
- Key test files: `dag_test.go`, `dag_recalculate_test.go`, `path_test.go`

## Current State (per TDD)

**Phase 0 — Hygiene (pending):**
- Remove PostgreSQL leftovers (README, Makefile, docker-compose)
- Fix error messages in sheet repository ("template not found" → "sheet not found")
- Normalize `floor`/`max` int/float coercion in engine
- Eliminate template seed on boot; serve templates from in-memory registry
- Persist PATCH with dot-notation `$set` (not nested map)

## Conventions

- **Env vars:** `PORT`, `APP_NAME`, `DATABASE_URI` (defaults in `shared/env/env.go`)
- **Error handling:** `shared/apperr` typed errors → `HTTPErrorHandler` in `router.go`
- **JSON decoding:** `shared/json/decode.go` helper
- **Logging:** `shared/logger` structured logger (zerolog-based)
- **Mocks:** generated via `mockery` (config in `go.mod` tool directive)

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

- **Templates are code, not DB** — don't look for a `template` collection in Mongo (will be removed in Phase 0)
- **DAG keys use dot notation** (`stats.dexterity`) but engine env uses underscores (`stats_dexterity`) — collision risk if field names contain dots/underscores (tracked in `dag_test.go` LGN-41)
- **PartialUpdate validates editability** against DAG before recalc — non-editable fields in payload = 400
- **Seed runs on every boot** currently — will be removed; templates served from memory registry
- **No auth yet** — all endpoints open; ownership/authorization in Phase 2

## File Patterns

```
internal/
  app/           # HTTP layer (controllers, router)
  auth/          # (planned) JWT, bcrypt, middleware
  engine/        # formula engine (expr-lang wrapper)
  sheet/
    template/    # DAG, types, seeders, errors
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