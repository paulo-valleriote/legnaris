# Phase 0 Research: Web Auth Session

**Feature**: `.specs/features/007-web-auth-session/` | **Date**: 2026-10-16

Every decision below answers an open question in the feature's Technical Context or resolves a choice
the program plan's F7 task list (`docs/plan-web-pwa.md`, Parte B F7) leaves implicit. No
`NEEDS CLARIFICATION` remains. The desktop originals — `desktop/lib/auth/` and
`desktop/lib/api/authenticated_client.dart` — are the rule source; this feature re-expresses them on
the web stack against the delivered F6 cookie contract, it does not redesign them. Two deliberate
deviations from the desktop are recorded here: registration signs the browser in with **one** request
(the backend F6 decision), and the access token has **no** token store (the refresh credential is an
`HttpOnly` cookie the script never reads).

---

## R1. One shared request helper, and the widened credential posture

**Decision**: extract the F2 request machinery (`attempt` + `call`: the fail-fast timeout, the single
GET retry, the envelope decode, the `DomainFailure`/`DefensiveParseFailure` classification) out of
`client.ts` into `web/src/api/request.ts` as `createCaller({ baseUrl, transport, timeoutMs,
retryDelayMs, sleep })` returning `call(method, path, body, decode, options)`. `options` carries
`credentials` (`'omit'` by default, `'include'` for the auth calls) and `headers` (the CSRF echo).
`client.ts` and `auth_api.ts` both build on this one helper. `transport.ts` widens
`HttpRequest.credentials` from the literal `'omit'` to `'omit' | 'include'` and adds `'DELETE'` to
`HttpMethod`.

**Rationale**: F2 fixed one transport as "the single place a credential will ever be attached"
(`API-07`, `API-29`). F7's auth calls need the same transport with a different credential posture and
an extra header; duplicating the timeout/retry/decode logic in `auth_api.ts` would be the exact
`dupl` the review flags, and giving the auth client its own transport would break the one-seam rule.
Extracting the helper keeps `client.ts` a thin operation table, keeps the auth client over the same
injected transport (so `fake_transport.ts` counts every attempt in one place), and makes the
`credentials` posture an explicit per-request value rather than a second hard-coded constant.

**Alternatives considered**:

- *Give `ApiClient` the four auth methods* — rejected: the auth routes are public and cookie-driven,
  the domain operations are Bearer-driven; folding both into one interface hides the posture
  difference and couples the auth lifecycle to the domain client.
- *A second transport for `/auth/*`* — rejected: it breaks the F2 one-seam rule and the suites would
  have to count attempts across two injectables.
- *Keep `credentials: 'omit'` and let the browser attach the cookie automatically* — rejected: cross-
  origin XHR/fetch sends no cookie without `credentials: 'include'`, so the refresh cookie would
  never be attached (WAS-06).

## R2. The auth operations module (`auth_api.ts`)

**Decision**: add `web/src/api/auth_api.ts` with an `AuthApi` interface and `createAuthApi(options)`
over the R1 caller and the injected `Transport`:

| Method | Request | Success | Notes |
|---|---|---|---|
| `register(input)` | `POST /auth/register`, `credentials: 'include'` | `201` + `SessionBody` | signs the browser in; no follow-up login (WAS-02) |
| `login(input)` | `POST /auth/login`, `credentials: 'include'` | `200` + `SessionBody` | (WAS-01) |
| `refresh()` | `POST /auth/refresh`, `credentials: 'include'`, `X-CSRF-Token` | `200` + `SessionBody` | reads the cookie; rotates (WAS-18) |
| `logout()` | `POST /auth/logout`, `credentials: 'include'`, `X-CSRF-Token` | `204`, empty body | no body to decode (WAS-19) |

`SessionBody` is `{ accessToken, tokenType, expiresIn }` decoded defensively from
`{ data: { access_token, token_type, expires_in } }`; the CSRF header comes from R5 and is resolved
at request time. A `204` decodes through a no-content path (the envelope decoder requires a body), so
logout returns `void`.

**Rationale**: the four calls share the base address, the transport, the timeout and the decode, so
one module keeps them together and keeps the credentials/CSRF posture visible. The plan named this
file (`F7-T001`); the desktop's `auth_api_client.dart` is the unit being ported. The backend F6
contract fixes the wire shapes (`contracts/auth-session.contract.md`), and envelope decoding stays
the F2 decoder, so a drift still surfaces as a `DefensiveParseFailure`.

**Alternatives considered**:

- *Fold register/login/refresh/logout into the transport decorator* — rejected: the decorator should
  not know the four routes; it only attaches/repairs credentials.
