# Front End Fase 6 — Session Auth Validation

**Date**: 2026-10-08
**Spec**: `.specs/features/front-end-phase-6-auth/spec.md`
**Diff range**: `85b5bdd..170891d` (9 atomic commits: `ca3b9a2` T1 … `170891d` T9)
**Verifier**: independent sub-agent (author ≠ verifier)

---

## Task Completion

| Task | Status  | Notes |
| ---- | ------- | ----- |
| T1   | ✅ Done | `secure_token_store.dart` + provider + fake; tests cover write/read/clear and unavailable-storage fallback. |
| T2   | ✅ Done | `auth_api_client.dart` 4 endpoints; `X-Refresh-Token` asserted, no cookie. |
| T3   | ✅ Done | `auth_session.dart`; restore/login/register/logout/revoked/storage-failure + late-response guard tested. |
| T4   | ✅ Done | `authenticated_client.dart`; bearer scoping, refresh+replay, forced logout, single-flight tested. |
| T5   | ✅ Done | `getMySheet` 200/404/5xx tested; `apiClientProvider` wired over the decorator. |
| T6   | ✅ Done | `auth_screen.dart`; both modes, mapped error keeps fields, pending disables submit. |
| T7   | ✅ Done | `auth_gate.dart`; main mounts it; public health poll kept alive. |
| T8   | ✅ Done | `authenticated_home.dart`; 200 open / 404 create / 5xx Retry, one lookup per transition. |
| T9   | ✅ Done | `app_shell.dart` logout action; late-response guard tested. |

One task-text inaccuracy (not a code gap): T6 "Done when" says "strings go through l10n in **both locales**", but the app ships a single locale (`en`) — `AppLocalizations.supportedLocales == [Locale('en')]` (`desktop/lib/l10n/app_localizations.dart:95`). No pt locale exists, so nothing was missing; the wording over-claims.

---

## Spec-Anchored Acceptance Criteria

> The spec carries **23 AC lines** (7 + 6 + 5 story criteria + 5 edge criteria) but the Requirement Traceability table defines only **21 IDs** (FE6-01…FE6-21). Two story criteria share an ID with a sibling clause (FE6-09 covers "refresh once+replay" and "failed refresh → logout"; FE6-14 covers "authenticated → GET /sheets" and "200 → open Screen 2"). This is a bookkeeping under-count in `spec.md`, not a missing behavior; every criterion below has located evidence.

### P1: Session and token lifecycle

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| FE6-01 WHEN start with stored refresh token THEN exchange + restore without prompting | one `/auth/refresh` with the stored token, authenticated shell | `desktop/test/auth/auth_session_test.dart:95-118` - `expect(recorded.single.url.path, '/auth/refresh')`; `expect((state as AuthAuthenticated).accessToken, 'access-2')`. Gate: `desktop/test/auth/auth_gate_test.dart:122-135` - `expect(find.byType(AppShell), findsOneWidget)` | ✅ PASS |
| FE6-02 SHALL keep access token in memory only, persist refresh in OS secure storage | refresh token under the single secure key; access token held in memory | `desktop/test/auth/secure_token_store_test.dart:8-16` - `expect(storage.writtenKeys, [FlutterSecureTokenStore.defaultKey])`; `desktop/test/auth/auth_session_test.dart:60-62` - `expect(session.currentAccessToken(), 'access-1')`; `expect(store.token, 'refresh-1')` | ✅ PASS |
| FE6-03 WHEN valid credentials THEN persist refresh, hold access in memory, render app | `store.token == 'refresh-1'`, state `AuthAuthenticated` | `desktop/test/auth/auth_session_test.dart:44-64` - `expect(store.token, 'refresh-1')`; `expect(state, isA<AuthAuthenticated>())`. Screen: `desktop/test/auth/auth_screen_test.dart:79-103` | ✅ PASS |
| FE6-04 WHEN registers successfully THEN immediately log in and reach app | POST `/auth/register` then `/auth/login`, authenticated | `desktop/test/auth/auth_session_test.dart:66-93` - `expect(recorded.map(...).toList(), ['/auth/register', '/auth/login'])`. Screen: `desktop/test/auth/auth_screen_test.dart:122-165` | ✅ PASS |
| FE6-05 IF login/registration fails THEN mapped error and keep auth screen | mapped copy inline, state stays unauthenticated | `desktop/test/auth/auth_screen_test.dart:186-205` - `expect(find.text(errorCopy(ApiErrorKind.unknown)), findsOneWidget)`; `expect(sessionState(tester), isA<AuthUnauthenticated>())` | ✅ PASS |
| FE6-06 IF refresh rejected (family revoked) THEN clear stored token and return to auth | state `AuthUnauthenticated`, store null; gate maps that to `AuthScreen` | `desktop/test/auth/auth_session_test.dart:138-153` - `expect(store.token, isNull)`; `expect(container.read(authSessionProvider), isA<AuthUnauthenticated>())`. Screen mapping: `desktop/test/auth/auth_gate_test.dart:110-120` | ✅ PASS |
| FE6-07 WHEN logs out THEN send token to `/auth/logout`, clear storage, return to auth | logout header `refresh-1`, store null, `AuthScreen` | `desktop/test/shell/logout_test.dart:87-107` - `expect(logout.headers[AuthApiClient.refreshTokenHeader], 'refresh-1')`; `expect(result.store.token, isNull)`; `expect(find.byType(AuthScreen), findsOneWidget)`. Unit: `desktop/test/auth/auth_session_test.dart:232-257` | ✅ PASS |

