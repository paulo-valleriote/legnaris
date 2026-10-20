# Contract: Email canonicalization CLI

**Feature**: `.specs/features/015-auth-prerequisites/` | **Requirement IDs**: `PRQ-11`, `PRQ-18` |
**Owner**: `cmd/canonicalize-emails` (shell), `internal/user` (plan + apply)

## Purpose

Repair legacy rows whose stored address is not canonical, safely and once, before the email features
depend on the stored value.

## Command

```text
canonicalize-emails [-apply]
```

| Flag | Default | Meaning |
|---|---|---|
| `-apply` | `false` | Write the planned changes. Without it the command is a dry run: it reports intent and writes nothing. |

Configuration comes from the same environment as the API (`DATABASE_URI`, `DATABASE_NAME`, database
timeouts). `JWT_SECRET` is not required, although the shared env loader may log its usual
"not set" warning.

## Report (stdout, English)

```text
scanned: <n>
change: <id> <from> -> <to>          (one line per planned change)
collision: <canonical> <id> <id> …   (grouped)
invalid: <id> <raw> (<reason>)
written: <n>                         (apply mode)
dry run: <n> changes not written     (dry-run mode)
```

The report never adds fields, never rewrites an id and never merges accounts.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Clean run: applied, or dry run, or already canonical (0 changes) |
| 1 | Blocked (collision or unparseable row, nothing written) or a store failure |
| 2 | Usage error (unknown flag, bad arguments) |

## Invariants

- **Validate everything before the first write**: a collision or an invalid row aborts the run with
  0 writes (`PRQ-11`, `SC-008`).
- Only `email` and `updated_at` change; the identifier and every other field are preserved.
- The same `CanonicalEmail` rule as registration is used.
- Running twice over repaired data plans 0 changes and writes 0 rows.
- The command is never invoked by `cmd/api.go`; the API boot path cannot run it.
- A mid-run store failure exits 1; because each row is written independently and the plan is
  recomputed from stored values, re-running finishes the remaining rows (Mongo standalone has no
  transactions, F-12).

## Packaging and operation

- `Dockerfile` builds `/canonicalize-emails` into the runtime image next to `/app`; the image entry
  point is unchanged.
- In the deployed stack:
  `docker compose run --rm --entrypoint /canonicalize-emails app -apply` (drop `-apply` for the dry
  run).
- Locally: `make migrate-emails ARGS="-apply"` (dry run with no `ARGS`).

## Test evidence

- `internal/user/email_migration_test.go`: plan table (no changes, changes, collision, invalid,
  already-canonical), blocked plans write nothing, apply sets only the two fields, dry run writes
  nothing, second run is a no-op, store failure surfaces.
- `cmd/canonicalize-emails/main_test.go`: the run seam reports and returns the documented exit codes
  for dry run, apply, blocked and store-failure cases.
- `internal/user/repository_test.go` (Mongo, self-skip): `ListEmailIdentities` and `UpdateEmail`
  semantics.
