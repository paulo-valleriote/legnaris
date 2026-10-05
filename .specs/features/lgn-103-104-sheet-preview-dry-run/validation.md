# sheet-preview-dry-run Validation

**Date**: 2026-10-04
**Round**: 2 (re-verification after `ed7900c` closed the round-1 coverage gaps)
**Spec**: `.specs/features/lgn-103-104-sheet-preview-dry-run/spec.md`
**Diff range**: `8ae4252..HEAD` (branch `feat/lgn-103-104-sheet-preview-dry-run`, 17 commits; adds `ed7900c` over round 1)
**Verifier**: independent sub-agent (author ≠ verifier)

## Amendment (2026-10-05) — preview marks (PRV-12 … PRV-14)

Author-side amendment (not independently re-verified) adding the dry-run
identification the round-2 report predates: after a successful preview, the
editable delta rows are labelled **Modified** and the derived rows the cascade
recalculated **Recalculated** (both with a change tint), while untouched derived
rows keep the base **Calculated** (RND-10). Marks clear on the next edit, save,
or discard, and a failed preview publishes none.

| ID | Criterion (WHEN X THEN Y) | Spec-defined outcome | Evidence | Result |
| -- | ------------------------- | -------------------- | -------- | ------ |
| PRV-12 | preview `200` | delta leaves labelled "Modified" with a change tint | `desktop/test/engine/field_control_test.dart` (`preview marks`: modified input + toggle tint), `desktop/test/engine/exclusive_control_test.dart` (group dropdown tint), `desktop/test/sheet/sheet_screen_test.dart` (row mark for a leaf and for the `armorClassAbility` group) | ✅ PASS (author) |
| PRV-13 | preview `200` | cascade leaves labelled "Recalculated" + tint; untouched derived keep "Calculated" | `field_control_test.dart` (recalculated read-only tints the value and drops the base label), `sheet_screen_test.dart` ("a preview marks the changed rows…": untouched `derived.initiative` keeps "Calculated") | ✅ PASS (author) |
| PRV-14 | new edit/save/discard after a preview; a failed preview | marks cleared; failure publishes none | `sheet_edit_provider_test.dart` (new edit clears; exclusive selection clears; failure/`NOT_FOUND` assert `previewMarks isNull`; save clears), `sheet_screen_test.dart` ("a new edit clears the preview marks", "a failed preview marks nothing") | ✅ PASS (author) |

Source: `desktop/lib/sheet/sheet_edit_provider.dart` (`PreviewMarks` published by
`preview()`), `desktop/lib/engine/sheet_mark.dart`, `desktop/lib/engine/field_control.dart`,
`desktop/lib/engine/exclusive_control.dart`, `desktop/lib/engine/sheet_form.dart`,
`desktop/lib/sheet/sheet_screen.dart`, l10n `fieldModified` / `fieldRecalculated`.
Requirement count is now **52** (DRY-01…17, PRV-01…14, SVE-01…07, DRB-01…14).

---

## Validation Result: PASS ✅

---

## Task Completion

