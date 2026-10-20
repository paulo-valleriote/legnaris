# Contract: Configuration

**Feature**: `.specs/features/018-auth-2fa-session-gate/` | Requirements: `TFA-07`, `TFA-10`, `TFA-16`

One new optional variable and one new reader. Nothing joins the required set.

## Variable

| | |
|---|---|
| Name | `AUTH_REQUIRE_EMAIL_VERIFICATION` |
| Type | boolean |
| Default | `false` |
| Required | no |
| Governs | whether `Register`/`Login` answer a code step instead of a session, and whether `Refresh` consults the verification state |

## Reader

```go
// shared/env/env.go — same posture as lookupDurationEnv / lookupIntEnv
func (e *Env) lookupBoolEnv(key string, defaultValue bool) bool
```

- Unset or empty → default, no warning beyond the shared `lookupEnv` warning.
- `strconv.ParseBool` accepts `1/0/t/f/true/false/TRUE/FALSE/True/False`; anything else → a structured
  warning naming the key, the raw value and the default, then the default (`R2`).
- Never a boot error: an invalid optional value must not be able to take the service down, and the
  fallback direction (disabled) is the safe one.

`Variables.RequireEmailVerification bool` is populated in `initVariables` and copied into
`auth.Config.RequireEmailVerification` at the composition root.

## Deployment wiring

| File | Change |
|---|---|
| `deploy/.env.example` | the variable with its documented default and a one-line description |
| `deploy/docker-compose.yml` | passed through to the `api` service |
| `deploy/README.md` | what the gate governs, that it is optional, and that stage 7 is the flip |
| `Makefile` | exported to the app process (`make run`) |
| `deploy/compose_test.go` | the required set is **unchanged**; the gate is present and not required |

## Invariants

1. **Required set unchanged** (`TFA-16`, `SC-004`): `env.RequiredKeys()` keeps exactly
   `{DATABASE_URI, JWT_SECRET, EMAIL_CODE_SECRET}`, and `deploy/compose_test.go` keeps asserting it.
2. **Documented** (`TFA-16`): the variable appears in `.env.example` and `deploy/README.md`; an
   undocumented setting is not shipped.
3. **No secret** (`TFA-16`): the gate carries no credential and is safe to log by name; no log line
   this feature adds carries a code or an unmasked address.
4. **No cross-talk** (`TFA-11`): the gate does not alter the challenge policy values
   (`AUTH_OTP_*`), the email settings (`EMAIL_*`) or the rate limits (`AUTH_RATE_LIMIT_*`).

## Tests

- Default: unset → `false`; empty string → `false`.
- Parsing: each accepted spelling yields the expected value.
- Invalid value (`"maybe"`) → the default plus one warning (`assert` on the returned value; the
  warning is asserted through the logger fake used by the existing `env_test.go` conventions).
- `RequiredKeys()` does not contain the new name.
- Wiring: `NewModules`/`bootstrap` carries the parsed value into `auth.Config`.
