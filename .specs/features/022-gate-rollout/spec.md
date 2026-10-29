# Feature Specification: The Rollout — Turning the Rule On in Two Steps, and Knowing the Way Back

**Feature Branch**: `022-gate-rollout`

**Created**: 2026-10-21

**Status**: Draft

**Input**: User description: "Activate the sign-in hardening in stages: a mode that turns the email-verification rule on for new accounts first and for sign-in later, the renewal containment of the sessions issued before the cutover, and the operator's order, observation windows and way back" (plan `docs/plan-auth-otp-2fa-email.md` §9 stage 7; decisions D1, D2, D4, O5; the gate the preceding feature shipped dark)

## Clarifications

### Session 2026-10-21

- Q: How is the staged activation expressed — a mode with three states, or two independent switches? → A: One mode with three states — off, registration only, and everything (user decision, 2026-10-21): the plan's order is a sequence, not two independent facts, and one setting cannot be half on. The setting replaces the delivered boolean, which is safe because the rule has never been turned on in any deployment (`ROL-01`, `ROL-02`).
- Q: The plan offers an optional cleanup of the refresh families issued before the cutover. Is it delivered? → A: No (user decision, 2026-10-21): the renewal refusal already revokes each family the first time it is presented, so an active account's session dies at its first renewal without any operator action; the rows that remain are marked revoked and expire on their own, and a cleanup command would only touch the accounts that never come back. The plan's "optionally" permits not doing it, and the runbook records why (`ROL-11`, `ROL-12`, Assumptions).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - The rule can be turned on in two steps (Priority: P1)

The rule that a code by email is needed to sign in is a breaking change: from the moment it is on,
nobody signs in with a password alone. Turning it on for everyone at once would lock out every
existing account on the same night, with no way to see whether the machinery works before it matters.
So the rule needs to be switchable in stages: first only a brand-new account has to prove its mailbox,
so the flow is exercised on the people who can still be helped; later, when the machinery has proved
itself, everybody has to prove theirs. The operator chooses the step; people experience nothing
unexplained.

**Why this priority**: this is the plan's stage 7, and it is the step that makes the whole program
safe to turn on. Without it, the only available move is the hard one.

**Independent Test**: with the setting in each of its three states, register and sign in and assert
what each answers: off — both establish a session as before; registration only — a registration asks
for a code and a sign-in still establishes a session; everything — both ask for a code, and a renewal
of an account that never verified is refused. Assert the client's contract does not change between the
states: the same two answers, and the client learns nothing new.

**Acceptance Scenarios**:

1. **Given** the rule is off, **When** someone registers or signs in, **Then** both behave exactly as they did before the rule existed: a session, no code. (ROL-01)
2. **Given** the rule covers registration only, **When** someone registers, **Then** the account is created unverified, a code step is started and no session is issued; **When** the same person signs in, **Then** the password establishes a session as before. (ROL-02)
3. **Given** the rule covers everything, **When** someone registers or signs in, **Then** both start a code step and issue no session, and a renewal of an account that never verified is refused. (ROL-03)
4. **Given** any state of the setting, **When** the answers are compared, **Then** they are the same two answers the client already knows — a session or a code step — and the client is told nothing about the state. (ROL-04)
5. **Given** an account that registered and verified while the rule covered registration only, **When** the rule later covers everything, **Then** that account signs in with a code like everybody else — the code is the second factor of every sign-in — and its renewal is not refused. (ROL-05)

---

### User Story 2 - The cutover contains the sessions issued before it (Priority: P1)

The moment the rule covers sign-in, the sessions that were issued before it belong to accounts that
never proved their mailbox. They must not keep renewing — that is the whole point of the rule — but
the person holding one must not be stranded either: they must land on the code step and be let in
once they prove their mailbox. And the operator must not have to perform a ritual over the store to
make that happen.

**Why this priority**: without the containment, the rule is only true for people who happen to sign in
again; the sessions issued before the cutover would keep renewing for weeks, and "only verified
accounts may hold a session" would be a claim, not a fact.

**Independent Test**: with the rule covering everything, present a refresh credential of an account
that never verified and assert it is refused and its whole session family is revoked; present the
credential of an account that did verify and assert it renews; assert no operator command exists to
clean the store and that the runbook explains why none is needed.

**Acceptance Scenarios**:

