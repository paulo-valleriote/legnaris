# Contract: Observability and disclosure

**Feature**: `.specs/features/017-otp-challenges/` | **Requirement IDs**: `OTP-15`, `OTP-16`,
`OTP-21` | **Owner**: `internal/auth` (`challenge_observer.go`), `internal/platform/metrics`

## Purpose

Make the code pipeline operable — issuance, verification outcomes, resend behavior and cap refusals —
without ever emitting a code, a challenge identity or an unmasked address, and without widening the
existing auth observer for unrelated implementers.

## Interface

```go
// ChallengeObserver receives code-step signals (OTP-16). Separate from Observer so
// existing implementers and their generated mocks do not move (R10).
type ChallengeObserver interface {
    ChallengeIssued(purpose Purpose)
    ChallengeVerified(result string) // "verified" | "invalid" | "expired" | "consumed" | "exhausted" | "unknown"
    ResendRequested(result string)   // "sent" | "suppressed" | "capped"
    SendCapBlocked()
}

func NewNopChallengeObserver() ChallengeObserver
```

`internal/platform/metrics` implements it; metrics never receives a code or an unmasked address.

## Series

| Metric | Labels | Meaning |
|---|---|---|
| `legnaris_auth_otp_issued_total` | `purpose` (`verify_email`, `login_otp`) | challenges issued (each of which recorded one send intention) |
| `legnaris_auth_otp_verify_total` | `result` | verification outcomes, including the internal classification of the uniform failure |
| `legnaris_auth_otp_resend_total` | `result` | resend outcomes: a new send recorded, the cool-down suppressed it, or the cap refused it |
| `legnaris_auth_otp_cap_blocked_total` | — | requests refused because the daily send cap was reached |

## Rules

- The code MUST NOT appear in a response body, a header, a log line, an error message or a message
  subject (`OTP-15`). It exists in exactly two places: the send intention's payload and the HTTP
  request that submitted it.
- Logs identify a challenge by its identity and purpose only, and the recipient address is masked as
  its first local-part character plus the domain (`a***@example.com`), matching the preceding
  feature's rule.
- `result` labels are a closed set; an unclassified failure is `invalid`, never a free-text label
  derived from an error.
- The uniform 401 body is not distinguished by any response field; the classification reaches only
  metrics and logs (`OTP-08`, `OTP-16`).
- Warn-level logs are reserved for genuinely notable events: the cap being reached and a store
  failure during a challenge flow; per-attempt logging stays off (the volume is attacker-controlled).
- No series carries a challenge identity: the identity is a capability and must not be indexed in a
  metrics store.
- All identifiers, comments and log messages are English (`OTP-21`).

## Boundary cases

| Case | Result |
|---|---|
| a code appears anywhere in a log line or an error message | the assertion scanning the suite's output fails the build (`OTP-15`, `SC-003`) |
| a full address appears in a log line | the masking assertion fails the build (`OTP-15`) |
| an unknown challenge identity is verified | `verify_total{result="unknown"}` increments; the response stays the uniform 401 (`OTP-08`) |
| a challenge is verified past its expiry | `verify_total{result="expired"}` increments (`OTP-06`, `OTP-16`) |
| the fifth wrong attempt exhausts a challenge | `verify_total{result="exhausted"}` increments |
| a resend inside the cooldown | `resend_total{result="suppressed"}` increments and no intention is recorded (`OTP-10`) |
| the cap refuses an issuance or a resend | `cap_blocked_total` and `resend_total{result="capped"}` increment; the answer is 503 (`OTP-13`) |
| a store failure during verification | the error is logged with the challenge identity and purpose, no code, and the response is 500 (`OTP-17`) |

## Test evidence

- `internal/platform/metrics/metrics_test.go`: each counter increments with the documented label set;
  the observer methods satisfy `auth.ChallengeObserver`.
- `internal/auth/challenge_service_test.go`: the classification reaching the observer matches the
  conditional-write outcome for all six verification results and all three resend results.
- `internal/app/auth_challenge_test.go`: a suite-level scan asserts no 6-digit code and no unmasked
  address appear in the responses, headers or captured logs of every failure path.
