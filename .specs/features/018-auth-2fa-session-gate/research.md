# Phase 0 Research: Sign-in Hardening — The Code Step Replaces the Immediate Session, and Unverified Accounts Cannot Renew

**Feature**: `.specs/features/018-auth-2fa-session-gate/` | **Date**: 2026-10-20

Every decision below answers an open question in the plan's Technical Context or makes explicit a
choice the program plan (`docs/plan-auth-otp-2fa-email.md` §3/§9) and the spec leave to the design.
No `NEEDS CLARIFICATION` remains; the three product-level choices were closed in the spec's
`## Clarifications` (2026-10-20).

---

## R1. The gate is decided in the auth service, not in the controller

**Decision**: `internal/auth` gains one boolean on its existing `Config` (`RequireEmailVerification`)
and the decision is taken inside `Register` and `Login`. Both keep their signatures' shape but return
a discriminated result — one new type in `internal/auth/start.go` whose `Session` and `Challenge`
halves are mutually exclusive — and the controllers project whichever half the service filled:
`Session` (legacy) sets the cookies and answers the session body; `Challenge` sets nothing and answers
the challenge handle. `Refresh` reads the same field to decide whether to consult the verification
state.

**Rationale**: the service already owns every other piece of this policy — the challenge policy
(`ChallengeDeps.Policy`), the session lifetime (`Config.RefreshTTL`), the injected clock — and it is
the only place that can decide *after* the credential proof succeeded but *before* a session is
minted. Keeping the decision here gives one point of truth: the controllers cannot disagree with the
service about whether a session was issued, because the answer is what they are handed back.

**Alternatives considered**:

- *The controller reads the flag and calls either `Register` or a new `RegisterChallenge`* —
  rejected: it duplicates the validation, hashing and duplicate-detection paths into two service
  methods (or forces the controller to branch around them), and moves policy into transport code,
  which the constitution's layering rule exists to prevent.
- *Two service constructors / a build-time strategy selected at wiring time* — rejected: it makes the
  flag unmockable in a single test binary and hides the legacy path behind a constructor the suites
  can no longer flip per case.
- *A second method pair (`StartRegistration`, `StartLogin`) and no discriminated result* — rejected:
  the registration validation, the password comparison and the duplicate check would have to be
  factored out anyway, so the split buys nothing over the result type and adds two names for one
  behaviour.

## R2. The setting: one optional boolean, read through the existing configuration surface

**Decision**: a new optional variable, `AUTH_REQUIRE_EMAIL_VERIFICATION`, read by a new
`lookupBoolEnv(key, default)` helper (`shared/env/env.go`) that parses with `strconv.ParseBool`
(accepting `1/0/t/f/true/false/TRUE/FALSE/True/False`) and — like the existing `lookupDurationEnv`
and `lookupIntEnv` — **logs a structured warning and returns the default for an unset, empty or
unparseable value**. The default is `false`. The value joins `Variables` and is copied into
`auth.Config` at the composition root. It is **not** added to `RequiredKeys()`.

**Rationale**: the repository's optional-configuration convention is exactly this shape, and the
existing helpers show the failure posture to mirror: an invalid optional value is never a boot error,
it warns and falls back. Falling back to `false` is also the fail-safe direction — a typo in the
deployment disables the new contract rather than silently turning the breaking behaviour on. Keeping
the variable out of `RequiredKeys()` preserves `deploy/compose_test.go`'s contract and matches
`TFA-16`.

**Alternatives considered**:

- *Reusing `EMAIL_*` or an existing variable* — rejected: the gate governs the sign-in contract, not
  email delivery, and overloading an existing key would make the two impossible to change
  independently.
- *Failing fast on an invalid value* — rejected: inconsistent with every other optional variable here,
  and it would turn a one-character deployment typo into an outage.
- *A `REQUIRE_EMAIL_VERIFICATION` name without the `AUTH_` prefix* — rejected: every auth setting in
  `env.go` is `AUTH_*`-prefixed (`AUTH_RATE_LIMIT_*`, `AUTH_OTP_*`, `AUTH_REFRESH_TTL`).

## R3. The session is issued by the same `signIn` and reached only through a verified challenge

