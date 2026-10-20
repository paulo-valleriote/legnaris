# Contract: Sender port and its adapters

**Feature**: `.specs/features/016-email-infrastructure/` | **Requirement IDs**: `EML-08`…`EML-13` |
**Owner**: `internal/email` (port + sentinels), `internal/email/resend` (real adapter),
`internal/email` (`LogSender`), `internal/email/emailtest` (recorder)

## Purpose

One port through which every caller sends a rendered email, with a failure classification the caller
can act on, and adapters that keep the provider, the development path and the tests apart.

## Port

```go
type Outbound struct {
    IdempotencyKey string
    To             string
    Subject        string
    Text           string
    HTML           string
}

type Sender interface {
    Send(ctx context.Context, m Outbound) error
}

var (
    ErrTransient = errors.New("transient send failure")
    ErrPermanent = errors.New("permanent send failure")
)
```

Normative classification: an adapter returns `nil` on success, an error wrapping `ErrTransient` when
a retry may succeed, and an error wrapping `ErrPermanent` when it cannot. Callers decide with
`errors.Is`.

## Adapters and outcomes

| Adapter | Construction | `Send` behaviour |
|---|---|---|
| `internal/email/resend` | `New(apiKey, from string, timeout time.Duration) (*Sender, error)`; empty `apiKey`/`from` → error | `client.Emails.SendWithOptions(ctx, req, &resend.SendEmailOptions{IdempotencyKey: m.IdempotencyKey})`; classification in the table below |
| `internal/email.LogSender` | `NewLogSender(log)` | writes one structured line (id, purpose, masked recipient) and returns `nil`; **no network call** |
| `internal/email/emailtest.Recorder` | `NewRecorder()` | records the `Outbound`; returns an injected sentinel when configured |

## Resend classification (`http.RoundTripper` wraps the SDK transport)

| Observed | Sentinel |
|---|---|
| 2xx | success |
| 429, 5xx | `ErrTransient` |
| network error, `context.DeadlineExceeded`, `net.Error` timeout | `ErrTransient` |
| 400, 422, any other 4xx | `ErrPermanent` |
| `*resend.MissingRequiredFieldsError`, `resend.ErrFailedToCreateEmailsSendRequest` | `ErrPermanent` |
| empty `apiKey`/`from` at construction | a build error (fail fast, not a send outcome) |

## Invariants

- `IdempotencyKey` is the intention identity on every send (EML-09, EML-18).
- The per-send timeout bounds every adapter call; a timeout is `ErrTransient`.
- No adapter writes the code or the full recipient to a log or an error (EML-13).
- No unit test invokes the real provider; the `resend` suite drives the SDK against an `httptest`
  server (EML-10).

## Boundary cases

| Case | Result |
|---|---|
| provider answers 429 with a `Retry-After` header | `ErrTransient` (the header is not trusted for scheduling; the relay's backoff decides) |
| provider answers 500 / 502 / 503 | `ErrTransient` |
| provider answers 400 / 422 (invalid recipient) | `ErrPermanent` |
| request context already cancelled | the SDK returns immediately; classified `ErrTransient` |
| empty `IdempotencyKey` | still sent, but the caller contract forbids it; a test pins that the port carries the identity |
| development adapter | 0 network calls, 0 errors |

## Test evidence

- `internal/email/sender_test.go`: the port contract and the sentinel classification helpers.
- `internal/email/resend/sender_test.go` (httptest): 200 success; 429/5xx/timeout → `ErrTransient`;
  400/422 → `ErrPermanent`; the `Idempotency-Key` header is sent; empty key/from → build error.
- `internal/email/resend/transport_test.go`: status capture, body truncation, body close.
- `internal/email/sender_log_test.go`: no network, masked recipient, code never logged.
- `internal/email/emailtest` + relay suites: the recorder and injected sentinels.