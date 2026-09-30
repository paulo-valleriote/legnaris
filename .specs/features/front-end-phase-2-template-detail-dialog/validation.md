# front-end-phase-2-template-detail-dialog Validation

**Date**: 2026-09-30
**Spec**: `.specs/features/front-end-phase-2-template-detail-dialog/spec.md`
**Diff range**: `4710a38..HEAD` (branch `feat/LGN-97-front-end-phase-2-template-detail-dialog`)
**Verifier**: independent sub-agent (author ≠ verifier)

## Validation

**Result**: PASS

---

## Task Completion

| Task | Status | Notes |
| ---- | ------ | ----- |
| T1: Derive the field summary from the template tree | ✅ Done | `template_summary.dart` + 9 unit tests |
| T2: Add the template detail provider with on-demand fetch | ✅ Done | `template_detail_provider.dart` (+ committed `.g.dart`) + 11 unit tests |
| T3: Render the creation surface from the template detail | ✅ Done | `create_sheet_dialog.dart`, l10n keys, harness routing updated |
| T4: Show inline detail errors with retry and prove fresh reopen | ✅ Done | Error branch + lifecycle widget tests |

All four tasks are checked off in `tasks.md` and backed by committed code and tests.

---

## Spec-Anchored Acceptance Criteria

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| TPL-01 open surface → GET /templates/:id + loading until settled | exactly 1 GET to `/templates/:id`, `isLoading` true until resolve | `desktop/test/catalog/template_detail_provider_test.dart:80` - `expect(recorded, hasLength(1))`; `:82` - `expect(recorded.single.url.path, '/templates/fantasy_v1')`; `:83` - `expect(...isLoading, isTrue)`; `desktop/test/catalog/create_sheet_dialog_test.dart:125` - `expect(detailRequests, hasLength(1))`; `:126` - `expect(detailRequests.single.method, 'GET')` | ✅ PASS |
| TPL-02 success → name + one summary line per top-level key with leaf count | name shown; `baseArmorClass:1, config:2, stats:2` | `desktop/test/catalog/create_sheet_dialog_test.dart:107` - `expect(inDialog(find.text('Fantasy Template')), findsOneWidget)`; `:110-112` - `'baseArmorClass: 1'`, `'config: 2'`, `'stats: 2'`; `desktop/test/catalog/template_summary_test.dart:55` - fixture `[(baseArmorClass,1),(config,5),(derived,2),(modifiers,2),(others,1),(stats,2)]` | ✅ PASS |
| TPL-03 group counts leaves recursively; top-level leaf = 1 | nested group `outer` → 3; top-level leaf → 1 | `desktop/test/catalog/template_summary_test.dart:76` - `expect(summarizeFields(fields), [(prefix: 'outer', leafCount: 3)])`; `:87` - `expect(..., [(prefix: 'proficiencyBonus', leafCount: 1)])` | ✅ PASS |
| TPL-04 in-flight → loading, no stale summary from a previous template | new id starts `isLoading` true, `template` null; no summary while loading | `desktop/test/catalog/template_detail_provider_test.dart:130-132` - `fresh.isLoading isTrue`, `fresh.template isNull`; `desktop/test/catalog/create_sheet_dialog_test.dart:173` - `expect(inDialog(find.text('Loading template...')), findsOneWidget)`; `:174` - `expect(inDialog(find.textContaining('config:')), findsNothing)` | ✅ PASS |
| TPL-05 summary from FieldSchema only, no hardcoded prefixes | unexpected prefix renders unchanged | `desktop/test/catalog/template_summary_test.dart:104` - `expect(..., [(prefix:'attributeSet',1),(prefix:'zzz',1)])`; grep confirms zero `stats/modifiers/config/derived/baseArmorClass` literals in `desktop/lib/catalog/` | ✅ PASS |
| TPL-06 detail exposed to state layer decoded (id, name, fields intact) | id/name/type of nested leaf intact | `desktop/test/catalog/template_detail_provider_test.dart:107-113` - `template!.id == 'fantasy_v1'`, `name == 'Fantasy Template'`, `fields['stats']?.schema isNull`, `children['dexterity']?.schema?.type == 'number'` | ✅ PASS |
| TPL-07 transport/4xx/5xx fail → mapped message inline, dialog stays open | `errorCopy(serverError)` / `errorCopy(offline)`, dialog present | `desktop/test/catalog/template_detail_provider_test.dart:147` - `expect(state.errorMessage, errorCopy(ApiErrorKind.serverError))`; `:213` - `errorCopy(ApiErrorKind.offline)`; `desktop/test/catalog/create_sheet_dialog_test.dart:285-288` - dialog `findsOneWidget` + `errorCopy(serverError)` inline | ✅ PASS |
| TPL-08 Retry re-requests exactly once per activation | request count 1 → 2, recovers | `desktop/test/catalog/template_detail_provider_test.dart:246` - `expect(recorded, hasLength(2))`; `:268` - single-flight `expect(recorded, hasLength(1))`; `desktop/test/catalog/create_sheet_dialog_test.dart:360`/`:366` - `detailRequestCount` 1 → 2 | ✅ PASS |
| TPL-09 NOT_FOUND → mapped not-found inline, never closes/crashes | `errorCopy(notFound)`, dialog present | `desktop/test/catalog/template_detail_provider_test.dart:162` - `expect(state.errorMessage, errorCopy(ApiErrorKind.notFound))`; `desktop/test/catalog/create_sheet_dialog_test.dart:303-306` - dialog present + `errorCopy(notFound)` | ✅ PASS |
| TPL-10 2xx undecodable → parse failure inline, never half summary | `parseCopy()` and `template isNull` | `desktop/test/catalog/template_detail_provider_test.dart:177` - no `data` → `parseCopy()`, `:178` `template isNull`; `:193-194` - malformed shape → same; `desktop/test/catalog/create_sheet_dialog_test.dart:321` - `parseCopy()` inline; `:322` - `textContaining('config:') findsNothing` | ✅ PASS |
| TPL-11 surface open → empty sheet-name input with localized label + placeholder | `Sheet name` label, `e.g. My Character` hint, controller text `''` | `desktop/test/catalog/create_sheet_dialog_test.dart:190-195` - `find.text('Sheet name')`, `find.text('e.g. My Character')`, `controller?.text == ''` | ✅ PASS |
| TPL-12 primary action disabled + tooltip explaining later phase | `onPressed isNull`; tooltip message exact string | `desktop/test/catalog/create_sheet_dialog_test.dart:222` - `expect(button.onPressed, isNull)`; `:230` - `expect(tooltip.message, 'Sheet creation arrives in a later phase.')`; `:251-254` - hover reveals same copy | ✅ PASS |
| TPL-13 no POST/mutation; opening side-effect free | every request GET, no POST; typing emits nothing | `desktop/test/catalog/create_sheet_dialog_test.dart:264` - `expect(recorded.every((r) => r.method == 'GET'), isTrue)`; `:265` - `expect(recorded.any((r) => r.method == 'POST'), isFalse)`; `:209` - `expect(recorded.length, before)` after typing | ✅ PASS |
| TPL-14 dismiss (Esc/close) → closes, back to catalog, no new request | dialog gone, catalog surface, detail count unchanged | `desktop/test/catalog/create_sheet_dialog_test.dart:400-401` - `CreateSheetDialog findsNothing`, `detailRequestCount` unchanged; `desktop/test/catalog/create_sheet_navigation_test.dart:104-107` (Esc) and `:123-126` (Close) - dialog gone, `ShellSurface.catalog`, `listRequestCount == 1` | ✅ PASS |
| TPL-15 reopen same id → GET again, no cache | detail request count 2 after reopen | `desktop/test/catalog/create_sheet_dialog_test.dart:386` - `expect(detailRequestCount(recorded, 'fantasy_v1'), 2)` | ✅ PASS |

