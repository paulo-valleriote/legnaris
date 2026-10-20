# Contract: Published message

**Feature**: `.specs/features/016-email-infrastructure/` | **Requirement IDs**: `EML-02`, `EML-07`,
`EML-18`, `EML-22` | **Owner**: `internal/email` (shape), `internal/email/relay` (encode),
`internal/email/amqp` (publish)

## Purpose

One wire shape handed to the broker, pinned by a golden file, so the relay that encodes it, the
broker adapter that publishes it and the future worker that decodes it cannot drift.

## Shape

```go
type Message struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    To        string    `json:"to"`
    Template  string    `json:"template"`
    Data      Data      `json:"data"`
    CreatedAt time.Time `json:"created_at"`
    Attempt   int       `json:"attempt"`
}

type Data struct {
    Code             string  `json:"code"`
    ExpiresInMinutes int     `json:"expires_in_minutes"`
    Purpose          Purpose `json:"purpose"`
}
```

Normative JSON (the golden file):

```json
{
  "id": "018f4a1e-5c6b-7d8e-9f01-23456789abcd",
  "type": "email.otp",
  "to": "player@example.com",
  "template": "login_otp",
  "data": {
    "code": "042917",
    "expires_in_minutes": 10,
    "purpose": "login_otp"
  },
  "created_at": "2026-10-20T12:00:00Z",
  "attempt": 0
}
```

## Rules

- `ID` is the intention's canonical UUID v7 and is also the AMQP `MessageId` (EML-18).
- `Type` is the constant `email.otp` for this feature.
- `Template` is the purpose string that selected the template (`verify_email` | `login_otp`).
- `Data.Purpose` repeats the purpose; `Data.Code` is the short-lived credential.
- `Attempt` is the **send** attempt: the relay publishes `0`; the worker increments it when it
  republishes to a retry tier. The outbox's own `attempts` (publish retries) is a different counter
  and never appears on the wire.
- AMQP properties: `ContentType: application/json`, `DeliveryMode: Persistent`, `MessageId: id`.

## Invariants

- The JSON body always satisfies the golden file's shape and field order is irrelevant.
- The code never appears in the AMQP properties (only inside `data.code`).
- An intention whose identity is not a canonical UUID v7 is never encoded or published (EML-18).

## Boundary cases

| Case | Result |
|---|---|
| 6-digit code with a leading zero | preserved as a string (`"042917"`) |
| attempts beyond the first publish | `attempt` stays `0` from the relay |
| unknown `purpose` on an intention | the relay still publishes; the renderer rejects it later (worker) |

## Test evidence

- `internal/email/message_test.go`: golden-file equality, leading-zero code, `MessageId` derivation,
  and the code absent from the encoded properties.
- `internal/email/relay/relay_test.go`: the encoded message carries the intention identity and `0`.