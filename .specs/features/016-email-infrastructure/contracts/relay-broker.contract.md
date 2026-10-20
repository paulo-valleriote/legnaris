# Contract: Relay and broker

**Feature**: `.specs/features/016-email-infrastructure/` | **Requirement IDs**: `EML-02`…`EML-07`,
`EML-15`, `EML-19`, `EML-23` | **Owner**: `internal/email/relay` (loop), `internal/email/amqp`
(topology + publish), `cmd/relay` (lifecycle)

## Purpose

Move stored intentions to the broker at least once, confirming each, and back off cleanly on failure
— with the broker specifics isolated behind one port so the loop is testable without a broker.

## Ports

```go
// internal/email/relay
type Publisher interface {
    Publish(ctx context.Context, msg email.Message) error
}

type Observer interface {
    PublishResult(result string) // "published" | "retried" | "failed"
    BacklogPending(n int)
}

type Config struct {
    PollInterval   time.Duration
    Lease          time.Duration
    Batch          int
    ConfirmTimeout time.Duration
    MaxAttempts    int
    Backoff        []time.Duration
    Now            func() time.Time
    ClaimerID      string
}
```

## Relay loop

```text
tick (PollInterval)
  ├─ rows := store.Claim(claimerID, now, Lease, Batch)      // atomic per row
  ├─ observer.BacklogPending(store.Backlog())               // gauge
  └─ for each row:
       ├─ id invalid?            → store.MarkTerminal(id, now, "invalid identity"); observer "failed"
       ├─ publisher.Publish(...) → store.MarkPublished(id, now); observer "published"
       ├─ error, attempts+1 < cap → store.Reschedule(id, now, now+backoff(attempts+1), err); observer "retried"
       └─ error, attempts+1 >= cap → store.MarkTerminal(id, now, err); observer "failed"
```

After the pass, `WaitContext(ConfirmTimeout + margin)`-bounded publishes are already done (the port
blocks); the loop then sleeps until the next tick. `Run(ctx)` returns when `ctx` is cancelled, after
the in-flight batch finishes (graceful shutdown, EML-05).

## Broker adapter (`internal/email/amqp`)

- `Dial(url string, cfg Config) (*Broker, error)` → `amqp.DialConfig(url, amqp.Config{Recovery:
  &amqp.Recovery{}})`, then one channel with `Confirm(false)`, `NotifyReturn`, and a
  `NotifyBlocked`/`NotifyStateChange` drain goroutine.
- The connection and channel are reached through the `channel` seam declared in
  `internal/email/amqp/channel.go`: a small interface over the operations the adapter uses, plus an
  injectable dial function value, so the publish, nack, timeout, `basic.return` and blocked paths are
  unit-tested against a fake with no broker running (`EML-07`, `EML-19`).
- `DeclareTopology()` declares the entities in `data-model.md` §4 idempotently — including
  `email.otp`'s `x-max-length: 10000` and its 80%-of-limit alert — and is called once at startup
  before the first publish (EML-06).
- `Publish(ctx, msg)`:
  1. marshal `msg` (the message contract);
  2. `PublishWithDeferredConfirmWithContext(ctx, "legnaris.email", "email.otp", true, false,
     Publishing{ContentType, DeliveryMode: Persistent, MessageId: msg.ID, Body})`;
  3. `conf.WaitContext(confirmCtx)`; failure → `ErrTransient`;
  4. a recorded `NotifyReturn` for this delivery → `ErrTransient` (unroutable, EML-07);
  5. otherwise `nil`.
- `Close()` drains and closes the channel and connection.

## Failure classification and the relay's action

| Condition | Class | Relay action |
|---|---|---|
| publish confirmed + routed | — | `MarkPublished` |
| broker nack, confirm timeout, blocked, transport error | `ErrTransient` | `Reschedule` or `MarkTerminal` at the cap |
| unroutable (`basic.return`) | `ErrTransient` | `Reschedule` (topology/queue may come back) |
| non-canonical identity | `ErrInvalidIntentID` | `MarkTerminal` |
| blank/unparseable `RABBITMQ_URL` at startup | `ErrBrokerUnavailable` | fail fast, never poll |

## Invariants

- An intention is never marked published without a positive confirmation (EML-02, SC-003).
- The topology is declared before the first publish; a declaration failure stops startup.
- The relay never writes Mongo directly; all state moves through the outbox port.
- The confirm wait is bounded independently of the broker; a stalled broker cannot pin the loop.

## Boundary cases

| Case | Result |
|---|---|
| broker down when the relay starts | fail fast with `ErrBrokerUnavailable`, no polling |
| broker down mid-run | publishes return transient; rows reschedule; no loss |
| confirm arrives with `Ack=false` | transient; retried |
| message unroutable (no binding) | `basic.return` detected; transient; retried |
| connection blocked (memory/disk alarm) | publish refused; transient; retried after unblock |
| context cancelled mid-pass | the current row finishes or is left leased; `Run` returns |
| two relays against one outbox | the claim gives each row to one; a lease-expired crash row is re-claimed |
| queue full (`x-max-length`) | publish nacks; transient; the outbox retains the intent |

## Test evidence

- `internal/email/relay/relay_test.go`: success, transient, terminal at the cap, invalid identity,
  two concurrent relays, empty outbox, context cancellation.
- `internal/email/amqp/topology_test.go`: the declared entity set and arguments.
- `internal/email/amqp/broker_test.go`: a fake `Channel`-shaped collaborator drives confirm/nack/
  return/blocked; no broker required for `go test ./...`.
- `cmd/relay/main_test.go`: fail-fast on a bad URL; a cancelled context drains in-flight work.