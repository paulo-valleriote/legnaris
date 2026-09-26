# AGENTS.md — internal/ (Application Layer)

## Architecture Overview

| Layer | Package | Responsibility |
|-------|---------|----------------|
| API | `internal/app` | Echo controllers, routing, error handling |
| Auth | `internal/auth` (planned) | JWT, bcrypt, refresh rotation |
| Templates | `internal/sheet/template` | DAG build, formula compile, recalculation, immutable registry |
| Sheets | `internal/sheet` | CRUD, partial update with cascading recalc |
| Engine | `internal/engine` | expr-lang wrapper (floor, max functions) |
| Shared | `shared/*` | env, logger, db, apperr, response, json |

**Entry point:** `cmd/api.go` → builds DAG registry from code-defined templates → starts Echo on `PORT` (default `:8080`).

## Layering Rules

- **app** → calls **sheet** service only
- **sheet** service → calls **template** (DAG/Registry) and **repository**
- **template** → calls **engine** for formula evaluation
- **engine**, **repository**, **shared/*** → no upward imports

## Service Conventions

- Services in `internal/sheet/service.go`, controllers in `internal/app/*.go`
- Use **only `apperr`** for domain errors: `apperr.NewError(code, message, err)`
- The `err` cause must be `nil` or a lower-layer typed error; never a bare `errors.New`
- Export user-facing message constants (plain strings) from the service package for controller reuse
- Translate lower-layer errors at the boundary using `errors.Is`/`errors.As`

## Testing

- Unit tests: `testify` + Mockery mocks in `internal/*/mocks/`
- Run single test: `go test -run TestName ./package/path`
- Integration tests self-skip when MongoDB unavailable (see `deploy/` stack)