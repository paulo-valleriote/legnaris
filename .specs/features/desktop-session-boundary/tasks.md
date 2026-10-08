# Desktop Session Boundary Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/desktop-session-boundary/design.md`
**Status**: Draft

---

## Test Coverage Matrix

> Generated from codebase, project guidelines, and spec - confirm before Execute. Guidelines found: root `AGENTS.md` (testing conventions: table-driven, `Happy path:`/`Edge:`/`Error:` case names, edge cases first), `desktop/AGENTS.md` (l10n + desktop interaction rules), `.github/workflows/ci-flutter.yml` (`flutter analyze` + `flutter test` gate), root `Makefile` `lint-specs` (requirement-ID citations).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Widget/surface logic (gate, home, shell, dialogs, screens) | widget (`flutter_test`) | Every spec AC 1:1 + all listed edge cases; happy path + error paths; case names `Happy path:`/`Edge:`/`Error:` | `desktop/test/**/*_test.dart` | `cd desktop && flutter test` |
| Session/transport/store units (`auth_session`, `authenticated_client`, library/draft stores) | unit | All branches; 1:1 to spec ACs; typed failures asserted; edge: empty/double-activation/late completion | `desktop/test/{auth,api,library,sheet}/*_test.dart` | `cd desktop && flutter test` |
| UI copy (new l10n dialog keys) | widget (through dialog flows) | Dialog appears, cancel, and confirm paths per AC | `desktop/test/shell/*_test.dart` | `cd desktop && flutter test` |

## Gate Check Commands

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | Single-area test run during a task | `cd desktop && flutter test test/<area>` |
| Full | Every task before its commit (CI parity) | `cd desktop && flutter analyze && flutter test` |
| Build | Last task of a phase / citation-bearing changes | Full + `make lint-specs` (repo root) |

---

## Execution Plan

Phases are ordered and run sequentially - each phase completes before the next begins, and tasks within a phase execute in order.

### Phase 1: Navigation boundary

The authenticated branch owns its navigation stack, its dialogs, and its post-login Screen 2 push.

```
T1 -> T2
T3 -> T4
T1 -> T4
```

### Phase 2: Session-local data

Store-level clearing primitives, the session-end sweep, logout activation hardening, and writer guards.

```
T5 -> T7
T6 -> T7
T7 -> T8
T7 -> T10
```

### Phase 3: Session/transport integrity

Session-version contract and the stale-response guard.

```
T8 -------> T11 ------> T12
(from Phase 2)
```

---

## Task Breakdown

### T1: Mount a session navigator in the auth gate

**What**: Give the authenticated branch of `AuthGate` its own `Navigator` (fresh per authenticated transition, destroyed on any session end) plus a `navigatorObservers` seam for tests.
**Where**: `desktop/lib/auth/auth_gate.dart`
**Depends on**: None
**Reuses**: the existing state swap at `auth_gate.dart:49`; `MaterialApp.navigatorObservers` naming for the seam
**Requirement**: NAV-01, NAV-02, NAV-03, NAV-04 (surface half)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] The `AuthAuthenticated` branch renders `Navigator(onGenerateRoute: …)` whose initial route is `AuthenticatedHome`; unauthenticated/restoring branches unchanged
- [x] `AuthGate` accepts `navigatorObservers` (default `const []`) passed to that `Navigator`
- [x] New gate tests: logout from a pushed SheetScreen route leaves only `AuthScreen` mounted (NAV-01); a second login starts from an empty stack — no SheetScreen from the previous session (NAV-02); a forced logout (transport-driven rejected refresh) unmounts every pushed route (NAV-03); unauthenticated renders no `CatalogScreen`/`SheetScreen` (NAV-04)
- [x] Existing gate/logout suites still pass (a session's routes now die with the branch)
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [x] Test count: full suite green, no silent deletions

**Tests**: widget
**Gate**: full

**Commit**: `feat(auth): own the authenticated navigation stack`

---

### T2: Host authenticated-surface dialogs on the session navigator

**What**: Pass `useRootNavigator: false` at the three `showDialog` call sites (creation dialog `catalog_screen.dart:224`, library removal `library_section.dart:118`, discard guards `sheet_screen.dart:379`) so no dialog can outlive its session.
**Where**: `desktop/lib/catalog/catalog_screen.dart`
**Depends on**: T1
**Reuses**: existing dialog builders and dismiss semantics (barrier/Esc = cancel) unchanged
**Requirement**: NAV-04

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] All three dialogs push on the nearest (session) navigator: creation dialog, library-removal dialog, sheet leave/refresh confirmations
- [x] New gate test: opening the creation dialog and forcing a logout leaves only `AuthScreen` — no creation-dialog surface survives (NAV-04)
- [x] New gate test: with the sheet's discard confirmation open, a forced logout tears the dialog down without it (spec edge case), and the session ends
- [x] Existing dialog-flow tests still pass (single-navigator harnesses are unaffected)
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [x] Test count: full suite green, no silent deletions

