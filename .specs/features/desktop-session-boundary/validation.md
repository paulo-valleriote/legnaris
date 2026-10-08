# Desktop Session Boundary Validation

**Date**: 2026-10-08
**Spec**: `.specs/features/desktop-session-boundary/spec.md`
**Diff range**: `31a7b32..9faaad8` (12 commits, b32cfd5..9faaad8)
**Verifier**: independent sub-agent (author ≠ verifier)

**Verdict: PASS ✅** — 18/18 ACs traced to a `file:line` assertion that targets the spec-defined outcome, gate green (`flutter analyze` clean, 706 passed / 4 skipped, `make lint-specs` clean), and the discrimination sensor killed 5/5 mutants. Two edge-case scenarios have indirect-only test evidence (noted below, neither is an AC).

---

## Task Completion

All 12 tasks are complete: 70/70 `Done when` boxes checked, 0 unchecked (`grep '^- \[ \]' tasks.md` → none).

| Task | Status | Notes |
| ---- | ------ | ----- |
| T1 Mount a session navigator in the auth gate | ✅ Done | `desktop/lib/auth/auth_gate.dart:60-65` branch `Navigator`, `:17-22` observer seam |
| T2 Host dialogs on the session navigator | ✅ Done | `useRootNavigator: false` at `catalog_screen.dart:225`, `library_section.dart:121`, `sheet_screen.dart:383` |
| T3 Extract the shared Screen 2 route builder | ✅ Done | `desktop/lib/sheet/sheet_route.dart:21-46`, adopted at `catalog_screen.dart:42` + `:235` |
| T4 Post-login sheet as a route above the catalog | ✅ Done | `authenticated_home.dart:72-75` push, `:52` shell-surface reset; spec-mandated assertion update confirmed in diff (`CatalogScreen findsNothing` → catalog beneath + 2 shells) |
| T5 Clear the sheet library at session end | ✅ Done | `sheet_library_provider.dart:101-110` |
| T6 Delete every draft backup at session end | ✅ Done | `draft_backup_store.dart:152-167` |
| T7 Sweep session-local data when the session ends | ✅ Done | `auth_session.dart:196` (forced), `:227` (logout), `:246-257` per-store `try/catch` |
| T8 At most one logout request per activation | ✅ Done | `auth_session.dart:207-219` single-flight |
| T9 Confirm logout while edits are pending | ✅ Done | `app_shell.dart:121-156`; 4 l10n keys with DATA-05 descriptions (`app_en.arb:297-313`) |
| T10 Guard local writes against a dead session | ✅ Done | `draft_auto_backup.dart:69`, `create_sheet_dialog.dart:141` |
| T11 Expose the session version to the transport | ✅ Done | `auth_session.dart:78`, bumps at `:117`, `:145`, `:192`, `:223` |
| T12 Drop auth responses from a finished session | ✅ Done | `authenticated_client.dart:45`, `:54`; `make lint-specs` clean (run by this verifier) |

---

## Spec-Anchored Acceptance Criteria

### P1: Navigation dies with the session

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| WHEN logout completes THEN only the auth screen renders, no route of that session still mounted (NAV-01) | `AuthScreen` present; `SheetScreen`/`CatalogScreen`/`AppShell` absent | `desktop/test/auth/auth_gate_test.dart:653-656` — `expect(find.byType(AuthScreen), findsOneWidget); expect(find.byType(SheetScreen), findsNothing); expect(find.byType(CatalogScreen), findsNothing); expect(find.byType(AppShell), findsNothing)` | ✅ PASS |
| WHEN a session becomes authenticated THEN empty navigation stack, no previous-session route (NAV-02) | After 2nd login: catalog mounted, no `SheetScreen`, exactly one `AppShell` | `desktop/test/auth/auth_gate_test.dart:681-683` — `expect(find.byType(CatalogScreen), findsOneWidget); expect(find.byType(SheetScreen), findsNothing); expect(find.byType(AppShell), findsNWidgets(1))`; shell-surface reset `:983-987` `expect(container.read(shellStateProvider), ShellSurface.catalog)` | ✅ PASS |
| WHEN a forced logout (rejected refresh) ends the session THEN every pushed route unmounts (NAV-03) | `AuthScreen` only; state `AuthUnauthenticated`; exactly one transport refresh | `desktop/test/auth/auth_gate_test.dart:623-636` — `expect(find.byType(AuthScreen), findsOneWidget)` … `expect(container.read(authSessionProvider), isA<AuthUnauthenticated>())` … `expect(refreshes, hasLength(refreshesBefore + 1))` | ✅ PASS |
| WHILE no session is authenticated THEN no catalog / sheet / creation-dialog surface (NAV-04) | All three absent while unauthenticated | `desktop/test/auth/auth_gate_test.dart:559-562` — `expect(find.byType(CatalogScreen), findsNothing); expect(find.byType(SheetScreen), findsNothing); expect(find.byType(AppShell), findsNothing)`; dialogs torn down `:701-705` (`CreateSheetDialog` absent) and `:766-769` | ✅ PASS |

