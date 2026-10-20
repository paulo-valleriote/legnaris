# Phase 0 Research: Email Delivery Infrastructure — Provider-Agnostic Sender, Embedded Templates, Durable Outbox and Relay Publisher

**Feature**: `.specs/features/016-email-infrastructure/` | **Date**: 2026-10-20

Every decision below answers an open question in the plan's Technical Context or makes explicit a
choice the program plan (`docs/plan-auth-otp-2fa-email.md` §4/§5/§9) and the spec leave to the
design. No `NEEDS CLARIFICATION` remains; the two product-level choices were closed in the spec's
`## Clarifications` (2026-10-20). Dependency APIs below were read from the pinned module sources
(`resend-go/v4@v4.9.0`, `amqp091-go@v1.16.0`), not from memory.

---

## R1. The email domain is one package with focused subpackages

**Decision**: a new `internal/email` domain plus five subpackages, each with one responsibility:

| Package | Responsibility | Imports |
|---|---|---|
| `internal/email` | `Message`, `Purpose`, `Data`, the `Sender` port, the classification sentinels, the embedded `Renderer`, the development sender | stdlib, `shared/logger` |
| `internal/email/resend` | the real provider adapter (SDK + status-capturing transport) | `internal/email`, `resend-go/v4` |
| `internal/email/outbox` | the durable intention: `Intent`, its lifecycle, the Mongo store, the index declarations | `shared/db`, `shared/idgen`, `shared/logger`, mongo driver |
| `internal/email/relay` | the `Publisher` port and the poll/claim/publish/backoff loop | `internal/email`, `internal/email/outbox` |
| `internal/email/amqp` | the RabbitMQ adapter: topology declaration and confirmed publish | `internal/email`, `amqp091-go` |
| `internal/email/emailtest` | the recording sender/publisher and shared fixtures | `internal/email` |

`cmd/relay` is the only wiring point.

**Rationale**: the message contract and the sender port are shared by two processes that must not
own each other (the relay publishes; the future worker sends), so they belong in a domain package
that both import. Keeping the broker, the provider and the store in separate adapter packages means
the relay loop is testable against interfaces with no service running, and each external dependency
is isolated to one package — the same shape the repository already uses for `internal/platform/*`
and `internal/sheet/sheettest`.

**Alternatives considered**:

- *One big `internal/email` package* — rejected: it would put the mongo driver, the AMQP client and
  the provider SDK in one import graph, so any test of the renderer would link the whole world.
- *Put the outbox in `internal/user` or `internal/auth`* — rejected: the intention is not an account
  or a session, and the future challenge feature would create a cycle.
- *Put the broker client in `internal/platform/amqp`* — rejected: `internal/platform/*` is documented
  as leaf cross-cutting middleware; a broker client that must know the `email.Message` shape would
  import the domain, so it lives with the domain instead.
- *A single `cmd/email` binary with a `ROLE` flag* — rejected by O2 (separate binaries isolate the
  API, relay and worker lifecycles).

## R2. The published message is one JSON contract owned by `internal/email`

**Decision**: `internal/email.Message` is:

```go
type Message struct {
    ID        string    `json:"id"`         // intention identity = provider idempotency key
    Type      string    `json:"type"`       // "email.otp"
    To        string    `json:"to"`
    Template  string    `json:"template"`   // = the purpose that selected the template
    Data      Data      `json:"data"`
    CreatedAt time.Time `json:"created_at"`
    Attempt   int       `json:"attempt"`    // send attempt; 0 on first publish, bumped by the worker
}
type Data struct {
    Code              string  `json:"code"`
    ExpiresInMinutes  int     `json:"expires_in_minutes"`
    Purpose           Purpose `json:"purpose"` // "verify_email" | "login_otp"
}
```

The relay encodes the intent into this shape with `Attempt: 0`; the `id` is also carried as the AMQP
`MessageId`. `Template` equals the intent's purpose, because the clarification fixed one embedded
template per purpose (EML-11).

