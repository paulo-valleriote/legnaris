# Plano Web PWA (Legnaris) — v3

> **Programa**: substituir o app Flutter desktop por um PWA React/TS, seguindo as fases do
> desktop, com SCA/SAST/DAST e testes de lógica e de requisitos de interface.
> **Status**: v3 FINAL (2026-10-15). Documento de **planejamento** — a execução acontece pelo
> ciclo Spec Kit (ver `.specs/features/001-web-pwa-foundation/`).
> **Escopo dos arquivos**: este documento é o plano-programa (F0–F10) e fica em pt-BR, na
> exceção prevista pela constituição para `docs/`. Ele não define requirement IDs — quem os
> define é `.specs/**/spec.md` (`FND-*`, `SEC-*`, …); decisões arquiteturais ficam em
> `.specs/STATE.md` (`AD-*`).
>
> Este arquivo consolida os dois documentos de planejamento produzidos em 2026-10-15
> (`web-pwa-tecnologias-e-tarefas.md` e `web-pwa-plano-detalhado.md`): a Parte A é o resumo e a
> Parte B é o detalhamento autoritativo de cada tarefa.

---

## Parte A — Resumo

> **Status: FINAL (v3)** — resumo. O detalhamento autoritativo de cada tarefa está na Parte B
> (inclui o Apêndice A — CSP report).
> Objetivo: substituir o app Flutter desktop por um **PWA React/TS**, espelhando as fases
> do desktop, com **SCA/SAST/DAST**, testes de **lógica** e de **requisitos de interface**.
> Data: 2026-10-15. **Não implementar** (este documento é só planejamento).

---

### 1. Contexto do repositório (fatos verificados)

- Monorepo: backend Go (`internal/`, `shared/`, `cmd/`), desktop Flutter (`desktop/`),
  e `web/` **já criado e vazio** (`web/specs/`, `web/src/`) — destino do PWA.
- Spec Kit instalado em `.specify/` (v1.2.0, integração `opencode`). Ciclo:
  `constitution → specify → clarify → plan → tasks → analyze → implement`.
  Specs em `.specs/features/NNN-<slug>/` (fonte canônica de requirement IDs).
- `.specify/memory/constitution.md` v1.0.0 fixa o stack: *"Desktop: Flutter with Riverpod
  under `desktop/`"* → **trocar de stack exige emenda da constituição**.
- CI atual: `ci-lint` (Go), `ci-flutter` (desktop Windows), `ci-spec-lint`
  (lê `*.go`/`*.dart` + `.specs/`), `opencode-review`.
- Decisões duras do desktop já registradas em `.specs/STATE.md` (AD-001…AD-010):
  granularidade de rebuild por campo, grupos exclusivos, biblioteca local, draft backup,
  sessão, dry-run/preview, merge incremental de cascata.

#### 1.1 Achados que corrigem o TDD (importante)

1. **Auth já é obrigatória.** `internal/app/router.go` põe `auth.Middleware` em **todas**
   as rotas `/templates` e `/sheets`; só `health`, `metrics` e `/auth/*` são públicos.
   A premissa "API pública sem auth" (V1 desktop) **não existe mais**.
2. **Refresh via header, não cookie.** `internal/app/auth.go`:
   - `POST /auth/login|register` → `TokenPair {access_token, refresh_token, token_type, expires_in}` no corpo.
   - `POST /auth/refresh|logout` → refresh lido do header **`X-Refresh-Token`** (sem body).
   - `auth_integration_test.go:58` **proíbe** `Set-Cookie` para clientes nativos.
   Decisão (rev. 2): **migrar para cookie httpOnly** e **remover** o caminho nativo — o desktop será descontinuado (feature `backend-browser-auth-cookies`).
3. **`GET /sheets` e `DELETE /sheets/:id` já existem** (ownership Fase 2 pronta) →
   a "biblioteca local de IDs" do desktop deixa de ser fonte da verdade; o servidor é a
   lista; IndexedDB passa a **cache offline + drafts**.

---

### 2. Decisões fechadas (com o usuário)

| Tema | Decisão |
|---|---|
| Stack | **React + Vite + TypeScript + `vite-plugin-pwa`** |
| Auth | **Autenticada** (alvo = backend real, hoje com JWT obrigatório) |
| PWA/offline | **Instalável + offline shell + IndexedDB local** (cache + drafts) |
| Segurança | **GitHub-native + `osv-scanner` + Trivy + Semgrep + OWASP ZAP + SBOM + headers/CSP evaluator** |
| Refresh | **Cookie httpOnly** (`__Host-...`; Secure, SameSite=Strict) + access em memória; **CSRF** via SameSite + double-submit + Origin |
| Desktop | **Descontinuado** → remover o caminho nativo `X-Refresh-Token` no backend |

---

### 3. Stack e trade-offs

| Dimensão | Escolha | Trade-off / justificativa |
|---|---|---|
| UI/build | React + TS + Vite | Reescreve o **UI Engine** (lógica já fixada em AD-001…AD-010 = *port*, não redesign). Ganha PWA/SEO/a11y/tooling de segurança maduros. |
| PWA | `vite-plugin-pwa` (Workbox) | `generateSW` (simples) vs **`injectManifest`** (SW custom p/ offline shell + drafts). Escolher `injectManifest` no PWA (F8). |
| Estado servidor | TanStack Query | cache + invalidation; substitui os providers de rede do Riverpod. |
| Estado de edição | Zustand + `selectors` (ou Jotai por-path) | precisa **preservar AD-001**: 1 edição → rebuild de 1 controle (subscrição por caminho). |
| Rotas | React Router (history) | exige fallback `index.html` no host + SW (rota profunda em PWA). |
| HTTP | `fetch` nativo + wrapper de envelope | sem axios; erro tipado replicando `desktop/lib/api/error_mapper.dart`. |
| Persistência local | IndexedDB (`idb`) | espelha AD-003/AD-008 (drafts), mas como **cache offline**, versionado. |
| i18n | módulo de chaves tipado (ou i18next) | mantém "English only" e o pipeline de strings do desktop (`*.arb`). |
| Testes | Vitest + Testing Library + MSW; Playwright + axe; screenshots | cobre lógica **e** requisitos de interface. |
| Segurança | Dependabot + CodeQL + Secret scanning + `osv-scanner` + OWASP ZAP | gates no CI desde a Fase 0. |

#### Alternativa descartada
**Flutter Web** (reuso do engine/models/api). Prós: quase zero reescrita. Contras:
canvas ⇒ SEO/a11y pobres, bundle grande, PWA/roteamento atípicos, ecossistema de
**SAST/SCA fraco para Dart**. Não atende ao foco do pedido (segurança + PWA + testes de interface).

---

### 4. Modelo de auth com cookie httpOnly

Como o desktop será descontinuado, o browser passa a ser o único cliente:
o refresh vai em **cookie httpOnly** e o access token fica **em memória**.

- **Refresh**: `Set-Cookie: __Host-legnaris_refresh=<opaque>; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=<AUTH_REFRESH_TTL>`
  (prefixo `__Host-` exige `Secure` + `Path=/` + sem `Domain`). O refresh **não** vai no corpo.
- **Access token**: no corpo, guardado **só em memória**; enviado como `Authorization: Bearer`
  em `/sheets` e `/templates` (essas rotas **não** usam cookie → sem CSRF nelas).
- **Reload**: **silent refresh** via `POST /auth/refresh` (`credentials: 'include'`), que rotaciona e
  devolve novo access token.
- **CSRF** (novo vetor, pois o cookie é credencial ambiente): `SameSite=Strict` +
  **double-submit token** (`__Host-legnaris_csrf` não-httpOnly, reenviado em `X-CSRF-Token` e
  validado contra o servidor) + **validação de `Origin`/`Referer`** — só em `/auth/refresh` e `/auth/logout`.