| Task | Status | Notes |
| ---- | ------ | ----- |
| T1: Extract the shared recalculation helper | ✅ Done | `[x]` boxes; commit `35f25d8`; `recalculate` present at `internal/sheet/service.go:172`, `PartialUpdate` persists only |
| T2: Sheet preview service method | ✅ Done | commit `9cc082f`; `Preview` at `internal/sheet/service.go:237` |
| T3: Sheet preview HTTP route | ✅ Done | commit `b320a6c`; `POST /:id/preview` at `internal/app/sheet.go:25` |
| T4: Template preview service method | ✅ Done | commit `1a22bf7`; `PreviewTemplate` at `internal/sheet/service.go:250` |
| T5: Template preview HTTP route | ✅ Done | commit `b7367ac`; `RegisterTemplatePreviewRoute` at `internal/app/sheet.go:32`, wired `internal/app/router.go:34` |
| T6: API client preview method | ✅ Done | commit `006aed2`; `previewSheet` at `desktop/lib/api/api_client.dart:85` |
| T7: Draft backup store | ✅ Done | commit `da2a87c`; `desktop/lib/sheet/draft_backup_store.dart` |
| T8: Draft backup scheduler | ✅ Done | commit `6867a91`; `desktop/lib/sheet/draft_backup_scheduler.dart` |
| T9: Rate limiter, clock, cooldown | ✅ Done | commit `247112e`; `desktop/lib/sheet/action_rate_limiter.dart` |
| T10: Preview/limits/restore/delete on controller | ✅ Done | commit `be3f1cc`; `desktop/lib/sheet/sheet_edit_provider.dart`; the "rebuilds only changed controls" done-when is now asserted (`sheet_screen_test.dart:2124-2169`) |
| T11: Draft auto-backup coordinator | ✅ Done | commit `82a057a`; `desktop/lib/sheet/draft_auto_backup.dart` |
| T12: Footer Preview action + activation | ✅ Done | commit `f3b812e`; `desktop/lib/sheet/sheet_screen.dart` |
| T13: Delete draft on NOT_FOUND eviction | ✅ Done | commit `a0073ee`; `SheetOpenController._evict` `desktop/lib/sheet/sheet_open_provider.dart:98` |
| T14: Delete draft on library removal | ✅ Done | commit `371a4cf`; `library_section.dart:153` |
| T15: Delete draft on confirmed discard | ✅ Done | commit `02815aa`; `sheet_screen.dart:325`, `:354` |

All 15 `[x]` boxes are checked and the tasks.md Requirement Traceability table marks every ID `Done`. `ed7900c` (`test(desktop): close the preview verification gaps`) adds only test cases (+44 lines `sheet_edit_provider_test.dart`, +47 lines `sheet_screen_test.dart`, 0 deletions) and no task scope change. Spec.md's traceability is pre-filled `Verified`; this report independently confirms it.

---

## Spec-Anchored Acceptance Criteria

49 requirements (DRY-01…17, PRV-01…11, SVE-01…07, DRB-01…14). Every row cites a real `file:line` + assertion; the asserted value is checked against the spec-defined outcome.

### P1: Stateless sheet preview endpoint (DRY-01…12)

