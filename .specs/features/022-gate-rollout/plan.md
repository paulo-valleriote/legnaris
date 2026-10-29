# Implementation Plan: The Rollout — Turning the Rule On in Two Steps, and Knowing the Way Back

**Branch**: `022-gate-rollout` | **Date**: 2026-10-21 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/022-gate-rollout/spec.md`

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §9 stage 7 — the activation of the sign-in
hardening the fourth feature shipped dark. This feature delivers the staged setting, the containment
of the sessions issued before the cutover, and the operator's guide. It changes no client code and
delivers no operator command.

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Turn the rule on without locking anybody out, and leave the operator a way back.

1. **One setting, three states.** The delivered boolean becomes a mode: `off`, `register` and `all`.
   In the first state a registration asks for a code and a sign-in still establishes a session; in
   the last both ask for a code and a renewal of an account that never verified is refused. The mode
   is read once at start-up, an unknown value stops the process instead of guessing a stage, and the
   state in force is written to the log.
2. **The cutover contains itself.** The renewal refusal the fourth feature delivered already revokes
   the whole session family the first time it is presented, so the sessions issued before the cutover
   die on their own. No cleanup command is delivered — the runbook says why — and the runbook is
   honest about what the cutover is *not* exact about: the access credential issued before it keeps
   working until it expires.
3. **The operator's guide.** The runbook states the order of the two steps, the counters to watch
   between them, the window to wait, the consequences of each flip for the accounts that verified,
   the accounts that did not and the code steps in flight, and the way back.

The change is confined to the Go backend's configuration and sign-in policy, the deployment's
settings and runbook, and the corpus: the delivered feature's setting is marked as amended so the
corpus carries one contract. **No client change**, **no new dependency**, **no new required
environment variable**.

## Technical Context

**Language/Version**: Go 1.26.4 (`go.mod` `go 1.26.4`; `go version` confirms `go1.26.4 linux/amd64`) —
the toolchain the `Dockerfile` builds with. No new language and no new runtime.

**Primary Dependencies**: unchanged. The feature touches `shared/env`, `internal/auth`,
`internal/app`, `deploy/` and the spec corpus; it adds no library and no dependency.

**Storage**: unchanged. MongoDB 7.0 standalone; no new collection, no new index, no migration. The
feature reads and writes nothing new — it decides which of the delivered answers is returned.

**Testing**: `go test ./...` and `go test -race ./...`. The auth suites are table-driven with `t.Run`
and `Happy path:` / `Edge:` / `Error:` prefixes, `require` for preconditions and `assert` otherwise,
mockery mocks for the interfaces and ordinary fakes for the rest. The mode's three states are a table
in the auth suite: each state drives a registration, a sign-in and a renewal and asserts the answer.
The parse's failure path asserts the typed error and that the boot refuses to start. The deployment's
suite asserts the setting's default and its documented values. No test needs a database or a broker.

**Target Platform**: the Go backend on Linux. The setting is read by the request-serving process at
start-up; the flip is a configuration change and a restart.

**Project Type**: backend service plus a deployment setting; the web client is untouched.

**Performance Goals**: none. The mode replaces a boolean in three decision points; the cost is one
string comparison per decision.

**Constraints**: the setting is optional with a documented default of `off`, and the deployment's
required-variable list is unchanged (ROL-17); an unknown value stops the process with a typed error
(ROL-06); the delivered feature's setting is reconciled in the corpus, not silently overwritten
(ROL-18); the client is not touched (ROL-22); every source, runbook and log message is English
(ROL-21); `go test -race` stays clean (ROL-20).

**Scale/Scope**: 3 user stories (2×P1, 1×P2), 22 requirements, 2 internal contracts; roughly 12
touched or created files across `backend/shared/env`, `backend/internal/auth`, `backend/internal/app`,
`deploy/` and `.specs/features/018-auth-2fa-session-gate/`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The one new failure mode — a setting value nobody recognises — is a typed sentinel in `internal/auth` (`ErrUnknownVerificationMode`) with an exported message, and the boundary (the boot) acts on it exactly by refusing to start (ROL-06, ROL-19). It is a plain sentinel, not `apperr`, because it is a configuration failure with no HTTP status; the same precedent `shared/idgen.ErrInvalidID` sets. | PASS |
| II. Canonical Identifiers | No identifier is generated or changed; the feature reads and decides, and creates nothing. | PASS (not involved) |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | The mode's three states, the unknown value, the boundary between the states and the refusal that only applies in the last state all land with tests in the same commit, edge cases first (the unknown value, the state that must not apply the refusal, the account that verified) and then one happy path per state; failure paths assert the typed sentinel (ROL-20). | PASS |
| IV. English-Only Source | All identifiers, comments, log messages and the runbook are English (ROL-21). | PASS |
| V. Layered Architecture | `internal/auth` owns the mode and its parse — the policy belongs to the domain that decides it — and `shared/env` keeps to reading the environment; `internal/app` wires them. Nothing imports upward, and the client is untouched. | PASS |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | The feature makes the authentication contract stricter, never looser: the mode only ever adds a code step and a renewal refusal. No gate is weakened and no secret is introduced. | PASS (directly served) |
| VII. Installable, Offline-Capable Web Client | No client surface changes: the same two answers, in the same shapes, as the client already handles (ROL-22). The PWA's flows keep working unchanged. | PASS (not involved) |
| Additional Constraints — Stack | Go on Echo with MongoDB and `shared/env` for configuration — unchanged; the choice of a mode over two booleans is documented in `research.md` (R1) and entered as an `AD-*` decision in `.specs/STATE.md` when the change lands. No principle is waived. | PASS |
| Additional Constraints — Spec corpus and traceability | `ROL-01`…`ROL-22` are defined in `spec.md`; the delivered feature's superseded setting is marked with a pointer to this one, so the corpus carries one contract (ROL-18). The prefix `ROL` is exclusive to this feature. | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card; recorded for the PR, not claimed by this artifact. | PASS (noted) |
| Development Workflow | `make fmt && make lint && make vet && make lint-specs`, then `make test-race` covers the change: the auth and environment suites are Go packages, and the deployment's suite asserts the setting's values. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps one implementation per rule:
the mode's names and their parse live once, in `internal/auth`, and the environment surface carries
the raw value; the three decision points that consult the setting read the same parsed value, so the
registration, the sign-in and the renewal cannot disagree about which stage the deployment is in. One
choice that could have drifted was resolved in research: the mode's parse is strict where the
delivered boolean's lookup was lenient, because with three states the fallback direction is no longer
the safe one (R3).

## Project Structure

### Documentation (this feature)

```text
.specs/features/022-gate-rollout/
├── spec.md                     # Requirements (ROL-*)
├── plan.md                     # This file
├── research.md                 # Phase 0 — decisions and rejected alternatives
├── data-model.md               # Phase 1 — the setting's states, the cutover, the session family
├── quickstart.md               # Phase 1 — runnable validation guide
├── contracts/                  # Phase 1 — one contract per seam
│   ├── setting.contract.md
│   └── runbook.contract.md
├── checklists/
│   └── requirements.md         # Spec quality checklist
└── tasks.md                    # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
backend/internal/auth/
├── verification_mode.go        # NEW: the mode, its three states, its strict parse and the typed error (ROL-01…ROL-06, R3)
├── verification_mode_test.go   # NEW: every state, every unknown value, the blank value
├── service.go                  # EXTENDED: the three decision points read the mode instead of the boolean (ROL-02, ROL-03, ROL-05, ROL-09)
├── service_test.go             # EXTENDED: the three states drive a registration, a sign-in and a renewal
└── start.go                    # EXTENDED: the discriminated answer per state (ROL-04)