**Tests**: widget
**Gate**: full

**Commit**: `fix(desktop): host authenticated dialogs on the session navigator`

---

### T3: Extract the shared Screen 2 route builder

**What**: Move the pushed-Route shape inlined in `CatalogScreen._openSheetScreen` (`catalog_screen.dart:243-270`) into `pushSheetScreen(context, {sheetId, sheet})` in a new `lib/sheet/sheet_route.dart`, and adopt it in the catalog (creation flow + library open) with no behavior change.
**Where**: `desktop/lib/sheet/sheet_route.dart`
**Depends on**: None
**Reuses**: verbatim route body — `MaterialPageRoute`, AD-004 `ProviderScope` overrides, `AppShell(content: SheetScreen(...))`
**Requirement**: NAV-05 (enabler)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `pushSheetScreen` owns the single route shape; `catalog_screen.dart` calls it from both call sites
- [x] No behavior change: creation and library open routes carry the same overrides as before
- [x] Existing navigation tests pass unchanged: `create_sheet_navigation_test`, `library_open_navigation_test`, `create_sheet_dialog_test`
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [x] Test count: full suite green, no silent deletions

**Tests**: widget (existing navigation suite regression)
**Gate**: full

**Commit**: `refactor(desktop): extract the shared screen 2 route builder`

---

### T4: Open the post-login sheet as a route above the catalog

**What**: `AuthenticatedHome` always renders `AppShell(content: CatalogScreen())` once the lookup settles, and a `200` pushes Screen 2 through `pushSheetScreen` exactly once per authenticated transition; reset `ShellSurface` on branch mount.
**Where**: `desktop/lib/auth/authenticated_home.dart`
**Depends on**: T1, T3
**Reuses**: `_load` single-flight (`_inFlight`), `pushSheetScreen` (T3), `shellStateProvider`
**Requirement**: NAV-05, NAV-06, NAV-07, NAV-08, NAV-02 (shell-surface reset)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `200` after authentication → `SheetScreen` is a route pushed above a mounted `CatalogScreen` (NAV-05); the pre-existing assertion `CatalogScreen findsNothing` in `auth_gate_test.dart:284` is updated to the spec outcome (catalog beneath, sheet above)
- [x] Back on that Screen 2 returns to the catalog (NAV-06), including the discard-confirmed dirty path
- [x] Retry-after-failure plus a rebuild push exactly one Screen 2 route — counted through the `navigatorObservers` seam (NAV-07); `404` still presents the catalog with no push
- [x] `Ctrl+S` (and F5) dispatch to the post-login-mounted SheetScreen — `Ctrl+S` with a pending edit issues one `PATCH /sheets/:id` (NAV-08); existing shortcut tests still pass
- [x] `initState` resets `shellStateProvider` to `ShellSurface.catalog` post-frame (NAV-02)
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [x] Test count: full suite green, no silent deletions

**Tests**: widget
**Gate**: full

**Commit**: `fix(auth): open the post-login sheet above the catalog`

---

### T5: Clear the sheet library at session end

**What**: Add `SheetLibrary.clear()` — empties the in-memory index, republishes state, then best-effort persists an empty index.
**Where**: `desktop/lib/library/sheet_library_provider.dart`
**Depends on**: None
**Reuses**: `_publish`, `sheetLibraryStoreProvider.save`, the best-effort swallow pattern of `draft_backup_store.delete`
**Requirement**: DATA-01 (primitive), DATA-03 (swallow)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `clear()` empties memory (`entryFor` returns null, `entries` empty) and writes `[]` to `sheet_library.json`
- [x] A `save` failure is swallowed — `clear()` never throws (DATA-03)
- [x] Unit tests: happy path (memory + disk), edge: store save throws → no exception, memory still cleared
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [x] Test count: full suite green, no silent deletions

