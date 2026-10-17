# Implementation Plan: web-pwa-offline (PWA Offline Shell)

**Branch**: `012-web-pwa-offline` | **Date**: 2026-10-17 | **Spec**: `.specs/features/012-web-pwa-offline/spec.md`

**Status**: Draft — ready for tasks/implementation (artifacts + code for F8 only; prerequisites 006/001/007/005/004/002 must be in place).

## Summary
Port `desktop/` offline behavior into `web/`: `injectManifest` SW, `IndexedDB` library, CSP enforce, cookie/CSRF, Lighthouse gate.

## Technical Context
**Language/Version**: TypeScript 5.x / React 18 / Vite 6 / `vite-plugin-pwa`; Node pinned.
**Dependencies**: `vite-plugin-pwa`, `idb`, `zustand`, Playwright, `axe-core`; Go Echo backend for `/csp-report`, cookie/auth, CORS.
**Storage**: IndexedDB (versioned, no tokens); MongoDB backend.
**Testing**: Vitest + RTL + MSW; Playwright (E2E, a11y, visual, Lighthouse); Go `test -race`; `axe-core`; NO `continue-on-error`.
**Target**: HTTPS subdomain `app.legnaris.com.br` (API `api.legnaris.com.br`); modern browsers.
**Constraints**: No `dangerouslySetInnerHTML`/`eval`; `script-src 'self'`; cookie `HttpOnly`; SW no auth cache; Background Sync out of scope.

## Constitution Check (pre + post design)
- VI Security Gates blocking (`ci-web.yml`): `osv-scanner`, Trivy, CodeQL, Semgrep, gitleaks, secret-scanning, dependency-review, lockfile-lint, ZAP, SBOM (`syft`), CSP/header test; zero `continue-on-error`.
- VII PWA offline installable: `injectManifest`, manifest, `registerSW` external, `IndexedDB`, deep-link fallback, Lighthouse gate, `axe` critical=0.
- III Tests ship with behavior: edge cases first (auth cached denied, IndexedDB quota, CSP PII stripped, concurrent 401 single-flight, CSRF missing 403), happy path, non-vacuous assertions, `check_requirement_ids.py` extended to `.ts/.tsx`.
- I Typed errors: backend `shared/apperr`, web `error_mapper.ts` replicates desktop.
- VI cookies: `__Host-` prefix (`Secure`, `Path=/`, no `Domain`); refresh cookie only; `SameSite=Strict`; access in memory; `X-CSRF-Token` server-bound; `Origin` validation.

## Project Structure (feature scope)
```
web/src/pwa/{sw.ts,registerSW.ts}
web/src/library/{sheet_library.ts,draft_backup.ts}
web/src/auth/session_store.ts  (depends F7)
web/src/api/authenticated_client.ts (F8/F7)
public/manifest.webmanifest
vite.config.ts (VitePWA injectManifest, registerType: prompt)
.github/workflows/ci-web.yml (security + Lighthouse gate)
internal/app/auth.go  (F6B if pending: cookie/CSRF/rate/CSP)
internal/app/router.go (CSP endpoint + CORS)
```

## Design / Contracts
- SW (`contracts/sw.contract.md`): `injectManifest`, precache shell + safe GETs, NO auth endpoints, `registerType: prompt`, `clients.claim()`.
- Library (`contracts/library.contract.md`): `schema_version`, edit-only restore, quarantine corrupt `.old`, graceful quota failure.
- CSP (`contracts/security.contract.md`): enforce header, `report-uri` + `report-to`, `POST /csp-report` (cap, sanitize URI, structured log → Loki, metric → Prometheus → Grafana), Trusted Types.
- Auth cookie (`contracts/auth-cookie.contract.md`): cookie attributes, `X-CSRF-Token`, `Origin`, CORS `Allow-Credentials: true` + `Vary: Origin`, rate limit shared.

## Research (verified)
`vite-plugin-pwa` installed; `node` pinned; `.specs/001-web-pwa-foundation/` exists; `006-backend-browser-auth-cookies/` exists (BLOCKER); `internal/app/auth.go` uses `X-Refresh-Token`; `auth_integration_test.go:58` prohibits `Set-Cookie`; `web/src/` scaffolded; Lighthouse/a11y tooling already referenced in `F1-T008`; `check_requirement_ids.py` needs `.ts/.tsx` extension (`F1-T010`) before `PWA-*` citations validate.

## Quickstart
```bash
# Prerequisites: 006 (cookie/auth/CSP/rate) → 001 (scaffold) → 007 → 005 → 004 → 002 delivered
# Spec at .specs/features/012-web-pwa-offline/; plan/tasks/analyze complete
# Implement tasks.md in order; tests first; one logical commit per task
make fmt && make lint && make vet && make lint-specs
make lint-web && make test-web && make sec-scan
# CI gate ci-web.yml must stay green (no continue-on-error)
# Lighthouse PWA gate (F8-T005) passes before F10 parity / desktop removal
```