### P1: The catalog is always reachable from Screen 2

| Criterion | Spec-defined outcome | `file:line` + assertion | Result |
| --------- | -------------------- | ----------------------- | ------ |
| WHEN `GET /sheets` responds `200` after authentication THEN Screen 2 opens as a route pushed above the catalog (NAV-05) | `SheetScreen` mounted **and** `CatalogScreen` still mounted beneath | `desktop/test/auth/auth_gate_test.dart:319-330` — `expect(find.byType(SheetScreen), findsOneWidget)` + `expect(find.byType(CatalogScreen, skipOffstage: false), findsOneWidget)` + `expect(find.byType(AppShell, skipOffstage: false), findsNWidgets(2))` | ✅ PASS |
| WHEN back is activated on the post-login Screen 2 THEN return to the catalog (NAV-06) | After back: no `SheetScreen`, catalog visible, one `AppShell` | `desktop/test/auth/auth_gate_test.dart:797-799` — `expect(find.byType(SheetScreen), findsNothing); expect(find.byType(CatalogScreen), findsOneWidget); expect(find.byType(AppShell), findsNWidgets(1))`; dirty path `:831-832` after `'Discard and leave'` | ✅ PASS |
| IF the resolution settles more than once THEN no second Screen 2 push (NAV-07) | Exactly one pushed Screen 2 route for the whole transition (observer count) | `desktop/test/auth/auth_gate_test.dart:874-879` — `expect(observer.pushed, hasLength(2), reason: 'exactly one Screen 2 route for the whole transition (NAV-07)')` (1 = initial route); 404 case `:900-903` `hasLength(1)` | ✅ PASS |
| The system SHALL keep F5 and Ctrl+S dispatching to the mounted Screen 2 (NAV-08) | F5 refreshes the sheet; `Ctrl+S` issues exactly one `PATCH /sheets/:id` | `desktop/test/auth/auth_gate_test.dart:928-960` — `expect(countPath(...'/templates/fantasy_v1'), detailLoads + 1)` and `expect(patches, hasLength(1))`, `expect(patches.single.url.path, '/sheets/sheet-1')` | ✅ PASS |

### P1: Session-local data does not survive the session

| Criterion | Spec-defined outcome | `file:line` + assertion | Result |
| --------- | -------------------- | ----------------------- | ------ |
| WHEN logout completes THEN library index cleared (memory + disk) and every draft backup deleted (DATA-01) | In-memory empty, `sheet_library.json` empty on disk, `drafts/` empty | `desktop/test/shell/logout_test.dart:186-192` — `expect(SheetLibraryStore(directory: result.support).load().entries, isEmpty)` + `expect(draftFiles(result.support), isEmpty)`; unit level `desktop/test/auth/auth_session_test.dart:541-547`, `desktop/test/library/sheet_library_provider_test.dart:276-287`, `desktop/test/sheet/draft_backup_store_test.dart:156-162` | ✅ PASS |
| WHEN a forced logout ends the session THEN same sweep (DATA-02) | Library empty on disk, drafts empty | `desktop/test/auth/auth_gate_test.dart:1020-1025` — `expect(SheetLibraryStore(directory: support).load().entries, isEmpty)` + `expect(draftFiles(support), isEmpty)`; unit `desktop/test/auth/auth_session_test.dart:563-566` | ✅ PASS |
| IF clearing the library or a draft backup fails THEN logout still completes (DATA-03) | State unauthenticated, token cleared, drafts still swept | `desktop/test/shell/logout_test.dart:212-220` — failing save store: `isA<AuthUnauthenticated>()` + `expect(draftFiles(result.support), isEmpty)`; unreachable stores `desktop/test/auth/auth_session_test.dart:587-589`; primitive swallows `desktop/test/library/sheet_library_provider_test.dart:296-306` (`returnsNormally`) and `desktop/test/sheet/draft_backup_store_test.dart:166-171`, `:190-194` | ✅ PASS |
| IF the app falls back to the auth screen after a transient restore failure THEN keep refresh token, library, drafts (DATA-04) | Token `'stored-refresh'` survives; 1 library entry; 1 draft | `desktop/test/auth/auth_gate_test.dart:1069-1079` — `expect(await container.read(secureTokenStoreProvider).read(), 'stored-refresh')` + `hasLength(1)` + `expect(draftFiles(support), hasLength(1))`; unit `desktop/test/auth/auth_session_test.dart:606-616` | ✅ PASS |
| WHEN logout is activated with pending edits THEN confirmation states unsaved edits will be discarded (DATA-05) | Dialog shown with the discard copy; only confirm ends the session | `desktop/test/shell/logout_confirm_test.dart:209-227` — `expect(find.byType(AlertDialog), findsOneWidget)` + `expect(find.text('Your unsaved edits will be discarded when you log out.'), findsOneWidget)` + confirm → `hasLength(1)` `/auth/logout` and files emptied | ✅ PASS |
| IF the user cancels (action, Esc, barrier) THEN session, edits, files kept and no `/auth/logout` (DATA-06) | Session `AuthAuthenticated`, token intact, pending edits intact, files intact, zero logout requests | `desktop/test/shell/logout_confirm_test.dart:247-265` (button), `:279-289` (Esc), `:303-313` (barrier) — each `expectNoLogoutRequest(app.auth)` (`:193-199`) + `expectLocalFilesKept` (`:182-191`) + `isPending, isTrue` | ✅ PASS |
| WHEN logout is activated without pending edits THEN session ends with no confirmation (DATA-07) | `AlertDialog` absent; one `/auth/logout`; unauthenticated | `desktop/test/shell/logout_confirm_test.dart:332-346` — `expect(find.byType(AlertDialog), findsNothing, reason: 'DATA-07…')` + `hasLength(1)` logout request | ✅ PASS |
| IF logout is activated again while the first is in flight THEN no second `/auth/logout` (DATA-08) | Exactly one request; both callers share the teardown; sequential activation issues a new one | `desktop/test/auth/auth_session_test.dart:344-358` — `expect(recorded…'/auth/logout').length, 1` + `store.token, isNull`; `:381-385` second activation after completion → `2` | ✅ PASS |

### P2: A finished session cannot disturb the current one

| Criterion | Spec-defined outcome | `file:line` + assertion | Result |
| --------- | -------------------- | ----------------------- | ------ |
| WHEN login/register completes THEN a refresh started before it is invalidated (SES-01) | Stale refresh returns `null`; new access token and new stored refresh token survive | `desktop/test/auth/auth_session_test.dart:640-643` — `expect(await refreshing, isNull)` + `expect(session.currentAccessToken(), 'access-1')` + `expect(store.token, 'refresh-1')`; bumps pinned `:685-698` (`0→1→2→3→4`) | ✅ PASS |
| IF a `401` from a previous session arrives after a new session authenticated THEN the new session's tokens are not cleared (SES-02) | No forced logout; `access-b` and `refresh-b` intact; no refresh, no replay | `desktop/test/api/authenticated_client_test.dart:343-347` — `expect(session.unauthorizedCalls, 0)` + `expect(session.accessToken, 'access-b')` + `expect(session.storedRefreshToken, 'refresh-b')` + `expect(recorded, hasLength(1))`; pre-refresh half `:314-317` (`refreshCalls, 0`) | ✅ PASS |

**Status**: ✅ All 18 ACs covered and asserted on the spec-defined outcome · 0 spec-precision gaps (every AC defines a concrete observable state; each assertion targets that exact value)

---

## Discrimination Sensor

Run in a scratch worktree (`git worktree add /tmp/opencode/dsb-scratch HEAD`), removed afterwards with `git worktree remove --force`. `git status --porcelain` of the real tree was empty before and after; matches baseline.

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| M1 | `desktop/lib/api/authenticated_client.dart:45-50` | Deleted the post-`401` session-version check (two-point guard, point 1) | ✅ Killed — `test/api/authenticated_client_test.dart` `+11 -1`, `Expected: <0> Actual: <1>` (unauthorizedCalls) |
| M2 | `desktop/lib/api/authenticated_client.dart:54-58` | Deleted the post-`null`-refresh session-version check (two-point guard, point 2) | ✅ Killed — same suite `+11 -1`, `Expected: <0> Actual: <1>` |
| M3 | `desktop/lib/auth/auth_session.dart:227` | Removed the `_clearSessionLocalData()` call from `_doLogout` (skip the sweep on explicit logout) | ✅ Killed — `test/shell/logout_test.dart` `+2 -2`, `Expected: empty Actual: [Instance of 'SheetLibraryEntry']` |
| M4 | `desktop/lib/auth/auth_session.dart:207-211` | Removed the single-flight early return in `logout()` (two activations → two requests) | ✅ Killed — `test/auth/auth_session_test.dart` `+24 -1`, `Expected: <1> Actual: <2>` |
| M5 | `desktop/lib/catalog/create_sheet_dialog.dart:141` | Un-guarded the writer upsert (`is AuthAuthenticated` → `if (true)`) | ✅ Killed — `test/catalog/create_sheet_dialog_test.dart` `+26 -1`, `Expected: empty Actual: [Instance of 'SheetLibraryEntry']` |

