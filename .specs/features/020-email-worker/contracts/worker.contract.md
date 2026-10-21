# Contract: The delivery worker

**Feature**: `.specs/features/020-email-worker/` | **Requirement IDs**: `WKR-01` … `WKR-27` | **Owner**:
`internal/email/worker` (the policy), `internal/email/amqp` (the delivery source), `cmd/worker` (the
process)

## Purpose

The behaviour of the process that turns a queued code into a delivered email: what it does with each
delivery, when it acknowledges, when it retries, when it gives up, and what it exposes. The broker
mechanics are a separate contract (`broker-consumer.contract.md`).

## The delivery loop

```text
delivery arrives
      │
      ├─ body does not decode ───────────────▶ dead-letter, acknowledge
      ├─ identity is not canonical ───────────▶ dead-letter, acknowledge
      ├─ code is missing ─────────────────────▶ dead-letter, acknowledge
      ├─ type is not the OTP type ────────────▶ dead-letter, acknowledge
      ├─ purpose selects no template ─────────▶ dead-letter, acknowledge
      ├─ deadline has passed ─────────────────▶ acknowledge, count as expired
      │
      └─ render, then send
             │
             ├─ the provider accepted it ──────▶ acknowledge, count as sent
             ├─ the provider failed ───────────▶ is the failure retryable?
             │                                    │
             │                                    ├─ no ──────────▶ dead-letter, acknowledge
             │                                    └─ yes ─────────▶ attempt at the cap?
             │                                                       ├─ yes ─▶ dead-letter, acknowledge
             │                                                       └─ no  ─▶ republish to the tier,
             │                                                             acknowledge only after the confirm
             └─ the republish was not confirmed ─▶ leave unacknowledged (the broker redelivers)
```

## Rules

- **Acknowledgement is the last step.** A delivery is acknowledged after the send succeeded, after it
  was deliberately dead-lettered, or after it was deliberately found expired — never before (WKR-02).
- **A message is never dropped.** The only two paths that leave a delivery unacknowledged are an
  unconfirmed republish and an unconfirmed dead-letter publish; in both the broker still owns the
  message and redelivers it (WKR-13).
- **The deadline is the message's own.** `CreatedAt + ExpiresInMinutes` against the injected clock; the
  challenge store is never consulted (WKR-07, the clarification of 2026-10-21).
- **The attempt count is the message's own.** It counts the send attempts already made; the broker's
  dead-letter history is never consulted (WKR-10).
- **A republished message changes nothing but the count.** Identity, recipient, template, data and
  creation time are carried over verbatim (WKR-15).
- **The code never escapes.** It is rendered into the bodies, handed to the sending port, and never
  written to a log, an error, a metric or a subject; the recipient is masked in every log line
  (WKR-06, WKR-21, WKR-22).
- **The identity is the idempotency key.** The same identity is handed to the provider on every
  attempt, so a redelivered message produces a duplicate the provider de-duplicates (WKR-08).

## Outcomes and their counters

| Outcome | Counter | Series |
|---|---|---|
| sent | `email_send_total{result="sent"}` | +1 |
| retried | `email_send_total{result="retried"}` | +1 |
| dead-lettered | `email_send_total{result="dead_lettered"}` and `email_dead_total` | +1 each |
| expired | `email_send_total{result="expired"}` and `email_expired_total` | +1 each |

## Failure modes and their effects

| Failure | Typed sentinel | Effect |
|---|---|---|
| the body is not a message | `ErrUnusableMessage` | dead-letter, acknowledge |
| the purpose selects no template | `ErrUnknownTemplate` (delivered) | dead-letter, acknowledge |
| the provider failed retryably | `email.ErrTransient` (delivered) | republish to the tier, acknowledge after the confirm |
| the provider failed permanently | `email.ErrPermanent` (delivered) | dead-letter, acknowledge |
| the republish was not confirmed | `ErrPublishUnconfirmed` | leave unacknowledged |
| the broker is unavailable at startup | `amqp.ErrBrokerUnavailable` (delivered) | stop the process before consuming |
| the provider cannot be built | `factory.ErrUnknownProvider` (delivered) | stop the process before consuming |
| the clock is missing | — | the default clock is used (the configuration fills it) |

Every sentinel is a plain `errors.New` sentinel, never `apperr`: this is a non-HTTP layer with no
status to map, the same precedent the delivered email domain sets (`email.ErrTransient`,
`amqp.ErrBrokerUnavailable`, `shared/idgen.ErrInvalidID`).

## Boundary cases

| Case | Result |
|---|---|
| the body is empty | undecodable → dead-letter, acknowledge |
| the body carries unknown fields | decoded — the contract may only grow, so a newer publisher's message is not dead-lettered |
| `expires_in_minutes` is zero or negative | expired → acknowledged without a send |
| `created_at` is in the future | the deadline is in the future too → sent |
| `attempt` is negative | treated as zero |
| `attempt` is at or above the cap | dead-lettered without a send |
| the provider answers slowly | the send exceeds the configured timeout → retryable |
| two deliveries arrive at once | each is handled by its own handler; the broker delivers each to one consumer |

## Test evidence

- `internal/email/worker/worker_test.go`: the decode-and-validate matrix, the expiry decision, the
  acknowledgement rules, two consumers over one fake, a redelivery after a crash, and the shutdown
  with a send in flight.
- `internal/email/worker/retry_test.go`: the ladder, the cap, an unconfirmed republish and an
  unconfirmed dead-letter.
- `internal/email/worker/config_test.go`: the documented defaults and the zero-value behaviour.
- `internal/email/amqp/consuming_test.go`: the delivery stream, the acknowledgement paths and the
  stream ending.
- A suite-wide scan asserts that no code and no full address reaches a log line, an error or a series.
