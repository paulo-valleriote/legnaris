# Contract: The dashboard

**Feature**: `.specs/features/023-delivery-observability/` | **Requirement IDs**: `OBS-10` … `OBS-15`,
`OBS-21` | **Owner**: `deploy/grafana/dashboards/`

## Purpose

One screen that tells the operator the shape of the pipeline, so an alarm has something to be read
against.

## The panels

| Panel | Reads | Shows |
|---|---|---|
| the codes requested and verified | the site's issuance and verification counters | how many people are being asked and how many made it |
| the backlog of unpublished codes | the publisher's backlog gauge | how many codes are waiting to leave |
| the outcomes of every delivery | the worker's send outcomes | sent, retried, dead-lettered, expired |
| the provider's answer time | the worker's duration measure | how long the provider takes |
| the expired and dead-lettered counts | the worker's counters | how many codes were late and how many were lost |
| the queue's depth and its consumers | the broker's queue series | how much is waiting and whether anybody is listening |
| the oldest message's age | the broker's head-message timestamp | how long the oldest code has been waiting |
| the daily allowance | the site's cap refusals | how close the ceiling is |

## Rules

- **One dashboard, provisioned from the repository.** No manual import, no editing in the browser
  (OBS-10).
- **Every panel names the series it reads.** A panel whose series stops arriving is an empty panel,
  not a silent zero (OBS-15).
- **The dashboard carries no credential** and no code, and it is reachable only through the operator's
  route (OBS-21).
- **The panels read the process that owns the counter**, exactly as the rules do, so the dashboard and
  the alarms never disagree (OBS-16).

## Boundary cases

| Case | Result |
|---|---|
| the worker has never run | its panels are empty; the site's and the publisher's are not |
| the broker's per-object metrics are unavailable | the queue panels are empty rather than wrong |
| the dashboard is opened without the operator's sign-in | the operator's route answers first |

## Test evidence

- `deploy/compose_test.go`: the dashboard exists, is a valid document, names the series each panel
  reads, and covers the pipeline's stages.
- `deploy/traefik_test.go`: the dashboard's route keeps the operator's gate.
