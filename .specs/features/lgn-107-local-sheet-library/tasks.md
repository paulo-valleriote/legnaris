# Local Sheet Library Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/lgn-107-local-sheet-library/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: `AGENTS.md` (root testing section: edge cases first + happy path, typed errors asserted), `desktop/AGENTS.md` (English-only, reuse app infrastructure), `.github/workflows/ci-flutter.yml` (`flutter analyze`, `flutter test`, `flutter build windows --release`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Domain model (`SheetLibraryEntry`) | unit | All branches of `fromJson`/`toJson`/`isValid`; 1:1 to spec ACs; edge cases: missing field, wrong type, empty strings, non-ISO date, `data` never emitted | `desktop/test/library/sheet_library_entry_test.dart` | `flutter test test/library/sheet_library_entry_test.dart` |
| Error type (`SheetLibraryException`) | unit | `kind` + `message` + `toString`; both kinds | `desktop/test/library/sheet_library_exception_test.dart` | `flutter test test/library/sheet_library_exception_test.dart` |
| Persistence store (`SheetLibraryStore`) | unit (real temp-dir I/O, no platform channel) | Every branch: load recovery matrix (absent, empty, corrupt, malformed shape, older/newer version, migration present/absent), quarantine preserves bytes, atomic save replaces target, invalid-entry rejection, write failure; every listed edge case | `desktop/test/library/sheet_library_store_test.dart` | `flutter test test/library/sheet_library_store_test.dart` |
| Riverpod providers / notifier (`SheetLibrary`) | unit | Write-through (success + failure), upsert dedupe, remove present/missing, ordering with tie-break, `entryFor`, outcome exposure, read provider rebuild; 1:1 to spec ACs | `desktop/test/library/sheet_library_provider_test.dart` | `flutter test test/library/sheet_library_provider_test.dart` |
| Bootstrap glue (`main.dart`, pubspec) | none | Build gate only (platform channel + `runApp` are not unit-testable) | - | build gate only |

## Gate Check Commands

> Generated from codebase - confirm before Execute. All commands run from `desktop/` (the CI works in `desktop` per `.github/workflows/ci-flutter.yml`).

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | After tasks with unit tests only | `flutter test test/library/<file>` |
| Full | After tasks that touch the notifier / cross-file behavior | `flutter test` |
| Build | After the bootstrap/config task and at phase completion | `flutter analyze && flutter test` (CI also runs `flutter build windows --release`) |

---

## Execution Plan

Phases are ordered and run sequentially - each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Foundation

Two independent leaves, ordered for readability.

```
T1
T2
```

### Phase 2: Persistence store

```
T3 → T4
```

### Phase 3: Riverpod surface

```
T5 → T6
```

### Phase 4: Bootstrap

```
T7
```

---

## Task Breakdown

### T1: Sheet library entry model

**What**: Immutable `SheetLibraryEntry` (`id`, `name`, `templateId`, `createdAt`) with JSON round-trip, ISO-8601 handling, and `isValid`.
**Where**: `desktop/lib/library/sheet_library_entry.dart`
**Depends on**: None
**Reuses**: Field naming and ISO-8601 handling from `desktop/lib/api/models/sheet.dart`
**Requirement**: LIB-02, LIB-03, LIB-08

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `fromJson` reads `id`, `name`, `template_id`, `created_at`; a missing field or wrong type throws `FormatException`
- [x] `toJson` writes `id`, `name`, `template_id`, and `created_at` as ISO-8601 UTC; it never emits a `data` key
- [x] `isValid` is false when `id`, `name`, or `templateId` is empty
- [x] Gate check passes: `flutter test test/library/sheet_library_entry_test.dart`
- [x] Test count: 8 tests pass (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): add the sheet library entry model`

---

### T2: Typed sheet library error

**What**: `SheetLibraryException` with `SheetLibraryErrorKind { writeFailed, invalidEntry }`, mirroring `ApiError`'s kind + message shape.
**Where**: `desktop/lib/library/sheet_library_exception.dart`
**Depends on**: None
**Reuses**: `desktop/lib/api/api_error.dart` (kind + message + `toString`)
**Requirement**: LIB-11, LIB-20

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Both kinds exist and carry a message
- [x] `toString` includes the kind and message
- [x] Gate check passes: `flutter test test/library/sheet_library_exception_test.dart`
- [x] Test count: 3 tests pass (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): add typed sheet library errors`