| ID | Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| -- | ------------------------- | -------------------- | ----------------------- | ------ |
| DRY-01 | valid delta for existing sheet → response | `200`, `data` = recalculated cascade | `internal/app/error_mapping_test.go:304` `require.Equal(http.StatusOK, rec.Code)`; `:318` `assert.Equal(4.0, modifiers["dexterityMod"])`; `internal/sheet/service_test.go:692` cascade map | ✅ PASS |
| DRY-02 | computed through the same recalculation PATCH uses | one shared helper | `internal/sheet/service.go:238` (`Preview` calls `s.recalculate`) vs `:217` (`PartialUpdate` calls `s.recalculate`); `service_test.go:716` `assert.Equal(patchCascade, previewCascade)` | ✅ PASS |
| DRY-03 | same sheet + same delta → same cascade for PATCH and preview | byte-equal cascades | `internal/app/integration_test.go:486` `assert.Equal(patched.Data, previewed.Data)` | ✅ PASS |
| DRY-04 | persist no change during preview | zero repository writes, stored data unchanged | `internal/sheet/service_test.go:696` `mockRepo.AssertNotCalled(t, "PartialUpdate")`; `integration_test.go:475` `assert.Equal(10.0, fetchedStats["dexterity"])` after preview | ✅ PASS |
| DRY-05 | repeat same preview against unchanged sheet | identical cascade | `service_test.go:734` `assert.Equal(first, second)` | ✅ PASS |
| DRY-06 | empty delta | `200` + empty `data` object, persist nothing | `service_test.go:748` `assert.Equal(map[string]any{}, cascades)`; `:749` no write (route wraps with `response.OK`, `internal/app/sheet.go:92`) | ✅ PASS |
| DRY-07 | missing / non-canonical UUID v7 id | `400 INVALID_REQUEST`, same message as PATCH | `service_test.go:779` `assert.Equal(tc.wantMsg, appErr.Message)` + `:777-778`; `error_mapping_test.go:332-333` `"INVALID_REQUEST"` + `sheet.MsgSheetIDInvalid` | ✅ PASS |
| DRY-08 | sheet does not exist | `404 NOT_FOUND` | `service_test.go:800-802` `CodeNotFound`/`404`/`"sheet not found"`; `error_mapping_test.go:347-348` | ✅ PASS |
| DRY-09 | delta names a key the DAG does not know | `400 UNKNOWN_FIELD` | `service_test.go:816` `wantCode: CodeUnknownField` + `:850`; `error_mapping_test.go:371` + `:399` `"UNKNOWN_FIELD"` | ✅ PASS |
| DRY-10 | delta names a derived (non-editable) leaf | `400 INVALID_REQUEST` | `service_test.go:821-823` `CodeInvalidRequest` + `:850`; `error_mapping_test.go:376` | ✅ PASS |
| DRY-11 | two members of one exclusive group | `400 INVALID_REQUEST`, same conflict mapping as PATCH | `service_test.go:826-829` `"conflicting exclusive options: armorClassAbility"`; HTTP `error_mapping_test.go:381` | ✅ PASS |
| DRY-12 | delta nests deeper than supported depth | `400 INVALID_REQUEST` | `service_test.go:879-881` `CodeInvalidRequest` + `"nested too deeply"`; `error_mapping_test.go:416` | ✅ PASS |

### P2: Stateless template preview endpoint (DRY-13…17)

| ID | Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| -- | ------------------------- | -------------------- | ----------------------- | ------ |
| DRY-13 | valid delta for existing template | `200` + cascade seeded from template fields | `internal/sheet/service_test.go:937-940`; `error_mapping_test.go:486` `assert.Equal(4.0, mod["dexterityMod"])`; `integration_test.go:503-506` | ✅ PASS |
| DRY-14 | seed = initial data `POST /sheets` builds | template preview cascade == a preview over the created sheet | `service_test.go:945` test + `:980` `assert.Equal(t, previewCascade, previewTemplateCascade)` (compares against `Create` output) | ✅ PASS |
| DRY-15 | missing / invalid template slug | `400 INVALID_REQUEST` | `service_test.go:1039-1041`; `error_mapping_test.go:505-506` (`"template id is not a valid slug"`), `:523-524` (`"template id not provided"`) | ✅ PASS |
| DRY-16 | template does not exist | `404 NOT_FOUND` | `service_test.go:1008-1014` + `:1039-1041`; `error_mapping_test.go:541-542` `"template not found"` | ✅ PASS |
| DRY-17 | unknown key / derived leaf | same `UNKNOWN_FIELD` / `INVALID_REQUEST` mapping as sheet preview | `service_test.go:1053-1064` + `:1085-1087`; `error_mapping_test.go:558-591` | ✅ PASS |

### P1: Preview the recalculated cascade without persisting (PRV-01…11)

