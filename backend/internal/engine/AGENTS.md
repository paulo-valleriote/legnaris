# AGENTS.md — backend/internal/engine (Formula Engine)

## Rules

- expr-lang wrapper providing `floor`, `max`, `min` and `lookup`, with normalized int/float coercion; pure evaluation only — no side effects, no external dependencies.
- `min(a, b)` shares the arity, coercion and `ErrInvalidFloat` failures of `max`. `lookup(x, k1, v1, k2, v2, ...)` takes an input plus at least one key/value pair with strictly ascending keys, and returns the value of the greatest key less than or equal to `x`; below the first key it returns the first value and above the last the last value. Wrong arity, unordered keys and non-numeric arguments are typed engine errors (`ErrInvalidLookupArity`, `ErrInvalidLookupKeyOrder`, `ErrInvalidFloat`).
- Declare own typed errors — **do NOT import `apperr`**.
- Wrap errors with `fmt.Errorf("...: %w", err)`; the service layer translates them at the boundary.
- All identifiers, comments and logs in English.
