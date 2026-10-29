# Quickstart: Validate Watching the Pipeline

**Feature**: `.specs/features/023-delivery-observability/` | **Date**: 2026-10-21

This feature ships no application behaviour. "Works" means: every way the pipeline can stall has a
named alarm, every alarm has a runbook entry, the dashboard shows the pipeline, and the monitoring
stack accepts all of it.

## Prerequisites

- Go 1.26.4 (`go version`).
- Docker for the optional end-to-end run (the monitoring stack's image carries its own rule
  validator).

## 1. The always-on gates

```bash
make fmt && make lint && make vet && make lint-specs   # linters + spec/test lint
make test-race                                         # the race detector must stay clean
```

Expected: clean. The deployment's checks parse the rules file and the dashboard; they need no
monitoring stack and no broker.

## 2. Prove the alarms exist and are complete

```bash
go test ./deploy/... -run 'Alerts|Rules' -v
```

Expected: every alarm exists, is named, carries a severity and a patience period, its expression names
the series it watches, the thresholds are configurable, and the runbook has an entry per alarm.

## 3. Prove the monitoring stack accepts the rules

```bash
docker run --rm --entrypoint promtool \
  -v "$PWD/deploy/prometheus/rules.yml:/etc/prometheus/rules.yml:ro" \
  prom/prometheus:v3.1.0 check rules /etc/prometheus/rules.yml
```

Expected: the rules are valid and the file parses. A rule the stack cannot evaluate fails here.

## 4. Prove the dashboard is a valid document

```bash
docker run --rm --entrypoint promtool --version   # the image is already pulled by the stack
python3 -c "import json;json.load(open('deploy/grafana/dashboards/email-pipeline.json'))"
```

Expected: the dashboard parses and the deployment's checks have already asserted that every panel names
its series.

## 5. Optional: the whole stack

```bash
docker compose -f deploy/docker-compose.yml up -d prometheus grafana
```

Expected: the monitoring stack starts with the rules loaded, the alarms appear in its rules view, and
the dashboard appears in the dashboard host with the pipeline's panels.

## References

- `contracts/alerts.contract.md` — the rules, their shapes and their boundary cases.
- `contracts/dashboard.contract.md` — the panels and the series they read.
- `data-model.md` — the counters, the rules, the panels and the runbook entries.
