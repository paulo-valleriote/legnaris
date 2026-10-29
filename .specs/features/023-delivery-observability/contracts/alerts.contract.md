# Contract: The alarms

**Feature**: `.specs/features/023-delivery-observability/` | **Requirement IDs**: `OBS-01` … `OBS-09`,
`OBS-16`, `OBS-19` | **Owner**: `deploy/prometheus/rules.yml`

## Purpose

The named conditions under which the delivery pipeline is declared broken. Every rule is read by the
monitoring stack and answered by a runbook entry.

## The rules

| Name | Watches | Fires when | Severity | Patience |
|---|---|---|---|---|
| the dead-letter queue holds something | the broker's queue depth | that queue is not empty | the worst | short |
| nobody is listening to the work queue | the broker's consumer count | that queue has no consumer | the worst | short |
| the oldest message has waited too long | the broker's head-message age | a message has waited past the patience | high | the patience itself |
| the backlog of unpublished codes is too large | the publisher's backlog gauge | above the threshold | high | short |
| the publication is failing | the publisher's outcomes | the failures keep coming | high | the documented period |
| the provider is refusing | the worker's outcomes | the retryable failures keep coming | high | short |
| the daily allowance is spent | the site's cap refusals | the refusals keep coming | medium | short |
| the verification failures are climbing | the site's verification outcomes | the refusals keep coming | medium | short |

## Rules

- **Every rule is named, severitied and patient.** A bare threshold with no name is not an alarm
  (OBS-17).
- **Every rule's expression names the series that carry the fact.** A rule that reads a counter two
  processes expose names the process that owns it, so the site's zero is never mistaken for the
  publisher's truth (OBS-16).
- **The thresholds and the patience periods are configurable**, so an operator can tune them without
  editing the rules (OBS-03, OBS-04).
- **The queue-scoped rules need the broker's per-object metrics.** The RabbitMQ documentation states
  that alerting is not possible with the aggregated mode (`research.md` R2); the broker is configured
  accordingly.
- **The oldest message's age is read through the broker's narrow query**, not through its full
  per-object output (`research.md` R3).
- **The rules are evaluated by the monitoring stack and need no notification service.** Where they are
  read is the operator's configuration (`OBS-09`, `OBS-14`).
- **No rule carries a credential** (`OBS-21`).

## Boundary cases

| Case | Result |
|---|---|
| the pipeline is healthy and quiet | no rule fires |
| the broker stops reporting | the queue-scoped rules see a stale value; the absence of the series is visible in the dashboard |
| the backlog is zero because nobody asked for a code | the backlog rule does not fire |
| the worker is down | the "nobody is listening" rule fires, and the age rule follows |
| the provider is refusing and the allowance is spent | both rules fire; the runbook separates them |
| the site and the publisher both report the backlog | the rule reads the publisher's job |
| a rule names a series that no longer exists | the monitoring stack's own tooling catches it, and the deployment's checks fail the build |

## Test evidence

- `deploy/compose_test.go`: every rule exists, is named, has a severity, a patience period and an
  expression; the expressions name the series they watch; the thresholds are configurable; the runbook
  has an entry per rule.
- The monitoring stack's own tooling validates the expressions (`promtool check rules`).
