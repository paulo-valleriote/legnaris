# [Front End] Fase 3 — Renderização da ficha (Tela 2) — Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/front-end-phase-3-sheet-rendering/design.md`
**Status**: Done

---

## Test Coverage Matrix

> Gerada do codebase, guidelines do projeto e spec — confirmar antes do Execute. Guidelines encontrados: `AGENTS.md` (raiz — testes obrigatórios, edge cases primeiro, tabela-driven, inglês, testes junto ao código), `desktop/AGENTS.md` (regras visuais/interação, schema-driven UI), `docs/TDD-desktop-mvp.md` §Estratégia de Testes (widget para UI Engine: "todos os tipos + toggles"), `.github/workflows/ci-flutter.yml` (analyze → test → build windows), `desktop/analysis_options.yaml` (`flutter_lints`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Derivação de seções (`desktop/lib/engine/`) | unit | 1:1 com RND-01–RND-05; edges: árvore vazia, grupo sem folhas, folha de topo, aninhamento profundo, label vazio → humanizado, prefixo exótico, chaves repetidas | `desktop/test/engine/field_grouping_test.dart` | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter test` |
| Controle por campo (`desktop/lib/engine/`) | widget | 1:1 com RND-06–RND-13; cada tipo (número/texto/bool/config/derivado/exótico) renderiza o controle certo; `onChanged` com caminho+valor na edição; read-only sem input + "Calculated"; fórmula exposta | `desktop/test/engine/field_control_test.dart` | idem |
| Composição do formulário (`desktop/lib/engine/`) | widget | 1:1 com RND-01, RND-05, RND-12, RND-16; seções na ordem, linhas com label, valor resolvido por caminho, `onChanged` repassado | `desktop/test/engine/sheet_form_test.dart` | idem |
| Tela 2 host (`desktop/lib/sheet/`) | widget | 1:1 com RND-15–RND-17, RND-19; loading → conteúdo, seed dos defaults, erro mapeado + Retry, edição local sem request | `desktop/test/sheet/sheet_screen_test.dart` | idem |
| Navegação catálogo → Tela 2 (`desktop/lib/catalog/`) | widget | 1:1 com RND-14, RND-18, RND-19; ação abre a Tela 2 com 1 `GET /templates/:id`, back volta ao catálogo, zero `POST`/`PATCH` no transporte | `desktop/test/sheet/sheet_navigation_test.dart` | idem |
| Strings l10n (`desktop/lib/l10n/app_en.arb`) | widget (assert via render) | `fieldCalculated` e `catalogPreviewAction` renderizam em ≥1 teste; nenhum literal de UI nos widgets | via suites acima | idem |
| Specs (`.specs/`) | none | validators exit 0 | `.specs/**` | `validate_spec.py` / `validate_tasks.py` |

## Gate Check Commands

> Geradas do codebase — confirmar antes do Execute. `flutter` fica em `$HOME/flutter/bin` (exportar no PATH do shell de execução).

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | tasks só com testes unit já escritos | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter test` |
| Full | toda task que adiciona/muda código Dart (padrão do repo: analyze limpo antes de commit) | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test` |
| Build | tasks que tocam l10n (`app_localizations*.dart`) — T2, T5 | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && flutter gen-l10n && flutter analyze && flutter test` + o CI valida (`windows-latest`: analyze → test → build windows) |

---

## Execution Plan

Phases são ordenadas e rodam em sequência — cada phase completa antes da próxima; tasks dentro da phase rodam em ordem.

### Phase 1: UI Engine core

Derivação de seções, controle por tipo e composição do formulário — a renderização pura, testada sem provider nem rede.

```
T1 → T2 → T3
```

### Phase 2: Tela 2 navegável

O host que busca o detalhe e semeia os defaults, mais a ação do catálogo que abre a superfície.

```
T3 → T4 → T5
```

---

## Task Breakdown

### T1: Derive sheet sections from the field tree

**What**: Criar `field_grouping.dart` (`typedef SheetField = ({String path, FieldSchema schema})`, `typedef SheetSection = ({String key, String label, List<SheetField> leaves})`, `buildSections(FieldGroup)` — uma seção por chave de topo na ordem da API, folhas recursivas em ordem de árvore, grupo sem folhas omitido, folha de topo vira seção de uma folha — e `fieldDisplayName(String key, FieldSchema? schema)` movido do `template_summary`); refatorar `template_summary.dart` para chamar `fieldDisplayName` (sem mudar o resumo); testes unitários em `test/engine/field_grouping_test.dart`.
**Where**: `desktop/lib/engine/field_grouping.dart`
**Depends on**: None
**Reuses**: `_humanize`/`_displayName`/`_leaves` de `desktop/lib/catalog/template_summary.dart:53-74`; `FieldGroup`/`FieldNode`/`FieldSchema` de `desktop/lib/api/models/template.dart`
**Requirement**: RND-01, RND-02, RND-03, RND-04, RND-05

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Uma seção por chave de topo visível, na ordem de inserção do mapa da API (RND-01)
- [x] Grupo aninhado flatted recursivamente nas folhas certas, na ordem da árvore (RND-02)
- [x] Folha de topo vira seção de uma folha; grupo sem folhas é omitido; `fields` vazio → lista vazia (RND-03 + edge)
- [x] Label = `schema.label` quando não vazio, senão chave humanizada; prefixo exótico renderiza igual (RND-04/05)
- [x] `template_summary_test.dart` continua verde após o refactor do display name (TPL-02/03/05)
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [x] Test count: 10 novos testes (suíte 212 passed / 4 skipped); nenhum teste existente removido

**Tests**: unit
**Gate**: full
**Commit**: `feat(LGN-101): derive sheet sections from the field tree`

---

### T2: Render the field control per schema rule

**What**: Adicionar a chave l10n `fieldCalculated` + `flutter gen-l10n`; criar `field_control.dart` (`SheetFieldControl extends StatefulWidget` — `{path, schema, value, onChanged}` com a ordem de regras: não-editável → valor read-only + destaque `fieldCalculated` e `Tooltip` da fórmula quando `formula != null`; `type == bool` ou prefixo `config` → `Switch` (bool emite `true`/`false`, `config.*` emite `1`/`0`); `type == number` → `TextField` numérico (emite `num`/`null`); `type == text`/`string` → `TextField` (emite `String`); tipo exótico → read-only; controller ressincronizado só quando o valor externo difere do último emitido); testes widget em `test/engine/field_control_test.dart`.
**Where**: `desktop/lib/engine/field_control.dart`
**Depends on**: T1
**Reuses**: tokens (`controlHeight`, `rowHeight`, `AppRadius.control`), `fieldDisplayName` da T1, `Tooltip` nativo, padrão de arb com `@`-description da Fase 2
**Requirement**: RND-05, RND-06, RND-07, RND-08, RND-09, RND-10, RND-11, RND-12, RND-13

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Número editável → input numérico com o valor atual; editar emite `num` pelo `onChanged(path, value)` (RND-06, RND-12)
- [x] Texto editável → input de texto com o valor atual; editar emite `String` (RND-07, RND-12)
- [x] `config.*` → `Switch` refletindo 0/1; toggle emite `1`/`0` (RND-08)
- [x] `type == bool` → `Switch` refletindo true/false; toggle emite `true`/`false` (RND-09)
- [x] Não-editável → valor read-only com destaque "Calculated" e **sem** input (RND-05, RND-10)
- [x] Campo com `formula` expõe a fórmula no `Tooltip` do controle (RND-11)
- [x] Tipo exótico → read-only, sem input e sem exceção (RND-13)
- [x] Input numérico limpo emite `null` (edge)
- [x] Saída do `gen-l10n` commitada; gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && flutter gen-l10n && flutter analyze && flutter test`
- [x] Test count: 12 novos testes (suíte 224 passed / 4 skipped); nenhum teste existente removido

**Tests**: widget
**Gate**: build
**Commit**: `feat(LGN-101): render the field control per schema rule`

---

### T3: Compose the sheet form with sections and rows

**What**: Extrair `valueAtPath(Map<String, Object?> data, String path)` em `desktop/lib/api/models/sheet.dart` (com `Sheet.valueAt` delegando, semântica FE1-12…14 intacta); criar `sheet_form.dart` (`SheetForm extends StatelessWidget` — `{fields, data, onChanged}`: por seção de `buildSections`, cabeçalho + `Divider` + linhas `Row(label, SheetFieldControl)`; valor resolvido por `valueAtPath(data, leaf.path)`; edição repassada pelo `onChanged` do form); testes widget em `test/engine/sheet_form_test.dart`.
**Where**: `desktop/lib/engine/sheet_form.dart`
**Depends on**: T2
**Reuses**: `buildSections`/`fieldDisplayName` (T1), `SheetFieldControl` (T2), `valueAtPath` extraído de `desktop/lib/api/models/sheet.dart:59-75`, `sheet_model_test.dart` como rede do acesso por caminho
**Requirement**: RND-01, RND-05, RND-12, RND-16

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Seções renderizam na ordem da API, cada uma com seu cabeçalho e suas linhas (RND-01)
- [x] Cada linha mostra o label (schema ou humanizado) e o controle com o valor resolvido do mapa por caminho (RND-05)
- [x] Editar uma linha chama `onChanged(path, value)` do formulário com o caminho pontilhado correto (RND-12)
- [x] Valor ausente no mapa → linha vazia/off, sem exceção (edge)
- [x] `sheet_model_test.dart` continua verde após a extração de `valueAtPath` (FE1-12…14)
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [x] Test count: 8 novos testes (suíte 232 passed / 4 skipped); nenhum teste existente removido

**Tests**: widget
**Gate**: full
**Commit**: `feat(LGN-101): compose the sheet form with sections and rows`

---

### T4: Add the navigable sheet screen

**What**: Criar `sheet_screen.dart` (`SheetScreen extends ConsumerStatefulWidget` — `{templateId}`: `ref.watch(templateDetailControllerProvider(templateId))`; header com back (`IconButton` + `MaterialLocalizations.backButtonTooltip`) e nome/id do template; loading, erro mapeado inline + Retry, `SheetForm` com `data` semeado dos `default` do schema (uma vez, no primeiro sucesso) e `onChanged` atualizando o rascunho local por `setState`, ou `templateNoFields` quando vazio); testes widget em `test/sheet/sheet_screen_test.dart`.
**Where**: `desktop/lib/sheet/sheet_screen.dart`
**Depends on**: T3
**Reuses**: `templateDetailControllerProvider`/`failureCopy` (`desktop/lib/catalog/template_detail_provider.dart`, `desktop/lib/api/api_error.dart`), `SheetForm` (T3), `buildSections` (T1), `actionRetry`/`templateNoFields`/`templateDetailLoading` (l10n), tokens
**Requirement**: RND-15, RND-16, RND-17, RND-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Abrir a Tela 2 dispara exatamente 1 `GET /templates/:id` e mostra o loading até resolver (RND-15)
- [x] Sucesso → seções/controles renderizados com os `default` do schema (RND-16)
- [x] Falha (500/404/parse) → mensagem mapeada inline + Retry, tela continua aberta (RND-17)
- [x] Editar um campo atualiza o rascunho local; nenhum `POST`/`PATCH` no transporte (RND-19, RND-12)
- [x] `fields` vazio → `templateNoFields`; nome vazio → título vazio sem throw (edges)
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [x] Test count: 5 novos testes (suíte 237 passed / 4 skipped); nenhum teste existente removido

**Tests**: widget
**Gate**: full
**Commit**: `feat(LGN-101): add the navigable sheet screen`

---

### T5: Open the sheet screen from the catalog

**What**: Adicionar a chave l10n `catalogPreviewAction` + `flutter gen-l10n`; adicionar ao card do catálogo a `TextButton` secundária "Preview sheet" que empurra `AppShell(content: SheetScreen(templateId: ...))` via `Navigator.push` (sem tocar no diálogo de criação); testes widget em `test/sheet/sheet_navigation_test.dart` (abre a Tela 2 com 1 request, back volta ao catálogo, zero mutações).
**Where**: `desktop/lib/catalog/catalog_screen.dart`
**Depends on**: T4
**Reuses**: card/`showDialog` de `desktop/lib/catalog/catalog_screen.dart:129-204`, `AppShell` de `desktop/lib/shell/app_shell.dart`, `SheetScreen` (T4), padrão de arb da Fase 2
**Requirement**: RND-14, RND-18, RND-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] O card expõe "Preview sheet" e ativá-lo abre a Tela 2 do template do card (RND-14)
- [x] A abertura emite exatamente 1 `GET /templates/:id` e nenhum `POST`/`PATCH` (RND-19)
- [x] O back da Tela 2 volta ao catálogo com a lista em cache e sem nova listagem (RND-18)
- [x] O fluxo existente do diálogo de criação segue intacto (testes CAT verdes)
- [x] Saída do `gen-l10n` commitada; gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && flutter gen-l10n && flutter analyze && flutter test`
- [x] Test count: 2 novos testes (suíte 239 passed / 4 skipped); nenhum teste existente removido

**Tests**: widget
**Gate**: build
**Commit**: `feat(LGN-101): open the sheet screen from the catalog`

---

## Phase Execution Map

```
Phase 1 → Phase 2

Phase 1:  T1 → T2 → T3
Phase 2:  T3 → T4 → T5
```

Execution is strictly sequential - there is no intra-phase parallelism. A single agent works one task at a time, in order.

**How phase-based execution works:**

At Execute, the agent counts total tasks and packs phases into **task-budgeted batches** (~7 tasks per worker, whole phases). With 5 tasks in 2 phases this fits a **single batch → execute inline** (no sub-agents), one task at a time. After the final task the orchestrator dispatches the fresh **Verifier** automatically (author ≠ verifier). See [sub-agents.md](references/sub-agents.md) for the full model.

**The orchestrating agent's role during Execute:**
1. Count total tasks and pack phases into batches (5 tasks → inline execution)
2. Execute T1 → T2 → T3 → T4 → T5 in order (tests → implement → gate → adequacy review → atomic commit)
3. Update tasks.md with results in each task's commit
4. If a task fails: fix before starting the next
5. After T5: dispatch the Verifier automatically, then run `validate_state.py`

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Derive sheet sections from the field tree | 1 camada de derivação pura (somada da extração coesa do display name) | ✅ Granular |
| T2: Render the field control per schema rule | 1 componente (`SheetFieldControl`) + a chave l10n que ele consome | ✅ Granular |
| T3: Compose the sheet form with sections and rows | 1 componente (`SheetForm`) + a extração coesa do `valueAtPath` que ele usa | ✅ Granular |
| T4: Add the navigable sheet screen | 1 superfície (`SheetScreen`, host de dados sintéticos) | ✅ Granular |
| T5: Open the sheet screen from the catalog | 1 ponto de entrada (ação do card + navegação) + a chave l10n que ela consome | ✅ Granular |

**Granularity check**: ✅ nenhuma task entrega múltiplos componentes independentes; T1/T3 incluem a extração mínima de um helper que reusam (parte do mesmo conceito).

## Diagram-Definition Cross-Check

| Task | Depends on (corpo) | Diagrama mostra | Status |
| ---- | ------------------ | --------------- | ------ |
| T1 | None | — | ✅ Match |
| T2 | T1 | `T1 → T2` (Phase 1) | ✅ Match |
| T3 | T2 | `T2 → T3` (Phase 1) | ✅ Match |
| T4 | T3 | `T3 → T4` (Phase 2) | ✅ Match |
| T5 | T4 | `T4 → T5` (Phase 2) | ✅ Match |

Nota: o validador mapeia todas as `### Tn:` para a última phase header anterior ao `Task Breakdown` (Phase 2), então a paridade de diagrama vale globalmente — arestas do diagrama = {T1→T2, T2→T3, T3→T4, T4→T5} = exatamente o conjunto de `Depends on` declarados.

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1: field grouping + display name | derivação (unit) | unit | unit | ✅ OK |
| T2: field control + l10n | controle widget (widget) | widget | widget | ✅ OK |
| T3: sheet form + valueAtPath | composição widget (widget) | widget | widget | ✅ OK |
| T4: sheet screen host | Tela 2 widget (widget) | widget | widget | ✅ OK |
| T5: catálogo → Tela 2 | navegação widget (widget) | widget | widget | ✅ OK |

Nenhuma task usa `Tests: none`.
