# F1a: VPS migration runbook

Epic: LGN-221. Written from the repository; not executed against production.
CD deploys a commit that is an ancestor of `origin/main`
(`deploy/vps/deploy.sh <sha>`), so merging the PR starts the deploy. Prepare
steps 1 to 4 before merging.

## 1. Before merging

1. Back up MongoDB (`mongodump` of the Legnaris database) and note the current
   `deploy/.deployed_sha` on the VPS for rollback.
2. OpenRouter dashboard: set a spending limit, turn prompt logging off and fix
   the workspace guardrail so the ZDR endpoint is allowed (LGN-227).
3. Add to the VPS `.env` (the CD env-key check lists the required keys):
   `LLM_API_KEY`, `LLM_MODEL` (never a `:free` variant),
   `GENERATION_ENABLED=false`, `GENERATION_DAILY_CAP=200`. Keep generation off
   until step 7.
4. Confirm MongoDB runs as a replica set (LGN-207 transactions) and RabbitMQ is
   healthy.

## 2. Stop the API

The Sheet-kind migration must run with the API stopped, because it replaces the
one-sheet-per-user index.

```
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.vps.yml stop app relay worker
```

## 3. Run the migration with the new image

The image now ships `/migrate-sheet-kinds` (this PR). With the new tag pulled:

```
IMAGE_TAG=sha-<sha> docker compose --project-directory deploy \
  -f deploy/docker-compose.yml -f deploy/docker-compose.vps.yml \
  run --rm --entrypoint /migrate-sheet-kinds app
```

It turns legacy Sheets into `character` Sheets, rebuilds the per-owner counters
and replaces the index. Read the report; a non-zero exit means stop and restore
from the backup.

## 4. Start the stack

`deploy/vps/deploy.sh <sha>` (or let CD run after merge). Wait for
`up -d --wait` to report healthy, including `relay` and `worker`.

## 5. Verify

- `GET /health` on the API; the relay and worker logs show no boot errors.
- Prometheus: rules from `deploy/prometheus/rules-f1a.yml` loaded (no promtool
  errors); Grafana dashboard `f1a-generation` present.
- RabbitMQ: generation and events queues exist and have consumers; `events.unrouted` is empty.
- An existing user still sees their Sheet as a character.

## 6. Note on event latency

`event_outbox` runs on the 5-minute sweeper interval, so events such as
`user.deleted` can take up to 5 minutes.

## 7. Enable generation

Only after LGN-227 and LGN-228 are done: set `GENERATION_ENABLED=true`, redeploy
the env, run one generation with a test account, and watch the cap metric and
alerts.

## Rollback

Set `GENERATION_ENABLED=false` first (kill-switch). To revert the code,
`deploy/vps/deploy.sh <previous sha>`; the migration is not reversible in place,
so restore the backup if the old build must run.