| ID | Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| -- | ------------------------- | -------------------- | ----------------------- | ------ |
| PRV-01 | pending delta differs from last previewed | Preview enabled | `desktop/test/sheet/sheet_edit_provider_test.dart:867` `canPreview` false → `:869` `isTrue` after edit; widget `test/sheet/sheet_screen_test.dart:2053` (tap enabled after `enterText`) | ✅ PASS |
| PRV-02 | pending delta equals last previewed | disabled + tooltip "nothing new to preview" | `sheet_edit_provider_test.dart:899` `canPreview` false; `sheet_screen_test.dart:2112` `sheetPreviewNoChangesTooltip` | ✅ PASS |
| PRV-03 | Preview activated | exactly one `POST /sheets/:id/preview` carrying the pending editable delta | `sheet_edit_provider_test.dart:874-880`; `test/api/api_client_test.dart:180-190` verb/path/body; `sheet_screen_test.dart:2088-2095` | ✅ PASS |
| PRV-04 | Preview activated | zero `PATCH /sheets/:id` | `sheet_edit_provider_test.dart:875` `patches(recorded), isEmpty`; `sheet_screen_test.dart:2095` | ✅ PASS |
| PRV-05 | preview `200` | merge cascade into in-memory draft; **update only the controls whose value changed** | merge: `sheet_edit_provider_test.dart:883-889` (`derived.armorClass` 13, `modifiers.dexterityMod` 3). Granular rebuild: `sheet_screen_test.dart:2143-2149` captures the untouched `stats.dexterity` control, `:2151` activates Preview, `:2154-2158` `identical(before, untouchedControl()), isTrue` with `reason: 'only the control whose value changed may rebuild (PRV-05)'`, and `:2159-2167` asserts the changed control updates (`sheet-value-derived.armorClass` data `'13'`) | ✅ PASS (round-1 gap closed) |
| PRV-06 | preview `200` | keep every local edit pending; do NOT advance baseline | `sheet_edit_provider_test.dart:890` `isPending, isTrue`; `:892-895` baseline `stats.dexterity` still 16; `sheet_screen_test.dart:2101-2104` | ✅ PASS |
| PRV-07 | preview `200` | record current pending delta as previewed | `sheet_edit_provider_test.dart:898` `previewedRevision == revision`; `:899` `canPreview` false | ✅ PASS |
| PRV-08 | preview fails | keep edits; render mapped copy inline in footer | `sheet_edit_provider_test.dart:962-964`; `sheet_screen_test.dart:2143-2151` (`sheet-preview-error` above action row) | ✅ PASS |
| PRV-09 | preview fails | do NOT record delta; Preview stays available | `sheet_edit_provider_test.dart:965-966` `previewedRevision isNull` + `canPreview isTrue` | ✅ PASS |
| PRV-10 | save succeeds | clear recorded preview delta | `sheet_edit_provider_test.dart:1034` `previewedRevision isNull`; `:1038-1039` new edit re-enables | ✅ PASS |
| PRV-11 | loading / error / no fields | Preview action not rendered | `sheet_screen_test.dart:2316` (loading), `:2336` (detail error), `:2352` (no fields) | ✅ PASS |

### P1: Preview and Save single-flight and rate-limited (SVE-01…07)

| ID | Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| -- | ------------------------- | -------------------- | ----------------------- | ------ |
| SVE-01 | preview or save in flight | both disabled; no second request | `sheet_edit_provider_test.dart:1045-1069`, `:1072-1096`; `sheet_screen_test.dart:2160` (both `onPressed` null) + `:2273` (back/F5 no-op) | ✅ PASS |
| SVE-02 | Preview with 3 previews in 60 s | Preview stays disabled + preview-limit tooltip | `sheet_edit_provider_test.dart:1124-1128` (blocked, cooldown 60 s); `sheet_screen_test.dart:2224` `sheetPreviewLimitTooltip` | ✅ PASS |
| SVE-03 | Save with 3 saves in 60 s | Save stays disabled + save-limit tooltip | `sheet_edit_provider_test.dart:1174-1176`; `sheet_screen_test.dart:2260` `sheetSaveLimitTooltip` | ✅ PASS |
| SVE-04 | rolling window frees up | re-enable when otherwise allowed | `test/sheet/action_rate_limiter_test.dart:34` + `:43`; `sheet_edit_provider_test.dart:1138-1147` (advance clock + `cooldown.fire()` → `canPreview isTrue`) | ✅ PASS |
| SVE-05 | count preview and save separately | independent 60 s windows | `action_rate_limiter_test.dart:89-91`; `sheet_edit_provider_test.dart:1217-1224` | ✅ PASS |
| SVE-06 | save issued | recorded in the save window | `sheet_edit_provider_test.dart:1174` (3 PATCHes) → `:1175` `isSaveBlocked isTrue` | ✅ PASS |
| SVE-07 | PATCH only on explicit Save | never automatic | `sheet_edit_provider_test.dart:655-668` (no change → no PATCH), `:875` (preview → no PATCH), `:264-265` (edits issue no request beyond detail GET) | ✅ PASS |

