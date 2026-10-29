# Legnaris

Backend (Go + MongoDB) and web client (React PWA). Production runs behind
Traefik with TLS and an OAuth gate for the ops tools — see
[`deploy/README.md`](deploy/README.md). None of that is needed for development.

## Local development

One command starts everything you need to debug, on plain `http://localhost`:

```bash
make dev
```

| Piece | Address                                | Notes                                         |
| ----- | -------------------------------------- | --------------------------------------------- |
| Web   | <http://localhost:5173>                | Vite dev server with hot reload               |
| API   | <http://localhost:8080>                | Built from `backend/cmd/api.go`               |
| DB    | `mongodb://localhost:27017/` (no auth) | Docker container `legnaris-mongo`, always new |

Open <http://localhost:5173>, register and sign in. Press `Ctrl-C` to stop: the
API, the web server and the MongoDB container are all shut down.

**The database is always clean.** Every `make dev` removes any previous
`legnaris-mongo` container and starts a fresh one without a volume, and it is
removed again on exit. Nothing survives between runs.

### Prerequisites

- Docker (for MongoDB), Go (version in `go.mod`) and Node.js 24 (`web/.nvmrc`).
- No `.env` is required. The script falls back to development-only secrets
  (`JWT_SECRET`, `EMAIL_CODE_SECRET`); values from the environment or from the
  root `.env` take precedence. Never reuse those fallbacks outside development.
  `DATABASE_URI` from `.env` is ignored by `make dev` because it talks to its own
  throwaway MongoDB.
- Use **Chrome, Edge or Firefox**. The session cookies are `Secure` +
  `__Host-` prefixed (a fixed contract); those browsers accept them on
  `http://localhost`, Safari does not.

### Changing ports

```bash
PORT=9000 WEB_PORT=3000 MONGO_PORT=27018 make dev
```

The script derives `CORS_ALLOWED_ORIGINS`, `VITE_API_BASE_URL` and the Vite
dev-server CSP from those values, so nothing else needs editing.

### What does not run in development

Traefik, oauth2-proxy, the Cloudflare tunnel, RabbitMQ, the email relay/worker
and the observability stack. Sign-in works without email because
`AUTH_REQUIRE_EMAIL_VERIFICATION` is off by default; to exercise the email-code
flow you need a broker (`make relay`, `make worker`, see `deploy/README.md`).

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

## Other commands

`make test`, `make test-race`, `make lint`, `make vet`, `make lint-specs`,
`make fmt` — see the [`Makefile`](Makefile) and [`AGENTS.md`](AGENTS.md). The web
client has its own [`README`](web/README.md).
