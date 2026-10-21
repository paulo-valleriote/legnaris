# Contract: The worker's counters

**Feature**: `.specs/features/020-email-worker/` | **Requirement IDs**: `WKR-21`, `WKR-22` | **Owner**:
`internal/platform/metrics`

## Purpose

The numbers that say whether the pipeline is flowing, and the rule that keeps a credential out of
them.

## The series

| Series | Type | Labels | Incremented when |
|---|---|---|---|
| `legnaris_email_send_total` | counter | `result` = `sent` \| `retried` \| `dead_lettered` \| `expired` | a delivery reaches its outcome |
| `legnaris_email_send_duration_seconds` | histogram | — | a send attempt finishes (successfully or not) |
| `legnaris_email_dead_total` | counter | — | a message reaches the dead-letter route |
| `legnaris_email_expired_total` | counter | — | a message is dropped because its code had expired |

`legnaris_email_dead_total` is the series the earlier feature already declared; this feature is its
first writer.

## Rules

- **Every outcome is counted separately.** A quiet pipeline and a broken one look different: the
  counts move from `sent` to `retried`, `dead_lettered` or `expired` (WKR-21).
- **No series carries a code.** The code is a short-lived credential; a metric is not a place for it.
- **No series carries an address.** The recipient is never a label, and never a value.
- **The listener is the worker's own**, addressed by `EMAIL_WORKER_METRICS_ADDR` (default `:9101`),
  beside the publisher's listener on `:9100`; the worker publishes no host port for it (WKR-20).

## Reading the counters

| Question | Series |
|---|---|
| is the pipeline delivering? | `email_send_total{result="sent"}` |
| is the provider refusing? | `email_send_total{result="retried"}` and the duration histogram |
| are messages dying? | `email_send_total{result="dead_lettered"}` and `email_dead_total` |
| are codes expiring before delivery? | `email_send_total{result="expired"}` and `email_expired_total` |
| how slow is the provider? | `email_send_duration_seconds` |

The alert rules that read these series belong to the next feature.

## Boundary cases

| Case | Result |
|---|---|
| a delivery ends in no outcome (the process is stopped mid-send) | no counter moves; the message stays with the broker |
| the same message is delivered twice (a redelivery) | each attempt is counted; the provider de-duplicates the send itself |
| the metrics listener cannot bind | the process fails fast with a typed error — a worker nobody can observe is a worker nobody can trust |

## Test evidence

- `internal/platform/metrics/metrics_test.go`: each outcome increments its own counter, the duration
  histogram observes, and a suite-wide scan asserts that no series carries a code or a full address.
