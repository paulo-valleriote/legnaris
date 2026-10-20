# Implementation Plan: Email Delivery Infrastructure — Provider-Agnostic Sender, Embedded Templates, Durable Outbox and Relay Publisher

**Branch**: `016-email-infrastructure` | **Date**: 2026-10-20 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/016-email-infrastructure/spec.md`

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §9 stage 2 — decisions D5 (Resend), D6 (RabbitMQ
+ outbox), O1 (TTL + DLX retry), O2 (relay as a separate process), O4 (free tier). This feature is
the delivery-infrastructure slice of the 2FA/email program; it deliberately ships no HTTP endpoint,
no consumer/worker, no challenge logic and no verification gate.

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Build the asynchronous email-delivery foundation the later 2FA features sit on, in three seams:

1. **A provider-agnostic sender.** One `Sender` port takes a rendered message; a real adapter
   delivers over HTTPS with an idempotency key, a bounded timeout and a retryable/permanent
   classification; a development adapter makes no network call; an in-memory recorder backs the
   tests. Embedded per-purpose templates (`verify_email`, `login_otp`) render a code and its
   lifetime into a plain-text and an HTML body, and never into the subject.
2. **A durable outbox.** A new `email_outbox` collection records the send intention (identity,
   recipient, template, payload, lifecycle, attempts, claim) before anyone is told the request was
   accepted. Claiming is one atomic conditional write per row, so a crash, two publishers, or a
   broker outage never lose the intent; a duplicate publish is harmless because the identity is the
   provider's idempotency key.
3. **A relay publisher.** A separate `cmd/relay` process polls the outbox, claims a batch with a
   lease, publishes to RabbitMQ with publisher confirm + mandatory routing, marks success, and
   applies bounded backoff on failure. It declares the whole broker topology (main exchange, quorum
   work queue, delay-tier retry queues, dead-letter queue) idempotently at startup, exposing the
   pending backlog and publish outcomes for alerting, and shutting down gracefully.

The change is confined to the Go backend: a new `internal/email` domain with its adapters and
stores, one new binary (`cmd/relay`), `shared/env`, `shared/db`, `internal/platform/metrics`, and
the deployment image/compose/Prometheus/Makefile. **Two new third-party dependencies**
(`github.com/resend/resend-go/v4`, `github.com/rabbitmq/amqp091-go`) and **no new required
environment variable** (`env.RequiredKeys()` is unchanged; the broker is required only by the
relay).

## Technical Context

**Language/Version**: Go 1.26.4 (`go.mod` `go 1.26.4`; `go version` confirms `go1.26.4 linux/amd64`)
on Echo v5.3.1 (`github.com/labstack/echo/v5 v5.3.1`), MongoDB driver v2.9.1
(`go.mongodb.org/mongo-driver/v2 v2.9.1`), Prometheus client v1.24.1, `testify` v1.12.1 — the pins
the repository already carries. Two additions, both verified available: `github.com/resend/resend-go/v4`
**v4.9.0** (latest published) and `github.com/rabbitmq/amqp091-go` **v1.16.0** (latest published).

**Primary Dependencies**: the two additions above plus the existing stack.
`resend-go/v4` exposes `NewCustomClient(httpClient, apiKey)`, `Client.Emails.SendWithOptions(ctx,
*SendEmailRequest, *SendEmailOptions{IdempotencyKey})` and `errors.Is(err, resend.ErrRateLimit)`;
`amqp091-go` exposes `DialConfig(url, Config{Recovery, …})`, `Connection.NotifyStateChange`,
`Connection.NotifyBlocked`, `Channel.Confirm(false)`, `Channel.NotifyReturn`,
`Channel.PublishWithDeferredConfirmWithContext` and `DeferredConfirmation.WaitContext` — all checked
against the pinned module source. The domain uses `shared/idgen` (UUID v7), `shared/logger`,
`shared/env` and `shared/db`; `go:embed` + `text/template`/`html/template` render the templates.
**No dependency is added beyond these two.**

**Storage**: MongoDB 7.0 standalone (`deploy/docker-compose.yml`), no transactions or change streams
(F-12). One new collection, `email_outbox`, with two indexes: `{status, next_attempt_at}` for the
claim query and a TTL on `published_at` (retention default 7 days) so published rows retire without
touching pending or failed ones. Durability is per-document: the store is written before the request
is answered (EML-01); the account/challenge-before-intent ordering is the API-side feature's
concern. No existing collection or index changes.

**Testing**: `go test ./...` and `go test -race ./...` (the constitution keeps the race detector
clean; the relay suite runs two publishers and two claims concurrently). Table-driven suites with
`t.Run` and `Happy path:` / `Edge:` / `Error:` prefixes, `require` for preconditions and `assert`
otherwise, mockery mocks (`NewMockX(t)`, `make mocks` after an interface change) for interfaces and
ordinary fakes for the recording doubles. Pure unit suites for the message contract (golden file),
the renderer, the outbox planner/claim semantics, the relay loop and the error classification; a
`resend` suite drives the adapter against an `httptest` server through the real SDK; an `amqp`
suite drives the broker against a fake `Channel`-shaped collaborator; Mongo-dependent behavior lives
in self-skipping integration suites. Nothing in `go test ./...` requires a broker (EML-20, SC-008).

**Target Platform**: the Go backend on Linux. `cmd/relay` joins the internal network only, beside
`mongodb` and the new `rabbitmq` service; it publishes no host port (the broker's management UI is
out of scope — program stage 6, O6). The relay exposes its metrics on an internal listener so
Prometheus can scrape the backlog and publish outcomes.

**Project Type**: backend web service plus operator/background binaries in the same Go module; the
`web/` client is untouched (the HTTP contract does not change).

**Performance Goals**: no throughput target. One publish pass is O(batch) broker confirms; one
claim is one conditional write per row; the backlog gauge is one `countDocuments` per pass. The
relay polls on the order of a second (configurable) and confirmations have a bounded wait, so a
stalled broker cannot pin the loop.

**Constraints**: `env.RequiredKeys()` and `deploy/compose_test.go` keep validating an unchanged
required list (EML-14); the broker config is validated by the relay, not the API (EML-15); the API
does not depend on the broker to start (EML-16); every new failure mode is a typed sentinel with a
documented effect and the relay acts on it exactly (EML-19); codes never reach a log, an error or a
subject (EML-13); identifiers are canonical UUID v7 from `shared/idgen` (EML-18); layering stays
`cmd/relay → internal/email/{relay,outbox,amqp,resend} → internal/email → shared/*` with no upward
import; all source and strings are English (EML-21); `go test -race` stays clean (EML-20).

**Scale/Scope**: 3 user stories (2 P1, 1 P2), 23 requirements, 6 internal contracts; one new domain
with 6 packages, one new binary, roughly 35 touched or created files across `internal/email`,
`cmd/`, `shared/{env,db}`, `internal/platform/metrics`, `deploy/`, `Dockerfile`, `Makefile` and
`.mockery.yaml`; about 16 new or extended test suites.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The email domain is a non-HTTP layer consumed by the relay (and, later, the worker). Its failures are exported typed sentinels — `ErrTransient`/`ErrPermanent` (EML-12), `ErrUnknownTemplate` (EML-11), `ErrInvalidIntentID` (EML-18), `ErrBrokerUnavailable` (EML-15) — each with a documented effect, and `relay` acts on them exactly: retry with backoff, fail the intention, or stop (EML-19). No HTTP boundary is added, so no `apperr` code is introduced; the sentinels mirror `internal/user`'s `ErrInvalidEmail` and `shared/db`'s own typed errors, which the constitution's layering already tolerates. | PASS |
| II. Canonical Identifiers | Every outbox intention identity is a canonical lowercase UUID v7 from `shared/idgen.New`; a claim validates each read identity with `idgen.Validate` and a non-canonical one is rejected rather than published (EML-18). The identity doubles as the provider idempotency key (EML-09). | PASS |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | Every requirement lands with tests in the same commit, edge cases first (crash-between-publish-and-mark, two concurrent claims, broker nack/timeout, unroutable message, empty outbox, attempt cap, malformed payload/id, unknown template, provider 429/5xx/4xx/timeout, blank broker URL, published-only retention), then a representative happy path per suite. Failure paths assert the typed sentinel and the relay's action; `go test -race` stays clean (EML-20, SC-008). | PASS |
| IV. English-Only Source | All identifiers, comments, log messages and the email copy are English; the templates and the relay report are English too (EML-21). | PASS |
| V. Layered Architecture | `internal/email` is a new leaf domain importing only the standard library and `shared/*`; `internal/email/{outbox,relay,amqp,resend}` depend downward on it and on `shared/*`, never on `internal/app` or `internal/sheet`; `cmd/relay` wires them with `shared/env`, `shared/db` and `shared/logger`. Logging goes through `shared/logger`; the only production decode path (the broker consumer) is deferred, so the `shared/json` decoding rule is not exercised here. | PASS |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | The code never logs the code, masks the recipient, and carries no secret: `RESEND_API_KEY`/`RABBITMQ_URL` come from the environment (never committed), the broker publishes over the internal network only, and the code-carrying messages carry a MessageId + short TTL. No gate is weakened. | PASS (directly served) |
| VII. Installable, Offline-Capable Web Client | No client surface changes; the HTTP contract, routes and cookie attributes are untouched, so the PWA's existing flows keep working. | PASS (not involved) |
| Additional Constraints — Stack | Go on Echo with MongoDB and `shared/env` for configuration, `testify` + mockery tests — unchanged. This feature adds a broker client and an email client for a background pipeline; the choice is documented here and in `research.md` (`R1`, `R5`, `R10`) and is entered as an `AD-*` decision in `.specs/STATE.md` when the dependencies land, following the `AD-016` precedent (a client library added without a constitution amendment). No principle is waived. | PASS (decision documented) |
| Additional Constraints — Spec corpus and traceability | `EML-01`…`EML-23` are defined in `spec.md`; `scripts/check_requirement_ids.py` scans `*.go`, so the citations in the delivered sources resolve. | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card; recorded for the PR, not claimed by this artifact. | PASS (noted) |
| Development Workflow | The backend sequence (`make fmt && make lint && make vet && make lint-specs`, then `make test-race`) covers the change. `Dockerfile` builds the relay binary, `Makefile` gains the relay target and the optional variables, `deploy/compose_test.go` keeps validating `RequiredKeys()` against the stack, and `prometheus.yml` scrapes the new internal listener. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps one implementation per rule:
the message contract is built once in `internal/email` and reused by the relay, the broker adapter
and the golden test; the sender classification is defined once (the sentinels plus the Resend
adapter's status mapping) and consumed by the (later) worker; the outbox state transitions live only
in `internal/email/outbox` and the relay never writes Mongo directly; and the broker topology is
declared once in `internal/email/amqp`. Two choices that could have drifted were resolved in
research: the Resend adapter wraps the official SDK (not a raw client) because the SDK's own error
handling collapses 4xx and 5xx and the classification is a requirement (`R5`), and the relay exposes
its own metrics listener rather than depending on the HTTP server's `/metrics` (`R11`).

## Project Structure

### Documentation (this feature)

```text
.specs/features/016-email-infrastructure/
├── spec.md                     # Requirements (EML-*)
├── plan.md                     # This file
├── research.md                 # Phase 0 — decisions and rejected alternatives
├── data-model.md               # Phase 1 — intent, message, template, topology, configuration
├── quickstart.md               # Phase 1 — runnable validation guide
├── contracts/                  # Phase 1 — one contract per internal seam
│   ├── message.contract.md
│   ├── sender.contract.md
│   ├── templates.contract.md
│   ├── outbox.contract.md
│   ├── relay-broker.contract.md
│   └── configuration.contract.md
├── checklists/
│   └── requirements.md         # Spec quality checklist
└── tasks.md                    # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/relay/
├── main.go                             # NEW: wiring, poll loop, graceful shutdown, metrics listener (EML-05, EML-15, EML-16)
└── main_test.go                        # NEW: run seam, fail-fast on a bad broker URL, SIGTERM drains in-flight

internal/email/                         # NEW domain: message contract, sender port, templates, dev sender
├── message.go                          # NEW: Message, Purpose, Data, the JSON contract (EML-07, EML-18)
├── sender.go                           # NEW: Sender port, Outbound, ErrTransient/ErrPermanent (EML-08…EML-12)
├── template.go                         # NEW: embedded Renderer; ErrUnknownTemplate (EML-11)
├── templates/                          # NEW: verify_email.{subject.txt,body.txt,body.html}, login_otp.* (EML-11)
├── testdata/message.golden.json        # NEW: the pinned wire contract (EML-22)
├── sender_log.go                       # NEW: development sender, no network call (EML-10, EML-13)
├── message_test.go                     # NEW: golden file, contract fields, code never in the subject
├── sender_test.go                      # NEW: port contract, classification sentinels
├── template_test.go                    # NEW: per-purpose rendering, unknown purpose, code-free subject
└── sender_log_test.go                  # NEW: no network, masked recipient, code never logged

internal/email/resend/                  # NEW adapter: the real provider
├── sender.go                           # NEW: SendWithOptions + idempotency key + timeout (EML-09)
├── transport.go                        # NEW: http.RoundTripper mapping non-2xx to a typed status error (R5)
├── sender_test.go                      # NEW: 200/429/5xx/4xx/timeout against an httptest provider
└── transport_test.go                   # NEW: status capture, body limit, close

internal/email/outbox/                  # NEW store: the durable intention
├── intent.go                           # NEW: Intent, Status lifecycle, NewIntent (EML-01, EML-18)
├── repository.go                       # NEW: Create, Claim, MarkPublished, Reschedule, MarkTerminal, Backlog (EML-03, EML-04, EML-17)
├── repository_test.go                  # NEW: Mongo, self-skip: unique claim, lease expiry, backoff, retention
├── intent_test.go                      # NEW: lifecycle invariants, invalid identity
└── mocks/                              # NEW: mockery-generated Repository

internal/email/relay/                   # NEW service: poll, claim, publish, backoff
├── publisher.go                        # NEW: Publisher port (implemented by email/amqp and by fakes) (EML-02, EML-07)
├── relay.go                            # NEW: pass, backoff schedule, attempt cap, backlog observer (EML-02…EML-06, EML-23)
├── relay_test.go                       # NEW: success/failure/terminal, crash-between, two relays, empty outbox
└── mocks/                              # NEW: mockery-generated Publisher

internal/email/amqp/                    # NEW adapter: RabbitMQ publisher + topology
├── broker.go                           # NEW: DialConfig(Recovery), DeclareTopology, confirmed mandatory publish (EML-06, EML-07)
├── topology.go                         # NEW: exchange/queues/retry tiers/DLQ arguments (EML-06)
├── broker_test.go                      # NEW: confirms, nack, unroutable return, blocked, reconnect state
└── topology_test.go                    # NEW: the declared entity set and its arguments

internal/email/emailtest/               # NEW test double: the fake sender/publisher and fixtures
├── recorder.go                         # NEW: recording Sender and Publisher, injectable failures (EML-10)
└── fixtures.go                         # NEW: intent/message builders for suites

internal/platform/metrics/
└── metrics.go                          # EXTENDED: outbox backlog gauge, publish counter, Handler() for the relay (EML-23)

shared/env/
├── env.go                              # EXTENDED: optional email/broker variables, documented defaults (EML-14)
└── env_test.go                         # EXTENDED: absent/blank/valid values; RequiredKeys unchanged

shared/db/indexes.go                    # EXTENDED: email_outbox claim index + published TTL (EML-17)

deploy/
├── docker-compose.yml                  # EXTENDED: rabbitmq service (internal only) + relay service (EML-16)
├── .env.example                        # EXTENDED: documented optional email/broker variables
├── compose_test.go                     # EXTENDED: broker has no published port; the relay is a separate service; required set unchanged
└── prometheus/prometheus.yml           # EXTENDED: scrape the relay listener

Dockerfile                              # EXTENDED: build /relay into the runtime image
Makefile                                # EXTENDED: export the optional variables; relay target
.mockery.yaml                           # EXTENDED: email Sender/Renderer, outbox Repository, relay Publisher/Observer
```

**Structure Decision**: the message contract and the sender port live in `internal/email` because
both the relay (publishing) and the future worker (sending) must agree on one shape, and neither
should own the other; the package imports only the standard library and `shared/*`, so it can be
exercised with plain values and no services.

The durable intention lives in `internal/email/outbox`, beside the collection it owns: the claim is
one atomic conditional write per row, which is exactly where Mongo's per-document atomicity is used
(F-12), and the transition rules (`pending → published`, `pending → pending` with backoff,
`pending → failed`) have a single home. The API-side feature will call `Create`; this feature ships
the seam with tests and no HTTP caller.

The relay loop lives in `internal/email/relay`, declared against interfaces (`Publisher` and the
outbox store) so the whole failure matrix — confirm, nack, timeout, unroutable, crash-between,
concurrent claims — is unit-tested with fakes and no broker. The RabbitMQ specifics live in
`internal/email/amqp` (topology + confirmed publish) and the provider specifics in
`internal/email/resend` (idempotency + timeout + classification), so a provider or broker swap is an
adapter swap.

`cmd/relay` is a separate binary (O2): it owns the process lifecycle (env validation fail-fast,
Mongo + broker connections, the metrics listener, SIGTERM draining), so a broker outage can never
reach the HTTP API and the relay can be restarted or scaled independently. The Dockerfile builds it
into the runtime image beside `/app` and `/canonicalize-emails`; nothing in `cmd/api.go` references
the email domain, so the API boot path is untouched (EML-16).

Tests stay next to the code they cover (`*_test.go`), with the recording doubles and fixtures in
`internal/email/emailtest` (following `internal/sheet/sheettest`) and MongoDB-dependent behavior in
self-skipping integration suites, matching the repository convention.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |