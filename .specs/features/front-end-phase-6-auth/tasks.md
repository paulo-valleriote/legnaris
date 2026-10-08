# Front End Fase 6 — Session Auth Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/front-end-phase-6-auth/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: `desktop/AGENTS.md` (English only, reuse app infrastructure, tokens for visuals), `AGENTS.md` (edge cases first + happy path, mapped errors), `.github/workflows/ci-flutter.yml` (`flutter analyze`, `flutter test`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Token store | unit | Read/write/clear; storage-unavailable fallback; 1:1 to FE6-02, FE6-17 | `desktop/test/auth/secure_token_store_test.dart` | `flutter test test/auth/` |
| Auth client | unit | Login/register/refresh/logout success + mapped failure; `X-Refresh-Token`; 1:1 to FE6-03, FE6-05, FE6-07, FE6-10, FE6-21 | `desktop/test/api/auth_api_client_test.dart` | `flutter test test/api/` |
| Session notifier | unit | Restore, login, register, logout, revoked-family reset, storage failure; 1:1 to FE6-01…FE6-06, FE6-17 | `desktop/test/auth/auth_session_test.dart` | `flutter test test/auth/` |
| Transport decorator | unit | Bearer on protected, skipped on public, one refresh + one replay, single-flight, forced logout; 1:1 to FE6-08…FE6-12, FE6-18 | `desktop/test/api/authenticated_client_test.dart` | `flutter test test/api/` |
| Domain client | unit | `getMySheet` 200 → Sheet, 404 → null, 5xx → mapped error; 1:1 to FE6-15, FE6-20 | `desktop/test/api/api_client_test.dart` | `flutter test test/api/` |
| Auth screen | widget | Login/register modes, mapped error keeps fields, submit disables while pending; 1:1 to FE6-05, FE6-21 | `desktop/test/auth/auth_screen_test.dart` | `flutter test test/auth/` |
| Gate + home | widget | Auth-only when unauthenticated, one `GET /sheets`, open vs create, Retry on failure, logout; 1:1 to FE6-12…FE6-16, FE6-19, FE6-20 | `desktop/test/auth/auth_gate_test.dart` | `flutter test test/auth/` |
| l10n / codegen wiring | build | Generated strings and providers compile; analyze clean | `desktop/lib/l10n/`, `*.g.dart` | `flutter analyze` |

**Note on `Tests: none`:** no task here is `none`; every task creates or modifies a tested layer.

## Gate Check Commands

> Generated from codebase - confirm before Execute. All commands run from `desktop/`.

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | After tasks with unit/widget tests in one area | `flutter test test/<area>/` |
| Full | After tasks that regenerate l10n/codegen or wire cross-file behavior | `flutter test` |
| Build | At phase completion / final validation | `flutter analyze && flutter test` |

---

## Execution Plan

Phases are ordered and run sequentially - each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Storage and auth transport

```
T1
T2
```

### Phase 2: Session and decorated transport

```
T1 -> T3
T2 -> T3
T3 -> T4
T4 -> T5
```

### Phase 3: Auth gate UI

```
T3 -> T6
T6 -> T7
```

### Phase 4: Sheet resolution and logout

```
T5 -> T8
T7 -> T8
T7 -> T9
```

---

## Task Breakdown

### T1: Add the secure token store

**What**: Add `flutter_secure_storage` and a `SecureTokenStore` (read/write/clear) with an injectable provider and an in-memory test fake.
**Where**: `desktop/lib/auth/secure_token_store.dart`
**Depends on**: None
**Reuses**: the injected-provider convention from `appSupportDirectoryProvider`
**Requirement**: FE6-02, FE6-17

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `flutter_secure_storage` is added and justified in the commit body
- [x] The store persists only the refresh token; a storage failure is swallowed into an in-memory fallback
- [x] Unit tests cover write/read/clear and the unavailable-storage path
- [x] Gate check passes: `flutter test test/auth/`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(auth): add secure refresh-token store`

---

### T2: Add the public auth API client

**What**: A plain client for `POST /auth/register|login|refresh|logout` using `X-Refresh-Token`, with mapped errors.
**Where**: `desktop/lib/api/auth_api_client.dart`
**Depends on**: None
**Reuses**: `envelope.dart`, `error_mapper.dart`, `api_config.dart`
**Requirement**: FE6-03, FE6-05, FE6-07, FE6-10, FE6-21

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `register`, `login`, `refresh`, `logout` return the decoded `TokenPair`/user or a mapped domain error
- [x] `refresh`/`logout` send the token in the `X-Refresh-Token` header and never a cookie
- [x] Unit tests cover each endpoint's success and the mapped failure cases
- [x] Gate check passes: `flutter test test/api/`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(auth): add public auth API client`

