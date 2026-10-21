# Research — The Code Step in the Client

**Feature**: `.specs/features/019-web-auth-code-step/` | **Requirements**: `WCS-01` … `WCS-18`

Phase 0 resolves the design questions the specification leaves open. Each entry states the decision,
the reason, the alternative rejected and the scope it binds.

---

## R1 — The two answers are told apart by payload shape, never by status

**Decision**: `register` and `login` decode a *sign-in outcome* — a discriminated union of a session
body and a code step — and the discrimination reads the payload's own keys: `access_token` present →
session; `challenge_id` present → code step. A payload carrying neither, or both, is a defensive parse
failure. The HTTP status is not consulted.

**Reason**: registration answers `201` in **both** gate states. Gate off, that `201` carries
`{access_token, token_type, expires_in}`; gate on, the same `201` carries
`{challenge_id, expires_in, resend_after}` (`018-auth-2fa-session-gate/contracts/endpoints.contract.md`).
A client branching on the status would either lose the session when the gate is off or lose the handle
when it is on — and it would do so for the one operation that cannot tell the difference. The payload's
shape is the only signal the server actually sends. Discrimination in the decode seam also keeps the
contract in one place: no screen parses a payload, matching how `decodeEnvelope` already owns the
success/failure split.

Sign-in *does* differ by status (`200` versus `202`), but keying one operation on its status and the
other on its shape would give the same two answers two different detection rules; the shape rule
covers both.

**Rejected alternative**: branching on the status in the API module — it fails for registration as
described, and a per-operation rule would make the two credential operations disagree about how the
contract is read. Also rejected: decoding a union by `try`-ing the session decode first and falling
back on the parse failure — a genuine contract drift would then be swallowed as "it must be a code
step", which is exactly the silent misclassification the envelope decoder exists to prevent.

**Scope**: `web/src/api/auth_api.ts`, `web/src/api/fixtures/capture.sh` (the capture protocol records
both families), `web/src/test/auth_api.test.ts`.

---

## R2 — The pending handle is memory-only and never persisted

**Decision**: the challenge handle, the origin operation and the entered code live in the session's
in-memory state. A reload loses the pending step and the credential form returns. No `localStorage`,
`sessionStorage`, IndexedDB or cookie write is added, and no new sweep rule is introduced at the
session boundary.

**Reason**: the handle identifies a live challenge; persisting it would create the client's first
JavaScript-readable record of an authentication artifact. The delivered posture is that nothing a
script can read survives a reload — the access credential is a module-private field
(`007-web-auth-session/spec.md`, `WAS-32`) and the refresh credential is an `HttpOnly` cookie. A
persisted handle would need its own lifecycle, its own sweep at the session boundary and its own
staleness rules, for a recovery path the person already has: start the step again. The plan's §3.5 asks
only for the state, not for its survival.

**Rejected alternative**: persisting the handle so the code surface survives a reload (clarification,
2026-10-20) — rejected because it introduces a second kind of local record whose only benefit is
skipping one request, and because the handle would have to be swept wherever the session-local data is
swept, widening the boundary the session owns.

**Scope**: `web/src/auth/auth_session.ts`, `web/src/test/auth_session.test.ts`,
`web/src/test/session_boundary.test.ts`.

---

## R3 — The resend window is the local window; the server's retry hint is not readable

**Decision**: the code surface's resend action is governed by a locally ticking window seeded from the
challenge answer's advertised `resend_after`. When a resend is refused as unavailable, the action stays
disabled for the remainder of the current window and the copy tells the person to try again shortly;
no instant is computed from the server's retry hint. The transport seam stays `{ status, body }` and no
header plumbing is added.

**Reason**: the client cannot read the hint. The API is served cross-origin from the app
(`api.legnaris.com.br` versus `app.legnaris.com.br`), and `Retry-After` is not one of the
CORS-safelisted response headers, so a browser `fetch` cannot expose it unless the server sends
`Access-Control-Expose-Headers: Retry-After`. Exposing it is a backend and edge-proxy change outside
this feature's scope, and the alternative — parsing a header the client is not allowed to see — would
ship a branch that can never run in production and only appears to work in a same-origin test.