**Decision**: `signIn` is not moved, split or duplicated. With the gate enabled, `Register` and
`Login` simply do not call it; the challenge domain's `VerifyChallenge` continues to call it after
consuming the challenge and recording the verification timestamp. With the gate disabled, `Register`
and `Login` call it exactly as today.

**Rationale**: `OTP-07` already establishes the session "exactly as the current sign-in path does",
including the timestamp semantics of D4, so the feature's only job is to remove the *other* two
callers. Keeping one issuer means the refresh family, the CSRF binding and the cookie contract cannot
drift between the legacy and gated paths — they are literally the same code.

**Alternatives considered**:

- *A new `signInVerified` that additionally asserts the timestamp* — rejected: `VerifyChallenge`
  records the timestamp before it signs in, so a second issuer would only re-derive a fact it already
  holds, and two issuers is exactly the drift the plan's "one implementation per rule" check forbids.
- *Making `signIn` itself refuse unverified accounts* — rejected: `Register` with the gate disabled
  must keep signing in an account that has no timestamp, so the check belongs at the call site
  (`Register`/`Login` under the gate) and in `Refresh`, not inside the shared tail.

## R4. The identity read reports a boolean, projected in one place, on a bearer-guarded route

**Decision**: `GET /auth/me` is mounted on the existing `/auth` group with the repository's existing
bearer middleware (`auth.Middleware(modules.Auth.TokenManager)`) applied per route, so the route sits
with its auth siblings while sharing the one token parser. The controller reads the account identity
the middleware resolved and asks the auth service for the profile; the service loads the account with
`user.Repository.FetchByID` and returns the existing `user.UserDTO`, which gains one derived field,
`email_verified bool`, set by `ToDTO` from `EmailVerifiedAt != nil`. The stored instant is never
projected.

**Rationale**: the bearer middleware already exists and is already applied to `/sheets` and
`/templates`, so reusing it costs nothing and keeps one token parser. Projecting the boolean inside
`ToDTO` means the read path and any future one cannot disagree, and it keeps `internal/auth` from
re-implementing a wire shape it does not own. Reporting the instant would leak when an account was
created-and-verified, which no requirement needs — the boolean is exactly the fact the client and
renewal act on.

**Alternatives considered**:

- *A dedicated `auth.ProfileResponse` DTO* — rejected: it would duplicate the user's wire shape and
  put a second definition of "a user on the wire" in the corpus, against `TFA-09`'s "one
  representation".
- *Placing the route behind a new middleware group* — rejected: Echo groups take middleware per route,
  so a second group for one route adds structure without adding safety.
- *Reporting `email_verified_at` as a timestamp* — rejected: unnecessary disclosure (`TFA-08`), and it
  would make the client depend on a clock rather than a state.
- *Answering `404` for an account deleted after the credential was minted* — rejected: the credential
  no longer names a usable account, so 401 (`UNAUTHORIZED`) is the honest outcome and matches
  renewal's treatment of the same case (`PRQ-05`).

## R5. The uniform sign-in spends the password work even when the address is unknown

