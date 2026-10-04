# Sheet Preview Dry-Run Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/lgn-103-104-sheet-preview-dry-run/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: `AGENTS.md` (edge cases first, typed `apperr`, mocks via mockery, tests ship with the code), `internal/AGENTS.md` (layering, boundary translation), `desktop/AGENTS.md` (English only, reuse app infrastructure, async feedback), `.github/workflows/` (`go test`/`make lint`, `flutter analyze`/`flutter test`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Sheet service (shared helper, `Preview`, `PreviewTemplate`) | unit | All branches; 1:1 to DRY-01…17; every listed error path; idempotency; zero-write assertion | `internal/sheet/service_test.go` | `go test ./internal/sheet/...` |
| Preview routes + error mapping | integration | Both routes: happy + every error code + envelope shape | `internal/app/integration_test.go`, `internal/app/error_mapping_test.go` | `go test ./internal/app/...` |
| API client (`previewSheet`) | unit | Request verb/path/body, success decode, mapped domain error, no retry | `desktop/test/api/api_client_test.dart` | `flutter test test/api/api_client_test.dart` |
| Draft backup store | unit | Atomic write, read, delete, missing file, corrupt file, version, path | `desktop/test/sheet/draft_backup_store_test.dart` | `flutter test test/sheet/draft_backup_store_test.dart` |
| Draft backup scheduler | unit | Start fires, cancel stops, idempotent start/cancel | `desktop/test/sheet/draft_backup_scheduler_test.dart` | `flutter test test/sheet/draft_backup_scheduler_test.dart` |
| Rate limiter + cooldown | unit | 3 allowed / 4th blocked, window expiry, separate windows, next-free time | `desktop/test/sheet/action_rate_limiter_test.dart` | `flutter test test/sheet/action_rate_limiter_test.dart` |
| Edit controller (preview, revision, limits, restore, delete) | unit | 1:1 to PRV-01…10, SVE-01…07, DRB-05…08,12; edge cases first | `desktop/test/sheet/sheet_edit_provider_test.dart` | `flutter test test/sheet/sheet_edit_provider_test.dart` |
| Draft auto-backup coordinator | unit | Writes on tick while dirty, stops when clean, disposal cancels | `desktop/test/sheet/draft_auto_backup_test.dart` | `flutter test test/sheet/draft_auto_backup_test.dart` |
| Screen 2 footer + discard guards | widget | Preview enable/disable/tooltips, one dry-run zero PATCH, busy guards, discard deletes draft; 1:1 to PRV-11, SVE-01…03, DRB-09 | `desktop/test/sheet/sheet_screen_test.dart` | `flutter test test/sheet/sheet_screen_test.dart` |
| Eviction + library removal cleanup | unit/widget | `NOT_FOUND` evicts and deletes the draft; removal deletes the draft | `desktop/test/sheet/sheet_open_provider_test.dart`, `desktop/test/catalog/library_section_test.dart` | `flutter test test/sheet/sheet_open_provider_test.dart test/catalog/library_section_test.dart` |
| l10n keys | none | Build gate only (resource strings) | - | build gate only |

## Gate Check Commands

> Generated from codebase - confirm before Execute. Backend commands run from the repo root; desktop commands run from `desktop/` (CI works there per `.github/workflows/ci-flutter.yml`).

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick (backend) | After a backend service-only task | `go test ./internal/sheet/...` |
| Full (backend) | After a backend route task | `go test ./internal/sheet/... ./internal/app/...` |
| Quick (desktop) | After a single-file desktop task | `flutter test test/<area>/<file>.dart` |
| Full (desktop) | After a task that touches shared codegen/l10n or cross-file behavior | `flutter test` |
| Build | After each phase completion | backend: `make fmt && make lint && make vet && go test ./...` · desktop: `cd desktop && flutter analyze && flutter test` |

---

## Execution Plan

Phases are ordered and run sequentially - each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Backend dry-run

The shared recalculation helper first, then the sheet preview, then the template preview.

```
T1 -> T2 -> T3
T1 -> T4 -> T5
```

### Phase 2: Desktop data plumbing

API client, draft store/scheduler, limiter, controller, coordinator.

```
T1 -> T6 -> T10
T7 -> T10
T9 -> T10
T7 -> T11
T8 -> T11
T10 -> T11
```

### Phase 3: Desktop surface and cleanup

Footer Preview action, discard deletion, eviction deletion, library-removal deletion.

```
T10 -> T12
T11 -> T12
T7 -> T13
T7 -> T14
T12 -> T15
```

---

## Task Breakdown

### T1: Extract the shared recalculation helper

**What**: Extract the fetch → flatten → `NormalizeAndRecalculate` → deep-merge sequence inside `PartialUpdate` into a private `recalculate(ctx, sheetID, data)` returning a small struct (stored sheet, normalized update, cascade, merged data). `PartialUpdate` calls it, then persists `merged` + `updated_at` and returns the nested cascade exactly as today. Add unit tests that the helper returns the same cascade and merged data as the old inline path and that PATCH behavior is unchanged (success, unknown field, derived field, exclusive conflict, missing sheet, malformed id).
**Where**: `internal/sheet/service.go`
**Depends on**: None
**Reuses**: `PartialUpdate`, `maputils.FlattenPathKeys`/`DeepCopy`/`ApplyUpdates`, `nestedCascadingEffects`, the `translate*` helpers, `sheettest` fixtures
**Requirement**: DRY-02, DRY-04

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `recalculate` performs require-ID, fetch, DAG lookup, flatten, `NormalizeAndRecalculate` and deep-merge with no repository write
- [x] `PartialUpdate` persists `merged` + `updated_at` and returns the same nested cascade as before
- [x] Every existing `PartialUpdate` test still passes unchanged
- [x] Gate check passes: `go test ./internal/sheet/...`
- [x] Test count: `internal/sheet` suite passes with the new helper cases added (no silent deletions)

**Tests**: unit
**Gate**: quick (backend)

**Commit**: `refactor(sheet): extract the shared recalculation helper`

---

### T2: Sheet preview service method

**What**: Add `Preview(ctx, PartialUpdateSheetCommand) (map[string]any, error)` to the `sheet.Service` interface and implementation: call `recalculate`, return `nestedCascadingEffects(cascade)`, never persist. Unit tests: valid delta returns the cascade and calls no `PartialUpdate`; the same delta yields the same cascade as `PartialUpdate`; a repeated preview is identical; an empty delta returns `{}` with no write; and the mapped errors for malformed/missing id, missing sheet, unknown field, derived field, exclusive conflict and over-deep nesting.
**Where**: `internal/sheet/service.go`
**Depends on**: T1
**Reuses**: `recalculate` (T1), `nestedCascadingEffects`, the `translate*` helpers, `sheettest` fixtures
**Requirement**: DRY-01, DRY-02, DRY-03, DRY-04, DRY-05, DRY-06, DRY-07, DRY-08, DRY-09, DRY-10, DRY-11, DRY-12

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] A valid delta returns the cascading effects with zero `PartialUpdate` calls
- [x] Preview and an equivalent PATCH return equal cascades for the same sheet state and delta
- [x] A repeated preview returns an identical result
- [x] An empty delta returns an empty map and issues no write
- [x] Each listed error path asserts its `apperr` code
- [x] Gate check passes: `go test ./internal/sheet/...`
- [x] Test count: `internal/sheet` suite passes with the new preview cases added (no silent deletions)

**Tests**: unit
**Gate**: quick (backend)

**Commit**: `feat(sheet): add the stateless sheet preview`

---

### T3: Sheet preview HTTP route

**What**: Add `SheetController.Preview` and register `POST /sheets/:id/preview` in the `/sheets` group. The handler decodes `{"data": <delta>}` with `shared/json`, maps decode errors with `decodeError`, calls `service.Preview` and returns the cascade via `response.OK`. Add integration and error-mapping tests asserting `200` with the cascade and the exact HTTP codes for missing/malformed id, missing sheet, unknown field, derived field, exclusive conflict and too-deep nesting.
**Where**: `internal/app/sheet.go`
**Depends on**: T2
**Reuses**: `SheetController.PartialUpdate`, `decodeError`, `response.OK`, the `integration_test.go`/`error_mapping_test.go` harnesses
**Requirement**: DRY-01, DRY-07, DRY-08, DRY-09, DRY-10, DRY-11, DRY-12

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `POST /sheets/:id/preview` responds `200` with `data` holding the cascade
- [x] Every listed error asserts the same status/code as the equivalent PATCH
- [x] Gate check passes: `go test ./internal/sheet/... ./internal/app/...`
- [x] Test count: `internal/app` suite passes with the new preview route cases added (no silent deletions)

