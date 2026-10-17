# Feature Specification: web-pwa-offline (PWA Offline Shell)

**Feature Branch**: `012-web-pwa-offline`

**Created**: 2026-10-17

**Status**: Draft

**Feature Directory**: `.specs/features/012-web-pwa-offline/`

**Requirement Prefix**: `PWA-`

**Input**: PWA offline: service worker (`injectManifest`), deep-link fallback (`index.html`), offline reading of cached shell and drafts (`IndexedDB`), installable manifest (`manifest.webmanifest`), Lighthouse installability gate; no third-party CDN scripts; refresh cookie never cached by SW.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Install and load the PWA offline (Priority: P1)

The user opens the web app in a browser over HTTPS, installs it (Add to Home Screen / install prompt), then loses network connectivity. The installed app shell still loads from the service-worker precache; reading cached templates/sheets works without a network call; deep links (`/sheet/:id`) resolve through the SW fallback to `index.html`.

**Why this priority**: Installability and offline reading are the defining contracts of a PWA (Constitution VII) and block the desktop decommission (F10-T004 parity gate).

**Independent Test**: With the deployed preview build offline, navigate to `/sheet/test-id`; expect the shell to render (no blank screen) and a cached sheet to load; Lighthouse PWA audit must pass installability.

**Acceptance Scenarios**:

1. **Given** the app is served over HTTPS with a valid `manifest.webmanifest` and registered SW (`registerSW` external, `injectRegister: false`), **When** the user installs and reloads, **Then** the browser reports installable and the SW activates.
2. **Given** the user navigates to a previously cached route (`/sheet/:id`) with no network, **When** the page loads, **Then** the shell renders from precache and the sheet data comes from `IndexedDB` (not from a failed `fetch`).
3. **Given** a deep-link `/sheet/nonexistent` with no network, **When** the route resolves via `index.html` fallback, **Then** the app shows the `NOT_FOUND` message (mapped `PWA-03` error) without a network error.

---

### User Story 2 — Drafts survive reload and are restored offline (Priority: P2)

The user edits a sheet, the draft is saved to `IndexedDB` on a timer (injected cadence), the browser reloads (or the device restarts), and the user opens the app offline; the draft is restored (only editable leaves, no derived/read-only fields overwritten) and the dirty state is preserved.

**Why this priority**: Offline-capable editing requires that drafts survive restarts (AD-003/AD-008 port) and that derived fields are never restored incorrectly.

**Independent Test**: Create a draft (modify an editable leaf), reload the page while online, confirm the draft restores; disconnect and reload, confirm the same.

**Acceptance Scenarios**:

1. **Given** an editable leaf was changed and the draft backup wrote a versioned entry (`library/draft_backup`), **When** the app reloads (online or offline), **Then** the draft restores only for editable fields (`PWA-04`) and derived/read-only leaves remain computed.
2. **Given** the `IndexedDB` schema version is out of date (`schema_version` mismatch), **When** the app opens, **Then** the library quarantines the old file (preserves bytes) and starts fresh (`PWA-05`).
3. **Given** the user deletes the cached entry for a sheet (`DELETE /sheets/:id`) and the server responds `NOT_FOUND`, **When** the library tries to read the deleted entry offline, **Then** it is evicted from local cache (`PWA-02`).

---

### User Story 3 — Security gates enforce CSP, Trusted Types, and no inline scripts (Priority: P1 — security)

Every PR that touches `web/` triggers blocking gates: CSP delivered as a real HTTP header (`Content-Security-Policy` enforce mode, `connect-src https://api.legnaris.com.br`), `script-src 'self'` (no CDN, no inline), Trusted Types (`require-trusted-types-for 'script'`), `report-uri/report-to` pointing to the backend `POST /csp-report`, `frame-ancestors 'none'`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `SameSite=Strict` cookies (`__Host-`), double-submit CSRF (`X-CSRF-Token`), and `Origin` validation.

**Why this priority**: Constitution VI requires blocking security gates and CSP enforce; `injectRegister: false` enables `script-src 'self'` without `unsafe-inline`.

**Independent Test**: A Playwright test asserts the `Content-Security-Policy` response header contains `default-src 'self'; script-src 'self'; frame-ancestors 'none'; report-uri ...`; a sink test (`dangerouslySetInnerHTML`) fails at build time (ESLint `react/no-danger`).

**Acceptance Scenarios**:

1. **Given** the app is loaded, **When** a CSP violation occurs (e.g., an inline script attempt), **Then** the browser sends the report to `POST /csp-report` and the header is present in enforce mode (`PWA-07`).
2. **Given** the `refresh` cookie is set with `HttpOnly; Secure; SameSite=Strict; Path=/`, **When** a cross-site request attempts `/auth/refresh`, **Then** the cookie is not sent (blocked by `SameSite=Strict`) and the request fails (`PWA-06`).
3. **Given** a state-changing call (`POST /auth/refresh`) includes the cookie but no valid `X-CSRF-Token`, **When** the server validates, **Then** the response is `403` (`PWA-08`).

