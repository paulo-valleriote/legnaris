# Feature Specification: The Code Step in the Client — Registration and Sign-in Hand Back a Code Screen, Not a Session

**Feature Branch**: `019-web-auth-code-step`

**Created**: 2026-10-20

**Status**: Draft

**Input**: User description: "adapt the web client to the email-code sign-in: the registration and sign-in answers carry a code step instead of a session, the client holds a pending-code state and renders a code screen, and the new failure codes are classified and copied" (plan `docs/plan-auth-otp-2fa-email.md` §9 stage 5 and §3.5; decisions D1, D2, D4; finding F-11; the premise recorded as superseded by `.specs/features/018-auth-2fa-session-gate/spec.md`)

## Clarifications

### Session 2026-10-20

- Q: `GET /auth/me` was delivered by the preceding feature (`TFA-08`) and the plan's §3.2 argues the client needs it after a reload. Does this feature consume it? → A: Defer it (user decision, 2026-10-20): stage 5's bullet list is the challenge operations, the pending state, the code surface, the mapper and the copy, and nothing in this feature renders verification-dependent content, so the identity read stays delivered on the server and unused on the client until the stage whose screens need it; the deferral is recorded in Assumptions.
- Q: The plan's §3.5 asks for a "code screen" without saying where it lives. Does it replace the credential form in place, or does it become its own declared route? → A: In place on the single auth surface (user decision, 2026-10-20): the auth gate renders either the credential form or the code surface, and no route is declared, because the handle is memory-only and a deep link would arrive with no handle to verify (`WCS-02`, `WCS-06`).
- Q: A reload while a code step is pending: is the pending handle remembered? → A: Memory-only (user decision, 2026-10-20): the handle lives in memory exactly like the access credential, a reload loses it and the credential form returns, and the person starts the step again; no new local record and no new sweep rule is introduced (`WCS-02`, `WCS-03`).
- Q: US3 promises the refusal names the real cause, but a wrong password on the credential form still resolves to the generic session copy. Is that in scope? → A: No — recorded as a residual (user decision, 2026-10-20): this feature gives kinds only to the codes it introduces, so the credential-surface refusal's copy is unchanged and the mismatch is documented in Assumptions, exactly as the preceding feature recorded the registration half of finding F-05 (`WCS-11`, Assumptions).

### Session 2026-10-20 (design review)