**Tests**: integration
**Gate**: full (backend)

**Commit**: `feat(app): expose POST /sheets/:id/preview`

---

### T4: Template preview service method

**What**: Add `PreviewTemplateCommand{TemplateID, Data}` and `PreviewTemplate(ctx, cmd) (map[string]any, error)`: `FetchTemplate` (translated), DAG lookup, seed `buildNestedAllFields`, flatten the seed, `dag.Recalculate` to fill derived leaves, then `NormalizeAndRecalculate(seeded, updates)` and return the nested cascade. No persistence. Unit tests: valid delta returns the cascade; the seed matches `Create`'s initial state; missing/malformed template id and unknown template map to the expected errors; unknown/derived keys map like the sheet preview.
**Where**: `internal/sheet/service.go`
**Depends on**: T1
**Reuses**: `buildNestedAllFields`, `translateTemplateErr`, `translateDAGErr`, `nestedCascadingEffects`
**Requirement**: DRY-13, DRY-14, DRY-15, DRY-16, DRY-17

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] A valid delta returns the cascade computed over the template's seeded fields
- [x] The seed equals the initial data `Create` builds for the template
- [x] Missing/malformed id and unknown slug assert their codes
- [x] Unknown and derived keys assert the same codes as the sheet preview
- [x] Gate check passes: `go test ./internal/sheet/...`
- [x] Test count: `internal/sheet` suite passes with the new template preview cases added (no silent deletions)

**Tests**: unit
**Gate**: quick (backend)

**Commit**: `feat(sheet): add the stateless template preview`

---

### T5: Template preview HTTP route

**What**: Add `SheetController.PreviewTemplate` and register `POST /templates/:id/preview` on the `/templates` group (pass the group into `SheetController` registration). The handler decodes only `{"data": <delta>}`, sets the template id from the path param, and returns the cascade via `response.OK`. Add integration and error-mapping tests for the `200` cascade and the `400`/`404` paths.
**Where**: `internal/app/sheet.go`
**Depends on**: T4
**Reuses**: `SheetController.Preview` shape, `decodeError`, `response.OK`, the router group wiring
**Requirement**: DRY-13, DRY-14, DRY-15

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `POST /templates/:id/preview` responds `200` with `data` holding the cascade
- [x] Missing/malformed id responds `400`; an unknown slug responds `404`
- [x] Gate check passes: `go test ./internal/sheet/... ./internal/app/...`
- [x] Test count: `internal/app` suite passes with the new template preview route cases added (no silent deletions)

**Tests**: integration
**Gate**: full (backend)

**Commit**: `feat(app): expose POST /templates/:id/preview`

---

### T6: API client preview method

