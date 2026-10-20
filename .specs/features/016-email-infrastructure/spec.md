# Feature Specification: Email Delivery Infrastructure — Provider-Agnostic Sender, Embedded Templates, Durable Outbox and Relay Publisher

**Feature Branch**: `016-email-infrastructure`

**Created**: 2026-10-20

**Status**: Draft

**Input**: User description: "Deliver the async email delivery foundation: the EmailSender port with Resend and log adapters, embedded templates, configuration, a fake sender for tests, and the Mongo outbox plus relay publisher" (plan `docs/plan-auth-otp-2fa-email.md` §9 stage 2, decisions D5, D6, O1, O2, O4)

## Clarifications

### Session 2026-10-20

- Q: In which language should the OTP email subject and both bodies (plain-text and HTML) be written? → A: English (user decision, 2026-10-20): the email copy is a user-facing string under Principle IV, so it ships in English and no constitution amendment is needed (`EML-11`, `EML-21`).
- Q: How should the embedded OTP templates be organised for the two known purposes (email verification and login code)? → A: One embedded template per purpose (user decision, 2026-10-20): each purpose selects its own template with its own subject and bodies, giving per-purpose test targets (`EML-11`, `EML-22`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A queued email is not lost when the publisher crashes (Priority: P1)

Someone has just entered their credentials and waits for a short code by email; without it they cannot finish signing in. The intention to send that code must be written durably before anyone is told the request was accepted, and a separate publisher process must move stored intentions to the message broker — confirming each one, and re-publishing anything a crash left unconfirmed. A crash, a broker outage, or a second publisher running at the same time must never silently lose the code, and a duplicate must be harmless because every intention carries a stable identity the provider can de-duplicate.

**Why this priority**: the whole 2FA program depends on the code actually arriving; a lost intention locks the user out with no recovery path, and the request-serving process must stay healthy when the broker is down.

**Independent Test**: with a recording publisher and a store (self-skipping without a database), write pending intentions, run one publish pass and assert each is published exactly once and marked published with a timestamp; refuse confirmations and assert the row stays pending with an incremented attempt and a later eligibility time; expire a lease without marking and assert a second pass republishes and the row converges to published; lease from two publishers in the same window and assert no intention is published twice; exhaust the attempt cap and assert the row is failed and never republished.

**Acceptance Scenarios**:

1. **Given** a stored pending intention, **When** the publisher runs a pass, **Then** the intention is published to the broker, the broker confirms acceptance, and the intention is marked published with a timestamp. (EML-02, EML-07)
2. **Given** a publisher whose publication is not confirmed, **When** a pass completes, **Then** the intention remains unpublished with an incremented attempt count and is not retried before its scheduled backoff. (EML-04)
3. **Given** two publishers claiming work at the same time, **When** they lease, **Then** no intention is claimed by both, and a claim held by a stopped publisher is invisible to the other until it expires. (EML-03)
4. **Given** a publisher that stopped after publishing but before marking, **When** it restarts or its expired claim is reclaimed, **Then** the intention is republished and the duplicate is harmless because its identity is stable. (EML-03, EML-18)
5. **Given** an intention that reaches the attempt cap, **When** the cap is exceeded, **Then** it is marked failed for operator attention and is never published again. (EML-04)
6. **Given** the publisher is starting, **When** the broker topology is not yet declared, **Then** the publisher declares the exchange, work queue, retry tiers and dead-letter queue idempotently before its first publish. (EML-06)

---

### User Story 2 - One sending port, a real provider, and a safe stand-in (Priority: P1)

Every part of the system that needs to send an email goes through one port. The real provider adapter delivers over HTTPS with an idempotency key and a bounded timeout; a development adapter makes no network call; a test double records what would have been sent. A send that fails is classified as retryable (rate limit, server error, timeout) or permanent (invalid recipient, unknown template) so the pipeline can decide between retrying and giving up. Templates are embedded, render both a plain-text and an HTML body carrying the code and its lifetime, and never place the code in the subject line.

**Why this priority**: this is the only path by which an email leaves the system; a wrong classification either drops a recoverable email forever or retries a permanent failure endlessly, and the code is a short-lived credential that must not leak.

**Independent Test**: render a message for each known purpose and assert the plain-text and HTML bodies carry the code and its lifetime while the subject carries no code; drive the real adapter against a stub provider and assert a rate-limit/server-error/timeout outcome is retryable and a permanent client error is permanent; assert the idempotency key is the intention identity on every send; assert the development adapter performs no network call.

**Acceptance Scenarios**:

1. **Given** a message for a known purpose, **When** it is rendered, **Then** the plain-text and HTML bodies carry the code and its lifetime and the subject carries no code. (EML-11, EML-13)
2. **Given** the real provider adapter and a provider that answers with a rate limit, a server error, or a timeout, **When** the send is attempted, **Then** the outcome is classified retryable. (EML-09, EML-12)
3. **Given** the real provider adapter and a provider that answers with a permanent client error, **When** the send is attempted, **Then** the outcome is classified permanent. (EML-12)
4. **Given** any adapter, **When** a send is attempted, **Then** the intention's stable identity is supplied as the idempotency key. (EML-09, EML-18)
5. **Given** the development adapter in a test, **When** a send is attempted, **Then** no network call is made and the send is recorded. (EML-10)
6. **Given** a message whose purpose selects no known template, **When** it is rendered, **Then** the outcome is a permanent failure and no send is attempted. (EML-11, EML-12)

---

### User Story 3 - The pipeline can be enabled without changing the required configuration (Priority: P2)

All email and broker settings are optional with documented defaults, so the request-serving process still starts when none of them is set, and the deployment's required-variable list is unchanged. The publisher validates its own broker settings at startup and fails fast with a clear message when they are absent or unusable. The broker is reachable only on the internal network.

**Why this priority**: a deploy that requires the broker or the provider for the whole API to boot turns a new, optional capability into an outage risk; the pipeline must be able to land dark and be turned on deliberately.

**Independent Test**: start the request-serving process with no email or broker variables and assert it starts and the documented required-variable list is unchanged; run the publisher with a blank and with an unparseable broker URL and assert each fails fast with a typed error and never begins polling; inspect the deployment description and assert the broker declares no published host port.

**Acceptance Scenarios**:

1. **Given** no email or broker variables are set, **When** the request-serving process starts, **Then** it starts and the documented required-variable list is unchanged. (EML-14)
2. **Given** a blank or unparseable broker URL, **When** the publisher starts, **Then** it fails fast with a typed error and does not begin polling. (EML-15)
3. **Given** the deployment description, **When** it is validated, **Then** the broker service has no published host port, has a healthcheck, and the app's required-variable list is unchanged. (EML-16)
4. **Given** a stored intention backlog, **When** it is observed, **Then** the pending count and publication outcomes by result are exposed for alerting. (EML-17, EML-23)

---

### Edge Cases

- **Publisher crash between publish and mark**: the intention is republished on the next eligible pass; the duplicate is de-duplicated by the provider because the identity is stable. (EML-03, EML-18)
- **Two publishers polling the same window**: the atomic claim gives an intention to exactly one; the other sees nothing until the claim expires. (EML-03)
- **Claim held by a publisher that never returns**: the claim expires after its lease and the intention becomes eligible again. (EML-03)
- **Broker unavailable while the publisher is already running**: publications fail, intentions stay pending with backoff and are not lost; the broker coming back drains the backlog. (EML-04)
- **Broker refuses to confirm, or the confirmation times out**: treated as a failed publication, never as a success. (EML-02, EML-04)
- **Message the broker cannot route** (no queue bound): detected through mandatory publication and treated as a failure, not a success. (EML-07)
- **Empty outbox**: a pass claims nothing, publishes nothing, and is not an error. (EML-05)
- **Attempt cap reached**: the intention is failed and surfaced; it is not retried and not silently dropped. (EML-04)
- **Intention payload that cannot produce a message** (missing/invalid recipient, unknown template): a permanent outcome, never retried forever. (EML-11, EML-12)
- **Provider rate limit** (including a spent daily free-tier allowance reported as a rate limit): retryable, so the pipeline backs off rather than failing the message. (EML-09, EML-12)
- **Provider permanent client error** (for example an invalid recipient): permanent, so the pipeline stops retrying. (EML-12)
- **Send exceeds the per-send timeout**: retryable. (EML-09, EML-12)
- **Code present in a candidate log line, error message, or subject**: prohibited by contract and caught by an assertion that scans the suite's logs, errors and rendered subjects. (EML-13)
- **Broker URL blank, malformed, or pointing at a foreign scheme**: the publisher fails fast at startup with a typed error. (EML-15)
- **Unknown purpose selects no template**: permanent failure at render time. (EML-11)
- **Published intentions older than the retention window**: eligible for retirement by a TTL index; retirement never affects pending or failed intentions. (EML-17)
- **Non-identifier intention identity on read**: rejected rather than published with a malformed idempotency key. (EML-18)

## Requirements *(mandatory)*

### Functional Requirements

**Durable intent and publication (US1)**

- **EML-01**: WHEN an email is requested, the system MUST persist a durable send intention before reporting acceptance, and the request-serving process MUST NOT contact the email provider or the message broker.
- **EML-02**: A separate publisher process MUST move stored intentions to the message broker at least once, and MUST mark an intention published only after the broker has confirmed it accepted the message for routing.
- **EML-03**: The publisher MUST claim a batch of intentions atomically so that concurrent publishers never publish the same intention, and a claim held by a stopped publisher MUST expire so the intention becomes eligible again. A duplicate publish from a crash between publish and mark MUST be harmless.
- **EML-04**: WHEN publication fails, the intention MUST remain unpublished with an incremented attempt count and a bounded, increasing backoff; WHEN the attempt cap is reached, the intention MUST be marked failed for operator attention and MUST NOT be republished.
- **EML-05**: The publisher MUST poll on a bounded interval and MUST shut down gracefully — stop claiming new work, finish in-flight publications, then release resources.
- **EML-06**: The publisher MUST declare the broker topology idempotently before publishing: a main direct exchange, a durable main work queue that dead-letters to a retry exchange, delay-tier retry queues that dead-letter back to the main exchange, and a durable dead-letter queue; the topology MUST NOT require a broker plugin or a version-specific extension.
- **EML-07**: Every published message MUST conform to the documented contract (stable identity, type, recipient, template, data, created time, attempt) and MUST be published mandatorily and persistently, so a message the broker cannot route is treated as a failure rather than a success.

**Sending (US2)**

- **EML-08**: All callers MUST send through one port; provider-specific behaviour MUST stay behind an adapter selected by configuration.
- **EML-09**: The real provider adapter MUST send over HTTPS, MUST supply a stable idempotency key per intention, MUST honour a bounded per-send timeout, and MUST map provider outcomes to the retryable/permanent classification.
- **EML-10**: The system MUST provide a development adapter that performs no network call and a test double that records sends; no unit test may invoke the provider.
- **EML-11**: Each message MUST be rendered from one embedded template per purpose (email verification and login code), selected by its purpose, producing both a plain-text and an HTML body that carry the code and its lifetime; the subject MUST NOT carry the code, and an unknown purpose MUST be a permanent failure.
- **EML-12**: A send failure MUST be classified as retryable (rate limit, server error, timeout) or permanent (invalid recipient, unknown template); the classification MUST be a typed, testable outcome at the port boundary.
- **EML-13**: The code MUST never appear in a log line, an error message, or a message subject, and logs MUST identify a message by its identity and purpose only, masking the recipient.

**Configuration, data and deployment (US3)**

- **EML-14**: All new configuration MUST come from the shared configuration surface, MUST be optional with documented defaults, and MUST NOT change the deployment's required-variable list or prevent the request-serving process from starting when absent.
- **EML-15**: Broker settings MUST be validated by the publisher at startup, failing fast with a typed error when absent or invalid; they MUST NOT be validated by the request-serving process.
- **EML-16**: The deployment MUST describe a broker service reachable only on the internal network with no published host port and a healthcheck, and the publisher as a separate process; the request-serving process MUST NOT depend on the broker to start.
- **EML-17**: The stored intentions MUST expose an idempotently created index for the pending/next-attempt queue and MUST make published intentions eligible for retirement after a documented retention window without affecting pending or failed intentions.
- **EML-18**: Every stored intention identity MUST be a canonical lowercase UUID v7 from the shared generator and MUST be used as the provider idempotency key; a read identity that is not canonical MUST be rejected rather than published.

**Cross-cutting**

- **EML-19**: Every new failure mode MUST be a typed error with an exported message and a documented effect (retry, fail the intention, or stop the publisher), and the publisher MUST act on it exactly.
- **EML-20**: Every requirement MUST ship tests — edge cases first, then one happy path — asserting the typed error on failure paths and staying clean under the race detector.
- **EML-21**: All identifiers, comments, log messages and user-facing strings MUST be written in English.
- **EML-22**: A golden-file test MUST pin the published message contract, and rendering tests MUST pin the subject and both bodies for every known purpose.
- **EML-23**: The publisher MUST expose the pending backlog and publication outcomes by result so a stalled pipeline can be alerted on.

### Key Entities

- **Send intention**: the durable record that an email must be sent — stable identity, recipient, template, payload (code, lifetime, purpose), lifecycle status, attempt count, next-eligible time, claim owner and expiry, and publication timestamps.
- **Message**: the contract-shaped payload handed to the broker — stable identity, type, recipient, template, data, created time, attempt.
- **Template**: an embedded, per-purpose pair of plain-text and HTML bodies, rendered with the code and its lifetime, with a code-free subject.
- **Send outcome**: the typed result of a send attempt — success, retryable failure, or permanent failure.
- **Broker topology**: the exchange, main work queue, delay-tier retry queues, and dead-letter queue the publisher declares and publishes through.
- **Configuration**: the optional, documented email and broker settings sourced from the shared configuration surface.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 0 stored intentions are lost across a publisher crash between publish and mark; every such intention converges to published, and its duplicate is harmless.
- **SC-002**: 0 intentions are published by two concurrent publishers within one claim window.
- **SC-003**: 100% of published messages validate against the pinned contract, and 0 messages are reported published when the broker did not confirm.
- **SC-004**: 100% of send attempts yield a retryable-or-permanent classification, and the provider is invoked by 0 unit tests.
- **SC-005**: 0 codes appear in any log line, error message, or rendered subject across the suite.
- **SC-006**: The documented required-variable list is unchanged, and the request-serving process starts successfully with 0 email and broker variables set.
- **SC-007**: The broker declares 0 published host ports and the topology requires 0 broker plugins.
- **SC-008**: The outbox and publisher suites are race-clean and non-vacuous; every failure path asserts its typed error.

## Assumptions

- This is the second of the features derived from `docs/plan-auth-otp-2fa-email.md` §9; it delivers stage 2 (the infrastructure seams) only. The consumer that reads messages from the broker, checks code expiry, and calls the sender is a later feature, and so are the challenge HTTP endpoints and the API-side enqueue. Nothing calls the sender or the outbox from a running process yet, exactly as the preceding feature shipped seams with no HTTP surface change.
- The chosen transactional provider and the chosen message broker are the plan's decisions (D5, D6). The provider stays behind the port, so switching to another provider is a configuration-plus-adapter change, not a rewrite of callers.
- The SMTP adapter is not delivered: the plan makes it conditional and only the real provider plus the development/test stand-ins are in scope.
- The retry policy is the plan's decision O1 — delay tiers of 30 seconds, 2 minutes and 5 minutes, an attempt cap of 5, a publication-confirmation timeout on the order of seconds, a claim lease of about 30 seconds, and a poll interval on the order of a second; the exact defaults are documented and configurable.
- The stored-intention retention window is the plan's suggestion of 7 days for published rows.
- The daily provider-quota guard (the request path answering an "unavailable" outcome when the free-tier ceiling is reached) belongs to the API-side feature that enqueues intentions; this feature only requires that a provider rate limit is classified retryable.
- Domain authentication (SPF/DKIM/DMARC) and bounce/complaint webhooks are operational tasks outside this code deliverable.
- The data store remains standalone: publication durability is per-document and there are no multi-document transactions; the ordering of account/challenge writes before the intention is the API-side feature's concern.
- Identifiers are canonical UUID v7 from the shared generator, and the intention identity doubles as the provider idempotency key.
- The broker runs on a single node: without high availability an outage may delay emails, but the durable intentions mean they are not lost.
- No HTTP contract changes: the request-serving routes, their payloads and their status codes are untouched by this feature.
- The email copy (subject and both bodies) is written in English, consistent with Principle IV, and there is no localisation path in this feature. One embedded template exists per purpose, and the message's purpose selects it.
- Requirement-ID prefix `EML` is exclusive to this feature; `D5`, `D6`, `O1`, `O2`, `O4`, `F-16` belong to the plan and are not requirement IDs.