- **CORS**: `Access-Control-Allow-Credentials: true` com **origem exata** (nunca `*`), `Vary: Origin`.
- **Topologia**: app em `www.legnaris.com.br`, API em `www.api.legnaris.com.br` — cross-origin,
  **same-site** (não é third-party cookie, por isso subdomínio); CSP emitida pela origem do
  **frontend**, `connect-src` da API.
- **XSS**: o refresh httpOnly **não vaza** para JS; um XSS ainda age na sessão (access em memória),
  mas **não** rouba refresh persistente.
- **Backend**: exige a feature **`backend-browser-auth-cookies`** (pré-requisito bloqueante) e a
  remoção do caminho `X-Refresh-Token`.

---

### 5. Sequência Spec Kit

Cada linha vira `spec.md → plan.md → tasks.md` sob `.specs/features/NNN-<slug>/`.

| # | Comando | Slug | Espelha desktop |
|---|---|---|---|
| 0 | `/speckit.constitution` | — | emenda de stack/PWA/segurança |
| 1 | `/speckit.specify` | `web-pwa-foundation` | Fase 0 (setup + CI segurança + tokens) |
| 2 | `/speckit.specify` | `web-api-client` | Fase 1 |
| 3 | `/speckit.specify` | `web-catalog` | Fase 2 |
| 4 | `/speckit.specify` | `web-ui-engine` | Fase 3 |
| 5 | `/speckit.specify` | `web-sheet-cycle` | Fase 4 |
| 6 | `/speckit.specify` | **`backend-browser-auth-cookies`** | **novo (backend, pré-requisito de auth)** |
| 7 | `/speckit.specify` | `web-auth-session` | Fase 6 |
| 8 | `/speckit.specify` | `web-pwa-offline` | PWA |
| 9 | `/speckit.specify` | `web-security-hardening` | hardening |
| 10 | `/speckit.specify` | `web-polish-decommission` | Fase 5 + retirada desktop |

Por feature: `/speckit.clarify` → `/speckit.plan` → `/speckit.tasks` → `/speckit.analyze`.

---

### 6. Estrutura de código proposta (`web/`)

```
web/
  AGENTS.md                 # regras visuais (portadas de desktop/AGENTS.md → CSS)
  package.json  vite.config.ts  tsconfig.json  index.html
  public/manifest.webmanifest  public/icons/
  src/
    main.tsx
    app/{router.tsx,providers.tsx}
    api/{client.ts,envelope.ts,error_mapper.ts,auth_api.ts,authenticated_client.ts,models/*}
    auth/{auth_session.ts,auth_gate.tsx,auth_screen.tsx,token_store.ts}
    catalog/{catalog_screen.tsx,create_sheet_dialog.tsx,queries.ts}
    engine/{field_control.tsx,field_grouping.ts,exclusive_control.tsx,sheet_form.tsx,sheet_defaults.ts,sheet_mark.ts}
    sheet/{sheet_open.ts,sheet_edit.ts,sheet_screen.tsx}
    library/{sheet_library.ts,draft_backup.ts}   # IndexedDB + drafts
    shell/{app_shell.tsx,left_nav.tsx,right_panel.tsx,shortcut_registry.ts}
    theme/{tokens.css,theme.ts}
    pwa/{sw.ts,registerSW.ts}
    l10n/strings.ts
    test/fixtures/*
```

---

### 7. `tasks.md` por feature (esboço pronto para virar artifact)

> Formato Spec Kit: `- [ ] T0xx [P?] [US?] descrição + caminho`. `[P]` = paralelizável.
> IDs reiniciam por feature no run real.

#### F1 — `web-pwa-foundation` — US1: "app shell carrega e instala"

- [ ] T001 Scaffold `web/` (Vite + React + TS) e `web/package.json`
- [ ] T002 [P] ESLint (`typescript-eslint`) + Prettier + `web/tsconfig.json`
- [ ] T003 [P] `web/src/theme/tokens.css` — portar tokens de `desktop/AGENTS.md` (escala, raio, espaçamento, foco, hover)
- [ ] T004 `web/AGENTS.md` — regras visuais (veto "AI SaaS aesthetic", densidade, resize)
- [ ] T005 Shell base `web/src/shell/app_shell.tsx` + `left_nav.tsx` + `right_panel.tsx` (ordem nav|conteúdo|painel; colapso do painel primeiro)
- [ ] T006 [P] `web/vite.config.ts` com `vite-plugin-pwa` (`injectManifest`, manifest, `registerType`)
- [ ] T007 `web/public/manifest.webmanifest` + ícones + `index.html`
- [ ] T008 **CI `ci-web.yml`** (paths `web/**`): `npm ci`, lint, typecheck, Vitest, build, **`osv-scanner`** no lockfile, **CodeQL**, **Secret scanning**
- [ ] T009 [P] `.github/dependabot.yml` para `web/`
- [ ] T010 Ajustar path-filters: `ci-lint.yml` ignorar `web/**`; `ci-spec-lint`/`Makefile` incluir web
- [ ] T011 Teste de fumaça de shell (Vitest + RTL) provando tokens/rotas

#### F2 — `web-api-client` — US1: "consumo tipado da API"

- [ ] T001 `web/src/api/client.ts` — `fetch`, timeout, retry **GET** idempotente
- [ ] T002 [P] `web/src/api/envelope.ts` + `web/src/api/models/*` (Template, FieldSchema, Sheet, TokenPair)
- [ ] T003 `web/src/api/error_mapper.ts` — espelha `desktop/lib/api/error_mapper.dart` (códigos `apperr` → UX)
- [ ] T004 Testes de contrato com **fixtures fixadas** (envelope + delta do PATCH "só cascata")
- [ ] T005 [P] Testes de erro: código desconhecido, corpo sem `error.code`, timeout/rede

#### F3 — `web-catalog` — US: "listar e abrir templates"

- [ ] T001 Queries TanStack (`web/src/catalog/queries.ts`) de catálogo + detalhe
- [ ] T002 `web/src/catalog/catalog_screen.tsx` (loading/vazio/erro distintos, strings ancoradas)
- [ ] T003 [P] `web/src/catalog/create_sheet_dialog.tsx`
- [ ] T004 Testes de interface: estados distintos, navegação, erro inline posicionado junto ao dado

#### F4 — `web-ui-engine` (núcleo; porta de `desktop/lib/engine/`)

- [ ] T001 `web/src/engine/field_grouping.ts` (agrupamento por prefixo) + testes de borda
- [ ] T002 [P] `web/src/engine/field_control.tsx` (numérico/texto/read-only derivado) + testes por tipo
- [ ] T003 `web/src/engine/exclusive_control.tsx` (AD-002) + testes de exclusividade
- [ ] T004 `web/src/engine/sheet_defaults.ts` — `seedFromSheet` + `mergeCascade` (deep-merge)
- [ ] T005 `web/src/engine/sheet_mark.ts` + tint (AD-009: Modified/Recalculated/Calculated)
- [ ] T006 `web/src/engine/sheet_form.tsx` com seams de `control`/`rowMark` (granularidade AD-001)
- [ ] T007 Testes: template com **todos** os tipos, `config.*`→toggle, derivado read-only, marcas de preview (valor presente **e** vazio)

#### F5 — `web-sheet-cycle` — US: "criar/abrir/editar com cascata"

- [ ] T001 `web/src/sheet/sheet_open.ts` — GET por id; `NOT_FOUND` → remove do cache local
- [ ] T002 `web/src/sheet/sheet_edit.ts` — dirty + delta + `save()` PATCH "só delta" + `mergeCascade`
- [ ] T003 [P] `web/src/library/sheet_library.ts` (IndexedDB, `schema_version`) — **cache offline**
- [ ] T004 `web/src/library/draft_backup.ts` — cadência injetável; restore filtrado a folhas editáveis (AD-008)
- [ ] T005 Footer/Save + `Ctrl+S`/`F5` via `web/src/shell/shortcut_registry.ts`
- [ ] T006 Testes **lógicos** (prioridade): cascata (chave nova/sobrescrita/aninhada/`{}`), delta (revertido fora; derivado nunca), idempotência, erro mantém dirty
- [ ] T007 Testes de interface: salvar ok/erro, offline claro (sem merge fantasma), drafts restaurados

