# Implementation Plan: Sign-in Hardening — The Code Step Replaces the Immediate Session, and Unverified Accounts Cannot Renew

**Branch**: `018-auth-2fa-session-gate` | **Date**: 2026-10-20 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/018-auth-2fa-session-gate/spec.md`

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §9 stage 4 — decisions D1 (2FA on every sign-in),
D2 (a session is only issued after verification), D4 (existing accounts verify at first sign-in),
O5 (renewal checks the verification state). This feature is the hardening slice: it wires registration
and sign-in to the challenge domain the preceding feature delivered, gates renewal on the verification
timestamp, and exposes the signed-in account's verification state. It deliberately changes no client
code, does not perform the operational activation (stage 7) and adds no email or broker work
(stage 2).

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Turn the code step delivered by `017-otp-challenges` into the required middle of registration and
sign-in, behind one switch that ships dark:

1. **Registration and sign-in stop issuing a session.** With the gate enabled, `Register` validates
   and creates the unverified account and starts a `verify_email` challenge; `Login` verifies the
   password and starts a `login_otp` challenge. Both answer the challenge handle (created / accepted)
   and set no cookie and no access credential. A session exists only when the already-shipped
   `POST /auth/challenge/verify` consumes a challenge — which records the verification timestamp when
   it was absent and then signs in exactly as before (the `D4` behaviour `OTP-07` already
   implements).
2. **The uniform first step.** Sign-in answers one outcome for an unknown address and for a wrong
   password, and spends the same bcrypt work in both cases, so neither the body nor the timing
   reveals whether an account exists (finding F-05). The purpose of the started challenge is chosen
   by the server from the operation and is never accepted from the client.
3. **Renewal consults the verification state.** After the account is loaded (the seam `PRQ-05`
   delivered) and before anything is issued, `Refresh` refuses an account with no verification
   timestamp: 401, family revoked, no credential — the same containment as the missing-account
   branch, so a session minted before the rule cannot renew once more anywhere.
4. **The signed-in account becomes readable.** `GET /auth/me`, behind the existing bearer
   middleware, reports the account's identifier, canonical address, display name and whether the
   email is verified — as a boolean; the stored instant stays internal. The account wire shape gains
   `email_verified`, superseding the earlier "never on the wire" rule for the read path.

The change is confined to the Go backend: `internal/auth` (the gate decision, the start outcome, the
renewal check, the identity read), `internal/app` (controllers, the new route, wiring),
`internal/user` (one DTO field), `shared/env` (one optional setting and a boolean reader),
`deploy/` and `Makefile`. **No collection changes, no migration, no third-party dependency, and no
addition to `env.RequiredKeys()`.** Three delivered specifications are amended for truthfulness
(`AUTH-01`, `AUTH-02`, `PRQ-12`, `PRQ-14`, `WAS-02`), which is the traceability half of the change
(spec `TFA-14`).

## Technical Context

**Language/Version**: Go 1.26.4 (`go.mod` `go 1.26.4`; `go version` confirms `go1.26.4 linux/amd64`)
on Echo v5.3.1 (`github.com/labstack/echo/v5 v5.3.1`), MongoDB driver v2.9.1
(`go.mongodb.org/mongo-driver/v2 v2.9.1`), `testify` v1.12.1, mockery v2.53.5 — the pins the
repository already carries.

**Primary Dependencies**: none added. The gate decision is plain service policy; the timing-uniform
sign-in uses the existing `PasswordHasher` (bcrypt) with one package-level dummy digest; the identity
read reuses `user.Repository.FetchByID`; configuration reuses `shared/env`; logging stays on
`shared/logger`; typing stays on `shared/apperr`; route guards reuse the existing bearer middleware
and per-address limiter.

**Storage**: MongoDB 7.0 standalone. **No collection, index or document shape changes.** `users` is
read through `FetchByID` and its verification timestamp continues to be written only by the existing
idempotent `MarkEmailVerified` (PRQ-13), which `VerifyChallenge` already calls. The gate is a
boot-time setting and rewrites nothing, so enabling or disabling it leaves every account byte-for-byte
unchanged (`TFA-07`, `SC-007`).

**Testing**: `go test ./...` and `go test -race ./...` with no service required — the service is
driven through mockery `Repository` mocks and the existing challenge fakes; the controllers through
the Echo router with the same fakes; the flag parsing, the DTO projection and the timing path as pure
unit suites. The end-to-end register → verify → session flow lives in the self-skipping Mongo
integration suite. Table-driven `t.Run` suites with `Happy path:` / `Edge:` / `Error:` prefixes,
`require` for preconditions and `assert` otherwise. Failure paths assert the typed `apperr` code and,
at the HTTP boundary, the mapped status. The injected `auth.Clock` drives every boundary with **zero
sleeps** (`TFA-15`, `SC-005`). The suites that currently assume "registration and sign-in return a
session" (`auth_test.go`, `auth_integration_test.go`, and the mock expectations in
`service_test.go`) are updated in the same commit, with the legacy path asserted under the gate
disabled so the two behaviours cannot drift.

**Target Platform**: the Go backend on Linux, behind the existing edge proxy. One process, no new
binary, no broker and no provider call: this feature writes nothing to the outbox directly — it calls
the challenge domain, which does.

**Project Type**: backend web service in the existing Go module; `web/` is untouched, but the client's
premise ("registration authenticates", `WAS-02`) is recorded as superseded for stage 5.

**Performance Goals**: no throughput target. Sign-in adds one bcrypt comparison on the unknown-address
path (deliberate, to equalize work) and one challenge issuance on the success path; renewal adds one
indexed document read ahead of the existing rotation; the identity read is one indexed document read.
No additional round trip on any path.

**Constraints**: the gate is a single on/off setting, optional and documented, **not** added to
`env.RequiredKeys()` (`TFA-07`, `TFA-16`), so `deploy/compose_test.go` keeps validating the same
required set. With the gate disabled every register, sign-in and renewal outcome is exactly today's
(`TFA-12`, `SC-004`). No new failure outcome is added to the wire beyond the typed errors already
mapped: the renewal refusal and the identity-read failures are `UNAUTHORIZED` (401), a registration
whose challenge cannot be started is the existing `EMAIL_UNAVAILABLE` (503) when the daily allowance
is spent and `INTERNAL_ERROR` (500) on a store failure, and a malformed body keeps `INVALID_JSON` /
`INVALID_REQUEST` (`TFA-10`). Layering stays `internal/app → internal/auth → internal/user` with no
upward import; `internal/auth` continues not to import the email domain. All source and strings are
English (`TFA-16`); `go test -race` stays clean (`TFA-15`).

**Scale/Scope**: 4 user stories (2 P1, 2 P2), 16 requirements, 5 internal contracts; roughly 20
touched or created files across `internal/auth`, `internal/app`, `internal/user`, `shared/env`,
`deploy/`, `Makefile` and three existing specs; 8 new or extended suites; 1 regenerated mock
(`auth.Service`).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | No new failure mode introduces a code without a message constant and a mapped status (`TFA-10`): the renewal refusal of an unverified account reuses the existing uniform `UNAUTHORIZED` (401) so it is indistinguishable from a missing account, a registration whose challenge cannot be started keeps `EMAIL_UNAVAILABLE` (503) with a retry hint or `INTERNAL_ERROR` (500), and the identity read answers `UNAUTHORIZED` (401) without a credential or for an account that no longer exists. `shared/apperr`'s mapping test extends to the statuses asserted here; no string-matched error is introduced. | PASS |
| II. Canonical Identifiers | No identity is generated or parsed by this feature: the account identity comes from the bearer token (already validated), and challenge identities are minted and validated entirely by the preceding feature (`OTP-01`). The new route accepts no identifier from the client. | PASS |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | Every requirement lands with tests in the same commit, edge cases first: both gate states for register, sign-in and renewal; a registration whose challenge cannot be started (allowance spent, store failure); unknown address versus wrong password (body and work); renewal for an absent timestamp and an existing one, with the family revoked; the identity read before and after verification, without a credential, and for a missing account; the boolean parsing of the setting; and the "no `Set-Cookie`, no access credential" assertion on the challenge-issuing answers. Then one happy path per suite. `go test -race` stays clean with concurrent sign-ins and concurrent renewals (`TFA-15`, `SC-005`). | PASS |
| IV. English-Only Source | Identifiers, comments, log messages, message constants and the (unchanged) English copy are English (`TFA-16`); no user-facing string is added by this feature. | PASS |
| V. Layered Architecture | The gate is service policy: the decision lives in `internal/auth` (it already owns the challenge policy and the session issuance), the controllers stay thin projections, and the identity read goes through the auth service rather than the controller reaching into `internal/user`. No import direction changes; `internal/auth` still never imports `internal/email`. Decoding stays on `shared/json`, logging on `shared/logger`. | PASS |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | The change strengthens every relevant control: the second factor becomes mandatory on every sign-in, the timing oracle on the sign-in path is removed (`TFA-05`), pre-existing sessions cannot renew without verification (`TFA-06`), and the challenge-issuing answers carry no credential. One session-issuing path remains (the verified challenge), so no dictionary of secrets is created. No credential enters JS-readable storage; no gate is disabled and no CSP or CSRF rule is touched. | PASS (directly served) |
| VII. Installable, Offline-Capable Web Client | No client surface changes in this feature; the PWA's flows and storage rules are untouched and no authenticated response is cached by it. The client's adaptation to the new contract is stage 5, recorded as deferred (`WAS-02` superseded). | PASS (not involved) |
| Additional Constraints — Stack | Go on Echo with MongoDB and `shared/env`, `testify` + mockery tests — unchanged; **no dependency is added** and no new binary is introduced. | PASS |
| Additional Constraints — Spec corpus and traceability | `TFA-01`…`TFA-16` are defined in `spec.md`; the superseded rules (`AUTH-01`, `AUTH-02`, `PRQ-12`, `PRQ-14`, `WAS-02`) remain defined in their own specs and are amended in place with a pointer to the superseding requirement, so `scripts/check_requirement_ids.py` stays green and no citation resolves to a statement that no longer describes the shipped behaviour (`TFA-14`). | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card; recorded for the PR, not claimed by this artifact. | PASS (noted) |
| Development Workflow | The backend sequence (`make fmt && make lint && make vet && make lint-specs`, then `make test-race`) covers the change; `make mocks` regenerates the widened `auth.Service` mock; `deploy/compose_test.go` still validates the required set unchanged and gains a case for the optional switch. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. One implementation per rule: the gate is decided
in **one** place (the service), and the controllers branch on the result rather than re-reading the
setting; the session is issued by **one** method (`signIn`) reachable from **one** verified path; the
uniform sign-in failure has **one** producer (the existing `invalidCredentials` tail, which now always
performs the password work); the renewal refusal is expressed **once**, sharing the missing-account
branch's revocation; and the verification state is projected in **one** place (`user.UserDTO`) so the
read path and any future one cannot disagree. Two choices that could have drifted were closed in
research: the gate lives in the service rather than the controller (`R1`), and the identity read
reports a boolean derived from the stored instant rather than the instant itself (`R4`).

## Project Structure

### Documentation (this feature)

```text
.specs/features/018-auth-2fa-session-gate/
├── spec.md                     # Requirements (TFA-*)
├── plan.md                     # This file
├── research.md                 # Phase 0 — decisions and rejected alternatives
├── data-model.md               # Phase 1 — the gate, the start outcome, the identity read
├── quickstart.md               # Phase 1 — runnable validation guide
├── contracts/                  # Phase 1 — one contract per internal seam
│   ├── endpoints.contract.md
│   ├── start-outcome.contract.md
│   ├── refresh-gate.contract.md
│   ├── identity.contract.md
│   └── configuration.contract.md
├── checklists/
│   └── requirements.md         # Spec quality checklist
└── tasks.md                    # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/auth/
├── service.go                  # EXTENDED: Config.RequireEmailVerification; Register/Login return the start outcome; Refresh refuses an unverified account (TFA-01, TFA-02, TFA-06, TFA-07)
├── start.go                    # NEW: the start outcome type and the single gate decision (TFA-01, TFA-02, TFA-07)
├── profile.go                  # NEW: the identity read over the user store (TFA-08, TFA-09)
├── errors.go                   # EXTENDED: the message constants the new failure paths reuse (TFA-10)
├── service_test.go             # EXTENDED: register/sign-in under both gate states, the uniform work, the renewal gate, no credential on the start answers
├── profile_test.go             # NEW: the identity read, absent account, store failure
└── mocks/mock_Service.go       # REGENERATED: the widened Service signatures

