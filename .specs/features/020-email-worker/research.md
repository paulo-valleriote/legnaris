# Research: The Email Worker

**Feature**: `.specs/features/020-email-worker/` | **Requirements**: `WKR-01` … `WKR-27` | **Date**:
2026-10-21

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §5.6 (publisher and consumer), §5.7 (retry), §5.9
(observability), §5.10 (tests) and §9 stage 6 (the worker as a deployed process) — decisions D5, D6,
O1, O2.

This document records the design decisions the spec leaves open. Every decision was checked against
the code the earlier features delivered: `internal/email` (the message contract, the sender port, the
embedded templates), `internal/email/amqp` (the topology and the confirmed publish), `internal/email/relay`
(the publication loop and its ports), `internal/email/factory` (the provider selection),
`internal/platform/metrics`, `shared/env`, `shared/idgen` and `shared/json`.

## R1. Where the consumer lives

**Decision**: the consumer is a new package `internal/email/worker`, driven by a new binary
`backend/cmd/worker`, and the RabbitMQ specifics stay in `internal/email/amqp` (extended with a
consumer channel and a routed publish).

**Rationale**: the worker is email-domain policy — decode, decide whether the code is still worth
sending, render, send, decide the retry ladder — and it belongs beside the message contract it
consumes (`internal/email`) and the publisher that feeds it (`internal/email/relay`). The broker
vocabulary (channels, prefetch, delivery tags, confirm) stays in the adapter, exactly as the
publisher's vocabulary does today.

**Alternatives considered**:

- *Inside `internal/email/amqp`* — rejected: it would mix transport with policy, and the package's
  current seam is a publisher; the worker needs a consumer of a different shape.
- *A new top-level `internal/worker`* — rejected: the worker has no domain of its own; it speaks
  only the email contract, and a second top-level domain would duplicate the layering rule.
- *Extend `internal/email/relay`* — rejected: the relay polls a store with a lease and a backoff;
  the worker is driven by the broker and has no store at all (the clarification of 2026-10-21).

## R2. The two ports the worker speaks

**Decision**: the worker depends on two interfaces it owns:

- a **delivery source** that hands it one delivery at a time (a body, a delivery tag and the means to
  acknowledge or reject it);
- a **republisher** that publishes one message to a named destination and reports success only when
  the broker confirmed it.

**Rationale**: the whole failure matrix — send succeeded, send failed retryably, send failed
permanently, republish unconfirmed, body undecodable, code expired — is then a table-driven unit
suite with two fakes, no broker and no provider (WKR-27). The ports mirror the shape the delivered
`relay.Publisher` already has, so the two halves of the pipeline read alike.

**Alternatives considered**:

- *One combined broker port* (`Deliver` + `Ack` + `Publish` on one interface) — rejected: it would
  make the fake carry broker mechanics, and the two ports have different failure owners.
- *Consume `amqp.Delivery` directly* — rejected: it leaks the broker's delivery struct into the
  policy and makes the fake a broker emulator.

## R3. Where the destination vocabulary lives

**Decision**: the four destinations — the three delay tiers and the dead-letter route — are named in
`internal/email` (the domain) as a small `Destination` type; `internal/email/amqp` maps a
destination to the exchange and routing key it already owns.

**Rationale**: the worker decides *policy* ("a first retry waits in the short tier"), and the adapter
owns *wiring* ("the short tier is the `30s` routing key on the retry exchange"). Putting the names in
the domain keeps the worker's tests readable and stops a routing key from being spelled twice in two
packages — the failure mode that 016's topology already guards against for the publisher.

**Alternatives considered**:

- *Export the routing keys from `internal/email/amqp` and let the worker import them* — rejected: the
  worker would depend on the broker adapter for a policy decision, and the adapter's vocabulary would
  leak into the domain.
- *One queue for every retry with a per-message delay* — rejected by the plan (decision O1): a single
  queue makes a five-minute message block the thirty-second ones behind it (head-of-line blocking).