### P1: Authenticated transport

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| FE6-08 SHALL attach `Authorization: Bearer <access>` to every protected request | exact header value on a protected call | `desktop/test/api/authenticated_client_test.dart:60-71` - `expect(recorded.single.headers['Authorization'], 'Bearer access-1')`. Wired: `desktop/test/api/api_client_provider_test.dart:42-54` - `expect(recorded.single.headers['Authorization'], 'Bearer access-1')` | ✅ PASS |
| FE6-09 IF protected 401 THEN refresh once and replay once | exactly one refresh, exactly two sends, replay carries new token | `desktop/test/api/authenticated_client_test.dart:104-126` - `expect(session.refreshCalls, 1)`; `expect(recorded, hasLength(2))`; `expect(recorded.last.headers['Authorization'], 'Bearer access-2')` | ✅ PASS |
| FE6-09 (failed-refresh clause) IF refresh fails THEN force logout and NOT replay | one refresh, one `onUnauthorized`, zero replays | `desktop/test/api/authenticated_client_test.dart:156-174` - `expect(session.unauthorizedCalls, 1)`; `expect(recorded, hasLength(1), reason: 'no replay after a failed refresh')` | ✅ PASS |
| FE6-10 SHALL send refresh token in `X-Refresh-Token` and NOT rely on cookies | header value `refresh-1` on refresh/logout; no `cookie` header | `desktop/test/api/auth_api_client_test.dart:92-111` - `expect(request.headers[AuthApiClient.refreshTokenHeader], 'refresh-1')`; `:131-151` - `isNot(contains('cookie'))` | ✅ PASS |
| FE6-11 IF several 401 while refresh in flight THEN await the same single refresh | exactly one refresh call across concurrent 401s | `desktop/test/api/authenticated_client_test.dart:176-201` - `expect(session.refreshCalls, 1, reason: 'single-flight refresh')`; responses `[200, 200]` | ✅ PASS |
| FE6-12 SHALL NOT attach auth header to `/auth/*` or `/health` | no `Authorization` key on public paths | `desktop/test/api/authenticated_client_test.dart:73-87` - `expect(request.headers.containsKey('Authorization'), isFalse)`; `:220-235` - public 401 not refreshed (`refreshCalls == 0`) | ✅ PASS |

