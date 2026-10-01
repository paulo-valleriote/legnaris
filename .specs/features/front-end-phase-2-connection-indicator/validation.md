# front-end-phase-2-connection-indicator Validation

**Date**: 2026-09-30
**Spec**: `.specs/features/front-end-phase-2-connection-indicator/spec.md`
**Diff range**: `1ecd89d..7f0c7c5` (commits `bb4c010`, `c3d6afd`, `3bbba99`, `7f0c7c5`)
**Verifier**: independent sub-agent (author ≠ verifier)

**Verdict: PASS ✅** — 13/13 ACs matched to spec-defined outcomes, gate green (235/0/4), sensor 3/3 killed, real worktree untouched.

---

## Task Completion

| Task | Status     | Notes   |
| ---- | ---------- | ------- |
| T1   | ✅ Done    | All 9 Done-when boxes checked; commit `bb4c010` |
| T2   | ✅ Done    | All 7 Done-when boxes checked; commit `c3d6afd` |
| T3   | ✅ Done    | All 6 Done-when boxes checked; commit `3bbba99` |
| T4   | ✅ Done    | All 4 Done-when boxes checked; commit `7f0c7c5` |

No blocked or partial tasks. Declared test counts match the gate exactly: 18 (T1) + 5 (T2) + 4 (T3) + 6 (T4) = 33 new tests = 235 − 202 baseline.

---