---

### T3: Add the session notifier

**What**: `AuthSession` owns the in-memory access token and drives restore/login/register/logout/refresh against the store and the auth client.
**Where**: `desktop/lib/auth/auth_session.dart`
**Depends on**: T1, T2
**Reuses**: Riverpod `@Riverpod` async pattern
**Requirement**: FE6-01, FE6-03, FE6-04, FE6-06, FE6-10, FE6-17

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `restore()` refreshes from stored token; login/register persist the refresh token and hold the access token in memory
- [x] A revoked-family refresh clears storage and returns the unauthenticated state
- [x] Logout calls `/auth/logout`, clears storage, and resets state
- [x] Unit tests cover restore, login, register, logout, revoked refresh, and storage failure
- [x] Gate check passes: `flutter test test/auth/`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(auth): add auth session with restore and logout`

---

### T4: Add the authenticated transport decorator

**What**: An `http.BaseClient` decorator that adds `Authorization: Bearer`, skips public paths, and on `401` refreshes once (single-flight) and replays once.
**Where**: `desktop/lib/api/authenticated_client.dart`
**Depends on**: T3
**Reuses**: `http.BaseClient`
**Requirement**: FE6-08, FE6-09, FE6-11, FE6-12, FE6-18

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Protected requests carry the bearer header; `/auth/*` and `/health` do not
- [x] A `401` triggers exactly one refresh and one replay; a failed refresh forces logout and zero replays
- [x] Concurrent `401`s share a single refresh
- [x] Unit tests cover header scoping, refresh+replay, failed refresh, and single-flight
- [x] Gate check passes: `flutter test test/api/`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(auth): decorate transport with bearer and 401 refresh`

---

### T5: Add `getMySheet` and wire the domain client

**What**: Add `GET /sheets` to `ApiClient` (200 → `Sheet`, 404 → null) and wire `apiClientProvider` to build the decorator over the injected transport.
**Where**: `desktop/lib/api/api_client.dart`
**Depends on**: T4
**Reuses**: existing `_get`/`_decode`, `api_client_provider.dart`
**Requirement**: FE6-15

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `getMySheet()` maps `200` to a `Sheet` and `404` to `null`; other errors map as before
- [x] `apiClientProvider` wraps the transport in `AuthenticatedClient` with the session delegate
- [x] Unit tests cover `200`, `404`, and `5xx`
- [x] Gate check passes: `flutter test test/api/`

**Tests**: unit
**Gate**: quick

**Commit**: `feat(api): add getMySheet and authenticate the domain client`

---

### T6: Add the auth screen

**What**: A login/register screen with a mode switch, mapped error display, and pending-state handling.
**Where**: `desktop/lib/auth/auth_screen.dart`
**Depends on**: T3
**Reuses**: desktop form/footer patterns, theme tokens, `error_mapper`
**Requirement**: FE6-05, FE6-21

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Both modes submit through `AuthSession`; errors render the mapped message and keep the entered fields
- [x] The primary action disables while pending; strings go through l10n in both locales
- [x] Widget tests cover both modes, the mapped error, and the pending state
- [x] Gate check passes: `flutter test test/auth/`

**Tests**: widget
**Gate**: quick

**Commit**: `feat(auth): add login and register screen`

---

### T7: Add the auth gate and mount it

**What**: `AuthGate` renders `AuthScreen` while unauthenticated and the authenticated home otherwise; `main.dart` mounts it; existing tests override the session as authenticated.
**Where**: `desktop/lib/auth/auth_gate.dart`
**Depends on**: T6
**Reuses**: `main.dart` root, existing widget-test provider overrides
**Requirement**: FE6-12, FE6-16

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Unauthenticated → only the auth screen renders; authenticated → the app shell renders
- [ ] `main.dart` uses `AuthGate` as `home`; the public health indicator still polls
- [ ] Existing widget tests are updated to override the session so `flutter test` stays green
- [ ] Widget tests assert the gated render and the public health path
- [ ] Gate check passes: `flutter analyze && flutter test`

**Tests**: widget
**Gate**: build

**Commit**: `feat(auth): gate the app behind the session`

---

### T8: Resolve the user's sheet after login

**What**: After authentication, call `GET /sheets` and either open the sheet or present the create flow on `404`, with Retry on other failures.
**Where**: `desktop/lib/auth/authenticated_home.dart`
**Depends on**: T5, T7
**Reuses**: `SheetOpenController` (AD-005), `create_sheet_dialog.dart`
**Requirement**: FE6-13, FE6-14, FE6-15, FE6-20

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `200` opens Screen 2 for that sheet id; `404` shows the create flow; 5xx/offline shows Retry and keeps the session
- [ ] Exactly one `GET /sheets` per authentication transition
- [ ] Widget tests cover the three branches and the single-request rule
- [ ] Gate check passes: `flutter test test/auth/`

**Tests**: widget
**Gate**: quick

**Commit**: `feat(auth): resolve the owned sheet via GET /sheets`

---

### T9: Add the logout action

**What**: A logout action in the shell that ends the session and returns to the auth screen, discarding late responses.
**Where**: `desktop/lib/shell/app_shell.dart`
**Depends on**: T7
**Reuses**: the shell action/toolbar pattern, `AuthSession.logout`
**Requirement**: FE6-06, FE6-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Logout calls `/auth/logout`, clears storage, and the gate returns to the auth screen
- [ ] A late in-flight response after logout does not restore the session
- [ ] Widget tests cover logout and the late-response guard
- [ ] Gate check passes: `flutter test test/shell/`

**Tests**: widget
**Gate**: quick

**Commit**: `feat(auth): add logout action to the shell`

---

## Phase Execution Map

Visual representation of task ordering. Phases run in sequence, and tasks within a phase run in order:

```
Phase 1 → Phase 2 → Phase 3 → Phase 4

Phase 1:  T1   T2
Phase 2:  T1 -> T3
          T2 -> T3
          T3 -> T4
          T4 -> T5
Phase 3:  T3 -> T6
          T6 -> T7
Phase 4:  T5 -> T8
          T7 -> T8
          T7 -> T9
```

Execution is strictly sequential - there is no intra-phase parallelism.

**Batching:** 9 tasks → two batches (>8): Batch A = Phases 1–2 (T1–T5), Batch B = Phases 3–4 (T6–T9).

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: token store | 1 class + provider + fake | ✅ Granular |
| T2: auth client | 1 client (4 endpoints) | ✅ Granular |
| T3: session | 1 notifier | ✅ Granular |
| T4: decorator | 1 interceptor | ✅ Granular |
| T5: getMySheet + provider wiring | 1 method + 1 provider | ⚠️ OK (same seam) |
| T6: auth screen | 1 screen | ✅ Granular |
| T7: gate + mount + test overrides | 1 widget + root wiring | ⚠️ OK (cohesive) |
| T8: sheet resolution | 1 widget | ✅ Granular |
| T9: logout action | 1 action | ✅ Granular |

---

## Diagram-Definition Cross-Check

| Task | Depends On (body) | Diagram Shows | Status |
| ---- | ----------------- | ------------- | ------ |
| T1 | None | (leaf) | ✅ Match |
| T2 | None | (leaf) | ✅ Match |
| T3 | T1, T2 | T1 -> T3, T2 -> T3 | ✅ Match |
| T4 | T3 | T3 -> T4 | ✅ Match |
| T5 | T4 | T4 -> T5 | ✅ Match |
| T6 | T3 | T3 -> T6 | ✅ Match |
| T7 | T6 | T6 -> T7 | ✅ Match |
| T8 | T5, T7 | T5 -> T8, T7 -> T8 | ✅ Match |
| T9 | T7 | T7 -> T9 | ✅ Match |

---

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1 | Token store | unit | unit | ✅ OK |
| T2 | Auth client | unit | unit | ✅ OK |
| T3 | Session notifier | unit | unit | ✅ OK |
| T4 | Transport decorator | unit | unit | ✅ OK |
| T5 | Domain client | unit | unit | ✅ OK |
| T6 | Auth screen | widget | widget | ✅ OK |
| T7 | Gate + l10n/codegen | widget / build | widget | ✅ OK |
| T8 | Gate + home | widget | widget | ✅ OK |
| T9 | Shell action | widget | widget | ✅ OK |
