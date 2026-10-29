# Contract: The broker's provisioning

**Feature**: `.specs/features/021-broker-stack/` | **Requirement IDs**: `BRK-09`, `BRK-10`, `BRK-11`,
`BRK-16`, `BRK-17` | **Owner**: `deploy/README.md`

## Purpose

What the operator runs, once per broker volume, to give the broker its delivery vhost and its two
accounts — and what each command must produce.

## The steps

In this order, against the running broker:

1. **Create the delivery vhost** — the place where the code queues live.
2. **Create the operator's account** — a distinct account, with the console right, and with
   permissions on the delivery vhost so it can see the queues.
3. **Narrow the application's account** — remove the right it was created with, so it can never open
   the console, and grant it exactly the delivery vhost.

The passwords come from the environment. The application's account already exists — the broker's
image creates it from the environment — so its password is never re-entered; only its rights are
narrowed.

## The rules

- **Every step is idempotent.** Re-running the provisioning on an already-provisioned broker changes
  nothing and fails nothing, so the runbook can be re-run after a restart, after a version change, or
  after a mistake.
- **No credential is committed.** The passwords are read from the environment at the moment the
  command runs; the repository carries the commands, never a secret (`BRK-11`).
- **The operator's account is a different account**, never a second tag on the application's
  (`BRK-10`).
- **The application's account ends with no console right** and with permissions on the delivery vhost
  only — so it can publish and consume the codes and can neither read the console nor see any other
  vhost (`BRK-09`, `BRK-17`).
- **The provisioning is a visible operator act.** It is documented, it is run by a person, and it is
  not performed silently by the stack on every boot (the user's decision, 2026-10-21).

## The consequence the runbook must state

The pipeline must not be deployed before the provisioning has run: an unprovisioned broker has no
delivery vhost, so the publisher and the worker cannot connect to it. The steps must be re-run after
the broker's volume is replaced.

## Boundary cases

| Case | Result |
|---|---|
| the provisioning is re-run on a provisioned broker | no change, no failure |
| the operator's account already exists | the command updates its rights and fails nothing |
| the application's account is left with the right it was created with | prohibited: it could open the console |
| the operator's password is missing from the environment | the command is not run; the application keeps working |
| the broker's volume is replaced | the provisioning must be re-run; the runbook says so |

## Test evidence

- `deploy/compose_test.go`: the runbook carries the provisioning steps, and the operator's password
  never reaches a service.
- The provisioning itself is exercised by the operator during the deploy; it is not a test.
