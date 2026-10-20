# Quickstart: Validate the Email Delivery Infrastructure

**Feature**: `.specs/features/016-email-infrastructure/` | **Date**: 2026-10-20

This feature ships no HTTP endpoint. "Works" means: the message contract is pinned, the renderer and
sender classify correctly, the outbox claims atomically, and the relay publishes confirmed without a
running broker (fakes) — plus one optional end-to-end run against a real broker and Mongo.

## Prerequisites

- Go 1.26.4 (`go version`).
- Docker for the optional broker/DB run (`make dev-db` starts MongoDB; the broker is optional).
- No environment variable is required for the unit/race gates.

## 1. The always-on gates

```bash
make fmt && make lint && make vet && make lint-specs   # linters + spec/test lint
make test-race                                         # the race detector must stay clean
```

Expected: clean. `go test ./...` must not need Mongo or a broker; Mongo-backed suites self-skip and
print `mongo unavailable — skipping integration test` (EML-20, SC-008).

## 2. Prove the message contract and the templates (EML-07, EML-11, EML-22)

```bash
go test ./internal/email/... -run 'Message|Template' -v
```

Expected:

- the golden file `internal/email/testdata/message.golden.json` matches the encoded message exactly;
- for each known purpose, the subject does **not** contain the code while both bodies do, and the
  lifetime appears in both bodies;
- an unknown purpose renders nothing and returns `ErrUnknownTemplate`.

## 3. Prove the sender classification (EML-09, EML-12)

```bash
go test ./internal/email/resend/... -v
```

Expected (against an `httptest` provider — no real key, no network):

- 200 → success, and the provider received the `Idempotency-Key` header equal to the intention id;
- 429 / 5xx / timeout → `errors.Is(err, ErrTransient)` is true;
- 400 / 422 → `errors.Is(err, ErrPermanent)` is true;
- empty `apiKey` or `from` → the adapter refuses to build.

## 4. Prove the outbox and the relay with Mongo (EML-01…EML-06, EML-17, EML-18)

```bash
make dev-db           # MongoDB on localhost:27017 (uses DATABASE_URI from .env)
go test ./internal/email/outbox/... ./internal/email/relay/... -v
```

Expected:

- two concurrent `Claim` calls never return the same row; a lease expiry makes a row claimable again;
- `Reschedule` increments `attempts` and pushes `next_attempt_at`; the cap marks the row `failed`;
- a crash between publish and mark is simulated by publishing and *not* marking: the next pass
  republishes the same identity and marks it published;
- the backlog count matches the number of pending rows;
- a non-canonical `_id` is rejected and never published;
- the outbox indexes exist after boot (`EnsureIndexes`).

## 5. Prove the relay's process behaviour without a broker (EML-15, EML-19)

```bash
go test ./cmd/relay/... -v
```

Expected:

- an empty or unparseable `RABBITMQ_URL` makes the process fail fast with `ErrBrokerUnavailable`
  before it polls;
- a cancelled context lets the in-flight batch finish and the process exit cleanly.

## 6. Optional: run the real pipeline against Mongo + RabbitMQ

```bash
# Start MongoDB and RabbitMQ (the compose stack's broker service publishes no host port;
# use `docker compose ... exec` or map a local port for this manual run).
docker run --rm -d --name legnaris-rabbit -p 5672:5672 rabbitmq:4-management

export DATABASE_URI="mongodb://localhost:27017/legnaris"
export RABBITMQ_URL="amqp://guest:guest@localhost:5672/"
export EMAIL_PROVIDER=log          # the dev sender: no provider call

go run ./cmd/relay                 # declares the topology, then polls the outbox
```

Insert one intention (through the outbox `Create` seam or a small `mongosh` document with a canonical
UUID v7 `_id`), and verify:

- the relay logs a `published` outcome and marks the row `published` (`published_at` set);
- `GET http://localhost:9100/metrics` shows `legnaris_email_outbox_backlog 0` and
  `legnaris_email_publish_total{result="published"} 1`;
- the message sits in `email.otp` (inspect via the broker's management UI, not published to the host).

Kill the relay mid-publish and restart it: the same identity is republished and the provider would
de-duplicate it — the pipeline loses nothing.

## 7. Optional: validate the deployment surface

```bash
go test ./deploy/... -v
docker compose -f deploy/docker-compose.yml config >/dev/null   # from deploy/, with a filled .env
```

Expected: the broker service has no published host port and a healthcheck; the relay is a separate
service; the app's required-variable list is unchanged (EML-16).

## What "done" looks like

| Check | Evidence |
|---|---|
| No lost intent on crash | `SC-001` — relay suite, crash-between case |
| No double publish | `SC-002` — outbox suite, two concurrent claims |
| Contract pinned | `SC-003` — message golden file |
| Classification complete | `SC-004` — sender + resend suites |
| No code leaked | `SC-005` — template + sender-log suites |
| Required set unchanged | `SC-006` — env + compose suites |
| Broker not publicly exposed | `SC-007` — compose suite |
| Race-clean | `SC-008` — `make test-race` |