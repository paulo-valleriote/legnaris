# AGENTS.md — PR Review Checklist (AI Reviewers)

> Loaded only during PR review. Not part of development-time instructions.

## Automated Review Criteria

AI reviewers + linters evaluate PRs against these criteria. AI findings are advisory; the `ci-lint` workflow (golangci-lint + `go test -race`) is blocking on new issues.

### Cyclomatic Complexity
- Keep functions small; `gocyclo` flags complexity above 15.

### Code Duplication
- Extract shared logic; `dupl` flags near-duplicate blocks.

### Conventions
- Follow AGENTS.md: typed errors via `backend/shared/apperr`, JSON decoding via `backend/shared/json`, structured logging via `backend/shared/logger`, layering (app → sheet → template → engine → shared).

### English Only
- All identifiers, comments, log messages, user-facing strings in English. No Portuguese (pt-BR) code or strings.
- Exception: `docs/TDD.md` stays in pt-BR.

### Race Conditions
- No unsynchronized access to shared state; prefer channels or mutexes; `go test -race` must stay clean.

### Memory Leaks
- Close resources (files, DB cursors, response bodies), avoid unbounded goroutines, use context cancellation.

### Card Compliance
- The PR must implement its Jira card (LGN-\d+ in the title). Verdicts: Met (requires file:line evidence from the diff), Not evidenced, Out of scope of this diff. Criteria come only from the card; never invent requirements.