#### F6 — `backend-browser-auth-cookies` (backend, pré-requisito) — SEGURANÇA

- [ ] T001 `Set-Cookie: __Host-legnaris_refresh` (HttpOnly/Secure/SameSite=Strict) em login/register; refresh lido do cookie em /refresh e /logout; refresh **fora** do corpo
- [ ] T002 CSRF token (`__Host-legnaris_csrf` não-httpOnly) + validação `X-CSRF-Token` (server-bound) em /refresh e /logout
- [ ] T003 Validação de `Origin`/`Referer` em métodos state-changing
- [ ] T004 CORS com `Allow-Credentials: true` + origem exata (sem `*`) + `Vary: Origin`
- [ ] T005 Remover o caminho nativo `X-Refresh-Token` e o teste que proíbe `Set-Cookie`
- [ ] T006 Rate limiting em `/auth/*`
- [ ] T007 Testes: atributos do cookie; refresh sem CSRF → 403; origem inválida → 403; rotação/expirado

#### F7 — `web-auth-session` — US: "login/registro/logout"

- [ ] T001 `web/src/api/auth_api.ts` (register/login/refresh/logout com `credentials:'include'`)
- [ ] T002 `web/src/auth/session_store.ts` (access **só em memória**; sem persistência de token)
- [ ] T003 `web/src/api/authenticated_client.ts` — 401 → silent refresh **single-flight** + `X-CSRF-Token`; refresh inválido → logout forçado
- [ ] T004 `web/src/auth/auth_gate.tsx` + `auth_screen.tsx` + boundary de sessão (AD-010)
- [ ] T005 Testes: rotação/reuso, CSRF ausente/errado, troca de sessão limpa dados, 401 concorrente
- [ ] T006 CRUD de fichas: `GET /sheets` + `DELETE /sheets/:id` com confirmação

#### F8 — `web-pwa-offline` — US: "instalar e usar offline"

- [ ] T001 SW `injectManifest` (`web/src/pwa/sw.ts`): precache do shell + runtime cache GET
- [ ] T002 `web/src/pwa/registerSW.ts` + UX de update (prompt vs auto)
- [ ] T003 Fallback de rotas deep-link (SW + host)
- [ ] T004 Offline: leitura do cache + drafts; (fila de PATCH fica fora do escopo salvo pedido)
- [ ] T005 Lighthouse PWA/performance como gate no CI

#### F9 — `web-security-hardening`

- [ ] T001 CSP + headers compatíveis com React/Vite (`web/`), `report-only → enforce`
- [ ] T002 Trusted Types (`require-trusted-types-for 'script'`)
- [ ] T003 DAST: **OWASP ZAP baseline** contra preview deploy no CI (falha em novo alerta)
- [ ] T004 `osv-scanner` + **Trivy** + **Semgrep** + SBOM (Syft) + revisão de supply-chain
- [ ] T005 [P] **axe** (a11y) no Playwright + correções
- [ ] T006 Lint anti-XSS + teste de escaping (`<script>` como texto)
- [ ] T007 Teste de headers (falha se CSP/HSTS/nosniff sumirem)

#### F10 — `web-polish-decommission` — US: "paridade e retirada do desktop"

- [ ] T001 Golden/visual (Playwright screenshots) do template de referência
- [ ] T002 Contrato/integração contra API real (self-skip sem API)
- [ ] T003 Cobertura ≥ meta; `web/README.md` + `quickstart.md`
- [ ] T004 **Gate de paridade** desktop↔web antes de remover
- [ ] T005 Remover `desktop/` + `ci-flutter.yml`; ajustar `Makefile`, `.gitignore`, docs
- [ ] T006 Emenda final da constituição + `AGENTS.md` raiz

---

### 8. Segurança & CI (arsenal de controle)

| Brecha | Ferramenta | Onde |
|---|---|---|
| Dependências (SCA) | `osv-scanner` + Dependabot + **Trivy fs** + `npm audit` | `ci-web.yml` + agendado |
| Integridade do lockfile | **lockfile-lint** + `npm ci` + verificação de assinaturas | `ci-web.yml` |
| Código (SAST) | **CodeQL** + **Semgrep** (`p/owasp-top-ten`, `p/typescript`, `p/react`) | `ci-web.yml` |
| Lint anti-XSS | ESLint (`react/no-danger`, `no-eval`, **`no-unsanitized`**) | `web/` |
| Segredos | **gitleaks** + GitHub Secret scanning + push protection | repo |
| Config/IaC | **Trivy config** + hadolint + compose config | `ci-web.yml`/`deploy/` |
| DAST | **OWASP ZAP baseline** (PR) + full agendado | preview deploy |
| Headers/CSP | teste de headers + CSP Evaluator + Trusted Types | `ci-web.yml`/`web/` |
| Supply-chain | **SBOM (Syft)** + `dependency-review` + provenance/SLSA | `ci-web.yml` |
| PWA/perf/a11y | Lighthouse + axe | `ci-web.yml` |
| Citações de requisito | `scripts/check_requirement_ids.py` **estendido a `*.ts/tsx`** | `ci-spec-lint.yml` |
| Processo | threat model + checklist OWASP ASVS + revisão de PR de segurança | `.specs/`/PR |

> Sem estender o `check_requirement_ids.py` para TS/TSX, as citações de requirement IDs no
> web ficam **sem gate** (hoje ele só lê `.go`/`.dart`).

---

### 9. Mapa: pedido → garantia

| Requisito do pedido | Como é garantido |
|---|---|
| Funcionamento **lógico** | Vitest: `mergeCascade`, delta do PATCH, `error_mapper`, exclusividade, drafts (edge-cases primeiro) |
| Requisitos de **interface** | RTL (schema→controle, read-only, toggles, loading/erro/vazio), MSW, axe (a11y), Playwright visual |
| **Auth/CSRF** | cookie httpOnly (atributos), rotação/reuso, CSRF ausente/errado/válido, `Origin` (Go + Vitest) |
| **Dependency scan (SCA)** | osv-scanner + Trivy + Dependabot + `npm audit` (gate) |
| **SAST** | CodeQL + Semgrep |
| **DAST** | OWASP ZAP baseline |
| **PWA** | Lighthouse installability como gate |

---

### 10. Riscos

| Risco | Impacto | Mitigação |
|---|---|---|
| CSRF (novo vetor com cookie httpOnly) | Alto | `SameSite=Strict` + double-submit token + validação de `Origin` (feature backend) |
| XSS ainda age na sessão (access em memória) | Alto | CSP enforce + Trusted Types + SRI + escaping (refresh httpOnly não vaza) |
| Backend precisa mudar antes do web | Alto | `backend-browser-auth-cookies` como pré-requisito bloqueante |
| CORS com credenciais mal configurado | Alto | origem exata, sem `*`, teste de preflight |
| `check_requirement_ids.py` não cobre TS | Médio | estender o script (F1/T010) |
| Portar UI Engine com divergência | Alto | ADs já especificam; testes de paridade por tipo/cascata |
| Deploy sem fallback de rota | Médio | SW + host servem `index.html` (F8/T003) |
| Desktop removido cedo demais | Alto | **gate de paridade** (F10/T004) |

---

### 11. Decisões registradas / pontos abertos

**Decidido:**
- Topologia: app `www.legnaris.com.br` + API `www.api.legnaris.com.br` (cross-origin, **same-site**
  → cookie httpOnly funciona com `SameSite=Strict`; não é third-party cookie).
