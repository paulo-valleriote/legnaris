# Tasks: Email Delivery Infrastructure — Provider-Agnostic Sender, Embedded Templates, Durable Outbox and Relay Publisher

**Input**: Design documents from `.specs/features/016-email-infrastructure/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Mandatory, not optional. The constitution (Principle III, NON-NEGOTIABLE) and
`AGENTS.md` require every behavior change to ship tests in the same commit: edge cases first, then
one representative happy path, with the typed sentinel asserted on every failure path. Tests must
never require a broker or MongoDB unless the suite self-skips.

**Organization**: Tasks are grouped by user story so each story can be implemented, tested and
delivered independently. Requirement IDs (`EML-*`) come from `spec.md`; the feature prefix `EML` is
exclusive to this feature.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task). Two tasks
  that edit the same file are never both `[P]`.
- **[Story]**: `[US1]`, `[US2]`, `[US3]` for user-story phases. Setup, Foundational and Polish tasks
  carry no story label.
- Every task names the exact repository file(s) it touches.

## Path Conventions

This is a Go backend plus a background binary in one module; all paths are repository-relative
(`internal/…`, `shared/…`, `cmd/…`, `deploy/…`). Tests live next to the code as `*_test.go`; shared
test doubles live in `internal/email/emailtest` (US2).

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the baseline before changing anything. This feature adds two dependencies, but
each is introduced by the task that first imports it, so the baseline is a verification step.

- [ ] T001 Confirm `go version` reports go1.26.4; run the existing baseline gates `make fmt && make lint && make vet && make lint-specs` and `make test-race` and record that they are clean; confirm `go.mod`/`go.sum` are unchanged before the first edit and note the two dependencies the feature will add (`EML-20`).

**Checkpoint**: Clean baseline; no dependency drift to attribute later failures to.

---

## Phase 2: Foundational (Shared Seam, No User Story)

**Purpose**: Deliver the one shape both P1 stories consume — the published message contract — so the
relay (US1) and the templates/sender (US2) build on a fixed wire format.

- [ ] T002 Add `internal/email/message.go` (`Message` with `id`/`type`/`to`/`template`/`data`/`created_at`/`attempt`, `Data` with `code`/`expires_in_minutes`/`purpose`, the `Purpose` type and its constants, the `email.otp` type constant, and the AMQP property derivation: `ContentType: application/json`, `DeliveryMode: Persistent`, `MessageId: id`), `internal/email/testdata/message.golden.json` with the exact contract JSON, and `internal/email/message_test.go` (golden-file equality, leading-zero code preserved as a string, RFC 3339 `created_at`, and the code absent from the derived AMQP properties) (`EML-07`, `EML-18`, `EML-22`).

**Checkpoint**: The wire contract is frozen and pinned; US1 and US2 can proceed in parallel.

---

## Phase 3: User Story 1 — A queued email is not lost when the relay crashes (Priority: P1) 🎯 MVP

**Goal**: The intention is stored durably, claimed atomically with a lease, and published to the
broker at least once with confirmation; a crash, a second relay, or a broker outage loses nothing,
and the broker topology exists before the first publish.

**Independent Test**: With mockery `Publisher`/`Observer` mocks and a Mongo-backed (self-skipping)
store, write pending intentions and run one pass — each is published once and marked published;
refuse confirmations — the row stays pending with an incremented attempt and a later eligibility;
expire a lease without marking — a second pass republishes and converges; lease from two relays — no
shared row; reach the publication-attempt cap — the row is failed. Without a broker, the topology
and publish semantics are proven against the `channel` seam's fake.

> The relay suite uses the mockery `Publisher`/`Observer`/`Repository` mocks (no cross-story import);
> the recording double lives in US2's `emailtest`.

### Tests for User Story 1 (write first; they must fail to compile until T007–T011 land — see Notes)

- [ ] T003 [P] [US1] Add `internal/email/outbox/intent_test.go`: `NewIntent` produces a canonical UUID v7 that `idgen.Validate` accepts, starts `pending` with `attempts 0` and `next_attempt_at == now` and no lease, and rejects a supplied non-canonical identity with `ErrInvalidIntentID`; the payload round-trips the code, lifetime and purpose (`EML-01`, `EML-18`).
- [ ] T004 [P] [US1] Add `internal/email/outbox/repository_test.go` (Mongo, self-skip when `DATABASE_URI` is unreachable): `Create` writes one document; two concurrent `Claim` calls never return the same row; a live lease hides a row while `locked_until == now` is claimable; `Reschedule` increments `attempts` and moves `next_attempt_at`; `MarkPublished` sets `published_at` once and hides the row; `MarkTerminal` hides the row permanently; `Backlog` counts pending; a row whose `_id` is not canonical is rejected with `ErrInvalidIntentID`; `db.EnsureIndexes` created `email_outbox_status_next_attempt` and `email_outbox_published_at_ttl` (`EML-03`, `EML-04`, `EML-17`, `EML-18`).
- [ ] T005 [P] [US1] Add `internal/email/relay/relay_test.go` with the mockery `Publisher`, `Observer` and outbox `Repository` mocks: a confirmed publish marks published and reports `published`; a transient error reschedules with the tier backoff and reports `retried`; reaching the publication-attempt cap marks terminal and reports `failed`; a non-canonical identity marks terminal without calling the publisher; a crash-between (published but unmarked) republishes the same identity and converges; two relays never publish the same identity; an empty outbox is a no-op; a cancelled context returns after the in-flight batch (`EML-02`…`EML-05`, `EML-23`, `SC-001`, `SC-002`).
- [ ] T006 [P] [US1] Add `internal/email/amqp/topology_test.go` (the declared entity set and each queue's arguments: quorum type, `x-message-ttl`, DLX and routing key, `email.otp`'s `x-max-length: 10000` and `x-overflow: reject-publish`) and `internal/email/amqp/broker_test.go` against a fake implementing the `channel` seam (a confirmed publish returns nil; `Ack=false`, a wait timeout, a transport error and a blocked connection are transient; a recorded `basic.return` is transient; the publishing carries `ContentType`, `DeliveryMode: Persistent` and `MessageId`) with no broker required (`EML-06`, `EML-07`, `EML-19`).

### Implementation for User Story 1

- [ ] T007 [P] [US1] Add `internal/email/outbox/intent.go`: the `Intent` record (identity, recipient, template, `Payload`, `Status` and the `pending`/`published`/`failed` constants, `Attempts`, `NextAttemptAt`, `LockedUntil`, `LockedBy`, `CreatedAt`, `PublishedAt`, `LastError`) with the `bson` tags from `data-model.md` §1, `NewIntent` generating a canonical UUID v7 via `shared/idgen` and validating a supplied one, and `ErrInvalidIntentID` carrying the one-line "why" comment that cites the non-HTTP-layer precedent (`shared/idgen.ErrInvalidID`, `shared/db`) for using a bare sentinel instead of `apperr` (`EML-01`, `EML-18`, `EML-19`).
- [ ] T008 [P] [US1] Add `internal/email/relay/publisher.go`: the `Publisher` port (`Publish(ctx, email.Message) error`), the `Observer` port (`PublishResult(result string)`, `BacklogPending(n int)`) and the relay `Config` (`PollInterval`, `Lease`, `Batch`, `ConfirmTimeout`, `MaxAttempts`, `Backoff`, `Now`, `ClaimerID`) with documented defaults matching `data-model.md` §5 (`EML-02`, `EML-07`, `EML-23`).
- [ ] T009 [US1] Add `internal/email/outbox/repository.go`: `Create`, `Claim` (one conditional `findOneAndUpdate` per row with the filter and lease from `contracts/outbox.contract.md`, sorted by `next_attempt_at`, capped at `limit`), `MarkPublished`, `Reschedule`, `MarkTerminal`, `Backlog`, plus the `Repository` interface and `NewRepository`; validate every identity with `idgen.Validate` and return `ErrInvalidIntentID` for a non-canonical one; add `legnaris/internal/email/outbox` to `.mockery.yaml` and run `make mocks`; extend `shared/db/indexes.go` with `email_outbox_status_next_attempt` and `email_outbox_published_at_ttl` (expireAfterSeconds = the retention default) (`EML-01`, `EML-03`, `EML-04`, `EML-17`, `EML-18`).
- [ ] T010 [US1] Add `internal/email/relay/relay.go`: `Run(ctx)` polling on `PollInterval`, claiming a `Batch` with `Lease`, encoding each intent into an `email.Message` with `Attempt: 0`, publishing through the `Publisher`, marking published on success, rescheduling with the tier backoff (`5s/15s/1m/5m`), marking terminal at the **publication-attempt** cap, reporting each outcome to the `Observer`, updating the backlog gauge, and returning after the in-flight batch on cancellation; add `legnaris/internal/email/relay` to `.mockery.yaml` and run `make mocks` (`EML-02`…`EML-06`, `EML-19`, `EML-23`).
- [ ] T011 [P] [US1] Add `internal/email/amqp/channel.go` (the `channel` seam: a small interface over the channel operations the adapter uses, plus an injectable dial function value), `internal/email/amqp/topology.go` (the exchange, the quorum work queue with `x-max-length: 10000`/`x-overflow: reject-publish`/DLX, the three delay-tier queues with `x-message-ttl` and DLX back, and the DLQ) and `internal/email/amqp/broker.go` (`Dial(url)` via `amqp.DialConfig` with `Config.Recovery` and a `NotifyStateChange`/`NotifyBlocked` drain goroutine; `DeclareTopology()`; `Publish(ctx, msg)` using `Confirm(false)` + `PublishWithDeferredConfirmWithContext(..., mandatory=true, ...)` + `WaitContext`, refusing to report success on a recorded `basic.return`; `Close()`); add the `github.com/rabbitmq/amqp091-go` dependency (`EML-06`, `EML-07`, `EML-19`).

**Checkpoint**: US1 is independently complete — a stored intention reaches the broker once, survives a crash, and the topology is declared; no other story is required.

---

## Phase 4: User Story 2 — One sending port, a real provider, and a safe stand-in (Priority: P1)

**Goal**: One port every caller uses; a real adapter that classifies retryable versus permanent; a
development adapter that never touches the network; a recording double; and per-purpose embedded
templates whose subject never carries the code.

**Independent Test**: Render each known purpose and assert the code is in both bodies and not in the
subject; drive the real adapter against an `httptest` provider and assert 200 → success, 429/5xx/
timeout → `ErrTransient`, 400/422 → `ErrPermanent`, and the idempotency key on the request; assert
the development adapter performs no network call; assert an unknown purpose is a permanent failure.

### Tests for User Story 2 (write first; they must fail to compile until T016–T020 land — see Notes)

- [ ] T012 [P] [US2] Add `internal/email/sender_test.go`: `ErrTransient` and `ErrPermanent` are distinct sentinels and an adapter error wrapping either is recognised by `errors.Is` (`EML-08`, `EML-12`).
- [ ] T013 [P] [US2] Add `internal/email/template_test.go`: for `verify_email` and `login_otp`, the subject does not contain the code while both the text and HTML bodies contain the code and the lifetime; an unknown purpose returns `ErrUnknownTemplate` with an empty result; HTML-significant characters in the code are escaped in HTML and verbatim in text; rendering is deterministic; a leading-zero code is preserved (`EML-11`, `EML-13`, `EML-22`).
- [ ] T014 [P] [US2] Add `internal/email/resend/transport_test.go` (status capture, body truncation, body close) and `internal/email/resend/sender_test.go` against an `httptest` provider: 200 → success and the provider received `Idempotency-Key` equal to the intention id; 429/5xx/timeout/network error → `errors.Is(err, ErrTransient)`; 400/422 → `errors.Is(err, ErrPermanent)`; empty `apiKey` or `from` → a build error (`EML-09`, `EML-10`, `EML-12`).
- [ ] T015 [P] [US2] Add `internal/email/sender_log_test.go`: the development sender makes no network call, records the recipient masked as `a***@example.com` (first local-part character plus domain), and never logs the code (`EML-10`, `EML-13`).

### Implementation for User Story 2

- [ ] T016 [P] [US2] Add `internal/email/sender.go`: the `Outbound` value (`IdempotencyKey`, `To`, `Subject`, `Text`, `HTML`), the `Sender` interface (`Send(ctx, Outbound) error`), and the `ErrTransient`/`ErrPermanent` sentinels carrying the one-line "why" comment that cites the non-HTTP-layer precedent (`shared/idgen.ErrInvalidID`, `shared/db`) for a bare sentinel instead of `apperr`; add `legnaris/internal/email` (interface `Sender`) to `.mockery.yaml` and run `make mocks` (`EML-08`, `EML-12`, `EML-19`).
- [ ] T017 [US2] Add `internal/email/template.go` and `internal/email/templates/` (`verify_email.{subject.txt,body.txt,body.html}`, `login_otp.{subject.txt,body.txt,body.html}`, English copy, code-free subject): the `Renderer` interface, `Rendered`, `ErrUnknownTemplate`, and the `//go:embed`-backed implementation parsing the subject/text with `text/template` and the HTML body with `html/template` (`EML-11`, `EML-13`).
- [ ] T018 [P] [US2] Add `internal/email/sender_log.go`: the development `LogSender` (structured line with the id, purpose and recipient masked as `a***@example.com`; returns nil; no network; a built-in placeholder `From`) (`EML-10`, `EML-13`).
- [ ] T019 [US2] Add `internal/email/resend/transport.go` (the `http.RoundTripper` mapping non-2xx to a typed status error) and `internal/email/resend/sender.go` (`New(apiKey, from, timeout) (*Sender, error)` rejecting an empty `apiKey` or empty `from`, building the SDK client via `resend.NewCustomClient` and sending with `SendWithOptions` + `SendEmailOptions{IdempotencyKey}`, mapping outcomes per `contracts/sender.contract.md`); add the `github.com/resend/resend-go/v4` dependency (`EML-09`, `EML-12`).
- [ ] T020 [US2] Add `internal/email/emailtest/recorder.go` (a recording `Sender` with injectable failure sentinels), `internal/email/emailtest/fixtures.go` (message/outbound builders over `email.Message` and `email.Outbound` — never `outbox.Intent`, to keep the import inside US2) and `internal/email/emailtest/recorder_test.go` proving the recorder records and surfaces the injected sentinel (`EML-10`).

