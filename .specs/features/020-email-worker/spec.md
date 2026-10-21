# Feature Specification: The Email Worker — One Process That Turns a Queued Code into a Delivered Email

**Feature Branch**: `020-email-worker`

**Created**: 2026-10-21

**Status**: Draft

**Input**: User description: "Deliver the email worker: a broker consumer process that reads queued OTP messages, checks the code's expiry, renders the template, sends through the provider port, retries transient failures through the delay tiers, dead-letters permanent and exhausted ones, and shuts down gracefully" (plan `docs/plan-auth-otp-2fa-email.md` §5.6, §5.7, §5.9, §5.10 and §9 stage 6; decisions D5, D6, O1, O2; findings F-14, F-16)

## Clarifications

### Session 2026-10-21

- Q: Does the delivery process consult the challenge store before it sends an email? → A: No (user decision, 2026-10-21): the message carries its own deadline — its creation time and its advertised lifetime — so the process needs no database and the deadline it honours is the one the message states; a code the person already typed may still be delivered once, and the delivery is harmless because the code is single-use (`WKR-01`, `WKR-07`).
- Q: What happens to a queued message whose code expired before the worker got to it? → A: Acknowledge it and count it as expired (user decision, 2026-10-21): the dead-letter route stays reserved for messages that can never succeed, so a real failure is never buried under stale traffic, and a systemic backlog shows up in the expired counter and the queue-depth alert instead (`WKR-03`, `SC-002`).
- Q: Is a tool that replays the dead-letter queue part of this feature? → A: No (user decision, 2026-10-21): the dead-letter route is where an operator looks; re-sending a dead code is a separate operator tool for a later cycle, and this feature ships no operator command beyond the delivery process itself (`WKR-01`, Assumptions).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - The queued code arrives, once, and never stale (Priority: P1)

Someone has just proved a password or made a fresh account, and is waiting for six digits. Those
digits were written down durably and handed to the message broker; now a separate process has to
pick them up, check that they are still worth sending, render them into an email and hand it to
the provider — and only then is the message gone from the queue. A code whose lifetime has
already elapsed must never reach an inbox: delivering it invites a person to type digits that are
already dead. A message the process cannot understand must not be retried forever either; it goes
where an operator can see it.

**Why this priority**: without a consumer, the pipeline the earlier features built stops at the
broker. Every queued code is a person waiting for an email that will never be sent, and the
session that code unlocks never arrives. This is the last missing link between "the code was
requested" and "the code is in the inbox".

**Independent Test**: drive the consumer with a recorded broker and a recording sender: deliver a
valid message and assert the rendered subject, text body and HTML body reach the sending port with
the message identity as the idempotency key, and that the message is acknowledged only after the
send succeeded; deliver a message whose lifetime has elapsed and assert nothing is sent, the
message is acknowledged and the outcome is counted as expired; deliver a body that is not a
message and assert it is dead-lettered and acknowledged; deliver a message whose purpose selects
no template and assert the same.

**Acceptance Scenarios**:

1. **Given** a queued message whose code is still valid, **When** the consumer receives it, **Then** it is rendered from its purpose's template and handed to the sending port with the message identity as the idempotency key, and the message is acknowledged only after the send succeeded. (WKR-02, WKR-06)
2. **Given** a queued message whose code lifetime has elapsed, **When** the consumer receives it, **Then** nothing is sent, the message is acknowledged and the outcome is recorded as expired. (WKR-03, WKR-07)
3. **Given** a queued message that cannot be decoded, whose identity is not canonical, or whose code is missing, **When** the consumer receives it, **Then** it is moved to the dead-letter route, acknowledged, and never sent. (WKR-04)
4. **Given** a queued message whose purpose selects no template, **When** the consumer receives it, **Then** it is moved to the dead-letter route and acknowledged, and no send is attempted. (WKR-05)
5. **Given** two consumers running at the same time, **When** one message is queued, **Then** the message reaches exactly one of them, so the code is sent once. (WKR-16)

---

### User Story 2 - A provider outage delays the code, it never loses it (Priority: P1)

The provider is a third party on the internet: it rate-limits, it has bad minutes, it rejects an
address outright. None of that may cost a person their code. A failure that could succeed later is
put aside for a short while and tried again — the first retry after about half a minute, the next
after two minutes, and the last ones after five — so a provider that recovers in a minute does not
cost the person a second wait, while a provider that is down does not cost them a flood of
requests. A failure that can never succeed, and a message that has used up its attempts, are not
retried at all: they are set aside where an operator can see them, so a person who cannot sign in
becomes an operator's task instead of a mystery.

