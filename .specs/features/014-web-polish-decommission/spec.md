# Feature Specification: Web PWA Polish and Desktop Decommission (F10)

**Feature Branch**: `014-web-polish-decommission`

**Created**: 2026-10-17

**Status**: Draft (specify step)

**Input**: User: follow Spec Kit for F10 (`web-polish-decommission`) per `docs/plan-web-pwa.md` v3.1 — visual parity gate, desktop removal, security hardening closure.

**Requirement-ID prefix**: `POL-` (verified unique in `.specs/`; no existing `POL-` usage).

---

## User Scenarios & Testing

### User Story 1 — Visual parity validated (Priority: P1)

As a user, I see the web sheet/template UI matching the desktop reference, verified by screenshot comparison, so I trust the migration is complete.

**Why this priority**: The parity gate (F10-T004) is the hard blocker for removing `desktop/`; without it, decommission is unsafe.

**Independent Test**: Run Playwright golden comparison against desktop reference template; compare screenshots pixel-level for shell + sheet form + catalog screens.

**Acceptance Scenarios**:

1. **Given** the reference desktop render of the template `Template-01`, **When** the web app opens the same sheet, **Then** Playwright screenshot diff is within perceptual threshold (no missing controls / wrong colors).
2. **Given** a completed edit with `Modified` / `Recalculated` marks (AD-009), **When** rendered in web, **Then** tint/marks match desktop.

---

### User Story 2 — Security gates all green with enforce (Priority: P1)

As a security owner, I see CSP in enforce mode, headers present, DAST/ZAP clean, no new secrets, so I approve removal of legacy desktop CI.

**Why this priority**: Principle VI (Security Gates Are Blocking, non-negotiable); F9-T001/T008 and F1-T008 must stay green.

**Independent Test**: Run `make sec-scan` (osv-scanner + Trivy + Semgrep + CodeQL + ZAP baseline + gitleaks); assert zero new findings; assert CSP header is `enforce`; assert `X-Refresh-Token` removed.

**Acceptance Scenarios**:

1. **Given** a PR that touches `web/`, **When** CI runs, **Then** all security jobs pass with no `continue-on-error`.
2. **Given** the backend `POST /auth/refresh`, **When** called, **Then** no `X-Refresh-Token` header is accepted (path removed).
3. **Given** the CSP policy at `app.legnaris.com.br`, **When** loaded, **Then** header is `Content-Security-Policy` (not only `Report-Only`) with `report-uri` / `report-to` set.

---

### User Story 3 — Desktop fully removed and CI cleaned (Priority: P2)

As a maintainer, I find no `desktop/` folder, no `ci-flutter.yml`, and `Makefile` only references web/backend, so the stack is unified.

**Why this priority**: Completes the decommission (plan §F10-T005/T006) and updates the constitution to reflect deprecated desktop.

**Independent Test**: `grep -r "desktop/" Makefile .github/workflows/ .specify/` returns nothing; `ls desktop/` fails; `ci-flutter.yml` absent.

**Acceptance Scenarios**:

1. **Given** the repo at `main`, **When** `git ls-files` is filtered, **Then** no paths under `desktop/` remain.
2. **Given** the root `AGENTS.md`, **When** read, **Then** it states web as the only client and desktop as removed.

---

### User Story 4 — Coverage, docs, and traceability (Priority: P2)

As a reviewer, I see `web/README.md`, `quickstart.md`, requirement-ID traceability for `*.ts/tsx`, and `check_requirement_ids.py` covering TS, so the feature is auditable.

**Independent Test**: `make lint-specs` passes with `POL-*` resolved; `check_requirement_ids.py` scans `web/src/`; coverage report ≥ meta.

**Acceptance Scenarios**:

1. **Given** TypeScript source under `web/src/`, **When** `check_requirement_ids.py` runs, **Then** citations of `POL-*`, `AD-*`, `SEC-*`, `FND-*` resolve to `.specs/`.
2. **Given** the feature directory, **When** `checklists/requirements.md` is inspected, **Then** all `POL-*` requirements are mapped to tasks.

