# Pinned backend captures

Contract fixtures for the web API client (API-34, API-35). Each `*.json` file is a
real response captured from the local backend, or the request that produced it.
The pinned suite decodes them with **no backend running**, and a shape change in
any capture fails the suite (API-36).

## Reproducing

```bash
make dev-db                          # MongoDB for local development
go run ./backend/cmd/api.go                  # the API on its local port (default :8080)
bash web/src/api/fixtures/capture.sh # registers a throwaway user and captures every case
```

`capture.sh` reads `API_BASE_URL` (default `http://localhost:8080`), registers a
throwaway user, exercises the consumed operations and the failure cases, and
writes each case here. It never writes a credential: the access token lives only
in a temporary directory that the script deletes on exit.

The files are excluded from Prettier (`.prettierignore`) so their bytes stay as
the server sent them.

## The two gate states

The stack answers either a session or a code step, and the script follows the
answer it actually received: the session family on a stack whose gate is off, the
code-step family on a stack whose gate is on (R8). The other family's files are
left untouched, so **one run per gate state** refreshes the whole set:

```bash
# the session family
AUTH_REQUIRE_EMAIL_VERIFICATION=false bash web/src/api/fixtures/capture.sh

# the code-step family
AUTH_REQUIRE_EMAIL_VERIFICATION=true bash web/src/api/fixtures/capture.sh
```

The script never assumes a gate state: registration answers `201` in both, so
the branch reads the payload's own shape (`R1`).

### What the code-step run needs

- `mongosh` on the `PATH`, to read the code the stack recorded. The code never
  reaches a log line or the sender's output, so the durable send intention is
  the only place it can be read (`EML-10`, `EML-13`).
- `DATABASE_URI` and `DATABASE_NAME` (default `legnaris`) pointing at the
  stack's database.
- The daily send allowance to be spendable: the run spends it on purpose, to
  capture the unavailable-send refusal. The run removes the send intentions it
  recorded, so the next run starts from the same budget.

A short local configuration makes the run quick:

```bash
AUTH_REQUIRE_EMAIL_VERIFICATION=true AUTH_OTP_DAILY_SEND_CAP=5 AUTH_RATE_LIMIT_ATTEMPTS=200 make run
```

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

The credential requests record their method and path but never their body: the
body carries the password. The verification capture keeps the response's own
shape and drops the credential — its `access_token` holds the
`redacted-access-token` placeholder, so no token is ever committed (`API-35`).

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
| `auth_register_challenge.json`              | `POST /auth/register` (gate on)   | 201, the started code step and no cookie                                          |
| `auth_login_challenge.json`                 | `POST /auth/login` (gate on)      | 202, the started code step and no cookie                                          |
| `auth_challenge_verify.json`                | `POST /auth/challenge/verify`     | 200, the session body with a redacted token                                       |
| `auth_challenge_resend.json`                | `POST /auth/challenge/resend`     | 202, the advertised retry window                                                  |
| `error_otp_invalid.json`                    | `POST /auth/challenge/verify`     | 401 `OTP_INVALID` — the one uniform refusal for every wrong code                  |
| `error_email_unavailable.json`              | `POST /auth/register` (cap spent) | 503 `EMAIL_UNAVAILABLE`                                                           |
| `error_conflict.json`                       | `POST /auth/register` (duplicate) | 409 `CONFLICT`                                                                    |

**A capture is never hand-edited to make a test pass.** A capture that stops
matching the models is a backend contract change: the suite is expected to go red
on it, and the models are updated deliberately — never the capture.
