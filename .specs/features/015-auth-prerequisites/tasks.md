# Tasks: Auth Prerequisites — Real Client Address, Canonical Email, Verification-Ready Accounts

**Input**: Design documents from `.specs/features/015-auth-prerequisites/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: Mandatory, not optional. The constitution (Principle III, NON-NEGOTIABLE) and
`AGENTS.md` require every behavior change to ship tests in the same commit: edge cases first,
then one representative happy path, with the typed `apperr` code and mapped HTTP status asserted
on every failure path. Tests must never sleep across an expiry boundary (`SC-007`).

**Organization**: Tasks are grouped by user story so each story can be implemented, tested and
delivered independently. Requirement IDs (`PRQ-*`) come from `spec.md`; the feature prefix `PRQ`
is exclusive to this feature.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an incomplete task). Two tasks
  that edit the same file are never both `[P]`.
- **[Story]**: `[US1]`, `[US2]`, `[US3]` for user-story phases. Setup, Foundational and Polish
  tasks carry no story label.
- Every task names the exact repository file(s) it touches.

## Path Conventions

This is a Go backend plus an operator CLI in one module; all paths are repository-relative
(`internal/…`, `shared/…`, `cmd/…`, `deploy/…`). Tests live next to the code as `*_test.go`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm the baseline before changing anything. This feature adds no dependency and
changes no toolchain pin, so setup is a verification step, not scaffolding.

- [ ] T001 Confirm `go version` reports go1.26.4 and `go.mod`/`go.sum` stay byte-identical (no new module, including no test-only one); run the existing baseline gates `make fmt && make lint && make vet && make lint-specs` and `make test-race` and record that they are clean before the first change (`PRQ-18`, `PRQ-19`).

**Checkpoint**: Clean baseline; no dependency drift to attribute later failures to.

---

## Phase 2: Foundational (Shared Seams, No User Story)

**Purpose**: Deliver the two story-agnostic seams the rest of the feature consumes: one injectable
clock (`PRQ-16`) and the verification-ready account record (`PRQ-12`…`PRQ-15`). US1 does not
depend on this phase and can start in parallel; US2 and US3 consume the clock, and later program
features consume the verification operation.

**⚠️ CRITICAL**: T003 depends on T002, and T005 depends on T004. T002 and T004 are independent
packages and may proceed in parallel.

- [ ] T002 [P] Introduce the clock seam in the token manager: add `internal/auth/clock.go` (`type Clock func() time.Time` and a nil→`time.Now` default), change `NewTokenManager` in `internal/auth/token.go` to accept `now Clock`, use it for `iat`/`exp`, and pass it to `jwt.ParseWithClaims` via `jwt.WithTimeFunc(now)`; wire `time.Now` from `internal/app/bootstrap.go` and update the existing constructor call in `internal/app/auth_integration_test.go` so the suite keeps compiling; add `internal/auth/token_test.go` cases for issuance timestamps from a frozen clock, parse honouring the injected clock, and the exact-expiry boundary (expired at `exp`, valid 1 ns before) with zero `time.Sleep` (`PRQ-16`, `SC-007`).
- [ ] T003 Add `Now Clock` to `auth.Config` in `internal/auth/service.go`, default nil to `time.Now` in `NewService`, replace every `time.Now()` in `Register`, `Refresh` and `issueRefreshToken` with `s.now()`, and change the refresh expiry comparison to `!now.Before(stored.ExpiresAt)` so the exact instant is expired; pass one `now` value from `internal/app/bootstrap.go` to both the service and the token manager; extend `internal/auth/service_test.go` with a frozen clock (exact `created_at`/`updated_at`/`expires_at`) and a stepped clock (1 ns before expiry valid, exact instant expired, no sleep) (`PRQ-16`, `SC-007`).
- [ ] T004 [P] Extend the account record: add `EmailVerifiedAt *time.Time` with `bson:"email_verified_at,omitempty"` and `json:"email_verified_at,omitempty"` to `internal/user/user.go`, leave `UserDTO` untouched, and add `internal/user/user_test.go` asserting a zero-value account has no verification timestamp, `Register`-shaped construction leaves it nil, and `ToDTO()`/its JSON expose exactly `id`, `email`, `username`, `created_at` (no `email_verified_at`) (`PRQ-12`, `PRQ-14`, `PRQ-15`).
- [ ] T005 Add `MarkEmailVerified(ctx context.Context, id string, at time.Time) error` to the exported store contract in `internal/user/repository.go` (interface + `RepositoryImpl`), implemented as one `UpdateOne` with filter `{_id: id, email_verified_at: nil}` and `$set: {email_verified_at: at, updated_at: at}`; on `MatchedCount == 0` call `FetchByID` and return `nil` when found (already verified, `updated_at` untouched) or `ErrUserNotFound` when absent, wrapping driver errors with `mark email verified: %w`; regenerate `internal/user/mocks/mock_Repository.go` with `make mocks`; extend `internal/user/repository_test.go` (Mongo, self-skip) for unknown id → `ErrUserNotFound`, first call sets only the two fields, second call preserves both values (no-op), a freshly created account has no timestamp, and concurrent calls keep the first timestamp (`PRQ-13`).

**Checkpoint**: `auth.Clock` is the only auth time source; `email_verified_at` exists and is writable exactly once, with no HTTP surface.

---

## Phase 3: User Story 1 — A blocked client is the attacker, not everyone behind the proxy (Priority: P1) 🎯 MVP

**Goal**: Every per-address decision uses the real client resolved from the forwarding chain, and
only when the direct peer is a trusted proxy. One attacker spends only its own budget; a forged
header from an untrusted peer changes nothing.

**Independent Test**: Through the real `NewModules`/`NewRouter`, with a trusted direct peer and a
forwarded client A, exhaust the per-address limit and assert client B behind the same peer is still
admitted; then send a forged chain from an untrusted peer and assert the forged value has no effect.

### Tests for User Story 1 (write first; they must fail before T009–T011)

- [ ] T006 [P] [US1] Add `internal/platform/clientip/clientip_test.go` with the normative resolution matrix from `contracts/client-address.contract.md`: trusted peer + `A`; trusted peer + `A, P1` where P1 is trusted; all-trusted chain returns the furthest (leftmost) hop; absent header, empty header and unparseable entry return the direct peer; a chain with two header values is walked as one ordered chain; untrusted peer ignores even a syntactically valid chain; unparseable `RemoteAddr` returns the non-empty `Unknown` sentinel and never reads the header; IPv6 with/without brackets, `host:port`, `[::1]:443` and IPv4-mapped IPv6 normalize to one value; the result is never `""`; plus the policy table (nil ranges = loopback + private + link-local, configured ranges replace the non-loopback portion, loopback always trusted, empty non-nil = loopback only, invalid CIDR = error) (`PRQ-01`, `PRQ-02`, `PRQ-03`, `SC-001`, `SC-002`).
- [ ] T007 [P] [US1] Extend `shared/env/env_test.go` for `TRUSTED_PROXY_CIDRS`: unset → `nil`; empty string → `nil`; `" 10.0.0.0/8 , 172.16.0.0/12 "` → trimmed two-element slice; `","` → empty non-nil slice (configured-empty, fail-closed); a syntactically invalid value is preserved for `clientip.New` to reject at boot (env does not silently drop or default it); `RequiredKeys()` stays exactly `DATABASE_URI`, `JWT_SECRET` (`PRQ-03`, `PRQ-18`).
- [ ] T008 [P] [US1] Add `internal/app/client_ip_test.go` using the `newWiringRouter` pattern from `internal/app/router_wiring_test.go` (Mongo client that never dials; no database): set `AUTH_RATE_LIMIT_ATTEMPTS=1` before `env.NewEnv`, build a fresh router per case, and drive `POST /auth/login` with explicit `RemoteAddr` and `X-Forwarded-For` — trusted peer + client A: first call reaches the handler (500 from the unreachable store, not 429), second is 429 with `Retry-After`; trusted peer + client B: not 429; untrusted peer + forged chain: the direct peer's bucket is consumed and varying the forged value changes nothing; no header: the direct peer is used; a `host:port` chain still resolves and must not collapse to the proxy's bucket (`PRQ-01`, `PRQ-02`, `PRQ-04`, `SC-001`, `SC-002`).

### Implementation for User Story 1

- [ ] T009 [US1] Add `internal/platform/clientip/clientip.go` (standard library only: `net`, `net/netip`, `strings`): `const Unknown = "unknown"`, an immutable `Policy` built by `New(configured []string) (*Policy, error)`, and `func (p *Policy) Resolve(r *http.Request) string` implementing the contract — parse `RemoteAddr` host portion (bare address accepted), unparseable → `Unknown` without reading headers, untrusted peer → peer, otherwise walk every `X-Forwarded-For` value right-to-left and return the first untrusted entry (or the leftmost when all are trusted), distrust the whole header on any unparseable entry, and return `addr.Unmap().String()`; `New(nil)` trusts loopback (127.0.0.0/8, ::1/128), private (10/8, 172.16/12, 192.168/16, fc00::/7) and link-local (169.254/16, fe80::/10); a non-nil list trusts loopback plus exactly those prefixes; an invalid prefix returns an error; never return an empty string (`PRQ-01`, `PRQ-02`, `PRQ-03`).
- [ ] T010 [US1] Extend `shared/env/env.go`: add `TrustedProxyCIDRs []string` to `Variables`, read `TRUSTED_PROXY_CIDRS` with `os.LookupEnv` (not the existing slice helper) so unset and empty both yield `nil`, while a set value is split on commas, trimmed, blank entries dropped and returned as a non-nil slice (preserving `","` as configured-empty); do not validate CIDRs here; keep `RequiredKeys()` unchanged (`PRQ-03`, `PRQ-18`).
- [ ] T011 [US1] Wire the policy: in `internal/app/bootstrap.go` build the policy once with `clientip.New(cfg.Variables.TrustedProxyCIDRs)`, return `fmt.Errorf("build client IP policy: %w", err)` on an invalid CIDR, and expose the immutable resolver on `Modules` (e.g. `ClientIP func(*http.Request) string = policy.Resolve`); in `internal/app/router.go` assign `router.IPExtractor = modules.ClientIP` before any route is registered so every existing `c.RealIP()` consumer (the auth limiter and the CSP limiter) reads the same resolution (`PRQ-03`, `PRQ-04`).
- [ ] T012 [P] [US1] Add the optional deployment surface: `deploy/docker-compose.yml` app environment gains `TRUSTED_PROXY_CIDRS: ${TRUSTED_PROXY_CIDRS:-}`; `deploy/.env.example` documents the default (loopback + private + link-local) and a commented example; `deploy/compose_test.go` gains a case asserting the key is present as interpolation on the app service and that `env.RequiredKeys()` still contains only `DATABASE_URI` and `JWT_SECRET`; `Makefile` adds `TRUSTED_PROXY_CIDRS` to `APP_ENV_KEYS` so local runs export it and deploy recipes strip it (`PRQ-03`, `PRQ-18`).

**Checkpoint**: US1 is independently complete — the rule, its configuration, the router wiring and the route-level proof exist; no other story is required.

---

## Phase 4: User Story 2 — A session cannot renew for an account that no longer exists (Priority: P1)

**Goal**: Refresh loads the credential's account after the existing credential rules and before
issuing anything. A missing account answers 401 and revokes the family; a store failure answers 500
and issues nothing; a success names the loaded account.

**Independent Test**: With mocks, mint a refresh credential, remove its account, refresh and assert
401, no `Issue`/rotation, family revoked; repeat with the account present and assert the access
token names it; drive a store failure and assert 500, no credential, family untouched.

### Tests for User Story 2 (write first; they must fail before T016)

- [ ] T013 [P] [US2] Extend `internal/auth/service_test.go` with a table that proves the order in `contracts/refresh-account.contract.md` using mockery `NewMockRepository(t)` and the existing refresh mocks: missing account is checked after CSRF, revoked and expiry (a revoked/expired credential keeps its existing outcome even when the account is missing) and before the first `Issue`; missing account → `CodeUnauthorized` + `MsgInvalidRefreshToken`, `RevokeFamily` called, no access token and no refresh `Create`; account-store failure → `CodeInternal` + `MsgInternal`, no issuance, `RevokeFamily` not called; a failed family revocation on the missing-account path → 500 and still no credential; success issues for the loaded account's id (mock returns an account whose id differs from the credential's `user_id` to discriminate) and rotates in the same family (`PRQ-05`, `PRQ-06`, `PRQ-07`, `PRQ-17`).
- [ ] T014 [P] [US2] Extend `internal/app/auth_test.go` (mock `auth.Service`) with the HTTP-boundary mappings for refresh: missing account → 401 `UNAUTHORIZED` + `MsgInvalidRefreshToken` and no `Set-Cookie`; account-store failure → 500 `INTERNAL_ERROR` + `MsgInternal` and no `Set-Cookie`; assert the response body shape is unchanged (`PRQ-07`, `PRQ-14`, `PRQ-17`).
- [ ] T015 [P] [US2] Extend `internal/app/auth_integration_test.go` (Mongo, self-skip) with the end-to-end refresh case: register, delete the account document, refresh with the stored cookies and CSRF header → 401, no `Set-Cookie`, and the family is revoked so a replay with the same cookie is also 401; with the account present, refresh → 200 and the new access token's `sub` is that account's id (`PRQ-05`, `PRQ-06`, `SC-003`).

### Implementation for User Story 2

- [ ] T016 [US2] Implement the account check in `ServiceImpl.Refresh` in `internal/auth/service.go`, immediately after the revoked/expiry rules and before the first `s.tokens.Issue`: `FetchByID(ctx, stored.UserID)`; on `ErrUserNotFound` revoke `stored.FamilyID` (a revocation failure returns `CodeInternal` + `MsgInternal` and still issues nothing) then return `CodeUnauthorized` + `MsgInvalidRefreshToken`; on any other store error return `CodeInternal` + `MsgInternal` without touching the family and without issuing; issue the access token and the rotated refresh credential for `account.ID`, and build the session for `account.ID`; keep CSRF, reuse, expiry and concurrent-rotation outcomes exactly as they are and add no new `apperr.Code` (`PRQ-05`, `PRQ-06`, `PRQ-07`, `PRQ-17`).

**Checkpoint**: US2 is independently complete; US1 is untouched.

---

## Phase 5: User Story 3 — One mailbox is one account, however the address is spelled (Priority: P2)

**Goal**: Creation, sign-in and the per-account rate-limit key share one canonicalization rule; the
store holds the bare lowercase address; a one-shot operator command repairs legacy rows under full
validation and never runs on the API boot path.

**Independent Test**: Register a display-name spelling and assert the stored value is the bare
lowercase address; register another spelling of the same mailbox → 409 with one row; sign in with a
third spelling → same account; run the migration dry-run and apply over a seeded legacy row and
assert only `email` and `updated_at` change; seed a collision/invalid row and assert 0 writes and a
failed exit; assert 254 accepted / 255 refused and invalid forms keep the existing typed 400.

### Tests for User Story 3 (write first; T017 fails before T021, T019 before T022, T018 before T023, and T020/T025 before their implementations)

- [ ] T017 [P] [US3] Add `internal/user/email_test.go` with the full rule table from `contracts/email-canonicalization.contract.md`: `"  Player@Example.COM  "` → `player@example.com`; `"Name <Player@Example.com>"` → `player@example.com`; `+tag`, dots, underscores and quoted local parts preserved verbatim; a 254-byte canonical address accepted and 255 bytes → `ErrInvalidEmail`; empty, whitespace-only, no-`@`, domain-less, `@example.com` and two-address inputs → `ErrInvalidEmail`; display names never survive; the function is pure (same input, same output) (`PRQ-08`, `SC-004`).
- [ ] T018 [P] [US3] Extend `internal/app/ratelimit_test.go` to prove `accountKey` uses `user.CanonicalEmail`: `"Name <Player@Example.com>"`, `"  player@example.com "` and `"PLAYER@EXAMPLE.COM"` all produce the same `account:player@example.com` key; `+tag` and dotted spellings keep distinct keys; an unparseable value falls back to trimmed lowercase so it still occupies one stable bucket (never an empty key) (`PRQ-09`).
- [ ] T019 [US3] Extend `internal/auth/service_test.go` (same file as T003/T013, so not `[P]`) with canonical create/sign-in cases: `Register` persists the bare lowercase address for a display-name/capitalized/whitespace input; an unparseable or 255-byte address → `CodeInvalidRequest` + `MsgInvalidEmail` (400); a duplicate key from the store still → `CodeConflict` + `MsgEmailAlreadyExists`; `Login` canonicalizes before `FetchByEmail` and an equivalent spelling reaches the same account; an unparseable sign-in address → the uniform `CodeUnauthorized` + `MsgInvalidCredentials` with no account-existence leak; `+tag`/dot spellings are passed through unchanged (`PRQ-08`, `PRQ-09`, `PRQ-10`, `PRQ-17`).
- [ ] T020 [P] [US3] Extend `internal/app/integration_test.go` (Mongo, self-skip) with the canonical journey: register `"  Player One <Player@Example.COM>  "` → 201 and the stored `email` is exactly `player@example.com` with no `email_verified_at`; a second registration with a different spelling of the same mailbox → 409 and exactly one row; sign in with `"Another Name <player@example.com>"` → 200 for the same account id; insert a legacy-shaped document without the field, re-run the boot path (`db.EnsureIndexes`), and assert the document still carries no `email_verified_at` (`PRQ-08`, `PRQ-10`, `PRQ-12`, `PRQ-15`, `SC-004`, `SC-005`).

### Implementation for User Story 3 — canonicalization rule and its consumers

- [ ] T021 [US3] Add `internal/user/email.go`: `const MaxEmailLength = 254`, `var ErrInvalidEmail = errors.New("email is invalid")`, and `func CanonicalEmail(raw string) (string, error)` implementing the exact steps — trim, `mail.ParseAddress`, error → `ErrInvalidEmail`, take `parsed.Address`, trim and lowercase, length cap on the canonical value, return it; dots, `+tags`, underscores and other local-part characters are preserved; no other transformation (`PRQ-08`).
- [ ] T022 [US3] Replace `normalizeEmail` in `internal/auth/service.go` with `user.CanonicalEmail`: `Register` persists the canonical value and maps `ErrInvalidEmail` to `CodeInvalidRequest` + `MsgInvalidEmail`; `Login` canonicalizes before `FetchByEmail` and maps any canonicalization failure to the existing uniform `invalidCredentials` outcome; remove the now-unused `net/mail`/`normalizeEmail` code and keep the duplicate-conflict mapping (`CodeConflict` + `MsgEmailAlreadyExists`) unchanged (`PRQ-08`, `PRQ-09`, `PRQ-10`, `PRQ-17`).
- [ ] T023 [US3] Update `internal/app/ratelimit.go`: `accountKey` calls `user.CanonicalEmail`, falling back to `strings.ToLower(strings.TrimSpace(email))` when it fails, so equivalent spellings share one bucket and an invalid spelling still has a stable non-empty key; update the explanatory comment; `peekEmail` applies the same rule (`PRQ-09`).

### Implementation for User Story 3 — operator migration

- [ ] T024 [P] [US3] Add the migration seam and its mock: create `internal/user/email_migration.go` with `EmailIdentity{ID, Email}`, `EmailChange{ID, From, To}`, `EmailCollision{Canonical, IDs}`, `InvalidEmailRow{ID, Raw, Reason}`, `EmailMigrationPlan{Changes, Collisions, Invalid}` with `Blocked()`, `var ErrMigrationBlocked`, the `EmailMigrationStore` interface (`ListEmailIdentities(ctx)` and `UpdateEmail(ctx, id, email string, updatedAt time.Time)`), `PlanEmailCanonicalization(rows []EmailIdentity) EmailMigrationPlan`, and `(p EmailMigrationPlan) Apply(ctx, store, now, dryRun) (int, error)`; add `EmailMigrationStore` to `.mockery.yaml` under `legnaris/internal/user` and run `make mocks` to generate `internal/user/mocks/mock_EmailMigrationStore.go`; no behavior is wired into `cmd/api.go` (`PRQ-11`).
- [ ] T025 [US3] Add `internal/user/email_migration_test.go` (before completing the planner/apply bodies) with the plan table: all-canonical → 0 changes; display-name/case/whitespace rows → one `EmailChange` each with the same `CanonicalEmail` output; two rows mapping to one canonical → one `EmailCollision` listing both ids; an unparseable or >254-byte row → one `InvalidEmailRow` with an English reason; `Blocked()` true when either list is non-empty; `Apply` on a blocked plan returns `ErrMigrationBlocked` and calls `UpdateEmail` zero times; dry-run calls `UpdateEmail` zero times and returns 0; apply writes one call per change with `email` + the supplied `now`; a second run over the repaired rows plans 0 changes; a store failure surfaces and stops; ids are never changed and no merge is attempted; then implement `PlanEmailCanonicalization` and `Apply` in `internal/user/email_migration.go` until the table passes (`PRQ-11`, `SC-008`).
- [ ] T026 [P] [US3] Implement `ListEmailIdentities(ctx)` and `UpdateEmail(ctx, id, email, updatedAt)` on `RepositoryImpl` in `internal/user/repository.go` so it satisfies `EmailMigrationStore`: list projects only `_id` and `email`; update matches `_id` and sets only `email` and `updated_at` (never `_id` or any other field) and never creates a document; expose the seam through `NewEmailMigrationStore(db *mongo.Database, log logger.StructuredLogger) EmailMigrationStore` so the operator command and its tests obtain it without widening `Repository`; extend `internal/user/repository_test.go` (Mongo, self-skip) to prove the projection, the two-field update, identifier preservation, no document creation, and that listing returns every row (`PRQ-11`).
- [ ] T027 [P] [US3] Add `cmd/canonicalize-emails/main.go` with a testable `run(ctx, args, stdout, store) int` seam and `cmd/canonicalize-emails/main_test.go` (fake `EmailMigrationStore`): `-apply` defaults to false (dry run); exit 0 on clean dry-run/apply/already-canonical, 1 on blocked plan (collision or invalid row, 0 writes), store failure, 1 on a mid-run failure, 2 on an unknown flag/bad arguments; the stdout report is English and exactly the documented lines (`scanned`, one `change`, grouped `collision`, `invalid`, `written` or `dry run`); `main()` loads the shared env/Mongo, builds the store with `user.NewEmailMigrationStore`, and calls `run`, and nothing in `cmd/api.go` imports this package (`PRQ-11`, `PRQ-20`, `SC-008`).
- [ ] T028 [US3] Package the operator command: `Dockerfile` builds `/canonicalize-emails` from `./cmd/canonicalize-emails` into the runtime image next to `/app`, keeping the entry point unchanged; `Makefile` gains `migrate-emails` (dry run by default, `ARGS="-apply"` to write) and includes it in `.PHONY`, and `APP_ENV_KEYS` already exports the database variables the command needs; the API boot path still cannot reach the command (`PRQ-11`, `PRQ-18`).

> T024 establishes the seam/mock, T025 completes the validated plan/apply behavior, T026 adds the
> repository implementation, T027 adds the CLI, T028 packages it.

**Checkpoint**: US3 is independently complete; the migration is operator-only and the HTTP surface is unchanged.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Guard the unchanged surface, document operation, and run the full gates.

- [ ] T029 [P] Guard the unchanged HTTP surface (`PRQ-14`): extend `internal/app/router_wiring_test.go` to capture `app.NewRouter(...).Router().Routes()` and assert the auth route set is exactly `POST /auth/register`, `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout` — no `/auth/me`, `/auth/verify`, `/auth/verify/confirm` or any other new route — and that registering those routes does not change any response shape (the DTO assertion lives in `internal/user/user_test.go`) (`PRQ-12`, `PRQ-13`, `PRQ-14`).
- [ ] T030 [P] Document the operator surface in `deploy/README.md`: under §6 document `TRUSTED_PROXY_CIDRS` as optional (unset = loopback + private + link-local; a comma-separated CIDR set replaces the non-loopback portion; an invalid value fails boot), and under §12 add the one-shot migration commands (`docker compose run --rm --entrypoint /canonicalize-emails app` for dry run and `… -apply` to write, with the collision/invalid abort semantics) (`PRQ-03`, `PRQ-11`).
- [ ] T031 Run the complete repository gate set and resolve every finding without weakening an assertion: `make fmt && make lint && make vet && make lint-specs` then `make test-race`; confirm every `PRQ-*` citation resolves via `python3 scripts/check_requirement_ids.py`, all strings/comments are English (`PRQ-20`), no new dependency appears in `go.mod`/`go.sum`, and the race detector is clean (`PRQ-19`, `SC-006`).
- [ ] T032 Execute the [quickstart.md](./quickstart.md) validation against a local MongoDB (`make dev-db`): §2 unit/boundary suites, §3 two clients behind one trusted peer, §4 canonical registration/conflict/sign-in, §5 migration dry-run/apply/idempotency/collision/apply-fail, §6 refresh with a deleted account, §7 required-env unchanged and both binaries present; fix any deviation in the touched files and report the observed status codes, exit codes and stored documents (`SC-001`…`SC-008`).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001)**: no dependencies; blocks nothing technically but must be run first to establish a clean baseline.
- **Foundational (T002–T005)**: T002/T003 form the clock track; T004/T005 form the record track. Both tracks may run in parallel. US1 does not consume them; US2 consumes the clock; US3 consumes the clock and the record stays independent.
- **US1 (T006–T012)**: depends only on T001. Fully independent of US2 and US3.
- **US2 (T013–T016)**: depends on T001 and T003 (frozen/stepped clock for the boundary assertions). Independent of US1 and US3.
- **US3 (T017–T028)**: depends on T001, T003 (register timestamps) and T021 (the canonicalization rule) for its consumers. Independent of US1 and US2.
- **Polish (T029–T032)**: depends on every story that is in scope; T029 needs T011; T031/T032 need all implemented tasks.

### Within Each Story

- Tests (and the seam they compile against, T024) come before the behavior they cover; a test task must be seen failing before its implementation task completes.
- Rule/value object → service consumer → HTTP key/boundary → integration.
- Every task ships its tests in the same commit; no assertion is weakened to make a gate pass.

### User Story Dependency Graph

```text
T001 ──┬── T002 → T003 ──┬── T013 → T016 (US2)
       │                 └── T019/T022 (US3 clock assertions)
       ├── T004 → T005
       ├── T006/T007/T008 → T009/T010 → T011 → T012 (US1)
       └── T017 → T021 ──┬── T022 (US3)
                         ├── T023 (US3)
                         └── T024 → T025 → T026/T027 → T028 (US3)
