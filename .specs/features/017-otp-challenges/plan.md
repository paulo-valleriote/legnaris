# Implementation Plan: Email Code Challenges — Six-digit Codes, Atomic One-shot Consumption and the Unified Verify/Resend Endpoints

**Branch**: `017-otp-challenges` | **Date**: 2026-10-20 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/017-otp-challenges/spec.md`

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §9 stage 3 — decisions D1 (2FA on every sign-in),
D3 (six-digit code), D4 (existing accounts verify at first sign-in), D6 (durable outbox), O3 (unified
HTTP surface), O4 (free-tier daily ceiling). This feature is the challenge slice of the program: it
owns the challenge record, the code, the resend rules, the limits and the two unified endpoints. It
deliberately does not touch registration or sign-in, does not gate renewal and adds no verification
state to any response — that is stage 4.

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Deliver the code step both later flows depend on, in four parts:

1. **A challenge record with one-shot, expiring, attempt-capped consumption.** A new
   `auth_challenges` collection holds one active record per account and purpose: identity, account,
   purpose, keyed digest of the code, expiry, an absolute ceiling, consumed marker, attempt count and
   last-sent instant. Consumption is a single-document atomic claim (increment the attempt, compare
   the digest in constant time, claim the row), so a replay, a race or a restart can never consume
   one challenge twice.
2. **A code that is generated once and never stored in the clear by this domain.** The code is six
   digits from `crypto/rand`, zero-padded; the challenge keeps only a keyed digest that binds the
   account and the purpose, keyed by a new **required** `EMAIL_CODE_SECRET` distinct from
   `JWT_SECRET`. The code's one durable home is the send intention the delivery infrastructure
   already defines, which is what makes a resend able to re-send the *same* code without rotating it.
3. **The unified endpoints.** `POST /auth/challenge/verify` consumes a challenge and establishes a
   session exactly as the current sign-in path does, recording the account's verification timestamp
   when absent; `POST /auth/challenge/resend` is idempotent for the client, cooldown-bounded and
   answered uniformly. Every verification failure is one typed outcome (`OTP_INVALID`, 401), so
   unknown, expired, consumed, wrong and exhausted are indistinguishable.
4. **The bounds and the observability.** Per-address limits reuse the existing limiter; per-account
   limits are a durable window count over the account's challenges; the daily send cap is checked
   against the send intentions actually recorded that day, and its exhaustion is a typed
   `EMAIL_UNAVAILABLE` (503) with a retry hint. Issuance, verification results and cap refusals are
   counters; the code never reaches a response, a log line or an error message.

The change is confined to the Go backend: `internal/auth` (challenge domain), `internal/app`
(controller, routes, the intention adapter), `internal/email/outbox` (two store additions),
`internal/platform/metrics`, `shared/{apperr,env,db}` and `deploy/`. **No third-party dependency is
added.** One environment variable joins the required list (`EMAIL_CODE_SECRET`), the first such
addition in this program (spec Clarifications, 2026-10-20).

## Technical Context

**Language/Version**: Go 1.26.4 (`go.mod` `go 1.26.4`; `go version` confirms `go1.26.4 linux/amd64`)
on Echo v5.3.1 (`github.com/labstack/echo/v5 v5.3.1`), MongoDB driver v2.9.1
(`go.mongodb.org/mongo-driver/v2 v2.9.1`), Prometheus client v1.24.1, `testify` v1.12.1,
mockery v2.53.5 — the pins the repository already carries.

**Primary Dependencies**: none added. The code generator uses `crypto/rand` (`rand.Int` against
`big.NewInt(1_000_000)` for a uniform, zero-padded six-digit value); the digest uses
`crypto/hmac` + `crypto/sha256`; comparison uses `hmac.Equal`; identities come from `shared/idgen`
(UUID v7) and inbound ones are screened with `idgen.Validate`; logging goes through `shared/logger`;
configuration through `shared/env`; indexes through `shared/db`; the durable intention through
`internal/email/outbox`, the store delivered by the preceding feature (its `NewIntent` already
accepts a caller-supplied, validated identity, which is what lets the challenge know the intention
identity before the intention is written).

**Storage**: MongoDB 7.0 standalone (`deploy/docker-compose.yml`), no multi-document transactions
(F-12), so every invariant here is per-document. One new collection, `auth_challenges`:

- `{user_id: 1, purpose: 1}` **unique, partial** (`consumed_at` null) — the one-active-challenge
  invariant, enforced by the store rather than by a pre-check;
- `{expires_at: 1}` TTL (`expireAfterSeconds: 0`) — hygiene only; expiry is always decided by
  comparing the stored instant with the injected clock (F-13, `OTP-06`).

`email_outbox` gains one non-unique index, `{created_at: 1}`, to back the daily-cap count. No
existing collection changes shape; `users` is only read (`FetchByID`) and written through the
existing idempotent `MarkEmailVerified` (PRQ-13).

**Testing**: `go test ./...` and `go test -race ./...` with **no service required**: the challenge
service is tested against a mockery `ChallengeRepository` and a fake code toolkit/mailer; the
endpoints are tested through the Echo router with the same fakes; the digest, generator and policy
are pure unit suites; the atomic claim, the one-active invariant, the TTL index and the daily count
live in self-skipping Mongo suites (the repository convention). Table-driven `t.Run` suites with
`Happy path:` / `Edge:` / `Error:` prefixes, `require` for preconditions and `assert` otherwise.
Expiry, cooldown and ceiling boundaries are driven by the injected `auth.Clock` with **zero
sleeps** (`OTP-20`, `SC-007`); failure paths assert the typed `apperr` code and, at the HTTP
boundary, the mapped status.

**Target Platform**: the Go backend on Linux, behind the existing edge proxy; the API is the only
process involved. No new binary, no broker, no provider call from the request path: the feature
writes a send intention into MongoDB and the preceding feature's relay owns everything after that.

**Project Type**: backend web service in the existing Go module; the `web/` client is untouched in
this feature (the endpoints' client mapping is stage 5).

**Performance Goals**: no throughput target. One issuance is O(1) writes (close + insert + intention);
one verification is at most two conditional writes plus one diagnostic read on the failure path; one
resend is one conditional write plus one intention write; the per-account limit and the daily cap are
each a single indexed count. Constant-time digest comparison is the only cryptographic cost.

**Constraints**: `EMAIL_CODE_SECRET` is required and validated at boot, failing fast when absent,
blank or shorter than the documented minimum (spec Clarifications; `OTP-18`, `SC-008`) — the first
addition to `env.RequiredKeys()` in this program, mirrored in `deploy/.env.example`, the compose
file and `deploy/compose_test.go`. Every other setting is optional with a documented default. The
verification failure is one code and one status (`OTP_INVALID`, 401); the cap refusal is
`EMAIL_UNAVAILABLE`, 503; the address-limit refusal reuses `RATE_LIMITED`, 429 (`OTP-08`, `OTP-13`,
`OTP-17`). The digest secret is never the session secret (F-09). The code never appears in a
response, log, error or subject (`OTP-15`). Layering stays `internal/app → internal/auth →
internal/user` plus `internal/app → internal/email/outbox` for the intention adapter, with no upward
import; `internal/auth` never imports the email domain (it declares the `CodeMailer` port the app
adapts). All source and strings are English (`OTP-21`); `go test -race` stays clean (`OTP-20`).

**Scale/Scope**: 4 user stories (2 P1, 2 P2), 21 requirements, 6 internal contracts; roughly 30
touched or created files across `internal/auth`, `internal/app`, `internal/email/outbox`,
`internal/platform/metrics`, `shared/{apperr,env,db}`, `deploy/` and `.mockery.yaml`; 16 new or
extended suites; 2 newly mocked interfaces (the challenge repository, the mailer) plus 2 regenerated
mocks (the widened `auth.Service` and the extended outbox `Repository`).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | Every new failure mode is an `apperr` code with an exported message constant and a documented status: `OTP_INVALID` (401, one code for every verification failure, `OTP-08`), `EMAIL_UNAVAILABLE` (503, send cap, `OTP-13`), the existing `RATE_LIMITED` (429, `OTP-14`) and the existing `INVALID_REQUEST`/`INVALID_JSON` for a malformed body; a store failure is `INTERNAL_ERROR` and never reported as an invalid code (`OTP-17`). Lower-layer failures (Mongo, `idgen`, the outbox store) keep their own typed sentinels and are translated at the service boundary with `errors.Is`. `shared/apperr` gains a status-mapping test for each new code. | PASS |
| II. Canonical Identifiers | Every challenge identity is a canonical lowercase UUID v7 from `idgen.New`; the inbound challenge identity is screened with `idgen.Validate` at the service boundary, and a malformed one takes the same uniform failure as an unknown one (`OTP-08`). The intention identity the challenge stores is generated by the same generator and validated by `outbox.NewIntent`. | PASS |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | Every requirement lands with tests in the same commit, edge cases first: replay, concurrent consumption, cap boundary, expiry at the exact instant, cooldown boundary, ceiling boundary, unknown/consumed/expired identities, malformed identities, digest binding across accounts and purposes, unset/short secret, empty outbox, cap reached, store failure mid-flow, masked logging. Then one happy path per suite. Failure paths assert the typed code and the mapped status; `go test -race` stays clean with two concurrent verifications and two concurrent issuances (`OTP-20`, `SC-001`, `SC-007`). | PASS |
| IV. English-Only Source | Identifiers, comments, log messages and the message constants are English (`OTP-21`); the code's user-facing copy already lives in the preceding feature's English templates. | PASS |
| V. Layered Architecture | The challenge domain lives in `internal/auth` beside the session it issues, importing only `internal/user` and `shared/*`. It does **not** import the email domain: it declares a `CodeMailer` port, and `internal/app` adapts that port to `internal/email/outbox` at the composition root, so the email store never imports upward and the auth domain stays testable with plain fakes. Decoding goes through `shared/json`; logging through `shared/logger`. | PASS |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | The code is a short-lived credential: it is generated from a cryptographic source, stored by the domain only as a keyed digest, compared in constant time, bounded by five attempts and an expiry, and never logged, echoed or placed in a subject (`OTP-02`, `OTP-03`, `OTP-05`, `OTP-15`). The digest secret is a separate, required secret never committed and never reused as the session secret (F-09, `OTP-18`, `OTP-21`). The new routes are per-address and per-account limited and, like the existing sign-in and registration routes, present a capability rather than ambient cookie authority, so no origin guard is applied; no gate is weakened and no credential enters JS-readable storage. | PASS (directly served) |
| VII. Installable, Offline-Capable Web Client | No client surface changes in this feature; the PWA's existing flows and storage rules are untouched. The client mapping of the new codes is stage 5 and is recorded as deferred. | PASS (not involved) |
| Additional Constraints — Stack | Go on Echo with MongoDB and `shared/env`, `testify` + mockery tests — unchanged; **no dependency is added** (stdlib cryptography). No new binary. | PASS |
| Additional Constraints — Spec corpus and traceability | `OTP-01`…`OTP-21` are defined in `spec.md`; `scripts/check_requirement_ids.py` scans `*.go`, so every citation in the delivered sources resolves. | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card; recorded for the PR, not claimed by this artifact. | PASS (noted) |
| Development Workflow | The backend sequence (`make fmt && make lint && make vet && make lint-specs`, then `make test-race`) covers the change; `make mocks` regenerates the widened `auth.Service` mock and the new repository/mailer mocks; `deploy/compose_test.go` validates the new required key against the stack. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. One implementation per rule: the digest is
computed once (`code.go`) and the store never hashes; the consumption decision lives only in the
repository's conditional writes and the service never does read-then-write; the uniform failure is
produced in one place (the service's `invalidCode` path) so the five cases cannot drift; the send
intention is written through the port by both issuance and resend; and the cap policy (window, cap
value) is decided in the auth service, with only the count taken from the store. Two choices that
could have drifted were closed in research: the challenge keeps a digest rather than the code and
resend re-records a **new** intention cloned from the original payload (`R3`) — rejecting both a
plaintext/encrypted copy on the challenge and re-queuing the same intention identity, which the
provider's idempotency key would silently suppress; and the daily cap is a count of recorded
intentions rather than a second counter collection (`R6`).

## Project Structure

### Documentation (this feature)

```text
.specs/features/017-otp-challenges/
├── spec.md                     # Requirements (OTP-*)
├── plan.md                     # This file
├── research.md                 # Phase 0 — decisions and rejected alternatives
├── data-model.md               # Phase 1 — challenge, code, policy, configuration
├── quickstart.md               # Phase 1 — runnable validation guide
├── contracts/                  # Phase 1 — one contract per internal seam
│   ├── challenge.contract.md
│   ├── code.contract.md
│   ├── endpoints.contract.md
│   ├── send-intent.contract.md
│   ├── configuration.contract.md
│   └── observability.contract.md
├── checklists/
│   └── requirements.md         # Spec quality checklist
└── tasks.md                    # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/auth/
├── challenge.go                    # NEW: Challenge record, Purpose, lifecycle predicates (OTP-01, OTP-04…OTP-06)
├── challenge_repository.go         # NEW: Mongo store — CloseActive, Issue, ClaimUsable, Consume, Extend, CountForUserSince (OTP-01, OTP-04…OTP-06, OTP-14)
├── code.go                         # NEW: CodeGenerator + CodeDigest (HMAC-SHA256, constant-time compare, secret validation) (OTP-02, OTP-03)
├── challenge_mailer.go             # NEW: CodeMailer port + ErrSendBudgetExhausted sentinel (OTP-12, OTP-13)
├── challenge_observer.go           # NEW: ChallengeObserver interface + nop (OTP-16)
├── challenge_policy.go             # NEW: ChallengePolicy (attempts, lifetime, ceiling, cooldown, cap, per-account window) (OTP-05, OTP-10, OTP-13, OTP-14)
├── challenge_service.go            # NEW: IssueChallenge, VerifyChallenge, ResendChallenge (OTP-07…OTP-11)
├── challenge_test.go               # NEW: lifecycle predicates, purpose effects
├── challenge_repository_test.go    # NEW (Mongo, self-skip): unique claim, one-active unique index, TTL, count
├── code_test.go                    # NEW: generator uniformity/shape, digest binding, constant-time compare, secret validation
├── challenge_policy_test.go        # NEW: expiry/cooldown/ceiling boundaries on a stepped clock, no sleeps
├── challenge_service_test.go       # NEW: the consumption/resend matrix against fakes
├── service.go                      # EXTENDED: challenge deps in the constructor; Service gains Issue/Verify/Resend
├── errors.go                       # EXTENDED: the uniform invalid-code message and the new message constants
└── mocks/                          # EXTENDED: regenerated Service mock + new ChallengeRepository and CodeMailer mocks

internal/app/
├── auth_challenge.go               # NEW: Verify and Resend controllers — 200 session / 202 resend (OTP-07…OTP-11, OTP-19)
├── auth_challenge_test.go          # NEW: status mapping, uniform failure bodies, no code in any response
├── email_intent.go                 # NEW: the CodeMailer adapter over internal/email/outbox (OTP-10, OTP-12, OTP-13)
├── email_intent_test.go            # NEW: the adapter's mapping, count and clone behaviour (outbox faked)
├── router.go                       # EXTENDED: the two routes with the address limiter (OTP-14, OTP-19)
├── bootstrap.go                    # EXTENDED: wire repository, toolkit, mailer, observer, policy; fail fast on the secret (OTP-18)
├── bootstrap_test.go               # NEW: NewModules fails fast on an absent/blank/short secret and starts with a valid one (OTP-18, SC-008)
├── auth_integration_test.go        # EXTENDED: issue → verify → session, and resend, end to end (Mongo, self-skip) (OTP-07…OTP-11)
└── auth_test.go                    # EXTENDED: the route inventory assertion keeps covering the new paths

internal/email/outbox/
├── repository.go                   # EXTENDED: Clone(ctx, id, now) and CountSince(ctx, since) (OTP-10, OTP-13)
├── repository_test.go              # EXTENDED: clone copies payload with a fresh identity; count honours the range
└── mocks/mock_Repository.go        # REGENERATED

internal/platform/metrics/
└── metrics.go                      # EXTENDED: otp issued/verify/resend/cap counters (OTP-16)

shared/apperr/
├── err.go                           # EXTENDED: OTP_INVALID → 401, EMAIL_UNAVAILABLE → 503 (OTP-08, OTP-13)
└── err_test.go                      # EXTENDED: the status mapping for both codes

shared/env/
├── env.go                           # EXTENDED: required EMAIL_CODE_SECRET + the optional challenge settings (OTP-18)
└── env_test.go                      # EXTENDED: the required set changed deliberately; defaults; invalid values

shared/db/indexes.go                 # EXTENDED: auth_challenges (partial unique + TTL) and email_outbox.created_at (OTP-01, OTP-06, OTP-13)
shared/db/indexes_test.go            # EXTENDED: index presence and partial filter (Mongo, self-skip)

deploy/
├── docker-compose.yml               # EXTENDED: pass the required secret and the optional settings through
├── .env.example                     # EXTENDED: the required secret and the documented defaults
├── compose_test.go                  # EXTENDED: the required set now includes EMAIL_CODE_SECRET
└── README.md                        # EXTENDED: the new variables and what they govern

Makefile                             # EXTENDED: export the new variables to the app process
.mockery.yaml                        # EXTENDED: ChallengeRepository, CodeMailer
```

**Structure Decision**: the challenge domain lives in `internal/auth` because it is the second half
of the same sign-in story — it consumes a challenge and issues exactly the session `signIn` already
issues, sharing the token manager, the refresh/CSRF machinery, the user store and the injected
clock. A separate package would have to either duplicate that machinery or reach upward into it, so
the record, the store, the code toolkit, the policy and the policy-bearing service are new files
beside the existing ones instead (mirroring `refresh_repository.go`'s placement).

The email domain stays untouched except for two additive store methods on the outbox: `Clone`
(create a fresh intention from an existing one's recipient, template and payload) and `CountSince`
(count intentions recorded since an instant). Both belong to the store that owns the rows, which is
what keeps the code's single durable home in the outbox — the resend path re-records the *same code*
by cloning the original payload instead of keeping a second copy of it on the challenge (`R3`).

The port between the two domains is declared by the consumer (`internal/auth.CodeMailer`) and
adapted at the composition root (`internal/app/email_intent.go`), so `internal/auth` never imports
`internal/email/outbox` and the endpoints can be tested with a plain fake. This is the repository's
existing seam style — `auth.Observer` implemented by `internal/platform/metrics`, the rate-limit
`Observer` implemented by the same package — and it keeps the daily-cap policy (the window and the
cap value) in the service while only the count comes from the store.

Tests stay next to the code they cover (`*_test.go`), Mongo-dependent behavior self-skips, and the
integration suite drives issuance through the service because this feature deliberately has no
public route that creates a challenge until stage 4 wires registration and sign-in to it.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
