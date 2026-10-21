# Quickstart — Validating the Code Step in the Client

**Feature**: `.specs/features/019-web-auth-code-step/` | **Requirements**: `WCS-01` … `WCS-18`

A runnable guide to proving the feature by hand. The automated suites are the authority; this guide is
for a human verifying the behaviour end to end, including the parts a suite can only simulate.

## Prerequisites

- Node `>=24 <25` and the client dependencies installed: `cd web && npm ci`.
- MongoDB for the manual scenarios: `make dev-db` (it reads the credentials from the repo-root `.env`,
  the same file `make run` includes).
- The API and the client are two processes, and the gate is a **server** setting: it is set on the API
  process, never on `npm run dev`, which starts only the client.

```bash
# terminal 1 — the API, gate off (the default)
make run

# terminal 2 — the client
cd web && npm run dev
```

Switching the gate restarts the API with the variable set; the client needs no rebuild, because one
build handles both answers (`WCS-01`).

```bash
# the API, gate on
AUTH_REQUIRE_EMAIL_VERIFICATION=true make run
```

**Where the code comes from locally.** The delivery worker belongs to a later stage of the program
plan, and neither the API nor the development email sender ever logs a code (`EML-10`, `EML-13`). With
the gate on, the code lives in the durable send intention the API wrote:

```bash
URI="$(sed -nE 's/^DATABASE_URI=//p' .env | tr -d '"')"
mongosh --quiet "$URI" \
  --eval 'db.email_outbox.find().sort({created_at:-1}).limit(1).forEach(d => printjson(d.payload))'
```

Read the six digits from there. Nothing in the client, the API log or the sender output will show them.

## Gates

```bash
cd web
npm run lint          # eslint, including the token/copy scans and the DOM-sink rules
npm run typecheck     # tsc --noEmit
npm test              # Vitest, all suites
npm run test:e2e      # Playwright + axe over the auth surface and the code surface
```

The repository's aggregate targets run the same work: `make lint-web && make test-web && make sec-scan`
for the client, and `make lint-specs` for the amended spec and the new citations.

## Scenario 1 — Gate off: nothing changes (`WCS-01`, `SC-002`)

Start the API with the gate off (`make run`, the default — see Prerequisites).

1. Register a fresh address in the running client.
2. Expect: **no code surface**; the app goes straight to the catalog and the network panel shows the
   single `POST /auth/register` returning `201` with `access_token`.
3. Sign out, sign in again with the same credentials.
4. Expect: a single `POST /auth/login` returning `200` with `access_token`, and the catalog.

This is the deployment that must keep working byte-for-byte (`WAS-02`'s gate-off behaviour).

## Scenario 2 — Gate on: register, then enter the code (`WCS-01`, `WCS-02`, `WCS-03`)

Restart the API with the gate on: `AUTH_REQUIRE_EMAIL_VERIFICATION=true make run` (see Prerequisites).
The client keeps running unchanged.

1. Register a fresh address.
2. Expect: the **code surface** replaces the credential form in place — the URL does not change — and
   the heading says the step is a registration confirmation. The `POST /auth/register` answer is `201`
   with `challenge_id`, `expires_in`, `resend_after` and **no `access_token`**, and **no `Set-Cookie`**.
3. Confirm no token is held: reload the page. Expect the credential form, with no session and no
   resumed code surface (the handle is memory-only, `R2`).
4. Register again (or sign in) to get a step, read the code from the outbox row above, and submit it.
5. Expect: `POST /auth/challenge/verify` returns `200` with `access_token` plus the two `Set-Cookie`
   headers; the app enters the catalog; the request carried `credentials: include`, no `Authorization`
   header and no `X-CSRF-Token`.

## Scenario 3 — A wrong code names the real cause (`WCS-04`, `WCS-11`)

1. With a step pending, submit a wrong six-digit code (for example a correct code with one digit
   changed).
2. Expect: the code surface stays, the entered code is kept so it can be corrected, and the copy says
   the code is invalid or expired — **not** "Your session has ended" and **not** the generic
   "Something went wrong". The answer is `401 OTP_INVALID`.
3. Repeat until the attempt cap is reached (five). Expect the same uniform copy each time, and a
   `429 RATE_LIMITED` if the per-address limit trips first.
4. Use "start over" and confirm the credential form returns with no request issued.

## Scenario 4 — Resend and its countdown (`WCS-07`, `WCS-08`, `WCS-09`, `WCS-10`)

1. With a step pending, look at the resend action: it is **disabled** and shows the remaining seconds,
   seeded from the answer's `resend_after`.
2. Wait out the window (60 s by default) and activate resend once.
3. Expect: exactly **one** `POST /auth/challenge/resend` returning `202` with `resend_after`; the
   countdown restarts from the answer. A second activation inside the window issues no request.
4. Submit the code you already read from the outbox row. Expect it still verifies — the resend rotated
   neither the handle nor the code.
5. To exercise the unavailable outcome, spend the daily send allowance
   (`AUTH_OTP_DAILY_SEND_CAP`, deliberately below the provider's cap) and activate resend: expect the
   `503 EMAIL_UNAVAILABLE` copy telling the person to try again shortly, the handle kept, and the
   action re-arming at the end of the current window. The client does not read the server's
   `Retry-After` — it cannot, cross-origin (`R3`).

## Scenario 5 — A duplicate address names the conflict (`WCS-11`)

1. With the gate on, register an address that already has an account.
2. Expect: `409 CONFLICT` and a copy that names the address as already registered and offers sign-in —
   not the generic copy. The credential form keeps the values that were typed.
3. Follow the offered path: switch to sign-in and complete a code step for that address.

## Scenario 6 — Abandoning a step issues nothing (`WCS-06`, `SC-003`)

1. Start a step, then use "start over" (or navigate away).
2. Expect: the credential form returns, the handle and the entered code are cleared, and the network
   panel shows **no** new request.
3. Start a step and submit a code, then immediately start over before the answer lands: expect the
   answer to be discarded and the credential form to stay — no session is established for an abandoned
   step (`R10`).

## Scenario 7 — The surfaces stay accessible (`WCS-17`, `SC-006`)

1. Run `npm run test:e2e`. The auth spec intercepts `POST /auth/register` with a code-step answer,
   asserts the code surface renders with its field labelled and the countdown announced, completes the
   step against an intercepted verification answer, and runs the axe sweep over the code surface.
2. Expect: zero critical violations on the credential surface, the code surface and the signed-in
   shell, and the committed screenshot baseline to match.

## Scenario 8 — The pinned contract cannot drift (`WCS-14`)

```bash
AUTH_REQUIRE_EMAIL_VERIFICATION=false bash web/src/api/fixtures/capture.sh   # refreshes the session family
AUTH_REQUIRE_EMAIL_VERIFICATION=true  bash web/src/api/fixtures/capture.sh   # refreshes the code-step family
```

1. Each run captures the family the stack actually answered and leaves the other family's files
   untouched (`R8`); no body is ever hand-written.
2. Run `npm test`. Expect the pinned suite to decode every fixture with no backend running; change a
   key in any fixture by hand and expect the suite to fail on it.

## What "done" looks like

- Every requirement in `spec.md` has at least one task in `tasks.md` and at least one assertion in a
  suite; the moved suites assert the legacy gate-off path beside the code-step path.
- `npm run lint && npm run typecheck && npm test && npm run test:e2e` are green.
- No credential, no handle and no code appears in `localStorage`, `sessionStorage`, IndexedDB, a URL or
  a log record (`SC-007`).
- No new dependency, no new route and no Go source change appears in the diff.
