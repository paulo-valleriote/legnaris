# Runbook — dead sweeper items (LGN-209, ADR 0013)

The relay's sweeper drives every pending-work queue. When an item fails
`AttemptCap` times, it becomes `dead`: the sweeper never claims it again, and an
operator decides what happens next. This page says how to find dead items, how to
triage them and how to requeue or discard them. Each queue has one entry below,
written from the template at the end.

> **Status:** written from the repository only; nothing here has been executed
> against production.

## When this runbook applies

The alerts that point here are defined in `deploy/prometheus/rules-f1a.yml`
(LGN-226):

- `LegnarisSweeperHandlerFailing` (`legnaris_sweeper_failed_total{queue}`
  increased): an attempt failed. A single transient failure is normal; a rising
  rate is not.
- `LegnarisSweeperOldestPendingTooOld` (`legnaris_sweeper_oldest_pending_seconds{queue}`
  above 3600): the oldest pending item has waited more than 1 hour, so the queue
  is stalled or starved.
- `LegnarisEventsUnroutedNotEmpty` (`events.unrouted` depth above zero): see the
  `event_outbox` entry below for the triage.
- `LegnarisGenerationCapAt80Percent`: the global daily generation cap is above
  80%. It is not a dead-item alert; its description covers the staleness after
  UTC midnight.

Start with the queue named by the alert label `queue`.

## Shared triage steps

1. Read the queue's `last_error` on a dead item (the queue entry below says where).
   It is the reason of the final failed attempt.
2. Check the dependency the queue publishes to or calls (broker, provider, Mongo).
   Fix the cause before requeueing; otherwise the items go dead again.
3. Requeue only what is safe to repeat. Every queue's handler must be idempotent
   for the item it works on (for `event_outbox`, consumers deduplicate by the
   envelope `id`).

## Requeue template rule

Requeue sets the item back to pending, eligible now, with a fresh attempt count:

```js
// mongosh, against the queue's collection. Replace the filter with the ids to requeue.
db.<collection>.updateMany(
  { _id: { $in: [ "<id>" ] }, status: "dead" },
  { $set: { status: "pending", next_attempt_at: new Date(), attempts: 0, locked_until: null, locked_by: "" } }
)
```

Never requeue an item whose root cause is still unfixed: it returns to dead after
`AttemptCap` more attempts and only adds noise to `failed_total`.

## Queue: `event_outbox`

- **Collection:** `event_outbox` (database `DATABASE_NAME`).
- **Interval and lease:** the sweeper claims it every 5 minutes with a 5-minute lease.
- **Dead state:** `status: "dead"`, set after `EMAIL_RELAY_MAX_ATTEMPTS` failed
  publishes. A non-canonical `_id` is dead at once.
- **Find:**

  ```js
  db.event_outbox.find(
    { status: "dead" },
    { event_type: 1, attempts: 1, last_error: 1, created_at: 1 }
  ).sort({ created_at: 1 })
  ```

- **Triage:** `last_error` is the broker's nack or the confirm timeout. Check the
  RabbitMQ `legnaris.events` exchange and its bindings. An event with no bound
  queue lands in `legnaris.events.unrouted`, which is an alert of its own
  (`events.unrouted` non-empty).
- **Requeue:** after the broker is healthy, apply the requeue template with the ids
  found above. The envelope `id` is unchanged, so a consumer that already applied
  the event skips the duplicate.
- **Discard:** a dead event that no consumer needs can be deleted, but record the
  `event_type`, `aggregate_id` and `created_at` in the incident note first.

## Queue entry template

Copy this block for each new queue (for example `generation`, `users deleting`,
`billing_events`, `campaigns deleting`). Fill every field; leave none blank.

```
## Queue: `<queue name>`

- **Collection:** `<collection>`
- **Interval and lease:** <interval> with a 5-minute lease.
- **Dead state:** <the status or flag that means dead>, set after
  <attempt cap, and its env or config name> failed attempts. <Any permanent
  failure that is dead at once.>
- **Find:** <a mongosh query that lists dead items, with the fields to read>
- **Triage:** <the dependency to check first, and what the last_error means>
- **Requeue:** <the exact state change, and why it is safe to repeat>
- **Discard:** <when a dead item may be deleted, and what to record first>
```