All five kills are assertion failures, not compile errors. Scratch removed, `git worktree list` shows no `dsb-scratch`, real-tree porcelain still empty.

**Sensor depth**: lightweight (5 injections, the maximum of the light tier; risk-weighted toward auth transport, sweep, single-flight, writer guard)
**Result**: 5/5 killed — PASS ✅

---

## Interactive UAT Results

Not performed — no interactive session available to this verifier. Automated gate + sensor stand in for it; the feature's user-facing flows (dirty-logout dialog, back-to-catalog, post-login open) are exercised as widget tests.

---

## Code Quality

Changed files reviewed: `auth_gate.dart`, `authenticated_home.dart`, `auth_session.dart`, `authenticated_client.dart`, `app_shell.dart`, `catalog_screen.dart`, `library_section.dart`, `sheet_screen.dart`, `sheet_route.dart`, `sheet_library_provider.dart`, `draft_backup_store.dart`, `draft_auto_backup.dart`, `create_sheet_dialog.dart`, `l10n/*`, plus 11 test files.

| Principle | Status |
| --------- | ------ |
| Minimum code (no features beyond what was asked) | ✅ Each change maps to a task; no extra surface (e.g. no state-listener cleanup, exactly what the design rejected) |
| Surgical changes (no unrelated "improvements") | ✅ Only `auth_gate_test.dart:324`'s pre-existing assertion was touched, and it is the spec-mandated NAV-05 reversal documented in `design.md` (Risks) — replaced with a *stronger* assertion (catalog beneath + 2 shells), not weakened |
| No unnecessary abstraction | ✅ `pushSheetScreen` (`sheet_route.dart:21`) has 3 real call sites (catalog library open, creation flow, post-login) — extraction rule met, moved verbatim |
| Only touched files required for task | ✅ Diff surface = 15 lib/l10n files + 11 test files + 2 spec docs; no drive-by refactors |
| Matches existing patterns/style | ✅ Best-effort swallow mirrors `draft_backup_store.delete`; single-flight mirrors `AuthenticatedClient._refreshOnce`; l10n keys follow the arb description convention |
| Spec-anchored outcome check (asserted values match spec) | ✅ 18/18 table above |
| Per-layer coverage expectation (domain 1:1 AC; widget happy+edge+error) | ✅ Every AC has a happy-path and/or an error/edge test; widget flows run through the real `AuthenticatedClient` + gate, not stubs |
| Every test maps to an AC, edge case, or Done-when — no unclaimed tests | ✅ New/changed tests: NAV-01…08 (`auth_gate_test`), DATA-01…04 (`auth_session_test`, `logout_test`, `sheet_library_provider_test`, `draft_backup_store_test`, `draft_auto_backup_test`, `create_sheet_dialog_test`), DATA-05…07 (`logout_confirm_test`), DATA-08 (`auth_session_test:323`), SES-01 (`auth_session_test:620`), SES-02 (`authenticated_client_test:301`); `draft_fixtures.dart` is a shared fixture; `create_sheet_navigation_test.dart` only adds the session override the guard requires |
| Test integrity (no weakened/deleted/skipped tests) | ✅ Diff contains zero removed `test(`/`testWidgets(` declarations, zero added `skip:`, one removed assertion replaced by a stronger spec-mandated one |
| Documented guidelines followed | ✅ `AGENTS.md` (test naming `Happy path:`/`Edge:`/`Error:` — all new tests conform; edge cases first; English only; comments explain *why* only), `desktop/AGENTS.md` (destructive confirmation rule → DATA-05 dialog; l10n via arb), `desktop/AGENTS.md` "no new package" (no dependency added) |

---

## Edge Cases

