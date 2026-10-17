# Tasks: 014-web-polish-decommission (F10 — POL-*)

**Branch**: `014-web-polish-decommission` | **Status**: Implement (user authorized) | **Plan**: `.specs/features/014-web-polish-decommission/plan.md`

**Dependencies & Order**:
- Phase 1 (Foundational) → Phase 2 (US1 parity) → Phase 3 (US2 security gates / cookie / CSP) → Phase 4 (US3 removal / docs) → Phase 5 (US4 traceability / final gate)
- US2 (security) depends on existing F6 (cookie auth) and F9 (security hardening) — assume delivered.
- US3 (removal) HARD DEPENDENCY on US1 (parity passes). No desktop removal until POL-01 green.

---

## Phase 1: Foundational / Setup (blocking all stories)

- [ ] T001 [P] [FND] Verify `desktop/` reference artifacts exist (Playwright reference) or document absence; confirm `make lint-web` / `test-web` / `sec-scan` work.
  - *File*: `Makefile`, `.specs/features/014-web-polish-decommission/quickstart.md`
  - *Test*: `make sec-scan` passes (zero new findings); `npm ci` clean.

---

## Phase 2: User Story 1 — Visual Parity (P1) — POL-01

*Goal*: Playwright golden comparison passes before any removal.

### Tests (must fail before implementation)

- [x] T002 [US1] Playwright contract: screenshot comparison of shell/template/sheet/catalog against desktop reference; fail if artifacts missing.
  - *File*: `web/src/test/e2e/parity.spec.ts` (or add to existing Playwright suite)
  - *Evidence*: `POL-01` / contract `parity-gate.contract.md`

### Implementation

- [x] T003 [US1] Configure Playwright visual comparison thresholds; add reference artifact download / artifact reference config.
  - *File*: `playwright.config.ts` / `.github/workflows/` (if artifact upload needed)
- [ ] T004 [US1] Add `web/src/test/e2e/parity.spec.ts` with golden comparison for template/sheet/catalog/shell (using existing `template-01` reference or manual sign-off if no artifact).
  - *File*: `web/src/test/e2e/parity.spec.ts`

**Checkpoint US1**: `npm run test:web -- --grep="parity"` passes (or documented sign-off if no artifact). Gate: POL-01 must be green before T010+ removal tasks.

---

## Phase 3: User Story 2 — Security Gates / Auth / CSP (P1) — POL-02 / POL-03 / POL-04 / POL-05 / POL-06

*Goal*: CSP enforce, cookie-only auth, rate limit, report endpoint, zero new security findings.

### Tests (must fail before implementation)

- [ ] T005 [US2] CSP header test: `curl -I` asserts `Content-Security-Policy` enforce mode (not only Report-Only) with full policy; asserts `frame-ancestors 'none'` and Trusted Types.
  - *File*: `tests/security/csp_enforce_test.py` or `scripts/check_csp.sh` (add to CI gate `ci-web.yml`)
  - *Evidence*: `POL-02`, contract `csp-enforce.contract.md`

- [ ] T006 [US2] Cookie auth contract test: login → cookie `__Host-legnaris_refresh` (attributes verified); `/auth/refresh` reads cookie; `X-Refresh-Token` header rejected; `/auth/logout` revokes family.
  - *File*: `tests/auth/cookie_contract_test.py` (Go test under `internal/app/auth_integration_test.go` or separate)
  - *Evidence*: `POL-03`, contract `cookie-auth.contract.md`

- [ ] T007 [US2] Rate limit test: exceed `/auth/login` and `/csp-report` → 429 + `Retry-After`; below limit → 200/204.
  - *File*: `tests/security/rate_limit_test.py` (or script in `scripts/`)
  - *Evidence*: `POL-04`

- [ ] T008 [US2] CSP report endpoint test: POST to `/csp-report` with both payload formats; sanitize query stripped; 204 returned; body cap enforced; rate limit applies.
  - *File*: `tests/security/csp_report_test.py`
  - *Evidence*: `POL-05`

