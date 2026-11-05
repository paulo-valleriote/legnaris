# Contract: Configuration

**Feature**: `.specs/features/017-otp-challenges/` | **Requirement IDs**: `OTP-18`, `OTP-21` |
**Owner**: `shared/env` (values), `internal/auth` (secret validation), `deploy/` (the required key)

## Purpose

Add the settings the code step needs. Unlike the two preceding features, this one adds **one
required** variable, because the request-serving process itself digests codes and a weak or reused
key would silently weaken every code (spec Clarifications, 2026-10-20; F-09). Every other setting is
optional with a documented default.

## Surface

| Variable | Default constant | Required | Validated by |
|---|---|---|---|
| `EMAIL_CODE_SECRET` | — | **yes** | `auth.NewCodeDigest` at boot: absent, blank, or < 32 bytes → `ErrCodeSecretWeak`, propagated by `app.NewModules` (`OTP-18`) |
| `AUTH_OTP_TTL` | `10m` | no | the policy builder (non-positive/invalid → default + warning, like `lookupDurationEnv`) |
| `AUTH_OTP_CEILING` | `30m` | no | the policy builder |
| `AUTH_OTP_RESEND_COOLDOWN` | `60s` | no | the policy builder |
| `AUTH_OTP_MAX_ATTEMPTS` | `5` | no | the policy builder (non-positive/invalid → default + warning) |
| `AUTH_OTP_DAILY_SEND_CAP` | `80` | no | the policy builder; deliberately below the provider's free-tier 100 (`O4`) |
| `AUTH_OTP_RESEND_CAP_RATIO` | `0.30` | no | the policy builder; the share of `AUTH_OTP_DAILY_SEND_CAP` resends may use, in (0, 1] (invalid → default + warning) (D5) |
| `AUTH_OTP_USER_WINDOW` | `1h` | no | the policy builder |
| `AUTH_OTP_USER_LIMIT` | `10` | no | the policy builder |

## Rules

- `env.RequiredKeys()` becomes exactly `[]string{"DATABASE_URI", "JWT_SECRET", "EMAIL_CODE_SECRET"}`;
  `deploy/.env.example`, `deploy/docker-compose.yml` and `deploy/compose_test.go` follow in the same
  commit (`OTP-18`, `SC-008`).
- The secret is read as a required variable (the existing `lookupRequiredStringEnv`), and its strength
  is enforced by the toolkit that consumes it, so a deploy that sets `EMAIL_CODE_SECRET=` fails at
  boot instead of hashing with an empty key.
- The secret is distinct from `JWT_SECRET`; nothing derives either from the other (F-09, `OTP-21`).
- A policy value of zero or a negative duration is treated exactly like an invalid value: the
  documented default applies with the existing structured warning; the feature never runs with a
  zero-length expiry or a zero attempt cap.
- No setting is user-facing; the deployment documents them and their meaning, and none of them is
  echoed in a response.

## Boundary cases

| Case | Result |
|---|---|
| `EMAIL_CODE_SECRET` unset, blank, or 31 bytes | boot fails with a typed error; no route is served (`SC-008`) |
| `EMAIL_CODE_SECRET` of exactly 32 bytes | boot succeeds |
| `EMAIL_CODE_SECRET` equal to `JWT_SECRET` | accepted by the feature but flagged in review as a policy violation; the deployment documents distinct values (F-09) |
| `AUTH_OTP_TTL=0` or `-5m` or `abc` | default `10m`, warning logged |
| `AUTH_OTP_MAX_ATTEMPTS=0` | default `5`, warning logged |
| `AUTH_OTP_DAILY_SEND_CAP` unset | `80`, which is below the provider's 100 (`O4`) |
| `AUTH_OTP_CEILING` smaller than `AUTH_OTP_TTL` | both are honoured literally: the deadline is clamped to the ceiling, so the effective lifetime is the ceiling (documented, not an error) |
| every optional variable unset, required ones set | the API starts; the policy is the documented default set |
| `deploy/compose_test.go` | the required set matches `RequiredKeys()` including the new secret |

## Test evidence

- `shared/env/env_test.go`: the required set now includes `EMAIL_CODE_SECRET`; each optional variable
  absent/valid/invalid; positive-value enforcement.
- `internal/auth/code_test.go`: empty/blank/short secret rejected, 32 bytes accepted.
- `internal/app/bootstrap_test.go` (or the existing module test): a weak secret fails module
  construction with a typed error.
- `deploy/compose_test.go`: the required key is present in the stack's required set and in the example
  file.
