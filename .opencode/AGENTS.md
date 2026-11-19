# AGENTS.md — PR Review Checklist (AI Reviewers)

> Loaded only during PR review. Not part of development-time instructions.

## Automated Review Criteria

AI findings are advisory. The CI `gate` job in `.github/workflows/ci.yml` is blocking: it needs `go-lint` (golangci-lint plus the repo scanners), `go-race`, `go-test-nodb`, `tests-required`, `pr-title` and `web-quality`.

Mechanical rules (complexity, duplication, imports, error wrapping, resource leaks, comments, English-only, test naming) are enforced there; see `.golangci.yml` and the `go-lint` steps. Review only what those checks cannot decide.

### Cyclomatic Complexity
- Threshold lives in `.golangci.yml` (`gocyclo`). Judge whether a flagged split actually improves readability.

### Race Conditions
- No unsynchronized access to shared state; prefer channels or mutexes. `go-race` runs the race detector.

### Memory Leaks
- `bodyclose` and `noctx` cover HTTP bodies and contexts. Still check Mongo cursors are closed and goroutines are bounded.

### Card Compliance
- `pr-title` guarantees an `LGN-\d+` key. Verdicts: Met (requires file:line evidence from the diff), Not evidenced, Out of scope of this diff. Criteria come only from the card; never invent requirements.
