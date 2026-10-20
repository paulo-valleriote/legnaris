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
   └─ /mongo             → Mongo Express
```

Why split into three subdomains? Each surface has its own security rule: `api.`
and `app.` are public, `ops.` requires a GitHub login. Keeping the client on its
own origin (`app.`) is what lets the API stay a pure JSON service with a CORS
allowlist instead of serving two kinds of traffic from one host and one route
table.

**Threat model:** the `ops.` subdomain is the only administrative surface. It is
gated by GitHub OAuth at the Traefik edge; Grafana, Prometheus, and Mongo
Express run on the `internal` Docker network only (not published, not on
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
# JWT_SECRET, MONGO_ROOT_PASSWORD, MONGO_APP_PASSWORD, MONGO_EXPORTER_PASSWORD,
# MONGO_VIEW_PASSWORD, ME_CONFIG_BASICAUTH_USERNAME/PASSWORD,
# GRAFANA_ADMIN_USER/PASSWORD
openssl rand -base64 32 | tr '+/' '-_'   # use as OAUTH2_PROXY_COOKIE_SECRET
openssl rand -base64 48                  # use as JWT_SECRET
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

## 10. Observability

- **P50/P95/P99 per route**: Grafana → `legnaris_http_request_duration_seconds`
  (labels method/path/status).
- **Structured logs**: Grafana → Explore (Loki), `{service="app"} | json`.
  Correlation via `X-Request-Id`.
- **Edge/DB metrics**: Prometheus scrapes Traefik and MongoDB Exporter.
- **App metrics**: Prometheus scrapes the app's `/metrics` directly on the
  internal network; the endpoint is not routed on the public `api.` domain.
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
