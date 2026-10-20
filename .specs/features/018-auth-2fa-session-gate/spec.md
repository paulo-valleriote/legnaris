# Feature Specification: Sign-in Hardening — The Code Step Replaces the Immediate Session, and Unverified Accounts Cannot Renew

**Feature Branch**: `018-auth-2fa-session-gate`

**Created**: 2026-10-20

**Status**: Draft

**Input**: User description: "harden sign-in: registration and sign-in issue an email code challenge instead of a session, renewal refuses accounts without a verification timestamp, and the client can read its verification state" (plan `docs/plan-auth-otp-2fa-email.md` §9 stage 4; decisions D1, D2, D4, O5; findings F-02, F-03, F-05, F-10)

## Clarifications

### Session 2026-10-20

- Q: When renewal is refused because the account has never verified its email, should the presented refresh credential's whole session family be revoked, ending that session everywhere? → A: Revoke the family (user decision, 2026-10-20): the refusal mirrors the existing missing-account branch, so a session issued before the rule cannot renew once more anywhere, and the person recovers by signing in and completing the code step (`TFA-06`, `SC-003`).
- Q: This feature fixes sign-in's timing oracle, but registration still answers 409 for an address that already has an account (plan finding F-05). Is fixing that part of this feature's scope? → A: Out of scope here (user decision, 2026-10-20): the plan's stage-4 text addresses only the sign-in oracle, so registration keeps its conflict outcome and F-05's registration half remains a documented residual risk — changing it would break the existing contract and the client (`TFA-01`, Assumptions).
- Q: Plan §9 stage 7 wants to turn the rule on for new accounts before everyone. Should this feature's toggle already support that staged order? → A: No — one on/off gate, default off (user decision, 2026-10-20). The staged order and any cohort notion remain stage 7's; the operator flip is a configuration act, and this feature ships the gate (`TFA-07`, `TFA-12`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Proving a password is no longer proving the mailbox (Priority: P1)

Someone registers, or signs in with a correct password. Today both actions hand back a working session on the spot. From now on they must hand back only a code step: the account exists, the password was accepted, and an email with six digits is on its way — but no session, no cookie, no access credential is issued until those digits come back. The code step is the only door into a session.

**Why this priority**: this is decision D1, and it is the change that makes every account's second factor mandatory rather than optional. It is also the breaking half of the feature: the register and sign-in contracts change for every client.

**Independent Test**: with the gate enabled, register a fresh account and assert the response carries a challenge handle and no credential of any kind; sign in with the correct password and assert the same; verify the returned handle with the code and assert a session is finally established; sign in with a wrong password and assert no challenge was started.

**Acceptance Scenarios**:

1. **Given** the gate is enabled, **When** a registration succeeds, **Then** the unverified account is created, a verification-purpose code step is started, and the response carries the challenge handle and its lifetime information with no session, no access credential and no cookie. (TFA-01)
2. **Given** the gate is enabled and an account with a correct password, **When** the person signs in, **Then** a sign-in-purpose code step is started and the response is the accepted outcome carrying the challenge handle — never a session. (TFA-02)
3. **Given** the gate is enabled, **When** the correct password is supplied at sign-in, **Then** the code is started only after the password matched; **When** the password does not match, **Then** the existing uniform invalid-credentials outcome is answered and no code is started for anyone. (TFA-03)
4. **Given** the gate is enabled and a started code step, **When** the handle is verified with the correct code, **Then** the session is established exactly as the challenge domain already does it — consumed once, the account's verification timestamp recorded when it was absent, and an existing timestamp preserved. (TFA-04)
5. **Given** the gate is enabled, **When** sign-in is attempted for an address with no account and for an address with an account but the wrong password, **Then** both answers are the same outcome and take the same password-checking work, so neither the response nor its timing reveals which case occurred. (TFA-05)

---

### User Story 2 - A session that predates the rule cannot outlive it (Priority: P1)

A session issued before this change carries a refresh credential that renews for thirty days. If renewal keeps renewing it, "verify on your next sign-in" is false for exactly the accounts that have never verified — the ones the rule exists for. Renewal must look at the account's verification state and end the session when the account has never verified.

**Why this priority**: decision O5. Without it the whole feature applies only to sessions that do not yet exist, and every pre-existing session silently bypasses the verification requirement for its full thirty-day life.

**Independent Test**: mint a session for an account with no verification timestamp, enable the gate, then renew and assert 401 with no access credential, no rotated credential and the family revoked; set the verification timestamp and assert the same renewal now succeeds; disable the gate and assert renewal no longer consults the state at all.

**Acceptance Scenarios**:

1. **Given** the gate is enabled and a valid refresh credential whose account has no verification timestamp, **When** renewal is attempted, **Then** it is refused with the existing uniform invalid-credential outcome, issues no access or refresh credential, and revokes the presented credential's family. (TFA-06)
2. **Given** the gate is enabled and a valid refresh credential whose account carries a verification timestamp, **When** renewal is attempted, **Then** it succeeds exactly as the existing rotation contract does. (TFA-06)
3. **Given** the gate is disabled, **When** register, sign in and renew are exercised, **Then** every observable outcome is exactly today's. (TFA-07, TFA-12)

---

### User Story 3 - The signed-in client can see whether its email is verified (Priority: P2)

After a page reload the client has only an access credential and no idea whether the account behind it ever verified its email. Without a readable state, the only moment the client can know is the sign-in answer itself, and any screen that depends on verification is guessing. The signed-in account must be readable, including whether verification happened.

**Why this priority**: finding F-10. It is the read that the next stage's screens need, and it is additive — no account can be harmed by it.

**Independent Test**: with a valid access credential, read the signed-in account and assert it reports the stable identifier, the address, the display name and whether the email is verified; repeat before and after verification and assert only the verification state changes; read again with no, or an invalid, access credential and assert the existing uniform unauthorized outcome.

**Acceptance Scenarios**:

1. **Given** a signed-in account whose email is not verified, **When** its identity is read, **Then** the answer reports the identifier, the canonical address, the display name and an unverified state. (TFA-08)
2. **Given** the same account after a successful verification, **When** its identity is read again, **Then** only the verification state changed and the verification instant itself is not disclosed. (TFA-08)
3. **Given** no access credential, an expired one, or a forged one, **When** the identity is read, **Then** it is answered with the existing uniform unauthorized outcome and no account data. (TFA-09)

---

### User Story 4 - The whole change can be deployed dark and turned on deliberately (Priority: P2)

The register and sign-in contracts break every client at once. Shipping that only as "the new behavior" means the deploy itself is the cutover, with no way to back out. The change must be one configuration gate, off by default, that the operator turns on when the clients and the deployment are ready — and the gate must change nothing else.

**Why this priority**: plan §9 stage 7 exists precisely because a breaking contract change needs a dark-ship and a deliberate flip; this feature delivers the gate so that stage is a configuration act.

**Independent Test**: with the gate off, run the existing register, sign-in and renewal suites unchanged and assert they pass byte-for-byte on status, body, cookies and stored data; with it on, assert the three contract changes hold and that the challenge domain's own suites pass unchanged; assert the gate is absent from the deployment's required-variable list.

**Acceptance Scenarios**:

1. **Given** the gate is off, **When** a registration succeeds, **Then** the account is created and the session is established exactly as today. (TFA-07, TFA-12)
2. **Given** the gate is off, **When** the client reads its identity, **Then** the read is still available and truthful. (TFA-08)
3. **Given** the gate is on, **When** codes are issued, verified and left to expire, **Then** the challenge domain's own rules, limits and counters are unchanged. (TFA-11)
4. **Given** the daily send allowance is already spent, **When** a registration would start a code step, **Then** the refusal is the existing typed unavailable outcome with a retry hint and no session, and the same credentials can still obtain a code once the daily allowance frees at the next UTC day, instead of the account becoming unreachable. (TFA-13)

---

### Edge Cases

- **Two sign-ins in a row for one account**: each answers the accepted outcome and starts a code step; only the most recently issued challenge remains usable, so a client holding a stale handle fails uniformly and must start over. (TFA-02, OTP-01)
- **Registration whose address already has an account**: the existing conflict outcome is unchanged — no second account, no challenge, and no send. (TFA-01)
- **Registration with an unparseable address, an invalid display name or a too-short password**: the existing typed invalid-request outcome, no account, no challenge. (TFA-01)
- **Verification of a registered account**: the code step carries the registration purpose and the verification timestamp it records is the account's first. (TFA-04)
- **Sign-in for an account with no verification timestamp**: the code step carries the sign-in purpose, and verifying it records the timestamp as part of establishing the session (decision D4). (TFA-04)
- **Sign-in for an account already verified**: the existing timestamp is preserved unchanged. (TFA-04)
- **Unknown address versus wrong password at sign-in**: identical status, identical body and indistinguishable work performed. (TFA-05)
- **Renewal for an account deleted between the credential lookup and the account lookup**: unchanged — the missing-account rule decides. (TFA-06)
- **Renewal for an account with no verification timestamp while the gate is on**: refused, family revoked, no credential issued; a second attempt with the same credential also fails, and the person recovers by signing in and completing the code step. (TFA-06)
- **Gate flipped while sessions exist**: renewal of a pre-existing session is refused from the next renewal onward; nothing else changes. (TFA-06, TFA-12)
- **Gate flipped off again**: register, sign-in and renewal return to today's behavior; no account data is rewritten either way. (TFA-12)
- **Send allowance exhausted at registration**: no session, the typed unavailable outcome with a retry hint, and the account is not left unable to obtain a code later. (TFA-13)
- **Identity read for an account removed after the access credential was minted**: the credential no longer names a usable account, so it is answered 401 unauthorized with no account data — never 404 and never a partial record. (TFA-09)
- **Verification instant leaking**: only the boolean state is reported; the instant never appears in a response. (TFA-08)
- **Credential appearing on a challenge-issuing answer**: prohibited — the register and sign-in answers of the gate carry no cookie and no access credential. (TFA-01, TFA-02)
- **Challenge start failing in the store after a registration created the account**: reported as an internal failure with no session and no credential, and the code step can be started again with the same credentials. (TFA-13)

## Requirements *(mandatory)*

### Functional Requirements

**The credential step replaces the immediate session (US1)**

- **TFA-01**: WHEN the gate is enabled and a registration succeeds, the server MUST create the account with no verification timestamp, start a code step whose purpose is the registration's, and answer the created outcome carrying the challenge identity, the claimed lifetime and the resend window — and MUST establish no session, set no session cookie and return no access credential.
- **TFA-02**: WHEN the gate is enabled and sign-in presents valid credentials, the server MUST start a code step whose purpose is the sign-in's and answer the accepted outcome carrying the same information as TFA-01 — and MUST establish no session. WHEN the credentials are invalid, the existing uniform invalid-credentials outcome MUST be unchanged and no code step MUST be started.
- **TFA-03**: A code step MUST be started only after the request's own credential proof succeeded — a valid registration input, or a matching password for a known account — so no request can cause a code to be sent for an account it did not authenticate. The purpose MUST be derived by the server from the operation and MUST NOT be accepted from the client.
- **TFA-04**: The only way to establish a session while the gate is enabled MUST be a successful verification of a started code step, and that verification MUST keep the challenge domain's existing contract exactly: consumed at most once, the account's verification timestamp recorded when absent and preserved when present, then the session established exactly as sign-in establishes it. A verification whose purpose is the registration's MUST establish the session too.
- **TFA-05**: Sign-in MUST perform the same password-verification work for an unknown address as for a known one — whichever state the gate is in — so neither the status, the body nor the response time distinguishes an account that does not exist from one whose password is wrong.

**Renewal consults the verification state (US2)**

- **TFA-06**: WHEN the gate is enabled and renewal resolves a valid, unrevoked, unexpired credential, the server MUST consult the account's verification state after loading the account and before issuing anything; IF the account has no verification timestamp, renewal MUST be refused with the existing uniform invalid-credential outcome, MUST issue no access or refresh credential, and MUST revoke the presented credential's family.

**Reading the signed-in account (US3)**

- **TFA-08**: The server MUST expose an authenticated read of the signed-in account that reports the stable identifier, the canonical address, the display name and whether the email is verified as a boolean state; the verification instant itself MUST NOT be reported, and this read MUST be available whether or not the gate is enabled.
- **TFA-09**: The identity read MUST require a valid access credential, MUST answer the existing uniform unauthorized outcome when it is absent, expired or forged, and MUST expose neither the password hash nor any credential. The account representation used on the wire MUST carry the verification state, superseding the earlier rule that the field never reaches the wire; the stored instant remains internal.

**The rollout gate (US4)**

- **TFA-07**: The gate MUST be a single on/off configuration setting, optional and documented, whose default is disabled; enabling it changes only the register, sign-in and renewal contracts above and MUST NOT rewrite any account data or change the stored verification semantics.
- **TFA-11**: The gate MUST NOT change the challenge domain delivered by the preceding feature: its purposes, its limits, its consumption, its resend rules and its counters MUST behave identically with the gate either way, and no new session, cookie or access-credential format MUST be introduced.
- **TFA-12**: WHILE the gate is disabled, the register, sign-in and renewal **contract** — status, body, cookies and stored data — MUST be exactly today's, so the existing suites pass unchanged; response time alone is not part of that parity, because the equalized password work of TFA-05 applies in both states; the identity read of TFA-08 remains available.
- **TFA-13**: WHEN the daily send allowance refuses the code step that a registration would start, or the store fails while starting it, the server MUST answer the typed failure with no session and no credential, and MUST NOT leave the person unable to obtain a code later with the same credentials.

**Cross-cutting**

- **TFA-10**: Every new failure mode — a refused renewal for an unverified account, an unavailable send at registration, a store failure while starting a code step, an unauthorized identity read, and an identity read whose credential names an account that no longer exists — MUST be a typed error with an exported message constant and a documented HTTP status, and the boundary MUST map it exactly. The missing-account read is 401 `UNAUTHORIZED` and MUST NOT be 404, so it is indistinguishable from an invalid credential. No new credential surface MUST be added: the register, sign-in and code-step routes keep the guards and per-address limits they already have.
- **TFA-14**: The requirement corpus MUST record the contract this feature supersedes: the existing registration and sign-in requirements that describe a session established immediately no longer describe the gate-enabled behavior, and the client-side premise that registration authenticates MUST be recorded as superseded by the client stage. Requirement IDs cited in source MUST keep resolving to a definition that matches the shipped behavior.
- **TFA-15**: Every requirement MUST ship tests — edge cases first, then one happy path — updating the suites that assume "registration and sign-in return a session" in the same change, using the injected clock for every boundary and with no sleeps, asserting the typed error and mapped status on every failure path, and staying clean under the race detector.
- **TFA-16**: The gate MUST come from the shared configuration surface and MUST be documented; it MUST NOT be added to the deployment's required-variable set, and the deployment description and its validation MUST follow it. All identifiers, comments, log messages and user-facing strings MUST be written in English, and no log line this feature adds may carry a code or an unmasked address.

### Key Entities

- **Rollout gate**: the single configuration setting that decides whether registration and sign-in hand back a code step or a session, and whether renewal consults the verification state; disabled by default, changes no stored data.
- **Code step (started challenge)**: the challenge record of the preceding feature, now the required middle of registration and sign-in while the gate is enabled — its identity is the handle the client presents back, and its purpose is derived from the operation.
- **Verification state**: whether an account's address has ever been verified, read by the client and consulted by renewal; reported as a boolean, stored as an instant.
- **Session**: the credential set the challenge domain already establishes — access credential in the body, refresh and CSRF cookies — unchanged in format and now reachable only through a verified code step while the gate is enabled.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With the gate enabled, 0 successful registrations and 0 valid-credential sign-ins establish a session, set a session cookie or return an access credential; 100% answer with the challenge handle instead.
- **SC-002**: 0 sign-in responses are distinguishable — in status, body or password-checking work — between an address with no account and an address whose password is wrong.
- **SC-003**: 100% of renewals for an account with no verification timestamp answer unauthorized with the family revoked and issue 0 credentials; 0 such sessions renew even once.
- **SC-004**: With the gate disabled, 0 register, sign-in or renewal **contract** outcomes differ from today's (status, body, cookies, stored data), and 100% of the pre-existing suites pass unchanged.
- **SC-005**: 100% of new failure paths assert their typed code and mapped status; the suite is race-clean and runs with 0 sleeps.
- **SC-006**: The verification state is readable in one authenticated request in 100% of signed-in states, and 0 responses disclose the verification instant, the password hash or any credential.
- **SC-007**: With the gate enabled, 100% of the challenge domain's own suites and limits behave identically to the preceding feature, and 0 accounts are rewritten by enabling or disabling the gate.

## Assumptions

- This is the fourth of the features derived from `docs/plan-auth-otp-2fa-email.md` §9; it delivers stage 4 (the register/sign-in hardening, the renewal gate and the verification read) only. The client changes — the pending-code state, the code screen, the error mapping and the copy — are stage 5, and this feature changes no client code while recording its premise as superseded.
- The gate is one global on/off setting whose default is disabled (clarification, 2026-10-20). Plan §9 stage 7 describes a staged activation ("new accounts first, then everyone"); a single gate cannot express a cohort, so the staged order, the operational flip and any optional cleanup of pre-existing refresh families remain stage 7's, which may amend the setting if a cohort is required.
- Decision D4 is already delivered by the challenge domain (the verification that sets the timestamp when absent and preserves it when present, `OTP-07`); this feature only wires registration and sign-in to it and gates renewal on the same fact.
- The renewal refusal for an unverified account revokes the presented credential's family, matching the existing missing-account branch and the plan's containment intent (clarification, 2026-10-20).
- Registration's existing duplicate-address conflict outcome is unchanged (clarification, 2026-10-20: out of scope for this feature), so it still reveals that an address has an account; plan finding F-05 is only partially addressed here, because the sign-in timing oracle is fixed and the registration conflict is not.
- The identity read reuses the existing bearer authentication and the existing success body envelope; the exact field names and envelope shape are fixed during design, and the verification state is a boolean while the stored instant stays internal.
- The preceding features' seams are what this feature builds on: the canonical address and real client address (`PRQ`), the durable send intention and its daily cap (`EML`, `OTP-13`), and the challenge issuance, uniform verification failure and per-key limits (`OTP`). No new third-party dependency is expected.
- No account data is migrated: every existing account has no verification timestamp, and enabling the gate does not backfill one; a person with an existing session and no timestamp recovers by signing in and completing the code step.
- Superseded identifiers to be reconciled in the corpus: `AUTH-01` and `AUTH-02` (registration/sign-in establish the session immediately) and `WAS-02` (registration authenticates) no longer describe the gate-enabled behavior; the verification field's "never on the wire" rule (`PRQ-12`, `PRQ-14`) is superseded for the read path only.
- Requirement-ID prefix `TFA` is exclusive to this feature; identifiers `D1`, `D2`, `D4`, `O5`, `F-02`, `F-03`, `F-05`, `F-10`, `F-11`, `F-18`, `AUTH-01`, `AUTH-02`, `WAS-02`, `PRQ-12`, `PRQ-14` and `OTP-07` belong to the plan and to other features and are not requirement IDs of this one.
