# [Front End] Fase 2 — Detalhe do template e diálogo de criação (Tela 3)

> Spec da fase Specify (`tlc-spec-driven`) · origem: `.specs/features/front-end-phase-2-template-detail-dialog/spec.md` · gate `validate_spec.py --strict`: 0 erros, 0 warnings · épico: LGN-91 · fonte: `docs/TDD-desktop-mvp.md` + `desktop/AGENTS.md`

> Fonte: [LGN-97](https://paulovalleriotedev.atlassian.net/browse/LGN-97) · [LGN-91](https://paulovalleriotedev.atlassian.net/browse/LGN-91) (épico de fase, sob [LGN-88](https://paulovalleriotedev.atlassian.net/browse/LGN-88)) · `docs/TDD-desktop-mvp.md` (aprovado — fonte da verdade) · `desktop/AGENTS.md` (regras de construção) · wireframe Tela 3 · contrato real: `GET /templates/:id` (`internal/app`, DTO da Fase 1).
> Relação entre specs desta fase: depende de `front-end-phase-2-catalog` (shell + ação que abre a superfície de criação); entrega em paralelo com `front-end-phase-2-connection-indicator`.
> Status: **Approved** — aprovada pelo mantenedor em 2026-09-30 (premissas marcadas `n` aceitas como default).

## Problem Statement

O catálogo lista templates, mas o mantenedor não sabe o que há dentro deles antes de criar uma ficha: o wireframe Tela 3 pede o resumo do template por prefixo de campo (`stats: 7 - modifiers: 7 - derived: 3`), que só existe depois de `GET /templates/:id`. Criar a ficha em si é Fase 4 — logo a Fase 2 precisa entregar o detalhe carregado (e o `FieldSchema` pronto para o UI Engine) numa superfície de criação que, por escopo, ainda não cria nada.

## Goals

- [ ] `GET /templates/:id` consumido on-demand ao abrir a superfície de criação, com estados carregando / erro (mensagem mapeada + Retry) inline no diálogo.
- [ ] Resumo por prefixo derivado do `FieldSchema` — nenhuma lista de prefixos hardcoded, qualquer template renderiza.
- [ ] `FieldSchema` carregado e exposto ao estado da aplicação com a árvore intacta (pré-condição para o UI Engine da Fase 3).
- [ ] Campo de nome da ficha presente e o botão primário desabilitado com tooltip explicando que a criação chega na Fase 4.
- [ ] Toda string de UI pela camada de localização (`en`, única locale da V1).

## Out of Scope

Explicitamente excluído. Documentado para prevenir scope creep.

| Feature | Reason |
| --- | --- |
| `POST /sheets` (criar a ficha de fato) | Fase 4 — o épico coloca "criação de ficha" fora do escopo desta fase |
| Validação do nome (obrigatório, duplicado, tamanho) | Fase 4 — o botão está desabilitado; validar nome sem poder criar é ruído |
| Renderização dos campos (`FieldSchema` → inputs/toggles) | Fase 3 (UI Engine) — aqui os fields só alimentam o resumo por prefixo |
| Lista de templates e estados de catálogo | Spec `front-end-phase-2-catalog` |
| Indicador online/offline e polling de `GET /health` | Spec `front-end-phase-2-connection-indicator` |
| Cache persistente de detalhe / invalidação | V1 busca a cada abertura; Local Store nunca guarda conteúdo de template |
| Biblioteca local de fichas ("My sheets"), abertura de ficha | Fase 4 |
| Editar template, criar template customizado | V2+ do backend (não existe hoje) |
| Identidade visual (paleta, tipografia próprias) | V2 — a V1 usa Material defaults dentro dos tokens |

---

## Assumptions & Open Questions

Toda ambiguidade está resolvida ou registrada aqui — nada fica silenciosamente incerto.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --- | --- | --- | --- |
| Escopo do diálogo na Fase 2 | Resumo do template + campo de nome + botão "Create sheet" **desabilitado** com tooltip | Decisão do mantenedor ("Resumo + nome, botão desabilitado") — affordance visível como no wireframe, criação preservada para a Fase 4 | y |
| Quando o detalhe é buscado | A cada abertura do diálogo (on-demand); sem cache entre aberturas | Simplicidade: sem invalidação para acertar; custo é um GET local em uso pessoal; cache de detalhe pode vir com o UI Engine (Fase 3) quando houver medição de custo | n |
| Regra do resumo por prefixo | Cada chave de topo de `fields` vira uma linha: grupo → número de folhas descendentes (recursivo); folha de topo → 1 sob o próprio nome | Derivada do contrato (árvore folha/grupo da Fase 1) e do wireframe Tela 3; não inventa rótulo humanizado nem bucket "general" | y |
| Exibição do resumo | Linhas no formato `prefixo: contagem`, na ordem de chaves devolvida pela API | Fiel ao wireframe Tela 3; ordenação própria seria comportamento inventado | y |
| Estado inicial do campo de nome | Vazio, com placeholder de exemplo localizado ("e.g. My Character") | O wireframe mostra um exemplo, não um valor default; pré-preencher geraria fichas com nome inventedado na Fase 4 | y |
| Texto do tooltip do botão | "Sheet creation arrives in a later phase." (l10n) | `desktop/AGENTS.md`: "disabled primary actions explain themselves via tooltip" | y |
| Comportamento em falha | Erro mapeado exibido **inline no diálogo**, diálogo continua aberto com Retry (nunca fecha, nunca crash) | TDD Tela 3: "Erros mapeados (`NOT_FOUND`, rede) exibidos inline no diálogo" | y |
| Template desconhecido (`NOT_FOUND`) | Mesmo caminho de erro inline do diálogo (mensagem mapeada + Retry) | Caso real: id de ficha antiga/instância trocada; mesma ação de recuperação | y |
| Template sem campos (`fields` vazio) | Linha única localizada "This template has no fields." no lugar do resumo | Cobre o caso-limite sem quebrar layout nem deixar área em branco ambígua | n |
| Template sem grupo de topo (só folhas) | Cada folha vira sua própria linha no resumo (mesma regra de folha de topo) | Consistente com a regra de prefixo; nenhum caso especial | y |
| Estado do detalhe no app | Provider Riverpod expõe o `TemplateDetail` decodificado (id, name, fields) após o sucesso | Fato do TDD: "State (Riverpod) — providers por feature"; pré-condição para Fases 3/4 | y |

**Open questions:** none — todas as ambiguidades foram resolvidas na discussão com o mantenedor ou registradas como premissas acima.

---

## User Stories

### P1: Detalhe do template on-demand com resumo por prefixo ⭐ MVP

**User Story**: Como mantenedor, quero que abrir um template no catálogo carregue `GET /templates/:id` e mostre o resumo por prefixo de campo, para que eu saiba o que a ficha contém antes de decidir criá-la.

**Why P1**: item de entrega do épico ("detalhe do template carregando o `FieldSchema` para a renderização e o resumo por prefixo do diálogo de criação — Tela 3").

**Acceptance Criteria**:

1. WHEN the creation surface is opened for a template id THEN the app SHALL request `GET /templates/:id` through the Phase-1 API client and SHALL show a loading state until the response settles. (TPL-01)
2. WHEN `GET /templates/:id` succeeds THEN the dialog SHALL show the template name and one summary line per top-level key of `fields`, each with the number of leaf fields it contains. (TPL-02)
3. WHEN the summary is computed, a group entry SHALL count every leaf beneath it recursively, and a top-level leaf SHALL count as 1 under its own key. (TPL-03)
4. WHILE the detail request is in flight, the dialog SHALL show the loading state and SHALL NOT show a summary left over from a previously opened template. (TPL-04)
5. The summary SHALL be derived only from the returned `FieldSchema` tree — no hardcoded prefix list (no `stats`/`modifiers`/`config` literals in the code) — so any template renders without code changes. (TPL-05)
6. The detail fetched SHALL be exposed to the state layer as the decoded template (id, name, `fields` intact) for the later phases. (TPL-06)

**Independent Test**: abrir dois templates diferentes e conferir o resumo de cada; fixture com grupo, folha de topo e árvore aninhada → contagens corretas; fixture sintética com prefixo inexistente → resumo renderiza.

---

### P1: Estados de erro inline no diálogo ⭐ MVP

**User Story**: Como mantenedor, quero que falha de rede ou `NOT_FOUND` apareça como mensagem mapeada dentro do diálogo com uma ação de Retry, para que eu nunca saia de uma tela em branco nem perca o contexto.

**Why P1**: critério de aceite do épico (estados claros) e exigência explícita do wireframe Tela 3 — é o que separa o diálogo de um modal quebrado.

**Acceptance Criteria**:

1. IF `GET /templates/:id` fails (transport failure, 4xx or 5xx) THEN the dialog SHALL show the mapped message from the Phase-1 error mapper inline and SHALL remain open. (TPL-07)
2. WHEN the detail request fails THEN the dialog SHALL offer a Retry action that re-requests `GET /templates/:id` exactly once per activation. (TPL-08)
3. IF a template id does not exist (`NOT_FOUND`) THEN the dialog SHALL show the mapped not-found message inline instead of closing or crashing. (TPL-09)
4. IF the response is 2xx but the payload cannot be decoded (missing `data`, malformed shape) THEN the dialog SHALL show the defensive parse failure inline — never a half-rendered summary. (TPL-10)

**Independent Test**: transporte fake devolvendo 500, 404, offline e JSON quebrado → em os quatro, diálogo aberto com mensagem mapeada + Retry contando chamadas.

---

### P1: Superfície de criação fiel ao wireframe (sem criação) ⭐ MVP

**User Story**: Como mantenedor, quero a Tela 3 com campo de nome e botão primário desabilitado explicando o porquê, para que a tela exista como affordance e a criação real entre na Fase 4 sem redesenho.

**Why P1**: decisão do mantenedor nesta fase; mantém o wireframe e a separação de escopo entre as Fases 2 e 4.

**Acceptance Criteria**:

1. WHEN the creation surface is open THEN it SHALL show a sheet-name text input, empty by default, with localized label and example placeholder. (TPL-11)
2. WHEN the creation surface is open THEN its primary action ("Create sheet") SHALL be disabled and SHALL expose a tooltip explaining that sheet creation arrives in a later phase. (TPL-12)
3. The creation surface SHALL NOT issue `POST /sheets` or any other mutation — opening it SHALL be side-effect free. (TPL-13)
4. WHEN the user dismisses the creation surface (`Esc` or its close action) THEN it SHALL close and return to the catalog without any additional API request. (TPL-14)

**Independent Test**: abrir diálogo → input vazio, botão desabilitado com tooltip, zero mutações no transporte; digitar no input → nenhuma requisição; `Esc` → fecha sem chamada nova.

---

### P2: Reabertura sempre fresca

**User Story**: Como mantenedor, quero que reabrir o mesmo template busque o detalhe de novo, para que uma troca de instância ou de seed nunca me mostre um resumo obsoleto.

**Why P2**: sem cache, o comportamento é trivialmente verdadeiro; com cache, seria preciso definir invalidação — decisão adiada para quando houver medição de custo (Fase 3).

**Acceptance Criteria**:

1. WHEN the creation surface is reopened for the same template id THEN the app SHALL request `GET /templates/:id` again — no cached reuse in Phase 2. (TPL-15)

**Independent Test**: abrir, fechar, reabrir → 2 requisições de detalhe no transporte fake.

---

## Edge Cases

- IF `fields` resolves to zero leaves THEN the dialog SHALL show one localized line ("This template has no fields.") in place of the summary (assumption pending confirmation).
- WHEN a top-level key holds a single leaf THEN the summary SHALL still render its own line with count 1.
- IF two top-level keys share the same leaf name THEN the summary SHALL count them independently — each under its own key.
- WHEN the template name is an empty string THEN the dialog SHALL render the layout with an empty title instead of throwing.
- Varredura de dimensões implícitas: **input validation & bounds** → nenhum dado do usuário é validado nesta fase (botão desabilitado — validação de nome é Fase 4); **failure/partial-failure** → TPL-07…10; **idempotência/retry** → TPL-08 (Retry por ativação) + TPL-15 (reabertura sempre fresca, sem cache a invalidar); **concorrência/ordenação** → TPL-04 (sem resumo obsoleto durante o voo); **external-dependency failure** → mensagem mapeada via client da Fase 1; **state-transition integrity** → TPL-14 (fecha e volta ao catálogo). N/A nesta spec: **auth/rate limits** (V1 sem auth), **data lifecycle/expiry** (nenhum dado persistido pelo diálogo), **observabilidade** (logs são Fase 5).

---

## Requirement Traceability

Cada requisito tem ID único para rastreamento em design, tasks e validação.

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| TPL-01 | P1: Detalhe do template on-demand com resumo | Design | Verified |
| TPL-02 | P1: Detalhe do template on-demand com resumo | Design | Verified |
| TPL-03 | P1: Detalhe do template on-demand com resumo | Design | Verified |
| TPL-04 | P1: Detalhe do template on-demand com resumo | Design | Verified |
| TPL-05 | P1: Detalhe do template on-demand com resumo | Design | Verified |
| TPL-06 | P1: Detalhe do template on-demand com resumo | Design | Verified |
| TPL-07 | P1: Estados de erro inline no diálogo | Design | Verified |
| TPL-08 | P1: Estados de erro inline no diálogo | Design | Verified |
| TPL-09 | P1: Estados de erro inline no diálogo | Design | Verified |
| TPL-10 | P1: Estados de erro inline no diálogo | Design | Verified |
| TPL-11 | P1: Superfície de criação fiel ao wireframe | Design | Verified |
| TPL-12 | P1: Superfície de criação fiel ao wireframe | Design | Verified |
| TPL-13 | P1: Superfície de criação fiel ao wireframe | Design | Verified |
| TPL-14 | P1: Superfície de criação fiel ao wireframe | Design | Verified |
| TPL-15 | P2: Reabertura sempre fresca | Design | Verified |

**Coverage:** 15 requisitos no total, 15 mapeados 1:1 a critérios de aceite; 0 não mapeados.

---

## Success Criteria

Como sabemos que a fase foi bem-sucedida:

- [ ] Detalhe entrega `fields` prontos para o UI Engine — **critério de aceite do épico** — e o resumo por prefixo bate com a contagem real de folhas de `fantasy_v1`.
- [ ] Zero prefixos hardcoded: um template sintético com chaves de topo inesperadas renderiza o resumo sem mudança de código.
- [ ] `NOT_FOUND` e rede aparecem como mensagem mapeada inline no diálogo, com Retry — nunca tela em branco.
- [ ] Abrir o diálogo não emite nenhuma mutação (`POST` nunca aparece no transporte de teste).
- [ ] `flutter test` verde sem API de pé e `flutter analyze` limpo.
