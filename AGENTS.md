# AGENTS.md — Legnaris Backend

## Quick Start

```bash
make dev                 # Clean MongoDB + API + web, one command (see README.md)
go run ./backend/cmd/api.go  # Run API alone (MongoDB required; no Mongo yet? `make dev-db`)
go test ./...            # Run tests
make mocks               # Generate mocks (after interface changes)
make fmt                 # Format code
```

## Commands

| Task | Command |
|------|---------|
| Run everything locally (clean DB + API + web) | `make dev` |
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

Comments are documentation, never narration. They must not substitute clear code.

**Language**

- Always English — every comment (and identifier, log message, and user-facing
  string) must be written in English, never in Portuguese or any other language.

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
- Every behavior change ships tests covering edge cases + happy path (exceptions:
  generated/trivial changes); run them with race detector: `make test-race`

## References

- `backend/internal/AGENTS.md` — architecture, layering, service conventions
- `backend/internal/sheet/template/AGENTS.md` — DAG, Template, Registry, Gotchas
- `backend/internal/engine/AGENTS.md` — formula engine (expr-lang wrapper)
- `backend/shared/apperr/AGENTS.md` — error codes, HTTP mapping, boundary translation
- `backend/shared/idgen/AGENTS.md` — server-generated ID contract (UUID v7, generate/validate)
- `backend/shared/db/AGENTS.md` — MongoDB initialization
- `web/AGENTS.md` — React PWA client: visual rules, tokens, security rules, interaction patterns
- `web/AGENTS.md` — PWA React/TS rules; desktop deprecated and removed (F10)
