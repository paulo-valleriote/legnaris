# Admin backoffice runbook

The admin backoffice is **API only**: there is no UI. It exposes support
operations for Plans, quotas and refunds under `/admin/*`. Every admin action is
recorded in the append-only `admin_audit` collection in the same MongoDB
transaction as the mutation it describes.

> **Production exposure is not settled here.** LGN-262 covers serving `/admin/*`
> behind the mTLS host, and LGN-263 covers OTP step-up for admins. Until both
> land, treat these routes as operator-only and keep `ADMIN_USER_IDS` empty or
> limited to a controlled account.

## Access control

`ADMIN_USER_IDS` is a comma-separated list of canonical user IDs (lowercase
UUID v7, the same value as `users._id`). An empty value disables the backoffice.

- Every ID is validated at boot with `idgen.Validate`. A malformed, uppercase or
  non-v7 ID makes the API refuse to start with an error naming the offending
  value.
- `/admin/*` requires a normal authenticated session **and** a user ID in the
  allowlist. Any failure — missing token, invalid token, valid token outside the
  allowlist — answers `404 NOT_FOUND`, so the routes stay hidden from ordinary
  users.
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
| 400 | `INVALID_REQUEST` | malformed user id, missing/blank/overlong reason, missing or malformed email, unknown/free plan, non-future `expiresAt` |
| 400 | `INVALID_JSON` / `UNKNOWN_FIELD` | malformed body or an unknown field |
| 404 | `NOT_FOUND` | not an admin, unauthenticated (routes hidden), unknown user, no live comp |
| 409 | `CONFLICT` | live paid subscription exists, live comp exists, live subscription is not a comp, comp state changed concurrently |
| 409 | `REFUND_NOT_ELIGIBLE` | refund window closed without override, not a paid subscription, refund already used, comp subscription |
| 500 | `INTERNAL_ERROR` | store or transaction failure |

`reason` is mandatory on every write. It is trimmed, must be non-empty and is
capped at 500 characters.

## Consolidated user view

### `GET /admin/users/{id}`

```bash
curl -sS "http://localhost:8080/admin/users/$TARGET_ID" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### `GET /admin/users?email=`

The address is canonicalized before lookup (`Player@Example.COM` works); a
legacy mixed-case row is also tried verbatim.

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
    "daily_quota": {"limit": 30, "used": 4, "remaining": 26, "resets_at": "2026-11-19T00:00:00Z"},
    "trial": {"allowance": 3, "used": 1, "remaining": 2}
  }
}
```

`plan` is the **effective** plan: an active comp past its `current_period_end`
is ignored immediately, even before the sweeper cancels it.

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
- `expiresAt` must be in the future.
- `planId` must be a real paid plan; `free` and unknown ids are rejected.
- Answers `201` with the created subscription.
- Answers `409 CONFLICT` when the user already has an active live subscription
  (paid or comp), or when a `pending` row wins the unique live index.

## Revoke a comp

### `DELETE /admin/users/{id}/comp`

```bash
curl -sS -X DELETE "http://localhost:8080/admin/users/$TARGET_ID/comp" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"reason":"chargeback investigation resolved"}' | jq
```

The live comp is canceled immediately (`status: canceled`), not at period end.

- `404` when the user has no live subscription.
- `409` when the live subscription is not a comp.

## Reset the daily quota

### `POST /admin/users/{id}/quota/reset`

Deletes the current UTC day counter. The call is idempotent: resetting a user
with no counter is a success. The lifetime trial allowance is untouched.

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
- Answers `202` with `{"status":"requested"}`.
- Refusals use `REFUND_NOT_ELIGIBLE` with a `details.reason` of
  `window_expired`, `already_used`, `not_subscribed` or `comp_not_refundable`.

## Comp expiry

A comp whose `current_period_end` has passed stops counting toward the effective
plan immediately. The relay sweeper queue `billing_comp_expiry` (hourly) then
moves it to `canceled` on the entity, so an operator sees the state and the
reconcile pass no longer treats it as live. The sweeper writes no audit row: it
is a system transition, not an admin action.

## Audit trail

Every write appends one document to `admin_audit`:

| Field | Meaning |
|-------|---------|
| `_id` | UUID v7 generated by the server |
| `actor_user_id` | admin session user ID |
| `action` | `comp.grant`, `comp.revoke`, `quota.reset`, `refund.request` |
| `target_user_id` | affected account |
| `payload` | action-specific data (`plan_id`, `expires_at`, `subscription_id`, `override`, `scope`) |
| `reason` | operator text, trimmed, at most 500 characters |
| `created_at` | UTC timestamp |

The collection is append-only from the application: there is no update or delete
path. Inspect a user's history:

```bash
mongosh "$DATABASE_URI" --quiet --eval '
  db.admin_audit.find({target_user_id: "'"$TARGET_ID"'"}).sort({created_at: -1}).limit(20)
'
```

If the mutation fails, the transaction rolls back and no audit row is written.
