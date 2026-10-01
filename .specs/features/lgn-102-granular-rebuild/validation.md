# lgn-102-granular-rebuild Validation

**Date**: 2026-10-01
**Spec**: `.specs/features/lgn-102-granular-rebuild/spec.md`
**Diff range**: `480946e^..34df4d4` (T1 `480946e` → T8 `34df4d4`, branch `feat/LGN-100-front-end-phase-3-sheet-preview`)
**Verifier**: independent sub-agent (author ≠ verifier)

**Verdict: PASS ✅** — 19/19 ACs traced to `file:line` assertions matching the spec-defined outcome, 7/7 sensor mutants killed, gate green (analyze: 0 issues; 326 passed, 4 skipped, 0 failed).

---

## Task Completion

| Task | Status  | Notes                                                    |
| ---- | ------- | -------------------------------------------------------- |
| T1   | ✅ Done | `sheet_edit_provider.dart` + 7 unit tests                 |
| T2   | ✅ Done | `controlBuilder` injection + 3 tests in `sheet_form_test` |
| T3   | ✅ Done | Screen wiring + sibling-identity tests (GRN-07/08)        |
| T4   | ✅ Done | Marker + leave guard + l10n strings                       |
| T5   | ✅ Done | `sheet_refresh.dart` + 4 unit tests                       |
| T6   | ✅ Done | F5 surface dispatch + 2 tests                             |
| T7   | ✅ Done | Refresh flow behind confirmation + edge tests             |
| T8   | ✅ Done | 200-field render-budget test under the user-approved amendment |

All 8 tasks committed in range; tasks.md Done-when boxes all checked.

---

## Spec-Anchored Acceptance Criteria

### P1: Isolated in-memory edit state

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| GRN-01: WHEN the edit state initializes THEN every leaf is seeded with its schema default | fixture defaults: `stats.dexterity`=10, `config.acUsesDex`=1, `derived.armorClass`=0 | `desktop/test/sheet/sheet_edit_provider_test.dart:79` — `expect(valueAtPath(seeded.data, 'stats.dexterity'), 10)`; `:80` `…, 1)`; `:81` `…, 0)`; `:82` `expect(seeded.isPending, isFalse)` | ✅ PASS |
| GRN-02: WHEN the user edits an editable control THEN the value is stored at its dotted path | `data['stats']['dexterity']` == 16 (and toggle stores 0) | `desktop/test/sheet/sheet_edit_provider_test.dart:104` — `expect(valueAtPath(after.data, 'stats.dexterity'), 16)`; `desktop/test/sheet/sheet_screen_test.dart:300` — `expect(valueAtPath(state.data, 'stats.dexterity'), 16)`; `sheet_screen_test.dart:468` — `expect(valueAtPath(state.data, 'config.acUsesDex'), 0)` | ✅ PASS |
| GRN-03: WHEN the user edits any field THEN no network request is issued | zero requests beyond the initial detail GET; no non-GET | `sheet_screen_test.dart:274` — `expect(recorded, hasLength(1))`; `:275` — `expect(recorded.single.method, 'GET')` (asserted after `enterText '42'` at `:259`) | ✅ PASS |
| GRN-04: WHILE mounted THEN edit state exists only in memory, never persisted or sent to the API | no request beyond detail GET; no persistence path exists | `sheet_edit_provider_test.dart:127` — `expect(recorded, hasLength(1))`; `:128` — `expect(recorded.every((request) => request.method == 'GET'), isTrue)` (after 3 `setPath` writes). Persistence: state lives only in `SheetEditState` (`desktop/lib/sheet/sheet_edit_provider.dart:21-76`) — no disk/API write path in the feature | ✅ PASS |
| GRN-05: WHILE `is_editable: false` THEN the control stays non-interactive | no input widget for `derived.armorClass`; read-only value display | `sheet_screen_test.dart:254` — `expect(find.byKey(const Key('sheet-input-derived.armorClass')), findsNothing)`; `:250` — `expect(find.byKey(const Key('sheet-value-derived.armorClass')), findsOneWidget)`; `:257` — `expect(find.text('Calculated'), findsOneWidget)`; control level `desktop/test/engine/field_control_test.dart:185` — `expect(find.byType(TextField), findsNothing)` | ✅ PASS |
| GRN-06: WHEN the user leaves THEN the state is discarded; reopening reseeds from defaults | dispose drops state; fresh subscription shows default 10 | `sheet_edit_provider_test.dart:243` — `expect(valueAtPath(fresh.data, 'stats.dexterity'), 10)`; `:244` — `expect(fresh.isPending, isFalse)`; screen level `sheet_screen_test.dart:601` — `expect(container.exists(sheetEditControllerProvider('fantasy_v1')), isFalse)` after confirm-leave | ✅ PASS |

