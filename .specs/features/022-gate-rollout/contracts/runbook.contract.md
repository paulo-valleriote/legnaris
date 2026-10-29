# Contract: The runbook

**Feature**: `.specs/features/022-gate-rollout/` | **Requirement IDs**: `ROL-11` … `ROL-16` | **Owner**:
`deploy/README.md`

## Purpose

What the operator reads at night, before turning the rule on, between the two steps, and if they have
to turn it back off.

## The order

1. **Deploy with the rule off.** The machinery is in place and nothing changes for anybody.
2. **Move to `register`.** Every new account must prove its mailbox; every existing account still
   signs in with a password. Watch the counters for a window.
3. **Move to `all`.** Everybody must prove their mailbox, and the accounts that never did can no
   longer renew.

## What to watch

| Between the steps | What a bad number means |
|---|---|
| the codes issued, by purpose | how many people are being asked for a code |
| the code verifications, by result | how many refused — a rising refusal rate means the delivery or the wording is wrong |
| the codes expired | people asking too late — the queue or the provider is slow |
| the daily cap refusals | the provider's free allowance is being reached |
| the renewal refusals | how many people the cutover bounced |

The window between the two steps is the operator's choice; the runbook states that the second step
waits for a quiet period, not for a number, and that the alerts which watch these arrive with the
next feature.

## The consequences of each flip

| At the flip to `register` | At the flip to `all` |
|---|---|
| a new account gets a code instead of a session | everybody gets a code instead of a session |
| an existing account signs in exactly as before | an account that never verified is refused at its renewal and signs in again with a code |
| an account that verified keeps signing in with a password | an account that verified signs in with a code like everybody else, and its renewal is renewed |
| a code step in flight continues | a code step in flight continues |
| the renewal is not refused | the renewal is refused and the session's whole family ends at once |

## The way back

- Setting the value back to `off` and restarting restores the previous contract exactly.
- Nothing is undone: an account that verified stays verified, and the codes that were issued stay
  expired.
- A person caught mid-step loses the pending code and starts again; the client keeps nothing between
  reloads.

## What the runbook does not tell the operator to do

- **No cleanup of the stored sessions.** The renewal's refusal revokes the family the first time the
  credential is presented, so an active account's session dies by itself; the rows that remain are
  already revoked and expire on their own. A command to clean them would only reach the accounts
  that never come back (ROL-11, ROL-12).

## Boundary cases

| Case | Result |
|---|---|
| the flip happens while a code step is in flight | the step completes; the setting decides what a new request answers |
| the rule is turned back off | the previous contract returns; nothing is undone |
| the rule is turned back on | the sessions already issued are not revoked; an account that never verified meets the rule at its next renewal |
| the operator cannot reach the counters | the flip waits — the runbook says which numbers to look at and where |

## Test evidence

- `deploy/compose_test.go`: the runbook carries the order, the counters, the consequences and the way
  back, and the deployment's documented values of the setting.
