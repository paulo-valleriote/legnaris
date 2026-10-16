# Contract: Transport seam

**Requirements**: API-07, API-29, API-31…API-33 | **Backing document**:
[research.md R1, R5, R11](../research.md)

The transport is the only way the client reaches the network, and the only place a credential will ever
be attached. This contract is what the fake transport in the test suite implements and what the native
adapter maps onto the platform.

## Request

| Field | Type | Required | Rules |
|---|---|---|---|
| `url` | `string` | yes | absolute; resolved base address joined with the operation path; the path identifier is percent-encoded |
| `method` | `'GET' \| 'POST' \| 'PATCH'` | yes | no other method exists in this feature |
| `headers` | `Readonly<Record<string, string>>` | yes | `content-type: application/json` present exactly when a body is present, absent otherwise |
| `body` | `string` | only for `POST`/`PATCH` | JSON text; the transport must not log it |
| `credentials` | `'omit'` | yes | fixed value — no ambient credential is attached implicitly, in any environment |
| `signal` | `AbortSignal` | yes | aborted on timeout expiry or caller cancellation |

## Response

| Field | Type | Rules |
|---|---|---|
| `status` | `number` | the HTTP status as received |
| `body` | `string` | the raw response text, undecoded; an empty string is allowed and becomes a defensive parse failure |

## Obligations

1. The client constructs exactly one transport per instance; no operation creates its own (API-07).
2. Every operation issues its request through this transport and nothing else — the client never calls
   the platform `fetch` directly.
3. A transport **rejection** means the transport itself failed (network, timeout, abort) and is the only
   condition that can trigger a read retry; a transport **resolution** — any status, including 4xx/5xx —
   ends the attempt (API-31).
4. Writes never repeat, whatever the failure (API-32).
5. The transport receives an already-absolute URL: base-address resolution happens before the transport
   is called, so a transport never needs to know about configuration.
6. No field of the request may be written to a log by the client (API-29); the module contains no
   logging call at all.

## Native adapter

The production adapter maps the request onto the platform request API one-to-one: `url`, `method`,
`headers`, `body`, `credentials` and `signal` are passed through unchanged, and the response status and
text are read back. The adapter is the only module in `web/src/api/` that touches a browser global, so
the rest of the layer is testable without a DOM.

## Test obligations

- A fake transport records every request, so the suite asserts method, path, headers, body and
  `credentials: 'omit'` per operation.
- The retry suite asserts the number of transport invocations: two for a read that fails at the
  transport, one for a read answered with a failure status, one for any write failure.
- A source-scanning test asserts `web/src/api/` contains no `console.*` call and imports no logger
  (API-29).
