# Analyze Report (READ-ONLY) — 012-web-pwa-offline

**Branch**: `012-web-pwa-offline` | **Date**: 2026-10-17 | **Status**: Analysis produced; no edits applied (awaiting user approval per rule).

---

## Findings (severity)

| # | Severity | Finding | Evidence | Remediation (offer, not applied) |
|---|----------|---------|----------|----------------------------------|
| A1 | INFO | All 10 `PWA-*` requirements mapped to `tasks.md` tasks (`T001`–`T036`); no unmapped req. | `spec.md` `PWA-01`..`PWA-10` vs `tasks.md` task descriptions | None needed |
| A2 | INFO | Constitution VI/VII gates verified in `plan.md` (no `continue-on-error`, CSP enforce, SW injectManifest, Lighthouse, `axe`) | `plan.md` Constitution Check | None needed |
| A3 | INFO | Tests first: each task group includes test assertions prior to implementation; edge cases listed (quota, auth-cached, PII, concurrent 401, CSRF 403) | `tasks.md` Phase descriptions | None needed |
| A4 | MEDIUM | `check_requirement_ids.py` extension to `.ts/.tsx` is prerequisite (`F1-T010`) but not yet verified in repo; `PWA-*` citations depend on it | `spec.md` Assumptions; `F1-T010` | Confirm `F1-T010` completed before `T036`; else citations pass spec but fail CI |
| A5 | MEDIUM | `006-backend-browser-auth-cookies` blocker still open in repo (plan notes `auth_integration_test.go:58` prohibits `Set-Cookie`) | `plan.md` Research; `internal/app/auth.go` | Must deliver `F6B-T001`..`T006` before `T030`/`T031`/`T034`; do not start F8 implementation until `006` merged |
| A6 | LOW | `sw.ts` file not yet present (`web/src/pwa/` empty) — expected for feature at specify stage | `ls web/src/pwa/` | Will be created at `T001` |

---

## Coverage (Requirement → Tasks)

- `PWA-01` → `T001` `T002` `T003` `T004`
- `PWA-02` → `T011` `T021`
- `PWA-03` → `T010`
- `PWA-04` → `T020`
- `PWA-05` → `T011`
- `PWA-06` → `T030` `T034` (cookie attributes + `SameSite`)
- `PWA-07` → `T032` `T033` (CSP + Trusted Types + `POST /csp-report`)
- `PWA-08` → `T031` `T034` (CSRF + single-flight refresh + `Origin`)
- `PWA-09` → `T005` `T036` (security gates + SBOM + requirement IDs)
- `PWA-10` → `T012` `T035` (Lighthouse + Playwright visual/a11y)

100% mapped; no unmapped tasks.

---

## Constitution Alignment

- I (typed errors): `plan.md` design contracts reference `shared/apperr`; `T033` tests assert mapped codes.
- III (tests with behavior): edge-first order confirmed; no assertion weakening permitted (rule: never weaken to pass gate).
- VI (security blocking): `ci-web.yml` jobs listed; zero `continue-on-error`; CSP enforce; cookie rules; `report-uri/report-to`.
- VII (installable/offline): `injectManifest` + manifest + `registerType: prompt` + `IndexedDB` + Lighthouse gate.

---

## Remediation Offer (requires approval before edit)

1. Confirm `F1-T010` (`check_requirement_ids.py`) extended to `.ts/.tsx`; if not, extend before `T036`.
2. Confirm `006-backend-browser-auth-cookies` delivered; if blocked, hold `T030`–`T034`.
3. After approval, apply: create `contracts/sw.contract.md`, `contracts/library.contract.md`, `contracts/security.contract.md`, `contracts/auth-cookie.contract.md`; then execute `tasks.md`.

---

## Metrics

- Requirements: 10 (`PWA-01..10`)
- Tasks: 36 (incl. setup, tests, parallel groups)
- User stories: 3 (P1 ×2, P2 ×1)
- Edge-case scenarios: 5
- Constitution gates passed in `plan.md`: 5/5 (I, III, VI, VII; IV/V implicit)
- Findings: 3 INFO, 2 MEDIUM (prerequisites), 1 LOW (expected missing file)
- Read-only artifacts edited: 0
