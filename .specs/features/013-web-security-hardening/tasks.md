# Tasks: Web Security Hardening (F9)

Fonte: `docs/plan-web-pwa.md` §6 (F9); `spec.md` (`SEC-01` a `SEC-11`). Nenhum código implementado ainda (autorização explícita do usuário requerida pelo Skill para Passo 7).

---

## Dependency / Prerequisites

- `F6B-T001` (cookie `__Host-`), `F6B-T002` (CSRF token), `F6B-T006` (rate limit `/auth/*`) — feature `006-backend-browser-auth-cookies`.
- `F1-T006` (`injectManifest`), `F1-T008` (`ci-web.yml`) — feature `001-web-pwa-foundation`.
- `F7-T003` (`authenticated_client.ts`) — feature `007-web-auth-session`.
- `F8-T001` (SW não cacheia autenticado) — feature `012-web-pwa-offline`.

---

## Implementation Strategy

- **First**: testes de falha (headers removidos, CSP violado, ZAP alerta novo, escaping falha) — devem falhar antes da fix.
- **Parallel**: `li` (lint/config) e `SW` (service worker) são independentes; `backend` (endpoint + rate limit) depende de `F6`.
- **Last**: `enforce` (`report-only` → `enforce`) só após coleta (`F9-T008`).

---

## Tasks by User Story / Priority

### US-1 / P1 — Security gates block bad PRs

- [ ] T001 [US1] `security-gates-ci`: estender `ci-web.yml` com `sca` (`osv-scanner` + `Trivy fs` + `npm audit`), `sast` (`CodeQL` + `Semgrep`), `secrets` (`gitleaks`), `dependency-review`, `lockfile-lint`; **sem** `continue-on-error`; `web/.github/workflows/ci-web.yml`.
- [ ] T002 [US1] `dependabot-config`: criar `.github/dependabot.yml` para `npm /web`; `web/`.
- [ ] T003 [US1] `lint-anti-xss`: ativar `typescript-eslint` com `react/no-danger`, `no-eval`, `no-new-func`, `no-unsanitized`; `eslint-plugin-security`; `web/package.json` + `.eslintrc`.

### US-2 / P1 — CSP protege o app

- [ ] T004 [US2] `csp-header-config`: configurar `Content-Security-Policy` + `Reporting-Endpoints` + `report-uri` (fallback) via proxy (`app.legnaris.com.br`); `connect-src` inclui `https://api.legnaris.com.br`; `require-trusted-types-for 'script'`; começar em `report-only` (`F9-T008`).
- [ ] T005 [US2] `trusted-types-policy`: inicializar `trusted-types` default em `web/src/main.tsx`; testes `F4-T007` + `F9-T006`; `web/src/main.tsx`.

### US-3 / P2 — Headers presentes e testados

- [ ] T006 [US3] `headers-test`: criar `web/src/test/headers.spec.ts` (ou `tests/integration/headers.test.ts`) que faz `curl -I` e confirma `HSTS`, `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`, `COOP`, `CORP`; falha se algum sumir (`F9-T010`).

### US-4 / P2 — DAST detecta novos alertas

- [ ] T007 [P] [US4] `zap-baseline-cfg`: criar `.zap/rules.tsv`; configurar `zap-baseline` no `ci-web.yml`; salvar artefato; falha se novo alerta sem justificativa (`F9-T005`).

### US-5 / P2 — Trusted Types bloqueia sinks

- [ ] T008 [US5] `trusted-types-sink-test`: testar `element.innerHTML = ...` rejeitado sem política confiável; testear `<script>` como texto renderiza escapado (`F9-T006`).

### US-6 / P3 — Rate limiting protege endpoints

- [ ] T009 [P] [US6] `backend-csp-report-endpoint`: implementar `POST /csp-report` em `internal/app/auth.go` (ou rota dedicada); aceitar `application/csp-report` + `application/reports+json`; responder `204`; sanitizar URL; log + métrica (`F9-T003`).
- [ ] T010 [US6] `rate-limit-shared`: aplicar rate limit compartilhado (`F6B-T006`) a `/auth/*` + `POST /csp-report`; responder `429` + `Retry-After`; testar (`F9-T004`).

### Cross-cutting (todas as US)

- [ ] T011 [P] [US1/US2/US3/US4/US5/US6] `check-requirement-ids-ts`: estender `scripts/check_requirement_ids.py` para `*.ts/*.tsx`; atualizar `ci-spec-lint.yml`; `scripts/check_requirement_ids.py`; `web/`.
- [ ] T012 [P] [US3/US8] `csp-report-to-enforce`: registrar violações coletadas (`report-only`); virada para `enforce` (`Content-Security-Policy`) após coleta; artefato do relatório; `F9-T008`.
- [ ] T013 [P] [US1/US4] `sec-scan-make`: adicionar `sec-scan` ao `Makefile` (`lint-web` + `lint-specs` + `npm audit` + `osv-scanner`); `Makefile`; `web/AGENTS.md` atualizado.
- [ ] T014 [US2/US5] `sw-external-register`: confirmar registro de SW externo (`registerSW.ts`, não `injectRegister` inline) para manter `script-src 'self'`; `web/src/pwa/registerSW.ts`; `F1-T006`.

---

## Parallel Examples

- `T001` + `T003` + `T011`: CI + lint + script de validação são independentes do backend.
- `T004` + `T006` + `T005`: CSP header + headers test + Trusted Types podem ser feitos em paralelo (não dependem de endpoint backend).
- `T009` + `T010`: endpoint + rate limit são sequenciais (endpoint primeiro, depois rate limit sobre ele).
- `T007` + `T012`: DAST + virada `enforce` são sequenciais (coleta primeiro, depois enforce).

---

## Implementation Strategy (detalhado)

1. **Pre-implementation (testes devem falhar)**: rodar `F9-T010` (headers ausentes) → falha; `F9-T005` (ZAP alerta novo) → falha; `F9-T006` (escaping falha) → falha; `F4-T007` (XSS dado falha) → falha.
2. **Phase A (CI + lint)**: `T011` → `T003` → `T001` → `T013`. Confirmar `make sec-scan` passa.
3. **Phase B (CSP + Trusted Types)**: `T014` (SW) → `T004` (CSP report-only) → `T005` (Trusted Types) → `T006` (headers test passa).
4. **Phase C (Backend)**: `T009` (endpoint) → `T010` (rate limit) → testes Go verdes.
5. **Phase D (DAST + enforce)**: `T007` (ZAP config) → rodar `zap-baseline` → corrigir alertas → `T012` (virada enforce).
6. **Verification final**: `make fmt && make lint && make vet && make lint-specs && make test-race && make sec-scan`; todos os `SEC-*` têm testes passando.