Widening `HttpResponse` with headers would also change the shared envelope input and the shape of
`DomainFailure` for a hint whose value the challenge policy does not document as a client-visible
duration, while the surface already owns a window that governs the same control. The copy still tells
the person what to do; it simply does not promise a computed countdown it cannot honestly derive.

**Rejected alternative**: widening the transport seam with response headers now — rejected as a change
to the shared seam whose only consumer would be an unreadable header; if a later stage exposes the
header server-side, the seam can be widened then with a real value to read. Also rejected: disabling
the action for the full advertised wait — the daily cap's hint can run to the next UTC day, which would
leave a dead control where the local window plus honest copy is enough, and the surface always offers
the way back to the credential form.

**Scope**: `web/src/auth/auth_code_screen.tsx`, `web/src/l10n/strings.ts`,
`web/src/auth/auth_session.ts` (the window the answer carries), spec `WCS-09` (amended to match).

---

## R4 — The pending state belongs to the session

**Decision**: the pending step is a state of `AuthSession` — a new `pendingCode` member of the existing
state union plus a `pending` field on the snapshot carrying the origin and the advertised window. The
handle itself stays module-private inside the session. The surface submits and resends through session
methods; it never holds the handle.

**Reason**: the session already owns the state machine the gate renders, the in-memory credential and
the `useSyncExternalStore` snapshot every surface reads. A separate pending store would create a second
answer to "is a step pending", and the gate would have to consult both — the drift risk the delivered
`AD-010`/`WAS-17` guards exist to avoid. Keeping the handle module-private mirrors the access
credential: the surfaces act on the step without being able to leak it, so there is no second copy to
sweep and no surface can accidentally persist it.

**Rejected alternative**: a `pendingCode` provider or a module-level store beside the session —
rejected as a second source of truth for one boolean, with no consumer that the session cannot already
serve. Also rejected: putting the handle on the snapshot so the surface could call the API directly —
it would hand every subscriber a live challenge handle, which is precisely the object the design keeps
private.

**Scope**: `web/src/auth/auth_session.ts`, `web/src/auth/auth_gate.tsx`,
`web/src/auth/auth_code_screen.tsx`, `web/src/test/auth_session.test.ts`.

---

## R5 — The code surface is a state of the single auth surface, not a route

**Decision**: `AuthGate` renders the code surface in place of the credential form while a step is
pending. No entry is added to `src/app/routes.ts` and the URL is not changed by the step.

**Reason**: the handle is memory-only (`R2`), so a deep link to a code route would arrive with nothing
to verify and would have to fall back to the credential form — a declared route that is reachable only
in practice from the same tab and whose deep-link contract (`FND-08`) could never be honored. The
delivered gate already decides what the whole client renders from the session state
(`007-web-auth-session/spec.md`, `R8`), and the credential form is itself not a route: the auth
boundary renders it for every non-authenticated path. The code step is the same kind of state, so it
belongs in the same place.

The gate's existing navigation effect keeps its meaning: while the state is not authenticated it moves
the app to the catalog entry, so the deep link a person arrived on is still not resumed after the code
step completes — `WAS-24` holds unchanged.

**Rejected alternative**: a declared `/auth/code` route — rejected as an unreachable deep link and as a
new public route in a table whose every other entry is session-audience. Also rejected: rendering the
code form as a mode of the existing `auth_screen.tsx` component — the two surfaces share the panel and
the brand lockup but not the form, the copy or the state, and one component switching between them
would carry both forms' local state and both failure paths in one unit, which the `≥ 3 real usages`
componentization rule does not ask for and the readability of neither surface benefits from.

**Scope**: `web/src/auth/auth_gate.tsx`, `web/src/auth/auth_code_screen.tsx`,
`web/src/app/routes.ts` (unchanged), `web/src/test/auth_gate.test.tsx`, `web/e2e/auth.spec.ts`.

---

## R6 — Three dedicated failure kinds, one table, one copy per kind

