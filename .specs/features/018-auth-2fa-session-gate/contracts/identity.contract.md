# Contract: Reading the signed-in account

**Feature**: `.specs/features/018-auth-2fa-session-gate/` | Requirements: `TFA-08`, `TFA-09`, `TFA-10`, `TFA-12`

## Route

| | |
|---|---|
| Method & path | `GET /auth/me` |
| Guard | existing bearer middleware (`auth.Middleware(modules.Auth.TokenManager)`), applied per route on the `/auth` group |
| Body | none — the account identity comes from the credential, never from the request |
| Success | `200 OK` with the standard success envelope carrying the user DTO |

Mounting the guard per route keeps the route with its auth siblings while reusing the one token
parser (`R4`). No new middleware, no new group.

## Payload

```json
{
  "id": "<uuid-v7>",
  "email": "player@example.com",
  "username": "player",
  "email_verified": false,
  "created_at": "2026-10-20T12:00:00Z"
}
```

`email_verified` is **new** on `user.UserDTO`; the other fields are today's. The field is derived in
`ToDTO` from `EmailVerifiedAt != nil` — one projection, one definition (`R4`).

## Invariants

1. **Boolean, not instant** (`TFA-08`, `SC-006`): no response, log line or error carries
   `email_verified_at` or any timestamp derived from it. The stored value stays internal; this
   supersedes `PRQ-12`/`PRQ-14` for the read path only (`TFA-09`).
2. **No credential or hash** (`TFA-09`): the payload never contains `password_hash`, an access token,
   a refresh token or a CSRF token.
3. **Gate-independent** (`TFA-12`): the read is available and truthful whether the gate is enabled or
   disabled, so a client can always learn its state after a reload.
4. **Read-only** (`TFA-11`): the route performs one `FetchByID` and writes nothing; it cannot mark an
   account verified.

## Failure outcomes (`TFA-10`)

| Case | Code | Status |
|---|---|---|
| No `Authorization` header | `UNAUTHORIZED` (existing middleware message) | 401 |
| Malformed, expired or forged access credential | `UNAUTHORIZED` (existing middleware message) | 401 |
| Credential valid, account no longer exists | `UNAUTHORIZED` (new message constant) | 401 |
| Account-store failure | `INTERNAL_ERROR` | 500 |

The route accepts no request body; a body sent anyway is ignored (nothing is decoded).

## Tests

- Unverified account → `200`, `email_verified: false`, all other fields present.
- Verified account → `200`, `email_verified: true`.
- The response body and the log lines contain no `email_verified_at` and no credential
  (pattern assertion, mirroring `OTP-15`'s style).
- No credential / malformed / expired / forged → 401 with no account data.
- Account deleted after the credential was minted → 401, not 404, and no account data.
- Store failure → 500.
- The route inventory/wiring test asserts the route is mounted behind the guard.