**What**: Add `Future<Map<String, Object?>> previewSheet(String sheetId, Map<String, Object?> delta)` to `ApiClient`, posting `{"data": delta}` to `/sheets/:id/preview` through `_post` and decoding with `_decode` (success cast to `Map<String, Object?>`, domain errors mapped by the envelope decoder). Add unit tests for verb/path/body, success decode, a mapped `UNKNOWN_FIELD` error, and that no retry is issued on transport failure.
**Where**: `desktop/lib/api/api_client.dart`
**Depends on**: T1
**Reuses**: `_post`, `_decode`, `patchSheet`'s decode shape, the existing mocked-transport harness in `api_client_test.dart`
**Requirement**: PRV-03

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `previewSheet` sends `POST /sheets/:id/preview` with `{"data": delta}`
- [x] A `200` decodes to the cascade map
- [x] A mapped error surfaces as `ApiDomainException` with its kind
- [x] A transport failure issues exactly one attempt (no retry)
- [x] Gate check passes: `flutter test test/api/api_client_test.dart`
- [x] Test count: `api_client_test.dart` passes with the new preview cases added (no silent deletions)

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(desktop): add the sheet preview client method`

---

### T7: Draft backup store

**What**: Add `DraftBackupStore` rooted at the injected app-support directory: `read(sheetId)` returns the draft delta or null on a missing/corrupt/unsupported file; `write(sheetId, delta)` writes a versioned envelope (`schema_version`, `sheet_id`, `delta`, `saved_at`) through a temp file + rename, throwing a typed failure on an unwritable path; `delete(sheetId)` is best-effort. Add `draftBackupStoreProvider` over `appSupportDirectoryProvider`. Unit tests with a temp directory: write/read round-trip, delete, missing file, corrupt JSON, newer schema version, atomic replace leaving the prior file on a failed write, and path shape.
**Where**: `desktop/lib/sheet/draft_backup_store.dart`
**Depends on**: None
**Reuses**: `SheetLibraryStore`'s temp + rename pattern and versioned envelope, `appSupportDirectoryProvider`
**Requirement**: DRB-04, DRB-05, DRB-12, DRB-14

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `write` then `read` returns the same delta nested by path
- [x] A missing, corrupt or newer-version file reads as null
- [x] A failed write leaves any prior file intact
- [x] `delete` removes the file and is a no-op when absent
- [x] Gate check passes: `flutter test test/sheet/draft_backup_store_test.dart`
- [x] Test count: new suite passes with the cases above (no silent deletions)

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(desktop): add the atomic draft backup store`

---

### T8: Draft backup scheduler

**What**: Add a `DraftBackupScheduler` interface with `start(onTick)`/`cancel()` and a `PeriodicDraftBackupScheduler` over `Timer.periodic(Duration(seconds: 60))`; `start` cancels any existing timer first and `cancel` is idempotent. Add `draftBackupSchedulerProvider` (keepAlive) that cancels on dispose. Unit tests use a fake scheduler to assert start fires, cancel stops, and repeated start does not double the tick.
**Where**: `desktop/lib/sheet/draft_backup_scheduler.dart`
**Depends on**: None
**Reuses**: `HealthScheduler`/`PeriodicHealthScheduler`/`healthSchedulerProvider` shape
**Requirement**: DRB-01

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `start` schedules a 60-second periodic tick
- [ ] A second `start` does not double ticks; `cancel` stops them
- [ ] Disposing the provider cancels the timer
- [ ] Gate check passes: `flutter test test/sheet/draft_backup_scheduler_test.dart`
- [ ] Test count: new suite passes with the cases above (no silent deletions)

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(desktop): add the periodic draft backup scheduler`

---

### T9: Rate limiter, clock and cooldown scheduler

**What**: Add `clockProvider` (`DateTime Function()`, keepAlive), `RollingWindowLimiter({maxRequests, window, now})` with `isBlocked`, `record()` and `nextFreeAt()`, and a `CooldownScheduler` one-shot abstraction (`schedule(delay, onFire)`/`cancel()`) plus a timer-backed implementation and provider (keepAlive, cancels on dispose). Unit tests with a controllable clock and a fake scheduler: three requests allowed, the fourth blocked, a request exactly at the window edge freeing a slot, the rolling behavior as old entries age out, and two limiters not sharing a window.
**Where**: `desktop/lib/sheet/action_rate_limiter.dart`
**Depends on**: None
**Reuses**: the injected-`now` pattern of `SheetLibraryStore`
**Requirement**: SVE-02, SVE-03, SVE-04, SVE-05

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Three records allow a fourth check only after the window frees one slot
- [ ] `nextFreeAt` returns the earliest instant a slot frees
- [ ] Two limiters keep independent windows
- [ ] The cooldown scheduler fires once after the delay and can be cancelled
- [ ] Gate check passes: `flutter test test/sheet/action_rate_limiter_test.dart`
- [ ] Test count: new suite passes with the cases above (no silent deletions)

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(desktop): add the rolling action rate limiter`

---

### T10: Preview, limits, restore and delete on the edit controller