- [ ] T009 [US2] Security gate regression test: `make sec-scan` zero new findings (osv-scanner, Trivy, CodeQL, Semgrep, gitleaks, ZAP, SBOM) — assert no new alerts.
  - *File*: existing `ci-web.yml` / `scripts/check_security_gates.sh`
  - *Evidence*: `POL-06`

### Implementation

- [ ] T010 [P] [US2] Update `internal/app/auth.go`: remove `X-Refresh-Token` read; enforce cookie-only refresh (`__Host-legnaris_refresh`).
  - *File*: `internal/app/auth.go`
  - *Requires*: F6 delivered (cookie mechanism exists).

- [x] T011 [P] [US2] Update `internal/app/router.go` / middleware: inject CSP `enforce` header (not only report-only); include `report-uri` / `report-to`; set `connect-src https://api.legnaris.com.br`; `frame-ancestors 'none'`; `require-trusted-types-for 'script'`.
  - *File*: `internal/app/router.go` (or CSP middleware file)
  - *Requires*: CSP collect phase from F9 completed (if any violations exist, fix before enforce).

- [ ] T012 [P] [US2] Implement `POST /csp-report` endpoint (no auth): sanitize URLs (strip query/hash), normalize payload (`application/csp-report` + `application/reports+json`), cap body, structured Loki log, Prometheus metric (`csp.violation` with labels `effective_directive`, `disposition`), respond 204, share rate limiter with `/auth/*`.
  - *File*: `internal/app/csp_report_handler.go` (or add to `router.go` / `auth.go` based on structure); `shared/logger/` for structured log; Prometheus metrics in `deploy/` config.
  - *Evidence*: `POL-05`, contracts.

- [ ] T013 [P] [US2] Add / verify rate limiting middleware (`/auth/*` + `/csp-report`): key = IP (+ identity when auth present); window + limit configured in `Makefile` / env; 429 response with `Retry-After` header; metric emitted.
  - *File*: `internal/app/ratelimit.go` (or existing middleware update)
  - *Requires*: F6 rate limit already in scope (per plan v3.1).

- [ ] T014 [US2] Run full security gate (`make sec-scan`): verify zero new findings; document result in `plan.md` / feature notes.
  - *Evidence*: `POL-06`

**Checkpoint US2**: All tests T005–T014 pass; CSP enforce verified; cookie-only auth verified; rate limit 429 verified; CSP report endpoint 204 verified; `sec-scan` zero new findings.

---

## Phase 4: User Story 3 — Desktop Decommission (P2) — POL-07 / POL-08 — ONLY AFTER US1 (POL-01) PASSES

*Hard dependency: T002 / T004 (US1 parity) must be green before any T015+ removal.*

### Tests (must fail before removal — verify absence after removal)

- [ ] T015 [US3] Desktop removal verification: `ls desktop/` fails; `grep -r "desktop/" Makefile .github/workflows/ .specify/` returns nothing; `grep -r "flutter" .github/workflows/` returns nothing; `ci-flutter.yml` absent.
- [ ] T016 [US3] Constitution update verification: `.specify/memory/constitution.md` has no active Flutter reference; `AGENTS.md` references web-only client; `POL-*` cited in code comments or tasks.
- [ ] T017 [US3] Requirement-ID traceability verification: `make lint-specs` passes with `POL-*` IDs resolving; `check_requirement_ids.py` covers `*.ts` / `*.tsx`.

### Implementation

- [ ] T018 [US3] **Wait for POL-01 pass** (manual gate check or automated via CI dependency).
- [ ] T019 [US3] Remove directory `desktop/` (all files); delete `ci-flutter.yml`.
  - *File*: `desktop/*`, `ci-flutter.yml`
  - *Requires*: T018 gate.

- [ ] T020 [US3] Update `Makefile`: remove `deploy-flutter` / `lint-flutter` / desktop targets; keep `lint-web`, `test-web`, `sec-scan`.
  - *File*: `Makefile`

