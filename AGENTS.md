# AGENTS.md — Legnaris Backend

## Quick Start

```bash
go run ./cmd/api.go      # Run API (MongoDB required; no Mongo yet? `make dev-db`)
go test ./...            # Run tests
make mocks               # Generate mocks (after interface changes)
make fmt                 # Format code
```

## Commands

| Task | Command |
|------|---------|
| Run server | `make run` |
| Build | `make build` |
| Test (race) | `make test-race` |
| Lint | `make lint` |
| Vet | `make vet` |
| Run local MongoDB (dev only) | `make dev-db` |
| Deploy (production) | `make deploy-up` |
| Deploy (local tunnel) | `make deploy-local` |
| Stop deploy | `make deploy-down` |

## Environment

- `PORT`, `APP_NAME`, `DATABASE_URI`, `CORS_ALLOWED_ORIGINS` — defaults in `shared/env/env.go`
- **GitHub token:** read `OPENCODE_PR` from repo-root `.env` (gitignored); never print/commit; use via `Authorization` header / push URL only

## Conventions (Repo-wide)

- **Typed errors** — see `shared/apperr/AGENTS.md` (reference)
- **JSON decoding** — use `shared/json/decode.go`
- **Logging** — use `shared/logger` (zerolog-based)
- **Mocks** — generated via `mockery` (config in `go.mod` tool directive)
- **English only** — all identifiers, comments, log messages, and user-facing strings must be in English. Exception: `docs/TDD.md` stays in pt-BR.

## References

- `internal/AGENTS.md` — architecture, layering, service conventions
- `internal/sheet/template/AGENTS.md` — DAG, Template, Registry, Gotchas
- `internal/engine/AGENTS.md` — formula engine (expr-lang wrapper)
- `shared/apperr/AGENTS.md` — error codes, HTTP mapping, boundary translation
- `shared/db/AGENTS.md` — MongoDB initialization
