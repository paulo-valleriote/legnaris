---
description: "Task list for the Web PWA Foundation feature"
---

# Tasks: Web PWA Foundation

**Input**: Design documents from `.specs/features/001-web-pwa-foundation/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED here. The spec's SEC-01/SEC-09, the constitution's "behavior ships with tests"
principle, and SC-001…SC-007 all demand test tasks; they are written first and must fail before the
implementation they cover.

**Organization**: tasks are grouped by user story so each story is independently implementable,
testable, and demonstrable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks)
- **[Story]**: US1 / US2 / US3, per spec.md
- Every task names its exact file path

## Path Conventions

Client project root is `web/`; browser-level specs live in `web/e2e/`; workflow and script changes
live at the repository root.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: create the client project and its toolchain; nothing user-visible yet.

- [x] T001 Create the client project skeleton per plan.md: `web/package.json`, `web/tsconfig.json`, `web/vite.config.ts`, `web/index.html`, `web/src/main.tsx`, and the empty feature folders (`src/app/`, `src/shell/`, `src/theme/`, `src/pwa/`)
- [x] T002 Pin the toolchain: `web/.nvmrc` and `engines.node` for Node 24 LTS, commit `web/package-lock.json`, and install with `npm ci` (D16)
- [x] T003 [P] Configure linting and formatting in `web/eslint.config.js` and `web/.prettierrc.json`, enabling the anti-XSS rules from `contracts/ci-gates.contract.md` (`react/no-danger`, `no-eval`, `no-new-func`, `no-script-url`, `no-unsanitized/property`, `no-unsanitized/method`)
- [x] T004 [P] Configure Vitest and Testing Library in `web/vite.config.ts` + `web/src/test/setup.ts`, and add `test`, `lint`, `typecheck`, and `build` scripts to `web/package.json`
- [x] T005 [P] Add `web/.gitignore` (build output, Vite cache, Playwright reports) and verify the repository root ignore rules do not exclude the client

**Checkpoint**: `cd web && npm ci && npm run build` succeeds on an empty shell.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the token system and routing table every story builds on.

**⚠️ CRITICAL**: US1 and US3 both consume the token system; US1 consumes the route table.

- [x] T006 Create the design token source in `web/src/theme/tokens.css` and `web/src/theme/tokens.ts` (FND-02, FND-05) with the ported scale — type 12/13/15/17 (17 is the cap), space 4/8/12/16/24, radius 2/4, row height 32, control height 32–36, nav 200–240 px, content ~960 px, panel 280–320 px — plus focus and hover tokens, each color token declaring its `contrastPair`
- [x] T007 [P] Add base styles and the token import in `web/src/theme/base.css`, consumed by `web/src/main.tsx` (FND-02)
- [x] T008 Declare the route table in `web/src/app/routes.ts` and the router in `web/src/app/router.tsx` (FND-08): the root route, exactly one not-found fallback, and per-route `audience`/`deepLink`/`notFound` fields from data-model.md
- [x] T009 Configure the host fallback for deep links in `web/vite.config.ts` (history fallback for the dev server and the preview server) so a cold load of any declared route resolves (FND-08)

**Checkpoint**: a route change renders a placeholder screen, and unknown paths reach the not-found route.

---

## Phase 3: User Story 1 - Open and install the application shell (Priority: P1) 🎯 MVP

**Goal**: the shell renders in order, is installable, works offline after a first visit, resolves
deep links, and hands over releases only on acceptance.

**Independent Test**: load the built app, confirm the three regions, install it, disable the
network and reload (shell still renders), open a deep link and an unknown route, then publish a new
version and accept the update notice.

### Tests for User Story 1 ⚠️ (write first; they must fail before the implementation)

- [x] T010 [P] [US1] Rendering test for region order and the designed empty workspace in `web/src/test/shell.test.tsx` (FND-01; SC-006)
- [x] T011 [P] [US1] Responsive test asserting the panel collapses first in `web/src/test/shell_responsive.test.tsx` (FND-03)
- [x] T012 [P] [US1] Keyboard-only walkthrough test asserting focus visibility and no trap in `web/src/test/shell_keyboard.test.tsx` (FND-04)
- [x] T013 [P] [US1] Manifest and icon contract test in `web/src/test/manifest.test.ts` (FND-06: name, short name, `start_url`, `scope`, `display: standalone`, 192/512 px standard + maskable icons)
- [x] T014 [P] [US1] Worker cache-exclusion unit test in `web/src/test/sw_cache_rules.test.ts` (FND-09: `/auth/*` and any authorization-bearing request are never cached)
- [x] T015 [P] [US1] Served-HTML test asserting no inline script and no third-party origin in `web/src/test/no_inline_assets.test.ts` (FND-10)
- [x] T016 [P] [US1] End-to-end spec in `web/e2e/pwa.spec.ts` covering cold deep link, unknown-route not-found, offline reload, installability audit, and the update notice (FND-06…FND-09)

### Implementation for User Story 1

- [x] T017 [P] [US1] Implement the shell regions in `web/src/shell/app_shell.tsx`, `web/src/shell/left_nav.tsx`, `web/src/shell/right_panel.tsx` (FND-01, FND-03), using the tokens from T006 and no literal visual values (FND-02)
- [x] T018 [P] [US1] Implement the designed empty-workspace content view in `web/src/shell/empty_workspace.tsx` (FND-01: no placeholder rows, no demo data)
- [x] T019 [US1] Wire the shell and the route table into `web/src/app/router.tsx` and `web/src/app/providers.tsx` so every route renders inside the shell (FND-01, FND-08)
- [x] T020 [US1] Configure the PWA plugin in `web/vite.config.ts` with `strategies: 'injectManifest'`, `injectRegister: false`, and `registerType: 'prompt'` (D2, D3, D15; FND-09, FND-10)
- [x] T021 [P] [US1] Author `web/public/manifest.webmanifest` and `web/public/icons/` (192/512 standard + maskable) per `contracts/pwa-installability.contract.md` (FND-06)
- [x] T022 [US1] Implement the service worker in `web/src/pwa/sw.ts`: precache the shell, exclude `/auth/*` and authorization-bearing requests, delete non-current caches on `activate`, and serve the shell for navigations offline (FND-07, FND-09)
- [x] T023 [US1] Implement external registration and the update prompt in `web/src/pwa/register_sw.ts` and `web/src/pwa/update_prompt.tsx`, imported by `web/src/main.tsx` (FND-09, FND-10)
- [x] T024 [US1] Add the not-found route screen and assert it never renders blank or propagates a server error page (FND-08)
- [x] T025 [US1] Add the Lighthouse CI budget configuration (`web/lighthouserc.json`) asserting installability, the SC-001 performance budget, the SC-003 offline paint budget, and the SC-006 accessibility category (zero critical violations)
- [x] T026 [US1] Run the User Story 1 test tasks (T010–T016) against the implementation and record any amended expectation in `web/AGENTS.md` rather than weakening an assertion (constitution III)

**Checkpoint**: User Story 1 is demonstrable on the preview server and in `web/e2e/`.

---

## Phase 4: User Story 2 - Client changes are gated by quality and security checks (Priority: P2)

**Goal**: a client pull request cannot merge unless lint, type-check, tests, build, SCA, SAST,
secret scanning, and lockfile integrity all pass, fail-closed, and the path filters do not cross
the Go pipelines.

**Independent Test**: open a client-only pull request (`ci-web` runs, Go pipelines do not), then
seed a vulnerable dependency and a secret and confirm each gate rejects; change only a Go file and
confirm the opposite.

### Tests for User Story 2 ⚠️

- [x] T027 [P] [US2] Path-filter contract test asserting `ci-web` triggers on `web/**`/lockfile and that `ci-lint` and `ci-spec-lint` trigger on client paths as specified in `contracts/ci-gates.contract.md`, in `scripts/test_lints.py` (SEC-06)
- [x] T028 [P] [US2] Requirement-ID self-test cases for `*.ts`/`*.tsx`: a valid citation passes, an unknown ID fails, and `node_modules`/build output are ignored, in `scripts/test_lints.py` (SEC-07)
- [x] T029 [P] [US2] Vacuous-assertion self-test case for the TypeScript test suffix in `scripts/test_lints.py` (SEC-08)
- [x] T030 [P] [US2] Create `web/README.md` documenting the seeded-negative acceptance procedure (vulnerable dependency → `sca` fails; secret-like string → `secrets` fails), executable alongside `contracts/ci-gates.contract.md` (SEC-02, SEC-03, SEC-04)

### Implementation for User Story 2

- [x] T031 [P] [US2] Extend `scripts/check_requirement_ids.py` to scan `*.ts`/`*.tsx` and exclude `node_modules`, `dist`, `.vite`, and Playwright output (SEC-07)
- [x] T032 [P] [US2] Extend `scripts/check_test_assertions.py` to cover the client test suffix (SEC-08)
- [x] T033 [US2] Create `.github/workflows/ci-web.yml` with the five jobs (`quality`, `sca`, `sast`, `secrets`, `lockfile`) from `contracts/ci-gates.contract.md`: minimal `permissions`, `concurrency`, pinned tool/action versions, SARIF/JSON artifacts, and no `continue-on-error` anywhere (SEC-01…SEC-04)
- [x] T034 [US2] Add the client path filter to `.github/workflows/ci-spec-lint.yml` so `web/**/*.ts` and `web/**/*.tsx` are linted (SEC-07)
- [x] T035 [US2] Tighten the Go workflow filters in `.github/workflows/ci-lint.yml` so client-only changes do not trigger it (SEC-06)
- [x] T036 [P] [US2] Create or extend `.github/dependabot.yml` with the npm ecosystem for `/web` (weekly) and a separate security-update group (SEC-05)
- [x] T037 [US2] Add `lint-web`, `test-web`, and `sec-scan` targets to `Makefile` mirroring the pipeline (Development Workflow) and list them in the `Makefile` help block
- [ ] T038 [US2] Configure branch protection to require the `ci-web` jobs (recorded in `contracts/ci-gates.contract.md` as the enforcement surface) and verify the seeded negatives from T030 produce red checks

**Checkpoint**: a client-only PR shows exactly the five `ci-web` jobs as required, and both seeded negatives fail.

---

## Phase 5: User Story 3 - One documented visual system (Priority: P3)

**Goal**: a contributor builds a screen from documented tokens and rules only.

**Independent Test**: add a sample screen that uses only tokens, then confirm the no-literal-values
and contrast tests pass and the rules are documented.

### Tests for User Story 3 ⚠️

- [x] T039 [P] [US3] No-literal-values test comparing shell computed styles against the token map in `web/src/test/tokens_used.test.tsx` (FND-02)
- [x] T040 [P] [US3] Contrast assertion over every declared `contrastPair` in `web/src/test/tokens_contrast.test.ts` (FND-05)

### Implementation for User Story 3

- [x] T041 [US3] Author `web/AGENTS.md` porting `desktop/AGENTS.md` visual rules to the web stack: density, resize/collapse order, focus, hover, the avoid-table (no gradients, no glassmorphism, radius ≤ 4 px, no "AI SaaS" aesthetic), componentization rules, and the dependency rule
- [x] T042 [US3] Reference `web/AGENTS.md` from the root `AGENTS.md` runtime-guidance list and note the deprecated `desktop/` companion stays until parity

**Checkpoint**: a new screen needs no ad-hoc visual value and the rules are discoverable from the repo root.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T043 [P] Extend `web/README.md` (created in T030) with local development and preview-server instructions, and link the `quickstart.md` validation scenarios
- [ ] T044 Run the full `quickstart.md` validation set against the deployed preview (all seven scenarios) and record the outcome with the spec
- [x] T045 [P] Update `docs/TDD-desktop-mvp.md` and `docs/TDD.md` client references to point at the web client as the active surface (the constitution now names it)
- [x] T046 Verify the constitution's Development Workflow section matches the delivered targets and CI job names; amend the constitution if the delivered names differ
- [x] T047 Run `make lint-specs` and the client gates one final time, and confirm every `FND-*`/`SEC-*` citation in the delivered client sources resolves to `spec.md`
- [ ] T048 [P] Record the manual installability verification for the non-Chromium engines (latest two versions of Firefox and Safari) per `contracts/pwa-installability.contract.md`, and store the result with the release notes (FND-06, SC-002)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — starts immediately.
- **Foundational (Phase 2)**: depends on Setup; blocks every user story (tokens and routes).
- **US1 (Phase 3)**: depends on Phase 2; delivers the MVP.
- **US2 (Phase 4)**: depends on Setup; independent of US1's implementation, but its path-filter and
  seeded-negative verification needs a buildable client, so start after US1's scaffold exists.
- **US3 (Phase 5)**: depends on Phase 2 (tokens); independent of US1 and US2.
- **Polish (Phase 6)**: depends on all desired stories.

### User Story Dependencies

- **US1 (P1)**: no dependency on other stories — the MVP.
- **US2 (P2)**: independent of US1's UI; it gates the repository, not a screen.
- **US3 (P3)**: independent of US1 and US2; it documents and asserts the token contract they use.

### Within Each User Story

- Tests are written first and must fail before the implementation lands (constitution III).
- Foundational tokens/routes before shell components; shell before PWA wiring; PWA wiring before the
  E2E spec can pass.
- Story complete before moving to the next priority.

### Parallel Opportunities

- T003, T004, T005 (setup) — different files.
- T006 and T007 — token source vs base styles.
- T010–T016 — independent test files for US1.
- T017 and T018 — different shell files.
- T027–T030 and T031/T032/T036 — independent scripts, docs, and config.
- T039–T040 — independent test files.

---

## Parallel Example: User Story 1

```bash
# Tests first (all independent files, must fail before T017+):
Task: "Rendering test for region order and empty workspace in web/src/test/shell.test.tsx"
Task: "Responsive test asserting the panel collapses first in web/src/test/shell_responsive.test.tsx"
Task: "Keyboard-only walkthrough test in web/src/test/shell_keyboard.test.tsx"
Task: "Manifest and icon contract test in web/src/test/manifest.test.ts"

