# Contract: The setting

**Feature**: `.specs/features/022-gate-rollout/` | **Requirement IDs**: `ROL-01` … `ROL-09`, `ROL-17` |
**Owner**: `internal/auth` (the mode and its parse), `shared/env` (the raw value)

## Purpose

The one setting that decides in which stage of the rollout a deployment is, and exactly what each
stage answers.

## The states

| State | A registration | A sign-in | A renewal of an account that never verified |
|---|---|---|---|
| `off` | a session | a session | renewed |
| `register` | a code step | a session | renewed |
| `all` | a code step | a code step | refused, and the family revoked |

## Rules

- **Three states, no more.** An absent or blank value means `off`; a value the setting does not name
  stops the process at start-up with a typed error (ROL-01, ROL-06).
- **The strictness is the point.** With three states there is no safe fallback: a typo meant to be
  `all` must not deploy as `off`. The delivered boolean's lenient lookup — an unparseable value falls
  back to its default — is deliberately not inherited (`research.md` R3).
- **The mode is read once.** A change takes effect on the next start, never halfway through a request;
  a code step already in flight continues and completes (ROL-07).
- **The state in force is logged at start-up**, so an operator can tell which stage a running
  deployment is in without reading its configuration (ROL-08).
- **The setting is optional** with a documented default of `off`, and the deployment's
  required-variable list is unchanged (ROL-17).
- **The client is never told.** The answers are the two the client already handles, in both the
  registration and the sign-in operation (ROL-04, ROL-22).

## Boundary cases

| Case | Result |
|---|---|
| the value is absent | `off` |
| the value is blank | `off` |
| the value is `off`, `register` or `all` | that state |
| the value is `true` or `false` | unknown — the process refuses to start |
| the value is `All` or `ALL` | unknown — the states are lower case, and a guess is not a state |
| the account verified its mailbox | it is never treated as unverified: its renewal is renewed in every state; a sign-in in the `all` state still asks for a code |
| the account never verified, in the `register` state | its renewal is renewed — the rule does not apply to sign-in yet |
| the account never verified, in the `all` state | its renewal is refused and its family revoked |

## Test evidence

- `backend/internal/auth/verification_mode_test.go`: every state, every unknown value, the blank
  value, and the typed error.
- `backend/internal/auth/service_test.go`: the three states drive a registration, a sign-in and a
  renewal.
- `backend/shared/env/env_test.go`: the raw value's absent, blank and present cases; the
  required-variable list unchanged.
- `backend/internal/app/bootstrap_test.go`: the boot refuses an unknown value and records the state.