**Status**: ✅ All 15 ACs covered. 1 spec-precision gap flagged (see below), no AC gap.

### Spec-precision gap

- **Empty-fields edge (spec.md:132 vs spec.md:53)**: the spec's edge-case bullet says *"IF `fields` resolves to zero leaves THEN ... one localized line ('This template has no fields.')"* — a broad condition — while the assumptions table (spec.md:53) and the approved design narrow it to an **empty `fields` map**. The implementation follows the design: `desktop/lib/catalog/create_sheet_dialog.dart:147` returns the no-fields line only when `summarizeFields(...).isEmpty`, i.e. only for an empty top-level map. A non-empty map whose keys all resolve to zero leaves (e.g. `{'ghost': FieldNode()}`) renders `ghost: 0`, not the localized line. The test `desktop/test/catalog/create_sheet_dialog_test.dart:130-141` covers only the empty-map case. The spec wording ("zero leaves") should be reconciled with the design (empty map) or the implementation extended; this is a precision gap, not an AC failure.

---

## Discrimination Sensor

Scratch: temporary git worktree `/tmp/opencode/lgn97-sensor` at `HEAD` (`96031f8`); mutations applied only there; worktree removed with `git worktree remove --force`. Real worktree `git status --porcelain` matched the pre-sensor baseline before and after (untracked `.claude/`, `.cursor/`, `.windsurf/` skill dirs only; the one generated-file change caused by the mandated `build_runner` gate was restored with `git checkout`).

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| 1 | `desktop/lib/catalog/template_summary.dart:23` | Leaf branch `if (node.schema != null) return 1;` → `return 0;` | ✅ Killed (7 failures in `template_summary_test.dart`) |
| 2 | `desktop/lib/catalog/template_detail_provider.dart:41` | Removed auto-load scheduling `Future.microtask(load);` from `build` | ✅ Killed (11/11 failures in `template_detail_provider_test.dart`) |
| 3 | `desktop/lib/catalog/create_sheet_dialog.dart:155` | Summary line `${line.leafCount}` → `${line.leafCount + 1}` | ✅ Killed (3 failures in `create_sheet_dialog_test.dart`) |

