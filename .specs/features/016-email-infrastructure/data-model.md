# Phase 1 Data Model: Email Delivery Infrastructure

**Feature**: `.specs/features/016-email-infrastructure/` | **Date**: 2026-10-20

Three persisted or wire shapes (the intention, the message, the template set) plus two declarative
shapes (the broker topology and the configuration). Nothing here touches the existing `users`,
`refresh_tokens`, `sheets` or `template` collections.

---

## 1. Send intention — `email_outbox` collection

The durable record that an email must be sent. Owned by `internal/email/outbox`.

| Field | Type | BSON | Rules |
|---|---|---|---|
| `ID` | string | `_id` | canonical lowercase UUID v7 from `shared/idgen.New`; the provider idempotency key; validated with `idgen.Validate` on read (EML-18) |
| `To` | string | `to` | the canonical recipient address supplied by the caller; never empty (created by the API-side caller) |
| `Template` | string | `template` | the purpose string that selects the template (`verify_email` \| `login_otp`) (EML-11) |
| `Payload` | object | `payload` | `{code, expires_in_minutes, purpose}`; the code is the short-lived credential (EML-13) |
| `Status` | string | `status` | `pending` \| `published` \| `failed` |
| `Attempts` | int | `attempts` | **failed publications** count; starts 0; incremented only by `Reschedule` (EML-04) |
| `NextAttemptAt` | time | `next_attempt_at` | eligible-again time; set at creation and by `Reschedule` |
| `LockedUntil` | time (nullable) | `locked_until` | claim lease expiry; `null` when unclaimed |
| `LockedBy` | string | `locked_by` | claim owner (relay instance id); `""` when unclaimed |
| `CreatedAt` | time | `created_at` | set at creation |
| `PublishedAt` | time (nullable) | `published_at` | set only by `MarkPublished`; drives the retention TTL |
| `LastError` | string | `last_error` | the last publication failure, for operators; never contains the code |

### Payload

| Field | Type | Rules |
|---|---|---|
| `Code` | string | 6-digit numeric code with leading zeros; generated and hashed by the API-side challenge feature, carried here only to be rendered |
| `ExpiresInMinutes` | int | positive; rendered into the body and the message `data` |
| `Purpose` | string | `verify_email` \| `login_otp` |

### State transitions

```text
                 Create
                   │
                   ▼
              ┌─────────┐   Claim (lease)      ┌─────────┐
              │ pending │ ───────────────────▶ │ pending │  (locked, same status)
              └─────────┘                      └─────────┘
                   ▲                               │
   Reschedule      │                    ┌──────────┴──────────┐
   (attempts+1)    │                    │ publish confirmed    │ publish failed
                   │                    ▼                      ▼
                   │              ┌───────────┐          ┌─────────┐
                   └──────────────│ published │          │ pending │
                                  └───────────┘          └─────────┘
                                                              │ attempts == cap
                                                              ▼
                                                         ┌────────┐
                                                         │ failed │
                                                         └────────┘
```

Invariants:

- `published` and `failed` are terminal; neither is ever claimed again.
- A claim never changes `status` or `attempts`; it only moves the lease.
- A crash after publish and before `MarkPublished` leaves the row `pending` with an expired lease;
  the next pass republishes it. The duplicate is harmless because `ID` is the provider idempotency
  key (EML-03, SC-001).
- `published_at` is set exactly once, by `MarkPublished`.

### Indexes (`shared/db/indexes.go`, idempotent)

| Name | Keys | Options | Purpose |
|---|---|---|---|
| `email_outbox_status_next_attempt` | `{status: 1, next_attempt_at: 1}` | — | the claim query (EML-17) |
| `email_outbox_published_at_ttl` | `{published_at: 1}` | `expireAfterSeconds = EMAIL_OUTBOX_RETENTION` (168h) | retire published rows; `null` (`pending`/`failed`) is never expired by the TTL monitor (EML-17) |

