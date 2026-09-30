# [Front End] Fase 2 — Catálogo: lista de templates, shell e navegação rasa — Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/front-end-phase-2-catalog/design.md`
**Status**: Approved

---

## Test Coverage Matrix

> Gerada do codebase, guidelines do projeto e spec — confirmar antes do Execute. Guidelines encontrados: `AGENTS.md` (raiz — testes obrigatórios, edge cases primeiro, tabela-driven, inglês, testes junto ao código), `desktop/AGENTS.md` (regras visuais/interação), `docs/TDD-desktop-mvp.md` §Estratégia de Testes (unit >80% camadas não-widget, contrato self-skip sem API), `.github/workflows/ci-flutter.yml` (analyze → test → build windows), `desktop/analysis_options.yaml` (`flutter_lints`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Theme tokens + app theme (`desktop/lib/theme/`) | unit | 1:1 com CAT-02; nav/painel nas faixas 200–240/280–320, type scale ∈ {12,13,15,17} (nunca >17), radius ≤4, spacing ∈ {4,8,12,16,24}, hairline = `outlineVariant` | `desktop/test/theme/*_test.dart` | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter test` |
| Catalog provider + client provider (`desktop/lib/catalog/catalog_provider.dart`, `desktop/lib/api/api_client_provider.dart`) | unit (transporte `MockClient`) | 1:1 com CAT-07, 09–14, 18, 19; edges: sem chave `data`, entry malformada, single-flight com completer, ordem da API, kind `offline`, nome vazio no model | `desktop/test/catalog/catalog_provider_test.dart` | idem |
| Shell (`desktop/lib/shell/`) | widget | 1:1 com CAT-01–06; resize encolher→restaurar sem novo request, nav nunca sobreposta, contagem de chamadas estável | `desktop/test/shell/app_shell_test.dart` | idem |
| Catalog screen (`desktop/lib/catalog/catalog_screen.dart`) | widget | 1:1 com CAT-08–12, 19 (render); 4 estados distintos, Retry = exatamente 1 chamada, lista permanece no refresh falho, nome vazio renderiza | `desktop/test/catalog/catalog_screen_test.dart` | idem |
| Diálogo de criação + navegação | widget | 1:1 com CAT-15–17; id correto no diálogo, Esc/Close sem refetch, exatamente 1 push de rota | `desktop/test/catalog/create_sheet_navigation_test.dart` | idem |
| Shortcuts + wiring final (`desktop/lib/shell/shortcut_registry.dart`, `desktop/lib/main.dart`) | unit + widget | 1:1 com CAT-18 + edge F5 fora do catálogo; boot CAT-01/05/07; mapa do registry e defaults preservados | `desktop/test/shell/shortcut_registry_test.dart`, `desktop/test/app_boot_test.dart` | idem |
| Strings l10n (`desktop/lib/l10n/app_en.arb`) | widget (assert via render) | toda chave nova aparece renderizada em pelo menos um teste; nenhum literal de UI nos widgets | via suites acima | idem |
| Specs (`.specs/`) | none | validators exit 0 | `.specs/**` | `validate_spec.py` / `validate_tasks.py` |

## Gate Check Commands

> Geradas do codebase — confirmar antes do Execute. `flutter` fica em `$HOME/flutter/bin` (exportar no PATH do shell de execução).

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | tasks só com testes unit já escritos | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter test` |
| Full | toda task que adiciona/muda código Dart (padrão do repo: analyze limpo antes de commit) | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test` |
| Build | tasks que tocam codegen Riverpod ou l10n (`lib/*.g.dart`, `app_localizations*.dart`) — T2, T3 | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test` + o CI valida (`windows-latest`: analyze → test → build windows; **build Windows é impossível em dev Linux, é exclusivo do CI**) |

---

## Execution Plan

Phases são ordenadas e rodam em sequência — cada phase completa antes da próxima; tasks dentro da phase rodam em ordem.

### Phase 1: Foundation

Tema único (tokens + theme) e a camada de dados do catálogo (provider com cache em memória e single-flight) — independentes entre si, rodam em ordem.

```
T1 → T3
T2 → T3
```

### Phase 2: Shell e tela

O shell monta o layout único sobre o tema e o provider; a tela de catálogo vive dentro do shell.

```
T3 → T4
```

### Phase 3: Navegação, atalhos e wiring

Diálogo de criação (navegação rasa), registry de atalhos (F5/Esc) e ligação final do `main.dart`.

```
T5 → T6
```

---

## Task Breakdown

### T1: Add theme tokens and app theme

**What**: Criar a fonte única de tokens (`AppSpacing`, `AppRadius`, `AppSizes` — escala tipográfica, larguras de nav/painel/conteúdo, `shellMinWidthWithPanel` derivado) e `buildAppTheme()` (type scale mapeado para {12,13,15,17} com cap de 17, dividers 1px `outlineVariant`, card/dialog radius 4, button radius 2, tudo lido dos tokens); ligar `theme:` no `LegnarisApp` (placeholder screen inalterada).
**Where**: `desktop/lib/theme/`
**Depends on**: None
**Reuses**: `desktop/AGENTS.md` §Tokens; `ThemeData(useMaterial3: true)` (Material defaults — sem paleta inventada)
**Requirement**: CAT-02 (suporta CAT-01/03/06)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] `tokens.dart` expõe todos os valores da tabela de tokens do AGENTS; `shellMinWidthWithPanel` é derivado (número mágico proibido)
- [x] `buildAppTheme()` mapeia todo `TextStyle` em {12,13,15,17}; divider/card/dialog/button themes leem tokens
- [x] `MaterialApp(theme: buildAppTheme())` ligado; placeholder continua renderizando
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [x] Test count: 19 novos testes (faixas de tokens + assertions do theme); nenhum teste existente removido

**Tests**: unit
**Gate**: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
**Commit**: `feat(LGN-96): add theme tokens and app theme`

---

### T2: Add catalog provider with in-memory cache and single-flight

**What**: Criar `apiClientProvider` (um `http.Client`, `onDispose` fecha) + `CatalogState` imutável + notifier keepAlive `Catalog` (primeira carga via `Future.microtask` em `build()`, `refresh()` single-flight, sucesso substitui lista/`lastUpdated`, falha mantém cache + `errorMessage`), `failureCopy(Object)` em `api_error.dart` (domain → `errorCopy(kind)`, resto → nova chave `errorParse` no `app_en.arb`).
**Where**: `desktop/lib/catalog/catalog_provider.dart`
**Depends on**: None
**Reuses**: `ApiClient.listTemplates()`, `error_mapper.errorCopy`, `ApiDomainException`/`ApiParseException`, padrão `MockClient` de `desktop/test/api/api_client_test.dart`, codegen Riverpod (`part` + `build_runner`)
**Requirement**: CAT-07, CAT-09, CAT-10, CAT-11, CAT-12, CAT-13, CAT-14, CAT-18, CAT-19

**Tools**:

- MCP: `context7` (só se surgir dúvida de API do Riverpod — já pesquisada no Design)
- Skill: NONE

**Done when**:

- [x] Primeiro read dispara exatamente 1 `GET /templates` e fica em loading até resolver (CAT-07)
- [x] Sucesso grava lista na ordem da API + `lastUpdated` (CAT-09); lista vazia → estado vazio (CAT-10)
- [x] Falha de domínio → copy mapeada (CAT-11); payload malformado/chave `data` ausente → copy `errorParse` (CAT-14 + edge)
- [x] Single-flight: gatilho durante voo = 0 chamadas novas (CAT-13); Retry = exatamente 1 (CAT-12)
- [x] Refresh falho mantém lista cacheada + `errorMessage` (CAT-19); F5-sucesso substitui (CAT-18)
- [x] `flutter pub get` + `dart run build_runner build --delete-conflicting-outputs` executados e `.g.dart` commitados
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test`
- [x] Test count: 17 novos testes; nenhum teste existente removido

**Tests**: unit
**Gate**: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test`
**Commit**: `feat(LGN-96): add catalog provider with in-memory cache and single-flight`

---

### T3: Add app shell with nav, header, content and right panel

**What**: `AppShell(content:)` com LayoutBuilder (painel some primeiro abaixo de `shellMinWidthWithPanel`, nav nunca sobreposta), `LeftNav`/`NavEntry` (1 entrada "Catalog", ativa via `sectionIsActive(ShellSurface)`), `Header` (título l10n + slot `connection-indicator-slot`), `RightPanel` (seção Connection + slot, hairline, seção Catalog com contagem + última atualização), `shell_state.dart` (`ShellSurface` + keepAlive notifier) e chaves l10n (`appTitle`, `navCatalog`, `sectionConnection`, `sectionCatalog`, `catalogCount`, `catalogLastUpdated`).
**Where**: `desktop/lib/shell/`
**Depends on**: T1, T2
**Reuses**: tokens/theme (T1), `catalogProvider` (T2), `AppLocalizations` gerado, `DateFormat.Hm()` do `intl`
**Requirement**: CAT-01, CAT-02, CAT-03, CAT-04, CAT-05, CAT-06

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Shell renderiza nav/header/conteúdo/painel (CAT-01); larguras = tokens 220/300, dentro das faixas da spec (CAT-02)
- [x] Janela < 1000px → painel some antes de qualquer sobreposição da nav; restaurar → painel volta sem novo request (CAT-03 + edge)
- [x] Exatamente 1 entrada de nav com styling de ativo derivado de `ShellSurface` (CAT-04)
- [x] Header com título l10n + `Key('connection-indicator-slot')` reservado (CAT-05)
- [x] Painel: bloco Connection + slot, hairline, bloco Catalog com contagem e última atualização só quando existir (CAT-06)
- [x] `build_runner` executado; gate check passes (comando do gate Build)
- [x] Test count: 11 novos testes; nenhum teste existente removido

**Tests**: widget
**Gate**: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test`
**Commit**: `feat(LGN-96): add app shell with nav header content and right panel`

---

### T4: Add catalog screen with loading, empty and error states

**What**: `CatalogScreen` (ConsumerWidget) renderizando `CatalogState`: loading = `LinearProgressIndicator` + `catalogLoading` (nunca empty-state), vazio = uma linha `catalogEmpty`, erro sem cache = mensagem mapeada + `OutlinedButton` `actionRetry`, lista = `Wrap` de cards (nome 15px, id 12px, botão `catalogCreateAction`) + erro inline de refresh acima da lista; chaves l10n `catalogLoading`, `catalogEmpty`, `actionRetry`, `catalogCreateAction`.
**Where**: `desktop/lib/catalog/catalog_screen.dart`
**Depends on**: T3
**Reuses**: `catalogProvider` (estado single-flight já provado), tokens (card radius/hairline/controlHeight), `errorCopy`/`failureCopy`
**Requirement**: CAT-08, CAT-09, CAT-10, CAT-11, CAT-12, CAT-19

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Com transporte engatellado, loading visível e empty-state ausente (CAT-08)
- [x] N cards na ordem da API, cada um com o nome (CAT-09); nome vazio renderiza com layout íntegro (edge)
- [x] Lista vazia → uma linha de texto localizada, sem arte/ícone (CAT-10)
- [x] Erro inicial → mensagem mapeada + Retry; Retry = exatamente 1 nova chamada (CAT-11/12)
- [x] Refresh falho → lista permanece + erro inline + Retry (CAT-19); refresh em voo → progresso sem sumir a lista
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [x] Test count: 10 novos testes; nenhum teste existente removido

**Tests**: widget
**Gate**: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
**Commit**: `feat(LGN-96): add catalog screen with loading empty and error states`

---

### T5: Add creation dialog with shallow navigation

**What**: `CreateSheetDialog` (AlertDialog radius 4: título `createSheetTitle`, label + `Text(templateId, key: 'create-sheet-template-id')`, ação `actionClose`) aberto com `showDialog(barrierDismissible: true)` — fluxo na tela marca `ShellSurface.creationDialog` e restaura em `finally`; chaves l10n `createSheetTitle`, `createSheetTemplateLabel`, `actionClose`.
**Where**: `desktop/lib/catalog/create_sheet_dialog.dart`
**Depends on**: T4
**Reuses**: cards da T4 (tap/criar), `shell_state` da T3, dismiss framework (`DismissIntent` gated em `barrierDismissible` — `routes.dart:981`)
**Requirement**: CAT-15, CAT-16, CAT-17

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Tap no card ou em "Create sheet" abre o diálogo com o id do template clicado (CAT-15)
- [ ] Esc e "Close" voltam ao catálogo com lista cacheada e 0 novas chamadas de lista (CAT-16)
- [ ] Exatamente 1 push de rota acima do catálogo (NavigatorObserver) — navegação rasa (CAT-17)
- [ ] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [ ] Test count: ≥5 novos testes; nenhum teste existente removido

**Tests**: widget
**Gate**: full
**Commit**: `feat(LGN-96): add creation dialog with shallow navigation`

---

### T6: Add shortcut registry and final app wiring

**What**: `shortcut_registry.dart` (`enum AppShortcut { refreshCatalog, closeOverlay }`, `defaultShortcuts` F5/Esc, `appShortcuts()` com spread de `WidgetsApp.defaultShortcuts`, `RefreshCatalogIntent` + `appActions(ref)` com guarda de `ShellSurface.catalog`) e ligação final do `main.dart` (theme, delegates/supportedLocales, shortcuts, actions, `home: AppShell(content: CatalogScreen())`); reescrever o `widget_test.dart` placeholder como testes de boot/AC.
**Where**: `desktop/lib/shell/shortcut_registry.dart`
**Depends on**: T5
**Reuses**: `WidgetsApp.defaultShortcuts`/`defaultActions` (SDK os substitui, não merge — `widgets/app.dart:1820`), `shell_state` (T3), `catalogProvider` (T2), app theme (T1), `AppLocalizations.localizationsDelegates` gerado
**Requirement**: CAT-18 (boot: CAT-01, CAT-05, CAT-07; edge: F5 fora do catálogo)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Registry contém F5→`refreshCatalog` e Esc→`closeOverlay`; `appShortcuts()` preserva os defaults do framework (test asserts)
- [ ] F5 no catálogo re-requesta e substitui a lista (CAT-18); F5 com diálogo aberto → 0 chamadas (edge); falha do refresh mantém lista + erro inline (CAT-19 in situ)
- [ ] Boot: shell + header slot + exatamente 1 `GET /templates` na primeira exibição (CAT-01/05/07)
- [ ] `widget_test.dart` placeholder reescrito (nenhum teste órfão de tela placeholder)
- [ ] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [ ] Test count: ≥8 novos testes; nenhum teste existente removido

**Tests**: unit + widget
**Gate**: full
**Commit**: `feat(LGN-96): add shortcut registry and final app wiring`

---

## Phase Execution Map

```
Phase 1 → Phase 2 → Phase 3

Phase 1:  T1   T2   (independentes)
Phase 2:  T3 → T4
Phase 3:  T5 → T6

T1 → T3
T2 → T3
T4 → T5
```

As três últimas linhas são as arestas entre phases (Phase 1 → Phase 2 e Phase 2 → Phase 3); cada seta do bloco corresponde a um `Depends on` declarado no corpo da task apontada.

Execution is strictly sequential - there is no intra-phase parallelism. A single agent (or batch worker) works one task at a time, in order.

**How phase-based execution works:**

At Execute, the agent counts total tasks and packs phases into **task-budgeted batches** (~7 tasks per worker, whole phases). The cut only ever lands on a phase boundary - a phase is never split across workers. With 6 tasks in 3 phases the agreed packing is: **Batch 1 = Phase 1 + Phase 2 (T1–T4)**, **Batch 2 = Phase 3 (T5–T6)**, run sequentially (batch 2 starts only after batch 1 reports all tasks complete). After the final task the orchestrator dispatches the fresh **Verifier** automatically (author ≠ verifier). See [sub-agents.md](references/sub-agents.md) for the full model.

**The orchestrating agent's role during Execute:**
1. Count total tasks, pack phases into batches - batch sub-agents already confirmed by the user
2. Dispatch the next batch (to a worker)
3. Receive the compact batch summary
4. Update tasks.md with results
5. If the batch summary shows all tasks complete: proceed to the next batch
6. If a task failed: decide fix/escalate before dispatching the next batch

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Add theme tokens and app theme | 2 arquivos coesos (tokens + theme) de um mesmo conceito | ✅ Granular |
| T2: Add catalog provider with in-memory cache and single-flight | 1 componente de estado + seu provider de client + 1 helper de copy | ✅ Granular |
| T3: Add app shell with nav, header, content and right panel | 1 componente (shell) + subviews coesas do shell | ✅ Granular |
| T4: Add catalog screen with loading, empty and error states | 1 componente (tela) | ✅ Granular |
| T5: Add creation dialog with shallow navigation | 1 componente (diálogo) + fluxo de abertura | ✅ Granular |
| T6: Add shortcut registry and final app wiring | 1 registry + wiring do `main.dart` | ✅ Granular |

**Granularity check**: ✅ nenhuma task nomeia múltiplos arquivos no `Where`; cada uma entrega um conceito coeso.

## Diagram-Definition Cross-Check

| Task | Depends on (corpo) | Diagrama mostra | Status |
| ---- | ------------------ | --------------- | ------ |
| T1 | None | — | ✅ Match |
| T2 | None | — | ✅ Match |
| T3 | T1, T2 | `T1 → T3`, `T2 → T3` (diagrama da Phase 1) | ✅ Match |
| T4 | T3 | `T3 → T4` (Phase 2) | ✅ Match |
| T5 | T4 | `T4 → T5` (Phase Execution Map) | ✅ Match |
| T6 | T5 | `T5 → T6` (Phase 3) | ✅ Match |

Nota: o validador trata todas as tasks como mesma fase (o `Task Breakdown` vem após os headers de fase), então a paridade global foi conferida: arestas do diagrama = {T1→T3, T2→T3, T3→T4, T4→T5, T5→T6} = dependências declaradas. `T5` depende de `T4` — a aresta `T4 → T5` precisa existir em algum bloco com `->`: incluída abaixo no mapa de execução.

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1: theme tokens + app theme | theme (unit) | unit | unit | ✅ OK |
| T2: catalog provider + client provider + failureCopy | provider (unit) | unit | unit | ✅ OK |
| T3: app shell + shell_state + l10n | shell (widget) | widget | widget | ✅ OK |
| T4: catalog screen + l10n | screen (widget) | widget | widget | ✅ OK |
| T5: create dialog + fluxo | navegação (widget) | widget | widget | ✅ OK |
| T6: shortcut registry + main wiring | shortcuts/wiring (unit + widget) | unit + widget | unit + widget | ✅ OK |

Nenhuma task usa `Tests: none`.