### P1: Unsaved drafts survive a restart (DRB-01…14)

| ID | Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| -- | ------------------------- | -------------------- | ----------------------- | ------ |
| DRB-01 | sheet has pending edits | write backup every 60 s | `draft_backup_scheduler_test.dart:12` (`defaultInterval == 60s`), `:33-36`; `draft_auto_backup_test.dart:104-106`; `sheet_screen_test.dart:2366` | ✅ PASS |
| DRB-02 | transitions no-pending → pending | start periodic backup | `draft_auto_backup_test.dart:100-101` (`running`, `startCalls == 1`), `:179` (exactly one start per transition) | ✅ PASS |
| DRB-03 | no pending edits | stop periodic backup | `draft_auto_backup_test.dart:129-133`, `:156` (`running isFalse`) | ✅ PASS |
| DRB-04 | write backup | atomic replace; prior backup intact on failure | `draft_backup_store_test.dart:91-96` (`DraftBackupException` + prior content equal); store temp+rename `draft_backup_store.dart:104-105` | ✅ PASS |
| DRB-05 | persist draft | only the pending editable delta, never derived | `draft_auto_backup_test.dart:104-106` (file == `{stats:{dexterity:18}}`); `draft_backup_store_test.dart:33` round-trip; `pendingDelta` built from `editedPaths` only (`sheet_edit_provider.dart:109-118`) | ✅ PASS |
| DRB-06 | open sheet with backup | seed edited state from stored sheet overlaid with delta; restored fields pending | `sheet_edit_provider_test.dart:1246-1265` (`stats.dexterity` 18, `isPending isTrue`, delta `{stats:{dexterity:18}}`) | ✅ PASS |
| DRB-07 | restoring a draft | overlay only editable leaves of the template | `sheet_edit_provider_test.dart:1247-1251` (derived not overlaid), `:1252-1256` (unknown key ignored) | ✅ PASS |
| DRB-08 | save succeeds | delete that sheet's backup | `sheet_edit_provider_test.dart:1035` `draftFile().existsSync(), isFalse` | ✅ PASS |
| DRB-09 | confirmed discard on leave or refresh | delete backup | `sheet_screen_test.dart:2399` → `:2423` (leave), `:2456` → `:2476` (refresh); cancel keeps `:2451`; clean sheet `:2525` | ✅ PASS |
| DRB-10 | `NOT_FOUND` + eviction | delete backup | `sheet_open_provider_test.dart:246-264` `draftFile('sheet-1').existsSync(), isFalse`; failed delete still publishes `:267-285`; successful load leaves it `:288-304` | ✅ PASS |
| DRB-11 | library entry removed | delete backup | `library_section_test.dart:246` (removed), failed removal keeps `:278`, cancel keeps `:304` | ✅ PASS |
| DRB-12 | backup cannot be read/decoded | ignore; seed from stored sheet | `draft_backup_store_test.dart:59`, `:67`, `:70`, `:82`; `sheet_edit_provider_test.dart:1280-1283` | ✅ PASS |
| DRB-13 | backup write fails | keep accepting edits, no error | `draft_auto_backup_test.dart:200-207` (`scheduler.tick` returns normally, `isPending isTrue`) | ✅ PASS |
| DRB-14 | store backups | under the injected app-support directory | `draft_backup_store_test.dart:40` (`<root>/drafts/sheet-1.json` exists); `draft_backup_store.dart:51` | ✅ PASS |

