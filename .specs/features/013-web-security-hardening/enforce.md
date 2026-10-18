# CSP Enforce Rollout (F9-T012 / SEC-09)

**Status**: `enforce` entregue como header HTTP real em `app.legnaris.com.br`.

**Onde a política é entregue**: middleware **dedicado** `web-security-headers` da Traefik
(`deploy/traefik/dynamic.yml`), aplicado somente ao router `web` (`WEB_DOMAIN`). O middleware
`security-headers` compartilhado com Grafana/Prometheus/Mongo Express **não** foi alterado, então
uma mudança de CSP aqui não pode derrubar a stack de observabilidade.

**Cabeçalhos entregues**: `Content-Security-Policy` (`default-src 'self'`, `frame-ancestors 'none'`,
`require-trusted-types-for 'script'`, `report-to`/`report-uri`), `Reporting-Endpoints`,
`Strict-Transport-Security`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
`Referrer-Policy`, `Permissions-Policy`, `Cross-Origin-Opener-Policy: same-origin` e
`Cross-Origin-Resource-Policy: same-origin`.

**Passos realizados**:
1. CSP **entregue direto em `enforce`**, sem fase `report-only`: esta é a primeira entrega de CSP como
   header HTTP do produto (o rollout anterior era `<meta>` + middleware Echo, ambos em enforce), não
   uma virada sobre tráfego pré-produção. A coleta de violações (`F9-T008`) segue ativa em produção
   via `POST /csp-report`, mas não foi pré-requisito da virada — desvio registrado em `tasks.md`
   T004/T012.
2. `Trusted Types` inicializado (`F9-T005`).
3. `.zap/rules.tsv` preservado e ampliado: as regras de CSP/headers viram `FAIL` no baseline scan
   (`F9-T007`, `F10` Workstream C).
4. `POST /csp-report` implementado de fato (`F10` Workstream A): aceita
   `application/csp-report` e `application/reports+json` (tolera `application/json`), normaliza para
   um struct único, sanitiza `document-uri`/`blocked-uri`, emite log estruturado, incrementa
   `legnaris_csp_reports_total`, aplica rate limit próprio (chave `csp:ip:`) e **sempre** responde
   `204`. O stub `AuthController.CSPReport` foi removido (`internal/app/csp_report.go` substitui).

**Alterações de código desta virada** (correção do registro anterior, que afirmava "nenhuma
alteração de código é necessária além do header"):

- Remoção do `<meta http-equiv="Content-Security-Policy">` de `web/index.html`: meta não cobre
  `frame-ancestors` nem entrega reporting, e duplicava a política.
- Remoção do `cspMiddleware()` do Echo (`internal/app/router.go`): a CSP é cabeçalho de documento,
  servido na origem do frontend pela Traefik; a API só devolve JSON, então manter a política em dois
  lugares era drift garantido.
- `web/vite.config.ts` espelha a mesma lista de cabeçalhos em `server`/`preview`, para o gate local
  de headers ter uma origem real contra a qual rodar (Workstream B).

**Evidência de verificação**:
- `web/e2e/headers.spec.ts` (Playwright) falha se CSP/HSTS/nosniff/Referrer/COOP/CORP/Permissions
  sumirem, e falha se o header vier como `report-only`; substitui o teste vazão
  `web/src/test/headers.spec.ts`.
- `grep` confirma a ausência de `cspMiddleware` no backend.
- Produção: `curl -I https://app.legnaris.com.br` retorna a CSP em `enforce` mais os demais
  headers (ver `quickstart.md`).

**Artefato desta virada**: este arquivo (`F9-T012`), atualizado por `F10` (POL-02 / POL-09).