- *Have `login()` compose `register()` then `login()` like the desktop* — rejected: F6 signs the
  browser in on registration, so a second request would be a needless rotation (WAS-02).

## R3. The session module (`auth_session.ts`) — memory-only token, no token store

**Decision**: add `web/src/auth/auth_session.ts` holding the session as a small mutable
`AuthSession` (plain TypeScript) emitting immutable snapshots for `useSyncExternalStore` — the F5
store pattern, no state library:

- `accessToken: string | null`, never persisted (WAS-01, WAS-32).
- `version: number`, incremented **only** at a definitive boundary (login, registration, logout,
  forced logout) and **never** when a refresh applies a rotated token (`AD-010`, WAS-17).
- `state: 'restoring' | 'unauthenticated' | 'authenticating' | 'authenticated'`.
- `restore()` — one silent refresh on a cold start, showing `restoring` (WAS-03); a `401` ends at
  `unauthenticated` (WAS-04); a transient failure ends at `unauthenticated` **keeping** the
  session-local data (WAS-05).
- `login(input)` / `register(input)` — set `authenticating`, call the `AuthApi`, then apply the
  session body; a failure returns to `unauthenticated` and rethrows the mapped error (WAS-07).
- `refresh()` — the single method the transport decorator drives: returns the fresh token, or `null`
  on a definitive `401`, and rethrows a transient failure (WAS-12, WAS-14, WAS-15).
- `logout()` — single-flight (R6 companion, WAS-23): best-effort `/auth/logout`, drop the token, flip
  to `unauthenticated`, sweep the session-local data (WAS-19).
- `onUnauthorized()` — the forced logout the decorator calls on a rejected refresh (WAS-14).

The F7 task's `web/src/auth/session_store.ts` becomes `auth_session.ts`, because there is no stored
token to read: the refresh credential is the `HttpOnly` cookie (recorded deviation).

**Rationale**: the desktop `AuthSession` holds the same fields and states; the one field the web does
not need is the `SecureTokenStore`, because the cookie replaces the stored refresh token, and the one
complication the desktop added — persisting the refresh token — disappears entirely. The version
counter is the web expression of `AD-010` and exists for exactly the reason the desktop added it: a
token comparison breaks concurrent-`401` refresh sharing after rotation, while a boundary-only
version does not (`R4`).

**Alternatives considered**:

- *Persist the access token in `sessionStorage` to survive a reload* — rejected: WAS-01 forbids
  persisting a credential; a reload silently refreshes against the cookie instead.
- *Riverpod-style provider family* — rejected: the web uses no state library (`AD-001`, `AD-016`); a
  `useSyncExternalStore` store behind a React context is the F5 shape.
- *A `token_store.ts` mirroring the desktop* — rejected: there is no token to store; a store module
  would be dead code.

## R4. The authenticated transport decorator

**Decision**: add `web/src/api/authenticated_transport.ts`:
`createAuthenticatedTransport({ inner, session })` returns a `Transport` that

1. attaches `Authorization: Bearer <access>` to a request whose path is **not** `/health` and does
   not start with `/auth/` (WAS-11, WAS-16);
2. captures `session.version()` before sending;
3. on a `401` from a protected path, drops the response unchanged if the version changed in between
   (a finished session gets neither refresh nor replay — WAS-17);
4. otherwise runs **one** refresh shared by every concurrent `401` through a single in-flight future
   (WAS-13);
5. if the refresh returns a token and the version is still current, replays the request **once** with
   the new Bearer and returns that response (WAS-12);
6. if the refresh is rejected (`null`) and the version is still current, calls
   `session.onUnauthorized()` and returns the original `401` so the client surfaces the refusal
   (WAS-14); a transient refresh failure propagates so the session survives (WAS-15).

The domain `ApiClient` is built over this decorator; the `AuthApi` (R2) is built over the raw
transport, so `/auth/*` never sees a Bearer and can never trigger a refresh. The decorator keeps the
path guard anyway, so `WAS-16` is assertable at one seam regardless of which client a future feature
uses.

**Rationale**: the desktop implemented this as an `http.BaseClient` decorator
(`authenticated_client.dart`) around the same session, and F2 made the transport the one network
seam; a transport decorator is the direct web translation. Replaying is trivial here because an
`HttpRequest` carries its body as a string, so the replay is a shallow clone with a fresh header —
no streamed-request caveat like the desktop's `http.Request` cast. The version check is the desktop's
`SES-01`/`SES-02` guard, kept verbatim.

**Alternatives considered**:

- *Refresh inside each operation* — rejected: it would duplicate the single-flight, the replay and
  the guard in every method and race the rotation.
