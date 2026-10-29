# Tasks: The Broker Stack — The Delivery Worker Runs, and the Broker's Console Is the Operator's

**Input**: Design documents from `.specs/features/021-broker-stack/`

**Prerequisites**: `plan.md`, `spec.md` (`BRK-01`…`BRK-22`), `research.md` (`R1`…`R9`), `data-model.md`,
`contracts/`, `quickstart.md`

**Tests**: Required, not optional — the constitution makes behavior ship with tests in the same commit
(`III. Behavior Ships With Tests`). The stack's behaviour is configuration, so its test surface is the
deployment's Go suite; those checks are written **first**, must fail against the current tree, and land
in the same commit as the change they cover.

**Organization**: tasks are grouped by user story so each story is independently verifiable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: the user story the task serves (`US1`, `US2`, `US3`, `US4`)
- Every task names the exact file it changes.

## Path Conventions

All paths are under `deploy/`. The Go backend is not touched by this feature: the delivery worker
itself is the preceding feature (`.specs/features/020-email-worker/`).

---

## Phase 1: Setup

**Purpose**: confirm the change needs nothing installed and nothing new declared.

- [x] T001 [P] Confirm no dependency is added and no backend file changes: `go.mod`, `go.sum` and the `backend/` tree stay byte-identical — `go.mod`, `go.sum` (BRK-19)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the broker's configuration and its derived image — the two things that make the broker
behave differently from the stock image. The stories below all read them.

- [x] T002 [P] Add the derived image that enables the broker's Prometheus plugin at build time, with the note that explains why it is not enabled at run time — `deploy/rabbitmq/Dockerfile`, `deploy/rabbitmq/README.md` (BRK-14, R6)
- [x] T003 [P] Add the broker's configuration file with the console's path prefix and the metrics listener's own port, and the note that records the version it is written for — `deploy/rabbitmq/rabbitmq.conf` (BRK-07, R5)

**Checkpoint**: the broker's configuration and its image exist; the stories can begin.

---

## Phase 3: User Story 1 — The delivery worker runs beside the site (Priority: P1) 🎯 MVP

**Goal**: the worker is a service of its own in the stack — internal only, waiting for the broker,
carrying exactly the settings it reads — and the site never waits for either.

**Independent Test**: read the stack description and assert the worker service exists with its own
entrypoint, no published port, the broker as its only dependency and the right environment; assert the
site receives neither the broker's address nor any worker setting and does not depend on either; assert
the required-variable list is unchanged.

### Tests for User Story 1 (write first, ensure they FAIL) ⚠️

- [x] T004 [P] [US1] The worker's service: it exists, it is built from the shared image, it starts on its own entrypoint, it publishes no host port, it depends on the broker and not on the database — `deploy/compose_test.go` (BRK-01, BRK-02, BRK-03, SC-001)
- [x] T005 [P] [US1] The worker's environment carries the broker's address, the provider settings and the worker's own settings, and the site's environment carries neither the broker's address nor any worker setting — `deploy/compose_test.go` (BRK-04, BRK-21, SC-005)
- [x] T006 [P] [US1] The site depends on neither the broker nor the worker, and the stack's required-variable list is unchanged — `deploy/compose_test.go` (BRK-03, BRK-20, BRK-21, SC-005, SC-008)

### Implementation for User Story 1

- [x] T007 [US1] Add the worker service to the stack: the shared image, the worker entrypoint, the internal network, the broker's health check as its only dependency, the restart policy that brings it back after a failure, and the settings it reads — `deploy/docker-compose.yml` (BRK-01, BRK-02, BRK-03, BRK-04, BRK-20, SC-001, SC-008)
- [x] T008 [US1] Point the publisher's and the worker's broker address at the delivery vhost, and document the new and changed variables — `deploy/.env.example`, `deploy/docker-compose.yml` (BRK-04, R3)

**Checkpoint**: the worker runs in the stack; US1 is the MVP.

---

## Phase 4: User Story 2 — The broker's console is the operator's (Priority: P1)