## Spec-Anchored Acceptance Criteria

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| CONN-01: WHEN app starts THEN polling `GET /health` begins, new check every 15s | Boot check fires without a tick; schedule started once; interval = 15s; ticks drive the next check | `desktop/test/connection/connection_provider_test.dart:107-116` — `expect(scheduler.startCount, 1)`, `expect(healthCount(recorded), 1, reason: 'the boot check needs no tick')`, `expect(healthRequest.method, 'GET')`; `desktop/test/connection/health_scheduler_test.dart:9` — `expect(PeriodicHealthScheduler.defaultInterval, const Duration(seconds: 15))`; `connection_provider_test.dart:152` — `expect(healthCount(recorded), 2)` after `scheduler.fire()`; in-situ `desktop/test/app_boot_test.dart:92` — `expect(healthRequestCount(recorded), 1, reason: 'exactly one boot health check')` | ✅ PASS |
| CONN-02: WHILE no health response yet THEN checking state | Initial state `checking`, `lastSuccess` null, held until the response lands | `connection_provider_test.dart:73-74` — `expect(initial.status, ConnectionStatus.checking)` / `expect(initial.lastSuccess, isNull)`; hold at `:82-86` — status stays `checking` while gated; label `desktop/test/connection/connection_indicator_test.dart:66` — `expect(find.text(en.connectionChecking), findsOneWidget)` | ✅ PASS |
| CONN-03: WHEN check succeeds (2xx) THEN online | Success → `online` + timestamp; header renders Online | `connection_provider_test.dart:175-176` — `expect(state.status, ConnectionStatus.online, reason: label)` / `expect(state.lastSuccess, isNotNull)`; `connection_indicator_test.dart:79` — `expect(find.text(en.connectionOnline), findsOneWidget)` | ✅ PASS |
| CONN-04: WHEN check fails (transport after retry, or 4xx/5xx) THEN offline | 404/500 and transport failure → `offline`; header offline label in `scheme.error` | `connection_provider_test.dart:200-201` (404/500) — `expect(state.status, ConnectionStatus.offline)`; `:237-239` (transport) — `expect(state.status, ConnectionStatus.offline)` + `expect(healthCount(recorded), 2, reason: 'one retry per the client policy')`; `connection_indicator_test.dart:92-96` — offline text found, `expect(label.style?.color, scheme.error)` | ✅ PASS |
| CONN-05: WHILE check in flight, tick SHALL NOT start a second concurrent request | Ticks during flight discarded; guard re-arms after resolution | `connection_provider_test.dart:261-265` — `expect(healthCount(recorded), 1, reason: 'ticks during the flight are discarded (CONN-05)')`; re-arm `:278-282` — `expect(healthCount(recorded), 2, reason: 'the guard releases once the check resolves')` | ✅ PASS |
| CONN-06: WHEN state changes THEN header updates without user action and holds until a new check resolves | Label flips on scheduled check; current state holds while a check flies; offline→online recovers | `connection_indicator_test.dart:162-169` — after `scheduler.fire()` (no interaction): `expect(find.text(en.connectionOffline), findsOneWidget)` / `expect(find.text(en.connectionOnline), findsNothing)`; hold `connection_provider_test.dart:309-314` — `expect(inflight.status, ConnectionStatus.online)` + `expect(inflight.lastSuccess, timestamp)`; recovery `:431` — `expect(recovered.status, ConnectionStatus.online)` | ✅ PASS |
| CONN-07: Schedule time-based and independent of other API traffic — catalog/detail SHALL NOT start, stall or reset it | Catalog load/refresh issues no health request and never restarts the schedule; ticks still drive checks; timer cadence independent | `connection_provider_test.dart:388-389` — `expect(healthCount(recorded), 1, ...)` / `expect(scheduler.startCount, 1, reason: 'catalog load does not restart the schedule')`; `:392-394` (refresh: `listCount == 2`, `healthCount == 1`, `startCount == 1`); `:399-402` — schedule still drives the check; time-based cadence `health_scheduler_test.dart:26-30` (no tick before interval, one per interval). *Observation: the "detail request" sub-clause has no dedicated assertion — covered structurally (only `Connection.build` starts the scheduler; no file outside `lib/connection/` references `healthSchedulerProvider`/`connectionProvider`) and by proxy (dialog-open flow issues `GET /templates/:id` while offline in `connection_navigation_test.dart:170-186` with both surfaces unaffected). Not a failure; recorded as a coverage note.* | ✅ PASS (see note) |
| CONN-08: WHEN shell renders THEN indicator in top-right slot on every screen, localized 3 states | Indicator right-aligned in keyed slot of fixed width; checking/online/offline all localized; visible on every app surface | `connection_indicator_test.dart:135` — `expect(tester.getSize(slot).width, AppSizes.connectionSlotWidth)`; `:139` — `expect(indicatorRect.right, closeTo(slotRect.right, 0.01))`; labels `:66`, `:79`, `:92`; surfaces: `connection_navigation_test.dart:181-184` — indicator + offline label rendered with dialog open (app has exactly 2 surfaces: `ShellSurface { catalog, creationDialog }`, `desktop/lib/shell/shell_state.dart:8`) | ✅ PASS |
| CONN-09: WHEN right panel renders THEN Connection section with state + timestamp of last success, or "never" | Before 1st success: state + "never"; after: `Last check HH:mm` of `lastSuccess`; failure keeps timestamp and shows offline | `connection_detail_test.dart:86-90` — `expect(...find.text(en.connectionChecking)...)` + `expect(...find.text(en.connectionLastCheckNever)...)`; `:104` — `expect(find.text(lastCheckText(tester)), findsOneWidget)` (template with `DateFormat.Hm().format(lastSuccess)`); `:127-129` — offline + `expect(find.text(timestamp), findsOneWidget)` + `expect(find.text(en.connectionLastCheckNever), findsNothing)` | ✅ PASS |
| CONN-10: WHILE offline, app SHALL NOT block navigation, catalog actions or dialog actions | With offline shown: card tap opens dialog, Esc closes, F5 refreshes catalog exactly once | `connection_navigation_test.dart:193-202` — offline precondition then `expect(find.byType(CreateSheetDialog), findsOneWidget)` / `expect(currentSurface(tester), ShellSurface.creationDialog)`; `:216-221` — Esc closes, back to catalog; `:251-261` — `expect(listRequestCount(recorded), 2, reason: 'F5 re-requests the catalog exactly once')` + cards rendered | ✅ PASS |
| CONN-11: All visible strings through localization layer, no UI literals in widgets | Rendered labels equal l10n values; widget sources contain no string literals | Render: `connection_indicator_test.dart:66,79,92` and `connection_detail_test.dart:86-90,104` — `find.text(en.connection*)` (from `lookupAppLocalizations`); source: `desktop/lib/connection/connection_indicator.dart:23-25` and `connection_detail.dart:24-37` — every string via `localizations.*`; grep for `Text('/find.text('` literals in `desktop/lib/connection/` → none; keys at `desktop/lib/l10n/app_en.arb:50,54,58,62,71` | ✅ PASS |
| CONN-12: Test suite drives ticks through injectable scheduler — no test waits real 15s | Scheduler overridable; ticks fired on command; suite runs in seconds | `connection_provider_test.dart:49` — `healthSchedulerProvider.overrideWithValue(scheduler)`; `manual_health_scheduler.dart:25` — `void fire() => _onTick?.call()`; `health_scheduler_test.dart:21` — `PeriodicHealthScheduler(interval: tickInterval)` (20ms, not 15s); full suite (all suites) completes in ~18s per the gate run | ✅ PASS |
| CONN-13: WHEN health responds 2xx with payload other than `healthy` THEN still online | Any decoded 2xx payload → online | `connection_provider_test.dart:159-175` — case `('200 degraded', degradedBody)` → `expect(state.status, ConnectionStatus.online, reason: label)` | ✅ PASS |

