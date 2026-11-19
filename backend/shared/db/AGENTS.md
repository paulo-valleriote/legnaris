# AGENTS.md — backend/shared/db

Why MongoDB is a replica set and why pending work is not a transaction:
[`context/architecture.md`](../../../context/architecture.md).

## Working rules

- `DATABASE_URI` must name `replicaSet` and carry a username and password;
  `newClientOptions` forces retryable writes. `InitMongo` fails startup
  otherwise and never echoes the URI.
- Declare typed errors here; do not import `apperr`. Wrap with
  `fmt.Errorf("...: %w", err)` and let the service translate.
- `NewTransactor(client, hook)` is the only transaction entry. Every repository
  call inside the callback uses the context the callback receives, and the
  callback has no side effects outside the database because the driver may
  re-run it.
- One transaction call is capped at 10 s (`WithTimeout` overrides). Failures
  are `ErrTransactionAborted` (wraps the callback error, still matchable) or
  `ErrTransactionFailed` (start or commit). `Step` is a no-op with a nil hook;
  production passes nil.
- Reads outside a transaction that decide authorization or a compensation must
  request `readpref.Primary()` themselves.
- `dbtest.Client` skips only when `DATABASE_URI` is unset; a set but unreachable
  or standalone server fails the test. `dbtest.Database` isolates a database,
  and failpoint tests need a member started with `enableTestCommands=1`.
