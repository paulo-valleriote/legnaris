# Implementation Plan: Typed Web API Client

**Branch**: `002-web-api-client` | **Date**: 2026-10-16 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/002-web-api-client/spec.md`

**Program plan**: `docs/plan-web-pwa.md` (Parte B, feature F2) — the umbrella plan this feature
implements.

## Summary

Give the web client one isolated network layer: six typed operations over a native transport, the
response envelope decoded in a single place, models that keep the template field tree and the sheet's
dynamic data faithful to the backend contract, and a complete mapping from backend error codes to the
failure kinds a screen can branch on. The layer is state-free and framework-free — it takes an
injected transport, so the transport is the single seam the authentication feature will attach a
credential to — and it is pointed at the public API over TLS by default, with an explicit build-time
override for local work. Attempts are bounded by a five-second fail-fast timeout; only reads repeat,
exactly once; writes never do. The contract is pinned by real captures taken from the running backend
so any drift fails the suite before a screen renders the wrong thing.

## Technical Context

**Language/Version**: TypeScript 5.9.3 with the F1 strictness settings (`strict`, `noUnusedLocals`,
`noUnusedParameters`, `noFallthroughCasesInSwitch`, `isolatedModules`, `resolveJsonModule`,
`@typescript-eslint/consistent-type-imports`) on React 19.3 / Vite 8.3, Node.js 24 LTS. The version is
the one the delivered F1 toolchain pins: `typescript-eslint` does not yet accept TypeScript 6, so the
plan's aspirational pin is corrected here and in `web/README.md`.

**Primary Dependencies**: none added at runtime — the client uses the platform `fetch`,
`AbortController`, `URL` and `JSON`. Tests use the Vitest 5.0.5 stack already installed by F1.
`MSW` (named in the program plan's stack table) is deliberately **not** added: the feature's own
contract is an injected transport (API-07), which makes call counts, bodies and retry behaviour
directly assertable, and the network-level interception MSW offers belongs to the authentication
feature, where the refresh flow needs it.

**Storage**: none. The client caches nothing, persists nothing, and holds no state; sheet library and
draft storage belong to the sheet-cycle feature.

**Testing**: Vitest 5.0.5 with pinned JSON captures under `web/src/api/fixtures/`, a fake transport for
the behaviour suite (call counting, retry, timeout via fake timers), and a self-skipping live suite
that runs against a real backend only when a base address is supplied through the environment.

**Target Platform**: browsers — the latest two versions of Chrome, Edge, Firefox and Safari, served
over HTTPS; the module is engine-agnostic and touches no DOM API beyond `fetch`/`AbortController`.

**Project Type**: web application (client) inside an existing Go monorepo; the backend is read, never
modified — this feature only consumes its contract.

**Performance Goals**: bounded failure, not throughput — every attempt is capped at 5 s and a read
costs at most two attempts plus one 300 ms delay, so the offline kind is reached in about 10.3 s worst
case.

**Constraints**: no implicit credentials on any request (API-29); no request body, credential or
identifier-bearing URL reaches a log (API-29); no user-facing literal outside the translation layer
(API-23); every source citation of an `API-*` requirement resolves through the spec lint already
covering `*.ts`/`*.tsx` (F1's `SEC-07`); no new runtime dependency.

**Scale/Scope**: 6 consumed operations, 7 failure kinds, 12 backend codes covered (5 with a dedicated
kind, the rest through the generic fallback), 4 models, 1 configuration resolver, 1 translation
catalog, 1 pinned capture set, 2 test suites (pinned + live).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The client's failures are typed, not string-matched: one domain error carrying status/code/message, one distinct defensive parse error, one typed path error (API-03, API-04, API-14, API-24). The backend's `shared/apperr` contract is consumed as-is; no backend error surface changes. | PASS |
| II. Canonical Identifiers | No identifier is generated here. Identifiers arriving from the backend are opaque and are never re-parsed or re-formatted; only URL interning is performed, via `encodeURIComponent` on the path segment. | PASS (not exercised) |
| III. Behavior Ships With Tests | Every requirement lands with a test in the same commit: the mapping table is table-driven (API-15…API-22), edge cases come first (empty body, malformed path, unmapped code, transport failure, already-applied attempts), and the interface-facing copy is asserted to come from the catalog (API-23). | PASS |
| IV. English-Only Source | All identifiers, comments, test names and user-facing strings are English. The translation catalog holds English copy only. | PASS |
| V. Layered Architecture | Only `web/` gains code; the Go layering (`app → sheet → template → engine → shared`) is untouched. Inside the client, the layer is a plain module: `l10n` and `shared`-style helpers are consumed, no React import, no shell coupling. | PASS |
| VI. Security Gates Are Blocking | No gate changes are needed: F1's `ci-web` pipeline already runs the suite. This feature must not weaken it, and adds the security requirements the plan names — no implicit credentials, no logging of bodies/credentials/URLs (API-29). | PASS (maintained) |
| VII. Installable, Offline-Capable Web Client | The offline kind (API-20) is the state the offline-capable client renders later; this feature delivers the kind, not the screen. No credential is stored, which keeps F1's FND-09 intact. | PASS |
| Additional Constraints — Stack | The module uses the stack the constitution names (React/Vite/TypeScript under `web/`); it adds no framework and no runtime dependency. | PASS |
| Additional Constraints — Traceability | `API-*` is a new prefix defined in `spec.md`; the F1 lint already scans `*.ts`/`*.tsx`, so citations in the delivered sources are gated without further work. | PASS |
| Additional Constraints — Jira | This is pre-work planning; the implementing PR must reference its `LGN-\d+` card. Recorded for the PR, not a gate failure here. | PASS (noted) |
| Development Workflow | The documented PR sequence (`make lint-web && make test-web`) already covers the new module; no Makefile change is required. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design kept the layer dependency-free and
state-free, put every user-visible string behind one catalog, and did not need to touch the CI
pipeline, the backend, or the shell. The one design decision that could have drifted — mocking through
a network interception library — was resolved in favour of the injected transport the contract already
requires, which keeps the security requirements (no implicit credentials, nothing logged) assertable at
the seam instead of at the platform.

## Project Structure

### Documentation (this feature)

```text
.specs/features/002-web-api-client/
├── spec.md              # Requirements (API-*)
├── plan.md              # This file
├── research.md          # Phase 0 — decisions and rejected alternatives
├── data-model.md        # Phase 1 — envelope, errors, models, configuration, captures
├── quickstart.md        # Phase 1 — runnable validation guide
├── contracts/           # Phase 1 — transport, endpoints, error mapping, capture provenance
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
web/src/
├── api/
│   ├── config.ts            # base address resolution: default, override, validation, security flag
│   ├── errors.ts            # domain error, defensive parse error, typed path error
│   ├── error_mapper.ts      # apperr code → failure kind, and kind → catalog message key
│   ├── envelope.ts          # the single envelope decoder (data / error / defensive parse)
│   ├── transport.ts         # transport request/response shapes + the native fetch adapter
│   ├── client.ts            # the six typed operations, timeout, read-only retry
│   ├── models/
│   │   ├── template.ts      # catalog entry, field node, field schema
│   │   └── sheet.ts         # sheet + dotted-path accessor
│   └── fixtures/            # pinned captures taken from the running backend + capture script
├── l10n/
│   └── strings.ts           # typed English catalog; the only place user-facing copy lives
└── test/
    ├── api_config.test.ts
    ├── api_envelope.test.ts
    ├── api_error_mapper.test.ts
    ├── api_models_template.test.ts
    ├── api_models_sheet.test.ts
    ├── api_client.test.ts
    ├── api_client_retry.test.ts
    ├── api_contract_captures.test.ts
    ├── api_live_contract.test.ts
    └── api_no_logging.test.ts
```

**Structure Decision**: the module lives under `web/src/api/` — the path the program plan already
reserves for it — with `l10n/` as its only sibling addition, because the contract requires that no
user-facing literal exists in the client and that copies resolve from the translation layer. Fixtures
sit inside the module (`api/fixtures/`) with the script that produced them, so a re-capture is a
documented, reviewable step rather than a mystery file drop. Tests stay flat in `web/src/test/` with
the `api_*` prefix, matching the layout F1 established.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