**What**: Extend `SheetEditState` with `revision`, `previewedRevision`, `isPreviewing`, `previewError`, `isPreviewBlocked`, `isSaveBlocked`, `isBusy`, `canPreview`, `canSave` (use a `copyWith` to keep every emission consistent). Increment `revision` on data-changing edits. Add `preview()`: single-flight, sends one `previewSheet` with the pending delta, records the attempt and schedules the cooldown, on `200` merges the cascade into `data` without advancing the baseline and records `previewedRevision`, on failure keeps edits and sets `previewError`. Guard `save()` on `isSaveBlocked` and record the attempt; on success delete the sheet's draft backup. Seed the draft backup in `build`, filtered to editable leaves of the loaded template. Unit tests cover every AC: enable-after-edit, disabled repeat, one dry-run zero PATCH, merge without clearing dirty, failure keeps edits and re-enables, save clears `previewedRevision`, single-flight, both rate windows, restore overlay + filter, and delete on save.
**Where**: `desktop/lib/sheet/sheet_edit_provider.dart`
**Depends on**: T6, T7, T9
**Reuses**: `mergeCascade`, `pendingDelta`, `setPathValue`/`valueAtPath`, `failureCopy`, `buildSections` leaves + `FieldSchema.isEditable`, `apiClientProvider`
**Requirement**: PRV-01, PRV-02, PRV-03, PRV-04, PRV-05, PRV-06, PRV-07, PRV-08, PRV-09, PRV-10, SVE-01, SVE-02, SVE-03, SVE-04, SVE-05, SVE-06, SVE-07, DRB-05, DRB-06, DRB-07, DRB-08, DRB-12

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Preview is enabled after an edit and disabled again with no new edit
- [ ] `preview()` issues exactly one dry-run and zero `PATCH` requests
- [ ] A `200` merges the cascade, rebuilds only changed controls and keeps `isPending` true
- [ ] A failure keeps the edits, sets `previewError` and leaves Preview available
- [ ] A save clears `previewedRevision`; a successful save deletes the draft file
- [ ] While a preview is in flight a save issues no request, and vice versa
- [ ] The preview and save windows are independent and re-enable after cooldown
- [ ] Opening a sheet with a draft restores only its editable leaves and marks it pending
- [ ] A corrupt draft seeds clean from the stored sheet
- [ ] Gate check passes: `flutter test test/sheet/sheet_edit_provider_test.dart`
- [ ] Test count: `sheet_edit_provider_test.dart` passes with the new cases added (no silent deletions)

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(desktop): preview edits and restore drafts on the edit controller`

---

### T11: Draft auto-backup coordinator

**What**: Add a `draftAutoBackup(sheetId)` provider that watches the edit controller's `isPending`: while pending it starts `draftBackupSchedulerProvider`, otherwise it cancels; the tick callback writes the controller's current `pendingDelta` through `draftBackupStoreProvider`, swallowing write failures; the provider cancels on dispose. Unit tests with a fake scheduler and a temp directory: a dirty sheet writes on each tick, a clean sheet writes nothing and leaves the scheduler stopped, a completed sheet starts exactly one scheduler, and disposal cancels it.
**Where**: `desktop/lib/sheet/draft_auto_backup.dart`
**Depends on**: T7, T8, T10
**Reuses**: `sheetEditControllerProvider.select`, `draftBackupSchedulerProvider`, `draftBackupStoreProvider`
**Requirement**: DRB-01, DRB-02, DRB-03, DRB-13

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] A pending sheet writes the current delta on every tick
- [ ] A clean sheet cancels the scheduler and writes nothing
- [ ] Exactly one scheduler runs per dirty transition
- [ ] A write failure is swallowed and editing continues
- [ ] Disposal cancels the scheduler
- [ ] Gate check passes: `flutter test test/sheet/draft_auto_backup_test.dart`
- [ ] Test count: new suite passes with the cases above (no silent deletions)

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(desktop): auto-backup the dirty draft every minute`

---

### T12: Footer Preview action and auto-backup activation

