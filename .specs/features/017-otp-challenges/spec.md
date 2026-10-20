# Feature Specification: Email Code Challenges — Six-digit Codes, Atomic One-shot Consumption and the Unified Verify/Resend Endpoints

**Feature Branch**: `017-otp-challenges`

**Created**: 2026-10-20

**Status**: Draft

**Input**: User description: "Deliver the email code challenge domain: durable challenge records with atomic one-shot consumption, cryptographically generated six-digit codes stored as a keyed hash, the resend/cooldown/expiry-ceiling rules, the per-key limits and the daily send cap, and the unified verify and resend HTTP endpoints" (plan `docs/plan-auth-otp-2fa-email.md` §9 stage 3, decisions D1, D3, D4, D6, O3, O4; findings F-05, F-07, F-08, F-13, F-15)

## Clarifications

### Session 2026-10-20

- Q: How must the secret that keys the code digest (`EMAIL_CODE_SECRET`) be configured, now that the API process itself digests codes? → A: Required with a fail-fast boot (user decision, 2026-10-20): an absent, blank, or too-short value is a typed boot error, the variable joins the deployment's required-variable list, and the deployment description and its validation follow. This is a deliberate exception to the optional-configuration norm of the preceding features, because a weak or session-derived key (finding F-09) would silently weaken every code (`OTP-18`, `OTP-21`, `SC-008`).
- Q: What concrete HTTP outcome must the uniform verification failure (OTP-08) map to, since it defines the contract the client maps in the next stage? → A: A new typed code `OTP_INVALID` mapped to 401 (user decision, 2026-10-20): one code, one message and one status for every verification failure, kept distinct from the rate-limit 429 and the unavailable 503, and consistent with the 401 the primary credential already uses (`OTP-08`, `SC-002`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A code works once, expires, and cannot be guessed (Priority: P1)

Someone who has just proved something else — a password at sign-in, or a fresh registration — receives a six-digit code by email and types it back. That code is a short-lived credential: it must stop working the moment it is used, stop working once it is old, and stop working after a small, fixed number of wrong guesses. Two requests that arrive at the same instant with the same correct code must not both succeed, and two requests that race must not let the loser consume the winner's code.

**Why this priority**: this is the whole security value of the feature. Six digits is roughly twenty bits, so nothing else about the feature matters if a code can be replayed, kept alive indefinitely, or brute-forced by repetition. Every later feature builds its session on this step.

**Independent Test**: issue a challenge, verify it with the correct code and assert a session is produced; verify the same challenge again and assert the uniform failure with no session; drive a wrong code repeatedly until the attempt budget is exhausted and assert the challenge is no longer usable and no further attempt is counted; issue a challenge and verify it past its expiry with the injected clock and assert the uniform failure; drive two concurrent verifications with the correct code and assert exactly one succeeds.

**Acceptance Scenarios**:

1. **Given** a freshly issued challenge, **When** it is verified with the correct code, **Then** it is consumed and a session is established exactly as the current sign-in path does. (OTP-07)
2. **Given** a challenge already consumed, **When** the same correct code is presented again, **Then** the request fails with the uniform verification outcome and issues no session. (OTP-04, OTP-08)
3. **Given** a challenge past its expiry instant, **When** the correct code is presented, **Then** the request fails with the uniform verification outcome. (OTP-06, OTP-08)
4. **Given** a challenge with a wrong code presented, **When** the request fails, **Then** the wrong guess is counted durably against that challenge, not against process memory. (OTP-05)
5. **Given** a challenge whose attempt budget is exhausted, **When** any further code is presented, **Then** the challenge is unusable and the request fails with the uniform verification outcome. (OTP-05, OTP-08)
6. **Given** two concurrent verifications of one challenge with the correct code, **When** both are processed, **Then** exactly one is consumed successfully and the other fails. (OTP-04)

---

### User Story 2 - Asking for the code again is safe and predictable (Priority: P1)

An email is slow, or the person mistyped. Asking again must not be a way to multiply email traffic, and it must not invalidate the code that is already in the inbox — a first email arriving after a second request would otherwise show a code that no longer works. Asking again must behave the same whether it actually sent a new email or the sender was still inside its cool-down, and it must never reveal whether the challenge existed at all.

**Why this priority**: resend is the only recovery path a user has while they are locked out of their account, and it is simultaneously the cheapest abuse vector the feature opens.

**Independent Test**: issue a challenge, resend it and assert the same code and the same challenge remain valid; resend again inside the cool-down and assert no additional send intention is recorded while the response is unchanged; resend repeatedly and assert the challenge's expiry never moves past its absolute ceiling; resend an unknown, a consumed and an expired challenge identity and assert each answer is indistinguishable from a successful resend.

**Acceptance Scenarios**:

1. **Given** an active challenge, **When** a resend is requested, **Then** the code is not rotated, the challenge identity is unchanged, and a new send intention is recorded for the same code. (OTP-10)
2. **Given** a resend inside the cool-down window, **When** it is requested, **Then** no additional send intention is recorded and the response is the same as an accepted resend. (OTP-10)
3. **Given** repeated resends over a long period, **When** each extends the deadline, **Then** the challenge's expiry never moves past its absolute ceiling. (OTP-10)
4. **Given** an unknown, an already-consumed, or an expired challenge identity, **When** a resend is requested, **Then** the answer is indistinguishable in status and body from a successful resend. (OTP-11)
5. **Given** a resend whose code is inside the cool-down, **When** it is answered, **Then** the response still carries the lifetime information a client needs to render the retry countdown. (OTP-10)

---

### User Story 3 - The code step reveals nothing and leaks nothing (Priority: P2)

The code step sits in front of account existence. Its responses, its timing, and its logs must not tell an observer whether an account exists, whether a challenge was ever issued, or what any code is. The code is a credential: it may travel in the email that delivers it and nowhere else — not in a response body, not in a log line, not in an error message, not in a subject line.

**Why this priority**: a leak here does not break one account, it breaks the anonymity of the whole surface — and the code is the second factor protecting every account.

**Independent Test**: assert every verification failure path produces one status and one body regardless of which failure occurred (unknown, expired, consumed, wrong, exhausted) and that no path re-emits the submitted code; scan the suite's responses, logs and error strings for a code pattern and assert none is present; assert logs identify a challenge by identity and purpose and mask the address.

**Acceptance Scenarios**:

1. **Given** an unknown challenge identity, an expired one, a consumed one, a wrong code, and an exhausted budget, **When** each is verified, **Then** all five answers carry the same status and the same body. (OTP-08)
2. **Given** any response, log line, or error message produced by the feature, **When** it is inspected, **Then** it contains no code. (OTP-15)
3. **Given** a challenge event is logged, **When** the line is read, **Then** it identifies the challenge by identity and purpose and the address is masked. (OTP-15)
4. **Given** a verification that succeeds for an account with no verification timestamp, **When** the session is established, **Then** the timestamp is recorded as part of the same successful verification. (OTP-07)
5. **Given** a verification that succeeds for an account already verified, **When** the session is established, **Then** the existing timestamp is preserved unchanged. (OTP-07)

---

### User Story 4 - The code path cannot be turned into an email cannon, and it can be watched (Priority: P2)

Sending a code costs money, consumes a shared daily allowance, and burns the reputation of the sending domain. Every key that can trigger a send must be bounded — one challenge, one account, one mailbox, one address — and the whole system must stop before it exhausts the provider's allowance, answering honestly that the code temporarily cannot be sent rather than accepting a request whose email will never leave. Operators must be able to see issuance, verification outcomes and expiration without ever seeing a code.

**Why this priority**: the free allowance is roughly one hundred sends a day; a single abuser, or one bug in a retry loop, drains it for everyone and takes sign-in down with it.

**Independent Test**: drive each limit key past its budget and assert the typed refusal without a send; saturate the daily cap and assert no send intention is recorded and the refusal is the unavailable outcome with a retry hint rather than the uniform accepted one; assert the reference counters advance for issuance, verification results and expirations.

**Acceptance Scenarios**:

1. **Given** one address that exceeds its request budget, **When** it requests again, **Then** it is refused with the typed rate-limit outcome and no send is recorded. (OTP-14)
2. **Given** one mailbox that exceeds its request budget from several addresses, **When** it requests again, **Then** it is refused with the typed rate-limit outcome and no send is recorded. (OTP-14)
3. **Given** a send budget already reached for the day, **When** a code is requested, **Then** no send intention is recorded and the answer is the typed unavailable outcome with a retry hint. (OTP-13)
4. **Given** the feature is running, **When** codes are issued, verified and left to expire, **Then** each is observable as a counter without any code being observable. (OTP-16)

---

### Edge Cases

- **Two verifications of one challenge in the same instant**: the single-document consumption gives the challenge to exactly one and the loser fails uniformly. (OTP-04)
- **Verification exactly at the expiry instant**: the challenge is expired; one nanosecond before, it is still valid. (OTP-06)
- **Wrong guess and a correct guess racing**: the wrong guess may consume an attempt, but it can never consume the challenge successfully. (OTP-04, OTP-05)
- **Attempt budget reached by a wrong guess**: the challenge becomes unusable before the code is even compared on any later request. (OTP-05)
- **Code with a leading zero**: preserved and compared as a six-character string. (OTP-02)
- **Code value colliding with the stored hash of a different account or purpose**: the keyed hash binds the account and the purpose, so a code valid for one cannot be verified for another. (OTP-03)
- **Hash secret unset, blank, or too short**: the process fails fast at boot with a typed error rather than hashing with a weak or shared key. (OTP-18, OTP-21)
- **Challenge that does not exist**: identical answer to one that is expired, consumed, or answered wrongly. (OTP-08)
- **Resend on a challenge past its absolute ceiling**: answered like any other resend, and the deadline does not move. (OTP-10, OTP-11)
- **Resend immediately after the challenge was consumed**: answered like any other resend; the consumed challenge is not resurrected. (OTP-11)
- **Daily cap reached while a legitimate user retries**: the unavailable outcome carries a retry hint and reveals no account state. (OTP-13)
- **Issuing a challenge when one is already active for the same account and purpose**: the previous challenge becomes unusable and only the new one can be verified. (OTP-01)
- **Code appearing in a candidate response, log line, or message subject**: prohibited by contract; the suite asserts none is present. (OTP-15)
- **Store failure during issuance, consumption, or a wrong-guess count**: reported as an internal failure with no session, and never as an invalid code. (OTP-17)
- **Clock boundary on the cool-down**: a resend exactly one nanosecond before the cool-down ends is still suppressed; at the boundary it is allowed. (OTP-10, OTP-20)
- **Challenge record past its lifetime**: removable by the store's own expiry mechanism, but expiry must never be relied upon for the decision. (OTP-06)

## Requirements *(mandatory)*

### Functional Requirements

**The challenge record and the code (US1)**

- **OTP-01**: A challenge MUST be persisted with its own canonical identity, the account identity, a server-side purpose (`verify_email` for a freshly registered account, `login_otp` for a sign-in), the code's keyed hash, an expiry instant, a consumed marker, an attempt count, a creation instant and a last-sent instant. At most one challenge MUST be active per account and purpose: issuing a new one for a pair that already has an active challenge MUST make the previous one unusable.
- **OTP-02**: The code MUST be six decimal digits generated from a cryptographically secure source, preserved with its leading zeros, and compared as a six-character value.
- **OTP-03**: The code MUST be stored only as a keyed digest that binds the account identity and the purpose, keyed by a secret distinct from the session-signing secret, so a code cannot be verified against a different account or purpose, and comparison MUST be constant-time.
- **OTP-04**: Consuming a challenge MUST be a single-document atomic operation so that at most one request can succeed per challenge even under concurrency; a challenge that is consumed, expired, or past its attempt budget MUST NOT be consumable.
- **OTP-05**: A wrong code MUST durably increment the challenge's attempt count; at the documented cap (five) the challenge MUST become unusable and MUST require a new challenge. The count MUST live with the challenge, not in process memory, so a restart or a second instance cannot reset it.
- **OTP-06**: Expiry MUST be decided by comparing the stored expiry instant with the injected clock on every read; automatic removal of old records MUST never be relied upon to refuse a code.
- **OTP-07**: WHEN a verification succeeds, the challenge MUST be consumed, the account's email-verification timestamp MUST be recorded when it was absent, and an existing timestamp MUST be preserved unchanged; the response MUST establish a session exactly as the current sign-in path does, whichever purpose the challenge carried.

**The verify endpoint (US1, US3)**

- **OTP-08**: Any verification failure — an unknown challenge identity, an expired one, a consumed one, a wrong code, or an exhausted attempt budget — MUST answer with one uniform typed outcome, one status and one body, so no case is distinguishable from another: the outcome is the new code `OTP_INVALID`, mapped to 401, with one message and no session. An exhausted budget MUST NOT be reported differently from any other failure.
- **OTP-09**: The challenge purpose MUST be resolved from the stored record and MUST NOT be accepted from the client; the client MUST supply only the challenge identity and the submitted code.

**The resend endpoint (US2)**

- **OTP-10**: A resend MUST be idempotent for the client: it MUST answer the accepted outcome whether or not it recorded a new send, it MUST NOT rotate the code or change the challenge identity, it MUST record a new send intention for the same code only when the cool-down window has elapsed, and it MUST extend the deadline by at most the documented absolute ceiling from the challenge's creation.
- **OTP-11**: A resend for an unknown, consumed, or expired challenge identity MUST answer indistinguishably from an accepted resend and MUST NOT resurrect or create anything.

**The send cap (US4)**

- **OTP-12**: Issuing a challenge and issuing a resend MUST both record the durable send intention delivered by the email infrastructure before the request is reported accepted, and the request-serving process MUST NOT contact the email provider or the message broker.
- **OTP-13**: A configurable daily cap on recorded send intentions MUST be enforced before a new intention is written; WHEN the cap is reached, no intention MUST be written and the request MUST answer a typed unavailable outcome with a retry hint, distinct from the accepted outcome and revealing no account state.

**Limits and disclosure (US3, US4)**

- **OTP-14**: The new routes MUST be bounded per client address, per mailbox, per account and per challenge, using the resolved client address and the canonical mailbox already required by the auth prerequisites; a refusal MUST be the existing typed rate-limit outcome with a retry hint, recorded in the existing auth metric, and MUST NOT record a send.
- **OTP-15**: The code MUST never appear in a response body, a log line, or an error message, and MUST travel only inside the recorded send intention; logs MUST identify a challenge by identity and purpose and MUST mask the recipient address.
- **OTP-16**: Issuance, verification outcomes by result, expiration and the send-cap state MUST be observable as counters, with no series carrying a code or an unmasked address.

**Cross-cutting**

- **OTP-17**: Every new failure mode — an unknown/challenge not usable, an invalid code, an exhausted budget, a send-cap refusal, a rate-limit refusal, and any store failure — MUST be a typed error with an exported message constant and a documented HTTP status, and the boundary MUST map it exactly. A store failure MUST never be reported as an invalid code.
- **OTP-18**: Configuration for this feature MUST come from the shared configuration surface and MUST be documented. The keyed-digest secret MUST be required: an absent, blank, or too-short value MUST fail fast at boot with a typed error, the secret MUST be added to the deployment's required-variable list, and the deployment description and its validation MUST follow it. Every other setting this feature adds (attempt cap, cool-down, code lifetime, absolute ceiling, daily cap, limits) MUST be optional with a documented default.
- **OTP-19**: Every new route MUST reuse the existing per-address and per-account rate limiting and, on a route that establishes a session, the existing allowed-origin guard; the route contract added here MUST be the unified surface (`verify`, `resend`) and MUST NOT expose the challenge purpose.
- **OTP-20**: Every requirement MUST ship tests — edge cases first, then one happy path — using the injected clock for expiry and cool-down boundaries with no sleeps, asserting the typed error and mapped status on every failure path, and staying clean under the race detector.
- **OTP-21**: All identifiers, comments, log messages and user-facing strings MUST be written in English; the keyed-digest secret MUST be a separate secret from the session-signing secret and MUST NOT reuse it.

### Key Entities

- **Challenge**: the durable record of one pending code step — identity, account, purpose, code digest, expiry, consumed marker, attempt count, creation and last-sent instants. At most one active record per account and purpose.
- **Code**: the six-digit value generated for a challenge — carried in the send intention that delivers it, stored in the challenge only as a keyed digest, and never exposed anywhere else.
- **Verification outcome**: the typed result of a verification attempt — success, or one undifferentiated failure; never a discriminator between unknown, expired, consumed, wrong and exhausted.
- **Resend request**: a client's repeat request for the same challenge's code — accepted whether it sent again or was still cooling down, bounded by a cool-down window and an absolute ceiling.
- **Send intention**: the durable record (delivered by the email infrastructure of the preceding feature) that the code must be emailed; issuance and resend both write one, and the daily cap counts them.
- **Send cap**: the configurable daily ceiling on recorded send intentions, below the provider's own allowance, whose exhaustion yields the unavailable outcome.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 0 challenges are consumable twice, including under concurrent verification; 100% of successful verifications are single.
- **SC-002**: 100% of verification-failure answers are byte-identical across the unknown, expired, consumed, wrong-code and exhausted-budget cases, in status and body.
- **SC-003**: 0 codes appear in any response body, log line, error message or message subject across the suite.
- **SC-004**: 0 send intentions are recorded once the daily cap is reached, and 100% of refusals at the cap answer the unavailable outcome with a retry hint.
- **SC-005**: 0 resends rotate a code, and 0 resends inside the cool-down window record a send intention.
- **SC-006**: 100% of wrong guesses are counted durably, and 100% of exhausted challenges refuse every subsequent attempt.
- **SC-007**: 100% of new failure paths assert their typed code and mapped status; expiry and cool-down tests run with 0 sleeps and the suite is race-clean.
- **SC-008**: The keyed-digest secret is on the deployment's required-variable list, starting without a valid one fails fast with a typed boot error, and 0 code-step paths are served with an unset or too-short key.

## Assumptions

- This is the third of the features derived from `docs/plan-auth-otp-2fa-email.md` §9; it delivers stage 3 (the challenge domain, its endpoints and their error codes) only. Wiring registration and sign-in to issue challenges and stop establishing sessions immediately — together with renewing only verified accounts and exposing the verification state — is the next feature (stage 4). Nothing public creates a challenge yet: the endpoints exist against an internal issuance operation that an integration test drives, exactly as the preceding features shipped seams ahead of their callers.
- The unified surface of decision O3 is delivered here: one verification endpoint and one resend endpoint whose purpose is resolved server-side. The response payload field names, the schema, and every status other than the verification failure are fixed during design, not here; the failure's code and status are fixed by `OTP-08`.
- The challenge record's hash gives nothing away, but the code itself must exist in the clear inside the deliverable send intention, because the message contract of the preceding feature carries it to the worker that renders the email; that is the only place it is stored in the clear, it is subject to that feature's retention window, and it is never logged, echoed or placed in a subject.
- The request-serving process writes the send intention directly into the store delivered by the preceding feature and does not import the provider adapter or the broker client; a delivery outage is absorbed by that feature's outbox and relay, so an unavailable provider never fails a challenge silently — the user still has resend.
- The defaults are the plan's suggestions and are configurable: a five-attempt cap, a cool-down on the order of a minute, a code lifetime on the order of ten minutes with an absolute ceiling on the order of thirty, and a daily cap on the order of eighty, deliberately below the provider's free allowance of one hundred.
- Codes, purpose names and the challenge record are English-neutral; any user-facing copy this feature adds is written in English (Principle IV).
- The store is a standalone MongoDB: consumption is atomic per document and no multi-document transaction is available, so the ordering of account, challenge and intention writes across the sign-up and sign-in flows is the next feature's concern; this feature guarantees only that its own consumption decision is atomic.
- Identifiers are canonical lowercase UUID v7 from the shared generator; inbound challenge identities are validated at the boundary.
- No third-party dependency is added: code generation, keyed digesting and constant-time comparison use the standard library, and the existing email infrastructure, typed-error taxonomy and rate limiter are reused.
- The per-mailbox and per-address limits depend on the client-address resolution and the canonical mailbox delivered by the auth-prerequisites feature.
- Requirement-ID prefix `OTP` is exclusive to this feature; identifiers `D1`, `D3`, `D4`, `D6`, `O3`, `O4`, `F-05`, `F-07`, `F-08`, `F-13`, `F-15` belong to the plan and are not requirement IDs.
- The keyed-digest secret is the first variable added to the deployment's required set: the email-infrastructure feature kept its settings optional because the API never needed them, whereas here the API itself digests codes. The deliberate exception and its rationale are recorded in the Clarifications session.