internal/user/
├── user.go                     # EXTENDED: UserDTO gains email_verified; ToDTO derives it from the stored instant (TFA-08)
└── user_test.go                # EXTENDED: the projection for absent and present timestamps

internal/app/
├── auth.go                     # EXTENDED: Register/Login answer the start outcome (201/200 legacy, 201/202 challenge, no cookies); Me controller (TFA-01, TFA-02, TFA-08)
├── auth_test.go                # EXTENDED: both gate states at the HTTP boundary; no Set-Cookie on the challenge answer
├── auth_integration_test.go    # EXTENDED: register → verify → session end to end with the gate on (Mongo, self-skip)
├── router.go                   # EXTENDED: GET /auth/me behind the bearer middleware (TFA-09)
├── router_wiring_test.go       # EXTENDED: the new route is mounted and guarded
├── bootstrap.go                # EXTENDED: read the switch into auth.Config
└── bootstrap_test.go           # EXTENDED: the switch reaches the service config

shared/env/
├── env.go                      # EXTENDED: AUTH_REQUIRE_EMAIL_VERIFICATION + lookupBoolEnv (TFA-07, TFA-16)
└── env_test.go                 # EXTENDED: default off, true/false parsing, an unparseable value, and the required set unchanged

deploy/
├── docker-compose.yml          # EXTENDED: pass the switch through to the api service
├── .env.example                # EXTENDED: the switch and its documented default
├── compose_test.go             # EXTENDED: the required set still holds; the switch is present and optional
└── README.md                   # EXTENDED: what the switch governs and the stage-7 flip