---

### Edge Cases

- **Parität fails (screenshot diff exceeds threshold)**: parity gate blocks decommission; no deletion of `desktop/` until gate passes.
- **New CSP violation reported**: collect in Loki/Grafana first; correct violations; only then flip to enforce (F9-T008).
- **Rate-limit / CSRF test fails after removal of native auth path**: backend must still enforce `SameSite=Strict` + double-submit + `Origin`; regression must fail CI.
- **Coverage below meta after decommission**: do not merge; add missing interface tests (Playwright + axe) before removal.

---

## Requirements

### Functional Requirements

- **POL-01**: The web UI MUST pass a visual paridade gate against the desktop reference (Playwright golden screenshot comparison) for the template sheet screen, catalog, and shell, before any desktop removal is authorized.
- **POL-02**: The CSP MUST be delivered as a real HTTP header in `enforce` mode (not `Report-Only`) with full policy (`default-src 'self'`; `script-src 'self'`; `connect-src 'self' https://api.legnaris.com.br`; `frame-ancestors 'none'`; `require-trusted-types-for 'script'`; `report-uri` + `report-to`) on `app.legnaris.com.br`.
- **POL-03**: The backend MUST remove the native client refresh path entirely: no `X-Refresh-Token` header accepted; `POST /auth/refresh|logout` MUST read refresh exclusively from the `__Host-legnaris_refresh` cookie (`HttpOnly`; `Secure`; `SameSite=Strict`; `Path=/`).
- **POL-04**: The backend MUST enforce rate limiting on `/auth/*` and on `POST /csp-report` (shared limiter); exceeding the window MUST return HTTP 429 with `Retry-After`.
- **POL-05**: The backend CSP report endpoint (`POST /csp-report`, no auth) MUST sanitize `document-uri` / `blocked-uri` (strip query/fragment), cap body (~16–64 KB), normalize both `application/csp-report` and `application/reports+json`, emit structured log to Loki, emit Prometheus metric with labels (`effective-directive`, `disposition`), respond 204, and use the shared rate limiter.
- **POL-06**: All security CI gates (`osv-scanner`, Trivy fs, CodeQL, Semgrep, gitleaks, ZAP baseline, SBOM / Syft, dependency-review) MUST pass with zero new findings and MUST NOT use `continue-on-error`; a new finding MUST block the pipeline.
- **POL-07**: The `desktop/` directory, `ci-flutter.yml`, and all references to Flutter/desktop in `Makefile`, `.gitignore`, root docs, and `.specify/` MUST be removed once the parity gate passes.
- **POL-08**: The root `AGENTS.md` and `.specify/memory/constitution.md` MUST be updated to reflect the deprecated desktop, the web-only stack, and the `POL-*` / `AD-*` traceability for TypeScript sources.
- **POL-09**: `check_requirement_ids.py` MUST cover `*.ts` / `*.tsx` sources under `web/`; `make lint-specs` MUST resolve all `POL-*` IDs.
- **POL-10**: The feature MUST ship with tests covering: parity diff (Playwright), CSP header presence / enforce mode, cookie attribute assertions, `X-Refresh-Token` absence, rate-limit 429, CSP report normalization / sanitization, security gate zero-findings, desktop removal, and requirement-ID resolution.

*No [NEEDS CLARIFICATION] markers remain.*

### Key Entities

- **Reference Render** (desktop): the fixed visual baseline for parity comparison; not persisted, only used by Playwright tests.
- **CSP Report Payload**: normalized struct holding `effective-directive`, `disposition`, `document-uri` (sanitized), `blocked-uri`; emitted to Loki + Prometheus.
- **Freshness / Rate Limit Counter**: shared limiter state for `/auth/*` and `/csp-report`; key = IP (+ identity when available).
- **Desktop Decommission Set**: `desktop/`, `ci-flutter.yml`, `Makefile` entries, docs references, `.specify/` references.

---

## Success Criteria

### Measurable Outcomes

