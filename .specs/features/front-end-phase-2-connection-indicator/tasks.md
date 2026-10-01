# [Front End] Fase 2 — Indicador de conexão: health check periódico (online/offline) — Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/front-end-phase-2-connection-indicator/design.md`
**Status**: Approved

---

## Test Coverage Matrix

> Gerada do codebase, guidelines do projeto e spec — confirmar antes do Execute. Guidelines encontrados: `AGENTS.md` (raiz — testes obrigatórios, edge cases primeiro, tabela-driven, inglês, testes junto ao código), `desktop/AGENTS.md` (regras visuais/interação), `docs/TDD-desktop-mvp.md` §Estratégia de Testes (unit >80% camadas não-widget, widget para UI, contrato self-skip sem API), `.github/workflows/ci-flutter.yml` (analyze → test → build windows), `desktop/analysis_options.yaml` (`flutter_lints`).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Scheduler + connection provider (`desktop/lib/connection/`) | unit (transporte `MockClient`, scheduler manual) | 1:1 com CONN-01–07, CONN-12, CONN-13; edges: 2xx sem `data.status`, single-flight com completer, transporte/404/500 → offline, `lastSuccess` preservado, tráfego de catálogo não mexe no agendamento, `defaultInterval` = 15s, cancel/duplo-start do timer | `desktop/test/connection/connection_provider_test.dart`, `desktop/test/connection/health_scheduler_test.dart` | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter test` |
| Widgets de conexão — indicator + detail (`desktop/lib/connection/`) | widget | 1:1 com CONN-02, CONN-06, CONN-08, CONN-09, CONN-11; três estados renderizados via l10n, alinhamento no slot do header, update sem ação do usuário, cores do scheme (offline = `error`), "never" → timestamp → timestamp preservado na falha | `desktop/test/connection/connection_indicator_test.dart`, `desktop/test/connection/connection_detail_test.dart` | idem |
| Wiring de boot/shell/tema (`desktop/lib/shell/`, `desktop/lib/main.dart`) | widget | boot emite exatamente 1× `GET /health` + 1× `GET /templates` (contagem por path); slot CAT-05/CAT-06 intacto; `app_theme_test` sem rede real | `desktop/test/app_boot_test.dart`, `desktop/test/shell/app_shell_test.dart`, `desktop/test/theme/app_theme_test.dart` | idem |
| Navegação in-situ com offline | widget | 1:1 com CONN-10 + CONN-08 (todas as superfícies): com offline, tap abre diálogo, Esc fecha, F5 refresha, indicator + detail visíveis com diálogo aberto | `desktop/test/connection/connection_navigation_test.dart` | idem |
| Strings l10n (`desktop/lib/l10n/app_en.arb`) | widget (assert via render) | as 5 chaves novas renderizam em ≥1 teste; nenhum literal de UI nos widgets (CONN-11) | via suites acima | idem |
| Specs (`.specs/`) | none | validators exit 0 | `.specs/**` | `validate_spec.py` / `validate_tasks.py` |

## Gate Check Commands

> Geradas do codebase — confirmar antes do Execute. `flutter` fica em `$HOME/flutter/bin` (exportar no PATH do shell de execução).

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | tasks só com testes unit já escritos | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter test` |
| Full | toda task que adiciona/muda código Dart (padrão do repo: analyze limpo antes de commit) | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test` |
| Build | tasks que tocam codegen Riverpod ou l10n (`lib/*.g.dart`, `app_localizations*.dart`) — T1, T2 | `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && flutter gen-l10n && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test` + o CI valida (`windows-latest`: analyze → test → build windows; **build Windows é impossível em dev Linux, é exclusivo do CI**) |

---

## Execution Plan

Phases são ordenadas e rodam em sequência — cada phase completa antes da próxima; tasks dentro da phase rodam em ordem.

### Phase 1: State core

Scheduler injetável + notifier de estado (a máquina de estados pura, testada sem widgets).

```
T1
```

### Phase 2: Header e painel

Os dois renderizadores do estado, cada um preenchendo o slot que o catálogo reservou; T2 também deixa as suítes existentes verdes diante do health check de boot.

```
T1 → T2
T1 → T3
T2 → T3
```

### Phase 3: Garantias in-situ

Prova ponta a ponta no app ligado de que offline não bloqueia nada e o indicador aparece em toda superfície.

```
T2 → T4
T3 → T4
```

---

## Task Breakdown

### T1: Add connection provider with injectable health scheduler

**What**: Criar `health_scheduler.dart` (`abstract interface class HealthScheduler` com `start(onTick)`/`cancel`, `PeriodicHealthScheduler` com `defaultInterval = 15s`, `start` idempotente, e `healthSchedulerProvider` keepAlive com `ref.onDispose(cancel)`) + `connection_provider.dart` (`enum ConnectionStatus { checking, online, offline }`, `ConnectionState(status, lastSuccess)`, notifier keepAlive `Connection`: `Future.microtask(_check)` no `build` (primeiro check no boot), single-flight `_check` lendo `ref.read(apiClientProvider).health()`, sucesso → `online` + `DateTime.now()`, qualquer throw → `offline` mantendo `lastSuccess`) + helper de teste `test/connection/manual_health_scheduler.dart` + testes unitários (`connection_provider_test.dart`, `health_scheduler_test.dart`); regenerar `.g.dart`.
**Where**: `desktop/lib/connection/`
**Depends on**: None
**Reuses**: padrão de `desktop/lib/catalog/catalog_provider.dart` (keepAlive + microtask + single-flight), `apiClientProvider`, harness `MockClient`/`ProviderContainer` de `desktop/test/catalog/catalog_provider_test.dart`
**Requirement**: CONN-01, CONN-02, CONN-03, CONN-04, CONN-05, CONN-06, CONN-07, CONN-12, CONN-13 (lado provider de CONN-09)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] Estado inicial = `checking` + `lastSuccess` null; o check de boot dispara exatamente 1× sem nenhum tick (CONN-01, CONN-02)
- [x] Tabela: 200 `healthy` → online; 200 `degraded` → online (CONN-13); falha de transporte → offline; 404 → offline; 500 → offline; 2xx sem `data.status` → offline (CONN-04 + edge do parse defensivo)
- [x] Single-flight: tick durante voo engatillado → exatamente 1 requisição; estado mantém o valor anterior até resolver (CONN-05, CONN-06)
- [x] `lastSuccess` null antes do 1º sucesso, preenchido no sucesso e preservado em falhas posteriores (edge "never"/timestamp do CONN-09)
- [x] `listTemplates()` não emite health request e não reinicia o agendamento (CONN-07)
- [x] Scheduler: `defaultInterval == 15s`; dispara no intervalo (~20ms em teste); `cancel()` para; segundo `start()` não duplica ticks (CONN-12)
- [x] `flutter pub get` + `flutter gen-l10n` + `dart run build_runner build --delete-conflicting-outputs` executados e `.g.dart` commitados
- [x] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && flutter gen-l10n && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test`
- [x] Test count: 18 novos testes (suíte 220 passed / 4 skipped); nenhum teste existente removido

**Tests**: unit
**Gate**: build
**Commit**: `feat(LGN-98): add connection provider with injectable health scheduler`

---

### T2: Show the connection indicator in the header

**What**: Adicionar as 5 chaves l10n (`connectionChecking`, `connectionOnline`, `connectionOffline`, `connectionLastCheck`, `connectionLastCheckNever`) + `flutter gen-l10n`; criar `connection_indicator.dart` (ConsumerWidget: dot 8px `AppSpacing.sm` + label `bodySmall`, cores `primary`/`error`/`onSurfaceVariant`, `Key('connection-indicator')`); preencher o slot do `Header` com `child: Align(alignment: centerRight, child: ConnectionIndicator())` mantendo chave e largura (CAT-05 intacto); adaptar as suítes quebradas pelo health check de boot: harnesses `pumpApp`/`pumpShell` ganham override do `healthSchedulerProvider` com scheduler manual e contagem por path (`healthCount == 1`), `app_theme_test` ganha override de `apiClientProvider` com `MockClient` (elimina a chamada real de rede que ele já faz hoje); testes widget do indicator.
**Where**: `desktop/lib/connection/`
**Depends on**: T1
**Reuses**: slot reservado em `desktop/lib/shell/app_shell.dart:88`, tokens (`connectionSlotWidth`, `AppSpacing.sm`/`xs`), padrão de placeholder do `app_en.arb` (`catalogLastUpdated`), harnesses existentes
**Requirement**: CONN-02, CONN-06, CONN-08, CONN-11

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [x] O `Header` renderiza `ConnectionIndicator` alinhado à direita dentro do slot `connection-indicator-slot` (chave + largura 160 preservadas — asserções CAT-05 continuam verdes)
- [x] checking → "Checking…", online → "Online", offline → "Offline", todos vindos do l10n; cores do scheme (offline = `error`) (CONN-02, CONN-08, CONN-11)
- [x] Resultado do health atualiza o widget sem ação do usuário (CONN-06)
- [x] Boot emite exatamente 1× `GET /health` (asserção por path) e os harnesses com scheduler manual mantêm as contagens determinísticas (CONN-01/CONN-12 in situ)
- [x] `app_theme_test` não faz mais chamada de rede real (override com `MockClient`)
- [x] Saída do `gen-l10n` commitada; gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && flutter gen-l10n && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test`
- [x] Test count: 5 novos testes + 8 atualizados (boot/shell/theme) — suíte 225 passed / 4 skipped; nenhum teste existente removido ou enfraquecido

**Tests**: widget
**Gate**: build
**Commit**: `feat(LGN-98): show the connection indicator in the header`

---

### T3: Show connection detail in the right panel

**What**: Criar `connection_detail.dart` (ConsumerWidget: linha de estado = dot + label com estilo explícito `bodyMedium`; linha de última checagem = `connectionLastCheckNever` antes do 1º sucesso, `connectionLastCheck(DateFormat.Hm())` depois) e preencher o slot `connection-panel-slot` do `RightPanel` com `child: ConnectionDetail()` (chave CAT-06 intacta); testes widget do detail.
**Where**: `desktop/lib/connection/`
**Depends on**: T1, T2
**Reuses**: chaves l10n da T2, padrão `DateFormat.Hm()` + placeholder de `right_panel.dart:56` (`catalogLastUpdated`), estilo explícito pós-UAT (regressão T8)
**Requirement**: CONN-09, CONN-11

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Antes do 1º sucesso: estado atual + "Last check never" (CONN-09)
- [ ] Após sucesso: "Last check HH:mm" com o timestamp do último sucesso; falha seguinte mantém o timestamp e mostra offline (CONN-09)
- [ ] Linha de estado acompanha o provider (`checking` → `online`/`offline`) com strings do l10n, estilos explícitos (nunca fallback 48px) (CONN-11)
- [ ] Slot `connection-panel-slot` segue com chave e asserções CAT-06 verdes
- [ ] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [ ] Test count: 4 novos testes; nenhum teste existente removido

**Tests**: widget
**Gate**: full
**Commit**: `feat(LGN-98): show connection detail in the right panel`

---

### T4: Prove offline does not block navigation

**What**: Testes widget de ponta a ponta sobre o app ligado (`ProviderScope` + `LegnarisApp`, transporte `MockClient` com `/health` falhando e scheduler manual): com o estado `offline`, o header mostra "Offline" e o painel "Last check never"/timestamp; tap no card abre o diálogo; Esc fecha; F5 refresha o catálogo; indicator + detail continuam visíveis com o diálogo aberto (toda superfície); e o caminho feliz inverso (health voltando a responder → online na próxima checagem).
**Where**: `desktop/test/connection/`
**Depends on**: T2, T3
**Reuses**: harness `pumpApp` de `desktop/test/app_boot_test.dart`, scheduler manual da T1, widgets das T2/T3
**Requirement**: CONN-10, CONN-08, CONN-06 (recuperação offline → online)

**Tools**:

- MCP: NONE
- Skill: NONE

**Done when**:

- [ ] Com offline: tap no card abre o diálogo, Esc fecha, F5 re-requesta — nada bloqueado (CONN-10)
- [ ] Indicator e detail visíveis em todas as superfícies, inclusive com o diálogo aberto (CONN-08)
- [ ] Falha de boot → offline; health voltando a responder → online na checagem seguinte sem reiniciar o app (edge de recuperação, CONN-06)
- [ ] Gate check passes: `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter analyze && flutter test`
- [ ] Test count: 6 novos testes; nenhum teste existente removido

**Tests**: widget
**Gate**: full
**Commit**: `test(LGN-98): prove offline does not block navigation`

---

## Phase Execution Map

```
Phase 1 → Phase 2 → Phase 3

Phase 1:  T1
Phase 2:  T1 → T2
          T1 → T3
          T2 → T3
Phase 3:  T2 → T4
          T3 → T4
```

Execution is strictly sequential - there is no intra-phase parallelism. A single agent works one task at a time, in order.

**How phase-based execution works:**

At Execute, the agent counts total tasks and packs phases into **task-budgeted batches** (~7 tasks per worker, whole phases). With 4 tasks in 3 phases this fits a **single batch → execute inline** (no sub-agents), one task at a time. After the final task the orchestrator dispatches the fresh **Verifier** automatically (author ≠ verifier). See [sub-agents.md](references/sub-agents.md) for the full model.

**The orchestrating agent's role during Execute:**
1. Count total tasks and pack phases into batches (4 tasks → inline execution)
2. Execute T1 → T2 → T3 → T4 in order (tests → implement → gate → adequacy review → atomic commit)
3. Update tasks.md with results in each task's commit
4. If a task fails: fix before starting the next
5. After T4: dispatch the Verifier automatically, then run `validate_state.py`

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Add connection provider with injectable health scheduler | 1 camada de estado (scheduler + notifier) coesa de um mesmo conceito | ✅ Granular |
| T2: Show the connection indicator in the header | 1 componente (indicator) + fill do slot que o reserva + chaves l10n que ele consome + adaptação das suítes que o boot poll quebra | ✅ Granular |
| T3: Show connection detail in the right panel | 1 componente (detail) + fill do slot que o reserva | ✅ Granular |
| T4: Prove offline does not block navigation | 1 superfície de garantia (testes in-situ do AC CONN-10/08) | ✅ Granular |

**Granularity check**: ✅ nenhuma task nomeia múltiplos conceitos no `Where` (cada uma entrega um componente/coorte coeso); T2 é a maior e ainda gira em torno de um único conceito — o indicador no header.

## Diagram-Definition Cross-Check

| Task | Depends on (corpo) | Diagrama mostra | Status |
| ---- | ------------------ | --------------- | ------ |
| T1 | None | — | ✅ Match |
| T2 | T1 | `T1 → T2` (Phase 2) | ✅ Match |
| T3 | T1, T2 | `T1 → T3`, `T2 → T3` (Phase 2) | ✅ Match |
| T4 | T2, T3 | `T2 → T4`, `T3 → T4` (Phase 3) | ✅ Match |

Nota: o validador mapeia todas as `### Tn:` para a última phase header anterior ao `Task Breakdown` (Phase 3), então a paridade de diagrama vale globalmente — arestas do diagrama = {T1→T2, T1→T3, T2→T3, T2→T4, T3→T4} = exatamente o conjunto de `Depends on` declarados.

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1: scheduler + connection provider | provider/scheduler (unit) | unit | unit | ✅ OK |
| T2: l10n + indicator + header fill + adaptação de suítes | widget + wiring (widget) | widget | widget | ✅ OK |
| T3: detail + panel fill | widget (widget) | widget | widget | ✅ OK |
| T4: navegação in-situ offline | widget (widget) | widget | widget | ✅ OK |

Nenhuma task usa `Tests: none`.