- Report CSP: **rota no backend** `POST /csp-report` → Loki/Grafana; sem coletor externo por ora.
- CSP emitida pela origem do **frontend**; `connect-src` da API.
- **Rate limiting entra** (`/auth/*` + limitador compartilhado no `/csp-report`).

**Aberto:**
- Fila de PATCH offline (Background Sync) está **fora de escopo** salvo pedido.

---

### 12. Próximos passos

1. Revisar este plano.
2. Trocar para o agente de **build** e rodar, em ordem:
   `/speckit.constitution` → `/speckit.specify` (F1…F10, com `backend-browser-auth-cookies`
   antes de `web-auth-session`) → `/speckit.clarify` → `/speckit.plan` → `/speckit.tasks` →
   `/speckit.analyze`.

---

## Parte B — Detalhamento (autoritativo)

> Documento de planejamento **FINAL**. **Não implementar.** Cada tarefa traz objetivo, passos,
> arquivos, critérios de aceite e testes. Ênfase nas tarefas de **segurança**.
> Data: 2026-10-15. Versão 3 (final).
>
> Changelog: v1 (rascunho) → v2 (auth migrada para **cookie httpOnly** + arsenal de controle
> ampliado) → v3 (topologia app/API definida, **CSP report** e **rate limiting** em escopo,
> referências de tarefas normalizadas, critérios de saída).

---

### 0. Contexto e decisões

**Fatos verificados no código (o TDD interno está desatualizado):**

1. `internal/app/router.go` protege **todas** as rotas `/templates` e `/sheets` com
   `auth.Middleware` — a "API pública" descrita no TDD do desktop **não existe mais**.
2. Refresh hoje por header: `/auth/refresh` e `/auth/logout` leem **`X-Refresh-Token`**;
   `internal/app/auth.go` não usa cookie e há teste que **proíbe** `Set-Cookie` para clientes nativos.
3. `GET /sheets` (lista) e `DELETE /sheets/:id` já existem → a biblioteca local de IDs do
   desktop deixa de ser fonte da verdade; IndexedDB passa a ser **cache offline + drafts**.

**Decisões:**

| Tema | Decisão |
|---|---|
| Stack | React + Vite + TypeScript + `vite-plugin-pwa` |
| Auth | **Cookie httpOnly para o refresh** (browser-native) + access token em memória |
| CSRF | `SameSite=Strict` + **double-submit token** + validação de `Origin` (defesa em profundidade) |
| PWA/offline | Instalável + offline shell + IndexedDB (cache + drafts) |
| Segurança | GitHub-native + `osv-scanner` + **Trivy** + **Semgrep** + OWASP ZAP + SBOM + headers/CSP evaluator |
| Desktop | **Descontinuado** → remover o caminho nativo (`X-Refresh-Token`) do backend |
| Topologia | App `www.legnaris.com.br` + API `www.api.legnaris.com.br` (cross-origin, same-site) |
| CSP report | Rota no backend `POST /csp-report` → Loki/Grafana (sem coletor externo por ora) |
| Rate limiting | **Em escopo** — `/auth/*` + `/csp-report` (limitador compartilhado) |
| Fora de escopo | Fila de escrita offline (Background Sync de PATCH); import/export pela UI |

> **Mudança de premissa (rev. 2):** como o desktop será descontinuado, o modelo de auth do
> browser passa a ser o **único** e o backend vai setar o refresh em **cookie httpOnly**.
> Isso exige mudanças no **backend deste mesmo monorepo** (nova feature `backend-browser-auth-cookies`),
> que passa a ser **pré-requisito bloqueante** de `web-auth-session`.

---

### 1. Arquitetura e stack

| Dimensão | Escolha | Trade-off |
|---|---|---|
| UI/build | React + TS + Vite | Reescreve o UI Engine (porta da lógica já fixada em AD-001…AD-010); ganha PWA/a11y/tooling |
| PWA | `vite-plugin-pwa` (`injectManifest`) | SW custom p/ offline shell + drafts |
| Estado servidor | TanStack Query | cache/invalidation |
| Estado de edição | Zustand + `selectors` (ou Jotai por-path) | preserva AD-001 (1 edição → 1 controle) |
| Rotas | React Router (history) | exige fallback `index.html` no host + SW |
| HTTP | `fetch` + wrapper de envelope, `credentials: 'include'` no auth | cookie httpOnly + Bearer para o resto |
| Local | IndexedDB (`idb`), versionado | cache offline + drafts (nunca tokens) |
| Testes | Vitest + RTL + MSW; Playwright + axe | lógica e requisitos de interface |
| Segurança | SCA/SAST/DAST/secrets/SBOM/headers (ver §4) | gates no CI sem `continue-on-error` |

#### Estrutura de código proposta (`web/`)

```
web/
  AGENTS.md  package.json  vite.config.ts  tsconfig.json  index.html
  public/manifest.webmanifest  public/icons/
  src/
    main.tsx
    app/{router.tsx,providers.tsx}
    api/{client.ts,envelope.ts,error_mapper.ts,auth_api.ts,authenticated_client.ts,csrf.ts,models/*}
    auth/{auth_session.ts,auth_gate.tsx,auth_screen.tsx,session_store.ts}
    catalog/{catalog_screen.tsx,create_sheet_dialog.tsx,queries.ts}
    engine/{field_control.tsx,field_grouping.ts,exclusive_control.tsx,sheet_form.tsx,sheet_defaults.ts,sheet_mark.ts}
    sheet/{sheet_open.ts,sheet_edit.ts,sheet_screen.tsx}
    library/{sheet_library.ts,draft_backup.ts}
    shell/{app_shell.tsx,left_nav.tsx,right_panel.tsx,shortcut_registry.ts}
    theme/{tokens.css,theme.ts}
    pwa/{sw.ts,registerSW.ts}
    l10n/strings.ts
    test/fixtures/*
```

---

### 2. Modelo de ameaças

Com o refresh em **cookie httpOnly**, o roubo de token via XSS deixa de ser trivial
(o JS **não lê** o cookie). Em troca, o cookie é **credencial ambiente** → surge **CSRF**.
O access token permanece em memória (JS lê), então XSS ainda permite **agir em nome do
usuário durante a sessão**, mas **não** obter refresh persistente.

| Ameaça | Vetor | Mitigação | Tarefa |
|---|---|---|---|
| **Exfiltração do refresh** | XSS lê storage | Cookie **httpOnly** (`__Host-`), `Secure`, `SameSite=Strict` | F6B-T001, F7-T002 |
| **CSRF** (novo) | site externo dispara request com cookie | `SameSite=Strict` + double-submit **CSRF token** + validação de `Origin` | F6B-T002, F6B-T003, F7-T003 |
| XSS (ação na sessão) | script injetado chama a API | CSP enforce + Trusted Types + escaping + SRI (XSS não é eliminado, é contido) | F9-T001, F9-T003, F9-T006 |
| Supply chain | dependência/build comprometido | lockfile + osv-scanner + Trivy + Semgrep + CodeQL + SBOM + provenance | F1-T008, F1-T009, F9-T004, F9-T005 |
| Clickjacking | iframe hospeda o app | `frame-ancestors 'none'` + `X-Frame-Options: DENY` | F9-T001 |
| MITM | downgrade p/ http | HTTPS default + HSTS + `Secure` no cookie | F2-T001, F9-T001 |
| Reuso de refresh | token vazado reusado | rotação + detecção de reuso (backend) → revoga família | F6B-T001, F7-T003 |
| Cache poisoning | SW cacheia resposta autenticada | SW não cacheia autenticado nem `/auth/*` | F8-T001 |
| Vazamento em log | token/URL/Referrer | nunca em log/URL; `Referrer-Policy`; redação | F2-T001, F7-T001, F9-T001 |
| Força bruta em auth | tentativa massiva | rate limiting `/auth/*` (em escopo) | F6B-T006 |
| XSS via dado do servidor | `characterName` com `<script>` | React escapa; nunca `dangerouslySetInnerHTML` | F4-T007, F9-T006 |
| Violação de CSP invisível | política barra em silêncio | **CSP report** → Loki/Grafana | F9-T001, F9-T011 |

