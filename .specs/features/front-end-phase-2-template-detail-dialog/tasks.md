# [Front End] Fase 2 — Detalhe do template e diálogo de criação (Tela 3) — Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/front-end-phase-2-template-detail-dialog/design.md`
**Status**: Done

---

## Test Coverage Matrix

> Gerada do codebase, guidelines do projeto e spec — confirmar antes do Execute. Guidelines encontrados: `AGENTS.md` (raiz — testes obrigatórios, edge cases primeiro, tabela-driven, inglês, testes junto ao código), `desktop/AGENTS.md` (regras visuais/interação), `docs/TDD-desktop-mvp.md` §Estratégia de Testes, `.github/workflows/ci-flutter.yml` (analyze → test → build windows), `desktop/analysis_options.yaml` (`flutter_lints`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Resumo por prefixo (`desktop/lib/catalog/template_summary.dart`) | unit | 1:1 com TPL-02/03/05; todas as bordas: `fields` vazio, folha de topo, grupo aninhado, chave inesperada, nó sem folhas, ordem preservada | `desktop/test/catalog/template_summary_test.dart` | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter test` |
| Provider de detalhe (`desktop/lib/catalog/template_detail_provider.dart`) | unit (transporte `MockClient`) | 1:1 com TPL-01/04/06/07/08/09/10; edges: single-flight, `NOT_FOUND`, offline, sem `data`, shape malformado, sem resumo obsoleto, descarte em voo | `desktop/test/catalog/template_detail_provider_test.dart` | idem |
| Superfície de criação (`desktop/lib/catalog/create_sheet_dialog.dart`) | widget | 1:1 com TPL-01/02/04/07/08/09/10/11/12/13/14/15; edges: `fields` vazio, nome vazio, nome digitado sem request, reabertura = 2 GETs, zero POST | `desktop/test/catalog/create_sheet_dialog_test.dart` | idem |
| Navegação compartilhada (`desktop/test/catalog/create_sheet_navigation_test.dart`, `desktop/test/app_boot_test.dart`) | widget (atualização) | Harness passa a rotear `/templates/:id` e a contar apenas requisições de catálogo — CAT-15/16/17/18 preservados | via arquivos acima | idem |
| Strings l10n (`desktop/lib/l10n/app_en.arb`) | widget (assert via render) | toda chave nova aparece renderizada em pelo menos um teste; nenhum literal de UI nos widgets | via suite da superfície | idem |
| Specs (`.specs/`) | none | validators exit 0 | `.specs/**` | `validate_spec.py` / `validate_tasks.py` |

## Gate Check Commands

> Geradas do codebase — confirmar antes do Execute. `flutter` fica em `$HOME/flutter/bin` (exportar no PATH do shell de execução).

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | tasks só com testes unit já escritos | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter test` |
| Full | toda task que adiciona/muda código Dart (padrão do repo: analyze limpo antes de commit) | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test` |
| Build | tasks que tocam codegen Riverpod ou l10n (`lib/*.g.dart`, `app_localizations*.dart`) — T2, T3 | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test` |

---

## Execution Plan

Phases são ordenadas e rodam em sequência — cada phase completa antes da próxima; tasks dentro da phase rodam em ordem.

### Phase 1: Foundation

Domínio do resumo (função pura) e a camada de dados do detalhe (provider family autoDispose) — independentes entre si, rodam em ordem.

```
T1
T2
```

### Phase 2: Superfície de criação

O diálogo renderiza o detalhe (T3) e depois os estados de erro + garantias de reabertura/descarte (T4).

```
T3 → T4
```

---

## Task Breakdown

### T1: Derive the field summary from the template tree

**What**: Criar `summarizeFields(FieldGroup) → List<FieldSummary>` (record `({String prefix, int leafCount})`), uma linha por chave de topo na ordem da API, contando folhas recursivamente pelo grupo e 1 para folha de topo; `fields` vazio → lista vazia. Nenhum literal de prefixo.
**Where**: `desktop/lib/catalog/template_summary.dart`
**Depends on**: None
**Reuses**: `FieldGroup`/`FieldNode`/`FieldSchema` de `api/models/template.dart`; árvore folha/grupo da Fase 1
**Requirement**: TPL-02, TPL-03, TPL-05

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `fields` de `fantasy_v1` (fixture) → `[baseArmorClass:1, config:5, derived:2, modifiers:2, others:1, stats:2]` na ordem do JSON
- [x] Grupo conta folhas recursivamente; folha de topo conta 1; nó sem schema nem children conta 0
- [x] Nenhum literal `stats`/`modifiers`/`config`/`derived` no código — template sintético com prefixo inesperado renderiza (TPL-05)
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter test`
- [x] Test count: 9 novos testes (suíte 179 passed / 4 skipped); nenhum teste existente removido

**Tests**: unit
**Gate**: quick

**Commit**: `feat(LGN-97): derive the field summary from the template tree`

---

### T2: Add the template detail provider with on-demand fetch

**What**: Criar `TemplateDetailState` imutável (template / isLoading / errorMessage) e `TemplateDetailController` — provider **family autoDispose** keyed por `templateId`: `build` guarda o id, observa `apiClientProvider`, agenda `load()` e retorna `isLoading: true`; `load()` com single-flight zera para loading (TPL-04), sucesso → detalhe decodificado (TPL-06), falha → `failureCopy` (TPL-07/09/10); `if (!ref.mounted) return` após o await.
**Where**: `desktop/lib/catalog/template_detail_provider.dart`
**Depends on**: None
**Reuses**: `ApiClient.getTemplate`, `apiClientProvider`, `failureCopy` (`api_error.dart`), padrão de estado imutável + notifier do `catalog_provider.dart`, `ref.mounted` (riverpod 3.4.3)
**Requirement**: TPL-01, TPL-04, TPL-06, TPL-07, TPL-08, TPL-09, TPL-10

**Tools**:

- MCP: `context7` (só se surgir dúvida da API family/autoDispose do Riverpod)
- Skill: NONE

**Done when**:

- [x] Primeiro watch dispara exatamente 1 `GET /templates/:id` e fica em loading até resolver (TPL-01)
- [x] Sem resumo obsoleto: novo provider (outro id) começa com `template == null` em loading (TPL-04)
- [x] Sucesso expõe o `TemplateDetail` com `fields` intactos (TPL-06); falha de domínio → copy do kind (TPL-07); `NOT_FOUND` → copy notFound (TPL-09); sem `data`/shape inválido → copy parse (TPL-10)
- [x] `load()` re-requesta exatamente 1× por ativação (simula Retry, TPL-08); single-flight absorve duplo-toque
- [x] `build_runner` executado e `.g.dart` commitado
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test`
- [x] Test count: 11 novos testes (suíte 182 passed / 4 skipped); nenhum teste existente removido

**Tests**: unit
**Gate**: build

**Commit**: `feat(LGN-97): add the template detail provider with on-demand fetch`

---

### T3: Render the creation surface from the template detail

**What**: Reescrever `CreateSheetDialog` como `ConsumerStatefulWidget`: identidade (label "Template" caption + id com `Key('create-sheet-template-id')` + nome do template quando carregado), área de detalhe (loading → `LinearProgressIndicator` + copy; sucesso → linha `prefixo: contagem` por chave de topo ou linha `templateNoFields`), campo de nome vazio com label + placeholder localizados, ação primária "Create sheet" **desabilitada** com `Tooltip`. Novas chaves l10n; remover `createSheetTitle` (órfã). Atualizar o harness dos testes de navegação/boot para rotear `/templates/:id` e contar só requisições de catálogo.
**Where**: `desktop/lib/catalog/create_sheet_dialog.dart`
**Depends on**: T1, T2
**Reuses**: `templateDetailControllerProvider` (T2), `summarizeFields` (T1), tokens/theme (LGN-96), `CreateSheetDialog` + fluxo `_openCreationDialog` (CAT-15/16/17), `AppLocalizations`
**Requirement**: TPL-01, TPL-02, TPL-04, TPL-05, TPL-11, TPL-12, TPL-13

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Abrir a superfície dispara `GET /templates/:id` e mostra loading, sem resumo de template anterior (TPL-01/04)
- [x] Sucesso: nome do template + uma linha por chave de topo com a contagem de folhas; `fields` vazio → `templateNoFields`; nome vazio renderiza sem quebrar (TPL-02/03)
- [x] Campo de nome vazio com label + placeholder localizados; digitar não emite nenhuma requisição (TPL-11)
- [x] Ação primária "Create sheet" desabilitada e com tooltip localizado (TPL-12); zero mutações no transporte (TPL-13)
- [x] Harness compartilhado atualizado (rota `/templates/:id`; asserções contam só `/templates`) sem enfraquecer CAT-15/16/17/18
- [x] `build_runner`/l10n regenerados; gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test`
- [x] Test count: 10 novos testes de widget + 2 harnesses atualizados (suíte 192 passed / 4 skipped); nenhum teste existente removido

**Tests**: widget
**Gate**: build

**Commit**: `feat(LGN-97): render the creation surface from the template detail`

---

### T4: Show inline detail errors with retry and prove fresh reopen

**What**: Adicionar ao diálogo a ramificação de erro: mensagem mapeada inline (`failureCopy`) + `OutlinedButton` Retry que chama `load()` (TPL-07/08/09/10), diálogo permanece aberto; e cobrir as garantias de TPL-13/14/15 em widget test — abrir/fechar/reabrir emite 2 `GET /templates/:id`, `Esc`/Close não emite requisição nova, nenhum `POST` no transporte.
**Where**: `desktop/lib/catalog/create_sheet_dialog.dart`
**Depends on**: T3
**Reuses**: `failureCopy` / `errorCopy` da Fase 1, `actionRetry`, harness `MockClient` do T3, fluxo de dismiss CAT-16 (`barrierDismissible`)
**Requirement**: TPL-07, TPL-08, TPL-09, TPL-10, TPL-13, TPL-14, TPL-15

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Falha de transporte/4xx/5xx → mensagem mapeada inline, diálogo aberto (TPL-07); `NOT_FOUND` idem (TPL-09); 2xx sem `data`/shape inválido → copy parse, nunca resumo meio-renderizado (TPL-10)
- [x] Retry re-requesta `GET /templates/:id` exatamente 1× por ativação (TPL-08)
- [x] Reabrir o mesmo template → 2 requisições de detalhe no transporte fake (TPL-15)
- [x] `Esc` e Close fecham e voltam ao catálogo sem requisição nova (TPL-14); nenhum `POST` (TPL-13)
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [x] Test count: 7 novos testes (suíte 199 passed / 4 skipped); nenhum teste existente removido

**Tests**: widget
**Gate**: full

**Commit**: `feat(LGN-97): show inline detail errors with retry in the creation dialog`

---

## Phase Execution Map

```
Phase 1 → Phase 2

Phase 1:  T1
Phase 1:  T2
Phase 2:  T3 → T4

T1 → T3
T2 → T3
T3 → T4
```

As arestas entre phases aparecem nas linhas `T1 → T3`, `T2 → T3` e `T3 → T4`; cada seta corresponde a um `Depends on` declarado no corpo da task apontada. Execution is strictly sequential - there is no intra-phase parallelism. A single agent works one task at a time, in order.

**How phase-based execution works:**

With 4 tasks in 2 phases the feature fits a single task-budgeted batch (≤ ~8 tasks): execute inline in the main window, no sub-agent dispatch. After the final task the fresh **Verifier** runs automatically (author ≠ verifier). See [sub-agents.md](references/sub-agents.md) for the full model.

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Derive the field summary from the template tree | 1 função pura (+ typedef) | ✅ Granular |
| T2: Add the template detail provider with on-demand fetch | 1 componente de estado (provider) | ✅ Granular |
| T3: Render the creation surface from the template detail | 1 componente (diálogo) + chaves l10n que ele renderiza | ✅ Granular |
| T4: Show inline detail errors with retry and prove fresh reopen | 1 ramificação de estado do mesmo diálogo + provas de ciclo de vida | ✅ Granular |

**Granularity check**: ✅ T3 e T4 tocam o mesmo arquivo porque são fatias sequenciais do mesmo componente (render do detalhe → estados de erro/ciclo de vida), não escopos distintos; cada uma entrega um conceito coeso.

## Diagram-Definition Cross-Check

| Task | Depends on (corpo) | Diagrama mostra | Status |
| ---- | ------------------ | --------------- | ------ |
| T1 | None | — | ✅ Match |
| T2 | None | — | ✅ Match |
| T3 | T1, T2 | `T1 → T3`, `T2 → T3` (Phase Execution Map) | ✅ Match |
| T4 | T3 | `T3 → T4` (Phase 2 e map) | ✅ Match |

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1: resumo por prefixo | função pura (unit) | unit | unit | ✅ OK |
| T2: provider de detalhe | provider (unit) | unit | unit | ✅ OK |
| T3: superfície de criação + l10n + harness | diálogo (widget) | widget | widget | ✅ OK |
| T4: erros inline + ciclo de vida | diálogo (widget) | widget | widget | ✅ OK |

Nenhuma task usa `Tests: none`.