**Status**: ✅ All 13 ACs covered and matched to spec-defined outcomes | 0 spec-precision gaps (every AC states a precise state/value; all test assertions target that value directly)

---

## Discrimination Sensor

Run in a scratch worktree at `/tmp/opencode/conn-sensor` (`git worktree add … HEAD`), never in the real tree. Each mutation reverted (`git checkout --`) before the next. Covering tests: `flutter test test/connection/` from `desktop/` in the scratch.

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| 1 | `desktop/lib/connection/connection_provider.dart:59` | Success branch writes `ConnectionStatus.offline` instead of `online` (flips CONN-03/13 outcome) | ✅ Killed — 12 tests failed (exit 1) |
| 2 | `desktop/lib/connection/connection_provider.dart:52` | Removed the single-flight guard `if (_inFlight) return;` | ✅ Killed — CONN-05 test failed (`connection_provider_test.dart:261`), 1 failure total |
| 3 | `desktop/lib/connection/connection_provider.dart:65` | Failure branch drops timestamp retention: `lastSuccess: state.lastSuccess` → `lastSuccess: null` | ✅ Killed — 3 tests failed (CONN-09 provider, CONN-09 detail, CONN-06 hold) |

**Sensor depth**: lightweight (3 mutations, highest-risk new code)
**Result**: 3/3 killed — PASS ✅

**Isolation**: `git status --porcelain` before sensor = after removal of the worktree, byte-identical:
`?? .claude/skills/tlc-spec-driven/`, `?? .cursor/skills/tlc-spec-driven/`, `?? .windsurf/skills/tlc-spec-driven/` (pre-existing untracked skill dirs, unrelated to this feature). Worktree removed with `git worktree remove --force`; real worktree never mutated.

---

## Interactive UAT Results (if performed)

⏭️ **Skipped** — verifier runs without a user channel; interactive UAT was not performed. Automated gate + sensor cover the observable behavior; the orchestrator may schedule UAT separately.

---

## Code Quality

| Principle        | Status |
| ---------------- | ------ |
| Minimum code     | ✅ — 4 files + 2 generated `.g.dart`; scheduler is 53 lines, no wrapper layers |
| Surgical changes | ✅ — shell diffs only fill the pre-reserved slots (`app_shell.dart`, `right_panel.dart`); adapted suites change counting granularity only (path-based counts, strictly more assertions than before) |
| No scope creep   | ✅ — every Out-of-Scope row respected: no retry/backoff, no lifecycle gating, no persistence, no degraded actions; only l10n keys consumed by these widgets were added |
| Matches patterns | ✅ — keepAlive + `Future.microtask` first check + `_inFlight` guard mirrors `catalog_provider.dart` (design.md line 45); test naming `Happy path:`/`Edge:`/`Error:` per root `AGENTS.md` |
| Spec-anchored outcome check (asserted values match spec) | ✅ — see AC table; every assertion targets the spec-defined state/value, not mere presence |
| Per-layer Coverage Expectation met (domain 1:1 ACs; routes happy+edge+error) | ✅ — provider unit layer maps 1:1 to CONN-01…07/12/13; widget layer covers happy + edge + error for all three states; navigation suite covers offline happy/edge paths |
| Every test maps to a spec requirement - no unclaimed tests | ✅ — 33 new tests, each names its CONN id(s); `manual_health_scheduler.dart` is a shared fixture (not a test); adapted boot/shell/theme tests map to T2 Done-when + CONN-01/12 in situ |
| Documented guidelines followed: root `AGENTS.md` (tests mandatory, edge-first, English, colocated), `desktop/AGENTS.md` (colors from `ColorScheme` only — offline = `error` at `connection_indicator.dart:30`; spacing tokens `AppSpacing.sm/xs`; strings English via arb; no new packages; connection status in right panel per Layout) | ✅ |

Test integrity: no test deleted or weakened. Existing assertions `expect(recorded, hasLength(1))` became per-path counts **plus** a new health-count assertion (stricter, e.g. `app_boot_test.dart:92`). Baseline 202 → 235 (+33), 0 removed.

---

## Edge Cases

