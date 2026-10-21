# Contract: The code surface

**Feature**: `.specs/features/019-web-auth-code-step/` | **Requirement IDs**: `WCS-03`, `WCS-05`,
`WCS-07`, `WCS-09`, `WCS-16`, `WCS-17` | **Owner**: `web/src/auth/auth_code_screen.tsx`,
`web/src/auth/auth_gate.tsx`

## Purpose

Fix what the person sees and what the surface sends while a code step is pending: one code field, one
resend action with an honest countdown, one way back — rendered in place of the credential form on the
single auth surface, with no route and no stored state.

## Shape

```tsx
// Rendered by AuthGate while snapshot.state === 'pendingCode'.
function AuthCodeScreen(): JSX.Element;
```

It reads the session through `useAuthSession()` and takes exactly: `snapshot.pending` (the origin and
the window) plus `submitCode`, `resendCode`, `cancelCodeStep`. It never sees the handle.

## Rules

- The heading and subtitle are chosen by `pending.origin`: a registration step says the address must be
  confirmed; a sign-in step says the emailed code signs the person in. The purpose is never sent or
  chosen by the client (`TFA-03`).
- The code field is a text input with `inputMode="numeric"`, `autoComplete="one-time-code"` and a
  label, so a numeric keyboard and the platform's OTP auto-fill are offered and a paste lands whole
  (`WCS-03`, `WCS-05`).
- Submitting sends the code through `session.submitCode`; while the submission is in flight the action
  is disabled and shows its progress label, so a double submit issues one request.
- A refusal shows `failureCopy(cause)` inline as an alert, keeps the entered code and the pending step,
  and leaves the submit available (`WCS-04`).
- The resend action is disabled while the window runs and shows the remaining seconds as text, seeded
  from `pending.resendAfter` and ticking locally; at zero it enables (`WCS-07`).
- Activating resend calls `session.resendCode()` once and re-seeds the countdown from the answer's
  window; while in flight it is disabled, and it never clears the entered code (`WCS-08`, `WCS-10`).
- An unavailable resend shows the mapped copy and keeps the action disabled for the remainder of the
  current window; the surface does not read or compute the server's `Retry-After` (`WCS-09`, `R3`).
- The "start over" action calls `session.cancelCodeStep()`, which returns the credential form and
  clears the handle and the code, issuing no request (`WCS-06`).
- Every string resolves through `src/l10n/strings.ts`; every visual value comes from the `--lg-*` token
  set declared in `src/theme/`; the styles join `auth.css` (`WCS-16`).
- The surface reuses the auth panel's structure and brand lockup so the boundary keeps one look, and it
  exposes an accessible role and name for its field, its actions and its alert (`WCS-17`).
- The countdown is one effect interval, cleared on unmount and re-seeded when the window moves (`R9`).

## Invariants

1. **No handle, no code on the wire beyond the request.** The handle is never rendered, never placed in
   the DOM, never put in the URL and never written to a store; the code appears only in the request body
   (`WCS-05`).
2. **No credential while pending.** The surface holds no token and offers no path to the app shell
   until the verification succeeds.
3. **One live window.** At most one interval runs; a re-seed replaces the previous one.
4. **One request per activation.** A double submit or a double resend issues one request.
5. **In-place.** The surface replaces the credential form without changing the URL or declaring a route
   (`R5`).
6. **Token-only visuals.** No literal colour, size or spacing appears in the surface or its styles.
7. **Accessible.** Zero critical automated violations; the field, both actions and the alert expose
   accessible roles and names; the countdown is announced as text, not as a bare spinner.

## Boundary cases

| Case | Result |
|---|---|
| code submitted with leading zeros | sent verbatim (`WCS-03`) |
| submission in flight, submit pressed again | one request (`WCS-08` sibling rule for verify) |
| refusal shown | code kept, submit available, alert copy from the catalog (`WCS-04`) |
| window running | action disabled, remaining seconds shown (`WCS-07`) |
| window elapses while typing | action enables, code untouched (`WCS-07`) |
| resend answered | countdown re-seeded, code untouched (`WCS-08`, `WCS-10`) |
| resend answered unavailable | copy shown, handle kept, action re-arms at the window's end (`WCS-09`) |
| "start over" | credential form returns, handle and code cleared, no request (`WCS-06`) |
| surface unmounted while the window runs | the interval is cleared; no leak (`R9`) |

## Test evidence

- `web/src/test/auth_code_screen.test.tsx`: the origin-specific copy; the field's
  `inputMode`/`autoComplete`/label; submit in flight; the refusal copy and the kept code; the countdown
  enable boundary under fake timers; the resend single activation and re-seed; the unavailable copy;
  "start over" clearing and issuing nothing; the alert's role and name.
- `web/src/test/auth_gate.test.tsx`: the gate renders the code surface while pending and never renders
  the shell; a code-step answer is never treated as a session.
- `web/e2e/auth.spec.ts`: the code-step flow end to end and the axe sweep over the code surface.
