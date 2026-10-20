# AGENTS.md — backend/shared/apperr (Typed Application Errors)

## Purpose

Central error type for **services and controllers** only. Lower layers (repository, template, engine, backend/shared/db, etc.) **must NOT import this package**.

## Usage (Services & Controllers)

```go
// Create
apperr.NewError(code, message, err)

// Causes
// - err = nil, or
// - lower-layer typed error (repository/DAG/json/maputils/…)
// - NEVER bare errors.New()
// - NO package-level errors.New sentinels in backend/internal/app or backend/internal/sheet
```

## Error Codes & HTTP Status Mapping

Defined in `err.go`. When adding a new `apperr.Code`:

1. Add HTTP status mapping in `apperr.NewError` — unmapped codes fall back to 500 via `HttpStatus()`
2. Confirm `HTTPErrorHandler` (`backend/internal/app/router.go`) serializes it correctly
3. Add test asserting the status mapping for the new error

## Boundary Translation (Service Layer)

Map lower-layer errors to `apperr` using `errors.Is`/`errors.As`:

```go
if errors.Is(err, repository.ErrSheetNotFound) {
    return apperr.NewError(apperr.CodeNotFound, "sheet not found", err)
}
```

This ensures `HTTPErrorHandler` returns correct status instead of 500 `INTERNAL_ERROR` fallback.

## Message Constants

Export user-facing message constants (plain strings) from the owning service package so controllers reuse wording. These are **not errors** — just strings.