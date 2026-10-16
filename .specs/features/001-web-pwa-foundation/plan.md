# Implementation Plan: Web PWA Foundation

**Branch**: `001-web-pwa-foundation` | **Date**: 2026-10-15 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/001-web-pwa-foundation/spec.md`

**Program plan**: `docs/plan-web-pwa.md` (Parte B, feature F1) — the umbrella plan this feature
implements.

## Summary

Stand up the product's web client under `web/` as an installable Progressive Web App with a dense,
token-driven application shell, plus the blocking quality and security pipeline that every later
client feature inherits. The shell renders navigation, content, and a contextual panel (content
showing a designed empty-workspace state until the catalog feature lands), is reported installable
by the target browser matrix, boots offline after a first visit, and switches versions only when the
visitor accepts. A path-scoped `ci-web` workflow runs lint, type-check, unit tests, build,
dependency scanning, static analysis, secret scanning, and lockfile integrity as non-optional
gates, and the existing requirement-ID lint is extended to TypeScript so client citations stay
traceable.

## Technical Context

**Language/Version**: TypeScript 7.0 (strict), React 19.3, built with Vite 8.3; Node.js 24 LTS
pinned via `engines` + `.nvmrc` locally and selected explicitly in the CI workflow (T033).

**Primary Dependencies**: `vite-plugin-pwa` 2.0 (Workbox under the hood) for the service worker and
manifest; `workbox-window` for update notification; `idb` 8.0 for versioned IndexedDB access in
later features. No UI component library: tokens plus plain CSS modules are the visual layer
(per the desktop rule "no design-system dependency in V1").

**Storage**: browser-local only — IndexedDB for cache and drafts (later features); this feature
persists no credentials and no authenticated data. No server-side storage is introduced.

**Testing**: Vitest 5.0 + Testing Library 16.3 + MSW 3.0 for unit and rendering tests; Playwright
1.64 + Lighthouse CI for installability, offline, deep-link, and accessibility checks; ESLint 10.12
+ Prettier 3.9 for static style, with `no-unsanitized`, `react/no-danger`, `no-eval`, and
`no-new-func` enabled.

**Target Platform**: browsers — latest two versions of Chrome, Edge, Firefox, and Safari; served
over HTTPS. Automated verification covers the Chromium engine; Firefox and Safari are verified
manually at each release (resolution recorded in `contracts/pwa-installability.contract.md`).

**Project Type**: web application (client) inside an existing Go monorepo; the backend stays
untouched by this feature.

**Performance Goals**: shell visible under 2.5 s at the 75th percentile on a mid-range mobile
profile (Lighthouse mobile throttling) and offline shell paint under 1 s (SC-001, SC-003).

**Constraints**: no inline script and no third-party script origin (FND-10) so the content policy
can be enforced later; no credentials or authenticated responses in offline storage (FND-09); every
security gate fail-closed with no `continue-on-error` (SEC-04); CI path filters must not trigger the
Go pipelines for client-only changes (SEC-06).

**Scale/Scope**: three always-present shell regions, one declared root route plus the not-found
route in this feature, one service worker scope, five gate groups, one new CI workflow, one
Dependabot ecosystem entry.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | No backend error surface is added; the client mapper arrives in the API-client feature. No bare `errors.New` is introduced. | PASS (not exercised) |
| II. Canonical Identifiers | No entity ID is generated or validated in this feature. | PASS (not exercised) |
| III. Behavior Ships With Tests | Every behavior in this feature ships a test: shell regions and token application (unit/rendering), installability/offline/deep-link (E2E), gate behaviour (seeded-failure test), spec traceability (lint self-test). | PASS |
| IV. English-Only Source | All identifiers, comments, strings, and docs are English; `docs/*` (pt-BR) is untouched. | PASS |
| V. Layered Architecture | Only `web/` is added; the Go layering (`app → sheet → template → engine → shared`) and `shared/json`/`shared/logger` rules are untouched. | PASS |
| VI. Security Gates Are Blocking | This feature is where SCA, SAST, secret scanning, and lockfile integrity become required checks, with no `continue-on-error` and a fail-closed rule (SEC-01…SEC-04). | PASS — implements the principle |
| VII. Installable, Offline-Capable Web Client | Installability, offline shell, deep-link resolution, interface tests, accessibility, and no credentials in offline storage are the feature's core (FND-06…FND-10, SC-002/SC-003/SC-006). | PASS — implements the principle |
| Additional Constraints — Stack | React + Vite + TypeScript under `web/` is the stack the amended constitution names; `shared/env` remains the only configuration source for the backend. | PASS |
| Additional Constraints — Traceability | `FND-*` and `SEC-*` are defined in `spec.md`; SEC-07 extends the lint to `*.ts/tsx` so citations stay traceable. | PASS |
| Additional Constraints — Jira | The plan is pre-work; the implementing PR must reference its `LGN-\d+` card. Deferred to the PR, not a gate failure here. | PASS (noted) |
| Development Workflow | Adds `make lint-web`, `make test-web`, `make sec-scan` so the documented PR sequence covers client changes. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design added no new dependency on a
waived principle: tokens are CSS variables (no design-system dependency), the worker is
external-registered (keeps FND-10 satisfiable), gates stay fail-closed, and traceability for
TypeScript is part of the plan rather than a follow-up. The only cross-cutting risk is that CI
branch protection must list the `ci-web` jobs as required — recorded as an explicit task and
tracked in `contracts/ci-gates.contract.md`.

## Project Structure

### Documentation (this feature)

```text
.specs/features/001-web-pwa-foundation/
├── spec.md              # Requirements (FND-*, SEC-*)
├── plan.md              # This file
├── research.md          # Phase 0 — decisions and rejected alternatives
├── data-model.md        # Phase 1 — shell, tokens, routes, gates, offline artifacts
├── quickstart.md        # Phase 1 — runnable validation guide
├── contracts/           # Phase 1 — shell layout, PWA/installability, CI gates, traceability
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
web/
├── AGENTS.md                     # visual/interaction rules ported from desktop/AGENTS.md
├── package.json  package-lock.json  tsconfig.json  vite.config.ts  index.html
├── .nvmrc  .gitignore  eslint.config.js  .prettierrc.json
├── public/
│   ├── manifest.webmanifest
│   └── icons/                    # maskable + standard sizes
└── src/
    ├── main.tsx                  # entry; registers the service worker externally
    ├── app/{router.tsx,providers.tsx,routes.ts}
    ├── shell/{app_shell.tsx,left_nav.tsx,content_region.tsx,right_panel.tsx,empty_workspace.tsx}
    ├── theme/{tokens.css,tokens.ts,base.css}
    ├── pwa/{sw.ts,register_sw.ts,update_prompt.tsx}
    └── test/{setup.ts,shell.test.tsx,installability.test.ts,offline.test.ts}
web/e2e/{shell.spec.ts,pwa.spec.ts}
.github/workflows/ci-web.yml
.github/dependabot.yml            # added (npm, /web)
scripts/check_requirement_ids.py  # extended to *.ts/*.tsx
Makefile                          # lint-web / test-web / sec-scan targets
```

**Structure Decision**: one new top-level client project (`web/`) alongside the existing Go backend
and the deprecated `desktop/`. Feature-scoped folders (`shell/`, `theme/`, `pwa/`, `app/`) mirror
the ported desktop mental model (screen/shell/engine separation) so later features add `engine/`,
`sheet/`, `catalog/`, and `library/` without reorganizing. Tests live next to the code, with
browser-level specs under `web/e2e/`.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