**Checkpoint**: US2 is independently complete; US1 is untouched.

---

## Phase 5: User Story 3 — The pipeline can be enabled without changing the required configuration (Priority: P2)

**Goal**: Every new setting is optional with a documented default; the request-serving process still
starts with none of them; the relay validates its own broker settings and fails fast; the broker is
reachable only internally and is a separate service; the relay exposes its metrics.

**Independent Test**: Start the app with no email/broker variables and assert it starts and the
required-variable list is unchanged; run the relay with a blank and with an unparseable broker URL
and assert each fails fast with a typed error and never polls; validate the deployment description
and assert the broker has no published port and the relay is a separate service; observe the backlog
and publish-outcome metrics.

### Tests for User Story 3 (write first; they must fail to compile until T024–T028 land — see Notes)

- [ ] T021 [P] [US3] Extend `shared/env/env_test.go` for every new variable: absent → default, blank → default, valid → parsed, invalid duration/int → default with a warning; `RequiredKeys()` stays exactly `DATABASE_URI`, `JWT_SECRET` (`EML-14`, `EML-16`).
- [ ] T022 [P] [US3] Extend `deploy/compose_test.go`: each new key is present on the app/relay service as an optional interpolation; the `rabbitmq` service declares no `ports:` and a healthcheck; the `relay` service is defined separately and only `relay` depends on the broker; `env.RequiredKeys()` is unchanged (`EML-16`).
- [ ] T023 [P] [US3] Add `cmd/relay/main_test.go`: an empty and an unparseable `RABBITMQ_URL` make the process fail fast with `ErrBrokerUnavailable` before any poll; a cancelled context lets the in-flight batch finish and returns (`EML-15`, `EML-19`).

