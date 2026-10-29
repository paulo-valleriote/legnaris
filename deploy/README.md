# Legnaris — Production stack (Docker Compose + Traefik)

Self-contained stack for a single VPS: the Go app (Echo v5) + the React PWA
client (nginx) + MongoDB + observability (Prometheus, Grafana, Loki, Alloy) + a
database UI (Mongo Express), with admin access protected by **GitHub OAuth**
(oauth2-proxy) and **Traefik** at the edge.

---

## 1. Architecture and domains

```
legnaris.com.br          → Vercel (frontend)           [untouched]
api.legnaris.com.br      → Docker/Traefik              [public, no OAuth]
   └─ /                  → app API (health, templates, sheets)
app.legnaris.com.br      → Docker/Traefik              [public, no OAuth]
   └─ /                  → web client (service `web`, nginx)
ops.legnaris.com.br      → Docker/Traefik              [GitHub OAuth]
   ├─ /oauth2/callback   → GitHub OAuth callback
   ├─ /grafana           → Grafana
   ├─ /prometheus        → Prometheus
   ├─ /mongo             → Mongo Express
   └─ /rabbitmq          → RabbitMQ console
```

Why split into three subdomains? Each surface has its own security rule: `api.`
and `app.` are public, `ops.` requires a GitHub login. Keeping the client on its
own origin (`app.`) is what lets the API stay a pure JSON service with a CORS
allowlist instead of serving two kinds of traffic from one host and one route
table.

**Threat model:** the `ops.` subdomain is the only administrative surface. It is
gated by GitHub OAuth at the Traefik edge; Grafana, Prometheus, Mongo
Express and the RabbitMQ console run on the `internal` Docker network only (not published, not on
`edge`), so they are reachable solely through Traefik, which enforces that
login. Their own credentials (Grafana admin, Mongo Express basic auth) are
defense-in-depth for a trusted internal network — not a substitute for the
OAuth gate. Anything that can reach those containers directly on the `internal`
network is already trusted.

---

## 2. Understanding each layer (no jargon)

### DNS — the "address book"
DNS translates names (`api.legnaris.com.br`) into IPs. Inside your domain you
create **records** that point subdomains to destinations:

| Record | Meaning |
|---|---|
| `A` | "this name → this IP" (e.g. `api` → `203.0.113.10`). |
| `CNAME` | "this name → another name" (e.g. `api` → `tunel.cfargotunnel.com`). |
| `@` or empty | the apex domain (`legnaris.com.br`). |

The apex `legnaris.com.br` already points to Vercel. You only **add** three new
records (`api`, `app` and `ops`) — leave what already exists untouched.

### Traefik — the "doorman" (reverse proxy)
Traefik sits in front of everything, listening on ports 80/443. It is **not**
your web server — it decides, for each request, which service should handle it,
based on the **Host** (requested domain) and the **path**:

- Requests `api.legnaris.com.br/...` → forwards to the app.
- Requests `app.legnaris.com.br/...` → forwards to the web client.
- Requests `ops.legnaris.com.br/grafana` → forwards to Grafana, but first asks
  oauth2-proxy ("is this person logged in?").

It also handles **TLS/HTTPS**: it issues and renews certificates via Let's
Encrypt.

### Routing — Host vs path
- **By Host** (`api.` vs `app.` vs `ops.`): different domains → different services.
- **By path** (`/grafana` vs `/prometheus` on the same `ops.`): same domain,
  different paths.

Here we use both: **Host** to separate public/admin, and **path** to split the
tools inside `ops.`.

### OAuth (oauth2-proxy) — the login
When someone hits an `ops.` route, Traefik asks oauth2-proxy whether there is a
valid session cookie. If not, it redirects to GitHub; after login, GitHub
returns to `ops.legnaris.com.br/oauth2/callback` and oauth2-proxy checks that
the user is in the allowlist (`GITHUB_USER`) before letting them through.

### Cloudflare Tunnel (local environment only)
To test locally without opening ports or having a VPS, `cloudflared` creates a
"tunnel" that publishes your local Docker stack on a public HTTPS URL.
Cloudflare terminates TLS; the local Traefik speaks plain HTTP behind the
tunnel.