**Regras invioláveis do projeto web:**
- Proibido `dangerouslySetInnerHTML`, `innerHTML`, `eval`, `new Function`, `document.write` (ESLint).
- Proibido persistir tokens em `localStorage`/`sessionStorage` (o refresh nem chega ao JS).
- Proibido script/asset de terceiros via CDN (tudo self-hosted e versionado).
- CSP entregue por **header HTTP**, em **enforce**.

---

### 3. Modelo de autenticação com cookie httpOnly

#### 3.1 Fluxo

1. `POST /auth/register|login` → backend responde **access token no corpo** e
   **`Set-Cookie: __Host-legnaris_refresh=...`** (httpOnly). O refresh **não** vai no corpo.
2. App guarda o access token **em memória** e o envia como `Authorization: Bearer` em
   `/sheets` e `/templates` (sem cookie nessas rotas → **não** há CSRF nelas).
3. No load/reload e em `401`, o app chama `POST /auth/refresh`
   (`credentials: 'include'`) → o cookie é enviado, o backend **rotaciona** e devolve novo
   access token (+ novo cookie). É o **silent refresh**.
4. `POST /auth/logout` → revoga a família e expira o cookie.

#### 3.2 Atributos do cookie (backend)

```
Set-Cookie: __Host-legnaris_refresh=<opaque>; HttpOnly; Secure; SameSite=Strict; Path=/; Max-Age=<AUTH_REFRESH_TTL>
```
- Prefixo **`__Host-`** (exige `Secure`, `Path=/`, sem `Domain`) → maximiza a proteção.
- Sem `Domain`/`Path` largo: o cookie só vai para o host da API.
- `AUTH_REFRESH_TTL` já existe no `Makefile`/`shared/env`.

#### 3.3 CSRF (controles combinados)

1. **`SameSite=Strict`** no cookie de refresh → requests cross-site não anexam o cookie.
2. **Double-submit token**: no login/refresh, o backend também seta
   `__Host-legnaris_csrf=<random>` (**não** httpOnly, `Secure`, `SameSite=Strict`); o cliente
   lê e reenvia em `X-CSRF-Token`; o backend **valida contra o registro do servidor** (não só
   igualdade cega) em `/auth/refresh` e `/auth/logout`.
3. **Validação de `Origin`/`Referer`** contra allowlist no backend, para métodos state-changing.

> Topologia adotada: **app e API em subdomínios** (cross-origin, same-site) — ver §3.6.
> `SameSite=Strict` já bloqueia CSRF cross-site; o double-submit cobre o caso **same-site
> mas cross-origin** e o de subdomínio comprometido.

#### 3.4 CORS

- `Access-Control-Allow-Origin: <origem exata do web>`, **`Access-Control-Allow-Credentials: true`**,
  `Vary: Origin`. **Nunca** `*` com credenciais.
- Allowlist = `CORS_ALLOWED_ORIGINS` (`shared/env` / `deploy/`).

#### 3.5 Access token e XSS

- Access token em **memória** (nunca persistido).
- Um XSS ainda consegue (a) ler o access token em memória e (b) ler o CSRF cookie e chamar a API
  → **impacto limitado à sessão**; o refresh httpOnly **não vaza** e, ao rotacionar/expirar,
  o atacante perde acesso. **Não** é persistente.

#### 3.6 Topologia (app em `www.legnaris.com.br`, API em `www.api.legnaris.com.br`)

- **Cross-origin, same-site** (registrable domain `legnaris.com.br`) ⇒ ao emitir a CSP, use
  `connect-src https://www.api.legnaris.com.br`; o cookie httpOnly **é enviado** e o
  `SameSite=Strict` bloqueia CSRF **cross-site**.
- **Não é third-party cookie** ⇒ não é bloqueado por ITP/Safari nem pelo Chrome — motivo para
  usar **subdomínio** e não um domínio registrável diferente (que quebraria o silent refresh).
- CORS com credenciais: origem **exata** + `Allow-Credentials: true` + `Vary: Origin` +
  `Access-Control-Allow-Headers: X-CSRF-Token`.
- **CSP é header do documento**: emitida pela origem do **frontend** (Traefik/proxy de
  `www.legnaris.com.br`), **não** pela API (Echo, que só devolve JSON).
- **Endpoint de report CSP**: `POST /csp-report` no backend — sem auth, body cap + rate limit,
  **remover query/hash** da URL antes de logar; log estruturado em **Loki** + métrica no
  **Prometheus** → alerta no **Grafana** (stack já existe em `deploy/`). Coletor externo fica
  como opção futura, não agora. Headers: `report-to` + `Reporting-Endpoints` (novos browsers) e
  `report-uri` (fallback); aceitar `application/csp-report` e `application/reports+json`.

---

### 4. Matriz de controle de segurança (ferramentas para "cobrir brechas")

| Brecha / objetivo | Ferramenta | Onde |
|---|---|---|
| Dependências (SCA) | **osv-scanner** + Dependabot (+ security updates) + **Trivy fs** + `npm audit` | `ci-web.yml` + agendado |
| Integridade do lockfile | **lockfile-lint** + `npm ci` (frozen) + verificação de assinaturas (`npm audit signatures`) | `ci-web.yml` |
| Código (SAST) | **CodeQL** (js/ts) + **Semgrep** (`p/security-audit`, `p/owasp-top-ten`, `p/typescript`, `p/react`) | `ci-web.yml` |
| Lint anti-XSS | ESLint (`react/no-danger`, `no-eval`, `no-new-func`, **`no-unsanitized`**) | `web/` |
| Segredos | **gitleaks** (CI + pre-commit) + GitHub Secret scanning + push protection | repo |
| Config/IaC | **Trivy config** + **hadolint** (Dockerfile) + compose config (já existe) | `ci-web.yml` / `deploy/` |
| DAST | **OWASP ZAP baseline** (PR) + full scan agendado | preview deploy |
| Headers/CSP | teste de headers + **CSP Evaluator** + **Trusted Types** + report-uri | `ci-web.yml` + `web/` |
| Supply chain | **SBOM (Syft)** + **SLSA/provenance** + `dependency-review-action` | `ci-web.yml` |
| Assinatura de imagem | **cosign** (se houver imagem web) | `deploy/` |
| a11y | **axe-core** (Playwright) + Lighthouse | `ci-web.yml` |
| PWA/perf | Lighthouse CI | `ci-web.yml` |
| Processo | threat model + checklist **OWASP ASVS** + revisão de PR de segurança | `.specs/` / PR |
| Rastreabilidade (regras do repo) | `check_requirement_ids.py` estendido a TS/TSX; `check_test_assertions.py` | `scripts/` / `ci-spec-lint.yml` |

---

### 5. Sequência Spec Kit

| # | Comando | Slug | Espelha |
|---|---|---|---|
| 0 | `/speckit.constitution` | — | emenda de stack/PWA/segurança |
| 1 | `/speckit.specify` | `web-pwa-foundation` | Fase 0 |
| 2 | `/speckit.specify` | `web-api-client` | Fase 1 |
| 3 | `/speckit.specify` | `web-catalog` | Fase 2 |
| 4 | `/speckit.specify` | `web-ui-engine` | Fase 3 |
| 5 | `/speckit.specify` | `web-sheet-cycle` | Fase 4 |
| 6 | `/speckit.specify` | **`backend-browser-auth-cookies`** | **novo (backend, pré-requisito)** |
| 7 | `/speckit.specify` | `web-auth-session` | Fase 6 |
| 8 | `/speckit.specify` | `web-pwa-offline` | PWA |
| 9 | `/speckit.specify` | `web-security-hardening` | hardening |
| 10 | `/speckit.specify` | `web-polish-decommission` | Fase 5 + retirada desktop |

