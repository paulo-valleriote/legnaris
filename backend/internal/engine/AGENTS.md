# AGENTS.md — backend/internal/engine (Formula Engine)

## Responsibility

- expr-lang wrapper providing `floor`, `max` functions
- Normalized int/float coercion for `floor`/`max`
- Pure evaluation logic — no side effects (import allow-list: depguard `engine-allowlist`)

## Conventions

- Declare own typed errors (the `apperr` ban and `%w` wrapping are enforced by depguard and errorlint)
- Service layer translates to `apperr` at boundary
- All identifiers, comments, logs in English