---

## 3. Prerequisites

1. **Domain** `legnaris.com.br` (registered at Registro.br, DNS managed at Cloudflare) — already exists.
2. **GitHub OAuth App** (see §4).
3. **DNS for the `api.` and `ops.` subdomains** (see §5).
4. **oauth2-proxy cookie secret** (see §6).
5. **Docker Compose >= 2.24** — the local override uses the `!override`
   tag (see §8); older clients fail to parse `docker-compose.local.yml`.

## 4. Create the GitHub OAuth App

1. GitHub → Settings → Developer settings → OAuth Apps → **New OAuth App**.
2. Fill in:
   - **Application name**: `Legnaris`.
   - **Homepage URL**: `https://api.legnaris.com.br`.
   - **Authorization callback URL**: `https://ops.legnaris.com.br/oauth2/callback` ⚠️ exact.
3. **Register application**.
4. Copy the **Client ID** and generate the **Client secret**.

> The callback requires HTTPS (GitHub only accepts `http://` for `localhost`).

## 5. Migrate DNS to Cloudflare

DNS becomes managed at **Cloudflare** (which also provides edge analytics and
native Cloudflare Tunnel integration). The domain stays **registered** at
Registro.br — only *where* the zone is managed changes.

### 5.1 Migrate the zone (once)

1. Create an account at **https://dash.cloudflare.com** and click **Add a site**,
   entering `legnaris.com.br` (the Free plan is enough).
2. Cloudflare **scans and imports** the current records (the Vercel apex).
   Check they came through correctly.
3. Cloudflare shows **two nameservers** (e.g. `xxx.ns.cloudflare.com`,
   `yyy.ns.cloudflare.com`). Note them down.
4. At **Registro.br**, replace the domain's **NS** with those two nameservers:
   go to https://registro.br → My domains → `legnaris.com.br` →
   **DNS servers / Change NS**.
5. Wait for propagation (minutes to a few hours). In Cloudflare the zone changes
   to **Active**.

### 5.2 Create the subdomains (in Cloudflare)

**Production (VPS) — type A, leave "DNS only" (no proxy):**
| Name | Type | Value | Proxy |
|---|---|---|---|
| `api` | A | VPS IP | DNS only |
| `app` | A | VPS IP | DNS only |
| `ops` | A | VPS IP | DNS only |

**Local (Cloudflare Tunnel):** don't create them by hand — use
`cloudflared tunnel route dns` (see §8), which creates the CNAMEs automatically.

### 5.3 Notes

- **Vercel apex**: keep `legnaris.com.br` in **DNS only** (don't enable the
  proxy/orange cloud). Putting Cloudflare in front of Vercel causes a
  double-CDN and cache conflict; Vercel already has its own analytics.
- **Edge analytics (optional)**: if you want Cloudflare analytics for the
  backend, enable the proxy (orange) on `api.`/`ops.`. In that case disable the
  cache on `api.` and configure Traefik to trust the Cloudflare headers
  (forwardedHeaders). The origin already uses Let's Encrypt, which works in
  "Full (strict)" mode.

## 6. Configure `.env`

```bash
cd deploy
cp .env.example .env
# fill in: API_DOMAIN, WEB_DOMAIN, WEB_API_BASE_URL, OPS_DOMAIN, ACME_EMAIL,
# GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, GITHUB_USER, CORS_ALLOWED_ORIGINS,
# JWT_SECRET, EMAIL_CODE_SECRET, MONGO_ROOT_PASSWORD, MONGO_APP_PASSWORD,
# MONGO_EXPORTER_PASSWORD,
# MONGO_VIEW_PASSWORD, ME_CONFIG_BASICAUTH_USERNAME/PASSWORD,
# GRAFANA_ADMIN_USER/PASSWORD
openssl rand -base64 32 | tr '+/' '-_'   # use as OAUTH2_PROXY_COOKIE_SECRET
openssl rand -base64 48                  # use as JWT_SECRET
openssl rand -base64 48                  # use as EMAIL_CODE_SECRET (distinct!)
```