- **SC-01**: Playwright golden comparison passes with perceptual parity ≥ threshold on template/sheet/catalog/shell; no missing UI elements vs desktop reference.
- **SC-02**: `make sec-scan` reports zero new findings across SCA (osv-scanner + Trivy + audit) / SAST (CodeQL + Semgrep) / secrets (gitleaks) / DAST (ZAP baseline) / SBOM (Syft); all blocking (no `continue-on-error`).
- **SC-03**: `curl -I https://app.legnaris.com.br` shows `Content-Security-Policy` header with `enforce` disposition (not only `Report-Only`); `report-uri` or `report-to` present; `frame-ancestors 'none'`; Trusted Types requested.
- **SC-04**: `grep -i "X-Refresh-Token" internal/app/auth.go` and `grep -i "Set-Cookie.*refresh"` tests confirm native path removed; cookie attributes (`__Host-`, `HttpOnly`, `Secure`, `SameSite=Strict`) verified by Go test.
- **SC-05**: `ls desktop/` fails; `ci-flutter.yml` absent; `Makefile` / `.github/workflows/` contain zero `flutter` / `desktop` references; `AGENTS.md` updated.
- **SC-06**: `make lint-specs` passes with `POL-*` IDs resolving; `check_requirement_ids.py` covers `web/src/*.ts` / `*.tsx`; coverage ≥ meta (edge cases first + happy path for parity, CSP, cookie, rate-limit, removal, traceability).
- **SC-07**: CSP report endpoint responds 204; body cap enforced; query stripped; Prometheus metric emitted; Loki log structured; rate-limit 429 returned when exceeded.

---

## Assumptions

- The desktop reference visuals exist in CI or an artifact store for Playwright comparison; if not available, parity is demonstrated by manual sign-off + screenshot archive.
- The backend `POST /csp-report` route and the `__Host-legnaris_refresh` cookie mechanism from `backend-browser-auth-cookies` (feature 006) are already in production / merged; F10 only closes the remaining enforcement, removal, and reporting work.
- `check_requirement_ids.py` extension to TypeScript is delivered under F1 (feature 001); F10 verifies it stays working for `POL-*`.
- The parity gate is a hard stop: desktop removal (POL-07) is only permitted when POL-01 passes, per the constitution's Principle VII (installable/offline-capable client with parity) and the plan's definition of done.
- All new requirement IDs use `POL-` only; no reuse of `SEC-`, `FND-`, `AD-`, `LGN-` prefixes (those are reserved per `.specs/STATE.md` and existing specs).

---

## Clarifications

> Clarify step completed (2026-10-17). No critical ambiguity remains; no clarification questions needed per spellit.clarify taxonomy.

- **Functional Scope**: Clear — closure of F9 (security enforce) + parity gate + desktop removal; out-of-scope declared (no new PWA features, no auth redesign, no UI redesign).
- **Domain / Data**: Clear — entities (Reference Render, CSP Report, Rate Limit Counter, Decommission Set) defined; no new persistence.
- **Interaction / UX Flow**: Clear — 4 independent stories (parity, security, removal, audit); error/loading states covered by edge cases.
- **Non-Functional**: Clear — security gates blocking (Principle VI); performance/scale not primary (closure feature).
- **Integration / Dependencies**: Clear — depends on F6 (`backend-browser-auth-cookies`); F1 (`check_requirement_ids.py` TS extension); desktop reference artifact.
- **Edge / Failure**: Clear — parity failure blocks removal; CSP violation requires collect-then-enforce; rate-limit 429 documented.
- **Constraints / Tradeoffs**: Clear — desktop deprecated by constitution v1.1.1; no Flutter feature work; `POL-*` prefix isolated.
- **Terminology**: Consistent — `POL-*` for this feature; `SC-*` for measurable outcomes; `AD-*` referenced only for existing decisions.
- **Completion Signals**: Clear — all `POL-*` mapped to `SC-*`; checklist complete; no `NEEDS CLARIFICATION` markers remain.

**Result**: Move to plan (`/speckit.plan`) without additional questions.
