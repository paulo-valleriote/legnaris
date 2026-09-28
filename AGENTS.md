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

## Comments

Comments are documentation, never narration. They must not substitute clear code.

**Allowed**

- Doc comments on exported identifiers: functions, methods, types, interfaces,
  and package-level constants/variables (Go doc-comment style, starting with the
  identifier name: `// coerceFloat64 normalizes ...`).
- Comments explaining **why**: constraints, trade-offs, gotchas, and non-obvious
  decisions that the code cannot express by itself.
- Package headers (e.g. `doc.go`).

**Not allowed**

- Comments inside function bodies — no line-by-line narration. Refactor instead:
  better names, smaller functions, early returns.
- Commented-out code — delete it; Git history keeps it.

**TODO / FIXME**

- Create a Jira card for the pending work first.
- Only if a card is not possible (e.g. trivial or blocked), leave a `// TODO(owner):`
  comment with a one-line rationale.

**Exempt**

- Generated code (mockery) and directives (`//go:generate`, `//nolint`, build tags).

## PR Checklist

Before opening a PR, ensure:

- Run all required linters and formatters: `make fmt && make lint && make vet`
- If the PR contains code changes, also run tests with race detector: `make test-race`

## References

- `internal/AGENTS.md` — architecture, layering, service conventions
- `internal/sheet/template/AGENTS.md` — DAG, Template, Registry, Gotchas
- `internal/engine/AGENTS.md` — formula engine (expr-lang wrapper)
- `shared/apperr/AGENTS.md` — error codes, HTTP mapping, boundary translation
- `shared/db/AGENTS.md` — MongoDB initialization
