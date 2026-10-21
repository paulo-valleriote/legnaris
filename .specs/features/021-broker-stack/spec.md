# Feature Specification: The Broker Stack — The Delivery Worker Runs, and the Broker's Console Is the Operator's

**Feature Branch**: `021-broker-stack`

**Created**: 2026-10-21

**Status**: Draft

**Input**: User description: "Deploy the email pipeline: the delivery worker as its own service, the broker's management interface behind the operator gateway, separated broker accounts and a dedicated delivery vhost, and the broker's own metrics exposed for scraping" (plan `docs/plan-auth-otp-2fa-email.md` §5.8 and §9 stage 6; decisions D6, O2, O6; findings F-12, F-16)

## Clarifications

### Session 2026-10-21

- Q: How should the operator's broker account be provisioned? → A: By an operator command documented in the runbook, reading the password from the environment (user decision, 2026-10-21): no credential material enters the repository, no password hash is generated at deploy time, and creating the account is a visible operator act rather than something the stack does silently (`BRK-10`, `BRK-11`, `BRK-16`).
- Q: How should the broker's own metrics plugin be enabled? → A: Through a small derived image that enables it at build time (user decision, 2026-10-21): the plugin is on before the broker ever starts, so the health check never races a broker that is still configuring itself, and the stack's service gains a build step instead of a command it has to trust (`BRK-14`).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - The delivery worker runs beside the site, and its failures stay beside it (Priority: P1)

Someone has asked for a code; the code was written down and handed to the message broker. For the code to
reach an inbox, something has to be running that takes it out of the broker and sends it. That
something is a separate program, and it must actually be running in the stack — started, restarted when
it dies, waiting for the broker before it starts consuming, reachable by nobody outside the machine. It
must never be a dependency of the site: if the worker is down, or the broker is down, or the provider is
refusing every address, people can still use the site, and only the code is late.

**Why this priority**: without the process in the stack, everything the last three features built stops
at a queue. The code is durable, but nothing sends it, and a person waiting for a code cannot sign in.

**Independent Test**: read the stack description and assert the worker is a service of its own, built
from the same image as the site, reachable only inside the machine, publishing no host port, waiting for
the broker to be healthy before it consumes, and absent from the site's own dependencies; assert the
site's required-variable list is unchanged.

**Acceptance Scenarios**:

1. **Given** the running stack, **When** the worker service is inspected, **Then** it exists as its own service, built from the shared image and started on its own entrypoint, and the site does not depend on it. (BRK-01, BRK-03)
2. **Given** the running stack, **When** the worker's exposure is inspected, **Then** it is reachable only from inside the machine and publishes no host port. (BRK-02)
3. **Given** the broker is down, **When** the site is used, **Then** the site keeps serving: the site's start-up does not wait for the broker or the worker. (BRK-03, BRK-20)
4. **Given** the worker service, **When** its settings are inspected, **Then** it carries exactly the broker address, the provider settings and the worker's own settings, and the site's own service carries neither the broker address nor the worker's settings. (BRK-04)

---

### User Story 2 - The broker's console is the operator's, not the public's (Priority: P1)

The broker holds every queued code in the clear, and its console can read them. That console must never
be a public page: it is the operator's window into a queue that carries short-lived credentials. It has
to live on the operator's own domain, behind the same sign-in gate the other operator pages use, with
the same security headers, under a path of its own so it does not collide with the other consoles. And
it must never be reachable by the ordinary path into the machine.

**Why this priority**: the console is the one place where a code is visible to someone who did not ask
for it. The plan makes this an explicit operational decision (O6): the console is exposed, but only
through the operator's gateway, and never publicly.

**Independent Test**: read the edge configuration and assert a route to the broker's console exists on
the operator's domain under its own path prefix, carrying the operator's sign-in gate and the security
headers; assert the broker service publishes no host port for it and that the site itself serves no
route to it.

**Acceptance Scenarios**:

1. **Given** the edge configuration, **When** the operator's domain is inspected, **Then** a route to the broker's console exists under its own path prefix, and it carries the operator's sign-in gate and the security headers. (BRK-07, BRK-08)
2. **Given** the running stack, **When** the broker's exposure is inspected, **Then** no host port is published for its console, and the site itself exposes no route to it. (BRK-05, BRK-07)
3. **Given** a request to the console path without the operator's sign-in, **When** it is made, **Then** the gate answers before the broker is reached. (BRK-08)

---

### User Story 3 - The broker's accounts are separated by the job they do (Priority: P1)

The application connects to the broker to move code in and out; the operator connects to the console to
look at what is stuck. Those are different powers and must not share an account. The application's
account may only see the one place where the code queues live, and it must never be able to open the
console. The operator's account must be a different account with the right to use the console. Neither
credential may ever be written into the repository.

**Why this priority**: one shared account with console rights would make the application — and anything
that can reach the broker — able to read every queued code and change the queue's configuration. The
plan calls for the separation explicitly (O6).

