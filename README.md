# Legnaris

Legnaris keeps tabletop character sheets alive: a Go + MongoDB API and an
installable React PWA that opens, edits, previews and saves one sheet at a time.
Development runs locally with plain `http://localhost`; production is a Docker
Compose stack behind Traefik with TLS and an OAuth gate for the ops tools — see
[`deploy/README.md`](deploy/README.md), which nothing in this README needs.

Before changing an area, read [`context.md`](context.md): it maps every part of
the system to the file that explains its purpose, decisions, flows, contracts,
code and limits. Working rules for agents live in [`AGENTS.md`](AGENTS.md) and
the nested `AGENTS.md` files next to the code.

## Prerequisites

- Docker (for MongoDB and RabbitMQ), Go (version in `go.mod`) and Node.js 24
  (`web/.nvmrc`).
- `make` for the commands below.
- Use **Chrome, Edge or Firefox**. The session cookies are `Secure` +
  `__Host-` prefixed (a fixed contract); those browsers accept them on
  `http://localhost`, Safari does not.

## Setup

```bash
make setup    # enable the versioned pre-commit hook (formats staged Go and web files)
cd web && npm ci && cd ..   # web dependencies, frozen lockfile
```

The hook only formats; lint and tests stay in `make` and CI so it stays fast
enough that nobody reaches for `--no-verify`. `make dev` installs the web
dependencies itself when needed.

No `.env` is required for development. The scripts fall back to
development-only secrets (`JWT_SECRET`, `EMAIL_CODE_SECRET`); values from the
environment or from the root `.env` take precedence. Never reuse those
fallbacks outside development. For the API alone, copy [`.env.example`](.env.example).

## Run

### `make dev`

```bash
make dev
```

One command starts everything you need to debug, on plain `http://localhost`:

| Piece | Address                                | Notes                                         |
| ----- | -------------------------------------- | --------------------------------------------- |
| Web   | <http://localhost:5173>                | Vite dev server with hot reload               |
| API   | <http://localhost:8080>                | Built from `backend/cmd/api.go`               |
| DB    | `mongodb://localhost:27017/` (no auth) | Docker container `legnaris-mongo`, always new |
| Broker | `amqp://localhost:5672/legnaris`      | Docker container `legnaris-rabbitmq`, always new (`RABBITMQ_PORT`) |
| Relay and worker | (no port of their own)      | Local processes; the worker uses the `dev` sender |

Open <http://localhost:5173>, register and sign in. Press `Ctrl-C` to stop: the
API, the web server, the MongoDB and RabbitMQ containers are all shut down.

**The database and the broker are always clean.** Every `make dev` removes any
previous `legnaris-mongo` and `legnaris-rabbitmq` containers and starts fresh
ones without a volume; both are removed again on exit. Nothing survives between
runs. If RabbitMQ does not answer within 60 seconds the script stops with an
error and the container logs to check.

**Sign-up codes are printed to the terminal.** `make dev` runs the email worker
with `EMAIL_PROVIDER=dev`, which logs each message body in clear. To finish a
registration, copy the 6-digit code from the `[worker]` lines into the web form.
No mailbox is needed. The dev sender refuses to start when `APP_ENV=production`,
so the code-in-logs behavior cannot reach production.

### Changing ports

```bash
PORT=9000 WEB_PORT=3000 MONGO_PORT=27018 make dev
```

The script derives `CORS_ALLOWED_ORIGINS`, `VITE_API_BASE_URL` and the Vite
dev-server CSP from those values, so nothing else needs editing.

### What does not run in development

Traefik, oauth2-proxy, the real email provider (Resend) and the observability
stack. Registration always starts the email-code step; in `make dev` the code is
printed by the worker (see above). For the real delivery path, run the pieces
separately (`make relay`, `make worker`, see `deploy/README.md`).

### Running the pieces separately

```bash
make dev-db                       # MongoDB only (uses DATABASE_URI from .env)
make run                          # API only (needs .env, see .env.example)
cd web && npm ci && npm run dev   # web only (API at http://localhost:8080)
```

`web/.env.development` points the dev server at `http://localhost:8080`; use
`VITE_API_BASE_URL` to point it elsewhere.

