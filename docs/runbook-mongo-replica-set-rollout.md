# Runbook — MongoDB replica set rollout on the production VPS (LGN-207 T0.6)

Converts the existing standalone MongoDB of the production Compose stack
(`legnaris-prod`, single Hostinger VPS) to the single-member replica set `rs0`
with a keyfile, as decided in ADR 0013. T0.6 is the exit gate of phase F0 of
LGN-181: nothing that writes more than one document may ship before it passes.

> **Status:** written from the repository only. Nothing here has been executed
> against any server. Rehearse steps 2 to 4 on a copy of the volume in a scratch
> Compose project first (`docker compose -p legnaris-rehearsal ...`, same files,
> same `.env`, volume copied with `docker run --rm -v legnaris-prod_mongo_data:/src:ro -v legnaris-rehearsal_mongo_data:/dst alpine cp -a /src/. /dst/`).

## What changes

| | Before (standalone) | After (rs0) |
|---|---|---|
| `mongodb` command | default `mongod` | `--replSet rs0 --keyFile /etc/mongo-keyfile/keyfile` |
| `mongodb` env | `MONGO_INITDB_ROOT_*`, `MONGO_*_PASSWORD` | no root env (the image's init path is never used) |
| Volumes | `mongo_data`, `./mongodb/init` on `docker-entrypoint-initdb.d` | `mongo_data`, `mongo_keyfile`, `./mongodb` on `/provision` |
| One-shots | none | `mongodb-keyfile` (writes the keyfile), `mongodb-bootstrap` (`deploy/mongodb/bootstrap.sh`) |
| Services that read Mongo | start after `mongodb` healthy | also wait for `mongodb-bootstrap` to complete |
| App URI (`app`, `relay`) | `mongodb://legnaris_app:…@mongodb:27017/legnaris` | same until T0.2 lands; T0.2 adds `replicaSet=rs0` and retryable writes (see Preconditions) |

Users, passwords and data are not changed by the conversion. The bootstrap is
designed to leave existing users alone (see "Known risks").

## Roles and tools

- Operator with SSH access to the VPS (deploy user or root), the repository
  checkout at `/opt/legnaris`, and read access to `ops-legnaris.paulovalleriote.io`
  (Grafana, Mongo Express behind GitHub OAuth).
- Deploys go through `deploy/vps/deploy.sh <40-hex sha>`, which only accepts a
  commit reachable from `origin/main` and rolls back to the previous commit when
  `up --wait` fails.
- Variables used below (set once in the operator shell):

```bash
ROOT=/opt/legnaris
BACKUP_DIR=/var/backups/legnaris
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ROOT_PW="$(sed -n 's/^MONGO_ROOT_PASSWORD=//p' "$ROOT/deploy/.env")"   # strip quotes if .env quotes it
dc() {  # usage: IMAGE_TAG=sha-<sha> dc <compose args>; same files deploy/vps/deploy.sh uses
  docker compose --project-directory "$ROOT/deploy" \
    -f "$ROOT/deploy/docker-compose.yml" -f "$ROOT/deploy/docker-compose.vps.yml" "$@"
}
```

## Step 0 — Preconditions (abort if any fails)

- [ ] Integration branch `integration/lgn-220-replica-set-transactions` is merged
      to `main` and CI images exist for the release commit (`sha-<40 hex>` in GHCR).
      `deploy.sh` refuses any commit that is not an ancestor of `origin/main`.
- [ ] T0.1 (Compose, keyfile, bootstrap) and T0.2 (connection string) are on that
      commit. Confirm T0.2 in the release commit's `deploy/docker-compose.yml` and
      `backend/shared/db`: `DATABASE_URI` names `replicaSet=rs0` and enables
      `retryWrites`. **At the time this runbook was written T0.2 was not in the
      branch** (the `DATABASE_URI` values carry no `replicaSet` option), so the
      exit gate cannot pass until it is.
- [ ] T0.4 (replica-set test fixture) merged; `make test-race` green on the release commit.
- [ ] Maintenance window outside peak hours. The API, relay and worker are
      stopped during step 2 (expected downtime: a few minutes).
- [ ] Record the rollback target and the current state:

```bash
cd "$ROOT"
PREVIOUS_SHA="$(cat "$ROOT/deploy/.deployed_sha")"    # the commit running now; write it down
echo "$PREVIOUS_SHA"
IMAGE_TAG="sha-$PREVIOUS_SHA" dc ps                   # every service Up, mongodb healthy
```

- [ ] Confirm the **current** data has authorization on and the expected users.
      The standalone was initialized by the old `deploy/mongodb/init/01-create-users.js`
      on first start, so the users below are expected. Run with the currently
      deployed images:

```bash
IMAGE_TAG="sha-$PREVIOUS_SHA" dc exec -T mongodb sh -c \
  'mongosh --quiet -u root -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin \
   --eval "db.getSiblingDB(\"admin\").system.users.find({}, {user: 1, db: 1, _id: 0}).toArray()"'
```

Expected: `root@admin`, `legnaris_exporter@admin`, `legnaris_app@legnaris`,
`legnaris_view@legnaris`. Then confirm the server refuses unauthenticated access:

```bash
IMAGE_TAG="sha-$PREVIOUS_SHA" dc exec -T mongodb mongosh --quiet \
  --eval 'db.getSiblingDB("legnaris").getCollectionNames()'
```

Expected: an "Unauthorized" / "requires authentication" error. **If it lists
collections, authorization is off on this volume. Stop: the conversion plan
below assumes it is on.** If the user list is empty, the volume was never
initialized with the users; stop and decide with the repository owner, because
the bootstrap would then create them on a volume the app has been using.

- [ ] Confirm the root password in `deploy/.env` is the one the volume was first
      initialized with. The root login is the only thing that proves it:

```bash
IMAGE_TAG="sha-$PREVIOUS_SHA" dc exec -T mongodb sh -c \
  'mongosh --quiet -u root -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --eval "1"' >/dev/null && echo "root login ok"
```

If this fails, the `.env` value is wrong. Do not continue: a changed root
password makes the bootstrap fail, and the fix is the value in `.env`, never a
rotation during the rollout.

- [ ] Record the baseline for the verification steps: the `system.users` output
      above, `/ready` (`curl -fsS https://api-legnaris.paulovalleriote.io/ready`)
      and one successful login.

## Step 1 — Backup (before anything changes)

Two copies: a logical dump taken while the old stack still runs, and a cold copy
of the data volume. The cold copy is what the revert restores from.

```bash
mkdir -p "$BACKUP_DIR"
# Logical dump, from the running standalone (its env still carries the root password).
IMAGE_TAG="sha-$PREVIOUS_SHA" dc exec -T mongodb sh -c \
  'mongodump --quiet -u root -p "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --archive --gzip' \
  > "$BACKUP_DIR/mongo-logical-$STAMP.archive.gz"
gzip -t "$BACKUP_DIR/mongo-logical-$STAMP.archive.gz"

# Stop the writers, then the member, so the volume copy is consistent.
IMAGE_TAG="sha-$PREVIOUS_SHA" dc stop app relay worker mongodb-exporter mongo-express
IMAGE_TAG="sha-$PREVIOUS_SHA" dc stop mongodb

# Cold copy of the data volume (project name legnaris-prod).
docker run --rm -v legnaris-prod_mongo_data:/data:ro -v "$BACKUP_DIR":/backup alpine:3 \
  tar -C /data -czf "/backup/mongo-data-$STAMP.tgz" .
tar -tzf "$BACKUP_DIR/mongo-data-$STAMP.tgz" | head -20
```

- [ ] The logical dump passes `gzip -t`; the volume archive lists `WiredTiger`
      and `collection-*` entries.
- [ ] Restore verified on a scratch volume (not production) before going on,
      as in `docs/runbook-otp-cutover.md` step 0. Record the two file paths.

If the backup fails, restart the old stack (`IMAGE_TAG="sha-$PREVIOUS_SHA" dc up -d --wait`)
and stop. Nothing has changed on the data.

## Step 2 — Convert the standalone to rs0 (data kept)

The bootstrap one-shot cannot do this on its own on existing data (see
"Known risks", item 2). The conversion is therefore done in two parts: the
member is restarted with `--replSet` and the keyfile, then the set is initiated
by hand as the root user, which the bootstrap then finds already initiated.

1. Check out the release commit and create the keyfile volume. `mongodb-keyfile`
   is the only one-shot this starts; it writes `legnaris-prod_mongo_keyfile`
   once (existing keyfile is kept):

```bash
cd "$ROOT" && git fetch --depth 1 origin <RELEASE_SHA> && git checkout --detach <RELEASE_SHA>
RELEASE_SHA="$(git rev-parse HEAD)"
IMAGE_TAG="sha-$RELEASE_SHA" dc up -d mongodb-keyfile
```

2. Start the member with the new command. `up -d mongodb` recreates the
   container (the command and volumes changed) and waits on the keyfile:

```bash
IMAGE_TAG="sha-$RELEASE_SHA" dc up -d mongodb
IMAGE_TAG="sha-$RELEASE_SHA" dc ps mongodb              # Up, healthy after ~20 s
```

   The healthcheck is an unauthenticated ping, so it passes with users present.

3. Initiate `rs0` as root. The member is reached as `mongodb:27017`, the same
   address the bootstrap and the clients use, because the replica-set
   configuration must name that address:

```bash
IMAGE_TAG="sha-$RELEASE_SHA" dc exec -T mongodb mongosh --quiet \
  -u root -p "$ROOT_PW" --authenticationDatabase admin \
  --eval 'rs.initiate({_id: "rs0", members: [{_id: 0, host: "mongodb:27017"}]})'
```

   Expected: `{ ok: 1 }`. If it reports "requires authentication", the root
   login failed: the `.env` value is wrong (see Step 0). Do not retry with
   another password.

4. Wait for the primary and check the set name (both calls are allowed
   unauthenticated, as in `deploy/mongodb/init/00-replset.js`):

```bash
IMAGE_TAG="sha-$RELEASE_SHA" dc exec -T mongodb mongosh --quiet \
  --eval 'print(db.hello().setName, db.hello().isWritablePrimary)'
```

   Expected: `rs0 true`. Give it up to 60 s; the bootstrap script waits the same
   way and fails after 60 s with "has no primary".

5. Run the one-shot and the rest of the stack. The bootstrap finds `rs0` already
   initiated, finds the root login working, and finds every user present, so it
   changes nothing. Only now is the release deployed through its normal path:

```bash
"$ROOT/deploy/vps/deploy.sh" "$RELEASE_SHA"
```

   `deploy.sh` pulls the images, runs `up -d --remove-orphans --wait`, and writes
   `deploy/.deployed_sha`. **Do not run `deploy.sh` before step 4 succeeds.** If
   `up --wait` fails, `deploy.sh` rolls back to `$PREVIOUS_SHA` automatically
   against the converted volume (see Revert, and "Known risks", item 8).

**Abort criteria for step 2:** `rs.initiate` fails for any reason other than a
transient one; no primary within 60 s; the root login fails; the
`system.users` list changes. Abort = restore the old stack (see Revert, path B).

## Step 3 — Verification

Run after step 2.5 and again after the reboot drill (below). All commands use
the release tag: `IMAGE_TAG="sha-$RELEASE_SHA"` (or the value in
`deploy/.deployed_sha`).

```bash
# 3.1 The member is the only PRIMARY of rs0 and is reached at mongodb:27017.
dc exec -T mongodb mongosh --quiet -u root -p "$ROOT_PW" --authenticationDatabase admin \
  --eval 'const s = rs.status(); print(s.set, s.members.map(m => m.stateStr + " " + m.name).join(", "))'
#   expected: rs0 PRIMARY mongodb:27017

# 3.2 The member runs with the replica-set flags and the keyfile.
docker inspect --format '{{json .Config.Cmd}}' legnaris-prod-mongodb-1
#   expected: ["--replSet","rs0","--keyFile","/etc/mongo-keyfile/keyfile"]

# 3.3 Keyfile owned by mongod's uid and readable by its owner only.
dc exec -T mongodb stat -c '%a %u:%g' /etc/mongo-keyfile/keyfile
#   expected: 400 999:999

# 3.4 One-shots finished cleanly; they are not running.
docker inspect --format '{{.State.ExitCode}}' legnaris-prod-mongodb-keyfile-1 legnaris-prod-mongodb-bootstrap-1
#   expected: 0 and 0

# 3.5 Users unchanged: same list as the Step 0 baseline.
dc exec -T mongodb mongosh --quiet -u root -p "$ROOT_PW" --authenticationDatabase admin \
  --eval 'db.getSiblingDB("admin").system.users.find({}, {user: 1, db: 1, _id: 0}).toArray()'

# 3.6 Services that read the database are up and ready.
curl -fsS https://api-legnaris.paulovalleriote.io/ready
dc ps          # app, relay, worker, mongodb-exporter, mongo-express Up; mongodb healthy

# 3.7 A transaction commits and leaves nothing behind (writes one probe
#     document in the application database and drops the collection).
dc exec -T mongodb mongosh --quiet -u root -p "$ROOT_PW" --authenticationDatabase admin --eval '
const s = db.getMongo().startSession();
const probe = s.getDatabase("legnaris").getCollection("t0_6_probe");
s.withTransaction(() => { probe.insertOne({ probe: true }); probe.deleteMany({ probe: true }); });
probe.drop();
print("transaction ok");'
```

- [ ] 3.1 `rs0 PRIMARY mongodb:27017`
- [ ] 3.2 replica-set flags and keyfile in the container command
- [ ] 3.3 keyfile `400 999:999`
- [ ] 3.4 both one-shots exit 0
- [ ] 3.5 users identical to the baseline
- [ ] 3.6 `/ready` answers 200 and no service is restarting
- [ ] 3.7 `transaction ok`
- [ ] A login from the web client succeeds and an existing session still works.
- [ ] `docker compose logs --since 15m mongodb app relay` shows no authentication
      failures (`Authentication failed`, `not master`, `NotWritablePrimary`).

Re-run the bootstrap once to prove idempotency (the acceptance criterion
"restarting the Mongo container on an initialized set does not fail"):

```bash
IMAGE_TAG="sha-$RELEASE_SHA" dc up -d --wait --wait-timeout 180
docker inspect --format '{{.State.ExitCode}}' legnaris-prod-mongodb-bootstrap-1   # 0
```

## Reboot recovery

The data, the keyfile and the replica-set configuration are in the volumes, so a
host reboot needs no re-initiation.

- Containers come back through `restart: unless-stopped`. `mongodb` starts as
  the single member and elects itself PRIMARY. The one-shots (`restart: "no"`) do
  not run again on boot, and they do not need to.
- After a reboot, run the Step 3 checks 3.1, 3.6 and 3.7.
- If `rs.status()` does not report `PRIMARY`, or a container is stopped, reconcile
  the stack at the deployed commit. This reruns the keyfile and bootstrap
  one-shots, which are idempotent:

```bash
IMAGE_TAG="sha-$(cat "$ROOT/deploy/.deployed_sha")" dc up -d --wait --wait-timeout 180
```

- **Never delete `legnaris-prod_mongo_keyfile`.** `keyfile.sh` keeps an existing
  keyfile on purpose; a new one changes the set's key. Recovery never needs it.
- A reboot during step 2 is recovered by re-running step 2.2 to 2.4 (the
  initiation is skipped when `rs0` already exists), then step 2.5.

## Revert

The revert is one step: redeploy the commit recorded in Step 0 as
`PREVIOUS_SHA`. Its Compose file carries the standalone command and the
`DATABASE_URI` without `replicaSet`, so Compose and connection config go back
together, and its images already exist in GHCR. Then fix `main` so the next
deploy does not undo the revert.

### Path A — redeploy the previous commit (use first)

```bash
"$ROOT/deploy/vps/deploy.sh" "$PREVIOUS_SHA"
```

Then verify:

- [ ] `dc ps` all Up. The previous `mongodb` healthcheck authenticates as root
      with `MONGO_INITDB_ROOT_PASSWORD`, so `mongodb` must turn healthy again; if
      it stays unhealthy, see "Known gap" below.
- [ ] The unauthenticated query from Step 0 is refused again ("Unauthorized"), as
      in the baseline.
- [ ] `/ready` answers 200; one login works.

**Known gap of path A:** the previous compose has no `--replSet` and no
keyfile, but the volume now carries the `rs0` configuration. Whether `mongod`
starts cleanly on it without `--replSet`, and whether it keeps authorization on,
is not tested in this repository. Check the first `mongod` log lines and the
Step 0 baseline before declaring the revert done.

### Path B — restore the cold copy (only if path A fails to start mongod)

Writes made after step 2 are lost with this path. Decide before running it.

```bash
IMAGE_TAG="sha-$RELEASE_SHA" dc stop app relay worker mongodb-exporter mongo-express mongodb
docker run --rm -v legnaris-prod_mongo_data:/data -v "$BACKUP_DIR":/backup alpine:3 \
  sh -c 'rm -rf /data/* /data/.[!.]* 2>/dev/null; tar -C /data -xzf "/backup/mongo-data-<STAMP>.tgz"'
"$ROOT/deploy/vps/deploy.sh" "$PREVIOUS_SHA"
```

Replace `<STAMP>` with the timestamp from Step 1. Then run the Path A checks.

### After the revert

- Revert the commit that carries the Compose and `DATABASE_URI` change on `main`
  (`git revert -m 1 <merge-sha>` if it is a merge, through a pull request, since
  `main` is protected). Do this before the next scheduled deploy, which would
  otherwise re-apply the rs0 Compose file.
- Record the revert and its cause on LGN-207 T0.6.

**Roll back when:** `mongod` does not reach a healthy state within 10 minutes of
step 2.2 and the cause is not understood, the root login or the app login fails
for most users after step 2.5, or `rs.initiate` cannot produce a primary.

**Do not roll back for:** a single failed one-shot that the idempotent rerun
fixes (`dc up -d --wait`), or `/ready` flapping for under a minute while the app
reconnects after the member restart.

## Exit gate (T0.6 / phase F0)

F0 is done when all of these hold on the production VPS:

- [ ] Step 3 passes, including the transaction probe (3.7).
- [ ] The reboot drill passed: a host reboot in the window (or, if not possible in
      the window, `dc restart mongodb` followed by the same checks) brought `rs0`
      back to PRIMARY with no manual steps other than the reconcile command above.
- [ ] Repeating `dc up -d --wait` leaves the set and the users unchanged (idempotency).
- [ ] The app connects with the URI that names `replicaSet=rs0` (T0.2), and
      `/ready` is green for 24 h after the cutover with no `NotWritablePrimary`
      or transaction errors in the logs.
- [ ] The backups from Step 1 are kept, and the revert path has been described
      in the release notes.

Record the outcome on LGN-207 T0.6. F1 cards may start only after this gate passes.

## Known risks

These are the points where the bootstrap design has **not** been tested against
existing data. The bootstrap was tested only on fresh volumes:
`deploy/mongodb_test.go` runs `bootstrap.sh` against a fake `mongosh`, and the
fresh-volume path is the only path covered. Nothing in the repository runs the
bootstrap on a standalone volume that already holds data and users.

1. **Untested on existing data.** The conversion relies on the reasoning below,
   not on a test. Rehearse steps 2 to 4 on a copy before production.
2. **`00-replset.js` runs unauthenticated, and `rs.initiate` may need
   authentication once users exist.** The bootstrap runs the replica-set script
   first, through the localhost exception. That exception is intended for a
   volume with no users; with the users from the standalone present, the
   initiate is expected to be refused. The script uses `set -eu`, so the
   one-shot would fail, and `mongodb-bootstrap` never completes, so `app`,
   `relay`, `mongodb-exporter` and `mongo-express` never start. The workaround
   is step 2.3 (initiate as root first). The bootstrap then skips the initiate,
   because `db.hello().setName` already reports `rs0`.
3. **Root password mismatch fails loudly, as designed.** If `MONGO_ROOT_PASSWORD`
   in `.env` differs from the first-init value, the root login fails and
   `bootstrap.sh` tries `createUser` without authentication, which fails with
   auth on. Fix the `.env` value. Never rotate the root password through a
   rerun of the bootstrap.
4. **User passwords are never rotated.** `01-create-users.js` creates a user only
   when it does not exist, so a `.env` value for `MONGO_APP_PASSWORD`,
   `MONGO_VIEW_PASSWORD` or `MONGO_EXPORTER_PASSWORD` that differs from the
   first-init value breaks the service that uses it, silently at bootstrap time
   and loudly at connection time. Step 3.5 and `/ready` catch it. Fix: change the
   password with root (`db.changeUserPassword`) and in `.env` in the same
   change, through a reviewed runbook step, not through the bootstrap.
5. **Authorization on the existing volume is assumed, not proven.** The
   `system.users` and unauthenticated checks in Step 0 are the proof. If the
   volume has no users or authorization is off, stop.
6. **The old init scripts ran only on first start.** `docker-entrypoint-initdb.d`
   never runs again on an existing volume, so the users in the volume are the
   ones the old script created. The bootstrap creates any that are missing.
7. **Revert onto rs-configured data is untested** (Revert, path A). Path B is the
   fallback, and it loses post-conversion writes.
8. **`deploy.sh` rolls back automatically.** If the release's `up --wait` fails
   after a conversion that was not finished (before step 2.4), `deploy.sh`
   redeploys `PREVIOUS_SHA` on the converted volume, which is the untested
   revert above. Finish or abort step 2 by hand before any deploy.
9. **Connection config is not in the repository yet.** Until T0.2 lands, the app
   and relay URIs carry no `replicaSet` option. The driver's behavior for
   transactions against a single member addressed without `replicaSet` is not
   established here; the exit gate (3.7 and the app check) is what proves it.
10. **Downtime.** The stop in Step 1 and the start in Step 2 take the API down
    for the duration; the web client shows errors until `/ready` answers again.