Por feature: `/speckit.clarify` → `/speckit.plan` → `/speckit.tasks` → `/speckit.analyze`.

> `backend-browser-auth-cookies` é **bloqueante** para `web-auth-session`.

---

### 6. Tarefas detalhadas por feature

> Formato: `T0xx [P?] [US?]`. `[P]` = paralelizável.

#### F0 — `constitution` (emenda)

**T000 — Emendar a constituição**
- **Objetivo**: trocar a cláusula "Desktop: Flutter under `desktop/`" pelo stack web (React/Vite/TS/PWA)
  e adicionar princípios de PWA, segurança e testes de interface.
- **Passos**: `/speckit.constitution`; seção *Stack* atualizada; novo princípio
  "Security gates are blocking"; *Development Workflow* ganha `make lint-web && make test-web && make sec-scan`.
- **Arquivos**: `.specify/memory/constitution.md`.
- **Aceite**: v2 com Sync Impact Report; nenhuma menção residual a Flutter como stack ativa.

---

#### F1 — `web-pwa-foundation` — US1: "shell carrega e instala"

**F1-T001 Scaffold** `[US1]` — Vite+React+TS; `web/.gitignore`; Node pinada; `npm ci && npm run build`.
**F1-T002 Lint/formatação** `[P]` `[US1]` — ESLint (typescript-eslint, jsx-a11y, **no-unsanitized**,
**eslint-plugin-security**) + Prettier; regras anti-XSS (ver F9-T006).
**F1-T003 Tokens de design** `[P]` `[US1]` — `theme/tokens.css`/`theme.ts` portados do desktop;
sem valores mágicos; contraste AA.
**F1-T004 `web/AGENTS.md`** `[US1]` — regras visuais (densidade/resize/foco/hover; veto "AI SaaS").
**F1-T005 Shell** `[US1]` — `app_shell/left_nav/right_panel`; ordem nav|conteúdo|painel; colapso do painel primeiro.
**F1-T006 Config do PWA** `[P]` `[US1]` — `VitePWA({ strategies:'injectManifest', injectRegister:false })`.
  - **Segurança**: registro do SW **externo** → permite `script-src 'self'` sem `unsafe-inline`.
**F1-T007 Manifest/ícones** `[US1]` — manifest válido; Lighthouse installable.
**F1-T008 CI `ci-web.yml`** `[US1]` — **SEGURANÇA**
  - `paths: ['web/**', '.github/workflows/ci-web.yml']`; `permissions: contents: read`;
    CodeQL job com `security-events: write`; `concurrency`.
  - Jobs: `quality` (lint+tsc+vitest+build); **`sca`** (osv-scanner + Trivy fs + `npm audit`);
    **`sast`** (CodeQL + Semgrep); **`secrets`** (gitleaks); **`dependency-review`**; **`lockfile-lint`**.
  - **Sem** `continue-on-error` nos gates de segurança.
**F1-T009 Dependabot** `[P]` `[US1]` — `npm /web` semanal + security updates.
**F1-T010 Path filters/Makefile/spec-lint** `[US1]` — `ci-lint.yml` ignora `web/**`;
  `Makefile` ganha `lint-web/test-web/sec-scan`; **`check_requirement_ids.py` cobre `*.ts/*.tsx`**;
  `check_test_assertions.py` cobre testes TS; `ci-spec-lint.yml` inclui `web/**`.
**F1-T011 Smoke do shell** `[US1]` — Vitest+RTL.

---

#### F2 — `web-api-client` — US1: "consumo tipado da API"

**F2-T001 HTTP client** `[US1]` — **SEGURANÇA**
  - base URL **https por default**; `http://` só dev (aviso visual); **retry só GET**; timeout/abort;
    nunca logar token/corpo.
**F2-T002 Envelope/models** `[P]` `[US1]` — parse defensivo; `data` dinâmico sem `eval`.
**F2-T003 `error_mapper.ts`** `[US1]` — espelha `error_mapper.dart`; mensagens como **texto**;
  fallback genérico; sem vazar stack/URLs.
**F2-T004 Testes de contrato** `[US1]` — fixtures (envelope, delta "só cascata").
**F2-T005 Testes de erro/edge** `[P]` `[US1]`.

---

#### F3 — `web-catalog` — US: "listar e abrir templates"

**F3-T001 Queries TanStack** `[US]` — cache `staleTime`.
**F3-T002 `catalog_screen.tsx`** `[US]` — loading/vazio/erro distintos (strings ancoradas).
**F3-T003 `create_sheet_dialog.tsx`** `[P]` `[US]` — nome obrigatório (trim; whitespace rejeitado).
**F3-T004 Testes de interface** `[US]` — erro inline posicionado junto ao dado.

---

#### F4 — `web-ui-engine` (núcleo; porta de `desktop/lib/engine/`)

**F4-T001 `field_grouping.ts`** `[US]` — agrupamento por prefixo.
**F4-T002 `field_control.tsx`** `[P]` `[US]` — numérico/texto/read-only; **sem HTML**.
**F4-T003 `exclusive_control.tsx`** `[US]` — grupos exclusivos (AD-002).
**F4-T004 `sheet_defaults.ts`** `[US]` — `seedFromSheet` + `mergeCascade`.
**F4-T005 `sheet_mark.ts`** `[US]` — Modified/Recalculated/Calculated (AD-009).
**F4-T006 `sheet_form.tsx`** `[US]` — seams control/rowMark; granularidade AD-001.
**F4-T007 Testes de interface + XSS de dado** `[US]` — **SEGURANÇA**: `characterName = "<script>"` renderiza **como texto**.

---

#### F5 — `web-sheet-cycle` — US: "criar/abrir/editar com cascata"

**F5-T001 `sheet_open.ts`** `[US]` — GET por id; `NOT_FOUND` remove do cache local.
**F5-T002 `sheet_edit.ts`** `[US]` — dirty + delta + `save()` PATCH "só delta" + `mergeCascade`.
**F5-T003 `library/sheet_library.ts`** `[P]` `[US]` — IndexedDB versionado; **não** guardar tokens;
  `JSON.parse` defensivo.
**F5-T004 `draft_backup.ts`** `[US]` — drafts sem derivados nem tokens; cadência injetável.
**F5-T005 Footer/Save + atalhos** `[US]` — `Ctrl+S`/`F5`.
**F5-T006 Testes lógicos** `[US]` — cascata/delta/idempotência (edge-cases primeiro).
**F5-T007 Testes de interface** `[US]`.

---

#### F6 (backend) — `backend-browser-auth-cookies` — **SEGURANÇA / pré-requisito**

> Alterações em `internal/auth` e `internal/app/auth.go` + `cors`.

**F6B-T001 Refresh em cookie httpOnly**
- **Objetivo**: `login/register/refresh` passam a setar `__Host-legnaris_refresh` (HttpOnly,
  Secure, SameSite=Strict, Path=/); `refresh/logout` leem **do cookie**; o refresh **deixa de ir no corpo**.
- **Arquivos**: `internal/app/auth.go`, `internal/auth/service.go` (se preciso), `shared/env`.
- **Aceite**: `Set-Cookie` presente com os atributos; corpo sem `refresh_token`.
- **Testes** (padrão do repo): presença/atributos do cookie; ausência no corpo; rotação;
  cookie expirado no logout; `NOT_FOUND`/rotação inválida → 401.

**F6B-T002 CSRF token (double-submit + server-bound)**
- **Objetivo**: emitir `__Host-legnaris_csrf` (não httpOnly) no login/refresh; validar
  `X-CSRF-Token` contra o registro do servidor em `/auth/refresh` e `/auth/logout`.