TTL is a janitor, not an expiry gate: nothing in the read path trusts the TTL monitor (the program
plan's F-13), and a row is only ever "gone" after the monitor runs.

---

## 2. Published message — the broker contract

Owned by `internal/email`. Encoded by the relay; the future worker will decode it.

| Field | JSON | Rules |
|---|---|---|
| `ID` | `id` | = the intention's `ID`; also the AMQP `MessageId`; the provider idempotency key |
| `Type` | `type` | constant `"email.otp"` for this feature |
| `To` | `to` | the recipient |
| `Template` | `template` | = the purpose that selected the template |
| `Data` | `data` | `{code, expires_in_minutes, purpose}` |
| `CreatedAt` | `created_at` | RFC 3339 UTC |
| `Attempt` | `attempt` | send attempt; the relay publishes `0`; the worker bumps it on a send retry (plan §5.5) |

AMQP properties: `ContentType: application/json`, `DeliveryMode: Persistent`, `MessageId: id`.

A golden file (`internal/email/testdata/message.golden.json`) pins this exact JSON (EML-22).

---

## 3. Template set

Owned by `internal/email`, embedded from `internal/email/templates/`.

| Purpose | Files | Subject carries the code? |
|---|---|---|
| `verify_email` | `verify_email.subject.txt`, `verify_email.body.txt`, `verify_email.body.html` | no |
| `login_otp` | `login_otp.subject.txt`, `login_otp.body.txt`, `login_otp.body.html` | no |

Render inputs: `{code, expires_in_minutes}`. Both bodies carry the code and its lifetime; the
subject never does (EML-11, EML-13). An unknown purpose renders nothing and returns
`ErrUnknownTemplate` (permanent).

---

## 4. Broker topology (declarative)

Owned by `internal/email/amqp`. All durable; declared idempotently before the first publish.

| Entity | Kind | Arguments | Purpose |
|---|---|---|---|
| `legnaris.email` | direct exchange | — | the main exchange; fresh publishes use key `email.otp` |
| `email.otp` | quorum queue | `x-max-length`, `x-overflow: reject-publish`, `x-dead-letter-exchange: legnaris.email.retry`, `x-dead-letter-routing-key: 30s` | the work queue; full → nack so the relay backs off and the outbox keeps the intent |
| `legnaris.email.retry` | direct exchange | — | the retry exchange (used by the worker) |
| `email.retry.30s` | quorum queue | `x-message-ttl: 30000`, DLX back to `legnaris.email` key `email.otp` | first delay tier |
| `email.retry.2m` | quorum queue | `x-message-ttl: 120000`, DLX back | second delay tier |
| `email.retry.5m` | quorum queue | `x-message-ttl: 300000`, DLX back | third delay tier |
| `email.dead` | quorum queue | — (bound to `legnaris.email` key `email.dead`) | dead letters for inspection/replay |

The relay declares the topology; the delay-tier DLX cycle is driven by the worker (deferred), so the
exchange and queues exist and are correct before any consumer is written.

---

## 5. Configuration (all optional, documented defaults)

Owned by `shared/env`. `RequiredKeys()` stays `{DATABASE_URI, JWT_SECRET}`.

| Variable | Default | Consumed by | Validation |
|---|---|---|---|
| `EMAIL_PROVIDER` | `log` | sender selection | unknown value → send build error |
| `RESEND_API_KEY` | empty | `internal/email/resend` | required when provider is `resend` |
| `EMAIL_FROM` | `Legnaris <no-reply@legnaris.local>` | `internal/email/resend` | required when provider is `resend` |
| `EMAIL_SEND_TIMEOUT` | `10s` | sender adapters | positive duration |
| `RABBITMQ_URL` | empty | `cmd/relay` | required, parseable AMQP URI (else `ErrBrokerUnavailable`) |
| `EMAIL_RELAY_POLL_INTERVAL` | `1s` | relay loop | positive |
| `EMAIL_RELAY_LEASE` | `30s` | claim | positive |
| `EMAIL_RELAY_BATCH` | `25` | claim | positive |
| `EMAIL_RELAY_CONFIRM_TIMEOUT` | `5s` | publish wait | positive |
| `EMAIL_RELAY_MAX_ATTEMPTS` | `5` | terminal cap | positive |
| `EMAIL_OUTBOX_RETENTION` | `168h` | outbox TTL index | positive |
| `EMAIL_RELAY_METRICS_ADDR` | `:9100` | relay metrics listener | a listen address |

Defaults live in `shared/env` as constants next to the existing ones; the relay re-validates the
settings it cannot run without so a misconfiguration fails at startup, not on the first publish
(EML-14, EML-15).

---

## 6. Relationships

```text
API-side caller (deferred)          relay (cmd/relay)
        │ Create                        │ Claim / MarkPublished / Reschedule
        ▼                               ▼
   ┌───────────────────────────────────────────┐
   │              email_outbox                 │  (Intent, Mongo)
   └───────────────────────────────────────────┘
                    │ encode
                    ▼
              Message (email.Message)
                    │ Publish (confirmed, mandatory)
                    ▼
              RabbitMQ topology
                    │ (worker deferred)
                    ▼
              Sender port ──▶ Resend adapter / log adapter
                    │
                    ▼
              Rendered (from embedded templates)
```

The intention and the message share one identity; the template set and the sender are joined only by
the (deferred) worker. This feature ships the left half of the diagram, its store, and the producer
of the message.