# Implementation Plan: Auth Prerequisites — Real Client Address, Canonical Email, Verification-Ready Accounts

**Branch**: `015-auth-prerequisites` | **Date**: 2026-10-20 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/015-auth-prerequisites/spec.md`

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §9 stage 1 — findings F-01, F-03, F-04, F-06 and
F-08. This feature is the prerequisite slice of the 2FA/email program; it deliberately ships no new
HTTP endpoint, no email delivery and no verification gate.

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Make the existing authentication flow correct before the email-verification features build on it, in
four independent seams:

1. **Client address** — the API is reached only through Traefik, so `c.RealIP()` is the proxy's own
   address and the per-IP auth limit is effectively global. The router gains an `IPExtractor` that
   walks `X-Forwarded-For` from the server outward, trusts hops only inside a configured proxy set
   (loopback, private and link-local by default), ignores the header from an untrusted direct peer,
   tolerates IPv6 brackets and `host:port` entries, and never returns an empty key.
2. **Refresh resolves the account** — renewal currently mints an access token from the credential's
   `user_id` without looking the account up. Refresh now loads the account after the existing
   credential rules (CSRF, revoked, expired) and before issuing anything; a missing account answers
   401 and revokes the family, a store failure answers 500, and both issue no credential.
3. **Canonical email** — registration, sign-in and the per-account rate-limit key share one
   canonicalization rule (`net/mail` parse → bare address → trim → lowercase → 254-byte cap), so one
   mailbox is one account however it is spelled. A one-shot, operator-run command rewrites existing
   non-canonical rows under full validation (dry-run by default, abort on collision or unparseable
   row, never on the API boot path).
4. **A verification-ready record and a deterministic clock** — the account gains an optional
   `email_verified_at` with an idempotent store operation (no HTTP surface change), and every
   timestamp the auth service writes or compares comes from one injectable clock.

The change is confined to the Go backend: `internal/auth`, `internal/user`, `internal/app`, a new
leaf `internal/platform/clientip`, `shared/env`, the operator CLI under `cmd/`, and the deployment
image/compose declarations. **No new third-party dependency** and **no new required environment
variable**: the address resolution and the canonicalization use the standard library, and the proxy
set has a documented default.

## Technical Context

**Language/Version**: Go 1.26.4 (`go.mod` `go 1.26.4`; `go version` confirms `go1.26.4 linux/amd64`)
on Echo v5.3.1 (`github.com/labstack/echo/v5 v5.3.1`), MongoDB driver v2.9.1, `golang-jwt/jwt/v5`
v5.3.1 — the pins the repository already carries. No toolchain change.

**Primary Dependencies**: the existing stack only. Echo v5 exposes the seam this feature needs —
`echo.IPExtractor func(*http.Request) string` is consulted by `Context.RealIP()`, and
`internal/app/ratelimit.go` already keys the limiter on `c.RealIP()`; `net/mail` parses an address
and returns its bare `Address`; `net`, `net/netip` and `time` implement the trust walk and the clock;
`go.mongodb.org/mongo-driver/v2` performs the conditional update; `testify` and mockery-generated
mocks cover the tests. **No new dependency** is declared.

**Storage**: MongoDB 7.0 standalone (`deploy/docker-compose.yml`), no transactions or change streams
(F-12). The `users` collection gains one optional field, `email_verified_at`; no new collection, no
new index. The existing unique index `users_email_unique` (`shared/db/indexes.go`) becomes the
canonical-duplicate guard once stored values are canonical. The migration command reuses the
collection and updates only `email` and `updated_at`.

**Testing**: `go test ./...` and `go test -race ./...` (the constitution keeps the race detector
clean). Table-driven suites with `t.Run` and `Happy path:` / `Edge:` / `Error:` prefixes, `require`
for preconditions and `assert` otherwise, mockery mocks (`NewMockX(t)`, `make mocks` after an
interface change). New pure unit suites for the address policy, the canonicalization rule and the
migration plan; service suites with a frozen/stepped clock; a route-level suite through the real
`NewModules`/`NewRouter` that proves two clients behind a trusted proxy are limited independently
without a database; repository and migration integration suites that self-skip without MongoDB. No
test sleeps to cross an expiry boundary (`SC-007`).

**Target Platform**: the Go HTTP API on Linux behind Traefik v3.3; the app container publishes no
port and is reached only through the proxy (`deploy/docker-compose.yml`), which is why the proxy
network is the trust boundary. The migration command runs as a second binary from the same image,
invoked explicitly by the operator.

**Project Type**: backend web service plus an operator CLI in the same Go module; the `web/` client is
untouched (the HTTP contract does not change, `PRQ-14`).

**Performance Goals**: no throughput target. Address resolution is a single pass over the forwarding
chain (O(hops), no state); canonicalization is one `net/mail` parse per register/login, replacing the
parse the service already performs; refresh gains exactly one `users` lookup, which `PRQ-05`
requires. The limiter's key set and window are unchanged.

**Constraints**: no HTTP request or response shape changes (`PRQ-14`); every new failure path is a
typed `apperr` mapped to its documented status (`PRQ-17`); configuration stays optional with
documented defaults and `env.RequiredKeys()` does not change (`PRQ-18`); the migration is never part
of the API boot path (`PRQ-11`); production uses the system clock and test behavior is driven by
injection (`PRQ-16`); layering stays `app → auth → user → shared` with `internal/platform/*` leaf
packages; all source and strings are English (`PRQ-20`); `go test -race` stays clean (`PRQ-19`).

**Scale/Scope**: 3 user stories (2 P1, 1 P2), 20 requirements, 6 internal contracts; roughly 20
touched files across `internal/{auth,user,app}`, one new leaf package, one new CLI package,
`shared/env`, `deploy/`, `Dockerfile`, `Makefile` and `.mockery.yaml`; about 10 new or extended test
suites.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The two new refresh failure paths use typed errors with exported messages and documented statuses: missing account → `CodeUnauthorized` + `MsgInvalidRefreshToken` (401, family revoked); account-store failure → `CodeInternal` + `MsgInternal` (500, no credential, family untouched). Registration keeps `CodeInvalidRequest`/`MsgInvalidEmail` (400) and conflicts keep `CodeConflict`/`MsgEmailAlreadyExists` (409). No new code is needed; the boundary mapping is asserted for every path (`PRQ-17`). | PASS |
| II. Canonical Identifiers | No new entity id is introduced; the migration preserves every existing id and never generates one. The account lookup on refresh uses the credential's persisted `user_id`, not a client-supplied id. | PASS (not exercised) |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | Every requirement lands with tests in the same commit, edge cases first (forged/absent/unparseable chains, trusted-set empty, port/bracket forms, 254/255-byte addresses, display-name/`+tag`/dot spellings, collision/invalid/idempotent migration, exact expiry instant, missing account, store failure, concurrent mark), then one representative happy path per suite. Failure paths assert the typed code and the mapped status; `go test -race` stays clean (`PRQ-19`, `SC-006`). | PASS |
| IV. English-Only Source | All identifiers, comments, log messages, CLI output and user-facing strings are English; the migration report is English too (`PRQ-20`). | PASS |
| V. Layered Architecture | `internal/platform/clientip` is a standard-library-only leaf; `internal/user` gains a pure canonicalization rule and the migration plan (no upward import); `internal/auth` imports `internal/user` as it already does; `internal/app` wires the policy and maps errors. JSON decoding stays in `shared/json`; logging stays in `shared/logger`. | PASS |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | This feature removes a real security defect: a single shared rate-limit key behind the proxy and a spoofable-by-construction decision surface. The header is believed only from a trusted peer, the trust set is configuration, and no credential is issued when the account cannot be resolved. No gate is weakened and no secret is added or committed. | PASS (directly served) |
| VII. Installable, Offline-Capable Web Client | No client surface changes; the HTTP contract and cookie attributes are untouched, so the PWA's existing flows keep working (`PRQ-14`). | PASS (not involved) |
| Additional Constraints — Stack | Go on Echo with MongoDB, `shared/env` for configuration, `testify` + mockery tests — unchanged; no dependency is added. | PASS |
| Additional Constraints — Spec corpus and traceability | `PRQ-01`…`PRQ-20` are defined in `spec.md`; `scripts/check_requirement_ids.py` already scans `*.go`, so the citations in the delivered sources resolve. | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card; recorded for the PR, not claimed by this artifact. | PASS (noted) |
| Development Workflow | The backend sequence (`make fmt && make lint && make vet && make lint-specs`, then `make test-race`) covers the change. `Makefile` gains the optional variable in `APP_ENV_KEYS` and an operator target; `deploy/compose_test.go` keeps validating `RequiredKeys()` against the stack. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps one implementation per rule:
the address policy is the only place that walks the forwarding chain, `user.CanonicalEmail` is the
only canonicalization rule (registration, sign-in, the account rate-limit key and the migration all
call it), the refresh account check sits in the service at the one point where the existing
credential rules have already decided, and the clock is a single function value threaded to the
service and the token manager. Two choices that could have drifted were resolved in research: Echo's
built-in extractor is replaced (not wrapped) because it rejects `host:port` entries and returns an
empty string for an unparseable peer (`R1`, `R3`), and the exact-instant expiry boundary changes from
"valid" to "expired" as the spec's edge case requires, which is the only intended semantic change
(`R8`). Storage stays one optional field plus one conditional update, so the data model adds no
migration framework and no index (`R7`).

## Project Structure

### Documentation (this feature)

```text
.specs/features/015-auth-prerequisites/
├── spec.md                     # Requirements (PRQ-*)
├── plan.md                     # This file
├── research.md                 # Phase 0 — decisions and rejected alternatives
├── data-model.md               # Phase 1 — account record, trust policy, clock, migration model
├── quickstart.md               # Phase 1 — runnable validation guide
├── contracts/                  # Phase 1 — one contract per internal seam
│   ├── client-address.contract.md
│   ├── email-canonicalization.contract.md
│   ├── refresh-account.contract.md
│   ├── auth-clock.contract.md
│   ├── verification-timestamp.contract.md
│   └── email-migration-cli.contract.md
├── checklists/
│   └── requirements.md         # Spec quality checklist
└── tasks.md                    # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
cmd/
├── api.go                              # unchanged entry point
└── canonicalize-emails/
    ├── main.go                         # NEW operator command: flags, wiring, report, exit code (PRQ-11)
    └── main_test.go                    # NEW: run seam over a fake store, dry-run and apply exit codes

internal/platform/clientip/
├── clientip.go                         # NEW: proxy policy + forwarding-chain resolver (PRQ-01…PRQ-04)
└── clientip_test.go                    # NEW: trust walk, forged/absent/malformed chains, ports, IPv6

internal/app/
├── router.go                           # EXTENDED: router.IPExtractor = modules.ClientIP (PRQ-04)
├── bootstrap.go                        # EXTENDED: build the policy and the clock, hand both to the modules
├── ratelimit.go                        # EXTENDED: account key uses the canonical rule (PRQ-08…PRQ-10)
├── client_ip_test.go                   # NEW: two clients behind one proxy limited independently; forged chain has no effect
├── ratelimit_test.go                   # EXTENDED: equivalent spellings share the account key
├── auth_test.go                        # EXTENDED: refresh missing account 401 mapping at the boundary
└── integration_test.go                 # EXTENDED: register/login spellings resolve one account, duplicate is 409

internal/auth/
├── clock.go                            # NEW: Clock func type and its system default (PRQ-16)
├── service.go                          # EXTENDED: clock, canonical email, account load on refresh (PRQ-05…PRQ-09)
├── token.go                            # EXTENDED: injected clock on issuance and parse (PRQ-16)
├── errors.go                           # unchanged messages reused (R11)
├── service_test.go                     # EXTENDED: refresh order/outcomes, canonical input, frozen clock
├── token_test.go                       # EXTENDED: issuance and expiry boundaries without sleeping
└── mocks/                              # unchanged (no interface signature changes)

internal/user/
├── user.go                             # EXTENDED: optional EmailVerifiedAt, absent until set (PRQ-12)
├── email.go                            # NEW: CanonicalEmail + ErrInvalidEmail + MaxEmailLength (PRQ-08)
├── repository.go                       # EXTENDED: MarkEmailVerified conditional update (PRQ-13)
├── email_migration.go                  # NEW: plan/apply split, EmailMigrationStore (PRQ-11)
├── user_test.go                        # NEW: the DTO carries no verification field (PRQ-14)
├── email_test.go                       # NEW: parse/trim/lowercase/length/boundary table
├── email_migration_test.go             # NEW: plan table, blocked plans write nothing, apply writes only two fields
├── repository_test.go                  # EXTENDED: MarkEmailVerified idempotency/not found/no default timestamp (Mongo, self-skip)
└── mocks/                              # regenerated: MockEmailMigrationStore

shared/
├── env/env.go                          # EXTENDED: optional TrustedProxyCIDRs, nil when unset (PRQ-03, PRQ-18)
└── env/env_test.go                     # EXTENDED: absent/empty/valid/invalid CIDR lists

deploy/
├── docker-compose.yml                  # EXTENDED: optional TRUSTED_PROXY_CIDRS pass-through
├── .env.example                        # EXTENDED: documented default and example
└── compose_test.go                     # EXTENDED: the new key is optional and interpolated, required set unchanged

Dockerfile                              # EXTENDED: build the second binary into the runtime image
Makefile                                # EXTENDED: export the optional variable; migrate-emails target
.mockery.yaml                           # EXTENDED: EmailMigrationStore mock generation
```

**Structure Decision**: the forwarding-chain rule lives in a new leaf `internal/platform/clientip`
so it can be exercised with plain `*http.Request` values, imports only `net`, `net/netip` and
`strings`, and can be assigned to `echo.IPExtractor` without the platform package depending on the
auth domain; `internal/app/bootstrap.go` builds the policy (fail-fast on an invalid CIDR) and
`router.go` installs it, so every `c.RealIP()` consumer — today the auth limiter — reads the same
resolution.

The canonicalization rule lives in `internal/user` because the stored address is the account's
property and because both `internal/auth` (which already imports `user`) and the migration (which
reads the same collection) must call it; the pure rule therefore has one home and no cycle.

The refresh account check stays in `auth.ServiceImpl.Refresh` — the only layer that knows the
credential, the family and the issuance order — right after the revoked/expiry rules and before the
first token call, which is what makes "issues nothing" structural rather than a sequence of early
returns a later edit could reorder.

The verification timestamp is a field on the existing record and one repository operation; the
`UserDTO` is untouched, so no response shape moves (`PRQ-14`).

The migration is split into a pure planner/apply pair in `internal/user` (testable with ordinary
tables and a mock store) and a thin `cmd/canonicalize-emails` shell (flags, report, exit code). The
runtime image carries the second static binary, and nothing in `cmd/api.go` references it, so the API
boot path can never run the rewrite (`PRQ-11`).

Tests stay next to the code they cover (`*_test.go`), with HTTP-boundary assertions concentrated in
`internal/app` and MongoDB-dependent behavior in self-skipping integration suites, matching the
repository convention.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
