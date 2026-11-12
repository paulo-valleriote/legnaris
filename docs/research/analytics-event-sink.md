# Research: where RabbitMQ domain events land for internal product analytics

Ticket: LGN-172. Status: research, no code changed. Scope: internal product
analytics only (template usage, LLM generation conversion, retention, plan
upgrades). Not a public or user-facing analytics feature.

## 1. Current state (from the repository)

- **Broker**: RabbitMQ 4.4.0 with the management and Prometheus plugins, built
  from `deploy/rabbitmq/Dockerfile` (pinned `rabbitmq:4.4.0-management`). It
  provisions its own vhost `legnaris` on start (`deploy/rabbitmq/provision.sh`,
  `deploy/rabbitmq/entrypoint.sh`). Source: `deploy/rabbitmq/Dockerfile`,
  `deploy/rabbitmq/rabbitmq.conf`, `.specs/STATE.md` (feature 021, 023).
- **Go client**: `github.com/rabbitmq/amqp091-go` v1.16.0, used only inside
  `backend/internal/email/amqp` (topology, confirmed publish, consumer). Binaries
  that touch the broker: `backend/cmd/relay` (publisher) and `backend/cmd/worker`
  (consumer). The API process never imports the broker
  (`.specs/STATE.md`, feature 016 plan).
- **Topology today**: one exchange `legnaris.email`, queues `email.otp` (quorum),
  three retry queues and `email.dead`. The topology is code constants in
  `backend/internal/email/amqp/topology.go`. There is no analytics exchange or
  queue yet, so the events would be a new, separate topology.
- **Prod stack** (`deploy/docker-compose.yml`): about 16 services on one VPS
  (Traefik, oauth2-proxy, app, web, MongoDB 7.0, Redis 7.4 capped at 64 MB,
  RabbitMQ, relay, worker, mongodb_exporter, Prometheus, Loki, Alloy, Grafana,
  Mongo Express). Prometheus is capped by retention (30 d or 20 GB). Most services
  have no `mem_limit`.
- **Resource budget**: no explicit RAM or CPU figure for the VPS is written in
  the repository. `docs/adr/0004` and `.specs/features/024-vps-cd-hardening/spec.md`
  name a Hostinger VPS, but its size is not recorded. This is the first open
  risk (section 7).
- **Grafana already runs** behind GitHub OAuth at `ops-legnaris` (`deploy/README.md`,
  section 1). It is the existing place for operator dashboards.
- **Existing LGPD posture**: `docs/TDD.md` (line ~375 and ~396) commits to account
  deletion removing user, sheet and tokens. `docs/plan-auth-otp-2fa-email.md`
  (section "Legal (LGPD)") says email and IP are personal data and must not be
  logged in full.

## 2. Options compared

| Option | Components added to prod | Published RAM guidance | License | Verdict |
|---|---|---|---|---|
| A. Mongo collection + Grafana (already deployed), later Metabase if needed | 1 consumer binary (Go, same image), 1 queue, 1 Mongo collection | Negligible beyond the existing Mongo and Grafana | Own code; amqp091-go BSD-2 | Recommended |
| B. Mongo collection + Metabase | +1 JVM service (Metabase), +1 app DB or reuse Mongo | Metabase publishes no RAM figure on the pages checked (see section 3) | AGPLv3 with a commercial `enterprise/` directory | Viable second step |
| C. PostHog self-hosted | About 8 services: PostHog app, Caddy, ClickHouse, Kafka, ZooKeeper, Redis, Postgres, MinIO | 4 vCPU, 16 GB RAM, more than 30 GB disk, as a "sizing guide" | MIT for the free Compose deployment; the page states self-hosting is "officially unsupported" | Rejected for this VPS |
| D. ClickHouse + Grafana/Metabase | +1 ClickHouse server (+ Grafana/Metabase) | Not verified (see section 3) | ClickHouse is open source; exact license not verified | Only if volume grows past Mongo |
| E. Umami or Plausible (self-hosted) | +1 app, +1 Postgres (Umami) or +Postgres and ClickHouse (Plausible) | Not verified for Plausible | Umami MIT; Plausible CE AGPLv3 | Not a fit: built for web pageviews from the browser, not server-side domain events |
| F. PostHog Cloud free tier | 0 on the VPS; a SaaS dependency and an outbound flow of user data | Not applicable | Proprietary cloud | Rejected by LGPD posture unless a DPA and region are settled; see section 5 |

## 3. Primary-source facts

Each claim cites the page it came from. Items marked "not verified" were
requested but the fetched page did not contain them.

