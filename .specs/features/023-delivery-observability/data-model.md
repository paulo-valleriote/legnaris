# Phase 1 Data Model: Watching the Pipeline

**Feature**: `.specs/features/023-delivery-observability/` | **Date**: 2026-10-21

No persisted shape changes. The feature introduces three *document* shapes — a rule, a panel and a
runbook entry — and reads the counters the processes already expose.

---

## 1. The counters being watched

| Counter | Exposed by | Reads |
|---|---|---|
| the codes issued, by purpose | the site | how many people are being asked for a code |
| the code verifications, by result | the site | how many made it through and how many refused |
| the resend outcomes, by result | the site | how many asked again and how many were refused |
| the daily-cap refusals | the site | how often the allowance was reached |
| the pending backlog | the publisher | how many codes are waiting to be published |
| the publication outcomes, by result | the publisher | how many were published, retried or failed |
| the dead-letter count | the worker | how many messages were parked for an operator |
| the send outcomes, by result | the worker | how many were sent, retried, dead-lettered or expired |
| the provider's answer time | the worker | how long the provider takes |
| the expired count | the worker | how many codes were delivered too late |
| the queue's depth and its consumers | the broker | whether anybody is listening and how much is waiting |
| the oldest message's age | the broker | how long a code has been waiting |

The counters the site and the publisher share are read from the process that owns them (`research.md`
R4).

---

## 2. The rules

| Rule | Watches | Fires when | Severity |
|---|---|---|---|
| the dead-letter queue holds something | the broker's queue depth | the queue is not empty, for a short while | the worst — a code is lost until an operator looks |
| nobody is listening to the work queue | the broker's consumer count | no consumer, for a short while | the worst — nothing is being delivered at all |
| the oldest message has waited too long | the broker's head-message age | a message has waited longer than the patience | high — codes are expiring in the queue |
| the backlog is too large | the publisher's backlog gauge | above the threshold, for a short while | high — codes are late |
| the publication is failing | the publisher's outcomes | the failures keep coming, for the documented period | high — the codes are not reaching the broker |
| the provider is refusing | the worker's outcomes | the retryable failures keep coming | high — the codes are not leaving the building |
| the daily allowance is spent | the site's cap refusals | the refusals keep coming | medium — the pipeline is working, the ceiling is reached |
| the verification failures are climbing | the site's verification outcomes | the refusals keep coming | medium — either the delivery is broken or somebody is guessing codes |

Every rule carries a name that says what broke, a severity, a patience period, a threshold where a
threshold applies, and an expression over the series that carry the fact. The thresholds and the
patience periods are configurable, so an operator can tune them without editing the rules.

---

## 3. The panels

| Panel | Reads |
|---|---|
| the codes requested and verified | the site's issuance and verification counters |
| the backlog of unpublished codes | the publisher's backlog gauge |
| the outcomes of every delivery | the worker's send outcomes |
| the provider's answer time | the worker's duration measure |
| the expired and dead-lettered counts | the worker's counters |
| the queue's depth and its consumers | the broker's queue series |
| the oldest message's age | the broker's head-message timestamp |
| the daily allowance | the site's cap refusals |

Every panel names the series it reads, so a panel that stops being fed is visible as an empty panel
rather than a silent zero.

---

## 4. The runbook entries

One entry per rule:

| Field | Meaning |
|---|---|
| the alarm | the rule's name, so the entry and the rule are the same thing |
| what it means | the failure in one sentence, in the operator's words |
| what to do | the first action, then the second |
| when to escalate | the condition that says this is not the operator's problem alone |

Plus the entries that are not about a rule: the provider's allowance and the trigger for buying more,
where to configure a notification channel, and which process owns which counter.