## R4. The expiry decision

**Decision**: the deadline is the message's own — its creation time plus its advertised lifetime,
compared against an injected clock — and the worker never consults the challenge store.

**Rationale**: this is the clarification of 2026-10-21 and it is also what makes the worker a
process without a database (WKR-01): the lifetime travels with the message, so the decision needs no
lookup and the process starts without MongoDB. The consequence — a code the person already typed may
still be delivered once — is harmless, because the code is single-use.

**Alternatives considered**:

- *Read the challenge before sending* — rejected: it adds a database dependency, a lookup per message
  and a second source of truth for the deadline; it also turns a broker outage into a store outage.
- *Trust the challenge's absolute deadline instead of the message's* — rejected: the message states
  its own lifetime in the email the person reads ("the code expires in N minutes"), and the email and
  the worker must never disagree about it.

## R5. The retry ladder

**Decision**: the attempt count on the message is the number of send attempts already made. A
retryable failure republishes the message to the tier that matches the attempt count — 30 seconds for
the first retry, 2 minutes for the second, 5 minutes for every later one — with the count
incremented, and the original is acknowledged only after the broker confirmed the republish. A
message whose count already reached the cap (5) is dead-lettered without another send.

**Rationale**: this is the plan's decision O1 expressed against the topology the earlier feature
declared: the three delay queues already exist with their TTLs, so the ladder is a mapping from an
attempt count to a destination, not new machinery. Keeping the count on the message (rather than
reading the broker's dead-letter history) is the plan's explicit instruction and it survives a
redelivery: a message that comes back from a delay tier still knows how many attempts it has had.

**Alternatives considered**:

- *Rely on `x-death` headers* — rejected: the plan forbids depending on them, and they are a broker
  implementation detail rather than a contract.
- *Requeue the original instead of republishing it* — rejected: a requeued message is retried
  immediately with no delay, so a provider outage becomes a hot loop against the provider.
- *Let the broker's dead-letter route do the retry* — rejected: the work queue's dead-letter route
  points at the first delay tier, so a rejected message would be retried on a fixed 30-second
  schedule with no attempt count and no cap.

## R6. Dead-lettering is a confirmed publish

**Decision**: dead-lettering publishes the message to the dead-letter route and acknowledges the
original only after the broker confirmed the publish; an unconfirmed dead-letter leaves the message
unacknowledged so the broker redelivers it.

**Rationale**: the same "never lose it" rule that governs the retry must govern the last resort: a
message that cannot be dead-lettered is still owned by the broker, and a message that is dead-lettered
is provably in the operator's queue. Acknowledging first and publishing second would drop the message
in the window between the two.

**Alternatives considered**:

- *Reject the original so the broker dead-letters it* — rejected: the work queue's dead-letter route
  points at the *first delay tier*, not at the dead-letter queue, so a rejection would schedule a
  retry instead of parking the message.
- *Acknowledge first, publish afterwards* — rejected: it loses the message when the publish fails.

## R7. The broker adapter's consumer surface, and who reconnects

**Decision**: `internal/email/amqp` gains a consumer channel (`Qos` + `Consume` on a channel of its
own) and a publish that takes an explicit destination. The connection is opened with the library's
recovery enabled — the pinned `amqp091-go` v1.16.0 re-establishes the connection and re-registers
consumers and topology automatically — and the worker adds the supervisor loop that re-opens a channel
and re-registers the consumer when the delivery stream ends (a broker-side cancel, or a channel that
the library could not recover).

**Rationale**: the library already does the hard half (reconnect plus consumer re-registration), so
the worker must not re-implement a reconnect loop; but a library that reconnects the *connection* does
not guarantee that the *channel* the worker holds is usable again, so the supervisor loop is the
worker's own and is testable with a fake delivery source that ends its stream. Two channels — one for
consuming, one for publishing — because a channel is not safe for concurrent use and the consumer
channel carries unacknowledged deliveries.

**Alternatives considered**:

- *One channel for both roles* — rejected: `amqp091-go` channels are not concurrency-safe, and mixing
  the confirm channel with a live consumer makes the publisher's confirm state ambiguous.
- *A goroutine in the API process* — rejected by the plan's decision O2 and already refused for the
  relay: the worker's lifecycle, its backpressure and its restarts are its own.

## R8. One bound on the work in flight

**Decision**: a single setting bounds how many messages the worker holds at once; it is the broker's
prefetch, and the handler pool is sized from it (the plan's §5.6 says `prefetch=10`).

**Rationale**: one number with one meaning — "how many messages may be in flight" — is the whole
backpressure story. A separate concurrency setting would let the two disagree (a large prefetch with a
small pool stalls the broker; the reverse overruns the provider). The default follows the plan.

**Alternatives considered**:

- *Separate prefetch and concurrency settings* — rejected: two knobs for one property, and the
  failure mode is a stalled or an unbounded pipeline.
- *Unbounded consumption* — rejected: a provider that stops answering would then have an unbounded
  number of messages in memory.

## R9. Graceful shutdown

**Decision**: on a stop signal the worker stops taking new messages, waits for the messages already in
flight for a bounded budget, acknowledges the ones that finished, leaves the rest unacknowledged and
closes its resources.

**Rationale**: an unacknowledged message is redelivered by the broker, so the worst case of a restart
is a duplicate send, which the provider de-duplicates by the message identity (WKR-08). The budget is
bounded by the send timeout, because a send that has not answered cannot be waited for forever.

**Alternatives considered**:

- *Wait without a budget* — rejected: a provider that never answers would keep the process alive
  forever, and a supervisor would then kill it mid-flight.
- *Cancel the in-flight sends on the stop signal* — rejected: it converts a shutdown into a lost send
  for no benefit; the message would be redelivered anyway.

## R10. Decoding the message body

**Decision**: the body is decoded through a new `shared/json` helper that decodes a bounded byte slice
*without* rejecting unknown fields, and the worker then validates the fields it needs explicitly (the
identity is a canonical UUID v7, the code is present, the purpose selects a template, the lifetime is
usable).

**Rationale**: the constitution routes JSON decoding through `shared/json`, but the package today
only decodes an HTTP request body and rejects unknown fields. A broker message is not an HTTP body, so
the HTTP decoder does not apply; and rejecting unknown fields would dead-letter every message produced
by a newer request-serving process during a rolling deploy — the message contract may only grow. The
explicit field checks are what the requirements actually need (WKR-04, WKR-05, WKR-07).

**Alternatives considered**:

- *`encoding/json` directly, as the delivered publisher does* — rejected: it would break Principle V
  for no benefit, and the shared helper is three lines.
- *`shared/json` with unknown-field rejection* — rejected: it makes the worker and the request-serving
  process have to be deployed in lockstep, which the plan already warns about only for the template
  version.
- *Validate the body against a JSON schema* — rejected: no schema validator is in the stack, and the
  field checks the requirements need are five explicit rules.

## R11. The counters

**Decision**: the worker's metrics extend `internal/platform/metrics`: a send-outcome counter by
result, a send-duration histogram, the reuse of the existing dead-letter counter, and a separate
expired counter. They are served by the worker's own listener, like the relay's.

**Rationale**: the plan's §5.9 names these series, and the counters have to exist before the next
feature's alert rules can read them. The existing package is the only place a metric name is defined,
so the worker does not open a second registry; the dead-letter counter already exists and is used
nowhere, so it becomes the worker's.

**Alternatives considered**:

- *A registry inside the worker package* — rejected: two metric registries in one process family, and
  the names would drift from the ones the API exposes.
- *No metrics in this feature* — rejected: the next feature's alerts would have nothing to read, and
  a pipeline that cannot be observed is discovered by people who cannot sign in.

## R12. The configuration surface

**Decision**: three new optional settings — the in-flight bound, the send-attempt cap and the metrics
address — with documented defaults; the provider settings (`EMAIL_PROVIDER`, `RESEND_API_KEY`,
`EMAIL_FROM`, `EMAIL_SEND_TIMEOUT`) and `RABBITMQ_URL` are reused unchanged, and the deployment's
required-variable list is unchanged.

**Rationale**: the worker is a new process, so it needs its own knobs; everything it shares with the
publisher (the broker address, the provider) must not drift into a second definition. The metrics
address gets its own default because the relay already listens on `:9100` and two processes on one
host cannot share a port.

**Alternatives considered**:

- *One generic `EMAIL_WORKER_*` blob* — rejected: the repository's convention is one variable per
  knob with a documented default.
- *Reusing the relay's metrics address* — rejected: two processes on one host, one port.

## R13. What the worker reuses unchanged

**Decision**: the worker uses `email.Renderer` (the embedded templates), `email.Sender` (the port),
`email.MaskRecipient` (the log masking), `factory.New` (the provider selection), `email.Message` (the
wire contract) and `idgen.Validate` (the identity check) exactly as they are.

**Rationale**: the earlier features delivered these seams with their own tests; a second rendering
path or a second masking helper would be two sources of truth for the same fact. The worker adds
policy, not capability.

**Alternatives considered**:

- *A worker-local renderer that also decides the subject* — rejected: the subject is code-free by
  contract (EML-13) and the templates are the single place it is written.

## R14. Deliberately out of this feature

**Decision**: the following stay out, and the reason is the same as in the earlier features:

| Out | Reason |
|---|---|
| the compose service, the broker's management UI behind the ops. gateway, the Prometheus scrape and the alert rules | program stage 6's deployment slice; the next feature |
| a dead-letter replay tool | clarification of 2026-10-21: the route is for inspection; replay is a separate operator concern |
| the SMTP adapter | the plan makes it conditional; the real provider plus the development and test doubles are enough |
| a daily send cap at the worker | already enforced where the code is requested (OTP-13) |
| the request-serving process | it still never talks to the broker or the provider (EML-16) |
| the request-serving process's own graceful shutdown | the plan's finding F-14 applies there too, but that is not this feature's change |

## Resolved unknowns

| Technical Context item | Resolution |
|---|---|
| Package layout | `internal/email/worker` + `cmd/worker`, with `internal/email/amqp` extended (`R1`) |
| Worker ports | a delivery source and a republisher, both owned by the worker (`R2`) |
| Destination names | a domain-owned `Destination` type in `internal/email` (`R3`) |
| Expiry rule | the message's own creation time plus its advertised lifetime, against an injected clock (`R4`) |
| Retry ladder | 30s / 2m / 5m by attempt count, cap 5, confirmed republish before the acknowledgement (`R5`) |
| Dead-lettering | a confirmed publish to the dead-letter route, then the acknowledgement (`R6`) |
| Reconnection | the library's recovery for the connection, the worker's supervisor loop for the consumer (`R7`) |
| Backpressure | one in-flight bound, default 10, from the plan's §5.6 (`R8`) |
| Shutdown | stop consuming, bounded wait for in-flight, leave the rest unacknowledged (`R9`) |
| Decoding | a new bounded `shared/json` byte-slice decoder without unknown-field rejection (`R10`) |
| Metrics | `email_send_total{result}`, `email_send_duration_seconds`, the existing `email_dead_total`, an expired counter, on the worker's own listener (`R11`) |
| Configuration | three optional settings; the provider and broker settings are reused (`R12`) |
| Reused seams | renderer, sender port, masking, factory, message contract, `idgen.Validate` (`R13`) |
| Deferred scope | the deployment slice, the replay tool, SMTP, the request-serving process (`R14`) |
