# Feature Specification: Web Security Hardening

**Feature Branch**: `013-web-security-hardening`

**Created**: 2026-10-17

**Status**: Draft

**Input**: User description from `docs/plan-web-pwa.md` F9 (`web-security-hardening`) — "hardening de segurança do PWA: CSP/headers, Trusted Types, escaping (anti-XSS), DAST (OWASP ZAP baseline), SAST (CodeQL + Semgrep), SCA (osv-scanner + Trivy + Dependabot), SBOM, rate-limiting em `/auth/*`, endpoint de CSP-report no backend (`POST /csp-report` → Loki/Grafana), testes de interface (axe + Playwright), e port de `check_requirement_ids.py` para TS/TSX. Requisito-ID prefixo: `SEC-`."

---

## Clarifications

### Session 2026-10-17

- **Q (implícito pelo plano §4/§9)**: A CSP é emitida pela origem do frontend (`app.legnaris.com.br`) ou pela API? **A**: Pelo frontend (proxy/Traefik); a API só retorna JSON. Confirmado no `Assumptions` e `SEC-02`.
- **Q (implícito pelo plano §4/§11)**: O `report-only` → `enforce` pode ser feito imediatamente ou precisa de coleta? **A**: Requer coleta prévia (`F9-T008`); o rollout é `report-only` → corrigir → `enforce`.
- **Q (implícito pelo plano §8)**: Fila de PATCH offline (Background Sync) entra no escopo? **A**: Fora de escopo salvo pedido (declarado no `Assumptions`).
- **Q (implícito pelo plano §10)**: `check_requirement_ids.py` precisa cobrir TS agora ou apenas depois? **A**: Mesmo ciclo (`F9-T010` / `SEC-08`) — é parte deste hardening.
- Nenhuma outra ambiguidade que altere arquitetura, segurança ou aceitação.

Nenhuma ambiguidade crítica permanece; não há perguntas adicionais que mudem o escopo do `SEC-`.

- **CSP emitida pela origem do frontend** (`app.legnaris.com.br`); a API (`api.legnaris.com.br`) só retorna JSON e não define CSP.
- **Rate limiting entra no escopo** (`/auth/*` + `POST /csp-report` com limitador compartilhado).
- **Fila de PATCH offline (Background Sync)** está fora de escopo salvo pedido explícito.
- **Não há alteração de design/UX** além de headers invisíveis; o usuário não interage diretamente com CSP/ZAP.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Security gates block bad PRs (Priority: P1)

Um colaborador abre um PR que muda `web/src/` e introduz uma dependência vulnerável, uma string tipo segredo, um código que usa `dangerouslySetInnerHTML`, ou remove um header de segurança. Os gates de CI (`osv-scanner`, `Trivy`, `Semgrep`, `CodeQL`, `gitleaks`, `lint-web`, `sec-scan`, `lint-specs`) falham e o merge é bloqueado.

**Why this priority**: sem esses gates, cada feature posterior pode introduzir vulnerabilidades irreversíveis; o hardening precisa existir antes da retirada do desktop (`F10`).

**Independent Test**: abrir um PR de teste com uma dependência vulnerável e confirmar que `osv-scanner`/`Trivy` rejeitam; outro PR com `dangerouslySetInnerHTML` e confirmar que o lint anti-XSS rejeita; outro com uma chave falsa e confirmar `gitleaks`; outro que altera `ci-web.yml` removendo um gate e confirmar que `lint-specs` + `ci-web.yml` detectam.

**Acceptance Scenarios**:

1. **Given** um PR que altera `web/src/engine/field_control.tsx` e introduz `<script>` como valor de texto, **When** `lint-web` roda, **Then** `no-unsanitized`/`react/no-danger` rejeita.
2. **Given** um PR que adiciona `lodash@4.17.20` (vulnerável) ao `package-lock.json`, **When** `osv-scanner` roda, **Then** o gate `sca` falha.
3. **Given** um PR que contém `AWS_ACCESS_KEY_ID=AKIA...`, **When** `gitleaks` roda, **Then** `secrets` falha.
4. **Given** um PR que altera `.github/workflows/ci-web.yml` e remove o job `sast`, **When** `lint-specs` roda, **Then** `check_test_assertions.py` / `lint-specs` detecta.