- **Aceite**: refresh sem token/sem correspondência → **403**; com token válido → 200.
- **Testes**: ausente; incorreto; válido; expirado.

**F6B-T003 Validação de `Origin`/`Referer`**
- **Objetivo**: rejeitar state-changing com origem fora da allowlist.
- **Aceite**: origem inválida → 403.
- **Testes**: origem ausente/inválida/válida.

**F6B-T004 CORS com credenciais**
- **Objetivo**: `Access-Control-Allow-Credentials: true` + origem exata (sem `*`); `Vary: Origin`.
- **Arquivos**: `internal/platform/cors`, `shared/env`.
- **Testes**: preflight; origem fora da lista rejeitada; sem wildcard com credenciais.

**F6B-T005 Remover o caminho nativo (`X-Refresh-Token`)**
- **Objetivo**: como o desktop será descontinuado, remover leitura de `X-Refresh-Token` e o teste
  que proíbe `Set-Cookie`; atualizar docs.
- **Aceite**: grep sem `X-Refresh-Token`; suíte verde.

**F6B-T006 Rate limiting em `/auth/*`** — **entra no escopo**
- **Objetivo**: limiter por IP **e** por conta em `login`/`register`/`refresh` (e compartilhado com
  `POST /csp-report`, F9-T011).
- **Passos**: middleware de rate limit (janela deslizante ou token bucket) com chave = IP + identidade
  quando houver; resposta `429` com `Retry-After`; métrica de bloqueios.
- **Aceite**: excesso → 429 (com `Retry-After`); janela testada.
- **Testes**: dentro/fora do limite; janela expira; chaves distintas isoladas; `429` mapeado.

---

#### F7 — `web-auth-session` — US: "login/registro/logout"

**F7-T001 `auth_api.ts`** `[US]` — login/register/refresh/logout com `credentials:'include'`.
  - **Segurança**: nunca logar; só HTTPS.
**F7-T002 `session_store.ts`** `[US]` — **access em memória**; **livre de persistência de token**
  (o refresh é cookie httpOnly, invisível ao JS).
  - **Aceite**: nenhum token em storage; reload = silent refresh.
**F7-T003 `authenticated_client.ts`** `[US]` — **SEGURANÇA (crítico)**
  - `401` → **silent refresh** single-flight (uma vez por falha; concorrentes compartilham);
    checar **versão de sessão** (AD-010) antes de aplicar; refresh inválido → logout forçado.
  - Envia **`X-CSRF-Token`** (lido do cookie não-httpOnly) nas chamadas de auth.
  - **CSRF**: `/sheets` e `/templates` usam Bearer (sem cookie) → não são alvo;
    `/auth/refresh|logout` são protegidos por SameSite+CSRF+Origin.
- **Testes**: 401 concorrente compartilha refresh; loop evitado; reuso ⇒ logout + purga;
  refresh sem CSRF → falha mapeada.
**F7-T004 `auth_gate.tsx` + `auth_screen.tsx`** `[US]` — password input correto; mensagens sem leak.
**F7-T005 Boundary de sessão** `[US]` — troca de conta purga IndexedDB (cache/drafts) — AD-010.
**F7-T006 CRUD de fichas do usuário** `[US]` — `GET /sheets` + `DELETE /sheets/:id` com confirmação.

---

#### F8 — `web-pwa-offline` — US: "instalar e usar offline"

**F8-T001 SW (`injectManifest`)** `[US]` — precache do shell + runtime cache GET;
  **não** cacheia autenticado nem `/auth/*`; limpa caches antigos no `activate`.
**F8-T002 Registro + update UX** `[US]` — `registerSW.ts` externo; `registerType:'prompt'`.
**F8-T003 Fallback de rotas deep-link** `[US]`.
**F8-T004 Offline de leitura + drafts** `[US]` — fila de escrita **fora de escopo**.
**F8-T005 Gate Lighthouse** `[US]` — installability/perf/a11y.

---

#### F9 — `web-security-hardening` — **FEATURE INTEIRA DE SEGURANÇA**

**F9-T001 CSP + headers (header HTTP, enforce)** — **SEGURANÇA (principal)**
  ```
  default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self';
  connect-src 'self' https://www.api.legnaris.com.br; object-src 'none'; base-uri 'self'; form-action 'self';
  frame-ancestors 'none'; require-trusted-types-for 'script';
  report-uri https://www.api.legnaris.com.br/csp-report
  ```
  - Emitida pela origem do **frontend** (Traefik/proxy de `www.legnaris.com.br`) — **não** pela API
    (Echo só devolve JSON); `report-only → enforce`; `connect-src https://www.api.legnaris.com.br`.
  - Complementos: `HSTS`, `nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`,
    `Permissions-Policy`, `X-Frame-Options: DENY`, `COOP: same-origin`.
  - **Aceite**: header em prod; nenhuma violação no fluxo; virada registrada (F9-T008).
**F9-T002 CORS/credenciais** — fecha F6B-T004; allowlist exata; sem `*` com credenciais.
**F9-T003 Trusted Types** — política default; sinks DOM; teste de sink bloqueado.
**F9-T004 DAST (ZAP)** — `zap-baseline` no preview + `.zap/rules.tsv` + artefato; **falha em novo alerta**.
**F9-T005 SBOM + supply chain** — Syft + `dependency-review` + verificação de assinaturas + provenance.
**F9-T006 Lint anti-XSS + teste de escaping** — ESLint (`react/no-danger`, `no-eval`, `no-new-func`,
  `no-unsanitized`) + teste `<script>` renderizado como texto; HTML dinâmico só via DOMPurify.
**F9-T007 a11y (axe)** `[P]` — fluxos principais; zero violações críticas.
**F9-T008 Report-only → enforce (saída)** — registrar violações coletadas e a virada; artefato do relatório.
**F9-T009 Semgrep ruleset dedicado** `[P]` — `p/owasp-top-ten` + regras custom (proibir sinks, exigir CSRF header); a F1 já subiu `p/default` no gate (único pack que detectou um sink semeado — ver `.specs/features/001-web-pwa-foundation/contracts/ci-gates.contract.md`), então aqui faltam as regras custom.
**F9-T010 Headers test** `[P]` — teste que falha se CSP/HSTS/nosniff/Referrer sumirem.
**F9-T011 Rota de report CSP (backend)** — `POST /csp-report` sem auth, body cap + rate limit,
  aceita `application/csp-report` e `application/reports+json`, **remove query/hash** da URL,
  emite log estruturado (Loki) + métrica (Prometheus) → alerta (Grafana); `Reporting-Endpoints`
  + `report-to` no header do frontend e `report-uri` como fallback.

---

#### F10 — `web-polish-decommission` — US: "paridade e retirada do desktop"

**F10-T001 Golden/visual** (Playwright). **F10-T002 Contrato/integração** (self-skip).
**F10-T003 Cobertura + `web/README.md`/`quickstart.md`.** **F10-T004 Gate de paridade.**
**F10-T005 Remover `desktop/` + `ci-flutter.yml`**; ajustar `Makefile`, `.gitignore`, docs, `AGENTS.md`.
**F10-T006 Emenda final da constituição.**

---

### 7. Estratégia de testes

