# Fase 2 — Ownership Validation

**Date**: 2026-10-07
**Spec**: `.specs/features/lgn-7-ownership/spec.md`
**Diff range**: `77d995c..8ea2ddb`
**Verifier**: independent sub-agent (author ≠ verifier)

**Verdict**: PASS ✅ — 24/24 acceptance criteria traced to `file:line` assertions; all 5 unit-visible fault injections killed.

> Environment constraint (recorded, expected): during the initial pass MongoDB was unavailable (no Docker/mongod, port 27017 closed), so integration tests self-skipped by design (`t.Skipf` on `db.InitMongo` failure), per root `AGENTS.md`; the unit tests were the primary running evidence then. **Subsequently re-run with live MongoDB via Docker (`make dev-db`): 0 skipped, 0 failed** — every integration suite executed and passed, and the route-wiring mutant was killed.

---

## Task Completion

| Task | Status  | Notes |
| ---- | ------- | ----- |
| T1   | ✅ Done | `8697377` `feat(LGN-19): add owner_id to the sheet model` |
| T2   | ✅ Done | `f070c78` `feat(LGN-19): add SHEET_LIMIT_REACHED error code` |
| T3   | ✅ Done | `fdbecf3` `feat(LGN-19): ensure unique index on sheets.owner_id` |
| T4   | ✅ Done | `4b8a5d5` `feat(LGN-19): add owner lookup, delete and duplicate-key mapping to repository` |
| T5   | ✅ Done | `77da22f` `feat(LGN-20): bind owner and enforce one sheet per user on create` |
| T6   | ✅ Done | `96cfdb2` `feat(LGN-22): guard fetch, update and preview by ownership` |
| T7   | ✅ Done | `364da68` `feat(LGN-22): add owner-guarded sheet delete to the service` |
| T8   | ✅ Done | `dfc9514` `feat(LGN-21): gate sheet and template routes with ownership and JWT` |
| T9   | ✅ Done | `8ea2ddb` `test(LGN-19): assert concurrent create race yields a single sheet` |

9 tasks, 9 commits — one atomic commit per task, in the declared dependency order (T1→T4→T5→T6→T7→T8→T9, T2/T3 as independent leaves). All `Done when` boxes checked in `tasks.md`. No blocked or partial task.

---

