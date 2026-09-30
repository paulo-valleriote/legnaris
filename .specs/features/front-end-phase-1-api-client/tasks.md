# [Front End] Fase 1 — API Client: cliente tipado, envelope e mapeamento apperr — Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: none (TDD fixou a arquitetura — API Client isolado, models, estado nas fases seguintes; decisões restantes estão nas premissas aprovadas da `spec.md`)
**Status**: Approved — 2026-09-29 · 9 tasks em 4 fases, execução em 2 batches de sub-agents (batch 1 = Fases 1–3, batch 2 = Fase 4), tools: context7 só em T3/T6

---

## Test Coverage Matrix

> Gerada do codebase, guidelines do projeto e spec — confirmar antes do Execute. Guidelines encontrados: `AGENTS.md` (raiz — testes obrigatórios, edge cases primeiro, tabela-driven, inglês, testes junto ao código), `docs/TDD-desktop-mvp.md` §Estratégia de Testes (unit >80% camadas não-widget, fixtures fixadas do contrato real, contrato self-skip sem API), `.github/workflows/ci-flutter.yml` (analyze → test → build windows), `desktop/analysis_options.yaml` (`flutter_lints`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Config de URL base (`desktop/lib/api/api_config.dart`) | unit | 1:1 com FE1-25…27; edges: override vazio, sem esquema, `http://` explícito | `desktop/test/api/*_test.dart` | `cd desktop && flutter test` |
| Decode de contrato — envelope + erro de domínio | unit | 1:1 com FE1-02…04, FE1-22, FE1-24; todos os branches (sucesso, erro, parse defensivo) | `desktop/test/api/*_test.dart` | `cd desktop && flutter test` |
| Mapeamento `apperr` + camada de localização | unit | 1:1 com FE1-15…21, FE1-23; tabela inteira de códigos, fallback genérico, copy via l10n (`en`) | `desktop/test/api/*_test.dart` | `cd desktop && flutter test` |
| Models (`template.dart`, `sheet.dart`) + path pontilhado | unit | 1:1 com FE1-08…14; edges int/float, campo ausente, intermediate não-mapa, segmento vazio | `desktop/test/api/*_test.dart` | `cd desktop && flutter test` |
| API client (endpoints, transporte, timeout/retry) | unit (transporte mockado) | 1:1 com FE1-01, 05…07, FE1-20, 28…31; rota/método/corpo por endpoint + contagem de chamadas no retry | `desktop/test/api/*_test.dart` | `cd desktop && flutter test` |
| Fixtures do contrato (pinned, sem serviço) | unit | 1:1 com FE1-32…34; roda sem API alcançável; drift de fixture quebra a suite | `desktop/test/**/*_test.dart` | `cd desktop && flutter test` |
| Contrato ao vivo (API real) | integration (self-skip) | 1:1 com FE1-35…37; 4 cenários do TDD; skip sem base → exit 0; falha nunca silenciosa | `desktop/test/contract/*_test.dart` | `cd desktop && flutter test` (self-skip sem API) |
| UI / estado Riverpod | none | fora desta fase — Fases 2–4 (spec §Out of Scope) | - | - |
| Specs (`.specs/`) | none | validators exit 0 | `.specs/**` | `validate_spec.py` / `validate_tasks.py` |

## Gate Check Commands

> Geradas do codebase — confirmar antes do Execute.

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | tasks só com testes unit já escritos | `cd desktop && flutter test` |
| Full | toda task que adiciona/muda código Dart (padrão do repo: analyze limpo antes de commit) | `cd desktop && flutter analyze && flutter test` |
| Build | fim de fase e tasks que tocam `pubspec.yaml`/codegen (T3, T6) | `cd desktop && flutter analyze && flutter test` + o PR valida no CI Flutter (`windows-latest`: analyze → test → build windows → artifact) — **build Windows é impossível em dev Linux, é exclusivo do CI** |

---

## Execution Plan

Phases são ordenadas e rodam em sequência — cada phase completa antes da próxima; tasks dentro da phase rodam em ordem.

### Phase 1: Fundação do contrato

Configuração de URL base, decode de envelope e mapeamento de erros — a base que o client usa.

```
T1
T2 → T3
```

### Phase 2: Models

Models fiéis ao contrato, com `data` dinâmico e acesso pontilhado.

```
T4
T5
```

### Phase 3: API client

Client tipado sobre o transporte injetável, depois a política de timeout/retry.

```
T6 → T7
```

### Phase 4: Contrato

Fixtures reais fixadas no CI e suite de contrato ao vivo self-skip.

```
T8 → T9
```

---

## Task Breakdown

### Phase 1: Fundação do contrato

#### T1: Configuração de URL base (default + override de dev)

**What**: Criar a resolução da base URL: default compilado `https://api.legnaris.com.br`, override via `--dart-define=API_BASE_URL` em qualquer modo de build, e validação que falha antes de qualquer requisição quando a base não é URL absoluta `http(s)`.
**Where**: `desktop/lib/api/api_config.dart`
**Depends on**: None
**Reuses**: Padrão `String.fromEnvironment` do Flutter; hostname de `deploy/cloudflared/config.yml`
**Requirement**: FE1-25, FE1-26, FE1-27

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [x] Sem flag, a base é `https://api.legnaris.com.br` (FE1-25)
- [x] Com `API_BASE_URL` definida, ela prevalece em qualquer build mode (FE1-26)
- [x] Override vazio cai no default; base não-absolute (`api:8080`, `ftp://x`, sem esquema) lança erro de configuração **antes** de qualquer requisição (FE1-27 + edge da spec)
- [x] `http://` só entra por override explícito (default é https — TLS obrigatório)
- [x] Gate check passa: `cd desktop && flutter analyze && flutter test`
- [x] Test count: ≥ 5 testes passando (default, override https, override http, vazio→default, malformado→erro) — 9 testes

**Tests**: unit
**Gate**: full — `cd desktop && flutter analyze && flutter test`
**Commit**: `feat(LGN-90): add base url config with dart-define override`

---

#### T2: Decode de envelope + erro de domínio

**What**: Criar o decodificador único de envelope — 2xx + `{"data"}` → payload tipado; 4xx/5xx + `{"error":{code,message}}` → erro de domínio carregando status/code/message; JSON inválido, 2xx sem `data` e 4xx/5xx sem `error.code` → erro de parse defensivo **distinto** do erro de domínio.
**Where**: `desktop/lib/api/`
**Depends on**: None
**Reuses**: Forma do envelope de `shared/response/response.go`; códigos de `shared/apperr/err.go`
**Requirement**: FE1-02, FE1-03, FE1-04, FE1-22, FE1-24

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [x] 2xx com `data` → payload decodificado no tipo pedido pelo chamador (FE1-02)
- [x] 4xx/5xx com `{"error":{code,message}}` → erro de domínio com `code` e `message` preservados (FE1-03)
- [x] 2xx sem chave `data`, JSON inválido ou corpo vazio → erro de parse defensivo, nunca sucesso vazio (FE1-04)
- [x] 4xx/5xx sem `error.code` (ou corpo que não é o envelope de erro) → erro de parse defensivo, **não** um erro de domínio mapeado (FE1-22)
- [x] Erro de domínio carrega status HTTP (quando recebido), código `apperr` (quando presente) e message do servidor (FE1-24)
- [x] Gate check passa: `cd desktop && flutter analyze && flutter test`
- [x] Test count: ≥ 7 testes (sucesso, envelope de erro, sem data, JSON quebrado, corpo vazio, sem error.code, campos do erro) — 10 testes

**Tests**: unit
**Gate**: full — `cd desktop && flutter analyze && flutter test`
**Commit**: `feat(LGN-90): decode response envelope into typed payload or domain error`

---

#### T3: Mapeamento `apperr` → kind de erro da UI + copy localizada

**What**: Criar o mapper `apperr` → kind de erro de domínio (`clientFault`, `invalidEdit`, `renderMismatch`, `notFound`, `serverError`, `offline`, `unknown`) com fallback genérico, e a camada de localização com `en` como única locale resolvendo a copy de cada kind — zero literais de UI no client.
**Where**: `desktop/lib/api/error_mapper.dart`
**Depends on**: T2
**Reuses**: Tabela `apperr` → tratamento da spec §Story "Mapeamento"; estrutura de erro de T2
**Requirement**: FE1-15, FE1-16, FE1-17, FE1-18, FE1-19, FE1-20, FE1-21, FE1-23

**Tools**:

- MCP: `context7` (confirmar configuração atual de `flutter gen-l10n`/`l10n.yaml` para o Flutter pinado 3.47.5 — synthetic package foi descontinuado; confirmar `output-dir` e `generate: true`)
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [x] Cada código mapeia ao kind da tabela: `INVALID_JSON`→`clientFault`, `INVALID_REQUEST`→`invalidEdit`, `UNKNOWN_FIELD`→`renderMismatch`, `NOT_FOUND`→`notFound`, `INTERNAL_ERROR`→`serverError` (FE1-15…19)
- [x] Falha de transporte esgotada → `offline` (FE1-20 — a classificação em si é ligada no T7; aqui o kind e a copy existem)
- [x] Código fora da tabela (`CONFLICT`, `CYCLE_DETECTED`, `UNAUTHORIZED`…) → kind `unknown` com fallback genérico, preservando código cru e message no objeto de erro (FE1-21)
- [x] Camada de localização com `en` como única locale resolve toda a copy por kind — nenhum literal de UI hardcoded (FE1-23)
- [x] Gate check passa: `cd desktop && flutter analyze && flutter test`
- [x] Test count: ≥ 10 testes (5 códigos + offline + fallback + 2 formatos de copy + ausência de literal) — 11 testes

**Tests**: unit
**Gate**: build — `cd desktop && flutter analyze && flutter test` (T3 toca pubspec/codegen de l10n)
**Commit**: `feat(LGN-90): map apperr codes to ui error kinds with english copy`

### Phase 2: Models

#### T4: Models de template (`TemplateCatalog`/`TemplateDetail`/`FieldSchema`)

**What**: Criar os models de template decodificando `id`, `name`, `default`, `fields` com a árvore folha/grupo (`schema`/`children`) e o schema de campo completo, com `formula`/`depends_on` opcionais.
**Where**: `desktop/lib/api/models/template.dart`
**Depends on**: None
**Reuses**: Shapes de `internal/sheet/template/dto.go` (`TemplateDTO`, `FieldNodeDTO`, `FieldSchemaDTO`)
**Requirement**: FE1-08, FE1-09

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [ ] Template decodifica `id`, `name`, `default` e `fields` preservando folha (`schema`) vs grupo (`children`) (FE1-08)
- [ ] Schema folha decodifica `type`, `label`, `formula`, `depends_on`, `is_editable`, `default`, com `formula`/`depends_on` ausentes tratados (FE1-09)
- [ ] Key JSON `default` (palavra reservada em Dart) mapeada explicitamente, sem depender de case convention (edge da spec)
- [ ] `type` mantido como string opaca — sem mapear para widgets (Fase 3)
- [ ] Gate check passa: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: ≥ 5 testes (lista, detalhe, folha completa, schema sem opcionais, árvore aninhada)

**Tests**: unit
**Gate**: full — `cd desktop && flutter analyze && flutter test`
**Commit**: `feat(LGN-90): add template and field schema models`

---

#### T5: Model de `Sheet` + acesso por caminho pontilhado

**What**: Criar o model de ficha com `data` dinâmico preservando aninhamento e o accessor pontilhado: resolve `stats.dexterity`, retorna null para ausente/intermediate não-mapa, lança erro tipado para path/segmento vazio.
**Where**: `desktop/lib/api/models/sheet.dart`
**Depends on**: None
**Reuses**: Shape de `internal/sheet/dto.go` (`SheetDTO`); decisões de path da spec (premissa confirmada)
**Requirement**: FE1-10, FE1-11, FE1-12, FE1-13, FE1-14

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [ ] Sheet decodifica `id`, `template_id`, `name`, `data`, `created_at`, `updated_at` com `data` aninhado dinâmico (FE1-10)
- [ ] `16` permanece `int` e `16.5` permanece `double` — sem truncamento nem cast forçado (FE1-11)
- [ ] Path válido resolve (`stats.dexterity`) (FE1-12)
- [ ] Segmento ausente ou intermediate não-mapa → `null`, nunca lança (FE1-13)
- [ ] Path vazio ou segmento vazio (`stats..dexterity`, `""`) → erro tipado de client-side (FE1-14)
- [ ] Gate check passa: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: ≥ 8 testes (decode, int, float, path válido, ausente, intermediate não-mapa, segmento vazio, path vazio)

**Tests**: unit
**Gate**: full — `cd desktop && flutter analyze && flutter test`
**Commit**: `feat(LGN-90): add sheet model with dotted path access`

### Phase 3: API client

#### T6: Client tipado com transporte injetável (6 endpoints)

**What**: Criar o client com um método tipado por endpoint (`health`, `listTemplates`, `getTemplate`, `createSheet`, `getSheet`, `patchSheet`), recebendo `http.Client` e a config na construção — nenhum método cria o próprio transporte — e ligando decode de envelope + mapper.
**Where**: `desktop/lib/api/api_client.dart`
**Depends on**: T1, T2, T3, T4, T5
**Reuses**: `package:http` (`http.Client`); config de T1; envelope de T2; mapper de T3; models de T4/T5
**Requirement**: FE1-01, FE1-05, FE1-06, FE1-07

**Tools**:

- MCP: `context7` (conferir API atual de `package:http` — `MockClient`, `BaseRequest`)
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [ ] 6 métodos tipados, um por endpoint consumido, cada um no seu tipo de retorno (FE1-01)
- [ ] `POST /sheets` envia `{"template_id": ..., "sheet_name": ...}` (FE1-05)
- [ ] `PATCH /sheets/:id` envia `{"data": {...}}` só com o delta informado (FE1-06)
- [ ] Transporte injetado na construção; nenhum método cria o próprio `http.Client` (FE1-07)
- [ ] Respostas passam por envelope (FE1-02/03) e erros pelo mapper (FE1-15…21) — wiring testado por endpoint
- [ ] Gate check passa: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: ≥ 12 testes (rota/método/corpo por endpoint + decode + propagação de erro)

**Tests**: unit
**Gate**: build — `cd desktop && flutter analyze && flutter test` (T6 pode tocar pubspec de `package:http`)
**Commit**: `feat(LGN-90): add typed api client for consumed endpoints`

---

#### T7: Timeout fail-fast + retry idempotente + classificação offline

**What**: Aplicar a política de timeout/retry ao client: 5s por tentativa; GET com exatamente 1 retry (300ms) só em falha de transporte; GET 4xx/5xx sem retry; POST/PATCH sempre 1 tentativa; transporte esgotado → erro com kind `offline`.
**Where**: `desktop/lib/api/api_client.dart`
**Depends on**: T6
**Reuses**: Cliente de T6; kind `offline` do T3
**Requirement**: FE1-28, FE1-29, FE1-30, FE1-31, FE1-20

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [ ] Timeout de 5s por tentativa é o default (constante testada) e é aplicado a cada tentativa (FE1-28)
- [ ] GET falhando no transporte repete **exatamente 1 vez** após 300ms — transport fake conta 2 chamadas (FE1-29)
- [ ] GET com 4xx/5xx → 1 chamada só, erro mapeado imediato (FE1-30)
- [ ] POST/PATCH falhando → 1 chamada só, sem retry por qualquer motivo (FE1-31)
- [ ] Esgotadas as tentativas de transporte → erro com kind `offline` (FE1-20)
- [ ] Gate check passa: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: ≥ 7 testes (retry ok, retry esgotado, 4xx sem retry, 5xx sem retry, POST sem retry, PATCH sem retry, default 5s)

**Tests**: unit
**Gate**: full — `cd desktop && flutter analyze && flutter test`
**Commit**: `feat(LGN-90): add fail-fast timeout and idempotent get retry`

### Phase 4: Contrato

#### T8: Fixtures do contrato real + suite contra fixtures

**What**: Capturar respostas reais da API local (backend rodando via `make dev-db` + `go run ./cmd/api.go`) para os 6 endpoints, o envelope de erro e o par delta do PATCH; fixá-las em `desktop/test/fixtures/` e escrever a suite que roda contra elas sem API — incluindo o teste de drift.
**Where**: `desktop/test/fixtures/`
**Depends on**: T7
**Reuses**: Padrão de fixtures do TDD (fixtures fixadas do contrato real); client de T6/T7
**Requirement**: FE1-32, FE1-33, FE1-34

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [ ] `flutter test` roda verde sem nenhuma API alcançável (FE1-32)
- [ ] Fixture real capturada para cada um dos 6 endpoints + envelope de erro + par `{"data":{"stats":{"dexterity":16}}}` → resposta só com cascatas (FE1-33)
- [ ] Alterar chave de uma fixture faz a suite falhar (drift detectado) — kill-confirm no scratch (FE1-34)
- [ ] Captura feita de resposta real local, não escrita à mão (premissa da spec)
- [ ] Gate check passa: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: ≥ 8 testes de contrato (6 endpoints + envelope de erro + delta)

**Tests**: unit
**Gate**: full — `cd desktop && flutter analyze && flutter test`
**Commit**: `test(LGN-90): pin real contract fixtures for client suite`

---

#### T9: Suite de contrato ao vivo self-skip

**What**: Criar a suite que bate na API real quando há base alcançável configurada (flag/env) e se auto-pula quando não, cobrindo os 4 cenários do TDD — nunca skip silencioso quando a base existe.
**Where**: `desktop/test/contract/`
**Depends on**: T8
**Reuses**: Fixtures e client do T8; cenários de contrato do TDD §Estratégia de Testes
**Requirement**: FE1-35, FE1-36, FE1-37

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [ ] Sem base de API configurada → suite se auto-pula e `flutter test` termina exit 0 (FE1-35)
- [ ] Com base configurada → 4 cenários rodam: `POST /sheets`→201 com valores derivados; PATCH editável→200 delta só de cascatas; PATCH derivado/desconhecido→400 `INVALID_REQUEST`/`UNKNOWN_FIELD`; GET inexistente→404 `NOT_FOUND` (FE1-36)
- [ ] Base configurada + status/código inesperado → falha exibindo status e `apperr` observados, nunca skip silencioso (FE1-37)
- [ ] Self-skip segue o padrão do repo (testes que se auto-pulam sem serviço externo)
- [ ] Gate check passa: `cd desktop && flutter analyze && flutter test`
- [ ] Test count: ≥ 4 cenários + 1 teste do mecanismo de skip

**Tests**: integration
**Gate**: full — `cd desktop && flutter analyze && flutter test` (self-skip sem API; o cenário real roda quando há instância local)
**Commit**: `test(LGN-90): add self-skipping live contract suite`

---

## Phase Execution Map

```
Phase 1 → Phase 2 → Phase 3 → Phase 4

Phase 1:  T1
          T2 → T3
Phase 2:  T4
          T5
Phase 3:  T6 → T7
Phase 4:  T8 → T9
```

Execução estritamente sequencial — sem paralelismo intra-phase. 9 tasks = **2 batches** (Phases 1+2+3 = 7 tasks; Phase 4 = 2) → acima do limite ~8, oferta de sub-agents de batch apresentada na aprovação (offer-then-confirm, nunca auto-spawn). Workers rodam sequencialmente; após o último task, o Verifier sub-agent (fresh) roda automaticamente.

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Config de URL base | 1 arquivo (resolução + validação da base) | ✅ Granular |
| T2: Decode de envelope + erro de domínio | 1 conceito coeso (decodificador + tipo de erro que ele devolve) | ✅ Granular |
| T3: Mapper `apperr` + localização | 1 conceito coeso (mapear kind + resolver copy) | ✅ Granular |
| T4: Models de template | 1 arquivo de models | ✅ Granular |
| T5: Model de `Sheet` + path | 1 arquivo de model | ✅ Granular |
| T6: Client tipado | 1 arquivo (6 métodos do mesmo client) | ✅ Granular |
| T7: Timeout/retry | 1 política aplicada ao client (modifica T6) | ✅ Granular |
| T8: Fixtures reais + suite | 1 entregável (assets de teste + suite de fixtures) | ✅ Granular |
| T9: Contrato ao vivo | 1 arquivo de suite (self-skip) | ✅ Granular |

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | Phase 1: `T1` isolado | ✅ Match |
| T2 | None | Phase 1: `T2 → T3` (T2 origem) | ✅ Match |
| T3 | T2 | Phase 1: `T2 → T3` | ✅ Match |
| T4 | None | Phase 2: `T4` isolado | ✅ Match |
| T5 | None | Phase 2: `T5` isolado | ✅ Match |
| T6 | T1, T2, T3, T4, T5 (cross-phase, backward) | Phase 3: `T6 → T7` (T6 origem) | ✅ Match — cross-phase backward não exige seta |
| T7 | T6 | Phase 3: `T6 → T7` | ✅ Match |
| T8 | T7 (cross-phase, backward) | Phase 4: `T8 → T9` (T8 origem) | ✅ Match — cross-phase backward não exige seta |
| T9 | T8 | Phase 4: `T8 → T9` | ✅ Match |

Nenhuma dependência aponta para phase futura. ✅

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1 | Config de URL base | unit | unit | ✅ OK |
| T2 | Decode de contrato | unit | unit | ✅ OK |
| T3 | Mapeamento + localização | unit | unit | ✅ OK |
| T4 | Models (template) | unit | unit | ✅ OK |
| T5 | Models (sheet + path) | unit | unit | ✅ OK |
| T6 | API client | unit | unit | ✅ OK |
| T7 | API client (timeout/retry) | unit | unit | ✅ OK |
| T8 | Fixtures de contrato | unit | unit | ✅ OK |
| T9 | Contrato ao vivo | integration (self-skip) | integration | ✅ OK |

Nenhum `Tests: none` contraria a matriz; nenhum defer de teste entre tasks. ✅

---

**Fechamento**: após T9, o Verifier sub-agent (fresh, author ≠ verifier) roda automaticamente — spec-anchored check + discrimination sensor → `validation.md` (PASS/FAIL) + `validate_state.py` exit 0. Gaps viram fix tasks; loop limitado a 3 iterações.