- [ ] T021 [US3] Update `.gitignore`: remove desktop-specific entries; add web-specific if missing.
  - *File*: `.gitignore`

- [ ] T022 [P] [US3] Update root `AGENTS.md`: reference web as only active client; reference `.specify/memory/constitution.md` v1.1.1; add `POL-*` / `AD-*` traceability note.
  - *File*: `AGENTS.md`

- [ ] T023 [US3] Update `.specify/memory/constitution.md` if any residual desktop references exist (verify v1.1.1 already reflects deprecation; document any needed amendment — but constitution should NOT change unless principle amended, which it isn't).
  - *File*: `.specify/memory/constitution.md` (verification only)

**Checkpoint US3**: `desktop/` removed; CI clean; docs updated; `POL-07`/`POL-08` verified.

---

## Phase 5: User Story 4 — Coverage / Docs / Final Gate (P2) — POL-09 / POL-10

### Tests

- [ ] T024 [US4] Requirement-ID resolution test: `make lint-specs` passes with `POL-*` IDs resolved; `scripts/check_requirement_ids.py` scans `*.ts` / `*.tsx`; zero unresolved.
- [ ] T025 [US4] Coverage / assertion quality test: `make test-race` clean; `make lint-specs` passes (except known pre-existing `vacuous assertion` in `catalog_screen.test.tsx`, which is pre-existing); all F10 tests cover edge cases first + happy path.
- [ ] T026 [US4] Security gate final verification (repeat): `make sec-scan` green; CSP enforce header present; cookie attributes verified; desktop removed; `POL-*` IDs traceable.

### Implementation

- [ ] T027 [P] [US4] Update `check_requirement_ids.py` to cover `*.ts` / `*.tsx` (if not done by F1 — verify it works for `POL-*`).
  - *File*: `scripts/check_requirement_ids.py` (verify extension exists and runs)

- [ ] T028 [US4] Commit `tasks.md` ticked; write feature `README` / update `docs/` if needed; update `.specs/features/014-web-polish-decommission/checklists/requirements.md` to completed (all `[x]`); verify checklist passes.
  - *File*: `tasks.md` (tick); checklist file.

- [ ] T029 [US4] Final commit message: conventional commit referencing `POL-*` requirements and gate results; include `tests:` in message if tests added; no token committed.
  - *Evidence*: `POL-10`; `make fmt && make lint && make vet && make lint-specs && make test-race`

**Checkpoint US4 / Final Gate**: All `POL-*` requirements mapped; all gates green; desktop removed; `sec-scan` zero new findings; `test-race` clean; `lint-specs` clean (except pre-existing). Feature complete — ready to merge.

---

## Dependencies & Execution Order

- T001 (Foundational) → T002–T004 (US1 parity) → T005–T013 (US2 security; depends on F6 cookie auth) → T015–T017 (US3 removal; HARD on T004 parity pass) → T024–T029 (US4 docs / gate)
- T003 / T006 / T010 can run in parallel with T002 once prerequisites met.
- T014 (security gate) depends on T010–T013.
- T015–T023 (removal) BLOCKED until T004 (parity) passes — enforce manually or via CI gate.

## Implementation Strategy

1. Setup (T001) → verify environment and artifacts.
2. US1 parity (T002–T004) — build and run Playwright comparison first.
3. US2 security (T005–T014) — enforce CSP, cookie-only auth, rate limit, CSP report, verify gates.
4. Once T004 green: US3 removal (T015–T023) — delete desktop, clean CI/docs.
5. US4 final (T024–T029) — traceability, checklist, final commit, gate verification.

---

## Notes

- Every behavior change ships with tests (Principle III); edge cases first (tests T005, T006, T007 cover error paths: cookie missing, CSP violation report, rate limit exceeded).
- Security gates blocking: no `continue-on-error`; new finding = pipeline failure (Principle VI).
- No desktop removal until parity gate passes (plan v3.1 §F10; contract `parity-gate.contract.md`).
- `POL-*` IDs isolated and traceable; `AD-*` referenced for design decisions; `SEC-*` / `FND-*` not reused.
