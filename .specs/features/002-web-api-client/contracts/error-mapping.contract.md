# Contract: Failure classification and copy

**Requirements**: API-15…API-24 | **Backing documents**: `spec.md` (US3),
[research.md R4, R10](../research.md), `shared/apperr/err.go`, `shared/response/response.go`

Every failure that leaves the client is one of three typed shapes and carries a classification the
interface can branch on without parsing anything.

## Response classification

| Response | Result | Requirements |
|---|---|---|
| 2xx, valid JSON, object with `data` | decode `data` with the operation's decoder | API-02 |
| 2xx, empty body / invalid JSON / no `data` key | `DefensiveParseFailure` | API-04 |
| non-2xx, valid JSON, `error.code` a non-empty string | `DomainFailure` with `status`, `code`, `message` | API-03, API-24 |
| non-2xx, empty body / invalid JSON / no `error.code` | `DefensiveParseFailure` (never a `DomainFailure`) | API-22 |
| transport rejection with attempts exhausted | `DomainFailure` with `kind: 'offline'`, no `status` | API-20 |

`error.message` absent or not a string becomes an empty string on the domain failure — the code, not the
message, is what classification depends on.

## Code → kind

The table is total over the twelve codes the backend defines in `shared/apperr/err.go`.

| Backend code | HTTP | Kind | Requirements |
|---|---|---|---|
| `INVALID_JSON` | 400 | `clientFault` | API-15 |
| `INVALID_REQUEST` | 400 | `invalidEdit` | API-16 |
| `UNKNOWN_FIELD` | 400 | `renderMismatch` | API-17 |
| `NOT_FOUND` | 404 | `notFound` | API-18 |
| `INTERNAL_ERROR` | 500 | `serverError` | API-19 |
| `CONFLICT` | 409 | `unknown` | API-21 |
| `CYCLE_DETECTED` | 400 | `unknown` | API-21 |
| `INVALID_DEPENDENCY` | 400 | `unknown` | API-21 |
| `INVALID_PATH` | 400 | `unknown` | API-21 |
| `SHEET_LIMIT_REACHED` | 409 | `unknown` | API-21 |
| `UNAUTHORIZED` | 401 | `unknown` | API-21 |
| `FORBIDDEN` | 403 | `unknown` | API-21 |
| anything else | any | `unknown` | API-21 |

The seven fallback codes are a decision, not an omission: none of them has a defined interface treatment
in this phase, and each returns the generic kind while the raw code and server message stay on the error
for the feature that will define that treatment. The status column is documentation of the backend's
mapping, never the input to classification — two codes share 400.

## Kind → message key

No user-visible literal exists in `web/src/api/`. The mapper resolves each kind to a key in
`web/src/l10n/strings.ts`, and the catalog holds the English copy (API-23).

| Kind | Catalog key |
|---|---|
| `clientFault` | `api.error.clientFault` |
| `invalidEdit` | `api.error.invalidEdit` |
| `renderMismatch` | `api.error.renderMismatch` |
| `notFound` | `api.error.notFound` |
| `serverError` | `api.error.serverError` |
| `offline` | `api.error.offline` |
| `unknown` | `api.error.unknown` |
| defensive parse failure | `api.error.parse` |

## Test obligations

- Table-driven over the twelve codes plus one code the backend does not define, asserting the kind and
  that an unmapped code keeps its raw code and message (API-21).
- Cases for: a 2xx without `data`, an empty body, invalid JSON on both branches, a failure body without
  `error.code`, a non-string `error.code`, and a missing `error.message`.
- A transport failure with attempts exhausted asserting `offline` and an absent `status`.
- One case per kind asserting the resolved copy is byte-equal to the catalog entry (API-23), which is
  the assertion that proves the message came from the translation layer.
