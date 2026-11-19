# AGENTS.md — backend/internal/engine (Formula Engine)

## Rules

- expr-lang wrapper providing `floor` and `max`, with normalized int/float coercion; pure evaluation only — no side effects, no external dependencies.
- Declare own typed errors — **do NOT import `apperr`**.
- Wrap errors with `fmt.Errorf("...: %w", err)`; the service layer translates them at the boundary.
- All identifiers, comments and logs in English.
