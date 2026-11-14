# AGENTS.md — backend/shared/db (MongoDB Initialization)

## Responsibility

- MongoDB client initialization and connection management
- Provides `*mongo.Database` to repositories
- Multi-document transactions on the replica set (`Transactor`, ADR 0013)
- Crash-injection step hooks for transaction tests (`Step`, `StepHook`)
- Replica-set test fixture for integration tests (`dbtest`)

## Connection

- `DATABASE_URI` must name the replica set and enable retryable writes:
  `?replicaSet=rs0&retryWrites=true&authSource=<db>`. Every shipped URI does
  (`.env.example`, `scripts/dev.sh`, `deploy/docker-compose*.yml`).
- `InitMongo` rejects a URI without `replicaSet` with `ErrMissingReplicaSet`
  (wraps `ErrInvalidConfig`), so a standalone URI fails at startup.
- `InitMongo` also rejects a URI without username and password
  (`ErrMissingCredentials`, wraps `ErrInvalidConfig`) and a malformed URI. Errors
  never echo the URI, so credentials cannot leak into logs.
- `newClientOptions` forces `retryWrites` on after the URI is applied, so a URI
  cannot turn it off.

## Transactor

- `NewTransactor(client, hook)` binds a transactor to a replica-set client.
  `WithTransaction(ctx, fn)` runs `fn` in a session-bound transaction and commits
  when `fn` returns nil.
- Every repository call inside `fn` must use the context `fn` receives. A call
  made with the outer context writes outside the transaction.
- The driver re-runs `fn` only on the `TransientTransactionError` label, and only
  within `DefaultTransactionTimeout` (10s, overridable per transactor with
  `WithTimeout`). A commit that fails with `UnknownTransactionCommitResult` is
  retried by the driver at the commit step only, so `fn` is not re-run for it.
  `fn` must have no side effects outside the database.
- A callback error that arrives after the transaction deadline loses its labels
  (`unlabeledError`), so the driver does not re-run a dead callback. A callback
  error seen while the context is live keeps its labels and is retried.
- Errors: `ErrNoClient`, `ErrNilCallback`, `ErrTransactionAborted` (the callback or
  a step hook failed and the transaction rolled back; the callback error stays
  matchable with `errors.Is`), `ErrTransactionFailed` (the transaction could not
  start or commit).
- `TransactionOptions()` is the only transaction option set: `w:majority` and
  the primary. Reads outside a transaction that decide authorization or a
  compensation must request `readpref.Primary()` themselves.

## Step hook

- `Step` names a point inside a transaction where a crash can be injected:
  `StepAfterEntityWrite`, `StepAfterOutboxInsert`, `StepAfterProviderRead`.
- `Transactor.Step(ctx, step)` calls the hook. An error from the hook aborts the
  enclosing transaction and is wrapped as `step <name>: <err>`.
- Production passes a nil hook; `Step` is then a no-op. Only tests install one.

## dbtest

- `dbtest.Client(t)` connects through `DATABASE_URI` and skips the test only
  when `DATABASE_URI` is unset; when it is set, an unreachable or standalone
  MongoDB fails the test. `dbtest.Database(t)` adds an isolated
  database with a unique name, dropped at cleanup before the client disconnects.
- `dbtest.Adopt` registers cleanup for a database whose name must be known before
  the environment is built (`DATABASE_NAME` for the application modules).
- Failpoint tests (`configureFailPoint` in `transaction_test.go`) need a member
  started with `enableTestCommands=1` and skip otherwise.

## Conventions

- Declare own typed errors — **do NOT import `apperr`**
- Wrap errors with `fmt.Errorf("...: %w", err)`
- Service layer translates to `apperr` at boundary
- All identifiers, comments, logs in English