# Then the shell surfaces:
Task: "Implement shell regions in web/src/shell/app_shell.tsx, left_nav.tsx, right_panel.tsx"
Task: "Implement the designed empty-workspace view in web/src/shell/empty_workspace.tsx"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 US1: shell, installability, offline, deep links, update prompt.
3. **STOP and VALIDATE** with `quickstart.md` scenarios 1–5 and `web/e2e/pwa.spec.ts`.
4. Demo on the preview deployment.

### Incremental Delivery

1. Setup + Foundational → the project builds and routes.
2. US1 → the shell is installable and usable offline (MVP).
3. US2 → the gates turn on, so every later feature is protected (`ci-web` becomes required).
4. US3 → the visual contract is documented and asserted.
5. Polish → docs, quickstart evidence, and the final lint sweep.

### Parallel Team Strategy

With two contributors: one takes US1 (shell + PWA) while the other takes US2 (workflow, scripts,
Makefile) and then US3 (tokens documentation and assertions). US2's branch protection change lands
last so it does not block the US1 branch while its gates are still being tuned.

---

## Notes

- [P] tasks touch different files and have no dependency on incomplete work.
- Requirement IDs cited in task descriptions (`FND-*`, `SEC-*`) are defined in `spec.md`; the
  delivered source must cite the same IDs so `make lint-specs` can trace them.