### P1: Granular rebuild per field

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| GRN-07: WHEN one field's value changes THEN that field's control rebuilds | edited control's widget instance is NOT identical | `sheet_screen_test.dart:354` — `expect(identical(after['stats.dexterity'], before['stats.dexterity']), isFalse)`; toggle `:419-425` — `identical(afterSecond['config.acUsesDex'], afterFirst['config.acUsesDex']), isFalse` | ✅ PASS |
| GRN-08: WHEN one field's value changes THEN every other control keeps its instance | all sibling controls (and labels/section headers) `identical() == true` | `sheet_screen_test.dart:357-365` — loop `expect(identical(after[path], before[path]), isTrue, reason: '$path must not rebuild…')`; labels `:366-370`; section headers `:371-378`; second-edit `:411-418` and `:426-432`; toggle edge `:477-485` | ✅ PASS |
| GRN-09: WHEN the template detail arrives THEN controls reflect seeded values | input shows '10', switch shows `true` after load | `sheet_screen_test.dart:147-155` — `tester.widget<TextField>(…'sheet-input-stats.dexterity'…).controller?.text, '10'`; `:156-163` — `Switch…value, isTrue`; fresh-schema reseed after refresh `:762-771` — text `'16'` | ✅ PASS |

### P1: Render performance on a full-size template

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| GRN-10: WHEN the detail response for ≤200 fields arrives THEN all field controls render within the gate | **Amended (user decision 2026-10-01, spec.md:46 / spec.md:100)**: all 200 controls mounted + measured frame <4000 ms debug regression budget; <100 ms enforced in profile mode | `sheet_screen_test.dart:983` — `expect(find.byType(SheetFieldControl), findsNWidgets(200))`; `:984-993` — `expect(stopwatch.elapsedMilliseconds, lessThan(4000))` with the amendment cited in the inline comment `:986-990` | ✅ PASS (matches amended outcome exactly) |

Note: the <100 ms product budget is verified manually in profile mode per the recorded decision; spec.md's traceability row still reads `GRN-10 | Design | Pending` (spec.md:152) — recommend the orchestrator flip it to `Verified` under the amended gating, or keep `Pending` until the profile measurement is recorded.