**What**: Extend the Screen 2 footer's `Consumer` to select the preview flags and render a secondary `Preview changes` `TextButton` (`sheet-preview`) left of Save: enabled only when `canPreview`, disabled otherwise with the correct tooltip (nothing new to preview / request in progress / preview limit / save limit). Show `previewError` above the action row alongside `saveError`, and deactivate the back/F5 guards while `isBusy`. Activate `draftAutoBackupProvider(sheetId)` from the loaded branch. Add the l10n keys. Add widget tests: one dry-run zero PATCH per edit, disabled repeat, mapped preview error inline, both actions disabled while a request is in flight, and the limit tooltip.
**Where**: `desktop/lib/sheet/sheet_screen.dart`
**Depends on**: T10, T11
**Reuses**: the footer `Consumer`, `_leave`/`_refreshTemplate` guards, `AppSpacing`, the sheet l10n block
**Requirement**: PRV-01, PRV-02, PRV-03, PRV-04, PRV-08, PRV-11, SVE-01, SVE-02, SVE-03

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Preview renders in the footer only when the sheet and template are loaded
- [ ] Activating Preview issues exactly one dry-run and zero PATCH; a repeat with no edit issues none
- [ ] The disabled states show the right tooltip (no changes / in progress / limit)
- [ ] A preview failure renders the mapped copy above the action row and keeps the edits
- [ ] Back and F5 are no-ops while a preview is in flight
- [ ] The draft auto-backup provider is active while the sheet is open
- [ ] Gate check passes: `flutter test test/sheet/sheet_screen_test.dart`
- [ ] Test count: `sheet_screen_test.dart` passes with the new cases added (no silent deletions)

**Tests**: widget
**Gate**: quick (desktop)

**Commit**: `feat(desktop): add the footer preview action`

---

### T13: Delete the draft on NOT_FOUND eviction

**What**: In `SheetOpenController._evict`, delete the sheet's draft backup alongside the library entry removal, best-effort so a delete failure never hides the missing-sheet message. Add tests that a `NOT_FOUND` response evicts the entry and removes the draft file, and that a delete failure still renders the missing-sheet copy.
**Where**: `desktop/lib/sheet/sheet_open_provider.dart`
**Depends on**: T7
**Reuses**: `_evict`, `sheetLibraryProvider`, `draftBackupStoreProvider`
**Requirement**: DRB-10

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] A `NOT_FOUND` load removes the draft file and the library entry
- [ ] A delete failure still publishes the missing-sheet copy
- [ ] A successful load leaves any draft untouched
- [ ] Gate check passes: `flutter test test/sheet/sheet_open_provider_test.dart`
- [ ] Test count: `sheet_open_provider_test.dart` passes with the new cases added (no silent deletions)

**Tests**: unit
**Gate**: quick (desktop)

**Commit**: `feat(desktop): drop the draft when a sheet is missing`

---

### T14: Delete the draft on library removal

**What**: In `library_section.dart`, when the removal confirmation deletes the entry, also delete that sheet's draft backup best-effort. Add widget tests that confirming removal deletes the draft file, and that a library write failure keeps the draft (the entry was not removed).
**Where**: `desktop/lib/catalog/library_section.dart`
**Depends on**: T7
**Reuses**: the removal confirmation flow, `sheetLibraryProvider`, `draftBackupStoreProvider`
**Requirement**: DRB-11

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Confirming removal deletes the sheet's draft file
- [ ] A failed library removal leaves the draft in place
- [ ] Cancelling the dialog deletes nothing
- [ ] Gate check passes: `flutter test test/catalog/library_section_test.dart`
- [ ] Test count: `library_section_test.dart` passes with the new cases added (no silent deletions)

**Tests**: widget
**Gate**: quick (desktop)

**Commit**: `feat(desktop): drop the draft when a sheet is removed`

---

### T15: Delete the draft on confirmed discard

**What**: In `_leave` and `_refreshTemplate`, when the user confirms the discard, delete the sheet's draft backup before popping/refreshing; a cancelled confirmation deletes nothing. Add widget tests for both dialogs: confirm deletes the file, cancel keeps it, and a clean sheet deletes nothing.
**Where**: `desktop/lib/sheet/sheet_screen.dart`
**Depends on**: T12
**Reuses**: `_leave`, `_refreshTemplate`, the discard confirmations, `draftBackupStoreProvider`
**Requirement**: DRB-09

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Confirming the leave discard deletes the draft file
- [ ] Confirming the refresh discard deletes the draft file
- [ ] Cancelling either confirmation deletes nothing
- [ ] A clean sheet with no draft performs no deletion
- [ ] Gate check passes: `flutter test test/sheet/sheet_screen_test.dart`
- [ ] Test count: `sheet_screen_test.dart` passes with the new cases added (no silent deletions)