**Why this priority**: this is the difference between "the provider was down for two minutes" and
"the person never got the code and cannot sign in". A pipeline that drops on the first error, or
that retries forever, is worse than one that waits.

**Independent Test**: drive the consumer with a sender that fails with a retryable outcome and
assert the message is republished to the first delay tier with its attempt count incremented and
that the original is acknowledged only after the republish was confirmed; fail again and assert
the second and third delay tiers are used; refuse the republish and assert the original stays
unacknowledged; exhaust the attempt cap and assert the message reaches the dead-letter route
without another send; fail with a permanent outcome and assert the message reaches the dead-letter
route at once.

**Acceptance Scenarios**:

1. **Given** a send that failed with a retryable outcome, **When** the consumer handles it, **Then** the message is republished to the delay tier that matches its attempt count, with the attempt count incremented, and the original is acknowledged only after the republish was confirmed. (WKR-09, WKR-15)
2. **Given** a message republished for a retry, **When** it is inspected, **Then** it carries the same identity, recipient, template, data and creation time as the original, and only the attempt count differs. (WKR-15)
3. **Given** a send that failed with a permanent outcome, **When** the consumer handles it, **Then** the message is moved to the dead-letter route, acknowledged, and counted as dead-lettered. (WKR-11)
4. **Given** a message whose attempt count already reached the cap, **When** the consumer receives it, **Then** it is moved to the dead-letter route without another send, acknowledged, and counted as dead-lettered. (WKR-12)
5. **Given** a republish the broker did not confirm, **When** the consumer handles it, **Then** the original message is left unacknowledged, so the broker redelivers it and no send is lost. (WKR-13)

---

### User Story 3 - The delivery process stands on its own and survives disruption (Priority: P1)

The process that serves requests must never be dragged down by a sending problem, and the process
that sends must never be dragged down by a broker problem. They are separate programs: the broker
can be down for an hour and people can still use the site, and the provider can be refusing every
address while the site stays up. The delivery process also has to survive its own disruption: a
broker connection that drops in the middle of the night is re-established on its own, and a
restart for a deploy drains what it was doing instead of dropping it. A configuration that cannot
work — no broker address, a provider that is not configured — is refused at startup with a clear
message, before anything is consumed, so the operator learns about it from the process and not from
a person's missing email.

**Why this priority**: an operator-facing process that starts broken, keeps failing silently, or
loses its work on every restart is a liability in the stack; the whole point of a separate
process is that it can be restarted and supervised without consequence.

**Independent Test**: start the delivery process with a blank broker address and with a provider
the configuration does not name, and assert each fails fast with a typed error and consumes
nothing; drop the broker connection in the middle of a run and assert the consumer re-registers
itself and consumes again; ask the process to stop and assert it takes no new message, finishes
the one in flight, leaves it unacknowledged and releases its resources; start the request-serving
process with no email and no broker variable set and assert it starts and its documented
required-variable list is unchanged.

**Acceptance Scenarios**:

1. **Given** a blank or unusable broker address, **When** the delivery process starts, **Then** it fails fast with a typed error and consumes nothing. (WKR-19)
2. **Given** a provider the configuration does not name, **When** the delivery process starts, **Then** it fails fast with a typed error and consumes nothing. (WKR-19)
3. **Given** a broker connection that drops while the process runs, **When** the broker comes back, **Then** the consumer re-establishes the connection, registers itself again and continues consuming, without a restart. (WKR-17)
4. **Given** a request to stop, **When** the process shuts down, **Then** it takes no new message, finishes the message already in flight, leaves an unfinished message unacknowledged and releases its resources. (WKR-18)
5. **Given** the request-serving process and no email and no broker variable set, **When** it starts, **Then** it starts, and the documented required-variable list is unchanged. (WKR-19)

---

### User Story 4 - The pipeline says what it is doing (Priority: P2)

An operator has to be able to tell a quiet pipeline from a broken one: how many sends succeeded,
how many are waiting for another try, how many died for good, how many were dropped because the
code was already expired, and how long the provider takes to answer. These are the numbers the
alerts of the following stage will read, so they have to exist here, with the same discipline the
rest of the system applies to secrets: no code, no full address, nothing that turns a metric into
a credential leak.