---

### Edge Cases

- What happens when `sw.ts` tries to cache an authenticated `GET /sheets` response? The SW MUST NOT precache or runtime-cache authenticated endpoints; only the shell (`/`, `/manifest.webmanifest`, assets) and safe GET fixtures are cached (`PWA-01`).
- What happens when the service-worker `update` is available but the user has unsaved drafts? The `registerSW` must prompt for update but never overwrite drafts; the update applies on next navigation after the user confirms (`PWA-03`).
- What happens when `IndexedDB` writes fail (quota exceeded, private browsing)? The library falls back to in-memory only, shows an offline-read-only banner, and does not crash (`PWA-05`).
- What happens when a `blocked-url` in a CSP report contains PII (query string with `sheet_id`)? The backend MUST strip query/hash before logging to Loki (`PWA-07`).
- What happens when `registerSW` is called with `registerType: 'prompt'` and the user denies install? The app continues fully functional; installability is a progressive enhancement (`PWA-01`).

---

## Requirements *(mandatory)*

### Requirement-ID Prefix

Every functional requirement in this feature uses the prefix `PWA-`. Examples: `PWA-01`, `PWA-02`, ….

### Functional Requirements

- **PWA-01** (F8-T001, F8-T002, F8-T003, F8-T005, F8-T006): The service worker (`sw.ts`, `injectManifest` strategy) MUST precache the app shell (`index.html`, `manifest.webmanifest`, icons, `assets/*`) and use a runtime cache for safe GET requests; it MUST NOT cache `/auth/*` or any authenticated response; `registerType` MUST be `prompt`; the SW MUST be registered externally (`registerSW` file, not inline) so `script-src 'self'` requires no `unsafe-inline`.
- **PWA-02** (F8-T001, F8-T004): The `IndexedDB` library (`library/sheet_library.ts` + `library/draft_backup.ts`) MUST store drafts with `schema_version`; on `NOT_FOUND` from `GET /sheets/:id`, the library MUST evict the local entry; drafts MUST restore only editable leaves (`PWA-04`).
- **PWA-03** (F8-T003, F8-T004): Deep-link routes (`/sheet/:id`, `/catalog`, `/`) MUST resolve through the host fallback (`index.html`) when offline; the app MUST show mapped error messages (not raw network errors) on `NOT_FOUND` or `401`.
- **PWA-04** (F8-T002, F8-T004): Draft restore MUST filter to editable fields; derived (`calculated`) and read-only leaves MUST never be written back by the restore; the dirty flag MUST survive reload.
- **PWA-05** (F8-T002, F8-T004): The library MUST handle `IndexedDB` failures (quota exceeded, version mismatch, private browsing) gracefully: quarantine corrupt files (preserve bytes), start fresh, and show a non-blocking banner; no crash.
- **PWA-06** (F8-T001, F9-T001, F9-T006): Every cookie (`__Host-legnaris_refresh`, `__Host-legnaris_csrf`) MUST have `HttpOnly; Secure; SameSite=Strict; Path=/`; the refresh cookie MUST NOT be readable by JavaScript; `SameSite=Strict` MUST block cross-site attachment.
- **PWA-07** (F9-T001, F9-T008, F9-T011): CSP MUST be delivered as an HTTP header (not meta tag) in enforce mode (`Content-Security-Policy`, not `Report-Only`) with `default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self' https://api.legnaris.com.br; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; require-trusted-types-for 'script'; report-uri https://api.legnaris.com.br/csp-report`; `Trusted Types` MUST enforce; `dangerouslySetInnerHTML`, `innerHTML`, `eval`, `new Function` MUST be forbidden (ESLint + build-time); CSP reports (`application/csp-report`, `application/reports+json`) MUST be accepted by `POST /csp-report` (cap ~16KB, rate-limited, strip query/hash, structured log → Loki, metric → Prometheus); roll out goes through `Report-Only` collection before `enforce` (`F9-T008`).
- **PWA-08** (F6B-T002, F7-T003): Every state-changing cookie-authenticated request (`POST /auth/refresh`, `POST /auth/logout`) MUST include a valid `X-CSRF-Token` (server-bound double-submit) and a matching `Origin` header; missing/invalid CSRF MUST return `403`; missing/invalid `Origin` MUST return `403`; concurrent `401` responses MUST trigger a single-flight silent refresh (`authenticated_client`).
- **PWA-09** (F9-T005): A `SBOM` (`Syft`) MUST be produced for `web/` and verified in `ci-web.yml`; `dependency-review-action` MUST block unreviewed dependency changes; `npm audit signatures` MUST verify lockfile integrity (`F9-T004`).
- **PWA-10** (F8-T005, F10-T001): Lighthouse PWA/performance/a11y gates MUST pass (`installable`, `service-worker`, `offline-capable`, `deep-link`, no critical a11y violations) before the feature is considered complete; Playwright screenshots of the reference template MUST match the desktop golden images within a defined pixel tolerance (`F10-T001`).

