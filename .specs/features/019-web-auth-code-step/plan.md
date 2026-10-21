# Implementation Plan: The Code Step in the Client — Registration and Sign-in Hand Back a Code Screen, Not a Session

**Branch**: `019-web-auth-code-step` | **Date**: 2026-10-20 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/019-web-auth-code-step/spec.md`

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §9 stage 5 and §3.5 — decisions D1 (2FA on every
sign-in), D2 (a session only after verification), D4 (existing accounts verify at first sign-in);
finding F-11 (the new codes fall to `unknown` on the client). This is the client slice: it adapts the
delivered web client to the gated contract the preceding features shipped, and it changes no Go
source. It does not consume `GET /auth/me` (deferred), does not declare a route for the code surface,
and does not persist the pending handle.

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Make the client understand the two answers the gated server can give, and give the person a way to
finish the step:

1. **The credential operations accept either answer.** `register` and `login` stop decoding a session
   body and decode a discriminated *sign-in outcome* instead — a session body or a code step. The
   discrimination comes from the payload's own shape (`access_token` versus `challenge_id`), never
   from the status, because registration answers `201` in both gate states. The session branches on
   the outcome: a session body establishes the session exactly as today, a code step enters the
   pending state and holds no credential at all.
2. **A pending-code state and a code surface.** `AuthSession` gains the `pendingCode` state carrying
   the handle (module-private), the origin operation, the address, the advertised lifetime and the
   resend window; `AuthGate` renders the new code surface instead of the credential form while a step is
   pending, on the same single auth surface and at the same URL. The surface names the address and the
   code's lifetime, collects the six-digit code over six single-digit positions, owns the focus, and
   offers resend with a countdown, an honest confirmation and one way back.
3. **Two new operations on the raw transport.** `verifyChallenge` and `resendChallenge` join the four
   session operations in `auth_api.ts`; both go through the existing caller over the **raw** transport
   with `credentials: 'include'`, so the verification answer's `Set-Cookie` is honored cross-origin,
   and neither carries a Bearer header, a CSRF header or a refresh.
4. **The new codes get a kind and a copy.** `error_mapper.ts` gains three kinds —
   `invalidCode`, `emailUnavailable`, `conflict` — and the catalog gains the matching copy, so a wrong
   digit, an exhausted send allowance and a duplicate address stop collapsing into "Something went
   wrong." The catalog also gains a **parameterized** resolution, so the copy that carries the address,
   the remaining time or the account mode is resolved by the catalog and never concatenated by a
   surface.

The change is confined to the web client: `src/api` (the outcome decode, the two operations, the
failure kinds and the mapping), `src/auth` (the session state, the code surface, its styles),
`src/l10n` (the copy and the parameterized resolution), `src/api/fixtures` (the pinned captures and the
capture script), `web/DESIGN.md` (the code field's and the surface's visual contract) and the flat
suites under `src/test/` plus the auth end-to-end spec. **No new dependency, no new route, no new
storage, and no Go change.** `WAS-02`'s premise is reconciled in the corpus in the same change
(spec `WCS-18`).

## Technical Context

**Language/Version**: TypeScript 6.0.3 (`web/package.json`) on React 19.3.1, `react-router-dom`
7.18.4, Vite 8.3.4 and Node `>=24 <25` — the pins the repository already carries. The client is the
React/TypeScript PWA under `web/`; the backend is untouched.

**Primary Dependencies**: none added. The work reuses the existing seams only: the injected
`Transport`, `createCaller`/`decodeEnvelope`, the `DomainFailure`/`DefensiveParseFailure` taxonomy,
the classification mapper and the translation catalog. React's `useSyncExternalStore` continues to
carry the session state to the gate and the surfaces, and the resend countdown is a plain effect timer
in the surface — no state library, no query library involvement, no date library. The translation
catalog gains a **parameterized** resolution — a second exported function in the same module, not a
dependency — and the code field's six positions are one editable field plus presentational markup, so
no component library and no formatting library come in. The auth objects are already constructed in
`src/app/providers.tsx`; its options are unchanged, so the composition root is not edited.

**Storage**: none. The pending handle, the origin, the address, the username and the entered code live
in the session's in-memory state and die with the tab; the access credential stays in the module-private
field it already uses. **No `localStorage`, `sessionStorage`, IndexedDB or cookie write is added**
(`WCS-05`, `WCS-16`, `SC-007`). The address and the username are identifiers the person typed, not
credentials, and they are never persisted (`WCS-19`, `WCS-21`). The registered-capture set under
`src/api/fixtures/` is committed fixture data, not runtime storage.

**Testing**: `npm test` (Vitest 5.0.5) with the injected fake transport and Testing Library
(`@testing-library/react` 16.3.3, `user-event` 14.6.8); `npm run test:e2e` (Playwright 1.64.1 with
`@axe-core/playwright` 4.14.0) for the auth surface's accessibility and the code-step flow. Suites are
flat under `src/test/` and keep the edge-case-first order with `Happy path:` / `Edge:` / `Error:`
cases; interface requirements are asserted through rendering tests rather than assumed. The pinned
capture suite decodes the committed fixtures with **no backend running**, so a shape change in the
code-step contract fails the suite (`WCS-14`). Failure paths assert the mapped kind and copy. The
suites that currently assume "registration and sign-in return a session" — `auth_api.test.ts`,
`auth_session.test.ts`, `auth_screen.test.tsx`, `auth_gate.test.tsx`, `session_boundary.test.ts` —
move in the same change, and the legacy path is asserted beside the code-step path so the two cannot
drift.

**Target Platform**: the browser (the installable PWA). The app is served at `app.legnaris.com.br`
and the API at `api.legnaris.com.br` — cross-origin, same site — which is why the challenge calls must
include credentials for the verification answer's `Set-Cookie` to be accepted, and why the response's
`Retry-After` header is **not** readable by the client (see Constraints).

**Project Type**: web client feature in `web/`; the Go backend and its suites are untouched.

**Performance Goals**: no throughput target. A completed code step costs one extra request over
today's flow (the verification), and a resend costs at most one; the countdown is a single interval
timer per mounted surface and is cleared on unmount. No polling, no background work.

**Constraints**: the discrimination between the two answers **must not** branch on the status, because
registration answers `201` with a session body when the gate is off and `201` with a challenge body
when it is on (`WCS-01`). The client cannot read the server's `Retry-After` hint: `Retry-After` is not
a CORS-safelisted response header, so a cross-origin `fetch` cannot expose it without a server-side
`Access-Control-Expose-Headers` change, which this feature does not make; the resend action's disabled
state is therefore governed by the local window seeded from the challenge answer's advertised
`resend_after`, and the unavailable-send copy tells the person to try again shortly without computing
an instant from the header (`WCS-07`, `WCS-09`, research `R3`). The transport seam stays
`{ status, body }` — no header plumbing is added. No credential and no handle is written to any
JavaScript-readable store, no third-party script/style/font is introduced, no DOM-injection sink is
added, every string resolves through `src/l10n/strings.ts` and every visual value from the token set,
and all source stays English (`WCS-16`, `WCS-15`).

**Scale/Scope**: 3 user stories (2 P1, 1 P2), 27 requirements, 4 internal contracts; 9 touched or
created client modules plus the capture script and its README, `web/DESIGN.md`, 8 suites (7 extended, 1
new) and 7 new fixtures; one amended spec (`WAS-02`). No backend file, no dependency, no route, no
storage.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | No new failure mode is introduced on the server, and the client's three new codes already exist in `backend/shared/apperr` with mapped statuses (`OTP_INVALID` 401, `EMAIL_UNAVAILABLE` 503, `CONFLICT` 409). The client's obligation is the mirror image: every one of them gets a dedicated `FailureKind` and a catalog copy, and an unclassified code keeps the generic kind rather than being coerced (`WCS-11`, `WCS-12`). No new failure is invented on the client. | PASS |
| II. Canonical Identifiers | No identifier is generated or validated on the client. The challenge handle is opaque: the client carries the server's canonical value and never parses, normalises or synthesises it, and the code is treated as an opaque digit string. The requirement-ID lint covers the TypeScript sources this feature touches. | PASS |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | Every requirement lands with tests in the same commit, edge cases first: both gate states for register and sign-in; the shape discrimination including the ambiguous and the malformed payload; a refused verification, a refused resend, an unavailable send, a transient failure, and an answer that arrives for an abandoned step; the countdown's enable boundary; the three new mappings and an unknown code; the captures decoding without a backend. Interface requirements are asserted through rendering tests; the auth surface and the code surface keep zero critical automated accessibility violations (`WCS-17`, `SC-005`, `SC-006`). The moved suites assert the legacy path beside the new one. | PASS |
| IV. English-Only Source | Identifiers, comments, log messages and every new user-facing string are English, and the strings live in the catalog rather than at the call site (`WCS-16`). No pt-BR string is added; `docs/` stays pt-BR and untouched. | PASS |
| V. Layered Architecture | The client's equivalent layering is respected: the API module keeps decoding and emitting codes/kinds only, the session keeps the state machine, and the surface keeps presentation — no screen parses a payload and no API module authors copy. The new surface imports the session through `src/app/providers` exactly as the existing surfaces do; no module reaches into another's internals. The API module keeps its no-logging and no-literal rules. | PASS |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | The change preserves every control and adds none that weakens one: the pending handle and the code are memory-only, no credential enters JS-readable storage, the challenge calls keep the raw transport with no Bearer and no refresh trigger, no third-party asset or DOM-injection sink is added, and the CSP/CSRF posture is unchanged. The one security-relevant judgement — carrying the handle in memory only — is the stricter of the two options (`SC-007`). | PASS |
| VII. Installable, Offline-Capable Web Client | The code surface is part of the application shell's auth boundary and is reachable offline exactly as the credential form is; no authenticated response is cached and the service worker's rules are untouched. The surface's accessibility is gated by the existing axe sweep, which gains the new surface (`WCS-17`, `SC-006`). | PASS |
| Additional Constraints — Stack | React with Vite and TypeScript under `web/`, Vitest + Testing Library + Playwright — unchanged; **no dependency is added** and no Go source is touched. | PASS |
| Additional Constraints — Spec corpus and traceability | `WCS-01`…`WCS-27` are defined in this feature's `spec.md`; the superseded `WAS-02` stays defined in its own spec and is amended in place with a pointer to the superseding requirement, so `scripts/check_requirement_ids.py` stays green and no citation resolves to a statement that no longer describes the shipped behaviour (`WCS-18`). | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card; recorded for the PR, not claimed by this artifact. | PASS (noted) |
| Development Workflow | The client sequence (`make lint-web && make test-web && make sec-scan`) covers the change; `make lint-specs` covers the amended spec and the new citations; the auth end-to-end spec gains the code-step case and the axe sweep over the new surface. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. One implementation per rule: the two answers are
told apart in **one** place (the decode seam in `auth_api.ts`), so no screen branches on the payload;
the pending state has **one** owner (the session), so the gate, the surface and the resend countdown
cannot disagree about whether a step is pending; the handle never leaves the session, mirroring the
access credential, so there is no second copy to keep in sync; and each new failure code is classified
in **one** table, so an unmapped code stays visible rather than silently generic. Three choices that
could have drifted were closed in research: the discrimination is by payload shape rather than status
(`R1`), the handle stays memory-only rather than persisted (`R2`), and the resend action's disabled
window is the local window rather than the server's unreadable `Retry-After` (`R3`).

**Post-design re-check (after the design review of 2026-10-20)**: still PASS, and three more
single-owner rules hold. The code's value lives in **one** place — the editable field — and the six
positions render it, so no copy of the code can disagree with the one that is sent (`R14`). The
surface's focus is owned by the surface itself, so no screen outside it can move the focus and leave it
stranded (`R13`). The copy that carries a value is resolved by the catalog through **one**
parameterized resolution, so no surface concatenates a sentence with a value (`WCS-13`). The design
review adds no dependency, no route, no store and no server contract: the address and the advertised
lifetime are fields the client already receives, and the email is the sibling surface the screen is
made to agree with (`R11`).

## Project Structure

### Documentation (this feature)

```text
.specs/features/019-web-auth-code-step/
├── spec.md                          # Requirements (WCS-*)
├── plan.md                          # This file
├── research.md                      # Phase 0 — decisions and rejected alternatives
├── data-model.md                    # Phase 1 — the outcome, the pending state, the surface
├── quickstart.md                    # Phase 1 — runnable validation guide
├── contracts/                       # Phase 1 — one contract per internal seam
│   ├── auth-operations.contract.md
│   ├── session-states.contract.md
│   ├── failure-mapping.contract.md
│   └── code-surface.contract.md
├── checklists/
│   └── requirements.md              # Spec quality checklist
└── tasks.md                         # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
web/src/api/
├── auth_api.ts                      # EXTENDED: the sign-in outcome, the shape discrimination, verifyChallenge and resendChallenge (WCS-01, WCS-03, WCS-07, WCS-15)
├── error_mapper.ts                  # EXTENDED: three new kinds in the mapping table and their copy keys (WCS-11, WCS-13)
└── errors.ts                        # EXTENDED: FailureKind gains invalidCode, emailUnavailable, conflict (WCS-11)

