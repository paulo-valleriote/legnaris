# Implementation Plan: Web Auth Session

**Branch**: `007-web-auth-session` | **Date**: 2026-10-16 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/007-web-auth-session/spec.md`

**Program plan**: `docs/plan-web-pwa.md` (Parte B, feature F7) — the umbrella plan this feature
implements; the consumed backend contract is the delivered `006-backend-browser-auth-cookies`.

## Summary

Give the web client its session: a browser session over the delivered cookie transport, holding a
short-lived access token **in memory only**. The feature ports `desktop/lib/auth/` and
`desktop/lib/api/authenticated_client.dart` to the web stack, minding the deviations the backend
contract forces (registration signs the browser in — one request, not register-then-login; there is
no stored token, because the refresh credential is an `HttpOnly` cookie). It adds the four `/auth/*`
operations behind the F2 `Transport` (register/login/refresh/logout, `credentials: 'include'`, CSRF
echoed from the script-readable `__Host-legnaris_csrf` cookie), an **authenticated transport
decorator** that attaches `Authorization: Bearer` to `/sheets` and `/templates` and, on a `401`,
performs exactly one **single-flight** silent refresh and replays the refused call once, a session
boundary (`AD-010`) that sweeps the F5 IndexedDB cache and drafts at every definitive session end,
an auth gate that renders the sign-in form whenever no session is held, and a session-only
left-navigation "Your sheet" destination that resolves `GET /sheets` (empty state, open, confirmed
`DELETE /sheets/:id`). It widens the F2 transport's `credentials` posture for exactly the four auth
calls, adds the `DELETE` method and two F6 codes to the error mapper, and adds **no new dependency**:
the work is the existing `fetch` transport, the existing TanStack Query cache, the existing
translation/token modules, and one new operation pair on the sheet-store port (`clearAll`).

## Technical Context

**Language/Version**: TypeScript 5.9.3 with the F1 strictness settings (`strict`, `noUnusedLocals`,
`noUnusedParameters`, `noFallthroughCasesInSwitch`, `isolatedModules`, `resolveJsonModule`,
`@typescript-eslint/consistent-type-imports`) on React 19.3.1 / Vite 8.3.4, Node.js 24 LTS, with
react-router-dom 7.18.4 and TanStack Query 5.104.3 (the F1/F3 pins). Reading `web/package.json` and
the installed tree confirms those versions; no toolchain change is introduced.

**Primary Dependencies**: **no new runtime and no new dev dependency** (spec Assumptions). The
feature reuses the F2 `Transport` (widened so a request may declare `credentials: 'include'`, and
extended with the `DELETE` method), the F2 envelope decoder and error mapper (extended with `401`,
`403` and `429`), the F3 query client, the F5 `SheetStore` port (extended with one clear-all
operation), and the F1 tokens, copy catalog, error mapper and router. The CSRF token is read from
the cookie the server already sets, so no cookie parser is added either. `fake-indexeddb` 6.2.6,
already a dev dependency from F5, covers the extended adapter.

**Storage**: the **in-memory only** access token (never persisted) plus a session-version counter
and a `restoring | unauthenticated | authenticating | authenticated` state. The one local-data
operation F7 performs is the **session-boundary sweep** over the F5 IndexedDB store: a new
`SheetStore.clearAll()` (delete every `sheets` and `drafts` record) is added to the port, the
in-memory double and the `idb` adapter, and `library/session_data.ts` runs it best-effort — a
failing store never blocks the session end (WAS-21). No credential is read from or written to any
local store (WAS-32, SC-001). The CSRF token is read, per request, from the script-readable
`__Host-legnaris_csrf` cookie via an injected cookie string.

**Testing**: Vitest 5.0.5 with Testing Library 16.3.3 and `user-event` 14.6.8 (the F1 pins), flat
suites under `web/src/test/` with an `auth_*` / `session_*` / `your_sheet_*` prefix, edge cases
first (constitution III). Request counts, the Bearer posture, the single refresh, the one replay and
the CSRF header are asserted against the existing `fake_transport.ts`, so no network-interception
package is added. The adapter's `clearAll` is exercised under `fake-indexeddb`. The interface
outcome (SC-010) is measured by a Playwright case against the real build with `@axe-core/playwright`,
following `AD-014` — no axe dependency is added to the Vitest tree. The F1 token scan
(`tokens_used.test.tsx`) and the F3 copy scan (`l10n_strings.test.ts`) are extended to
`web/src/auth/` and `web/src/sheet/`.

**Target Platform**: browsers — the latest two versions of Chrome, Edge, Firefox and Safari, served
over HTTPS. The screens render React DOM components; they use the browser's fetch, history routing,
IndexedDB and the cookie jar, and no browser global beyond that.

**Project Type**: web application (client) inside an existing Go monorepo; the backend is not read
and not modified — the feature consumes the delivered F6 contract as built.

**Performance Goals**: not throughput; correctness and politeness. A cold start issues **exactly
one** silent refresh (WAS-03); a burst of `N` concurrent `401`s shares **one** refresh and rotates
the session **once** (WAS-13); a refused call is replayed **exactly once** (WAS-12); sign-out issues
**at most one** `/auth/logout` (WAS-23); a delete issues **exactly one** `DELETE` per confirm and
**zero** on cancel (WAS-28).

**Constraints**: no credential in `localStorage`/`sessionStorage`/IndexedDB (WAS-01, WAS-32,
constitution VI); the `HttpOnly` refresh cookie is never read by script (WAS-18); no DOM-injection
sink, server text and data rendered as text (WAS-31); no user-facing literal outside `src/l10n`
(WAS-31, SC-008); every visual value from the shared tokens, no ad-hoc hex/px; identifiers, comments
and copy in English only; every `WAS-*` citation resolves through the spec lint already covering
`*.ts`/`*.tsx`. No new CI gate is introduced (no new dependency).

**Scale/Scope**: 10 new client modules (`request.ts`, `auth_api.ts`, `authenticated_transport.ts`,
`csrf.ts` under `api/`; `auth_session.ts`, `auth_gate.tsx`, `auth_screen.tsx`, `auth.css` under `auth/`;
`session_data.ts` under `library/`; `your_sheet.ts` + `your_sheet_screen.tsx` under `sheet/`) plus ~10
extended modules (`transport.ts`, `client.ts`, `errors.ts`, `error_mapper.ts`, `sheet_store.ts`,
`idb_sheet_store.ts`, `providers.tsx`, `routes.ts`, `router.tsx`, `left_nav.tsx`, `strings.ts`), a
group of l10n keys, ~10 test suites, the ~14 F1/F3 `renderApp` suites and the four `web/e2e/*.spec.ts`
specs retargeted behind the session, the extended scan pair, and one Playwright case, 4 user
stories (3 P1, 1 P2) and 33 requirements.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The feature raises no `apperr` and adds no backend boundary; the backend F6 codes (`UNAUTHORIZED`, `FORBIDDEN`, `RATE_LIMITED`) are classified into interface-facing kinds in the one `error_mapper`, and no screen parses a code or matches an error string. | PASS (consumer side) |
| II. Canonical Identifiers | No identifier is generated here; the feature reads the opaque sheet id from `GET /sheets` and the route, and never parses, formats or validates it. | PASS (not exercised) |
| III. Behavior Ships With Tests | Every requirement lands with a test in the same commit: the session states, the single-flight refresh, the once-only replay, the CSRF echo, the session-version guard, the boundary sweep, the logout single-flight, the "Your sheet" resolve/empty/failure/delete situations and the mapped kinds are proven by unit and rendering tests, edge cases first (no cookie, rejected cookie, transient refresh, concurrent 401s, late response from a finished session, absent CSRF, `429`, cancel, failing store). | PASS |
| IV. English-Only Source | All identifiers, comments, test names and user-facing strings are English; every new string is a key in the English catalog. | PASS |
| V. Layered Architecture | Only `web/` gains code, inside the client: `auth/` sits above `api/`, `library/` and `l10n/`; `api/` stays a leaf over `l10n/` and `shared`-independent helpers; the Go layering is untouched. Nothing imports `auth/` except the router, the shell and `providers.tsx`. | PASS |
| VI. Security Gates Are Blocking | No gate changes and no gate weakened. The access token lives in memory only (WAS-01, SC-001); the refresh credential stays `HttpOnly` and is never read (WAS-18); no credential reaches a local store, a log record or a URL (WAS-32); the sign-in surface and the "Your sheet" surface introduce no DOM-injection sink and render data as text (WAS-31) — the F1 ESLint anti-sink rules cover the new `.tsx` sources. No dependency is added, so the SCA/SAST surface is unchanged. | PASS (maintained) |
| VII. Installable, Offline-Capable Web Client | The auth screen and the "Your sheet" route are deep-linkable through the existing history router; the session boundary writes and clears the same IndexedDB store the PWA feature later reads, and the interface outcome is verified by rendering tests plus the browser-level accessibility sweep this feature owns (SC-010). The offline read path itself remains F8. | PASS |
| Additional Constraints — Stack | The feature uses the stack the constitution names (React/Vite/TypeScript under `web/`), adds no framework, no state library and no package, and keeps the session in one small external store consumed through `useSyncExternalStore` (the F5 pattern, `AD-016`). | PASS |
| Additional Constraints — Traceability | `WAS-*` is defined in `spec.md` and the F1 lint already scans `*.ts`/`*.tsx`, so citations in the delivered sources are gated without further work. | PASS |
| Additional Constraints — Jira | This is pre-work planning; the implementing PR must reference its `LGN-\d+` card. Recorded for the PR, not a gate failure here. | PASS (noted) |
| Development Workflow | The documented PR sequence (`make lint-web && make test-web && make sec-scan`) covers the new modules; the Playwright a11y case runs in the existing e2e suite. No Makefile change is required. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps the network layer the F2
client's — one shared request helper (`R1`) feeding both the domain client and the auth client over
the one injected transport, so no operation builds its own request and the credential is attached at
exactly one seam. The two decisions that could have drifted were resolved in research: the access
token stays in memory with **no** token store (the F7 task's `session_store.ts` becomes
`auth_session.ts`, because there is no stored token to read — `R3`), and the session boundary reuses
the F5 store through a new `clearAll()` rather than a second database or a per-record walk (`R6`).
The session-version guard (`R4`) is the web expression of the desktop `AuthenticatedClient`
(`SES-01`/`SES-02`, `AD-010`) and is asserted at the transport seam. No new dependency is added, so
the "no new package without strict necessity" rule is honored and the SCA/SAST gates are untouched.

## Project Structure

### Documentation (this feature)

```text
.specs/features/007-web-auth-session/
├── spec.md              # Requirements (WAS-*)
├── plan.md              # This file
├── research.md          # Phase 0 — decisions and rejected alternatives
├── data-model.md        # Phase 1 — session, transport, CSRF, your-sheet, sweep, store port
├── quickstart.md        # Phase 1 — runnable validation guide
├── contracts/           # Phase 1 — one contract per feature interface
│   ├── auth-api.contract.md
│   ├── authenticated-transport.contract.md
│   ├── auth-gate.contract.md
│   ├── session-boundary.contract.md
│   └── your-sheet.contract.md
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
web/src/
├── api/
│   ├── transport.ts                # extended: credentials 'omit' | 'include', DELETE method (WAS-06, WAS-11)
│   ├── request.ts                  # NEW: the shared caller (timeout, GET retry, envelope decode)
│   ├── client.ts                   # extended: getMySheet(), deleteSheet(id) (WAS-25, WAS-28)
│   ├── auth_api.ts                 # NEW: register/login/refresh/logout, credentials include + CSRF (WAS-02, WAS-06, WAS-18)
│   ├── authenticated_transport.ts  # NEW: Bearer + single-flight refresh + one replay (WAS-11 … WAS-17)
│   ├── csrf.ts                     # NEW: reads __Host-legnaris_csrf from an injected cookie string (WAS-18)
│   ├── errors.ts                   # extended: unauthorized/rateLimited/forbidden kinds (WAS-30)
│   └── error_mapper.ts             # extended: UNAUTHORIZED/FORBIDDEN/RATE_LIMITED → kinds (WAS-30)
├── auth/                           # the session (NEW)
│   ├── auth_session.ts             # in-memory token, version, states, restore/login/register/logout (WAS-01 … WAS-24)
│   ├── auth_gate.tsx               # gate + restoring state (WAS-03, WAS-08, WAS-24)
│   ├── auth_screen.tsx             # sign-in / registration form (WAS-07, WAS-09, WAS-10)
│   └── auth.css                    # token-only styles for the auth surface
├── library/                        # local persistence
│   ├── sheet_store.ts              # extended: clearAll() on the port + in-memory double (WAS-21)
│   ├── idb_sheet_store.ts          # extended: clearAll() clears both object stores (WAS-21)
│   └── session_data.ts             # NEW: best-effort session-local sweep (WAS-19 … WAS-22)
├── sheet/                          # the open/edit surface
│   ├── your_sheet.ts               # NEW: resolve GET /sheets + confirmed delete (WAS-25 … WAS-29)
│   └── your_sheet_screen.tsx       # NEW: empty / loading / failure / delete confirmation (WAS-26 … WAS-29)
├── shell/
│   └── left_nav.tsx                # extended: "Your sheet" entry + sign-out (session-only) (WAS-19, WAS-25)
├── app/
│   ├── providers.tsx               # extended: AuthApi + authenticated ApiClient + session provider
│   ├── routes.ts                   # extended: /your-sheet declared; data routes are session-only (WAS-08, WAS-25)
│   └── router.tsx                  # extended: gate wraps the route table; /your-sheet screen mapped
├── l10n/
│   └── strings.ts                  # auth/session/your-sheet copy keys + the three error kinds (WAS-31)
└── test/
    ├── auth_session.test.ts        # states, restore, single-flight logout, version, sweep (NEW)
    ├── auth_screen.test.tsx        # form modes, in-flight, failure copy, kept values (NEW)
    ├── authenticated_transport.test.ts  # Bearer posture, 401→refresh→replay, single-flight, guard (NEW)
    ├── auth_api.test.ts            # credentials include, CSRF echo, decode, logout 204 (NEW)
    ├── auth_gate.test.tsx          # restoring, unauthenticated, authenticated, deep-link reset (NEW)
    ├── session_boundary.test.ts    # sweep clears cache+drafts; transient keeps them (NEW)
    ├── your_sheet.test.tsx         # resolve/empty/failure/retry/delete cancel+confirm (NEW)
    ├── api_error_mapper.test.ts    # extended: 401/403/429 kinds, unmapped stays generic (extended)
    ├── library_store.test.ts       # extended: clearAll (fake-indexeddb) (extended)
    ├── api_no_logging.test.ts      # extended: scans src/auth/ too (WAS-32) (extended)
    ├── render.tsx                  # extended: mounts an injected session, authenticated by default
    ├── tokens_used.test.tsx        # extended: scans src/auth/ (extended)
    └── l10n_strings.test.ts        # extended: auth-*/sheet.own* keys + source scan (extended)
