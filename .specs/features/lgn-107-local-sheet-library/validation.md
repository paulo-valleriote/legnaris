# Local Sheet Library Validation

## Validation: Local Sheet Library - PASS ✅

**Date**: 2026-10-02
**Spec**: `.specs/features/lgn-107-local-sheet-library/spec.md`
**Diff range**: `3b6dc46..HEAD` (HEAD `3b5a4f9705e84cb684a53f176d2d164e7c935bf9`); implementation commits `caefec2`, `50b2214`, `bef69d4`, `3fdc9e9`, `9d8942d`, `4c844ed`, `5150146`; test-coverage fix `3b5a4f9` (`0a56390` is docs only)
**Verifier**: independent sub-agent (author ≠ verifier), round 2

Round 1 returned FAIL on three gaps (LIB-03 partial replacement, LIB-13 durability, LIB-03 UTC serialization). Fix commit `3b5a4f9` strengthens the two test files; all three gaps are now closed and the two mutants that survived round 1 are now killed. The full suite is green and the sensor is 8/8 killed.

---

## Task Completion

| Task | Status     | Notes   |
| ---- | ---------- | ------- |
| T1   | ✅ Done    | 8 entry tests, incl. non-UTC `toJson` conversion |
| T2   | ✅ Done    | 3 exception tests |
| T3   | ✅ Done    | atomic save covered (`sheet_library_store_test.dart:42-139`) |
| T4   | ✅ Done    | recovery matrix covered (`sheet_library_store_test.dart:141-230`) |
| T5   | ✅ Done    | directory seam covered (`sheet_library_provider_test.dart:22-57`) |
| T6   | ✅ Done    | notifier covered, incl. full upsert replacement and real-store sequential durability |
| T7   | ✅ Done    | bootstrap glue, gate-only per matrix |

---

