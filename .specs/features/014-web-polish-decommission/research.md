# Research: 014-web-polish-decommission (F10)

**Date**: 2026-10-17 | **Plan reference**: `.specs/features/014-web-polish-decommission/plan.md`

## Decisions / Rationale / Rejected Alternatives

### 1. CSP rollout (POL-02)

- **Decision**: Keep `Content-Security-Policy-Report-Only` briefly (collect 1 cycle), then flip header to `enforce` with same policy string.
- **Rationale**: Principle VI requires enforce; `Report-Only` is only for rollout validation (F9-T008). The backend (`internal/app/auth.go` / router middleware) controls headers, so flip is a single middleware change.
- **Rejected**: Stay in report-only (violates POL-02 / gate); jump directly to enforce (skips validation, increases risk of unexpected breakage from browser extensions / third-party assets).

### 2. Cookie refresh path removal (POL-03)

- **Decision**: Remove `X-Refresh-Token` from `internal/app/auth.go`; enforce cookie read (`__Host-legnaris_refresh`) only; delete the test that prohibited `Set-Cookie` (feature 006 already delivered cookie auth).
- **Rationale**: Desktop decommission requires native path removal; cookie-only aligns with browser-native model and CSP enforcement.
- **Rejected**: Keep both paths (maintains desktop dependency, prevents removal); keep cookie but also read header (unnecessary complexity, violates decommission).

### 3. Rate limit scope (POL-04)

- **Decision**: Use shared limiter for `/auth/*` and `POST /csp-report`.
- **Rationale**: Both are unauthenticated / credential-sensitive; a shared limiter reduces state; key = IP (+ identity when present).
- **Rejected**: Separate limiters (extra complexity with no security gain for this feature).

### 4. Desktop removal order (POL-01 / POL-07)

- **Decision**: Playwright golden comparison passes (POL-01) BEFORE deleting `desktop/` and `ci-flutter.yml`.
- **Rationale**: Parity gate is defined in plan v3.1 (§11); removal without proof risks broken UI with no reference.
- **Rejected**: Remove desktop first (violates definition of done; no rollback reference).

### 5. CSP report endpoint design (POL-05)

- **Decision**: Backend route (`POST /csp-report`, Echo handler) — sanitize query/hash, normalize payloads, structured Loki log, Prometheus metric, 204 response, shared rate limiter.
- **Rationale**: Backend already manages auth/session/state; collecting reports in the client would require new storage and break privacy (no cookies in reports). Loki/Prometheus/Grafana stack exists in `deploy/`.
- **Rejected**: Client-side collection (violates privacy, requires persistence); external vendor (cost/dependency without gain for this feature).

### 6. Reference artifact storage

- **Decision**: Playwright reference screenshots stored as CI artifacts (not committed); comparison uses `toMatchSnapshot` / `perceptual` thresholds configured in Playwright config.
- **Rejected**: Commit reference images to repo (bloat); rely solely on manual sign-off (not reproducible, violates testability).

---

## References to external docs / code

- Plan: `docs/plan-web-pwa.md` v3.1 (§F10, §CSP report appendix, §Rate limiting).
- Constitution: `.specify/memory/constitution.md` v1.1.1 (Principles VI, VII; deprecated desktop clause).
- Existing features: 001 (`check_requirement_ids.py`), 006 (`cookie auth`), 009 (`security hardening`), 013 (`security hardening` — CSP enforce).
- Existing code: `internal/app/auth.go`, `internal/app/router.go`, `shared/env/`, `.specs/STATE.md` (`AD-001`…`AD-010`).
