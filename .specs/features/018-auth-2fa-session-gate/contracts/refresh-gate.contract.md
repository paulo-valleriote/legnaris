# Contract: The renewal verification gate

**Feature**: `.specs/features/018-auth-2fa-session-gate/` | Requirements: `TFA-06`, `TFA-07`, `TFA-11`, `TFA-12`

One predicate added to `ServiceImpl.Refresh`, at one position in the existing sequence.

## Position

```text
1. RefreshToken == ""                       ─▶ 401 (unchanged)
2. FetchByHash(credential)                  ─▶ unknown: 401 (unchanged)
3. csrf.Verify(token, family)               ─▶ invalid: 403 (unchanged)
4. revoked                                  ─▶ reuse: revoke family, 401 (unchanged)
5. expired                                  ─▶ 401 (unchanged)
6. FetchByID(account)                       ─▶ not found: revoke family, 401 (PRQ-05, unchanged)
7. gate enabled AND account.EmailVerifiedAt == nil
                                            ─▶ NEW: revoke family, 401, no credential
8. Issue access token                        ─▶ (unchanged)
9. issueRefreshToken + RevokeByID rotation   ─▶ (unchanged)
```

Steps 1–6 and 8–9 are untouched. The predicate is inserted **after** the account load and **before**
the first issuance, so a refusal leaves no successor credential behind.

## Outcome

| | |
|---|---|
| Code | `UNAUTHORIZED` (`MsgInvalidRefreshToken`, the existing uniform credential outcome) |
| Status | 401 |
| Cookies | none set, none cleared (the caller's cookie is simply no longer valid) |
| Side effect | `RevokeFamily(presented credential's family)` |
| Credentials issued | none — no access token, no rotated refresh token |

The refusal is deliberately indistinguishable from the missing-account outcome (`PRQ-05`), so a
caller cannot tell "your account is unverified" from "your account is gone"; the user's recovery route
is the same in both cases — sign in and complete the code step.

## Invariants

1. **Containment** (`SC-003`): a session minted before the gate was enabled cannot rotate even once
   after it; the second attempt with the same credential fails too, because the family is revoked.
2. **Ordering** (`TFA-06`): the predicate never runs before the credential rules, so revoked/expired/
   reused credentials keep their existing outcomes, and a missing account keeps deciding first.
3. **No session-format change** (`TFA-11`): the access token, refresh token, CSRF token and cookie
   attributes are unchanged on every success path.
4. **Gate-scoped** (`TFA-12`): with the gate disabled the predicate is not evaluated, so renewal for an
   unverified account rotates exactly as today.
5. **Idempotence of the refusal**: repeating the refused call revokes nothing new and issues nothing.

## Rejected alternatives

- *Refusing without revoking* (spec Clarifications, 2026-10-20): the credential would keep failing
  while staying live for thirty days, and the session would never be actively ended.
- *Checking the state inside `signIn`* (`R3`): the legacy `Register` path must sign in an account with
  no timestamp, so the check cannot live in the shared tail.
- *Returning `FORBIDDEN`/a new code to signal "unverified"*: it would disclose account state and add a
  wire outcome the spec does not require (`TFA-10`).

## Tests

- Gate on × {absent timestamp, present timestamp} → {refusal with family revoked and 0 issued,
  rotation unchanged}.
- Gate off × absent timestamp → rotation unchanged (`SC-004`).
- Second attempt after the refusal → fails again, nothing minted.
- Missing account and revoked/expired credential with the gate on → unchanged outcomes (no
  interference).
- Store failure while revoking the family on the refusal → `INTERNAL_ERROR`, still nothing issued.
- Race: two concurrent renewals of an unverified account → 0 credentials, family revoked once.