### P1: Local-edit awareness and discard guards

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| GRN-11: WHEN state differs from defaults THEN the marker shows | marker absent before first edit, present after | `sheet_screen_test.dart:500` — `expect(find.byKey(const Key('sheet-local-edits')), findsNothing)`; `:508` — `expect(find.text('Local edits'), findsOneWidget)` | ✅ PASS |
| GRN-12: WHEN state equals defaults THEN the marker hides | marker gone after reverting to default; `isPending` false | `sheet_screen_test.dart:534` — `expect(find.byKey(const Key('sheet-local-edits')), findsNothing)`; `:538-541` — `…sheetEditControllerProvider('fantasy_v1')).isPending, isFalse)`; provider level `sheet_edit_provider_test.dart:156` | ✅ PASS |
| GRN-13: WHEN back is pressed with pending edits THEN a discard confirmation shows | `AlertDialog` with the exact discard message | `sheet_screen_test.dart:563` — `expect(find.byType(AlertDialog), findsOneWidget)`; `:564-569` — `expect(find.text('Your local edits on this sheet were never saved and will be discarded.'), findsOneWidget)` | ✅ PASS |
| GRN-14: IF the user confirms THEN discard the state and navigate back | dialog gone, screen popped, edit provider disposed | `sheet_screen_test.dart:595-597` — dialog `findsNothing`, `SheetScreen` `findsNothing`, `sheet-landing` `findsOneWidget`; `:601-605` — `expect(container.exists(sheetEditControllerProvider('fantasy_v1')), isFalse)` | ✅ PASS |
| GRN-15: IF the user cancels THEN keep the screen and its edit state | screen + marker stay; `isPending` still true; no request | `sheet_screen_test.dart:631` — `expect(find.byType(SheetScreen), findsOneWidget)`; `:636-639` — `…isPending, isTrue)`; `:640` — `expect(recorded, hasLength(1))` | ✅ PASS |
| GRN-16: WHEN back is pressed clean THEN navigate with no dialog | zero dialogs, screen popped | `sheet_screen_test.dart:656` — `expect(find.byType(AlertDialog), findsNothing)`; `:657-658` — `SheetScreen` `findsNothing` + landing `findsOneWidget` | ✅ PASS |
| GRN-17: WHEN F5 fires with no pending edits THEN refetch once and reseed from the fresh schema | 2 detail GETs total, form shows fresh default '16', marker hidden | `sheet_screen_test.dart:754` — `expect(detailGets, 2)`; `:755-761` — path filter `hasLength(2)` + `expect(recorded.every((request) => request.method == 'GET'), isTrue)`; `:762-771` — controller text `'16'`; `:772` — marker `findsNothing` | ✅ PASS |
| GRN-18: IF edits are pending on F5 THEN show the confirmation before refetching | dialog up, request count unchanged until confirm, state still pending | `sheet_screen_test.dart:801` — `expect(find.byType(AlertDialog), findsOneWidget)`; `:802-807` — exact message `'Your local edits will be discarded and the latest template will be loaded.'`; `:808-812` — `expect(detailGets, 1, reason: 'no new detail request until the user confirms')`; `:814-817` — `isPending, isTrue` | ✅ PASS |
| GRN-19: IF the refresh dialog is cancelled THEN no template detail request is issued | total requests stay at 1; edits kept | `sheet_screen_test.dart:844-848` — `expect(recorded, hasLength(1), reason: 'cancel must issue no template detail request')`; `:852-856` — `isPending, isTrue` | ✅ PASS |

**Status**: ✅ 19/19 ACs covered and matched to the spec-defined outcome | 0 spec-precision gaps

**Payload/conjunction rule**: all AC evidence asserts value/state, not merely call occurrence — exact dialog copy (`:566`, `:804`), request counts + method + path (`:755-761`, `:844-848`), stored values (`:300`, `:468`), widget identity (`:354`, `:357-365`), and provider existence (`:601-605`). No call-only assertions found in the AC evidence.

---

## Discrimination Sensor

Scratch: temporary `git worktree add /tmp/opencode/lgn102-sensor HEAD` (commit `34df4d4`); never `git stash`; real tree untouched. Each mutant was applied, run against the relevant test file, then reverted via `git checkout -- <file>`.

| # | Mutation (original `file:line`) | Description | Tests run | Killed? |
| - | ------------------------------- | ----------- | --------- | ------- |
| 1 | `desktop/lib/sheet/sheet_screen.dart:307-309` | Removed `select` — `_control` watches the whole edit state, so any edit rebuilds every control | `test/sheet/sheet_screen_test.dart` | ✅ Killed (3 failures: GRN-07/08 identity tests) |
| 2 | `desktop/lib/sheet/sheet_screen.dart:306` | Control emits a mutation request (`POST /sheets` via `createSheet(...).ignore()`) on build | `test/sheet/sheet_screen_test.dart` | ✅ Killed (14 failures incl. GRN-03/GRN-05 zero-mutation test, GRN-02) |
| 3 | `desktop/lib/sheet/sheet_screen.dart:160` | `if (pending)` → `if (pending && false)` — pending back skips the confirmation dialog | `test/sheet/sheet_screen_test.dart` | ✅ Killed (GRN-13 ×2, GRN-14, GRN-15) |
| 4 | `desktop/lib/sheet/sheet_screen.dart:167` | Dropped `!confirmed` from the leave guard — cancelling the dialog still navigates back | `test/sheet/sheet_screen_test.dart` | ✅ Killed (GRN-15, modal-barrier test) |
| 5 | `desktop/lib/shell/shortcut_registry.dart:49-52` | Removed the active-sheet branch — F5 always refreshes the catalog, never bumps the sheet tick | `test/shell/shortcut_registry_test.dart` | ✅ Killed ("F5 with a registered sheet…" GRN-17…19 test) |
| 6 | `desktop/lib/sheet/sheet_screen.dart:312` | Forced `isEditable: true` on every control — derived/read-only control becomes interactive | `test/sheet/sheet_screen_test.dart` (GRN-03/05 test) | ✅ Killed (`sheet-input-derived.armorClass` `findsNothing` fails) |
| 7 | `desktop/lib/sheet/sheet_edit_provider.dart:42-44` | `isPending => editedPaths.isNotEmpty` — pending no longer requires a value differing from defaults | `test/sheet/sheet_screen_test.dart` + `test/sheet/sheet_edit_provider_test.dart` | ✅ Killed (GRN-12 marker test + provider pending-flag test) |

