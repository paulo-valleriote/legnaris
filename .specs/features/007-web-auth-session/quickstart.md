# Quickstart: validate the web auth session

**Feature**: `.specs/features/007-web-auth-session/` | **Date**: 2026-10-16

Runnable scenarios that prove the feature works end to end. Each lists what to run and the outcome
that means it passed. Prerequisites: the F1 client toolchain (`web/` with `npm install` already done).
The Vitest suites need no backend and no MongoDB: every request is driven through the injected fake
transport, the cookie string is a literal, and every local record goes through the in-memory
`SheetStore` double. Only the accessibility scenario (9) needs a built client.

Paths referenced here are defined in [plan.md](./plan.md) (source layout),
[data-model.md](./data-model.md) (shapes) and the five documents under [contracts/](./contracts/).

---

## 1. The suite runs with no backend at all

```bash
cd web && npm test
```

**Outcome**: the whole suite passes with the API and MongoDB stopped. The auth suites drive the F2/`AuthApi`
clients through `fake_transport.ts`, the cookie through a literal, and the store through the in-memory
double, so nothing touches the network or a database — the state continuous integration runs in.

---

## 2. Establishing a session keeps the credential in memory only

```bash
cd web && npx vitest run src/test/auth_screen.test.tsx src/test/auth_api.test.ts
```

**Outcome**: login sends exactly one `credentials: 'include'` `POST /auth/login` and holds the returned
access token in memory (WAS-01); registration establishes the session from its own response and issues
no follow-up login (WAS-02); the password field is a hidden-content control and the single primary
action is disabled with progress shown while a submit is in flight (WAS-09); a failure keeps the typed
values and renders the mapped copy inline with no session established (WAS-07); the `409` duplicate and
any other registration failure render the same catalog copy, never the server message, so no email
existence is revealed (WAS-10); an inspection of `localStorage`, `sessionStorage` and the store records
after login finds no credential (WAS-01, WAS-32, SC-001).

---

## 3. A cold start restores or falls back

```bash
cd web && npx vitest run src/test/auth_gate.test.tsx
```

**Outcome**: mounting with a valid cookie issues exactly one silent refresh and shows the restoring
state until it settles, never the sign-in form (WAS-03); a `401` settles at the sign-in form with no
session (WAS-04); an offline/`5xx`/unreadable refresh settles at the sign-in form **keeping** the
session-local data (WAS-05, SC-006); while no session is held only the auth surface renders and the
catalog, sheet and "Your sheet" surfaces do not (WAS-08); a deep link to `/sheets/abc` visited signed
out renders the sign-in form and is not resumed — after sign-in the app starts on the catalog
(WAS-24).

---

## 4. Silent refresh recovers without a second rotation

```bash
cd web && npx vitest run src/test/authenticated_transport.test.ts
```

**Outcome**: a `/sheets` or `/templates` call carries `Authorization: Bearer` and sends no cookie
(WAS-11); a protected `401` triggers one refresh and one replay that succeeds (WAS-12); several
concurrent `401`s share a single refresh, so the family rotates once (WAS-13, SC-003); a rejected
refresh forces logout, drops the token, clears the session-local data and surfaces the original
refusal (WAS-14); a transient refresh rethrows and keeps the session and its data (WAS-15); `/health`
and every `/auth/*` path carry no Bearer and start no refresh (WAS-16, SC-004); a `401` or refresh
belonging to a session that has since ended neither ends nor overwrites the current one (WAS-17).

---

## 5. The CSRF token is echoed from the cookie

```bash
cd web && npx vitest run src/test/auth_api.test.ts
```

**Outcome**: `refresh` and `logout` echo the `__Host-legnaris_csrf` cookie value in `X-CSRF-Token`,
resolved at request time (WAS-18); an absent or malformed cookie sends no header and the server's `403`
is mapped to `forbidden` rather than a silent local failure (WAS-18, WAS-30); the `HttpOnly` refresh
cookie is never read; a `204` logout body is accepted and returns `void`.

