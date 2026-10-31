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

## Step by step

Both paths lift the rate limits, so use them on a throwaway environment only.

### A. Local (`make dev`) — k6, the DAG benchmark and optional dashboards

`make dev` runs MongoDB, the API and the web client only. `make
dev-observability` adds a dev Prometheus + Grafana beside it.

1. Requirements: Docker running, Go, and `make`.
2. Terminal 1 — start the stack with the limits lifted (the API inherits the
   exported variables):
   ```bash
   export AUTH_RATE_LIMIT_ATTEMPTS=100000 USER_RATE_LIMIT_ATTEMPTS=1000000 PUBLIC_RATE_LIMIT_ATTEMPTS=1000000
   make dev
   ```
   Wait for `[dev] api  http://localhost:8080`.
3. Terminal 2 — start the dashboards (optional) and open
   http://localhost:3000 (no login), dashboard **Legnaris — Load test (k6)**:
   ```bash
   make dev-observability
   ```
4. Terminal 2 — run the load test (defaults: 100 req/s, 2 minutes, 50 users):
   ```bash
   K6_OUT=experimental-prometheus-rw make loadtest   # omit K6_OUT for no dashboard feed
   # tune it: RATE=150 DURATION=5m USERS=1 make loadtest   (USERS=1 = one hot sheet)
   ```
5. Read the k6 summary. It exits non-zero when p95 >= 200 ms, the error rate
   is >= 0.1 % or any iteration was dropped.
6. Run the DAG benchmark (needs no stack): `make bench-dag`.
7. Ctrl+C in terminal 1 stops the API and removes the database; `make
   dev-observability-down` stops Prometheus and Grafana.

### B. Deployed stack (`make deploy-local`) — with live dashboards

`make deploy-local` brings up the full stack (Traefik, app, MongoDB,
Prometheus, Grafana, ...) through the Cloudflare Tunnel.

1. Requirements: Docker Compose >= 2.24 and a filled `deploy/.env`
   (`cp deploy/.env.example deploy/.env`, replace every `replace_me`).
2. Start the stack:
   ```bash
   make deploy-local
   ```
3. Apply the load-test overlay to the services it changes (limits lifted,
   Prometheus remote-write receiver):
   ```bash
   cd deploy
   F="-f docker-compose.yml -f docker-compose.local.yml -f docker-compose.loadtest.yml"
   docker compose $F up -d app prometheus
   ```
4. Open Grafana at `https://<OPS_DOMAIN>/grafana` (GitHub login, then the
   `GRAFANA_ADMIN_*` credentials) and pick the dashboard
   **Legnaris — Load test (k6)**; keep **Legnaris — API** in another tab.
5. Run k6 (it reaches the app on the internal network, bypassing Traefik):
   ```bash
   docker compose $F --profile loadtest run --rm k6
   ```
   Tune it with `LOADTEST_RATE`, `LOADTEST_DURATION` and `LOADTEST_USERS`.
6. In the dashboard, select the test in the `Test` variable. Compare
   throughput (target 100 req/s), p95 (< 200 ms), error rate (< 0.1 %), CPU,
   memory and MongoDB operations. Copy the numbers into
   `docs/loadtest/results.md`.
7. Restore the production limits when done:
   ```bash
   cd .. && make deploy-local
   ```


Contention on a single sheet: run with `USERS=1`, so every PATCH hits the same
sheet. The script needs `AUTH_REQUIRE_EMAIL_VERIFICATION=false` (the default),
since it expects a session from `/auth/register`. `make loadtest` uses
`--network host` (Linux only); users registered by a run are not removed.
Staging runs need the same limits lifted, so prefer a throwaway copy.