**Sensor depth**: standard (7 manual behavior-level mutations; feature is not P0 — full mutation tooling not warranted)
**Result**: 7/7 killed — **PASS ✅** (0 survived)

**Isolation**: pre-sensor `git status --porcelain` = `?? .claude/skills/tlc-spec-driven/`, `?? .cursor/skills/tlc-spec-driven/`, `?? .windsurf/skills/tlc-spec-driven/`. After `git worktree remove --force`, porcelain is byte-identical to the baseline. Worktree removed.

---

## Interactive UAT Results

Not performed — the Verifier runs automated checks only; the flows (marker, leave dialog, F5) are exercised end-to-end by widget tests. Interactive UAT is the orchestrator/user's step if desired.

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code | ✅ — one provider file, one seam file, screen wiring; no speculative abstraction (`SheetEditState` is the smallest state that supports pending-diff) |
| Surgical changes | ✅ — diff touches only the 7 production files + l10n + tests named in tasks.md; nothing unrelated rewritten |
| No scope creep | ✅ — Out-of-scope table honored: no PATCH, no Save, no persistence, no app-close guard (grep confirms zero mutation requests in the diff) |
| Matches patterns | ✅ — Riverpod codegen (`.g.dart` via build_runner), `shared/json`, l10n via `.arb`, `desktop/AGENTS.md` destructive-dialog pattern (explicit consequence, explicit buttons), table-driven `t.Run`-style `t.run` prefixes `Happy path:`/`Edge:`/`Error:` |
| Spec-anchored outcome check (asserted values match spec) | ✅ — see AC table; GRN-10 asserted against the amended outcome, not the pre-amendment wording |
| Per-layer Coverage Expectation met | ✅ — provider unit tests 1:1 with GRN-01…06; screen widget tests cover GRN-02…19 happy + edge + error; shell routing test for F5; engine builder tests for T2 |
| Every test maps to a spec AC / edge case / Done-when — no unclaimed tests | ✅ — all 52 tests in the 5 changed files carry AC/RND tags in name or group; refresh-failure and registration tests map to spec edge cases + T7 Done-when |
| Documented guidelines followed | ✅ — root `AGENTS.md` (Testing: edge cases first, case-name prefixes, race-free tests), `desktop/AGENTS.md` (English strings, tokens, reuse existing infra, destructive confirmations), `.github/workflows/ci-flutter.yml` gates |

**Test integrity**: no test deletions. Two `-` lines in the diff are reformatting of tests that still exist (`sheet_screen_test.dart:203`, `shortcut_registry_test.dart:97`). One deliberate amendment: the stale pre-LGN-102 assertion `enabled == false` / `onChanged == null` was replaced by `enabled == true` with zero-mutation still asserted — recorded in tasks.md T3 Done-when ("stale … test is amended to the spec outcome") and required by the new spec (GRN-02/03 make controls editable, GRN-05 keeps derived read-only). Not a weakening.

---

## Edge Cases

