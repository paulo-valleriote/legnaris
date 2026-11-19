# Legnaris context

The context files explain how each part of Legnaris works: its purpose, the
reasoning behind it, its flows and contracts, where the code lives, what it
depends on, and what it cannot do yet. They are written for a person (or an
agent) about to change that area, not as API documentation.

Read the owning file before changing code, and update it in the same pull
request when behaviour, contracts, boundaries or dependencies move. The
machine-readable owners are in [`context/paths.yaml`](context/paths.yaml);
`backend/internal/contextdocs` enforces the map and the template in
`go test ./...`.

## Which file to read

| Context file | Read it when you touch... |
| --- | --- |
| [product](context/product.md) | The problem, the users, V1 scope, product rules and brand commitments; read it before shaping a feature or deciding what is out of scope. |
| [architecture](context/architecture.md) | Layering, import rules, the composition root, shared API conventions and shared primitives; read it before adding a package, an import edge or a shared helper. |
| [auth](context/auth.md) | Registration, sign-in, OTP and second factors, sessions and cookies, JWT, account lifecycle and access control. |
| [sheets](context/sheets.md) | Templates, the field DAG, recalculation, sheet CRUD, layout, import/export and the sheet library. |
| [dice](context/dice.md) | Dice grammar, parsing, fitting dice expressions to rows and evaluation. |
| [generation](context/generation.md) | AI-assisted generation, prompts, the LLM gateway, consent, quotas and generation limits. |
| [billing](context/billing.md) | Stripe subscriptions, plans and entitlements, trials, refunds, billing webhooks and the customer portal. |
| [campaigns](context/campaigns.md) | Campaign membership, roles and permissions, invite codes, the join flow and shared sheets. |
| [events-and-email](context/events-and-email.md) | The event and email outboxes, relay, workers, queue topology and transactional mail delivery. |
| [admin](context/admin.md) | CI/CD workflows, deployment topology, observability stack and operational runbooks. |
| [web-client](context/web-client.md) | The React PWA shell, routing, the typed API client, local storage, localization, testing, theme and shared UI. |
| [platform](context/platform.md) | Runtime configuration and env keys, MongoDB and Redis, logging, metrics, middleware, rate limits, health and background maintenance. |

## File template

Every feature file carries the same seven second-level headings, in this order.
There is one template, described here; the files themselves contain only the
headings. Do not add, remove, rename or reorder them.

| Heading | What belongs there |
| --- | --- |
| `## Purpose` | What this area is for, in one short paragraph. |
| `## Why` | The decisions and constraints that shaped it; link to ADRs or specs instead of retelling them. |
| `## Flows` | The end-to-end paths a user or a job can take, named by entry point. |
| `## Contracts` | Boundaries other code depends on: requests, responses, invariants, errors, events, persisted shapes and env keys. State the rule, not the implementation. |
| `## Code map` | The packages, folders and entry points that own this area and what each one does. |
| `## Tools & dependencies` | Libraries, services, external APIs and operational tooling this area relies on. |
| `## Known limits` | What is deliberately out of scope, what is unfinished, and the failure modes that still exist. |

## Update rules

- A pull request that changes behaviour, a contract, a persisted shape, an env
  key, a dependency or an ownership boundary updates the owning context file in
  the same pull request.
- A new package under `backend/internal/`, a new package under
  `backend/shared/`, a new folder under `web/src/`, a new env key, or a change to
  `deploy/`, `.github/workflows`, `go.mod` or `web/package.json` also updates
  [`context/paths.yaml`](context/paths.yaml). Every target maps to exactly one
  context file.
- Documentation-only, formatting and purely mechanical changes need no context
  edit. When a code change genuinely has no context impact, add the
  `no-context-change` label to the pull request: it is the explicit exemption
  the context-update check honours. The label never substitutes for documenting
  a real behaviour or contract change.
- Keep entries current and short. Delete claims that stopped being true instead
  of rewriting around them; Git history and Jira carry the rationale that does
  not affect future readers.
- Write in English. Visual values stay in [`web/DESIGN.md`](web/DESIGN.md); do
  not restate them here.
