# Implementation Plan: The Email Worker — One Process That Turns a Queued Code into a Delivered Email

**Branch**: `020-email-worker` | **Date**: 2026-10-21 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/020-email-worker/spec.md`

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §5.6 (publisher and consumer), §5.7 (retry), §5.9
(observability), §5.10 (tests) and §9 stage 6 (the worker as a deployed process) — decisions D5
(Resend), D6 (RabbitMQ + outbox), O1 (TTL + DLX retry tiers), O2 (a separate process). This feature is
the consumer half of the delivery pipeline: the earlier feature delivered the topology, the message
contract, the sending port, the templates and the publisher, and deferred the consumer; this feature
builds it. It ships the process and its executable, and deliberately ships no compose service, no
management UI, no Prometheus scrape and no alert rules — the deployment slice is the next feature.

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Build the process that closes the email pipeline: it takes a queued code and turns it into a delivered
email, and it never loses one.

1. **A consumer of the queued message.** A new `internal/email/worker` package owns the delivery
   policy: decode the body against the delivered message contract, decide from the message's own
   creation time and advertised lifetime whether the code is still worth sending, render it from the
   embedded template, hand it to the delivered sending port with the message identity as the
   provider's idempotency key, and acknowledge only after the send succeeded. A body that cannot be
   decoded, an identity that is not a canonical UUID v7, a missing code and a purpose that selects no
   template are all permanently unusable: they are dead-lettered and acknowledged, never retried
   forever. An expired message is acknowledged without being sent and counted as expired.
2. **A retry ladder the broker already owns.** A retryable failure republishes the message to the
   delay tier that matches its attempt count — 30 seconds, 2 minutes, then 5 minutes — with the count
   incremented, and the original is acknowledged only after the broker confirmed the republish. A
   permanent failure and a message that reached the attempt cap go to the dead-letter route. Nothing
   is dropped: an unconfirmed republish leaves the message unacknowledged so the broker redelivers it.
3. **A process that stands on its own.** `cmd/worker` is a separate executable: it validates its own
   broker and provider configuration and fails fast, it bounds the messages it holds in flight, it
   re-establishes its consumer when the broker connection drops, it stops gracefully on a stop signal
   (finishing what is in flight and leaving the rest to the broker), and it exposes the counters that
   say whether the pipeline is flowing — send outcomes by result, send duration, dead letters and
   expired messages.

The change is confined to the Go backend: a new `internal/email/worker` package, an extended
`internal/email/amqp` adapter (a consumer channel and a publish addressed by destination), a new
`internal/email` destination vocabulary, one new binary (`cmd/worker`), one new bounded decoder in
`shared/json`, `shared/env`, `internal/platform/metrics`, the `Dockerfile` and the `Makefile`.
**No new third-party dependency** — the pinned `amqp091-go` v1.16.0 already provides the consumer
surface — and **no new required environment variable**.

## Technical Context

**Language/Version**: Go 1.26.4 (`go.mod` `go 1.26.4`; `go version` confirms `go1.26.4 linux/amd64`),
the same toolchain the `Dockerfile` builds with (`golang:1.26.4-alpine`). No new language and no new
runtime.

**Primary Dependencies**: unchanged — `github.com/rabbitmq/amqp091-go` v1.16.0 (already required by
the publisher), `github.com/resend/resend-go/v4` v4.9.0, `github.com/prometheus/client_golang`
v1.24.1, `github.com/stretchr/testify` v1.12.1, `github.com/vektra/mockery/v2` v2.53.5 (the `tool`
directive), `github.com/labstack/echo/v5` v5.3.1 (untouched by this feature). The pinned
`amqp091-go` v1.16.0 is verified to expose the whole consumer surface this feature needs —
`Channel.Qos(prefetchCount, prefetchSize int, global bool)`, `Channel.Consume(queue, consumer string,
autoAck, exclusive, noLocal, noWait bool, args Table) (<-chan Delivery, error)`,
`Channel.Ack(tag uint64, multiple bool)`, `Channel.Nack(tag uint64, multiple, requeue bool)`,
`Channel.NotifyCancel(chan string) chan string` and `Channel.Cancel(consumer string, noWait bool)` —
and its `Recovery` option already re-establishes the connection and re-registers the consumers and
the topology (`consumers.go` tracks every consumer configuration for re-subscription during recovery).
**No dependency is added.**

**Storage**: none. The worker keeps no store and needs no database: the code's lifetime travels on
the message, so the expiry decision is local (the clarification of 2026-10-21). The MongoDB 7.0
standalone of the deployment is untouched, and so is the `email_outbox` collection the publisher owns.

**Testing**: `go test ./...` and `go test -race ./...` — the race detector must stay clean (the
worker suite runs two consumers and two concurrent deliveries over one fake). Table-driven suites with
`t.Run` and `Happy path:` / `Edge:` / `Error:` prefixes, `require` for preconditions and `assert`
otherwise, mockery mocks (`NewMockX(t)`, `make mocks` after an interface change) for the worker's two
ports and ordinary fakes for the recording doubles. Pure unit suites for the decode-and-validate
rules, the expiry decision, the retry ladder, the acknowledgement rules and the shutdown behaviour;
the broker adapter is driven against the package's existing `channel` seam (an interface plus an
injectable dial function); the provider is the delivered `emailtest` recorder. **No unit test requires
a broker or a provider** (WKR-27); the broker integration suite self-skips when `RABBITMQ_URL` is
absent, like the Mongo integration suites do.

**Target Platform**: the Go backend on Linux. `cmd/worker` is a background binary in the same runtime
image as `cmd/api.go` and `cmd/relay`; it publishes no host port and listens only for its own metrics
on an internal address. It runs in the deployment's internal network beside the broker — the compose
service itself belongs to the next feature.

**Project Type**: backend service plus an operator/background binary in the same Go module; the
`web/` client is untouched (no HTTP contract changes).

**Performance Goals**: no throughput target; the pipeline's ceiling is the provider's free-tier
allowance (about 100 emails a day, O4). One delivery is one decode, one render and one provider call;
the in-flight bound (default 10, the plan's §5.6) caps the memory and the provider concurrency. The
worker polls nothing: it is driven by the broker's deliveries.

**Constraints**: `env.RequiredKeys()` and `deploy/compose_test.go` keep validating an unchanged
required list — the worker validates the broker and the provider itself and the request-serving
process does not (WKR-19); the request-serving process never talks to the broker or the provider
(EML-16); every new failure mode is a typed sentinel with a documented effect — retry, dead-letter,
acknowledge without sending, or stop the process — and the worker acts on it exactly (WKR-23); codes
never reach a log, an error, a metric or a subject (WKR-06, WKR-21); identities are canonical UUID v7
from `shared/idgen` and a non-canonical one is rejected (WKR-04); layering stays
`cmd/worker → internal/email/{worker,amqp,factory} → internal/email → shared/*` with no upward import;
JSON decoding goes through `shared/json` (Principle V); all source and strings are English (WKR-25);
`go test -race` stays clean (WKR-24).

**Scale/Scope**: 4 user stories (3 P1, 1 P2), 27 requirements, 5 internal contracts; one new package
(`internal/email/worker`), one new binary (`cmd/worker`), one extended adapter, one new shared helper,
roughly 20 touched or created files across `internal/email`, `internal/platform/metrics`,
`shared/{json,env}`, `cmd/`, `Dockerfile` and `Makefile`; about 10 new or extended test suites.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The worker is a non-HTTP layer, so it has no status to map and no `apperr` code; its failures are exported typed sentinels in `internal/email/worker` — a permanently unusable message, an expired message, a failed send and an unusable configuration — each with a documented effect, and the worker acts on them exactly: dead-letter, acknowledge without sending, retry, or stop (WKR-23). This mirrors the precedent the delivered email domain already sets (`email.ErrTransient`, `email.ErrPermanent`, `email.ErrUnknownTemplate`, `amqp.ErrBrokerUnavailable`) and `shared/idgen.ErrInvalidID`. No HTTP boundary is added, so no `apperr` code is introduced. | PASS |
| II. Canonical Identifiers | The worker generates no identifier: it reads the message identity, which the publisher already minted as a canonical lowercase UUID v7 from `shared/idgen.New`, and it screens it with `idgen.Validate` before anything else — a non-canonical identity is permanently unusable and never reaches the provider as an idempotency key (WKR-04). The identity doubles as the provider's idempotency key, so a duplicate send de-duplicates (WKR-08). | PASS |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | Every requirement lands with tests in the same commit, edge cases first (undecodable body, non-canonical identity, missing code, unknown purpose, expired code, zero lifetime, future creation time, rate limit, invalid recipient, timeout, unconfirmed republish, unconfirmed dead-letter, exhausted attempt cap, two consumers, redelivery after a crash, stop with a send in flight, blank broker URL, unknown provider), then a representative happy path per suite. Failure paths assert the typed sentinel and the worker's action; `go test -race` stays clean (WKR-24, SC-009). | PASS |
| IV. English-Only Source | All identifiers, comments, log messages and the worker's structured fields are English (WKR-25). No user-facing string is added: the email copy is the delivered template's. | PASS |
| V. Layered Architecture | `internal/email/worker` is a new leaf package: it imports `internal/email` (the contract, the renderer, the sending port), `shared/idgen`, `shared/json` and `shared/logger` — never `internal/app`, `internal/auth` or `internal/sheet`. The broker specifics stay in `internal/email/amqp`; `cmd/worker` wires them with `shared/env` and `shared/logger`, exactly as `cmd/relay` does. Logging goes through `shared/logger`; the message body is decoded through `shared/json`, extended with a bounded byte-slice decoder because the delivered HTTP decoder takes an `echo.Context` and rejects unknown fields (`research.md` R10). | PASS |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | The worker carries no secret of its own: `RABBITMQ_URL`, `RESEND_API_KEY` and `EMAIL_FROM` come from the environment and are never committed. The code never reaches a log, an error, a metric or a subject; the recipient is masked with the delivered `email.MaskRecipient`. The message that carries the code lives on the broker's internal network only, and no gate is weakened. | PASS (directly served) |
| VII. Installable, Offline-Capable Web Client | No client surface changes: the HTTP contract, the routes and the cookie attributes are untouched, so the PWA's existing flows keep working. | PASS (not involved) |
| Additional Constraints — Stack | Go on Echo with MongoDB and `shared/env` for configuration, `testify` + mockery tests — unchanged. This feature adds a background process and no client library, so no constitution amendment is needed; the choice is documented here and in `research.md` (`R1`, `R2`) and is entered as an `AD-*` decision in `.specs/STATE.md` when the process lands, following the `AD-016` precedent. No principle is waived. | PASS (decision documented) |
| Additional Constraints — Spec corpus and traceability | `WKR-01`…`WKR-27` are defined in `spec.md`; `scripts/check_requirement_ids.py` scans `*.go`, so the citations in the delivered sources resolve. The prefix `WKR` is exclusive to this feature. | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card; recorded for the PR, not claimed by this artifact. | PASS (noted) |
| Development Workflow | The backend sequence (`make fmt && make lint && make vet && make lint-specs`, then `make test-race`) covers the change. The `Dockerfile` builds the worker binary beside `/app` and `/relay`, the `Makefile` gains the worker target and the optional variables, and `deploy/compose_test.go` is unchanged because this feature adds no service to the stack. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps one implementation per rule:
the message contract is built once in `internal/email` and consumed as delivered (no second shape);
the destination names live once in `internal/email` and the broker adapter maps them to the exchange
and routing key it already owns (no routing key is spelled twice); the send classification stays the
delivered `email.ErrTransient` / `email.ErrPermanent` pair and the worker adds no provider logic; the
expiry decision is one function over the message's own fields, with no store lookup; the metrics names
stay in `internal/platform/metrics`, the only place a metric name is defined. Two choices that could
have drifted were resolved in research: the decoding goes through a new `shared/json` byte-slice
decoder without unknown-field rejection, because the HTTP decoder both takes an `echo.Context` and
would dead-letter every message a newer request-serving process produced during a rolling deploy
(`R10`), and the reconnection is split between the library's recovery (the connection and the
consumer re-registration) and the worker's own supervisor loop (the channel and the consumer tag)
(`R7`).

## Project Structure

### Documentation (this feature)

```text
.specs/features/020-email-worker/
├── spec.md                     # Requirements (WKR-*)
├── plan.md                     # This file
├── research.md                 # Phase 0 — decisions and rejected alternatives
├── data-model.md               # Phase 1 — message, destination, outcomes, settings, counters
├── quickstart.md               # Phase 1 — runnable validation guide
├── contracts/                  # Phase 1 — one contract per internal seam
│   ├── worker.contract.md
│   ├── broker-consumer.contract.md
│   ├── destination.contract.md
│   ├── configuration.contract.md
│   └── metrics.contract.md
├── checklists/
│   └── requirements.md         # Spec quality checklist
└── tasks.md                    # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
backend/cmd/worker/
├── main.go                           # NEW: wiring, fail-fast, consume loop, graceful shutdown, metrics listener (WKR-01, WKR-17…WKR-20)
└── main_test.go                      # NEW: run seam, fail-fast on a bad broker/provider, SIGTERM drains in-flight

backend/internal/email/worker/        # NEW: the delivery policy
├── worker.go                         # NEW: consume, decode, validate, decide expiry, render, send, acknowledge (WKR-02…WKR-08)
├── retry.go                          # NEW: the retry ladder and the attempt cap (WKR-09…WKR-15)
├── ports.go                          # NEW: the delivery source and the republisher ports (R2)
├── config.go                         # NEW: the worker's knobs and their defaults (WKR-14, WKR-16)
├── errors.go                         # NEW: the typed sentinels and their documented effects (WKR-23)
├── worker_test.go                    # NEW: decode/validate matrix, expiry, ack rules, two consumers, redelivery
├── retry_test.go                     # NEW: the ladder, the cap, unconfirmed republish, unconfirmed dead-letter
├── config_test.go                    # NEW: defaults, zero values, the attempt cap and the in-flight bound
└── mocks/                            # NEW: mockery-generated DeliverySource and Republisher

backend/internal/email/               # EXTENDED: the domain
├── destination.go                    # NEW: the Destination type and its four names (R3)
├── destination_test.go               # NEW: the names, the mapping, no broker vocabulary leaks
├── message.go                        # EXTENDED: the wire contract's doc comment names the consumer (WKR-04)
└── message_test.go                   # EXTENDED: the contract still round-trips

backend/internal/email/amqp/          # EXTENDED: the consumer side of the adapter
├── channel.go                        # EXTENDED: the channel seam gains Qos, Consume, Ack, Nack, NotifyCancel (R7)
├── broker.go                         # EXTENDED: a consumer channel, a routed publish, the destination mapping (R5, R6, R7)
├── consuming.go                      # NEW: the delivery source over the broker (consume, ack, nack, re-consume) (WKR-16, WKR-17)
├── consuming_test.go                 # NEW: the delivery stream, the ack and nack paths, the stream ending
└── topology_test.go                  # EXTENDED: the destination mapping matches the declared topology

backend/internal/email/factory/       # UNCHANGED: the provider selection is reused as delivered (R13)

backend/internal/platform/metrics/
├── metrics.go                        # EXTENDED: send outcomes, send duration, expired; the dead-letter counter is reused (WKR-21, R11)
└── metrics_test.go                   # EXTENDED: the new series and their labels, no code and no full address

backend/shared/json/
├── decode.go                         # EXTENDED: a bounded byte-slice decoder without unknown-field rejection (R10)
└── decode_test.go                    # NEW: the bound, the unknown-field tolerance, the typed error

backend/shared/env/
├── env.go                            # EXTENDED: the worker's optional settings (WKR-14, WKR-16, WKR-20)
└── env_test.go                       # EXTENDED: absent/blank/valid values; RequiredKeys unchanged

Dockerfile                            # EXTENDED: build /worker into the runtime image (WKR-26)
Makefile                              # EXTENDED: export the worker's optional variables; the worker target
```

**Structure Decision**: the delivery policy lives in `internal/email/worker`, beside the message
contract it consumes and the publisher that feeds it, and the RabbitMQ mechanics stay in
`internal/email/amqp` — the same split the delivered publisher already uses (`relay` owns the policy,
`amqp` owns the wiring). The worker's two ports (`DeliverySource` and `Republisher`) make the whole
failure matrix a table-driven suite with two fakes and no broker (WKR-27), and they keep the broker's
delivery structs out of the policy.

The destination names live in `internal/email` because they are domain vocabulary — "the short retry
tier", "the dead-letter route" — while the exchange and routing key behind each name belong to the
broker adapter that already declares them (`R3`). The result is that a routing key is spelled once, in
the package that owns the topology, and the worker's tests speak in destinations.

`cmd/worker` is a separate binary (the plan's decision O2): it owns the process lifecycle (the
fail-fast validation, the consume loop, the metrics listener, the graceful stop), so a broker outage
never reaches the HTTP API and the worker can be restarted or scaled independently. It needs no
database, unlike the relay, and the `Dockerfile` builds it into the runtime image beside `/app` and
`/relay`; nothing in `cmd/api.go` references the email domain, so the request-serving boot path is
untouched (EML-16).

Tests stay next to the code they cover (`*_test.go`), with the recording doubles in the delivered
`internal/email/emailtest` and the new mocks generated by mockery in `internal/email/worker/mocks`,
matching the repository convention.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