---

### User Story 2 — CSP protege o app de injeção (Priority: P1)

O app é servido em HTTPS (`app.legnaris.com.br`). O browser recebe `Content-Security-Policy` (enforce, não `report-only`) com `script-src 'self'`, `connect-src 'self' https://api.legnaris.com.br`, `frame-ancestors 'none'`, `base-uri 'self'`, `object-src 'none'`, `require-trusted-types-for 'script'`. Se uma tentativa de XSS injeta um script externo, o browser bloqueia e envia um `POST /csp-report` ao backend.

**Why this priority**: CSP é a última linha de defesa contra XSS; sem ela, um ataque ainda age na sessão; com ela + Trusted Types, o impacto é contido.

**Independent Test**: servir o app com a CSP ativa; tentar injetar `<script src="https://evil.example.com/x.js">` no conteúdo; confirmar que o browser bloqueia e envia o report ao endpoint.

**Acceptance Scenarios**:

1. **Given** o app está servido com `Content-Security-Policy: ...` (enforce), **When** um script externo é injetado, **Then** o browser bloqueia (`blocked-uri` no report) e envia `POST /csp-report`.
2. **Given** o endpoint `POST /csp-report` existe no backend (`internal/app/auth.go` ou rota dedicada), **When** recebe o payload, **Then** responde `204`, sanitiza `document-uri` (remove query/hash) e emite log estruturado (`shared/logger`) + métrica Prometheus.
3. **Given** o `report-only` está ativo em pré-produção, **When** uma violação ocorre, **Then** o app não quebra, mas o report é coletado.

---

### User Story 3 — Headers de segurança estão presentes e testados (Priority: P2)

Cada resposta HTTP do frontend traz `HSTS`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy`, `COOP`, `CORP`. Os testes falham se algum header sumir.

**Why this priority**: headers são baratos e bloqueiam MITM, clickjacking e MIME-sniffing; sem testes, podem ser removidos acidentalmente.

**Independent Test**: fazer uma requisição `curl -I` ao app e confirmar todos os headers; rodar `F9-T010` e confirmar que a falha ocorre se algum header é removido.

**Acceptance Scenarios**:

1. **Given** uma requisição `GET` ao app, **When** os headers são inspecionados, **Then** todos os headers obrigatórios estão presentes.
2. **Given** um teste de integração (`F9-T010`), **When** um header é removido do `vite.config.ts` / proxy, **Then** o teste falha com mensagem clara.

---

### User Story 4 — DAST detecta novos alertas (Priority: P2)

O pipeline `ci-web.yml` roda `zap-baseline` contra um preview deploy. Se um novo alerta de alta/criticalidade é introduzido, o gate falha. Os resultados (`zap-baseline` output) são salvos como artefato, e `.zap/rules.tsv` define os alertas aceitos (nenhum aceito por padrão, a menos que registrado com justificativa).

**Why this priority**: DAST captura problemas de runtime (configuração, integração) que SAST/SCA não veem.

**Independent Test**: rodar `zap-baseline` contra `https://preview.legnaris.com.br`; confirmar que novos alertas falham.

**Acceptance Scenarios**:

1. **Given** um preview deploy, **When** `zap-baseline` roda, **Then** novos alertas de alta/critical falham; alertas aceitos (registrados em `.zap/rules.tsv`) passam.
2. **Given** um alerta é aceito, **When** `.zap/rules.tsv` é atualizado, **Then** o gate passa com evidência do arquivo.

---

### User Story 5 — Trusted Types bloqueiam sinks não confiáveis (Priority: P2)

O backend serve o header `Content-Security-Policy` com `require-trusted-types-for 'script'`. A aplicação inicializa uma política default (`trusted-types` no `main.tsx`) e só usa sinks DOM (`innerHTML`, `setAttribute`, `document.write`) após sanitização (`DOMPurify`) ou com objetos confiáveis (`TrustedHTML`). O lint (`no-unsanitized`) e testes (`F4-T007`) cobrem o caso de `<script>` como texto.

