# Phase 1 Data Model: The Broker Stack

**Feature**: `.specs/features/021-broker-stack/` | **Date**: 2026-10-21

No persisted collection and no new domain entity. The feature's shapes are *deployment* shapes: the
services of the stack, the route to the console, the broker's accounts and its vhost, and the targets
the monitoring stack reads. The broker's own data (its queues, its messages) is unchanged.

---

## 1. The services of the stack

| Service | Role | Network | Published ports | Depends on |
|---|---|---|---|---|
| `app` | the request-serving process | edge + internal | 80/443 (through the router) | `mongodb` |
| `relay` | the publisher | internal | none | `mongodb`, `rabbitmq` |
| `worker` | the delivery worker (**new**) | internal | none | `rabbitmq` |
| `rabbitmq` | the message broker | internal + edge (**new**: the console's route) | none | — |
| `traefik` | the edge router | edge + internal | 80/443 | `oauth2-proxy` |

The `worker` is the only service this feature adds. It shares the application's image and starts on
its own entrypoint, so it can be restarted or scaled without touching the site (`BRK-01`).

---

## 2. The broker's names

| Name | Value | Where it is used |
|---|---|---|
| the delivery vhost | `legnaris` | the broker's address the publisher and the worker use; the permissions the application's account holds |
| the application's account | the broker's default account | the publisher's and the worker's connection; no console right |
| the operator's account | a second account | the console's login; the only account with the console right |
| the console's path prefix | `/rabbitmq` | the broker's own configuration and the edge route's rule |

The vhost is named in the broker's address, so the delivery queues live apart from the default
vhost: an account without a permission on a vhost can neither publish to it nor see it, through the
messaging protocols or through the console (`BRK-09`, `BRK-17`).

---

## 3. The console's route

| Field | Value |
|---|---|
| the rule | the operator's domain **and** the console's path prefix |
| the service | the broker's console, on its internal port |
| the middlewares | the operator's sign-in gate, then the security headers — the same two the other operator routes carry |
| the path | passed through unchanged; the broker serves the console under its own prefix |

The rule is the same shape as the three consoles already on the operator's domain, so the operator's
surface reads as one thing (`BRK-07`, `BRK-08`).

---

## 4. The accounts' rights

| Account | Tags | Permissions | May open the console |
|---|---|---|---|
| the application's | none | the delivery vhost only | **no** |
| the operator's | the console right | the delivery vhost | yes |

A newly created broker account has no tag at all, and an account with no tag has no access to the
console — that is the broker's own permission model, not a convention this feature imposes. The
image's default account, by contrast, is created with the administrator tag, so its rights are
narrowed explicitly by the provisioning (`research.md` R2).

---

## 5. The scrape targets

| Target | Endpoint | Reads |
|---|---|---|
| `legnaris-app` (existing) | the site's metrics | the request counters |
| `legnaris-relay` (existing) | the publisher's listener | the outbox backlog and the publication outcomes |
| `legnaris-worker` (**new**) | the worker's listener | the send outcomes, the dead letters, the expired messages |
| `legnaris-rabbitmq` (**new**) | the broker's Prometheus endpoint | the queue depth, the unacknowledged messages, the consumer count |
| `traefik` (existing) | the router's metrics | the edge traffic |
| `mongodb` (existing) | the exporter's metrics | the database |

Every target is on the internal network and none is reachable from the host (`BRK-12`, `BRK-13`).

---

## 6. The variables

| Variable | Required? | Used by | Meaning |
|---|---|---|---|
| `RABBITMQ_PASSWORD` | yes (already) | the broker's default account | the application's broker credential |
| `RABBITMQ_OPS_PASSWORD` | **no** (operator-only) | the provisioning command | the operator's broker credential |
| `RABBITMQ_URL` | no (already) | the publisher, the worker | the broker's address, naming the delivery vhost |

The stack's required-variable list is unchanged: the operator's password never reaches a service, so
it is not an interpolation the stack depends on (`BRK-21`).
