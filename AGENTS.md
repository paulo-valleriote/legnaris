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
- **GitHub token:** read `OPENCODE_PR` from repo-root `.env` (gitignored); never print/commit; use via `Authorization` header / push URL only

## Conventions (Repo-wide)

- **Typed errors** — see `backend/shared/apperr/AGENTS.md` (reference)
- **IDs** — every server-generated entity ID is a canonical lowercase UUID v7 via `backend/shared/idgen` (`New` to generate, `Validate` at the service boundary); never call `uuid.New`/`uuid.NewString` directly. Template IDs are code-defined slugs, exempt from `idgen` but validated as `^[a-z][a-z0-9_]*$` in `FetchTemplate` — see `backend/shared/idgen/AGENTS.md`
- **JSON decoding** — use `backend/shared/json/decode.go`
- **Logging** — use `backend/shared/logger` (zerolog-based)
- **Mocks** — generated via `mockery` (config in `go.mod` tool directive)
- **English only** — all identifiers, comments, log messages, and user-facing strings must be in English. Exception: `docs/TDD.md` stays in pt-BR.

## Testing

**Never commit code without tests.** Any change that adds or modifies behavior must
ship with tests in the same commit — code without tests is not done.

**Exceptions:** generated code (mockery), changes with no behavior (docs, comments, CI
config), and pure refactors that keep the public contract.

Every test suite must cover, in this order of priority:

1. **Edge cases (the main focus)** — nil/empty/zero values, boundary values, invalid
   and unexpected input, missing keys, already-applied changes (idempotency),
   concurrency. Failure paths must assert the typed error (`apperr` code) and, at the
   HTTP boundary, the mapped status code.
2. **Happy path** — a representative valid input proving the code does exactly what it
   claims (correct output, side effects, cascades).

**Conventions**

- Table-driven tests: `tests := []struct{ name string; ... }` + `t.Run(tc.name, ...)`;
  prefix case names with `Happy path:`, `Edge:` or `Error:` so intent is scannable.
- `testify`: `require` for setup/preconditions and assertions that invalidate the rest
  of the test; `assert` for everything else.
- Mocks: mockery-generated in `<pkg>/mocks/`, built via `NewMockX(t)` (cleanup asserts
  expectations). Never hand-write mocks; run `make mocks` after interface changes.
- Tests live next to the code (`*_test.go`); use `package X_test` when only the public
  API is exercised. Shared fixtures go in a dedicated package (e.g.
  `backend/internal/sheet/sheettest`).
- Unit tests must not require external services; integration tests self-skip when
  MongoDB is unavailable (see `deploy/` stack).
- Run: `go test ./...` · single test: `go test -run TestName ./pkg/path` ·
  `make test-race` (race detector must stay clean).

## Comments

No comments in source code (Go, TS/TSX). Names, small functions, and tests carry the
intent; Git history and Jira carry the rationale. This covers doc comments, package
headers, "why" notes, TODO/FIXME, and commented-out code.

**Allowed (functional directives only)**

- Go: `//go:*` (`//go:embed`, `//go:generate`, `//go:build`), `//nolint`, and the
  `// Code generated ... DO NOT EDIT.` header of generated code.
- TS/TSX: `/// <reference ... />`, `@ts-expect-error`, `eslint-disable*` (without a
  `-- reason` suffix), `prettier-ignore`.

Generated code (mockery) is exempt. An intentionally empty `catch {}` needs no comment
(ESLint `no-empty` allows empty catch blocks).

## PR Checklist

Before opening a PR, ensure:

- Run all required linters and formatters: `make fmt && make lint && make vet`
- Every behavior change ships tests covering edge cases + happy path (exceptions:
  generated/trivial changes); run them with race detector: `make test-race`

## References

- `backend/internal/AGENTS.md` — architecture, layering, service conventions
- `backend/internal/sheet/template/AGENTS.md` — DAG, Template, Registry, Gotchas
- `backend/internal/engine/AGENTS.md` — formula engine (expr-lang wrapper)
- `backend/internal/dice/AGENTS.md` — Dice expression grammar; shared vectors in `testdata/dice/`
- `backend/shared/apperr/AGENTS.md` — error codes, HTTP mapping, boundary translation
- `backend/shared/idgen/AGENTS.md` — server-generated ID contract (UUID v7, generate/validate)
- `backend/shared/db/AGENTS.md` — MongoDB initialization
- `web/AGENTS.md` — React PWA client: security, API client, engine, sheet cycle, dependencies (visual rules live in `web/DESIGN.md`)
