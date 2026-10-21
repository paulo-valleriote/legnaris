# Tasks: Watching the Pipeline — The Alerts, the Dashboard and What to Do About Them

**Input**: Design documents from `.specs/features/023-delivery-observability/`

**Prerequisites**: `plan.md`, `spec.md` (`OBS-01`…`OBS-22`), `research.md` (`R1`…`R8`), `data-model.md`,
`contracts/`, `quickstart.md`

**Tests**: Required, not optional — the constitution makes behavior ship with tests in the same commit
(`III. Behavior Ships With Tests`). The stack's behaviour is configuration, so its test surface is the
deployment's Go suite; those checks are written **first**, must fail against the current tree, and land
in the same commit as the change they cover.

**Organization**: tasks are grouped by user story so each story is independently verifiable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: the user story the task serves (`US1`, `US2`, `US3`)
- Every task names the exact file it changes.

## Path Conventions

All paths are under `deploy/`. No application file changes: every counter this feature watches is
already exposed by the process that owns it.

---

## Phase 1: Setup

**Purpose**: confirm the change needs nothing installed and nothing new declared.

- [ ] T001 [P] Confirm no dependency is added and no application file changes: `go.mod`, `go.sum` and the `backend/` tree stay byte-identical — `go.mod`, `go.sum` (OBS-20)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the broker's metrics mode and the monitoring stack's loading of the rules. The alarms
cannot be written before the series they read exist in the shape they need.

- [ ] T002 [P] Configure the broker to expose its per-object metrics, so a rule can name a queue, and scrape the narrow query for the oldest message's age — `deploy/rabbitmq/rabbitmq.conf`, `deploy/prometheus/prometheus.yml` (OBS-02, OBS-03, R2, R3)
- [ ] T003 [P] Verify the broker's real series names against a running broker — the names are the broker's, not ours — and record them beside the rules — `deploy/prometheus/rules.yml`, `deploy/README.md` (OBS-16, R2, R3)
- [ ] T004 [P] Load the rules file in the monitoring stack's configuration — `deploy/prometheus/prometheus.yml` (OBS-19)

**Checkpoint**: the series exist in the shape the rules need; the alarms can be written.

---

## Phase 3: User Story 1 — A stalled pipeline is visible before anyone reports a missing code (Priority: P1) 🎯 MVP

**Goal**: every way the pipeline can stall has a named rule with a severity, a patience period and an
expression over the series that carry the fact.

**Independent Test**: read the rules and assert each of the eight conditions has its own named rule
with a threshold, a patience period and a severity, and that each expression names the series it
watches.

### Tests for User Story 1 (write first, ensure they FAIL) ⚠️

- [ ] T005 [P] [US1] Every alarm exists — the dead-letter queue, the missing consumer, the oldest message, the backlog, the publication, the provider, the allowance and the verification failures — each named, with a severity and a patience period — `deploy/compose_test.go` (OBS-01, OBS-02, OBS-03, OBS-04, OBS-05, OBS-06, OBS-07, OBS-08, OBS-17, SC-001, SC-002)
- [ ] T006 [P] [US1] Each expression names the series it watches, and the backlog and the publication rules read the publisher's job — `deploy/compose_test.go` (OBS-16, SC-003)
- [ ] T007 [P] [US1] The thresholds and the patience periods are configurable, not hard-coded in the expression — `deploy/compose_test.go` (OBS-03, OBS-04, SC-003)

### Implementation for User Story 1

- [ ] T008 [US1] Write the eight rules with their names, severities, patience periods, thresholds and expressions — `deploy/prometheus/rules.yml` (OBS-01, OBS-02, OBS-03, OBS-04, OBS-05, OBS-06, OBS-07, OBS-08, OBS-17, SC-001, SC-002)

**Checkpoint**: every stall has an alarm; US1 is the MVP.

---

## Phase 4: User Story 2 — The operator sees the pipeline's state at a glance (Priority: P2)

**Goal**: one provisioned screen tells the operator the shape of the pipeline, and every panel says
which series it reads.

**Independent Test**: read the dashboard and assert it covers the pipeline's stages — the codes
requested and verified, the backlog, the queue's depth, its consumers and the oldest message, the
send outcomes, the provider's latency, the expired and dead-lettered counts — and that every panel
names its series.

### Tests for User Story 2 (write first, ensure they FAIL) ⚠️

- [ ] T009 [P] [US2] The dashboard exists, is a valid document, covers the pipeline's stages, and every panel names the series it reads — `deploy/compose_test.go` (OBS-10, OBS-11, OBS-12, OBS-15, SC-005)

### Implementation for User Story 2

- [ ] T010 [US2] Write the dashboard — the codes, the backlog, the queue, the outcomes, the provider's latency, the allowance — with every panel naming its series and carrying no credential — `deploy/grafana/dashboards/email-pipeline.json` (OBS-10, OBS-11, OBS-12, OBS-15, OBS-21, SC-005, SC-006)
- [ ] T011 [US2] Prove the dashboard stays behind the operator's route and carries no credential — `deploy/traefik_test.go` (OBS-21)

