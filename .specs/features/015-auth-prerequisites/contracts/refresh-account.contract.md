# Contract: Refresh resolves the account

**Feature**: `.specs/features/015-auth-prerequisites/` | **Requirement IDs**: `PRQ-05`…`PRQ-07`,
`PRQ-17` | **Owner**: `internal/auth` (service), `internal/app` (status mapping)

## Purpose

A renewal must not survive the account it names, and it must give the next features a point where
account state can be consulted.

## Order of checks (normative)

| # | Check | Failure outcome | Family | Credential issued |
|---|---|---|---|---|
| 1 | Credential present | 401 `MsgRefreshTokenRequired` | untouched | no |
| 2 | Credential lookup | 401 `MsgInvalidRefreshToken` (not found) / 500 `MsgInternal` (store) | untouched | no |
| 3 | CSRF bound to the family | 403 `MsgCSRFInvalid` | untouched | no |
| 4 | Revoked credential | existing reuse path: 401 `MsgInvalidRefreshToken` | revoked | no |
| 5 | Expiry (`now >= expires_at`) | 401 `MsgInvalidRefreshToken` | untouched | no |
| 6 | **Account lookup** | 401 `MsgInvalidRefreshToken` (missing) / 500 `MsgInternal` (store) | revoked on missing; untouched on store error | no |
| 7 | Access-token issue | 500 `MsgInternal` | untouched | no |
| 8 | Refresh rotation create | 500 `MsgInternal` | untouched | no |
| 9 | Rotation claim | 401 `MsgInvalidRefreshToken` (concurrent rotation, unchanged) | untouched (deliberate) | no |

A missing account **revokes the presented credential's family** (`PRQ-05`, `SC-003`). If that
revocation itself fails, the response is 500 and still no credential is issued: the 401 contract
cannot be claimed when its side effect did not happen.

## Success

The access token names the **loaded account's** id, and the rotated refresh credential belongs to
that same account and family (`PRQ-06`). The public session body and both cookies are unchanged.

## Failure taxonomy

| Failure | Typed error | Status | Message constant |
|---|---|---|---|
| Missing account | `apperr.CodeUnauthorized` | 401 | `auth.MsgInvalidRefreshToken` |
| Account-store failure | `apperr.CodeInternal` | 500 | `auth.MsgInternal` |

No new `apperr.Code` is introduced; the status mapping is asserted at the HTTP boundary
(`PRQ-17`). The two failure modes are distinguishable server-side (log/metric) but not by the
client.

## Invariants

- A request that cannot resolve its account issues **no** access token and **no** refresh credential.
- A revoked or expired credential keeps its existing outcome even when the account is missing.
- The account lookup happens after the existing credential rules and before the first issuance call.
- The CSRF check is not weakened or reordered.

## Test evidence

- `internal/auth/service_test.go`: order per row of the table (mock expectations prove no issuance),
  missing account revokes the family, store failure is 500 and leaves the family untouched, the
  token names the loaded account even when the credential's `user_id` differs, equivalent spellings
  resolve, boundary outcomes unchanged.
- `internal/app/auth_test.go`: 401/500 codes and statuses at the HTTP boundary.
- `internal/app/auth_integration_test.go` (Mongo, self-skip): delete the account, refresh → 401, no
  `Set-Cookie`, family revoked.