## Spec-Anchored Acceptance Criteria

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| LIB-01 absent/empty store → empty index, no error | `LibraryLoadOutcome.empty`, `entries == []` | `desktop/test/library/sheet_library_store_test.dart:143` - `expect(absent.outcome, LibraryLoadOutcome.empty)`; `:144` - `expect(absent.entries, isEmpty)` | ✅ PASS |
| LIB-02 new id added → store 4 fields + persist full index | entry with `{id,name,template_id,created_at}`, one save with full list | `desktop/test/library/sheet_library_provider_test.dart:109` - `expect(idsOf(container), ['id-1'])`; `:111` - `expect(saved.single.map((s) => s.id), ['id-1'])`; `desktop/test/library/sheet_library_entry_test.dart:19-22` | ✅ PASS |
| LIB-03 existing id → replace `name`, `template_id`, `created_at`, no second row | count stays 1; all three fields equal the new entry | `desktop/test/library/sheet_library_provider_test.dart:141` - `expect(entries, hasLength(1))`; `:143` - `expect(entries.single.name, 'New')`; `:144` - `expect(entries.single.templateId, 'new_v1')`; `:145` - `expect(entries.single.createdAt, DateTime.parse('2026-10-05T10:00:00.000Z'))` (loaded entry differs in all three fields, `:119-124`) | ✅ PASS |
| LIB-04 list ordered by `created_at` desc, ties reverse insertion | `['newest','tie-b','tie-a','oldest']` | `desktop/test/library/sheet_library_provider_test.dart:176` - `expect(idsOf(container), ['newest', 'tie-b', 'tie-a', 'oldest'])` | ✅ PASS |
| LIB-05 remove present id → remove + persist full index | remaining ids persisted | `desktop/test/library/sheet_library_provider_test.dart:191` - `expect(idsOf(container), ['id-2'])`; `:192` - `expect(saved.single.map((s) => s.id), ['id-2'])` | ✅ PASS |
| LIB-06 remove absent id → no-op, no persisted revision, no error | state unchanged, zero `save` calls | `desktop/test/library/sheet_library_provider_test.dart:205` - `expect(idsOf(container), ['id-1'])`; `:206` - `expect(store.saveCalls, 0)` | ✅ PASS |
| LIB-07 lookup one id → entry or null | matching entry; null when missing | `desktop/test/library/sheet_library_provider_test.dart:219` - `expect(found?.name, 'Aria')`; `:225-228` - `isNull` | ✅ PASS |
| LIB-08 never persist sheet `data`/cell/derived value | exactly the four keys, no `data` | `desktop/test/library/sheet_library_entry_test.dart:66` - `expect(json.keys.toSet(), {'id','name','template_id','created_at'})`; `:67` - `expect(json.containsKey('data'), isFalse)`; `desktop/test/library/sheet_library_store_test.dart:50` | ✅ PASS |
| LIB-09 top-level int `schema_version` == current on every write | `1` | `desktop/test/library/sheet_library_store_test.dart:46` - `expect(envelope['schema_version'], 1)`; `:128` (empty index) | ✅ PASS |
| LIB-10 mutation persists complete index before success | `save` receives the full next list; publish only after | `desktop/test/library/sheet_library_provider_test.dart:111`; `:192`; failed-save `:255` proves no publish on throw | ✅ PASS (ordering proven indirectly via LIB-11) |
| LIB-11 failed persist → in-memory == pre-mutation + report | throws `writeFailed`; ids unchanged; file content unchanged | `desktop/test/library/sheet_library_provider_test.dart:247-252` (kind `writeFailed`), `:255` - `expect(after.map((s) => s.id), before)`; `desktop/test/library/sheet_library_store_test.dart:98` - `expect(storeFile().readAsStringSync(), before)` | ✅ PASS |
| LIB-12 atomic replace (temp then rename), no truncation | temp gone, target holds new content, previous intact on failure | `desktop/test/library/sheet_library_store_test.dart:107` - `expect(tempFile().existsSync(), isFalse)`; `:110` - `expect(..., 'Second')`; `:98` | ✅ PASS |
| LIB-13 serialize mutations, each store complete and parseable | sequential mutations leave a reloadable, complete index | `desktop/test/library/sheet_library_provider_test.dart:148-165` - real temp-dir `SheetLibraryStore` (`:151`), 4 sequential mutations (`:154-157`), fresh store reload (`:159`) asserts `expect(reloaded.outcome, LibraryLoadOutcome.loaded)` (`:160`) and ids `{'id-2','id-3'}` (`:161-164`) | ✅ PASS (closed in round 2) |
| LIB-14 valid same-version store → entries unchanged | `loaded`, same ids/names | `desktop/test/library/sheet_library_store_test.dart:149` - `expect(loaded.outcome, LibraryLoadOutcome.loaded)`; `:150` - ids `['id-1','id-2']`; `:227-229` | ✅ PASS |
| LIB-15 older version + registered migration → apply chain | `loaded` with migrated entry | `desktop/test/library/sheet_library_store_test.dart:202` - `expect(upgraded.outcome, LibraryLoadOutcome.loaded)`; `:203` - `expect(upgraded.entries.single.id, 'id-1')` | ✅ PASS |
| LIB-16 older version, no migration → quarantine + empty | `quarantined`, `entries == []` | `desktop/test/library/sheet_library_store_test.dart:207` - `expect(noMigration.outcome, LibraryLoadOutcome.quarantined)`; `:208` | ✅ PASS |
| LIB-17 unreadable/corrupt → quarantine distinct name, bytes preserved, empty, no crash | one `.corrupt-*` file with original bytes, `quarantined`, empty | `desktop/test/library/sheet_library_store_test.dart:174` - `expect(result.outcome, LibraryLoadOutcome.quarantined)`; `:181` - `expect(quarantined, hasLength(1))`; `:182` - `expect(quarantined.single.readAsStringSync(), body)` | ✅ PASS |
| LIB-18 newer `schema_version` → quarantine + empty, no parse | `quarantined`, `entries == []` | `desktop/test/library/sheet_library_store_test.dart:217` - `expect(result.outcome, LibraryLoadOutcome.quarantined)`; `:218` | ✅ PASS (the "without parsing" mechanism is not separately observable; outcome asserted) |
| LIB-19 load reports `loaded`/`quarantined`/`empty`, no sheet data | enum outcome surfaced; no data field | `desktop/test/library/sheet_library_store_test.dart:143` (empty), `:149` (loaded), `:174` (quarantined); `desktop/test/library/sheet_library_provider_test.dart:98` - `expect(state.outcome, LibraryLoadOutcome.loaded)` | ✅ PASS |
| LIB-20 empty `id`/`name`/`template_id` → reject, no write | throws `invalidEntry`; no file created | `desktop/test/library/sheet_library_store_test.dart:71-78` (kind `invalidEntry`); `:79` - `expect(storeFile().existsSync(), isFalse)`; `desktop/test/library/sheet_library_entry_test.dart:108`; `desktop/test/library/sheet_library_exception_test.dart:22-23` | ✅ PASS |
| LIB-21 loaded store exposed via Riverpod read provider | state `entries` + `outcome`; store rooted at override | `desktop/test/library/sheet_library_provider_test.dart:98-99` - outcome + `expect(idsOf(container), ['id-2','id-1'])`; `:39-40` - store provider | ✅ PASS |
| LIB-22 successful mutation republished so consumers rebuild | listener receives the new list | `desktop/test/library/sheet_library_provider_test.dart:269` - `expect(emissions.last, ['id-1'])` | ✅ PASS |
| LIB-23 failed persist → published index == last persisted | published ids equal pre-mutation | `desktop/test/library/sheet_library_provider_test.dart:255` - `expect(after.map((s) => s.id), before)`; `:256` - `expect(after, hasLength(1))` | ✅ PASS |