**Rationale**: one type in one package means the relay (encoder), the broker adapter (publisher) and
the future worker (decoder) cannot drift; a golden-file test pins the wire shape (EML-07, EML-22).
`Attempt` is present from the start so the contract does not change when the worker begins bumping it
on a send retry (plan §5.5/§5.7): the outbox's own `attempts` counter counts **publish** retries and
is deliberately not the message's `attempt`.

**Alternatives considered**:

- *`map[string]any` payload* — rejected: untyped, untestable, and the code/expiry fields are
  contract.
- *Have the relay bump `attempt` from the outbox `attempts`* — rejected: it would conflate publish
  retries with send retries, so a republish after a crash would look like a failed send.
- *Encode with `encoding/json` in the broker adapter* — rejected: the contract belongs to the domain
  so the golden test can pin it without importing the broker.

## R3. The outbox claim is one atomic conditional write per row; attempts count publish retries only

**Decision**: `internal/email/outbox.Intent`:

```go
type Intent struct {
    ID            string     `bson:"_id"`
    To            string     `bson:"to"`
    Template      string     `bson:"template"`
    Payload       Payload    `bson:"payload"`   // code + expiry + purpose
    Status        Status     `bson:"status"`    // pending | published | failed
    Attempts      int        `bson:"attempts"`
    NextAttemptAt time.Time  `bson:"next_attempt_at"`
    LockedUntil   *time.Time `bson:"locked_until"`
    LockedBy      string     `bson:"locked_by"`
    CreatedAt     time.Time  `bson:"created_at"`
    PublishedAt   *time.Time `bson:"published_at"`
    LastError     string     `bson:"last_error"`
}
```

`Claim(claimer, now, lease, limit)` runs, per row, one
`findOneAndUpdate({status: pending, next_attempt_at: {$lte: now}, $or:[{locked_until: null},
{locked_until: {$lte: now}}]}, {$set: {locked_until: now+lease, locked_by: claimer}}, sort by
next_attempt_at ascending, returnDocument: after)`; it loops until it has `limit` rows or no row
matches. Claiming does **not** increment `attempts` — a claim is not a failed publication.

**Rationale**: Mongo standalone has no multi-document transaction (F-12), so the claim must be a
single-document conditional write; the `$or` on the lease makes a stopped publisher's claim expire
and the row eligible again, which is the "no lost email on crash" guarantee (EML-03, SC-001). Not
incrementing on claim keeps the attempt count equal to the number of *failed publications*, so the
cap means "five publish attempts" (EML-04).

**Alternatives considered**:

- *Read a batch, then `UpdateMany` a lease* — rejected: `UpdateMany` cannot return which rows were
  leased atomically, so two publishers could both think they own a row.
- *Increment `attempts` on claim* — rejected: a crashed publisher that never publishes would burn an
  attempt, and the cap would no longer mean "failed publications".
- *A separate lock collection* — rejected: two documents cannot be updated atomically on standalone,
  which is the exact problem.
- *Change streams* — unsupported on standalone (F-12).

## R4. Publication failure reschedules with a fixed tier schedule; the cap marks the row failed

**Decision**: a failed publication runs `Reschedule(id, now, nextAt, lastError)` with
`attempts = attempts+1`, `next_attempt_at = now + backoff(attempts)`, lease cleared; when
`attempts >= maxAttempts`, `MarkTerminal(id, now, lastError)` sets `status: failed` and the row is
never claimed again. The default backoff schedule is `[5s, 15s, 1m, 5m]` (the program plan's
schedule) and the default cap is 5; both are configuration with documented defaults. The schedule is
capped at its last entry for attempts beyond its length, so the cap always wins before the schedule
runs out.

**Rationale**: `EML-04` requires a bounded, increasing backoff and a terminal state; a fixed list is
trivially testable and matches O1's "backoff explícito na topologia" spirit without reading the
clock. Keeping the cap as the terminal condition means the failure is visible on the row (and
counted) instead of retrying forever.

