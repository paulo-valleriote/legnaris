# Contract: The broker consumer surface

**Feature**: `.specs/features/020-email-worker/` | **Requirement IDs**: `WKR-13`, `WKR-16`, `WKR-17`,
`WKR-18` | **Owner**: `internal/email/amqp`

## Purpose

The RabbitMQ side of the worker: how a delivery is obtained, how it is acknowledged or rejected, and
how a message is republished to a destination. The topology itself is delivered — the main exchange,
the quorum work queue, the three delay-tier queues and the dead-letter queue are already declared
(`EML-06`); this contract fixes the *consumer* surface and the destination mapping.

## The delivery source

The adapter exposes one delivery at a time. The worker's policy never sees a broker struct.

```go
// Delivery is one message handed to the worker.
type Delivery struct {
    // Body is the raw message body.
    Body []byte
    // Delivery identifies the delivery for acknowledgement and rejection.
    Delivery DeliveryHandle
}

// DeliveryHandle acknowledges or rejects one delivery.
type DeliveryHandle interface {
    // Ack acknowledges the delivery.
    Ack() error
    // Nack rejects the delivery without requeueing it.
    Nack() error
}
```

- The consumer runs on a **channel of its own**: the publisher's confirm channel and the consumer's
  channel are different, because a channel is not safe for concurrent use and the consumer's channel
  carries unacknowledged deliveries.
- The consumer channel sets the **prefetch** to the worker's in-flight bound before it starts
  consuming (`Qos(prefetchCount, 0, false)`).
- Consumption is **manual acknowledgement** (`autoAck=false`): the delivery is acknowledged only after
  the worker decided what to do with it (WKR-02).
- The consumer tag is unique per process, so two workers never share one and a broker-side cancel can
  be told apart from a stream that simply ended.

## The delivery stream

```go
// DeliverySource hands the worker one delivery at a time. The stream ends when
// the broker cancels the consumer or the channel is closed.
type DeliverySource interface {
    // Consume registers the consumer and returns the delivery stream.
    Consume(ctx context.Context) (<-chan Delivery, error)
}
```

- The stream **ends** when the broker cancels the consumer (`basic.cancel`) or the channel closes. The
  worker treats the end of the stream as "re-register and continue", not as a fatal error (WKR-17).
- A registration failure is reported to the caller; the worker retries it with a short backoff until
  the stop signal arrives.

## Republishing to a destination

```go
// Republisher publishes one message to one destination and reports success
// only once the broker confirmed it accepted the message for routing.
type Republisher interface {
    // PublishTo publishes the message to the destination and waits for the broker's confirmation.
    PublishTo(ctx context.Context, destination email.Destination, msg email.Message) error
}
```

- The publish is **mandatory** and **persistent**, exactly like the publisher's, and it waits for the
  publisher confirm with the configured timeout. A nack, a timeout, a transport error, a blocked
  connection or an unroutable message is a failure, never a success (WKR-13).
- The destination vocabulary is owned by `internal/email` (`destination.contract.md`); this adapter
  maps it to the exchange and routing key it already declares.

| Destination | Exchange | Routing key | Queue it lands in |
|---|---|---|---|
| `DestinationRetry30s` | the retry exchange | `30s` | the 30-second delay tier |
| `DestinationRetry2m` | the retry exchange | `2m` | the 2-minute delay tier |
| `DestinationRetry5m` | the retry exchange | `5m` | the 5-minute delay tier |
| `DestinationDeadLetter` | the main exchange | `email.dead` | the dead-letter queue |

- Each delay tier's message TTL dead-letters it back to the main exchange, so a message that waits in
  a tier returns to the work queue on its own — no worker is involved in the waiting (the plan's
  decision O1).

## Reconnection

- The connection is opened with the library's recovery enabled, so the connection itself, the declared
  topology and the registered consumers are re-established by the library when the broker comes back.
- The worker owns the loop above it: when the delivery stream ends, the worker re-registers the
  consumer (and the adapter re-opens the channel if the old one is gone) and continues consuming
  (WKR-17).
- A publish that fails because the broker is gone is a failure, not a success: the worker leaves the
  original unacknowledged and the broker redelivers it (WKR-13).

## Shutdown

- On a stop signal the adapter cancels the consumer (`basic.cancel`), which ends the delivery stream;
  deliveries that were never acknowledged stay with the broker (WKR-18).
- Closing the adapter closes the consumer channel and the publisher channel and then the connection.

## Boundary cases

| Case | Result |
|---|---|
| the broker cancels the consumer | the stream ends; the worker re-registers |
| the channel closes mid-delivery | the acknowledgement fails; the message stays with the broker |
| the broker is blocked by a memory or disk alarm | the publish fails as transient; the original stays unacknowledged |
| the broker cannot route the republished message | the publish fails as transient (mandatory routing) |
| the confirm times out | the publish fails as transient; the original stays unacknowledged |
| two workers consume the same queue | the broker delivers each message to one of them |

## Test evidence

- `internal/email/amqp/consuming_test.go`: the delivery stream, the acknowledgement and rejection
  paths, the stream ending, the re-registration and the destination mapping, driven against the
  package's `channel` seam — no broker required.
- `internal/email/amqp/topology_test.go`: the destination mapping matches the declared topology.
- An optional integration suite, self-skipping when `RABBITMQ_URL` is absent, drives the real broker.