**Checkpoint**: the pipeline has a screen.

---

## Phase 5: User Story 3 — Every alarm says what to do about it (Priority: P2)

**Goal**: each alarm has an entry saying what it means, what to do and when to escalate — plus the
provider's allowance and the trigger for buying more.

**Independent Test**: read the runbook and assert every rule has an entry, that each entry names an
action and an escalation condition, and that the allowance and the trigger are there.

### Tests for User Story 3 (write first, ensure they FAIL) ⚠️

- [ ] T012 [P] [US3] Every rule has a runbook entry, and each entry names what the alarm means, what to do and when to escalate — `deploy/compose_test.go` (OBS-13, SC-004)
- [ ] T013 [P] [US3] The runbook states the provider's allowance, what happens when it is spent, the trigger for buying more, and where to configure a notification channel — `deploy/compose_test.go` (OBS-14, OBS-18, SC-008)

### Implementation for User Story 3

- [ ] T014 [US3] Write the runbook section: one entry per alarm, the allowance and its trigger, where to add a notification channel, and which process owns which counter — `deploy/README.md` (OBS-13, OBS-14, OBS-16, OBS-18)

---

## Phase 6: Polish & Cross-Cutting

- [ ] T015 [P] Record the feature's decisions in the decision log — `.specs/STATE.md` (AD-026): the rules in the monitoring stack, the broker's per-object metrics, the narrow age query, the duplicated counters, the dashboard, and the channel left to the operator (OBS-09, OBS-02)
- [ ] T016 Run the gates and record the result: `make fmt && make lint && make vet && make lint-specs`, then `make test-race`, plus the monitoring stack's own rule validation; every rule name, panel title and runbook entry is English (OBS-19, OBS-20, OBS-22, SC-007, SC-007)

---

## Dependencies

```text
Phase 1 (Setup)
   ↓
Phase 2 (Foundational: the broker's metrics mode, the series names, the rules' loading)
   ↓
Phase 3 (US1 — the alarms)  ─────────┐
   ↓                                 │  US2's panels read the same series
Phase 4 (US2 — the dashboard)         │  and US3's entries answer US1's alarms
   ↓                                 │
Phase 5 (US3 — the runbook)  ←────────┘
   ↓
Phase 6 (Polish: the decision log, the gates)
```

Story completion order: **US1 → US2 → US3**. The alarms are the fact; the dashboard and the runbook
are the two ways of reading them.

## Parallel execution examples

```text
# Phase 2 — the three foundations touch three different files
T002 (the broker's config and the monitoring config) ∥ T003 (the series names) ∥ T004 (the rules' loading)

# Phase 3 — the checks are written apart from the rules they guard
T005 ∥ T006 ∥ T007 (deploy/compose_test.go) ∥ T008 (deploy/prometheus/rules.yml)

# Phases 4-5 — the checks and the runbook are independent
T009 ∥ T012 ∥ T013 (deploy/compose_test.go) ∥ T011 (deploy/traefik_test.go)
```

## Implementation strategy

1. **Phase 1–2 first**: the series must exist in the shape the rules need; the broker's metrics mode
   and the real series names come first.
2. **US1 is the MVP**: the alarms are the whole point of the stage — without them the pipeline is
   still silent.
3. **US2 second**: the dashboard reads the series the alarms already proved.
4. **US3 last**: the runbook answers the alarms as they are written.
5. **One commit per task or logical group**, the checks in the same commit as the change they cover, and
   the gates run before the branch is reported.

## Requirement coverage

| Requirement | Tasks |
|---|---|
| `OBS-01` | T005, T008 |
| `OBS-02` | T002, T005, T008, T015 |
| `OBS-03` | T002, T005, T007, T008 |
| `OBS-04` | T005, T007, T008 |
| `OBS-05` | T005, T008 |
| `OBS-06` | T005, T008 |
| `OBS-07` | T005, T008 |
| `OBS-08` | T005, T008 |
| `OBS-09` | T015 |
| `OBS-10` | T009, T010 |
| `OBS-11` | T009, T010 |
| `OBS-12` | T009, T010 |
| `OBS-13` | T012, T014 |
| `OBS-14` | T013, T014 |
| `OBS-15` | T009, T010 |
| `OBS-16` | T003, T006, T014 |
| `OBS-17` | T005, T008 |
| `OBS-18` | T013, T014 |
| `OBS-19` | T004, T016 |
| `OBS-20` | T001, T016 |
| `OBS-21` | T010, T011 |
| `OBS-22` | T016 |
| `SC-001` | T005, T008 |
| `SC-002` | T005, T008 |
| `SC-003` | T006, T007 |
| `SC-004` | T012 |
| `SC-005` | T009, T010 |
| `SC-006` | T010 |
| `SC-007` | T016, T016 |
| `SC-008` | T013 |