### Implementation for User Story 3

- [ ] T024 [P] [US3] Extend `shared/env/env.go` with the `Variables` fields and default constants for `EMAIL_PROVIDER` (`log`), `RESEND_API_KEY` (empty), `EMAIL_FROM` (**empty**; the `resend` adapter requires it non-empty), `EMAIL_SEND_TIMEOUT` (`10s`), `RABBITMQ_URL` (empty; the relay requires it), `EMAIL_RELAY_POLL_INTERVAL` (`1s`), `EMAIL_RELAY_LEASE` (`30s`), `EMAIL_RELAY_BATCH` (`25`), `EMAIL_RELAY_CONFIRM_TIMEOUT` (`5s`), `EMAIL_RELAY_MAX_ATTEMPTS` (`5`), `EMAIL_OUTBOX_RETENTION` (`168h`) and `EMAIL_RELAY_METRICS_ADDR` (`:9100`), using the existing lookup helpers; keep `RequiredKeys()` unchanged (`EML-14`).
- [ ] T025 [P] [US3] Extend `internal/platform/metrics/metrics.go` with `email_outbox_backlog` (gauge), `email_publish_total{result}` (counter) and `email_dead_total` (counter), a `Handler() http.Handler` for a non-Echo server, and `PublishResult`/`BacklogPending` implementing the relay `Observer` declared in T008; keep the API's `Register(e)` and existing collectors unchanged (`EML-23`).
- [ ] T026 [US3] Add `cmd/relay/main.go` with a testable `run(ctx, cfg, deps) error` seam: load `shared/env`, fail fast with `ErrBrokerUnavailable` on a blank/unparseable `RABBITMQ_URL`, open Mongo and the broker, declare the topology, build the outbox store and the relay with the configured knobs, serve `metrics.Handler()` on `EMAIL_RELAY_METRICS_ADDR`, and run with SIGINT/SIGTERM draining (`EML-05`, `EML-15`, `EML-16`).
- [ ] T027 [P] [US3] Extend `deploy/docker-compose.yml` (a `rabbitmq` service on `internal` only, no published ports, a `rabbitmq-diagnostics -q ping` healthcheck, a data volume; a `relay` service built from the same image with the broker/Mongo dependencies and the email/broker variables), `deploy/.env.example` (the documented optional variables) and `deploy/prometheus/prometheus.yml` (scrape the relay listener) (`EML-16`).
- [ ] T028 [P] [US3] Extend `Dockerfile` (build `/relay` from `./cmd/relay` into the runtime image beside `/app` and `/canonicalize-emails`) and `Makefile` (add the new optional variables to `APP_ENV_KEYS` and a `relay` target) (`EML-16`).