web/e2e/
    └── auth.spec.ts                # sign-in + your-sheet axe sweep (SC-010) (NEW)
```

**Structure Decision**: the session lives under `web/src/auth/` and the wire layer under
`web/src/api/`, the paths the program plan already reserves (Parte 6 listing
`auth/{auth_session.ts,auth_gate.tsx,auth_screen.tsx,token_store.ts}` and
`api/{...,auth_api.ts,authenticated_client.ts}`), so the ported units map onto their desktop
counterparts (`auth_session.dart`, `auth_gate.dart`, `auth_screen.dart`, `authenticated_client.dart`)
and `auth_session.ts` replaces the task's `session_store.ts` name because there is no stored token to
read, while `authenticated_transport.ts` replaces `authenticated_client.ts` because it is a transport
decorator rather than a client (`R3`, `AD-017`). The split is deliberate: `api/` owns the wire — one shared request helper, the auth
operations, the authenticated-transport decorator and the CSRF reader — `auth/` owns the in-memory
session, the gate and the form, `library/` owns the session-local sweep, and `sheet/` owns the
"Your sheet" resolve/delete surface. `transport.ts` keeps its name and is widened in place, because
the request's credential posture is a property of the request, not of the transport (F2 `API-29`
keeps `omit` for every non-auth call). The gate wraps the route table inside `AppShell`, so the
catalog, the sheet surface and the "Your sheet" destination render only with a session (WAS-08); the
left-nav entry and sign-out are session-only children of the persistent nav. Styles stay in `--lg-*`
tokens: the auth surface gets one new `web/src/auth/auth.css`, the "Your sheet" surface reuses the
existing `sheet.css`, and the sign-out control reuses `shell.css`, so the F1 token-usage scan can
inspect the new directory the same way it inspects `shell/`, `catalog/` and `engine/`. Tests stay
flat in `web/src/test/` with the feature prefixes; the shared `render.tsx` helper mounts an injected
session (authenticated by default) so the earlier `renderApp` suites and the four `web/e2e/*.spec.ts`
specs render truthfully behind the gate rather than being deleted (`T019`, `T020`, `WAS-08`), and the
cross-cutting scans — including the no-logging scan, now covering `web/src/auth/` — plus the
Playwright axe case are extended rather than duplicated so the token, copy and accessibility rules
keep one gate each.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
