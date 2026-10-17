# Data Model: Web Security Hardening (F9)

**Note**: Nenhuma nova entidade de dados é introduzida. Os dados envolvidos são existentes ou configuracionais.

| Entidade / Config | Fonte | Uso nesta feature |
|---|---|---|
| `SEC-*` (requisito IDs) | `.specs/features/013-web-security-hardening/spec.md` | Rastreamento de testes e gates |
| `csp-report` payload | `app.legnaris.com.br` → `POST /api.legnaris.com.br/csp-report` | Relatório de violação de CSP (formato `application/csp-report` ou `application/reports+json`) |
| `.zap/rules.tsv` | `.zap/` (novo) | Alertas DAST aceitos (justificativa) |
| `package-lock.json` | `web/` | Entrada para `osv-scanner` / `trivy` / `npm audit` |
| `today` / `build-id` | CI artifacts | Rastreabilidade de testes (`F9-T010`, `F9-T005`) |

**Identidade / Unicidade**: Nenhuma chave nova; rate limit usa chave composta (`IP + identity`) já existente.
