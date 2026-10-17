# Tasks: 012-web-pwa-offline (PWA Offline Shell)

Branch: `012-web-pwa-offline` · Spec: `.specs/features/012-web-pwa-offline/spec.md` · Prefix: `PWA-`

Tests-first order: each task group ships tests that fail before the code they cover; edge cases (quota, auth-cached-denied, PII stripped, concurrent 401, CSRF 403) come before happy path.

---

## Dependency Order (stories must complete in priority)

1. US1 (P1): Install/offline shell — `PWA-01`, `PWA-03`, `PWA-06`, `PWA-07`, `PWA-09`
2. US2 (P2): Draft restore — `PWA-02`, `PWA-04`, `PWA-05`
3. US3 (P1): Security gates — `PWA-07`, `PWA-08`, `PWA-09`, `PWA-10`

Blocker: `006-backend-browser-auth-cookies`; prerequisites: `001`, `007`, `005`, `004`, `002`.

---

## Phase 1 — Foundational (blocking all stories)

- [ ] T001 [US1] `web/src/pwa/sw.ts`: `injectManifest` SW with precache list, runtime `GET` non-auth cache, `skipWaiting` after `registerType: 'prompt'` confirmation; test: SW activates; no auth endpoint cached. (`PWA-01`)
- [ ] T002 [P] `web/src/pwa/registerSW.ts`: external registration; test: no inline `<script>`; `script-src 'self'` passes; `registerType` = `prompt`. (`PWA-01`)
- [ ] T003 [US1] `vite.config.ts`: `VitePWA({ strategies: 'injectManifest', injectRegister: false, registerType: 'prompt' })`; build passes; `dist/service-worker.js` emitted. (`PWA-01`)
- [ ] T004 [P] `public/manifest.webmanifest` + icons: valid manifest; Lighthouse installable passes. (`PWA-01`)
- [ ] T005 [US3] `.github/workflows/ci-web.yml`: security jobs (SCA `osv-scanner`/Trivy/`npm audit`, SAST CodeQL+Semgrep, secrets gitleaks, dependency-review, lockfile-lint, ZAP baseline, CSP/header test, Lighthouse); ZERO `continue-on-error`; `failure` on new finding. (`PWA-09`, `PWA-07`)

---

## Phase 2 — US1 (P1) Install / Offline Shell

- [ ] T010 [US1] `web/src/pwa/sw.ts` — deep-link fallback (`index.html` for all routes); test offline load of `/sheet/test-id`; shell renders; `NOT_FOUND` mapped. (`PWA-03`)
- [ ] T011 [US1] `web/src/library/sheet_library.ts` — `IndexedDB` open, `schema_version`, versioned entry; test quota failure graceful; quarantined `.old`; fresh start. (`PWA-02`, `PWA-05`)
- [ ] T012 [US1] Playwright smoke: install prompt + offline reload + deep-link + shell render; Lighthouse PWA score ≥ 90. (`PWA-10`, `SC-PWA-01`)

---

## Phase 3 — US2 (P2) Draft Restore

- [ ] T020 [US2] `web/src/library/draft_backup.ts`: editable-only filter, dirty preservation, cadence injectable; test restore only editable leaves; derived fields unchanged; dirty flag survives reload. (`PWA-04`)
- [ ] T021 [US2] `web/src/library/sheet_library.ts`: `NOT_FOUND` eviction; test `DELETE /sheets/:id` removes local entry offline; no phantom read. (`PWA-02`)
- [ ] T022 [US2] Playwright: edit leaf → reload (online) → reload (offline) → draft preserved; no derived corruption. (`PWA-04`)

---

## Phase 4 — US3 (P1) Security / CSP / Auth / Rate

- [ ] T030 [US3] `web/src/auth/session_store.ts`: access token **only** memory (`sessionStorage` forbidden); reload = silent refresh via `authenticated_client`; test no token persistence in storage. (`PWA-06`, `F7-T002`)
- [ ] T031 [US3] `web/src/api/authenticated_client.ts`: `401` → single-flight silent refresh + `X-CSRF-Token`; refresh invalid → forced logout; concurrent 401 shares refresh; test loop prevented. (`PWA-08`)
- [ ] T032 [US3] CSP header + Trusted Types + `dangerouslySetInnerHTML` banned (`ESLint` + build): test header present enforce; `CSP Evaluator` clean; sink blocked. (`PWA-07`)
- [ ] T033 [US3] `POST /csp-report` backend (`internal/app/router.go`): accept both formats; cap body; sanitize `document-uri` (strip query/fragment); structured log (Loki); Prometheus metric; 204; rate limit shared with `/auth/*`; test report + metric + log. (`PWA-07`, `F9-T011`)
- [ ] T034 [US3] Cookie attributes + CSRF + `Origin`: `__Host-` cookie; `X-CSRF-Token` missing/invalid → 403; `Origin` invalid → 403; `SameSite=Strict` blocks cross-site; `CORS_ALLOW_CREDENTIALS` + `Vary: Origin`; rate limit `429` + `Retry-After`; test all paths. (`PWA-06`, `PWA-08`)
- [ ] T035 [US3] `axe-core` Playwright + Lighthouse + screenshots: zero critical a11y; installable; reference template match. (`PWA-10`, `F9-T007`)
- [ ] T036 [P] `check_requirement_ids.py` extension to `.ts/.tsx`; `PWA-01`..`PWA-10` citations resolve; `ci-spec-lint.yml` includes `web/**`. (`PWA-09`, `F1-T010`)

---

## Implementation Strategy (MVP first)

1. SW (`T001`–`T003`) + manifest (`T004`) + CI gate (`T005`) → shell works offline.
2. Library (`T011`) + draft (`T020`–`T021`) → offline editing survives.
3. Security (`T030`–`T035`) → CSP enforce, cookie/CSRF validated, a11y/Lighthouse gate.
4. `check_requirement_ids.py` extension (`T036`) validates citations.

Every group has tests first; each commit includes tests; gate `ci-web.yml` must stay green (no weakening assertions).

## Parallel Examples

- [P] `T001` + `T002` (SW + register) can run together.
- [P] `T020` + `T021` (draft restore + eviction) can run together (same `library/` module).
- [P] `T030` + `T031` (session store + client) can run together (same auth flow).
