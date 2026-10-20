# Phase 0 Research: Auth Prerequisites — Real Client Address, Canonical Email, Verification-Ready Accounts

**Feature**: `.specs/features/015-auth-prerequisites/` | **Date**: 2026-10-20

Every decision below answers an open question in the plan's Technical Context or makes explicit a
choice the program plan (`docs/plan-auth-otp-2fa-email.md` §7/§9) and the spec leave to the design.
No `NEEDS CLARIFICATION` remains; the three product-level choices were closed in the spec's
`## Clarifications` (2026-10-20).

---

## R1. The client-address resolver is our own, not Echo's built-in extractor

**Decision**: add a leaf package `internal/platform/clientip` that implements the resolution rule and
returns an `echo.IPExtractor`-compatible `func(*http.Request) string`:

1. Read the direct peer from `req.RemoteAddr` (host portion; a bare address is also accepted). If it
   cannot be parsed, return the non-empty sentinel `"unknown"` (`R3`) and never consult a header.
2. If the peer is **not** trusted, return the peer; the forwarding header is ignored entirely
   (`PRQ-02`).
3. Read every `X-Forwarded-For` header value in request order, split each on `,`, and walk the
   resulting entries **from right to left**. The first untrusted entry is the client (`PRQ-01`).
4. An entry that cannot be parsed distrusted the whole header: return the peer (`PRQ-01`).
5. If every entry is trusted, return the leftmost (furthest) entry (`PRQ-01`, edge case).
6. Normalize the result with `netip.Addr.Unmap().String()` so an IPv4-mapped IPv6 peer and its IPv4
   form share one key; accept IPv6 with or without brackets and entries carrying `:port`.

`internal/app/bootstrap.go` builds the policy from configuration and stores it on `Modules`;
`internal/app/router.go` assigns `router.IPExtractor = modules.ClientIP` before the routes are
registered.

**Rationale**: the spec's edge cases exceed Echo v5.3.1's `ExtractIPFromXFFHeader`: its entry parser
calls `net.ParseIP` on the raw entry, so `1.2.3.4:5678` and `[::1]:443` are "unparseable" and a
legitimate proxy chain would degrade to the proxy address. Its `extractIP` also returns `""` for an
unparseable `RemoteAddr`, which violates `PRQ-03`'s "never keyed by an empty address". Reimplementing
the documented walk (the library's own guide) keeps the security semantics — right-to-left,
nearest-untrusted, distrust-on-unparseable — while covering the forms the spec names. One
implementation means every `c.RealIP()` consumer agrees; today that is the auth limiter, and the
future challenge endpoints inherit it for free (`PRQ-04`).

**Alternatives considered**:

- *Use `echo.ExtractIPFromXFFHeader` as-is* — rejected: `host:port` entries and an unparseable peer
  fail the spec's edge cases, and `TrustIPRange` cannot express "loopback always, configured set
  replaces the rest" without also re-enabling the defaults we must turn off.
- *Pre-process the header into a synthetic request and delegate to Echo* — rejected: mutating or
  cloning the header inside an extractor is harder to reason about than ~60 lines of explicit walk,
  and the clone still returns `""` for a bad peer.
- *A middleware that stores the resolved address in the context* — rejected: `c.RealIP()` already
  exists and the limiter already calls it; a second source of truth invites drift.
- *Trust `X-Real-IP` as a fallback* — rejected by the spec's Assumptions (one source of truth, the
  header the deployment proxy forwards).

## R2. The trusted proxy set is optional configuration with a fail-closed default

**Decision**: `shared/env` gains `TrustedProxyCIDRs []string`, read from `TRUSTED_PROXY_CIDRS`
(comma-separated CIDRs). The value is `nil` when the variable is unset or empty; a set value is
trimmed, blank entries are dropped, and an explicitly set but empty list is preserved as an empty
non-nil slice. `clientip.New(ranges []string)` interprets:

- `nil` → trust **loopback + private + link-local** (the documented default);
- non-nil → trust **loopback** plus exactly the configured prefixes;
- any invalid CIDR → an error, which `NewModules` turns into a boot failure (`build client IP
  policy: …`);
