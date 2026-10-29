# Implementation Plan: The Broker Stack — The Delivery Worker Runs, and the Broker's Console Is the Operator's

**Branch**: `021-broker-stack` | **Date**: 2026-10-21 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/021-broker-stack/spec.md`

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §5.8 (deploy in the current stack), §5.9
(observability) and §9 stage 6 — decisions D6 (RabbitMQ), O2 (separate processes), O6 (the management
UI behind `ops.`). This feature is the deployment slice of stage 6: the delivery worker joins the
stack, the broker's console is put behind the operator's gate, the broker's accounts are separated by
role, and the broker's own metrics become readable from inside the machine. It deliberately ships no
alert rule and no dashboard — the next feature reads what this one makes reachable.

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Deploy the pipeline the last three features built, and put the broker's console where it belongs.

1. **The delivery worker runs.** A `worker` service joins the stack beside the publisher: the same
   image, its own entrypoint, the internal network only, no published port, waiting for the broker's
   health check. It needs no database, and the site's own start-up does not wait for either the broker
   or the worker — a broker outage delays emails and nothing else.
2. **The console is the operator's.** The broker's console is served under its own path prefix on the
   operator's domain, behind the same sign-in gate and the same security headers as the three consoles
   already there, and it is never published on a host port. The broker stays on the internal network, which
   the edge router already shares, so the route needs no wider reach.
3. **The accounts are separated by role.** The application's account can move code in and out of the
   delivery vhost and can never open the console; the operator's account is a distinct one that can.
   The accounts and the vhost are provisioned by documented operator commands, so no broker credential
   ever enters the repository.
4. **The broker's state is readable.** The broker's Prometheus plugin is enabled in a small derived
   image — on before the broker starts, never while it is starting — and the monitoring stack scrapes
   the broker and the worker beside the ones it already scrapes.

The change is confined to the deployment: `deploy/docker-compose.yml`, `deploy/traefik/dynamic.yml`,
`deploy/prometheus/prometheus.yml`, `deploy/.env.example`, a new derived image for the broker, the
deployment's runbook and its Go test suite. **No backend code changes** and **no new required
environment variable**.

## Technical Context

**Language/Version**: the deployment's own language is Docker Compose and YAML; the checks are Go
1.26.4 (`go.mod` `go 1.26.4`; `go version` confirms `go1.26.4 linux/amd64`) with `testify` v1.12.1 and
`gopkg.in/yaml.v3` v3.0.1 — the pins the repository already carries. The stack's images are pinned by
the repository: Traefik `v3.3`, Prometheus `v3.1.0`, Grafana `11.4.0`, Loki `3.3.2`, Alloy `v1.17.1`,
MongoDB `7.0`, `oauth2-proxy v7.8.0`.

**Primary Dependencies**: no new library. The broker's image moves from the floating `rabbitmq:4-management`
to an exact 4.x minor — the RabbitMQ documentation's current release series is 4.4, and the plan
requires the minor to be pinned at deploy time (`research.md` R5). The broker's Prometheus plugin ships
with the management image and is enabled by the derived image (`R6`).

**Storage**: the broker's existing `rabbitmq_data` volume is unchanged; it now also holds the delivery
vhost. No new volume, no new collection. The stack's data store (MongoDB) is untouched.

**Testing**: `go test ./...` and `go test -race ./...`. The deployment's suite (`deploy/compose_test.go`,
`deploy/traefik_test.go`) is the test surface: it parses the compose file and the edge configuration
and asserts the contract. The suites are table-driven with `t.Run` and `Happy path:` / `Edge:` /
`Error:` prefixes, `require` for preconditions and `assert` otherwise. The continuous integration gate
runs `docker compose config` over the base and the local override with `.env.example`, so every
variable this feature adds must interpolate from the example file. No test needs Docker, MongoDB or a
broker.

**Target Platform**: the deployment's Linux host, Docker Compose. The broker, the publisher and the
worker live on the internal network; the site's edge router reaches the broker through the internal network
and is the only component that routes to its console.

**Project Type**: deployment configuration plus the checks that guard it. The Go backend is untouched.

**Performance Goals**: none. The feature adds one service that consumes nothing and one scrape
target; the console's route adds a page load on the operator's domain.

**Constraints**: the stack's required-variable list is unchanged and the request-serving process keeps
receiving neither the broker's address nor the worker's settings (`BRK-04`, `BRK-21`); no broker
credential is committed (`BRK-11`); the console is reachable only through the operator's route
(`BRK-07`); the broker publishes no host port for anything (`BRK-05`, `BRK-12`); the broker's version
is pinned (`BRK-06`); every source, runbook and configuration string is English (`BRK-19`).

**Scale/Scope**: 4 user stories (3×P1, 1×P2), 22 requirements, 2 internal contracts; roughly 12
touched or created files under `deploy/`, plus the deployment's two test files.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | No new failure mode crosses a boundary: the stack's failure modes are the compose gate's interpolation errors (a missing required variable fails `docker compose config`, which the existing required-variable test already covers) and the broker's own errors. The deployment's checks assert the typed outcome (a missing service, a published port, an absent gate) with `require`/`assert`. | PASS |
| II. Canonical Identifiers | No identifier is generated by this feature: the delivery vhost, the account names and the path prefix are code-defined names, not server-generated entity IDs, and they are stable strings read by the stack. | PASS (not involved) |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | The stack's behaviour ships with the deployment's Go suite in the same commit: the worker's service and exposure, the broker's exposure and pinned version, the console's route and its gate, the accounts' separation and the scrape targets, each asserted edge case first (a port that must not exist, a gate that must not be missing) and then the happy path. The suite runs under the race detector like the rest (`BRK-15`, `BRK-22`). | PASS |
| IV. English-Only Source | All identifiers, comments, configuration comments and the runbook are English (`BRK-19`). | PASS |
| V. Layered Architecture | No Go code changes; the deployment's checks import only the shared configuration surface (`backend/shared/env`) and the YAML parser, exactly as they do today. The backend's packages are untouched. | PASS (not involved) |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | The console carries the queued codes in the clear, so this feature is directly served by the principle: the console is behind the operator's sign-in gate and the security headers, no host port is published, the application's account has no console right, and no credential is committed. No gate is weakened and no secret scan is turned off. | PASS (directly served) |
| VII. Installable, Offline-Capable Web Client | No client surface changes; the PWA's contract and routes are untouched. | PASS (not involved) |
| Additional Constraints — Stack | Go on Echo with MongoDB and `shared/env` for configuration — unchanged. This feature changes only the deployment, and the one new artefact (the derived broker image) is documented in `research.md` (R6). No principle is waived. | PASS |
| Additional Constraints — Spec corpus and traceability | `BRK-01`…`BRK-22` are defined in `spec.md`; the citations in the deployment's checks and in the runbook resolve against the corpus. The prefix `BRK` is exclusive to this feature. | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card; recorded for the PR, not claimed by this artifact. | PASS (noted) |
| Development Workflow | `make fmt && make lint && make vet && make lint-specs`, then `make test-race` covers the change: the deployment's suites are Go packages under `deploy/`, so the backend sequence runs them. The continuous integration gate's `docker compose config` step covers the interpolation of the new variables. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps one implementation per rule:
the broker's account rights are stated once — in the runbook's provisioning commands and in the
checks that assert the runbook carries them — and the console's exposure is stated once, in the edge
configuration and in the check that reads it. Two choices that could have drifted were resolved in
research: the accounts are provisioned by operator commands rather than by a definitions file (R2),
which keeps every credential out of the repository and makes the provisioning a visible act, and the
metrics plugin is enabled at build time rather than at run time (R6), which removes the race between
the broker's first start and its health check.

## Project Structure

### Documentation (this feature)

```text
.specs/features/021-broker-stack/
├── spec.md                     # Requirements (BRK-*)
├── plan.md                     # This file
├── research.md                 # Phase 0 — decisions and rejected alternatives
├── data-model.md               # Phase 1 — the stack's services, routes, accounts and targets
├── quickstart.md               # Phase 1 — runnable validation guide
├── contracts/                  # Phase 1 — one contract per seam
│   ├── stack.contract.md
│   └── provisioning.contract.md
├── checklists/
│   └── requirements.md         # Spec quality checklist
└── tasks.md                    # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
deploy/
├── docker-compose.yml                  # EXTENDED: the worker service, the broker's derived image, the edge join, the pinned tag (BRK-01…BRK-06)
├── .env.example                        # EXTENDED: the operator's broker password, the delivery vhost in the broker's address
├── prometheus/prometheus.yml           # EXTENDED: the broker's and the worker's scrape targets (BRK-13)
├── traefik/dynamic.yml                 # EXTENDED: the console's route on the operator's domain, behind the existing gate (BRK-07, BRK-08)
├── traefik/rabbitmq.conf               # NEW: the broker's configuration — the console's path prefix (BRK-07)
├── rabbitmq/Dockerfile                 # NEW: the derived image that enables the broker's metrics plugin (BRK-14)
├── rabbitmq/README.md                  # NEW: how the derived image is built and why the plugin is enabled there
├── README.md                           # EXTENDED: the provisioning commands, the console's address, the stall procedure (BRK-16)
├── compose_test.go                     # EXTENDED: the worker's service and exposure, the broker's exposure and pin, the accounts, the scrape targets (BRK-15)
└── traefik_test.go                     # EXTENDED: the console's route, its rule, its middlewares (BRK-15)
```

**Structure Decision**: the broker's configuration lives in one file (`deploy/rabbitmq/rabbitmq.conf`)
mounted into the broker's service, next to the derived image that enables the plugin, so the two things
that make the broker behave differently from the stock image sit together and are read together. The
edge route stays in the shared dynamic configuration with the other operator routes — the console is
the fourth of a known kind, and giving it its own file would make the operator's surface harder to
read, not easier.

The checks stay where they are: the stack's contract is asserted by the deployment's Go suite, beside
the checks that already guard the broker's exposure and the publisher's separation. A new test file
would fragment one contract across two places.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