- **PostHog self-host hardware**: "something equivalent to a Hetzner VM with 4
  vCPU, 16GB RAM, and more than 30GB storage." Services listed:
  `posthog/posthog`, Caddy 2.6.1, ClickHouse 22.3, Kafka (bitnami 2.8.1),
  ZooKeeper 3.7.0, Redis 6.2.7, Postgres 12, MinIO.
  Source: https://posthog.com/docs/self-host/deploy/hobby
- **PostHog license**: "a free Docker Compose deployment under an MIT license";
  self-hosted instances are "officially unsupported." Source:
  https://posthog.com/docs/self-host (and the hobby page above).
- **PostHog Cloud free allowance**: not verified. The pricing page fetch was
  truncated before the numbers. Source checked: https://posthog.com/pricing and
  https://posthog.com/docs/billing/limits-alerts (the latter only confirms alerts at
  80% and 100% of the free allotment).
- **Metabase license**: AGPL, with the Metabase Commercial License for the
  top-level `enterprise/` directory. Source:
  https://github.com/metabase/metabase/blob/master/LICENSE.txt
- **Metabase MongoDB**: MongoDB is listed as a supported database. The page lists
  limitations: TLS certificate parameters (`tlsCertificateKeyFile`,
  `tlsCertificateKeyFilePassword`, `tlsCAFile`) are not supported, and "Metabase
  may not sync all of your fields." Source:
  https://www.metabase.com/docs/latest/databases/connections/mongodb
- **Metabase memory**: not verified. Both the Docker page and the tuning page
  fetched contain no RAM or JVM figure. Sources:
  https://www.metabase.com/docs/latest/installation-and-operation/running-metabase-on-docker
  and https://www.metabase.com/docs/latest/installation-and-operation/tuning-performance
  (the tuning URL returned 404 on a second fetch).
- **Umami license**: MIT. Source: https://github.com/umami-software/umami/blob/master/LICENSE
- **Plausible CE license**: AGPLv3 or later. Source:
  https://plausible.io/docs/self-hosting (license statement) and
  https://github.com/plausible/analytics/blob/master/LICENSE.md. Plausible's
  self-hosting page points to https://github.com/plausible/community-edition/ for
  install requirements, which were not fetched.
- **ClickHouse**: install options listed; RAM and license not verified. Source:
  https://clickhouse.com/docs/en/install
- **amqp091-go license**: BSD 2-clause. Source:
  https://github.com/rabbitmq/amqp091-go/blob/main/LICENSE
- **LGPD (Lei 13.709/2018)**, source: https://www.planalto.gov.br/ccivil_03/_ato2015-2018/2018/lei/l13709.htm
  - Art. 5, I: personal data is "informação relacionada a pessoa natural identificada ou identificável"
  - Art. 5, III: anonymized data is data that "não possa ser identificado"
  - Art. 5, XI: anonymization is the use of reasonable technical means, which is
    the closest definition to pseudonymization in Art. 5 itself
  - Art. 13, §4: pseudonymization is treatment by which a datum "perde a possibilidade"
    of direct association with a person, except with additional information kept
    separately (research context, but the definition is used as the reference)
  - Art. 16: personal data is eliminated after treatment ends, within technical limits
  - Art. 18, IV: anonymization, blocking or deletion of unnecessary or excessive data
  - Art. 18, VI: deletion of data processed with consent, subject to the exceptions in Art. 16
  - Art. 12, §2: data used to form a behavioural profile can count as personal data
  The ANPD guidance pages were not fetched; cite them before shipping (open item).

## 4. Ingestion path from the Go consumer (option A)

1. The application publishes domain events (template chosen, generation
   requested or completed, retention checkpoints, plan change) to a new topic
   exchange, `legnaris.events`, in the existing `legnaris` vhost. This reuses
   the existing confirmed-publish pattern from `backend/internal/email/amqp`.
2. A new binary, `cmd/analytics` (its own process, same runtime image, like
   `cmd/worker`), declares a durable quorum queue `analytics.events` bound to that
   exchange with a dead-letter exchange for poison messages. It uses manual acks
   and `prefetch` as `cmd/worker` does.
3. The consumer decodes with `shared/json` (the bounded decode rule in `AGENTS.md`),
   validates each event against an allow-list schema, and inserts into a Mongo
   collection `analytics_events` with an idempotency key (the AMQP `MessageId`,
   a UUID v7 from `shared/idgen`). A unique index on that key makes redelivery safe.
4. Ack only after the Mongo write succeeds; on write failure, nack with requeue
   up to a cap, then dead-letter.
5. Queries: a Grafana Mongo datasource (ops-only, behind OAuth) or Metabase later.
   The Grafana Mongo plugin's license was not verified, so confirm before
   choosing it (open item).

Event shape (proposal, not implemented):

