# Quickstart: Validate The Rollout

**Feature**: `.specs/features/022-gate-rollout/` | **Date**: 2026-10-21

This feature ships no new endpoint. "Works" means: the setting has exactly three states, each state
answers what it says it answers, an unknown value stops the process, and the operator's runbook says
everything the activation needs.

## Prerequisites

- Go 1.26.4 (`go version`).
- No environment variable is required for the unit and race gates.

## 1. The always-on gates

```bash
make fmt && make lint && make vet && make lint-specs   # linters + spec/test lint
make test-race                                         # the race detector must stay clean
```

Expected: clean. No test needs a database or a broker.

## 2. Prove the three states

```bash
go test ./internal/auth/... -run 'VerificationMode|Service' -v
```

Expected, per state:

- **off** — a registration and a sign-in both establish a session; a renewal of an account that never
  verified is renewed.
- **register** — a registration starts a code step and issues no session; a sign-in still establishes a
  session; a renewal is renewed.
- **all** — a registration and a sign-in both start a code step; a renewal of an account that never
  verified is refused and its family revoked; the renewal of an account that verified succeeds.

## 3. Prove the unknown value stops the boot

```bash
go test ./internal/app/... -run Bootstrap -v
AUTH_REQUIRE_EMAIL_VERIFICATION=enabled go run ./backend/cmd/api.go   # the manual check
```

Expected: the typed error and the refusal to start. The manual run must not reach the point of
serving a request.

## 4. Prove the deployment's default

```bash
go test ./deploy/... -v
```

Expected: the setting's default is the off state, the documented values are the three states, and the
required-variable list is unchanged.

## 5. Prove the runbook is complete

```bash
go test ./deploy/... -run Runbook -v
```

Expected: the runbook carries the order of the two steps, the counters to watch, the consequences of
each flip and the way back.

## References

- `contracts/setting.contract.md` — the three states and the boundary cases.
- `contracts/runbook.contract.md` — the order, the counters, the consequences and the way back.
- `data-model.md` — the states, the session family and the cutover.