- [x] **Logout activated twice in the same frame → at most one `/auth/logout`** — `desktop/test/auth/auth_session_test.dart:344-348` (`length, 1`) and `:381-385` (guard clears after completion). Guard is set synchronously before the first `await` (`auth_session.dart:208-213`).
- [x] **Pending-edit confirmation open when the session otherwise ends (forced logout) → proceed without the dialog** — `desktop/test/auth/auth_gate_test.dart:733-741` (`AlertDialog` absent, session `AuthUnauthenticated`) and `desktop/test/shell/logout_confirm_test.dart:364-375` (no `/auth/logout` sent).
- [⚠️ Indirect evidence] **App killed without logout → persisted library and drafts kept (refresh token restores the same session)** — no test boots a stored session over seeded files. Adjacent evidence: the sweep runs only at `auth_session.dart:196` and `:227`; `desktop/test/auth/auth_gate_test.dart:1037-1048` proves the first-launch path never sweeps (1 library entry + 1 draft survive) and `:1074-1079` proves the stored-token transient path keeps both. The restore-*success* + files-kept case is not directly asserted. Handled in code; test depth is the gap.
- [⚠️ Partially injected] **Cleanup of one draft file fails → library still cleared and logout completes** — AC-level outcome covered: a store-level draft failure still completes logout (`desktop/test/auth/auth_session_test.dart:585-589`) and a failing library store never skips the drafts sweep (`desktop/test/shell/logout_test.dart:212-220`). The per-file swallow inside the loop (`draft_backup_store.dart:161-165`) is never exercised by an injected delete failure; the sibling edges (missing dir `:166`, subdirectory skip `:173`, path-as-file `:188` of the test file) are covered. Handled in code; no test forces one file's `deleteSync` to throw.

Neither ⚠️ is an AC gap: DATA-03's spec outcome (logout completes) is asserted at both store-failure levels, and the no-kill path is asserted for two of the three session-preserving launches.

---

## Gate Check

- **Gate command**: `cd desktop && flutter analyze && flutter test` (full) + `make lint-specs` (build)
- **Result**: analyze → `No issues found!` (exit 0) · tests → **706 passed, 0 failed, 4 skipped** (`All tests passed!`, exit 0) · `make lint-specs` → exit 0 (`check_test_assertions: clean`, `check_requirement_ids: clean (399 spec IDs, 23 decision IDs)`, `check_validation_citations: clean (31 changed files)`)
- **Test count before feature**: 661 passed, 4 skipped (measured by running the suite at `31a7b32` in the scratch worktree — matches the ~661 baseline)
- **Test count after feature**: 706 passed, 4 skipped
- **Delta**: **+45** new tests, **0** deleted, **0** weakened (verified by diff: no removed `test(` declarations, no added skips, one assertion replaced by a stronger spec-mandated one)
- **Skipped tests**: 4 — `desktop/test/contract/live_contract_test.dart:120,149,175,194`, `contractSkip(base)` self-skip when no `API_BASE_URL` is configured (FE1-35, pre-existing integration suite that requires the live backend). Justified; unrelated to this feature.
- **Failures**: none

---

## Fix Plans (if issues found)

None blocking. Two non-blocking test-depth observations for a future hardening pass (not routed as fix tasks — no AC is uncovered and no mutant survived):

1. Add a boot test that restores a stored session over a seeded library + draft and asserts both survive (spec edge case 3).
2. Force one draft file's deletion to fail inside `clearAll` and assert the others still go and the library ends empty (spec edge case 4, per-file branch of DATA-03).

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ---------- |
| NAV-01 … NAV-08 | Verified (author) | ✅ Verified (independent) |
| DATA-01 … DATA-08 | Verified (author) | ✅ Verified (independent) |
| SES-01, SES-02 | Verified (author) | ✅ Verified (independent) |

18/18 confirmed against `file:line` evidence in this pass; no requirement regresses.

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 18/18 ACs matched the spec-defined outcome · 0 spec-precision gaps
**Sensor**: 5/5 mutations killed
**Gate**: 706 passed, 0 failed, 4 skipped (pre-existing contract self-skips)

**What works**: session-owned navigation (`AuthGate` branch `Navigator`) kills every route on logout and forced logout; Screen 2 always opens above the catalog with a working back; the session-end sweep empties the library index and every draft on both definitive ends; the dirty-logout confirmation gates on tap-time pending edits with cancel/Esc/barrier keeping everything; single-flight logout issues one request; the session-version two-point guard drops stale `401`s and stale refreshes.

**Issues found**: two edge-case scenarios without direct test injection (see Edge Cases). Both are code-handled and AC-covered at the store level; they lower test depth, not correctness.

**Next steps**: feature is validated — no fix tasks required. Optional follow-ups above if the team wants the two edge cases directly injected.