- an empty non-nil list → loopback only (the configured set replaced the non-loopback portion).

Loopback is always trusted because the local development path and same-host proxies rely on it
(`PRQ-03`). `deploy/docker-compose.yml` passes `TRUSTED_PROXY_CIDRS: ${TRUSTED_PROXY_CIDRS:-}` so the
deployed stack can narrow or widen the set, `deploy/.env.example` documents the default, and
`Makefile` exports the variable for local runs. `env.RequiredKeys()` and the compose required-variable
test do not change (`PRQ-18`).

**Rationale**: trusting a wrong network is a security decision, not a cosmetic default: silently
falling back on a typo could trust the internet or trust nothing. The repository's env helpers are
lenient for durations and integers because those values are self-evident in logs; a CIDR list is the
security boundary, so a malformed value is a startup error. The default matches the container
topology: Traefik reaches the app from a private Docker network, so the default trusts the proxy
without any deployment change.

**Alternatives considered**:

- *Hardcode loopback and private ranges* — rejected by `PRQ-03` ("MUST come from configuration").
- *Treat an invalid entry as "skip it and keep the rest"* — rejected: a one-character typo would
  silently change the trust boundary; boot failure is observable and safe.
- *Make the variable required* — rejected by `PRQ-18`: it would change the deployment's required
  list and the compose contract for a feature with a correct default.
- *Reuse `CORS_ALLOWED_ORIGINS`-style string handling* — done, but with the `nil` vs empty
  distinction preserved, which the existing helper cannot express for a nil default.

## R3. An unparseable peer collapses to a non-empty sentinel

**Decision**: `clientip.Unknown = "unknown"` is returned when `RemoteAddr` yields no parseable
address. The limiter key becomes `ip:unknown`, a single fail-closed bucket, and forwarding headers
are never read in that case.

**Rationale**: `PRQ-03` forbids keying a request by an empty address; an empty key would put every
anomalous request into `ip:` and, worse, an empty value reads like a bug in logs. A named sentinel is
grep-able and keeps the limiter's contract (non-empty string keys). Collapsing them is intentionally
conservative: such requests cannot be attributed, so they must not gain independent budgets.

**Alternatives considered**:

- *Return `""`* — rejected by `PRQ-03`.
- *Reject the request with 400* — rejected: no requirement or status exists for it, and the request
  may be legitimate (a test client without `RemoteAddr`); failing closed on attribution is enough.
- *Use the raw `RemoteAddr` string* — rejected: it may contain a port and produce one key per
  connection.

## R4. Canonical email is one pure rule in `internal/user`

**Decision**: `internal/user/email.go` exports:

```go
const MaxEmailLength = 254
var ErrInvalidEmail = errors.New("email is invalid")
func CanonicalEmail(raw string) (string, error)
```

The rule is: `mail.ParseAddress(strings.TrimSpace(raw))`; on error return `ErrInvalidEmail`; take
`parsed.Address` (drops display name and surrounding whitespace); apply `strings.ToLower` and
`strings.TrimSpace`; if `len(canonical) > MaxEmailLength` return `ErrInvalidEmail`. Dots,
`+tags`, underscores and any other local-part character are preserved; nothing else is rewritten.

`auth.Register` stores the canonical value and maps `ErrInvalidEmail` to `CodeInvalidRequest` +
`MsgInvalidEmail`; `auth.Login` canonicalizes before `FetchByEmail` and maps a canonicalization
failure to the same uniform invalid-credentials outcome (`PRQ-08`, `PRQ-09`). The unique index on
`users.email` then makes duplicate detection a canonical comparison (`PRQ-10`). The migration calls
the same function (`PRQ-11`).

**Rationale**: the current code already calls `mail.ParseAddress` and discards the result, so the
parser is the repository's established acceptance rule; reusing it keeps the accepted set unchanged
except for the canonical projection. Lowercasing the local part is not RFC 5321-pure, but it is the
product's existing normalization (the current `normalizeEmail`) and the spec fixes it as the only
transformation (`PRQ-08`). The 254-byte cap follows RFC 5321's maximum path length and is applied to
the **canonical address**, not to a display-name-wrapped input: the stored value is what later
features must deliver to.

