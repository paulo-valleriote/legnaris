# Research: Web Security Hardening (F9)

**Status**: Phase 0 complete (design decisions recorded below; no open NEEDS CLARIFICATION).

## Decisions

- **CSP emitida pelo frontend** (`app.legnaris.com.br` via proxy/Traefik), não pela API Echo (`api.legnaris.com.br`). Justificativa: CSP é header do documento; API só retorna JSON.
- **Trusted Types via política default** (`main.tsx`). Justificativa: fecha a última porta de DOM-based XSS sem alterar lógica de UI Engine (`F4`).
- **Rate limit compartilhado** entre `/auth/*` e `POST /csp-report`. Justificativa: plano §10 define limitador compartilhado; reutiliza infraestrutura existente.
- **DAST: `zap-baseline` com `.zap/rules.tsv`**. Justificativa: planeamento §8 exige falha em novo alerta; registros aceitos têm justificativa.
- **Report CSP endpoint (`POST /csp-report`) no backend Go** (`internal/app/auth.go` ou rota dedicada), sem auth, resposta `204`, cap de corpo, sanitização de URL, log (`shared/logger`) + métrica Prometheus → alerta Grafana. Justificativa: plano Apêndice A e §4.
- **`check_requirement_ids.py` estendido para `*.ts/*.tsx`**. Justificativa: plano §8; sem isso, citações `SEC-*` no web não passam pelo gate.

## Rejected Alternatives

- **CSP apenas em meta-tag** (`<meta http-equiv="Content-Security-Policy">`): rejeitada — meta-tag não permite `report-uri` nem `frame-ancestors` completo; header HTTP é a única forma válida para `Trusted Types` e `report-to`.
- **DAST apenas em deploy completo, não PR**: rejeitada — o plano exige gate no PR (`ci-web.yml`); apenas agendado seria tarde demais.
- **CSP `report-only` permanente**: rejeitada — o plano define saída (`F9-T008`) com `enforce`; `report-only` é fase de rollout, não estado final.
- **Remover desktop (`F10`) antes de F9**: rejeitada — o plano (§5) coloca `F6` (auth cookies) antes de `F7`, e `F9` após `F7`; `F10` é o último.
- **Usar `dangerouslySetInnerHTML` com `DOMPurify` sem `trusted-types`**: rejeitada — o plano exige `require-trusted-types-for 'script'` (F9-T002); `DOMPurify` é usado, mas dentro de uma política confiável.

## Rationale / Evidence

- Constituição v1.1.1 confirma `Security Gates Are Blocking` (VI) e `Installable Web Client` (VII); `make lint-web`, `make test-web`, `make sec-scan` existem.
- `internal/app/auth.go` já protege todas as rotas `/templates` e `/sheets`; `auth_integration_test.go` proíbe `Set-Cookie`; `F6B-T001` emenda isso para cookie `__Host-`.
- `docs/plan-web-pwa.md` §4 confirma topologia `app.legnaris.com.br` + `api.legnaris.com.br`; `connect-src` deve incluir API.
- `desktop/AGENTS.md` define tokens de design; `F1-T003` já os porta para `theme/tokens.css`; `F9` não altera UI.