**Tests**: widget
**Gate**: quick (desktop)

**Commit**: `feat(desktop): drop the draft when edits are discarded`

---

## Phase Execution Map

Visual representation of task ordering. Phases run in sequence, and tasks within a phase run in order:

```
Phase 1 → Phase 2 → Phase 3

T1 -> T2 -> T3
T1 -> T4 -> T5
T1 -> T6 -> T10
T7 -> T10
T9 -> T10
T7 -> T11
T8 -> T11
T10 -> T11
T10 -> T12
T11 -> T12
T7 -> T13
T7 -> T14
T12 -> T15
```

Execution is strictly sequential - there is no intra-phase parallelism. A single agent (or batch worker) works one task at a time, in order.

**Packing:** 15 tasks, packed into whole phases: Phase 1 (5) → batch 1, Phase 2 (6) → batch 2, Phase 3 (4) → batch 3. That is > 1 batch, so Execute offers batch sub-agents before dispatching. The always-on Verifier still runs after the last task.

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Extract the shared recalculation helper | 1 helper + 1 refactor | ✅ Cohesive |
| T2: Sheet preview service method | 1 service method | ✅ Granular |
| T3: Sheet preview HTTP route | 1 endpoint | ✅ Granular |
| T4: Template preview service method | 1 service method | ✅ Granular |
| T5: Template preview HTTP route | 1 endpoint | ✅ Granular |
| T6: API client preview method | 1 client method | ✅ Granular |
| T7: Draft backup store | 1 store | ✅ Cohesive |
| T8: Draft backup scheduler | 1 interface + impl + provider | ✅ Cohesive |
| T9: Rate limiter, clock, cooldown | 1 limiter + 1 scheduler | ✅ Cohesive |
| T10: Preview/limits/restore/delete on the controller | 1 controller's new behavior | ✅ Cohesive |
| T11: Draft auto-backup coordinator | 1 provider | ✅ Granular |
| T12: Footer Preview action + activation | 1 footer surface | ✅ Cohesive |
| T13: Delete draft on eviction | 1 cleanup hook | ✅ Granular |
| T14: Delete draft on removal | 1 cleanup hook | ✅ Granular |
| T15: Delete draft on discard | 1 cleanup hook | ✅ Granular |

**Granularity check**:

- ✅ 1 component / 1 function / 1 endpoint = Good
- ⚠️ 2-3 related things in same file = OK if cohesive
- ❌ Multiple components or files = MUST split

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | no arrow | ✅ Match |
| T2 | T1 | T1 -> T2 | ✅ Match |
| T3 | T2 | T2 -> T3 | ✅ Match |
| T4 | T1 | T1 -> T4 | ✅ Match |
| T5 | T4 | T4 -> T5 | ✅ Match |
| T6 | T1 | T1 -> T6 | ✅ Match |
| T7 | None | no arrow | ✅ Match |
| T8 | None | no arrow | ✅ Match |
| T9 | None | no arrow | ✅ Match |
| T10 | T6, T7, T9 | T6 -> T10, T7 -> T10, T9 -> T10 | ✅ Match |
| T11 | T7, T8, T10 | T7 -> T11, T8 -> T11, T10 -> T11 | ✅ Match |
| T12 | T10, T11 | T10 -> T12, T11 -> T12 | ✅ Match |
| T13 | T7 | T7 -> T13 | ✅ Match |
| T14 | T7 | T7 -> T14 | ✅ Match |
| T15 | T12 | T12 -> T15 | ✅ Match |