MongoDB is created with least-privileged users (provisioned by
`mongodb/init/01-create-users.js` on first boot): `legnaris_app` (read/write),
`legnaris_exporter` (server metrics), and `legnaris_view` (read-only, used by
Mongo Express). Services never connect as `root`.

`TRUSTED_PROXY_CIDRS` is optional. Left unset, the API trusts loopback, private
and link-local peers, which covers the proxy on the same host or Docker network
without configuration. A comma-separated CIDR set replaces the non-loopback
portion:

```bash
TRUSTED_PROXY_CIDRS=172.18.0.0/16,10.0.0.0/8
```

Loopback stays trusted in every case, and an invalid value fails the API boot
instead of silently trusting nothing. Only the direct peer decides whether the
forwarding header is read; a forged chain from any other address is ignored.

## 7. Deploy (production)

```bash
cd deploy
docker compose up -d --build
docker compose ps        # mongodb "healthy" and the rest "Up"
```

## 8. Run locally (via Cloudflare Tunnel)

```bash
# 1. once, on the host:
cloudflared tunnel login
cloudflared tunnel create legnaris      # writes ~/.cloudflared/<TUNNEL_UUID>.json
cloudflared tunnel route dns legnaris api.legnaris.com.br   # creates the CNAME automatically
cloudflared tunnel route dns legnaris app.legnaris.com.br
cloudflared tunnel route dns legnaris ops.legnaris.com.br

# 2. configure the tunnel:
cp cloudflared/config.yml.example cloudflared/config.yml
# edit and replace <TUNNEL_UUID> with the value from `cloudflared tunnel list`
cloudflared tunnel --config cloudflared/config.yml ingress validate

# 2b. the cloudflared container runs as uid 65532 and reads ~/.cloudflared
#     through a read-only bind mount, so the directory must be traversable and
#     the credentials file readable by that uid:
chmod 755 ~/.cloudflared
chmod 644 ~/.cloudflared/<TUNNEL_UUID>.json

# 3. bring it up with the local override:
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build
```

If the tunnel already exists and its credentials are not on this host, fetch them
instead of creating a second tunnel: `cloudflared tunnel token --cred-file
~/.cloudflared/<TUNNEL_UUID>.json legnaris`.

What the local override changes (vs production):
- Traefik uses the shared `traefik.yml` with plain HTTP and **no** Let's Encrypt
  (the production-only TLS flags are dropped; Cloudflare terminates TLS).
- oauth2-proxy sets `OAUTH2_PROXY_COOKIE_SECURE=false` (the origin is HTTP
  behind the tunnel).
- Adds the `cloudflared` service that publishes `api.`, `app.` and `ops.`.

The `web` service is built by this same command: its bundle is compiled with
`WEB_API_BASE_URL`, so pointing the client at a different API (or serving it on
a different `WEB_DOMAIN`) needs `docker compose up -d --build web`, not a
restart. `CORS_ALLOWED_ORIGINS` must list `WEB_DOMAIN` — the two are configured
together in `deploy/.env`.

**Quick alternative (random URLs, just to check it boots):**
```bash
docker run --rm --network legnaris-prod_edge cloudflare/cloudflared:2026.9.1 \
  tunnel --no-autoupdate --url http://traefik:80
```

### Extra security for the local tunnel: Cloudflare Access (Zero Trust)

The tunnel makes `api.`/`ops.` reachable from the internet. For local testing,
add an authentication layer **before** your machine — so nothing reaches it
without authenticating at Cloudflare first:

1. Go to **https://one.dash.cloudflare.com** (Zero Trust) → **Access** →
   **Applications** → **Add an application** → **Self-hosted**.
2. In *Application domain*, enter `ops.legnaris.com.br` (and `api.legnaris.com.br`
   if you also want to protect the API during testing).
3. Add an *Allow* **policy** for your email (PIN/OTP login) or via GitHub.
4. Save. Now Cloudflare asks for your login **before** it reaches oauth2-proxy —
   two barriers in sequence.

