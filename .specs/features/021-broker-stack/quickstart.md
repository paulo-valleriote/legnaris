# Quickstart: Validate the Broker Stack

**Feature**: `.specs/features/021-broker-stack/` | **Date**: 2026-10-21

This feature ships no HTTP endpoint of its own. "Works" means: the stack says what it must say, the
deployment's own checks say so, and the operator can reach the console and provision the broker.

## Prerequisites

- Go 1.26.4 (`go version`).
- Docker with Compose for the optional end-to-end run.
- No environment variable is required for the unit and race gates.

## 1. The always-on gates

```bash
make fmt && make lint && make vet && make lint-specs   # linters + spec/test lint
make test-race                                         # the race detector must stay clean
```

Expected: clean. The deployment's suites parse the compose file, the edge configuration and the
monitoring configuration; they need no Docker, no database and no broker.

## 2. Prove the stack's contract

```bash
go test ./deploy/... -v
```

Expected:

- the worker service exists, shares the image, starts on its own entrypoint, waits for the broker,
  carries the broker's address and the provider settings, and publishes no host port;
- the site receives neither the broker's address nor any worker setting;
- the broker publishes no host port, declares a health check, and names an exact 4.x version;
- the console's route exists on the operator's domain under its own prefix and carries the sign-in
  gate and the security headers;
- the monitoring configuration scrapes the broker and the worker;
- the required-variable list is unchanged and the operator's password reaches no service.

## 3. Prove the stack still interpolates (the continuous integration gate)

```bash
docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config >/dev/null
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.local.yml --env-file deploy/.env.example config >/dev/null
```

Expected: both succeed. A missing required variable fails here, which is the point.

## 4. Optional: the stack end to end

```bash
cp deploy/.env.example deploy/.env      # then fill every replace_me value
docker compose -f deploy/docker-compose.yml up -d --build rabbitmq worker
```

Expected: the broker becomes healthy, the worker starts after it, and the worker's metrics appear at
`http://localhost:9101/metrics` from inside the network.

The broker provisions itself on start (the delivery vhost, the operator's account from
`RABBITMQ_OPS_PASSWORD`, the application's rights) and reports healthy only afterwards. Open the console at `https://<ops-domain>/rabbitmq/`: the
operator's gate answers first, and the broker's login accepts the operator's account and refuses the
application's.

## 5. Optional: the broker's own metrics

```bash
docker compose -f deploy/docker-compose.yml exec rabbitmq rabbitmq-diagnostics -s listeners
```

Expected: the Prometheus endpoint is listed on its own port, and the console's listener is bound on
the internal interface.

## References

- `contracts/stack.contract.md` — the services, the route, the targets and the variables.
- `contracts/provisioning.contract.md` — the operator's steps and their rules.
- `data-model.md` — the stack's shapes.
- `research.md` — the decisions behind them.
