# Tasks: Sign-in Hardening — The Code Step Replaces the Immediate Session, and Unverified Accounts Cannot Renew

**Input**: Design documents from `.specs/features/018-auth-2fa-session-gate/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Mandatory, not optional. The constitution (Principle III, NON-NEGOTIABLE) and `AGENTS.md`
require every behavior change to ship tests in the same commit: edge cases first, then one
representative happy path, with the typed `apperr` code and the mapped HTTP status asserted on every
failure path. The gate is exercised on **both** settings so the legacy contract stays measured
(`TFA-12`, `SC-004`).

**Organization**: Tasks are grouped by user story so each story can be implemented, tested and
delivered independently. Requirement IDs (`TFA-*`) come from `spec.md`; the feature prefix `TFA` is
exclusive to this feature.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task). Two tasks that
  edit the same file are never both `[P]`.
- **[Story]**: `[US1]`…`[US4]` for user-story phases. Setup, Foundational and Polish tasks carry no
  story label.
- Every task names the exact repository file(s) it touches.

## Path Conventions

This is a Go backend in one module; all paths are repository-relative (`internal/…`, `shared/…`,
`deploy/…`, `.specs/…`). Tests live next to the code as `*_test.go`; MongoDB-dependent suites self-skip
without a reachable database.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the baseline before changing anything. This feature adds no dependency, so setup
is a verification step.

- [X] T001 Confirm `go version` reports go1.26.4 and `go.mod`/`go.sum` stay byte-identical (no new module); run the baseline gates `make fmt && make lint && make vet && make lint-specs` and `make test-race` and record them clean before the first change (`TFA-15`).

**Checkpoint**: Clean baseline; no dependency drift to attribute later failures to.

---

## Phase 2: Foundational (Shared Shapes, No User Story)

**Purpose**: Deliver the two shapes every story consumes — the gate setting and the start outcome —
so no user story has to invent them. Two tasks, two different files, both `[P]`.

- [X] T002 [P] Extend `shared/env/env.go` with the optional `AUTH_REQUIRE_EMAIL_VERIFICATION` (default `false`) read through a new `lookupBoolEnv(key, defaultValue)` that parses with `strconv.ParseBool` and, like `lookupDurationEnv`/`lookupIntEnv`, logs a structured warning and returns the default for an unset, empty or unparseable value; add `Variables.RequireEmailVerification`; extend `shared/env/env_test.go` (absent → false; empty → false; each accepted spelling; `maybe` → false plus one warning; `RequiredKeys()` still exactly `{DATABASE_URI, JWT_SECRET, EMAIL_CODE_SECRET}`) — the contract is `contracts/configuration.contract.md` (`TFA-07`, `TFA-16`).
- [X] T003 [P] Add `internal/auth/start.go`: the `StartOutcome` type of `contracts/start-outcome.contract.md` (mutually exclusive `Session *Session` / `Challenge *ChallengeIssued`) with its doc comment stating the exactly-one-half invariant, and add `RequireEmailVerification bool` to `auth.Config` in `internal/auth/service.go` with a comment naming it as the single gate decision point (`TFA-01`, `TFA-02`, `TFA-07`).

**Checkpoint**: The gate can be configured and the outcome shape exists; no behaviour changed yet, so
every existing suite still passes.

---

## Phase 3: User Story 1 — Proving a password is no longer proving the mailbox (Priority: P1) 🎯 MVP

**Goal**: With the gate enabled, `POST /auth/register` creates the unverified account and starts a
`verify_email` challenge (201, no cookie) and `POST /auth/login` starts a `login_otp` challenge (202, no
cookie); invalid credentials answer the one uniform 401 and start nothing; the recorded challenge is the
only route to a session.

**Independent Test**: With the fakes, register under the gate — 201 with `{challenge_id, expires_in,
resend_after}` and **no `Set-Cookie`**; sign in — 202 with the same shape; verify the returned handle —
200 with a session; fail the password and the unknown-address case — byte-identical 401 and the password
comparison performed in both; drive a challenge-start failure (allowance spent, store error) — the typed
failure with no session and no cookie, and the account still reachable through sign-in.

### Tests for User Story 1 (write first; they must fail to compile until T006–T007 land — see Notes)

- [X] T004 [US1] Extend `internal/auth/service_test.go` (mockery `Repository`, `ChallengeRepository`, `CodeMailer`, `RefreshTokenRepository`, `TokenManager`, `CSRF`, `PasswordHasher`): register with the gate off returns a `Session` and no challenge (legacy parity); register with the gate on returns a `Challenge` whose purpose is `verify_email`, calls `Create` before `IssueChallenge`, and leaves `EmailVerifiedAt` absent; login with the gate on returns a `Challenge` whose purpose is `login_otp`; **exactly one half is ever set**; the password comparison is invoked on the unknown-address path too (assert on the hasher fake's call count) while the answer stays the one uniform 401; a mismatching password starts nothing; an unparseable address is the same uniform 401; `IssueChallenge` failing with `CodeEmailUnavailable` and with a store error propagates unchanged with no credential and leaves the account recoverable; two concurrent sign-ins produce two challenges of which only the later verifies (`TFA-01`, `TFA-02`, `TFA-03`, `TFA-04`, `TFA-05`, `TFA-13`).
- [X] T005 [P] [US1] Extend `internal/app/auth_test.go` through the router with fakes: both gate states for `POST /auth/register` (201 session body + both cookies vs 201 challenge body + **no `Set-Cookie`**) and `POST /auth/login` (200 session body + cookies vs 202 challenge body + no cookie); assert the challenge body's three fields and that no response carries an access token, a refresh token or a CSRF token; the four credential failures answer byte-identical 401 `UNAUTHORIZED` bodies; a malformed body answers 400 `INVALID_JSON`/`INVALID_REQUEST`/`UNKNOWN_FIELD`; the allowance-spent registration answers 503 `EMAIL_UNAVAILABLE` with `Retry-After`, the store failure 500 `INTERNAL_ERROR`; extend `shared/apperr/err_test.go` so every code asserted here has its documented status, which is the plan's mapping-test promise; and scan every captured response body, response header and log line from the new register and sign-in paths for `\b\d{6}\b` and for a full address, asserting neither appears (`TFA-01`, `TFA-02`, `TFA-05`, `TFA-10`, `TFA-13`, `TFA-16`, `SC-001`, `SC-002`).

### Implementation for User Story 1

- [X] T006 [US1] Extend `internal/auth/service.go` and `internal/auth/start.go` with the gate branch: `Register` keeps validation/hashing/`Create` then, per `Config.RequireEmailVerification`, returns `Session` (via `signIn`, unchanged) or `Challenge` (via `IssueChallenge` with `PurposeVerifyEmail` and the canonical address, propagating its typed errors); `Login` keeps canonicalization/`FetchByEmail`/`Compare`, performs one comparison against the package-level dummy bcrypt digest on the unknown-address path before the uniform 401, and then branches the same way with `PurposeLoginOTP`; the purpose is bound at the call site and never read from a command (`TFA-01`, `TFA-02`, `TFA-03`, `TFA-04`, `TFA-05`, `TFA-13`); run `make mocks` to regenerate `internal/auth/mocks/mock_Service.go` for the changed signatures (`TFA-15`).
- [X] T007 [US1] Extend `internal/app/auth.go` (project the `StartOutcome`: legacy → `setSessionCookies` + session body with 201/200; challenge → the challenge body with 201/202 and **no** cookie write) and `internal/app/bootstrap.go` (copy `Variables.RequireEmailVerification` into `auth.Config`), plus `internal/app/bootstrap_test.go` asserting the parsed gate reaches the service config and that `NewModules` still fails fast only on the existing required keys (`TFA-01`, `TFA-02`, `TFA-07`, `TFA-10`).
- [X] T008 [US1] Extend `internal/app/auth_integration_test.go` (Mongo, self-skip) with the gated journey end to end: register → 201 challenge with no cookie → read the recorded intention's payload code → `POST /auth/challenge/verify` → 200 session + both cookies → the account carries a verification timestamp; sign in again → 202 → verify → 200; a second registration of the same address → 409 with no challenge and no new intention (`TFA-01`, `TFA-04`, `TFA-11`, `TFA-13`).

**Checkpoint**: US1 is independently complete — the code step is the only door to a session, and the
legacy path is untouched with the gate off.

---

## Phase 4: User Story 2 — A session that predates the rule cannot outlive it (Priority: P1)

**Goal**: `Refresh`, with the gate enabled, refuses an account that has no verification timestamp: 401,
the presented credential's family revoked, no access and no rotated credential — after the account load
and before any issuance.

**Independent Test**: Mint a session for an account with no timestamp, enable the gate, renew — 401, zero
credentials issued, family revoked; set the timestamp and renew — 200 with a rotated pair; disable the
gate and renew the same unverified account — 200, unchanged.

### Tests for User Story 2 (write first; they must fail until T010 lands)

- [X] T009 [US2] Extend `internal/auth/service_test.go` with the renewal predicate matrix (mockery `Repository`, `RefreshTokenRepository`): gate on with `EmailVerifiedAt == nil` → `UNAUTHORIZED` with the existing message, `RevokeFamily` called once, `TokenManager.Issue` and `RefreshTokenRepository.Create` never called; gate on with a timestamp → the rotation assertions the existing cases make stay identical; gate off with a timestamp absent → rotation unchanged; a second attempt after the refusal fails again and mints nothing; a missing account and a revoked/expired credential keep deciding first; a `RevokeFamily` failure on the refusal is `INTERNAL_ERROR` with nothing issued; two concurrent renewals of an unverified account issue zero credentials (`TFA-06`, `TFA-12`, `SC-003`).

### Implementation for User Story 2

- [X] T010 [US2] Extend `internal/auth/service.go`'s `Refresh` with the predicate exactly at the position `contracts/refresh-gate.contract.md` fixes (after `FetchByID`, before `Issue`), sharing the missing-account branch's revocation and reusing the existing uniform credential failure (`TFA-06`, `TFA-11`).

**Checkpoint**: Both P1 stories are complete and independently testable — a pre-existing session cannot
outlive the rule.

---

## Phase 5: User Story 3 — The signed-in client can see whether its email is verified (Priority: P2)

**Goal**: `GET /auth/me`, behind the existing bearer middleware, answers the account's identifier,
canonical address, display name and a boolean verification state — never the instant, never a credential.

**Independent Test**: Read as an unverified account → 200 with `email_verified: false`; verify, read
again → only that field changed and no timestamp appears anywhere; read without/with a forged credential
and for a deleted account → 401 with no account data; a store failure → 500.

### Tests for User Story 3 (write first; they must fail until T012–T013 land)

- [X] T011 [P] [US3] Extend `internal/user/user_test.go` with the DTO projection: `EmailVerifiedAt == nil` → `email_verified: false`; a timestamp present → `true`; the instant itself is absent from the JSON for both; the hash is never serialized (`TFA-08`, `TFA-09`).
- [X] T012 [P] [US3] Add `internal/auth/profile_test.go`: the profile of an existing account projects the identifier, canonical address, display name and the derived boolean; an account that no longer exists is a typed `UNAUTHORIZED` (401) with a message constant; a store failure is `INTERNAL_ERROR` and never reported as unauthorized; the stored instant is not part of the result (`TFA-08`, `TFA-09`, `TFA-10`).
- [X] T013 [US3] Extend `internal/app/auth_test.go` and `internal/app/router_wiring_test.go`: `GET /auth/me` answers 200 with the four fields and the boolean for a verified and an unverified account; no credential/malformed/expired/forged → 401 with no account data; a deleted account → 401 (not 404) with the new message constant; a store failure → 500; the route is mounted inside the `/auth` group behind the bearer guard and the route inventory test covers it; extend `internal/app/auth_integration_test.go` (Mongo, self-skip) so the gated journey of T008 re-reads the identity after verification and asserts only `email_verified` changed (`TFA-08`, `TFA-09`, `TFA-10`, `SC-006`).

### Implementation for User Story 3

- [X] T014 [US3] Add `internal/user/user.go` the derived `EmailVerified bool json:"email_verified"` field on `UserDTO`, set in `ToDTO` from `EmailVerifiedAt != nil` (`TFA-08`, `TFA-09`).
- [X] T015 [US3] Add `internal/auth/profile.go` (the profile read over `user.Repository.FetchByID` returning the DTO, mapping a missing account the typed way `contracts/identity.contract.md` fixes) and expose it on the `Service` interface; extend `internal/auth/errors.go` with the message constant the missing-account case reuses; `make mocks` regenerates `internal/auth/mocks/mock_Service.go` (`TFA-08`, `TFA-09`, `TFA-10`).
- [X] T016 [US3] Extend `internal/app/auth.go` with the `Me` controller (read the identity the bearer middleware resolved, call the service, answer `response.OK` with the DTO; no body decoded) and mount `GET /auth/me` in `internal/app/router.go` behind `auth.Middleware(modules.Auth.TokenManager)` per route (`TFA-09`).

**Checkpoint**: The read is available with the gate either way, so the next stage's screens have a
truthful source of verification state.

---

## Phase 6: User Story 4 — The whole change can be deployed dark and turned on deliberately (Priority: P2)

**Goal**: One documented gate, off by default, that the operator flips without changing the required
variable set, the challenge domain, or any account; with it off, the legacy contract is provably
identical.

**Independent Test**: With the gate off, the pre-existing register/sign-in/renewal suites pass
byte-for-byte and the challenge suites pass unchanged; with it on, the three contract changes hold and
the challenge suites still pass; the gate is absent from `RequiredKeys()` and present in the
deployment files.

### Tests and implementation for User Story 4

- [X] T017 [US4] Extend `deploy/.env.example`, `deploy/docker-compose.yml`, `deploy/README.md`, `Makefile` and `deploy/compose_test.go` with the optional gate exactly as `contracts/configuration.contract.md` fixes it: the required set is unchanged, the key is present and optional, and the readme states what it governs and that stage 7 performs the flip (`TFA-07`, `TFA-16`).
- [X] T018 [US4] Extend `internal/app/auth_integration_test.go` and `internal/app/auth_test.go` with the gate parity sweep: with the gate off, the register/sign-in/renewal contract outcomes are byte-identical to the pre-change expectations (status, body and cookies; response time is not part of the parity, because the equalized password work of `TFA-05` applies in both states); with the gate on, the challenge domain's own suites (`internal/auth/challenge_*`) pass unchanged and no account document is rewritten by either setting (`TFA-05`, `TFA-11`, `TFA-12`, `SC-004`, `SC-007`).

**Checkpoint**: The change is deployable dark; the flip is a configuration act, and stage 7 needs no
code change.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T019 [P] Amend the three delivered specs in place with the supersession pointers of `data-model.md` §6 — `.specs/features/006-backend-browser-auth-cookies/spec.md` (`AUTH-01`, `AUTH-02` → `TFA-02`, `TFA-01`), `.specs/features/007-web-auth-session/spec.md` (`WAS-02` → `TFA-02`, client work deferred to stage 5) and `.specs/features/015-auth-prerequisites/spec.md` (`PRQ-12`, `PRQ-14` → `TFA-09`, `TFA-08`, read path only); run `make lint-specs` and confirm every cited ID still resolves (`TFA-14`).
- [X] T020 [P] Record the decision in `.specs/STATE.md` as a new `AD-*` entry: the gate as one optional setting decided inside the auth service with `Register`/`Login` returning a discriminated outcome, the renewal refusal revoking the family, and the boolean projection of the verification state — following the `AD-016`/`AD-020`/`AD-021` precedent (`TFA-06`, `TFA-07`, `TFA-08`).
- [X] T021 Run the full gate — `make fmt && make lint && make vet && make lint-specs` then `make test-race` — and validate `quickstart.md` section by section (the legacy parity run, the gated register → verify → session run, the renewal refusal, the gate parsing, the traceability sweep); record any deviation instead of weakening an assertion (`TFA-15`, `TFA-16`, `SC-001`…`SC-007`).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies.
- **Foundational (Phase 2)**: depends on Setup; **blocks every user story** (T002 and T003 are `[P]`
  and touch different files).
- **US1 (Phase 3)**: depends on Phase 2. T004 (service tests) precedes T006; T005 (HTTP tests)
  precedes T007; T006 precedes T007 (the controller projects the outcome the service now returns);
  T008 depends on T006 and T007.
- **US2 (Phase 4)**: depends on Phase 2 and on US1's `service.go`/`service_test.go` (T006 appends to
  the same file), so it is sequenced after US1 rather than parallel.
- **US3 (Phase 5)**: depends on Phase 2 only for the gate-independent parts; T011 is independent of
  US1/US2, but T013, T015 and T016 touch `auth.go` which US1's T007 also edits, so US3's controller
  task is sequenced after US1.
- **US4 (Phase 6)**: T017 depends on T002 (the variable exists); T018 depends on US1, US2 and US3
  (it sweeps their paths).
- **Polish (Phase 7)**: T019 and T020 are independent; T021 depends on everything.

### User Story Dependencies

- **US1 (P1)**: independent once Phase 2 lands; the MVP.
- **US2 (P1)**: behaviourally independent of US1, but it edits `service.go`/`service_test.go`, so it
  is sequenced after US1.
- **US3 (P2)**: independently testable; its controller edit shares `auth.go`/`auth_test.go` with US1,
  so it follows it.
- **US4 (P2)**: the cross-cutting gate; its parity sweep consumes every story.

### Within Each User Story

- Tests are written first and must fail (they will not compile until the implementation tasks land).
- Shapes before repositories, repositories before services, services before controllers, controllers
  before integration.
- No task weakens an existing assertion; a failing gate is fixed in behaviour, never in the test.

### Parallel Opportunities

- T002 and T003 (Phase 2) run in parallel: two different files.
- T004 and T005 (US1 tests) run in parallel once Phase 2 is done.
- T011 and T012 (US3 tests) run in parallel.
- T019 and T020 (Polish) are independent files (`006`/`007`/`015` specs vs `STATE.md`).

---

## Parallel Example: Phase 2

```bash
# Two different files, no dependency between them:
Task: "Extend shared/env with AUTH_REQUIRE_EMAIL_VERIFICATION and lookupBoolEnv (TFA-07, TFA-16)"
Task: "Add internal/auth/start.go (StartOutcome) and auth.Config.RequireEmailVerification (TFA-01, TFA-02, TFA-07)"
```

## Parallel Example: User Story 1 tests

```bash
# Two different suites, both failing until T006/T007 land:
Task: "internal/auth/service_test.go: both gate states, uniform work, challenge-failure propagation (TFA-01…TFA-05, TFA-13)"
Task: "internal/app/auth_test.go: 201/200 vs 201/202, no Set-Cookie, uniform 401 (TFA-01, TFA-02, TFA-05, TFA-10)"
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. Phase 1 Setup, then Phase 2 Foundational.
2. Phase 3 US1: the gate branch, the two halves of the outcome, the controllers and the gated
   end-to-end journey.
3. **STOP and VALIDATE**: the US1 independent test on both gate settings; the legacy suites still pass.
4. The code step is the only door to a session — deployable with the gate still off.

### Incremental Delivery

1. Foundation → US1 (start a challenge) → validate → US2 (renewal gate) → validate → US3 (identity
   read) → US4 (the gate and the parity sweep) → Polish.
2. Each story keeps the previous ones green and adds one capability; with the gate off, no story
   changes an observable outcome for a single deploy.

### Notes

- Requirement IDs cited in each task resolve to `spec.md`; `make lint-specs` is the gate.
- Tests are written before their implementation and are expected not to compile until it lands; the
  integration suite drives the whole gated journey because the challenge is created by the very
  routes US1 wires.
- The gate branch and the renewal predicate both live in `internal/auth/service.go`, so US1 and US2
  are sequenced rather than parallel; every other `[P]` pair touches different files.
- `make mocks` runs once per interface change (T006, T015); a stale mock is a compile failure, not a
  silent pass.
