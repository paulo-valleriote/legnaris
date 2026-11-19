# AGENTS.md — backend/shared/idgen

Why IDs are UUID v7 and where they are validated:
[`context/architecture.md`](../../../context/architecture.md).

## Working rules

- `idgen.New()` is the only generator; map its crypto/rand failure to
  `apperr.CodeInternal`.
- Call `idgen.Validate` on every client-provided id before the first repository
  call and map `ErrInvalidID` to `apperr.CodeInvalidRequest` (400). Validation
  is defense in depth, not authorization.
- Never call `uuid.New()` or `uuid.NewString()` — both mint v4.
- This package imports only `github.com/google/uuid`; it must not import
  `apperr`.
- Template IDs are code-defined slugs, not generated: `FetchTemplate` enforces
  `^[a-z][a-z0-9_]*$` and returns `ErrTemplateIDInvalid`.