**Sensor depth**: lightweight (3 targeted behavior-level mutations on the highest-risk new code)
**Result**: 3/3 killed - PASS ✅

---

## Interactive UAT Results

Not performed — automated widget/unit coverage is sufficient for this P1 feature; no human-judgment UI flow required beyond the hover assertion (covered programmatically).

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code | ✅ |
| Surgical changes | ✅ |
| No scope creep | ✅ |
| Matches patterns | ✅ |
| Spec-anchored outcome check (asserted values match spec) | ✅ (1 precision gap flagged) |
| Per-layer Coverage Expectation met (domain 1:1 ACs; widget covers happy+edge+error) | ✅ |
| Every test maps to a spec requirement — no unclaimed tests | ✅ |
| Documented guidelines followed: `AGENTS.md`, `desktop/AGENTS.md`, `.github/workflows/ci-flutter.yml`, `desktop/analysis_options.yaml` | ✅ |

Notes:
- `desktop/lib/catalog/` contains no hardcoded prefix literals (grep verified); all UI strings flow through `lib/l10n/app_en.arb`.
- The orphan key `createSheetTitle` was removed from `app_en.arb` (only a stale binary build cache still references the string).
- Generated provider is `isAutoDispose: true` (`template_detail_provider.g.dart:37`), matching the design decision that backs TPL-04/TPL-15.
- Observation (non-blocking): the mandated `dart run build_runner build --delete-conflicting-outputs` emitted `These options have been removed and were ignored: --delete-conflicting-outputs` (newer riverpod_generator dropped the flag), and regenerating changed only the `debugGetCreateSourceHash` literal in `template_detail_provider.g.dart` (`ae4b8a34…` → `2aaf2f03…`). The committed file is what analyze/test validate; the gate command in `tasks.md` is stale w.r.t. the flag name. No behavior impact.

---

## Edge Cases

- [x] `fields` empty → localized no-fields line: `create_sheet_dialog_test.dart:130-141`. *(Precision gap: non-empty tree with zero total leaves not covered — see above.)*
- [x] Top-level single-leaf key still renders its own line with count 1: `template_summary_test.dart:90-96`.
- [x] Two top-level keys sharing a leaf name count independently: `template_summary_test.dart:116-126`.
- [x] Empty template name renders layout without throwing: `create_sheet_dialog_test.dart:143-155`.
- [x] Nested group recursion: `template_summary_test.dart:66-78`.
- [x] Node with neither schema nor children counts 0: `template_summary_test.dart:110-114`.
- [x] Dispose during in-flight load does not throw: `template_detail_provider_test.dart:277-304`.

---

## Gate Check

- **Gate command**: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test`
- **Result**: 199 passed, 0 failed, 4 skipped
- **Test count before feature** (`4710a38`, measured in a scratch worktree): 162 passed / 4 skipped
- **Test count after feature** (`HEAD`): 199 passed / 4 skipped
- **Delta**: +37 new tests (T1 9 + T2 11 + T3/T4 17); harness edits in `create_sheet_navigation_test.dart` / `app_boot_test.dart` changed assertions only, adding/removing no tests
- **Skipped tests**: 4 in `test/contract/live_contract_test.dart:120,149,175,194` — pre-existing live-backend contract tests, self-skip without a running API; unrelated to this feature
- **Failures**: none; `flutter analyze` reported `No issues found!`

---

## Fix Plans (if issues found)

None blocking. Recommended (non-blocking) follow-up:

### Fix 1: Reconcile the empty-fields spec wording

- **Root cause**: spec.md:132 says "zero leaves"; spec.md:53 and design.md:89 narrow it to an empty `fields` map; implementation + test follow the narrow reading.
- **Fix task**: Clarify spec.md:132 to "empty `fields` map", or extend `create_sheet_dialog.dart` + add a test for a non-empty tree with zero total leaves.
- **Priority**: Minor

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ---------- |
| TPL-01 … TPL-15 | Implementing | ✅ Verified |

(All 15 requirements verified against the spec-defined outcomes; the empty-fields edge is a spec-precision clarification, not an unverified requirement.)

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 15/15 ACs matched spec outcome | 1 spec-precision gap flagged
**Sensor**: 3/3 mutations killed
**Gate**: 199 passed, 0 failed, 4 skipped (delta +37)

**What works**: On-demand `GET /templates/:id` on dialog open with loading state; recursive prefix summary with no hardcoded keys; decoded detail exposed via autoDispose family provider; inline mapped errors with single-flight Retry for transport/4xx/5xx/NOT_FOUND/parse failures; empty name-safe layout; empty-name/localized label + placeholder; disabled primary action with hover-revealed tooltip; zero mutations; fresh refetch on reopen.

**Issues found**: none blocking. One spec-precision gap (empty-fields wording).

**Next steps**: Optionally reconcile spec.md:132 with the approved design's empty-map reading (Fix 1, Minor). No code or test changes required for acceptance.