---

### T3: Persist the index atomically (store.save)

**What**: `SheetLibraryStore` constructed with an injected `Directory`; `currentSchemaVersion = 1`; `save(List<SheetLibraryEntry>)` validates, encodes `{schema_version, sheets}`, writes `<file>.tmp`, then renames over `<file>`.
**Where**: `desktop/lib/library/sheet_library_store.dart`
**Depends on**: T1, T2
**Reuses**: `dart:io` + `dart:convert` (same usage as `desktop/lib/api/envelope.dart`)
**Requirement**: LIB-08, LIB-09, LIB-10, LIB-11, LIB-12, LIB-13, LIB-20

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `save` writes a parseable `{schema_version: 1, sheets: [...]}` document with exactly the four entry fields
- [x] An invalid entry throws `SheetLibraryException.invalidEntry` and writes nothing
- [x] A filesystem failure throws `SheetLibraryException.writeFailed` and leaves the previous file content intact
- [x] After a successful save the temp file is gone and the target holds the new content (rename replaced it)
- [x] Gate check passes: `flutter test test/library/sheet_library_store_test.dart`
- [x] Test count: 8 tests pass (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): persist the sheet library index atomically`

---

### T4: Load the index with version recovery (store.load)

**What**: `SheetLibraryStore.load()` returning `LibraryLoadResult(outcome, entries)`: parse, version checks, migration chain, quarantine of bad/unsupported files, leftover `.tmp` cleanup, and empty-store handling.
**Where**: `desktop/lib/library/sheet_library_store.dart` (modify)
**Depends on**: T3
**Reuses**: T3 store (same class); `LibraryLoadOutcome`
**Requirement**: LIB-01, LIB-04, LIB-14, LIB-15, LIB-16, LIB-17, LIB-18, LIB-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Absent file → `empty` with no error; a saved index → `loaded` with the same entries across a fresh store instance (round-trip proof)
- [x] Zero-byte / invalid JSON / malformed shape / non-int version / bad entry → `quarantined` + `empty`, and the quarantined file still holds the original bytes
- [x] A registered migration for an older `schema_version` upgrades and returns `loaded`; a missing migration → `quarantined` + `empty`
- [x] `schema_version` newer than `currentSchemaVersion` → `quarantined` + `empty`, without parsing entries
- [x] A leftover `<file>.tmp` is deleted on load and the outcome is unaffected
- [x] Gate check passes: `flutter test test/library/sheet_library_store_test.dart`
- [x] Test count: 13 tests pass (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): load the sheet library index with version recovery`

---

### T5: Store providers and directory seam

**What**: `appSupportDirectoryProvider` (throws `StateError` until overridden at bootstrap/tests) and `sheetLibraryStoreProvider` building `SheetLibraryStore` over the injected directory.
**Where**: `desktop/lib/library/sheet_library_provider.dart`
**Depends on**: T4
**Reuses**: `@Riverpod(keepAlive: true)` pattern from `desktop/lib/api/api_client_provider.dart`
**Requirement**: LIB-21

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Overriding `appSupportDirectoryProvider` with a temp dir yields a store rooted there
- [x] Reading `appSupportDirectoryProvider` without an override throws `StateError`
- [x] Gate check passes: `flutter test test/library/sheet_library_provider_test.dart`
- [x] Test count: 3 tests pass (no silent deletions)

**Tests**: unit
**Gate**: quick

**Commit**: `feat(desktop): expose the sheet library store providers`

---

### T6: Sheet library notifier

**What**: `SheetLibraryState(entries, outcome)` and the `SheetLibrary` keepAlive notifier with `upsert`, `remove`, `entryFor`, sorted entries, and write-through persistence.
**Where**: `desktop/lib/library/sheet_library_provider.dart` (modify)
**Depends on**: T5
**Reuses**: `@Riverpod(keepAlive: true) class Catalog extends _$Catalog` shape from `desktop/lib/catalog/catalog_provider.dart`
**Requirement**: LIB-02, LIB-03, LIB-04, LIB-05, LIB-06, LIB-07, LIB-10, LIB-11, LIB-13, LIB-20, LIB-21, LIB-22, LIB-23

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `build` loads through the injected store and publishes `entries` + `outcome`
- [x] `upsert` appends a new id and replaces an existing one (entry count unchanged on duplicate)
- [x] `remove` deletes a present id and is a safe no-op for a missing id (no save call)
- [x] `entries` is ordered by `created_at` descending, ties broken by reverse insertion order
- [x] `entryFor` returns the entry or null
- [x] A forced `save` failure leaves the published entries equal to the last persisted state and rethrows
- [x] A successful mutation republishes so the read provider rebuilds
- [x] Gate check passes: `flutter test`
- [x] Test count: 10 tests pass (no silent deletions)

