# [Front End] Fase 2 — Catálogo: lista de templates, shell e navegação rasa

> Fonte: [LGN-96](https://paulovalleriotedev.atlassian.net/browse/LGN-96) · [LGN-91](https://paulovalleriotedev.atlassian.net/browse/LGN-91) (épico de fase, sob [LGN-88](https://paulovalleriotedev.atlassian.net/browse/LGN-88)) · `docs/TDD-desktop-mvp.md` (aprovado — fonte da verdade) · `desktop/AGENTS.md` (regras de construção) · wireframes Tela 1/Tela 3 · API Client da Fase 1 (LGN-90, entregue).
> Relação entre specs desta fase: esta spec entrega o shell, a tela de catálogo e a navegação rasa; o conteúdo do diálogo de criação (Tela 3) é `front-end-phase-2-template-detail-dialog`; o indicador online/offline é `front-end-phase-2-connection-indicator`.
> Status: **Approved** — aprovada pelo mantenedor em 2026-09-30 (premissas marcadas `n` aceitas como default). Spec da fase Specify (tlc-spec-driven) · gate `validate_spec.py --strict`: 0 erros, 0 warnings · origem: `.specs/features/front-end-phase-2-catalog/spec.md`.

## Problem Statement

O app desktop é um placeholder: existe um API Client tipado e testado (Fase 1), mas nenhuma tela consome `GET /templates` — o mantenedor não enxerga nenhum template disponível sem chamar curl. Sem shell (left nav / conteúdo / painel) e sem telas, as Fases 3–5 não têm onde nascer: o UI Engine precisa de uma tela, o ciclo da ficha precisa da navegação, e o indicador de conexão precisa do header.

## Goals

- [ ] Shell desktop construído uma única vez conforme `desktop/AGENTS.md`: left nav persistente (200–240px), conteúdo (máx. ~960px), right panel (280–320px) e header — tudo lido do tema único em `lib/theme/` (tokens: type 12/13/15/17, spacing 4·8·12·16·24, radius 2/4, row 32px, cores só do ColorScheme).
- [ ] Tela de catálogo (Tela 1) renderizando `GET /templates` com estados carregando / vazio / erro claramente distintos e retry acionável.
- [ ] Provider Riverpod do catálogo com cache em memória (nunca persistido) e refresh manual via F5.
- [ ] Navegação rasa Catálogo → diálogo de criação (Tela 3) → Catálogo, sem routing profundo na V1.
- [ ] Toda string de UI pela camada de localização (en, única locale da V1).

## Out of Scope

Explicitamente excluído. Documentado para prevenir scope creep.

| Feature | Reason |
| ------- | ------ |
| Conteúdo do diálogo de criação (resumo por prefixo, campo de nome, botão) | Spec `front-end-phase-2-template-detail-dialog` — aqui só existe a ação que abre a superfície |
| Indicador online/offline e polling de `GET /health` | Spec `front-end-phase-2-connection-indicator` — o shell apenas reserva o slot no header e no painel |
| `POST /sheets`, campo de nome funcional, biblioteca local "My sheets", abertura de ficha | Fase 4 (ciclo da ficha) — a API existe, mas criar ficha é entregue junto com o merge de cascata |
| Renderização de FieldSchema (inputs, toggles, agrupamento de campos) | Fase 3 (UI Engine) — o catálogo só lista templates |
| Busca, filtro e cards por gênero no catálogo | TDD Questão #4 — adiada; o volume atual (poucos templates) não justifica |
| Contagem de campos no card do catálogo | Decisão confirmada na Fase 1: a UI do catálogo ignora `fields` da lista; o resumo por prefixo vem do detalhe (Tela 3) |
| Persistir o catálogo em disco / cache offline | TDD: Local Store guarda índice de fichas, nunca catálogo; fonte de verdade é a API |
| Identidade visual (paleta, tipografia próprias) | V2 — a V1 usa Material defaults dentro dos tokens |
| Logs locais de requests, crash reporting | TDD: observabilidade local é Fase 5 / V2 |
| Atalhos Ctrl+N (nova ficha), Ctrl+S (salvar), Ctrl+1/Ctrl+2 | Chegam com as telas que os implementam (Fases 3/4) — ver premissas |

---

## Assumptions & Open Questions

Toda ambiguidade está resolvida ou registrada aqui — nada fica silenciosamente incerto.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | -------------- | --------- | ---------- |
| Estrutura do shell | Left nav (200–240px) + conteúdo + right panel (280–320px), construídos desde a Fase 2 | Decisão do mantenedor ("Left nav + content + right panel") — estrutura `desktop/AGENTS.md` desde a primeira tela, evita retrabalho nas Fases 3–5 | y |
| Conteúdo do right panel na Fase 2 | Seção Connection (estado + última checagem — entregue pela spec de conexão) e seção Catalog (nº de templates carregados + última atualização) | Honra a estrutura de painel escolhida sem inventar interação nova: nenhum dado que exija chamada extra. O slot "template summary" do painel entra na Fase 3, quando existir ficha aberta | n |
| Itens da left nav na Fase 2 | Apenas "Catalog" (ativo); novos itens entram com suas telas (Fase 4: "My sheets") | Evita item placeholder clicável sem tela; nav é estrutura, não inventário de features futuras | n |
| Card do catálogo | Mostra o nome do template (e o id como linha secundária); sem contagem de campos | FE1-08 decidiu que a UI do catálogo ignora `fields` da lista; o "26 campos" do wireframe Tela 1 é atendido pelo resumo por prefixo do detalhe (Tela 3) | y |
| Política de cache | Cache em memória no provider; primeira exibição carrega uma vez; F5 refaz a requisição e substitui a lista; nada é persistido | TDD: "catálogo de templates (cache em memória)"; Local Store nunca guarda catálogo | y |
| Requisições concorrentes | Single-flight: uma requisição de catálogo em voo por vez; gatilho extra durante o voo é ignorado | Evita corrida F5-espaçado renderizar resposta antiga por cima da nova | y |
| Falha em refresh com cache válido | Lista cacheada permanece visível + mensagem de erro inline; falha sem cache → estado de erro com Retry | Não piora uma tela que já tem dados por causa de um refresh manual; mantém o estado de erro claro exigido pelo épico quando não há o que mostrar | y |
| Atalhos registrados na Fase 2 | F5 (refresh do catálogo) e Esc (fechar diálogo); Ctrl+N/Ctrl+S/Ctrl+1/Ctrl+2 entram com suas telas | `desktop/AGENTS.md` fixa o registro central de atalhos, mas todo atalho precisa de ação real; registrar atalho sem ação é dizer falso ao usuário | n |
| Navegação | Superfície de criação é um diálogo modal aberto a partir do catálogo; Esc/fechar volta ao catálogo sem recarregar | TDD: "Navegação: fluxo raso de 2–3 telas… sem routing profundo na V1"; o wireframe Tela 3 é um diálogo, não uma rota | y |
| Copy de estados | Uma linha de texto puro localizado (sem arte decorativa), mensagens de erro vindas do mapper da Fase 1 | `desktop/AGENTS.md`: empty-state = uma linha de texto; "every async action shows progress and an inline result (success or mapped error)" | y |
| Ordem da lista | Ordem devolvida pela API; sem ordenação/filtragem própria | Nenhuma ordenação é especificada no TDD; ordenar é inventar comportamento | y |

**Open questions:** none — todas as ambiguidades foram resolvidas na discussão com o mantenedor ou registradas como premissas acima.

---

## User Stories

### P1: Shell desktop e tema único ⭐ MVP

**User Story**: Como mantenedor, quero o shell do app (left nav, header, conteúdo, painel) construído uma única vez a partir dos tokens de `desktop/AGENTS.md`, para que todas as telas das Fases 2–5 montem nele sem cada tela reinventar layout, tamanho ou cor.

**Why P1**: é a fundação da fase — o indicador de conexão e o diálogo de criação (outras duas specs) dependem do header e da área de conteúdo existirem.

**Acceptance Criteria**:

1. WHEN the app boots THEN the shell SHALL render a persistent left navigation, a top header, a content area and a right panel. (CAT-01)
2. WHEN the shell renders THEN the left navigation SHALL be 200–240px wide and the right panel SHALL be 280–320px wide, with both sizes read from the app theme (`lib/theme/`) — no widget-level magic numbers for size, radius, spacing or color. (CAT-02)
3. WHEN the window is too narrow to fit navigation + content + right panel THEN the shell SHALL hide the right panel first and SHALL never overlap the left navigation. (CAT-03)
4. The shell SHALL render exactly one left-navigation entry in Phase 2 — "Catalog" — and SHALL mark it active while the catalog is on screen. (CAT-04)
5. The header SHALL show the app title and SHALL reserve the connection-indicator slot consumed by the connection spec. (CAT-05)
6. The right panel SHALL render the sections defined in this phase's assumptions (Connection, Catalog) as separate blocks separated by the theme's hairline border. (CAT-06)

**Independent Test**: renderizar o shell com viewport largo → nav/painel nas faixas de largura e seções visíveis; encolher a janela → painel some antes de qualquer sobreposição.

---

### P1: Lista de templates com estados vazio, carregando e erro ⭐ MVP

**User Story**: Como mantenedor, quero a tela de catálogo (Tela 1) listando os templates de `GET /templates` com estados carregando/vazio/erro claros, para que eu veja o que a API tem antes de qualquer chamada manual.

**Why P1**: é o item de entrega número 1 do épico — sem a lista não existe catálogo, e as Fases 3–4 não têm ponto de partida.

**Acceptance Criteria**:

1. WHEN the catalog screen is first displayed THEN it SHALL request `GET /templates` exactly once through the Phase-1 API client and SHALL enter a loading state until the response settles. (CAT-07)
2. WHILE the first catalog request is in flight THEN the catalog SHALL show the loading state and SHALL NOT show the empty state. (CAT-08)
3. WHEN `GET /templates` succeeds with N entries THEN the catalog SHALL render N entries in API order, each showing the template name. (CAT-09)
4. IF `GET /templates` succeeds with an empty list THEN the catalog SHALL show a single line of plain localized text ("No templates available yet") and SHALL NOT show decorative art. (CAT-10)
5. IF the first catalog request fails THEN the catalog SHALL show the mapped error message produced by the Phase-1 error mapper together with a Retry action. (CAT-11)
6. WHEN the user activates Retry THEN the catalog SHALL re-request `GET /templates` exactly once. (CAT-12)
7. WHILE a catalog request is in flight, a further trigger (Retry or F5) SHALL NOT start a second concurrent catalog request. (CAT-13)
8. IF a template entry is missing a field required by the catalog model (malformed payload) THEN the catalog SHALL show the defensive parse failure as the error state with Retry — never a blank screen and never a partial list. (CAT-14)

**Independent Test**: transporte fake devolvendo 2 templates, lista vazia, 500 e JSON quebrado → os quatro estados distintos; contador de chamadas prova o single-flight.

---

### P1: Navegação rasa Catálogo → diálogo de criação ⭐ MVP

**User Story**: Como mantenedor, quero abrir a superfície de criação (Tela 3) a partir de um template no catálogo e voltar com Esc, para que a navegação da V1 seja rasa e as Fases 3–4 tenham o fluxo de entrada pronto.

**Why P1**: critério de aceite do épico ("navegação rasa Catálogo → criação/abertura de ficha, sem routing profundo na V1") — é o elo entre o catálogo e o detalhe do template.

**Acceptance Criteria**:

1. WHEN the user activates a catalog entry or its "Create sheet" action THEN the app SHALL open the creation surface for that template id. (CAT-15)
2. WHEN the creation surface is dismissed (Esc or its close action) THEN the app SHALL return to the catalog with the cached list intact and SHALL NOT re-request `GET /templates`. (CAT-16)
3. The app SHALL keep navigation shallow — catalog ↔ creation dialog only — with no deep routing stack in V1. (CAT-17)

**Independent Test**: clicar em um template → superfície de criação abre com o id certo; Esc → volta ao catálogo sem nova chamada de lista.

---

### P2: Refresh do catálogo (F5)

**User Story**: Como mantenedor, quero atualizar o catálogo com um atalho, para que um template recém-adicionado no backend apareça sem reiniciar o app.

**Why P2**: o catálogo muda raramente (templates default do backend); sem refresh a única saída é reiniciar, mas o valor não justifica bloquear o MVP.

**Acceptance Criteria**:

1. WHEN the user triggers refresh (F5) on the catalog screen THEN the catalog SHALL re-request `GET /templates` and replace the rendered list with the new result. (CAT-18)
2. IF a refresh fails while cached entries exist THEN the catalog SHALL keep rendering the cached entries and SHALL show the mapped error inline. (CAT-19)

**Independent Test**: primeira carga com 1 template, F5 devolvendo 2 → lista atualizada; F5 falhando → lista antiga permanece + erro inline.

---

## Edge Cases

- IF the catalog request succeeds but the payload's `data` key is absent THEN the catalog SHALL take the defensive parse failure path (error state with Retry) — never an empty success (CAT-14 covers the model side).
- WHEN a catalog entry's name is an empty string THEN the entry SHALL still render (layout intact) instead of collapsing or throwing.
- IF the user triggers F5 on a screen other than the catalog THEN no catalog request SHALL be issued.
- WHEN the window is resized below the panel threshold and restored THEN the right panel SHALL reappear without a catalog reload.
- Varredura de dimensões implícitas: input validation & bounds → nenhum input do usuário nesta spec (só navegação e refresh); failure/partial-failure → CAT-11/CAT-14/CAT-19; idempotência/retry → CAT-12/CAT-13 (single-flight, Retry exato de uma chamada); concorrência/ordenação → CAT-13 (single-flight) + ordem da API preservada (CAT-09); external-dependency failure → mapa de erro da Fase 1 já cobre transporte/4xx/5xx; state-transition integrity → transições de navegação CAT-15/16/17. N/A nesta spec: auth/rate limits (V1 sem auth), data lifecycle/expiry (cache em memória, não persistido — descarte por descarte de processo), observabilidade (logs são Fase 5).

---

## Requirement Traceability

Cada requisito tem ID único para rastreamento em design, tasks e validação.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------ |
| CAT-01 | P1: Shell desktop e tema único | Design | Verified |
| CAT-02 | P1: Shell desktop e tema único | Design | Verified |
| CAT-03 | P1: Shell desktop e tema único | Design | Verified |
| CAT-04 | P1: Shell desktop e tema único | Design | Verified |
| CAT-05 | P1: Shell desktop e tema único | Design | Verified |
| CAT-06 | P1: Shell desktop e tema único | Design | Verified |
| CAT-07 | P1: Lista de templates com estados | Design | Verified |
| CAT-08 | P1: Lista de templates com estados | Design | Verified |
| CAT-09 | P1: Lista de templates com estados | Design | Verified |
| CAT-10 | P1: Lista de templates com estados | Design | Verified |
| CAT-11 | P1: Lista de templates com estados | Design | Verified |
| CAT-12 | P1: Lista de templates com estados | Design | Verified |
| CAT-13 | P1: Lista de templates com estados | Design | Verified |
| CAT-14 | P1: Lista de templates com estados | Design | Verified |
| CAT-15 | P1: Navegação rasa Catálogo → diálogo | Design | Pending |
| CAT-16 | P1: Navegação rasa Catálogo → diálogo | Design | Pending |
| CAT-17 | P1: Navegação rasa Catálogo → diálogo | Design | Pending |
| CAT-18 | P2: Refresh do catálogo (F5) | Design | Verified |
| CAT-19 | P2: Refresh do catálogo (F5) | Design | Verified |

**Coverage:** 19 requisitos no total, 19 mapeados 1:1 a critérios de aceite; 0 não mapeados.

---

## Success Criteria

Como sabemos que a fase foi bem-sucedida:

- [ ] O catálogo renderiza a lista de templates da API com os 4 estados claramente distintos (carregando, vazio, erro, lista) — critério de aceite do épico.
- [ ] O shell existe uma única vez (`lib/theme/` + estrutura) e as outras duas specs da fase montam nele sem duplicar layout.
- [ ] Abrir e fechar a superfície de criação não dispara nenhuma requisição de lista — navegação rasa sem efeitos colaterais.
- [ ] Nenhum literal de UI nos widgets: todas as strings vêm da camada de localização (en).
- [ ] `flutter test` verde com transporte fake (sem API de pé) e `flutter analyze` limpo.
