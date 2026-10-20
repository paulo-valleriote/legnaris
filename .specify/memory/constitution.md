<!--
Sync Impact Report
- Current version: 1.1.2 | Ratified: 2026-10-15 | Last amended: 2026-10-18
- Version history:
  - 1.0.0 → 1.1.0 (MINOR: two principles added; Additional Constraints and Development Workflow
    materially expanded; no principle removed or redefined).
  - 1.1.0 → 1.1.1 (PATCH: runtime-guidance reference updated now that the web client exists; no
    principle added, removed, or redefined).
  - 1.1.1 → 1.1.2 (PATCH: the deprecated-client clause, the runtime-guidance list, the traceability
    scope and the Development Workflow are corrected now that `desktop/` and `ci-flutter.yml` are
    removed; no principle added, removed, or redefined).
- Modified principles: none at 1.1.2. At 1.1.0, III. Behavior Ships With Tests was unchanged in text
  and became the gate for the web client through the stack constraint (Vitest / Testing Library /
  Playwright); no redefinition.
- Added principles: VI. Security Gates Are Blocking (NON-NEGOTIABLE) and VII. Installable,
  Offline-Capable Web Client (both 1.1.0).
- Added/removed sections: none (Additional Constraints and Development Workflow expanded in place).
- Stack change: the product client is the React/TypeScript PWA under `web/`. The Flutter desktop
  under `desktop/` is gone: it was deprecated, accepted no further feature work, and was deleted —
  together with `ci-flutter.yml` — once the Playwright parity gate passed. Requirement-ID
  traceability now covers Go and TypeScript sources.
- Amendment verified: `desktop/` and `ci-flutter.yml` no longer exist in the tree, the web parity
  gate is green (`web/e2e/baselines/` committed), and the CI job names `ci-lint`, `ci-spec-lint`
  and `ci-web` still match the workflows in `.github/workflows/`. The lint scanners retain `.dart`
  support for legacy sources, but no Dart source is part of the delivered surface.
- Deferred TODOs: none.
-->

# Legnaris Constitution

## Core Principles

### I. Typed Errors at Every Boundary

Domain errors MUST be expressed with `backend/shared/apperr` — `apperr.NewError(code, message, err)` —
never a bare `errors.New`. The cause passed to `apperr` MUST be `nil` or a lower-layer typed error.
Services MUST export the user-facing message constants (plain strings) that controllers reuse, and
each boundary MUST translate lower-layer errors with `errors.Is`/`errors.As` and map the typed code
to its documented HTTP status. A new failure mode without a typed code is not done.

Rationale: one error taxonomy keeps the API contract stable, makes failure paths testable, and
prevents string-matched errors from leaking across layers.

### II. Canonical Identifiers

Every server-generated entity ID MUST be a canonical lowercase UUID v7 produced by `backend/shared/idgen`
(`idgen.New`), and every inbound ID MUST be validated at the service boundary with `idgen.Validate`.
Calling `uuid.New`/`uuid.NewString` directly is forbidden. Code-defined template IDs are the sole
exception: they are slugs matching `^[a-z][a-z0-9_]*$`, validated where templates are fetched.

Rationale: a single generator and validator guarantee sortable, collision-resistant IDs and stop
ad-hoc formats from entering persisted data.

### III. Behavior Ships With Tests (NON-NEGOTIABLE)

Any change that adds or modifies behavior MUST ship tests in the same commit. Each suite MUST cover
edge cases first — nil/empty/zero values, boundaries, invalid and unexpected input, missing keys,
idempotency, concurrency — and then a representative happy path. Failure paths MUST assert the typed
`apperr` code and, at the HTTP boundary, the mapped status code. Tests are table-driven with
`t.Run` and `Happy path:` / `Edge:` / `Error:` case prefixes; assertions use `testify` (`require`
for preconditions, `assert` otherwise); mocks are mockery-generated (`NewMockX(t)`) and never
hand-written. `go test -race` MUST stay clean. The same rule holds for the web client: assertions
cover edge cases first, and interface requirements MUST be verified through rendering tests rather
than assumed. Exceptions: generated code, changes with no behavior (docs, comments, CI config), and
pure refactors that preserve the public contract.

Rationale: untested behavior is not delivered behavior; edge-case-first coverage is where defects
actually live.

### IV. English-Only Source

All identifiers, comments, log messages, and user-facing strings MUST be written in English. The
only exception is the technical design documents under `docs/`, which stay in pt-BR.

Rationale: a single language across every artifact keeps code, reviews, logs, and UI consistent and
searchable.

### V. Layered Architecture

Imports MUST respect `app → sheet → template → engine → shared`. `backend/internal/app` calls the sheet
service only; the sheet service calls `template` (DAG/Registry) and the repository; `template` calls
`engine` for formula evaluation; `engine`, the repository, and `backend/shared/*` MUST NOT import upward.
JSON decoding MUST go through `backend/shared/json`, and logging MUST go through `backend/shared/logger`.

Rationale: the DAG and the formula engine are the product's core; strict directionality keeps them
independently testable and prevents business logic from leaking into transport code.