**Status**: ✅ All ACs covered - 23/23 ACs matched the spec-defined outcome; no spec-precision gaps remain.

---

## Discrimination Sensor

Scratch: `git worktree add /tmp/opencode/lgn107-verify2 HEAD`, `flutter pub get` once, mutate, run the relevant `flutter test` file, `git checkout -- .`, then `git worktree remove --force`. No `git stash` used. Mutations 1-6 were injected in round 1 (against `5150146`); mutations 7 and 8 survived round 1 and were re-injected in round 2 against the fix `3b5a4f9`.

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| 1 | `desktop/lib/library/sheet_library_store.dart:151` | Saved `'schema_version': currentSchemaVersion + 1` | ✅ Killed (4 store tests fail; LIB-09) |
| 2 | `desktop/lib/library/sheet_library_store.dart:94` | Flipped newer-version guard `version > currentSchemaVersion` → `>=` | ✅ Killed (2 store tests fail; LIB-14 round-trip) |
| 3 | `desktop/lib/library/sheet_library_provider.dart:108` | Flipped tie-break `right.$1.compareTo(left.$1)` → `left.$1.compareTo(right.$1)` | ✅ Killed (order test fails; LIB-04) |
| 4 | `desktop/lib/library/sheet_library_provider.dart:77-79` | Removed the `if (index == -1) return;` no-op guard in `remove` | ✅ Killed (missing-id remove test fails; LIB-06) |
| 5 | `desktop/lib/library/sheet_library_entry.dart:50` | Weakened `isValid` (dropped `templateId.isNotEmpty`) | ✅ Killed (entry isValid test; LIB-20) |
| 6 | `desktop/lib/library/sheet_library_exception.dart:18` | `toString` drops the kind (`=> message`) | ✅ Killed (exception toString test) |
| 7 | `desktop/lib/library/sheet_library_provider.dart:64-66` | Upsert replaces only `name`, keeping old `template_id`/`created_at` | ✅ Killed (re-tested round 2) - `sheet_library_provider_test.dart:114` fails on `templateId`/`createdAt` |
| 8 | `desktop/lib/library/sheet_library_entry.dart:58` | `toJson` drops `.toUtc()` | ✅ Killed (re-tested round 2) - `sheet_library_entry_test.dart:70` fails on `endsWith('Z')` |