**Status**: ✅ 49/49 requirements have a located `file:line` + assertion; **0 open gaps** — both round-1 spec-precision gaps are now closed with dedicated tests (PRV-05 rebuild-granularity, edge case 8 below).

---

## Edge Cases

- [x] Nested cascade merges every nested key without flattening — `sheet_edit_provider_test.dart:885-889` (`modifiers.dexterityMod` merged).
- [x] Empty cascade keeps pending edits and rendered values — `sheet_edit_provider_test.dart:930-933`.
- [x] Edit made while a preview is in flight is merged without discarding the newer edit — `sheet_edit_provider_test.dart:995-1004`.
- [x] Rate-limit window holds exactly three entries → extra activation issues no request — `sheet_edit_provider_test.dart:1120-1136`; `sheet_screen_test.dart:2220-2232`.
- [x] Draft key no longer an editable leaf is ignored on restore — `sheet_edit_provider_test.dart:1252-1256`.
- [x] Derived draft key is never overlaid — `sheet_edit_provider_test.dart:1247-1251`.
- [x] Preview while sheet loading renders no action → no request — `sheet_screen_test.dart:2316`.
- [x] Preview fails with `NOT_FOUND`: keep edits, render mapped copy, without evicting the local entry — **round-1 gap closed** by `sheet_edit_provider_test.dart:975-1012`: `:1001` `after.isPending, isTrue` (edits kept), `:1002` `valueAtPath(after.data, 'stats.dexterity'), 18`, `:1003` `after.previewError, errorCopy(ApiErrorKind.notFound)` (mapped copy), `:1005-1010` `entries, hasLength(1)` with `reason: 'a failed preview must never evict the local entry'` and `entries.single.id, sheetId` (no eviction). Production `preview()` never calls `_evict` (`sheet_edit_provider.dart:295-303`).
- [x] Save attempted while a preview is in flight → no `PATCH` — `sheet_edit_provider_test.dart:1045-1065`; `sheet_screen_test.dart:2160-2195`.
- [x] App closed after a backup was written, sheet reopened → delta restored — `sheet_edit_provider_test.dart:1235-1268` (fresh container reads the persisted file).

---

## Discrimination Sensor

Scratch: temporary `git worktree` at HEAD (`/tmp/opencode/sensor-r2`, detached at `ed7900c`). Real-tree `git status --porcelain` captured before sensor work (3 entries: modified `.specs/LESSONS.md`, modified `.specs/lessons.json`, untracked `validation.md`) and re-confirmed byte-identical after cleanup. Scratch had zero tracked changes after each revert and was removed. No `git stash` used.

### Round 1 — 9 behavior-level mutations

| # | Mutation | File:line (real tree) | Description | Killed? |
| - | -------- | --------------------- | ----------- | ------- |
| 1 | Preview persists | `internal/sheet/service.go:237` | `Preview` calls `repository.PartialUpdate` before returning the cascade | ✅ Killed (4 `Service_Preview` subtests + `PreviewTemplate` seed test + HTTP `PreviewSheet` + `Integration_Sheets_Preview`) |
| 2 | Wrong `apperr` code | `internal/sheet/service.go:335` | Unknown-field maps to `CodeInvalidRequest` instead of `CodeUnknownField` | ✅ Killed (preview + PATCH + template + integration cases) |
| 3 | Broken template seed | `internal/sheet/service.go:411` (`buildNestedAllFields`) | Seed returns an empty map (drop field seeding) | ✅ Killed (`PreviewTemplate` happy + seed-equals-Create, plus `Create` cases) |
| 4 | Preview advances baseline | `desktop/lib/sheet/sheet_edit_provider.dart:287` | Preview success also merges delta into `defaults` | ✅ Killed (3 failed: baseline/pending assertions) |
| 5 | Broken restore filter | `desktop/lib/sheet/sheet_edit_provider.dart:388` | `_restoreDraft` overlays every key, not only editable leaves | ✅ Killed (derived `armorClass` 99 overlaid vs expected 0.0) |
| 6 | Draft not deleted on save | `desktop/lib/sheet/sheet_edit_provider.dart:366` | Drop `draftBackupStore.delete(sheetId)` in `_applySaved` | ✅ Killed (`draftFile().existsSync()` expected false, was true) |
| 7 | Single-flight broken | `desktop/lib/sheet/sheet_edit_provider.dart:134` | `canSave` drops `!isBusy` | ✅ Killed (a PATCH issued while a preview was in flight; 4 failures) |
| 8 | Rate-limit record dropped | `desktop/lib/sheet/sheet_edit_provider.dart:275` | Remove `_previewLimiter.record()` | ✅ Killed (4th preview not blocked; `isPreviewBlocked` false) |
| 9 | Auto-backup not activated | `desktop/lib/sheet/sheet_screen.dart:179` | Remove `ref.watch(draftAutoBackupProvider(...))` from the footer | ✅ Killed (`scheduler.running` false after an edit) |