**Goal**: the console is served under its own prefix on the operator's domain, behind the same gate
and the same headers as the other operator pages, and it is never published on a host port.

**Independent Test**: read the edge configuration and assert the console's route, its rule, its
service and its middlewares; assert the broker publishes no host port; assert the broker's version is
exact.

### Tests for User Story 2 (write first, ensure they FAIL) ⚠️

- [x] T009 [P] [US2] The console's route: the operator's domain and the console's own prefix, the broker's service, the sign-in gate and the security headers in that order, and the operator's other routes unchanged — `deploy/traefik_test.go` (BRK-07, BRK-08, SC-002)
- [x] T010 [P] [US2] The broker's exposure and its version: no published host port, a health check declared, and a base image that names an exact 4.x version wherever the pin sits — `deploy/compose_test.go` (BRK-05, BRK-06, SC-002, SC-006)

### Implementation for User Story 2

- [x] T011 [US2] Add the console's route to the edge configuration, with the sign-in gate and the security headers — `deploy/traefik/dynamic.yml` (BRK-07, BRK-08, SC-002)
- [x] T012 [US2] Pin the broker's base image to an exact 4.x version, keep it on the internal network only (Traefik already reaches it there; see AD-024), mount its configuration file, build it from the derived image and keep its durable volume unchanged so the queued messages survive a broker restart — `deploy/docker-compose.yml` (BRK-05, BRK-06, BRK-14, BRK-18, R5, SC-002, SC-006)
- [x] T013 [US2] Document the console's address, what it shows and that it carries the queued codes — `deploy/README.md` (BRK-16, BRK-17)

**Checkpoint**: the console is the operator's and nobody else's.

---

## Phase 5: User Story 3 — The broker's accounts are separated by role (Priority: P1)

**Goal**: the application's account can move codes and can never open the console; the operator's
account is a distinct one that can; no credential is committed.

**Independent Test**: read the runbook and assert the provisioning steps are there, in order, with the
passwords read from the environment; assert the operator's password reaches no service; assert the
runbook states that the pipeline waits for the provisioning.

### Tests for User Story 3 (write first, ensure they FAIL) ⚠️

- [x] T014 [P] [US3] The runbook carries the provisioning steps — the delivery vhost, the operator's account, the application's rights — and states that the pipeline waits for them and that the steps are re-runnable — `deploy/compose_test.go` (BRK-09, BRK-10, BRK-16, SC-003)
- [x] T015 [P] [US3] The operator's password reaches no service and is not part of the stack's required-variable list — `deploy/compose_test.go` (BRK-11, BRK-21, SC-004, SC-005)

### Implementation for User Story 3

- [x] T016 [US3] Write the provisioning section: the steps in order, the idempotency, the passwords from the environment, and the consequence of an unprovisioned broker — `deploy/README.md` (BRK-09, BRK-10, BRK-11, BRK-16, R2, SC-003, SC-004)
- [x] T017 [US3] Add the operator's password to the deployment's example environment as an operator-only value, with the note that it never reaches a service — `deploy/.env.example` (BRK-11, BRK-21, SC-004, SC-005)

**Checkpoint**: the two roles are separated and the separation is documented.

---

## Phase 6: User Story 4 — The broker's state can be read by the monitoring stack (Priority: P2)

**Goal**: the broker's own numbers and the worker's counters reach the monitoring stack from inside the
machine.

**Independent Test**: read the monitoring configuration and assert the broker's and the worker's
targets are scraped, and that the broker's metrics port is not published.

### Tests for User Story 4 (write first, ensure they FAIL) ⚠️

- [x] T018 [P] [US4] The monitoring configuration scrapes the broker's Prometheus endpoint and the worker's listener, beside the four it already scrapes — `deploy/compose_test.go` (BRK-12, BRK-13, SC-007)

### Implementation for User Story 4

- [x] T019 [US4] Add the two scrape targets to the monitoring configuration — `deploy/prometheus/prometheus.yml` (BRK-13, R7, SC-007)

**Checkpoint**: the pipeline's numbers are reachable; the alerts that read them are the next feature.

---

## Phase 7: Polish & Cross-Cutting

