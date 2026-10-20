# Contract: Client address and trusted proxy policy

**Feature**: `.specs/features/015-auth-prerequisites/` | **Requirement IDs**: `PRQ-01`…`PRQ-04` |
**Owner**: `internal/platform/clientip` (rule), `internal/app` (wiring)

## Purpose

Define which address every per-client decision uses when the API is reached only through the edge
proxy, without letting a request choose its own key.

## Inputs

| Input | Source | Rules |
|---|---|---|
| Direct peer | `http.Request.RemoteAddr` | Host portion; a bare address is accepted; IPv4-mapped IPv6 is unmapped |
| Forwarding chain | Every `X-Forwarded-For` header value, in request order, split on `,` | Entries may carry IPv6 brackets and `:port` |
| Trusted set | `TRUSTED_PROXY_CIDRS` at boot | `nil` = loopback + private + link-local; set = loopback + the listed prefixes; invalid = startup error |

`X-Real-IP` and any other header are **not** consulted.

## Output

A non-empty address string, assigned to `echo.Echo.IPExtractor` before the routes are registered, so
`echo.Context.RealIP()` returns it. Consumers today: the auth limiter key `ip:<address>`; any future
per-address decision inherits it (`PRQ-04`).

## Resolution algorithm (normative)

1. Parse the direct peer. If it cannot be parsed → `"unknown"`; do **not** read the chain.
2. If the peer is outside the trusted set → return the peer; ignore the chain (`PRQ-02`).
3. Walk the chain from right to left. The first untrusted entry is the client (`PRQ-01`).
4. If any entry cannot be parsed → return the peer (the whole header is distrusted) (`PRQ-01`).
5. If all entries are trusted → return the leftmost entry (`PRQ-01`).
6. Normalize the result with `Addr.Unmap().String()`.

## Invariants

- The returned value is never empty (`PRQ-03`).
- A request from an untrusted peer cannot influence the result through headers (`PRQ-02`,
  `SC-002`).
- Two clients behind one trusted proxy resolve to two different values (`SC-001`).
- The policy is immutable after boot; request handling performs no configuration lookup.

## Failure modes

| Condition | Behavior | Requirement |
|---|---|---|
| `TRUSTED_PROXY_CIDRS` contains an invalid CIDR | `NewModules` returns an error; the API does not boot | `PRQ-03`, `PRQ-18` |
| `RemoteAddr` unparseable | `"unknown"`, one fail-closed bucket | `PRQ-03` |
| Header absent, empty or malformed | Direct peer | `PRQ-01` |

## Test evidence

- `internal/platform/clientip/clientip_test.go`: the full resolution table, forged/absent/malformed
  chains, multiple headers, IPv6/port forms, the sentinel, and the policy matrix.
- `internal/app/client_ip_test.go`: two clients behind a trusted peer limited independently; a
  forged chain from an untrusted peer has no effect; the router installs the extractor.
- `shared/env/env_test.go`: absent/empty/valid/invalid `TRUSTED_PROXY_CIDRS`.
