# [Front End] Fase 0 — Setup: Monorepo Flutter + CI de Build Windows — Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: none (escopo Medium — decisões inline na `spec.md`, premissas aprovadas)
**Status**: In Progress

**Commit atomicidade (FE0-07)**: T2 e T3 empacotam num **commit único** — o primeiro commit que introduz `desktop/` já chega com o CI completo ("nunca um `desktop/` sem CI"). T1 e T4 e T5 têm commits próprios.

**Validação remota**: FE0-08…12 e FE0-13…16 (comportamento real dos workflows em PRs) só validam na primeira execução do CI no PR. `git push` requer go-ahead explícito do mantenedor.

---

## Test Coverage Matrix

> Gerada do codebase, guidelines do projeto e spec — confirmar antes do Execute. Guidelines encontrados: `AGENTS.md` (raiz — convenções de teste, inglês obrigatório, testes junto ao código), `docs/TDD-desktop-mvp.md` §Estratégia de Testes (flutter_test, foco em edge cases, self-skip sem serviço externo). Nenhuma suíte Flutter existe ainda (território novo no repo — `desktop/` nasce aqui).

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| UI skeleton (`desktop/lib/`) | widget | Boot em `ProviderScope` sem crash (FE0-02); placeholder renderiza o valor do provider (FE0-06) | `desktop/test/**_test.dart` | `cd desktop && flutter test` |
| Provider/state (codegen Riverpod) | unit | Provider `@riverpod` retorna o valor esperado via `ProviderContainer` (FE0-06) | `desktop/test/**_test.dart` | `cd desktop && flutter test` |
| CI workflows (`.github/workflows/`) | none | Build gate only: parse YAML + revisão estrutural; o comportamento real (FE0-08…16) valida no runner do PR | - | `python3 -c "import yaml,sys; [yaml.safe_load(open(f)) for f in sys.argv[1:]]" .github/workflows/ci-lint.yml .github/workflows/ci-flutter.yml` |
| Docs/spec (`.specs/`, `docs/`) | none | - | - | `validate_spec.py` / `validate_tasks.py` exit 0 |

## Gate Check Commands

> Geradas do codebase — confirmar antes do Execute.

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | tasks só com testes unit/widget | `cd desktop && flutter test` |
| Build | fim de task com build_runner ou config-only | `cd desktop && flutter analyze && flutter test` · T4 prefixa `dart run build_runner build &&` · T3/T5 também rodam o parse YAML da matriz acima |

---

## Execution Plan

Phases são ordenadas e rodam em sequência — cada phase completa antes da próxima; tasks dentro da phase rodam em ordem.

### Phase 1: Fundação (docs)

```
T1
```

### Phase 2: Bootstrap desktop/ + CI Windows (commit atômico duplo — FE0-07)

```
T2 → T3
```

### Phase 3: Toolchain Riverpod + skeleton

```
T4
```

### Phase 4: Isolamento de pipelines Go

```
T5
```

Fechamento (não-task): Verifier sub-agent (fresh) — spec-anchored check + discrimination sensor → `validation.md` + `validate_state.py` exit 0.

---

## Task Breakdown

### Phase 1: Fundação (docs)

#### T1: Commit da spec aprovada + TDD desktop

**What**: Registrar a aprovação da spec no repositório: premissas `Confirmed? → y`, `Status → Approved`, e o commit docs com spec.md, este tasks.md e o TDD (`docs/TDD-desktop-mvp.md`, untracked — inclusão aprovada).
**Where**: `.specs/features/front-end-phase-0-setup/`
**Depends on**: None
**Reuses**: Branch naming do repo (`feat/LGN-NN-slug`)
**Requirement**: baseline da spec FE0 inteira

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [x] spec.md com todas as 8 premissas `Confirmed? → y` e `Status: Approved`
- [x] tasks.md presente com `Status: Approved`
- [x] commit docs contém exatamente: `spec.md`, `tasks.md`, `docs/TDD-desktop-mvp.md` (nada além — skill files e `docs/plans/` ficam de fora)

**Tests**: none (matriz: docs/spec → none)
**Gate**: build — `validate_spec.py` e `validate_tasks.py` exit 0
**Commit**: `docs(LGN-89): add front-end phase 0 spec and desktop tdd`