**Tests**: unit
**Gate**: full

**Commit**: `feat(desktop): add the sheet library notifier`

---

### T7: Bootstrap the store directory

**What**: Add `path_provider: ^2.1.6` (pubspec) and make `main()` await `WidgetsFlutterBinding.ensureInitialized()` + `getApplicationSupportDirectory()`, overriding `appSupportDirectoryProvider` before `runApp`.
**Where**: `desktop/lib/main.dart`
**Depends on**: T5
**Reuses**: Existing `ProviderScope` root in `desktop/lib/main.dart`
**Requirement**: LIB-21

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `path_provider` is in `desktop/pubspec.yaml` and `pubspec.lock` resolved
- [ ] `main` resolves the support directory and overrides `appSupportDirectoryProvider`
- [ ] Existing `app_boot_test.dart` and the whole suite stay green
- [ ] Gate check passes: `flutter analyze && flutter test`
- [ ] Test count: 0 new (bootstrap glue; existing suite unchanged and green)

**Tests**: none
**Gate**: build

**Commit**: `feat(desktop): bootstrap the local store directory`

---

## Phase Execution Map

Visual representation of task ordering. Phases run in sequence, and tasks within a phase run in order:

```
Phase 1 → Phase 2 → Phase 3 → Phase 4

T1 → T3
T2 → T3
T3 → T4
T4 → T5
T5 → T6
T5 → T7
```

Execution is strictly sequential - there is no intra-phase parallelism. A single agent (or batch worker) works one task at a time, in order.

**Packing:** 7 tasks total. That fits one task-budgeted batch (≤ ~8), so Execute runs inline in the main window - no batch sub-agents. The always-on Verifier still runs after the last task.

---

## Task Granularity Check

Before approving tasks, verify they are granular enough:

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Sheet library entry model | 1 model | ✅ Granular |
| T2: Typed sheet library error | 1 small type | ✅ Granular |
| T3: store.save | 1 method + private helpers of one class | ✅ Cohesive |
| T4: store.load | 1 method + private helpers of the same class | ✅ Cohesive |
| T5: Store providers and directory seam | 2 thin providers, 1 file | ✅ Cohesive |
| T6: Sheet library notifier | 1 notifier | ✅ Granular |
| T7: Bootstrap the store directory | 1 file change | ✅ Granular |

**Granularity check**:

- ✅ 1 component / 1 function / 1 endpoint = Good
- ⚠️ 2-3 related things in same file = OK if cohesive
- ❌ Multiple components or files = MUST split

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | no arrow | ✅ Match |
| T2 | None | no arrow | ✅ Match |
| T3 | T1, T2 | T1 → T3, T2 → T3 | ✅ Match |
| T4 | T3 | T3 → T4 | ✅ Match |
| T5 | T4 | T4 → T5 | ✅ Match |
| T6 | T5 | T5 → T6 | ✅ Match |
| T7 | T5 | T5 → T7 | ✅ Match |

Every `Depends on` points backward or within the same phase; no forward-phase dependency exists.

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1: Sheet library entry model | Domain model | unit | unit | ✅ OK |
| T2: Typed sheet library error | Error type | unit | unit | ✅ OK |
| T3: store.save | Persistence store | unit | unit | ✅ OK |
| T4: store.load | Persistence store | unit | unit | ✅ OK |
| T5: Store providers and directory seam | Riverpod providers | unit | unit | ✅ OK |
| T6: Sheet library notifier | Riverpod notifier | unit | unit | ✅ OK |
| T7: Bootstrap the store directory | Bootstrap glue (`main.dart`, pubspec) | none | none | ✅ OK |

No test is deferred to another task. Each store/provider task ships its tests in the same commit.

---

## Task Verification Standards

Every task MUST follow the `Done when` + `Tests` + `Gate` fields defined above. Each `Done when` entry is binary and references a gate command from the **Gate Check Commands** section. Include the expected test count to prevent silent deletions.
