# Contract: The stack

**Feature**: `.specs/features/021-broker-stack/` | **Requirement IDs**: `BRK-01` … `BRK-22` | **Owner**:
`deploy/`

## Purpose

What the deployment must say for the delivery pipeline to run and for the broker's console to be the
operator's. The contract is read by the deployment's own checks, so every line here is a line a check
asserts.

## The services

| Service | Must exist | Must not |
|---|---|---|
| `worker` | be built from the shared image, start on its own entrypoint, sit on the internal network, wait for the broker's health check, carry the broker's address, the provider settings and the worker's own settings | publish a host port, depend on the database, be depended on by the site |
| `rabbitmq` | publish no host port, declare a health check, name an exact 4.x version, join the edge network for its console's route only | publish the console's port or its metrics port on the host |
| `app` | keep its own environment unchanged | receive the broker's address or any worker setting |
| `relay` | keep its own environment unchanged | receive the operator's password |

## The console's route

| Field | Must be |
|---|---|
| the rule | the operator's domain **and** the console's path prefix |
| the service | the broker's console on its internal port |
| the middlewares | the operator's sign-in gate and the security headers, in that order |
| the path | passed through unchanged |

No other domain may route to the console, and the site itself must expose no route to it
(`BRK-07`).

## The monitoring targets

The monitoring stack must scrape the broker's Prometheus endpoint and the worker's listener, both on
the internal network, in addition to the four it already scrapes (`BRK-13`).

## The variables

| Variable | Where | Interpolation |
|---|---|---|
| `RABBITMQ_PASSWORD` | the broker's default account | required — a missing value fails the stack |
| `RABBITMQ_OPS_PASSWORD` | the operator's provisioning only | not an interpolation — it never reaches a service |
| `RABBITMQ_URL` | the publisher's and the worker's environment | optional, with the delivery vhost in the address |

The stack's required-variable list is unchanged (`BRK-21`).

## Boundary cases

| Case | Result |
|---|---|
| the broker is down | the publisher and the worker wait; the site serves |
| the worker is down | the site serves; the codes queue up in the broker |
| the broker's image tag is a floating minor | the check fails — the version must be exact |
| the console's route loses its sign-in gate | the check fails — the console must never be public |
| a service publishes a host port | the check fails |
| the required-variable list grows | the check fails — this feature adds none |

## Test evidence

- `deploy/compose_test.go`: the worker's service, its entrypoint, its dependencies, its environment and
  its exposure; the broker's exposure and its pinned version; the required-variable list; the
  operator's password never reaching a service.
- `deploy/traefik_test.go`: the console's rule, its service and its middlewares; the operator's other
  routes unchanged.
- `deploy/prometheus/prometheus.yml` is parsed by the deployment's checks for the scrape targets.
