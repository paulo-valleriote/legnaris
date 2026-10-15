# Phase 0 — Research: Web PWA Foundation

All Technical Context unknowns are resolved here. Each entry records the decision, its rationale,
and the alternatives that were rejected, so the design phase does not revisit them.

## D1 — Build and UI stack

- **Decision**: React 19.3 + TypeScript 7.0 (strict) + Vite 8.3, versions pinned in
  `web/package.json` and installed with `npm ci` from a committed lockfile.
- **Rationale**: the constitution names the React/Vite/TypeScript PWA as the product client; Vite
  gives a small, fast build with first-class PWA plugin support, and TypeScript keeps the
  requirement-ID and interface contracts checkable at build time.
- **Alternatives considered**: Flutter Web (near-zero rewrite of the desktop engine, but canvas
  rendering produces poor SEO/a11y, a large bundle, atypical PWA/routing, and a thin SAST/SCA
  ecosystem for Dart — it fails the security-gate goal). Next.js (adds a server runtime the project
  does not need; the API is a separate Go service). Plain esbuild without Vite (no PWA plugin
  ecosystem, manual manifest/SW wiring).

## D2 — PWA strategy: `injectManifest`, not `generateSW`

- **Decision**: `vite-plugin-pwa` 2.0 with `strategies: 'injectManifest'` and a custom
  `src/pwa/sw.ts`.
- **Rationale**: this feature needs behaviour `generateSW` cannot express — never caching
  authenticated responses, excluding `/auth/*`, cleaning stale caches on `activate`, and controlling
  the update flow; later features add draft/offline behaviour to the same worker.
- **Alternatives considered**: `generateSW` (simplest, but the generated worker's runtime-caching
  routes cannot encode the "never cache authenticated" rule safely, and there is no seam to add
  draft handling later). A hand-written service worker without the plugin (loses precache manifest
  generation and dev-mode integration with the Vite build graph).

## D3 — Service worker registered from an external script

- **Decision**: `injectRegister: false`, and an explicit `navigator.serviceWorker.register()` call
  from `src/pwa/register_sw.ts`, imported by the entry module.
- **Rationale**: FND-10 forbids inline script so a strict content policy can be enforced later; the
  plugin's inline registration helper would be blocked by `script-src 'self'` without
  `unsafe-inline`.
- **Alternatives considered**: `injectRegister: 'auto'`/`'inline'` (writes an inline registration
  script into `index.html` — blocks enforce-mode CSP). Registering from a snippet in `index.html`
  (still inline).

## D4 — Design tokens as CSS custom properties with a typed mirror

- **Decision**: `theme/tokens.css` holds the values as CSS custom properties; `theme/tokens.ts`
  exports the same names for tests and dynamic styling. Tokens are ported from
  `desktop/AGENTS.md` (type scale 12/13/15/17 with 17 as the cap, spacing 4/8/12/16/24, radius 2/4,
  row/control height, nav 200–240 px, content max ~960 px, panel 280–320 px) and extended with the
  focus ring and AA-checked foreground/background pairs the web shell needs.
- **Rationale**: one source with two access paths keeps "no magic numbers" provable in tests, and
  CSS variables are the only mechanism that needs no component library.
- **Alternatives considered**: a Tailwind-style atomic framework (introduces a second styling
  language and a large dependency for a dense, token-driven UI). A CSS-in-JS library (runtime cost,
  and tokens become code-only, so the CSS contract cannot be inspected). A component library
  (prohibited by the project's "no design-system dependency in V1" rule and would import an
  "AI SaaS" look the design rules veto).

## D5 — Routing with history, resolved by host fallback plus service worker

- **Decision**: client-side router with history-style URLs; the host serves `index.html` for
  unknown paths and the service worker serves the shell for navigations after the first visit.
- **Rationale**: FND-08 requires deep links to render the same screen as in-app navigation, which
  needs both a server fallback (cold load) and a worker fallback (offline load). Declared routes
  live in one module so the not-found state and the deep-link test can enumerate them.
- **Alternatives considered**: hash routing (avoids the host fallback but produces ugly, non-
  canonical URLs and does not behave like a normal web app). A single-page app with no deep links
  (fails FND-08).

## D6 — Test stack

- **Decision**: Vitest + Testing Library for component/unit behaviour, MSW for request stubbing
  (used from the next feature onward), Playwright for installability/offline/deep-link/accessibility
  end-to-end checks, and Lighthouse CI for the performance and installability budgets.
- **Rationale**: this is the only combination that covers both halves of the project's testing rule
  — logic (edge cases first) and interface requirements verified by rendering — while running
  without a browser server in unit mode.
- **Alternatives considered**: Jest (slower, needs extra transform configuration over Vite).
  Cypress (single-tool convenience, but weaker accessibility/Lighthouse integration and a heavier
  runtime). Manual verification only (fails "interface requirements are verified through tests").

## D7 — CI topology: one path-scoped client workflow

- **Decision**: a new `.github/workflows/ci-web.yml` triggered by `web/**` and its own file, plus a
  `package.json`/lockfile path, and narrowing the existing Go pipeline so it does not run for
  client-only changes (SEC-06).
- **Rationale**: path scoping keeps feedback fast and satisfies SEC-06 in both directions without
  a monorepo build tool.
- **Alternatives considered**: one monolithic workflow with conditional jobs (every client change
  would still spin the Go toolchain). A third-party monorepo runner (Nx/Turborepo — a large
  dependency for two projects).

## D8 — Software composition analysis and dependency integrity