### Phase 2: Bootstrap desktop/ + CI Windows

#### T2: Projeto Flutter em `desktop/`

**What**: Criar o projeto Flutter no monorepo: `flutter create` com plataformas `windows` + `linux`, project name `legnaris`, pin de `environment.flutter` no pubspec, baseline `flutter_lints` e `.gitignore` do Flutter — analyze e test verdes.
**Where**: `desktop/`
**Depends on**: T1
**Reuses**: Padrão de convenção do repo (`AGENTS.md` — inglês, testes junto ao código)
**Requirement**: FE0-01, FE0-03, FE0-05

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [x] `desktop/` com projeto completo (pubspec.yaml, lib/, test/) e plataformas `windows` + `linux` habilitadas (FE0-01)
- [x] `environment.flutter` pinado como `3.47.5` no pubspec (edge: pin obrigatório — fonte única do CI)
- [x] `analysis_options.yaml` incluindo `flutter_lints` (FE0-03)
- [x] `.gitignore` cobrindo `build/`, `.dart_tool/` — nada gerado commitado (FE0-05)
- [x] `cd desktop && flutter analyze` exit 0 (FE0-03)
- [x] `cd desktop && flutter test` verde com o widget test do app gerado

**Tests**: widget
**Gate**: build — `cd desktop && flutter analyze && flutter test`
**Commit**: mesmo commit do T3 (FE0-07)

#### T3: Workflow CI Flutter (`ci-flutter.yml`)

**What**: Criar o workflow completo: runner `windows-latest`, `subosito/flutter-action` com `flutter-version-file: desktop/pubspec.yaml`, steps `flutter analyze` → `flutter test` → `flutter build windows`, upload do artifact `Release`, triggers `pull_request` + `push` em `main`, `paths: [desktop/**, .github/workflows/ci-flutter.yml]`.
**Where**: `.github/workflows/ci-flutter.yml`
**Depends on**: T2
**Reuses**: `ci-lint.yml` (padrão de actions checkout@v7, permissions contents: read)
**Requirement**: FE0-07, FE0-08, FE0-09, FE0-10, FE0-11, FE0-12, FE0-16

**Tools**:

- MCP: `context7` (conferir uso atual de `subosito/flutter-action` — versão da action e inputs)
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [x] job em `windows-latest` com steps em sequência: analyze, test, `flutter build windows` (FE0-08)
- [x] artifact = `desktop/build/windows/x64/runner/Release` inteiro, só publicado após build verde (FE0-09, FE0-10)
- [x] `flutter-version-file: desktop/pubspec.yaml` — sem versão duplicada no workflow (FE0-11)
- [x] triggers: `pull_request` + `push: branches: [main]` (FE0-12)
- [x] `paths: [desktop/**, .github/workflows/ci-flutter.yml]` (FE0-16 — autovalidação)
- [x] YAML parse exit 0
- [x] FE0-07: no mesmo commit do T2 — `desktop/` nunca entra sem CI

**Tests**: none (matriz: CI workflows → none; comportamento real valida no PR)
**Gate**: build — parse YAML da matriz + `cd desktop && flutter analyze && flutter test`
**Commit**: mesmo commit do T2 — `feat(LGN-89): bootstrap flutter desktop project with windows ci`

### Phase 3: Toolchain Riverpod + skeleton

#### T4: Riverpod 3 codegen + skeleton placeholder

**What**: Adicionar a toolchain Riverpod (deps + codegen) e transformar o app default em skeleton placeholder: `ProviderScope` + `ConsumerWidget` renderizando um provider `@riverpod` trivial, com `.g.dart` gerado e commitado, widget test e unit test derivados da spec.
**Where**: `desktop/lib/`
**Depends on**: T2
**Reuses**: Setup codegen da doc oficial Riverpod 3 (`riverpod_annotation` + `riverpod_generator` + `build_runner`)
**Requirement**: FE0-02, FE0-04, FE0-06

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [x] pubspec com `flutter_riverpod`, `riverpod_annotation` (deps) e `riverpod_generator`, `build_runner` (dev_deps) — premissa aprovada
- [x] `main.dart` com `ProviderScope` + placeholder consumindo o provider via `ref.watch` (FE0-02)
- [x] provider `@riverpod` com `.g.dart` gerado pelo build_runner e commitado (FE0-06)
- [x] widget test: app boota sem crash em `ProviderScope` e renderiza "Legnaris" (FE0-02, FE0-04)
- [x] unit test: `ProviderContainer` lê o provider e retorna "Legnaris" (FE0-06)
- [ ] demo manual `flutter run -d linux` abre a janela placeholder — requer toolchain apt (não bloqueia o gate; evidência local do FE0-02 é o widget test; o run manual fica registrado no Success Criteria)