**Round-1 result**: 9/9 killed, 0 survived.

### Round 2 — 2 targeted mutations (close the two flagged gaps)

| # | Mutation | File:line (real tree) | Description | Killed? |
| - | -------- | --------------------- | ----------- | ------- |
| 10 | Field controls watch whole edit state | `desktop/lib/sheet/sheet_screen.dart:523-525` (`_control`, PRV-05) | Replace `ref.watch(provider.select((state) => valueAtPath(state.data, field.path)))` with `valueAtPath(ref.watch(provider).data, field.path)` so every state emission rebuilds every field control | ✅ Killed — new Gap-1 widget test fails at `sheet_screen_test.dart:2154` (`identical(before, untouchedControl())` false) |
| 11 | Preview failure evicts the local entry | `desktop/lib/sheet/sheet_edit_provider.dart:295-303` (`preview()` catch, edge case 8) | Add `ref.read(sheetLibraryProvider.notifier).remove(sheetId)` in the preview failure handler | ✅ Killed — new Gap-2 test fails at `sheet_edit_provider_test.dart:1005` (`Expected: an object with length of <1>; Actual: []`) |

**Sensor depth**: expanded (feature guards data integrity: preview-never-persist, draft delta fidelity, single-flight/rate-limit drift, rebuild granularity, no-eviction-on-failure). Round 1: 9 behavior-level mutations (9 killed). Round 2: 2 targeted behavior-level mutations (2 killed). **Combined 11/11 killed, 0 survived.**

Both round-2 mutations revert cleanly; the scratch worktree is discarded and the real-tree porcelain matches the pre-sensor baseline exactly.

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code | ✅ `recalculate` extracted once (`service.go:172`), both previews reuse it; no duplicated DAG logic |
| Surgical changes | ✅ `PartialUpdate` keeps its contract; previews are additive; `ed7900c` adds only tests (0 deletions) |
| No scope creep | ✅ No new package (`pubspec.yaml`/`go.mod` untouched); template preview is spec-sanctioned API capability with no desktop consumer |
| Matches patterns | ✅ Go typed `apperr` + translators, mockery mocks; Dart Riverpod annotation + `select` rebuild granularity, l10n keys in `.arb` |
| Spec-anchored outcome check | ✅ All 49 asserted values match the spec; both round-1 gaps closed with dedicated tests |
| Per-layer Coverage Expectation | ✅ Domain (`internal/sheet` + edit controller) is 1:1 to ACs; routes (`internal/app`) cover happy + every mapped error code + envelope; widget layer covers footer flags/tooltips/discard/rebuild granularity |
| Every test maps to a spec requirement | ✅ No unclaimed tests; new tests reference PRV-05 and the preview-`NOT_FOUND` edge case |
| Documented guidelines followed | ✅ `AGENTS.md` (edge-first, typed errors, tests ship with code, English only), `internal/AGENTS.md` (layering/boundary translation), `desktop/AGENTS.md` (tooltip-explained disabled actions, tokens, reuse infrastructure, no new dependency) |