**Tests**: unit
**Gate**: full

**Commit**: `feat(library): clear the sheet library at session end`

---

### T6: Delete every draft backup at session end

**What**: Add `DraftBackupStore.clearAll()` — deletes every file under `drafts/`, skipping non-file entries, swallowing per-file and listing failures; never throws.
**Where**: `desktop/lib/sheet/draft_backup_store.dart`
**Depends on**: None
**Reuses**: `_draftsDirectory`, `delete`'s best-effort contract
**Requirement**: DATA-01 (primitive), DATA-03 (edge: one failing file never blocks)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `clearAll()` deletes all stored backups; subsequent `read()` returns null for each id
- [x] Missing `drafts/` directory is a no-op; a subdirectory entry is skipped; a `drafts` path that exists as a file never throws (listing failure swallowed)
- [x] Unit tests cover happy path + those three edges
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [x] Test count: full suite green, no silent deletions

**Tests**: unit
**Gate**: full

**Commit**: `feat(sheet): delete every draft backup at session end`

---

### T7: Sweep session-local data when the session ends

**What**: `AuthSession` calls a private `_clearSessionLocalData()` (per-store `try/catch`) synchronously right after the state flip in `logout()` and `onUnauthorized()` — and nowhere else.
**Where**: `desktop/lib/auth/auth_session.dart`
**Depends on**: T5, T6
**Reuses**: the two explicit definitive-end call sites (`auth_session.dart:174`, `:165`); `SheetLibrary.clear`, `DraftBackupStore.clearAll`
**Requirement**: DATA-01, DATA-02, DATA-03, DATA-04

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Explicit logout clears the library file and every draft file by the time `logout()` completes (DATA-01) — asserted on disk in the widget logout suite (support dir seeded before logout)
- [x] Forced logout (`onUnauthorized`, transport-driven) sweeps the same way (DATA-02)
- [x] A failing library store never blocks logout and never skips the drafts sweep (DATA-03): logout completes, state is unauthenticated, drafts still cleared
- [x] Transient restore failure keeps stored token, library file, and drafts untouched (DATA-04)
- [x] No sweep on the no-stored-token first-launch path
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [x] Test count: full suite green, no silent deletions

**Tests**: unit + widget
**Gate**: full

**Commit**: `feat(auth): sweep session-local data when the session ends`

---

### T8: Issue at most one logout request per activation

**What**: Wrap the logout body in a shared in-flight future set synchronously before the first `await`, mirroring `AuthenticatedClient._refreshOnce`.
**Where**: `desktop/lib/auth/auth_session.dart`
**Depends on**: T7
**Reuses**: the share-in-flight pattern of `authenticated_client.dart:57`
**Requirement**: DATA-08

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Two `logout()` calls in the same frame issue exactly one `POST /auth/logout` (counted on the mock auth client) and both callers complete after the same teardown
- [x] A later logout after the first completes issues a new request (the guard clears)
- [x] Unit tests: double activation → one request; sequential activations → two requests
- [x] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [x] Test count: full suite green, no silent deletions

**Tests**: unit
**Gate**: full

**Commit**: `fix(auth): issue at most one logout request per activation`

---

### T9: Confirm logout while the active sheet has pending edits

**What**: `Header`'s logout handler reads the active sheet's `isPending` at tap time; pending edits open a destructive confirmation (new l10n keys) on the session navigator, only a confirm calls `logout()`.
**Where**: `desktop/lib/shell/app_shell.dart`
**Depends on**: None
**Reuses**: `_confirmDiscard`'s dialog shape (`sheet_screen.dart:372`), `activeSheetIdProvider` (`sheet_refresh.dart:19`), `SheetEditState.isPending`, AD-001 tap-time read
**Requirement**: DATA-05, DATA-06, DATA-07

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] l10n: `authLogoutDialogTitle`, `authLogoutDialogMessage`, `authLogoutConfirmAction`, `authLogoutCancelAction` added to `app_en.arb` (descriptions cite DATA-05) and regenerated via `flutter pub get`
- [ ] Pending edits at tap time → dialog stating the unsaved edits will be discarded appears (DATA-05); cancel via button, Esc, or barrier keeps session, edits, persisted library and drafts, and sends no `/auth/logout` (DATA-06)
- [ ] No pending edits → logout runs with no dialog (DATA-07)
- [ ] The session notifier is captured before the dialog await; `context.mounted` is re-checked after it (forced logout during the dialog ends the flow safely)
- [ ] Tests target the topmost `shell-logout` (a pushed route mounts a second `AppShell`)
- [ ] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: full suite green, no silent deletions