web/src/auth/
├── auth_session.ts                  # EXTENDED: the pendingCode state, the outcome branch, submitCode, resendCode, cancelCodeStep (WCS-02, WCS-03, WCS-04, WCS-06, WCS-08, WCS-19, WCS-21)
├── auth_code_screen.tsx             # NEW: the code surface — the instruction, the code field over six positions, the resend action and its countdown, the way back (WCS-03, WCS-05, WCS-07, WCS-09, WCS-19…WCS-26)
├── auth_gate.tsx                    # EXTENDED: render the code surface while a step is pending (WCS-02)
└── auth.css                         # EXTENDED: the code surface's token-only styles (WCS-16, WCS-24, WCS-25)

web/src/l10n/
└── strings.ts                       # EXTENDED: the code-surface copy, the three new failure copies, the parameterized resolution (WCS-13, WCS-16)

web/DESIGN.md                        # EXTENDED: the code field's and the code surface's visual contract (WCS-24, WCS-25)

web/src/api/fixtures/
├── capture.sh                       # EXTENDED: capture the code-step family when the stack's gate is on
├── auth_register_challenge.json     # NEW: registration answered with a code step (WCS-14)
├── auth_login_challenge.json        # NEW: sign-in answered with a code step (WCS-14)
├── auth_challenge_verify.json       # NEW: verification answered with a session (WCS-14)
├── auth_challenge_resend.json       # NEW: resend answered with its window (WCS-14)
├── error_otp_invalid.json           # NEW: the uniform invalid-code refusal (WCS-14)
├── error_email_unavailable.json     # NEW: the unavailable-send refusal (WCS-14)
└── error_conflict.json              # NEW: the duplicate-address refusal (WCS-14)

