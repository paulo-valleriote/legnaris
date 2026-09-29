# AGENTS.md — shared/idgen (Server-Generated IDs)

## Purpose

Single source of truth for the repository-wide ID contract: **every ID the
server generates is a canonical lowercase UUID v7 string** (RFC 9562).

## Contract

- **Generate** — `idgen.New() (string, error)`.
  - Time-ordered: the 48-bit millisecond prefix makes lexicographic order
    equal creation order, so listings/pagination can sort by `_id` and the
    `_id` index stays append-mostly.
  - Monotonic **per process** (12-bit sequence); across instances ordering
    depends on clock sync.
  - The only failure mode is crypto/rand exhaustion — translate it to
    `apperr.CodeInternal`.
- **Validate** — `idgen.Validate(id) error` returns `ErrInvalidID` for
  anything that is not exactly what `New` produces: other versions (v4, v1),
  other variants, uppercase, `urn:uuid:`/braced/no-hyphen encodings, garbage.
  Call it at the **service boundary before any repository call** and map it to
  `apperr.CodeInvalidRequest` (400) so obviously invalid input fails fast
  instead of becoming a 404 lookup.
- **Never** call `uuid.New()`/`uuid.NewString()` directly — they mint v4.

## Layering

- Imports only `github.com/google/uuid`; this package must **NOT** import
  `shared/apperr` (low-layer rule). Services wrap `ErrInvalidID` into their
  typed domain error with `apperr.NewError`.
- `ErrInvalidID` is a plain sentinel: check with `errors.Is`.

## Exemptions

- **Template IDs** are code-defined slugs (`fantasy_v1`), not generated —
  `idgen` does not apply to them. They carry their own rule instead:
  `template.FetchTemplate` rejects anything outside `^[a-z][a-z0-9_]*$` with
  `ErrTemplateIDInvalid` (400 at the boundary).
- IDs received from clients or path parameters still go through `Validate`
  before touching a repository; validation is defense in depth, not
  authorization (ownership guards are a separate concern).