### P1: Auth gate and single-sheet resolution

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| FE6-13 WHILE no session THEN render auth screen, NOT catalog/library/sheet | only `AuthScreen`; `AppShell` (which hosts all screens) absent | `desktop/test/auth/auth_gate_test.dart:110-120` - `expect(find.byType(AuthScreen), findsOneWidget)`; `expect(find.byType(AppShell), findsNothing)`; `expect(countPath(requests.domain, '/sheets'), 0)` | ✅ PASS |
| FE6-14 WHEN session becomes authenticated THEN request `GET /sheets` | exactly one `GET /sheets` | `desktop/test/auth/auth_gate_test.dart:269-287` - `expect(countPath(requests.domain, '/sheets'), 1)` | ✅ PASS |
| FE6-15 IF `GET /sheets` 404 THEN present create-sheet flow | `getMySheet()` returns null; gate renders `CatalogScreen` | `desktop/test/api/api_client_test.dart:180-191` - `expect(await client.getMySheet(), isNull)`; `desktop/test/auth/auth_gate_test.dart:289-303` - `expect(find.byType(CatalogScreen), findsOneWidget)` | ✅ PASS |
| FE6-14 (200 clause) WHEN `GET /sheets` 200 THEN open that sheet on Screen 2 | `SheetScreen.sheetId == 'sheet-1'` | `desktop/test/auth/auth_gate_test.dart:269-287` - `expect(tester.widget<SheetScreen>(...).sheetId, 'sheet-1')`; `expect(find.byType(CatalogScreen), findsNothing)` | ✅ PASS |
| FE6-16 IF health check runs while unauthenticated THEN reach `GET /health` and update indicator | boot health request while `AuthScreen` shown, then poll on tick | Reach: `desktop/test/auth/auth_gate_test.dart:228-252` - `expect(countPath(requests.domain, '/health'), 1)` then `2`. Indicator-update clause asserted indirectly: `desktop/test/connection/connection_provider_test.dart:96-118` - `expect(container.read(connectionProvider).status, ConnectionStatus.online)` after the poll | ⚠️ Spec-precision gap (see note) |

> **FE6-17 note (spec-precision gap).** The test is named "…updates the indicator while unauthenticated" but its assertions only count `/health` requests. The indicator lives in `AppShell`'s `Header`, which is not mounted while unauthenticated, so the clause is not directly observable there. The provider-state update is covered by `connection_provider_test.dart:96-118` and the indicator rendering by `connection_indicator_test.dart`. Outcome is covered by conjunction; the in-context assertion is imprecise. Flagged, not failed.

### Edge cases

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| FE6-17 IF secure storage unavailable THEN keep in-memory session, NOT persist | `AuthAuthenticated` this run; storage value stays null | `desktop/test/auth/auth_session_test.dart:215-228` - `expect(container.read(authSessionProvider), isA<AuthAuthenticated>())`; `expect(raw.value, isNull)`. Store: `desktop/test/auth/secure_token_store_test.dart:43-51` | ✅ PASS |
| FE6-18 WHEN access token expires mid-session THEN next protected request refreshes and replays, no user-visible error | 401 → 1 refresh → replay → 200 | `desktop/test/api/authenticated_client_test.dart:104-126` - `expect(response.statusCode, 200)`; `expect(session.refreshCalls, 1)` | ✅ PASS |
| FE6-19 IF logs out while a request is in flight THEN late response NOT restore session | late `refresh()` returns null; state stays unauthenticated; store null | `desktop/test/auth/auth_session_test.dart:275-299` - `expect(container.read(authSessionProvider), isA<AuthUnauthenticated>())`. Widget: `desktop/test/shell/logout_test.dart:109-151` - `expect(await late, isNull, reason: 'the version guard drops it')` | ✅ PASS |
| FE6-20 WHEN `GET /sheets` fails non-404 THEN mapped error with Retry, keep session | `errorCopy(serverError)` + Retry key; state stays `AuthAuthenticated` | `desktop/test/auth/auth_gate_test.dart:305-330` - `expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget)`; `expect(find.byKey(const Key('home-sheet-retry')), findsOneWidget)`; `isA<AuthAuthenticated>()`. Retry re-issues: `:332-355` | ✅ PASS |
| FE6-21 IF registration rejects input THEN mapped error and keep entered fields | `errorCopy(invalidEdit)` + field controllers retain text | `desktop/test/auth/auth_screen_test.dart:207-233` - `expect(find.text(errorCopy(ApiErrorKind.invalidEdit)), findsOneWidget)`; `expect(fieldText(tester, 'auth-email'), 'player@example.com')`; `'auth-username' == 'player'`; `'auth-password' == 'short'` | ✅ PASS |

