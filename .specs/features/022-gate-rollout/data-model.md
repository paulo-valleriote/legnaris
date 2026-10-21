# Phase 1 Data Model: The Rollout

**Feature**: `.specs/features/022-gate-rollout/` | **Date**: 2026-10-21

No persisted shape changes. The feature introduces one *configuration* shape (the setting's three
states) and reasons about one *existing* shape (the session family). The accounts, the challenges and
the refresh tokens are unchanged.

---

## 1. The setting's states

| State | What it means | A registration | A sign-in | A renewal of an account that never verified |
|---|---|---|---|---|
| `off` | the rule does not exist | a session | a session | renewed |
| `register` | a new account must prove its mailbox | a code step | a session | renewed |
| `all` | everybody must prove their mailbox | a code step | a code step | refused, the family revoked |

| Field | Type | Rules |
|---|---|---|
| the raw value | string | read from the environment; absent means `off`; blank means `off`; an unrecognised value stops the boot (ROL-01, ROL-06) |
| the parsed state | the mode | owned by `internal/auth`; three constants, no other value (ROL-01) |
| where it is read | once, at start-up | never re-read per request; a code step in flight continues (ROL-07) |
| where it is recorded | the start-up log | the state in force, so an operator can tell which stage a deployment is in (ROL-08) |

An account that verified its mailbox is never treated as unverified: the verification is a fact stored
on the account, not a stage of the rollout, so its renewal is never refused. The code asked at every
sign-in in the last state is the sign-in's second factor, not a repeated proof of the address (ROL-05).

---

## 2. The session family — what the cutover contains

| Field | Type | Rules |
|---|---|---|
| the family | the set of refresh credentials one sign-in created | revoked together, in one write, when the renewal is refused |
| the revocation | a flag on every token of the family | the rows stay until they expire; they are unusable from the moment they are revoked |
| the trigger | a renewal of an account with no verification timestamp | only in the `all` state (ROL-09) |

The cutover therefore needs no operator act: an account that comes back loses its session at its
first renewal, on every device at once (ROL-11, ROL-12).

---

## 3. The cutover — what it is and is not exact about

| Moment | What happens |
|---|---|
| the flip | the setting changes and the process restarts; the new state is in force from the first request |
| the access credential issued before the flip | still valid until it expires — the API never learns whether the account verified |
| the renewal issued before the flip | refused in the `all` state; the family is revoked |
| the person holding it | signs in again, receives a code, and is let in once the code verifies (ROL-10) |
| the code step in flight at the flip | continues and completes; the setting decides what a *new* request answers (ROL-07) |
| turning the setting back to `off` | the previous contract returns exactly; a verification that already happened is not undone (ROL-16) |

---

## 4. The counters the runbook names

| Counter | What it tells the operator |
|---|---|
| the codes issued, by purpose | how many people are being asked for a code |
| the code verifications, by result | how many of them made it through, and how many refused |
| the codes expired | how many asked too late |
| the daily cap refusals | whether the provider's free allowance is being reached |
| the renewal refusals | how many people were bounced by the cutover |

All of them exist on the server today; the alerts that read them arrive with the next feature.