**Checkpoint**: US3 is independently complete; the pipeline lands dark and the app is unchanged.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Document operation, record the decision, and run the full gates.

- [ ] T029 [P] Document the relay and the new optional variables in `deploy/README.md`: how to run the relay, the fail-fast behaviour on a missing/invalid broker URL, the variable defaults, and the metrics endpoint (`EML-15`, `EML-16`).
- [ ] T030 Record the two new dependencies and the package placement as an `AD-*` decision in `.specs/STATE.md`, as the plan's Constitution Check promises (`EML-08`, `EML-14`).
- [ ] T031 Run the complete repository gate set and resolve every finding without weakening an assertion: `make fmt && make lint && make vet && make lint-specs` then `make test-race`; confirm every `EML-*` citation resolves via `python3 scripts/check_requirement_ids.py`, all strings/comments are English (`EML-21`), and the race detector is clean (`EML-20`, `SC-008`).
- [ ] T032 Execute the [quickstart.md](./quickstart.md) validation against a local MongoDB (`make dev-db`): §2 contract/templates, §3 classification against `httptest`, §4 outbox/relay, §5 relay process fail-fast, §7 deployment surface; fix any deviation in the touched files and report the observed outcomes (`SC-001`…`SC-008`).
- [ ] T033 Optional smoke against a real broker (quickstart §6, requires Docker): declare the topology against `rabbitmq:4-management`, insert one intention, assert the relay publishes it, the row is `published`, the metrics endpoint reports the backlog and the publish outcome, and a killed-and-restarted relay republishes the same identity (`EML-06`, `SC-001`, `SC-007`).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001)**: no dependencies; blocks nothing technically but must be run first to establish a clean baseline.
- **Foundational (T002)**: depends on T001. Blocks US1 and US2 (both consume the message contract). US3 does not consume it.
- **US1 (T003–T011)**: depends on T001 and T002. Independent of US2 and US3.
- **US2 (T012–T020)**: depends on T001 and T002. Independent of US1 and US3.
- **US3 (T021–T028)**: depends on T001; T025 also depends on T008 (it implements the relay `Observer`), and T026 additionally depends on T010, T011, T024 and T025.
- **Polish (T029–T033)**: depends on every story in scope; T031/T032 need all implemented tasks.