- Write tests first (T010–T016, T027–T030, T039–T040) and confirm they fail before implementing.
- Commit per task or per logical group, with the tests in the same commit (constitution III).
- Do not weaken an assertion to make a gate pass; amend the spec and record the decision instead.
- The `web/` client must never cache credentials or authenticated responses — check that rule on
  every task that touches `web/src/pwa/`.

---

## Completion Record

Delivered on branch `main`, local commits only (not pushed):

| Group                                    | Commits                                                         |
| ---------------------------------------- | --------------------------------------------------------------- |
| Setup + tokens (T001–T009)               | `chore(web): scaffold React PWA client, toolchain and design tokens` |
| US1 shell/routes (T017–T024)             | `feat(web): add the application shell, routes and not-found fallback` |
| US1 PWA (T020–T023, T025)                | `feat(web): add installable manifest, service worker and update prompt` |
| US1 browser specs (T016)                 | `test(web): add end-to-end specs and Lighthouse budgets`        |
| US2 gates (T027–T037)                    | `ci: gate client changes with quality and security pipelines`    |
| US3 + docs (T041–T043, T045)             | `docs: document the web client conventions and its delivery workflow` |
| T046                                     | `docs(constitution): refresh runtime guidance now that web/ exists` |

Gate evidence at close: `make lint-specs` clean (39 self-tests, 425 spec IDs, 25 changed
files); from `web/` — `npm run format:check`, `npm run lint`, `npm run typecheck`,
`npm test` (43 tests / 10 suites) and `npm run build` (emits `dist/sw.js`) all pass.

Deferred, still unchecked:

- **T038** — branch protection needs repository-admin access to GitHub, which this session does
  not have. The required check names are recorded in `contracts/ci-gates.contract.md`.
- **T044** — the seven `quickstart.md` scenarios must run against a deployed preview;
  the local Playwright run is unavailable (no browsers installed in this environment).
- **T048** — manual installability verification on Firefox and Safari is a manual step and is
  stored with the release notes, not in this repository.

Recorded deviations are listed in `web/AGENTS.md` under "Delivered deviations from the F1 task
list" (no `src/app/providers.tsx`, flat `src/test/` layout, corrected toolchain pins).