### Troubleshooting

- **Port already in use** — the web server uses `--strictPort`; free the port or
  change it as above.
- **Requests blocked by CORS or CSP in the console** — the browser origin must
  match `WEB_PORT`, and the API address must match `PORT`. When running pieces
  separately, keep `CORS_ALLOWED_ORIGINS` and `VITE_API_BASE_URL` in step.
- **Signed out right after signing in** — the browser rejected the `Secure`
  cookie; use Chrome, Edge or Firefox on `http://localhost` (not an IP address or
  a LAN hostname).
- **`legnaris-mongo` name conflict** — `docker rm -fv legnaris-mongo`.

## Tests

Nothing here needs a cloud service. Go integration tests self-skip when MongoDB
is unavailable; the web unit suite uses pinned captures and needs no backend.

### Go

```bash
make test          # go test ./... (integration tests self-skip without MongoDB)
make test-race     # the same with the race detector; must stay clean
make bench-dag     # DAG recalculation benchmarks; fails if the 200-field p95 exceeds 50 ms
```

`go test -run TestName ./backend/pkg/path` runs one test. Lint, vet and format
before a pull request: `make fmt && make lint && make vet` (also `make mod-check`).

### Web unit and rendering

```bash
make test-web      # Vitest (unit, rendering, pinned captures)
make lint-web      # Prettier check, ESLint (anti-XSS), type-check
```

The live contract suite is the only backend-dependent one; it reports as skipped
unless `API_BASE_URL` is set:

```bash
cd web && API_BASE_URL=http://localhost:8080 npm test
```

### Browser end-to-end

```bash
cd web
npx playwright install --with-deps chromium
npm run test:e2e
```

Playwright builds the client, serves the preview on port 4173 and drives
Chromium against intercepted API routes, so no backend is needed. The specs
cover accessibility (axe), PWA installability and offline, security headers,
responsive layout and the auth flows. One spec at a time:
`npm run test:e2e -- sheet.spec.ts`.

## CI and local hooks

Every pull request runs [`.github/workflows/ci.yml`](.github/workflows/ci.yml):

| Job | What it does |
| --- | --- |
| `changes` | Path filter: decides whether the Go, web or image suites are needed |
| `context-docs` | Fails a mapped change that does not update its `context/*.md` file |
| `go-lint` | golangci-lint, `go mod tidy -diff`, `docker compose config`, shellcheck |
| `go-race` | `go test -race` against a MongoDB replica set |
| `web-quality` | Format check, ESLint, type-check, Vitest with coverage gates, build |
| `web-e2e` | Playwright/Chromium browser suite |
| `images-build` | Builds the app, web and broker images without pushing |
| `gate` | The only required check; fails when any suite that ran failed |

Branch protection on `main` requires only `gate`, which always runs, so a
skipped suite counts as passed instead of leaving a required check pending.

Two more workflows run outside pull requests:
[`.github/workflows/ci-images.yml`](.github/workflows/ci-images.yml) publishes
`ghcr.io/<owner>/legnaris-app`, `legnaris-web` and `legnaris-rabbitmq` on `main`
and `v*` tags, and
[`.github/workflows/cd-vps.yml`](.github/workflows/cd-vps.yml) deploys one
published commit to the VPS after the images land. Screenshot regression
comparisons were removed: visual changes made the committed baselines fail on
every UI pull request, and accessibility, responsive-layout and behavior specs
remain.

Local: `make setup` points `core.hooksPath` at `.githooks/`, whose `pre-commit`
formats staged Go and web files and re-stages them. The spec lint scripts under
`scripts/` (`check_*.py`) no longer run in CI.

## Web client

The client is a React + Vite + TypeScript PWA; its platform — shell, routing,
theme, tokens, copy, typed API client, offline behavior and security posture —
is documented in [`context/web-client.md`](context/web-client.md). Visual values
live in [`web/DESIGN.md`](web/DESIGN.md); working rules in
[`web/AGENTS.md`](web/AGENTS.md).