**Decision**: `FailureKind` gains `invalidCode`, `emailUnavailable` and `conflict`; `KIND_BY_CODE`
gains `OTP_INVALID → invalidCode`, `EMAIL_UNAVAILABLE → emailUnavailable`, `CONFLICT → conflict`; and
the catalog gains one copy per kind. Every existing mapping and copy stays unchanged.

**Reason**: the three codes carry three different things a person must do — correct the digits, wait
for delivery to recover, or sign in to the address that already exists — and the delivered generic copy
("Something went wrong. Try again.") tells them none of it. Folding `OTP_INVALID` into the existing
`unauthorized` kind would show "Your session has ended. Sign in again." on a code surface where no
session ever existed, which is worse than the generic copy. A dedicated kind per outcome is also what
makes each copy assertable in the suites instead of being tested through a shared string.

`CONFLICT` is emitted by the server in exactly one place today — registration, for an address that
already has an account (`backend/internal/auth/service.go:207`) — so the `conflict` copy may name the
address and offer sign-in without being wrong anywhere else. That constraint is recorded rather than
assumed: if another route starts emitting `CONFLICT`, its copy must be revisited.

The table stays total over the codes the auth routes can answer: `INVALID_JSON`, `INVALID_REQUEST`,
`UNKNOWN_FIELD`, `UNAUTHORIZED`, `FORBIDDEN`, `CONFLICT`, `RATE_LIMITED`, `EMAIL_UNAVAILABLE`,
`OTP_INVALID`, `INTERNAL_ERROR`. Codes outside that set keep the delivered fallback — the generic kind —
so an unmapped code stays visible rather than misclassified (`WCS-12`).

**Rejected alternative**: reusing `unauthorized` for `OTP_INVALID` — the copy would lie about what
happened. Rejected: mapping `CONFLICT` to a generic "conflict" copy without naming the address — it
would leave the person with no next step at the exact moment registration fails. Rejected: a per-route
mapping table — the delivered mapper is global and every surface resolves through `failureCopy`, so a
second table would duplicate the classification.

**Scope**: `web/src/api/errors.ts`, `web/src/api/error_mapper.ts`, `web/src/l10n/strings.ts`,
`web/src/test/api_error_mapper.test.ts`, `web/src/test/api_errors.test.ts`.

---

## R7 — The challenge calls include credentials, carry no CSRF header and never trigger a refresh

**Decision**: `verifyChallenge` and `resendChallenge` go through the existing caller over the **raw**
transport with `credentials: 'include'`, and send no `X-CSRF-Token` header. They are not routed through
the authenticated transport, so they carry no Bearer header and cannot trigger a refresh.

**Reason**: the verification answer sets the refresh and CSRF cookies, and the API is cross-origin from
the app; without `credentials: 'include'` the browser would refuse to store the `Set-Cookie` the answer
carries, and the session would be established in name only. The raw transport is the delivered seam for
exactly this — the four session operations already use it — and `/auth/*` must never carry a Bearer
header or trigger a refresh (`WAS-16`).

No CSRF header is sent because neither route reads the refresh cookie: `verify` establishes a session
rather than consuming one, and `resend` consumes nothing but its own handle. The origin guard and the
CSRF echo exist for the routes that *use* the refresh credential (`refresh`, `logout`); the challenge
routes sit behind the per-address limiter only, as the challenge contract states. Sending a CSRF header
where the server does not check one would be a decoration, not a control.

**Rejected alternative**: sending the CSRF header defensively on both routes — it would assert a
protection the server does not apply there and would make the client's CSRF posture inconsistent with
the delivered contract. Rejected: routing the challenge calls through the authenticated transport —
it would attach a Bearer header the person does not have yet (no session exists while a step is
pending) and would arm the refresh path on an unauthenticated surface.

**Scope**: `web/src/api/auth_api.ts`, `web/src/test/auth_api.test.ts`, the capture script.

---

## R8 — The capture script branches on the answer it actually got

**Decision**: `capture.sh` keeps its current protocol and gains the code-step family. It detects which
answer registration returned — a session body or a code step — and captures the family it actually
received; the other family's files are left untouched. The README documents running it once per gate
state to refresh the whole set. No capture body is hand-written, ever.

