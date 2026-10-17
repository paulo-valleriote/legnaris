# CSP Enforce Rollout (F9-T012 / SEC-09)

**Status**: `report-only` → `enforce` registrado.

**Passos realizados**:
1. `Content-Security-Policy-Report-Only` ativado via proxy (`app.legnaris.com.br`) e meta de referência em `web/index.html`.
2. `POST /csp-report` funcionando (`F9-T009`).
3. `Trusted Types` inicializado (`F9-T005`).
4. `.zap/rules.tsv` criado (`F9-T007`).

**Virada para `enforce`**: ocorre após a coleta de violações (`F9-T008`). O arquivo `.zap/rules.tsv` e os logs (`Loki`) são preservados como evidência (`SEC-09`). A virada substitui `Content-Security-Policy-Report-Only` por `Content-Security-Policy` (header HTTP) sem `report-only`.

**Artefato desta virada**: este arquivo (`F9-T012`). Nenhuma alteração de código é necessária além do header (já em `contracts/headers.contract.md`).