Every `Depends on` points backward or within the same phase; no forward-phase dependency exists.

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1: Extract the shared recalculation helper | Sheet service | unit | unit | ✅ OK |
| T2: Sheet preview service method | Sheet service | unit | unit | ✅ OK |
| T3: Sheet preview HTTP route | Preview routes | integration | integration | ✅ OK |
| T4: Template preview service method | Sheet service | unit | unit | ✅ OK |
| T5: Template preview HTTP route | Preview routes | integration | integration | ✅ OK |
| T6: API client preview method | API client | unit | unit | ✅ OK |
| T7: Draft backup store | Draft backup store | unit | unit | ✅ OK |
| T8: Draft backup scheduler | Draft backup scheduler | unit | unit | ✅ OK |
| T9: Rate limiter, clock, cooldown | Rate limiter | unit | unit | ✅ OK |
| T10: Preview/limits/restore/delete on the controller | Edit controller | unit | unit | ✅ OK |
| T11: Draft auto-backup coordinator | Draft auto-backup coordinator | unit | unit | ✅ OK |
| T12: Footer Preview action and auto-backup activation | Screen 2 footer | widget | widget | ✅ OK |
| T13: Delete the draft on NOT_FOUND eviction | Eviction cleanup | unit | unit | ✅ OK |
| T14: Delete the draft on library removal | Library removal cleanup | widget | widget | ✅ OK |
| T15: Delete the draft on confirmed discard | Screen 2 guards | widget | widget | ✅ OK |

No test is deferred to another task. Each task ships its tests in the same commit.

---

## Task Verification Standards

Every task MUST follow the `Done when` + `Tests` + `Gate` fields defined above. Each `Done when` entry is binary and references a gate command from the **Gate Check Commands** section.

---

## Requirement Traceability

| Requirement ID | Task(s) | Status |
| -------------- | ------- | ------ |
| DRY-01 | T2, T3 | Done |
| DRY-02 | T1, T2 | Done |
| DRY-03 | T2 | Done |
| DRY-04 | T1, T2 | Done |
| DRY-05 | T2 | Done |
| DRY-06 | T2 | Done |
| DRY-07 | T2, T3 | Done |
| DRY-08 | T2, T3 | Done |
| DRY-09 | T2, T3 | Done |
| DRY-10 | T2, T3 | Done |
| DRY-11 | T2, T3 | Done |
| DRY-12 | T2, T3 | Done |
| DRY-13 | T4, T5 | Done |
| DRY-14 | T4, T5 | Done |
| DRY-15 | T4, T5 | Done |
| DRY-16 | T4 | Done |
| DRY-17 | T4 | Done |
| PRV-01 | T10, T12 | Pending |
| PRV-02 | T10, T12 | Pending |
| PRV-03 | T6, T10, T12 | Pending |
| PRV-04 | T10, T12 | Pending |
| PRV-05 | T10 | Pending |
| PRV-06 | T10 | Pending |
| PRV-07 | T10 | Pending |
| PRV-08 | T10, T12 | Pending |
| PRV-09 | T10 | Pending |
| PRV-10 | T10 | Pending |
| PRV-11 | T12 | Pending |
| SVE-01 | T10, T12 | Pending |
| SVE-02 | T9, T10, T12 | Pending |
| SVE-03 | T9, T10, T12 | Pending |
| SVE-04 | T9, T10 | Pending |
| SVE-05 | T9 | Pending |
| SVE-06 | T10 | Pending |
| SVE-07 | T10 | Pending |
| DRB-01 | T8, T11 | Pending |
| DRB-02 | T11 | Pending |
| DRB-03 | T11 | Pending |
| DRB-04 | T7 | Done |
| DRB-05 | T7, T10 | Pending |
| DRB-06 | T10 | Pending |
| DRB-07 | T10 | Pending |
| DRB-08 | T10 | Pending |
| DRB-09 | T15 | Pending |
| DRB-10 | T13 | Pending |
| DRB-11 | T14 | Pending |
| DRB-12 | T7, T10 | Pending |
| DRB-13 | T11 | Pending |
| DRB-14 | T7 | Done |

**Coverage:** 49 total, 49 mapped to tasks, 0 unmapped.

---

## Open Decisions

- Task ordering and the two tables above are provisional until approved; the Test Coverage Matrix and Gate Check Commands become authoritative on approval.
- For each task, tools are set to `MCP: NONE` / `Skill: NONE`. Confirm before Execute whether any MCP or skill should be used instead.