**Why this priority**: without these counters, a broken pipeline is discovered by people who cannot
sign in. The counters themselves are not the alerting — that is the next feature — but nothing can
be alerted on until they exist.

**Independent Test**: run the consumer through a send that succeeded, a send that failed with a
retryable outcome, a dead-letter and an expired message, and assert each outcome is counted
separately and that the send duration is observed; scan every produced series and every log line
and assert no code and no unmasked address appears in any of them.

**Acceptance Scenarios**:

1. **Given** sends that succeeded, failed retryably, failed permanently and were never attempted because the code was expired, **When** the outcomes are observed, **Then** each is counted separately and the duration of each send attempt is observed. (WKR-21)
2. **Given** the consumer's logs and metrics, **When** they are inspected, **Then** a message is identified by its identity and its purpose and the recipient is masked, and no code and no full address appears anywhere. (WKR-06, WKR-21, WKR-22)

---

### Edge Cases

- **Code already expired when the message is consumed**: acknowledged without sending and counted as expired; the provider is never called, and the message does not reach the dead-letter route. (WKR-03)
- **Code the person already typed before the worker got to the message**: the message is still delivered — the consumer does not consult the challenge store, and the code is single-use, so the extra email is harmless. (WKR-07)
- **Message body that is not a message at all**: dead-lettered and acknowledged, so a poison payload cannot be retried forever. (WKR-04)
- **Message whose identity is not a canonical UUID v7**: dead-lettered and acknowledged; it is never republished, so a malformed identity never becomes a provider idempotency key. (WKR-04)
- **Message with no code**: dead-lettered and acknowledged; nothing is sent. (WKR-04)
- **Message whose purpose selects no template**: dead-lettered and acknowledged; the renderer is never asked for a template that does not exist. (WKR-05)
- **Message whose advertised lifetime is zero or negative**: treated as already expired, acknowledged without sending. (WKR-07)
- **Message whose creation time is in the future**: its deadline is in the future too, so it is sent; the clock is the only authority on the deadline. (WKR-07)
- **Send fails with a rate limit, including a provider that reports its daily allowance spent**: retryable, so the code waits and is tried again instead of being dropped. (WKR-09)
- **Send fails with an invalid recipient**: permanent, so the message is dead-lettered at once and no address is retried forever. (WKR-11)
- **Send exceeds the per-send timeout**: retryable. (WKR-09)
- **Republish the broker does not confirm**: the original stays unacknowledged and the broker redelivers it; the send is never lost. (WKR-13)
- **Process crashes after the provider accepted the send but before the acknowledgement**: the message is redelivered and sent again; the duplicate is harmless because the provider de-duplicates by the message identity. (WKR-08)
- **Process crashes before the send**: the message is redelivered and sent once. (WKR-08)
- **Message redelivered by the broker after a consumer cancelled**: re-registered and consumed again; the attempt count on the message keeps the retry budget from being reset. (WKR-17)
- **Two consumers running at the same time**: the broker delivers each message to one of them, so no code is sent twice. (WKR-16)
- **Broker blocked by a memory or disk alarm**: the retry republish fails, the original stays unacknowledged and the code waits. (WKR-13)
- **Queue at its maximum length**: the broker refuses new messages instead of dropping them; the publisher's confirmation is what tells the pipeline the message was not accepted. (WKR-09)
- **Stop requested while a send is in flight**: the in-flight send finishes, the message is acknowledged on success, and an unfinished message is left unacknowledged for redelivery. (WKR-18)
- **Provider configured but the key is blank**: the process refuses to start rather than failing on the first person's code. (WKR-19)
- **Code present in a candidate log line, error message, metric or subject**: prohibited by contract and caught by an assertion that scans the suite's logs, errors and series. (WKR-06, WKR-21)

## Requirements *(mandatory)*

### Functional Requirements

**Delivery (US1)**

