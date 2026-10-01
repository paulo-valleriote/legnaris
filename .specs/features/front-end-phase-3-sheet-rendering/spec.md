# [Front End] Fase 3 — Renderização da ficha (Tela 2): agrupamento por prefixo e controle por tipo

> Spec da fase Specify (`tlc-spec-driven`) · origem: `.specs/features/front-end-phase-3-sheet-rendering/spec.md` · gate `validate_spec.py --strict`: 0 erros, 0 warnings · épico: [LGN-92](https://paulovalleriotedev.atlassian.net/browse/LGN-92) · fonte: `docs/TDD-desktop-mvp.md` + `desktop/AGENTS.md`

> Fonte: [LGN-101](https://paulovalleriotedev.atlassian.net/browse/LGN-101) · [LGN-92](https://paulovalleriotedev.atlassian.net/browse/LGN-92) (épico de fase, sob [LGN-88](https://paulovalleriotedev.atlassian.net/browse/LGN-88)) · `docs/TDD-desktop-mvp.md` (aprovado — fonte da verdade) · `desktop/AGENTS.md` (regras de construção) · wireframe Tela 2 · contrato real: `GET /templates/:id` + `FieldSchema`/`FieldGroup` da Fase 1 (`desktop/lib/api/models/template.dart`).
> Relação entre specs desta fase: entrega o núcleo do UI Engine (Tela 2). [LGN-102](https://paulovalleriotedev.atlassian.net/browse/LGN-102) (rebuild granular por campo) depende desta; [LGN-100](https://paulovalleriotedev.atlassian.net/browse/LGN-100) (preview read-only no diálogo) reusa o controle read-only aqui definido.
> Status: **Approved** — aprovada pelo mantenedor em 2026-09-30 (decisões de escopo confirmadas na discussão: Tela 2 navegável com dados sintéticos, UI Engine controlado sem estado).
> **Amend (LGN-100 follow-up, 2026-10-01):** na revisão do preview read-only do diálogo de criação, o mantenedor apontou que a Tela 2 (aberta por "Preview sheet") **ainda aceitava edição**; como não existe ficha até a Fase 4, a Tela 2 passa a renderizar **somente-leitura** (host passa `onChanged: null`, como o card da criação). Novo requisito **RND-20**; a capacidade de edição do controle/renderer (RND-06/07/12) permanece, apenas não é usada pela Tela 2 até a Fase 4.

## Problem Statement

A Fase 2 entrega o catálogo e o detalhe do template, mas **nenhuma tela renderiza a ficha em si**. A promessa central do frontend (Marco M2) é renderizar _qualquer_ template do catálogo sem código por sistema de RPG, e nada disso existe hoje: sem o UI Engine, `FieldSchema` é só um DTO e a "flexibilidade radical" do backend continua teórica. Esta spec entrega o núcleo: transformar a árvore de campos do template + os valores da ficha em widgets na Tela 2.

## Goals

- [ ] Agrupar os campos por prefixo de topo do caminho (`stats`, `modifiers`, `derived`, `config`, ...) em seções rotuladas, na ordem da API.
- [ ] Desenhar o controle por regra do schema: `is_editable: true` → input por `type` (número / texto); `is_editable: false` → read-only com destaque de "calculado"; `config.*` (0/1) → toggle on/off; label e indicador de derivado saídos do schema.
- [ ] Derivar tudo do `FieldSchema` — sem hardcode por sistema de RPG; template sintético com todos os tipos e prefixo desconhecido não quebra.
- [ ] Tela 2 **navegável** a partir do catálogo, alimentada por dados sintéticos (defaults do schema), **somente-leitura até a Fase 4**, sem criar nem salvar ficha.
- [ ] Toda string de UI pela camada de localização (`en`, única locale da V1).

## Out of Scope

Explicitamente excluído. Documentado para prevenir scope creep.

| Feature | Reason |
| --- | --- |
| Ciclo de edição/Salvar (`PATCH`, delta, merge incremental de cascata, dirty state, rodapé Salvar) | Fase 4 — o épico isola o commit da ficha |
| Estado de edição por campo com rebuild granular (Riverpod `select`/`family`) | [LGN-102](https://paulovalleriotedev.atlassian.net/browse/LGN-102) — spec própria desta fase; aqui o renderer é controlado e a Tela 2 o alimenta somente-leitura (RND-20) |
| Preview read-only no diálogo de criação | [LGN-100](https://paulovalleriotedev.atlassian.net/browse/LGN-100) — superfície separada; esta spec entrega o controle read-only que aquele card reusa |
| Criar ficha (`POST /sheets`) e biblioteca local de fichas | Fase 4 — o botão da Tela 3 segue desabilitado; a Tela 2 é alimentada por defaults do schema |
| Cache/invalidação persistente do detalhe do template | Mesma decisão da Fase 2 (`front-end-phase-2-template-detail-dialog`): busca on-demand a cada abertura, nada persistido |
| Tipos além de número/texto/bool do `FieldSchema` atual e step configurável por campo | O `FieldSchema` não carrega `step`; V1 usa Material defaults (ver Assumptions) |
| Identidade visual (paleta, tipografia próprias) | V2 — a V1 usa Material defaults dentro dos tokens |
| Golden tests de renderização | Fase 5 (Polish) — aqui testes de widget por tipo |

---

## Assumptions & Open Questions

Toda ambiguidade está resolvida ou registrada aqui — nada fica silenciosamente incerto.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --- | --- | --- | --- |
| Escopo da entrega | UI Engine (seções + controles, renderer controlado) **mais** uma Tela 2 navegável a partir de uma ação do catálogo | Decisão do mantenedor na discussão: quer a Tela 2 navegável já neste card | y |
| Origem dos valores na Tela 2 | Valores semeados dos `default` do `FieldSchema` (dados sintéticos), mantidos num rascunho local; sem ficha real | Não existe ficha no app até a Fase 4; os defaults provam os controles sem tocar na API de fichas | y |
| Estado de edição | O renderer é controlado (recebe `data` + callback), sem estado próprio; **desde o amend LGN-100 a Tela 2 passa `onChanged: null`** (somente-leitura) — o callback segue existindo e testado no controle/renderer | Primeira metade da decisão do mantenedor; o isolamento por campo é [LGN-102](https://paulovalleriotedev.atlassian.net/browse/LGN-102) e o commit é Fase 4; sem ficha real, editar um rascunho volátil não tem valor | y |
| Tela 2 somente-leitura | Enquanto a Fase 4 não existir, a Tela 2 renderiza os valores base **sem aceitar edição** (nenhum input `enabled`, nenhum toggle acionável) | Amend 2026-10-01: o mantenedor informou que a Tela 2 ainda editava; consistência com o preview read-only do diálogo (LGN-100) | y |
| Regra do `config.*` | Caminho cujo primeiro segmento é `config` → toggle 0/1; interação emite `1`/`0` | Regra do contrato do backend (nó proxy), não um literal de sistema; TDD § Regras de Renderização | y |
| Tipo do toggle | `type == bool` → Switch com valor `true`/`false`; `config.*` numérico → Switch com `1`/`0` | Cobre número/texto/bool sem inventar tipo novo; o valor emitido preserva a semântica do schema | n |
| Seções com grupo sem folhas | Omitidas da Tela 2 | Seção sem campo não tem nada a renderizar; o resumo da Fase 2 ainda as mostra | n |
| Anotação de seção do wireframe (`(editavel)`/`(calculado)`/`(regras)`) | Demotada para o campo: só o valor não-editável ganha o destaque "Calculated" | O critério de aceite exige o destaque por **campo**; a anotação de seção é do wireframe, e derivá-la de forma genérica não agrega ao critério | n |
| "Step apropriado" do input numérico | Sem stepper; `TextField` numérico (teclado decimal), sem `step` — o `FieldSchema` não carrega `step` e um stepper seria controle customizado fora dos Material defaults da V1 | `desktop/AGENTS.md` proíbe UI moderninha/extra sem necessidade; o wireframe mostra um campo simples `[ 16 ]` | n |
| Indicador de derivado (`formula`) | Tooltip de hover com a fórmula sobre o valor calculado; o destaque textual "Calculated" é o indicador visível | `desktop/AGENTS.md`: hover é o affordance de desktop; a fórmula não deve poluir a linha | n |
| Tipo de campo desconhecido | Fallback read-only (valor formatado), nunca um editor inventado nem crash | Marco M2: qualquer template renderiza; falhar em um tipo novo quebraria a promessa central | n |
| Input numérico vazio/ inválido | Emite `null` (valor limpo) | Um valor não numérico não pode virar lixo silencioso no estado | n |
| Label do campo/grupo | `schema.label` quando não vazio, senão humanização mecânica da chave; helper extraído e reusado pelo resumo da Fase 2 | Regra do resumo (TPL-05) reusada: um mecanismo só, sem duplicação | y |
| Alcance da Tela 2 | Ação secundária no card do catálogo ("Preview sheet") que empurra a Tela 2; a Fase 4 troca pelo fluxo real de abrir ficha | Decisão do mantenedor ("Tela 2 navegável"); affordance temporária e explícita, sem criar ficha | y |
| Estado vazio / erro da Tela 2 | Sem campos → linha única localizada (`templateNoFields`); falha do `GET /templates/:id` → mensagem mapeada + Retry, tela permanece aberta | Consistência com o diálogo de criação (`front-end-phase-2-template-detail-dialog`), mesmo caminho de erro | n |
| Dependências novas | Nenhuma — Material defaults + tokens existentes | `desktop/AGENTS.md`: nenhum pacote novo sem necessidade estrita | y |

**Open questions:** none — todas as ambiguidades foram resolvidas na discussão com o mantenedor ou registradas como premissas acima.

---

## User Stories

### P1: Seções da ficha por prefixo de topo ⭐ MVP

**User Story**: Como mantenedor, quero que a ficha seja agrupada em seções por prefixo de caminho, com cada campo rotulado pelo schema, para que eu enxergue o template como um formulário e não como uma lista solta de chaves.

**Why P1**: item de entrega do épico ("agrupamento por prefixo de caminho em seções") e pré-condição para todo o resto da Tela 2.

**Acceptance Criteria**:

1. WHEN a template's fields tree is rendered THEN the app SHALL render one labeled section per top-level key, in the order the API returned the keys. (RND-01)
2. WHEN a top-level node is a group THEN its section SHALL list every leaf beneath it recursively, in tree order. (RND-02)
3. WHEN a top-level node is a leaf THEN the app SHALL render it as its own section holding that single field. (RND-03)
4. The section label SHALL be derived from the field tree itself (the schema label when present, otherwise the humanized key) and SHALL NOT be a per-system literal. (RND-04)
5. The field label SHALL be the schema label when it is non-empty, otherwise the humanized key. (RND-05)

**Independent Test**: fixture com grupo, folha de topo e árvore aninhada → uma seção por chave de topo na ordem da API, com as folhas certas; label vazio → chave humanizada; fixture sintética com prefixo inexistente → seção renderiza igual.

---

### P1: Controle correto por regra do schema ⭐ MVP

**User Story**: Como mantenedor, quero que cada campo desenhe o controle certo para o seu schema (input por tipo, read-only para calculado, toggle para `config.*`), para que editar a ficha seja possível sem que a UI conheça o sistema de RPG.

**Why P1**: critério de aceite do épico e a razão de existir do UI Engine (TDD § Regras de Renderização).

**Acceptance Criteria**:

1. WHEN a field is editable and its `type` is numeric THEN the app SHALL render an editable numeric input holding the field's current value. (RND-06)
2. WHEN a field is editable and its `type` is text THEN the app SHALL render an editable text input holding the field's current value. (RND-07)
3. WHEN a field's path is under `config` (the 0/1 proxy node) THEN the app SHALL render it as an on/off toggle reflecting the 0/1 value, and toggling it SHALL report `1` or `0`. (RND-08)
4. WHEN a field's `type` is bool THEN the app SHALL render it as an on/off toggle reflecting its boolean value. (RND-09)
5. IF a field is not editable THEN the app SHALL render its value read-only with a "Calculated" highlight and SHALL NOT render an input for it. (RND-10)
6. WHEN a field's schema carries a `formula` THEN the control SHALL expose that formula as the derived-field indicator. (RND-11)
7. WHEN the user edits an editable control THEN the app SHALL report the new value for that field's path through the form callback and SHALL NOT issue any API request. (RND-12)
8. IF a field's `type` is none of number, text or bool THEN the app SHALL render its value read-only instead of failing or inventing an editor. (RND-13)

**Independent Test**: template sintético com número/texto/bool/config/derivado → input numérico, input de texto, switches, read-only com "Calculated"; digitar/togglear emite o valor pelo callback com o caminho certo; tipo exótico → read-only, nenhuma exceção.

---

### P1: Tela 2 navegável a partir do catálogo ⭐ MVP

**User Story**: Como mantenedor, quero abrir a Tela 2 de um template a partir do catálogo e vê-la renderizada com os valores base, para validar o UI Engine visualmente antes da Fase 4.

**Why P1**: decisão do mantenedor nesta spec; sem uma superfície alcançável a renderização não é demonstrável.

**Acceptance Criteria**:

1. WHEN the catalog shows a template THEN it SHALL expose an action that opens Screen 2 for that template. (RND-14)
2. WHEN Screen 2 opens THEN the app SHALL request `GET /templates/:id` through the Phase-1 client and SHALL show a loading state until the response settles. (RND-15)
3. WHEN `GET /templates/:id` succeeds THEN Screen 2 SHALL render the template's sections and controls seeded from the schema defaults. (RND-16)
4. IF `GET /templates/:id` fails THEN Screen 2 SHALL show the mapped message inline with a Retry action and SHALL remain open. (RND-17)
5. WHEN Screen 2 is open THEN it SHALL offer a back action that returns to the catalog without creating, saving or mutating anything. (RND-18)
6. WHILE Screen 2 is open the app SHALL NOT issue any mutation request (`POST`/`PATCH`). (RND-19)
7. WHILE Screen 2 is open no control SHALL accept an edit — Screen 2 SHALL render the seeded values read-only (no enabled input, no actionable toggle) until Phase 4 introduces the real sheet cycle. (RND-20)

**Independent Test**: abrir "Preview sheet" de um template → 1 `GET /templates/:id`, seções renderizadas com os defaults, **nenhum controle editável**; back volta ao catálogo; transporte fake não registra nenhum `POST`/`PATCH`; 500 → mensagem mapeada + Retry.

---

## Edge Cases

- IF the template `fields` tree is empty THEN Screen 2 SHALL show one localized line ("This template has no fields.") in place of the sections.
- IF a top-level group has no leaves THEN Screen 2 SHALL omit its section (nothing to render for it).
- WHEN a schema label is empty THEN the field and section SHALL render the humanized key.
- WHEN a field's value is missing from the data map THEN the control SHALL render empty/off without throwing.
- WHEN two top-level keys share the same leaf name THEN each SHALL render under its own section independently.
- WHEN the template name is an empty string THEN Screen 2 SHALL render its layout with an empty title instead of throwing.
- IF an editable numeric input is cleared THEN the control SHALL report `null` for that path (control-level behavior, RND-12 — unreachable on Screen 2 while it renders read-only, RND-20).
- WHEN Screen 2 renders a field that the schema marks editable THEN it SHALL still draw the control for that type, disabled (read-only host), not a different widget.
- Varredura de dimensões implícitas: **input validation & bounds** → parse numérico e limpeza (RND-06/RND-12, edge do input vazio, nível do controle); **failure/partial-failure** → RND-17 (mensagem mapeada + Retry); **idempotência/retry** → RND-17 (Retry reusa o single-flight do `TemplateDetailController`, sem duplicar requisição); **concorrência/ordenação** → N/A (o renderer é síncrono; a fonte assíncrona é o detalhe, herdado da Fase 2); **data lifecycle/expiry** → N/A (nada é persistido; Tela 2 é somente-leitura, RND-20); **external-dependency failure** → RND-17; **state-transition integrity** → RND-15/16/18 (loading → conteúdo/erro, back sem efeito colateral); **auth/rate limits** → N/A (V1 sem auth); **observabilidade** → N/A (logs são Fase 5).

---

## Requirement Traceability

Cada requisito tem ID único para rastreamento em design, tasks e validação.

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| RND-01 | P1: Seções da ficha por prefixo de topo | Design | Verified |
| RND-02 | P1: Seções da ficha por prefixo de topo | Design | Verified |
| RND-03 | P1: Seções da ficha por prefixo de topo | Design | Verified |
| RND-04 | P1: Seções da ficha por prefixo de topo | Design | Verified |
| RND-05 | P1: Seções da ficha por prefixo de topo | Design | Verified |
| RND-06 | P1: Controle correto por regra do schema | Design | Verified |
| RND-07 | P1: Controle correto por regra do schema | Design | Verified |
| RND-08 | P1: Controle correto por regra do schema | Design | Verified |
| RND-09 | P1: Controle correto por regra do schema | Design | Verified |
| RND-10 | P1: Controle correto por regra do schema | Design | Verified |
| RND-11 | P1: Controle correto por regra do schema | Design | Verified |
| RND-12 | P1: Controle correto por regra do schema | Design | Verified |
| RND-13 | P1: Controle correto por regra do schema | Design | Verified |
| RND-14 | P1: Tela 2 navegável a partir do catálogo | Design | Verified |
| RND-15 | P1: Tela 2 navegável a partir do catálogo | Design | Verified |
| RND-16 | P1: Tela 2 navegável a partir do catálogo | Design | Verified |
| RND-17 | P1: Tela 2 navegável a partir do catálogo | Design | Verified |
| RND-18 | P1: Tela 2 navegável a partir do catálogo | Design | Verified |
| RND-19 | P1: Tela 2 navegável a partir do catálogo | Design | Verified |
| RND-20 | P1: Tela 2 navegável a partir do catálogo | Design | Verified |

**Coverage:** 20 requisitos no total, 20 mapeados 1:1 a critérios de aceite; 0 não mapeados.

---

## Success Criteria

Como sabemos que a fase foi bem-sucedida:

- [ ] Template sintético com número, texto, bool, `config.*` e derivado renderiza o controle correto para cada um (input numérico / input de texto / toggle / toggle zero-um / read-only "Calculated") — critério de aceite do épico.
- [ ] `fantasy_v1` renderiza as seções `stats`, `others`, `modifiers`, `derived` e `config`; `config.*` vira toggle e `derived`/`modifiers` aparecem read-only como "Calculated" — **Marco M2**.
- [ ] Zero hardcode por sistema: prefixo de topo desconhecido e tipo exótico renderizam sem mudança de código.
- [ ] Abrir a Tela 2 emite exatamente um `GET /templates/:id` e **nenhuma** mutação (`POST`/`PATCH` ausentes do transporte de teste).
- [ ] A Tela 2 renderiza somente-leitura: nenhum input `enabled` e nenhum toggle acionável enquanto aberta (RND-20).
- [ ] `flutter test` verde sem API de pé e `flutter analyze` limpo.