**Tests**: unit, widget
**Gate**: build — `cd desktop && dart run build_runner build && flutter analyze && flutter test`
**Commit**: `feat(LGN-89): add riverpod toolchain and placeholder skeleton`

### Phase 4: Isolamento de pipelines Go

#### T5: Filtro de path no workflow Go (`ci-lint.yml`)

**What**: Adicionar `paths-ignore: [desktop/**]` ao `on.pull_request` do `ci-lint.yml` — mudança só em `desktop/**` não dispara o pipeline Go; nada mais muda no workflow.
**Where**: `.github/workflows/ci-lint.yml`
**Depends on**: T3
**Reuses**: `ci-lint.yml` atual (jobs preservados intactos)
**Requirement**: FE0-13, FE0-14, FE0-15

**Tools**:

- MCP: none
- Skill: `tlc-spec-driven` (Execute flow)

**Done when**:

- [x] `paths-ignore` com `desktop/**` no trigger `pull_request` (FE0-13)
- [x] jobs lint e race intactos — diff toca só o bloco de trigger (FE0-15: PR misto dispara ambos)
- [x] YAML parse exit 0
- [ ] validação de comportamento (PRs só-Go / só-Flutter / misto) na execução real do PR do branch

**Tests**: none (matriz: CI workflows → none)
**Gate**: build — parse YAML + `git diff ci-lint.yml` confirma só o `paths-ignore`
**Commit**: `ci(LGN-89): keep go workflow out of desktop-only changes`

---

## Phase Execution Map

```
Phase 1 → Phase 2 → Phase 3 → Phase 4

Phase 1:  T1
Phase 2:  T2 → T3   (commit atômico duplo — FE0-07)
Phase 3:  T4
Phase 4:  T5
```

Execução estritamente sequencial — sem paralelismo intra-phase. 5 tasks = 1 batch → execução inline, sem sub-agents (limite ~8).

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1: Commit da spec + TDD | 1 commit docs (3 arquivos de planejamento) | ✅ Granular |
| T2: Projeto Flutter em `desktop/` | 1 deliverable (projeto gerado + pin + lints) | ✅ Granular |
| T3: Workflow CI Flutter | 1 arquivo de workflow | ✅ Granular |
| T4: Riverpod + skeleton | 1 componente (skeleton app) + deps | ✅ Granular |
| T5: Filtro de path no Go workflow | 1 mudança de config (paths-ignore) | ✅ Granular |

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | Sem dependências (Phase 1: T1) | ✅ Match |
| T2 | T1 | T2 na Phase 2; cross-phase backward — sem seta exigida | ✅ Match |
| T3 | T2 | Phase 2: `T2 → T3` | ✅ Match |
| T4 | T2 | T4 na Phase 3; cross-phase backward — sem seta exigida | ✅ Match |
| T5 | T3 | T5 na Phase 4; cross-phase backward — sem seta exigida | ✅ Match |

Nenhuma dependência aponta para phase futura. ✅

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1 | Docs/spec | none | none | ✅ OK |
| T2 | UI skeleton (projeto default) | widget | widget | ✅ OK |
| T3 | CI workflow | none | none | ✅ OK |
| T4 | UI skeleton + provider | widget + unit | unit, widget | ✅ OK |
| T5 | CI workflow | none | none | ✅ OK |

Nenhum `Tests: none` contraria a matriz; nenhum defer de teste entre tasks. ✅

---

**Fechamento**: após T5, o Verifier sub-agent (fresh) roda automaticamente — spec-anchored check + discrimination sensor → `validation.md` (PASS/FAIL) + `validate_state.py` exit 0. Falhas viram fix tasks (máx. 3 iterações).