Notes: no new `// SPEC_DEVIATION` marker was added by this feature (the only marker in the repo, `desktop/lib/library/sheet_library_store.dart:56`, predates this diff under LGN-107). Go comments are doc-comments or non-obvious "why" notes consistent with the pre-existing file style. `ed7900c` introduces no production-code change, so the round-1 code-quality assessment carries over unchanged.

---

## Gate Check

- **Gate command (backend)**: `make fmt && make lint && make vet && go test ./...`
- **Gate command (desktop)**: `cd desktop && flutter analyze && flutter test`

### Backend

| Check | Result |
| ----- | ------ |
| `make fmt` | clean (no changes reported) |
| `make lint` | `0 issues.` |
| `make vet` | clean |
| `go test ./...` (MongoDB up, env from `.env`) | **133 top-level passed, 615 total PASS lines (incl. subtests), 0 failed, 0 skipped** |
| `go test ./...` (no `DATABASE_URI` / no MongoDB auth) | 121 top-level passed, 12 integration tests self-skip (as designed) — shows the 133 count is genuinely running the Mongo-backed tests |
| Baseline `8ae4252` (MongoDB up) | 126 top-level passed |
| Delta | **+7 top-level tests**; `ed7900c` is desktop-only, so the backend increment is unchanged from round 1; 0 test declarations deleted |

### Desktop

| Check | Result |
| ----- | ------ |
| `flutter analyze` | `No issues found!` |
| `flutter test` | **552 passed, 4 skipped, 0 failed** |
| Baseline `8ae4252` | 487 passed, 4 skipped, 0 failed |
| Round-1 HEAD (`02815aa`) | 550 passed, 4 skipped, 0 failed |
| Delta | **+65 from baseline**, of which **+2 from `ed7900c`** (the two gap-closing tests); 0 `test(`/`testWidgets(` declarations deleted |

- **Pre-existing flake**: `desktop/test/api/api_client_retry_test.dart` ("timeout and retry policy") is a known wall-clock flake (failed once in the pre-feature baseline, passed on re-run). It did **not** fail in this round's full run, so no re-run was required.
- **Skipped tests**: 4 desktop tests skipped (platform-dependent, unchanged from baseline); backend integration tests skip only when MongoDB is unavailable/unauthenticated.

---

## Requirement Traceability Update

Spec.md already lists every ID as `Verified`; the independent check confirms it.

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ---------- |
| DRY-01 … DRY-17 | Verified | ✅ Verified |
| PRV-01 … PRV-11 | Verified (PRV-05 rebuild sub-clause gap) | ✅ Verified (PRV-05 gap closed — `sheet_screen_test.dart:2124-2169`) |
| SVE-01 … SVE-07 | Verified | ✅ Verified |
| DRB-01 … DRB-14 | Verified | ✅ Verified |

---

## Summary

**Overall**: ✅ Ready (PASS) — 11/11 mutants killed (9 round-1 + 2 round-2 targeted), both gates green, every AC has located evidence, both round-1 gaps closed.

**Spec-anchored check**: 49/49 requirements matched spec outcome | 0 open gaps (2 flagged in round 1, now closed)
**Sensor**: round 1 9/9 + round 2 2/2 targeted killed
**Gate**: backend 133 top-level passed (615 total) / desktop 552 passed, 0 failed

**What works**: stateless sheet + template previews sharing the PATCH `recalculate` with zero writes; full typed-error parity; desktop Preview single-flight + independent 3-per-minute windows + change-only rebuild granularity; atomic per-sheet draft backups with filtered restore and cleanup on save/discard/eviction/removal; a preview `NOT_FOUND` keeps the draft and never evicts the local entry.

**Issues found**: none — round 1's two Minor hardening gaps were closed by `ed7900c` and independently confirmed by two fresh targeted mutants.

**Next steps**: none; the feature is verified.

**validate_state.py**: exit 0 (see below).
