# Contract: The worker's configuration

**Feature**: `.specs/features/020-email-worker/` | **Requirement IDs**: `WKR-14`, `WKR-16`, `WKR-19`,
`WKR-20` | **Owner**: `shared/env`

## Purpose

The settings the delivery process reads, where they come from, and what happens when they are absent.

## The new settings

| Variable | Default | Meaning | Effect when absent or blank |
|---|---|---|---|
| `EMAIL_WORKER_PREFETCH` | `10` | how many messages the worker holds in flight at once | the default is used |
| `EMAIL_WORKER_MAX_ATTEMPTS` | `5` | the send-attempt cap | the default is used |
| `EMAIL_WORKER_METRICS_ADDR` | `:9101` | the address of the worker's own metrics listener | the default is used |

All three are **optional** and documented; the deployment's required-variable list is unchanged
(`env.RequiredKeys()` stays exactly what it is).

## The reused settings

The worker reads the settings the earlier features already declared, unchanged:

| Variable | Default | Used for |
|---|---|---|
| `RABBITMQ_URL` | — | the broker connection; **required by this process** and validated by it (WKR-19) |
| `EMAIL_PROVIDER` | `log` | which adapter the sending port is built from (`log` \| `resend`) |
| `RESEND_API_KEY` | — | the real provider's key; required when the provider is `resend` |
| `EMAIL_FROM` | — | the sender address; required when the provider is `resend` |
| `EMAIL_SEND_TIMEOUT` | `10s` | the bound on one send |

## Rules

- **The worker validates its own configuration, the request-serving process does not.** A blank or
  unusable broker address, an unknown provider name, a missing provider key or a missing sender
  address stop the worker before it consumes anything, with a typed error (WKR-19).
- **The request-serving process is unaffected.** It keeps its own required list and never reads the
  broker's address; it starts with none of these set (SC-006).
- **The metrics listener is internal.** Its address is a setting, but the worker publishes no host
  port for it (WKR-20).
- **Every default is documented** and every zero value in the worker's configuration struct is filled
  from it, so a partially configured worker still runs.

## Boundary cases

| Case | Result |
|---|---|
| `EMAIL_WORKER_PREFETCH` is zero, negative or not a number | the default is used |
| `EMAIL_WORKER_MAX_ATTEMPTS` is zero, negative or not a number | the default is used |
| `EMAIL_WORKER_METRICS_ADDR` is blank | the default address is used |
| `RABBITMQ_URL` is blank or malformed | the worker fails fast with a typed error and consumes nothing |
| `EMAIL_PROVIDER` names an adapter that does not exist | the worker fails fast with a typed error and consumes nothing |
| the provider is `resend` and the key or the sender address is blank | the worker fails fast with a typed error and consumes nothing |

## Test evidence

- `shared/env/env_test.go`: every new variable absent, blank, valid and malformed; the required list
  is unchanged.
- `backend/cmd/worker/main_test.go`: the fail-fast paths — a blank broker address, an unknown
  provider, a missing key — each stops the process before it consumes.
