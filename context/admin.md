# Admin

## Purpose

The backoffice is the operator-only, API-only surface under `/admin/*`: read an
account's plan, subscription and quotas, grant or revoke a comp, reset today's
generation counter, queue a refund. Every write appends an `admin_audit` row.

## Why

- **API only** over a UI because the surface is small and rarely used, and a UI
  would duplicate session handling and drift; cost: curl and database access.
- **A boot-time allow-list** over a role model because users carry no roles; cost:
  every admin is all-powerful, and a change needs a restart.
- **404 for every identity or routing failure** over 403/405 because anything else
  confirms the routes exist and the caller is not allowed; cost: a stale token
  looks like a wrong path.
- **The audit inside the mutation transaction** over a follow-up write because the
  row is the only record of who moved a paying account, and a crash between the
  two writes would lie either way; cost: MongoDB transactions, and a failing audit
  aborts the operation.
- **A dedicated mTLS host** (LGN-262) over IP allow-lists or a VPN because
  operators move between networks and the edge already terminates TLS; the
  public API host answers 404 for `/admin/*`. Cost: client certificates to
  issue, and revocation is a CA rotation plus a deploy.
- **The refund queue** over a synchronous provider call because refunds already
  retry and dead-letter; cost: `202 requested` is all the operator sees.

## Flows

The gate is the same for every route: an unmatched method or path, a missing or
invalid bearer token, an id outside `ADMIN_USER_IDS` and a missing or deleting
admin all answer `404 NOT_FOUND`. Only a store failure (500) and the per-user
limiter (429) answer differently. Reads write no audit row.

```mermaid
sequenceDiagram
    Controller->>Service: action, reason, request id, client ip
    Service->>Service: validate id, reason, plan, expiry
    Service->>Transaction: WithTransaction
    Transaction->>Transaction: re-read target, refuse a deleting account
    Transaction->>Transaction: entity write, then Step(after_entity_write)
    Transaction->>Transaction: SyncPlan, plan.changed in the outbox on comp flows
    Transaction->>admin_audit: insert the audit row
    alt commit
        Transaction-->>Controller: 201, 200 or 202
    else failure
        Transaction-->>Controller: rollback, no entity change and no audit row
    end
```

## Contracts

| Method | Path | Body | Success |
| --- | --- | --- | --- |
| GET | `/admin/users/{id}` | — | 200 consolidated view |
| GET | `/admin/users?email=` | — | 200 consolidated view |
| POST | `/admin/users/{id}/comp` | `planId`, optional `expiresAt`, `reason` | 201 subscription |
| POST | `/admin/users/{id}/comp/revoke` | `reason` | 200 `{"status":"canceled"}` |
| POST | `/admin/users/{id}/quota/reset` | `reason` | 200 `{"status":"reset"}` |
| POST | `/admin/users/{id}/refund` | `override`, `reason` | 202 `{"status":"requested"}` |

**Allow-list.** `ADMIN_USER_IDS` is comma-separated canonical user ids (the
`users._id` value), trimmed with blanks dropped; unset or empty disables every
route. Each id is validated at boot and a malformed, uppercase or non-v7 value
makes the API refuse to start, naming the value. Requests authenticate with a
normal access token in `Authorization: Bearer`, not the cookie session.

**Errors.**

| Status | Code | When |
| --- | --- | --- |
| 400 | `INVALID_REQUEST` | malformed id, blank reason, reason over 500 runes or with control/format characters, free or unknown plan, expiry under an hour ahead, missing or malformed e-mail |
| 400 | `INVALID_JSON` / `UNKNOWN_FIELD` | unreadable body or unknown body field |
| 404 | `NOT_FOUND` | bad or missing token, id outside the allow-list, missing or deleting admin, unknown user, no live comp, unknown `/admin` path, wrong method, any `OPTIONS` |
| 409 | `CONFLICT` | target deleting, live subscription in `pending`/`active`/`past_due`, live subscription not a comp, comp changed concurrently |
| 409 | `REFUND_NOT_ELIGIBLE` | `details.reason` is `window_expired`, `already_used`, `not_subscribed` or `comp_not_refundable` |
| 429 | `RATE_LIMITED` | `/admin/*` shares the other user routes' per-user budget |
| 500 | `INTERNAL_ERROR` | store, plan-sync or transaction failure; nothing committed |