**Alternatives considered**:

- *Exponential `2^n × base`* — rejected: unbounded growth makes the last wait unpredictable; O1
  already names discrete tiers for the broker side.
- *Reuse the attempt cap for the broker's retry tiers* — rejected: the broker's retry tiers govern
  *send* retries in the (deferred) worker; the outbox governs *publish* retries.

## R5. The Resend adapter wraps the official SDK and classifies with a status-capturing transport

**Decision**: `internal/email/resend` builds the SDK client with
`resend.NewCustomClient(&http.Client{Transport: classifier{base: transport}, Timeout: sendTimeout}, apiKey)`
and sends with `client.Emails.SendWithOptions(ctx, req, &resend.SendEmailOptions{IdempotencyKey: id})`.
The wrapped `http.RoundTripper` returns `(nil, *statusError{Status, Body})` for any non-2xx response
before the SDK's own error handling runs; the adapter maps:

| Observed | Classification |
|---|---|
| 2xx | success |
| 429, 5xx, network error, `context.DeadlineExceeded`, `net.Error` timeout | `ErrTransient` |
| 400, 422, any other 4xx | `ErrPermanent` |
| `*resend.MissingRequiredFieldsError`, `resend.ErrFailedToCreateEmailsSendRequest` | `ErrPermanent` (a malformed request cannot succeed on retry) |
| empty `apiKey` or `from` at construction | a startup error (the adapter refuses to build) |

**Rationale**: the spec's `EML-12` and US2 acceptance scenario 3 require a permanent 4xx to be
distinguishable from a retryable failure, but the pinned SDK (`resend.go:241-296`) maps 400/422 to a
plain `errors.New("[ERROR]: …")` and every other non-2xx to another plain error — only 429 survives
as `*resend.RateLimitError` (`errors.Is(err, resend.ErrRateLimit)`). Wrapping the transport recovers
the status the SDK throws away while keeping the SDK's request building, idempotency header and JSON
shape; `http.Client.Do` wraps the transport error in `*url.Error`, which `errors.As`/`errors.Is`
still unwrap.

**Alternatives considered**:

- *Use the SDK and classify conservatively (everything unknown is retryable)* — rejected: it
  violates `EML-12`/US2-AC3, and a permanently invalid recipient would be retried up to the cap.
- *Parse the SDK's `"[ERROR]: …"` string* — rejected: brittle string matching against an unversioned
  message.
- *Drop the SDK and call `POST https://api.resend.com/emails` directly with `net/http`* — viable and
  simpler in isolation, but rejected to keep the program plan's D5/§4.1 choice of the official SDK
  and its request/idempotency handling; the transport wrapper is ~30 lines and the SDK stays.
- *A custom `http.Client.CheckRedirect` or middleware* — no such seam; the transport is the only
  place that sees the raw response.

## R6. One sender port; failures are classified sentinels the caller acts on

**Decision**: `internal/email` exports:

```go
type Outbound struct { IdempotencyKey, To, Subject, Text, HTML string }
type Sender interface { Send(ctx context.Context, m Outbound) error }
var ErrTransient = errors.New("transient send failure")
var ErrPermanent = errors.New("permanent send failure")
```

The real adapter wraps its classification (`fmt.Errorf("%w: provider 429", ErrTransient)`) so callers
use `errors.Is`; the development sender always returns nil; `emailtest.Recorder` records the
`Outbound` and can be told to return a chosen sentinel.

**Rationale**: the port takes an already-rendered message (`Outbound`) because rendering is the
worker's job (plan §5.5) and keeping it out of the port lets the sender be tested with plain strings
and no template registry; the two sentinels are the entire classification surface the (deferred)
worker needs.

**Alternatives considered**:

- *The sender renders from the message* — rejected: it would couple the adapter to the template
  registry and duplicate rendering across adapters.
