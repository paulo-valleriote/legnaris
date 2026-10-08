# Front End Fase 6 — Session Auth (desktop client adaptation) Specification

Source: companion to backend epic [LGN-7](https://paulovalleriotedev.atlassian.net/browse/LGN-7) / feature `lgn-7-ownership`. Front-end epic [LGN-88](https://paulovalleriotedev.atlassian.net/browse/LGN-88) marks auth as V2. Contract source: backend `docs/TDD.md` §Autenticação and `docs/TDD-desktop-mvp.md` §V2 — Auth. Decision [LGN-111](https://paulovalleriotedev.atlassian.net/browse/LGN-111) fixes the native refresh transport (`X-Refresh-Token` header), which supersedes the stale "httpOnly cookie" wording in the desktop TDD.

## Problem Statement

Once `lgn-7-ownership` ships, every `/templates*` and `/sheets*` route requires a JWT and every sheet is owner-scoped with one sheet per user. The desktop client sends no credentials, so the catalog and sheet screens break with `401`. The app has no authentication surface: no login, no token storage, no header injection. This feature adds a session layer, an auth gate, and the authenticated transport so the desktop app keeps working — and replaces the local-only "my sheets" list with the server's `GET /sheets`.

## Goals

- [ ] A login/register surface and an auth gate that shows it while unauthenticated.
- [ ] Access token held in memory; refresh token persisted in OS secure storage (never in the local JSON store).
- [ ] Every protected request carries `Authorization: Bearer`; a `401` transparently refreshes once and replays.
- [ ] Refresh rotation and reuse detection handled as a forced logout back to the login screen.
- [ ] After login, the app resolves the user's sheet via `GET /sheets` and opens it, or offers creation on `404`.
- [ ] Logout revokes the refresh token and returns to the login screen.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Backend auth/ownership changes | `lgn-7-ownership` (this feature only consumes them) |
| Password reset / email verification / account management | Not in the backend; no endpoints exist |
| "Remember me", biometric unlock, SSO | Not requested |
| Public distribution packaging (MSIX, signing) | Separate V2 track in the desktop TDD |
| Removing the local sheet-library code entirely | Follow-up cleanup; this feature stops depending on it for open, it does not delete it |
| Offline mode / request queueing | Unchanged from V1 (out of scope) |
| Branding, palette, custom typography | V2 branding track |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | --------------- | --------- | ---------- |
| Refresh transport | `X-Refresh-Token` request header; the client reads it from the login/refresh response body and stores it | Backend decision LGN-111 for native clients; `auth_integration_test.go` asserts no `Set-Cookie` is emitted | y |
| Secure storage dependency | Add `flutter_secure_storage`; it is the only storage that satisfies the TDD secure-token requirement | `desktop/AGENTS.md` permits a new package only when strictly necessary; OS credential storage is mandatory for the refresh token | y |
| Access token persistence | Memory only (never written to disk) | `docs/TDD-desktop-mvp.md` §V2 — Auth | y |
| Login and register UX | A single auth screen with a mode switch (Log in / Register); no email verification step | Backend register returns the user without a session, so the client logs in immediately after | n |
| Token injection seam | Wrap the injected `http.Client` in an `AuthenticatedClient` (`http.BaseClient`) inside `apiClientProvider`; no endpoint method changes | `api_client.dart` documents the injected transport as "the single extension point for auth in V2" (FE1-07) | y |
| 401 handling | Single-flight refresh: the first `401` triggers one refresh; concurrent `401`s await the same result; each request is retried at most once | Avoids a refresh stampede and infinite retry loops | y |
| Sheet resolution after login | `GET /sheets` returns the single owned sheet (200) or 404; the app opens it or offers create | Backend now enforces one sheet per user; the local library is redundant | y |
| Fate of the local library | Kept in the codebase but no longer the source of truth for "my sheet"; the server `GET /sheets` wins | Bounds this feature; library removal is a follow-up card | n |
| Reuse-detection response | Treat a failed refresh (401) as a forced logout and clear storage | Backend revokes the whole family on reuse | y |
| Storage failure on login | The session works for the run but is not persisted; the next launch requires login | Secure storage can be unavailable (locked keychain); do not hard-fail login | n |
| Dev HTTP override | Allowed only via the existing `API_BASE_URL` build override, marked in the connection indicator | The desktop TDD requires TLS in production and a visibly marked dev override | n |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Session and token lifecycle ⭐ MVP

**User Story**: As a player, I want to log in once and stay logged in across restarts so that the app opens my sheet without retyping credentials.

**Why P1**: Nothing else works until the client can hold a session.

**Acceptance Criteria** (each line is one EARS pattern):

1. WHEN the app starts with a stored refresh token THEN the system SHALL exchange it for an access token and restore the authenticated session without prompting.  <!-- event-driven -->
2. The system SHALL keep the access token in memory only and SHALL persist the refresh token in OS secure storage.  <!-- ubiquitous -->
3. WHEN the user submits valid credentials THEN the system SHALL persist the refresh token, hold the access token in memory, and render the authenticated app.  <!-- event-driven -->
4. WHEN the user registers successfully THEN the system SHALL immediately log in and reach the authenticated app.  <!-- event-driven -->
5. IF a login or registration request fails THEN the system SHALL render the mapped error and keep the auth screen.  <!-- unwanted-behavior -->
6. IF a refresh is rejected because the token family was revoked THEN the system SHALL clear the stored token and return to the auth screen.  <!-- unwanted-behavior -->
7. WHEN the user logs out THEN the system SHALL send the refresh token to `/auth/logout`, clear secure storage, and return to the auth screen.  <!-- event-driven -->

**Independent Test**: With a mocked transport, start with a stored refresh token and assert one refresh then the authenticated app; assert storage holds only the refresh token; log in, register, force a login failure (mapped message, stays), force a revoked-family refresh (returns to auth, storage cleared), and log out (logout call + storage cleared).

---

### P1: Authenticated transport ⭐ MVP

**User Story**: As a player, I want my requests to carry my identity and to recover silently when my access token expires so that I never see a spurious login prompt.

**Why P1**: The backend rejects every unauthenticated call; transport authentication is the mechanism that keeps all screens working.

**Acceptance Criteria**:

1. The system SHALL attach `Authorization: Bearer <access>` to every request to a protected route.  <!-- ubiquitous -->
2. IF a protected request responds `401` THEN the system SHALL refresh once and replay that request once.  <!-- unwanted-behavior -->
3. IF the refresh itself fails THEN the system SHALL force a logout and SHALL NOT replay the request.  <!-- unwanted-behavior -->
4. The system SHALL send the refresh token in the `X-Refresh-Token` header and SHALL NOT rely on cookies.  <!-- ubiquitous -->
5. IF several requests receive `401` while a refresh is in flight THEN they SHALL await the same single refresh.  <!-- unwanted-behavior -->
6. The system SHALL NOT attach an authorization header to `/auth/*` or `/health` requests.  <!-- ubiquitous -->

**Independent Test**: With a recorded transport, assert the header on a protected call and its absence on `/auth/login` and `/health`; simulate a `401` then `200` and assert exactly one refresh and one replay; simulate a failing refresh and assert zero replays and a forced logout; fire two concurrent `401`s and assert a single refresh.

---

### P1: Auth gate and single-sheet resolution ⭐ MVP

**User Story**: As a player, I want the app to show my sheet when I am logged in and ask nothing of me when I am not so that the desktop flow mirrors the one-sheet backend rule.

**Why P1**: The gate is what makes the authenticated app reachable and the server the source of truth for the user's sheet.

**Acceptance Criteria**:

1. WHILE no session is authenticated the system SHALL render the auth screen and SHALL NOT render the catalog, library, or sheet screens.  <!-- state-driven -->
2. WHEN the session becomes authenticated THEN the system SHALL request the user's sheet through `GET /sheets`.  <!-- event-driven -->
3. IF `GET /sheets` responds `404` THEN the system SHALL present the create-sheet flow for the authenticated user.  <!-- unwanted-behavior -->
4. WHEN `GET /sheets` responds `200` THEN the system SHALL open that sheet on Screen 2.  <!-- event-driven -->
5. IF the health check runs while unauthenticated THEN the system SHALL still reach `GET /health` and update the connection indicator.  <!-- unwanted-behavior -->

**Independent Test**: Mount the app with no session and assert only the auth screen renders; authenticate and assert one `GET /sheets`; respond `404` and assert the create flow; respond `200` and assert Screen 2 opens with that id; assert the health indicator still polls unauthenticated.

---

## Edge Cases

Edge cases are usually unwanted-behavior (IF/THEN) or boundary (WHEN) criteria:

- IF secure storage is unavailable THEN the system SHALL keep the in-memory session for the run and SHALL NOT persist the refresh token.  <!-- unwanted-behavior -->
- WHEN an access token expires mid-session THEN the next protected request SHALL refresh and replay with no user-visible error.  <!-- event-driven -->
- IF the user logs out while a protected request is in flight THEN a late response SHALL NOT restore the session.  <!-- unwanted-behavior -->
- WHEN `GET /sheets` fails for a non-`404` reason THEN the system SHALL render the mapped error with Retry and SHALL keep the session.  <!-- event-driven -->
- IF registration rejects the input (short password, duplicate email/username) THEN the system SHALL render the mapped error and keep the entered fields.  <!-- unwanted-behavior -->

---

## Requirement Traceability

Each requirement gets a unique ID for tracking across design, tasks, and validation.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| FE6-01 | P1: Session and token lifecycle | Design | Implementing |
| FE6-02 | P1: Session and token lifecycle | Design | Implementing |
| FE6-03 | P1: Session and token lifecycle | Design | Implementing |
| FE6-04 | P1: Session and token lifecycle | Design | Implementing |
| FE6-05 | P1: Session and token lifecycle | Design | Implementing |
| FE6-06 | P1: Session and token lifecycle | Design | Implementing |
| FE6-07 | P1: Session and token lifecycle | Design | Implementing |
| FE6-08 | P1: Authenticated transport | Design | Implementing |
| FE6-09 | P1: Authenticated transport | Design | Implementing |
| FE6-10 | P1: Authenticated transport | Design | Implementing |
| FE6-11 | P1: Authenticated transport | Design | Implementing |
| FE6-12 | P1: Authenticated transport | Design | Implementing |
| FE6-13 | P1: Auth gate and single-sheet resolution | Design | Pending |
| FE6-14 | P1: Auth gate and single-sheet resolution | Design | Pending |
| FE6-15 | P1: Auth gate and single-sheet resolution | Design | Implementing |
| FE6-16 | P1: Auth gate and single-sheet resolution | Design | Implementing |
| FE6-17 | Edge: secure storage unavailable | Design | Implementing |
| FE6-18 | Edge: mid-session expiry | Design | Implementing |
| FE6-19 | Edge: logout vs in-flight request | Design | Unmapped ⚠️ |
| FE6-20 | Edge: `GET /sheets` non-404 failure | Design | Unmapped ⚠️ |
| FE6-21 | Edge: registration rejection | Design | Implementing |

**ID format:** `FE6-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 21 total, mapped to tasks at Tasks (see tasks.md).

---

## Success Criteria

How we know the feature is successful:

- [ ] Against the protected backend, a fresh install can register → land on the app → create a `fantasy_v1` sheet → edit → save, with no manual token handling.
- [ ] Restarting the app with a stored refresh token returns to the sheet without a login prompt.
- [ ] An expired access token is refreshed transparently; a revoked refresh token forces a clean logout.
- [ ] `flutter test` and `flutter analyze` are clean.

---

## Implicit-Requirement Dimensions Sweep (Large)

| Dimension | Resolution |
| --------- | ---------- |
| Input validation & bounds | FE6-05, FE6-21: backend validation (email, password length, username) is surfaced as mapped errors; the client does not duplicate rules. |
| Failure / partial-failure states | FE6-05, FE6-06, FE6-09, FE6-17, FE6-20: every failure has a defined UI state. |
| Idempotency / retry / duplicate handling | FE6-09, FE6-11: single-flight refresh, one replay per request. |
| Auth boundaries & rate limits | FE6-08, FE6-12, FE6-13, FE6-16: header scoping, auth gate, public health; rate limiting is the backend's Fase 5. |
| Concurrency / ordering | FE6-11, FE6-19: single-flight refresh; late responses after logout are discarded. |
| Data lifecycle / expiry | FE6-02, FE6-07, FE6-17: refresh token lifecycle in secure storage; access token in memory. |
| Observability | Unchanged from V1 (local logs, no token content); the connection indicator stays. |
| External-dependency failure | FE6-09, FE6-20: transport/5xx failures keep the session and offer Retry. |
| State-transition integrity | FE6-01, FE6-03, FE6-06, FE6-13: unauthenticated → authenticating → authenticated → logged out, each guarded. |