- *A `401` interceptor in the React layer* — rejected: the client must recover without a screen, and
  the transport is the only seam that sees every attempt.

## R5. The CSRF token (read, echo, never store)

**Decision**: add `web/src/api/csrf.ts` — `readCsrfToken(cookieString: string): string | undefined`,
a total, side-effect-free parser that finds `__Host-legnaris_csrf` in a cookie string and returns its
value, or `undefined`. The app passes `document.cookie`; the suites pass a literal. The `AuthApi`
resolves the token at request time and sets `X-CSRF-Token` on `refresh` and `logout` only. An absent
token sends no header (the server's `403` is mapped — `R7`); the `HttpOnly` refresh cookie is never
read.

**Rationale**: the CSRF cookie is deliberately script-readable (`csrf.contract.md`, AUTH-09) so the
page echoes it; reading it is a one-line, testable function rather than a `js-cookie` dependency. The
value is a per-request read, never cached, so a rotation between calls is picked up automatically
(WAS-18), and it is never written to a store or a log (WAS-32). The parser must not throw on a
malformed cookie string, matching the feature's total-input rule.

**Alternatives considered**:

- *A cookie-parsing dependency* — rejected: no new package without strict necessity, and the parse is
  a predicate over one name.
- *Read the CSRF token once at login and hold it in memory* — rejected: a refresh rotates it, so a
  cached value would be stale on the next logout; the per-request read is correct and cheap.

## R6. The session boundary and the local-data sweep

**Decision**: extend the F5 `SheetStore` port with `clearAll(): Promise<void>` — implemented by the
in-memory double (empty both maps) and by the `idb` adapter (clear both object stores) — and add
`web/src/library/session_data.ts` exporting `clearSessionData(store: SheetStore): Promise<void>`
that runs `bestEffort(() => store.clearAll())`. `AuthSession.logout()` and `onUnauthorized()` call it
at the state flip, and **only** there: a transient refresh failure (`WAS-05`, `R4` step 6 without a
`401`) never calls it, so the session-local data survives (`WAS-05`, `WAS-15`, `SC-006`).

**Rationale**: `AD-010` is the account-switch hazard — local data of a finished session must never be
visible to the next one — and F5 already put every session-local record behind one port
(`sheets` + `drafts`), so the sweep is one clear-all over that port rather than a second store or a
walk of unknown keys. The desktop's synchronous two-store sweep (`_clearSessionLocalData`) becomes one
best-effort async call because the web store is asynchronous; the guarantee that matters — a failing
store never blocks the session end and never skips the flip — is preserved by `bestEffort` and by
ordering the sweep after the state flip (`WAS-21`). The F5 envelopes hold no credential, so clearing
them cannot leave a token behind either way (`WAS-31`, `WAS-32`).

**Alternatives considered**:

- *Delete each cached sheet by id* — rejected: the store has no key enumeration and the session owns
  unknown keys (a draft for a sheet never opened this run), so a per-record walk would miss records.
- *Clear the whole IndexedDB database* — rejected: `deleteDatabase` would also drop the future
  service-worker state and is broader than the session's own records.
- *Sweep on every `unauthenticated` state* — rejected: a transient refresh failure also ends at
  `unauthenticated` but must **keep** the data (`WAS-05`); the sweep is tied to a **definitive**
  boundary, not to the state value.

## R7. Error classification: the three session codes

**Decision**: add three kinds to `FailureKind` (`web/src/api/errors.ts`) and three rows to the mapping
table (`web/src/api/error_mapper.ts`):

| Code | Kind | Copy key |
|---|---|---|
| `UNAUTHORIZED` (`401`) | `unauthorized` | `api.error.unauthorized` |
| `FORBIDDEN` (`403`) | `forbidden` | `api.error.forbidden` |
| `RATE_LIMITED` (`429`) | `rateLimited` | `api.error.rateLimited` |

Every other code — the F2 five, the F6 `CONFLICT` (`409`), and any code the backend does not emit —
keeps the existing behaviour and falls to `unknown`, so an unclassified code is visible rather than
misclassified (WAS-30).

**Rationale**: F6 added `UNAUTHORIZED`/`FORBIDDEN`/`RATE_LIMITED` to the `apperr` taxonomy; F2's
mapper does not know them, so without this they would all render "Something went wrong". `429`
carries the server's retry hint, and the spec asks for its own kind and copy (WAS-30, SC-008); `403`
arises on a CSRF/origin refusal and must not be swallowed locally (spec edge case); `409` is left
generic on purpose, so the auth surface never reveals whether an email already exists (WAS-10). The
mapping table stays total over the codes the backend emits (the F2 mapper's rule).

**Alternatives considered**:

- *Map `FORBIDDEN` to `unauthorized`* — rejected: a CSRF refusal is not a session end, and a shared
  kind would invite that misreading.
- *Give `CONFLICT` a dedicated "email exists" copy* — rejected: WAS-10 forbids revealing whether an
  email exists, so the generic copy is the correct behaviour.

## R8. The auth gate, the route audiences and the deep-link reset

**Decision**: add `web/src/auth/auth_gate.tsx`. It runs `restore()` exactly once on mount, renders a
minimal restoring state until that first restore settles (WAS-03), renders `AuthScreen` when
`unauthenticated`, and renders the route tree inside `AppShell` when `authenticated`. Every declared
data route (catalog, sheet, "Your sheet") is `audience: 'session'` in `routes.ts`, and the not-found
fallback is gated too, so a deep link visited without a session renders the sign-in form and is not
resumed — the next session starts on `/` (WAS-08, WAS-24). The gate holds no per-route memory: after
authentication it renders whatever the current location is, and a session end navigates the location
back to `/` so the next session cannot land on a route from the finished one (WAS-24).

**Rationale**: `FND-08` made the declared route table the deep-link mechanism; a `session` audience
on it is the truthful classification of "every data surface sits behind the session" (spec
Assumptions), and the gate is the one place that reads it. The desktop gate mounted its own
`Navigator` per authenticated branch; on the web the equivalent is the gate swapping the whole tree,
which already guarantees a fresh render per transition and needs no navigator seam. The deep-link
reset is a user decision (2026-10-16): the attempted route is not resumed, so the gate never has to
carry a "pending location".

**Alternatives considered**:

- *A `RequireAuth` wrapper per route* — rejected: it would repeat the session read and could show a
  half-authenticated shell; one gate at the top renders one decision.
- *Resume the attempted deep link after sign-in* — rejected: the user chose always-land-on-catalog
  (WAS-24), which is also simpler and avoids replaying a stale route.

## R9. The auth screen

**Decision**: add `web/src/auth/auth_screen.tsx` — one surface offering sign-in and registration
through a mode switch, submitting through `AuthSession`. Exactly one primary action (the footer
button) disables and shows progress while a submit is in flight; the password field is
`type="password"`; a failure keeps the typed values and renders the mapped copy inline (resolved
through `failureCopy`, `R7`), never the server's message; switching mode clears a stale error but
keeps the values (WAS-07, WAS-09, WAS-10).

**Rationale**: the desktop `AuthScreen` is the unit being ported (`desktop/lib/auth/auth_screen.dart`)
and its rules — one footer action, values kept on failure, mapped copy inline — are the ones
`web/AGENTS.md` restates ("every async action shows progress and an inline result"; "the primary
action in a fixed footer bar"). Rendering the mapped **kind** copy rather than `DomainFailure.message`
is what satisfies WAS-10's "must not reveal whether an email exists".

**Alternatives considered**:

- *Two screens (login / register)* — rejected: the desktop used one surface with a mode switch and
  WAS-09 asks for "sign-in and registration in one place".
- *A confirmation step after registration* — rejected: registration signs the browser in, so success
  is an immediate session (WAS-02).

## R10. The "Your sheet" destination and the confirmed delete

**Decision**: add `web/src/sheet/your_sheet.ts` (a resolve state machine over `GET /sheets`) and
`web/src/sheet/your_sheet_screen.tsx`:
`loading` → `empty` (server `404`/`NOT_FOUND`, the designed empty line — WAS-26) | `sheet` (with an
Open action that navigates to `/sheets/:id` — WAS-25) | `failure` (any other outcome, with a Retry
that re-requests exactly once — WAS-27). A delete control opens the screen's confirmation dialog
stating the exact consequence; cancel issues **zero** requests, confirm issues exactly one
`DELETE /sheets/:id`; on success the local cache and the sheet's draft are removed and the empty state
shows, on failure the sheet is kept with the mapped copy (WAS-28, WAS-29). The destination is reached
through a session-only left-nav entry (`nav.yourSheet`) and the server is the source of truth — the
local cache is never read to decide it (WAS-25).

**Rationale**: the program plan assigns `GET /sheets` + `DELETE /sheets/:id` to this feature
(`F7-T006`), and the user decision (2026-10-16, recorded in `spec.md` Clarifications) placed the entry
in the persistent navigation with the delete on that surface. `GET /sheets` answers `404` when the
owner has none (`internal/app/sheet.go` `FetchMySheet`), which is exactly the "no sheet" signal that
must render the empty state rather than a failure (WAS-26). Reusing the F5 `evictSheet`/`discardDraft`
helpers for the local removal keeps the delete consistent with the rest of the library. The
confirmation dialog is required by the destructive-action rule (`web/AGENTS.md`) and by WAS-28.

**Alternatives considered**:

- *A "My sheets" list* — rejected: the server holds at most one sheet per owner, so it is a single
  owner-scoped read.
- *Render the local cache when the server is unreachable* — rejected: WAS-25 makes the server the
  source of truth; the offline read path is F8.
- *One-click delete* — rejected: WAS-28 and the destructive-action rule require explicit
  confirmation.

## R11. Provider wiring

**Decision**: extend `web/src/app/providers.tsx` to build, in order, the raw `Transport`, the raw
`ApiClient` inputs — an `AuthApi` over the raw transport plus the CSRF reader, an `AuthSession` over
the `AuthApi` and the `SheetStore`, an authenticated `Transport` over the raw transport and the
session (R4), and the domain `ApiClient` over the authenticated transport — and provide them through
context (`useAuthSession`, `useAuthApi` if needed). The default `document.cookie` reader and the
default `fetch` are injected at the app entry; every suite injects a fake transport, a memory store
and a literal cookie string through the existing `AppProviders` seams.

**Rationale**: F3/F5 already put the client and the store in React context (never a module singleton)
so tests can drive them; the session must be built on the same seam, ordered so the decorator can
hold the session and the session can hold the raw auth client — one construction graph, one injection
point. Keeping the default `document.cookie` reader here (not inside `csrf.ts`) leaves `csrf.ts`
pure and testable.

**Alternatives considered**:

- *Build the session as a module singleton* — rejected: it would leak state across tests and across a
  hot reload, and F2/F3/F5 deliberately avoided singletons.
- *Let `AuthGate` construct the session* — rejected: `useApiClient`/`useSheetStore` already resolve
  from context, and a second construction path would let the decorator and the screen hold different
  sessions.

## R12. The copy keys, the tests and the accessibility measurement

**Decision**: extend the single English catalog (`web/src/l10n/strings.ts`) with the feature's keys —
the auth surface (`auth.title`, `auth.modeLogin`, `auth.modeRegister`, `auth.emailLabel`,
`auth.passwordLabel`, `auth.usernameLabel`, `auth.submitLogin`, `auth.submitRegister`,
`auth.restoring`), the navigation (`nav.yourSheet`, `nav.logout`), the "Your sheet" surface
(`yourSheet.title`, `yourSheet.loading`, `yourSheet.empty`, `yourSheet.open`, `yourSheet.retry`,
`yourSheet.delete`, `yourSheet.deleteTitle`, `yourSheet.deleteMessage`, `yourSheet.deleteConfirm`,
`yourSheet.cancel`) and the three error kinds (`api.error.unauthorized`, `api.error.forbidden`,
`api.error.rateLimited`). No literal string is authored in `web/src/auth/` or `web/src/sheet/`
(WAS-31, SC-008). Suites stay flat in `web/src/test/` with an `auth_*` / `session_*` /
`your_sheet_*` prefix, edge cases first, driving the existing `fake_transport.ts`, the in-memory
`SheetStore` and a literal cookie string. The pure rules (the CSRF parse, the single-flight refresh,
the version guard, the boundary sweep) are unit-tested; the surfaces (restoring/unauthenticated/
authenticated, the form modes and failure states, the "Your sheet" situations and delete) are
rendering tests; the extended adapter is exercised under `fake-indexeddb`. The mounted-surface
accessibility outcome (SC-010) is measured by a Playwright case against the real build with
`@axe-core/playwright`, following `AD-014` — no axe dependency is added to the Vitest tree. The F1
token scan (`tokens_used.test.tsx`) and the F3 copy scan (`l10n_strings.test.ts`) are extended to
`web/src/auth/`.

**Rationale**: constitution III, VI and VII require edge-case-first tests for every behavior and a
rendering/e2e measurement for interface requirements, and every user-facing string must resolve
through the catalog so a later localization feature replaces one module (WAS-31). Extending the two
existing scans keeps the token and copy rules at one gate each, and reusing F1's axe dependency keeps
SC-010 free of a new gate.

**Alternatives considered**:

- *MSW* — rejected (as in F2 and F5): the injected transport already proves request counts, headers
  (Bearer except on `/auth/*`, CSRF on refresh/logout) and the replay without an interception
  package.
- *A Vitest axe sweep* — rejected: `AD-014` fixed the browser-level measurement as the Playwright
  case; the Vitest suites assert ARIA structure instead.
