---
status: accepted
---

# Rate limits are counted in a shared Redis, failing open to the per-pod limiter

The limiter kept its counters in process memory, so with N replicas the effective cap was N times the configured one, weakening the login brute-force guard (`AUTH_RATE_LIMIT_*`), the per-user cap and the CSP collector cap. We decided:

1. **Counters live in Redis** (`REDIS_URL`), one sorted set per key under `rl:<namespace>:<key>`, namespaces `auth`, `user` and `public`. One Lua script prunes the window, checks the cap and records the attempt atomically, using the Redis clock so pod clock skew cannot move a window. The sliding-window semantics and `Retry-After` are unchanged.
2. **Redis failure fails open** to the in-process limiter of the same cap. Availability wins: a Redis outage must not lock every player out of login or `/me`. During the outage the effective cap is the old N-times approximation. A warning is logged at most once a minute per limiter.
3. **`REDIS_URL` is optional.** Unset keeps the in-process limiter, so local `go run`, tests and single-replica deployments need nothing new. A malformed value fails the boot.
4. **`TRUSTED_PROXY_CIDRS` is documented for Kubernetes** (`deploy/README.md`): the per-IP key is only the real client while the ingress source network is trusted.

## Considered Options

- **Shared MongoDB counters (`$inc` on a fixed window with a TTL index):** no new infrastructure, but adds a write to the primary store on every guarded request, including the `/me` hot path, and gives a fixed window instead of a sliding one.
- **Accept the approximate limit and document it (divide the cap by replicas):** no code, but the effective cap still drifts with autoscaling and a single client hitting one pod can exceed the intended share. Kept only as the documented degraded mode.
- **Fail closed on store failure:** safer against brute force during an outage, but turns a cache outage into a login and API outage for everyone.

## Consequences

- One new stateless dependency, with no volume: counters live one window, and losing them resets budgets at worst.
- Each guarded request costs one Redis round-trip (bounded by a 250 ms timeout); a slow Redis degrades to the per-pod cap instead of adding latency without bound.
- The in-process `Limiter` stays as both the no-Redis mode and the fallback, so the contract is tested in both.
- An attacker who can take Redis down gets the N-times cap, not an open door: the per-pod limiter still applies.
