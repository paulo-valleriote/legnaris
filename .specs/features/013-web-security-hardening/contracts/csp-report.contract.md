# Contract: CSP-Report Endpoint (`POST /csp-report`)

**Source**: `docs/plan-web-pwa.md` §4 / Apêndice A; `F9-T003` (`SEC-03`).

**Location**: `internal/app/auth.go` (ou rota dedicada no backend Go/Echo) — `POST /csp-report`.

**Request**:
- `Content-Type`: `application/csp-report` (formato `csp-report`) **OU** `application/reports+json` (formato `report-to`).
- `Body`: JSON (capped a 16 KB via `http.MaxBytesReader`).
- **Sem auth**: endpoint público (não requer cookie/token — reports não incluem credenciais).

**Response**:
- `204 No Content` (sempre, mesmo com erro interno — para não gerar ruído no browser).

**Comportamento do servidor**:
1. Aceita `application/csp-report` ou `application/reports+json` (e tolera `application/json`).
2. Normaliza os dois formatos para um struct único (`csp_report`).
3. Sanitiza `document-uri` e `blocked-uri` (remove query/hash antes de logar).
4. Emite log estruturado (`shared/logger`) com campos: `effective_directive`, `blocked_uri` (sanitizado), `disposition`, `timestamp`.
5. Incrementa métrica Prometheus (`csp_reports_total`) com label `effective_directive`.
6. Aplica rate limit (limitador compartilhado com `/auth/*` — `F6B-T006`).

**Falha / Erro interno**: nunca retorna 500 ao cliente; se o log/métrica falhar, o endpoint ainda retorna 204 (fire-and-forget). Erros internos são registrados no log com nível `error`.

**Testes (Go)**: `F9-T003` — teste que envia payload `application/csp-report` e `application/reports+json`, confirma `204`, sanitização de URL, log emitido, métrica incrementada.
