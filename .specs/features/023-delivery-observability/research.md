# Research: Watching the Pipeline

**Feature**: `.specs/features/023-delivery-observability/` | **Requirements**: `OBS-01` … `OBS-22` | **Date**:
2026-10-21

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §4.2 (the provider's allowance and the trigger to
buy more), §5.9 (the metrics and the alarms) and §9 stage 8.

Every counter this feature watches is already exposed by the process that owns it, so the feature adds
instrumentation nowhere. The work is in reading them, and in the two places where the broker's own
series need care.

## R1. The rules are the monitoring stack's own

**Decision**: the alarms live in a rules file beside the monitoring stack's configuration, and the
monitoring stack loads and evaluates them.

**Rationale**: the facts are already collected; the alarms are a reading of them, and the component
that already holds the series is the one that should hold the reading. The stack's gate already
validates its configuration, so the rules are validated by the same gate.

**Alternatives considered**:

- *Alerts inside the application* — rejected: the application is the thing being watched; an
  application that pages about itself is a second thing to keep alive.
- *Alerts in the dashboard's own alerting* — rejected: it splits the truth in two places, and the
  dashboard is meant to show the pipeline rather than to decide when it is broken.

## R2. The broker must expose per-object metrics for any queue-scoped alarm

**Decision**: the broker is configured to expose its per-object metrics, so a rule can name a queue.

**Rationale**: the RabbitMQ documentation is explicit — in the aggregated mode "per-object metrics and
**alerting are not possible with aggregation**". Three of the plan's alarms are queue-scoped (the
dead-letter queue holds something, nobody listens to the work queue, the oldest message has waited
too long), so the aggregated mode cannot express them at all.

**Alternatives considered**:

- *Keep the aggregated mode and alert on the whole broker* — rejected: "the broker has messages" is
  not "the dead-letter queue has messages"; the whole point of the alarm is to name the queue.
- *Scrape the per-object endpoint separately* — rejected: the same numbers would arrive twice under
  two jobs, and the duplicate-series warning would drown the real signal. One endpoint, per-object.

One consequence is worth recording: switching the broker from the aggregated mode to the per-object
mode changes the identity of the queue series (the queue label appears), so the monitoring stack sees
those series reset once when the mode changes. That happens while the pipeline is dark, and the
dashboard's panels read the new shape from then on.

## R3. The oldest message's age comes from the broker's narrow query

**Decision**: the age of the oldest message is read from the broker's detailed endpoint, with a
family filter, rather than from the full per-object output.

**Rationale**: the detailed endpoint exists for exactly this — the documentation notes that querying
only the families needed is "up to 60 times more efficient" than the full per-object output, and the
head message's timestamp lives in the queue's metric family. A rule that needs one number should not
pay for the whole set.

**Alternatives considered**:

- *Read the age from the full per-object output* — rejected: the same fact, at a cost the
  documentation says is up to sixty times higher.
- *Approximate the age from the backlog's growth* — rejected: an approximation in an alarm is a
  hypothesis, not a fact.

## R4. A counter that two processes expose is read from the right one

**Decision**: the rules that read the backlog and the publication outcomes name the publisher's job;
the site's registry exposes the same counter names at zero and is not the source.

**Rationale**: the metrics package is shared, so the site's `/metrics` endpoint publishes the
publisher's series names with zero values, and the publisher's own listener publishes the real ones.
An expression that sums across both would read a constant plus the truth; an expression that does not
name the job could read either. The runbook says which process owns which counter, so the rule and the
runbook entry agree.

**Alternatives considered**:

- *Stop the site from publishing the publisher's series* — rejected: it would split one registry into
  two shapes and break every existing consumer of those names.
- *Sum across the jobs* — rejected: the site's zero would be counted as a fact about the pipeline.

## R5. The dashboard is one provisioned document

**Decision**: one dashboard for the delivery pipeline, provisioned from the repository through the
stack's existing dashboard provider.

**Rationale**: the monitoring stack already provisions a dashboard directory, and it is empty. An
alarm with nothing to look at is an alarm nobody acts on, and the pipeline's story — requested,
verified, queued, sent, retried, dead, expired — does not fit on one chart; it fits on one screen.

**Alternatives considered**:

- *One dashboard per stage* — rejected: the stages are one story; splitting them means the operator
  assembles the picture by hand, which is what the dashboard exists to avoid.
- *No dashboard* — rejected by the user's decision (2026-10-21).

## R6. The runbook is one entry per alarm, and the channel is the operator's

**Decision**: the runbook has an entry per rule — what it means, what to do, when to escalate — and it
documents where to configure a notification channel without configuring one.

**Rationale**: the user's decision (2026-10-21) keeps the stack free of a notification service: the
stack has no channel today, and choosing one depends on a third-party account the project does not
have. An alarm that is evaluated, visible on the dashboard and written down in the runbook is a
complete feature; where it is read is the operator's choice.

**Alternatives considered**:

- *An alert manager in the stack with a contact point* — rejected by the user's decision.
- *The dashboard's own contact points* — rejected: same dependency on a third-party account, with the
  additional cost of a second place where the truth lives.

## R7. The provider's allowance has its own entry and its own trigger

**Decision**: the runbook states the provider's daily allowance, what happens when it is spent, and
the trigger for buying more: sustained use above the agreed daily volume, a public launch, or any
campaign.

**Rationale**: the plan fixes the free tier's ceiling and names the trigger. With two factors on every
sign-in, the allowance is not a theoretical limit — it is the number that decides how the product is
used. The alarm on the daily-cap refusals is the signal that the ceiling is near.

**Alternatives considered**:

- *A rule that fires at a percentage of the allowance* — rejected: the cap is already counted and
  already refuses, so a rule that guesses at the percentage would duplicate a fact the server knows
  exactly.

## R8. Deliberately out of this feature

**Decision**: the following stay out:

| Out | Reason |
|---|---|
| a notification service or a contact point | the user's decision (2026-10-21); the runbook says where to add one |
| new counters in the application | every fact this feature watches already exists |
| the broker's own memory and disk alarms | the broker's own alarms cover them; they are not the pipeline's |
| a dashboard for the site's request traffic | the site's own metrics are a separate concern |
| tracing | the stack has no tracing today, and the counters answer the questions the plan asks |

## Resolved unknowns

| Technical Context item | Resolution |
|---|---|
| Where the rules live | the monitoring stack's own rules file, validated by its gate (`R1`) |
| The broker's metrics mode | per-object, because alerting is impossible with aggregation (`R2`) |
| The oldest message's age | the broker's narrow detailed query (`R3`) |
| Duplicated counter names | the rules name the process that owns the counter (`R4`) |
| The dashboard | one provisioned document for the pipeline (`R5`) |
| The runbook and the channel | one entry per alarm; the channel is the operator's (`R6`) |
| The provider's allowance | its own entry and the plan's trigger (`R7`) |
| Deferred scope | notification services, new counters, the broker's own alarms, the site's traffic, tracing (`R8`) |
