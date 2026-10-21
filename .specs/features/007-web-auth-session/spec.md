# Feature Specification: Web Auth Session

**Feature Branch**: `007-web-auth-session`

**Created**: 2026-10-16

**Status**: Draft

**Input**: User description: "Sign in, register and sign out against the browser cookie session, keeping the access token in memory with a single-flight silent refresh and CSRF-protected auth calls, a session boundary that clears local data, and the signed-in user's sheet list with confirmed deletion"

## Clarifications

### Session 2026-10-16

- Q: Where is the signed-in person's own sheet reached from, and where does the confirmed delete
  live? → A: A new session-only left-navigation entry, "Your sheet" (user decision, 2026-10-16):
  the surface resolves `GET /sheets`, shows the designed empty state when the person owns none, or
  opens the person's sheet on the existing sheet route, and the confirmed delete lives on that
  surface (`WAS-25`…`WAS-29`).
- Q: When someone opens a deep link without a session and then signs in, should the app resume the
  attempted route or always land on the default surface? → A: Always land on the catalog (user
  decision, 2026-10-16): a deep link visited without a session renders the sign-in form and is not
  resumed; the next session starts on the catalog, never on a route from the finished session
  (`WAS-24`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Hold a session no script can persist (Priority: P1)

A person opens the app. If the browser still holds the refresh cookie from a previous
visit, the app silently refreshes and shows the signed-in surfaces without asking for
anything; otherwise it shows the sign-in form. Signing in or registering establishes the
session: the server returns a short-lived access token that the app keeps **in memory
only** and sets the refresh and CSRF cookies. Reloading the tab silently refreshes against
the cookie and returns the person to the signed-in app without retyping; a rejected cookie
returns them to the sign-in form. A failure on the form keeps the typed values and shows
the mapped reason. Every surface behind the session is unreachable without one.

**Why this priority**: it is the doorway to the whole client — the catalog, the sheet and
the user's own sheet are all behind the session, and keeping the access token in memory
while the refresh credential stays an `HttpOnly` cookie is the exact reason the deprecated
desktop was replaced.

**Independent Test**: drive login, registration and a cold start (no in-memory token) with
a valid cookie, a rejected cookie and a transient failure; assert the session state, that
no credential lands in any JavaScript-readable storage, that a rejected cookie shows the
sign-in form, and that the catalog is not rendered while no session is held.

**Acceptance Scenarios**:

1. **Given** valid credentials, **When** login succeeds, **Then** the app holds the
   returned access token in memory and redirects to the signed-in surfaces, and no token
   is written to `localStorage`, `sessionStorage` or IndexedDB. (WAS-01)
2. **Given** a valid new account, **When** registration succeeds, **Then** the app
   establishes the session from that response without issuing a second login. (WAS-02)
3. **Given** the app starts holding no in-memory access token and the browser holds a
   valid refresh cookie, **When** it resolves the session, **Then** it issues exactly one
   silent refresh and shows a restoring state until it settles. (WAS-03)
4. **Given** the silent refresh is answered 401, **When** it settles, **Then** the app
   shows the sign-in form and holds no session. (WAS-04)
5. **Given** the silent refresh fails transiently (offline, 5xx, unreadable response),
   **When** it settles, **Then** the app shows the sign-in form, keeps the session-local
   data, and does not treat the failure as a session end. (WAS-05)
6. **Given** no session is held, **When** the app renders any surface, **Then** only the
   sign-in form is rendered and the catalog, the sheet and the user's-sheet surfaces are
   not. (WAS-08)
7. **Given** an invalid credential, **When** login fails, **Then** the form keeps the
   typed values and shows the mapped failure copy inline, and no session is established.
   (WAS-07, WAS-10)
8. **Given** a submit is in flight, **When** the form renders, **Then** the single primary
   action is disabled and progress is shown; the password field is a hidden-content
   control. (WAS-09)

---

### User Story 2 - A lapsed access token recovers without a second rotation (Priority: P1)

Once signed in, every read of a template or a sheet carries the access token. When the
token has lapsed, the server refuses the call once; the app silently refreshes against the
cookie and replays the refused call exactly once. Several calls that lapse together share
one refresh, so the session is rotated once, not once per call. A refresh the server
definitively rejects ends the session; a transient refresh does not. The auth and health
endpoints never carry the access token and never trigger a refresh.

**Why this priority**: silent recovery is what makes a short-lived access token usable;
without it every expiry would throw the person back to the sign-in form, and without
single-flight a burst of refreshes would race the session's own rotation.

**Independent Test**: issue a protected call that answers 401 and assert one refresh and
one replay; fire several concurrent 401s and assert a single refresh; drive a rejected
refresh and assert the session ends; drive a transient refresh and assert it survives;
assert `/auth/*` and `/health` are never authenticated.

**Acceptance Scenarios**:

1. **Given** a session is held, **When** a `/sheets` or `/templates` call is sent, **Then**
   it carries `Authorization: Bearer <access>` and sends no cookie. (WAS-11)
2. **Given** a protected call is answered 401, **When** the app handles it, **Then** it
   performs one silent refresh and replays the call once with the refreshed token.
   (WAS-12)
3. **Given** several protected calls are refused 401 at the same time, **When** they
   recover, **Then** they share a single refresh and the family is rotated once. (WAS-13)
4. **Given** the refresh is answered 401, **When** it settles, **Then** the session ends,
   the in-memory token is dropped, the session-local data is cleared, and the original
   refusal surfaces. (WAS-14)
5. **Given** the refresh fails transiently, **When** it settles, **Then** the session and
   the session-local data survive and the failure surfaces as transient. (WAS-15)
6. **Given** a `/health` or `/auth/*` call, **When** it is sent, **Then** it carries no
   Bearer header and triggering a refresh is impossible. (WAS-16)
7. **Given** a 401 or a refresh that belongs to a session which has since ended, **When**
   it returns, **Then** it neither ends nor overwrites the current session. (WAS-17)
8. **Given** a refresh cookie is present, **When** refresh or logout is sent, **Then** the
   request echoes the CSRF token read from the script-readable cookie in the
   `X-CSRF-Token` header. (WAS-18)

---

### User Story 3 - Signing out leaves nothing of the session behind (Priority: P1)

A person signs out, or their session is ended by a refresh the server rejected. Either way
the app drops the in-memory token, returns to the sign-in form, and clears the local
records the session owned — the cached sheets and the unsaved drafts — so that whoever
signs in next cannot see the previous account's sheet. A transient failure is not a
session end and keeps those records. Signing out twice in one frame asks the server once.

**Why this priority**: it is the account-switch hazard the desktop shipped a whole feature
to fix (`AD-010`): local data of a finished session must never be visible to the next one.

**Independent Test**: sign in, cache a sheet and leave a draft, sign out, and assert the
records are gone and the sign-in form is shown; repeat through a forced logout; drive a
transient refresh failure and assert the records survive; activate sign-out twice and
assert one `/auth/logout`.

**Acceptance Scenarios**:

1. **Given** a session, **When** sign-out is activated, **Then** the app issues
   `/auth/logout` best-effort, drops the in-memory token, shows the sign-in form, and
   clears the session-local data. (WAS-19)
2. **Given** a forced logout ends the session, **When** it completes, **Then** the
   session-local data is cleared exactly as sign-out clears it. (WAS-20)
3. **Given** the session-local data holds cached sheets and drafts, **When** a definitive
   session end clears it, **Then** every cached sheet and every draft is removed and a
   failing store never blocks the session end. (WAS-21)
4. **Given** a session ended, **When** the next session renders, **Then** no cached sheet
   and no draft of the finished session is visible. (WAS-22)
5. **Given** a sign-out is in flight, **When** sign-out is activated again, **Then** at
   most one `/auth/logout` is issued. (WAS-23)
6. **Given** a session ends while a sheet route is open, **When** the app settles, **Then**
   the sign-in form is shown, and the next session starts on the catalog rather than on a
   route from the finished session. (WAS-24)

---

### User Story 4 - The signed-in person's own sheet is reachable and deletable (Priority: P2)

The server scopes sheets to their owner and holds at most one per person. After signing
in, a persistent left-navigation entry — "Your sheet" — resolves the person's own sheet:
if they own none, they see a plain empty line rather than an error; if they own one, the
entry surfaces it and opens it on the ordinary sheet route. Deleting the sheet asks for
explicit confirmation of the exact consequence first, then removes it on the server and
from the local copy, leaving the empty state.

**Why this priority**: it completes the authenticated journey the plan assigns to this
feature (`F7-T006`: `GET /sheets` plus a confirmed `DELETE /sheets/:id`) and removes the
client's last dependence on a local "my sheets" list as a source of truth, but the session
itself is usable without it.

**Independent Test**: sign in as a person with a sheet and assert the left-nav "Your
sheet" entry resolves it with one `GET /sheets` from the server; sign in with none and
assert the empty state; drive the delete with a cancel (zero requests) and a confirm (one
request) and assert the local copy and draft are gone and the empty state renders; drive a
delete failure and assert the sheet is kept with the mapped message.

**Acceptance Scenarios**:

1. **Given** a session, **When** the "Your sheet" destination resolves the user's sheet,
   **Then** it comes from the server (`GET /sheets`) and the server is treated as the
   source of truth. (WAS-25)
2. **Given** the person owns no sheet, **When** the surface resolves, **Then** it shows
   the designed empty state, distinct from loading and from failure. (WAS-26)
3. **Given** resolving the sheet fails for any reason other than "no sheet", **When** it
   settles, **Then** the surface shows the mapped failure with a retry that re-requests
   exactly once. (WAS-27)
4. **Given** a person activates delete, **When** the confirmation is open, **Then** cancel
   issues no request and confirm issues exactly one `DELETE /sheets/:id`. (WAS-28)
5. **Given** the delete succeeds, **When** it settles, **Then** the sheet and its draft are
   removed locally and the empty state is shown; on failure the sheet is kept and the
   mapped message is shown. (WAS-29)

---

### Edge Cases

- **Cold start with no cookie**: the silent refresh is answered 401 (or the CSRF cookie is
  absent); the app shows the sign-in form with no session and issues no further refresh.
  (WAS-04)
- **Cold start with a transient failure**: offline, 5xx or an unreadable refresh response
  shows the sign-in form but keeps the session-local data; the next load retries the
  silent refresh. (WAS-05)
- **Deep link without a session**: a route such as `/sheets/abc` visited while signed out
  renders the sign-in form; after sign-in the app starts on the catalog, and the attempted
  route is not resumed. (WAS-24)
- **Concurrent 401s**: N refused calls share one refresh; the family rotates once. (WAS-13)
- **Refresh rejected mid-flight**: the session ends once, the in-memory token is dropped,
  local data is cleared, and the refused call's error still reaches its caller. (WAS-14)
- **Late response from a finished session**: a 401 or refresh belonging to the previous
  session neither ends nor overwrites the current one. (WAS-17)
- **CSRF cookie absent**: refresh/logout are still attempted with whatever the client can
  read; the server's 403 is mapped, never a silent local failure. (WAS-18)
- **Rate limited**: the server's 429 is mapped to its own kind with copy that tells the
  person to retry shortly; it is never rendered as a generic failure. (WAS-30)
- **Sign-out during pending edits**: leaving the sheet with unsaved work is already guarded
  (`WSC-37`); sign-out clears the session's draft records regardless. (WAS-21)
- **Sign-out activated twice**: at most one `/auth/logout`. (WAS-23)
- **No sheet vs. server failure**: "no sheet" renders the empty state; any other failure
  renders the mapped message with a retry. (WAS-26, WAS-27)
- **Foreign or missing sheet id**: the server scopes by owner, so a sheet that is not the
  caller's resolves as missing and is never rendered as the user's sheet. (WAS-25)
- **Delete canceled**: zero `DELETE` requests; the sheet stays. (WAS-28)
- **Credential leakage**: neither the access token nor the CSRF token appears in a local
  store, a log record, or a URL. (WAS-32)
- **Markup in a server string** (e.g. a failure message carrying `<script>`): rendered as
  text. (WAS-31)
- **Dimension sweep** (explicitly covered / not applicable here):
  - *Input validation and bounds* — the form's required fields and the mapped failure copy
    (WAS-07, WAS-10); the server owns credential validation.
  - *Failure and partial failure* — login/registration failure (WAS-07), rejected and
    transient refresh (WAS-04, WAS-05, WAS-14, WAS-15), no-sheet vs. failure (WAS-26,
    WAS-27), delete failure (WAS-29).
  - *Idempotency and retry* — one silent refresh per cold start (WAS-03), one refresh per
    burst of 401s (WAS-13), one replay (WAS-12), one `/auth/logout` (WAS-23), one delete
    per confirm (WAS-28).
  - *Concurrency and ordering* — single-flight refresh (WAS-13), single-flight logout
    (WAS-23), the session-version guard (WAS-17).
  - *State-transition integrity* — a definitive session end clears local data (WAS-19 …
    WAS-22) while a transient failure keeps it (WAS-05); the access token is memory-only
    (WAS-01).
  - *Auth boundaries* — every surface requires a session (WAS-08); `/health` and `/auth/*`
    are never Bearer-authenticated (WAS-16); the refresh/logout routes carry CSRF
    (WAS-18).
  - *Not applicable* — the offline service-worker cache and offline reading (F8), the
    server-side rate limit itself (F6 owns it; the client only maps 429), and password
    reset / account switching (not in the program plan).

## Requirements *(mandatory)*

### Functional Requirements

**Establishing a session and keeping the credential in memory**

- **WAS-01**: WHEN login succeeds, the client MUST hold the returned access token in
  memory only, MUST NOT persist any credential in `localStorage`, `sessionStorage` or
  IndexedDB, and MUST attach the token to the protected calls that follow.
- **WAS-02**: WHEN registration succeeds, the client MUST establish the session from the
  registration response (the backend signs the browser in on registration) and MUST NOT
  issue a follow-up login. **Superseded by `TFA-02`** (`.specs/features/018-auth-2fa-session-gate/spec.md`): while the rollout gate is on, the registration response carries a code step and no session, so the client premise "registration authenticates" no longer holds. The client half of that change is delivered by **`WCS-01`**
  (`.specs/features/019-web-auth-code-step/spec.md`): the client accepts either answer, establishes no
  session from a code step and renders the code surface instead. This requirement therefore stands for
  the gate-disabled behavior only.
- **WAS-03**: WHEN the app starts or resumes holding no in-memory access token, the client
  MUST attempt exactly one silent refresh before deciding the session state and MUST show
  a restoring state, not the sign-in form, while it is in flight.
- **WAS-04**: IF the silent refresh is answered 401, THEN the client MUST show the
  sign-in form holding no session.
- **WAS-05**: IF the silent refresh fails transiently (offline, server error, unreadable
  response), THEN the client MUST show the sign-in form without a session, MUST keep the
  session-local data, and MUST NOT treat the failure as a definitive session end.
- **WAS-06**: The login, registration, refresh and logout calls MUST send credentials on
  the auth routes so the refresh cookie is attached, and a credential MUST NOT appear in a
  request URL or in any log record.
- **WAS-07**: IF login or registration fails, THEN the client MUST keep the entered
  values, MUST show the mapped failure copy inline on the auth surface, and MUST NOT
  establish a session.
- **WAS-08**: WHILE no session is held, the client MUST render the sign-in form and MUST
  NOT render the catalog, the sheet surface, or the user's-sheet surface.
- **WAS-09**: The auth surface MUST offer sign-in and registration in one place, MUST
  submit through the session through a single primary action that is disabled and shows
  progress while a submit is in flight, and MUST render the password as a hidden-content
  control.
- **WAS-10**: The auth surface's failure copy MUST be resolved from the shared translation
  catalog, MUST NOT reveal whether an email exists, and MUST render server text as text.

**Authenticated calls and the silent refresh**

- **WAS-11**: WHEN a session is held, calls to `/sheets` and `/templates` MUST carry
  `Authorization: Bearer <access>` and MUST NOT send the refresh cookie, so those routes
  carry no CSRF concern.
- **WAS-12**: WHEN a protected call is answered 401, the client MUST perform one silent
  refresh and replay that request exactly once with the refreshed token.
- **WAS-13**: Concurrent 401s MUST share a single in-flight refresh, so the session is
  rotated once rather than once per refused call.
- **WAS-14**: IF the refresh is answered 401, THEN the client MUST end the session (forced
  logout), drop the in-memory token, clear the session-local data, and surface the original
  refusal to its caller.
- **WAS-15**: IF the refresh fails transiently, THEN the client MUST NOT end the session
  and MUST NOT clear the session-local data, and the failure MUST surface as transient.
- **WAS-16**: The client MUST NOT attach a Bearer header to `/health` or `/auth/*`, and
  those routes MUST NOT trigger a refresh.
- **WAS-17**: A 401 or a refresh result that belongs to a session that has since ended MUST
  NOT end nor overwrite the current session (`AD-010`, desktop `SES-01`/`SES-02`).
- **WAS-18**: The refresh and logout calls MUST echo the CSRF token in an `X-CSRF-Token`
  header, resolved at request time from the script-readable `__Host-legnaris_csrf` cookie;
  the client MUST NOT read the `HttpOnly` refresh cookie.

**Sign-out and the session boundary**

- **WAS-19**: WHEN sign-out is activated with a session, the client MUST issue
  `/auth/logout` best-effort, drop the in-memory token, show the sign-in form, and clear
  the session-local data.
- **WAS-20**: WHEN a forced logout ends the session, the client MUST clear the
  session-local data exactly as sign-out does.
- **WAS-21**: The session-local data swept at a definitive session end MUST be every cached
  sheet and every draft, and the sweep MUST be best-effort so a failing store never blocks
  the session end.
- **WAS-22**: After a session ends, the next session MUST NOT be able to render any cached
  sheet or draft of the finished session.
- **WAS-23**: Sign-out MUST be single-flight: an activation while one is in flight MUST
  NOT issue a second `/auth/logout`.
- **WAS-24**: WHEN a session ends, the client MUST return to the sign-in form and the next
  session MUST start on the catalog rather than on a route from the finished session; a
  deep link visited without a session MUST render the sign-in form and MUST NOT be resumed
  after sign-in.

**The signed-in user's own sheet**

- **WAS-25**: WHILE a session is held, the client MUST expose a session-only "Your sheet"
  destination in the persistent navigation that resolves the user's own sheet from the
  server (`GET /sheets`) — never from the local cache — and MUST treat the server as the
  source of truth; a sheet that is not the caller's MUST NOT be rendered as the user's
  sheet.
- **WAS-26**: WHEN the "Your sheet" destination resolves and the server answers that none
  exists, the client MUST show the designed empty state, distinct from a loading state and
  from a failure.
- **WAS-27**: IF resolving the user's sheet fails for any reason other than "no sheet",
  THEN the destination MUST show the mapped failure with a retry that re-requests exactly
  once.
- **WAS-28**: WHEN the user deletes their sheet, the client MUST confirm first with a
  dialog stating the exact consequence, and only on confirm MUST issue `DELETE
  /sheets/:id`; on cancel it MUST issue no request.
- **WAS-29**: WHEN the delete succeeds, the client MUST remove the sheet from the local
  cache together with its draft and show the empty state; IF it fails, THEN the client MUST
  show the mapped failure and keep the sheet.

**Cross-cutting**

- **WAS-30**: The client MUST give the session codes the backend emits an interface-facing
  classification and copy — at least 401 (session/unauthorized) and 429 (rate limited,
  with copy that tells the person to retry shortly) — and MUST leave a code it does not
  classify on the generic kind rather than misclassifying it.
- **WAS-31**: Every user-visible string the feature renders MUST resolve through the shared
  translation catalog and every visual value from the shared token set; a value containing
  markup MUST render as text and the feature MUST NOT introduce a DOM-injection sink.
- **WAS-32**: The access token and the CSRF token MUST NOT appear in any local store, log
  record, or URL.
- **WAS-33**: Every requirement MUST ship tests — edge cases first, then a representative
  happy path — and interface requirements MUST be verified through rendering tests rather
  than assumed.

### Key Entities

- **Session**: the in-memory access token plus the session version counter and the state —
  restoring, unauthenticated, authenticating, authenticated. The token is never persisted;
  the version increments at every definitive session boundary (login, registration,
  logout, forced logout) and never when a refresh applies.
- **Silent refresh**: the one `POST /auth/refresh` call, credentials included and CSRF
  echoed, that exchanges the browser's refresh cookie for a fresh access token; shared by
  every concurrent 401 through a single in-flight operation.
- **Authenticated transport**: the seam that attaches `Authorization: Bearer` to the
  protected routes, never to `/auth/*` or `/health`, and drives the single-flight refresh
  and the exactly-once replay.
- **CSRF token**: the value read from the script-readable `__Host-legnaris_csrf` cookie and
  echoed in `X-CSRF-Token` on refresh and logout; never read from the `HttpOnly` refresh
  cookie and never stored.
- **Session-local data**: the cached sheets and the unsaved drafts of the current session;
  swept at a definitive session end and kept across a transient failure.
- **The user's own sheet**: the single sheet the server scopes to the caller and returns
  from `GET /sheets`, or none; the server, never the local cache, is its source of truth.
- **"Your sheet" destination**: the session-only persistent-navigation entry that resolves
  the user's own sheet — the empty state when none, the sheet with an Open action when one;
  it is also where the confirmed delete lives (`WAS-25`…`WAS-29`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Across login, registration, cold start, refresh, logout and account switch, 0
  credentials are written to `localStorage`, `sessionStorage` or IndexedDB.
- **SC-002**: A cold start with a valid cookie restores the session with exactly 1 refresh
  and 0 credential prompts; a rejected cookie shows the sign-in form with 0 tokens held.
- **SC-003**: N protected calls refused 401 together produce exactly 1 refresh and 1 replay
  each, and rotate the session once.
- **SC-004**: 0 requests to `/health` or `/auth/*` carry a Bearer header, and 100% of
  refresh/logout requests carry the CSRF header whenever the cookie is readable.
- **SC-005**: After a definitive session end, 0 cached sheets and 0 drafts of that session
  remain, and the next session renders 0 of them.
- **SC-006**: After a transient refresh failure, 0 session-local records are removed.
- **SC-007**: A delete is confirmed first: 0 `DELETE` requests on cancel and exactly 1 on
  confirm.
- **SC-008**: A person who owns no sheet sees the empty state, and every failure situation
  shows its mapped copy — 0 blank, half-rendered or crash states.
- **SC-009**: 100% of new failure paths assert the mapped classification/copy, and 0 suites
  are non-vacuous.
- **SC-010**: The auth surface, the signed-in shell and the user's-sheet surface report 0
  critical automated accessibility violations, and every control exposes an accessible
  role and name.

## Assumptions

- This feature consumes the delivered backend contract (`006-backend-browser-auth-cookies`),
  the blocking prerequisite the program plan names: `POST /auth/register` (201) and
  `POST /auth/login` (200) return a session body — `access_token`, `token_type`,
  `expires_in`, and **no** refresh token — and set the `HttpOnly`
  `__Host-legnaris_refresh` and script-readable `__Host-legnaris_csrf` cookies;
  `POST /auth/refresh` reads the cookie plus `X-CSRF-Token`, rotates both cookies and
  returns a new session body, answering 401 on an absent/unknown/rotated credential and
  403 on a bad CSRF token or origin; `POST /auth/logout` reads the same pair, revokes the
  family, clears both cookies and answers 204.
- Registration signs the browser in (F6 decision), so this feature's registration issues a
  single request, unlike the deprecated desktop's register-then-login composition.
- The app is served at `app.legnaris.com.br` and the API at `api.legnaris.com.br` (shared
  registrable domain, cross-origin but same-site), so `SameSite=Strict` still sends the
  refresh cookie on the credentialed auth calls and the configured frontend origin governs
  CORS (`AUTH-14`…`AUTH-16`).
- Every data surface sits behind the session, per the program plan's "auth is mandatory":
  the F1 and F3 suites that assumed a publicly reachable catalog are retargeted to render
  behind the session rather than deleted (the pattern already used by `AD-013`).
- The F2 client's fixed `credentials: 'omit'` posture is widened only for the four
  `/auth/*` sessions calls (which need the cookie); `/sheets` and `/templates` keep
  `omit` and authenticate with the Bearer header, so no CSRF concern is introduced outside
  the auth routes. The transport remains the single place a credential is attached.
- No new third-party dependency is expected: the work uses the existing `fetch` transport,
  the existing TanStack Query cache, and the shared translation/token modules; the CSRF
  token is read from the cookie the server already sets.
- The offline read path and the service-worker cache belong to the PWA feature (F8); this
  feature only **clears** the IndexedDB cache and drafts at a session boundary.
- The server holds at most one sheet per owner, so "the user's sheet" is a single
  owner-scoped read (`GET /sheets` → 404 when none) rather than a list.
- The signed-in person's own sheet is reached through a session-only "Your sheet" entry in
  the persistent navigation, and the confirmed delete lives on that surface (user decision,
  2026-10-16; `WAS-25`…`WAS-29`); the catalog keeps its template role only.
- A deep link visited without a session renders the sign-in form and is not resumed after
  sign-in — the next session starts on the catalog (user decision, 2026-10-16; `WAS-24`).
- A failed login/registration under the client is still answered with the server's mapped
  401/409; the client maps the new session codes (`401`, `429`) to interface-facing kinds
  in the single `error_mapper` and leaves unmapped codes on the generic kind.
- Continuous integration already runs the client suite, and the requirement-ID lint covers
  TypeScript sources, so this feature introduces no new gate.
- All identifiers, comments, log messages and user-facing strings remain English-only, per
  the project constitution.