**Decision**: when `FetchByEmail` reports no account, `Login` performs one bcrypt comparison against a
package-level dummy digest before answering the existing uniform `UNAUTHORIZED` outcome. The digest is
computed once (a fixed bcrypt hash at the repository's existing cost) and its result is discarded.

**Rationale**: the current code returns before the comparison on the unknown-address path, so the
response time falls into two distinct classes — the oracle finding F-05 names. Running the same
comparison for both cases makes the work identical without changing any status, body or message, and
the dummy digest cannot match any real password because no account stores it. `TFA-05` requires
exactly this, and the existing `invalidCredentials` tail remains the single producer of the outcome so
the two cases cannot diverge in the answer.

**Alternatives considered**:

- *Always hashing the submitted password* — rejected: hashing is far more expensive than comparing,
  so it would multiply the cost of every sign-in to equalize it, and the comparison already dominates
  the path.
- *Answering a shorter generic response and accepting the timing difference* — rejected: it is the
  defect being fixed.
- *Adding a random delay* — rejected: unbounded and untestable; the deterministic comparison is
  measurable in a test and costs nothing.

## R6. A registration whose challenge cannot be started answers the typed failure and stays recoverable

**Decision**: `Register` creates the account first, then starts the challenge through the existing
`IssueChallenge` path. If the daily allowance refuses the send (`EMAIL_UNAVAILABLE`, 503), or the
store fails (`INTERNAL_ERROR`, 500), the typed failure is returned with no session and no credential —
the account simply exists without a verification timestamp. The person recovers by signing in with the
same password once the allowance frees; if their `verify_email` challenge was never issued, the
`login_otp` verification records the timestamp anyway (`OTP-07`, D4).

**Rationale**: the ordering is the plan's (`docs/plan-auth-otp-2fa-email.md` §3.1 steps 2–5) and the
standalone MongoDB has no multi-document transaction (F-12), so the account and the intention cannot
be written atomically. Given that, the only two choices are "account first, failure after" and
"refuse before creating". The first keeps `IssueChallenge` as the single issuance path (bounds, cap
and observer live there) and the account's recoverability is guaranteed by the sign-in door, so the
failure is inconvenient rather than a dead end — which is what `TFA-13` requires.

**Alternatives considered**:

- *Pre-checking the daily allowance before creating the account* — rejected: it duplicates the cap
  policy outside `IssueChallenge`, and the check-then-write window would still be racy; the account
  would also be created on a path that never recorded the intent to email it.
- *Rolling the account back on failure* — rejected: a compensating delete is a second failure point on
  a standalone store, and deleting an account the person just created is worse than asking them to sign
  in.
- *Returning the conflict on the retry* (so the person cannot re-register) — rejected as the *only*
  path: it is why the sign-in door and the D4 timestamp rule must remain the recovery route, and the
  plan's §8 already records the "registered but unverifiable" account as a known risk handled by the
  same door.

## R7. The superseded rules are amended in their own specs, in place

**Decision**: the requirement bullets that no longer describe the shipped behaviour get a one-line
pointer appended — `AUTH-01`/`AUTH-02` (006) to `TFA-01`/`TFA-02`, `WAS-02` (007) to `TFA-02` with the
client work deferred to stage 5, and `PRQ-12`/`PRQ-14` (015) to `TFA-08`/`TFA-09` for the read path
only. The requirement IDs stay defined; nothing is deleted and no ID is renumbered.

**Rationale**: the constitution makes the corpus the single source of requirement IDs, and
`scripts/check_requirement_ids.py` requires every ID cited in source to resolve to a definition. The
delivered code will cite `TFA-*`; if the old bullets kept asserting "registration establishes the
session" with no marker, a reader (or the next feature) would take them as current. A pointer is the
smallest edit that keeps the resolution gate green and the corpus truthful, and it preserves the
history the earlier features' `validation.md` files cite.

**Alternatives considered**:

- *Editing the old bullets to describe the new behaviour* — rejected: it rewrites a delivered,
  validated feature's contract retroactively and would invalidate the citations its own validation
  makes.
- *Leaving them untouched and only superseding in the new spec* — rejected: a citation would then
  resolve to a statement the shipped code contradicts, which is the defect `TFA-14` names.
- *Deleting the superseded bullets* — rejected: it breaks `check_requirement_ids.py` for any source
  that still cites them and erases the record of what was delivered.

## R8. The suites that assume "register signs in" move with the gate, and the legacy path stays asserted

**Decision**: the affected suites (`internal/app/auth_test.go`, `auth_integration_test.go`,
`internal/auth/service_test.go` and the regenerated `mock_Service.go`) are updated in the same commit:
the existing cases run with the gate disabled (they keep asserting today's contract), and new cases
run with it enabled (the challenge answer, the absent cookie, the renewal refusal). The route
inventory test gains `/auth/me`.

**Rationale**: `TFA-12`/`SC-004` make "the legacy behaviour is unchanged" an outcome that must be
*measured*, not assumed — so the legacy cases are kept and exercised, not rewritten into the new
behaviour. Running the same fixtures against both settings is also what keeps the two paths from
silently converging: the flag is the only difference, so any other divergence fails a test.

**Alternatives considered**:

- *Rewriting the existing cases to the gated behaviour* — rejected: it would delete the evidence for
  `SC-004` and leave the legacy path untested on the day it is still the production default.
- *A build tag or a second test binary per setting* — rejected: the flag is already per-instance, so a
  table column is enough and keeps one binary under `go test -race`.
- *Leaving the suites untouched and adding only new ones* — rejected: they would fail against the
  changed service signature and the mock would not compile, so "in the same commit" is forced, not
  chosen.
