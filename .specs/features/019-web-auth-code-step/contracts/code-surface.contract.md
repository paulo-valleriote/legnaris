# Contract: The code surface

**Feature**: `.specs/features/019-web-auth-code-step/` | **Requirement IDs**: `WCS-03`, `WCS-05`,
`WCS-07`, `WCS-09`, `WCS-16`, `WCS-17`, `WCS-19`, `WCS-20`, `WCS-21`, `WCS-22`, `WCS-23`, `WCS-24`,
`WCS-25`, `WCS-26` | **Owner**: `web/src/auth/auth_code_screen.tsx`,
`web/src/auth/auth_gate.tsx`

## Purpose

Fix what the person sees and what the surface sends while a code step is pending: the address and the
code's lifetime named, one code field, one resend action with an honest countdown and an honest
confirmation, one way back — rendered in place of the credential form on the single auth surface, with
no route and no stored state.

## Shape

```tsx
// Rendered by AuthGate while snapshot.state === 'pendingCode'.
function AuthCodeScreen(): JSX.Element;
```

It reads the session through `useAuthSession()` and takes exactly: `snapshot.pending` (the origin, the
address, the username, the advertised lifetime and the window) plus `submitCode`, `resendCode`,
`cancelCodeStep`. It never sees the handle.

## The composition

The blocks appear in this order, and only in this order (`WCS-25`):

1. the brand lockup, exactly as the credential form renders it;
2. the **origin line** — a static line naming the account mode (`New account` / `Existing account`) that
   replaces the credential form's mode switch, so the panel keeps its shape and the person keeps the
   frame of what they are doing (`WCS-25`);
3. the heading;
4. the **instruction** — one line naming the address the code was sent to and how long the code remains
   valid, in the same unit the emailed message uses (`WCS-19`, `WCS-20`);
5. the **code field** — six single-digit positions over one editable field (`WCS-24`);
6. the **refusal** — the mapped copy, associated with the code field (`WCS-04`, `WCS-22`);
7. the **primary action** — the submit, disabled and showing its progress label while a verification is
   in flight;
8. the **resend row** — the lead-in, the action and the countdown (`WCS-07`, `WCS-23`, `WCS-26`);
9. the **way back** — always present, not only after a refusal (`WCS-04`, `WCS-21`).

The swap between the credential form and this surface is limited to a fade and a small movement,
lasts no longer than the longest declared motion duration, and is zeroed when the person asks for
reduced motion (`WCS-25`).

## Rules

- The heading and the instruction are chosen by `pending.origin`: a registration step says the address
  must be confirmed; a sign-in step says the emailed code signs the person in. The purpose is never sent
  or chosen by the client (`TFA-03`).
- The instruction names `pending.email` verbatim — it is not masked, because a mistyped address is the
  most common reason a code never arrives and the person must be able to see and correct it
  (`WCS-19`).
- The instruction states the remaining validity from `pending.expiresIn`, in the unit the emailed
  message uses. The statement is re-seeded when a resend extends the step (`WCS-20`). When the stated
  lifetime has passed, the surface shows the expired state and offers the resend and the way back
  (`WCS-20`).
- The code field is **one editable field** carrying the whole code, presented as six single-digit
  positions that follow the field's value. The field has a visible permanent label, an accessible name,
  a numeric input mode and the platform's one-time-code autofill, so a paste, an autofill and a code
  with leading zeros land whole (`WCS-24`, `WCS-03`, `WCS-05`). The positions are presentational and
  never a second copy of the value (`WCS-24`).
- When the surface appears, the code field takes the focus, and the field is described by the
  instruction line, so the address and the stated lifetime are read with the field (`WCS-22`).
- Submitting sends the code through `session.submitCode`; while the submission is in flight the action
  is disabled and shows its progress label, so a double submit issues one request. The resend action is
  unavailable while a verification is in flight (`WCS-03`, `WCS-08`).
- A refusal shows `failureCopy(cause)` inline, keeps the entered code and the pending step, and leaves
  the submit available (`WCS-04`). The refusal is associated with the code field — the field is marked
  invalid and the refusal is the field's description — and the focus stays on the field, because that
  is where the correction is made (`WCS-22`).