> This protects access *to your local service*. It does not replace the `ops.`
> OAuth (which authorizes by allowlist) — it is an extra layer, specific to the
> local environment. In production (VPS), Access is optional. When not testing,
> shut the tunnel down (`docker compose down`).

## 9. Access

| URL | What it is | Protection |
|---|---|---|
| `https://app.legnaris.com.br/` | Web client (PWA) | public |
| `https://api.legnaris.com.br/` | Public API | public |
| `https://ops.legnaris.com.br/grafana` | Grafana | GitHub OAuth |
| `https://ops.legnaris.com.br/prometheus` | Prometheus UI | GitHub OAuth |
| `https://ops.legnaris.com.br/mongo` | Mongo Express | GitHub OAuth + basic auth |
| `https://ops.legnaris.com.br/rabbitmq/` | RabbitMQ console | GitHub OAuth + the operator's broker account |

The RabbitMQ console lists the queued messages and their contents, so it shows
the one-time codes in the clear. It is the operator's window into a stuck
queue and nothing else: it has no host port and no other route, and the
application's broker account cannot sign in to it (see "RabbitMQ provisioning").

## 10. Observability

- **P50/P95/P99 per route**: Grafana → `legnaris_http_request_duration_seconds`
  (labels method/path/status).
- **Structured logs**: Grafana → Explore (Loki), `{service="app"} | json`.
  Correlation via `X-Request-Id`.
- **Edge/DB metrics**: Prometheus scrapes Traefik and MongoDB Exporter.
- **App metrics**: Prometheus scrapes the app's `/metrics` directly on the
  internal network; the endpoint is not routed on the public `api.` domain.
- **Relay metrics**: Prometheus scrapes the relay's own listener (`relay:9100`)
  for the outbox backlog and the publication outcomes by result.
- **Worker metrics**: Prometheus scrapes the worker's own listener
  (`worker:9101`) for the send outcomes, the dead letters and the expired
  codes.
- **Broker metrics**: Prometheus scrapes the broker's Prometheus plugin
  (`rabbitmq:15692`) for the queue depth, the unacknowledged messages and the
  consumer count. The plugin is enabled when the broker image is built
  (`deploy/rabbitmq/Dockerfile`); neither port is published on the host.
- **Query the database**: Mongo Express at `/mongo`.

## 11. CORS (web client → API)

The app allows origins from `CORS_ALLOWED_ORIGINS` (default in `.env.example`:
`https://app.legnaris.com.br`). Without it, the browser blocks every call from
the web client to the API. The origin must match exactly — scheme, host, and
port — or the preflight fails.

`make deploy-up` / `make deploy-local` strip the local-development variables the
root `Makefile` exports for `make run`, so this file's values win. A shell
variable outranks a compose environment file: a leaked empty
`CORS_ALLOWED_ORIGINS` would fall through to the compose default
(`http://localhost:3000`) and silently block the real frontend origin. Run the
deploy through `make`, or pass `env -u CORS_ALLOWED_ORIGINS …` yourself.

## 12. Day-to-day operation

```bash
docker compose ps
docker compose logs -f app traefik oauth2-proxy
docker compose restart app
docker compose down          # stop without deleting data
docker compose down -v       # stop AND delete volumes (destroys data)
```

### Email canonicalization (one-shot)

```bash
docker compose run --rm --entrypoint /canonicalize-emails app           # dry run
docker compose run --rm --entrypoint /canonicalize-emails app -apply    # write
```

The report lists `scanned`, `change`, `collision`, `invalid` and `written` (or
`dry run`) lines. A collision or an unparseable row aborts the run with exit 1
and writes nothing; resolve the duplicate manually, since the command never
merges accounts. Only `email` and `updated_at` change, and a second run over
repaired data is a no-op. The command is a separate binary and is never part of
the API boot path.

### Email relay (publisher, optional)

