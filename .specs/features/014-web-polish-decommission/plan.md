# Implementation Plan: 014-web-polish-decommission (F10)

**Branch**: `014-web-polish-decommission` | **Date**: 2026-10-17 | **Spec**: `.specs/features/014-web-polish-decommission/spec.md` | **Status**: Implemented (specify → clarify → plan → tasks → analyze → implement; user authorized implementation)

**Input**: Feature specification from spec.md (POL-01…POL-10; visual parity, CSP enforce, cookie/auth removal, rate limit, CSP report, security gates, desktop removal, traceability).

**Summary**: Close F9 hardening by enforcing CSP (POL-02), validating cookie auth removal (POL-03/04/05), confirming all security gates pass (POL-06), passing the visual parity gate (POL-01), removing desktop and cleaning CI/docs (POL-07/08/09), and shipping with tests + requirement traceability (POL-10).

---

## Technical Context

**Language/Version**: TypeScript / React / Vite / Vitest / Playwright (web client); Go 1.23 / Echo (backend — existing). Verified: `go version` and `node --version` present in env; `npm` under `web/`.

**Primary Dependencies**: `vite-plugin-pwa` (`injectManifest`), `idb` (IndexedDB), `tanstack-query`, `zustand`, `react-router`, `osv-scanner`, `trivy`, `semgrep`, `codeql`, `zap-baseline`, `syft`, `gitleaks`, `axe-core`, Playwright.

**Storage**: MongoDB (backend, existing); IndexedDB (`idb`) local (web cache/drafts — existing since F3/F5); no new persistence for F10.

**Testing**: Vitest + Testing Library + MSW (unit/interface); Playwright + axe (E2E / a11y / visual); `osv-scanner` / Trivy / Semgrep / CodeQL / ZAP / gitleaks (security gates, blocking).

**Target Platform**: HTTPS web (`app.legnaris.com.br` / API `api.legnaris.com.br`), installable PWA, cross-origin same-site.

**Project Type**: Web PWA (frontend) + backend closure (cookie/CSP/rate-limit/report endpoint removal and enforcement).

**Performance Goals**: Not primary — closure feature; CSP/report endpoint must respond <200ms; rate-limit enforcement must not degrade auth latency.

**Constraints**: Security gates blocking (Principle VI); no `continue-on-error`; desktop deprecated (constitution §Additional Constraints); parity gate must pass before removal completion; `POL-*` IDs isolated.

**Scale/Scope**: Single feature (014), closes program F10; touches `internal/app/auth.go`, `internal/app/router.go` (CSP/report), `web/src/`, `.github/workflows/`, `Makefile`, `AGENTS.md`.

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence / Action for F10 |
|---|---|---|
| I. Typed Errors | Clear | Existing `apperr`; no new boundary errors introduced by F10 (only removal/enforcement of existing paths). |
| II. Canonical IDs | Clear | No new server entities; existing UUID v7 unchanged. |
| III. Behavior Ships With Tests | Clear | POL-10 requires edge + happy path; Playwright + Vitest + Go tests for cookie/CSP/rate-limit. |
| IV. English-Only | Clear | Spec, comments, and docs in English; `docs/plan-web-pwa.md` stays pt-BR by exception. |
| V. Layered Architecture | Clear | F10 touches backend `app` layer (cookie/CSP) and frontend `web/`; no cross-import created. |
| VI. Security Gates Blocking | **GATE** | All gates (`osv-scanner`, Trivy, CodeQL, Semgrep, gitleaks, ZAP, SBOM) must pass; CSP enforce; cookie attributes verified; rate-limit active. |
| VII. Installable / Offline Capable | Clear | PWA already installed (`injectManifest`); F10 only enforces CSP / closes auth; offline shell preserved. |

**Post-design re-check** (after research/data-model): confirm no new security finding is introduced by CSP enforce or report endpoint; confirm `desktop/` removal doesn't break `Makefile` targets for web.

---

## Project Structure (chosen: Option 2 — Web + backend closure)