### Within Each Story

- Tests come before the behavior they cover. The story test tasks are written against the interfaces
  and types declared by the implementation tasks **in the same phase**; until those declarations
  land the tests do not compile, and that compilation failure is the recorded "fails first" evidence
  — there is no separate seam task.
- Record → store → relay → broker adapter (US1); port → templates → adapters → recorder (US2);
  config/metrics → process → deployment (US3).
- Every task ships its tests in the same commit; no assertion is weakened to make a gate pass.

### User Story Dependency Graph

```text
T001 ─┬── T002 ─┬── T003/T004/T005/T006 → T007 → T009 → T010   (US1)
      │         │                         T008 ─┘
      │         │                         T011                (US1)
      │         └── T012/T013/T014/T015 → T016 → T017 → T019   (US2)
      │                                    T018 ─┘
      │                                    T020                (US2)
      ├── T021/T022/T023 → T024 → T026 → T027/T028            (US3)
      │                    T025 ─┘   (T025 also needs T008)
      └── T010/T011 → T026
T030 → T031 → T032 → T033
```

---

## Parallel Opportunities

### Foundational

```text
T001  → T002 (message contract + golden test)
```

### User Story 1

```text
T003 intent table           (internal/email/outbox/intent_test.go)
T004 store table (Mongo)    (internal/email/outbox/repository_test.go)
T005 relay table            (internal/email/relay/relay_test.go)
T006 topology/broker table  (internal/email/amqp/*_test.go)
        ↓ after the tests are written
T007 (intent.go) ─┐
T008 (publisher.go)┤→ T009 (repository.go + indexes + mocks) → T010 (relay.go)
T011 (amqp/*.go)  ─┘ (independent of T009/T010; depends only on T002)
```