The relay is a separate process (`backend/cmd/relay`, shipped as `/relay` in the same
image) that drains the durable `email_outbox` collection to RabbitMQ. It is
optional: the API boots and serves without it and without any email or broker
variable. Turn it on by provisioning the broker (see "RabbitMQ provisioning")
and bringing the `relay` service up; `RABBITMQ_URL` defaults to the delivery
vhost and only needs setting to override it.

```bash
docker compose up -d rabbitmq relay      # start the broker and the publisher
docker compose logs -f relay             # publication outcomes
```

- **Fail-fast**: a blank or unparseable `RABBITMQ_URL` stops the relay with
  `ErrBrokerUnavailable` before it opens the database or polls; it never runs
  "with no broker". The API never reads `RABBITMQ_URL` and never depends on the
  broker to start.
- **Topology**: on startup the relay declares the exchange, the work queue, the
  three delay-tier retry queues and the dead-letter queue, idempotently — no
  broker plugin is required. The broker publishes no host port.
- **Optional variables** (with the app's defaults): `EMAIL_PROVIDER` (`log`),
  `RESEND_API_KEY` (empty), `EMAIL_FROM` (empty), `EMAIL_SEND_TIMEOUT` (`10s`),
  `RABBITMQ_URL` (the `legnaris` vhost on the stack's broker), `EMAIL_RELAY_POLL_INTERVAL`
  (`1s`), `EMAIL_RELAY_LEASE` (`30s`), `EMAIL_RELAY_BATCH` (`25`),
  `EMAIL_RELAY_CONFIRM_TIMEOUT` (`5s`), `EMAIL_RELAY_MAX_ATTEMPTS` (`5`),
  `EMAIL_OUTBOX_RETENTION` (`168h`), `EMAIL_RELAY_METRICS_ADDR` (`:9100`).

### Email worker (delivery)

The worker is a separate process (`backend/cmd/worker`, shipped as `/worker` in
the same image) that takes the queued messages out of RabbitMQ and sends each
through the configured provider. It needs no database. The API depends on
neither the worker nor the broker, so an outage of either only delays codes.

```bash
docker compose up -d rabbitmq relay worker   # after the provisioning below
docker compose logs -f worker                # send outcomes
```

- **Restart and waiting**: the service restarts unless stopped and starts after
  the broker's health check passes. The distroless image has no shell or HTTP
  probe to run a health check with, so a worker that is up but stuck shows in
  its counters (`legnaris_email_send_total`) and not in `docker compose ps`.
- **Variables** (all optional, defaults shown): `RABBITMQ_URL` (the delivery
  vhost, built from `RABBITMQ_PASSWORD`), `EMAIL_PROVIDER` (`log`),
  `RESEND_API_KEY`, `EMAIL_FROM`, `EMAIL_SEND_TIMEOUT` (`10s`),
  `EMAIL_WORKER_PREFETCH` (`10`), `EMAIL_WORKER_MAX_ATTEMPTS` (`5`) and
  `EMAIL_WORKER_METRICS_ADDR` (`:9101`). With `EMAIL_PROVIDER=resend` the key
  and the sender are required and the worker refuses to start without them. The
  API's required-variable list does not change.
- **When the pipeline stalls** (codes never arrive): open the console at
  `/rabbitmq/` and look at the queues. Messages piling up in `email.otp` with
  no consumer mean the worker is down or cannot reach the broker
  (`docker compose logs worker`); messages cycling through `email.retry.*` mean
  the provider is refusing; messages in `email.dead` were abandoned and need a
  look at their cause. The same numbers are in Prometheus
  (`rabbitmq_queue_messages_ready`, `rabbitmq_consumers`) and the worker's
  `legnaris_email_send_total{result}`.

## RabbitMQ provisioning

The delivery queues live in their own vhost, `legnaris`, and the broker has two
accounts with different powers: `legnaris` (the application's, created by the
image from `RABBITMQ_PASSWORD`) moves codes in and out of that vhost and can
never open the console; `operator` (the maintainer's) can open the console.
Vhosts, accounts and permissions are broker state, not configuration, so the
operator provisions them with the commands below. Nothing is provisioned
silently on boot and no broker credential is committed.

Run it once per broker volume, **before the relay and the worker are deployed**:
an unprovisioned broker has no `legnaris` vhost, so neither can connect. Run it
again whenever the broker's volume is replaced.

```bash
cd deploy
docker compose up -d --build rabbitmq
read -rs RABBITMQ_OPS_PASSWORD && export RABBITMQ_OPS_PASSWORD

docker compose exec -e RABBITMQ_OPS_PASSWORD rabbitmq sh -ec '
  : "${RABBITMQ_OPS_PASSWORD:?RABBITMQ_OPS_PASSWORD is required}"
  rabbitmqctl -q --no-table-headers list_vhosts | grep -qx legnaris \
    || rabbitmqctl add_vhost legnaris

  if rabbitmqctl -q --no-table-headers list_users | cut -f1 | grep -qx operator; then
    rabbitmqctl change_password operator "$RABBITMQ_OPS_PASSWORD"
  else
    rabbitmqctl add_user operator "$RABBITMQ_OPS_PASSWORD"
  fi
  rabbitmqctl set_user_tags operator management
  rabbitmqctl set_permissions -p legnaris operator ".*" ".*" ".*"

  rabbitmqctl set_user_tags legnaris
  rabbitmqctl clear_permissions -p / legnaris
  rabbitmqctl set_permissions -p legnaris legnaris ".*" ".*" ".*"
'
unset RABBITMQ_OPS_PASSWORD
```

- **Idempotent**: every step can be repeated. Running it again changes nothing
  that is already right, and re-running it rotates the operator's password to
  the one in your shell.
- **The operator's password** is read from the environment of your shell at the
  moment the command runs. It is never written to `.env` or the repository and
  no service receives it. If it is unset or empty the script stops before any
  change; the application keeps working, because it never uses that account.
  `rabbitmqctl` receives it as an argument, so it is visible in the broker
  container's process list while the command runs; anyone who can exec into
  that container already controls the broker.
- **Migrating an existing broker**: a `.env` that sets `RABBITMQ_URL` ending in
  `/` still targets the default vhost, and the last commands remove the
  application account's rights there. Point it at `.../legnaris` (or unset it)
  before provisioning. Queues already declared in the default vhost are not
  moved and can be deleted from the console once nothing reads them.
- **What the application's account ends with**: no tag, so the console refuses
  its login, and permissions on `legnaris` only, so it sees no other vhost. The
  image creates that account with the administrator tag and rights on `/`; the
  last three commands remove both.
- **Check**: sign in at `/rabbitmq/` as `operator` (after the GitHub login);
  the same page must refuse the `legnaris` account.

## One-time-code challenges

The API gates sign-in and registration behind a six-digit code emailed to the
account. Codes are generated with `crypto/rand`, stored only as an HMAC-SHA256
digest, consumed at most once and never logged.

- **Required**: `EMAIL_CODE_SECRET` keys the code digest and must be at least 32
  bytes and distinct from `JWT_SECRET`. An absent, blank or short value stops the
  API at boot (`EMAIL_CODE_SECRET:?` in the compose file), because a weak key
  would silently weaken every code.
- **Optional** (with the app's defaults): `AUTH_OTP_TTL` (`10m`, the code's
  lifetime), `AUTH_OTP_CEILING` (`30m`, the absolute limit a resend can never
  pass), `AUTH_OTP_RESEND_COOLDOWN` (`60s`), `AUTH_OTP_MAX_ATTEMPTS` (`5`),
  `AUTH_OTP_DAILY_SEND_CAP` (`80`, deliberately below the provider's free-tier
  allowance of 100), `AUTH_OTP_USER_WINDOW` (`1h`) and `AUTH_OTP_USER_LIMIT`
  (`10` challenges per account per window).
- **Verification failures** answer one uniform `OTP_INVALID` (401), so an
  unknown, expired, consumed, wrong or exhausted code is indistinguishable. The
  daily send cap answers `EMAIL_UNAVAILABLE` (503) with `Retry-After`, and the
  per-address and per-account bounds answer `RATE_LIMITED` (429).

## The sign-in hardening gate

`AUTH_REQUIRE_EMAIL_VERIFICATION` decides whether the code step is the **only**
door into a session.

- **Optional, off by default.** With it unset (or `false`), registration and
  sign-in answer a session exactly as they always did and renewal never looks
  at the verification state — the deployed contract is unchanged.
- **With it on**, registration answers `201` with a `verify_email` challenge
  and sign-in answers `202` with a `login_otp` challenge: no cookie, no access
  credential, no session. The session is established only when the emailed code
  is verified. Renewal refuses an account that never verified its address (401,
  the credential's family revoked) so a session minted before the rule cannot
  outlive it.
- **The flip is an operator act.** Set it to `true` in `deploy/.env`, restart
  the `api` service, and every client must already understand the new contract:
  this is a breaking change for the register and sign-in responses, so it is
  deliberately not automatic and not staged by the deployment itself.
- **No data moves.** Enabling or disabling the gate rewrites no account: the
  verification timestamp is written only by the code verification itself, and
  every existing account simply has none until it verifies.

- **Metrics**: the relay serves its own listener on `EMAIL_RELAY_METRICS_ADDR`
  (`:9100`), scraped by Prometheus on the internal network:
  `legnaris_email_outbox_backlog` (pending gauge) and
  `legnaris_email_publish_total{result}` (`published`/`retried`/`failed`).
- **Stuck emails**: a send intention is marked published only after the broker
  confirms it; a failed publication is retried with backoff and, once the cap
  (`EMAIL_RELAY_MAX_ATTEMPTS`) is reached, marked `failed` for operator
  attention. Inspect the `email_outbox` collection for pending/failed rows.

Backup: the `legnaris-prod_mongo_data`, `..._prometheus_data`, `..._loki_data`
volumes.

## 13. Gotchas (learned in testing)

1. **oauth2-proxy `cookie_secret`** requires **URL-safe** base64 (`tr '+/' '-_'`).
2. **Project name** of the deploy is `legnaris-prod` (scopes this stack's
   volumes/networks, avoiding collisions with other projects).
3. **Traefik `/metrics`** needs `metrics.prometheus.entryPoint: metrics`.
4. **The OAuth callback** lives on `ops.` (not `api.`), because that's where the
   login happens.
5. **`~/.cloudflared` must stay world-readable-ish**: the `cloudflared` image runs
   as uid 65532 and reaches the mount as that user, so a `700` directory (what
   `cloudflared tunnel login` creates) makes the container fail with a
   credentials error. It also has no shell, so the failure shows up as the
   tunnel container restarting, not as a readable error inside it.

6. **A stale iptables backend can black-hole container egress** (Docker-in-Docker
   dev container only, never production). When a daemon restart switches the
   iptables backend, the abandoned one keeps its rules in the kernel: a
   `FORWARD policy DROP` chain that only knows the default bridge, so every
   packet leaving `legnaris-prod_edge`/`_internal` is dropped. The Docker
   embedded resolver (`127.0.0.11`) still answers container names but forwards
   external lookups as the container IP, so the symptoms look like a DNS or
   application bug: `cloudflared` never registers an edge connection
   (`lookup _v2-origintunneld._tcp.argotunnel.com ... i/o timeout`), so
   `app.`/`api.`/`ops.` stop answering, and `app` crash-loops with a Mongo
   `server selection error` — while the host itself has working internet.
   Diagnose and repair with `sudo make net-check` then `sudo make net-fix`
   (the repair backs up the removed rules and verifies a real container lookup;
   `cloudflared` and `app` recover on their own once egress is back).
7. **One unsupported key in `traefik/dynamic.yml` disables every route**: the
   file provider rejects the whole configuration (`field not found, node: ...`
   in `docker logs traefik`), so all routers vanish and requests get a plain 404
   with no CORS headers — which the browser reports as a CORS error on login or
   registration. Traefik v3.3 has no `crossOriginOpenerPolicy` /
   `crossOriginResourcePolicy` header options; COOP/CORP are set through
   `customResponseHeaders`. `go test ./deploy/...` fails on any headers option
   the pinned version does not support.