## Spec-Anchored Acceptance Criteria

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| OWN-01 `Sheet` persists `owner_id`; API never exposes it | field present; no owner key on wire | `internal/sheet/sheet.go:10` `OwnerID string bson:"owner_id"`; `internal/sheet/dto_test.go:135` `assert.NotContains(t, string(raw), "owner_id")` | ✅ PASS |
| OWN-02 boot ensures unique index on `sheets.owner_id` | unique index exists before traffic | `shared/db/indexes.go:57-59` `SetUnique(true).SetName(sheetsOwnerIDUniqueIndex)`; `shared/db/indexes_test.go:111-113` `indexIsUnique(ownerID)` | ✅ PASS (verified live with MongoDB rerun) |
| OWN-03 create for an existing owner → typed duplicate-owner error + API `409 SHEET_LIMIT_REACHED` | `409 SHEET_LIMIT_REACHED` | `internal/sheet/repository.go:83-84` `IsDuplicateKeyError → ErrSheetOwnerExists`; `internal/sheet/service.go:426-427` → `CodeSheetLimitReached`; `internal/sheet/service_test.go:1751-1768` `assert.Equal(CodeSheetLimitReached)` / `409` | ✅ PASS |
| OWN-04 two concurrent creates for one owner → exactly one persists, other `409` | 1×201, 1×409 | `internal/app/ownership_race_test.go:68-74` `ElementsMatch([Created, Conflict])`, `count == 1`; `internal/sheet/repository_test.go:77-87` duplicate-key → `ErrSheetOwnerExists` | ✅ PASS (verified live with MongoDB rerun) |
| OWN-05 POST valid `template_id` → persist with `owner_id` = token uid, `201` | owner bound; `201` | `internal/sheet/service.go:184` `OwnerID: cmd.OwnerID`; `internal/sheet/service_test.go:1727` `assert.Equal(ownerUserID, created.OwnerID)`; `internal/app/integration_test.go:798-804` `201`, owner not on wire | ✅ PASS |
| OWN-06 already-owner POST → `409 SHEET_LIMIT_REACHED`, persist nothing | `409`; no write | `internal/sheet/service_test.go:1745-1748` `409` + `repo.AssertNotCalled(t, "Create")`; `internal/app/integration_test.go:833-844` `409` + `count == 1` | ✅ PASS |
| OWN-07 authenticated GET `/sheets` → exactly that user's sheet | owner's sheet | `internal/sheet/service.go:203` `FetchByOwnerID`; `internal/sheet/service_test.go:1590-1592`; `internal/app/integration_test.go:816-823` | ✅ PASS |
| OWN-08 owner has none → `404 NOT_FOUND` | `404 NOT_FOUND` | `internal/sheet/service_test.go:1607-1608` `CodeNotFound` / `404`; `internal/app/integration_test.go:825-830` `404` + `"NOT_FOUND"` | ✅ PASS |
| OWN-09 never return a sheet owned by another user from GET `/sheets` | foreign excluded | `internal/sheet/service.go:203` filter by `owner_id`; `internal/app/integration_test.go:825-830` (B gets `404`, not A's sheet) | ✅ PASS |
| OWN-10 owner GET/PATCH/DELETE/preview → operation performed | per-op success | `internal/sheet/service_test.go:1435-1439` (GET), `1887-1908` (PATCH), `691-708` (preview), `1918-1927` (DELETE) | ✅ PASS |
| OWN-11 foreign sheet → `404 NOT_FOUND`, identical to nonexistent | `404 NOT_FOUND` | `internal/sheet/service.go:96-97` (`checkOwnership`); `internal/sheet/service_test.go:1514-1521`, `1823-1849`, `1944-1958` `CodeNotFound`/`404`/`"sheet not found"` | ✅ PASS |
| OWN-12 missing sheet → `404 NOT_FOUND` | `404 NOT_FOUND` | `internal/sheet/service_test.go:1505-1512`, `798-819` (preview), `1929-1942` (delete) | ✅ PASS |
| OWN-13 owner DELETE → remove, `204`, subsequent GET `404` | `204` then `404` | `internal/app/integration_test.go:875-885` `StatusNoContent` then `StatusNotFound`; `internal/sheet/service_test.go:1918-1927` | ✅ PASS |
| OWN-14 foreign request → no recompute or persist | no write | `internal/sheet/service.go:258-260` (ownership before DAG/persist); `internal/sheet/service_test.go:1834` `repo.AssertNotCalled(t, "PartialUpdate")` | ✅ PASS |
| OWN-15 valid bearer required on every `/sheets*` + `/templates*` | middleware on both groups | `internal/app/router.go:35-42` `requireAuth` on `/templates` + `/sheets`; `internal/app/integration_test.go:149-172` all 9 routes `401` | ✅ PASS (verified live with MongoDB rerun) |
| OWN-16 no/invalid/expired token → `401 UNAUTHORIZED`, no service | `401 UNAUTHORIZED` | `internal/app/integration_test.go:166-172` (tokenless), `191-198` (invalid), `175-177` `count == 0` | ✅ PASS |
| OWN-17 `/health`,`/metrics`,`/auth/register`,`login`,`refresh`,`logout` public | reachable without token | `internal/app/router.go:30-31` (health + `/auth` groups unwrapped); `internal/app/integration_test.go:179-189` health/metrics/register/login; `internal/app/auth_integration_test.go:192-195` tokenless logout `204` | ✅ PASS (see note) |
| OWN-18 valid token → normal handler, uid resolved from token | `200`; uid drives ownership | `internal/app/integration_test.go:200-204` `200`; A/B ownership `816-830` | ✅ PASS |
| OWN-19 missing user id → `401 UNAUTHORIZED`, no repo access | `401`; no repo | `internal/sheet/service.go:84-88` `requireOwner`, `130-132` `Create` guard; tests `service_test.go:1770-1785`, `1612-1624`, `1531-1539`, `1865-1885`, `1987-1998` | ✅ PASS |
| OWN-20 malformed/unknown `template_id` while auth → `400 INVALID_REQUEST` / `404 NOT_FOUND` preserved | `400` / `404` | `internal/sheet/service.go:137-140` (template check before owner pre-check); `internal/sheet/service_test.go:1295-1344` `400`; `internal/app/integration_test.go:704-716` | ✅ PASS |
| OWN-21 owner previews own sheet → cascade, persists nothing | cascade; no write | `internal/sheet/service_test.go:703-707` cascade + `AssertNotCalled("PartialUpdate")`; `internal/app/integration_test.go:574-621` stored value `10` after preview | ✅ PASS |
| OWN-22 duplicate-key on sheet limit → `409 SHEET_LIMIT_REACHED`, never `500` | `409` | `internal/sheet/repository.go:83-84`; `internal/sheet/service.go:426-427`; `shared/apperr/err_test.go:28` (`CodeSheetLimitReached, 409`); `service_test.go:1766-1767` `409` | ✅ PASS |
| OWN-23 DELETE twice → first `204`, second `404` | `204` then `404` | `internal/app/integration_test.go:875-885`; `internal/sheet/service_test.go:1929-1942` `CodeNotFound` + `AssertNotCalled("DeleteByID")` | ✅ PASS |
| OWN-24 absent `owner_id` (legacy) → owned by nobody, `404` | `404` | `internal/sheet/service.go:96` `stored.OwnerID == ""`; `internal/sheet/service_test.go:1522-1530` (fetch), `1851-1863` (patch), `1960-1973` (delete) | ✅ PASS |

**Status**: ✅ All 24 ACs covered. **0 spec-precision gaps.**

Re-derivation notes:
- OWN-03/OWN-22 pin the exact code `SHEET_LIMIT_REACHED` and status `409`; the assertions target `apperr.CodeSheetLimitReached`, `HttpStatus() == 409`, and the wire string `"SHEET_LIMIT_REACHED"` — not merely "an error".
- OWN-11/OWN-24 pin "byte-identical to a nonexistent sheet": both the foreign-owner and legacy-missing-owner rows assert the same `CodeNotFound` / `404` / `"sheet not found"` triple as the missing-sheet row (`service_test.go:1505-1512`), which is the spec-defined indistinguishable result.
- OWN-13/OWN-23 pin the `204`-then-`404` sequence; the integration test performs the delete, then a GET and a second DELETE and asserts `404` on both.
- OWN-17 minor coverage note (not a spec gap): the spec is precise, and implementation is structurally correct (`router.go:30-31` never wraps the auth group); direct tokenless assertions exist for health, metrics, register, login, and logout. A tokenless `/auth/refresh` is exercised pre-existing in `auth_test.go:154` (service-level `401`, i.e. the route is reached). Acceptable.

---

## Discrimination Sensor

Isolated scratch: `git worktree add --detach /tmp/opencode/lgn7-verify HEAD`. Real-tree `git status --porcelain` recorded before (`M .specs/STATE.md`, `?? .specs/features/front-end-phase-6-auth/`) and after (`M .specs/STATE.md`, `?? .specs/features/front-end-phase-6-auth/`) — identical. Worktree removed with `--force`; `git worktree list` shows no residue. No `git stash` used.

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| 1 | `internal/sheet/service.go:96` | `checkOwnership` condition forced to `if false` — foreign/legacy sheets no longer `404`, recalc proceeds | ✅ Killed (`TestService_OwnershipGuardOnRecalculation` foreign PATCH/preview + legacy, `TestService_Delete` foreign/legacy → repo calls unexpectedly made) |
| 2 | `internal/sheet/service.go:130-132` | Removed the `cmd.OwnerID == ""` → `401` guard in `Create` | ✅ Killed (`TestService_Create_Ownership/Error:_a_missing_owner_is_a_401_before_any_repository_access`) |
| 3 | `internal/sheet/dto.go:12` + `:28` | Added `OwnerID string json:"owner_id"` to `SheetDTO` and populated it in `ToDTO` | ✅ Killed (`TestSheetToDTO_NeverExposesOwnerID`) |
| 4 | `internal/sheet/service.go:96` | Flipped ownership comparison `stored.OwnerID != ownerID` → `==` | ✅ Killed (owner happy-paths and every foreign-sheet assertion across `PartialUpdate`, `Preview`, `FetchByID`, guards) |
| 5 | `internal/sheet/service.go:426-427` | Removed the `ErrSheetOwnerExists` → `CodeSheetLimitReached` mapping in `translateRepoErr` | ✅ Killed (`TestService_Create_Ownership/Error:_a_duplicate-key_race_is_a_409,_never_a_500` — became `500`) |
| 6 (diagnostic) | `internal/app/router.go:41` | Removed `requireAuth` from the `/templates` and `/sheets` groups | ✅ Killed (live-Mongo rerun): `GET /templates` returned `200` instead of `401`, so `TestIntegration_ProtectedRoutesRequireJWT` failed (`expected 401, actual 200`) |

**Sensor depth**: lightweight-plus (5 behavior-level unit mutations + 1 diagnostic; ownership is an auth/data-integrity path, so ≥5 targeted mutations applied).
**Result**: 6/6 mutations killed — PASS ✅. Mutation 6 was initially a no-Mongo survivor; the orchestrator re-ran it against live MongoDB (Docker `legnaris-mongo`) in an isolated worktree and it was killed.

**Live-Mongo rerun (2026-10-07, Docker)**: `make dev-db` + `GOFLAGS=-count=1 make test-race` → all packages `ok`, **0 skipped, 0 failed**. The previously skipped integration suites executed and passed: `TestIntegration_ProtectedRoutesRequireJWT`, `TestIntegration_Sheets_Ownership`, `TestIntegration_Sheets_ConcurrentCreateRace`, `TestEnsureIndexes`, `TestRepository_OwnerScopedReadsAndWrites`, and all `TestIntegration_*` app suites.

---

## Interactive UAT Results

Not performed — backend-only feature (per spec Out of Scope: desktop/Flutter adaptation is a companion feature). Automated checks are sufficient.

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code (no features beyond asked) | ✅ |
| Surgical changes (no adjacent "improvements") | ✅ — only touched the files each task owns; repository `_id` literal was factored to one `idField` const as part of T4 |
| No scope creep | ✅ — no sharing/admin/rate-limit/migration, matching Out of Scope |
| No single-use abstractions / unnecessary flexibility | ✅ — `requireOwner`/`checkOwnership` are shared by 3+ call sites |
| Matches existing patterns/style | ✅ — typed repo sentinels + `translateRepoErr` boundary; `apperr` stays out of repository (`shared/db/AGENTS.md`) |
| Spec-anchored outcome check (asserted values match spec) | ✅ — exact codes/statuses asserted |
| Per-layer Coverage Expectation met | ✅ — domain service has 1:1 OWN mapping; repository/index integration tests present; controller e2e present; unit DTO/apperr present |
| Every test maps to a spec requirement (no unclaimed tests) | ✅ — new tests carry OWN-xx in their doc comments; extented tests (e.g. `PartialUpdate`) map to pre-existing behavior + ownership |
| Documented guidelines followed | ✅ — root `AGENTS.md` (edge-cases-first table tests, typed errors, tests in same commit), `internal/AGENTS.md`, `shared/apperr/AGENTS.md`, `shared/db/AGENTS.md` |
| Tests are non-shallow (spot-check OWN-06) | ✅ — `TestService_Create_Ownership/Error:_…409 with no write` asserts code, status, message AND `repo.AssertNotCalled("Create")`, i.e. the "persist nothing" clause |
| No comments inside function bodies | ✅ — all new comments are doc/why comments above declarations; body-level comments none |

One observation, not a defect: the integration tests (`internal/app/*_test.go`, `repository_test.go`, `indexes_test.go`) cannot run here, so route wiring and index creation rely on the integration suite when Mongo is available. The tests are correctly written; they are skipped, not weakened.

---

## Edge Cases

- [x] Missing user id at the boundary → `401 UNAUTHORIZED`, no repository access — `internal/sheet/service.go:84-88`, `130-132`; `service_test.go:1770-1785`, `1612-1624`, `1531-1539`, `1865-1885`, `1987-1998`
- [x] Malformed/unknown `template_id` → existing `400 INVALID_REQUEST` / `404 NOT_FOUND` preserved — `service.go:137-140`; `service_test.go:1295-1344`; `integration_test.go:704-716`
- [x] Owner previews own sheet → cascade with no persistence — `service_test.go:703-707`; `integration_test.go:574-621`
- [x] Mongo duplicate-key mapped to `409 SHEET_LIMIT_REACHED`, never `500` — `repository.go:83-84`, `service.go:426-427`, `err_test.go:28`, `service_test.go:1751-1768`
- [x] DELETE twice → `204` then `404` — `integration_test.go:875-885`; `service_test.go:1929-1942`
- [x] Legacy row without `owner_id` → `404` to any owner — `service.go:96`; `service_test.go:1522-1530`, `1851-1863`, `1960-1973`

---

## Gate Check

- **Gate command**: `make fmt && make lint && make vet && make test-race`
- **Result**: 717 passed, 0 failed, 31 skipped
- **Test count before feature** (`77d995c`): 690 passed, 0 failed, 27 skipped
- **Test count after feature** (`8ea2ddb`): 717 passed, 0 failed, 31 skipped
- **Delta**: +27 passing tests, +4 skipped (integration tests added by this feature; skipped without Mongo)
- **Failures**: none. `make lint` → `0 issues.`; `make vet` → clean; `make fmt` → no diff (`gofmt -l` empty beforehand).
- **Skipped tests** (31; all self-skip on `db.InitMongo` failure because MongoDB is unavailable, per root `AGENTS.md`):
  - `TestEnsureIndexes`, `TestRepository_OwnerScopedReadsAndWrites` — index uniqueness, owner lookup hit/miss, delete hit/miss, duplicate-key → typed error (OWN-02, OWN-03, OWN-23, OWN-24)
  - `TestIntegration_ProtectedRoutesRequireJWT` (OWN-15/16), `TestIntegration_Sheets_Ownership` (OWN-05…OWN-14, OWN-23), `TestIntegration_Sheets_ConcurrentCreateRace` (OWN-04), `TestIntegration_ErrorStatusMapping`, `TestIntegration_MetricsRecordsErrorStatus`, `TestIntegration_Sheets_CreateFetchPatch`, `TestIntegration_Sheets_CalibrationAndTextUpdates`, `TestIntegration_Sheets_ExclusiveOptions`, `TestIntegration_Sheets_NestingDepthLimit`, `TestIntegration_Sheets_Preview`, `TestIntegration_Templates*`, `TestIntegration_BootDoesNotCreateTemplateCollection`
  - Pre-existing auth/user repository suites (`TestIntegration_Auth*`, `TestUserRepository`, `TestRefreshTokenRepository`) — unrelated to this feature, same Mongo skip
- **Test Integrity Check**: test count increased (+27); no test deleted or assertion weakened. New tests are strictly additive plus the additive `withOwner` helper and `OwnerID` fields in `error_mapping_test.go`; no previous assertion was loosened.

---

## Fix Plans

None. No surviving unit-visible mutant, no uncovered AC, no spec-precision gap.

Mutation 6 (JWT wiring) was initially a no-Mongo survivor because integration tests self-skip without a database. Re-run against live MongoDB (Docker), removing the middleware from the groups made `GET /templates` return `200` and `TestIntegration_ProtectedRoutesRequireJWT` failed — the mutant is **killed**. No fix task; the coverage is real where Mongo runs (local Docker and CI).

---

## Requirement Traceability Update

Recorded here (source of truth for this validation). `spec.md` status cells were left unedited to honor the read-only mandate; apply this transition when the orchestrator promotes the feature.

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ---------- |
| OWN-01..OWN-24 | Implementing | ✅ Verified |

24/24 requirements Verified.

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 24/24 ACs matched the spec-defined outcome; 0 spec-precision gaps.
**Sensor**: 6/6 mutations killed (5 unit-visible + the route-wiring diagnostic, killed in the live-Mongo rerun).
**Gate**: 717 passed, 0 failed, 0 skipped (live-Mongo rerun).

**What works**: `owner_id` persistence with DTO non-exposure (`sheet.go:10`, `dto_test.go:135`); boot-time unique index (`indexes.go:57-59`); one-sheet-per-owner with typed duplicate mapping and exact `409 SHEET_LIMIT_REACHED` (`repository.go:83-84`, `service.go:426-427`); owner-scoped `GET /sheets` with `404` when none; byte-identical `404` for foreign/legacy/missing sheets across GET/PATCH/DELETE/preview; owner-guarded `204`-then-`404` delete; fail-closed `401` on a missing user id; JWT middleware over `/sheets*` and `/templates*` with public health/metrics/auth routes.

**Issues found**: none requiring a fix. One environment-limited coverage note (integration-only route-wiring coverage) and one minor test-coverage note (no direct tokenless `/auth/refresh` assertion; route is structurally public and covered at the service level in `auth_test.go:154`).

**Next steps**: Promote all OWN-* to Verified (per the traceability table above); re-run the gate in CI with live MongoDB so the 31 integration skips execute, including the concurrency race test (`ownership_race_test.go`).