Makefile                        # EXTENDED: export the switch to the app process

.specs/features/006-backend-browser-auth-cookies/spec.md   # AMENDED: AUTH-01/AUTH-02 marked superseded (TFA-14)
.specs/features/015-auth-prerequisites/spec.md             # AMENDED: PRQ-12/PRQ-14 superseded for the read path (TFA-14)
.specs/features/007-web-auth-session/spec.md               # AMENDED: WAS-02 marked superseded, deferred to stage 5 (TFA-14)
```

**Structure Decision**: the gate is a *sign-in policy*, so it lives in `internal/auth` — the package
that already owns the challenge policy, the session issuance and the injected clock. `Register` and
`Login` return one discriminated result whose `Session` and `Challenge` halves are mutually exclusive;
the service fills exactly one according to its configuration, and the controllers project whichever is
present. That keeps a single decision point (the service), avoids a controller reading configuration
in parallel with the service (the drift risk), and keeps the legacy behaviour byte-identical when the
gate is off, because the same method runs and only its tail differs.

No new package is created: the start outcome and the identity read are small files beside the service
that owns them, mirroring `challenge_service.go`'s placement from the preceding feature. The identity
read is exposed through the auth service rather than the controller reading the user store directly, so
`internal/app` continues to treat `internal/auth` as the only auth port and the verification-state
projection stays in one place.

The user store is untouched apart from the DTO: `FetchByID` already exists and `MarkEmailVerified`
already exists, so the only change to `internal/user` is the derived boolean on the wire shape, which
is what lets the read path and any later one report the same state.

The corpora of three delivered features are amended in place (a one-line "superseded by `TFA-0x`"
pointer on the affected requirement), because a citation must resolve to a statement that still
describes the shipped behaviour; the superseding requirements live in this feature's `spec.md`.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