| Tipo | Escopo | Ferramenta |
|---|---|---|
| Unit/lógico | `mergeCascade`, delta, `error_mapper`, exclusividade, drafts | Vitest |
| Interface | schema→controle, read-only, toggles, estados, marcas | Vitest + RTL + MSW |
| **Auth/CSRF** | cookie httpOnly (presença/atributos), rotação, CSRF ausente/errado/válido, Origin | Go (`internal/app`) + Vitest |
| Segurança | escaping (XSS de dado), proibições de sink, headers/CSP, tokens fora de storage | Vitest + integração |
| Contrato | PATCH "só delta", envelope, `apperr` | fixtures + API real (self-skip) |
| E2E | criar→editar→salvar; login/logout; silent refresh; offline | Playwright |
| a11y | fluxos principais | axe + Playwright |
| Visual | template de referência | Playwright screenshots |
| PWA/perf | installability/perf | Lighthouse CI |
| SCA/SAST/DAST/secrets | deps/código/runtime/segredos | osv-scanner, Trivy, CodeQL, Semgrep, gitleaks, ZAP |

**Aceite transversal:** nenhum gate de segurança usa `continue-on-error`; regressão de segurança,
citação de requirement ID ou assertiva vazia **quebra** o CI.

---

### 8. Governança e descontinuação

- Emenda da constituição (F0-T000) + `check_requirement_ids.py` cobrindo TS (F1-T010).
- Remover o caminho nativo `X-Refresh-Token` (F6B-T005) — viabilizado pela retirada do desktop.
- Retirada do `desktop/` (F10-T005) **somente** após o gateway de paridade (F10-T004).

---

### 9. Riscos

| Risco | Mitigação |
|---|---|
| CSRF (novo vetor com cookie) | SameSite=Strict + double-submit + Origin + teste dedicado |
| XSS ainda age na sessão | CSP enforce + Trusted Types + SRI + escaping (refresh httpOnly mitiga exfiltração) |
| Backend precisa mudar antes do web | `backend-browser-auth-cookies` como pré-requisito bloqueante (F6) |
| Config CORS com credenciais | sem `*`; allowlist exata; teste de preflight |
| `injectRegister` inline quebra CSP | registro do SW externo (F1-T006) |
| Divergência ao portar o UI Engine | ADs como fonte; testes de paridade |
| Remover desktop cedo | gate de paridade |

---

### 10. Decisões registradas / pontos abertos

**Decidido:**
- Topologia: app em `www.legnaris.com.br`, API em `www.api.legnaris.com.br` (cross-origin, same-site).
- Endpoint de report CSP: **rota no backend** (`POST /csp-report`) → Loki/Grafana; sem coletor externo por ora.
- CSP emitida pela origem do **frontend**; `connect-src` da API.
- **Rate limiting entra** no escopo → F6B-T006 (`/auth/*`) e limitador **compartilhado** também em
  `POST /csp-report` (F9-T011).

**Aberto:**
- Fila de escrita offline (Background Sync de PATCH): **fora de escopo** por padrão.

---

### 11. Definition of Done (critérios de saída do plano)

Gates que marcam o web app como pronto e destravam a retirada do desktop:

- [ ] **Paridade funcional** com o desktop (F10-T004): catálogo, fichas, cascata, preview, auth.
- [ ] Todos os **gates de segurança verdes e sem `continue-on-error`** (F1-T008, F9).
- [ ] **CSP em `enforce`** (não report-only), com as violações coletadas registradas (F9-T008, F9-T011).
- [ ] **Cookie httpOnly + CSRF** validados por teste (F6B-T001/T002/T003).
- [ ] **Rate limiting** ativo em `/auth/*` e `/csp-report` (F6B-T006, F9-T011).
- [ ] Cobertura ≥ meta; testes de **lógica** + **interface** + a11y + visual.
- [ ] `check_requirement_ids.py` cobrindo **TS/TSX**; `ci-spec-lint` verde (F1-T010).
- [ ] `desktop/` removido; CI/Makefile/AGENTS/docs atualizados (F10-T005/T006).

---

### 12. Próximos passos (execução via Spec Kit)

1. Revisar/validar este plano.
2. Trocar para o agente de **build** e rodar: `/speckit.constitution` → `/speckit.specify`
   (F1…F10, com `backend-browser-auth-cookies` antes de `web-auth-session`) →
   `/speckit.clarify` → `/speckit.plan` → `/speckit.tasks` → `/speckit.analyze`.
3. Implementar seguindo `tasks.md` (fora do escopo deste documento).

---

### Apêndice A — CSP report: o que é e implementação escolhida

#### A.1 O que é

A **CSP** (`Content-Security-Policy`) é um header que diz ao browser **de onde** pode carregar/
executar script, estilo, conexão, frame etc. Quando algo viola a política, o browser pode
**enviar um "report"** (JSON via POST) para um endpoint declarado — é o **CSP report**.

Serve para dois momentos:

1. **Rollout** — com `Content-Security-Policy-Report-Only`, o browser **não bloqueia**, só **reporta**
   o que *seria* bloqueado. Coleta-se a realidade antes de ligar o enforce.
2. **Produção** — com a CSP em **enforce**, os reports viram **detecção**: scripts injetados
   (XSS), dependências que mudaram de origem, gaps de política.

#### A.2 Como declarar o endpoint

```
# moderno (Reporting API)
Reporting-Endpoints: csp-endpoint="https://www.api.legnaris.com.br/csp-report"
Content-Security-Policy: ...; report-to csp-endpoint

# legado (fallback para browsers antigos)
Content-Security-Policy: ...; report-uri https://www.api.legnaris.com.br/csp-report

# fase de coleta (não bloqueia)
Content-Security-Policy-Report-Only: ...; report-uri https://www.api.legnaris.com.br/csp-report
```

#### A.3 Payloads (dois formatos → normalizar)

`report-uri` → `Content-Type: application/csp-report`:
```json
{ "csp-report": {
  "document-uri": "https://www.legnaris.com.br/sheet/abc",
  "violated-directive": "script-src-elem",
  "effective-directive": "script-src-elem",
  "original-policy": "default-src 'self'; ...",
  "disposition": "enforce",
  "blocked-uri": "https://evil.example.com/x.js",
  "source-file": "https://www.legnaris.com.br/assets/index-abc.js",
  "line-number": 12, "column-number": 345, "status-code": 200
}}
```

`report-to` → `Content-Type: application/reports+json` (array):
```json
[{ "type": "csp-violation", "url": "https://www.legnaris.com.br/...", "age": 123,
   "user_agent": "...", "body": {
     "documentURL": "...", "blockedURL": "...", "effectiveDirective": "script-src-elem",
     "originalPolicy": "...", "disposition": "enforce", "lineNumber": 12, "columnNumber": 345 }}
]
```

#### A.4 Cuidados

- Reports são **fire-and-forget**, enviados **sem credenciais/cookies** → não correlacionam usuário
  (bom p/ privacidade) e a página não os lê.
- **Ruído**: extensões de browser geram violações → espere falsos positivos; dedup é essencial.
- **PII**: `document-uri`/`blocked-url` podem conter **query string** → **remover** antes de gravar.
- **Sempre responder 2xx/204** e rápido; nunca devolver erro (o browser ignora e só gera ruído).

#### A.5 Implementação escolhida (backend)

Rota **`POST /csp-report`** no backend (Go/Echo), **não autenticada**:

1. aceitar `application/csp-report`, `application/reports+json` (e tolerar `application/json`);
2. **cap de corpo** (`http.MaxBytesReader`, ~16–64 KB);
3. **rate limit** (limitador compartilhado com `/auth/*`);
4. **normalizar** os dois formatos para um struct único;
5. **sanitizar** `document-uri`/`blocked-uri` (remover query/fragment);
6. **log estruturado** (`shared/logger`) → **Loki**;
7. **contador Prometheus** com labels (`effective-directive`, `disposition`) → alerta no **Grafana**;
8. responder **204**.

**Rollout**: `Report-Only` no frontend → coletar ~1 semana → corrigir → **enforce** (critério F9-T008).

**Por que não coletor externo**: o stack **Loki + Prometheus/Grafana + Alloy** já existe em `deploy/`;
um vendor seria custo/privacidade/dependência extras sem ganho. Fica como opção futura se o volume
ou a curadoria crescer.