**Tests**: widget
**Gate**: full

**Commit**: `feat(shell): confirm logout while edits are pending`

---

### T10: Guard local writes against a dead session

**What**: The draft auto-backup tick and the creation dialog's library `upsert` skip their write when `authSessionProvider` is no longer authenticated, closing the sweep-to-teardown window.
**Where**: `desktop/lib/sheet/draft_auto_backup.dart`
**Depends on**: T7
**Reuses**: the sweep from T7; existing writer call sites (`draft_auto_backup.dart:65`, `create_sheet_dialog.dart:160`)
**Requirement**: DATA-01 (re-write edge)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] A scheduler tick after the session ends writes no draft file (asserted with a fake clock past the 60s interval after `onUnauthorized`)
- [ ] A creation `201` completing after a forced logout does not `upsert` into the cleared library (asserted: library stays empty across the next session)
- [ ] Both writers behave exactly as before while authenticated (existing draft-backup and creation tests pass)
- [ ] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: full suite green, no silent deletions

**Tests**: unit + widget
**Gate**: full

**Commit**: `fix(desktop): guard local writes against a dead session`

---

### T11: Expose the session version to the transport

**What**: Add `int sessionVersion()` to `SessionDelegate` and bump `_version` when login/register complete, keeping bumps at session boundaries only (logout/forced logout already bump).
**Where**: `desktop/lib/auth/auth_session.dart`
**Depends on**: T8
**Reuses**: the existing `_version` stale-refresh check (`auth_session.dart:134`)
**Requirement**: SES-01

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] `sessionVersion()` returns `_version`; login and register completion bump it; a refresh applying a rotated token does not (FE6-11 concurrent sharing preserved)
- [ ] Test: a refresh started before a login completes with the old session's token → returns null, the new access token and the new stored refresh token survive (SES-01)
- [ ] Existing late-refresh-after-logout test still passes
- [ ] `FakeSessionDelegate` (test/api) implements the new member
- [ ] Gate check passes: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: full suite green, no silent deletions

**Tests**: unit
**Gate**: full

**Commit**: `feat(auth): expose the session version to the transport`

---

### T12: Drop auth responses from a finished session

**What**: `AuthenticatedClient.send()` captures the session version when sending and re-checks it after the `401` (stale → no refresh, no replay) and after a `null` refresh (stale → no `onUnauthorized()`).
**Where**: `desktop/lib/api/authenticated_client.dart`
**Depends on**: T11
**Reuses**: `_refreshOnce` sharing, `SessionDelegate.sessionVersion()`
**Requirement**: SES-02

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] A `401` arriving after the session changed returns the original response without issuing a refresh and without replaying (the foreign request never executes as the new session)
- [ ] A held refresh completing as `null` after the session changed does not call `onUnauthorized()` — the new session keeps its access and stored refresh token (SES-02, spec independent test)
- [ ] Same-session rejection still forces the logout (existing FE6 forced-logout tests pass)
- [ ] Concurrent `401`s still share one refresh within a session (existing FE6-11 test passes)
- [ ] Gate check passes: `cd desktop && flutter analyze && flutter test`; last task also runs `make lint-specs` clean (every cited requirement ID defined)
- [ ] Test count: full suite green, no silent deletions

**Tests**: unit
**Gate**: build

**Commit**: `fix(api): drop auth responses from a finished session`

---

## Phase Execution Map

```
Phase 1 -> Phase 2 -> Phase 3

Phase 1:  T1 -------> T2
          T3 -------> T4
          (T4 also depends on T1)

Phase 2:  T5 -- T6 --> T7 -------> T8
          T7 -------> T10
          T9 is independent

Phase 3:  T11 ------> T12
```

Execution is strictly sequential - there is no intra-phase parallelism. A single agent (or batch worker) works one task at a time, in order. Batches are packed from whole phases at Execute (~7 tasks per batch): Phase 1 (4 tasks), Phase 2 (6 tasks), Phase 3 (2 tasks).
