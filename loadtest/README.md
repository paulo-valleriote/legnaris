# Load tests (LGN-33)

Validates the TDD performance requirements: **100 req/s sustained on
`PATCH /sheets/:id` with p95 < 200 ms and error rate < 0.1 %**, and a DAG
recalculation **p95 < 50 ms for templates of up to 200 fields**.

## DAG benchmark

```bash
make bench-dag
```

`backend/internal/sheet/template/dag_bench_test.go` holds the benchmarks and a
test (`TestRecalculateChain200_P95UnderBudget`) that fails if the worst-case
cascade (200 fields, 100 derived in one chain) exceeds 50 ms at p95.

## k6 PATCH test

`k6/patch-sheet.js` registers `USERS` accounts (one sheet each, the API allows
one per user), then fires `RATE` PATCH/s for `DURATION` with a
`constant-arrival-rate` executor. Thresholds mirror the acceptance criteria, so
k6 exits non-zero when they fail.

The default rate limits (80 req/min per user, 10 auth attempts/min per IP)
would refuse this traffic. **Only on a throwaway environment**, raise
`AUTH_RATE_LIMIT_ATTEMPTS`, `USER_RATE_LIMIT_ATTEMPTS` and
`PUBLIC_RATE_LIMIT_ATTEMPTS`.

### Against `make dev` / a local API

```bash
AUTH_RATE_LIMIT_ATTEMPTS=100000 USER_RATE_LIMIT_ATTEMPTS=1000000 \
PUBLIC_RATE_LIMIT_ATTEMPTS=1000000 make dev   # or make run
make loadtest                                 # RATE=100 DURATION=2m USERS=50
```

### Against the compose stack, with live dashboards

```bash
cd deploy
F="-f docker-compose.yml -f docker-compose.local.yml -f docker-compose.loadtest.yml"
docker compose $F up -d --build
docker compose $F --profile loadtest run --rm k6
```

The overlay lifts the limits, lets Prometheus accept remote writes and streams
k6's metrics there. Watch **Legnaris — Load test (k6)** (client and server view
side by side) and **Legnaris — API** (RED, Go runtime, MongoDB) in Grafana under
`/grafana`. Both dashboards are provisioned from `deploy/grafana/dashboards/`.

Knobs: `LOADTEST_RATE`, `LOADTEST_DURATION`, `LOADTEST_USERS`.

Contention on a single sheet: run with `USERS=1`, so every PATCH hits the same
sheet. The script needs `AUTH_REQUIRE_EMAIL_VERIFICATION=false` (the default),
since it expects a session from `/auth/register`. `make loadtest` uses
`--network host` (Linux only); users registered by a run are not removed.
Staging runs need the same limits lifted, so prefer a throwaway copy.
