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

### Stress (`stress.js`)

The capacity ramp stops at the first breached step. `stress.js` keeps going to
learn **how the service behaves when overloaded**: `STRESS_STAGES` defaults to
`2000,3000,4500,6000,300`, the same mix, and the last step drops back to 300
req/s as a **recovery** check. Nothing aborts on a breached budget (only on a
meltdown: half the requests failing), so a red step is a finding, not a failed
run. Dropped iterations are expected and mean the generator could not hold the
rate.

```bash
SCRIPT=stress.js make loadtest-dev          # laptop: shares CPU with k6 and MongoDB
SCRIPT=stress.js make loadtest              # against a running API
```

Read it by step (**Steps** row of the dashboard): does p95 climb smoothly or
fall off a cliff, do errors or only latency appear, and does the 300 step return
to the baseline p95? A laptop run (30 s steps, k6 + API + MongoDB on one host)
gave p95 12 ms at 2000, 116 ms at 3000, 380 ms at 4500, 1.0 s at 6000, **0 %
errors at every step**, and 4.5 ms back at 300: it degrades by slowing down and
recovers fully. At the top steps k6 ran out of VUs (`dropped_iterations`), so
those numbers measure the host as much as the API; only the `deploy` stack gives
a figure worth quoting.

### Why login and registration are expensive, and the auth storm

`POST /auth/login` and `/auth/register` take ~230 ms against 2-5 ms for a sheet
operation because each one runs **one bcrypt hash at cost 12** (`TDD.md`
mandates cost >= 12; `AUTH_BCRYPT_COST`). bcrypt is slow on purpose: it makes
guessing a stolen hash expensive, and each +1 of cost doubles the work. Measured
on the 12-core development host:

| cost | one hash | hashes/s, 12 cores |
|---|---|---|
| 10 | 57 ms | 179 |
| 11 | 113 ms | 80 |
| **12** | **226 ms** | **42** |
| 13 | 458 ms | 21 |

So the whole API can verify about **40 passwords per second**, against several
thousand sheet operations per second. Two properties make it worse than it
looks: the hash is CPU bound, so it takes CPU from every other request; and an
unknown account also pays a full hash (`spendPasswordWork`) so the response time
does not reveal which accounts exist, which means an attacker needs no valid
account to burn the same CPU.

`auth-storm.js` measures the consequence: it holds `BACKGROUND_RATE` (200 req/s)
of sheet traffic and ramps logins over it (`LOGIN_STAGES`, default
`5,10,20,40,80` per second). On the laptop run, at 20 logins/s the sheet p95
stayed at 2.9 ms; at 60 logins/s login p95 was 4.4 s **and the sheet p95 rose to
488 ms**: past the hash ceiling, logins starve everything else.

```bash
SCRIPT=auth-storm.js make loadtest-dev
```

Do not lower the cost to make the numbers look better. What helps is bounding
the damage: cap concurrent hashes below the core count so overload queues logins
instead of starving other traffic, and keep the per-address and per-account
limiters (they are lifted in these test environments).

### Soak (`soak.js`)

Holds one rate (`SOAK_RATE`, default 100 req/s) for `SOAK_SECONDS` (default
1800 = 30 min) over the same mix as the ramp. **Set `SOAK_RATE` to about half of
the capacity the ramp measured**, on the `deploy` stack; a soak at the limit
measures the limit, not leaks. Requests carry a `phase` tag (first 20 % =
`start`, last 20 % = `end`), and each phase has its own p95 line, so latency
drift shows in the summary instead of being averaged away. On a laptop run a
short check only:

```bash
SCRIPT=soak.js SOAK_SECONDS=240 SOAK_RATE=50 make loadtest-dev   # smoke, not a soak
SCRIPT=soak.js SOAK_RATE=<half of the ramp's capacity> make loadtest   # real soak
```

The dashboard's **Soak (leak trend)** row plots the 15-minute slope of RSS, heap
and goroutines. A leak is a slope that stays positive for the whole run; the red
lines (10 MB/h, 100 goroutines/h) are heuristics to tune after the first soak.
Heap is noisy over short windows (GC), so read it after the first 15 minutes.

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

**Conflicts:** `PATCH /sheets/:id` is a compare-and-swap on the sheet
`version`; a write that loses the race answers 409 `CONFLICT` with
`Retry-After`, and the script resends it after a short jittered backoff
(`BACKOFF_MIN_MS`/`BACKOFF_MAX_MS`, `MAX_RETRIES`) instead of the 1 s hint.
`legnaris_conflicts` counts the 409s (informational); 409 is not an HTTP
failure for this scenario. A write still refused after `MAX_RETRIES` counts as
`legnaris_unexpected_status`.

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

