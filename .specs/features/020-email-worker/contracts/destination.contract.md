# Contract: The destination vocabulary

**Feature**: `.specs/features/020-email-worker/` | **Requirement IDs**: `WKR-09`, `WKR-11`, `WKR-12` |
**Owner**: `internal/email`

## Purpose

One name for each place a message can go, owned by the domain and mapped to the broker's exchange and
routing key by the adapter that already declares them. The worker's policy says "the short retry tier";
it never says a routing key.

## Shape

```go
// Destination names where a message goes. The broker adapter maps each name
// to the exchange and routing key it owns.
type Destination string

const (
    // DestinationRetry30s is the first retry: about half a minute.
    DestinationRetry30s Destination = "retry_30s"
    // DestinationRetry2m is the second retry: two minutes.
    DestinationRetry2m Destination = "retry_2m"
    // DestinationRetry5m is every later retry: five minutes.
    DestinationRetry5m Destination = "retry_5m"
    // DestinationDeadLetter is where a message that can never succeed is parked.
    DestinationDeadLetter Destination = "dead_letter"
)
```

## Rules

- The vocabulary is closed: exactly four names, and the worker never builds one from a string.
- The names are domain words, not broker words: a routing key appears in exactly one place, the
  adapter's mapping (`research.md` R3).
- The worker's intent is a **route**; the broker's entity is a **queue**. The two words describe the
  same thing from each side — the route the worker names, the queue the message lands in.
- The worker queue is not a destination: the worker only publishes *away* from it.

## Mapping

| Destination | Exchange | Routing key |
|---|---|---|
| `DestinationRetry30s` | the retry exchange | `30s` |
| `DestinationRetry2m` | the retry exchange | `2m` |
| `DestinationRetry5m` | the retry exchange | `5m` |
| `DestinationDeadLetter` | the main exchange | `email.dead` |

## Boundary cases

| Case | Result |
|---|---|
| an unknown destination | the publish is refused with a typed error; nothing is published |
| the same destination published twice | two independent messages; the provider de-duplicates by the message identity |

## Test evidence

- `internal/email/destination_test.go`: the four names are stable and the adapter's mapping matches
  the declared topology.
