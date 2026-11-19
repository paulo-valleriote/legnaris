# Admin backoffice runbook

The admin backoffice is **API only**: there is no UI. It exposes support
operations for Plans, quotas and refunds under `/admin/*`. Every admin action is
recorded in the append-only `admin_audit` collection in the same MongoDB
transaction as the mutation it describes.

> **Production exposure (LGN-262).** `/admin/*` is served only on
> `ADMIN_DOMAIN` (default `admin-api-legnaris.paulovalleriote.io`), and the TLS
> handshake there requires a client certificate signed by the admin CA. The
> public API host answers `404` for `/admin/*`. OTP step-up for admins is
> LGN-263; until it lands, keep `ADMIN_USER_IDS` limited to controlled accounts.

## Client certificates (mTLS)

The CA private key and every client key live only on the operator's machine,
in `ADMIN_CA_DIR` (default `~/.config/legnaris/admin-ca`). Back that directory
up offline. Only the public CA certificate is committed, as
`deploy/traefik/admin-client-ca.pem`, and Traefik mounts it read-only.

```bash
deploy/admin/mtls.sh init-ca deploy/traefik/admin-client-ca.pem
deploy/admin/mtls.sh issue alice 90
deploy/admin/mtls.sh list
```

`issue` writes `clients/<name>/cert.pem` and `key.pem` (at most 365 days). Call
the API with both:

```bash
curl --cert ~/.config/legnaris/admin-ca/clients/alice/cert.pem \
     --key  ~/.config/legnaris/admin-ca/clients/alice/key.pem \
     -H "Authorization: Bearer $TOKEN" \
     https://admin-api-legnaris.paulovalleriote.io/admin/users?email=x@example.com
```

Traefik does not check revocation lists, so revoking rotates the CA:

```bash
deploy/admin/mtls.sh revoke alice deploy/traefik/admin-client-ca.pem
```

The revoked certificate keeps working until the new CA is deployed: commit the
updated `admin-client-ca.pem`, merge to `main`, then hand each remaining
operator their reissued `cert.pem` (keys are unchanged). Renewing an expired
certificate is a `revoke` followed by an `issue` under the same name.

Expected answers: no or unknown certificate fails the TLS handshake; a valid
certificate on any path other than `/admin/*` is `404`; a request whose SNI is
another host but whose `Host` is the admin host is `421`.

## Access control

`ADMIN_USER_IDS` is a comma-separated list of canonical user IDs (lowercase
UUID v7, the same value as `users._id`). An empty value disables the backoffice.

- Every ID is validated at boot with `idgen.Validate`. A malformed, uppercase or
  non-v7 ID makes the API refuse to start with an error naming the offending
  value.
- `/admin/*` requires a normal authenticated session **and** a user ID in the
  allowlist. Any failure — missing token, invalid token, valid token outside the
  allowlist, an admin account that is being deleted — answers `404 NOT_FOUND`, so
  the routes stay hidden from ordinary users.
- A wrong HTTP method or an unknown `/admin/*` subpath also answers the same
  `404 NOT_FOUND` (never `405`), including with a valid admin token.
- Reads are not audited; writes are.

Set it in the API environment and restart:

```bash
export ADMIN_USER_IDS="0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f,0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e10"
make run
```

Find the ID of the operator account:

```bash
mongosh "$DATABASE_URI" --quiet \
  --eval 'db.users.findOne({email:"operator@example.com"},{_id:1})'
```

## Authenticate a curl session

```bash
TOKEN=$(curl -sS -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"operator@example.com","password":"..."}' \
  | jq -r '.data.access_token')
```

Every call below sends `Authorization: Bearer $TOKEN`.

## Error contract

| Status | Code | When |
|--------|------|------|
| 400 | `INVALID_REQUEST` | malformed user id, missing/blank/overlong reason, reason with control characters, missing or malformed email, unknown/free plan, `expiresAt` earlier than one hour ahead |
| 400 | `INVALID_JSON` / `UNKNOWN_FIELD` | malformed body or an unknown field |
| 404 | `NOT_FOUND` | not an admin, unauthenticated, deleting admin, wrong method or unknown `/admin/*` path (routes hidden), unknown user, no live comp |
| 409 | `CONFLICT` | the target account is being deleted, any live subscription exists (`pending`, `active` or `past_due`), live subscription is not a comp, comp state changed concurrently |
| 409 | `REFUND_NOT_ELIGIBLE` | refund window closed without override, not a paid subscription, refund already used, comp subscription |
| 500 | `INTERNAL_ERROR` | store or transaction failure |

`reason` is mandatory on every write. It is trimmed, must be non-empty, is
capped at 500 Unicode characters and must not contain control characters
(`\n`, `\r`, `\t`, …).

On `500`, no mutation was committed (the audit row is written in the same
transaction). Re-read the account with `GET /admin/users/{id}` before retrying;
the retry recipe is: re-GET the view, confirm the operation is still needed, then
repeat the same request with a fresh request ID.

## Consolidated user view

### `GET /admin/users/{id}`

```bash
curl -sS "http://localhost:8080/admin/users/$TARGET_ID" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### `GET /admin/users?email=`

The address is parsed with `mail.ParseAddress`. The canonical lowercase address
is tried first (`Player@Example.COM` works); if that misses, a legacy mixed-case
row is retried with the parsed address (case preserved), not the raw input.

```bash
curl -sS "http://localhost:8080/admin/users?email=player@example.com" \
  -H "Authorization: Bearer $TOKEN" | jq
