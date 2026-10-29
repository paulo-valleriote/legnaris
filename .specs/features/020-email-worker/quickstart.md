# Quickstart: Validate the Email Worker

**Feature**: `.specs/features/020-email-worker/` | **Date**: 2026-10-21

This feature ships no HTTP endpoint. "Works" means: a queued code is turned into a delivered email
exactly once, an expired code is never delivered, a provider failure delays the code instead of
losing it, and the process survives a broker outage and a clean stop — all provable without a broker
and without a provider (fakes), plus one optional end-to-end run against a real broker.

## Prerequisites

- Go 1.26.4 (`go version`).
- Docker for the optional broker run (`make dev-db` starts MongoDB for the publisher; the worker itself
  needs no database).
- No environment variable is required for the unit and race gates.

## 1. The always-on gates

```bash
make fmt && make lint && make vet && make lint-specs   # linters + spec/test lint
make test-race                                         # the race detector must stay clean
```

Expected: clean. `go test ./...` must not need a broker or a provider (WKR-27); the broker-backed
suite self-skips and prints a skip notice when `RABBITMQ_URL` is absent.

## 2. Prove the delivery policy (WKR-02 … WKR-08)

```bash
go test ./internal/email/worker/... -run 'Worker|Delivery' -v
```

Expected:

- a valid message reaches the sending port with the rendered subject, both bodies and the message
  identity as the idempotency key, and is acknowledged only after the send succeeded;
- a body that is not a message, a non-canonical identity, a missing code and an unknown purpose are
  dead-lettered and acknowledged, and nothing is sent;
- an expired message (the injected clock past the deadline) is acknowledged with nothing sent and the
  expired counter moved;
- two consumers over one fake deliver each message once;
- a redelivered message is sent again and the provider's idempotency key is unchanged.

## 3. Prove the retry ladder (WKR-09 … WKR-15)

```bash
go test ./internal/email/worker/... -run 'Retry|Dead' -v
```

Expected:

- a retryable failure republishes to the 30-second tier with the attempt count incremented, and the
  original is acknowledged only after the republish was confirmed;
- the second failure uses the 2-minute tier and the later ones the 5-minute tier;
- an unconfirmed republish leaves the original unacknowledged;
- a permanent failure and an exhausted attempt cap reach the dead-letter route;
- a republished message keeps its identity, recipient, template, data and creation time.

## 4. Prove the broker surface without a broker (WKR-13, WKR-16, WKR-17, WKR-18)

```bash
go test ./internal/email/amqp/... -v
```

Expected:

- the delivery stream hands over one delivery at a time, and the acknowledgement and rejection paths
  reach the channel;
- the prefetch is applied before the consumer starts;
- the destination mapping matches the declared topology (the retry tiers and the dead-letter route);
- a stream that ends is re-registered rather than treated as a failure.

## 5. Prove the process behaviour (WKR-01, WKR-17 … WKR-20)

```bash
go test ./backend/cmd/worker/... -v
```

Expected:

- a blank or malformed broker address, an unknown provider name and a missing provider key each stop
  the process before it consumes anything, with a typed error;
- a stop signal ends the consumer, finishes the delivery in flight and leaves an unfinished one
  unacknowledged;
- the process starts with none of the worker's settings set, using the documented defaults.

## 6. Prove nothing leaks (WKR-06, WKR-21, WKR-22)

```bash
go test ./internal/email/... ./internal/platform/metrics/... -v
```

Expected: the suite-wide scan finds no code and no full address in any log line, error message, metric
series or rendered subject.

## 7. Optional: the whole path against a real broker

```bash
docker compose -f deploy/docker-compose.yml up -d rabbitmq        # the broker only
RABBITMQ_URL=amqp://legnaris:<password>@localhost:5672/ \
  go test ./internal/email/amqp/... -run Integration -v
```

Expected: the integration suite connects, declares the topology, publishes a message, consumes it and
acknowledges it. Without `RABBITMQ_URL` the suite skips itself.

To see the process work end to end, run the publisher (the relay) and the worker together with the
development sender (`EMAIL_PROVIDER=log`, no provider key needed): a message published by the relay
appears in the worker's log as a masked recipient, and the worker's counters move. The compose service
that runs the worker in the stack belongs to the next feature.

## References

- `contracts/worker.contract.md` — the delivery loop, the acknowledgement rules and the outcomes.
- `contracts/broker-consumer.contract.md` — the delivery source, the republishing and the
  reconnection.
- `contracts/destination.contract.md` — the four destinations and their mapping.
- `contracts/configuration.contract.md` — the settings and their defaults.
- `contracts/metrics.contract.md` — the counters.
- `data-model.md` — the message, the deadline, the attempt ladder and the settings.
