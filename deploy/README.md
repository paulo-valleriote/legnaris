# Legnaris — Production stack (Docker Compose + Traefik)

Self-contained stack for a single VPS: the Go app (Echo v5) + MongoDB +
observability (Prometheus, Grafana, Loki, Alloy) + a database UI (Mongo
Express), with admin access protected by **GitHub OAuth** (oauth2-proxy) and
**Traefik** at the edge.

---

## 1. Architecture and domains

```
legnaris.com.br          → Vercel (frontend)           [untouched]
api.legnaris.com.br      → Docker/Traefik              [public, no OAuth]
   └─ /                  → app API (health, templates, sheets)
ops.legnaris.com.br      → Docker/Traefik              [GitHub OAuth]
   ├─ /oauth2/callback   → GitHub OAuth callback
   ├─ /grafana           → Grafana
   ├─ /prometheus        → Prometheus
   └─ /mongo             → Mongo Express
```

Why split into two subdomains? The frontend (Vercel) and the API/observability
live on different hosts, each with its own security rule. The `api.` is public;
the `ops.` requires a GitHub login.

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

The apex `legnaris.com.br` already points to Vercel. You only **add** two new
records (`api` and `ops`) — leave what already exists untouched.

### Traefik — the "doorman" (reverse proxy)
Traefik sits in front of everything, listening on ports 80/443. It is **not**
your web server — it decides, for each request, which service should handle it,
based on the **Host** (requested domain) and the **path**:

- Requests `api.legnaris.com.br/...` → forwards to the app.
- Requests `ops.legnaris.com.br/grafana` → forwards to Grafana, but first asks
  oauth2-proxy ("is this person logged in?").

It also handles **TLS/HTTPS**: it issues and renews certificates via Let's
Encrypt.

### Routing — Host vs path
- **By Host** (`api.` vs `ops.`): different domains → different services.
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
# fill in: API_DOMAIN, OPS_DOMAIN, ACME_EMAIL, GITHUB_CLIENT_ID,
# GITHUB_CLIENT_SECRET, GITHUB_USER, CORS_ALLOWED_ORIGINS, MONGO_ROOT_PASSWORD,
# MONGO_APP_PASSWORD, MONGO_EXPORTER_PASSWORD, MONGO_VIEW_PASSWORD,
# ME_CONFIG_BASICAUTH_USERNAME/PASSWORD, GRAFANA_ADMIN_USER/PASSWORD
openssl rand -base64 32 | tr '+/' '-_'   # use as OAUTH2_PROXY_COOKIE_SECRET
```

MongoDB is created with least-privileged users (provisioned by
`mongodb/init/01-create-users.js` on first boot): `legnaris_app` (read/write),
`legnaris_exporter` (server metrics), and `legnaris_view` (read-only, used by
Mongo Express). Services never connect as `root`.

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
cloudflared tunnel create legnaris
cloudflared tunnel route dns legnaris api.legnaris.com.br   # creates the CNAME automatically
cloudflared tunnel route dns legnaris ops.legnaris.com.br

# 2. configure the tunnel:
cp cloudflared/config.yml.example cloudflared/config.yml
# edit and replace <TUNNEL_UUID> with the value from `cloudflared tunnel list`

# 3. bring it up with the local override:
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build
```

What the local override changes (vs production):
- Traefik uses the shared `traefik.yml` with plain HTTP and **no** Let's Encrypt
  (the production-only TLS flags are dropped; Cloudflare terminates TLS).
- oauth2-proxy sets `OAUTH2_PROXY_COOKIE_SECURE=false` (the origin is HTTP
  behind the tunnel).
- Adds the `cloudflared` service that publishes `api.` and `ops.`.

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

## 11. CORS (frontend → API)

The app allows origins from `CORS_ALLOWED_ORIGINS` (default:
`https://legnaris.com.br` in `.env`). Without it, the browser blocks calls from
Vercel to the API.

## 12. Day-to-day operation

```bash
docker compose ps
docker compose logs -f app traefik oauth2-proxy
docker compose restart app
docker compose down          # stop without deleting data
docker compose down -v       # stop AND delete volumes (destroys data)
```

Backup: the `legnaris-prod_mongo_data`, `..._prometheus_data`, `..._loki_data`
volumes.

## 13. Gotchas (learned in testing)

1. **oauth2-proxy `cookie_secret`** requires **URL-safe** base64 (`tr '+/' '-_'`).
2. **Project name** of the deploy is `legnaris-prod` (scopes this stack's
   volumes/networks, avoiding collisions with other projects).
3. **Traefik `/metrics`** needs `metrics.prometheus.entryPoint: metrics`.
4. **The OAuth callback** lives on `ops.` (not `api.`), because that's where the
   login happens.