**Independent Test**: read the stack description and the operator's runbook and assert the
application's account is confined to the delivery vhost and carries no console right, the operator's
account is a distinct one with the console right, and no broker credential appears in the repository;
assert the automated checks fail if the application account ever gains console rights.

**Acceptance Scenarios**:

1. **Given** the broker's accounts, **When** the application's account is inspected, **Then** it may reach only the delivery vhost and may not use the console. (BRK-09)
2. **Given** the broker's accounts, **When** the operator's account is inspected, **Then** it is a different account from the application's and may use the console. (BRK-10)
3. **Given** the repository, **When** it is inspected, **Then** no broker credential is committed: every secret comes from the environment or from an operator action. (BRK-11)
4. **Given** the stack's automated checks, **When** the application account's rights widen, **Then** a check fails. (BRK-15)

---

### User Story 4 - The broker's own state can be read by the monitoring stack (Priority: P2)

When the pipeline stops, the first question is whether the broker is holding work nobody took. The
broker knows how many messages are waiting, how many are unacknowledged and whether anyone is
listening — and that information has to reach the monitoring stack from inside the machine, without
opening a port to the world.

**Why this priority**: the next feature's alerts read these numbers. Without them, a stalled pipeline
is discovered by a person who cannot sign in.

**Independent Test**: read the stack description and the monitoring configuration and assert the broker
exposes its metrics on the internal network only, publishes no host port for them, and that the
monitoring stack scrapes both the broker and the worker.

**Acceptance Scenarios**:

1. **Given** the running stack, **When** the broker's metrics endpoint is inspected, **Then** it is reachable only from inside the machine and publishes no host port. (BRK-12)
2. **Given** the monitoring configuration, **When** the scrape targets are inspected, **Then** the broker's metrics and the worker's metrics are both scraped. (BRK-13)
3. **Given** the broker's base image, **When** it is inspected, **Then** it names an exact 4.x version, so a deploy cannot silently change the broker under the operator's feet. (BRK-06)

---

### Edge Cases

- **The broker is down when the stack starts**: the worker waits for the broker's health check; the
  site starts and serves regardless, and the broker's queued work survives the outage because it is
  durable. (BRK-03, BRK-20)
- **The worker crashes**: it is restarted by the stack, and the message it was holding is redelivered by
  the broker — the worst case is one duplicate send, which the provider de-duplicates. (BRK-01)
- **The worker is deployed before the broker's console route exists**: the worker works unchanged; the
  console is an operator's convenience, not a dependency of the delivery. (BRK-01)
- **An operator reaches the console path without signing in**: the gate answers first; the broker is
  never asked. (BRK-08)
- **The application's account is used by something else that can reach the broker**: it still cannot
  open the console and cannot see any vhost but the delivery one. (BRK-09)
- **The operator's account is not created yet**: the console refuses the login; the runbook says how to
  create it, and the application keeps working because it does not use that account. (BRK-10, BRK-16)
- **The broker's base image floats**: a deploy could bring a different broker version with a
  different queue format; pinning the exact minor version removes that surprise, wherever the pin
  sits — the service's tag or the derived image's base. (BRK-06)
- **The monitoring stack cannot reach the broker's metrics**: the broker's own numbers are missing
  while the application's are not — the monitoring configuration must reach the broker from the same
  internal network. (BRK-13)
- **A broker credential appears in the repository**: prohibited; the secret scan and the deployment's
  checks must fail on it. (BRK-11)

## Requirements *(mandatory)*

### Functional Requirements

**The worker in the stack (US1)**

- **BRK-01**: The stack MUST run the delivery worker as a service of its own, built from the same image as the request-serving process and started on its own entrypoint, so it can be restarted or scaled independently.
- **BRK-02**: The worker service MUST be reachable only from inside the machine and MUST publish no host port.
- **BRK-03**: The worker service MUST wait for the broker to be healthy before it begins consuming, and the request-serving process MUST NOT depend on the worker or on the broker to start or to serve.
- **BRK-04**: The worker service's environment MUST carry the broker address, the provider settings and the worker's own settings, and the request-serving process's environment MUST carry neither the broker address nor the worker's settings.
- **BRK-20**: A broker outage or a worker failure MUST NOT take the request-serving process down; the stack MUST restart the failed service and the durable work MUST survive the outage.

**The console behind the operator's gate (US2)**

- **BRK-05**: The broker service MUST publish no host port and MUST declare a health check; the console MUST NOT be reachable from the host by any route.
- **BRK-06**: The broker's base image MUST be pinned to an exact 4.x version, never a floating minor tag, whether the pin sits in the stack's service tag or in the base of the derived image the service builds.
- **BRK-07**: The broker's console MUST be reachable only through the edge router, on the operator's domain, under a path prefix of its own; the request-serving process MUST expose no route to it and no other domain MUST route to it.
- **BRK-08**: The route to the console MUST carry the operator's sign-in gate and the security headers, exactly like the other operator-only pages, so an unauthenticated request is answered by the gate before the broker is reached.

**The accounts separated by role (US3)**

