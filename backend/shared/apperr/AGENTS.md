# AGENTS.md — backend/shared/apperr (Typed Application Errors)

## Purpose

Central error type for **services and controllers** only. Lower layers declare their own typed errors (depguard `lower-layers-no-apperr` enforces the import ban).

## Usage (Services & Controllers)

```go
// Create
apperr.NewError(code, message, err)

```

The cause is `nil` or a lower-layer typed error. Bare `errors.New` causes, `%w`-less `fmt.Errorf` causes and package-level sentinels in `app`/`sheet` fail lint (ruleguard rules in `backend/tools/rules/rules.go`); sheet sentinels live in `backend/internal/sheet/sheeterr`.

## Error Codes & HTTP Status Mapping

Defined in `err.go`. A new `apperr.Code` needs an explicit status mapping and a web `FailureKind`: `TestDeclaredCodesMapToExplicitStatus` and `TestCodesArtifactIsCurrent` (`make apperr-codes` regenerates `testdata/apperr/codes.json`) fail otherwise, and `web/src/test/api_error_mapper_codes.test.ts` checks the web side.

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