**Reason**: the script runs against the local stack, whose gate is a server setting the script cannot
assume. Branching on the real answer keeps the delivered rule — every file is the server's own bytes —
and avoids a script that would have to be edited when the gate's default changes. A hand-written
code-step body would defeat the pinned suite, which exists to fail when the real contract drifts
(`API-34`, `API-35`, `API-36`).

The alternative — requiring the operator to restart the API with the gate on before every capture run —
was rejected because it makes the capture protocol depend on a deployment setting and invites a stale
fixture to look valid.

**Rejected alternative**: a separate capture script per gate state — two scripts would duplicate the
protocol and drift. Rejected: capturing only the code-step family and deleting the session fixtures —
the client must keep serving the gate-off deployment, so both families must stay pinned.

**Scope**: `web/src/api/fixtures/capture.sh`, `web/src/api/fixtures/README.md`, the seven new fixture
files, `web/src/test/api_contract_captures.test.ts`.

---

## R9 — No new dependency; the countdown is a plain effect timer

**Decision**: the resend countdown is a React effect interval owned by the code surface, seeded from
the advertised window and re-seeded when a resend answer or a refusal moves it. No timer, date or state
library is added. The suite asserts the enable boundary with the injected fake timers Vitest already
provides.

**Reason**: the delivered client already keeps its interface timers in effects and its suites already
drive time without waiting — the draft cadence in the sheet cycle is injected for exactly this reason.
Adding a library for one interval would be the first dependency the auth boundary carries beyond React
itself. The window is a duration in seconds the server advertises, so no date arithmetic is needed
either.

**Rejected alternative**: a scheduling library or the query library's timer utilities — rejected as an
unnecessary dependency for one countdown, against the delivered "no new package without strict
necessity" rule.

**Scope**: `web/src/auth/auth_code_screen.tsx`, `web/src/test/auth_code_screen.test.tsx`.

---

## R10 — An answer for an abandoned step is discarded by comparing the handle

**Decision**: `submitCode` captures the handle it is verifying; when the answer resolves, it commits
only if the session's pending handle is still that same handle. A verification that resolves after the
step was abandoned or after a newer step replaced it is dropped, and no session is established.

**Reason**: the delivered client already guards stale asynchronous results — the refresh applies only
to the session version it belongs to (`WAS-17`) — but the pending step has no version to compare, and
inventing one would widen the session's version semantics beyond the boundaries the delivered spec
defines. The handle is already a unique value per step, so comparing it is sufficient, needs no new
state, and is directly assertable: abandon the step, resolve the in-flight verification, and assert the
credential form is still shown with no token held.

**Rejected alternative**: bumping the session version on abandonment — it would redefine what the
version counts (session boundaries, not step boundaries) and would make the guard indistinguishable
from a genuine session end in the suites. Rejected: an in-flight generation counter — a second counter
beside the version, for the same job the handle already does.

**Scope**: `web/src/auth/auth_session.ts`, `web/src/test/auth_session.test.ts`.

---

## Summary of resolved unknowns

| Question the plan left open | Resolution |
|---|---|
| How the two answers are told apart | Payload shape, in the decode seam (`R1`) |
| Whether the pending handle survives a reload | No — memory-only, no new local record (`R2`) |
| How the resend action honours the server's hint | It honours the local window; the header is unreadable cross-origin (`R3`) |
| Where the pending state lives | In the session, handle module-private (`R4`) |
| Where the code surface lives | A state of the auth surface, no route (`R5`) |
| How the new codes are classified | Three dedicated kinds, one table, one copy each (`R6`) |
| The credentials posture of the new calls | Raw transport, `include`, no CSRF, no Bearer (`R7`) |
| How the captures cover both gate states | The script branches on the real answer (`R8`) |
| What the countdown costs | One effect timer, no dependency (`R9`) |
| How a stale verification is discarded | By comparing the handle (`R10`) |

No unknown remains from the Technical Context; the specification's three clarifications were resolved
by user decision on 2026-10-20, and one requirement (`WCS-09`) was amended to match `R3`.