**Status**: ✅ All criteria covered / ⚠️ 1 spec-precision gap flagged (FE6-16 indicator clause)

---

## Discrimination Sensor

Scratch `git worktree` at `HEAD` (`/tmp/opencode/lgn-fe6-verify`), mutations applied one at a time, targeted tests run, then reverted via `git checkout`. Sensor depth: **P0-full (auth)**.

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| M1 | `desktop/lib/api/authenticated_client.dart:84` | `_isPublic()` flipped to always `false` (token attached to `/auth/*` + `/health`) | ✅ Killed (`authenticated_client_test.dart` — "public path no Authorization" / public 401 not refreshed) |
| M2 | `desktop/lib/api/authenticated_client.dart:42` | Removed `await _session.onUnauthorized()` on failed refresh | ✅ Killed (`unauthorizedCalls` expected 1, actual 0) |
| M3 | `desktop/lib/api/authenticated_client.dart:62` | Removed `_refreshInFlight = future` (single-flight broken) | ✅ Killed (`refreshCalls` expected 1, actual 2) |
| M4 | `desktop/lib/api/authenticated_client.dart:31` | Removed bearer header assignment | ✅ Killed (`Authorization` expected `Bearer access-1`, actual null) |
| M5 | `desktop/lib/auth/auth_session.dart:127-129` | Removed `_version` guard in `refresh()` | ✅ Killed (late refresh after logout restored session; late returns non-null) |
| M6 | `desktop/lib/auth/auth_session.dart:151` | Removed `await _store.clear()` in `logout()` | ✅ Killed (`store.token` expected null, actual `refresh-1`) |
| M7 | `desktop/lib/auth/authenticated_home.dart:85-86` | Swapped 200/404 branches | ✅ Killed (`SheetScreen` not found for 200; `CatalogScreen` not found for 404) |
| M8 | `desktop/lib/auth/auth_screen.dart:74` | Clear email field on submit error | ✅ Killed (`fieldText('auth-email')` expected `player@example.com`, actual `''`) |
| M9 | `desktop/lib/auth/auth_screen.dart:240` | Submit enabled while pending (`pending ? _submit : null`) | ✅ Killed (submit fired while pending; request list wrong) |

**Sensor depth**: P0-full (≥5 mutations; 9 injected covering transport, session, sheet resolution, and screen)
**Result**: 9/9 killed — PASS ✅

**Isolation verified**: pre-sensor real-tree baseline `git status --porcelain` = `?? .specs/features/front-end-phase-6-auth/design.md`. Post-cleanup (worktree removed) identical. Real tree never mutated.

---

## Interactive UAT Results

Not performed. Automated widget/unit coverage plus the discrimination sensor are sufficient for this feature; UAT was not requested.

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code | ✅ No features beyond the spec; the only additive UI state (`_RestoringSession`) prevents a login-form flash and is justified in-code. |
| Surgical changes | ✅ `main.dart` swaps `home`; `app_shell.dart` Header gains one action; `api_client.dart` gains one method; existing endpoints untouched. |
| No scope creep | ✅ Local library left in place (spec §Out of Scope); no library deletion. |
| Matches patterns | ✅ Riverpod `@Riverpod(keepAlive: true)` codegen, `error_mapper`/`failureCopy`, l10n ARB, injected-transport seam. |
| Spec-anchored outcome check (asserted values match spec) | ✅ except the FE6-16 indicator clause (flagged). |
| Per-layer Coverage Expectation met (domain 1:1 ACs; routes happy+edge+error) | ✅ Matrix layers all have tests; unit layers 1:1 to ACs; gate covers 200/404/5xx/retry. |
| Every test maps to a spec requirement — no unclaimed tests | ✅ New tests carry FE6-* / story references; provider/boot/connection tests are pre-existing or wiring. |
| Documented guidelines followed | ✅ `desktop/AGENTS.md` (tokens, l10n, error_mapper, English), root `AGENTS.md` (edge-cases-first table tests, typed errors). New dependency `flutter_secure_storage` justified by TDD §V2 secure-storage requirement. |
| Would a senior engineer approve? | ✅ Clean separation (public auth client vs decorated domain client) breaks the session↔transport cycle; no obvious over-engineering. |

