# Tasks: The Email Worker — One Process That Turns a Queued Code into a Delivered Email

**Input**: Design documents from `.specs/features/020-email-worker/`

**Prerequisites**: `plan.md`, `spec.md` (`WKR-01`…`WKR-27`), `research.md` (`R1`…`R14`),
`data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required, not optional — the constitution makes behavior ship with tests in the same commit
(`III. Behavior Ships With Tests`). Every story's tests are written **first**, must fail against the
current tree, and land in the same commit as the implementation they cover.

**Organization**: tasks are grouped by user story so each story is independently verifiable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: the user story the task serves (`US1`, `US2`, `US3`, `US4`)
- Every task names the exact file it changes.

## Path Conventions

All paths are under `backend/` (the Go backend). The `web/` client is not touched by this feature, and
the deployment stack is not touched either: the compose service, the broker's management UI and the
alert rules belong to the next feature.

---

## Phase 1: Setup

**Purpose**: confirm the change needs nothing installed and nothing new declared.

- [ ] T001 [P] Confirm no dependency is added: `go.mod` and `go.sum` stay byte-identical — the pinned `amqp091-go` v1.16.0 already carries the consumer surface this feature needs (`research.md` R1, R7)
- [ ] T002 [P] Confirm the deployment stack is untouched: `deploy/docker-compose.yml`, `deploy/.env.example`, `deploy/compose_test.go` and `deploy/prometheus/prometheus.yml` stay byte-identical, and `env.RequiredKeys()` is unchanged (WKR-19)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the shared vocabulary, the decoder, the settings, the ports and the counters every story
consumes. They land first — mirroring how the delivered features put a shared seam before the stories
that build on it.

- [ ] T003 [P] Add the `Destination` type and its four names — `retry_30s`, `retry_2m`, `retry_5m`, `dead_letter` — with the test that pins them; the names are domain words, never routing keys — `backend/internal/email/destination.go`, `backend/internal/email/destination_test.go` (WKR-09, WKR-11, WKR-12, R3)
- [ ] T004 [P] Add the bounded byte-slice decoder that tolerates unknown fields, with the test that pins the bound, the tolerance and the typed error; `RequiredKeys` and the HTTP decoder stay untouched — `backend/shared/json/decode.go`, `backend/shared/json/decode_test.go` (WKR-04, R10)
- [ ] T005 [P] Add the three optional worker settings with their documented defaults — `EMAIL_WORKER_PREFETCH` (10), `EMAIL_WORKER_MAX_ATTEMPTS` (5), `EMAIL_WORKER_METRICS_ADDR` (`:9101`) — and the tests that pin the absent, blank, valid and malformed cases; `env.RequiredKeys()` stays exactly what it is — `backend/shared/env/env.go`, `backend/shared/env/env_test.go` (WKR-14, WKR-16, WKR-19, WKR-20, R12)
- [ ] T006 [P] Add the worker's two ports — the delivery source with its acknowledge/reject handle, and the republisher that takes a destination — plus the typed sentinels with their documented effects; generate the mocks with `make mocks` — `backend/internal/email/worker/ports.go`, `backend/internal/email/worker/errors.go`, `backend/internal/email/worker/ports_test.go`, `backend/internal/email/worker/mocks/` (WKR-02, WKR-13, WKR-23, R2)
- [ ] T007 [P] Extend the metrics package with the send-outcome counter (`sent`, `retried`, `dead_lettered`, `expired`), the send-duration histogram and the expired counter, reusing the dead-letter counter the earlier feature declared; the tests pin the labels and assert no series carries a code or an address — `backend/internal/platform/metrics/metrics.go`, `backend/internal/platform/metrics/metrics_test.go` (WKR-21, R11)

**Checkpoint**: the vocabulary, the decoder, the settings, the ports and the counters are in place;
the stories can begin.

---

## Phase 3: User Story 1 — The queued code arrives, once, and never stale (Priority: P1) 🎯 MVP

**Goal**: a queued message is decoded, validated, decided against its own deadline, rendered and sent;
the acknowledgement comes last; a message that cannot be used is dead-lettered instead of retried
forever; an expired one is dropped without being sent.

**Independent Test**: drive the consumer with a recorded broker and a recording sender: a valid
message reaches the sending port with the rendered subject, both bodies and the message identity as
the idempotency key, and is acknowledged only after the send succeeded; an expired message is
acknowledged with nothing sent and the expired counter moved; an undecodable body, a non-canonical
identity, a missing code and an unknown purpose are dead-lettered and acknowledged; two consumers over
one fake deliver each message once.

### Tests for User Story 1 (write first, ensure they FAIL) ⚠️

- [ ] T008 [P] [US1] The decode-and-validate matrix — the undecodable body, the unknown field, the non-canonical identity, the missing code, the wrong type, the unknown purpose, the zero lifetime, the future creation time and the negative attempt count — each asserting the dead-letter, the acknowledgement and that nothing was sent — `backend/internal/email/worker/worker_test.go` (WKR-04, WKR-05, WKR-07)
- [ ] T009 [P] [US1] The expiry decision against the injected clock: past the deadline nothing is sent, the message is acknowledged and the expired counter moves; before the deadline the message is sent (WKR-03)
- [ ] T010 [P] [US1] The render-and-send path: the subject, the plain-text body, the HTML body and the identity as the provider idempotency key reach the sending port, and the acknowledgement happens only after the send succeeded — `backend/internal/email/worker/worker_test.go` (WKR-02, WKR-06, WKR-08)
- [ ] T011 [P] [US1] Two consumers over one fake delivery source deliver each message once, and a redelivered message is sent again with the same idempotency key — `backend/internal/email/worker/worker_test.go` (WKR-08, WKR-16)

### Implementation for User Story 1

- [ ] T012 [US1] Implement the decode, the field validation and the deadline decision over the message's own creation time and advertised lifetime; the challenge store is never consulted — `backend/internal/email/worker/worker.go` (WKR-03, WKR-04, WKR-05, WKR-07)
- [ ] T013 [US1] Implement the render-and-send path through the delivered renderer and sending port, and the acknowledgement rules — acknowledge after a successful send, after a deliberate dead-letter, after a deliberate expiry, never before — `backend/internal/email/worker/worker.go` (WKR-02, WKR-06, WKR-08)
- [ ] T014 [US1] Add the broker's delivery source: the consumer channel of its own, the prefetch applied before the consumer starts, the manual acknowledgement, the delivery stream and the acknowledge/reject handle — `backend/internal/email/amqp/consuming.go`, `backend/internal/email/amqp/channel.go`, `backend/internal/email/amqp/consuming_test.go` (WKR-02, WKR-16)
- [ ] T015 [US1] Add the consume loop over the delivery source: one delivery at a time, the worker's own clock and configuration, and the handler bound — `backend/internal/email/worker/worker.go`, `backend/internal/email/worker/config.go` (WKR-01, WKR-02, WKR-16)

**Checkpoint**: US1 is exercisable end to end against a fake broker and a fake sender; US1 is the MVP.

---

## Phase 4: User Story 2 — A provider outage delays the code, it never loses it (Priority: P1)

**Goal**: a retryable failure is put aside in the broker's own delay tier and tried again later, a
permanent failure and an exhausted attempt budget are parked where an operator can see them, and
nothing is ever dropped: an unconfirmed republish leaves the message with the broker.

**Independent Test**: drive the consumer with a sender that fails retryably and assert the message is
republished to the 30-second tier with the attempt count incremented and the original acknowledged only
after the republish was confirmed; fail again and assert the 2-minute and then the 5-minute tiers;
refuse the republish and assert the original stays unacknowledged; exhaust the cap and assert the
message reaches the dead-letter route without another send; fail permanently and assert the same at
once.

### Tests for User Story 2 (write first, ensure they FAIL) ⚠️

- [ ] T016 [P] [US2] The retry ladder: the first failure uses the 30-second tier, the second the 2-minute tier, the later ones the 5-minute tier, and the republished message keeps its identity, recipient, template, data and creation time with only the attempt count changed — `backend/internal/email/worker/retry_test.go` (WKR-09, WKR-14, WKR-15)
- [ ] T017 [P] [US2] The terminal paths: the permanent failure, the exhausted attempt cap and the attempt count already at the cap — each reaching the dead-letter route, acknowledged and counted, with no second send — `backend/internal/email/worker/retry_test.go` (WKR-11, WKR-12)
- [ ] T018 [P] [US2] The unconfirmed republish and the unconfirmed dead-letter publish: the original stays unacknowledged in both cases, so the broker redelivers it — `backend/internal/email/worker/retry_test.go` (WKR-13)
- [ ] T019 [P] [US2] The destination mapping and the routed publish in the broker adapter: each destination reaches its exchange and routing key, the publish is mandatory and confirmed, and an unroutable message is a failure — `backend/internal/email/amqp/consuming_test.go`, `backend/internal/email/amqp/topology_test.go` (WKR-09, WKR-11, R3)

### Implementation for User Story 2

- [ ] T020 [US2] Implement the retry ladder and the attempt cap over the message's own attempt count; the broker's dead-letter history is never consulted — `backend/internal/email/worker/retry.go` (WKR-09, WKR-10, WKR-12, WKR-14)
- [ ] T021 [US2] Add the routed, confirmed and mandatory publish to the broker adapter and the destination mapping it owns — `backend/internal/email/amqp/broker.go` (WKR-09, WKR-11, WKR-13)
- [ ] T022 [US2] Wire the worker's outcomes to the retry, the dead-letter and the expired paths, and the acknowledgement that follows each — `backend/internal/email/worker/worker.go` (WKR-02, WKR-03, WKR-11, WKR-12, WKR-13)

**Checkpoint**: US1 and US2 both work independently; a provider outage delays a code instead of losing
it.

---

## Phase 5: User Story 3 — The delivery process stands on its own and survives disruption (Priority: P1)

**Goal**: the process is its own executable with its own lifecycle: it refuses to start on a
configuration that cannot work, it re-registers itself when the broker comes back, it stops
gracefully, and it never needs the database.

**Independent Test**: start the process with a blank broker address, with a malformed one and with a
provider the configuration does not name, and assert each fails fast with a typed error and consumes
nothing; end the delivery stream and assert the consumer re-registers and consumes again; ask the
process to stop and assert it takes no new message, finishes the one in flight, leaves it
unacknowledged and releases its resources.

### Tests for User Story 3 (write first, ensure they FAIL) ⚠️

- [ ] T023 [P] [US3] The fail-fast paths — the blank broker address, the malformed address, the unknown provider, the missing provider key — each stopping the process before it consumes anything, with the typed error — `backend/cmd/worker/main_test.go` (WKR-19)
- [ ] T024 [P] [US3] The graceful stop: no new message is taken, the message in flight finishes, an unfinished message is left unacknowledged and the resources are released — `backend/cmd/worker/main_test.go` (WKR-18)
- [ ] T025 [P] [US3] The stream that ends and the consumer that re-registers — `backend/internal/email/amqp/consuming_test.go`, `backend/internal/email/worker/worker_test.go` (WKR-17)

### Implementation for User Story 3

- [ ] T026 [US3] Add the supervisor loop: when the delivery stream ends the consumer is registered again and consumption continues, with the broker outage logged rather than fatal — `backend/internal/email/worker/worker.go` (WKR-17)
- [ ] T027 [US3] Wire the process — the broker validation, the provider selection through the delivered factory, the renderer, the metrics listener, the consume loop and the stop signal — `backend/cmd/worker/main.go` (WKR-01, WKR-18, WKR-19, WKR-20)
- [ ] T028 [US3] Build the worker binary into the runtime image beside `/app` and `/relay`, and add the Makefile target that runs it — `Dockerfile`, `Makefile` (WKR-26)

**Checkpoint**: the process stands on its own; the stack that runs it is the next feature.

---

## Phase 6: User Story 4 — The pipeline says what it is doing (Priority: P2)

**Goal**: every outcome is counted separately, the provider's latency is observed, and neither a log
line nor a series carries the code or a full address.

**Independent Test**: run the consumer through a send that succeeded, a send that failed retryably, a
dead-letter and an expired message, and assert each outcome is counted separately and the send
duration is observed; scan every produced series and every log line and assert no code and no full
address appears in any of them.

### Tests for User Story 4 (write first, ensure they FAIL) ⚠️

- [ ] T029 [P] [US4] The counters per outcome and the duration histogram — `backend/internal/platform/metrics/metrics_test.go` (WKR-21)
- [ ] T030 [P] [US4] The logs: a message is identified by its identity and its purpose, the recipient is masked as the first character of the local part plus the domain, and the code appears in no attribute and no series — `backend/internal/email/worker/worker_test.go` (WKR-06, WKR-21, WKR-22)

### Implementation for User Story 4

- [ ] T031 [US4] Report every outcome to the metrics observer and mask the recipient in every log line — `backend/internal/email/worker/worker.go`, `backend/internal/platform/metrics/metrics.go` (WKR-21, WKR-22)

**Checkpoint**: the pipeline is observable; the alert rules that read these counters are the next
feature.

---

## Phase 7: Polish & Cross-Cutting

- [ ] T032 [P] Record the feature's decisions in the decision log — `.specs/STATE.md` (AD-023): the package layout, the two worker ports, the domain-owned destination vocabulary, the message-local expiry decision, the retry ladder, the library-plus-supervisor reconnection and the decode helper (WKR-01, WKR-07, WKR-09, WKR-23)
- [ ] T033 [P] Extend the shared mock configuration with the worker's two ports — `.mockery.yaml` (WKR-23)
- [ ] T034 Run the gates and record the result: `make fmt && make lint && make vet && make lint-specs`, then `make test-race`; the race detector must stay clean and no test may need a broker or a provider (WKR-24, WKR-27, SC-009)

---

## Dependencies

```text
Phase 1 (Setup)
   ↓