web/src/test/
├── auth_api.test.ts                 # EXTENDED: both answers decoded, the discrimination, the new operations, the credentials posture
├── auth_session.test.ts             # EXTENDED: the outcome branch, the pending state, submit, resend, abandonment, the stale-answer guard
├── auth_code_screen.test.tsx        # NEW: the surface's rendering, the countdown, the copy, the accessibility contract
├── auth_screen.test.tsx             # EXTENDED: the moved credential-form cases
├── auth_gate.test.tsx               # EXTENDED: the gate renders the code surface while pending and never treats a step as a session
├── session_boundary.test.ts         # EXTENDED: the session boundary rules still hold with the code step in front of them
├── api_error_mapper.test.ts         # EXTENDED: the three new kinds and the unmapped fallback
└── api_contract_captures.test.ts    # EXTENDED: the code-step fixtures decode and stay pinned (WCS-14)

web/e2e/
└── auth.spec.ts                     # EXTENDED: the code-step flow and the axe sweep over the code surface (WCS-17)

web/src/api/fixtures/README.md        # EXTENDED: how to capture the code-step family (both gate states)
```

**Structure Decision**: the two answers are told apart in the **API module**, because that is the only
place that sees the payload before it becomes a type — the same reason the envelope decoder lives
there. A screen that branched on the payload would duplicate the contract and drift from the decoder.

The pending state belongs to `auth_session.ts`, which already owns the session state machine, the
in-memory credential and the `useSyncExternalStore` snapshot the gate and the surfaces read; putting it
anywhere else would create a second source of truth for "is a step pending". The handle stays
module-private exactly as the access token does, so the surfaces can act on the step without ever
holding it.

The code surface is a new component beside `auth_screen.tsx`, rendered by `auth_gate.tsx` — a state of
the single auth surface rather than a declared route, because the handle is memory-only and a deep link
would arrive with nothing to verify (`WCS-02`, clarification 2026-10-20). Its styles join `auth.css`
so the auth boundary keeps one stylesheet and the token scan keeps one place to check.

No new module is created under `src/api`: the outcome decode and the two operations extend the file
that already owns the four session operations, mirroring how the challenge domain extended
`internal/auth` rather than growing a package. No new module is created under `src/l10n` either; the
copy extends the single catalog.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