- Q: The code surface describes a generic subtitle, so the person cannot tell which address the code went to — and a mistyped address is the most common reason a code never arrives. Does the surface name the address? → A: Yes: the pending step carries the address the credential operation carried, the surface names it in its instruction line, and the address is never persisted. The address is an identifier the person typed, not a credential (`WCS-19`).
- Q: The client discards the challenge's advertised lifetime, but the emailed message the person received already promises that the code expires in a number of minutes. Does the surface state the lifetime? → A: Yes: the pending step carries the advertised lifetime, the surface states it in the same unit and wording the email uses, and the statement is re-seeded when a resend extends the step (`WCS-20`).
- Q: Abandoning the step unmounts the credential form, so the typed values are lost and the person must retype the address — exactly the field they may have got wrong. Does the way back restore them? → A: Yes: the pending step carries the address and the account mode; the way back restores both and clears the password (`WCS-21`).
- Q: The contract never says where the focus goes when the surface appears, and the refusal is announced away from the field the person must correct. → A: The code field takes the focus when the surface appears, a refusal keeps the focus there and is associated with that field, and the way back moves the focus to the credential form (`WCS-22`).
- Q: A countdown that re-announces itself every second would flood an assistive technology, and the contract's "announced as text" is ambiguous about it. → A: The countdown is static text outside any live region; the change that makes the resend action available is announced once (`WCS-23`).
- Q: Is the code field a plain text input or a set of single-digit positions? → A: Six positions over one editable field that holds the whole code, so the digits become a visual fact, the person sees which digit is missing, and a paste, a platform autofill and leading zeros still land whole (`WCS-24`).
- Q: The contract lists what the surface does but never the order of its blocks, so the composition is left to the implementer, and the account-mode switch simply vanishes. → A: The composition is specified: the origin line replaces the mode switch, the blocks follow a fixed order, and the swap between the two surfaces is one restrained transition (`WCS-25`).
- Q: A resend confirmation could tell the person to use the newest code, which would be false — the earlier code still verifies. And the duplicate-address copy offers sign-in without offering a way to take it. → A: The resend confirmation is an inline status that says a new code was sent and that either code works; the duplicate-address refusal carries an inline action that switches to sign-in, keeps the address and moves the focus to the password field (`WCS-26`, `WCS-27`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Registering or signing in now asks for a code (Priority: P1)

Someone registers a fresh account, or signs in with a correct password. Today the answer is a working session: the app goes straight to the catalog. With the gate on, the same answer carries only a handle for a six-digit code that is on its way by email — no access credential, no session. The app must recognise that answer for what it is, keep no credential, and put a code surface in front of the person until the digits come back and the session is finally established.

**Why this priority**: this is the client half of decision D1. Without it the gated server contract has no client: registration and sign-in would establish nothing and the app would sit in a broken half-state for every person whose gate is on.

**Independent Test**: drive the credential operations with a challenge answer and assert no session is established, no token is held and the code surface renders; assert the surface names the address the code went to and the code's lifetime, and that the code field holds the focus; submit the correct code and assert the session is established exactly as a session answer does; submit a wrong code and assert the surface stays, the copy is the mapped invalid-code copy and no session appears; abandon and assert the credential form returns with the address kept and no request issued.

**Acceptance Scenarios**:

1. **Given** no session and a registration answered with a code step, **When** the answer arrives, **Then** the client holds no access credential, stores nothing, renders the code surface and remembers that registration started the step. (WCS-01, WCS-02)
2. **Given** a pending code step, **When** the correct code is submitted, **Then** the session is established — the access credential held in memory, the refresh and CSRF cookies set by the answer — and the app leaves the code surface. (WCS-03)
3. **Given** a pending code step, **When** the code is refused, **Then** the code surface stays, the mapped copy appears inline, no session is established and a corrected code can be submitted. (WCS-04)
4. **Given** a pending code step, **When** the person abandons the surface, **Then** the credential form returns with the handle and the entered code cleared, with the address and the account mode the step started with, and no request issued. (WCS-06, WCS-21)
5. **Given** no session and a sign-in answered with a code step, **When** the answer arrives, **Then** the client behaves exactly as in scenario 1 with sign-in as the remembered origin. (WCS-01, WCS-02)
6. **Given** the gate is off and registration or sign-in is answered with a session, **When** the answer arrives, **Then** the session is established exactly as today and the code surface is never shown. (WCS-01)
7. **Given** a pending code step, **When** the surface appears, **Then** it names the address the code was sent to, states how long the code remains valid, and the code field holds the focus. (WCS-19, WCS-20, WCS-22)
8. **Given** a pending code step, **When** the code is pasted or filled as a whole, **Then** every digit is presented and the code is sent exactly as typed, leading zeros included. (WCS-24)

---

### User Story 2 - The code never arrived (Priority: P1)

An email is slow, lands in a spam folder, or is never delivered. The person sitting on the code surface has no way forward unless the app offers to send the code again — and it must not let an impatient double activation flood the mailbox or burn the day's send allowance.

**Why this priority**: without a resend path the code step is a dead end for anyone whose mail fails, which is the single most likely way a real person is locked out of the product.

**Independent Test**: on the code surface, assert the resend action is unavailable while the window runs and shows the remaining time, and that it is still reachable with the keyboard; elapse the window, activate once and assert exactly one resend request; activate twice in one activation and assert still one request; assert the recorded resend is confirmed once without invalidating the code already held; answer the resend with the unavailable outcome and assert the mapped copy, the kept handle and the action re-arming at the end of the current window.

**Acceptance Scenarios**:

1. **Given** a code surface whose resend window is running, **When** the person reads the action, **Then** it cannot be used yet, the remaining time is shown as text, and the action is still there to be found and understood. (WCS-07)
2. **Given** the window has elapsed, **When** resend is activated, **Then** exactly one resend request is issued for the handle and the window restarts from the answer. (WCS-08)
3. **Given** the daily allowance is spent, **When** resend is answered, **Then** the mapped unavailable copy is shown, the handle is kept, and the action re-arms at the end of the current window. (WCS-09)
4. **Given** a resend has been answered, **When** the person submits the code they already hold, **Then** the same code still verifies — the resend neither rotated the handle nor invalidated the code. (WCS-10)
5. **Given** a resend has been answered, **When** the person reads the surface, **Then** a confirmation says that a new code was sent and that either code works, and it never says the earlier code is void. (WCS-26)
6. **Given** a resend window is running, **When** the countdown ticks, **Then** it is not announced second by second, and the change that makes the action available is announced once. (WCS-23)

---

### User Story 3 - The failure the person sees names the real cause (Priority: P2)

The gated backend answers with codes the client has never seen — a uniform invalid code, an unavailable send, a duplicate address — and today every one of them would land on the generic "Something went wrong. Try again." The person cannot tell a wrong digit from an exhausted allowance from an address that is already registered.

**Why this priority**: finding F-11. It is a copy and classification change, additive and cheap, but it is what makes the new states usable; without it the code step's refusals are all indistinguishable to the person.

**Independent Test**: map each new code and assert its kind and its resolved copy; feed an unknown code and assert the generic kind; assert no copy is authored outside the translation catalog.

**Acceptance Scenarios**:

1. **Given** the uniform invalid-code outcome, **When** the failure is classified, **Then** it resolves to a dedicated kind with copy that names the code as not valid or expired, names the newest email and the resend as the next step, and offers the way back to the credential form. (WCS-11)
2. **Given** the unavailable-send outcome and the duplicate-address conflict, **When** each is classified, **Then** each resolves to its own kind and copy. (WCS-11)
3. **Given** a code the mapping does not classify, **When** it is classified, **Then** it resolves to the generic kind and its raw code survives. (WCS-12)
4. **Given** a duplicate address at registration, **When** the refusal appears, **Then** it offers an inline action that switches the form to sign-in, keeps the address and moves the focus to the password field. (WCS-27)
5. **Given** a refused code, **When** the copy is shown, **Then** it names the newest email and the resend action as the next step, because a stale message is the most common cause of a refused code. (WCS-11)

---

### Edge Cases

- **Registration answered with a session while the gate is off**: the app must behave exactly as today — one request, session established, no code surface — so the same client serves both deployments. (WCS-01)
- **Registration answered with a code step while sign-in would answer a session**: the discrimination cannot come from the status, because registration answers the same status in both states; a client that branched on the status would either lose the session or lose the handle. (WCS-01)
- **The same person signs in twice**: the second step starts a new handle and the first handle stops being usable; the surface holding the stale handle is answered uniformly and must offer to start over. (WCS-04, WCS-06)
- **The code surface is reloaded**: the pending handle is gone and the credential form returns; the person starts the step again. (WCS-02, WCS-03)
- **A transient failure while submitting the code**: the handle is kept, the copy says the connection or the server failed, and submitting again is possible without starting over. (WCS-04)
- **The refresh cookie from an earlier session is still present while a code step is pending**: the pending state must not be read as a session, and the cold-start refresh must keep its existing behavior. (WCS-01, `WAS-03`)
- **The window elapses while the person is typing**: the resend action enables without discarding the entered code. (WCS-07)
- **Resend while a verify is in flight**: the app must not issue a second send for a step whose code is already being submitted. (WCS-08)
- **The person abandons the surface with a verify in flight**: the abandonment clears the pending state and the in-flight answer must not resurrect a session for an abandoned step. (WCS-06)
- **The send stays unavailable across several windows**: the action re-arms at each window's end and the copy keeps telling the person to try again shortly, without disclosing the account's state. (WCS-09)
- **A duplicate address at registration**: the conflict resolves to its own copy that names the address as already registered and offers to sign in instead. (WCS-11)
- **The code contains leading zeros**: the entered code must reach the server exactly as typed. (WCS-03)
- **The person never verifies**: no session is ever established while the gate is on; the credential form remains the recovery door. (WCS-01)
- **The step expires while the surface is open**: the next submission is refused uniformly and the surface offers the way back to the credential form, and the resend action never promises a send for a step the server would refuse as expired. (WCS-04, WCS-07)
- **The person mistyped the address**: the surface names the address it sent to, so the mistake is visible before the code arrives, and the way back restores that address for correction instead of clearing it. (WCS-19, WCS-21)
- **The step's lifetime elapses while the surface is open**: the surface says the code expired and points at the resend and at the way back, rather than letting the person discover it by being refused. (WCS-20)
- **A resend extends the step's lifetime**: the stated lifetime is re-seeded from the advertised value, and the statement stays truthful about the unit the email used. (WCS-20)
- **The whole code is pasted or filled by the platform at once**: every digit is presented and the code reaches the server exactly as it was entered, leading zeros included. (WCS-24, WCS-03)
- **The challenge has consumed its attempts and the person is out of tries**: the way back is offered at all times, not only after a refusal, so the person is never trapped on a step the server will refuse. (WCS-04)
- **The credential form is remounted after the step is abandoned**: the address and the account mode come back with it, and the password does not. (WCS-21)
- **The resend confirmation is shown while an earlier refusal is still on the surface**: the confirmation replaces the refusal, so the surface never shows two contradictory outcomes at once. (WCS-26)

## Requirements *(mandatory)*

### Functional Requirements

**Adopting the code step (US1)**

- **WCS-01**: WHEN registration or sign-in is answered, the client MUST accept either outcome — a session body or a code step — and MUST discriminate on the payload's own shape rather than on the status, because registration answers the same status whether or not the gate is enabled. WHEN the answer is a session body, the session MUST be established exactly as it is today; WHEN it is a code step, the client MUST establish no session, hold no access credential, store nothing, and enter the pending-code state.
- **WCS-02**: WHILE a code step is pending, the client MUST render the code surface in place of the credential form on the single auth surface, MUST hold the challenge handle in memory only, MUST remember which operation started the step so the surface's copy is truthful, MUST NOT attach a Bearer header to the challenge calls, and MUST NOT trigger a refresh from them. The pending state MUST NOT survive a reload.
- **WCS-03**: The code surface MUST submit the entered code with the handle through the challenge verification operation; WHEN that answer carries a session, the client MUST establish the session exactly as a session answer to registration or sign-in does — the same in-memory access credential, the same session state, the same version boundary — and MUST leave the code surface. The entered code MUST reach the server exactly as typed, leading zeros included.
- **WCS-04**: IF the verification is refused — the uniform invalid-code outcome, a rate limit, a transient failure or an unreadable response — THEN the client MUST keep the pending handle, MUST keep the entered code so a typo can be corrected, MUST show the mapped copy inline on the code surface, MUST establish no session, and MUST allow another submission; the surface MUST also offer a way back to the credential form to start a fresh step, and MUST offer it while the surface is shown rather than only after a refusal, because the step is consumed after a bounded number of attempts and the person must never be trapped on a step the server will refuse.
- **WCS-05**: The entered code MUST NOT be persisted in any local store, MUST NOT appear in a log record or in a request URL, and MUST NOT be disclosed through a page title or an accessible name; the surface renders it as text.
- **WCS-06**: WHEN the code surface is abandoned — a back action, or a navigation away — the client MUST return to the credential form, MUST clear the pending handle and the entered code, and MUST issue no request; a verification answer that arrives for an abandoned step MUST NOT establish a session and MUST NOT overwrite the state the abandonment left. The address and the account mode the step started with MUST survive the abandonment (`WCS-21`).

**The code never arrived (US2)**

- **WCS-07**: The code surface MUST offer a resend action that cannot be used while the resend window is unexhausted and MUST show the remaining time as text; the window MUST be seeded from the challenge answer's advertised resend interval and MUST count down locally. While the action cannot be used it MUST remain reachable by keyboard and by assistive technology, so the person can find it and learn why it cannot be used yet rather than meeting a control that simply vanished from the tab order.
- **WCS-08**: WHEN resend is activated and the window has elapsed, the client MUST issue exactly one resend request for the handle and MUST restart the window from the answer's advertised interval; a repeated activation while the request is in flight or while the window runs MUST NOT issue a second request.
- **WCS-09**: IF resend is answered with the unavailable-send outcome, THEN the client MUST keep the pending handle, MUST show the mapped copy telling the person that delivery is unavailable and to try again shortly, MUST keep the action unusable for the remainder of the current resend window so a refused send is not repeated immediately, and MUST NOT disclose the account's state. The client MUST NOT depend on the server's retry hint being readable, because that header is not a CORS-safelisted response header on the cross-origin API; the surface's window governs the control.
- **WCS-10**: A resend MUST NOT replace or rotate the challenge handle and MUST NOT discard the entered code.

**The failure names the real cause (US3)**

- **WCS-11**: The failure mapping MUST give a dedicated interface-facing kind and copy to the codes the auth surface can now receive — the uniform invalid-code outcome, the unavailable-send outcome and the duplicate-address conflict — while keeping every existing classification unchanged. The invalid-code copy MUST name the newest message and the resend action as the next step, because a stale message is the most common cause of a refused code and the resend is the action that recovers from it.
- **WCS-12**: A code the mapping does not classify MUST keep resolving to the generic kind, and its raw code and status MUST survive on the failure rather than being coerced into a classified one.
- **WCS-13**: Every new copy MUST resolve through the shared translation catalog; the API module MUST keep emitting codes and kinds and MUST NOT author user-facing text. The mapping MUST be total over the codes the auth routes can answer, so a new backend code cannot reach a person as the generic copy without a recorded mapping decision. The catalog MUST resolve copy that carries a value — the address, the remaining time, the account mode — through a parameterized resolution, so no surface concatenates a sentence with a value the catalog did not resolve.

**Cross-cutting**

- **WCS-14**: The code-step answers MUST be pinned as real captures beside the existing fixtures, and the pinned suite MUST fail on a shape change in any of them, so a drift in the code-step contract is caught without a backend running.
- **WCS-15**: The challenge operations MUST go through the raw transport with credentials included, so the session cookies the verification answer sets are accepted on the cross-origin API; they MUST NOT carry a Bearer header, MUST NOT trigger a refresh, and MUST NOT send a CSRF header. The registration and sign-in operations MUST keep their existing credentials posture.
- **WCS-16**: Every user-visible string this feature renders MUST resolve through the shared translation catalog and every visual value from the shared token set; the feature MUST NOT introduce a third-party script, style or font, MUST NOT introduce a DOM-injection sink, MUST NOT add a dependency, and MUST keep all identifiers, comments and strings in English.
- **WCS-17**: Every requirement MUST ship tests — edge cases first, then a representative happy path — with the existing suites that assume "registration and sign-in return a session" moved in the same change; the credential surface, the code surface and the signed-in shell MUST keep zero critical automated accessibility violations and every control MUST expose an accessible role and name.
- **WCS-18**: The requirement corpus MUST record the contract this feature delivers: `WAS-02`'s "registration authenticates" premise no longer describes the gate-enabled behavior and MUST keep resolving to a definition that matches what ships, and the client-side identifiers this feature supersedes MUST be reconciled in the same change.

### The surface's content and behaviour *(US1, US2, US3)*

- **WCS-19**: WHILE a code step is pending, the code surface MUST name the address the code was sent to, and that address MUST be the one the credential operation carried; the surface MUST NOT name any other address, MUST NOT mask it in a way that hides a mistake the person could still correct, and MUST NOT persist it anywhere.
- **WCS-20**: WHILE a code step is pending, the code surface MUST state how long the code remains valid, in the same unit and wording the emailed message uses, because the person reads both and they must not disagree; the statement MUST be re-seeded from the advertised lifetime when a resend extends the step, and the surface MUST offer the resend and the way back once the stated lifetime has passed.
- **WCS-21**: WHEN the code surface is abandoned, the client MUST return to the credential form with the address and the account mode the step started with, MUST clear the password, MUST clear the pending handle and the entered code, and MUST issue no request. The address and the mode MUST NOT be restored as a credential, and MUST NOT survive a reload.
- **WCS-22**: WHEN the code surface appears, the code field MUST take the focus; WHEN a verification is refused, the focus MUST remain on the code field and the refusal MUST be associated with that field so it is announced as the field's problem; WHEN the surface is abandoned, the focus MUST move to the credential form. The code field MUST be described by the instruction line, so the address and the stated lifetime are read with the field.
- **WCS-23**: The resend countdown MUST be rendered as static text outside any region that announces changes on its own, so a ticking second is not announced again and again; the change that makes the resend action available MUST be announced once, and the action's accessible name MUST stay the same while its availability changes.
- **WCS-24**: The code MUST be presented as six single-digit positions over one editable field that holds the whole code, so a pasted code, a platform-filled code and a code with leading zeros land whole and reach the server exactly as entered; the positions MUST NOT be the only way the code is read, the field MUST keep a visible label and an accessible name, and the positions MUST follow the field's value rather than a second copy of it.
- **WCS-25**: The code surface MUST present its blocks in a fixed order — the origin line, the heading, the instruction that names the address and the stated lifetime, the code field, the refusal, the primary action, the resend row and the way back — and the origin line MUST replace the credential form's account-mode switch rather than leave a hole in the panel; the swap between the two surfaces MUST be limited to a fade and a small movement, MUST last no longer than the longest declared motion duration, and MUST be zeroed when the person asks for reduced motion.
- **WCS-26**: WHEN a resend is recorded, the surface MUST confirm it inline, in a status an assistive technology announces once, and that confirmation MUST NOT state that the earlier code is void, because a resend neither rotates the handle nor invalidates the code the person already holds; a confirmation MUST replace an earlier refusal rather than sit beside it.
- **WCS-27**: WHEN the credential form is refused with the duplicate-address conflict, the refusal MUST offer an inline action that switches the form to sign-in, keeps the address the person typed and moves the focus to the password field, so the offered recovery can actually be taken.

### Key Entities

- **Code step (pending)**: the in-memory state between a credential answer and a verified session — the challenge handle, the origin operation, the address the code was sent to, the account mode, the advertised lifetime and the resend window. It holds no credential, is never persisted, and does not survive a reload.
- **Challenge handle**: the server-issued identity of a started code step, presented back on verification and on resend; it is the only thing the client sends besides the code, and it is not a credential.
- **Code surface**: the surface the auth gate renders while a step is pending — it names the address and the stated lifetime, collects the six-digit code over six single-digit positions, offers resend with its countdown and its confirmation, and returns to the credential form when abandoned. It is a state of the single auth surface, not a declared route.
- **Sign-in outcome**: the discriminated answer of registration and sign-in — either a session or a code step — which the client must tell apart without knowing the deployment's gate.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With the code step in place, 0 challenge answers establish a session, hold an access credential or write a credential to a local store; 100% of them render the code surface.
- **SC-002**: 0 challenge answers are misread as sessions and 0 session answers are misread as code steps, on both gate states, for registration and for sign-in.
- **SC-003**: A person can complete a code step in 1 verification request after the correct code, and 0 additional requests are issued by abandonment or by a repeated resend activation.
- **SC-004**: 100% of the new codes resolve to a dedicated kind and copy, and 0 unknown codes are coerced into a classified kind.
- **SC-005**: 100% of the moved suites assert the new states — edge cases first, then a happy path — and the suite runs with 0 sleeps.
- **SC-006**: The credential surface, the code surface and the signed-in shell report 0 critical automated accessibility violations, and the scans that enforce the copy and token rules — `l10n_strings.test.ts` and `tokens_used.test.tsx` — report 0 user-facing literals and 0 non-token visual values.
- **SC-007**: 0 credentials — access, refresh, CSRF or code — appear in a local store, a log record or a URL, and 0 pending states survive a reload.
- **SC-008**: 100% of the code surface's renders name the address the code was sent to and state the code's remaining validity; 0 renders show the code field without either.
- **SC-009**: 100% of abandonments return the credential form with the address and the account mode preserved; 0 abandonments make the person retype the address, and 0 restore the password.
- **SC-010**: 0 focus losses across the credential form, the code surface and a refusal — the focus is always inside the surface that is showing, and a refusal leaves it on the code field.
- **SC-011**: 0 resend confirmations state that the earlier code is void, and 0 surfaces show a resend confirmation beside an earlier refusal.
- **SC-012**: 0 countdowns are announced more than once per resend window, and 0 refusals are announced away from the field they concern.

## Assumptions

- This is the client half of `docs/plan-auth-otp-2fa-email.md` §9 stage 5, and it consumes the contracts the preceding features delivered: the unified challenge endpoints (`.specs/features/017-otp-challenges/contracts/endpoints.contract.md`), the gated credential answers and the identity read (`.specs/features/018-auth-2fa-session-gate/contracts/`). No client code changed in those features, so this feature is where `WAS-02`'s premise is finally delivered for the client.
- The identity read (`GET /auth/me`) stays delivered on the server and unused on the client in this feature (clarification, 2026-10-20): no screen renders verification-dependent content yet, so its consumer is the stage whose screens first need it; the endpoint is not removed and the server contract is unchanged.
- The gate is a server-side setting the client cannot observe, so both answers must be handled by the same code path in the same build; the client never learns, needs to learn, or sends the challenge purpose. The purpose stays server-derived (`TFA-03`).
- The app is served at `app.legnaris.com.br` and the API at `api.legnaris.com.br` (cross-origin, same site), so the challenge calls must include credentials for the verification answer's `Set-Cookie` to be honored; the refresh cookie remains `HttpOnly` and the access credential remains memory-only.
- The existing F2 client seams are reused unchanged: the injected transport, the envelope decoder, the failure taxonomy, the classification mapper and the translation catalog. The challenge handle and the entered code live in the session's in-memory state; no new dependency is expected.
- Registration's duplicate-address conflict and the daily send cap are the two new failure codes the person can act on; both already exist on the server and need only a kind and a copy on the client. The uniform invalid-code outcome is the third, already produced by the challenge domain (`OTP-08`).
- The client does not read the server's `Retry-After` hint: on the cross-origin API that header is not CORS-safelisted, so a browser `fetch` cannot expose it without a server-side change this feature does not make. The resend action's unavailable window is therefore the local window seeded from the challenge answer's advertised `resend_after`, and the unavailable-send copy tells the person to try again shortly rather than computing an instant from the header.
- The offline read path and the service-worker cache belong to the PWA features; this feature adds no offline behavior beyond the existing transient-failure handling.
- The existing generic session copy at the credential form is unchanged: a wrong password keeps resolving to it rather than naming a sign-in refusal, because this feature adds kinds only for the codes it introduces. That mismatch is a recorded residual, the same way the preceding feature recorded the registration half of finding F-05; a later stage may give the credential-surface refusal its own copy (`WCS-11`, clarification 2026-10-20).
- The client suite, the token and copy scans and the requirement-ID lint already run in continuous integration, so this feature introduces no new gate.
- The emailed message is the code screen's sibling surface: it already states the code's lifetime in minutes ("The code expires in N minutes"), so the screen speaks the same unit and the two never disagree (`WCS-20`). The screen does not restate the email's other sentences, and the email is not changed by this feature.
- The address is an identifier the person typed, not a credential: it lives in the pending state for the life of the tab, is never persisted, never leaves the screen and never reaches a request body (`WCS-19`, `WCS-21`).
- The server extends the challenge's deadline at each resend, bounded by an absolute ceiling the client cannot observe; the client's stated lifetime is re-seeded from the advertised value and may be slightly optimistic near that ceiling, so the refusal stays the authority (`WCS-20`).
- The six digit positions are presentational: one editable field holds the whole code, so the platform's one-time-code autofill, a paste and a leading zero behave exactly as a single field would (`WCS-24`).
- The visual contract for the code field and the surface's composition belongs to `web/DESIGN.md`; this feature records the behaviour the surface must satisfy, and the visual contract is amended in the same change (`WCS-24`, `WCS-25`).
