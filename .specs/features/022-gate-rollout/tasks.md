# Tasks: The Rollout — Turning the Rule On in Two Steps, and Knowing the Way Back

**Input**: Design documents from `.specs/features/022-gate-rollout/`

**Prerequisites**: `plan.md`, `spec.md` (`ROL-01`…`ROL-22`), `research.md` (`R1`…`R8`), `data-model.md`,
`contracts/`, `quickstart.md`

**Tests**: Required, not optional — the constitution makes behavior ship with tests in the same commit
(`III. Behavior Ships With Tests`). The tests are written **first**, must fail against the current
tree, and land in the same commit as the implementation they cover.

**Organization**: tasks are grouped by user story so each story is independently verifiable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on an incomplete task)
- **[Story]**: the user story the task serves (`US1`, `US2`, `US3`)
- Every task names the exact file it changes.

## Path Conventions

All paths are under `backend/` except the deployment's runbook and the corpus. The web client is not
touched by this feature: the two answers the setting produces are the two the client already handles.

---

## Phase 1: Setup

**Purpose**: confirm the change needs nothing installed and nothing new declared.

- [ ] T001 [P] Confirm no dependency is added and no client file changes: `go.mod`, `go.sum` and the `web/` tree stay byte-identical (ROL-21, ROL-22)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the mode, its strict parse and the configuration surface that carries the raw value. The
stories below all read them.

- [ ] T002 [P] Add the mode with its three states, the strict parse and the typed error for a value nobody names — `backend/internal/auth/verification_mode.go` (ROL-01, ROL-06, ROL-19)
- [ ] T003 [P] Add the mode's suite: every state, the absent and blank values, the lower-case spelling, the unknown value and the typed error — `backend/internal/auth/verification_mode_test.go` (ROL-01, ROL-06)
- [ ] T004 [P] Read the setting as a raw value with the documented `off` default, and keep the required-variable list unchanged — `backend/shared/env/env.go`, `backend/shared/env/env_test.go` (ROL-01, ROL-17)

**Checkpoint**: the mode and its configuration surface are in place; the stories can begin.

---

## Phase 3: User Story 1 — The rule can be turned on in two steps (Priority: P1) 🎯 MVP

**Goal**: a registration asks for a code while a sign-in still establishes a session, and the last
state asks for a code in both — with an unknown value stopping the process rather than guessing.

**Independent Test**: drive a registration, a sign-in and a renewal in each of the three states and
assert what each answers; assert an unknown value stops the boot; assert the client learns nothing new.

### Tests for User Story 1 (write first, ensure they FAIL) ⚠️

- [ ] T005 [P] [US1] The three states drive a registration, a sign-in and a renewal, and the answers are the two the client already knows — `backend/internal/auth/service_test.go` (ROL-02, ROL-03, ROL-04, SC-002, SC-005, SC-006)
- [ ] T006 [P] [US1] The boundary between the states: the renewal is renewed in the `register` state and refused in the `all` state; an account that verified is never refused in any state — `backend/internal/auth/service_test.go` (ROL-05, ROL-09, SC-004)
- [ ] T007 [P] [US1] The boot refuses an unknown value with the typed error and records the state it started with — `backend/internal/app/bootstrap_test.go` (ROL-06, ROL-08, ROL-19, SC-001)

### Implementation for User Story 1

- [ ] T008 [US1] Replace the boolean with the mode in the sign-in service's configuration and in its three decision points — the registration, the sign-in and the renewal — `backend/internal/auth/service.go`, `backend/internal/auth/start.go` (ROL-02, ROL-03, ROL-05, ROL-07, ROL-09)
- [ ] T009 [US1] Parse the raw value strictly at the boot, refuse to start on a value nobody names, and record the state in force in the log — `backend/internal/app/bootstrap.go` (ROL-06, ROL-07, ROL-08)

**Checkpoint**: the rule can be in any of its three states; US1 is the MVP.

---

## Phase 4: User Story 2 — The cutover contains the sessions issued before it (Priority: P1)

**Goal**: the sessions issued before the cutover die at their first renewal without an operator
touching anything, and the person holding one is let in again through the code step.

**Independent Test**: present a refresh credential of an account that never verified in the last state
and assert the refusal and the family revocation; present the credential of an account that verified
and assert it renews; assert the store needs no cleanup and that the runbook says why.

### Tests for User Story 2 (write first, ensure they FAIL) ⚠️

- [ ] T010 [P] [US2] The refusal in the last state revokes the whole family, so the session ends on every device at once, and the account that verified renews — `backend/internal/auth/service_test.go` (ROL-09, ROL-10, SC-003)
- [ ] T011 [P] [US2] The deployment carries no command that cleans the stored sessions, and the runbook states why none is needed — `deploy/compose_test.go` (ROL-11, ROL-12, SC-003)

### Implementation for User Story 2

- [ ] T012 [US2] State in the runbook what the cutover is exact about — the renewal — and what it is not — the access credential's remaining life — `deploy/README.md` (ROL-13)

**Checkpoint**: the cutover contains itself; the operator has nothing to clean.