---

## Edge Cases

- [x] Secure storage unavailable: in-memory session, token not persisted — `auth_session_test.dart:215-228`, `secure_token_store_test.dart:43-51`.
- [x] Access token expires mid-session: transparent refresh + replay — `authenticated_client_test.dart:104-126`.
- [x] Logout vs in-flight request: version guard discards late response — `auth_session_test.dart:275-299`, `logout_test.dart:109-151`.
- [x] `GET /sheets` non-404 failure: mapped error + Retry, session kept — `auth_gate_test.dart:305-355`.
- [x] Registration rejection: mapped error, fields kept — `auth_screen_test.dart:207-233`.
- [x] Concurrent 401s: single refresh — `authenticated_client_test.dart:176-201`.

---

## Gate Check

- **Gate command**: `flutter analyze && flutter test` (from `desktop/`)
- **Result**: **656 passed, 0 failed, 4 skipped**
- **`flutter analyze`**: `No issues found!` (exit 0)
- **Test count before feature** (`85b5bdd`): 590 passed, 4 skipped
- **Test count after feature** (`170891d`): 656 passed, 4 skipped
- **Delta**: **+66 tests** (no tests deleted; no assertions weakened)
- **Skipped tests**: 4 × `desktop/test/contract/live_contract_test.dart` (`skip: contractSkip(base)`) — self-skip when no live backend is configured; pre-existing, unrelated to this feature.
- **Failures**: none. The known `desktop/test/api/api_client_retry_test.dart` timing flake (FE1-28/FE1-29) did **not** fire on this run; it is unrelated to this feature and untouched by the diff.

---

## Fix Plans

None blocking. One advisory follow-up (not required for PASS):

### Advisory 1: Assert the connection-indicator/provider state in the unauthenticated gate test

- **Root cause**: `auth_gate_test.dart:228-252` claims to verify the indicator updates but only asserts `/health` request counts.
- **Fix task**: In the gate test, after `scheduler.fire()`, assert `container.read(connectionProvider).status == ConnectionStatus.online` (or the rendered indicator label once mounted). Trivial, non-behavioral.
- **Priority**: Minor.

---

## Requirement Traceability Update

All 21 requirement IDs have located evidence and their spec-defined outcomes match (with the single precision note above). Statuses updated in `spec.md`.

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ---------- |
| FE6-01 … FE6-21 | Implementing | ✅ Verified |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 23/23 AC lines matched the spec outcome (21 requirement IDs; spec traceability under-assigns 2 IDs) | 1 spec-precision gap flagged (FE6-16 indicator clause)
**Sensor**: 9/9 mutations killed (P0-full)
**Gate**: 656 passed, 0 failed, 4 skipped

**What works**: Full session lifecycle (restore/login/register/logout/revoked-family), secure refresh-token storage with in-memory fallback, bearer-scoped transport with single-flight 401 refresh+replay, forced logout on failed refresh, auth gate with public health polling, `GET /sheets` resolution (200 open / 404 create / 5xx Retry), and shell logout with late-response guard. All spec outcomes carry value-level assertions; every injected mutant was detected.

**Issues found**: One minor spec-precision gap — the unauthenticated health test does not assert the connection-indicator/provider state it names (outcome covered elsewhere by conjunction). Advisory fix only.

**Next steps**: Optional minor test hardening for FE6-16; otherwise mark the feature done.
