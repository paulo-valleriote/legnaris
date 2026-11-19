# AGENTS.md — backend/shared/apperr

Why errors are typed and translated at the boundary:
[`context/architecture.md`](../../../context/architecture.md).

## Working rules

- Services and controllers may import this package; repositories, engine, db,
  json, idgen and other data-layer packages must not.
- Create with `apperr.NewError(code, message, cause)`. The cause is `nil` or a
  lower-layer typed error, never a bare `errors.New`.
- Do not declare `errors.New` sentinels in `backend/internal/app` or domain
  packages; declare them in the owning lower layer.
- A new `apperr.Code` needs its HTTP status in `err.go`, a working
  `HTTPErrorHandler` path and a test for the mapping.
- Translate lower-layer errors with `errors.Is`/`errors.As` so they do not fall
  back to 500.
- Export user-facing message constants from the owning service package; they
  are strings, not errors.