T011 → T029
all  → T030 / T031 / T032
```

---

## Parallel Opportunities

### Foundational

```text
Track A: T002 → T003
Track B: T004 → T005
```

### User Story 1

```text
T006  clientip rule table        (internal/platform/clientip/clientip_test.go)
T007  environment table          (shared/env/env_test.go)
T008  route-level wiring proof   (internal/app/client_ip_test.go)
        ↓ after the tests are written
T009 (clientip.go) ─┐
T010 (env.go)     ──┴→ T011 (bootstrap.go + router.go)
T012 deployment surface (compose, .env.example, compose_test.go, Makefile) — parallel with T010/T011
```

### User Story 2

```text
T013 service order table   (internal/auth/service_test.go)
T014 HTTP boundary table   (internal/app/auth_test.go)
T015 Mongo integration     (internal/app/auth_integration_test.go)
        ↓
T016 service implementation
```

### User Story 3

```text
T017 email rule table             (internal/user/email_test.go)
T018 rate-limit key table         (internal/app/ratelimit_test.go)
T020 canonical journey            (internal/app/integration_test.go, Mongo)
T024 migration seam + mock        (internal/user/email_migration.go, .mockery.yaml)
        ↓
T021 → T022 → T023
T024 → T025 → T026 / T027 → T028
```

---

## Implementation Strategy

### MVP First (User Story 1 only)

1. Complete T001 and Phase 3 (T006–T012).
2. **STOP and VALIDATE**: run `go test ./internal/platform/clientip ./internal/app ./shared/env ./deploy -run 'ClientIP|RateLimit|Env|Compose'` and the quickstart §3 scenario.
3. Deploy/demo: US1 alone removes the shared-bucket lockout, which is the program plan's first security fix.

US2 is also P1. The complete P1 scope is US1 + US2; deliver US2 immediately after US1 because it
creates the account lookup point the later verification gate depends on.

### Incremental Delivery

1. T001 → clean baseline.
2. Phase 2 → clock + verification-ready record (no user-visible change).
3. US1 → real client address (MVP).
4. US2 → refresh resolves the account (completes P1).
5. US3 → canonical email + operator migration (P2).
6. Polish → surface guard, operator docs, full gates, quickstart validation.

### Parallel Team Strategy

- One developer takes the clock track (T002–T003), one takes the record track (T004–T005) while
  another starts US1 (T006–T012).
- After the foundational tracks land, US1, US2 and US3 can proceed in parallel because they touch
  disjoint seams (`internal/platform/clientip` + `internal/app` config; `internal/auth` refresh;
  `internal/user` email + `cmd/canonicalize-emails`).
- Within US3, one developer owns the canonicalization rule and its two consumers (T021–T023),
  another owns the migration plan/apply/repository/CLI (T024–T028).

---

## Requirement Traceability

| Requirement | Task(s) | Independent proof |
|---|---|---|
| `PRQ-01` | T006, T009, T011 | `clientip_test.go` resolution matrix; route-level fallback |
| `PRQ-02` | T006, T008, T009 | forged-chain case returns the untrusted peer |
| `PRQ-03` | T006, T007, T009, T010, T011, T012 | policy table, env table, boot failure on invalid CIDR, non-empty result |
| `PRQ-04` | T008, T009, T011 | two clients limited independently through the real router |
| `PRQ-05` | T013, T015, T016 | missing account → 401, family revoked, no issuance |
| `PRQ-06` | T013, T016 | access token names the loaded account; same family rotation |
| `PRQ-07` | T013, T014, T016 | store failure → 500, no credential, family untouched |
| `PRQ-08` | T017, T019, T020, T021, T022 | rule table, stored value, 400 on invalid/over-long |
| `PRQ-09` | T017, T018, T019, T022, T023 | equivalent spellings resolve one account/key; uniform 401 |
| `PRQ-10` | T019, T020, T022 | second spelling → 409, one row |
| `PRQ-11` | T024, T025, T026, T027, T028 | plan/apply table, CLI exit codes, operator-only binary |
| `PRQ-12` | T004, T005 | field absent by default and on every created account |
| `PRQ-13` | T005 | conditional write, first value wins, unknown → not found |
| `PRQ-14` | T004, T029 | DTO/response shape unchanged; auth route inventory unchanged |
| `PRQ-15` | T004, T005, T020 | boot path leaves a seeded legacy document untouched; new accounts start absent |
| `PRQ-16` | T002, T003 | frozen/stepped clock assertions in token and service suites |
| `PRQ-17` | T013, T014, T019, T022 | typed code + mapped status on every new failure path |
| `PRQ-18` | T001, T007, T010, T012 | no new dependency, optional config, required list unchanged |
| `PRQ-19` | T031 (and every task's tests) | `make test-race` clean; edge cases before happy path |
| `PRQ-20` | T031 (and every source task) | English-only identifiers, comments, logs, CLI/report strings |

**Coverage**: 20 requirements, 20 mapped to at least one task, 0 unmapped.

### Success Criteria Map

| Criterion | Proven by |
|---|---|
| `SC-001` | T008, T011, T032 §3 |
| `SC-002` | T006, T008, T032 §3 |
| `SC-003` | T013, T015, T016, T032 §6 |
| `SC-004` | T017, T020, T022, T032 §4 |
| `SC-005` | T004, T005, T020, T032 §4 |
| `SC-006` | T031, every boundary assertion |
| `SC-007` | T002, T003 — no `time.Sleep` in the clock suites |
| `SC-008` | T025, T027, T032 §5 |

---

## Notes

- `[P]` tasks touch different files and have no dependency on an incomplete task.
- Tests are not optional in this repository: each implementation task lands with the tests that
  prove it, in the same commit, edge cases first.
- No new third-party dependency and no new required environment variable are introduced anywhere
  in this task list (`PRQ-18`).
- The API boot path never calls `cmd/canonicalize-emails`; the migration is a separate binary
  (`PRQ-11`).
- Commits are local. Do not push, and never print, log or commit a token.
- Every task that touches `internal/auth`, `internal/user`, `internal/app`, `shared/env`,
  `internal/platform/clientip` or `cmd/canonicalize-emails` must keep `PRQ` citations in comments
  and test names where the requirement is not self-evident.
