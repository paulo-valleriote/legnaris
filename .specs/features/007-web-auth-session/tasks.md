---
description: "Task list for the web auth session feature"
---

# Tasks: Web Auth Session

**Input**: Design documents from `.specs/features/007-web-auth-session/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: Required — constitution III (NON-NEGOTIABLE) and WAS-33 make every behavior ship tests in
the same commit, edge cases first, and interface requirements verified by rendering tests. Test tasks
are therefore present for every story.

**Organization**: Tasks are grouped by user story so each is independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1, US2, US3 or US4 (setup/foundational/polish tasks carry no story label)
- Every task names the exact file path it touches.

## Path Conventions

Web client in the existing Go monorepo: sources under `web/src/`, suites flat under `web/src/test/`,
browser specs under `web/e2e/`. Specs under `.specs/features/007-web-auth-session/`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the shared test seam and copy keys every story reads.

- [ ] T001 [P] Extend `web/src/test/fake_transport.ts` so each recorded request exposes `method`, `credentials`, `url` and `headers` and the transport answers a scripted queue of `{status, body}` — the one seam every auth suite counts requests, headers and postures through.
- [ ] T002 [P] Add the feature's copy keys to `web/src/l10n/strings.ts` (`auth.title`, `auth.modeLogin`, `auth.modeRegister`, `auth.emailLabel`, `auth.passwordLabel`, `auth.usernameLabel`, `auth.submitLogin`, `auth.submitRegister`, `auth.restoring`, `nav.ownSheet`, `nav.logout`, `sheet.ownTitle`, `sheet.ownLoading`, `sheet.ownEmpty`, `sheet.ownOpen`, `sheet.ownRetry`, `sheet.ownDelete`, `sheet.ownDeleteTitle`, `sheet.ownDeleteMessage`, `sheet.ownDeleteConfirm`, `sheet.ownCancel`, `api.error.unauthorized`, `api.error.forbidden`, `api.error.rateLimited`) (WAS-31, SC-008).
- [ ] T003 [P] Extend `web/src/test/l10n_strings.test.ts` to scan `web/src/auth/` and the new `web/src/sheet/your_sheet*.tsx` for user-facing literals and unresolved keys, extend `web/src/test/api_no_logging.test.ts` to scan `web/src/auth/` as well as `web/src/api/` so no credential or URL reaches a log record, and extend `web/src/test/tokens_used.test.tsx` to scan `web/src/auth/` for literal hex/px (WAS-31, WAS-32, SC-008).

**Checkpoint**: the copy catalog and the three cross-cutting scans cover the feature's directories.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the wire seam and the failure taxonomy every story builds on. No user story starts until
this phase completes.

- [ ] T004 Widen `web/src/api/transport.ts`: `HttpRequest.credentials` becomes `'omit' | 'include'` and `HttpMethod` gains `'DELETE'` (WAS-06, WAS-11).
- [ ] T005 Extract the shared request caller (fail-fast timeout, single GET retry, envelope decode, `classifyDomainFailure`) from `web/src/api/client.ts` into `web/src/api/request.ts` as `createCaller({ baseUrl, transport, timeoutMs, retryDelayMs, sleep })` returning `call(method, path, body, decode, options)` where `options` carries `credentials` (default `'omit'`) and `headers` (R1); re-point `client.ts` over it with no behavior change, keeping `api_client.test.ts` green.
- [ ] T006 [P] Extend `web/src/api/errors.ts` (`FailureKind` gains `'unauthorized' | 'forbidden' | 'rateLimited'`) and `web/src/api/error_mapper.ts` (`UNAUTHORIZED` → `unauthorized`, `FORBIDDEN` → `forbidden`, `RATE_LIMITED` → `rateLimited`, every other code stays `unknown`) (WAS-30).
- [ ] T007 [P] Extend `web/src/test/api_error_mapper.test.ts`: edge cases first — `UNAUTHORIZED`/`FORBIDDEN`/`RATE_LIMITED` map to their kinds and copies, `CONFLICT` and an unknown code stay on the generic kind, `failureCopy` resolves a catalog string and never a server message (WAS-30, SC-008, SC-009).

**Checkpoint**: the transport can carry a cookie credential and a `DELETE`, and the three session codes
are classified — user story work can begin.

---

## Phase 3: User Story 1 - Hold a session no script can persist (Priority: P1) 🎯 MVP

**Goal**: login, registration and cold-start restore establish an in-memory session behind the four
`/auth/*` operations, and the auth gate renders only the sign-in form while no session is held.

**Independent Test**: drive login, registration and a cold start (no in-memory token) with a valid, a
rejected and a transiently failing cookie; assert the session state, that no credential lands in a
JavaScript-readable store, that a rejected cookie shows the sign-in form, and that the catalog is not
rendered while no session is held.

### Tests for User Story 1

- [ ] T008 [P] [US1] `web/src/test/auth_api.test.ts` — edge first: login and register each send exactly one `credentials: 'include'` request, register issues no follow-up login, neither sends CSRF, an absent CSRF on refresh sends no header, a malformed session body is a `DefensiveParseFailure`, a `204` logout body is accepted; happy path: the session body decodes to `{accessToken, tokenType, expiresIn}` (WAS-01, WAS-02, WAS-06, WAS-18).
- [ ] T009 [P] [US1] `web/src/test/auth_session.test.ts` — edge first: a login failure leaves `unauthenticated` and rethrows, a register failure likewise, the version bumps once on login and registration and **not** on a refresh apply, the token never appears in the store double; happy path: login holds the token and flips to `authenticated` (WAS-01, WAS-02, WAS-07, WAS-32).
- [ ] T010 [P] [US1] `web/src/test/auth_screen.test.tsx` — edge first: a failed submit keeps the typed values and renders the mapped copy, the duplicate-email and any other registration failure render the same copy, a server message carrying `<script>` renders as text, the password input is a hidden-content control; happy path: mode switch renders the login and registration fields, the single primary action disables and shows progress while a submit is in flight and a repeat activation issues no second request (WAS-07, WAS-09, WAS-10, WAS-31).
- [ ] T011 [P] [US1] `web/src/test/auth_gate.test.tsx` — edge first: a rejected cookie shows the sign-in form with no session, a transient restore failure shows the sign-in form and keeps the session-local data, a deep link to `/sheets/abc` while signed out shows the sign-in form and after sign-in lands on the catalog, no-session renders no catalog/sheet/your-sheet; happy path: a valid cookie restores through exactly one silent refresh, showing the restoring state until it settles (WAS-03, WAS-04, WAS-05, WAS-08, WAS-24).

### Implementation for User Story 1

- [ ] T012 [US1] `web/src/api/csrf.ts` — total `readCsrfToken(cookieString: string): string | undefined` for `__Host-legnaris_csrf`; absent/empty/malformed yields `undefined`, never a throw (R5).
- [ ] T013 [US1] `web/src/api/auth_api.ts` — `AuthApi` + `createAuthApi` over T005's caller and the injected `Transport`: `register`/`login`/`refresh`/`logout`, `credentials: 'include'`, CSRF header on `refresh`/`logout` from T012, `SessionBody` defensive decode, `204` no-content logout (WAS-01, WAS-02, WAS-06, WAS-18, WAS-30).
- [ ] T014 [US1] `web/src/auth/auth_session.ts` — the in-memory `AuthSession` (`accessToken`, `version`, `state`, `refreshInFlight`), `restore()` (one silent refresh; `401` → `unauthenticated`; transient → `unauthenticated` keeping local data), `login`/`register` (bump the version **before** applying), `refresh()` (`null` on `401`, rethrow transient), `onUnauthorized()`, and `logout()`/teardown without the sweep (the sweep is US3); emits immutable snapshots for `useSyncExternalStore` (WAS-01…WAS-05, WAS-07, WAS-14, WAS-15, WAS-17).
- [ ] T015 [US1] `web/src/auth/auth_screen.tsx` — one surface with a sign-in/registration mode switch, controls resolving copy through `message()`, one primary action in a footer disabled with progress while `authenticating`, inline mapped failure copy via `failureCopy`, values kept on failure, password a hidden-content control (WAS-07, WAS-09, WAS-10).
- [ ] T016 [US1] `web/src/app/routes.ts` — every data route (catalog `/`, sheet `/sheets/:id`, not-found) becomes `audience: 'session'` (WAS-08).
- [ ] T017 [US1] `web/src/auth/auth_gate.tsx` + `web/src/app/router.tsx` — the gate runs `restore()` once, renders the restoring state until it settles, renders `AuthScreen` when unauthenticated, renders the `AppShell` route tree when authenticated, and resets the location to `/` on a session end (WAS-03, WAS-04, WAS-05, WAS-08, WAS-24).
- [ ] T018 [US1] `web/src/app/providers.tsx` — build and provide the raw `Transport`, the `AuthApi`, the `AuthSession` and the domain `ApiClient` (over the raw transport for now; US2 decorates it), with the default `document.cookie` reader and `fetch` injected at the app entry; add `useAuthSession()` (R11).
- [ ] T019 [US1] Extend `web/src/test/render.tsx`: `renderApp` mounts an injected `AuthSession` and defaults to an **authenticated** session, and `RenderAppOptions` gains a `session: 'authenticated' | 'unauthenticated'` option so the gate suite can drive the unauthenticated path (WAS-03, WAS-08).
- [ ] T020 [US1] Reconcile the suites that assumed a publicly reachable catalog — the fourteen `renderApp` suites (`shell*`, `routing`, `catalog_*`, `create_sheet_*`, `sheet_*`) and the four `web/e2e/*.spec.ts` specs (`shell`, `catalog`, `sheet`, `pwa`) — so they render behind the session (an authenticated `renderApp` by default, a seeded refresh in the browser specs); retarget, do not delete (`AD-013` pattern), and record any retarget as a deviation note (spec Assumptions, `WAS-08`).

**Checkpoint**: login, registration and cold-start restore work end to end, the gate guards every
surface, and every earlier suite still renders truthfully behind the session; the MVP is demonstrable.

---

## Phase 4: User Story 2 - A lapsed access token recovers without a second rotation (Priority: P1)

**Goal**: protected calls carry the Bearer, a `401` triggers one single-flight silent refresh and one
replay, a rejected refresh ends the session, and the auth/health routes never authenticate.

**Independent Test**: issue a protected call that answers `401` and assert one refresh and one replay;
fire several concurrent `401`s and assert a single refresh; drive a rejected refresh and assert the
session ends; drive a transient refresh and assert it survives; assert `/auth/*` and `/health` are
never authenticated.

### Tests for User Story 2

- [ ] T021 [P] [US2] `web/src/test/authenticated_transport.test.ts` — edge first: a protected call with no token sends no Bearer, `/health` and each `/auth/*` path carry no Bearer and start no refresh, a replay that is again `401` is not replayed twice, a `401` or refresh belonging to a superseded version is dropped (no refresh, no replay, no forced logout), a transient refresh rethrows and keeps the token and the session-local data; happy path: a protected call carries Bearer, a `401` produces exactly one refresh and one replayed request, two concurrent `401`s share one refresh, a rejected refresh calls `onUnauthorized()` once and returns the original `401` (WAS-11, WAS-12, WAS-13, WAS-14, WAS-15, WAS-16, WAS-17, SC-003, SC-004).

### Implementation for User Story 2

- [ ] T022 [US2] `web/src/api/authenticated_transport.ts` — `createAuthenticatedTransport({ inner, session })`: attach `Bearer` to a non-public path, capture the version, single-flight refresh on a protected `401`, replay once, forced logout on a rejected refresh, rethrow a transient, and drop a response from a superseded version (WAS-11…WAS-17).
- [ ] T023 [US2] `web/src/app/providers.tsx` — wrap the domain `ApiClient`'s transport with T022's decorator over the session (R11), so `/sheets` and `/templates` are Bearer-authenticated and never cookie-authenticated.

**Checkpoint**: a session survives token expiry and a burst of refusals with one rotation.

---

## Phase 5: User Story 3 - Signing out leaves nothing of the session behind (Priority: P1)

**Goal**: sign-out and forced logout drop the token, return to the sign-in form and sweep every cached
sheet and draft; a transient failure keeps them; sign-out is single-flight.

**Independent Test**: sign in, cache a sheet and leave a draft, sign out, and assert the records are
gone and the sign-in form is shown; repeat through a forced logout; drive a transient refresh failure
and assert the records survive; activate sign-out twice and assert one `/auth/logout`.

### Tests for User Story 3

- [ ] T024 [P] [US3] `web/src/test/session_boundary.test.ts` — edge first: a failing `clearAll` still ends the session and shows the form, a transient refresh failure leaves the cached sheet and the draft in place, a failed `/auth/logout` never blocks the teardown, activating sign-out twice in one frame issues one `/auth/logout`; happy path: sign-out empties the store double (cache + drafts) and shows the sign-in form, and a forced logout clears it identically, so a later session renders none of the finished session's records (WAS-19, WAS-20, WAS-21, WAS-22, WAS-23, WAS-32).
- [ ] T025 [P] [US3] Extend `web/src/test/library_store.test.ts` — `clearAll` on the in-memory double empties both records, and under `fake-indexeddb` the adapter's `clearAll` empties both object stores and resolves on an already-empty database (WAS-21).

### Implementation for User Story 3

- [ ] T026 [US3] Extend `web/src/library/sheet_store.ts` (the `SheetStore` port gains `clearAll()` and the memory double implements it) and `web/src/library/idb_sheet_store.ts` (`clearAll()` clears both object stores) (WAS-21).
- [ ] T027 [US3] `web/src/library/session_data.ts` — `clearSessionData(store)` running `bestEffort(() => store.clearAll())`, so a failing store never blocks the session end (WAS-21).
- [ ] T028 [US3] `web/src/auth/auth_session.ts` — call `clearSessionData` from `logout()` and `onUnauthorized()` **only** (never from the transient restore fallback), make `logout()` single-flight (guard set synchronously before the first `await`), and issue `/auth/logout` best-effort (WAS-19, WAS-20, WAS-21, WAS-23).
- [ ] T029 [US3] `web/src/shell/left_nav.tsx` — a session-only sign-out control (`nav.logout`) calling `AuthSession.logout()`, disabled while in flight, so the gate returns to the sign-in form (WAS-19, WAS-23).

**Checkpoint**: an account switch leaves no local record of the previous session.

---

## Phase 6: User Story 4 - The signed-in person's own sheet is reachable and deletable (Priority: P2)

**Goal**: a session-only "Your sheet" destination resolves `GET /sheets`, shows the empty state when
none, opens the sheet, and deletes it behind an exact-consequence confirmation.

**Independent Test**: sign in as a person with a sheet and assert the entry resolves it with one `GET
/sheets`; sign in with none and assert the empty state; drive the delete with a cancel (zero requests)
and a confirm (one request) and assert the local copy and draft are gone; drive a delete failure and
assert the sheet is kept with the mapped message.

### Tests for User Story 4

- [ ] T030 [P] [US4] `web/src/test/your_sheet.test.tsx` — edge first: no sheet renders the designed empty state (a `404` is not a failure), any other outcome renders the mapped copy with a Retry that issues exactly one request, an undecodable payload is a failure not a crash, cancel issues zero `DELETE`s and keeps the sheet, a delete failure keeps the sheet and shows the mapped copy; happy path: a person with a sheet renders it from one `GET /sheets`, Open navigates to `/sheets/:id`, confirm issues exactly one `DELETE /sheets/:id`, evicts the sheet and its draft, and shows the empty state (WAS-25, WAS-26, WAS-27, WAS-28, WAS-29, SC-007).
- [ ] T031 [P] [US4] Extend `web/src/test/api_client.test.ts` — `getMySheet()` issues one `GET /sheets` and decodes the sheet, `deleteSheet(id)` issues one `DELETE /sheets/:id` and accepts a `204`, and a `404` on `getMySheet` classifies as `notFound` (WAS-25, WAS-28).

### Implementation for User Story 4

- [ ] T032 [US4] Extend `web/src/api/client.ts` — `getMySheet(): Promise<Sheet>` (`GET /sheets`, decoded by `parseSheet`, no-content-free) and `deleteSheet(id: string): Promise<void>` (`DELETE /sheets/:id`, `204`) (WAS-25, WAS-28).
- [ ] T033 [US4] `web/src/sheet/your_sheet.ts` — the resolve state machine (`loading | empty | sheet | failure`), `resolve()` mapping `NOT_FOUND` to `empty` and any other outcome to `failure`, and `delete(id)` on confirm evicting the sheet and its draft through the F5 `evictSheet`/`discardDraft` helpers (WAS-25…WAS-29).
- [ ] T034 [US4] `web/src/sheet/your_sheet_screen.tsx` — the destination: header, loading, the designed empty line, the sheet with an Open action, the mapped failure with a single-request Retry, and the delete confirmation dialog stating the exact consequence with a safe cancel action (WAS-25…WAS-29).
- [ ] T035 [US4] `web/src/app/routes.ts` (`/your-sheet` declared, `audience: 'session'`), `web/src/app/router.tsx` (map it to `YourSheetScreen`) and `web/src/shell/left_nav.tsx` (the session-only `nav.ownSheet` entry, routed through the leave registry like the catalog link) (WAS-25).

**Checkpoint**: all four stories work independently and together.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: the browser-level accessibility measurement and the final validation.

- [ ] T036 [P] `web/e2e/auth.spec.ts` — a Playwright axe sweep against the real build covering the auth surface, the signed-in shell and the "Your sheet" surface, asserting zero critical automated violations and an accessible role/name on every control (SC-010, `AD-014`).
- [ ] T037 [P] Update `web/README.md` and the F7 deviation notes (the `session_store.ts` → `auth_session.ts` and `authenticated_client.ts` → `authenticated_transport.ts` renames, the no-token-store deviation, the no-confirmation sign-out) so the recorded deviations are discoverable (`AD-017`).
- [ ] T038 Run the [quickstart.md](./quickstart.md) scenarios end to end (`cd web && npm test`, the individual suites, `npm run lint`, `npm run build`, the Playwright case) and `make lint-specs`; confirm no dependency was added and the security gates are unchanged.
- [ ] T039 Final `make fmt && make lint && make vet && make lint-specs` from the repo root and `make lint-web && make test-web && make sec-scan` from the client, with the full suite green under the race-equivalent web run.

**Total tasks**: 39.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — can start immediately.
- **Foundational (Phase 2)**: depends on Setup — **blocks all user stories**.
- **User Stories (Phase 3+)**: all depend on Foundational. US1 is the base; US2, US3 and US4 build on
  US1's session and each is independently testable.
- **Polish (Phase 7)**: depends on all desired stories.

### User Story Dependencies

- **US1 (P1)**: starts after Foundational; no dependency on other stories. Delivers the session, the
  gate, the sign-in surface and the reconciled test suites (the MVP).
- **US2 (P1)**: starts after Foundational; adds the authenticated transport over US1's session
  (`T022` reads `SessionDelegate`). Independently testable with an injected session.
- **US3 (P1)**: starts after Foundational; adds the sweep to US1's session (`T028`) and the sign-out
  control. Independently testable with an injected store.
- **US4 (P2)**: starts after Foundational; adds the domain operations and the "Your sheet" surface. The
  route/entry (`T035`) needs US1's audience change; independently testable with an injected client.

### Within Each User Story

- Tests are written first and must fail before implementation.
- Models/ports before services (`T026` before `T027`/`T028`), services before surfaces.
- Core implementation before integration; the story is complete before moving to the next priority.

### Parallel Opportunities

- Setup: `T001`, `T002`, `T003` in parallel (different files).
- Foundational: `T006`/`T007` parallel with each other; `T004` → `T005` sequential (same seam).
- US1 tests `T008`–`T011` in parallel; implementation `T012`→`T013`→`T014` sequential (each consumes
  the previous), then `T015`, `T016`, `T017`, `T018`, then `T019`→`T020` (the shared helper before the
  suites that consume it).
- US2: `T021` before `T022`→`T023`.
- US3: `T024`/`T025` in parallel; `T026`→`T027`→`T028`, then `T029`.
- US4: `T030`/`T031` in parallel; `T032`→`T033`→`T034`, then `T035`.
- Polish: `T036`/`T037` in parallel.

---

## Parallel Example: User Story 1

```bash
# Launch the four US1 suites together:
Task: "auth_api.test.ts — credentials include, one request, no follow-up login (WAS-01, WAS-02, WAS-06, WAS-18)"
Task: "auth_session.test.ts — states, version bump, memory-only token (WAS-01, WAS-02, WAS-07, WAS-32)"
Task: "auth_screen.test.tsx — modes, in-flight, failure keeps values (WAS-07, WAS-09, WAS-10)"
Task: "auth_gate.test.tsx — restoring, unauthenticated, authenticated, deep-link reset (WAS-03…WAS-08, WAS-24)"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 (Setup) and Phase 2 (Foundational).
2. Complete Phase 3 (US1): the four `/auth/*` operations, the session, the gate, the sign-in form and
   the reconciled suites.
3. **STOP and VALIDATE** independently (quickstart scenarios 1–3).
4. Demo: sign in, register, reload, and see every surface gated.

### Incremental Delivery

1. Setup + Foundational → the seam and taxonomy are ready.
2. US1 → the session and the gate (MVP).
3. US2 → silent recovery on expiry.
4. US3 → the session boundary and sign-out.
5. US4 → the "Your sheet" destination and the confirmed delete.
6. Polish → the accessibility measurement and the validation run.

---

## Requirement Coverage

| Story | Requirements | Success criteria |
|---|---|---|
| US1 | WAS-01…WAS-10, WAS-24 | SC-001, SC-002, SC-008 |
| US2 | WAS-11…WAS-18 | SC-003, SC-004 |
| US3 | WAS-19…WAS-23, WAS-32 | SC-005, SC-006 |
| US4 | WAS-25…WAS-29 | SC-007, SC-008 |
| Cross-cutting | WAS-30, WAS-31, WAS-33 | SC-008, SC-009, SC-010 |

Every `WAS-*` requirement and every `SC-*` outcome maps to at least one task above; `WAS-33` is the
test-in-the-same-commit rule carried by every `Tests for User Story` block.

`WAS-24` sits in the spec's *sign-out and the session boundary* group (next to US3) but is implemented
by US1's gate and route reset (`T011`, `T017`); it is listed under US1 because that is where it is
built, and `T024`'s session-boundary suite exercises it again from the US3 side.

---

## Notes

- `[P]` tasks target different files with no incomplete dependency.
- `[Story]` labels trace a task to its user story.
- Commit after each task or logical group; every behavior change ships its tests in the same commit.
- No dependency is added, so the SCA/SAST/secret gates see nothing new to vet.
- Stop at any checkpoint to validate a story independently.
