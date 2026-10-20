# Quickstart: Auth Prerequisites

**Feature**: `.specs/features/015-auth-prerequisites/` | **Date**: 2026-10-20

Runnable validation for the four seams. Contracts and the data model hold the normative detail; this
guide only proves the feature end to end.

## Prerequisites

- Go 1.26.4 (`go version`).
- MongoDB 7.0 locally: `make dev-db` (uses `DATABASE_URI` from the repo-root `.env`).
- A repo-root `.env` with at least `DATABASE_URI` and `JWT_SECRET` (see `.env.example`).
- No broker, email provider or client build is needed for this feature.

## 1. Repository gates

```bash
make fmt && make lint && make vet && make lint-specs
make test-race
```

Expected: all clean, every `PRQ-*` citation resolves, no race report. The migration and repository
integration suites self-skip when MongoDB is unavailable; run them with `make dev-db` up to include
them.

## 2. Unit and boundary suites

```bash
# Client address rule and the route wiring (no database required)
go test ./internal/platform/clientip ./internal/app -run 'ClientIP|RateLimit' -v

# Canonical email, migration plan, refresh order, clock boundaries
go test ./internal/user ./internal/auth -run 'Canonical|EmailMigration|Refresh|Clock|Token' -v

# Configuration default and optional pass-through
go test ./shared/env ./deploy -v
```

Expected highlights:

- `clientip`: a forged chain from an untrusted peer returns the peer; a missing/malformed chain
  returns the peer; `host:port` and bracketed IPv6 entries resolve; the result is never `""`.
- `app`: two clients behind one trusted peer are limited independently; the second request from one
  client is 429 while the other still reaches the handler.
- `user`: a 254-byte canonical address is accepted, 255 bytes is `ErrInvalidEmail`; dots and `+tags`
  are preserved.
- `user` migration plan: a collision or an invalid row blocks the plan and writes nothing.
- `auth`: refresh with a missing account returns 401 and revokes the family; a store failure returns
  500 and issues nothing; the exact expiry instant is expired and one nanosecond before is valid.
- `env`: `TRUSTED_PROXY_CIDRS` unset yields the documented default; an invalid entry fails the
  policy build.

## 3. Client address through the running API

Start the API (`make run`, default `:8080`). The direct peer is loopback, which the default policy
trusts, so a forwarded chain is honoured exactly as the edge proxy's would be.

```bash
# Client A: exhaust the per-address login limit (10 attempts per minute by default)
for i in $(seq 1 11); do
  curl -s -o /dev/null -w '%{http_code}\n' \
    -H 'Content-Type: application/json' \
    -H 'X-Forwarded-For: 203.0.113.10' \
    -d '{"email":"client-a@example.com","password":"wrong-password"}' \
    http://localhost:8080/auth/login
done
```

Expected: ten `401` (invalid credentials) then `429` with `Retry-After`. Then:

```bash
# Client B, same proxy, different address: still admitted
curl -s -o /dev/null -w '%{http_code}\n' \
  -H 'Content-Type: application/json' \
  -H 'X-Forwarded-For: 203.0.113.11' \
  -d '{"email":"client-b@example.com","password":"wrong-password"}' \
  http://localhost:8080/auth/login
```

Expected: `401` (not `429`). The forged-header-from-an-untrusted-peer case cannot be produced from
localhost because loopback is trusted by design; `internal/app/client_ip_test.go` covers it with an
explicit `RemoteAddr`.

## 4. Canonical email through the running API

```bash
# Register with a display name and capitals
curl -s -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"  Player One <Player@Example.COM>  ","username":"player-one","password":"sup3r-secret"}'

# A second spelling of the same mailbox must conflict
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"player@example.com","username":"player-two","password":"sup3r-secret"}'

# Sign in with yet another spelling reaches the same account
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"Another Name <player@example.com>","password":"sup3r-secret"}'
```

Expected: `201` with a session body; `409`; `200`. In MongoDB the stored `email` is
`player@example.com` with no display name or whitespace:

```bash
docker exec -it legnaris-mongo mongosh "$DATABASE_URI" --quiet \
  --eval 'db.users.find({}, {email:1, email_verified_at:1}).toArray()'
```

Expected: canonical `email`; no `email_verified_at` on any account.

## 5. Migration command

