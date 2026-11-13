---
status: accepted
---

# MongoDB runs as a single-node replica set, multi-document writes use transactions, and pending work lives on the entity

ADRs 0009 to 0012 were designed on a standalone MongoDB, which has no multi-document transactions. Every flow that writes two documents therefore had a window where a crash or a concurrent request left half-applied state, covered only by compensations and a daily reconciliation. A review of those ADRs found 17 such gaps: lost or duplicated guaranteed events, GM-exit cascades left half done, member counters that drift, links created by a user demoted a moment earlier, subscription status regressed by parallel consumers, and failures that surface a day later. MongoDB now runs as a single-node replica set, multi-document writes go through one transaction helper, and work that must finish later is recorded on the entity it belongs to and driven by one sweeper. Decided in LGN-181; the spec is posted there.

## Considered Options

- **Transactional outbox for every flow, on standalone MongoDB:** the outbox is transactional only when the entity write and the outbox insert share a transaction, which standalone cannot do. Without it there are two options: embed the pending event in the entity document, which delivers events but does not make two state changes atomic, or turn every cascade into a saga, which is the current design with its windows, compensations and read-time filters. Most of the gaps are state invariants across documents (role and links, member and counter), not event delivery.
- **Event sourcing for the Campaign:** the optimistic `expectedVersion` on a stream is the same mechanism as `campaign.version` below, and appending one event works even on standalone. But invariants across aggregates (a deleted Sheet removes its links, the owned-Campaign limit per Plan, `campaign_id` on character Sheets) still need sagas or transactions. Reads need projections, and authorization needs them consistent. Erasing personal data from an immutable store under LGPD needs crypto-shredding. `docs/TDD.md` already rejected event sourcing for lack of a product need. A read-only history next to the state stays possible if replay or auditing becomes a product need.
- **Three nodes now:** production is one Hostinger VPS. Three members on the same host share its disk and fail together, for three times the RAM and storage. High availability is a separate decision that needs separate hosts.
- **Primary, secondary and arbiter (PSA):** with the secondary down, `w: majority` writes stall, because the arbiter holds no data.
- **Two-phase commit by hand (pending state, compensation, reconciliation):** this is what ADRs 0009 to 0012 do today. It works, but every flow pays for it in code and in inconsistency windows.

## Decisions

- **Replica set:**
  - Every MongoDB service in every Compose stack (production, development, load test, observability) and the integration tests runs as replica set `rs0`, with a keyfile. The init script runs `rs.initiate()` idempotently.
  - The connection string names the replica set and enables retryable writes.
  - Transactions use `w: majority`. Reads that decide authorization, or that a compensation depends on, go to the primary.
  - Moving to three members on separate hosts is a configuration change (`rs.add()`), never a code change.
- **Transactions:**
  - One helper in the shared db package runs a function through the driver's `WithTransaction`, which retries `TransientTransactionError` and `UnknownTransactionCommitResult`. Nothing opens a session by hand.
  - Repositories accept the session-bound context. The helper returns `db` errors, and services translate them to `apperr`.
- **Write-skew rule:**
  - MongoDB transactions are snapshot isolated. Two transactions conflict only when both write the same document, so a transaction that reads a member and writes a link does not conflict with one that demotes that member.
  - **Every Campaign-scoped transaction increments `campaign.version`.** This covers join, leave, promote, demote, remove, link, unlink, reveal, hide and the deletion steps. Two concurrent decisions about one Campaign then conflict, and the driver retries the loser against the new state.
  - The same rule applies to any future aggregate whose invariants span documents.
- **Transactional outbox:**
  - Guaranteed events (`plan.changed`, `user.deleted`) are inserted into `event_outbox` in the same transaction as the entity change. A rolled-back transaction emits nothing.
  - Best-effort events keep the in-process publisher (ADR 0010).
- **Pending work lives on the entity:**
  - Work that must finish after the request is recorded on the entity itself (status, lease and attempt count), as the generation job already does (ADR 0007). It is never an event used as a job.
  - Campaign deletion is `status: deleting` on the Campaign; `campaign.deletion_requested` leaves the catalog.
  - Account deletion is `status: deleting` on the user, then a tombstone.
  - A refund is `refund_state` (`requested`, `refunded`, `canceled`, `failed`) on the subscription.
- **One sweeper:**
  - It runs in the relay every 5 minutes, with 5-minute leases, over `billing_events`, `event_outbox`, Campaigns and users in `deleting`, and refunds in `requested`.
  - After the attempt cap an item becomes `dead` and needs manual action.
  - Metrics are `oldest_pending_seconds{queue}` and `failed_total{queue}`. Alerts fire when the oldest pending item passes 1 hour or when anything fails.
  - The daily reconciliation stays as a safety net.
- **Aggregate version:** the event envelope gains `aggregate.version`, from `sheet_version`, `campaign.version` or `users.plan_version`. The field is additive, so the envelope `version` stays 1. Consumers drop an event older than the last one they applied.

## Consequences

- **ADR 0009:**
  - The billing consumer claims events per user with a lease. It writes a subscription only when the provider read is newer than `provider_synced_at`.
  - `users.plan` is derived from all of the user's subscriptions.
  - Checkout is guarded by a unique partial index on one live subscription per user, plus provider idempotency keys.
  - Refund follows `refund_state`, and webhooks for users in `deleting` are ignored.
- **ADR 0010:**
  - Guaranteed events no longer have the "a crash can still lose the event" gap.
  - `legnaris.events` gets the alternate exchange `legnaris.events.unrouted`, so an event with no bound queue is kept, not dropped.
  - The best-effort publisher drains for up to 5 s on SIGTERM and counts the rest as dropped, reason `shutdown`.
- **ADR 0011:** deletion, membership and counters run in transactions under the write-skew rule. A late link is undone when the Campaign is `deleting` or missing.
- **ADR 0012:**
  - The GM-exit cascade and link creation run in the Campaign transaction.
  - Polling authorizes before evaluating `If-None-Match`.
  - The `ETag` covers `sheet_version`, a new `layout_version` and the viewer's role.
- **`docs/TDD.md`:** the event-sourcing rejection no longer cites the standalone premise.
- **Cost:**
  - The init and Compose files change, and integration tests need a replica set.
  - The oplog uses disk.
  - Refunds become asynchronous for the user.
  - Every Campaign transaction must remember the `campaign.version` increment, so a test fails when it does not.
- **Implementation** follows the spec on LGN-181, starting with the replica set and the transaction helper, because everything else depends on them.