**Why this priority**: Trusted Types fecha a última porta de XSS baseada em DOM.

**Independent Test**: tentar atribuir `element.innerHTML = "<b>teste</b>"` sem `trusted-types` e confirmar que a política default lança exceção no browser; usar `DOMPurify` + `trusted-types` e confirmar que passa.

**Acceptance Scenarios**:

1. **Given** o header `require-trusted-types-for 'script'` está ativo, **When** um sink DOM recebe uma string não confiável, **Then** o browser rejeita e o teste `axe` / `lint` captura.
2. **Given** um dado com `<script>` é renderizado, **When** o componente `field_control` o exibe, **Then** o texto aparece como texto (escapado) e não executa.

---

### User Story 6 — Rate limiting protege `/auth/*` e CSP-report (Priority: P3)

O backend (`internal/app/auth.go`) aplica rate limiting (`F6B-T006`) a `/auth/login`, `/auth/register`, `/auth/refresh`, `/auth/logout` e ao endpoint `POST /csp-report` (limitador compartilhado). Excesso retorna `429` com `Retry-After`.

**Why this priority**: sem rate limit, `/auth/*` é vulnerável a força bruta; o CSP-report pode ser usado para DoS.

**Independent Test**: fazer 10 requisições `POST /auth/login` com senha errada; confirmar `429` após o limite.

**Acceptance Scenarios**:

1. **Given** requisições repetidas a `/auth/login`, **When** o limite é excedido, **Then** `429` com `Retry-After` é retornado.
2. **Given** requisições `POST /csp-report`, **When** o limite compartilhado é excedido, **Then** `429` é retornado.

---

## Edge Cases

- **CSP `report-only` → `enforce` sem coleta prévia**: se virarmos direto para `enforce` sem `report-only`, violações reais podem quebrar o app. Mitigação: gate `F9-T008` exige registro das violações coletadas antes da virada.
- **Trusted Types sem política default**: se a política default não é inicializada, sinks que não usam `trusted-types` ainda funcionam. Mitigação: `main.tsx` inicializa `trusted-types` antes de qualquer renderização.
- **CORS + cookie + CSP conflitam**: se `connect-src` da CSP omitir `https://api.legnaris.com.br`, o `fetch` com `credentials: 'include'` falha. Mitigação: `F9-T002` fecha CORS; `F9-T001` define `connect-src` incluindo a API.
- **Rate limit bloqueia usuário legítimo**: se o limite é muito baixo. Mitigação: o limite é configurado via `shared/env` (`AUTH_RATE_LIMIT`) e testado com valores razoáveis.
- **ZAP falha por falso positivo**: `zap-baseline` pode reportar alertas de baixa severidade que já estão aceitos. Mitigação: `.zap/rules.tsv` registra alertas aceitos com justificativa.

---

## Requirements *(mandatory)*

Prefixo: `SEC-`. Cada requisito é testável.

### Functional Requirements

