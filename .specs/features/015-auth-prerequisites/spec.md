# Feature Specification: Auth Prerequisites — Real Client Address, Canonical Email, Verification-Ready Accounts

**Feature Branch**: `015-auth-prerequisites`

**Created**: 2026-10-20

**Status**: Draft

**Input**: User description: "Harden backend authentication prerequisites: real client address behind the proxy, canonical email storage, injectable clock, user email-verification persistence, and refresh loading the user" (plan `docs/plan-auth-otp-2fa-email.md`, §9 stage 1, findings F-01, F-03, F-04, F-06, F-08)

## Clarifications

### Session 2026-10-20

- Q: How should existing accounts whose stored address is not canonical be handled? → A: One-shot
  operator-run migration command (user decision, 2026-10-20): an idempotent command with a dry-run
  mode canonicalizes existing rows and reports collisions without writing; the API boot path never
  mutates account data (`PRQ-11`).
- Q: Should the verification timestamp and its store operation land in this feature even though
  nothing calls them yet? → A: Yes, both (user decision, 2026-10-20): the account record gains the
  optional timestamp and the store gains the idempotent mutation, with no HTTP surface change
  (`PRQ-12`…`PRQ-15`).
- Q: Which trusted-proxy default should resolve the client address? → A: Private ranges by default,
  configurable (user decision, 2026-10-20): loopback plus private and link-local networks are trusted
  unless a comma-separated CIDR set replaces the non-loopback portion (`PRQ-03`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A blocked client is the attacker, not everyone behind the proxy (Priority: P1)

Every request reaches the API through the edge proxy, so the address the API sees on its own socket belongs to the proxy and is identical for every person on the internet. The API must resolve the address of the real client from the forwarded chain, trusting that chain only when it comes from the proxy. One person exhausting the login limit must be blocked alone; a person who forges a forwarding header must not be able to change which limit applies to them.

**Why this priority**: with one shared key, ten failed logins per minute from a single attacker lock out registration, login, refresh and any future code verification for the entire user base. The 2FA work increases auth traffic, so this must be true before it lands.

**Independent Test**: drive the auth routes with a request whose direct peer is inside the trusted proxy set and whose forwarded chain names a client, and assert the per-client limit is consumed for that named client; repeat with a second named client and assert it is unaffected; then send a forged chain from a peer outside the trusted set and assert the forged value has no effect.

**Acceptance Scenarios**:

1. **Given** a request whose direct peer is a trusted proxy and whose forwarding chain names one client, **When** the request is handled, **Then** the client address is the nearest untrusted address in the chain and the per-address decisions use it. (PRQ-01, PRQ-04)
2. **Given** two clients behind the same trusted proxy, **When** one exhausts its per-address limit, **Then** the other client is still allowed. (PRQ-04)
3. **Given** a request whose direct peer is not a trusted proxy, **When** it carries a forwarding header naming some other address, **Then** the header is ignored and the decisions use the direct peer. (PRQ-02)
4. **Given** a request with no forwarding header, or with one that cannot be parsed, **When** it is handled, **Then** the decisions use the direct peer. (PRQ-01)

---

### User Story 2 - A session cannot renew for an account that no longer exists (Priority: P1)

A refresh credential names the account it was minted for. Today renewal trusts that name and never looks the account up, so the session survives independently of the account and there is no point where account state can be consulted. Renewal must resolve the account first: if the account cannot be found, no new credential is issued and the session ends.

**Why this priority**: the email-verification gate of the next features must be evaluated on the account at renewal time; without this step there is no account to evaluate, and sessions issued before the deploy keep renewing for their full lifetime. It is also a latent correctness hole today.

**Independent Test**: mint a session, remove its account from the store, then refresh and assert 401 with no access token, no rotation, and the family revoked; repeat with the account present and assert the refresh succeeds with an access token naming that account; then drive an account-store failure and assert the internal error is not reported as an authentication failure.

**Acceptance Scenarios**:

1. **Given** a valid refresh credential whose account still exists, **When** refresh succeeds, **Then** the account is loaded before any credential is issued and the new access token names that account. (PRQ-05, PRQ-06)
2. **Given** a valid refresh credential whose account no longer exists, **When** refresh is called, **Then** it is answered 401, no access or refresh credential is issued, and the presented credential's family is revoked. (PRQ-05, PRQ-07)
3. **Given** an account-store failure while resolving the refresh credential's account, **When** refresh is called, **Then** it is answered as an internal failure, not as invalid credentials, and no credential is issued. (PRQ-07)
4. **Given** expired, revoked, reused or concurrently rotated credentials, **When** refresh is called, **Then** the existing outcomes and family rules are unchanged. (PRQ-05)

---

### User Story 3 - One mailbox is one account, however the address is spelled (Priority: P2)

People type the same mailbox in different ways: with a display name, in capitals, with leading or trailing spaces, with a `+tag`. Registration currently validates the address and then stores the raw input, so two spellings of one mailbox can become two accounts and the stored value may not be an address a mail provider can deliver to. The account store must hold the bare canonical address, duplicate detection must compare canonical addresses, sign-in must apply the same normalization, and existing non-canonical rows must be repairable before the email features depend on them.

**Why this priority**: every later feature sends a six-digit code to the stored address; a stored display name is an undeliverable recipient, and a duplicate account splits the verification state between two rows. It is not P1 only because no current release depends on the new flow yet.

**Independent Test**: register with a display-name form of an address and assert the stored value is the bare lowercase address; register again with a different spelling of the same mailbox and assert a conflict; sign in with a differently spelled form and assert the same account is reached and the per-account limit bucket is shared; run the migration in dry-run and then for real over a seeded non-canonical row and assert only the address and modification time change; seed a collision and assert the run aborts without writing; drive boundary lengths and invalid forms and assert the existing typed 400 on registration.

**Acceptance Scenarios**:

1. **Given** a registration whose address carries a display name, capitals or surrounding whitespace, **When** it succeeds, **Then** the stored address is the bare lowercase address with no display name or whitespace. (PRQ-08)
2. **Given** an account stored with a canonical address, **When** a second registration canonicalizes to the same address, **Then** it is answered 409 and no second account exists. (PRQ-10)
3. **Given** an account stored with a canonical address, **When** sign-in supplies an equivalent spelling, **Then** the same account is resolved. (PRQ-09)
4. **Given** an address that cannot be parsed, or one longer than the accepted maximum, **When** registration is called, **Then** it is refused with the existing typed invalid-request outcome. (PRQ-08)
5. **Given** a `+tag` or a dot inside the local part, **When** the address is canonicalized, **Then** neither is removed. (PRQ-08)
6. **Given** existing rows with non-canonical addresses, **When** the migration runs in dry-run mode, **Then** it reports the intended canonical values and writes nothing; **When** it runs for real, **Then** it rewrites only the address and modification time, and a run that finds two rows canonicalizing to one address aborts without writing. (PRQ-11)
7. **Given** one mailbox reachable through several spellings, **When** sign-in is attempted repeatedly using those spellings, **Then** the attempts consume one per-account budget and the account key is never empty. (PRQ-09)

---

### Edge Cases

- **Forwarding chain with several proxies**: the client is the nearest untrusted hop walking from the server outward; when every hop is trusted, the furthest named hop is used. (PRQ-01)
- **Forwarding chain containing an unparseable entry**: the whole header is distrusted and the direct peer is used. (PRQ-01)
- **Forwarding chain naming IPv6 addresses with or without brackets, or entries with ports**: the address is still resolved. (PRQ-01)
- **Two forwarding headers on one request**: their values are treated as one chain in order. (PRQ-01)
- **Forged forwarding header from an untrusted peer**: 0 effect on limits or on any address-derived decision. (PRQ-02)
- **Trusted proxy set unset or blank**: the documented loopback-plus-private-and-link-local default applies and the deployment still limits per client; a request is never keyed by an empty address. (PRQ-03)
- **Trusted proxy set configured but empty** (for example `","`): the configured set replaces the whole non-loopback portion, so only loopback is trusted; this is fail-closed and never a fallback to the default. (PRQ-03)
- **An attacker inside the trusted proxy network**: can choose its own forwarded address; the trust boundary is the proxy network, and the deployment keeps that network internal. (PRQ-03, Assumptions)
- **Address whose canonical form is 254 bytes** is accepted; one byte more is refused. (PRQ-08)
- **Empty, whitespace-only, `@`-less, or domain-less address** on registration: refused with the existing typed invalid request; on sign-in: the uniform invalid-credentials outcome. (PRQ-08, PRQ-09)
- **Login with a display-name form**: canonicalized before lookup; a parse failure still answers the same generic 401 and does not reveal account existence. (PRQ-09)
- **Duplicate account under a display-name spelling of an existing address**: conflict, no second row. (PRQ-10)
- **Legacy row whose stored address was never canonical**: the migration command rewrites it in place, preserving its identifier and every other field; the API boot path does not modify it. (PRQ-11)
- **Two legacy rows canonicalize to the same address**: the migration reports both, writes nothing, and exits unsuccessfully for manual resolution; no automatic merge. (PRQ-11)
- **A legacy row whose address cannot be parsed**: the migration reports it and, like a collision, writes nothing before exiting unsuccessfully. (PRQ-11)
- **Migration run twice**: the second run reports no pending changes and writes nothing. (PRQ-11)
- **Refresh for an account removed between the credential lookup and the account lookup**: answered 401 with the family revoked, exactly like a missing account. (PRQ-05)
- **Refresh with a revoked/expired/reused credential whose account is missing**: the existing credential rules decide first; the account check does not change those outcomes. (PRQ-05)
- **Account-store failure during refresh**: internal failure, no credential issued, family untouched. (PRQ-07)
- **Family revocation failure while handling a missing account**: internal failure, no credential issued, and the caller sees neither a 401 nor a session. (PRQ-05)
- **Verification timestamp already set**: marking it again is a no-op that preserves the first value and does not fail. (PRQ-13)
- **Verification timestamp for an unknown account**: reported as not found, never created. (PRQ-13)
- **Deploy over existing data**: no existing account gains a verification timestamp. (PRQ-15)
- **Clock at the exact expiry instant**: the credential is expired; one nanosecond before, it is still valid. (PRQ-16)

## Requirements *(mandatory)*

### Functional Requirements

**Resolve the real client address (US1)**

- **PRQ-01**: WHEN a request carries a forwarding chain, the server MUST resolve the client address as the nearest untrusted hop from the server outward; WHEN the chain is absent or cannot be parsed, it MUST use the direct peer address.
- **PRQ-02**: WHEN a request's direct peer is outside the trusted proxy set, the server MUST ignore any forwarding header it carries and MUST use the direct peer address.
- **PRQ-03**: The trusted proxy set MUST come from configuration; by default it MUST trust loopback, private, and link-local networks, and a configured comma-separated CIDR set MUST replace the non-loopback portion of that default (loopback stays trusted). An unset or blank variable selects the default; a configured value selects exactly the operator's set and never falls back to the default. The set MUST NOT be hardcoded in request handling, and a request MUST never be keyed by an empty address.
- **PRQ-04**: Every per-address decision on the auth routes — including the existing rate-limit keys — MUST use the resolved client address, so two clients behind one proxy are limited independently and an untrusted peer cannot choose its key.

**Resolve the account on renewal (US2)**

- **PRQ-05**: WHEN refresh resolves a credential, the server MUST load the credential's account before issuing anything; IF the account cannot be found, refresh MUST be answered 401, MUST NOT issue an access or refresh credential, and MUST revoke the presented credential's family. IF that revocation itself fails, refresh MUST be answered as an internal failure and MUST still issue nothing.
- **PRQ-06**: WHEN refresh succeeds, the issued access token MUST name the account that was loaded, and the rotated session MUST belong to that same account.
- **PRQ-07**: An account-store failure while resolving the account MUST be reported as an internal failure — never as invalid credentials — and MUST NOT issue any credential. The new failure path MUST use a typed error with an exported message and a documented status.

**Canonical email (US3)**

- **PRQ-08**: WHEN an account is created, the server MUST parse the supplied address, persist the bare canonical address (no display name, no surrounding whitespace, lowercase), and refuse an unparseable address, or an address whose canonical form is longer than 254 bytes, with the existing typed invalid-request outcome. Lowercasing and trimming MUST be the only transformations: dots and `+tags` in the local part MUST be preserved.
- **PRQ-09**: SIGN-IN MUST canonicalize the supplied address the same way before lookup so equivalent spellings resolve to the same account; an address that cannot be parsed MUST still produce the uniform invalid-credentials outcome so account existence is not revealed. The per-account rate-limit key MUST use the same canonicalization, with a trimmed-lowercase fallback when the address cannot be parsed, so every spelling of one mailbox occupies one budget.
- **PRQ-10**: Duplicate-account detection MUST compare canonical addresses, so two spellings of one mailbox cannot create two accounts (409, no second row).
- **PRQ-11**: The feature MUST provide a one-shot, operator-run, idempotent migration command that canonicalizes existing stored addresses using the same rules as creation, supports a dry-run mode that reports intended changes without writing, changes only the address and modification time of each repaired row, exits with status 1 when two rows canonicalize to one address or a stored address cannot be parsed (reporting the affected rows and writing nothing), exits with status 2 on a usage error, and is never part of the API boot path.

**A verification-ready account record**

- **PRQ-12**: The account record MUST carry an optional email-verification timestamp that is absent for every existing and newly created account; the feature MUST NOT backfill it. **Superseded for the read path by `TFA-09`** (`.specs/features/018-auth-2fa-session-gate/spec.md`): the identity read reports the derived verification state, while the stored instant stays internal and the record itself is unchanged.
- **PRQ-13**: The account store MUST expose an operation that records the verification timestamp for an existing account, leaves every other field untouched, updates the modification timestamp in the same write, is idempotent when the timestamp is already set (the first value is preserved), and reports an unknown account as not found.
- **PRQ-14**: The verification operation MUST NOT be reachable from the HTTP surface in this feature, and no existing endpoint MUST change its response shape. **Superseded for the read path by `TFA-08`** (`.specs/features/018-auth-2fa-session-gate/spec.md`): the new identity read reports the verification state as a boolean and never the instant, and no pre-existing endpoint changed its response shape.
- **PRQ-15**: Boot and index creation over existing accounts MUST NOT add the field: the count of accounts carrying a verification timestamp is unchanged after deployment.

**Deterministic time**

- **PRQ-16**: Every timestamp the auth service writes or compares — account creation, refresh-credential issuance, refresh-credential expiry, and access-token issuance and validation — MUST come from one injectable clock; production MUST use the system clock, and the observable behavior MUST be unchanged. Reuse and rotation decisions carry no timestamps and are decided by credential state alone.

**Cross-cutting**

- **PRQ-17**: Every new failure mode MUST be a typed error with an exported message constant and a documented HTTP status (401 for a missing account on refresh, 500 for store failures, 409 for a canonical duplicate, 400 for invalid registration input), and the boundary MUST map it exactly.
- **PRQ-18**: Configuration needed by this feature MUST be optional, with documented defaults, so the deployment's required-variable list and its compose validation do not change.
- **PRQ-19**: Every requirement MUST ship tests — edge cases first, then one happy path — asserting the typed error and mapped status on every failure path and staying clean under the race detector.
- **PRQ-20**: All identifiers, comments, log messages, and user-facing strings MUST be written in English.

### Key Entities

- **Client address**: the address the server uses for per-client decisions; derived from the forwarding chain only as far as it trusts hops, otherwise the direct peer.
- **Trusted proxy set**: the configured networks whose forwarding headers are believed; by default loopback plus private and link-local networks, where the deployment's edge proxy lives.
- **Account (user)**: the existing identity record (identifier, email, username, password hash, timestamps) extended with an optional email-verification timestamp that is absent until verification happens.
- **Refresh session**: the existing rotated credential chain; this feature adds the requirement that its account still exists and is loaded at renewal.
- **Auth clock**: the single time source for timestamp writes and expiry comparisons in the auth service.
- **Address-repair run**: the one-shot migration's report — each legacy row with its canonical destination, plus any collision that prevents writing.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a proxied deployment, 0 auth decisions are keyed by the proxy's own address, and 100% of requests from two different clients behind that proxy are limited independently.
- **SC-002**: A forged forwarding header from an untrusted peer changes 0 address-derived decisions.
- **SC-003**: 0 access or refresh credentials are issued on refresh when the account cannot be resolved, and 100% of such calls answer 401 with the family revoked.
- **SC-004**: 100% of created accounts store the bare lowercase address with 0 display names and 0 surrounding whitespace, and 0 duplicate accounts can be created from different spellings of one mailbox.
- **SC-005**: 0 existing accounts carry a verification timestamp after deployment, and 0 accounts are created with one.
- **SC-006**: 100% of new failure paths assert their typed code and mapped status; the suite is race-clean and non-vacuous.
- **SC-007**: Tests for expiry and rotation boundaries run without sleeping, using the injected clock.
- **SC-008**: The migration run over already-canonical data reports 0 changes and writes 0 rows; a run over a collision writes 0 rows and exits unsuccessfully.

## Assumptions

- This is the first of the features derived from `docs/plan-auth-otp-2fa-email.md` §9; it delivers stage 1 only. It intentionally changes no HTTP contract, adds no email infrastructure, and does not enable the verification gate — later features consume the seams it creates.
- The API is reachable only through the edge proxy in every non-local deployment, so the proxy's network is the trust boundary. A peer inside that network is trusted; the deployment keeps the network internal and does not publish the API port. A configured CIDR set narrows that boundary.
- The deployment proxy forwards the client chain in `X-Forwarded-For`. `X-Real-IP` is not treated as a second, competing source of truth.
- Existing accounts were registered through a path that already trimmed and lowercased the input, so the only plausibly non-canonical legacy rows are those whose input contained a display name; the migration command handles them and is run by the operator before the email features are enabled.
- The email-verification timestamp is optional and has no default; this feature does not backfill it and does not expose it on any response.
- The refresh credential, its rotation, CSRF binding, cookie attributes, and the access-token format are unchanged.
- Configuration added here is optional with documented defaults, so the required-variable list and its deployment test remain valid.
- No new third-party dependency is expected: the work uses the existing HTTP stack, standard library parsing, the existing typed-error taxonomy, and the repository's test conventions.
- Requirement-ID prefix `PRQ` is exclusive to this feature; identifiers `F-01`, `F-03`, `F-04`, `F-06`, `F-08`, `D1`–`D6`, `O1`–`O6` belong to the plan and are not requirement IDs.