| Script                 | What it does                                             |
| ---------------------- | -------------------------------------------------------- |
| `npm run dev`          | Vite dev server (SPA fallback for deep links)            |
| `npm run build`        | Type-check + production build (also emits `dist/sw.js`)  |
| `npm run preview`      | Serve the production build (SPA fallback, offline shell) |
| `npm run lint`         | ESLint, including the anti-XSS rules                     |
| `npm run format:check` | Prettier check                                           |
| `npm run typecheck`    | `tsc --noEmit`                                           |
| `npm test`             | Vitest unit + rendering tests                            |
| `npm run test:e2e`     | Playwright end-to-end specs (`e2e/`)                     |

`web/Dockerfile` builds the client into an nginx image, which the deploy stack
runs as the `web` service on `WEB_DOMAIN`. A local build check:

```bash
docker build -t legnaris-web:local web
```

The browser-facing API origin is a **runtime setting**: the container writes
`/config.json` from `API_BASE_URL` at start, and the client loads it uncached
before the first render, so one image serves dev and production and changing it
needs a restart, not `--build`. The origin must also appear in `connect-src` of
the Traefik CSP and the web origin in `CORS_ALLOWED_ORIGINS`.

Security scanning is local and on demand; CI runs only quality, tests and e2e:

```bash
make sec-scan    # npm audit + signatures + lockfile-lint (+ gitleaks when installed)
```

`package.json` pins TypeScript 6 and ESLint 9, the newest major lines every
required anti-XSS and a11y plugin supports (`typescript-eslint`,
`eslint-plugin-react`, `eslint-plugin-jsx-a11y`); bump them together once the
plugins publish compatible majors.

## Load testing

The PATCH acceptance target is **100 req/s sustained with p95 < 200 ms and
error rate < 0.1 %**; the DAG benchmark target is **p95 < 50 ms for templates of
up to 200 fields**. k6 exits non-zero when a scenario's thresholds fail.

```bash
make loadtest-dev    # everything in containers (DB, API, Prometheus, Grafana, k6), torn down after
make loadtest        # against a running API (BASE_URL), k6 summary in the terminal
make loadtest-seed   # seed USERS accounts once, in parallel (idempotent)
make bench-dag       # no stack needed
```

`make loadtest-dev` needs only Docker and `make`, and exposes the **Legnaris —
Load test (k6)** dashboard at <http://localhost:3000/d/legnaris-loadtest> (no
login). `KEEP_UP=1` keeps it afterwards; `make dev-observability-down` stops it.
`SCRIPT=` selects the scenario (default `patch-sheet.js`), and `RATE`,
`DURATION`, `USERS`, `STAGES`, `STEP_SECONDS` tune the run:

| Script | Question it answers |
| --- | --- |
| `patch-sheet.js` | Does 100 req/s on `PATCH /sheets/:id` hold the budgets? |
| `capacity.js` | Where does it break? A ramping run (default `100,200,400,800,1600` req/s) aborts at the first step over p95 200 ms or 1 % failures; the last step that did not abort is the measured capacity |
| `stress.js` | How does it behave when overloaded? Climbs past capacity and drops back to a recovery step, aborting only on a meltdown |
| `soak.js` | Does it leak? Holds one rate (set it to about half the measured capacity) and plots the 15-minute RSS/heap/goroutine slope |
| `concurrency.js` | Are concurrent writes on one sheet lost? `VARIANT=distinct` writes one field per VU, `VARIANT=same` writes one field from every VU, and reads the sheet back to verify the stored value and the derived cascade |
| `onboarding.js` | Register + create-sheet at a steady rate, with its own thresholds (registration hashes a password, so it is far slower than a sheet write) |
| `auth-storm.js` | What does a login storm do to sheet traffic? Holds background sheet load and ramps logins, showing that past the bcrypt hash ceiling (~40 verifications/s on a 12-core host at cost 12) logins starve everything else |

Accounts are deterministic (`lt-seed-<n>@loadtest.local`, see
`loadtest/k6/lib/accounts.js`) and seeded idempotently. The default rate limits
(80 req/min per user, 30 auth attempts/min per IP, 10 login attempts/min per
account) would refuse this traffic, so **on a throwaway environment only** raise
`AUTH_RATE_LIMIT_ATTEMPTS`, `AUTH_ACCOUNT_RATE_LIMIT_ATTEMPTS`,
`SESSION_RATE_LIMIT_ATTEMPTS`, `USER_RATE_LIMIT_ATTEMPTS` and
`PUBLIC_RATE_LIMIT_ATTEMPTS`. A laptop run shares CPU with k6, the API and
MongoDB: trust capacity numbers only from the `deploy` stack.