- **BRK-09**: The application's broker account MUST NOT be able to use the console and MUST be confined to the delivery vhost, where the code queues live.
- **BRK-10**: A distinct operator account MUST be able to use the console, and the two accounts MUST NOT be the same one.
- **BRK-11**: No broker credential may be committed: the accounts' secrets MUST come from the environment or from an operator action, and the secret scan MUST fail on a committed one.
- **BRK-17**: The console shows the queued messages and their contents, so it MUST be reachable only through the operator's route and only with the operator's account; the application's account MUST NOT be able to read them.

**The broker's own metrics (US4)**

- **BRK-12**: The broker's metrics endpoint MUST be reachable only from inside the machine and MUST publish no host port.
- **BRK-13**: The monitoring stack MUST scrape the broker's metrics and the worker's metrics.
- **BRK-14**: The broker's metrics MUST be exposed by the broker's own documented mechanism, enabled before the broker starts rather than while it is starting, with no broker plugin that the pinned version does not ship.
- **BRK-18**: The broker's queued messages MUST survive a broker restart, so a deploy that replaces the broker does not lose the codes in flight.

**Cross-cutting**

- **BRK-15**: The deployment's automated checks MUST assert this feature's contract — the worker's service and exposure, the broker's exposure and pinned version, the console's route and its gate, the accounts' separation and the monitoring targets — and MUST fail when any of them drifts.
- **BRK-16**: The operator's runbook MUST document how the console is reached, how the operator's account is provisioned, what every new variable does and what to do when the pipeline stalls.
- **BRK-19**: All identifiers, comments, log messages, runbook text and user-facing strings MUST be written in English.
- **BRK-21**: The stack's documented required-variable list MUST NOT change: the broker's and the worker's settings are validated by the processes that need them, never by the request-serving process.
- **BRK-22**: Every requirement MUST ship tests: the deployment's checks are the tests, edge cases first (a service that must not exist, a port that must not be published, a route that must not be public), then the representative happy path.

### Key Entities

- **Worker service**: the stack's entry for the delivery process — its own image entrypoint, its own
  environment, its own network reach and its own restart.
- **Broker service**: the message broker in the stack — internal only, health-checked, version-pinned.
- **Operator route**: the edge rule that carries the operator's domain and path prefix to the broker's
  console through the sign-in gate.
- **Application account**: the broker account the delivery processes use, confined to the delivery
  vhost and without console rights.
- **Operator account**: the distinct broker account the maintainer uses to open the console.
- **Scrape targets**: the endpoints the monitoring stack reads — the site's, the publisher's, the
  worker's and the broker's.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of the delivery worker's exposure is internal: 0 host ports published by the worker service.
- **SC-002**: 0 broker consoles reachable without the operator's sign-in, and 0 broker consoles reachable from the host.
- **SC-003**: 0 applications can open the broker's console with the application's own account.
- **SC-004**: 0 broker credentials committed to the repository.
- **SC-005**: The site's documented required-variable list is unchanged by this feature.
- **SC-006**: The broker's version is pinned: 0 floating broker image tags in the stack.
- **SC-007**: 100% of the pipeline's four processes — the site, the publisher, the worker and the broker — expose their metrics to the monitoring stack from inside the machine.
- **SC-008**: A broker outage leaves the site serving: 0 requests fail because the broker or the worker is unavailable.
- **SC-009**: The deployment's checks fail on every drift this feature forbids: 0 undetected changes to a service's exposure, a route's gate or an account's rights.
- **SC-010**: The deployment's checks are race-clean and non-vacuous; every check states what it forbids.

## Assumptions

- This is the seventh feature derived from `docs/plan-auth-otp-2fa-email.md` §9. It delivers the
  deployment slice of stage 6: the delivery worker joins the stack, the broker's console is put behind
  the operator's gate, the broker's accounts are separated by role, and the broker's own metrics become
  readable from inside the machine.
- The delivery worker itself — the program, its policy and its counters — is the preceding feature
  (`.specs/features/020-email-worker/`); this feature deploys it and does not change what it does.
- The alert rules, the dashboards and the stall runbook that read these metrics are the next feature;
  this feature only makes the numbers reachable.
- The broker runs on a single node, as the plan decides: without high availability an outage delays
  emails, but the durable intentions and the durable queue mean they are not lost.
- The console exposes the queued codes in the clear, so it is treated as a credential surface: the
  operator's route, the operator's account, and nothing else.
- The operator's account is provisioned by an operator action documented in the runbook rather than by a
  committed secret, so no broker credential ever enters the repository (clarification, 2026-10-21).
- The broker's metrics plugin is enabled by a small derived image, so it is on before the broker starts
  and the stack's health check never races a broker that is still configuring itself (clarification,
  2026-10-21).
- The stack's required-variable list stays exactly what it is; the broker's and the worker's settings
  are validated by the processes that need them.
- Requirement-ID prefix `BRK` is exclusive to this feature; `D6`, `O2`, `O6`, `F-12` and `F-16` belong
  to the plan and are not requirement IDs.