1. **Given** the rule covers everything and an account that never verified holds a session, **When** the session tries to renew, **Then** the renewal is refused with the uniform invalid-credential answer and the session's whole family is revoked, so the session ends on every device at once. (ROL-09)
2. **Given** the rule covers everything and an account that verified holds a session, **When** the session renews, **Then** it renews exactly as before. (ROL-09)
3. **Given** the rule covers everything, **When** the sessions issued before the cutover are considered, **Then** no operator action is needed to end them, and the runbook says why. (ROL-11, ROL-12)
4. **Given** the rule covers everything, **When** a person whose session was just refused signs in again, **Then** the code step asks for the code and the person is let in once the code verifies. (ROL-10)

---

### User Story 3 - The operator knows the order, what to watch and the way back (Priority: P2)

Whoever turns this on is doing it at night, on a service people use to work. They need to know what
to do first, how long to wait before the next step, which numbers tell them the machinery is coping,
what to do if it is not, and how to undo it if it must be undone — and they need it written down, not
remembered.

**Why this priority**: the change is a configuration act and a restart; the runbook is what makes it
an informed one.

**Independent Test**: read the runbook and assert it states the order of the two steps, the numbers to
watch between them and the window to wait, what to do when a number looks wrong, how to go back, and
what happens to the accounts and to the pending code steps at each flip.

**Acceptance Scenarios**:

1. **Given** the runbook, **When** it is read, **Then** it states the order of the two steps, the counters to watch between them, the window to wait before the next step, and the way back. (ROL-14)
2. **Given** the runbook, **When** the consequences of a flip are read, **Then** it states what happens to the accounts that already verified, to the accounts that never did, and to a code step that was in flight when the flip happened. (ROL-15)
3. **Given** the way back, **When** the rule is turned off again, **Then** the previous contract returns exactly, and nothing that was verified is undone. (ROL-16)

---

### Edge Cases

- **The setting is absent**: the rule is off, and the contract is exactly what it was before the
  program. (ROL-01)
- **The setting carries a value nobody recognises**: the process refuses to start rather than
  guessing a stage. (ROL-06)
- **The setting is changed while the process runs**: the change takes effect on the next start, not
  halfway through a request; the runbook says so. (ROL-07)
- **An account registered under the first step verifies its mailbox**: it keeps signing in with a
  session and is never asked again. (ROL-05)
- **An account that never verified tries to renew exactly at the flip**: the renewal is refused and
  the family is revoked; the person signs in again and completes the code step. (ROL-09)
- **A session issued before the flip is still within its access-token life**: it keeps working until
  it expires; the cutover is exact at the renewal, not at the token. The runbook says so. (ROL-13)
- **A code step was in flight when the setting changed**: the step continues and still completes; the
  setting decides what a *new* request answers. (ROL-07)
- **The rule is turned off after it was on**: the previous contract returns; the accounts that
  verified stay verified, and nothing is undone. (ROL-16)
- **A person is signed out by the cutover**: they sign in again, receive a code, and are let in — the
  refusal looks like any other expired session. (ROL-10)
- **The operator wants to clean the store before the cutover**: there is nothing to clean; the
  revocation happens by itself. (ROL-11)
- **The counters the runbook names are not being scraped yet**: the runbook says which are available
  today and that the alerts arrive with the next feature. (ROL-14)

## Requirements *(mandatory)*

### Functional Requirements

**The staged setting (US1)**

- **ROL-01**: The sign-in hardening MUST be governed by one setting with exactly three states — off, registration only, and everything — and the absent or unrecognised state MUST be handled explicitly rather than guessed.
- **ROL-02**: In the registration-only state, a registration MUST start a code step and issue no session, and a sign-in MUST establish a session exactly as it did before the rule existed.
- **ROL-03**: In the everything state, a registration and a sign-in MUST both start a code step and issue no session, and a renewal of an account that never verified MUST be refused.
- **ROL-04**: The setting MUST NOT change the two answers a client receives — a session or a code step — and the client MUST NOT be able to observe the state.
- **ROL-05**: An account that verified its mailbox MUST NOT be treated as unverified in any state: its renewal MUST NOT be refused, and a sign-in in the last state MUST still ask for a code, because the code is the sign-in's second factor and not a repeated proof of the address.
- **ROL-06**: A value the setting does not name MUST stop the process at start-up with a typed error, so no deployment ever runs on a guessed stage.
- **ROL-07**: The state MUST be read once at start-up and MUST NOT change while the process runs; a code step already in flight when the setting changes MUST continue and complete.
- **ROL-08**: The state the process started with MUST be recorded in its log at start-up, so an operator can tell which stage a running deployment is in without reading its configuration.

