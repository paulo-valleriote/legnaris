# Tasks: Email Code Challenges — Six-digit Codes, Atomic One-shot Consumption and the Unified Verify/Resend Endpoints

**Input**: Design documents from `.specs/features/017-otp-challenges/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Mandatory, not optional. The constitution (Principle III, NON-NEGOTIABLE) and
`AGENTS.md` require every behavior change to ship tests in the same commit: edge cases first, then
one representative happy path, with the typed `apperr` code and the mapped HTTP status asserted on
every failure path. Expiry/cooldown/ceiling boundaries are driven by the injected `auth.Clock` with
zero sleeps (`OTP-20`, `SC-007`).

**Organization**: Tasks are grouped by user story so each story can be implemented, tested and
delivered independently. Requirement IDs (`OTP-*`) come from `spec.md`; the feature prefix `OTP` is
exclusive to this feature.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task). Two tasks
  that edit the same file are never both `[P]`.
- **[Story]**: `[US1]`…`[US4]` for user-story phases. Setup, Foundational and Polish tasks carry no
  story label.
- Every task names the exact repository file(s) it touches.

## Path Conventions

This is a Go backend in one module; all paths are repository-relative (`internal/…`, `shared/…`,
`deploy/…`). Tests live next to the code as `*_test.go`; MongoDB-dependent suites self-skip without
a reachable database.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the baseline before changing anything. This feature adds no dependency, so setup
is a verification step.

- [ ] T001 Confirm `go version` reports go1.26.4 and `go.mod`/`go.sum` stay byte-identical (no new module); run the existing baseline gates `make fmt && make lint && make vet && make lint-specs` and `make test-race` and record that they are clean before the first change (`OTP-20`).

**Checkpoint**: Clean baseline; no dependency drift to attribute later failures to.

---

## Phase 2: Foundational (Shared Shapes, No User Story)

**Purpose**: Deliver the shapes every story consumes — the challenge record, the code toolkit, the
policy, the new error codes, the configuration and the indexes — plus the two ports (`CodeMailer`,
`ChallengeObserver`). No service and no endpoint yet; all six tasks edit different files and may run
in parallel.

- [ ] T002 [P] Add `internal/auth/challenge.go` (`Purpose` with `verify_email`/`login_otp`, the `Challenge` record with the `bson` tags of `data-model.md` §1, and the usability predicate `Usable(now, maxAttempts)` covering unconsumed, unexpired and under-cap) and `internal/auth/challenge_test.go` (zero-value record is unusable; consumed/expired/at-cap rows are unusable; a fresh row is usable; expiry exactly at the instant is expired and one nanosecond before is not, with no sleep) (`OTP-01`, `OTP-05`, `OTP-06`).
- [ ] T003 [P] Add `internal/auth/code.go` (the `codeLength`/`minCodeSecretLength` constants, `CodeGenerator` interface + `NewCodeGenerator` on `crypto/rand`, `CodeDigest` with `NewCodeDigest(secret)` returning `ErrCodeSecretWeak` for an empty, whitespace-only or sub-32-byte secret, `Digest(userID, purpose, code)` as the versioned NUL-separated HMAC-SHA256 hex, and `Equal` on `crypto/hmac.Equal`) and `internal/auth/code_test.go` (10,000 codes match `^\d{6}$` and include a leading-zero case; the digest is deterministic; the account and the purpose each change the digest; `Equal` rejects a one-character difference; the secret length boundary at 31/32 bytes) (`OTP-02`, `OTP-03`, `OTP-18`).
- [ ] T004 [P] Add `internal/auth/challenge_policy.go` (`ChallengePolicy` with `Lifetime`, `Ceiling`, `ResendCooldown`, `MaxAttempts`, `DailySendCap`, `UserWindow`, `MaxPerUserWindow` and the evaluation helpers `Deadline(createdAt, now)` returning `min(now+Lifetime, createdAt+Ceiling)`, `CooldownElapsed(lastSentAt, now)` and `DayStart(now)` — all on the injected clock) and `internal/auth/challenge_policy_test.go` (the deadline clamps at the ceiling; a cooldown boundary one nanosecond before is suppressed and at the instant is allowed; the day start is the UTC midnight of the given instant; no sleeps) (`OTP-05`, `OTP-06`, `OTP-10`, `OTP-13`, `OTP-14`).
- [ ] T005 [P] Add `apperr.CodeOTPInvalid` (`OTP_INVALID` → 401) and `apperr.CodeEmailUnavailable` (`EMAIL_UNAVAILABLE` → 503) to `shared/apperr/err.go` and the matching assertions to `shared/apperr/err_test.go` (each code maps to its documented status; an unmapped code still falls back to 500) (`OTP-08`, `OTP-13`, `OTP-17`).
- [ ] T006 [P] Extend `shared/env/env.go` with the required `EMAIL_CODE_SECRET` (added to `RequiredKeys()`, read with `lookupRequiredStringEnv`) and the optional `AUTH_OTP_TTL` (`10m`), `AUTH_OTP_CEILING` (`30m`), `AUTH_OTP_RESEND_COOLDOWN` (`60s`), `AUTH_OTP_MAX_ATTEMPTS` (`5`), `AUTH_OTP_DAILY_SEND_CAP` (`80`), `AUTH_OTP_USER_WINDOW` (`1h`) and `AUTH_OTP_USER_LIMIT` (`10`) through the existing duration/int lookups; extend `shared/env/env_test.go` (the required set now contains the secret; each optional variable absent/valid/invalid falls back to its documented default with a warning) (`OTP-18`).
- [ ] T007 [P] Extend `shared/db/indexes.go` with the `auth_challenges` indexes of `data-model.md` §1 — `auth_challenges_active_unique` on `{user_id: 1, purpose: 1}`, unique with `partialFilterExpression: {consumed_at: {$eq: nil}}`, and `auth_challenges_expires_at_ttl` on `{expires_at: 1}` with `expireAfterSeconds: 0` — plus `email_outbox_created_at` on `{created_at: 1}`; extend `shared/db/indexes_test.go` (Mongo, self-skip) to assert each index exists with its options and that a second active row for one account/purpose is rejected while a consumed one is admitted (`OTP-01`, `OTP-06`, `OTP-13`).
- [ ] T008 [P] Add `internal/auth/challenge_mailer.go` (the `CodeMailer` port with `Record(ctx, id, to, purpose, code, expiresInMinutes, now)`, `Requeue(ctx, id, now)` and `SentSince(ctx, since)`, and the `ErrSendBudgetExhausted` sentinel carrying the one-line "why" comment that cites the non-HTTP-layer precedent for a bare sentinel) and `internal/auth/challenge_observer.go` (the `ChallengeObserver` interface with `ChallengeIssued`, `ChallengeVerified`, `ResendRequested`, `SendCapBlocked`, and a nop implementation), with their unit tests in `internal/auth/challenge_mailer_test.go` asserting the nop observer is inert and the sentinel is matchable with `errors.Is` (`OTP-12`, `OTP-13`, `OTP-16`).

**Checkpoint**: The record, the code, the policy, the codes, the configuration, the indexes and both
ports exist; no service, endpoint or route yet.

---

## Phase 3: User Story 1 — A code works once, expires, and cannot be guessed (Priority: P1) 🎯 MVP

**Goal**: A challenge is issued (recording its send intention), consumed exactly once by the correct
code against the atomic store, and answered by `POST /auth/challenge/verify` with the same session
sign-in issues; every failure is the uniform `OTP_INVALID`.

**Independent Test**: With the mockery `ChallengeRepository`/`CodeMailer` mocks and a Mongo-backed
(self-skipping) store, issue a challenge and verify it — 200 with a session and the account's
verification timestamp set; replay it — 401 and nothing issued; race two correct verifications — one
200, one 401; step past the deadline with the injected clock — 401; exhaust five wrong guesses — 401
for every one and never a session.

### Tests for User Story 1 (write first; they must fail to compile until T012–T018 land — see Notes)

- [ ] T009 [US1] Add `internal/auth/challenge_repository_test.go` (Mongo, self-skip): `CloseActive` consumes only the active row; `Insert` admits one active row per `{user_id, purpose}` and rejects the second with `ErrChallengeActive`; `ClaimAttempt` increments `attempts` and returns the post-image while usable, and reports `ErrChallengeUnusable` for a consumed, expired, at-cap or unknown row; two concurrent `ClaimAttempt`s both return; two concurrent `Consume` calls report exactly one winner; a second `Consume` reports `false`; `Fetch` classifies unknown/expired/consumed/exhausted; every conditional write on an unusable row leaves it untouched (`OTP-01`, `OTP-04`, `OTP-05`, `OTP-06`).
- [ ] T010 [P] [US1] Extend `internal/email/outbox/repository_test.go` (Mongo, self-skip): `Clone` writes a new row with a fresh canonical identity, the same `to`/`template`/`payload`, `status = pending`, `attempts = 0` and `next_attempt_at == now`, and leaves the original untouched; `Clone` on an unknown identity is a typed not-found; `CountSince` counts only rows at or after the instant and `email_outbox_created_at` exists (`OTP-10`, `OTP-13`).
- [ ] T011 [P] [US1] Add `internal/app/email_intent_test.go` with a fake outbox store (depends on T015's `Clone`/`CountSince` interface): `Record` maps the identity, recipient, template and payload exactly as the message contract requires and logs nothing; `Requeue` returns the fresh identity from `Clone`; `SentSince` forwards the window; a store failure is returned unwrapped (`OTP-10`, `OTP-12`).
- [ ] T012 [US1] Add `internal/auth/challenge_service_test.go` with the mockery `ChallengeRepository` and `CodeMailer` mocks: `IssueChallenge` generates a canonical challenge identity and a canonical intention identity, closes the previous active challenge, inserts the challenge **before** recording the intention, and returns `challenge_id`/`expires_in`/`resend_after` from the policy; a wrong code increments the attempt and fails uniformly; a right code consumes exactly once, records the verification timestamp when absent and preserves an existing one, and issues a session; a consumed winner loses to a concurrent consumer; a store failure is `INTERNAL_ERROR` and never a code failure; a weak secret never reaches the service (constructed earlier) (`OTP-01`, `OTP-04`, `OTP-05`, `OTP-07`, `OTP-08`, `OTP-12`, `OTP-17`).
- [ ] T013 [P] [US1] Add `internal/app/auth_challenge_test.go` through the router with fakes: verify answers 200 with the session body and both cookies on success; every failure kind (unknown, malformed identity, expired, consumed, wrong, exhausted) answers the **byte-identical** 401 `OTP_INVALID` body; a malformed body answers 400 `INVALID_REQUEST`/`INVALID_JSON`/`UNKNOWN_FIELD`; a store failure answers 500; no response body or header contains a six-digit code (`OTP-07`, `OTP-08`, `OTP-15`, `OTP-17`).

### Implementation for User Story 1

- [ ] T014 [US1] Add `internal/auth/challenge_repository.go`: the `ChallengeRepository` interface and its Mongo implementation over `auth_challenges` exactly as `contracts/challenge.contract.md` fixes it — `CloseActive` (one `UpdateMany` on `{user_id, purpose, consumed_at: null}`), `Insert` (one `InsertOne`, translating the duplicate key to `ErrChallengeActive`), `ClaimAttempt` (one `findOneAndUpdate` with the `consumed_at`/`expires_at`/`attempts` filter and `$inc`), `Consume` (one `findOneAndUpdate` filtered by `code_hash`), `Fetch`, and the `ErrChallengeNotFound`/`ErrChallengeUnusable`/`ErrChallengeActive` sentinels; add `legnaris/internal/auth` to `.mockery.yaml` and run `make mocks` (`OTP-01`, `OTP-04`, `OTP-05`, `OTP-06`).
- [ ] T015 [US1] Extend `internal/email/outbox/repository.go` with `Clone(ctx, id, now)` (read the identified intention and insert a new pending row with a fresh canonical identity, the same `to`/`template`/`payload`, `attempts = 0`, `next_attempt_at = now`; typed not-found for an unknown identity) and `CountSince(ctx, since)` (one `CountDocuments` on `created_at ≥ since`); add `ErrIntentNotFound`; regenerate `internal/email/outbox/mocks/mock_Repository.go` with `make mocks` (`OTP-10`, `OTP-13`).
- [ ] T016 [P] [US1] Add `internal/app/email_intent.go`: the `CodeMailer` adapter over `internal/email/outbox` — `Record` builds the intent with `outbox.NewIntent(id, to, template, payload, now)` where `template` and the payload's `purpose` are the challenge purpose and the payload's `expires_in_minutes` is the policy lifetime, then calls `Repository.Create`; `Requeue` calls `Clone`; `SentSince` calls `CountSince`; the code is never logged and the address is masked in any log line (`OTP-10`, `OTP-12`, `OTP-15`).
- [ ] T017 [US1] Extend `internal/auth/service.go` (the `Service` interface gains `IssueChallenge`/`VerifyChallenge`/`ResendChallenge`; `ServiceImpl` gains the `ChallengeRepository`, `CodeGenerator`, `CodeDigest`, `CodeMailer`, `ChallengeObserver` and `ChallengePolicy` fields, supplied through one `ChallengeDeps` value added to `NewService` so the parameter list does not grow unbounded) and add `internal/auth/challenge_service.go` with `IssueChallenge` (close active, generate the code and its digest, check the daily cap through `SentSince`, insert the challenge, record the intention last) and `VerifyChallenge` (validate the identity, `ClaimAttempt`, compute the digest, `Equal`, `Consume`, `MarkEmailVerified` when absent, then the existing `signIn`) and the single uniform-failure producer (`MsgOTPInvalid` = `invalid or expired code`, `apperr.CodeOTPInvalid`, 401); map `ErrSendBudgetExhausted` to `apperr.CodeEmailUnavailable` (503) with its exported message constant, defined with the others in `internal/auth/errors.go`; regenerate `internal/auth/mocks/mock_Service.go` — used by `internal/app/auth_test.go`, `error_mapping_test.go` and `cors_test.go` — with `make mocks`, so widening the interface cannot break those suites' compilation (`OTP-01`, `OTP-04`, `OTP-05`, `OTP-07`, `OTP-08`, `OTP-09`, `OTP-12`, `OTP-13`, `OTP-17`).
- [ ] T018 [US1] Add `internal/app/auth_challenge.go` (the `Verify` controller: strict decode, service call, `setSessionCookies` + `response.OK(sessionResponse(session))` on success) and wire the route in `internal/app/router.go` (`POST /auth/challenge/verify` inside `registerAuthRoutes`, behind the address limiter, with no origin guard) and the new dependencies in `internal/app/bootstrap.go` (challenge repository, generator, digest built from `EMAIL_CODE_SECRET` failing the boot with a typed error when weak, the `CodeMailer` adapter, the observer and the policy from the environment) plus the matching `internal/app/bootstrap_test.go` asserting `NewModules` rejects a weak, blank and absent secret with a typed error and starts with a valid one (`OTP-07`, `OTP-08`, `OTP-18`, `OTP-19`, `SC-008`).
- [ ] T019 [US1] Extend `internal/app/auth_integration_test.go` (Mongo, self-skip) with the end-to-end issue → verify journey: a seeded account, `IssueChallenge`, `POST /auth/challenge/verify`, 200 with the session cookies and the account's verification timestamp set, then the replay answering the uniform 401; assert a stored digest that is not the code and exactly one pending intention whose payload carries it (`OTP-02`, `OTP-04`, `OTP-07`, `OTP-12`).
- [ ] T020 [P] [US1] Extend `internal/app/auth_test.go` (or the router inventory suite) so the auth route inventory assertion covers the two new paths and their methods — proving no existing route changed and that **no** route accepts or echoes the challenge purpose, which is resolved server-side (`OTP-09`, `OTP-19`).

**Checkpoint**: US1 is independently complete — a challenge can be issued and verified once, with a
session, and every failure is uniform; no resend, limit or metric exists yet.

---

## Phase 4: User Story 2 — Asking for the code again is safe and predictable (Priority: P1)

**Goal**: `POST /auth/challenge/resend` is client-idempotent, cooldown-bounded and ceiling-clamped,
never rotates the code, records a new intention carrying the same code, and is answered
indistinguishably for unknown, consumed, malformed and expired identities.

**Independent Test**: With the fakes and the Mongo store, resend an active challenge — 202, the code
unchanged, one new intention whose payload equals the original's; resend inside the cooldown — 202 and
no new intention; resend repeatedly with the stepped clock — the deadline clamps at the ceiling;
resend an unknown/consumed/expired identity — the same 202 body as an accepted resend.

### Tests for User Story 2 (write first; they must fail to compile until T022–T024 land — see Notes)

- [ ] T021 [US2] Extend `internal/auth/challenge_service_test.go` with the resend matrix: a resend inside the cooldown records no intention but answers 202 with the configured window; a resend beyond the cooldown extends the deadline via `Extend` and clones the stored intention, leaving the code and the challenge identity unchanged; the deadline never passes `ceiling_at`; an unknown, malformed, consumed or expired identity answers identically and records nothing; a concurrent pair of resends records one send (`OTP-10`, `OTP-11`).
- [ ] T022 [US2] Extend `internal/app/auth_challenge_test.go` with resend through the router: 202 with the identical body for every case (re-sent, suppressed, unknown, consumed, expired, malformed identity), `resend_after` always present, 400 for a malformed body, and no code in the response or the headers (`OTP-10`, `OTP-11`, `OTP-15`).

### Implementation for User Story 2

- [ ] T023 [US2] Extend `internal/auth/challenge_service.go` with `ResendChallenge`: validate the identity, `Fetch` the row for `last_intent_id` and `ceiling_at`, compute the clamped deadline, check the daily cap (mapping `ErrSendBudgetExhausted` to `apperr.CodeEmailUnavailable`, 503, with the shared message constant), `Extend` (returning suppressed when the cooldown or a concurrent resend wins → 202 with no send) and then `Requeue` the stored intention; a resend never rotates the code and never changes the identity (`OTP-10`, `OTP-11`, `OTP-13`).
- [ ] T024 [US2] Extend `internal/app/auth_challenge.go` with the `Resend` controller (strict decode, 202 with `{resend_after}`, `EMAIL_UNAVAILABLE` with `Retry-After` when the cap refuses) and register `POST /auth/challenge/resend` in `internal/app/router.go` inside `registerAuthRoutes` behind the address limiter and with no origin guard (`OTP-11`, `OTP-13`, `OTP-19`).
- [ ] T025 [US2] Extend `internal/app/auth_integration_test.go` with the issue → resend → verify journey: two resends answer the same 202, exactly one additional intention is recorded, the code still verifies afterwards, and a resend on the consumed challenge changes nothing (`OTP-10`, `OTP-11`).

**Checkpoint**: Both P1 stories are complete and independently testable — the code step works end to
end, with or without a resend.

---

## Phase 5: User Story 3 — The code step reveals nothing and leaks nothing (Priority: P2)

**Goal**: Every verification failure is indistinguishable in status and body, the classification
reaches only metrics and logs, and no code or unmasked address appears in any response, header, log
line or error message.

**Independent Test**: Drive all five verification failures and assert one identical body and status;
scan every response, header and captured log line from the suites for a six-digit code and for an
unmasked address and assert neither appears; assert a logged challenge event identifies the challenge
by identity and purpose with a masked recipient.

### Tests and implementation for User Story 3

- [ ] T026 [US3] Extend `internal/app/auth_challenge_test.go` with the disclosure sweep: a table driving unknown, malformed, expired, consumed, wrong and exhausted verification, asserting the response bodies are byte-identical and the status is always 401; a suite-level scan of every recorded response body, response header, error message and captured log line for `\b\d{6}\b` (outside payload fixtures) and for a full email address, asserting none is found (`OTP-08`, `OTP-15`, `SC-002`, `SC-003`).
- [ ] T027 [US3] Add the masked-address log helper to `internal/auth/challenge_service.go` (`maskEmail` → the first local-part character, `***`, the domain) and use it in every challenge log line together with the challenge identity and purpose; add `internal/auth/challenge_service_test.go` cases asserting the helper masks short local parts, addresses without a domain and a full address, and that no challenge log line contains the code (`OTP-15`, `OTP-21`).
- [ ] T028 [US3] Extend `internal/app/email_intent_test.go` and `internal/auth/challenge_service_test.go` to assert the disclosure rules on the send path: the recorded payload's `expires_in_minutes` is the configured lifetime (the copy rule behind `contracts/send-intent.contract.md`), the payload is never logged, and the adapter's log line masks the recipient (`OTP-15`).

**Checkpoint**: US3 is verifiable on its own through the disclosure sweep, whichever P1 path produced
the state.

---

## Phase 6: User Story 4 — The code path cannot be turned into an email cannon, and it can be watched (Priority: P2)

**Goal**: Every key that can trigger work is bounded — address, account and challenge — the daily cap
refuses with a typed 503 and a retry hint, and issuance, verification results, resends and cap
refusals are observable without a code ever being observable.

**Independent Test**: Drive the address limiter and the durable per-account bound past their budgets
and assert a typed 429 with `Retry-After` and no send; saturate the cap and assert 503 with
`Retry-After` and an unchanged outbox; assert each counter advances with its documented label.

### Tests for User Story 4 (write first; they must fail until T031–T033 land — see Notes)

- [ ] T029 [US4] Extend `internal/auth/challenge_service_test.go` with the bound matrix: the account past `MaxPerUserWindow` challenges in `UserWindow` is refused with the typed limit outcome before any work; a store failure during the count is `INTERNAL_ERROR`; the window boundary one nanosecond before and at the instant, on the stepped clock, with no sleeps (`OTP-14`, `OTP-20`).
- [ ] T030 [US4] Extend `internal/app/auth_challenge_test.go` and `internal/app/ratelimit_test.go` with the HTTP bound matrix: the address limiter on both routes answers 429 `RATE_LIMITED` with `Retry-After` and records no send; the account bound answers the same; the cap answers 503 `EMAIL_UNAVAILABLE` with `Retry-After` and leaves the outbox unchanged; the cap on a resend suppresses nothing that a legitimate later attempt could not retry (`OTP-13`, `OTP-14`).
- [ ] T031 [US4] Extend `internal/platform/metrics/metrics_test.go` with the four new series: `legnaris_auth_otp_issued_total{purpose}`, `legnaris_auth_otp_verify_total{result}` for all six results, `legnaris_auth_otp_resend_total{result}` for all three, and `legnaris_auth_otp_cap_blocked_total`; assert the type satisfies `auth.ChallengeObserver` and that no series carries a code, a challenge identity or an unmasked address (`OTP-16`).

### Implementation for User Story 4

- [ ] T032 [US4] Extend `internal/auth/challenge_service.go` with the durable per-account bound (one `CountForUserSince` over `policy.UserWindow` before issuance, verification and resend, refusing with the existing rate-limit outcome) and the observer calls behind every outcome (`ChallengeIssued`, `ChallengeVerified` with the six-value classification from the failure diagnostic, `ResendRequested`, `SendCapBlocked`) (`OTP-14`, `OTP-16`).
- [ ] T033 [US4] Add the four counters and the `ChallengeObserver` implementation to `internal/platform/metrics/metrics.go` (its own registry, names and labels exactly as `contracts/observability.contract.md` fixes them) and pass it as the challenge observer from `internal/app/bootstrap.go` (`OTP-16`).
- [ ] T034 [US4] Extend `internal/auth/challenge_repository_test.go` and `internal/app/auth_integration_test.go` with the cap end-to-end: seed intentions down to the cap, issue — 503 with no new intention; resend at the cap — 503 and the deadline unmoved; one below the cap — the intention is written (`OTP-13`).

**Checkpoint**: All four stories are complete; the bounds and the counters are in place and no code
is observable anywhere.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T035 [P] Extend the deployment for the new required secret and the optional settings: `deploy/.env.example`, `deploy/docker-compose.yml` (pass every variable through), `deploy/compose_test.go` (the required set now includes `EMAIL_CODE_SECRET`; the optional keys are interpolations) and `deploy/README.md` (what each setting governs), plus `Makefile` (export the new variables to the app process) (`OTP-18`).
- [ ] T036 [P] Record the decision in `.specs/STATE.md` as a new `AD-*` entry: the challenge domain living in `internal/auth`, the digest-only challenge with the intention as the code's single durable home, the required `EMAIL_CODE_SECRET` exception, and the count-based daily cap — following the `AD-016`/`AD-020` precedent for a decision that warrants a record (`OTP-03`, `OTP-13`, `OTP-18`).
- [ ] T037 Run the full gate — `make fmt && make lint && make vet && make lint-specs` then `make test-race` — and validate `quickstart.md` section by section (boot with and without the secret, the unit/race suites, the Mongo integration journeys, the disclosure sweep, the store invariants); record any deviation instead of weakening an assertion (`OTP-20`, `OTP-21`, `SC-007`, `SC-008`).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies.
- **Foundational (Phase 2)**: depends on Setup; **blocks every user story** (T002–T008 are all `[P]`
  and touch different files).
- **US1 (Phase 3)**: depends on Phase 2. T014 (store), T015 (outbox) and T017 (service) are
  sequential; T011 (the adapter test) and T016 (the adapter) both depend on T015; T018 depends on
  T017; T019 depends on T014–T018.
- **US2 (Phase 4)**: depends on Phase 2 and on US1's service/controller files (T017, T018) because it
  appends to them.
- **US3 (Phase 5)**: depends on US1 and US2 (it sweeps their paths).
- **US4 (Phase 6)**: depends on US1 and US2 for the paths it bounds; T033 depends on T008's observer
  interface.
- **Polish (Phase 7)**: T035 depends on T006 (the variables exist); T037 depends on everything.

### User Story Dependencies

- **US1 (P1)**: independent once Phase 2 lands; the MVP.
- **US2 (P1)**: independent of US1 in behavior, but it edits `challenge_service.go`,
  `auth_challenge.go` and `router.go`, so it is sequenced after US1 rather than parallel.
- **US3 (P2)** and **US4 (P2)**: both consume the state US1/US2 produce; they are independently
  testable and can proceed in either order, but both touch `challenge_service.go`, so they are
  sequenced one after the other.

### Within Each User Story

- Tests are written first and must fail (they will not compile until the implementation tasks land).
- Shapes before repositories, repositories before services, services before controllers, controllers
  before integration.
- No task weakens an existing assertion; a failing gate is fixed in behavior, never in the test.

### Parallel Opportunities

- T002–T008 (Phase 2) run in parallel: seven different files.
- T009, T010, T011 and T013 (US1 tests) run in parallel once Phase 2 is done.
- T016 and T020 are `[P]`; T005's `shared/apperr` edit and T006's `shared/env` edit never touch the
  same file.
- T029–T031 (US4 tests) touch three different test files and run in parallel.
- T035 and T036 (Polish) are independent.

---

## Parallel Example: Phase 2

```bash
# Seven different files, no dependency between them:
Task: "Add internal/auth/challenge.go and challenge_test.go (OTP-01, OTP-05, OTP-06)"
Task: "Add internal/auth/code.go and code_test.go (OTP-02, OTP-03, OTP-18)"
Task: "Add internal/auth/challenge_policy.go and challenge_policy_test.go (OTP-05, OTP-06, OTP-10)"
Task: "Add the two apperr codes and their mapping test (OTP-08, OTP-13, OTP-17)"
Task: "Extend shared/env with the required secret and the optional settings (OTP-18)"
Task: "Extend shared/db/indexes.go with the challenge indexes (OTP-01, OTP-06, OTP-13)"
Task: "Add the CodeMailer port and the ChallengeObserver seam (OTP-12, OTP-13, OTP-16)"
```

## Parallel Example: User Story 1 tests

```bash
# Four different suites, all failing until the implementations land:
Task: "internal/auth/challenge_repository_test.go (Mongo) (OTP-01, OTP-04…OTP-06)"
Task: "internal/email/outbox/repository_test.go Clone/CountSince (OTP-10, OTP-13)"
Task: "internal/app/email_intent_test.go (OTP-10, OTP-12)"
Task: "internal/app/auth_challenge_test.go verify paths (OTP-07, OTP-08, OTP-15, OTP-17)"
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. Phase 1 Setup, then Phase 2 Foundational.
2. Phase 3 US1: issue + verify + the verify endpoint + the integration journey.
3. **STOP and VALIDATE**: run the US1 independent test and the disclosure sweep for the verify path.
4. The code step is complete for the happy path and the replay/exhaustion edges — deployable behind
   the (still absent) stage-4 caller.

### Incremental Delivery

1. Foundation → US1 (verify) → validate → US2 (resend) → validate → US3 (disclosure) → US4 (bounds
   and metrics) → Polish.
2. Each story keeps the previous ones green and adds one independent capability; the route surface is
   additive and no existing route changes.

### Notes

- Requirement IDs cited in each task resolve to `spec.md`; `make lint-specs` is the gate.
- MongoDB-dependent suites (`challenge_repository_test.go`, the outbox additions, the integration
  journey, `shared/db`) self-skip without a reachable `DATABASE_URI`, so `go test ./...` needs no
  service.
- Tests are written before their implementation and are expected not to compile until it lands; the
  integration suite drives `IssueChallenge` because no public route creates a challenge in this
  stage (stage 4 wires registration and sign-in).
- The daily-cap check lands with T017 (issuance) and T023 (resend) because the policy lives in the
  service; US4 owns its tests, its 503 mapping and its counter (`OTP-13`, `OTP-16`).
- No file is edited by two `[P]` tasks; where two stories touch one file they are sequenced.
