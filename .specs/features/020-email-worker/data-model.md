# Phase 1 Data Model: The Email Worker

**Feature**: `.specs/features/020-email-worker/` | **Date**: 2026-10-21

No persisted collection. The worker reads one wire shape (the message the publisher already delivers)
and writes three declarative shapes (the destination vocabulary, the process settings and the
counters). Nothing here touches the existing `users`, `refresh_tokens`, `sheets`, `template` or
`email_outbox` collections — the worker stores nothing at all (WKR-01, the clarification of
2026-10-21).

---

## 1. The queued message — the wire shape the worker consumes

Owned by `internal/email` (delivered by the infrastructure feature). The worker decodes it, validates
it, renders it and republishes it unchanged apart from the attempt count.

| Field | JSON | Rules |
|---|---|---|
| `ID` | `id` | canonical lowercase UUID v7; screened with `idgen.Validate` before anything else; the provider idempotency key (WKR-04, WKR-06) |
| `Type` | `type` | constant `"email.otp"`; any other type is permanently unusable |
| `To` | `to` | the recipient; masked in every log line (WKR-22) |
| `Template` | `template` | the purpose that selected the template (`verify_email` \| `login_otp`); the worker renders from the *purpose*, not from this field |
| `Data.Code` | `data.code` | the short-lived credential; never logged, never in a metric, never in a subject (WKR-06) |
| `Data.ExpiresInMinutes` | `data.expires_in_minutes` | the advertised lifetime in minutes; zero or negative means the code is already expired (WKR-07) |
| `Data.Purpose` | `data.purpose` | selects the embedded template; an unknown purpose is permanently unusable (WKR-05) |
| `CreatedAt` | `created_at` | RFC 3339 UTC; the start of the deadline (WKR-07) |
| `Attempt` | `attempt` | the number of **send attempts already made**; the publisher writes `0`; the worker increments it when it republishes (WKR-10, WKR-15) |

### The deadline

```text
deadline = CreatedAt + Data.ExpiresInMinutes
expired  ⇔  now ≥ deadline   (now comes from the injected clock)
```

- A lifetime of zero or negative means the deadline has already passed: the message is expired.
- The worker never consults the challenge store; the message states its own deadline (the
  clarification of 2026-10-21, `research.md` R4).
- The deadline is never trusted for anything else: it does not authorize the send, it only decides
  whether sending is still worthwhile.

### Validation rules (all applied before a send is attempted)

| Rule | Failure | Effect |
|---|---|---|
| the body decodes into the message contract | the body is not a message | dead-letter, acknowledge (WKR-04) |
| the identity is a canonical UUID v7 | the identity is malformed | dead-letter, acknowledge (WKR-04) |
| the code is present and not blank | there is nothing to deliver | dead-letter, acknowledge (WKR-04) |
| the type is the OTP type | the message is not for this pipeline | dead-letter, acknowledge (WKR-04) |
| the purpose selects a template | no template exists for it | dead-letter, acknowledge (WKR-05) |
| the deadline has not passed | the code is stale | acknowledge without sending, count as expired (WKR-03) |

### The attempt count

The count travels on the message, so a redelivery keeps the budget:

| Arriving `Attempt` | Meaning | Action on a retryable failure |
|---|---|---|
| `0` | no send has been attempted | republish to the **30-second** tier with `Attempt = 1` |
| `1` | one send failed | republish to the **2-minute** tier with `Attempt = 2` |
| `2` | two sends failed | republish to the **5-minute** tier with `Attempt = 3` |
| `3`, `4` | three or four sends failed | republish to the **5-minute** tier with `Attempt + 1` |
| `5` or more | the cap is reached | dead-letter without another send (WKR-12) |

The outbox's own publish-attempt counter is a different counter and never appears on the wire.

---

## 2. Destination — the vocabulary of where a message goes

Owned by `internal/email`. The broker adapter maps each destination to the exchange and routing key it
already declares (`research.md` R3).

| Name | Meaning | Broker mapping (owned by `internal/email/amqp`) |
|---|---|---|
| `DestinationRetry30s` | the first retry waits about half a minute | the retry exchange, routing key `30s` |
| `DestinationRetry2m` | the second retry waits two minutes | the retry exchange, routing key `2m` |
| `DestinationRetry5m` | every later retry waits five minutes | the retry exchange, routing key `5m` |
| `DestinationDeadLetter` | the message can never succeed; an operator will look at it | the main exchange, routing key `email.dead` |

The work queue itself (`email.otp`) is never a destination: the worker only ever publishes *away* from
it.

---

## 3. The delivery — one message, one outcome

| Outcome | Trigger | What happens to the message | Counter |
|---|---|---|---|
| **sent** | the provider accepted the send | acknowledged | `email_send_total{result="sent"}` |
| **retried** | the provider failed retryably | republished to a delay tier with the attempt count incremented; the original is acknowledged only after the republish is confirmed | `email_send_total{result="retried"}` |
| **dead-lettered** | the provider failed permanently, or the attempt cap was reached | published to the dead-letter route and acknowledged | `email_send_total{result="dead_lettered"}` and `email_dead_total` |
| **expired** | the deadline had passed | acknowledged without a send | `email_send_total{result="expired"}` and `email_expired_total` |

Two outcomes never end in an acknowledgement of the original: an **unconfirmed republish** and an
**unconfirmed dead-letter** leave the message unacknowledged, and the broker redelivers it (WKR-13).

---

## 4. The process settings

Owned by `shared/env`. Every one is optional with a documented default; the required-variable list is
unchanged (WKR-19).

| Setting | Default | Meaning |
|---|---|---|
| `EMAIL_WORKER_PREFETCH` | `10` | how many messages the worker holds in flight at once — the broker's prefetch and the size of the handler pool (the plan's §5.6) |
| `EMAIL_WORKER_MAX_ATTEMPTS` | `5` | the send-attempt cap; a message at or above it is dead-lettered without another send (WKR-12, WKR-14) |
| `EMAIL_WORKER_METRICS_ADDR` | `:9101` | the address of the worker's own metrics listener; it publishes no host port (WKR-20) |

Reused unchanged from the delivered configuration surface: `RABBITMQ_URL`, `EMAIL_PROVIDER`,
`RESEND_API_KEY`, `EMAIL_FROM` and `EMAIL_SEND_TIMEOUT`.

---

## 5. The counters

Owned by `internal/platform/metrics`, served by the worker's own listener.

| Series | Type | Labels | Meaning |
|---|---|---|---|
| `legnaris_email_send_total` | counter | `result` (`sent` \| `retried` \| `dead_lettered` \| `expired`) | one increment per delivery outcome (WKR-21) |
| `legnaris_email_send_duration_seconds` | histogram | — | how long the provider took (WKR-21) |
| `legnaris_email_dead_total` | counter | — | messages that reached the dead-letter route; the series the earlier feature already declared, used here for the first time |
| `legnaris_email_expired_total` | counter | — | messages dropped because their code had expired (WKR-21) |

No series carries a code or a full address; the recipient is never a label (WKR-21).

---

## 6. The delivery ports

Owned by `internal/email/worker` (`research.md` R2).

| Port | Direction | Contract |
|---|---|---|
| `DeliverySource` | broker → worker | hands the worker one delivery at a time: the raw body, a delivery handle that can acknowledge or reject it, and the end of the stream |
| `Republisher` | worker → broker | publishes one message to one destination and reports success only once the broker confirmed it |

The worker owns both ports and the fakes that implement them in the tests; the RabbitMQ
implementation of the first lives in `internal/email/amqp`.