- **WKR-01**: The system MUST provide the delivery as its own executable, separate from the request-serving process; it MUST NOT require the database to run and the request-serving process MUST NOT depend on it to start.
- **WKR-02**: The consumer MUST acknowledge a message only after its send succeeded; every other outcome MUST either leave the message unacknowledged or move it to the dead-letter route.
- **WKR-03**: WHEN the message's code lifetime has elapsed, the consumer MUST acknowledge the message without sending it and MUST record the outcome as expired; it MUST NOT move an expired message to the dead-letter route.
- **WKR-04**: A message that cannot be decoded, whose identity is not a canonical UUID v7, or whose code is missing MUST be treated as permanently unusable: it MUST be moved to the dead-letter route and acknowledged, and MUST NOT be sent or republished.
- **WKR-05**: WHEN a message's purpose selects no template, the consumer MUST treat it as a permanent failure: the message MUST be moved to the dead-letter route and acknowledged, and no send MUST be attempted.
- **WKR-06**: The consumer MUST render the message's subject, plain-text body and HTML body from the template its purpose selects, MUST hand them to the sending port with the message identity as the provider idempotency key, and MUST NOT let the code reach a log line, an error message, a metric series or a subject.
- **WKR-07**: The expiry decision MUST be taken from the message's creation time and its advertised lifetime against an injected clock, so the decision is deterministic and a message with no usable lifetime is treated as already expired; the consumer MUST NOT consult the challenge store to decide whether a code is still worth sending.
- **WKR-08**: Delivery MUST be at-least-once: a message redelivered after a crash MUST be sent again, and the duplicate MUST be harmless because the provider de-duplicates it by the message identity.

**Retry (US2)**

- **WKR-09**: WHEN a send fails with a retryable outcome, the consumer MUST republish the message to the delay tier that matches its attempt count with the attempt count incremented, and MUST acknowledge the original only after the republish was confirmed by the broker.
- **WKR-10**: The attempt count MUST be carried by the message itself and MUST count the send attempts already made; the consumer MUST NOT depend on the broker's dead-letter history to decide whether to retry.
- **WKR-11**: WHEN a send fails with a permanent outcome, the consumer MUST move the message to the dead-letter route, acknowledge it and record the dead-letter.
- **WKR-12**: WHEN a message's attempt count has reached the configured cap, the consumer MUST move it to the dead-letter route without sending again, acknowledge it and record the dead-letter.
- **WKR-13**: WHEN a republish is not confirmed by the broker, the consumer MUST NOT acknowledge the original message, so the broker redelivers it and no send is lost.
- **WKR-14**: The delay tiers MUST be the ones the broker topology already declares — 30 seconds for the first retry, 2 minutes for the second and 5 minutes for every later retry — and the send-attempt cap MUST be 5; both MUST be configurable with documented defaults, and the retry MUST NOT require a broker plugin or a version-specific extension.
- **WKR-15**: A republished message MUST keep the identity, recipient, template, data and creation time of the original; only the attempt count may change.

**Lifecycle (US3)**

- **WKR-16**: The consumer MUST bound the messages it holds in flight with a configurable limit, and two consumers running at the same time MUST NOT send the same message.
- **WKR-17**: WHEN the broker connection is lost, the consumer MUST re-establish it, register itself again and continue consuming, without requiring a restart of the process.
- **WKR-18**: WHEN the process is asked to stop, it MUST take no new message, finish the message already in flight, leave an unfinished message unacknowledged, and release the broker connection and its resources.
- **WKR-19**: The delivery process MUST validate its own broker and provider configuration at startup and MUST fail fast with a typed error when it is absent or unusable; the request-serving process MUST NOT validate them and its documented required-variable list MUST NOT change.
- **WKR-20**: The delivery process MUST expose its metrics on its own listener, addressed by a configurable setting, and MUST NOT publish a host port for it.

**Observability (US4)**

- **WKR-21**: The consumer MUST expose counters for send outcomes by result, for dead-lettered messages and for expired messages, and MUST observe the duration of each send attempt; no series may carry a code or an unmasked address.
- **WKR-22**: Logs MUST identify a message by its identity and its purpose, and MUST mask the recipient as the first character of the local part plus the domain.

**Cross-cutting**

- **WKR-23**: Every new failure mode MUST be a typed error with an exported message and a documented effect — retry, dead-letter, acknowledge without sending, or stop the process — and the consumer MUST act on it exactly.
- **WKR-24**: Every requirement MUST ship tests: edge cases first, then one happy path, asserting the typed error on failure paths, and staying clean under the race detector.
- **WKR-25**: All identifiers, comments, log messages and user-facing strings MUST be written in English.
- **WKR-26**: The delivery process MUST be built as its own executable in the shipped image, so it can be started, restarted and scaled apart from the request-serving process.
- **WKR-27**: No unit test may require a broker or a provider: both stay behind seams with test doubles, and the integration test MUST self-skip when no broker is available.

