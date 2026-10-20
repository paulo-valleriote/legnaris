# Contract: Challenge endpoints and the service surface

**Feature**: `.specs/features/017-otp-challenges/` | **Requirement IDs**: `OTP-07`, `OTP-08`,
`OTP-09`, `OTP-10`, `OTP-11`, `OTP-13`, `OTP-14`, `OTP-17`, `OTP-19` | **Owner**: `internal/auth`
(service), `internal/app` (controller, routes)

## Purpose

Fix the two unified endpoints of decision O3, their payloads and their statuses, and pin the uniform
verification failure so the client mapping (stage 5) has one contract to build against.

## Service surface

```go
type IssueChallengeCommand struct {
    UserID  string   // resolved by the caller (credentials already verified); never an address alone
    Email   string   // the account's canonical address to send to
    Purpose Purpose
}

type ChallengeIssued struct {
    ChallengeID string // canonical UUID v7
    ExpiresIn   int    // seconds the code is claimed to be valid for (the configured lifetime)
    ResendAfter int    // seconds until a resend records a new send (the configured cooldown)
}

type VerifyChallengeCommand struct{ ChallengeID, Code string }
type ResendChallengeCommand struct{ ChallengeID string }
type ResendResult struct{ ResendAfter int }
```

`auth.Service` gains `IssueChallenge(ctx, IssueChallengeCommand) (*ChallengeIssued, error)`,
`VerifyChallenge(ctx, VerifyChallengeCommand) (*Session, error)` and
`ResendChallenge(ctx, ResendChallengeCommand) (*ResendResult, error)`. `IssueChallenge` is the
interim trigger of this stage: stage 4 calls it from registration and sign-in, and today only the
integration suite does.

## Endpoints

### `POST /auth/challenge/verify`

Request: `{"challenge_id": "<uuid>", "code": "042917"}` — decoded strictly (unknown fields are
rejected).

| Status | Body | When |
|---|---|---|
| 200 | `{"access_token":"…","token_type":"Bearer","expires_in":900}` plus the refresh and CSRF cookies | the code was correct and the challenge was consumed; the session is exactly what sign-in issues (`OTP-07`) |
| 401 | `{"error":{"code":"OTP_INVALID","message":"invalid or expired code"}}` | **every** failure: unknown, malformed, expired, consumed, wrong or exhausted (`OTP-08`) |
| 400 | `{"error":{"code":"INVALID_JSON"\|"UNKNOWN_FIELD"\|"INVALID_REQUEST",…}}` | malformed body, unknown field, missing/blank field (`R5`) |
| 429 | existing `RATE_LIMITED` + `Retry-After` | the client address or the account exceeded its bound (`OTP-14`) |
| 503 | `{"error":{"code":"EMAIL_UNAVAILABLE",…}}` + `Retry-After` | reserved here: verification never sends, so this is not produced on this route |
| 500 | existing `INTERNAL_ERROR` | store failure; never reported as a code failure (`OTP-17`) |

### `POST /auth/challenge/resend`

Request: `{"challenge_id": "<uuid>"}`.

| Status | Body | When |
|---|---|---|
| 202 | `{"resend_after":60}` | always, unless the daily cap is reached: a send was recorded, or the cool-down suppressed it, or the identity is unknown/malformed/consumed/expired — the body is identical in all of them (`OTP-10`, `OTP-11`) |
| 503 | `{"error":{"code":"EMAIL_UNAVAILABLE","message":"email delivery is temporarily unavailable; retry later"}}` + `Retry-After` | the daily send cap is reached; no intention is recorded (`OTP-13`) |
| 400 / 429 / 500 | as above | as above |

## Rules

- The route surface is exactly these two paths under `/auth`; no route accepts or echoes the
  challenge purpose, and the client never sends it (`OTP-09`, `OTP-19`).
- Verification establishes a session through the existing sign-in tail (`signIn` + the controller's
  cookie helper), and records the account's verification timestamp first when it was absent
  (`OTP-07`). Both purposes behave identically.
- The uniform failure is produced by one service path and carries one code, one message and one
  status; the internal classification (expired/consumed/exhausted/unknown) is exported only to
  metrics and logs (`OTP-08`, `OTP-15`, `OTP-16`).
- `IssueChallenge` answers `ChallengeIssued` (the stage-4 response shape: `challenge_id`,
  `expires_in`, `resend_after`) and answers `EMAIL_UNAVAILABLE` when the cap is reached.
- Both routes sit behind the existing per-address limiter; neither takes the origin guard, and
  neither changes any existing route (`OTP-19`, `R13`).
- The response never contains the code, and no error message or log line contains it either
  (`OTP-15`).

## Invariants

- One verification of a challenge can answer 200; every later one answers 401.
- The 401 body for all six failure kinds is identical byte for byte.
- A 202 resend never rotates the code and never changes the challenge identity.
- Reaching the cap never records an intention and never reveals account state.
- No status in this contract is produced by a route other than the one documented here.

## Boundary cases

| Case | Result |
|---|---|
| `challenge_id` not a canonical UUID | 401 uniform (verify) / 202 uniform (resend) (`OTP-08`, `R5`) |
| `code` of five digits, or seven | 401 uniform, attempt counted (`OTP-05`) |
| `code` missing from the body | 400 `INVALID_REQUEST` (a body-shape error, not a code error) |
| correct code one nanosecond before expiry | 200 (`OTP-06`) |
| correct code at the expiry instant | 401 uniform (`OTP-06`) |
| two concurrent 200-bound verifications | one 200, one 401 (`OTP-04`) |
| resend inside the cool-down | 202, no intention recorded (`OTP-10`) |
| resend after the challenge was consumed | 202, nothing resurrected (`OTP-11`) |
| resend while the cap is reached | 503, no intention recorded, no deadline moved (`OTP-13`) |
| body with an unknown field | 400 `UNKNOWN_FIELD` |
| account store failure while marking verified | 500; the challenge stays consumed (`OTP-17`) |

## Test evidence

- `internal/app/auth_challenge_test.go`: the status table above through the router with fakes, the
  byte-identical 401 bodies across all failure kinds, the identical 202 bodies, no code in any
  response body or header, and `Retry-After` present on 429/503.
- `internal/app/auth_integration_test.go` (Mongo, self-skip): issue → verify → session cookies, and
  issue → resend → verify, end to end; a consumed challenge failing uniformly afterwards.
- `internal/auth/challenge_service_test.go`: `IssueChallenge`/`VerifyChallenge`/`ResendChallenge`
  against fakes, including the cap and the store-failure classification.
- `internal/app/auth_test.go`: the route inventory still matches (the two new paths added).