---

## Phase 5: User Story 3 — The operator knows the order, what to watch and the way back (Priority: P2)

**Goal**: the runbook says when to flip, what to look at between the flips, what each flip does to
each kind of account, and how to go back.

**Independent Test**: read the runbook and assert the order, the counters, the window, the
consequences of each flip and the way back are all there.

### Tests for User Story 3 (write first, ensure they FAIL) ⚠️

- [ ] T013 [P] [US3] The runbook carries the order of the two steps, the counters to watch, the window to wait and the way back — `deploy/compose_test.go` (ROL-14, SC-007)
- [ ] T014 [P] [US3] The runbook states the consequences of each flip — the accounts that verified, the accounts that did not and the code steps in flight — and that going back undoes nothing that was verified — `deploy/compose_test.go` (ROL-15, ROL-16)

### Implementation for User Story 3

- [ ] T015 [US3] Write the runbook section: the order, the counters, the window, the consequences of each flip, the way back, and the two things the operator must not do — clean the store, or wait for a number that only the alerts will give — `deploy/README.md` (ROL-14, ROL-15, ROL-16, SC-007, SC-008)
- [ ] T016 [US3] Document the setting's three values in the deployment's example environment and in the stack's own default, with what each does — `deploy/.env.example`, `deploy/docker-compose.yml` (ROL-01, ROL-17, SC-001)

---

## Phase 6: Polish & Cross-Cutting

- [ ] T017 [P] Mark the delivered feature's setting as amended and point it at this one, so the corpus carries one contract for the setting — `.specs/features/018-auth-2fa-session-gate/spec.md` (ROL-18)
- [ ] T018 [P] Record the feature's decisions in the decision log — `.specs/STATE.md` (AD-025): the mode over two booleans, the strict parse, the state in the log, and the decision not to deliver a cleanup command (ROL-01, ROL-06, ROL-11)
- [ ] T019 Run the gates and record the result: `make fmt && make lint && make vet && make lint-specs`, then `make test-race`; every identifier, comment and string the change adds is English (ROL-20, ROL-21, SC-009, SC-009)

---

## Dependencies

```text
Phase 1 (Setup)
   ↓
Phase 2 (Foundational: the mode, its parse, the configuration surface)
   ↓
Phase 3 (US1 — the three states)  ─────────┐
   ↓                                    │  US2 and US3 describe what the
Phase 4 (US2 — the cutover's containment) │  states mean to an operator
   ↓                                    │
Phase 5 (US3 — the operator's guide)  ────┘
   ↓
Phase 6 (Polish: the corpus, the decision log, the gates)
```

Story completion order: **US1 → US2 → US3**. US2 and US3 both describe what the states mean to an
operator, so they read the mode US1 delivers.

## Parallel execution examples

```text
# Phase 2 — the three foundations touch three different files
T002 (internal/auth/verification_mode.go) ∥ T003 (its suite) ∥ T004 (shared/env)

# Phase 3 — the tests are written first and independently
T005 ∥ T006 (internal/auth/service_test.go) ∥ T007 (internal/app/bootstrap_test.go)

# Phase 6 — the record-keeping tasks are independent
T017 (.specs/features/018-.../spec.md) ∥ T018 (.specs/STATE.md)
```

## Implementation strategy

1. **Phase 1–2 first**: the mode and its parse are small and everything else reads them.
2. **US1 is the MVP**: the three states are the whole mechanism; without them there is no staged
   activation to describe.
3. **US2 second**: it names what the states mean for the sessions that already exist.
4. **US3 last**: the runbook is written once the states are fixed.
5. **One commit per task or logical group**, the tests in the same commit as the behaviour they cover,
   and the gates run before the branch is reported.

## Requirement coverage

| Requirement | Tasks |
|---|---|
| `ROL-01` | T002, T003, T004, T016, T018 |
| `ROL-02` | T005, T008 |
| `ROL-03` | T005, T008 |
| `ROL-04` | T005 |
| `ROL-05` | T006, T008 |
| `ROL-06` | T002, T003, T007, T009, T018 |
| `ROL-07` | T008, T009 |
| `ROL-08` | T007, T009 |
| `ROL-09` | T006, T008, T010 |
| `ROL-10` | T010 |
| `ROL-11` | T011, T018 |
| `ROL-12` | T011 |
| `ROL-13` | T012 |
| `ROL-14` | T013, T015 |
| `ROL-15` | T014, T015 |
| `ROL-16` | T014, T015 |
| `ROL-17` | T004, T016 |
| `ROL-18` | T017 |
| `ROL-19` | T002, T007 |
| `ROL-20` | T019 |
| `ROL-21` | T001, T019 |
| `ROL-22` | T001 |
| `SC-001` | T007, T016 |
| `SC-002` | T005 |
| `SC-003` | T010, T011 |
| `SC-004` | T006 |
| `SC-005` | T005 |
| `SC-006` | T005 |
| `SC-007` | T013, T015 |
| `SC-008` | T015 |
| `SC-009` | T019, T019 |