- **SEC-01**: O pipeline `ci-web.yml` DEVE incluir jobs `sca` (`osv-scanner` + `Trivy fs` + `npm audit`), `sast` (`CodeQL` + `Semgrep`), `secrets` (`gitleaks`), `dependency-review`, `lockfile-lint` e `quality` (`lint-web` + `test-web` + build), todos **sem** `continue-on-error`.
- **SEC-02**: A CSP emitida pelo frontend (`app.legnaris.com.br`) DEVE ser `enforce` (não `report-only` após a virada de `F9-T008`), incluir `script-src 'self'; connect-src 'self' https://api.legnaris.com.br; frame-ancestors 'none'; base-uri 'self'; object-src 'none'; require-trusted-types-for 'script'`, e ser declarada via header HTTP (`Content-Security-Policy`), não meta-tag.
- **SEC-03**: O endpoint `POST /csp-report` no backend DEVE aceitar `application/csp-report` e `application/reports+json`, responder `204`, sanitizar `document-uri`/`blocked-uri` (remover query/hash), emitir log estruturado (`shared/logger`) e métrica Prometheus.
- **SEC-04**: O rate limiting (`F6B-T006`) DEVE ser aplicado a `/auth/login`, `/auth/register`, `/auth/refresh`, `/auth/logout` e `POST /csp-report` (limitador compartilhado), retornando `429` com `Retry-After`.
- **SEC-05**: Os headers `HSTS`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy`, `COOP`, `CORP` DEVEM estar presentes em todas as respostas do app, e `F9-T010` DEVE falhar se algum sumir.
- **SEC-06**: A aplicação DEVE inicializar `trusted-types` com uma política default (`main.tsx`) antes de qualquer renderização; `F9-T006` DEVE testar que sinks não confiáveis são rejeitados.
- **SEC-07**: O lint anti-XSS (`typescript-eslint` com `react/no-danger`, `no-eval`, `no-new-func`, `no-unsanitized`) DEVE ser ativado em `web/`; `F4-T007` e `F9-T006` DEVEM testar que `<script>` como texto é renderizado como texto (escapado).
- **SEC-08**: `check_requirement_ids.py` DEVE ser estendido para cobrir `*.ts/*.tsx` (F1-T010); `ci-spec-lint.yml` DEVE incluir `web/`; todas as citações `SEC-*` no código DEVE ser validadas pelo gate.
- **SEC-09**: A transição `Report-Only` → `Enforce` (F9-T008) DEVE ser registrada: o arquivo `.zap/rules.tsv` e o log de violações coletadas DEVEM ser preservados; a virada só ocorre após a coleta.
- **SEC-10**: O `vite-plugin-pwa` (`injectManifest`) DEVE registrar o service worker externamente (não `injectRegister` inline) para manter `script-src 'self'` sem `unsafe-inline`.
- **SEC-11**: `make sec-scan` (novo no `Makefile`) DEVE rodar `lint-web`, `lint-specs`, `npm audit`, `osv-scanner` (se disponível no ambiente) e `codeql` (simulado) como gate local antes do commit.

---

## Assumptions

- O backend (`internal/app/auth.go`) já implementou o cookie `__Host-legnaris_refresh` (`F6B-T001`) e a validação de `X-CSRF-Token` (`F6B-T002`) — `F9` depende de `F6` para `POST /auth/*` e `POST /csp-report` funcionarem com `credentials: 'include'`.
- A topologia é `app.legnaris.com.br` (frontend) + `api.legnaris.com.br` (backend); `connect-src` da CSP inclui a API.
- `Loki` + `Prometheus` + `Grafana` já existem no `deploy/` para receber os logs/métricas do CSP-report.
- `playwright`, `axe-core`, `vite`, `typescript-eslint`, `semgrep`, `osv-scanner`, `trivy`, `zap-baseline` já estão disponíveis no ambiente CI (ou adicionados ao `package.json` / `Makefile`).

---

## Key Entities

- `SEC-*` — requisitos de segurança desta feature.
- `web/src/pwa/sw.ts` — service worker (`injectManifest`).
- `web/src/pwa/registerSW.ts` — registro externo do SW.
- `internal/app/auth.go` — backend: rate limit + CSP-report endpoint.
- `.github/workflows/ci-web.yml` — pipeline com gates de segurança.
- `.zap/rules.tsv` — alertas DAST aceitos.
- `check_requirement_ids.py` — script de validação de IDs de requisito.

---

## Success Criteria

1. Todos os requisitos `SEC-01` a `SEC-11` estão cobertos por testes (unit, integração, E2E) e validados pelo `lint-specs`.
2. Nenhum gate de segurança usa `continue-on-error`.
3. A CSP está em `enforce`, com `Trusted Types`, `CSP-report` funcionando, `rate-limiting` ativo e todos os headers presentes.
4. `F9-T011` (testes de headers) passa e falha corretamente ao remover qualquer header obrigatório.
5. A feature está documentada em `.specs/features/013-web-security-hardening/checklists/requirements.md` e `tasks.md`; `tasks.md` está completo com todas as tarefas tickadas após a implementação.
