# Fase 2 — Ownership Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/lgn-7-ownership/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: `AGENTS.md` (edge cases first + happy path, typed errors, never commit without tests), `shared/apperr/AGENTS.md` (error codes need a status-mapping test), `shared/db/AGENTS.md` (repository declares typed errors, no `apperr` import), `internal/AGENTS.md` (services translate at the boundary).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Service (domain) | unit | All branches; 1:1 to OWN-03…OWN-14, OWN-19, OWN-21, OWN-23, OWN-24; failure paths assert the `apperr` code | `internal/sheet/service_test.go` | `go test ./internal/sheet/...` |
| Repository (data access) | integration (self-skip without Mongo) | Owner lookup hit/miss, delete hit/miss, duplicate-key → typed error | `internal/sheet/repository_test.go` | `go test ./internal/sheet/...` |
| Controller / HTTP (e2e) | integration (self-skip without Mongo) | Every route: happy + 401 + 404 + 409; new `GET /sheets` and `DELETE /sheets/:id` | `internal/app/*_test.go` | `go test ./internal/app/...` |
| Index bootstrap | integration (self-skip without Mongo) | `sheets.owner_id` unique index exists after `EnsureIndexes` | `shared/db/indexes_test.go` | `go test ./shared/db/...` |
| Model / DTO / error codes | unit | `owner_id` persists; DTO omits it; `SHEET_LIMIT_REACHED` maps to 409 | `internal/sheet/dto_test.go`, `shared/apperr/err_test.go` | `go test ./internal/sheet/... ./shared/apperr/...` |

**Note on `Tests: none`:** no task here is `none`; every task creates or modifies a tested layer.

## Gate Check Commands

> Generated from codebase - confirm before Execute.

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | After tasks with unit tests only | `go test ./internal/sheet/... ./shared/apperr/...` |
| Full | After tasks that add/update integration tests | `go test ./...` |
| Build | At phase completion / final validation | `make fmt && make lint && make vet && make test-race` |

---

## Execution Plan

Phases are ordered and run sequentially - each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Persistence primitives

Independent leaves: the model, the error code, and the index.

```
T1
T2
T3
```

### Phase 2: Repository contract

```
T1 -> T4
```

### Phase 3: Service ownership

```
T4 -> T5 -> T6 -> T7
```

### Phase 4: Owner-scoped HTTP surface behind JWT

```
T5 -> T8
T6 -> T8
T7 -> T8
```

### Phase 5: Concurrency hardening

```
T8 -> T9
```

---

## Task Breakdown

### T1: Add `owner_id` to the sheet model

**What**: Persist an `owner_id` string on `Sheet` and prove the DTO never exposes it.
**Where**: `internal/sheet/sheet.go`
**Depends on**: None
**Reuses**: existing `Sheet` struct and `ToDTO`
**Requirement**: OWN-01

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `Sheet` has `OwnerID string` with bson tag `owner_id`
- [x] `ToDTO` output contains no owner field (assert on the serialized shape)
- [x] Gate check passes: `go test ./internal/sheet/...`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(LGN-19): add owner_id to the sheet model`

---

### T2: Add the `SHEET_LIMIT_REACHED` error code

**What**: Register `apperr.CodeSheetLimitReached` mapped to HTTP 409.
**Where**: `shared/apperr/err.go`
**Depends on**: None
**Reuses**: `apperr.NewError` status switch and the table-driven `err_test.go`
**Requirement**: OWN-22

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `CodeSheetLimitReached = "SHEET_LIMIT_REACHED"` exists and `NewError(CodeSheetLimitReached, …).HttpStatus() == 409`
- [x] The mapping test covers the new code alongside the existing ones
- [x] Gate check passes: `go test ./shared/apperr/...`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(LGN-19): add SHEET_LIMIT_REACHED error code`

---

### T3: Ensure a unique index on `sheets.owner_id`

**What**: Create the `sheets.owner_id` unique index during the boot-time index step.
**Where**: `shared/db/indexes.go`
**Depends on**: None
**Reuses**: existing `EnsureIndexes` structure and the auth index naming convention
**Requirement**: OWN-02

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `EnsureIndexes` creates `sheets.owner_id` with `SetUnique(true)` and an explicit name
- [x] Integration test asserts the index exists (and is unique) after `EnsureIndexes`; self-skips without Mongo
- [x] Gate check passes: `go test ./shared/db/...`

**Tests**: integration
**Gate**: full

**Commit**: `feat(LGN-19): ensure unique index on sheets.owner_id`

