# Contract: HTTP surface — sign-in hardening

**Feature**: `.specs/features/018-auth-2fa-session-gate/` | Requirements: `TFA-01`, `TFA-02`, `TFA-03`, `TFA-04`, `TFA-05`, `TFA-10`, `TFA-12`

Only the two credential-proving routes change shape, and only while the gate is enabled. The
challenge routes (`017`) keep their contract byte-for-byte (`TFA-11`); the new read is additive.

## Surface

| Method & path | Guard | Gate disabled | Gate enabled |
|---|---|---|---|
| `POST /auth/register` | per-address + per-account rate limit | `201` + session body + both cookies | `201` + challenge body, **no cookie** |
| `POST /auth/login` | per-address + per-account rate limit | `200` + session body + both cookies | `202` + challenge body, **no cookie** |
| `POST /auth/challenge/verify` | per-address rate limit (unchanged) | unchanged | unchanged (`200` + session body + both cookies) |
| `POST /auth/challenge/resend` | per-address rate limit (unchanged) | unchanged | unchanged (`202` + retry window) |
| `GET /auth/me` | bearer access credential (**new**) | `200` + user DTO | `200` + user DTO |
| `POST /auth/refresh` | per-address limit + origin guard | unchanged | refusal added (§ refresh gate) |
| `POST /auth/logout` | origin guard | unchanged | unchanged |

## `POST /auth/register`

**Request** (unchanged): `{ "email", "username", "password" }` decoded through `shared/json`.

**Gate enabled — `201 Created`**

```json
{ "challenge_id": "<uuid-v7>", "expires_in": 600, "resend_after": 60 }
```

This is the existing `auth.ChallengeIssued` shape (`OTP-10`), so the client needs no second shape.
The response sets **no** `Set-Cookie`.

**Gate disabled — `201 Created`** (unchanged, `AUTH-02`): `{ "access_token", "token_type", "expires_in" }`
plus the `__Host-` refresh cookie and the CSRF cookie.

**Failure outcomes** (identical in both states unless noted):

| Case | Code | Status |
|---|---|---|
| Malformed JSON / unknown field | `INVALID_JSON` / `UNKNOWN_FIELD` | 400 |
| Unparseable address, invalid display name, short/long password | `INVALID_REQUEST` | 400 |
| Duplicate canonical address | `CONFLICT` | 409 — no challenge, no send |
| Daily send allowance spent (gate enabled) | `EMAIL_UNAVAILABLE` (+`Retry-After`) | 503 — account exists, no session |
| Store failure while starting the code step (gate enabled) | `INTERNAL_ERROR` | 500 |

## `POST /auth/login`

**Request** (unchanged): `{ "email", "password" }`.

**Gate enabled — `202 Accepted`**: the same `{ "challenge_id", "expires_in", "resend_after" }` body,
no `Set-Cookie`. `202` (not `200`) is the accepted outcome the program plan fixes for the first step.

**Gate disabled — `200 OK`** (unchanged, `AUTH-01`): session body + both cookies.

**Failure outcome** (both states): any credential failure — unknown address, unparseable address,
wrong password — answers the one uniform outcome `UNAUTHORIZED` / 401 with `MsgInvalidCredentials`,
and starts nothing. The password comparison is performed in every one of those cases (`TFA-05`,
`R5`).

## Invariants

1. **No credential on a code-step answer** (`SC-001`): while the gate is enabled, `register` and
   `login` set no cookie and their bodies carry no access token, refresh token or CSRF token. Asserted
   by inspecting both the body and the `Set-Cookie` header.
2. **Purpose is server-derived** (`TFA-03`): the request bodies carry no purpose, and the service
   chooses `verify_email` for registration and `login_otp` for sign-in from the operation itself.
3. **One uniform credential failure** (`TFA-02`): the four failure cases above cannot be told apart
   by status or body.
4. **Legacy parity** (`TFA-12`, `SC-004`): with the gate disabled, status, body and cookies are
   exactly today's — the same service methods run, only the tail differs (`R1`).
5. **Guards unchanged** (`TFA-10`): both routes keep the limiter they already carry; `verify`/`resend`
   keep the per-address limiter and no origin guard (`OTP-19`); `/auth/me` is the only route to add a
   guard and it reuses the existing bearer middleware.

## Tests

- Both gate states × {happy registration, duplicate, invalid input, happy sign-in, wrong password,
  unknown address} at the HTTP boundary, asserting status, body and `Set-Cookie` presence.
- The uniform-failure assertion across the four credential failures.
- The route inventory test gains `/auth/me` and keeps every existing path.
- Malformed-body handling for the new route's absence of a body (it takes none).