**Alternatives considered**:

- *Regex validation* — rejected: weaker than `net/mail` and would change the accepted set the
  existing tests pin.
- *Lowercase the raw input before parsing* — rejected: it would rewrite a display name and a quoted
  local part before the parser sees them; parse first, canonicalize the result.
- *Strip dots or `+tags`* — explicitly forbidden by `PRQ-08` and the program plan (§8).
- *Apply the length cap to the raw input* — rejected: a long display name would reject a deliverable
  address, and `PRQ-08` speaks of "the supplied address" being persisted as the bare address.
- *Put the rule in `internal/auth`* — rejected: `internal/user` cannot import `auth` (the dependency
  runs the other way), and the migration lives with the collection, so the rule would split.
- *Put it in `shared/*`* — rejected: it is a domain rule about the account's identity, not a generic
  utility.

## R5. The per-account rate-limit key uses the same canonicalization

**Decision**: `internal/app/ratelimit.go` replaces `normalizeAccountKey` with a call to
`user.CanonicalEmail`; when the input cannot be parsed it falls back to
`strings.ToLower(strings.TrimSpace(...))`, so an invalid spelling still occupies a stable bucket.

**Rationale**: `AUTH-17`'s per-account key exists to stop one account from being attacked from many
addresses. Once sign-in accepts `"Name <a@b.com>"` as the same account, a raw lower+trim key would
give every spelling its own budget, quietly re-opening the brute-force path the second key closes.
The fallback keeps the guard total for malformed bodies without changing the handler's 400.

**Alternatives considered**:

- *Leave the key as lower+trim* — rejected: a bypass introduced by `PRQ-09`.
- *Key on the parsed account id after lookup* — rejected: the guard runs before the handler and must
  not touch the store.

## R6. Refresh loads the account after the existing credential rules

**Decision**: `ServiceImpl.Refresh` keeps its current order up to expiry and then inserts the account
load:

| Step | Failure | Outcome |
|---|---|---|
| empty credential | — | 401 `MsgRefreshTokenRequired` |
| credential lookup | not found | 401 `MsgInvalidRefreshToken` |
| credential lookup | store error | 500 `MsgInternal` |
| CSRF verification | invalid | 403 `MsgCSRFInvalid` |
| revoked credential | — | existing reuse path: family revoked, 401 `MsgInvalidRefreshToken` |
| expired credential | — | 401 `MsgInvalidRefreshToken` (no family revoke, unchanged) |
| **account load** | user not found | revoke the presented family, then 401 `MsgInvalidRefreshToken`, no credential |
| **account load** | store error | 500 `MsgInternal`, no credential, family untouched |
| issuance | store error | 500 `MsgInternal` (unchanged) |
| rotation claim | already rotated | 401 `MsgInvalidRefreshToken` (unchanged) |

The access token is issued for `account.ID`, and the rotated refresh credential is created for
`account.ID` in the same family (`PRQ-06`). The expiry comparison becomes
`!now.Before(stored.ExpiresAt)` so the exact instant is expired, as the spec's edge case requires
(`R8`).

**Rationale**: the edge case "a revoked/expired credential whose account is missing keeps the
existing outcome" forces the account check after those rules; `PRQ-05`'s "before issuing anything"
forces it before `tokens.Issue`. Putting it exactly there makes both properties structural. A missing
account is treated as an invalid credential (401) because the client must re-authenticate and the
response must not distinguish "no account" from "bad credential"; revoking the family prevents the
remaining chain from being retried. Existing typed codes and messages are reused, so no new
`apperr.Code` and no client error-mapping work is introduced (`PRQ-17`, `R11`).

**Alternatives considered**:

- *New `CodeAccountMissing` / distinct message* — rejected: it adds an observable code for a state
  the client handles identically to an invalid credential, and `PRQ-17` only requires the mapped
  status.
- *Load the account before the CSRF check* — rejected: it answers a different status for a missing
  account to a caller who cannot prove session possession, and it changes the existing rules the edge
  case preserves.