### Key Entities (data involved, no implementation details)

- **ServiceWorker**: the installable PWA runtime; owns precache of the shell and runtime cache rules for safe GETs.
- **IndexedDB (library)**: versioned local store for drafts and cached sheets; never holds tokens.
- **DraftEntry**: a saved edit state with `template_id`, `sheet_id`, `editable_only` filter, `timestamp`, `version`.
- **CSPReport**: a violation event (`document-uri`, `blocked-uri`, `effective-directive`, `disposition`); sanitized before logging.
- **CookiePair** (`__Host-` prefix): refresh (httpOnly) + CSRF (readable by JS for double-submit) used by auth.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-PWA-01**: Lighthouse PWA audit passes (`installable`, `service-worker`, `offline-capable`) on the preview deploy (`PWA-01`, `PWA-10`).
- **SC-PWA-02**: With network disabled (`offline` in DevTools), the shell loads and a previously opened sheet reads from `IndexedDB` (`PWA-01`, `PWA-03`).
- **SC-PWA-03**: Draft restore survives reload (online and offline) without corrupting derived/read-only fields (`PWA-04`).
- **SC-PWA-04**: The CSP header is present in enforce mode on every response (`PWA-07`); a Playwright test asserts its presence; `CSP Evaluator` shows no critical violations.
- **SC-PWA-05**: A `POST /csp-report` test confirms the endpoint accepts both payload formats, responds 204, logs structured data to Loki, and increments the Prometheus metric (`PWA-07`).
- **SC-PWA-06**: `osv-scanner` + `Trivy fs` + `CodeQL` + `Semgrep` + `gitleaks` + `OWASP ZAP baseline` are blocking in `ci-web.yml`; zero new secrets or critical SAST/DAST findings (`PWA-09`).
- **SC-PWA-07**: Requirement-ID citations (`PWA-01` … `PWA-10`) appear in the TypeScript source and pass `check_requirement_ids.py` (extended to `.ts/.tsx`) (`PWA-09`).

---

## Assumptions

- The backend feature `006-backend-browser-auth-cookies` is delivered and deployed first (blocking prerequisite); otherwise `PWA-06`/`PWA-08` cannot be validated (`PWA-01`).
- The web client stack (`React` + `Vite` + `TS` + `vite-plugin-pwa`) is already set up (`001-web-pwa-foundation`); this feature builds on it (`PWA-01`).
- `IndexedDB` is available in the target browsers (modern evergreen); no polyfill is required (`PWA-02`).
- `registerSW` uses the external-registration pattern (`registerSW` file, not inline script) to keep CSP strict (`PWA-07`).
- The `CSP report` endpoint (`POST /csp-report`) is implemented in the backend (Go/Echo) and exposed through the API subdomain; it does not require authentication (`PWA-07`).
- Rate limiting (`/auth/*` + `/csp-report`) is implemented with a shared limiter; `429` responses include `Retry-After` (`PWA-08`).
- `desktop/` removal (`F10-T005`) is blocked by the parity gate (`F10-T004`); this feature does not delete the desktop client (`PWA-10`).
- `check_requirement_ids.py` is extended to cover `*.ts/*.tsx` in `.specs/features/001-web-pwa-foundation/` (`F1-T010`) before this feature's citations are validated (`PWA-09`).
- The `injectManifest` SW (`sw.ts`) is built by `vite-plugin-pwa` and emitted to `dist/service-worker.js`; it is not edited directly (`PWA-01`).

---

## Dependencies

- **Blocker**: `006-backend-browser-auth-cookies` (cookie httpOnly, CSRF, CORS, rate limit, `POST /csp-report`) (`PWA-06`, `PWA-07`, `PWA-08`).
- **Prerequisite**: `001-web-pwa-foundation` (`web/` scaffold, tokens, `AGENTS.md`, `vite-plugin-pwa` config) (`PWA-01`).
- **Prerequisite**: `007-web-auth-session` (auth session, `authenticated_client`, `session_store`) (`PWA-06`, `PWA-08`).
- **Prerequisite**: `005-web-sheet-cycle` (sheet library, draft backup, `IndexedDB` base) (`PWA-02`, `PWA-04`).
- **Prerequisite**: `004-web-ui-engine` (editable/read-only filters, `field_control`, `exclusive_control`) (`PWA-04`).
- **Prerequisite**: `002-web-api-client` (`fetch` wrapper, error mapper) (`PWA-03`).
- **Post-dependency (not blocking)**: `010-web-polish-decommission` (parity gate, desktop removal) (`PWA-10`).