- *An `Outcome` enum return value* — rejected: Go's error idiom plus sentinels keeps the signature
  `error`-only and matches the repository's error-handling style.
- *A single `error` with no classification* — rejected by EML-12.

## R7. One embedded template per purpose; the subject never carries the code

**Decision**: `internal/email/templates/` holds six files — `verify_email.subject.txt`,
`verify_email.body.txt`, `verify_email.body.html`, `login_otp.subject.txt`, `login_otp.body.txt`,
`login_otp.body.html` — embedded with `//go:embed templates/*` and parsed with `html/template` for
HTML and `text/template` for the subject/text (the HTML body is the only file parsed by
`html/template`, so the code is escaped there and verbatim in text). `Renderer.Render(purpose, data)`
returns `Rendered{Subject, Text, HTML}`; an unknown purpose returns `ErrUnknownTemplate`. The copy is
English; the subject names the product and the action ("Your Legnaris verification code") but never
the code; both bodies carry the code and its lifetime ("expires in 10 minutes").

**Rationale**: `EML-11` and US2-AC1 require both bodies to carry the code and the subject never to;
one template per purpose (user decision) gives each purpose its own copy and its own test target.
Embedding removes any runtime file dependency from a distroless image.

**Alternatives considered**:

- *One shared template* — rejected by the clarification.
- *`text/template` for HTML* — rejected: it would not escape the (numerically generated, but
  nonetheless untrusted-in-principle) code in the HTML body.
- *Templates in the database or object storage* — rejected: no requirement, and it would add a
  failure mode to a code-delivery path.

## R8. Broker topology: direct exchange, quorum work queue, three delay-tier retry queues, one DLQ

**Decision**: `internal/email/amqp.DeclareTopology` declares idempotently (all durable):

| Entity | Kind / args | Binding |
|---|---|---|
| `legnaris.email` | direct exchange, durable | — |
| `email.otp` | quorum (`x-queue-type: quorum`), `x-max-length`, `x-overflow: reject-publish`, DLX `legnaris.email.retry`, DLX key `30s` | bound to `legnaris.email` key `email.otp` |
| `legnaris.email.retry` | direct exchange, durable | — |
| `email.retry.30s` | quorum, `x-message-ttl: 30000`, DLX `legnaris.email`, DLX key `email.otp` | bound to `legnaris.email.retry` key `30s` |
| `email.retry.2m` | quorum, `x-message-ttl: 120000`, DLX/back | bound key `2m` |
| `email.retry.5m` | quorum, `x-message-ttl: 300000`, DLX/back | bound key `5m` |
| `email.dead` | quorum, durable | bound to `legnaris.email` key `email.dead` |

The routing key for a fresh publish is `email.otp`. The relay declares the topology before its first
publish and fails fast if declaration fails.

**Rationale**: O1 chose TTL + DLX per tier over the native delayed retry so the deploy does not
depend on a specific broker minor; separate delay queues avoid head-of-line blocking (plan §5.4).
The RabbitMQ 4.x feature matrix confirms quorum queues support message TTL (per-queue
`x-message-ttl`) and dead-letter exchanges, so the whole topology is quorum — durable and
poison-message aware. `x-overflow: reject-publish` is used instead of `drop-head` so a full queue
refuses (and nacks) new messages rather than silently dropping the oldest, and
`reject-publish-dlx` is avoided because quorum queues do not support it. The relay's own retry tiers
are driven by the *worker* on a transient send failure; the queue-level DLX is the fallback.

**Alternatives considered**:

- *Native quorum delayed retry (4.x)* — rejected (O1): couples the deploy to a broker version.
- *The `x-delayed-message` plugin* — rejected (O1): an extra plugin to install.
- *A single retry queue with per-message `expiration`* — rejected: a 5-minute message would block a
  30-second one behind it (head-of-line blocking).
