# Feature Specification: Watching the Pipeline — The Alerts, the Dashboard and What to Do About Them

**Feature Branch**: `023-delivery-observability`

**Created**: 2026-10-21

**Status**: Draft

**Input**: User description: "Watch the email pipeline: the alert rules for a stalled delivery, a dashboard that shows the pipeline's state, and the runbook that says what to do when each alert fires" (plan `docs/plan-auth-otp-2fa-email.md` §4.2, §5.9 and §9 stage 8; decisions D5, D6, O4; the counters the preceding features expose)

## Clarifications

### Session 2026-10-21

- Q: Where should the pipeline's alerts notify? → A: The rules are evaluated by the monitoring stack and the dashboard shows what is firing; the notification channel is an operator configuration left to the deployment and documented in the runbook (user decision, 2026-10-21). The stack has no channel today, and choosing one — an email account, a chat webhook — depends on a third-party account the project does not have (`OBS-09`, `OBS-14`).
- Q: Is a dashboard part of this feature? → A: Yes, one dashboard for the pipeline (user decision, 2026-10-21): the monitoring stack already provisions an empty dashboard directory, and an alert that fires with nothing to look at is an alert nobody can act on (`OBS-10`, `OBS-11`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A stalled pipeline is visible before anyone reports a missing code (Priority: P1)

Someone asked for a code and it never arrived. The person blames the site; the operator has no idea
whether the code is stuck in the queue, refused by the provider, expired in flight, or buried in the
dead-letter queue where nobody looks. The pipeline already counts everything that matters — how many
codes were sent, how many messages are waiting, how many died, how many people are being asked again —
but nobody is watching the counts. What is missing is the alarm: a set of named conditions, each with
a threshold and a patience period, that fire when the pipeline stops doing its job, and that can be
told apart from one another.

**Why this priority**: every other feature in this program assumed somebody would notice a problem.
Nobody does, until a person writes in. The alarms are what turns a silent failure into a known one.

**Independent Test**: read the rules and assert each condition the pipeline can stall on has its own
named rule with a threshold, a patience period and a severity — the dead-letter queue holding
something, nobody listening to the work queue, the oldest message waiting too long, the backlog of
unpublished codes too large, the publication failing, the provider refusing, the daily allowance spent,
and the verification failures climbing — and assert each rule's expression names the series that carry
the fact.

**Acceptance Scenarios**:

1. **Given** the dead-letter queue is holding a message, **When** the rules are evaluated, **Then** the rule that says so fires, naming that queue. (OBS-01)
2. **Given** nothing is listening to the work queue, **When** the rules are evaluated, **Then** the rule that says nobody is consuming fires, naming that queue. (OBS-02)
3. **Given** the oldest message in the work queue has been waiting longer than the documented patience, **When** the rules are evaluated, **Then** the rule that says so fires. (OBS-03)
4. **Given** the backlog of unpublished codes is above the documented threshold, **When** the rules are evaluated, **Then** the rule that says so fires. (OBS-04)
5. **Given** the publication has been failing for the documented period, **When** the rules are evaluated, **Then** the rule that says so fires. (OBS-05)
6. **Given** the provider is refusing sends, **When** the rules are evaluated, **Then** the rule that says so fires. (OBS-06)
7. **Given** the daily allowance has been reached, **When** the rules are evaluated, **Then** the rule that says so fires. (OBS-07)
8. **Given** the verification failures are climbing, **When** the rules are evaluated, **Then** the rule that says so fires. (OBS-08)

---

### User Story 2 - The operator sees the pipeline's state at a glance (Priority: P2)

When an alarm fires, the operator needs to see the shape of the problem, not a single number: is the
backlog growing or flat, is the provider slow or refusing, are the failures concentrated in one
purpose, is the code expiring in flight. The monitoring stack already collects everything; what it
lacks is a place where the pipeline's story is told in one view.

**Why this priority**: an alert tells the operator that something is wrong; the dashboard tells them
what. Without it, every alarm turns into a round of hand-written queries.

**Independent Test**: read the dashboard and assert it shows the pipeline's stages — the codes
requested and verified, the backlog, the queue's depth and its consumers, the send outcomes and the
provider's latency, the expired and dead-lettered counts — and that every panel names the series it
reads.

**Acceptance Scenarios**:

1. **Given** the dashboard, **When** it is opened, **Then** it shows the codes requested and verified, the backlog of unpublished codes, and the outcomes of every delivery — sent, retried, dead-lettered, expired. (OBS-10, OBS-11)
2. **Given** the dashboard, **When** it is opened, **Then** it shows the queue's depth, its consumers and how long the oldest message has waited, and the provider's answer time. (OBS-12)

---

### User Story 3 - Every alarm says what to do about it (Priority: P2)

An alarm with no action is noise, and noise trains people to ignore alarms. Each one must say what it
means, what the operator should do, and what "doing it" looks like — including the one case where the
answer is not a fix but a purchase: the provider's free allowance runs out, and at some point the
answer is to pay for more of them.

**Why this priority**: the runbook is the difference between an alarm that is acted on and one that is
ignored.

**Independent Test**: read the runbook and assert every alarm in the rules has an entry saying what it
means, what to do and when to escalate, and that the provider's allowance has its own entry with the
trigger that says when to buy more.

**Acceptance Scenarios**:

1. **Given** an alarm fires, **When** the operator reads the runbook, **Then** there is an entry for it that says what it means, what to do and when to escalate. (OBS-13, OBS-14)
2. **Given** the provider's daily allowance is being reached, **When** the operator reads the runbook, **Then** it says what the allowance is, what happens when it is spent, and what the trigger for buying more is. (OBS-15)

---

### Edge Cases

- **The pipeline is healthy**: no rule fires; the dashboard shows the numbers and the operator does
  nothing. (OBS-01…OBS-08)
- **The provider is refusing everything**: the refusal rule fires and the daily-allowance rule may
  fire with it; the two say different things and the runbook separates them. (OBS-06, OBS-07)
- **The worker is down**: nobody is listening, so the "nobody is consuming" rule fires; the "oldest
  message" rule fires with it once the patience elapses. (OBS-02, OBS-03)
- **The broker is down**: the broker's series stop arriving; the rules that read them must not fire
  on a stale value, and the absence of the series is itself visible. (OBS-16)
- **A queue is empty because nobody has asked for a code yet**: the backlog rule must not fire on an
  empty, quiet pipeline. (OBS-04)
- **The site and the publisher expose the same counter**: a rule that reads it must not be confused by
  one process reporting zero while the other reports the truth. (OBS-16)
- **A rule fires once and the pipeline recovers**: the alarm clears on its own; nothing has to be
  dismissed. (OBS-17)
- **The operator wants to be told by phone**: the notification channel is theirs to configure; the
  runbook says where to configure it. (OBS-09)
- **A new counter is added to the pipeline later**: the rules must keep reading the series they name,
  and a rule that names a series that no longer exists must be visible as broken rather than silently
  inert. (OBS-16)

## Requirements *(mandatory)*

### Functional Requirements

**The alarms (US1)**

- **OBS-01**: There MUST be a rule that fires when the dead-letter queue is holding a message, and it MUST name that queue.
- **OBS-02**: There MUST be a rule that fires when nothing is listening to the work queue, and it MUST name that queue.
- **OBS-03**: There MUST be a rule that fires when the oldest message in the work queue has been waiting longer than a documented patience, and the patience MUST be configurable.
- **OBS-04**: There MUST be a rule that fires when the backlog of unpublished codes is above a documented threshold, and the threshold MUST be configurable.
- **OBS-05**: There MUST be a rule that fires when the publication has been failing for a documented period.
- **OBS-06**: There MUST be a rule that fires when the provider is refusing sends.
- **OBS-07**: There MUST be a rule that fires when the daily allowance of the provider has been reached.
- **OBS-08**: There MUST be a rule that fires when the verification failures are climbing.
- **OBS-17**: Every rule MUST carry a name that says what broke, a severity, a patience period and an expression over the series that carry the fact; a rule MUST NOT be a single bare threshold with no name.

**The dashboard (US2)**

- **OBS-10**: The monitoring stack MUST provide one dashboard for the delivery pipeline, and it MUST be provisioned from the repository with no manual step.
- **OBS-11**: The dashboard MUST show the codes requested and verified, the backlog of unpublished codes, and the outcome of every delivery — sent, retried, dead-lettered, expired.
- **OBS-12**: The dashboard MUST show the queue's depth, its consumers, how long the oldest message has waited, and the provider's answer time.
- **OBS-15**: Every panel of the dashboard MUST name the series it reads.

**The runbook (US3)**

- **OBS-13**: The runbook MUST have an entry for every rule, saying what the alarm means, what the operator should do and when to escalate.
- **OBS-14**: The runbook MUST document where and how to configure a notification channel, and MUST state that no channel is configured by default.
- **OBS-16**: The runbook MUST state which rules read which process's counters, so a counter that two processes expose is read from the right one.
- **OBS-18**: The runbook MUST state the provider's allowance, what happens when it is spent, and the trigger for buying more: sustained use above the agreed daily volume, a public launch, or any campaign.

**Cross-cutting**

- **OBS-09**: The rules MUST be evaluated by the monitoring stack and MUST NOT require a notification service to be added to the stack.
- **OBS-19**: The rules MUST be validated by the monitoring stack's own tooling, so a rule that cannot parse or a series that does not exist is caught by the gate rather than at three in the morning.
- **OBS-20**: Every rule, panel and runbook entry MUST be written in English.
- **OBS-21**: The rules MUST NOT contain a credential, and the dashboard MUST be reachable only through the operator's route.
- **OBS-22**: Every requirement MUST ship tests: the rules and the dashboard are validated by the deployment's checks, edge cases first — a rule that must exist, a series that must be named, a threshold that must be configurable — and then the representative happy path.

### Key Entities

- **A rule**: one named condition with a severity, a patience period, an expression over named series
  and a runbook entry.
- **A panel**: one chart or number on the dashboard, naming the series it reads.
- **A runbook entry**: what an alarm means, what to do about it and when to escalate.
- **The pipeline's stages**: the codes requested, the codes verified, the backlog, the queue, the
  provider, the outcomes — the shape the dashboard tells.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the pipeline's ways to stall have a named rule: 0 stalls without an alarm.
- **SC-002**: 0 rules fire on a healthy, quiet pipeline.
- **SC-003**: Every rule's expression names the series that carry the fact it watches, and the gate proves it: 0 rules reading a series that does not exist.
- **SC-004**: Every rule has a runbook entry, and every entry names an action: 0 alarms without a next step.
- **SC-005**: 100% of the pipeline's stages are visible on the dashboard: 0 questions that need a hand-written query to answer.
- **SC-006**: 0 credentials in the rules or the dashboard, and the dashboard is reachable only through the operator's route.
- **SC-007**: The rules are validated by the stack's own gate: a broken rule fails the build, not the night.
- **SC-008**: The provider's allowance and the trigger for buying more are documented: 0 surprise when the allowance is spent.

## Assumptions

- This is the ninth feature derived from `docs/plan-auth-otp-2fa-email.md` §9. It delivers stage 8: the
  observation of the pipeline the earlier features built.
- Every counter this feature watches is already exposed by the processes that own it — the site's, the
  publisher's, the worker's and the broker's — so the feature adds instrumentation nowhere.
- The broker's own series are the broker's, not ours: the names and the labels come from the broker's
  documentation, and the first task of the implementation verifies them against a running broker.
- The notification channel is the operator's to configure (clarification, 2026-10-21): the stack gains
  no notification service, and the runbook says where to add one.
- The dashboard is delivered (clarification, 2026-10-21): the monitoring stack already provisions an
  empty dashboard directory, and an alarm needs somewhere to be read.
- The provider's free allowance is a real ceiling, and the plan fixes the trigger for buying more:
  sustained use above the agreed daily volume, a public launch, or any campaign.
- The alert rules belong to the monitoring stack, not to the application: the application's own
  counters are the facts, the rules are the reading of them.
- Requirement-ID prefix `OBS` is exclusive to this feature; `D5`, `D6` and `O4` belong to the plan and
  are not requirement IDs.