- [x] T020 [P] Record the feature's decisions in the decision log — `.specs/STATE.md` (AD-024): the worker's place in the stack, the operator-command provisioning, the named vhost, the console's route, the pinned version and the build-time plugin (BRK-01, BRK-09, BRK-10)
- [x] T021 Run the gates and record the result: `make fmt && make lint && make vet && make lint-specs`, then `make test-race`, and the stack's interpolation gate over the base and the local override — every variable this feature adds must interpolate from `.env.example` (BRK-15, BRK-22, SC-009, SC-010)

---

## Dependencies

```text
Phase 1 (Setup)
   ↓
Phase 2 (Foundational: the broker's configuration and its derived image)
   ↓
Phase 3 (US1 — the worker in the stack)  ─────────┐
   ↓                                             │  US2's route serves the broker the
Phase 4 (US2 — the console behind the gate)      │  US1's worker and the publisher need
   ↓                                             │
Phase 5 (US3 — the accounts separated)            │
   ↓                                             │
Phase 6 (US4 — the scrape targets)  ←────────────┘
   ↓
Phase 7 (Polish: the decision log, the gates)
```

Story completion order: **US1 → US2 → US3 → US4**. US2 needs the broker's configuration (Phase 2) and
the broker's service shape (US1's compose edit); US3 needs the broker's addresses that US1 changed;
US4 needs the worker's listener that US1 deployed.

## Parallel execution examples

```text
# Phase 2 — the two artefacts are independent files
T002 (deploy/rabbitmq/Dockerfile) ∥ T003 (deploy/rabbitmq/rabbitmq.conf)

# Phase 3 — the checks and the implementation are written apart, the checks first
T004 ∥ T005 ∥ T006 (deploy/compose_test.go)

# Phases 4-6 — the two suites are independent files
T009 ∥ T010 ∥ T014 ∥ T015 ∥ T018 (deploy/compose_test.go and deploy/traefik_test.go)
```

## Implementation strategy

1. **Phase 1–2 first**: the broker's configuration and its derived image are small and everything else
   reads them.
2. **US1 is the MVP**: the worker running in the stack is the whole point of the feature — without it
   the pipeline stops at the queue.
3. **US2 second**: the console's route only matters once there is a console to reach; it changes no
   delivery behaviour.
4. **US3 third**: the accounts are provisioned in the same deploy that brings the console, and the
   runbook is the operator's guide for both.
5. **US4 last**: the scrape targets read what the earlier stories deploy.
6. **One commit per task or logical group**, the checks in the same commit as the change they cover, and
   the gates run before the branch is reported.

## Requirement coverage

| Requirement | Tasks |
|---|---|
| `BRK-01` | T004, T007, T020 |
| `BRK-02` | T004, T007 |
| `BRK-03` | T004, T006, T007 |
| `BRK-04` | T005, T007, T008 |
| `BRK-05` | T010, T012 |
| `BRK-06` | T010, T012 |
| `BRK-07` | T003, T009, T011 |
| `BRK-08` | T009, T011 |
| `BRK-09` | T014, T016, T020 |
| `BRK-10` | T014, T016, T020 |
| `BRK-11` | T015, T016, T017 |
| `BRK-12` | T018 |
| `BRK-13` | T018, T019 |
| `BRK-14` | T002, T012 |
| `BRK-15` | T021 |
| `BRK-16` | T013, T014, T016 |
| `BRK-17` | T013 |
| `BRK-18` | T012 |
| `BRK-19` | T001 |
| `BRK-20` | T006, T007 |
| `BRK-21` | T005, T006, T015, T017 |
| `BRK-22` | T021 |
| `SC-001` | T004, T007 |
| `SC-002` | T009, T010, T011, T012 |
| `SC-003` | T014, T016 |
| `SC-004` | T015, T016, T017 |
| `SC-005` | T005, T006, T015, T017 |
| `SC-006` | T010, T012 |
| `SC-007` | T018, T019 |
| `SC-008` | T006, T007 |
| `SC-009` | T021 |
| `SC-010` | T021 |