**Sensor depth**: P0-full (8 behavior-level mutations, all four files; the 2 round-1 survivors re-tested against the fix)
**Result**: 8/8 killed - PASS ✅

**Isolation check**: real-tree `git status --porcelain` after cleanup equals the round-2 baseline exactly (see Gate Check). The worktree was removed; `git worktree list` shows only the main worktree.

---

## Interactive UAT Results (if performed)

Not performed - backend/store feature with no user-facing surface in scope (per spec Out of Scope: Library UI is a separate card).

---

## Code Quality

| Principle        | Status |
| ---------------- | ------ |
| Minimum code     | ✅ - 60/19/202/127 lines; no speculative surface |
| Surgical changes | ✅ - fix `3b5a4f9` touches only the two test files; `main.dart`/`pubspec.yaml` are the round-1 bootstrap |
| No scope creep   | ✅ - no UI, no `data` caching, no settings; matches spec Out of Scope |
| Matches patterns | ✅ - `SheetLibrary` mirrors `Catalog`; `SheetLibraryException` mirrors `ApiError`; providers use `@Riverpod(keepAlive: true)` |
| Spec-anchored outcome check (asserted values match spec) | ✅ - LIB-03 now asserts all three replaced fields; LIB-13 durability and UTC serialization asserted |
| Per-layer Coverage Expectation met (domain 1:1 ACs; routes happy+edge+error) | ✅ - domain/store/provider cover every AC and listed edge case; bootstrap is gate-only by design |
| Every test maps to a spec requirement - no unclaimed tests | ✅ - all 38 new tests trace to an AC, edge case, or T1-T6 Done-when |
| Documented guidelines followed: `AGENTS.md` (root), `desktop/AGENTS.md` | ✅ - English only, typed errors, tests edge-first + happy path, no body narration, no hardcoded magic values |
| `// SPEC_DEVIATION` recorded | ✅ - `desktop/lib/library/sheet_library_store.dart:53` (migrations injected per store for testability, justified) |

---

## Edge Cases

- [x] Empty index lists empty (never null/error): `sheet_library_store_test.dart:144` - `expect(absent.entries, isEmpty)`.
- [x] Zero-byte file treated as corrupt and quarantined: `sheet_library_store_test.dart:155` variant `'zero bytes'`, asserted `:174-182`.
- [x] Missing OS store directory created on first write: `sheet_library_store_test.dart:60-63` - `expect(storeDir.existsSync(), isTrue)`.
- [x] Two entries sharing `created_at` ordered deterministically (reverse insertion): `sheet_library_provider_test.dart:176`.
- [x] Update stores the supplied `created_at` verbatim: `sheet_library_provider_test.dart:145` - `expect(entries.single.createdAt, DateTime.parse('2026-10-05T10:00:00.000Z'))` (no regeneration on update).
- [x] Rejected invalid entry writes no file: `sheet_library_store_test.dart:79` - `expect(storeFile().existsSync(), isFalse)`.

---

## Gate Check