### Key Entities

- **Queued message**: the contract-shaped payload the broker holds — stable identity, type, recipient, template, data (code, lifetime, purpose), creation time, attempt count.
- **Send attempt**: one delivery of one message, ending in exactly one of three outcomes — sent, retried, dead-lettered — plus the expired outcome, where nothing is sent at all.
- **Delay tier**: one of the broker's own delay queues (about 30 seconds, 2 minutes, 5 minutes) a retried message waits in before it is routed back to the work queue.
- **Dead-letter route**: the broker's queue where a message that can never succeed is set aside for an operator to inspect.
- **Delivery process**: the separate executable that consumes, renders, sends, retries and shuts down; it owns its broker connection, its sending port and its metrics listener.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 0 queued messages are lost: every message the consumer receives ends in a send, a deliberate dead-letter, a deliberate expiry or an unacknowledged redelivery, and none disappears silently.
- **SC-002**: 0 expired codes are delivered: no message whose lifetime has elapsed reaches the provider, and 0 expired messages reach the dead-letter route.
- **SC-003**: 0 messages are retried forever: every message that fails permanently or exhausts its attempt cap reaches the dead-letter route and is never sent again; every message that expired is acknowledged and counted instead.
- **SC-004**: 100% of send attempts end in exactly one of the four outcomes — sent, retried, dead-lettered, expired — and each outcome is counted separately.
- **SC-005**: 0 codes appear in any log line, error message, metric series or subject across the suite.
- **SC-006**: The request-serving process starts with 0 email and 0 broker variables set, and the documented required-variable list is unchanged.
- **SC-007**: The delivery process recovers from a broker outage on its own and resumes consuming without a restart.
- **SC-008**: The delivery process stops within its shutdown budget with 0 messages lost and 0 resources left open.
- **SC-009**: The worker and consumer suites are race-clean and non-vacuous; every failure path asserts its typed error.

## Assumptions

- This is the sixth feature derived from `docs/plan-auth-otp-2fa-email.md` §9. It delivers the broker consumer that stage 2 deferred (the preceding feature's research recorded it as "the broker consumer is the next feature") and that stage 6 requires as a deployed process. It is the worker half of the pipeline: the topology, the message contract, the sending port, the templates and the publisher are delivered and consumed here unchanged.
- The deployment of this process — its compose service, the broker's management UI behind the ops. gateway, the metrics scrape and the alert rules that read these counters — is the next feature, not this one. This feature ships the process, its executable and its own metrics; the stack that runs it is the next cycle's.
- The retry policy is the plan's decision O1: the delay tiers are the broker's own delay queues (a message TTL that dead-letters back to the work queue), so no broker plugin and no version-specific feature is needed, and the send-attempt cap is 5.
- The delivery guarantee is at-least-once, not exactly-once: a crash between the provider's acceptance and the acknowledgement produces a duplicate, and the message identity is the provider's idempotency key that de-duplicates it.
- The consumer needs no database: the code's lifetime travels on the message, so the expiry decision needs no lookup and the process starts without the store (clarification, 2026-10-21). The deadline it honours is the one the message states, not the challenge's: a code the person already typed may still be delivered once, and the code is single-use, so the extra email is harmless.
- The dead-letter route is for inspection: this feature ships no replay tool, and re-sending a dead code is a separate operator concern for a later cycle (clarification, 2026-10-21). An expired message is acknowledged and counted, never dead-lettered, so the route stays reserved for messages that can never succeed.
- The code travels in the clear inside the message because the request-serving process generated and hashed it; the consumer renders it, sends it and keeps no copy of it.
- The daily send cap is already enforced where the code is requested; this feature does not re-implement it, and a provider that reports its daily allowance spent arrives as a retryable outcome and is backed off, not dropped.
- The template version is deployed with the consumer: when a template changes, the process that renders it and the process that generated the message are released together.
- The request-serving process is untouched by this feature: it still never talks to the broker and never to the provider.
- The plan's finding F-14 (no graceful shutdown) applies to the request-serving process as well; this feature gives the delivery process the lifecycle it needs, and the request-serving process's own drain is not in scope.
- Requirement-ID prefix `WKR` is exclusive to this feature; `D5`, `D6`, `O1`, `O2`, `F-14` and `F-16` belong to the plan and are not requirement IDs.
