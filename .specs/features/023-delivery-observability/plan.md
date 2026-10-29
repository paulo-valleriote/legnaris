# Implementation Plan: Watching the Pipeline — The Alerts, the Dashboard and What to Do About Them

**Branch**: `023-delivery-observability` | **Date**: 2026-10-21 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/023-delivery-observability/spec.md`

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §4.2 (the provider's allowance), §5.9 (the metrics
and the alarms) and §9 stage 8. This feature is the observation slice of stage 6–8: it reads the
counters the preceding features expose and turns them into named alarms, one dashboard and one runbook
entry per alarm. It adds instrumentation nowhere, adds no service, and changes no application code.

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Watch the pipeline the earlier features built.

1. **Named alarms for every way it can stall.** The dead-letter queue holds something; nobody is
   listening to the work queue; the oldest message has waited too long; the backlog of unpublished
   codes is too large; the publication is failing; the provider is refusing; the daily allowance is
   spent; the verification failures are climbing. Each is a named rule with a severity, a patience
   period, a threshold and an expression over the series that carry the fact.
2. **One dashboard for the pipeline.** The codes requested and verified, the backlog, the queue's
   depth and consumers, the send outcomes, the provider's latency, the expired and dead-lettered
   counts — provisioned from the repository, every panel naming the series it reads.
3. **One runbook entry per alarm.** What it means, what to do, when to escalate — plus the provider's
   allowance, what happens when it is spent, and the trigger for buying more. The notification channel
   is the operator's to configure, and the runbook says where.

The change is confined to the deployment: the monitoring stack's rules file, one dashboard document,
the broker's metrics configuration, the runbook and the deployment's checks. **No application code
changes**, **no new service**, **no new dependency**.

## Technical Context

**Language/Version**: the deployment's own languages are YAML and JSON; the checks are Go 1.26.4
(`go.mod` `go 1.26.4`; `go version` confirms `go1.26.4 linux/amd64`) with `testify` v1.12.1 and
`gopkg.in/yaml.v3` v3.0.1 — the pins the repository already carries. The stack's images are pinned by
the repository: Prometheus `v3.1.0`, Grafana `11.4.0`, and the broker image the preceding feature pins.

**Primary Dependencies**: no new library. The rules are Prometheus's own rule format and the dashboard
is Grafana's own dashboard format; both are validated by the stack's own tooling and by the
deployment's checks.

**Storage**: none. The rules and the dashboard are documents; the monitoring stack's own time-series
store already holds the facts.

**Testing**: `go test ./...` and `go test -race ./...`. The deployment's suite parses the rules file
and the dashboard and asserts the contract: every alarm exists, each has a name, a severity, a
patience period and an expression, each expression names the series it watches, the thresholds are
configurable, and the runbook has an entry per alarm. The monitoring stack's own tooling
(`promtool check rules`) validates the expressions deeply and is run manually against the running
stack; the first implementation task verifies the broker's real series names against a running broker,
because those names are the broker's and not ours.

**Target Platform**: the deployment's Linux host, Docker Compose. The rules are evaluated by the
monitoring stack; the dashboard is served by the monitoring stack's dashboard host, which is already
behind the operator's route.

**Project Type**: deployment configuration plus the checks that guard it.

**Performance Goals**: none of its own. The broker's per-object metrics cost the broker a little; the
RabbitMQ documentation's guidance for a small deployment is that it is negligible, and the narrow
query for the oldest message keeps it that way (`research.md` R2, R3).

**Constraints**: no service is added and no notification channel is configured (`OBS-09`, `OBS-14`); no
credential appears in the rules or the dashboard, and the dashboard is reachable only through the
operator's route (`OBS-21`); the rules are validated by the gate so a broken one fails the build
(`OBS-19`, `OBS-22`); every rule, panel and runbook entry is English (`OBS-20`).

**Scale/Scope**: 3 user stories (1×P1, 2×P2), 22 requirements, 2 internal contracts; roughly 10
touched or created files under `deploy/`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | No new failure mode crosses a boundary: the feature reads counters and writes two documents. The one failure it can introduce — a rule that does not parse or names a series that does not exist — is caught by the monitoring stack's own tooling and by the deployment's checks, not swallowed (OBS-19). | PASS |
| II. Canonical Identifiers | No identifier is generated; the rules and the panels name series and queues, and the queues are the broker's code-defined names. | PASS (not involved) |
| III. Behavior Ships With Tests (NON-NEGOTIABLE) | The rules and the dashboard ship with the deployment's checks in the same commit: every alarm exists, each is named and has a severity, a patience period and a threshold, each expression names its series, the thresholds are configurable, and every alarm has a runbook entry — edge cases first (a rule that must exist, a threshold that must be configurable, an entry that must be present), then the happy path (OBS-22). | PASS |
| IV. English-Only Source | Every rule name, panel title and runbook entry is English (OBS-20). | PASS |
| V. Layered Architecture | No Go code changes; the deployment's checks read documents, exactly as they read the compose file and the edge configuration. The application's packages are untouched. | PASS (not involved) |
| VI. Security Gates Are Blocking (NON-NEGOTIABLE) | The feature is directly served by the principle: no credential appears in the rules or the dashboard, the dashboard stays behind the operator's route, and no gate is weakened. | PASS (directly served) |
| VII. Installable, Offline-Capable Web Client | No client surface changes; the PWA's contract and routes are untouched. | PASS (not involved) |
| Additional Constraints — Stack | Go with `shared/env` for configuration — unchanged; the feature adds no library and no service. The choice of the monitoring stack's own rule format and one provisioned dashboard is documented in `research.md` (R1, R5) and entered as an `AD-*` decision in `.specs/STATE.md` when the change lands. No principle is waived. | PASS |
| Additional Constraints — Spec corpus and traceability | `OBS-01`…`OBS-22` are defined in `spec.md`; the citations in the deployment's checks and in the runbook resolve against the corpus. The prefix `OBS` is exclusive to this feature. | PASS |
| Additional Constraints — Jira | The implementing PR must reference its `LGN-\d+` card; recorded for the PR, not claimed by this artifact. | PASS (noted) |
| Development Workflow | `make fmt && make lint && make vet && make lint-specs`, then `make test-race` covers the change: the deployment's checks are Go packages under `deploy/`. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps one implementation per rule:
each alarm exists once, in the rules file, and its entry in the runbook names it, so a rule cannot
exist without an action and an entry cannot describe an alarm that does not exist. Two choices that
could have drifted were resolved in research: the broker's per-object metrics are required because the
RabbitMQ documentation states that alerting is impossible with aggregation (R2), and the oldest
message's age is read through the broker's narrow detailed query rather than the full per-object
output (R3).

## Project Structure

### Documentation (this feature)

```text
.specs/features/023-delivery-observability/
├── spec.md                     # Requirements (OBS-*)
├── plan.md                     # This file
├── research.md                 # Phase 0 — decisions and rejected alternatives
├── data-model.md               # Phase 1 — the rules, the panels, the runbook entries
├── quickstart.md               # Phase 1 — runnable validation guide
├── contracts/                  # Phase 1 — one contract per seam
│   ├── alerts.contract.md
│   └── dashboard.contract.md
├── checklists/
│   └── requirements.md         # Spec quality checklist
└── tasks.md                    # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
deploy/
├── prometheus/rules.yml               # NEW: the named rules, their severities, their thresholds and their expressions (OBS-01…OBS-09)
├── prometheus/prometheus.yml          # EXTENDED: the rules are loaded; the broker's per-object endpoint is scraped (OBS-19, R2)
├── grafana/dashboards/email-pipeline.json  # NEW: the pipeline's dashboard, provisioned from the repository (OBS-10…OBS-15)
├── rabbitmq/rabbitmq.conf             # EXTENDED: the broker exposes its per-object metrics (OBS-02, OBS-03, R2)
├── README.md                          # EXTENDED: the runbook — one entry per alarm, the provider's allowance, where to add a channel (OBS-13…OBS-18)
├── compose_test.go                    # EXTENDED: every alarm exists, is named, has a threshold, names its series; the thresholds are configurable; the runbook has an entry per alarm (OBS-19, OBS-22)
└── traefik_test.go                    # EXTENDED: the dashboard's route stays behind the operator's gate (OBS-21)
```

**Structure Decision**: the rules live beside the monitoring stack's own configuration, because they
are read by the same process that holds the series and validated by the same gate. The dashboard lives
in the directory the stack already provisions for dashboards, and the runbook lives in the deployment's
README, beside the broker's provisioning and the gate's activation — one place an operator looks.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
