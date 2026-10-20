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