```

Sample response:

```json
{
  "data": {
    "user": {"id": "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e0f", "email": "player@example.com"},
    "plan": "paid",
    "subscription": {
      "id": "0196e4f2-6a1b-7c3d-8e4f-5a6b7c8d9e11",
      "provider": "comp",
      "plan_id": "paid",
      "status": "active",
      "current_period_end": "2026-12-18T12:00:00Z",
      "cancel_at_period_end": false,
      "refund_requested_at": null,
      "refund_used_at": null
    },
    "daily_quota": {"limit": 30, "used": 4, "remaining": 26, "resets_at": "2026-11-19T00:00:00Z", "binding": "daily"},
    "trial": {"allowance": 3, "used": 1, "remaining": 2}
  }
}
```

`plan` comes from `users.plan`, the single source of truth (LGN-211). It is only
changed by the billing consumer, admin comp operations and the comp expiry
sweeper, always in the same transaction as the subscription write.

`daily_quota.binding` tells which limit governs generation: `daily` for paid
users (`limit: 30`) or `trial` for free users (`limit: null`; the lifetime trial
block is the binding constraint). `used`/`remaining` still report the daily
counter for diagnosis.

## Grant a comp

### `POST /admin/users/{id}/comp`

```bash
curl -sS -X POST "http://localhost:8080/admin/users/$TARGET_ID/comp" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "planId": "paid",
    "expiresAt": "2026-12-18T12:00:00Z",
    "reason": "LGN-123 support comp"
  }' | jq
```

- `expiresAt` is optional; omit it for a comp with no expiry.
- `expiresAt` must be at least one hour ahead of the server clock.
- `planId` must be a real paid plan; `free` and unknown ids are rejected.
- Answers `201` with the created subscription.
- Answers `409 CONFLICT` when the user already has a live subscription
  (`pending`, `active` or `past_due`), when the target account is being deleted,
  or when a concurrent state change wins.
- An expired comp (`provider: comp`, `status: active`,
  `current_period_end <= now`) does **not** block a replacement. The expired row
  is CAS-canceled and the new comp inserted in the same transaction as the plan
  recompute and `plan.changed` event.

## Revoke a comp

### `POST /admin/users/{id}/comp/revoke`

```bash
curl -sS -X POST "http://localhost:8080/admin/users/$TARGET_ID/comp/revoke" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"reason":"chargeback investigation resolved"}' | jq
```

The live comp is canceled immediately (`status: canceled`), not at period end.
The user's plan is recomputed from the remaining live subscriptions and a
`plan.changed` event is emitted in the same transaction.

- `404` when the user has no live subscription.
- `409` when the live subscription is not a comp or the target account is being
  deleted.
- If the comp was canceled by a concurrent request between the read and the CAS
  write, the re-read confirms `canceled` and the call still answers `200` with an
  audit row. A comp that remains active after the lost race is a real conflict
  (`409`).

## Reset the daily quota

### `POST /admin/users/{id}/quota/reset`

Deletes the current UTC day counter. The call is idempotent: resetting a user
with no counter is a success. The lifetime trial allowance is untouched. A
target account that is being deleted answers `409 CONFLICT`.

```bash
curl -sS -X POST "http://localhost:8080/admin/users/$TARGET_ID/quota/reset" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"reason":"stuck generation reported by support"}' | jq
```

## Request a refund

### `POST /admin/users/{id}/refund`

Reuses the normal refund flow and queue: the subscription moves to
`refund_state: requested`, and the relay worker performs the provider refund and
the cancellation. The audit row is written in the same transaction as the state
change.

```bash
curl -sS -X POST "http://localhost:8080/admin/users/$TARGET_ID/refund" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"override": false, "reason": "customer request"}' | jq
```

- `override: false` applies the normal 7-day window.
- `override: true` skips **only** the window. It still refuses a double refund,
  a user without a paid subscription and a comp subscription.
- A target account that is being deleted answers `409 CONFLICT`.
- Answers `202` with `{"status":"requested"}`.
- Refusals use `REFUND_NOT_ELIGIBLE` with a `details.reason` of
  `window_expired`, `already_used`, `not_subscribed` or `comp_not_refundable`.

## Comp expiry

The relay sweeper queue `billing_comp_expiry` runs hourly. It claims up to 100
expired comps per batch and loops until no expired comp remains. Each row is
canceled by a CAS write in its own transaction; the same transaction recomputes
`users.plan` from the remaining live subscriptions and emits `plan.changed` when
the plan changes. The sweep logs the number of expired comps it canceled.

An expired comp (`status: active`, `current_period_end <= now`) is never
entitled: it does not block a new comp grant or checkout and does not keep the
user on `paid` once a plan sync runs. The sweeper writes no audit row: it is a
system transition, not an admin action.

## Audit trail

Every write appends one document to `admin_audit`:

| Field | Meaning |
|-------|---------|
| `_id` | UUID v7 generated by the server |
| `actor_user_id` | admin session user ID |
| `action` | `comp.grant`, `comp.revoke`, `quota.reset`, `refund.request` |
| `target_user_id` | affected account |
| `payload` | action-specific data (`plan_id`, `expires_at`, `subscription_id`, `override`, `scope`) |
| `reason` | operator text, trimmed, at most 500 characters, no control characters |
| `request_id` | `X-Request-ID` generated by the router |
| `client_ip` | resolved client address |
| `created_at` | UTC timestamp |

The collection is append-only from the application: there is no update or delete
path. Inspect a user's history:

```bash
mongosh "$DATABASE_URI" --quiet --eval '
  db.admin_audit.find({target_user_id: "'"$TARGET_ID"'"}).sort({created_at: -1}).limit(20)
'
```

If the mutation fails, the transaction rolls back and no audit row is written.