```text
backend (existing):
  internal/app/auth.go          # remove X-Refresh-Token; enforce cookie read; rate limit
  internal/app/router.go        # CSP header injection; /csp-report route; CORS allowlist
  internal/auth/service.go      # cookie rotation / family revocation
  shared/env/                   # CSP_REPORT_URI, CORS_ALLOWED_ORIGINS

frontend (web/):
  web/src/auth/auth_session.ts  # memory-only access token; silent refresh via cookie
  web/src/auth/auth_api.ts     # credentials: 'include'; X-CSRF-Token
  web/src/pwa/sw.ts            # no auth-caching; no credential caching
  web/src/theme/               # tokens / theme (already exists; parity uses this)
  web/src/shell/app_shell.tsx  # visual reference for comparison
  web/src/api/authenticated_client.ts # 401 → single-flight refresh

CI / repo (closure):
  .github/workflows/ci-web.yml  # security jobs (already exists; enhance gate enforcement)
  Makefile                      # desktop targets removed; keep lint-web / test-web / sec-scan
  desktop/                      # REMOVED (parity gate green; see parity-gate.contract.md)
  ci-flutter.yml                # REMOVE
```

**Structure Decision**: Keep existing web + backend monorepo; F10 only modifies existing paths and deletes deprecated ones. No new service or package created.

---

## Complexity Tracking

No constitution violations requiring justification — feature is purely closure / enforcement / removal, with no new architectural elements, no new storage, no new external services.

---

## Phase 0 — Research (to produce `research.md`)

**Objective**: Resolve any open design decisions for CSP enforce rollout, cookie attribute verification, and desktop removal ordering.

**Decisions / Rationale / Rejected Alternatives** (to document):

- **CSP rollout**: `Report-Only` (current) → collect → fix → `enforce`. Rejected: jump to enforce without collection (risky for unknown third-party / extension noise); rejected: stay in report-only (fails POL-02 / Principle VI).
- **Cookie refresh path removal**: remove `X-Refresh-Token` read + `Set-Cookie` prohibition test; rely solely on `__Host-legnaris_refresh`. Rejected: keep both (maintains desktop dependency, violates decommission goal).
- **Rate limit shared key**: use IP (+ identity when auth present) for `/auth/*` and `/csp-report`. Rejected: separate limiters (complex; shared is sufficient for closure). *As delivered, the plan's open decision was resolved the other way: `/csp-report` reuses the `ratelimit.Limiter` implementation but with its own instance, env (`CSP_REPORT_RATE_LIMIT_ATTEMPTS`/`CSP_REPORT_RATE_LIMIT_WINDOW`, default 30/1m) and key namespace `csp:ip:`, because the `/auth/*` default (10/1m) is too low for report bursts.* See `tasks.md` T012/T013.
- **Desktop removal gate**: only after PLAYWRIGHT parity passes (`POL-01`). Rejected: remove before gate (violates plan's definition of done / risk of broken UI without reference).
- **Reference artifact storage**: Playwright screenshots in CI artifacts (not in repo). Rejected: commit reference images (bloats repo; artifacts sufficient for review). *As delivered, the plan's decision was reversed: a small, stable baseline set is committed under `web/e2e/baselines/` so `toHaveScreenshot` is a real gate instead of passing vacuously when no artifact is downloaded.* See `contracts/parity-gate.contract.md` and `tasks.md` T003/T004.

---

## Phase 1 — Design (to produce `data-model.md`, `contracts/`, `quickstart.md`)

**Data model impact**: None new — using existing
template / sheet / CSP report normalized struct; rate-limit counter is ephemeral (shared limiter state, not persisted).

**Contracts** (to produce under `contracts/`):
- `cookie-auth.contract.md`: cookie attributes, rotation, family revocation, `X-Refresh-Token` absence.
- `csp-enforce.contract.md`: header content, report endpoint behavior, rate-limit integration.
- `parity-gate.contract.md`: Playwright comparison criteria, pass/fail thresholds, removal authorization.
- `desktop-removal.contract.md`: file set to delete; `Makefile` / CI adjustments; constitution update.

**Quickstart** (to produce): how to verify each POL gate locally (`curl -I` for CSP, `grep` for removal, `npm run test` + `npm run sec-scan` for security, Playwright for parity).

---

## Constitution Check (post-design)

Re-verified: no new dependencies violate Principle VI; CSP enforce and cookie enforcement strengthen, not weaken, security posture. Desktop removal aligns with deprecated-client clause (constitution §Constraints). Requirement-ID `POL-*` isolated and traceable.

---

## Open Questions (for Clarify — resolved; no critical open)

None — spec complete, checklist complete, no [NEEDS CLARIFICATION]. Plan proceeds to tasks.