`PATCH /sheets/:id` is a compare-and-swap on the sheet version, so a write that
loses a race answers 409 `CONFLICT`; `concurrency.js` resends it after a short
jittered backoff and counts the conflicts as informational.

To run k6 against `make dev` instead (terminal summary only, no Prometheus or
Grafana), start the stack with the limits lifted and fire the test from a second
terminal:

```bash
export AUTH_RATE_LIMIT_ATTEMPTS=100000 AUTH_ACCOUNT_RATE_LIMIT_ATTEMPTS=100000 SESSION_RATE_LIMIT_ATTEMPTS=100000 USER_RATE_LIMIT_ATTEMPTS=1000000 PUBLIC_RATE_LIMIT_ATTEMPTS=1000000
make dev
make loadtest    # second terminal; default 100 req/s, 2 minutes, 50 users
```

`make loadtest` reaches the API over `--network host`, which works on Linux; on
Docker Desktop or WSL prefer `make loadtest-dev`.

## Pinned API captures

`web/src/api/fixtures/` holds real requests and responses captured from the
local backend. The pinned suite decodes them with no backend running, and a
shape change in any capture fails it: a capture is never hand-edited to make a
test pass — that is a backend contract change, and the models are updated
deliberately instead.

Refresh them against the running stack:

```bash
make dev-db                                # MongoDB
go run ./backend/cmd/api.go                # the API on :8080
bash web/src/api/fixtures/capture.sh       # registers a throwaway user and captures every case
```

`capture.sh` reads `API_BASE_URL` (default `http://localhost:8080`), registers a
throwaway user, exercises the consumed operations and failure cases, and writes
each case verbatim. It never writes a credential: the access token lives in a
temporary directory deleted on exit, and the verification capture redacts it.
The files are excluded from Prettier (`web/.prettierignore`) so their bytes
stay as the server sent them.

A response capture records the request that produced it next to the raw status
and body (`{"operation", "provenance", "request", "response"}`); requests whose
body carries a password record only method and path. The set covers health, the
template catalog and detail, sheet creation, read and patch (plus the request
pairs), the uniform error codes (`NOT_FOUND`, `INVALID_REQUEST`,
`UNKNOWN_FIELD`, `INVALID_JSON`, `INTERNAL_ERROR`, `UNAUTHORIZED`,
`EMAIL_UNAVAILABLE`, `CONFLICT`, `OTP_INVALID`) and the auth challenge family
(register, login, verify, resend).

Registration always starts the code step, so a capture run also needs:

- `mongosh` on the `PATH`, to read the code the stack recorded (the code never
  reaches a log line).
- `DATABASE_URI` and `DATABASE_NAME` (default `legnaris`) pointing at the
  stack's database.
- A spendable daily send allowance: the run spends it on purpose to capture the
  unavailable-send refusal, then removes the send intentions it recorded. A
  short configuration keeps the run quick:
  `AUTH_OTP_DAILY_SEND_CAP=5 AUTH_RATE_LIMIT_ATTEMPTS=200 make run`.

## Repository map

| Path | What lives there |
| --- | --- |
| `backend/` | The API, relay, worker and supporting binaries |
| `web/` | The React PWA client |
| `deploy/` | Production Compose stack, edge proxy, observability and VPS scripts |
| `loadtest/` | k6 scenarios and shared helpers |
| `context/` | One file per area: purpose, decisions, flows, contracts, code, limits |
| `.specs/`, `docs/specs/` | Feature specs, plans and the runnable validation guides (`quickstart.md`) |
| `docs/adr/` | Architecture decision records; `context/` holds the current decision, these hold the history |
| `testdata/dice/` | Grammar vectors shared by the Go and TypeScript dice implementations |

`make test`, `make test-race`, `make lint`, `make vet` and `make fmt` are defined
in the [`Makefile`](Makefile).
