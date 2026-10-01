# [Front End] Fase 3 — Preview read-only da ficha com valores base no diálogo de criação

> Spec da fase Specify (`tlc-spec-driven`) · origem: `.specs/features/front-end-phase-3-sheet-preview/spec.md` · gate `validate_spec.py --strict`: 0 erros, 0 warnings · épico: [LGN-92](https://paulovalleriotedev.atlassian.net/browse/LGN-92) · fonte: `docs/TDD-desktop-mvp.md` + `desktop/AGENTS.md`

> Fonte: [LGN-100](https://paulovalleriotedev.atlassian.net/browse/LGN-100) · [LGN-92](https://paulovalleriotedev.atlassian.net/browse/LGN-92) (épico de fase, sob [LGN-88](https://paulovalleriotedev.atlassian.net/browse/LGN-88)) · `docs/TDD-desktop-mvp.md` (aprovado — fonte da verdade) · `desktop/AGENTS.md` (regras de construção) · contrato real: `GET /templates/:id` (`FieldSchema` já decodificado pela Fase 1).
> Relação entre specs desta fase: depende de `front-end-phase-2-template-detail-dialog` (diálogo de criação + `TemplateDetailState`, Tela 3) e de `front-end-phase-3-sheet-rendering` (UI Engine read-only que este card reusa — `SheetForm`/`SheetFieldControl`); [LGN-99](https://paulovalleriotedev.atlassian.net/browse/LGN-99) (flag de grupo no DTO) poderá substituir depois a visibilidade fixa de `config`/`derived`/`modifiers`. **Amend sobre a Fase 2:** o resumo textual por prefixo é substituído por este preview (ver Assumptions — TPL-02/03/04/05 são reescritos na spec `front-end-phase-2-template-detail-dialog`, precedente do amend de UAT).
> Status: **Approved** — aprovada pelo mantenedor em 2026-10-01 (premissas marcadas `n` aceitas como default).

## Problem Statement

O diálogo de criação (Tela 3, LGN-97) hoje mostra só o resumo do template (nome legível + folhas). O mantenedor pediu, como evolução, enxergar **o que a ficha contém antes de criá-la** — campos com seus valores base (ex.: `Armor class: 10`, `Constitution: 10`). A superfície decidida na discussão é o **próprio diálogo, em duas colunas**: a informação atual (identidade do template + nome da ficha) à esquerda e, à direita, um agrupamento (card) com a prévia somente-leitura dos campos — sem edição, sem `POST /sheets`, sempre visível assim que o detalhe carrega. É o primeiro recorte do UI Engine dentro da Tela 3.

## Goals

- [ ] Diálogo de criação em duas colunas: à esquerda a informação atual (identidade + campo de nome), à direita um card de preview sempre visível quando `GET /templates/:id` conclui.
- [ ] Campos renderidos com label (schema label ou chave humanizada) + valor base (`FieldSchema.default`), read-only, agrupados por prefixo de topo na ordem da API — incluindo `config`, `derived` e `modifiers`.
- [ ] Tudo derivado do `FieldSchema` — sem hardcode por sistema; qualquer template renderiza.
- [ ] Zero mutação: nenhum controle aceita edição e nenhuma requisição de escrita é emitida; as colunas compartilham o único `GET /templates/:id` do diálogo.
- [ ] Resumo textual da Fase 2 removido (amend), sem duplicar a mesma lista de campos no mesmo diálogo.
- [ ] Toda string de UI pela camada de localização (`en`, única locale da V1).

## Out of Scope

Explicitamente excluído. Documentado para prevenir scope creep.

| Feature | Reason |
| --- | --- |
| `POST /sheets`, edição de campo, ciclo Salvar | Fase 4 — o card é somente-leitura ("Sem edição e sem POST /sheets") |
| Recálculo de valores derivados (fórmula no cliente ou via API) | [LGN-103](https://paulovalleriotedev.atlassian.net/browse/LGN-103) (dry-run backend) — aqui derivadas mostram o `default` do schema |
| Flag de grupo no DTO para ocultar `config`/`derived`/`modifiers` | [LGN-99](https://paulovalleriotedev.atlassian.net/browse/LGN-99) (backend, To Do) — este card renderiza todos os grupos e deixa a visibilidade à flag futura |
| Rebuild granular por campo (Riverpod `select`/`family`) | [LGN-102](https://paulovalleriotedev.atlassian.net/browse/LGN-102) — não há edição para isolar |
| Preview no painel direito, push da Tela 2 ou segunda ação "Preview" | Superfícies descartadas na discussão com o mantenedor — o preview é conteúdo do próprio diálogo, sem toggle |
| Alterações no shell, painel direito ou `ShellSurface` | O diálogo é um overlay sobre o shell; o shell fica intocado |
| Ação de preview direto do card do catálogo | O catálogo já tem "Preview sheet" (RND-14), que empurra a Tela 2 — fluxo próprio, intocado |
| Cache persistente do detalhe / invalidação | Mesma decisão da Fase 2: busca on-demand, nada persistido |
| Indicador de conexão, lista de templates, refresh (F5) | Specs da Fase 2 (`front-end-phase-2-catalog`, `front-end-phase-2-connection-indicator`) |
| Validação do nome da ficha | Fase 4 — o botão primário segue desabilitado |
| Identidade visual (paleta, tipografia próprias) | V2 — a V1 usa Material defaults dentro dos tokens |

---

## Assumptions & Open Questions

Toda ambiguidade está resolvida ou registrada aqui — nada fica silenciosamente incerto.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --- | --- | --- | --- |
| Superfície do preview | Card dentro do diálogo de criação, à direita da informação atual (duas colunas); o diálogo nunca fecha | Decisão do mantenedor na discussão de LGN-100 (rejeitou o painel direito por descartar o nome digitado); o diálogo é um "creation flow", caso previsto pelo `desktop/AGENTS.md` | y |
| Visibilidade | Sempre visível assim que `GET /templates/:id` conclui — sem ação nem toggle | Decisão do mantenedor: "see before creating" deve aparecer sem clique extra; loading/erro/vazio renderizam na mesma área do card | y |
| Resumo textual da Fase 2 | Removido — a listagem de campos do diálogo passa a existir só no preview | Decisão do mantenedor: manter os dois listaria cada campo duas vezes no mesmo diálogo; amend na spec da Fase 2 com precedente do amend de UAT (TPL-02/03/04/05 reescritos) | y |
| Código do resumo | `template_summary.dart` e seus testes são removidos junto (código morto); `fieldDisplayName` permanece em `engine/field_grouping.dart` | Sem código morto no repositório (convenção do AGENTS.md); o nome de exibição é compartilhado com o renderer da Fase 3 | y |
| Grupos reservados no preview | `config`, `derived` e `modifiers` aparecem, read-only | Decisão do mantenedor na discussão: o preview serve para "enxergar o que a ficha contém" e o exemplo do card (`Armor class`) é derivado; a flag do [LGN-99](https://paulovalleriotedev.atlassian.net/browse/LGN-99) poderá ocultá-los depois | y |
| Origem dos valores base | `FieldSchema.default` de cada folha, inclusive as derivadas | Único valor-base disponível no cliente; fórmulas não são avaliadas nesta fase (dry-run é [LGN-103](https://paulovalleriotedev.atlassian.net/browse/LGN-103)) | y |
| Apresentação dos campos | Reusa `SheetForm` + `SheetFieldControl` com `onChanged: null` | A spec da Fase 3 define que este card reusa o controle read-only ali entregue; um segundo renderer violaria "uma única camada com regra de renderização" | y |
| Forma do agrupamento | Card plano com borda hairline e raio 4 (token `AppRadius.panel`), sem elevação, com cabeçalho localizado | O mantenedor pediu "card grouping"; `desktop/AGENTS.md` permite card para objeto genuinamente delimitado e proíbe elevação/decoração | y |
| Altura do card | Altura máxima limitada com scroll interno, para o diálogo nunca passar da viewport | Coluna direita tem ~20 folhas em `fantasy_v1`; sem limite o diálogo estouraria a tela (revisão de layout obrigatória do AGENTS.md) | n |
| Requisições | As duas colunas compartilham o `GET /templates/:id` do diálogo — zero requisição nova, zero cache | Mesma provider family já usada pela Fase 2; custo adicional nulo | y |
| Cabeçalho do card | Chave l10n nova ("Preview") em estilo caption, com `Key` de teste própria | Consistência com os cabeçalhos de seção do painel/dialogo; affordance mínima, sem decoração | n |
| Digitação do nome | Digitar no campo de nome não altera o preview nem emite requisição | Colunas independentes; a Fase 4 valida o nome, não aqui | y |
| Janela estreita | O diálogo é overlay (não sofre a regra CAT-03 do painel); em largura mínima as duas colunas seguem visíveis e o card rola internamente | O diálogo não é o shell; o limite de altura do card cobre a faixa estreita | n |

**Open questions:** none — todas as ambiguidades foram resolvidas na discussão com o mantenedor ou registradas como premissas acima.

---

## User Stories

### P1: Card de preview ao lado da informação do diálogo ⭐ MVP

**User Story**: Como mantenedor, quero ver, dentro do diálogo de criação, os campos do template com seus valores base ao lado do campo de nome, para decidir criar a ficha sabendo o que ela contém.

**Why P1**: é o pedido literal do card ("Renderizar, no diálogo de criação … uma prévia somente-leitura dos campos do template com seus valores default").

**Acceptance Criteria**:

1. WHEN `GET /templates/:id` succeeds THEN the creation dialog SHALL show, beside the dialog information, a preview card with one labeled section per top-level key of `fields`, in the order the API returned the keys, including the reserved groups `config`, `derived` and `modifiers`. (PRV-01)
2. WHEN a section is rendered THEN every leaf beneath it SHALL appear with its display name (the schema label when non-empty, otherwise the humanized key) and its base value from `FieldSchema.default`. (PRV-02)
3. The preview SHALL derive sections, labels and values only from the returned `FieldSchema` — no per-system literal — so any template renders without code changes. (PRV-03)
4. WHILE the detail request is in flight, the preview area SHALL show the loading state and SHALL NOT show fields left over from a previously opened template. (PRV-04)
5. The dialog SHALL list the template's fields only through the preview card — the Phase-2 text summary SHALL NOT be rendered. (PRV-05)
6. WHILE the preview is shown, no control SHALL accept an edit and the app SHALL NOT issue any mutation request — the transport SHALL carry only `GET` requests. (PRV-06)

**Independent Test**: fixture com grupo, folha de topo e árvore aninhada → uma seção por chave de topo na ordem da API, cada folha com label + `default`, `config`/`derived`/`modifiers` presentes; nenhuma linha do resumo antigo; fixture sintética com prefixo desconhecido renderiza sem mudança de código.

---

### P1: Estados de erro e vazio na área do preview ⭐ MVP

**User Story**: Como mantenedor, quero falha de rede, `NOT_FOUND` e template sem campos aparecendo na área do preview com Retry, para que o diálogo nunca fique em branco nem congelado.

**Why P1**: com o resumo removido, a coluna da direita é a única que reage ao carregamento — sem estados, uma falha deixaria o diálogo pela metade.

**Acceptance Criteria**:

1. IF the detail request fails (transport failure, 4xx or 5xx, or a payload that cannot be decoded) THEN the preview area SHALL show the mapped message from the Phase-1 error mapper inline with a Retry action and the dialog SHALL remain open. (PRV-07)
2. WHEN Retry is activated THEN the app SHALL re-request `GET /templates/:id` exactly once per activation. (PRV-08)
3. IF the loaded template has no renderable field (empty `fields`, or only groups without leaves) THEN the preview area SHALL show the localized "This template has no fields." line in place of the sections. (PRV-09)

**Independent Test**: transporte fake devolvendo 500, 404, offline e JSON quebrado → nos quatro, diálogo aberto com mensagem mapeada + Retry contando chamadas; `fields` vazio → linha localizada.

---

### P2: Duas colunas sem quebrar o diálogo

**User Story**: Como mantenedor, quero que o diálogo continue utilizável em qualquer largura e que reabri-lo refaça o preview do zero, para que a prévia nunca fique obsoleta nem estoure a janela.

**Why P2**: a coluna direita tem ~20 folhas num template real — sem limite de altura o diálogo estoura a viewport; e reuso de estado antigo mostraria campos de outra abertura.

**Acceptance Criteria**:

1. WHEN the preview card holds more fields than fit the viewport THEN the card SHALL scroll within a bounded height so the dialog (its name field and actions) stays reachable without resizing the window. (PRV-10)
2. The creation surface SHALL issue at most one `GET /templates/:id` per activation, shared by the dialog information and the preview card. (PRV-11)
3. WHEN the creation surface is reopened for the same template id THEN the preview SHALL be recomputed from a fresh `GET /templates/:id` response — no cached reuse in this phase. (PRV-12)
4. WHEN the user types in the sheet-name field THEN the preview SHALL remain unchanged and no API request SHALL be issued. (PRV-13)

**Independent Test**: fixture com 30+ folhas → card rola, campo de nome e botões alcançáveis; abrir/fechar/reabrir → 2 GETs no transporte; digitar no nome → preview estável, zero chamadas.

---

## Edge Cases

- WHEN a top-level group has no leaves THEN the preview SHALL omit that section (same rule as the Phase-3 renderer).
- IF the detail request is still failing when the dialog opens THEN the preview area SHALL show the mapped error with Retry (the dialog never shows an empty right column next to a filled left column).
- WHEN the template name is an empty string THEN the dialog SHALL render with an empty title instead of throwing.
- IF the window is at its minimum supported width THEN the two columns SHALL remain side by side, with the card scrolling internally rather than forcing the dialog wider than the window.
- Varredura de dimensões implícitas: **input validation & bounds** → N/A porque nenhum dado é validado aqui (validação de nome é Fase 4; PRV-13 cobre que digitar não faz nada); **idempotência/retry** → PRV-08 (Retry por ativação), PRV-11 (uma requisição por ativação) e PRV-12 (reabertura sempre fresca); **concorrência/ordenação** → PRV-04 (nada de preview obsoleto durante o voo); **state-transition integrity** → TPL-14 da Fase 2 permanece (fecha e volta ao catálogo, sem chamada nova) — o preview não cria estado próprio de navegação; **failure/partial-failure** e **external-dependency failure** → PRV-07/09. N/A nesta spec: **auth/rate limits** (V1 sem auth), **data lifecycle/expiry** (nada é persistido pelo diálogo), **observabilidade** (logs são Fase 5), **payments** (não existe).

---

## Requirement Traceability

Cada requisito tem ID único para rastreamento em design, tasks e validação.

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| PRV-01 | P1: Card de preview ao lado da informação do diálogo | Design | Pending |
| PRV-02 | P1: Card de preview ao lado da informação do diálogo | Design | Pending |
| PRV-03 | P1: Card de preview ao lado da informação do diálogo | Design | Pending |
| PRV-04 | P1: Card de preview ao lado da informação do diálogo | Design | Pending |
| PRV-05 | P1: Card de preview ao lado da informação do diálogo | Design | Pending |
| PRV-06 | P1: Card de preview ao lado da informação do diálogo | Design | Pending |
| PRV-07 | P1: Estados de erro e vazio na área do preview | Design | Pending |
| PRV-08 | P1: Estados de erro e vazio na área do preview | Design | Pending |
| PRV-09 | P1: Estados de erro e vazio na área do preview | Design | Pending |
| PRV-10 | P2: Duas colunas sem quebrar o diálogo | Design | Pending |
| PRV-11 | P2: Duas colunas sem quebrar o diálogo | Design | Pending |
| PRV-12 | P2: Duas colunas sem quebrar o diálogo | Design | Pending |
| PRV-13 | P2: Duas colunas sem quebrar o diálogo | Design | Pending |

**Coverage:** 13 requisitos no total, 13 mapeados 1:1 a critérios de aceite; 0 não mapeados.

---

## Success Criteria

Como sabemos que a fase foi bem-sucedida:

- [ ] Abrir o diálogo mostra, para `fantasy_v1`, os campos com seus `default` (ex.: `Base armor class`, `Stats: Constitution`, `Stats: Dexterity`, e também `Modifiers`/`Derived`/`Config`), agrupados na ordem da API, ao lado do campo de nome.
- [ ] Zero mutação: nenhuma escrita aparece no transporte de teste enquanto o diálogo está aberto; nenhum controle emite edição.
- [ ] Zero estrutura hardcoded: um template sintético com prefixos e tipos inesperados renderiza no card sem mudança de código.
- [ ] Falha de rede/404/JSON quebrado → mensagem mapeada + Retry na área do preview; o diálogo nunca fica pela metade.
- [ ] O resumo textual antigo não existe mais no diálogo (amend da Fase 2 aplicado) e o card rola em templates grandes sem estourar a janela.
- [ ] `flutter test` verde sem API de pé e `flutter analyze` limpo.