### VI. Security Gates Are Blocking (NON-NEGOTIABLE)

Dependency scanning (SCA), static analysis (SAST), secret scanning, and dynamic analysis (DAST)
MUST run in CI on every pull request that touches the client, and MUST be blocking: no gate may be
disabled or run with `continue-on-error`, and a new finding MUST fail the pipeline even when the
build and tests are green. Secrets MUST never be committed; scanned artifacts (lockfiles, SBOMs)
MUST be kept in sync with the code. Browser clients MUST NOT persist credentials in
JavaScript-readable storage: the refresh credential is an `HttpOnly`, `Secure`, `SameSite` cookie,
the access token lives in memory only, and every state-changing cookie-authenticated route MUST be
protected against CSRF. The Content-Security-Policy MUST be delivered as a real HTTP header in
enforce mode, scripts MUST be same-origin (no third-party CDN, no inline), and DOM injection sinks
(`dangerouslySetInnerHTML`, `innerHTML`, `eval`, `new Function`) are forbidden.

Rationale: security that is not enforced by a pipeline is a promise, not a control; a client that
holds a session credential is only as safe as the weakest lens that never runs.

### VII. Installable, Offline-Capable Web Client

The product client MUST be an installable Progressive Web App served over HTTPS: it loads an
application shell that works offline for reading, resolves deep links to the same routes a
navigation would produce, and never caches authenticated responses or credentials in the service
worker. Interface requirements MUST be verified by component tests and end-to-end tests of the
primary flows; accessibility (no critical automated violations) and installability are part of
"done", not improvements.

Rationale: an installable PWA is the deliverable surface, so its offline, deep-link, and interface
contracts are product requirements that must be gated like any other behavior.

## Additional Constraints

- **Stack.** Backend: Go on Echo with MongoDB, entry point `backend/cmd/api.go`. Web client: React with
  Vite and TypeScript under `web/`, packaged as a PWA. Tests use `testify` and mockery-generated
  mocks on the backend; Vitest, Testing Library, and Playwright on the web client. All environment
  configuration comes from `backend/shared/env`.
- **No deprecated client.** The Flutter desktop under `desktop/` was deprecated, received no new
  features, and was deleted — together with its `ci-flutter.yml` workflow — once the web client's
  Playwright parity gate passed. No desktop client is part of the delivered surface.
- **Spec corpus and traceability.** Feature specifications live under `.specs/features/<feature>/`
  and are the single source of requirement IDs. Every requirement ID cited in Go or TypeScript
  source (e.g. `RND-21`, `AD-001`) MUST be defined in that corpus: `AD-*` in
  `.specs/STATE.md`, all other IDs in a `.specs/**/spec.md`. `LGN-*` identifiers are Jira card keys
  and are exempt. A citation that cannot be traced back to a requirement is a defect.
- **Jira.** Every pull request MUST implement its Jira card, referenced as `LGN-\d+` in the title;
  requirements come from the card, never invented.
- **Runtime guidance.** `AGENTS.md` and its per-directory companions (`backend/internal/AGENTS.md`,
  `backend/internal/sheet/template/AGENTS.md`, `backend/internal/engine/AGENTS.md`, `backend/shared/apperr/AGENTS.md`,
  `backend/shared/idgen/AGENTS.md`, `backend/shared/db/AGENTS.md`, and `web/AGENTS.md`) carry the implementation
  detail; this constitution supersedes them on conflict.

## Development Workflow

- Before opening a pull request run, in order: `make fmt && make lint && make vet && make
  lint-specs`, then `make test-race`; changes under `web/` additionally run `make lint-web && make
  test-web && make sec-scan`.
- CI gates a PR: `ci-lint` (golangci-lint + `go test -race`) is blocking on new issues;
  `ci-spec-lint` enforces non-vacuous assertions, existing requirement IDs (Go and TypeScript),
  and resolving `validation.md` citations; `ci-web` covers the web client with its security gates.
  Generated code is exempt from the behavior-ships-with-tests rule.
- Feature work follows the Spec Kit cycle — constitution → specify → clarify → plan → tasks →
  analyze → implement — and every produced spec keeps the repository's requirement-ID and
  edge-case-first conventions.
- Comments are documentation, never narration: doc comments on exported identifiers and
  "why"-comments only. `TODO`/`FIXME` requires a Jira card first; only trivial or blocked work may
  leave a `TODO(<owner>):` with a one-line rationale.

## Governance

- This constitution supersedes all other practices. On conflict it wins and the conflicting document
  MUST be updated to match.
- Amendments are made through the Spec Kit `/speckit.constitution` flow and MUST be documented with
  a rationale and a Sync Impact Report before the amended constitution is committed.
- Versioning is semantic: MAJOR for governance or principle removals/redefinitions that are
  backward incompatible, MINOR for a new or materially expanded principle or section, PATCH for
  clarifications and non-semantic refinements.
- Every pull request and review MUST verify compliance with these principles. Complexity that cannot
  be justified against a principle MUST be removed, or the principle amended first.

**Version**: 1.1.2 | **Ratified**: 2026-10-15 | **Last Amended**: 2026-10-18
