# Contract: Configuration

**Feature**: `.specs/features/016-email-infrastructure/` | **Requirement IDs**: `EML-14`, `EML-15`,
`EML-16` | **Owner**: `shared/env` (values), `cmd/relay` (relay-scoped validation)

## Purpose

Add every setting this feature needs as optional configuration with a documented default, so the
deployment's required-variable list and the API's boot path do not change, and the process that
actually needs a setting validates it.

## Surface

| Variable | Default constant | Validated by |
|---|---|---|
| `EMAIL_PROVIDER` | `log` | the sender builder (unknown value → error) |
| `RESEND_API_KEY` | empty | `internal/email/resend.New` (required when provider is `resend`) |
| `EMAIL_FROM` | `Legnaris <no-reply@legnaris.local>` | `internal/email/resend.New` |
| `EMAIL_SEND_TIMEOUT` | `10s` | the sender adapters (positive duration) |
| `RABBITMQ_URL` | empty | `cmd/relay` (required, parseable) |
| `EMAIL_RELAY_POLL_INTERVAL` | `1s` | `cmd/relay` |
| `EMAIL_RELAY_LEASE` | `30s` | `cmd/relay` |
| `EMAIL_RELAY_BATCH` | `25` | `cmd/relay` |
| `EMAIL_RELAY_CONFIRM_TIMEOUT` | `5s` | `cmd/relay` |
| `EMAIL_RELAY_MAX_ATTEMPTS` | `5` | `cmd/relay` |
| `EMAIL_OUTBOX_RETENTION` | `168h` | `shared/db` TTL index |
| `EMAIL_RELAY_METRICS_ADDR` | `:9100` | `cmd/relay` |

## Rules

- `env.RequiredKeys()` stays exactly `[]string{"DATABASE_URI", "JWT_SECRET"}` (EML-14, SC-006).
- An unset or blank optional variable selects the documented default and logs the same structured
  warning the existing lookups log; an invalid value for a typed variable (duration/int) falls back
  to the default with a warning, exactly like the existing `lookupDurationEnv`/`lookupIntEnv`.
- The relay treats an empty or unparseable `RABBITMQ_URL` as a startup failure
  (`ErrBrokerUnavailable`), never as "run with no broker" (EML-15).
- The request-serving process never reads `RABBITMQ_URL` and never depends on the broker (EML-16).
- `deploy/docker-compose.yml` passes each variable through as `${VAR:-default}`; the broker service
  publishes no host port; `deploy/compose_test.go` asserts the required set is unchanged and the
  broker has no `ports:`.

## Boundary cases

| Case | Result |
|---|---|
| every new variable unset | the API starts; the relay fails fast on the empty broker URL |
| `EMAIL_PROVIDER=resend`, `RESEND_API_KEY` empty | building the sender fails with a clear error |
| `EMAIL_PROVIDER=bogus` | building the sender fails with a clear error |
| `EMAIL_SEND_TIMEOUT=abc` | default `10s`, warning logged |
| `EMAIL_RELAY_BATCH=0` | default `25`, warning logged (non-positive → default) |
| `RABBITMQ_URL=not-a-uri` | `cmd/relay` fails fast before polling |
| required set | unchanged before and after this feature |

## Test evidence

- `shared/env/env_test.go`: each variable absent/blank/valid/invalid; `RequiredKeys()` unchanged.
- `internal/email/resend/sender_test.go`: empty key/from → build error; unknown provider → error.
- `cmd/relay/main_test.go`: blank/unparseable URL → fail fast.
- `deploy/compose_test.go`: the new keys are optional interpolations; the broker has no published
  port; the app's required set is unchanged.