backend/shared/env/
├── env.go                      # EXTENDED: the setting is read as a raw value with the documented default (ROL-01, ROL-17)
└── env_test.go                 # EXTENDED: absent, blank and present values; RequiredKeys unchanged

backend/internal/app/
├── bootstrap.go                # EXTENDED: the strict parse, the typed boot failure, the state in the log (ROL-06, ROL-08, ROL-19)
└── bootstrap_test.go           # EXTENDED: the boot refuses an unknown value and logs the state

deploy/
├── .env.example                # EXTENDED: the setting's three documented values and what each does
├── docker-compose.yml          # EXTENDED: the setting's default is the off state
├── README.md                   # EXTENDED: the runbook — the order, the counters, the window, the consequences, the way back (ROL-13…ROL-16)
└── compose_test.go             # EXTENDED: the setting's default and its documented values

.specs/features/018-auth-2fa-session-gate/
└── spec.md                     # EXTENDED: the delivered setting is marked as amended and points here (ROL-18)

web/                            # UNTOUCHED: the same two answers, already handled (ROL-22)
```

**Structure Decision**: the mode and its parse live in `internal/auth`, beside the three decision
points that consult it — the mode is a policy of the sign-in domain, not a configuration format, and
the strictness belongs where an error can be returned. `shared/env` keeps its one job: reading the
environment and handing over the raw value with its documented default.

The runbook goes into the deployment's own README, beside the variables it explains and the broker's
provisioning it follows. A separate document would be one more place an operator has to look.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
