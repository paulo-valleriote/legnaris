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

`k6/patch-sheet.js` signs in `USERS` seeded accounts (one sheet each, the API
allows one per user), then fires `RATE` PATCH/s for `DURATION` with a
`constant-arrival-rate` executor. Thresholds mirror the acceptance criteria, so
k6 exits non-zero when they fail.

### Capacity ramp (`capacity.js`)

`patch-sheet.js` only answers "does 100 req/s pass?". `capacity.js` answers
"where does it break?": a `ramping-arrival-rate` run climbs through
`STAGES` (default `100,200,400,800,1600` req/s, `STEP_SECONDS`=60 each) over a
weighted mix: 55 % `GET /sheets/:id`, 30 % single-field `PATCH`, 15 % `PATCH`
of four fields including `identity.proficiencyBonus`, which cascades through
every skill and save (`MIX_GET`, `MIX_PATCH`, `MIX_CASCADE`).

Every request carries a `step` tag and every step its own thresholds: the run
aborts as soon as a step breaches **p95 > `P95_BUDGET_MS`** (200) or **> 1 %
failures**, so **the last step that did not abort is the measured capacity**.
Per-operation lines (GET p95 < 100 ms, PATCH p95 < 200 ms, errors < 0.1 %)
report without aborting.

```bash
SCRIPT=capacity.js make loadtest-dev
SCRIPT=capacity.js STAGES=50,100,200 STEP_SECONDS=30 USERS=20 make loadtest-dev
SCRIPT=capacity.js make loadtest            # against a running API
```

On a shared host (k6, API and MongoDB on one machine) the generator competes with
the API for CPU: a high `legnaris_http_in_flight_requests` with flat server
throughput means the API is saturated, a high `VUs` count with a low in-flight
means the generator is. Trust capacity numbers only from the `deploy` stack.

### Concurrent edits on one sheet (`concurrency.js`)

A sheet has one editor, but that editor can fire concurrent writes (slow
network, retries, several tabs, an event storm). `concurrency.js` runs
`CONCURRENCY` VUs (default 15) writing to **one** sheet, each waiting for its
answer before the next write, and re-sending ~30 % of the writes in parallel
(`DUPLICATE_RATE`). When the storm ends, `teardown()` reads the sheet back and
fails the run if anything was lost:

- `VARIANT=distinct` (default): each VU owns a field and writes `1000·n + k`
  for iteration `k`. The stored value of every field must be that VU's **last**
  write; `legnaris_lost_updates` counts the ones that are not.
- `VARIANT=same`: every VU writes `stats.strength`; the stored value must be
  one that was actually written.
- Both: every modifier and `derived.armorClass` is recomputed from the stored
  inputs; a mismatch is a cascade computed from a stale read
  (`legnaris_inconsistent_derived`).

```bash
SCRIPT=concurrency.js make loadtest-dev
VARIANT=same SCRIPT=concurrency.js make loadtest-dev
```

**Known failing:** `VARIANT=distinct` currently reports lost writes (about 12
of 15 fields). `PATCH /sheets/:id` reads the sheet, recomputes the whole
document and persists `data` as a whole (`service.go` `PartialUpdate`), so a
slower request that read an older state overwrites newer writes to *other*
fields. The test measures and reports; the fix (a versioned/atomic update or a
per-sheet serialisation) is a separate card.

### Onboarding (`onboarding.js`)

Register + create-sheet is not steady state: registration hashes a password
(~230 ms) and creating the sheet takes ~2 ms. `onboarding.js` drives it on its
own at `ONBOARDING_RATE` (5/s) with separate thresholds (register p95 < 1.5 s,
create p95 < 200 ms). Every iteration creates a new account and nothing removes
it: use a throwaway environment.

### Seeded accounts

Accounts are deterministic (`lt-seed-<n>@loadtest.local`, see
`k6/lib/accounts.js`) and the setup is idempotent: the first run registers an
account and creates its sheet, every later run only logs in. For a large `USERS`
seed once, in parallel, instead of paying for it inside the test's setup:

```bash
USERS=1000 make loadtest-seed     # against a running API (BASE_URL)
```

The access token lives 15 minutes, so the script signs in again when it is about
to expire; a long run does not die on 401. Raise `USERS` to extend the data set;
accounts already there are reused.

The default rate limits (80 req/min per user, 10 auth attempts/min per IP)
would refuse this traffic. **Only on a throwaway environment**, raise
`AUTH_RATE_LIMIT_ATTEMPTS`, `USER_RATE_LIMIT_ATTEMPTS` and
`PUBLIC_RATE_LIMIT_ATTEMPTS`.

## One command (local, Docker only)

```bash
make loadtest-dev                           # 100 req/s for 2 min
RATE=150 DURATION=5m USERS=1 make loadtest-dev
KEEP_UP=1 make loadtest-dev                 # keep Grafana afterwards
make dev-observability-down                 # stop it (after KEEP_UP=1)
```

It builds the API and runs it with MongoDB, Prometheus, Grafana and k6 as
containers on one compose network (rate limits lifted, no host port exposed
except Grafana/Prometheus on `127.0.0.1`), then tears everything down. Watch
**Legnaris — Load test (k6)** at http://localhost:3000/d/legnaris-loadtest
(no login). Needs only Docker and `make`.

## Step by step

### A. Against `make dev` — k6 summary in the terminal only

`make dev` runs MongoDB, the API and the web client, without Prometheus or
Grafana.

1. Terminal 1 — start the stack with the limits lifted (the API inherits the
   exported variables):
   ```bash
   export AUTH_RATE_LIMIT_ATTEMPTS=100000 USER_RATE_LIMIT_ATTEMPTS=1000000 PUBLIC_RATE_LIMIT_ATTEMPTS=1000000
   make dev
   ```
2. Terminal 2 — run the test (defaults: 100 req/s, 2 minutes, 50 users):
   ```bash
   make loadtest      # RATE=150 DURATION=5m USERS=1 make loadtest for a hot sheet
   ```
   `--network host` reaches the API on Linux; on Docker Desktop/WSL prefer
   `make loadtest-dev`. k6 exits non-zero when p95 >= 200 ms, errors >= 0.1 %
   or an iteration is dropped.
3. DAG benchmark (no stack needed): `make bench-dag`.
4. Ctrl+C in terminal 1 stops the API and removes the database.

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
