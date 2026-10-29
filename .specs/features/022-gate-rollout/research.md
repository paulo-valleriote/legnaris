# Research: The Rollout

**Feature**: `.specs/features/022-gate-rollout/` | **Requirements**: `ROL-01` … `ROL-22` | **Date**:
2026-10-21

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §9 stage 7 — the activation of the rule the fourth
feature shipped dark.

Every decision below was checked against the code the preceding features delivered.

## R1. One setting with three states, replacing the delivered boolean

**Decision**: the setting becomes a mode with exactly three states — `off`, `register` and `all` — and
it replaces the boolean the fourth feature shipped.

**Rationale**: the plan's stage 7 is a sequence: "ligar para contas novas" and "Depois, endurecer o
login para todos". A boolean cannot express a sequence — it is either on or off, and with it on, both
a registration and a sign-in start a code step. Two independent booleans could express the two steps,
but then the setting carries two facts and a precedence rule the operator has to remember, and a
mistaken combination (both off, but a renewal refusing) is representable.

Replacing the boolean is safe: the rule ships dark and no deployment has ever had it on, so there is
no live configuration to break. The fourth feature's own spec anticipated this — "the ordered rollout
and the operational flip remain the plan's later stage, which may amend the setting if a cohort is
required" — and its decision record names the amendment as the way forward.

**Alternatives considered**:

- *Keep the boolean and add a second one* — rejected: two switches with a precedence rule is the
  classic way to end up in a state nobody designed.
- *A percentage cohort* — rejected: the plan's order is a sequence of two steps, not a fraction of the
  users; a cohort would also make the rule's state unobservable from the outside.
- *A date cutover ("accounts created after X")* — rejected: the first step is about *which operation*
  needs the code, not about which accounts exist; the mode expresses the plan's sentence directly.

## R2. The state's names say what they require

**Decision**: the three states are `off`, `register` and `all` — the last one because the sign-in step
implies the registration step.

**Rationale**: a state named after the *operation* it changes (`register`) reads as "a registration
asks for a code", which is exactly what it does; `all` reads as "everything asks for a code". A state
named after the *audience* ("new accounts") would promise a cohort the setting does not implement —
the mode changes what a registration answers, not who is affected.

**Alternatives considered**:

- *`off` / `registration` / `sign_in`* — rejected: `sign_in` alone would read as "only sign-in", while
  it means sign-in *and* registration.

## R3. The state's value is read once, and an unknown value stops the process

**Decision**: the value is parsed once at start-up; an absent value means `off`; a value the setting
does not name stops the process with a typed error; the state in force is written to the log at
start-up.

**Rationale**: a start-up failure on an unknown value is the difference between "the deployment is in a
stage nobody designed" and "the deployment quietly behaves like the last stage that worked". The
logged state answers the operator's first question — which stage is this deployment in — without
reading the configuration.

The delivered boolean's own fallback is deliberately **not** inherited: it reads an unparseable value
as its default and logs a warning, because with two states the fallback direction is the safe one — a
typo disables the rule instead of enabling it. With three states the fallback is no longer safe: a
typo meant to be `all` would silently deploy as `off`, which is the one state nobody wants by
accident. The strict parse therefore lives with the policy it belongs to, where an error can be
returned, and the shared configuration surface exposes the raw value.

**Alternatives considered**:

- *Fall back to `off` on an unknown value* — rejected: it would silently turn the rule off in a
  deployment that meant to have it on.
- *Keep the delivered boolean's lenient lookup* — rejected: with three states there is no safe
  fallback; see above.
- *Re-read the value per request* — rejected: a rule that changes halfway through a request is a rule
  nobody can reason about; the flip is a start-up act.

## R4. The renewal refusal is the cutover's containment, and it needs no cleanup

**Decision**: the feature delivers no command to clean the stored sessions, and the runbook states why.

**Rationale**: the fourth feature's renewal refusal already revokes the *whole family* of the
presented credential when the account has no verification timestamp — the repository's
`RevokeFamily` marks every token of the family revoked in one write. So an account that holds a
session issued before the cutover loses it at its first renewal attempt, on every device at once,
without an operator touching anything. The rows that remain are marked revoked and expire on their
own; a cleanup command would only reach the accounts that never come back.

**Alternatives considered**:

- *An operator command that revokes the families of the never-verified accounts* — rejected by the
  user's decision (2026-10-21) and by the arithmetic above: it buys nothing for the accounts that
  matter and adds a command to maintain.
- *Deleting the revoked rows* — rejected: the rows are the evidence of a reuse attempt, and the
  repository marks rather than deletes for that reason.

## R5. The cutover is exact at the renewal, not at the access credential

**Decision**: the runbook states that a session issued before the cutover keeps working until its
access credential expires, and that the renewal is where the cutover bites.

**Rationale**: the access credential is signed and short-lived; the API validates it and never learns
whether the account verified. The renewal is the point where the rule is applied, because that is
where the account is loaded. Saying this plainly is better than an operator discovering it.

**Alternatives considered**:

- *Check the verification state on every request* — rejected: it would put a store read in front of
  every request, and the plan's decision O5 deliberately puts the check on the renewal.

## R6. The client does not change

**Decision**: the web client is not touched.

**Rationale**: the two answers the setting produces — a session or a code step — are exactly the two
the fifth feature taught the client to handle, in both the registration and the sign-in operation.
The setting is a server-side policy and the client is not told about it.

**Alternatives considered**:

- *Telling the client which state is in force* — rejected: it would leak a server policy to the
  browser and change the contract for no benefit.

## R7. The delivered feature is reconciled, not silently overwritten

**Decision**: the fourth feature's specification records that its setting's shape was amended here,
and this feature's spec names it.

**Rationale**: the corpus is the single source of requirement IDs and of contracts. The fourth
feature's `TFA-07` describes a boolean; this feature replaces it with a mode. A reader arriving at
either feature must find one contract, so the superseded requirement is marked as such with a pointer
to the one that replaced it — the pattern the repository already uses when a feature supersedes
another's premise.

**Alternatives considered**:

- *Editing the fourth feature's requirement in place* — rejected: the repository keeps the history of
  what was decided and when; a superseded requirement is marked, not rewritten.

## R8. Deliberately out of this feature

**Decision**: the following stay out:

| Out | Reason |
|---|---|
| the alerts and dashboards that watch the counters | the next feature; the runbook names the counters that exist today |
| the identity read the client never consumed | no screen needs the verification state in this stage |
| resetting a password, changing an address | outside the program |
| a per-cohort rollout | the plan's order is a sequence of two steps (R1) |
| revoking every session at the cutover | it would sign out the accounts that verified for no reason |

## Resolved unknowns

| Technical Context item | Resolution |
|---|---|
| The setting's shape | one mode with three states, replacing the delivered boolean (`R1`, `R2`) |
| How the state is read | once at start-up; unknown values stop the process; the state is logged (`R3`) |
| The cleanup | not delivered; the renewal's revocation is the containment (`R4`) |
| What the cutover is exact about | the renewal, not the access credential (`R5`) |
| The client | untouched (`R6`) |
| The corpus | the delivered feature's setting is marked as amended and points here (`R7`) |
| Deferred scope | alerts and dashboards, the identity read, password reset, cohorts, a mass revocation (`R8`) |