- **Decision**: `osv-scanner` (lockfile + OS package scan), `Trivy fs`, `npm audit` (including
  signature verification) in CI, plus Dependabot for npm under `/web` with a distinct group for
  security updates; `npm ci` (frozen lockfile) and lockfile-lint for integrity.
- **Rationale**: overlapping scanners have different advisory sources; the CI gate must be
  independent of GitHub's own advisory feed, and SEC-05 requires automated updates with security
  updates distinguishable.
- **Alternatives considered**: only `npm audit` (single advisory source, limited coverage).
  Only Dependabot (does not gate an incoming PR that adds a vulnerable dependency).

## D9 — Static application security testing

- **Decision**: CodeQL (JavaScript/TypeScript, `security-extended`) plus Semgrep with
  `p/owasp-top-ten`, `p/typescript`, and `p/react` rulesets.
- **Rationale**: CodeQL provides dataflow analysis with GitHub-native reporting;
  Semgrep adds OWASP-shaped rules and custom rules (forbidding DOM injection sinks, requiring the
  CSRF header on cookie-authenticated calls in the auth feature).
- **Alternatives considered**: ESLint security plugins alone (style-level only). A single SaaS
  scanner (adds a licence and a secret to manage; the repo already standardises on GitHub-native).

## D10 — Secret scanning

- **Decision**: `gitleaks` in CI plus GitHub secret scanning and push protection at the repository
  level.
- **Rationale**: SEC-03 requires the gate to block merge and not be suppressible repository-wide;
  push protection stops the secret before it reaches history, CI catches the rest.
- **Alternatives considered**: GitHub secret scanning alone (does not cover self-hosted runners or
  pre-push). A pre-commit hook only (bypassable with `--no-verify`).

## D11 — Gate policy: fail-closed, no `continue-on-error`

- **Decision**: every quality/security job runs unconditionally on the trigger paths, no job carries
  `continue-on-error`, and a gate that cannot run fails the pipeline. Required checks are recorded
  in `contracts/ci-gates.contract.md`, and the acceptance test seeds a vulnerable dependency and a
  secret to prove each gate fails closed.
- **Rationale**: SEC-04 exists because a silently skipped gate is indistinguishable from a passing
  one; the seeded-negative test is the only way to prove the gate is real.
- **Alternatives considered**: advisory scanners with a warning annotation (defeats the purpose).
  Enforcing only on `main` after merge (violates "blocking on the PR").

## D12 — Requirement-ID traceability for TypeScript

- **Decision**: extend `scripts/check_requirement_ids.py` to scan `*.ts`/`*.tsx` (adding them to
  `CODE_SUFFIXES` and the CI path filter), keeping the existing rule that every cited ID must be
  defined in `.specs/**/spec.md` (or `.specs/STATE.md` for `AD-*`), with `LGN-*` still exempt.
- **Rationale**: SEC-07 and the constitution's traceability clause; without the extension, web
  citations have no gate. The script's 30-case self-test already covers the parser, so the change is
  a suffix plus a fixture.
- **Alternatives considered**: a separate TypeScript-only lint (two rules to keep in sync). Not
  gating citations in the client (silently downgrades the constitution's traceability rule).

## D13 — Browser matrix verification

- **Decision**: automation (Playwright + Lighthouse) runs on the Chromium engine; Firefox and Safari
  are verified manually at each release using a recorded checklist, as clarified in the spec.
- **Rationale**: Chromium is the only engine with reliable installability tooling and worker
  control in CI; automating Safari/Firefox would need paid or self-hosted runners for marginal
  coverage of a criterion that is about the install affordance.
- **Alternatives considered**: Playwright on all three engines in CI (does not test offline/install
  affordances; multiplies runtime). BrowserStack/Sauce for Safari (external dependency and cost).

## D14 — Installability definition

- **Decision**: installable = HTTPS + a valid manifest (name, short name, `start_url`, `scope`,
  `display: standalone`, 192/512 px icons including a maskable icon) + a registered service worker
  controlling `start_url`, verified by the Lighthouse installability audit and a Playwright
  assertion on the manifest and worker state.
- **Rationale**: FND-06 and SC-002 need a definition that CI can assert, not an impression.
- **Alternatives considered**: relying on the browser's install prompt heuristic (not assertable).
  A curated manifest only (fails on Safari, which needs no installation prompt to be useful but
  still requires the manifest for "Add to Home Screen").

## D15 — Offline shell and update flow

- **Decision**: precache the shell (HTML, entry chunks, CSS, icons) at build time; runtime cache only
  public, non-authenticated GETs; never cache `/auth/*` or responses carrying an authorization
  header; delete previous caches on `activate`; `skipWaiting: false` with a prompt-to-reload update
  UI (`registerType: 'prompt'`).
- **Rationale**: FND-07, FND-09, and the clarified update answer; a user-accepted reload is the only
  way to avoid mixing versions while someone has unsaved work (the desktop's AD-008 draft decision
  relies on the same principle).
- **Alternatives considered**: `skipWaiting: true` (silent version mixing). Cache-first for all GETs
  (risks caching authenticated data, violating FND-09). No runtime cache at all (offline read for
  later features becomes impossible).

## D16 — Node toolchain pinning

- **Decision**: pin Node 24 LTS via `engines.node` and `.nvmrc`, and use `npm ci` everywhere.
- **Rationale**: reproducible CI; the repository's other pipelines already pin their toolchains.
- **Alternatives considered**: floating `node: current` (non-reproducible). pnpm/yarn (a second
  package manager in a repo that uses none — no benefit at this size).