---

### T4: Owner lookup, delete, and duplicate-key mapping in the repository

**What**: Add `FetchByOwnerID` and `DeleteByID`, and map Mongo duplicate-key on `owner_id` to `ErrSheetOwnerExists`.
**Where**: `internal/sheet/repository.go`
**Depends on**: T1
**Reuses**: `ErrSheetNotFound`, the `mongo.ErrNoDocuments` handling, the `logger` calls
**Requirement**: OWN-03, OWN-22, OWN-24

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `FetchByOwnerID` returns `ErrSheetNotFound` on a miss; `DeleteByID` returns it when zero documents matched
- [x] `Create` maps a duplicate key on `owner_id` to `ErrSheetOwnerExists` (never a wrapped raw error)
- [x] Mocks regenerated with `make mocks`; integration repository test covers hit/miss/duplicate; self-skips without Mongo
- [x] Gate check passes: `go test ./internal/sheet/...`

**Tests**: integration
**Gate**: full

**Commit**: `feat(LGN-19): add owner lookup, delete and duplicate-key mapping to repository`

---

### T5: Owner-scoped sheet creation with the one-sheet rule

**What**: Bind `owner_id` on create, pre-check the owner, and map the duplicate race to `409 SHEET_LIMIT_REACHED`.
**Where**: `internal/sheet/service.go`
**Depends on**: T4
**Reuses**: `translateRepoErr`, the `CreateSheetCommand` shape, the existing DAG seeding path
**Requirement**: OWN-03, OWN-04, OWN-05, OWN-06

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `CreateSheetCommand` carries `OwnerID`; the created `Sheet` stores it
- [x] An existing owner yields `CodeSheetLimitReached`; a duplicate-key race yields the same code (no `500`)
- [x] An empty owner id yields `CodeUnauthorized`
- [x] Service unit tests cover happy, owner-already-has-sheet, duplicate-race, and missing-owner; mocks regenerated
- [x] Gate check passes: `go test ./internal/sheet/...`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(LGN-20): bind owner and enforce one sheet per user on create`

---

### T6: Owner-guarded fetch and recalculation

**What**: Add `FetchMySheet`, make `FetchByID` owner-checked, and gate `PartialUpdate`/`Preview` through the same ownership check.
**Where**: `internal/sheet/service.go`
**Depends on**: T5
**Reuses**: `recalculate`, `requireSheetID`, `translateRepoErr`
**Requirement**: OWN-07, OWN-08, OWN-09, OWN-10, OWN-11, OWN-12, OWN-14, OWN-19, OWN-21, OWN-24

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `FetchMySheet` returns the owner's sheet or `404 NOT_FOUND`
- [x] A foreign or legacy (no owner) sheet returns `404 NOT_FOUND` from `FetchByID`/`PartialUpdate`/`Preview`, and no recalculation or write runs
- [x] Owner's own `PartialUpdate`/`Preview` behave exactly as before
- [x] Service unit tests cover owner hit, foreign sheet, missing sheet, and missing owner id; mocks regenerated
- [x] Gate check passes: `go test ./internal/sheet/...`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(LGN-22): guard fetch, update and preview by ownership`

---

### T7: Owner-guarded delete