- *Classic queues for the retry tiers* — allowed by the plan ("quorum/classic"), rejected for
  uniformity; quorum queues support the args used here.
- *`x-overflow: drop-head`* — rejected: silently dropping the oldest queued OTP is the worst outcome.

## R9. Publishing is confirmed, mandatory and blocked-aware

**Decision**: the broker adapter holds one connection (`amqp.DialConfig(url, Config{Recovery: &amqp.Recovery{}})`),
one publish channel (`Confirm(false)`), a `NotifyReturn` channel and a `NotifyBlocked` channel, and
per publish calls `PublishWithDeferredConfirmWithContext(ctx, "legnaris.email", "email.otp", true,
false, Publishing{ContentType: "application/json", DeliveryMode: Persistent, MessageId: msg.ID,
Body: body})` then `WaitContext(confirmCtx)`. Success requires: the publish call did not error, the
confirmation acked, and — checked before returning — no basic.return was recorded for that delivery.
An unacked confirmation, a transport error, a return, or a blocked connection is `ErrTransient`; the
relay reschedules. `NotifyStateChange` is drained by a supervisor goroutine; the relay logs state
changes and does not exit.

**Rationale**: `EML-02`/`EML-07` require "marked published only after the broker confirms routing";
a publisher confirm alone only says the broker accepted the message, so `mandatory=true` +
`NotifyReturn` is required to catch an unroutable message (plan §5.6). `Recovery` is available in
the pinned `amqp091-go` `Config`, so the connection reconnects without bespoke code; the supervisor
exists to drain `NotifyStateChange` (the library documents that not draining it leaks goroutines).

**Alternatives considered**:

- *Fire-and-forget `Publish`* — rejected: it cannot satisfy "marked published only after confirm".
- *`NotifyPublish` + a manual delivery-tag map* — rejected: `PublishWithDeferredConfirm` already
  correlates the confirmation, and rebuilding the map is error-prone.
- *Treat a blocked connection as fatal* — rejected: it is a transient resource condition; the relay
  should back off and retry.

## R10. Configuration is optional with documented defaults; the broker is required only by the relay

**Decision**: `shared/env.Variables` gains, all optional:

| Variable | Default | Consumer |
|---|---|---|
| `EMAIL_PROVIDER` | `log` | sender selection (`log` \| `resend`); unknown value fails the sender build |
| `RESEND_API_KEY` | empty | required by the `resend` adapter constructor |
| `EMAIL_FROM` | `Legnaris <no-reply@legnaris.local>` | the `resend` adapter |
| `EMAIL_SEND_TIMEOUT` | `10s` | the sender adapters |
| `RABBITMQ_URL` | empty | required by `cmd/relay` (fail fast with `ErrBrokerUnavailable`) |
| `EMAIL_RELAY_POLL_INTERVAL` | `1s` | the relay loop |
| `EMAIL_RELAY_LEASE` | `30s` | the claim |
| `EMAIL_RELAY_BATCH` | `25` | the claim |
| `EMAIL_RELAY_CONFIRM_TIMEOUT` | `5s` | the publish wait |
| `EMAIL_RELAY_MAX_ATTEMPTS` | `5` | the terminal cap |
| `EMAIL_OUTBOX_RETENTION` | `168h` | the `published_at` TTL |
| `EMAIL_RELAY_METRICS_ADDR` | `:9100` | the relay's metrics listener |

