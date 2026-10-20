# AGENTS.md — backend/internal/engine (Formula Engine)

## Responsibility

- expr-lang wrapper providing `floor`, `max` functions
- Normalized int/float coercion for `floor`/`max`
- Pure evaluation logic — no side effects, no external deps

## Conventions

- Declare own typed errors — **do NOT import `apperr`**
- Wrap errors with `fmt.Errorf("...: %w", err)`
- Service layer translates to `apperr` at boundary
- All identifiers, comments, logs in English