**The cutover's containment (US2)**

- **ROL-09**: WHEN the everything state is in force, a renewal of an account that never verified MUST be refused with the uniform invalid-credential answer and the presented credential's whole session family MUST be revoked.
- **ROL-10**: A person whose renewal was refused MUST be able to sign in again and be let in by completing the code step, with no path that leaves them unable to sign in.
- **ROL-11**: No operator action MUST be required to end the sessions issued before the cutover, and no command to clean the store MUST be delivered.
- **ROL-12**: The renewal's refusal MUST be the only mechanism the cutover needs, and the runbook MUST state why no cleanup of the stored sessions is required.
- **ROL-13**: The runbook MUST state that a session issued before the cutover remains usable until its access credential expires, and that the cutover is exact at the renewal.

**The operator's guide (US3)**

- **ROL-14**: The runbook MUST state the order of the two steps, the numbers to watch between them, the window to wait before the next step, and the way back.
- **ROL-15**: The runbook MUST state what happens at each flip to the accounts that already verified, to the accounts that never did, and to a code step in flight.
- **ROL-16**: Turning the setting back to off MUST restore the previous contract exactly, and MUST NOT undo a verification that already happened.

**Cross-cutting**

- **ROL-17**: The setting MUST come from the shared configuration surface, and the deployment's required-variable list MUST NOT change: the rule ships off and its value is an operator's choice.
- **ROL-18**: The delivered feature that introduced the setting MUST be reconciled with this one, so the corpus carries one contract for the setting and not two.
- **ROL-19**: Every new failure mode MUST be a typed error with an exported message and a documented effect, and the process MUST act on it exactly.
- **ROL-20**: Every requirement MUST ship tests: edge cases first, then one happy path, asserting the typed error on failure paths and staying clean under the race detector.
- **ROL-21**: All identifiers, comments, log messages and user-facing strings MUST be written in English.
- **ROL-22**: The web client MUST NOT change: the two answers it already handles are the two answers the setting produces.

### Key Entities

- **The setting's state**: the three stages of the rule — off, registration only, everything — and the
  process's reading of it at start-up.
- **A code step**: the pending verification a registration or a sign-in leaves behind, with the code
  and the lifetime the person was told.
- **A session family**: the set of refresh credentials one sign-in created, which the cutover
  revokes together.
- **The cutover**: the act of moving the setting from one state to the next, and what it contains.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: The rule can be in exactly one of its three states, and no deployment ever runs on an unrecognised one: 0 processes start with an unknown value.
- **SC-002**: 0 accounts are locked out by the first step: in the registration-only state, every existing account can still sign in with a password.
- **SC-003**: 100% of the sessions issued before the cutover stop renewing at their first renewal attempt, with no operator action.
- **SC-004**: 0 accounts that verified their mailbox are treated as unverified in any state: their renewal is never refused.
- **SC-005**: 0 client changes are needed: the answers a client receives are the same two it already handles.
- **SC-006**: The client contract is unchanged across the three states: 0 new answer shapes, 0 new failure codes.
- **SC-007**: The runbook states the order, the numbers, the window, the way back and the consequences of each flip: 0 undocumented steps in the activation.
- **SC-008**: Turning the setting back to off restores the previous contract exactly: 0 residual behaviour from the rule.
- **SC-009**: The suite is race-clean and non-vacuous; every failure path asserts its typed error.

## Assumptions

- This is the eighth feature derived from `docs/plan-auth-otp-2fa-email.md` §9. It delivers stage 7:
  the activation of the rule the fourth feature shipped dark.
- The rule has never been turned on in any deployment, so the setting's shape can change without
  breaking a live configuration (clarification, 2026-10-21).
- The renewal refusal and the family revocation are delivered by the fourth feature; this feature does
  not re-implement them, it only names the state in which they apply.
- The plan's optional cleanup of the refresh families is deliberately not delivered (clarification,
  2026-10-21): the refusal revokes the family when the credential is first presented, so the cutover
  is self-cleaning for every account that comes back, and the rows that remain are marked revoked and
  expire on their own.
- No cohort by percentage is needed: the plan's order is a sequence of two steps, and the setting's
  states express exactly that sequence.
- The client is not touched: it already handles both answers, and the setting is invisible to it.
- The counters the runbook names exist on the server today; the alerts that read them arrive with the
  next feature.
- Requirement-ID prefix `ROL` is exclusive to this feature; `D1`, `D2`, `D4` and `O5` belong to the
  plan and are not requirement IDs.