Seed a legacy row (an address that was stored raw before canonicalization):

```bash
docker exec -it legnaris-mongo mongosh "$DATABASE_URI" --quiet --eval '
db.users.insertOne({
  _id: "01900000-0000-7000-8000-0000000000aa",
  email: "Legacy Name <Legacy@Example.COM>",
  username: "legacy", password_hash: "x",
  created_at: new Date(), updated_at: new Date()
})'
```

Dry run (the default, writes nothing):

```bash
go run ./cmd/canonicalize-emails
echo "exit=$?"
```

Expected: `scanned: …`, one `change: …` line from the display-name form to `legacy@example.com`,
`dry run: 1 changes not written`, `exit=0`. The stored row is unchanged.

Apply, then prove idempotency:

```bash
go run ./cmd/canonicalize-emails -apply
echo "exit=$?"
go run ./cmd/canonicalize-emails
echo "exit=$?"
```

Expected: first apply reports `written: 1`, `exit=0`; the second run reports 0 changes and writes 0
rows, `exit=0`. Only `email` and `updated_at` changed.

Collision (nothing written, unsuccessful exit):

```bash
docker exec -it legnaris-mongo mongosh "$DATABASE_URI" --quiet --eval '
db.users.insertOne({_id:"01900000-0000-7000-8000-0000000000bb",email:"LEGACY@example.com",
username:"legacy-2",password_hash:"x",created_at:new Date(),updated_at:new Date()})'
go run ./cmd/canonicalize-emails -apply
echo "exit=$?"
```

Expected: a `collision:` line naming both ids, `exit=1`, and both rows still hold their original
addresses. Resolve the duplicate manually before re-running; the command never merges accounts.

Unparseable row: seed `email: "not-an-address"`, run `-apply`; expected `invalid:` line, `exit=1`,
no writes.

In the deployed stack:

```bash
docker compose -f deploy/docker-compose.yml run --rm --entrypoint /canonicalize-emails app
docker compose -f deploy/docker-compose.yml run --rm --entrypoint /canonicalize-emails app -apply
```

## 6. Refresh account check

Against a running API with MongoDB:

1. Register and keep the refresh cookie (`curl -c cookies.txt …`).
2. Delete the account:
   `docker exec -it legnaris-mongo mongosh "$DATABASE_URI" --quiet --eval 'db.users.deleteOne({email:"player@example.com"})'`.
3. Refresh with the cookie and the CSRF header:
   `curl -b cookies.txt -H "X-CSRF-Token: <value>" -X POST http://localhost:8080/auth/refresh`.

Expected: `401` with code `UNAUTHORIZED`, no `Set-Cookie`, and the refresh family revoked (the next
attempt with the same cookie is also `401`). With the account present, refresh returns `200` and the
new access token's `sub` is that account's id.

## 7. Deployment checks

```bash
# Required configuration is unchanged
go test ./deploy -run TestAppServiceProvidesRequiredEnv -v

# Optional proxy set reaches the container only when configured
TRUSTED_PROXY_CIDRS=10.0.0.0/8 docker compose -f deploy/docker-compose.yml config | grep TRUSTED_PROXY_CIDRS

# The image carries both binaries
docker build -t legnaris-app:local . && docker run --rm --entrypoint /canonicalize-emails legnaris-app:local -h
```

Expected: the required-env test passes; the compose render shows the pass-through; the migration
binary prints usage. With `TRUSTED_PROXY_CIDRS` unset, the API still trusts loopback plus private and
link-local networks, so the deployed proxy path works without configuration.

## 8. Success criteria map

| Criterion | Proven by |
|---|---|
| `SC-001` independent clients behind the proxy | §3, `internal/app/client_ip_test.go` |
| `SC-002` forged header has no effect | `clientip_test.go`, `client_ip_test.go` |
| `SC-003` no credential without an account | §6, `internal/auth/service_test.go` |
| `SC-004` canonical storage and no duplicate | §4, `email_test.go`, integration suite |
| `SC-005` no account carries a verification timestamp | §4, repository integration test |
| `SC-006` typed code/status and race-clean suite | §1, boundary suites |
| `SC-007` boundaries without sleeping | `service_test.go`, `token_test.go` |
| `SC-008` migration no-op/collision writes nothing | §5, `email_migration_test.go` |