- *Load the account after issuance and discard the tokens on failure* — rejected: it can leave a
  rotated credential behind, violating "MUST NOT issue".
- *Answer 404 for a missing account* — rejected by `PRQ-05` (401) and by account-existence hygiene.
- *Return 401 without revoking the family* — rejected by `PRQ-05` and `SC-003`.

## R7. The verification timestamp is an optional field and one conditional write

**Decision**: `user.User` gains `EmailVerifiedAt *time.Time` with `bson:"email_verified_at,omitempty"`
and `json:"email_verified_at,omitempty"`. `user.Repository` gains:

```go
MarkEmailVerified(ctx context.Context, id string, at time.Time) error
```

implemented as one `UpdateOne` with filter `{_id: id, email_verified_at: nil}` and
`$set: {email_verified_at: at, updated_at: at}`. `MatchedCount == 1` → success; `MatchedCount == 0`
→ `FetchByID`: found means "already verified" (no-op, `updated_at` untouched), `ErrUserNotFound`
means the account does not exist. Store failures are wrapped with `fmt.Errorf("mark email verified:
%w", err)`. `UserDTO` is unchanged, and `Register` keeps constructing the user without the field, so
existing and new rows stay absent until verification (`PRQ-12`…`PRQ-15`).

**Rationale**: Mongo's `{field: nil}` matches both a missing field and an explicit `null`, which is
exactly "not verified"; the filter makes the first write win under concurrency without a transaction
(the deployment is standalone, F-12). Splitting the not-found/already-set distinction with a second
read only happens when the update matched nothing, so the common path is one write. Using the same
`at` for both fields satisfies "the modification timestamp in the same write", and the conditional
filter makes a second call a true no-op (the spec's idempotency edge case).

**Alternatives considered**:

- *Unconditional `$set`* — rejected: it overwrites the first timestamp on a retry.
- *Aggregation-pipeline update with `$ifNull`* — rejected: it always rewrites `updated_at`, so the
  no-op is not a no-op, and it hides the first-value invariant inside an expression.
- *Read → check → write* — rejected: two concurrent calls could both write, violating "first value
  preserved".
- *A separate collection or status field* — rejected by `PRQ-12`/the program plan's data model.
- *Expose the field on the DTO* — forbidden by `PRQ-14`.

## R8. One clock function value, injected into the service and the token manager

**Decision**: `internal/auth/clock.go` declares `type Clock func() time.Time`. `auth.Config` gains
`Now Clock`; `NewService` defaults a nil clock to `time.Now`. `NewTokenManager` gains a third
parameter `now Clock` (nil defaults to `time.Now`), used by `Issue` for `iat`/`exp` and by `Parse`
through `jwt.WithTimeFunc(now)`. The service's `time.Now()` calls in `Register`, `Refresh` (expiry)
and `issueRefreshToken` are replaced by `s.now()`. `cmd/api.go`/`bootstrap.go` pass `time.Now`.
Exceptions are explicit: the CSRF manager has no timestamps, cookie `Max-Age` comes from
configuration, and the repository never reads the clock (it receives `at`).

The expiry check changes from `now.After(expiresAt)` to `!now.Before(expiresAt)`, making the exact
expiry instant expired (spec edge case). This is the feature's only intended semantic change; with
the system clock every other observable behavior is identical (`PRQ-16`).

**Rationale**: `PRQ-16` counts credential issuance and expiry among the timestamps the auth service
writes or compares, so the clock cannot stop at the service struct. A function value matches the
existing `ratelimit.New(..., now func() time.Time)` convention, injects without an interface, and
lets a test step time to the boundary instead of sleeping (`SC-007`). `jwt.WithTimeFunc` is the
library's supported seam for validation time; without it the token manager's clock would disagree
with the parser's.

**Alternatives considered**:

- *Package-level `var now = time.Now`* — rejected: a mutable global plus parallel tests is a race and
  hides the dependency.
- *A `Clock` interface with `Now()`* — rejected: one method, more boilerplate, and inconsistent with
  the limiter.
- *Only inject into the service* — rejected: token `iat`/`exp` are credential timestamps named by
  `PRQ-16`.
- *Keep `After` and accept "valid at the exact instant"* — rejected: contradicts the spec's edge
  case; the change is one nanosecond wide and covered by a boundary test.

## R9. The migration is a validated plan plus an explicit apply, packaged as its own binary

**Decision**: `internal/user/email_migration.go` defines:

```go
type EmailIdentity struct{ ID, Email string }
type EmailChange struct{ ID, From, To string }
type EmailCollision struct{ Canonical string; IDs []string }
type InvalidEmailRow struct{ ID, Raw, Reason string }
type EmailMigrationPlan struct{ Changes []EmailChange; Collisions []EmailCollision; Invalid []InvalidEmailRow }
var ErrMigrationBlocked = errors.New("email canonicalization blocked")

func PlanEmailCanonicalization(rows []EmailIdentity) EmailMigrationPlan
func (p EmailMigrationPlan) Apply(ctx context.Context, store EmailMigrationStore, now time.Time, dryRun bool) (int, error)
```

`EmailMigrationStore` exposes `ListEmailIdentities(ctx)` and
`UpdateEmail(ctx, id, email string, updatedAt time.Time)`. `Apply` refuses to write when the plan has
a collision or an invalid row (`ErrMigrationBlocked`), writes only `email` and `updated_at`, and
reports how many rows it changed. `cmd/canonicalize-emails` parses `-apply` (default **false** =
dry-run), loads `env`/Mongo, runs the plan, prints the report to stdout, and exits 0 on success or a
no-op, 1 on blocked/invalid/store failure, 2 on usage. The Dockerfile builds it as
`/canonicalize-emails` in the runtime image; the operator runs
`docker compose run --rm --entrypoint /canonicalize-emails app -apply`. Nothing in `cmd/api.go`
imports it.

**Rationale**: validating the whole set before the first write is what "writes nothing" means for
collisions and unparseable rows; a dry-run default makes an irreversible rewrite opt-in. The planner
is pure and the store is an interface, so the failure matrix is unit-tested; the actual update
semantics are covered by a self-skipping Mongo integration test. A separate binary satisfies "never
part of the API boot path" in the strongest sense — the API image's entry point cannot reach it.
Mongo standalone has no multi-document transaction, so a store failure between two updates can leave
a prefix applied; the command is idempotent by construction, and re-running finishes the job
(documented in the CLI contract).

**Alternatives considered**:

- *Repair on API boot* — forbidden by `PRQ-11` and dangerous: a rewrite would run on every restart.
- *A subcommand of `cmd/api.go`* — rejected: the binary's default path must stay the server, and a
  flag mistake would reach production startup.
- *One `UpdateMany` per canonical value* — rejected: it cannot report per-row intent in dry-run nor
  detect all collisions before writing.
- *Pull a migration framework* — out of scope (F-14) and a dependency `PRQ-18`'s no-new-required
  posture does not need.
- *Transact the writes* — unsupported on the standalone deployment (F-12).

## R10. Configuration and deployment surface stay additive

**Decision**: `Makefile` adds `TRUSTED_PROXY_CIDRS` to `APP_ENV_KEYS` (so local runs export it and
deploy recipes unset it) and gains a `migrate-emails` target that runs the CLI with the root `.env`;
`deploy/docker-compose.yml` adds the optional `${TRUSTED_PROXY_CIDRS:-}` pass-through;
`deploy/.env.example` documents the default and an example; `deploy/compose_test.go` gains a case
asserting the key is present as interpolation and that `RequiredKeys()` is unchanged.

**Rationale**: `PRQ-18` fixes "optional, documented defaults" and "required-variable list and compose
validation do not change". Without the compose pass-through the operator could set the variable in
`deploy/.env` and it would never reach the container, making `PRQ-03`'s "come from configuration"
vacuous in production. The Makefile target is operator ergonomics for the one-shot command.

**Alternatives considered**:

- *No compose change* — rejected: the default works, but a configured set would be silently
  ignored.
- *Add it to `RequiredKeys()`* — rejected by `PRQ-18`.

## R11. No new `apperr` code; the existing taxonomy covers the new failures

**Decision**: reuse `CodeUnauthorized` + `MsgInvalidRefreshToken` for a missing account on refresh
and `CodeInternal` + `MsgInternal` for an account-store failure; registration keeps
`CodeInvalidRequest` + `MsgInvalidEmail` and conflicts keep `CodeConflict` + `MsgEmailAlreadyExists`.
`shared/apperr/err.go` is unchanged. The HTTP boundary tests assert 401/500/400/409 and the code
strings for the new paths.

**Rationale**: `PRQ-17` names a status per failure mode and asks for an exported message constant;
both messages are already exported in `internal/auth`. A new code would change the JSON body for
states the web client already maps (`error_mapper.ts` falls back to `unknown` for unrecognized
codes), forcing a client change in a backend-only feature with no product benefit. The distinction
that matters — 401 versus 500 — is preserved.

**Alternatives considered**:

- *`CodeAccountStoreUnavailable`* — rejected: same 500, new client mapping, no user-visible gain.
- *`CodeAccountGone`* — rejected: reveals a state the client cannot act on differently from 401.

## R12. Test strategy for the route-level requirement without a database

**Decision**: a new `internal/app/client_ip_test.go` builds the production wiring
(`app.NewModules` + `app.NewRouter`) over a Mongo client that never dials (the
`router_wiring_test.go` pattern), sets `AUTH_RATE_LIMIT_ATTEMPTS=1` with `t.Setenv` before
`env.NewEnv`, and drives `POST /auth/login` with explicit `RemoteAddr` and `X-Forwarded-For`:

- trusted peer + chain naming client A → first call passes the guard (handler then fails on the
  unreachable store with 500), second call is 429;
- the same trusted peer + chain naming client B → not 429;
- an untrusted peer + a forged chain → the direct peer bucket is consumed, and varying the forged
  value does not change the outcome;
- no header → the direct peer is used; a malformed chain (or a `host:port` chain, as its own case) →
  resolution succeeds or falls back exactly as the spec says.

The extractor's own table (`clientip_test.go`) covers the full matrix without HTTP, and the guard
suite (`ratelimit_test.go`) covers key independence. This split keeps `PRQ-04` proven at the router
while the algorithm stays cheap to test.

**Rationale**: the requirement is about the wired router, not the function; a unit test of the
extractor alone could pass while the router never installs it. The unreachable-store handler is
deterministic (connection refused in ~50 ms) and needs no Mongo, so the suite runs in CI.

**Alternatives considered**:

- *Only unit-test the extractor* — rejected: it would not catch a missing `router.IPExtractor`
  assignment.
- *Spin up MongoDB for the boundary test* — rejected: the guard is decided before the handler, and
  the integration suite already self-skips.

## Resolved unknowns

| Technical Context item | Resolution |
|---|---|
| Which extractor implementation | Own leaf resolver (`R1`), not Echo's built-in |
| Default trusted set and its configuration | Loopback + private + link-local; `TRUSTED_PROXY_CIDRS` replaces the non-loopback part (`R2`) |
| Empty-address behavior | `"unknown"` sentinel, headers not consulted (`R3`) |
| Where the canonicalization rule lives | `internal/user`, shared by auth, the key and the migration (`R4`) |
| Canonicalization of the rate-limit account key | Same rule, lower+trim fallback (`R5`) |
| Where the refresh account check sits | After revoked/expiry, before issuance (`R6`) |
| Verification operation semantics | Conditional single write, first value wins (`R7`) |
| Clock shape and scope | One `Clock` func into service + token manager, `jwt.WithTimeFunc` (`R8`) |
| Migration packaging and default mode | Separate binary, dry-run default, plan/apply split (`R9`) |
| Deployment surface | Optional compose pass-through; required keys unchanged (`R10`) |
| Error taxonomy | No new code; existing messages/statuses (`R11`) |
| Proving the router wiring | Guard-boundary suite over real wiring, no database (`R12`) |