- The resend action is unavailable while the window runs and shows the remaining seconds as text, seeded
  from `pending.resendAfter` and ticking locally; at zero it becomes available (`WCS-07`). While it is
  unavailable it stays reachable by keyboard and by assistive technology, so the person can find it and
  learn why it cannot be used yet (`WCS-07`).
- The countdown is static text **outside any live region**: a ticking second is never announced again and
  again. The change that makes the action available is announced once, and the action's accessible name
  stays the same while its availability changes (`WCS-23`).
- Activating resend calls `session.resendCode()` once and re-seeds the countdown from the answer's
  window; while in flight it is unavailable, and it never clears the entered code (`WCS-08`, `WCS-10`).
- A recorded resend is confirmed inline, in a status an assistive technology announces once, and the
  confirmation says that either code works — a resend invalidates neither the handle nor the code the
  person already holds (`WCS-26`, `WCS-10`). The confirmation replaces an earlier refusal rather than
  sitting beside it (`WCS-26`).
- An unavailable resend shows the mapped copy and keeps the action unavailable for the remainder of the
  current window; the surface does not read or compute the server's `Retry-After` (`WCS-09`, `R3`).
- The way back calls `session.cancelCodeStep()`, which returns the credential form with the address and
  the account mode the step started with, an empty password field, and no request (`WCS-06`, `WCS-21`).
  The focus moves to the credential form (`WCS-22`).
- Every string resolves through `src/l10n/strings.ts`, including the three that carry a value; every
  visual value comes from the `--lg-*` token set declared in `src/theme/`; the styles join `auth.css`
  (`WCS-16`, `WCS-13`).
- The surface reuses the auth panel's structure and brand lockup so the boundary keeps one look, and it
  exposes an accessible role and name for its field, its actions and its alert (`WCS-17`).
- The countdown is one effect interval, cleared on unmount and re-seeded when the window moves (`R9`).
  The stated lifetime is tracked by the same surface without a second interval, so at most one timer
  runs (`WCS-25`).

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
   accessible roles and names; the countdown is announced as text, not as a bare spinner, and not once
   per tick (`WCS-23`).
8. **One value for the code.** The six positions render the field's value; no second copy of the code
   exists in the surface (`WCS-24`).
9. **The focus never leaves.** The focus is inside the surface that is showing at every moment: on the
   field when it appears, on the field after a refusal, on the form after the way back (`WCS-22`).

## Boundary cases

| Case | Result |
|---|---|
| code submitted with leading zeros | sent verbatim (`WCS-03`) |
| the whole code pasted at once | every position follows the field's value; sent verbatim (`WCS-24`) |
| submission in flight, submit pressed again | one request (`WCS-08` sibling rule for verify) |
| refusal shown | code kept, submit available, the refusal associated with the field, focus on the field (`WCS-04`, `WCS-22`) |
| window running | the action is unavailable but reachable, remaining seconds shown (`WCS-07`) |
| window elapses while typing | the action becomes available, the code untouched (`WCS-07`) |
| resend answered | countdown re-seeded, code untouched, the confirmation announced once (`WCS-08`, `WCS-10`, `WCS-26`) |
| resend answered unavailable | copy shown, handle kept, the action re-arms at the window's end (`WCS-09`) |
| a resend confirmation after an earlier refusal | the confirmation replaces the refusal (`WCS-26`) |
| the stated lifetime elapses | the expired state with the resend and the way back offered (`WCS-20`) |
| "start over" | the credential form returns with the address and the mode, the password empty, no request (`WCS-06`, `WCS-21`) |
| surface unmounted while the window runs | the interval is cleared; no leak (`R9`) |

## Test evidence

- `web/src/test/auth_code_screen.test.tsx`: the origin-specific copy; the instruction naming the address
  and the stated lifetime; the field's label, input mode and autofill; the six positions following the
  field's value; the focus on the field when the surface appears and after a refusal; the refusal
  associated with the field; the countdown's enable boundary under fake timers and its single
  announcement; the resend single activation, the re-seed and the confirmation; the unavailable copy;
  "start over" clearing and issuing nothing; the alert's role and name.
- `web/src/test/auth_gate.test.tsx`: the gate renders the code surface while pending and never renders
  the shell; a code-step answer is never treated as a session.
- `web/e2e/auth.spec.ts`: the code-step flow end to end and the axe sweep over the code surface.