### User Story 2

```text
T012 port table            (internal/email/sender_test.go)
T013 template table        (internal/email/template_test.go)
T014 resend table          (internal/email/resend/*_test.go)
T015 dev-sender table      (internal/email/sender_log_test.go)
        ↓
T016 (sender.go + Sender mock) → T017 (template.go + templates/) → T019 (resend/*.go)
                                 T018 (sender_log.go)
                                 T020 (emailtest/recorder.go + fixtures.go + recorder_test.go)
```

### User Story 3

```text
T021 env table        (shared/env/env_test.go)
T022 compose table    (deploy/compose_test.go)
T023 relay proc table (cmd/relay/main_test.go)
        ↓
T024 (env.go) ─┐
T025 (metrics)─┴→ T026 (cmd/relay/main.go) → T027 (compose/.env/prometheus) / T028 (Dockerfile/Makefile)
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. Complete T001, T002 and Phase 3 (T003–T011).
2. **STOP and VALIDATE**: run `go test ./internal/email/outbox/... ./internal/email/relay/... ./internal/email/amqp/...` and the quickstart §4 scenario.
3. Deploy/demo: US1 alone proves no email is lost with no broker running — the program plan's core
   reliability fix.

US2 is also P1. The complete P1 scope is US1 + US2; deliver US2 immediately after US1 because it is
the only path an email actually leaves the system.

### Incremental Delivery

1. T001 → clean baseline.
2. T002 → the wire contract is frozen.
3. US1 → durable outbox + relay + topology.
4. US2 → sender port + adapters + templates + recorder.
5. US3 → optional configuration + deployment surface + relay process.
6. Polish → docs, decision record, full gates, quickstart validation.

### Parallel Team Strategy

- One developer takes US1 (outbox + relay + broker), another takes US2 (port + templates +
  adapters) right after T002; they touch disjoint packages and do not import each other's doubles.
- One developer takes US3 (config + deployment) in parallel; T026 serialises behind US1's T010/T011.
- Within US1, one developer owns the outbox/relay half (T007–T010) and another the broker half
  (T011).

---

## Requirement Traceability

| Requirement | Task(s) | Independent proof |
|---|---|---|
| `EML-01` | T003, T007, T009 | `Create` writes before acceptance; the relay-only write path |
| `EML-02` | T005, T008, T010 | success marks published only after a confirm |
| `EML-03` | T004, T009, T010 | unique claim under two callers; lease expiry re-claim |
| `EML-04` | T004, T009, T010 | backoff on failure; terminal at the **publication-attempt** cap |
| `EML-05` | T005, T010, T026 | poll loop; cancellation drains the in-flight batch |
| `EML-06` | T006, T011 | the declared topology and its arguments; declared before the first publish |
| `EML-07` | T002, T005, T006, T011 | the message contract; mandatory publish + return detection |
| `EML-08` | T012, T016 | one port; adapter selection |
| `EML-09` | T014, T019 | idempotency key + timeout + status mapping on `httptest` |
| `EML-10` | T015, T018, T020 | recorder and dev sender; no provider call in a unit test |
| `EML-11` | T013, T017 | per-purpose subject/bodies; code-free subject; unknown purpose |
| `EML-12` | T012, T014, T019 | transient vs permanent sentinels |
| `EML-13` | T013, T015, T018 | code never in a subject/log/error; recipient masked as `a***@example.com` |
| `EML-14` | T021, T024 | optional variables with defaults; required set unchanged |
| `EML-15` | T023, T026 | fail fast on a blank/invalid broker URL |
| `EML-16` | T022, T024, T026, T027, T028 | broker not published; relay is separate; app starts without them |
| `EML-17` | T004, T009 | the two indexes; retention touches published rows only |
| `EML-18` | T002, T003, T004, T007, T009 | canonical UUID v7; non-canonical → `ErrInvalidIntentID` |
| `EML-19` | T006, T007, T010, T011, T016, T023, T026 | typed sentinels and the relay's exact action |
| `EML-20` | T031 (and every task's tests) | `make test-race` clean; edge cases before happy path |
| `EML-21` | T031 (and every source task) | English-only identifiers, comments, logs, templates |
| `EML-22` | T002, T013 | golden message file; per-purpose rendering assertions |
| `EML-23` | T005, T008, T010, T025 | backlog gauge + publish outcomes via `Observer` |

**Coverage**: 23 requirements, 23 mapped to at least one task, 0 unmapped.

### Success Criteria Map

| Criterion | Proven by |
|---|---|
| `SC-001` | T005, T010, T032, T033 |
| `SC-002` | T004, T005, T032 |
| `SC-003` | T002, T006, T011, T032 |
| `SC-004` | T012, T014, T015, T019, T032 |
| `SC-005` | T013, T015, T018, T032 |
| `SC-006` | T021, T022, T024, T032 |
| `SC-007` | T006, T022, T027, T033 |
| `SC-008` | T031, every boundary assertion |

---

## Notes

- `[P]` tasks touch different files and have no dependency on an incomplete task.
- Tests are not optional in this repository: each implementation task lands with the tests that
  prove it, in the same commit, edge cases first.
- **Tests-first evidence.** Each story's test tasks precede its implementation tasks and, by design,
  do not compile until the interfaces/types those tasks declare exist; the recorded "fails first"
  evidence for a test task is that compilation failure. No separate seam task is used.
- **Story independence.** US1's relay suite uses the mockery `Publisher`/`Observer`/`Repository`
  mocks and never imports US2's `emailtest`; `emailtest` lives entirely in US2 and its fixtures use
  only `email.Message`/`email.Outbound` (never `outbox.Intent`).
- **Bare sentinels.** `ErrTransient`, `ErrPermanent`, `ErrUnknownTemplate` and `ErrInvalidIntentID`
  are declared with `errors.New` (not `apperr`) and each carries the one-line "why" comment citing
  `shared/idgen`/`shared/db`; the email domain is a non-HTTP layer with no boundary to map.
- Two new third-party dependencies are introduced, each by the task that imports it (`amqp` in T011,
  `resend` in T019); no other dependency is added, and no new required environment variable appears
  (`EML-14`).
- The API boot path never imports `internal/email`: `cmd/relay` is a separate binary (`EML-16`).
- Commits are local. Do not push, and never print, log or commit a token.
- Every task that touches `internal/email`, `cmd/relay`, `shared/env`, `shared/db`,
  `internal/platform/metrics` or `deploy/` must keep `EML` citations in comments and test names
  where the requirement is not self-evident.