**Reason.** Mandatory on every write, trimmed, one to 500 runes, no control or
format characters, stored verbatim.

**View.** `user{id,email}`, `plan` from `users.plan` (the single source of truth),
`subscription` (null when none),
`daily_quota{limit,used,remaining,resets_at,binding}` and
`trial{allowance,used,remaining}`. `binding` is `trial` for a free plan, where
`limit` is null and the lifetime trial block governs generation, and `daily`
otherwise. The e-mail lookup tries the canonical lowercase address, then the
parsed spelling.

**Operations.** `planId` must be a real non-free plan and `expiresAt` at least one
hour ahead; an expired comp is CAS-canceled before the new one is inserted.
Revoke cancels the live comp immediately, and repeating it after a concurrent
cancel still answers 200 with a fresh audit row. Quota reset deletes today's
counter (idempotent, trial untouched). `override` skips only the refund window,
never a second refund, a missing paid subscription or a comp.

**Audit.** `_id` (UUID v7), `actor_user_id`, `action` (`comp.grant`,
`comp.revoke`, `quota.reset`, `refund.request`), `target_user_id`, `payload`,
`reason`, `request_id`, `client_ip`, `created_at`. `request_id` is the
server-generated `X-Request-ID` (a client value is ignored) and `client_ip` the
resolved client address. There is no update or delete path; boot indexes
`created_at`, target plus `created_at` and actor plus `created_at`.

## Code map

- `backend/internal/admin` — types, ports and messages in `admin.go`, operations
  in `service.go`, the insert-only audit store in `store.go`.
- `backend/internal/app` — `admin.go` routes, `router.go` group and 404 masks,
  `bootstrap.go` assembly; `backend/internal/auth/admin.go` the two gates.
- `backend/shared/env/env.go` for `ADMIN_USER_IDS`, `backend/shared/db/indexes.go`
  for the `admin_audit` indexes.
- `backend/internal/billing` — `comp_expiry.go`, `refund.go`, `store_comp.go`,
  `store_refund.go`, `plan_sync.go`: the rules the backoffice reuses.
- `.github/workflows/ci.yml` — the always-running `gate` job checks every
  tracked `.md` against `context/markdown-allowlist.txt` through
  `backend/cmd/markdown-check`; the same command backs the Claude Code
  `PreToolUse` hook in `.claude/settings.json`.

- `deploy/traefik/dynamic.yml` — the `admin` router and `admin-mtls` TLS options;
  `deploy/traefik/admin-client-ca.pem` the public admin CA;
  `deploy/admin/mtls.sh` issues and revokes client certificates
  (`deploy/admin_mtls_test.go`).

## Tools & dependencies

- MongoDB with replica-set transactions; one `db.Transactor` guards the entity
  write, the plan sync and the audit row.
- The billing ledger and plan sync, quota store, plan source, events outbox, the
  relay queues `billing_refunds` and `billing_comp_expiry`, `idgen`, `apperr`,
  `json` and the per-user limiter; no admin UI or external backoffice SDK.

## Known limits

- Per-admin step-up is still open (LGN-263); the mTLS certificate and the
  allow-list are the only factors beyond the session.
- Traefik checks no revocation list: revoking a client certificate rotates the
  admin CA, and the old certificate works until that rotation is deployed.
- No role model, per-action permission or read auditing; the allow-list is
  all-or-nothing and only changes on restart.
- The audit has no retention or rotation; it grows forever and is queryable only
  through the database.
- The limiter can answer 429, the one gate failure not masked as 404.
- Refunds are asynchronous: 202 means queued on `billing_refunds`; re-read the
  account to confirm the provider refund and cancellation.
- Comp expiry is a system transition: `billing_comp_expiry` cancels expired comps
  hourly without an audit row, so a plan change can have no `admin_audit` entry.
- One account per call; no bulk operation and no search beyond an exact e-mail.