---

## 6. Signing out leaves nothing behind

```bash
cd web && npx vitest run src/test/session_boundary.test.ts
```

**Outcome**: sign-out issues `/auth/logout` best-effort, drops the token, shows the sign-in form and
clears the session-local data (WAS-19); a forced logout clears it exactly the same way (WAS-20); the
sweep removes every cached sheet and every draft and a failing store never blocks the session end
(WAS-21); after the boundary the next session renders none of the finished session's records
(WAS-22, SC-005); activating sign-out twice in one frame issues at most one `/auth/logout` (WAS-23).

---

## 7. The signed-in person's own sheet is reachable and deletable

```bash
cd web && npx vitest run src/test/your_sheet.test.tsx
```

**Outcome**: the session-only "Your sheet" entry resolves the person's sheet with one `GET /sheets`
from the server (WAS-25); owning none shows the designed empty state, distinct from loading and failure
(WAS-26); any other outcome shows the mapped copy with a Retry that re-requests exactly once (WAS-27);
Open navigates to `/sheets/:id`; activating Delete opens a confirmation stating the exact consequence,
cancel issues zero requests and confirm issues exactly one `DELETE /sheets/:id` (WAS-28, SC-007); on
success the sheet and its draft are removed locally and the empty state shows, on failure the sheet is
kept with the mapped copy (WAS-29).

---

## 8. The session codes are classified

```bash
cd web && npx vitest run src/test/error_mapper.test.ts
```

**Outcome**: `UNAUTHORIZED` maps to `unauthorized`, `FORBIDDEN` to `forbidden`, `RATE_LIMITED` to
`rateLimited` with its own copy (WAS-30, SC-008); every unclassified code — `CONFLICT` included —
stays on the generic kind rather than being misclassified.

---

## 9. No copy and no visual value is authored in the feature

```bash
cd web && npx vitest run src/test/l10n_strings.test.ts src/test/tokens_used.test.tsx && npm run lint
```

**Outcome**: every string the auth surface, the gate, the sweep and the "Your sheet" surface render
resolves through `src/l10n/strings.ts` — removing a referenced key turns the suite red (WAS-31,
SC-008) — and the token scan finds no literal hex or px in `web/src/auth/` or the new `web/src/sheet/`
modules. The ESLint anti-sink rules cover the new `.tsx` sources, and a value carrying `<script>`
renders as text (WAS-31).

---

## 10. The mounted surfaces have no critical accessibility violation

```bash
cd web && npm run build && npx playwright test e2e/auth.spec.ts
```

**Outcome**: the auth surface, the signed-in shell and the "Your sheet" surface report zero critical
automated accessibility violations from `@axe-core/playwright` (SC-010), and every control exposes an
accessible role and name. No new axe dependency is added: F1 installed it and F3 established the
measurement pattern (`AD-014`).

---

## 11. The gates stay green

```bash
cd web && npm run format:check && npm run lint && npm run typecheck && npm test && npm run build
cd .. && make lint-specs
```

**Outcome**: all client gates pass and the spec lint reports clean, so every `WAS-*` citation in the
delivered sources resolves to `spec.md` and no suite assertion is vacuous. No dependency is added, so
the secret, SCA, SAST and lockfile gates see nothing new to vet.

---

## Traceability

| Scenario | Requirements |
|---|---|
| 1 | all `WAS-*` (no backend) |
| 2 | WAS-01, WAS-02, WAS-07, WAS-09, WAS-10, WAS-32 |
| 3 | WAS-03…WAS-05, WAS-08, WAS-24 |
| 4 | WAS-11…WAS-17 |
| 5 | WAS-18, WAS-30 |
| 6 | WAS-19…WAS-23, WAS-32 |
| 7 | WAS-25…WAS-29 |
| 8 | WAS-30 |
| 9 | WAS-31 |
| 10 | SC-010 |
| 11 | all `WAS-*` (lint) |