Phase 2 (Foundational: destination, decoder, settings, ports, counters)
   ↓
Phase 3 (US1 — the delivery policy)  ──────────┐
   ↓                                          │
Phase 4 (US2 — the retry ladder)              │  US2 consumes the delivery
   ↓                                          │  decision of US1
Phase 5 (US3 — the process lifecycle)          │
   ↓                                          │
Phase 6 (US4 — the counters)  ←──────────────┘
   ↓
Phase 7 (Polish: the decision log, the mocks, the gates)
```

Story completion order: **US1 → US2 → US3 → US4**. US2 needs US1's outcomes (a retry is one outcome
of a delivery); US3 needs US1's consume loop and US2's republish (the process owns both); US4 needs
the outcomes of US1 and US2 to count them.

## Parallel execution examples

```text
# Phase 2 — the five foundations touch five different packages
T003 (internal/email/destination.go) ∥ T004 (shared/json/decode.go) ∥ T005 (shared/env/env.go)
∥ T006 (internal/email/worker/ports.go) ∥ T007 (internal/platform/metrics/metrics.go)

# Phase 3 — the tests are written first and independently of each other
T008 ∥ T009 ∥ T010 ∥ T011   (all in backend/internal/email/worker/worker_test.go, as separate suites)

# Phase 4 — the retry tests and the adapter tests are independent
T016 ∥ T017 ∥ T018 (backend/internal/email/worker/retry_test.go) ∥ T019 (backend/internal/email/amqp/)

