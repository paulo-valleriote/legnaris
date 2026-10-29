# Research: The Broker Stack

**Feature**: `.specs/features/021-broker-stack/` | **Requirements**: `BRK-01` … `BRK-22` | **Date**:
2026-10-21

**Program plan**: `docs/plan-auth-otp-2fa-email.md` §5.8 (deploy in the current stack), §5.9
(observability), §9 stage 6 — decisions D6 (RabbitMQ), O2 (separate processes), O6 (the management UI
behind `ops.`).

Every mechanism below was checked against the RabbitMQ 4.4 documentation and against the stack as it
stands in `deploy/`.

## R1. The delivery worker joins the stack as a service of its own

**Decision**: a `worker` service in the base compose, built from the same image as the site and the
publisher, started on the `/worker` entrypoint, on the internal network only, with no published port,
waiting for the broker's health check.

**Rationale**: the plan's decision O2 makes the publisher and the worker separate processes, and the
stack already runs the publisher that way. The worker needs no database (the preceding feature's
`WKR-01`), so its only dependency is the broker; keeping the database out of its dependency list is
what makes that decision real rather than notional.

**Alternatives considered**:

- *Run the worker inside the site's process* — rejected by the plan: a sending failure must never
  reach the request-serving process.
- *Give the worker a database dependency for symmetry with the publisher* — rejected: the worker never
  reads a store; a dependency it does not need is a failure mode it does not need.

## R2. The broker's accounts are provisioned by operator commands, documented in the runbook

**Decision**: the broker's delivery vhost, the application's account rights and the operator's account
are all provisioned by `rabbitmqctl` commands that the operator runs once per broker volume, with the
passwords read from the environment. The commands are idempotent, so the runbook can be re-run safely.

**Rationale**: the RabbitMQ documentation is explicit that the console's access is controlled by the
permission **tags**, and that "newly created users do not have any tags set on them by default" — a
user with no tag has no access to the management plugin at all. The image's own default user, by
contrast, is created with the administrator tag, so the application's account has to be narrowed
explicitly. There is no configuration-file path for either fact: vhosts, users, tags and permissions
are broker *state*, not configuration.

The choice between the two candidate mechanisms was put to the user on 2026-10-21, and the operator
command was chosen over a definitions file rendered from the environment. The reasons are the ones that
survive into the runbook: no credential material in the repository, no password hash generated at
deploy time, and the provisioning is a visible operator act rather than something the stack does
silently on every boot.

**Alternatives considered**:

- *A definitions file rendered from the environment* — rejected by the user's decision (2026-10-21).
- *A definitions file committed with the accounts* — rejected: it would put a broker credential in the
  repository (`BRK-11`).
- *A provisioning service in the stack* — rejected: it would do silently, on every boot, what the
  user's decision asked to be a visible operator act.

**Consequence the runbook must state**: the pipeline must not be deployed before the provisioning
steps have run, and the steps must be re-run after the broker's volume is replaced.

## R3. The delivery vhost is named, and the broker's address carries it

**Decision**: the code queues live in a dedicated vhost (`legnaris`), and the broker address the
publisher and the worker use names that vhost.

**Rationale**: the plan asks for the application account to be confined to "the vhost `legnaris`", and
RabbitMQ's permission model is per vhost: an account with no permission on a vhost cannot see it at
all, neither through the messaging protocols nor through the console. Naming the vhost also means the
default one carries nothing, so a mistake in a permission never exposes the delivery queues.

**Alternatives considered**:

- *Keep the default vhost* — rejected: the default vhost is where an account's permissions land by
  default, and the plan asks for a dedicated one.

## R4. The console is served under a path prefix and routed through the operator's gate

**Decision**: the broker's console is served under its own path prefix (`/rabbitmq`) on the operator's
domain, behind the existing OAuth forward-auth middleware and the existing security headers, and the
broker stays on the internal network, which the edge router already shares (AD-024).

**Rationale**: the plan's decision O6. The operator's domain already carries three other consoles
behind the same two middlewares, so the broker's console is the fourth of a known kind rather than a
new pattern. The path prefix keeps the broker's assets from colliding with the other consoles' and
keeps the broker's own routing self-contained.

The RabbitMQ documentation notes that a reverse proxy in front of the HTTP API needs care with
encoded slashes **only when the default vhost is used**; with a dedicated vhost (R3) that caveat does
not apply, and the route passes the path through unchanged.

**Alternatives considered**:

- *Publish the console's port on the host* — rejected: the console carries the queued codes in the
  clear; it must never be a public page (`BRK-05`).