```json
{ "id": "<uuid v7>", "type": "template.used", "occurred_at": "<rfc3339>",
  "user": "<pseudonymous uuid v7>", "props": { "template_id": "fantasy_v1" } }
```

No email, username, IP, prompt text or sheet content goes into an event.

## 5. Retention, cost and LGPD

- **Retention**: a TTL index on `occurred_at` (e.g. 365 days) bounds the store.
  Aggregates that outlive the TTL (monthly counts) can be kept as rows with no
  user id.
- **Pseudonymous ids**: the `user` field is the internal account UUID. Under LGPD
  Art. 5, I, an id that can be joined back to a person is still personal data, so
  this is pseudonymization, not anonymization. Treat the whole collection as
  personal data (Art. 5, I; Art. 12, §2).
- **Deletion**: when an account is deleted (the TDD commits to this endpoint),
  the same deletion job must remove `analytics_events` rows with that `user`
  value (Art. 18, VI, and Art. 16). A test must prove it. Aggregated rows
  without a user id need no deletion.
- **Lawful basis and notice**: the basis for internal product analytics must be
  chosen and written into the privacy policy (Art. 7 and Art. 10; not verified
  against ANPD guidance). This is an open item, not a settled decision.
- **Cost**: option A adds no SaaS charge. Option C adds CPU and RAM on the single
  VPS (see section 7). Option F is free only inside its quota (not verified) and
  moves user-linked data to a third party, which needs a DPA and a region choice
  before use.

## 6. Recommendation

Use option A now: a dedicated `cmd/analytics` consumer on a new
`legnaris.events` topic, writing to a Mongo `analytics_events` collection with a
TTL index, pseudonymous account UUIDs only, and deletion wired into the account
deletion flow. Query it from the existing Grafana (ops-only) first. Move to
Metabase only if the operator dashboards are not enough, and then accept the
JVM footprint after measuring it. Reject PostHog self-hosted (16 GB guidance and
about 8 extra services on a VPS already running about 16 containers). Reject
Umami and Plausible because they model browser pageviews, not domain events.
Keep PostHog Cloud out unless a DPA and EU region are settled first.

## 7. Open risks

1. **VPS size is unknown.** The repository does not record RAM or CPU for the
   Hostinger VPS. Measure the current footprint (`docker stats`) before adding
   any consumer, and set `mem_limit` on the new service.
2. **Metabase and ClickHouse memory are unverified.** The pages checked give no
   figures; do not adopt either without measuring.
3. **PostHog Cloud free allowance is unverified** (pricing page truncated).
4. **Grafana Mongo plugin license is unverified.**
5. **LGPD basis and ANPD guidance are unverified.** Only the law text was read.
   Choose the basis (consent, legitimate interest or research) and update the
   privacy policy before collecting events in production.
6. **Deletion gap**: `analytics_events` must be part of the account deletion
   job, or the LGPD deletion claim is false. Add a test.
7. **Event volume and schema drift**: no schema registry exists. The allow-list in
   the consumer is the only guard; a rename silently breaks dashboards.
8. **Broker coupling**: a consumer outage fills the queue. The email alerts in
   `deploy/prometheus/rules.yml` cover queue depth only for `email.otp`, so a
   new alert is needed for `analytics.events`.
9. **Mongo is the same instance as production data.** Analytics writes compete
   with app writes on the same MongoDB and its connection budget
   (`deploy/README.md` section on pool sizing). Consider a separate database name
   at minimum.

## Sources

- https://posthog.com/docs/self-host/deploy/hobby
- https://posthog.com/docs/self-host
- https://posthog.com/pricing
- https://posthog.com/docs/billing/limits-alerts
- https://www.metabase.com/docs/latest/installation-and-operation/running-metabase-on-docker
- https://www.metabase.com/docs/latest/installation-and-operation/tuning-performance
- https://www.metabase.com/docs/latest/databases/connections/mongodb
- https://github.com/metabase/metabase/blob/master/LICENSE.txt
- https://github.com/umami-software/umami/blob/master/LICENSE
- https://plausible.io/docs/self-hosting
- https://github.com/plausible/analytics/blob/master/LICENSE.md
- https://clickhouse.com/docs/en/install
- https://github.com/rabbitmq/amqp091-go/blob/main/LICENSE
- https://www.planalto.gov.br/ccivil_03/_ato2015-2018/2018/lei/l13709.htm
- Repository: `deploy/docker-compose.yml`, `deploy/README.md`, `deploy/rabbitmq/Dockerfile`,
  `deploy/rabbitmq/rabbitmq.conf`, `backend/internal/email/amqp/topology.go`,
  `docs/adr/0004-gitops-argocd-sealed-secrets.md`, `docs/TDD.md`,
  `docs/plan-auth-otp-2fa-email.md`, `.specs/STATE.md`
