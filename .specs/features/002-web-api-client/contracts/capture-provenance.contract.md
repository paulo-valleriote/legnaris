# Contract: Capture set and its provenance

**Requirements**: API-34…API-36, API-37…API-39 | **Backing documents**: `spec.md` (US6, US7),
[research.md R8, R9](../research.md)

The suite runs against pinned captures, so the contract is validated without a reachable backend and any
drift fails the build. A second suite, gated on a configured base address, validates the same client
against a real backend.

## Required captures

| Capture | Operation | Notes |
|---|---|---|
| `health.json` | `GET /health` | the envelope-wrapped health payload |
| `templates.json` | `GET /templates` | the whole catalog as the backend returns it |
| `template_detail.json` | `GET /templates/:id` | includes a group, a leaf with a formula, a leaf with dependencies, an exclusive group and a derived (non-editable) leaf |
| `sheet_create_request.json` / `sheet_create.json` | `POST /sheets` | the request pair and a 201 response carrying derived values |
| `sheet_get.json` | `GET /sheets/:id` | the stored sheet |
| `patch_delta_request.json` / `patch_delta_response.json` | `PATCH /sheets/:id` | the request carries a single change; the response carries only the recalculated effects |
| `error_not_found.json` | any route | `NOT_FOUND` (404) |
| `error_invalid_request.json` | `PATCH` with an invalid edit | `INVALID_REQUEST` (400) |
| `error_unknown_field.json` | `PATCH` with an unknown field | `UNKNOWN_FIELD` (400) |
| `error_invalid_json.json` | any route with a malformed body | `INVALID_JSON` (400) |
| `error_internal.json` | any route | `INTERNAL_ERROR` (500) |
| `error_unauthorized.json` | any protected route without a credential | `UNAUTHORIZED` (401) — exercises the generic fallback with a real capture |

A capture file records the request that produced it (`method`, `path`, optional `body`) next to the raw
`status` and `body`, so a capture is a contract pair rather than a bare payload.

## Provenance and reproduction

Captures are produced by the capture script under `web/src/api/fixtures/` against the local stack:

```bash
make dev-db                          # MongoDB container for local development
go run ./cmd/api.go                  # the API on its local port
bash web/src/api/fixtures/capture.sh # registers a user, exercises the six operations and the failures
```

The script writes each response body verbatim. A capture is never hand-edited to make a test pass; a
shape change in a capture is a backend contract change and the suite is expected to fail on it
(API-35).

## Rules

1. The pinned suite never needs a backend: `npm test` from `web/` must pass with the API and MongoDB
   stopped (API-34).
2. At least one capture exists per consumed operation, plus the failure envelopes and the update pair
   (API-35).
3. Altering a key in a capture turns the suite red on the first run — asserted explicitly as a
   drift-detection case, not left to chance (API-36, and SC-004).
4. The live suite runs only when `API_BASE_URL` is present in the test environment and reports as
   skipped otherwise, keeping the default run green without a backend (API-37).
5. With a base address configured, the live suite exercises the four real flows and fails naming the
   observed status and code on anything unexpected — it never steps aside once a base address exists
   (API-38, API-39).
6. No capture, and no failure the client produces, may contain a credential: the live suite never
   persists the token it registers with into a fixture file (the security rule of the transport
   contract applies to test artifacts too).

## Test obligations

- A capture-driven suite that decodes every success capture through the corresponding model and asserts
  the declared shape (API-34, API-35).
- A drift case that mutates a capture copy and asserts the decode fails (API-36).
- A live suite with four flow cases and two mechanism cases (that it skips without a base address, and
  that it is not silently skipped when one is present).