- [x] Numeric unparseable draft restores stored value on blur: `desktop/test/engine/field_control_test.dart:444-464` — text `'-'` → unfocus → `controller?.text, '16'`.
- [x] Detail request failure → mapped error + Retry, no edit state created: `sheet_screen_test.dart:171-198` (error copy + Retry + re-request); state stays empty because `template == null` returns `SheetEditState()` (`desktop/lib/sheet/sheet_edit_provider.dart:60`), same branch asserted empty at `sheet_edit_provider_test.dart:75`. Note: no direct error-path assertion of state emptiness — covered by the shared null-template branch.
- [x] No fields → empty-state line, no edit state: `sheet_screen_test.dart:203-213` — `'This template has no fields.'` `findsOneWidget`. Note: `seedDefaults({})` yields empty data; no direct state assertion — covered by the shared seed path.
- [x] Different template → independent state (family isolation): `sheet_edit_provider_test.dart:161-187`.
- [x] Toggle flip displays new state with no sibling rebuild: `sheet_screen_test.dart:436-487`.
- [x] Marker hides when the last pending edit reverts: `sheet_screen_test.dart:512-543`.
- [x] Confirmed refresh failure → mapped error + Retry, state stays discarded, reseeds after success: `sheet_screen_test.dart:912-966` (`:945-949` `isPending, isFalse`; `:956-964` reseeded `'10'`).
- [x] Open dialog blocks further back/refresh: back blocked `sheet_screen_test.dart:663-695`; double F5 bump while confirming yields one dialog and zero extra requests `:797-813`.

---

## Gate Check

- **Gate command**: `cd desktop && flutter analyze && flutter test` (tasks.md Build gate)
- **Result**: analyze → `No issues found! (2.9s)`; tests → **326 passed, 4 skipped, 0 failed** (`00:22 +326 ~4: All tests passed!`)
- **Test count before feature**: 17 declarations across the 5 changed test files (screen 5, form 8, shortcut 4, provider 0, refresh 0) → full suite ≈ 295
- **Test count after feature**: 52 declarations across the same files (screen 24, form 11, shortcut 6, provider 7, refresh 4) → 330 total (326 + 4 skips)
- **Delta**: **+35 new tests**, 0 deleted
- **Skipped tests**: 4 — all in `desktop/test/contract/live_contract_test.dart:120,149,175,194`, `skip: contractSkip(base)`; pre-existing live-contract tests that self-skip without a running backend. Not introduced or touched by this feature.
- **Failures**: none

---

## Fix Plans (if issues found)

None — no failing AC, no surviving mutant, no spec-precision gap.

---

## Requirement Traceability Update

Verifier writes only `validation.md`; spec.md was not modified. Recommended status changes for the orchestrator:

| Requirement | Current (spec.md) | Recommended | Why |
| ----------- | ----------------- | ----------- | --- |
| GRN-01…GRN-09, GRN-11…GRN-19 | Execute / Verified | ✅ Verified (confirmed) | Evidence in AC table |
| GRN-10 | Design / Pending | ✅ Verified (amended gate) — or keep Pending until the profile-mode 100 ms measurement is recorded | Widget gate matches spec.md:46 amendment; profile check is manual by design |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 19/19 ACs matched the spec-defined outcome | 0 spec-precision gaps
**Sensor**: 7/7 mutations killed
**Gate**: 326 passed, 4 skipped (pre-existing live-contract self-skips), 0 failed

**What works**: per-sheet in-memory edit state seeded from schema defaults with zero network side effects; single-control rebuild proven by widget-identity assertions; 200-field render test holds the amended <4000 ms debug budget with all 200 controls mounted; local-edits marker flips on the pending diff; leave and F5 guards confirm, cancel, and discard exactly as specced, with F5 routed by surface through the shell registry.

**Issues found**: none blocking. Two notes: (1) two error/empty edge cases assert behavior via the shared null-template branch rather than a direct state assertion; (2) spec.md's GRN-10 traceability row still reads Pending under the amended gate — orchestrator's call to flip it.

**Next steps**: orchestrator surfaces this verdict; update spec.md traceability per the table above; interactive UAT optional for the user-facing dialog flows.