- *No route at all; inspect the broker with the CLI on the host* — rejected by the plan's decision O6:
  the console is the operator's window into a stuck queue, and the CLI cannot show what is inside a
  message.

## R5. The broker's version is pinned to an exact minor

**Decision**: the broker's image tag names an exact 4.x minor version instead of the floating
`4-management` tag.

**Rationale**: the plan says the minor must be pinned at deploy time. A floating tag means a deploy
can silently replace the broker — the process that holds every queued code and owns the queue's format
— with a different version, at the moment the operator is least expecting it. Pinning makes the
broker's version a fact of the stack rather than a fact of the registry.

**Alternatives considered**:

- *Keep the floating tag and rely on the volume* — rejected: the image is what defines the queue
  format and the plugin set; the volume does not protect the binary.

## R6. The broker's metrics come from the plugin the image already ships

**Decision**: the stack enables the broker's Prometheus plugin in a small derived image, built from
the pinned broker image, so the plugin is on before the broker ever starts.

**Rationale**: the plugin ships with the management image but is disabled by default, and the
RabbitMQ documentation's own production recipe enables it with `rabbitmq-plugins enable`. Enabling it
at build time removes the race between the broker's first start and its health check — a health check
that passes while the plugin is still being enabled would let the publisher connect to a broker whose
metrics endpoint is not yet there.

**Alternatives considered**:

- *Enable the plugin in the service's command at run time* — rejected by the user's decision
  (2026-10-21): the broker's first start would mutate itself while the health check was racing it.
- *Scrape the management plugin's HTTP API instead of the Prometheus endpoint* — rejected: the
  documentation is explicit that Prometheus is the recommended option for long-term collection, and
  the API's statistics are a subset.

## R7. The monitoring stack scrapes the broker and the worker

**Decision**: two new scrape targets — the broker's Prometheus endpoint on the internal network and
the worker's own metrics listener — beside the existing ones for the site, the publisher and the
router.

**Rationale**: the plan's §5.9 lists the broker's own series (queue depth, unacknowledged messages,
consumer count) as the ones that answer "is the pipeline stuck, and where". The worker already
exposes its own counters; nothing was reading them until now.

**Alternatives considered**:

- *One scrape target per broker* — rejected: the broker runs on a single node.
- *Read the broker's numbers through the console's API* — rejected: the same reason as R6.

## R8. The stack's tests are the deployment's contract

**Decision**: the deployment's own Go test suite (`deploy/compose_test.go`, `deploy/traefik_test.go`)
is extended to assert this feature's contract, and the checks are written before the change, the way
the backend's suites are.

**Rationale**: the stack is configuration, so its contract is only as good as the checks that read it.
The suite already asserts the broker's exposure, the publisher's separation and the required-variable
list; the console's route, the accounts' separation and the new scrape targets belong in the same
place. A change that quietly republishes a port or drops a gate fails the build, not a review.

**Alternatives considered**:

- *Assert the stack in the shell* — rejected: the repository's checks are Go tests, and a shell
  assertion would not run in the continuous integration gate that already exists.

## R9. Deliberately out of this feature

**Decision**: the following stay out:

| Out | Reason |
|---|---|
| the alert rules, the dashboards and the stall runbook | the next feature; this feature only makes the numbers reachable |
| the gate's activation | the feature after that; this feature deploys the machinery the gate needs |
| a broker replica set | the plan keeps the broker on a single node for the MVP |
| TLS on the broker's own listener | the broker is on the internal network; the plan makes `amqps` a requirement only if the broker leaves it |
| a bounce or complaint webhook for the provider | an operational task outside this code deliverable |

## Resolved unknowns

| Technical Context item | Resolution |
|---|---|
| The worker's place in the stack | a service of its own, internal only, broker-dependent only (`R1`) |
| How the accounts are provisioned | operator commands, documented, idempotent (`R2`) |
| The vhost | a dedicated `legnaris` vhost, named in the broker's address (`R3`) |
| The console's route | the operator's domain, its own path prefix, the existing gate and headers (`R4`) |
| The broker's version | pinned to an exact 4.x minor (`R5`) |
| The metrics plugin | enabled at build time in a derived image (`R6`) |
| The scrape targets | the broker's Prometheus endpoint and the worker's listener (`R7`) |
| Where the contract is asserted | the deployment's Go test suite (`R8`) |
| Deferred scope | alerts and dashboards, the gate's activation, a replica set, broker TLS, provider webhooks (`R9`) |