**What**: Add `Delete` to the service: verify ownership, then remove the document; `404` when absent.
**Where**: `internal/sheet/service.go`
**Depends on**: T6
**Reuses**: the ownership check from T6, `requireSheetID`, `translateRepoErr`
**Requirement**: OWN-13, OWN-23

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `Delete(DeleteSheetCommand{SheetID, OwnerID})` removes the owner's sheet; a second call returns `404`
- [x] A foreign or missing sheet returns `404` and deletes nothing
- [x] Service unit tests cover happy delete, second delete, foreign sheet, malformed id, missing owner; mocks regenerated
- [x] Gate check passes: `go test ./internal/sheet/...`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(LGN-22): add owner-guarded sheet delete to the service`

---

### T8: Owner-scoped sheet HTTP surface behind the JWT middleware

**What**: Thread the authenticated user id through the sheet controller, add `GET /sheets` and `DELETE /sheets/:id`, and gate `/sheets*` + `/templates*` with `auth.Middleware`; update the integration harness to authenticate.
**Where**: `internal/app/sheet.go`
**Depends on**: T5, T6, T7
**Reuses**: `auth.Middleware`, `auth.UserID`, `AuthModule` (expose `TokenManager`), `response`, the integration harness
**Requirement**: OWN-05, OWN-06, OWN-07, OWN-08, OWN-10, OWN-11, OWN-12, OWN-13, OWN-14, OWN-15, OWN-16, OWN-17, OWN-18, OWN-20

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Controller reads `auth.UserID(c)` and passes it into every sheet command; a missing id yields `401`
- [x] `GET /sheets` and `DELETE /sheets/:id` are registered; `AuthModule` exposes `TokenManager`; `NewRouter` applies the middleware to the `/sheets` and `/templates` groups only
- [x] `internal/app/bootstrap.go`, `internal/app/router.go`, `internal/app/sheet.go`, and `internal/app/integration_test.go` are updated in this commit (auth_integration_test unchanged: `/auth` stays public); the whole `go test ./...` suite is green (no unauthenticated sheet/template call remains)
- [x] Integration tests cover: unauthenticated `401` on every protected route; public routes still reachable; authenticated create/fetch/patch; second create `409`; `GET /sheets` self vs foreign; foreign `GET/PATCH/DELETE/preview` -> `404`; owner `DELETE` -> `204` then `404`
- [x] Gate check passes: `make fmt && make lint && make vet && make test-race`

**Tests**: integration
**Gate**: full

**Commit**: `feat(LGN-21): gate sheet and template routes with ownership and JWT`

---

### T9: Concurrency race test for the one-sheet rule

**What**: Prove the unique index turns two concurrent creates for one owner into one `201` and one `409`.
**Where**: `internal/app/ownership_race_test.go`
**Depends on**: T8
**Reuses**: `newIntegrationRouter`, the auth helper from T8
**Requirement**: OWN-04

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Two goroutines POST `/sheets` for the same authenticated user; exactly one `201` and one `409 SHEET_LIMIT_REACHED`
- [ ] Exactly one document exists for that owner afterwards
- [ ] The test self-skips without Mongo and is race-clean under `-race`
- [ ] Gate check passes: `make test-race`

**Tests**: integration
**Gate**: full

**Commit**: `test(LGN-19): assert concurrent create race yields a single sheet`

---

## Phase Execution Map

Visual representation of task ordering. Phases run in sequence, and tasks within a phase run in order:

```
Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5

Phase 1:  T1   T2   T3
Phase 2:  T1 -> T4
Phase 3:  T4 -> T5 -> T6 -> T7
Phase 4:  T5 -> T8
          T6 -> T8
          T7 -> T8
Phase 5:  T8 -> T9
```

Execution is strictly sequential - there is no intra-phase parallelism.

**Batching:** 9 tasks → two batches (>8), so the Execute offer applies: Batch A = Phases 1–3 (T1–T7), Batch B = Phases 4–5 (T8–T9). Batches run sequentially.

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: model field | 1 struct field + DTO test | ✅ Granular |
| T2: error code | 1 constant + mapping test | ✅ Granular |
| T3: index | 1 index + test | ✅ Granular |
| T4: repository methods | 3 cohesive repository methods | ⚠️ OK (one file, one contract) |
| T5–T7: service changes | 1 method / one behavior each | ✅ Granular |
| T8: HTTP vertical slice | Controller + 2 routes + middleware wiring + tests | ⚠️ Cohesive merge (behavior-tested only as a slice) |
| T9: race test | 1 test file | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (body) | Diagram Shows | Status |
| ---- | ----------------- | ------------- | ------ |
| T1 | None | (leaf) | ✅ Match |
| T2 | None | (leaf) | ✅ Match |
| T3 | None | (leaf) | ✅ Match |
| T4 | T1 | T1 -> T4 | ✅ Match |
| T5 | T4 | T4 -> T5 | ✅ Match |
| T6 | T5 | T5 -> T6 | ✅ Match |
| T7 | T6 | T6 -> T7 | ✅ Match |
| T8 | T5, T6, T7 | T5/T6/T7 -> T8 | ✅ Match |
| T9 | T8 | T8 -> T9 | ✅ Match |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1 | Model / DTO | unit | unit | ✅ OK |
| T2 | Error code | unit | unit | ✅ OK |
| T3 | Index bootstrap | integration | integration | ✅ OK |
| T4 | Repository | integration | integration | ✅ OK |
| T5 | Service | unit | unit | ✅ OK |
| T6 | Service | unit | unit | ✅ OK |
| T7 | Service | unit | unit | ✅ OK |
| T8 | Controller / HTTP | integration | integration | ✅ OK |
| T9 | Controller / HTTP | integration | integration | ✅ OK |
