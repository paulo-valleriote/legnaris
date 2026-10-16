# Pinned backend captures

Contract fixtures for the web API client (API-34, API-35). Each `*.json` file is a
real response captured from the local backend, or the request that produced it.
The pinned suite decodes them with **no backend running**, and a shape change in
any capture fails the suite (API-36).

## Reproducing

```bash
make dev-db                          # MongoDB for local development
go run ./cmd/api.go                  # the API on its local port (default :8080)
bash web/src/api/fixtures/capture.sh # registers a throwaway user and captures every case
```

`capture.sh` reads `API_BASE_URL` (default `http://localhost:8080`), registers a
throwaway user, exercises the six consumed operations and the failure cases, and
writes each case here. It never writes a credential: the access token lives only
in a temporary directory that the script deletes on exit.

The files are excluded from Prettier (`.prettierignore`) so their bytes stay as
the server sent them.

## File shape

A response capture records the request that produced it next to the raw status
and body:

```json
{
  "operation": "health",
  "provenance": "captured from the local stack … on <timestamp>",
  "request": { "method": "GET", "path": "/health" },
  "response": { "status": 200, "body": { "data": { "status": "healthy" } } }
}
```

`sheet_create_request.json` and `patch_delta_request.json` are request-only files,
kept separate so the update pair is inspectable as a pair.

## Inventory

| File                                        | Operation                         | Expected                                                                         |
| ------------------------------------------- | --------------------------------- | -------------------------------------------------------------------------------- |
| `health.json`                               | `GET /health`                     | 200, the envelope-wrapped health payload                                         |
| `templates.json`                            | `GET /templates`                  | 200, the whole catalog                                                           |
| `template_detail.json`                      | `GET /templates/fantasy_v1`       | 200, groups, a formula, dependencies, an exclusive group and derived leaves      |
| `sheet_create_request.json` / `sheet_create.json` | `POST /sheets`              | the request pair and a 201 carrying derived values                               |
| `sheet_get.json`                            | `GET /sheets/:id`                 | 200, the stored sheet                                                             |
| `patch_delta_request.json` / `patch_delta_response.json` | `PATCH /sheets/:id`    | a single-change request and a 200 carrying only recalculated effects             |
| `error_not_found.json`                      | `GET /sheets/:id` (deleted)       | 404 `NOT_FOUND`                                                                   |
| `error_invalid_request.json`                | `PATCH /sheets/:id` (derived)     | 400 `INVALID_REQUEST`                                                             |
| `error_unknown_field.json`                  | `PATCH /sheets/:id` (unknown key) | 400 `UNKNOWN_FIELD`                                                               |
| `error_invalid_json.json`                   | `POST /sheets` (malformed body)   | 400 `INVALID_JSON`                                                                |
| `error_internal.json`                       | `GET /no-such-route`              | 500 `INTERNAL_ERROR` (an unmatched route surfaces through the global handler)     |
| `error_unauthorized.json`                   | `GET /templates` (no credential)  | 401 `UNAUTHORIZED`                                                                |

**A capture is never hand-edited to make a test pass.** A capture that stops
matching the models is a backend contract change: the suite is expected to go red
on it, and the models are updated deliberately — never the capture.