`RequiredKeys()` stays exactly `{DATABASE_URI, JWT_SECRET}`; the relay validates `RABBITMQ_URL`
itself. `EMAIL_PROVIDER=resend` with an empty `RESEND_API_KEY` fails when the sender is built (the
worker's concern, but the constructor is here and tested).

**Rationale**: `EML-14`/`EML-15` require optional-with-defaults config that does not change the
required list and is validated by the process that needs it; this is why `RABBITMQ_URL` is empty by
default and the relay — not the API — rejects a blank value.

**Alternatives considered**:

- *Make `RABBITMQ_URL` required* — rejected by EML-14/EML-16 and the compose required-variable test.
- *Default `EMAIL_PROVIDER=resend`* — rejected: a missing key would fail sends in dev; `log` is the
  safe default and production sets `resend` explicitly.
- *One `EMAIL_*` prefix for everything* — rejected for the relay knobs, which are relay-scoped and
  would otherwise read as provider settings.

## R11. The relay exposes its own metrics listener

**Decision**: `internal/platform/metrics` gains `email_outbox_backlog` (gauge),
`email_publish_total{result}` (counter) and `email_dead_total` (counter, reserved for the worker),
plus a `Handler() http.Handler` returning the registry's `promhttp` handler. `cmd/relay` serves
`Handler()` on `EMAIL_RELAY_METRICS_ADDR` with `net/http`; the relay updates the backlog gauge each
pass and increments the publish counter per outcome. The API's `/metrics` (Echo) is unchanged.

**Rationale**: `EML-23`/SC-001 require an observable backlog and publish outcomes; the relay is a
separate process with no HTTP server, so it needs a listener of its own, and reusing the existing
registry keeps one metrics namespace. `prometheus.yml` scrapes the relay listener on the internal
network.

**Alternatives considered**:

- *Log-only observability* — rejected: `EML-23` says "so a stalled pipeline can be alerted on", and
  alerting belongs on a metric.
- *Give the relay an Echo server* — rejected: Echo is unnecessary weight for one endpoint.
- *Write the backlog into Mongo and expose it elsewhere* — rejected: more moving parts, no benefit.

## R12. Deliberately out of this feature

**Decision**: the following stay out, and the plan says why:

| Out | Reason |
|---|---|
| the worker/consumer that reads the broker, checks expiry and calls the sender | user scoping decision (stage 2 "as listed"); the broker consumer is the next feature |
| the challenge/HTTP endpoints and the API-side enqueue | program stage 3; this feature ships the `Create` seam with tests and no HTTP caller |
| the daily provider-quota guard (the request path's `503`) | program stage 3's request path owns the status; this feature only classifies a rate limit as transient |
| the SMTP adapter | the plan makes it conditional; only the real provider and the dev/test doubles |
| domain authentication (SPF/DKIM/DMARC) and bounce/complaint webhooks | operational tasks, not this code deliverable |
| the broker management UI on `ops.` | program stage 6 (O6) |
| broker-side metrics (`rabbitmq_prometheus`) | needs the broker deployed; stage 6 |
| a Mongo replica set | program plan: a future option, out of the MVP |

## Resolved unknowns

| Technical Context item | Resolution |
|---|---|
| Package layout | `internal/email` + `{resend,outbox,relay,amqp,emailtest}` + `cmd/relay` (`R1`) |
| Message contract | one `Message`/`Data` in `internal/email`, golden-pinned; `attempt` starts at 0 (`R2`) |
| Claim semantics | one conditional write per row, no attempt bump on claim (`R3`) |
| Backoff and cap | fixed tiers `5s/15s/1m/5m`, cap marks `failed` (`R4`) |
| Provider error classification | SDK wrapped by a status-capturing transport (`R5`) |
| Sender port and errors | `Outbound` in, `ErrTransient`/`ErrPermanent` out (`R6`) |
| Templates | one per purpose, embedded, text+html, code-free subject (`R7`) |
| Broker topology | direct exchange, quorum work queue, 3 delay tiers, DLQ; `reject-publish` overflow (`R8`) |
| Publish durability | confirm + mandatory + NotifyReturn + NotifyBlocked + Recovery (`R9`) |
| Configuration surface | 12 optional variables, defaults documented, required set unchanged (`R10`) |
| Relay observability | own `net/http` metrics listener, backlog + publish outcomes (`R11`) |
| Deferred scope | worker, challenge endpoints, quota guard, SMTP, domain config, management UI (`R12`) |