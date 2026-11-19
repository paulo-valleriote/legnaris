# AGENTS.md — Legnaris

## Quick Start

```bash
make setup               # Enable the pre-commit hook (formats staged Go and web files)
make dev                 # Clean MongoDB + API + web, one command (see README.md)
go run ./backend/cmd/api.go  # Run API alone (MongoDB required; no Mongo yet? `make dev-db`)
go test ./...            # Run tests
make mocks               # Generate mocks (after interface changes)
make fmt                 # Format code
```

## Commands

| Task | Command |
|------|---------|
| Enable git hooks (once per clone) | `make setup` |
| Run everything locally (clean DB + API + web) | `make dev` |
| Run server | `make run` |
| Build | `make build` |
| Test (race) | `make test-race` |
| Lint | `make lint` |
| Vet | `make vet` |
| Run local MongoDB (dev only) | `make dev-db` |
| Deploy (production) | `make deploy-up` |
| Stop deploy | `make deploy-down` |

## Environment

- `PORT`, `APP_NAME`, `DATABASE_URI`, `CORS_ALLOWED_ORIGINS` — defaults in `backend/shared/env/env.go`
- **GitHub token:** `OPENCODE_PR` in repo-root `.env`; use it only in an `Authorization` header or push URL (`scripts/hooks/guard-secrets.sh` and gitleaks enforce the rest)

## Conventions (Repo-wide)

- **Typed errors** — see `backend/shared/apperr/AGENTS.md` (reference)
- **IDs** — server-generated entity IDs are lowercase UUID v7 via `backend/shared/idgen` (`New`, `Validate` at the service boundary). Template IDs are code-defined slugs validated as `^[a-z][a-z0-9_]*$` in `FetchTemplate` — see `backend/shared/idgen/AGENTS.md`
- **Banned calls and imports** (`uuid.New`, raw JSON decoding, `fmt.Print*`, `log`, layering) — enforced by forbidigo and depguard in `.golangci.yml`; each message names the sanctioned replacement
- **English only** — identifiers, comments, logs and user-facing strings; `docs/TDD.md` stays pt-BR. Heuristic check: `make english` (allow-list in `scripts/ci/english-allowlist.txt`)

## Testing

**Never commit code without tests.** Any change that adds or modifies behavior ships
tests in the same commit. Exceptions (generated code, docs/CI-only changes, pure
refactors) need the `no-tests:refactor` PR label; the `tests-required` CI job checks it.

Every test suite must cover, in this order of priority:

1. **Edge cases (the main focus)** — nil/empty/zero values, boundary values, invalid
   and unexpected input, missing keys, already-applied changes (idempotency),
   concurrency. Failure paths must assert the typed error (`apperr` code) and, at the
   HTTP boundary, the mapped status code.
2. **Happy path** — a representative valid input proving the code does exactly what it
   claims (correct output, side effects, cascades).

**Conventions**

- Table-driven tests with case names prefixed `Happy path:`, `Edge:` or `Error:` (`make testnames`).
- `testify`: `require` when a failure invalidates the rest of the test, `assert` otherwise (testifylint covers the mechanical cases).
- Mocks are mockery-generated in `<pkg>/mocks/` via `NewMockX(t)`; run `make mocks` after interface changes (`make mocks-check` fails on drift or hand-written mocks).
- Tests live next to the code (`*_test.go`); use `package X_test` when only the public
  API is exercised. Shared fixtures go in a dedicated package (e.g.
  `backend/internal/sheet/sheettest`).
- Unit tests need no external services; integration tests self-skip without MongoDB (the `go-test-nodb` CI job proves it).
- Run: `go test ./...` · single test: `go test -run TestName ./pkg/path` · `make test-race`.

## Comments

No comments in source code (Go, TS/TSX): names, small functions and tests carry intent;
Git history and Jira carry rationale. Only functional directives are allowed; the list
lives in `backend/cmd/nocomments` (`make nocomments`, also run by CI, pre-commit and a
Claude hook). An intentionally empty `catch {}` needs no comment.

## PR Checklist

The CI `gate` job must be green; run `make lint test-race` locally (a Claude Stop hook
runs lint and tests for touched packages).

## References

- `backend/internal/AGENTS.md` — architecture, layering, service conventions
- `backend/internal/sheet/template/AGENTS.md` — DAG, Template, Registry, Gotchas
- `backend/internal/engine/AGENTS.md` — formula engine (expr-lang wrapper)
- `backend/internal/dice/AGENTS.md` — Dice expression grammar; shared vectors in `testdata/dice/`
- `backend/shared/apperr/AGENTS.md` — error codes, HTTP mapping, boundary translation
- `backend/shared/idgen/AGENTS.md` — server-generated ID contract (UUID v7, generate/validate)
- `backend/shared/db/AGENTS.md` — MongoDB initialization
- `web/AGENTS.md` — React PWA client: security, API client, engine, sheet cycle, dependencies (visual rules live in `web/DESIGN.md`)
