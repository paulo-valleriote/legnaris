# [Front End] Fase 0 — Setup: Monorepo Flutter + CI de Build Windows

> Fonte: [LGN-89](https://paulovalleriotedev.atlassian.net/browse/LGN-89) (épico de fase, sob [LGN-88](https://paulovalleriotedev.atlassian.net/browse/LGN-88)) · `docs/TDD-desktop-mvp.md` (aprovado — fonte da verdade) · Planejamento: Fase 0 (1 dia útil), não depende de nada, bloqueia as Fases 1–5.
> Status: **Approved** — 2026-09-29 · premissas confirmadas pelo mantenedor.

## Problem Statement

O frontend desktop aprovado no TDD (`docs/TDD-desktop-mvp.md`) ainda não existe nem como esqueleto: o monorepo não tem `desktop/`, toolchain Flutter, nem CI. O desenvolvimento ocorre em Linux e o alvo de uso é Windows — e o Flutter não suporta cross-compile para Windows —, então sem um runner Windows no CI a primeira validação do build real só aconteceria no fim do MVP (risco mapeado no TDD: "validar a cross-build cedo, não no fim"). Além disso, o workflow Go existente (`ci-lint.yml` — Lint & Race) dispara em todo PR sem filtro de path, então o Flutter só entra no repo junto com filtros que isolem as duas toolchains.

## Goals

- [ ] `desktop/` vivo no monorepo: projeto Flutter runnable (plataformas windows + linux), Riverpod 3 com codegen, `flutter_lints`, analyze e test verdes.
- [ ] Workflow CI Flutter em `windows-latest` gerando o build (.exe) e publicando artifact em todo PR que toca `desktop/**` — desde o primeiro commit.
- [ ] Pipelines isolados por path: Go no raiz do repo, Flutter em `desktop/**` — mudança numa toolchain não dispara a outra.

## Out of Scope

Explicitamente excluído. Documentado para prevenir scope creep.

| Feature | Reason |
| ------- | ------ |
| Features do app (API client, catálogo, UI Engine, ciclo da ficha, local store) | Fases 1–5 do TDD — épicos LGN-90–94 |
| Build/distribuição Linux, MSIX/instalador, code signing, auto-update | TDD: Linux é secundário sem compromisso; empacotamento é Fase 5/V2 |
| Identidade visual (paleta, tipografia, componentes) | V2 (TDD) — skeleton usa o visual padrão do Flutter |
| Trigger de CI em pushes fora de `main` | PRs já cobrem a validação; só `main` publica artifact |
| Refactors no workflow Go além do filtro de path | Escopo = isolamento por path; o restante do comportamento atual é preservado |

---

## Assumptions & Open Questions

Toda ambiguidade está resolvida ou registrada aqui — nada fica silenciosamente incerto.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | -------------- | --------- | ---------- |
| Versão/estilo do Riverpod | Riverpod 3.x estável com codegen: `flutter_riverpod` + `riverpod_annotation` + `riverpod_generator` + `build_runner`; `.g.dart` commitado | Setup recomendado pela doc oficial do Riverpod 3; define o estilo de providers das Fases 1–5 desde o dia 1 e evita migração; o TDD fixa "Riverpod" sem versão | y |
| Lints | `flutter_lints` (baseline oficial) via `analysis_options.yaml` em `desktop/` | Baseline da doc do Flutter; analyze limpo é critério do épico; suites mais rígidas (ex.: very_good_analysis) podem vir depois sem quebra | y |
| Setup do Flutter no CI | `subosito/flutter-action` com `flutter-version-file: desktop/pubspec.yaml`, channel stable | Action padrão de facto (repo oficial, alta reputação); pin único no pubspec = build reprodutível entre dev (Linux) e CI (Windows) | y |
| Triggers do workflow Flutter | `pull_request` (todas as branches) + `push` em `main` | PR valida cada mudança (mesmo padrão do workflow Go hoje); push em `main` publica artifact instalável a cada merge — uso pessoal da V1 baixa o .exe direto do Actions | y |
| Mecânica dos filtros de path | Flutter: `paths: [desktop/**, .github/workflows/ci-flutter.yml]`; Go (ci-lint.yml): allowlist `paths: [**/*.go, cmd/**, internal/**, shared/**, deploy/**, go.mod, go.sum, Dockerfile, .github/workflows/ci-lint.yml]`; spec-lint (`ci-spec-lint.yml`): `paths: [**/*.go, **/*.dart, scripts/**, .specs/**, .github/workflows/ci-spec-lint.yml]` | Critério do épico (isolamento por toolchain); cada workflow inclui o próprio arquivo para autovalidar mudanças nele. O Go usa allowlist — `paths` e `paths-ignore` não coexistem no mesmo evento — de modo que mudanças de `docs/`, `web/` ou `.specs/`, que não tocam a toolchain Go, também ficam fora do pipeline. **Mecanismo revisado 2026-10-15**; a `validation.md` do FE0 preserva o registro histórico do mecanismo anterior (`paths-ignore`) | y |
| Conteúdo do skeleton | App mínimo runnable: tela placeholder + 1 provider `@riverpod` trivial + widget test — zero features | A toolchain precisa ser exercitada desde o dia 1 (analyze/test/build têm o que validar); features são das Fases 1–5 (LGN-90–94) | y |
| Plataformas do projeto Flutter | `windows` (alvo, build no CI) + `linux` (dev-run do mantenedor; sem build CI) | Dev é Linux e precisa rodar/hot-reload local para desenvolver as Fases 1–5; TDD: Linux entra como secundário, sem compromisso | y |
| Artifact do build | Upload da pasta `build/windows/x64/runner/Release` inteira (exe + DLLs + data), via `actions/upload-artifact` | O .exe sozinho não roda sem as DLLs e a pasta data; a Release zipada é o "instalável" mínimo da V1; MSIX/instalador é Fase 5/V2 | y |

**Open questions:** none — todas as ambiguidades foram resolvidas pelo TDD aprovado (Questões #1, #2, #5) ou registradas como premissas acima.

---

## User Stories

### P1: Bootstrap do app Flutter em `desktop/` ⭐ MVP

**User Story**: Como mantenedor (dev em Linux, usuário em Windows), quero o projeto Flutter vivo em `desktop/` com Riverpod e lints configurados, para que as Fases 1–5 (LGN-90–94) nasçam sobre toolchain pronta e convenções ativas.

**Why P1**: o épico bloqueia todas as demais fases — sem projeto não há nada para CI validar.

**Acceptance Criteria**:

1. WHEN o repo é inspecionado THEN o `desktop/` SHALL conter um projeto Flutter completo (pubspec.yaml, lib/, test/) com as plataformas `windows` e `linux` habilitadas — sem tocar nos pacotes Go do raiz. (FE0-01)
2. WHEN o app é executado com `flutter run` em `desktop/` THEN a janela placeholder SHALL abrir sem crash, com o app montado dentro de um `ProviderScope`. (FE0-02)
3. WHEN `flutter analyze` roda em `desktop/` THEN o comando SHALL terminar com exit 0, sob o baseline `flutter_lints`. (FE0-03)
4. WHEN `flutter test` roda em `desktop/` THEN o widget test do skeleton SHALL passar (exit 0). (FE0-04)
5. WHEN o build gera artefatos locais THEN o Git SHALL ignorá-los via `.gitignore` em `desktop/` (build/, .dart_tool/ etc.) — nada gerado é commitado. (FE0-05)
6. WHEN um provider é definido no skeleton THEN ele SHALL usar a sintaxe codegen do Riverpod 3 (`@riverpod`, `.g.dart` gerado e commitado). (FE0-06)

**Independent Test**: em `desktop/`, rodar `flutter run`, `flutter analyze` e `flutter test` — janela abre, ambos os comandos verdes, sem nenhuma mudança fora de `desktop/` (exceto workflows/filters da história 3).

---

### P1: CI de build Windows desde o primeiro commit ⭐ MVP

**User Story**: Como mantenedor, quero todo PR que toca `desktop/**` rodando analyze, test e build Windows num runner `windows-latest` e publicando o artifact, para que a cross-build (impossível na minha máquina Linux) seja validada desde o dia 1 e bloqueie o merge quando quebrar.

**Why P1**: risco central do TDD — adiar a validação do build Windows para o fim do MVP descobriria quebra tarde demais.

**Acceptance Criteria**:

1. WHEN o primeiro commit que introduz `desktop/` chega ao repo THEN esse commit SHALL incluir o workflow Flutter completo e ativo — nunca um `desktop/` sem CI. (FE0-07)
2. WHEN um PR altera arquivos sob `desktop/**` THEN o workflow SHALL executar em `windows-latest` os steps `flutter analyze`, `flutter test` e `flutter build windows`. (FE0-08)
3. WHEN o job completa com sucesso THEN o workflow SHALL publicar um artifact contendo a pasta `build/windows/x64/runner/Release` inteira (exe + DLLs + data). (FE0-09)
4. IF qualquer step do job falha (analyze, test ou build) THEN o workflow SHALL falhar com exit ≠ 0, o PR SHALL ficar vermelho e nenhum artifact SHALL ser publicado. (FE0-10)
5. WHEN a versão do SDK muda no `desktop/pubspec.yaml` THEN o CI SHALL usar a nova versão automaticamente — `environment.flutter` do pubspec é a fonte única do pin. (FE0-11)
6. WHEN um push em `main` altera `desktop/**` THEN o workflow SHALL rodar os mesmos jobs e publicar o artifact do build. (FE0-12)

**Independent Test**: abrir um PR tocando só `desktop/` e ver o job Windows verde com artifact baixável; quebrar o analyze num branch de teste e ver o PR vermelho sem artifact.

---

### P1: Isolamento de pipelines por path ⭐ MVP

**User Story**: Como dev do monorepo, quero workflows filtrados por path (Go no raiz, Flutter em `desktop/**`), para que mudança numa toolchain não dispare o pipeline da outra — CI rápido e sinal limpo.

**Why P1**: critério de aceite do épico; condição para o Flutter entrar no monorepo sem poluir o CI Go existente.

**Acceptance Criteria**:

1. WHEN um PR altera apenas `desktop/**` THEN o workflow Go existente (Lint & Race) SHALL ser pulado e o workflow Flutter SHALL rodar. (FE0-13)
2. WHEN um PR altera apenas código Go do raiz (cmd/, internal/, shared/, go.mod etc.) THEN o workflow Flutter SHALL ser pulado e o workflow Go SHALL rodar. (FE0-14)
3. WHEN um PR altera os dois domínios THEN ambos os workflows SHALL rodar. (FE0-15)
4. WHEN um PR altera apenas o próprio arquivo do workflow Flutter (`.github/workflows/ci-flutter.yml`) THEN o workflow Flutter SHALL rodar (autovalidação da mudança). (FE0-16)

**Independent Test**: abrir três PRs de teste — só Go, só Flutter, misto — e conferir exatamente quais workflows disparam em cada um.

---

## Edge Cases

- The `desktop/pubspec.yaml` SHALL declarar `environment.flutter` (pin obrigatório — sem ele o CI não é reprodutível e FE0-11 não se sustenta).
- IF o `.g.dart` commitado divergir das fontes THEN `flutter analyze`/`flutter test` SHALL falhar no CI — drift detectado pelo gate existente, sem step extra de `build_runner` no CI nesta fase.
- WHEN um PR altera apenas arquivos fora de `desktop/**` e fora da toolchain Go (ex.: `docs/`, `web/`, `.specs/`) THEN nem o workflow Go nem o Flutter SHALL disparar — a allowlist do Go só casa paths da toolchain Go e do stack de deploy. (**mecanismo revisado 2026-10-15**: substitui o edge case anterior, que previa o Go disparando para `docs/`; a `validation.md` do FE0 preserva esse registro histórico.)
- IF o runner `windows-latest` estiver indisponível THEN o job SHALL falhar visível (PR vermelho/queued) — sem fallback silencioso para outro SO, pois build em ubuntu não valida o alvo Windows.
- Demais dimensões (auth, lifecycle de dados, observabilidade de runtime, concorrência de app) N/A neste escopo: setup de toolchain + CI, sem comportamento de produto.

---

## Requirement Traceability

Cada requisito tem ID único para rastreamento em design, tasks e validação.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------ |
| FE0-01 | P1: Bootstrap do app Flutter em `desktop/` | Execute | Verified |
| FE0-02 | P1: Bootstrap do app Flutter em `desktop/` | Execute | Verified |
| FE0-03 | P1: Bootstrap do app Flutter em `desktop/` | Execute | Verified |
| FE0-04 | P1: Bootstrap do app Flutter em `desktop/` | Execute | Verified |
| FE0-05 | P1: Bootstrap do app Flutter em `desktop/` | Execute | Verified |
| FE0-06 | P1: Bootstrap do app Flutter em `desktop/` | Execute | Verified |
| FE0-07 | P1: CI de build Windows desde o primeiro commit | Execute | Verified |
| FE0-08 | P1: CI de build Windows desde o primeiro commit | Execute | Verified |
| FE0-09 | P1: CI de build Windows desde o primeiro commit | Execute | Verified |
| FE0-10 | P1: CI de build Windows desde o primeiro commit | Execute | Verified |
| FE0-11 | P1: CI de build Windows desde o primeiro commit | Execute | Verified |
| FE0-12 | P1: CI de build Windows desde o primeiro commit | Execute | Implementing |
| FE0-13 | P1: Isolamento de pipelines por path | Execute | Verified |
| FE0-14 | P1: Isolamento de pipelines por path | Execute | Implementing |
| FE0-15 | P1: Isolamento de pipelines por path | Execute | Verified |
| FE0-16 | P1: Isolamento de pipelines por path | Execute | Implementing |

**Coverage:** 16 requisitos no total, 16 mapeados 1:1 a critérios de aceite e a tasks no `tasks.md` (T1–T5); 0 não mapeados.

---

## Success Criteria

Como sabemos que a fase foi bem-sucedida:

- [ ] Mantenedor baixa o artifact de um PR e abre o app no Windows — primeira evidência do alvo funcionando (a cross-build validada cedo, como pede o TDD).
- [ ] PR só-Go não dispara o workflow Flutter e vice-versa; PR misto dispara ambos.
- [ ] `flutter analyze` e `flutter test` terminam com exit 0 no CI desde o primeiro commit.
- [ ] Fase entregue dentro da estimativa do épico (1 dia útil).