- **Gate command**: `cd desktop && flutter analyze && flutter test` (Build gate, tasks.md §Gate Check Commands)
- **Result**: 417 passed, 0 failed, 4 skipped; `flutter analyze` → "No issues found!"
- **Exit code**: 0
- **Test count before feature**: 379 passed, 4 skipped (derived: 417 − 38 new library tests; no pre-existing test file was modified - the diff adds the four `test/library/*` files and the fix modifies two of them)
- **Test count after feature**: 417 passed, 4 skipped
- **Delta**: +38 tests (entry 8, exception 3, store 13, provider 14); the fix added the LIB-13 test (416 → 417) and strengthened existing assertions
- **Skipped tests**: 4 (pre-existing; outside this feature's diff surface - no library test is skipped)
- **Failures**: none

---

## Fix Plans (if issues found)

None. All three round-1 gaps are closed:
1. LIB-03 full replacement - `sheet_library_provider_test.dart:141-145`; mutant 7 killed.
2. LIB-13 durability - `sheet_library_provider_test.dart:148-165`.
3. LIB-03 UTC serialization - `sheet_library_entry_test.dart:79-88`; mutant 8 killed.

---

## Requirement Traceability Update

Not applied to `spec.md` (verifier is read-only over the real tree). Recorded here for the orchestrator:

| Requirement | Previous Status | New Status   |
| ----------- | --------------- | ------------ |
| LIB-01 … LIB-23 | Implementing | ✅ Verified |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 23/23 ACs matched spec outcome | 0 spec-precision gaps; all six listed edge cases handled
**Sensor**: 8/8 mutations killed (round-1 survivors 7 and 8 re-tested against `3b5a4f9` and killed)
**Gate**: 417 passed, 0 failed, 4 skipped; analyze clean

**What works**: Atomic temp+rename save; version recovery (absent/empty/corrupt/malformed/older+newer version) with byte-preserving quarantine; invalid-entry rejection with no write; full-field upsert replacement; write-through notifier whose state stays consistent on failure; deterministic ordering with tie-break; real-store sequential durability across a fresh reload; Riverpod read/write surface; bootstrap directory seam; ISO-8601 UTC serialization of non-UTC inputs; no `data` persisted.

**Issues found**: none.

**Next steps**: Feature is done. The orchestrator may record a lessons entry for the round-1 signal (weak upsert-replacement and UTC-conversion assertions, now fixed) per validate.md §10.

---

## Review-Fix Round Addendum (2026-10-03)

The Judge review of PR #54 published five findings (F1-F5); the code fixes
landed as `8a7e51f` (F2 load refactor), `2779131` (F3 rename-failure outcome),
`ff2da81` (F1 per-process temp + AC5 scoping) and `6ab7052` (F4 shared fixture).
The report above stays pinned to the validated snapshot `3b5a4f9` — its
`file:line` references point at that snapshot. This addendum registers the three
tests added by the fixes with current line numbers.

**Gate re-run after the fix commits** (same command as Gate Check, author-run):
`cd desktop && flutter analyze && flutter test` → analyze "No issues found!";
**420 passed, 0 failed, 4 skipped** (417 baseline + 3 new tests; the 4 skips are
the pre-existing `live_contract_test.dart` ones).

| New test | AC mapping | Current `file:line` + assertion | Result |
| -------- | ---------- | ------------------------------- | ------ |
| migrator `FormatException` is quarantined (F2, locks the single-try contract) | LIB-17 | `desktop/test/library/sheet_library_store_test.dart:221` (test), `:232` - `expect(result.outcome, LibraryLoadOutcome.quarantined)`; `:233` | ✅ PASS |
| quarantine rename failure reports `empty`, file kept, next save overwrites (F3) | LIB-17, LIB-19 | `desktop/test/library/sheet_library_store_test.dart:176` (test), `:189` - `expect(result.outcome, LibraryLoadOutcome.empty)`; `:191` - `expect(storeFile().readAsStringSync(), 'not json')`; `:194` - `expect(storeFile().readAsStringSync(), contains('"id-1"'))` | ✅ PASS |
| per-process temps: save never shares a foreign temp, load sweeps stale ones (F1) | LIB-12, LIB-14 | `desktop/test/library/sheet_library_store_test.dart:257` (test), `:267` - `expect(foreignTemp.existsSync(), isTrue)`; `:275` - ids `['id-1','id-2']`; `:276` - `expect(foreignTemp.existsSync(), isFalse)` | ✅ PASS |

**Shifted anchors** (post-fix coordinates for re-running the sensor above):
`sheet_library_store.dart` envelope line (mutant 1 target) → `:162`; newer-version
guard (mutant 2 target, `version > currentSchemaVersion`) → `:111`;
`SPEC_DEVIATION` → `:56`. Store test ranges: save block `:31-128`, load/recovery
and temp block `:130-277`. Test totals: store 16 (was 13), provider 14, entry 8,
exception 3 → 41 feature tests; suite 379 → 420.