# Phase 5 — the process tests and the adapter tests are independent
T023 ∥ T024 (backend/cmd/worker/main_test.go) ∥ T025 (the amqp and worker suites)

# Phase 7 — the record-keeping tasks are independent of each other
T032 (.specs/STATE.md) ∥ T033 (.mockery.yaml)
```

## Implementation strategy

1. **Phase 1–2 first**: the foundations are small, mechanical and unblock everything else. The
   destination vocabulary, the decoder, the settings, the ports and the counters land first, each with
   its own test, so the stories below have something to build on.
2. **US1 is the MVP**: the delivery policy — decode, validate, decide, render, send, acknowledge — is
   the whole value of the feature and it is provable with two fakes and no broker. A pipeline that
   delivers is worth more than a pipeline that retries perfectly.
3. **US2 second**: the retry ladder only means something once a delivery can fail, and it changes no
   part of US1's happy path.
4. **US3 third**: the process is a wrapper around what US1 and US2 already do; its lifecycle is
   worth the effort once there is a real delivery to keep alive.
5. **US4 last**: the counters read the outcomes the earlier stories produce; nothing else depends on
   them, and the alert rules that consume them are the next feature.
6. **One commit per task or logical group**, tests in the same commit as the behavior they cover, and
   the gates run before the branch is reported.

## Requirement coverage

| Requirement | Tasks |
|---|---|
| `WKR-01` | T015, T027 |
| `WKR-02` | T006, T010, T013, T014, T015, T022 |
| `WKR-03` | T009, T012, T022 |
| `WKR-04` | T004, T008, T012 |
| `WKR-05` | T008, T012 |
| `WKR-06` | T010, T030, T031 |
| `WKR-07` | T008, T009, T012, T032 |
| `WKR-08` | T010, T011, T013 |
| `WKR-09` | T003, T016, T019, T020, T021, T032 |
| `WKR-10` | T020, T032 |
| `WKR-11` | T003, T017, T019, T021, T022 |
| `WKR-12` | T003, T017, T020, T022 |
| `WKR-13` | T006, T018, T021, T022 |
| `WKR-14` | T005, T016, T020 |
| `WKR-15` | T016, T020 |
| `WKR-16` | T005, T011, T014, T015 |
| `WKR-17` | T025, T026, T032 |
| `WKR-18` | T024, T027 |
| `WKR-19` | T002, T005, T023, T027 |
| `WKR-20` | T005, T027 |
| `WKR-21` | T007, T029, T030, T031 |
| `WKR-22` | T030, T031 |
| `WKR-23` | T006, T032, T033 |
| `WKR-24` | T034 |
| `WKR-25` | every task (English-only source, Principle IV) |
| `WKR-26` | T028 |
| `WKR-27` | T034 |