- [x] API unreachable at start → offline after client fail-fast, recover to online on next success, no restart: transport failure → `connection_provider_test.dart:237` (after the client's retry, asserted at `:239`); recovery `connection_navigation_test.dart:148-166` (`healthRequestCount == 2`, "only the scheduled check, no restart"). Wall-clock ~10.3s composition is covered by the Phase-1 client's own tests (`api_client_retry_test.dart`, 5s timeout + 300ms retry) plus the 15s constant — deliberately no end-to-end 16s test per CONN-12.
- [x] Success while offline → online immediately: `connection_provider_test.dart:407-431`; in situ `connection_navigation_test.dart:153-164`.
- [x] 2xx body without `data.status` → parse failure → offline: `connection_provider_test.dart:205-216` (`missingStatusBody` → `expect(state.status, ConnectionStatus.offline)`).
- [x] Overlapping checks → only in-flight counts, no duplicate request: `connection_provider_test.dart:244-265`.
- [x] Dimension sweep (spec line 119): parse defensivo ✅ (above); idempotência/retry ✅ (single-flight `:261`, client retry `:239`); concorrência ✅ (CONN-05/07 rows); observabilidade ✅ (`lastSuccess` → panel, CONN-09 rows); state-transition integrity ✅ (checking→online CONN-02/03, offline→online CONN-06 recovery); auth/data-lifecycle N/A per spec (V1 no auth, no persisted state — nothing in the diff persists connection state).

---

## Gate Check

- **Gate command**: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && flutter gen-l10n && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test` (Build level, from tasks.md Gate Check Commands)
- **Result**: **235 passed, 0 failed, 4 skipped** — exit code 0
- **Test count before feature**: 202 (baseline stated for this feature; 4 skipped)
- **Test count after feature**: 235 (4 skipped)
- **Delta**: +33 new tests (18 T1 + 5 T2 + 4 T3 + 6 T4), 0 removed
- **Skipped tests**: 4 — `desktop/test/contract/live_contract_test.dart` (FE1-35/36 live scenarios against the real API): `Skip: No API base configured — set --dart-define=API_BASE_URL …`. Pre-existing baseline skips, unchanged by this feature, self-skip by design (root `AGENTS.md`: integration tests self-skip without external services).
- **Failures**: none.
- **Analyze**: `No issues found! (ran in 2.9s)`.
- Codegen: `build_runner` wrote 2 outputs (`connection_provider.g.dart`, `health_scheduler.g.dart` — committed); `gen-l10n` output committed; no dirty files produced in the real worktree (porcelain unchanged).

---

## Fix Plans (if issues found)

None. No failing AC, no surviving mutant, no spec-precision gap.

---

## Requirement Traceability Update

Update spec.md requirement statuses:

| Requirement | Previous Status | New Status   |
| ----------- | --------------- | ------------ |
| CONN-01 | Implementing | ✅ Verified  |
| CONN-02 | Implementing | ✅ Verified  |
| CONN-03 | Implementing | ✅ Verified  |
| CONN-04 | Implementing | ✅ Verified  |
| CONN-05 | Implementing | ✅ Verified  |
| CONN-06 | Implementing | ✅ Verified  |
| CONN-07 | Implementing | ✅ Verified  |
| CONN-08 | Implementing | ✅ Verified  |
| CONN-09 | Implementing | ✅ Verified  |
| CONN-10 | Implementing | ✅ Verified  |
| CONN-11 | Implementing | ✅ Verified  |
| CONN-12 | Implementing | ✅ Verified  |
| CONN-13 | Implementing | ✅ Verified  |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 13/13 ACs matched spec outcome | 0 spec-precision gaps
**Sensor**: 3/3 mutations killed
**Gate**: 235 passed, 0 failed, 4 skipped (pre-existing live-contract self-skips)

**What works**: boot health check + 15s schedule (CONN-01); checking/online/offline states end to end (CONN-02/03/04); single-flight under slow network (CONN-05); state holds and updates with no user action, offline→online recovery (CONN-06); schedule independent of catalog traffic (CONN-07); right-aligned header indicator on every surface (CONN-08); panel state + last-check timestamp / "never", preserved across failures (CONN-09); offline blocks nothing — dialog, Esc, F5 all work (CONN-10); zero UI literals (CONN-11); injectable scheduler, suite never waits 15s (CONN-12); any 2xx payload counts as online (CONN-13).

**Issues found**: none blocking. One coverage observation recorded in the AC table (CONN-07 "detail request" sub-clause is covered structurally and by proxy, not by a dedicated assertion) — noted for transparency, not routed as a fix task.

**Next steps**: orchestrator marks the feature Verified (traceability rows above) and may schedule interactive UAT separately if desired.
