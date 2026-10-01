# [Front End] Fase 2 — Indicador de conexão: health check periódico (online/offline)

> Spec da fase Specify (`tlc-spec-driven`) · origem: `.specs/features/front-end-phase-2-connection-indicator/spec.md` · gate `validate_spec.py --strict`: 0 erros, 0 warnings · épico: [LGN-91](https://paulovalleriotedev.atlassian.net/browse/LGN-91) · fonte: `docs/TDD-desktop-mvp.md` + `desktop/AGENTS.md`

> Fonte: [LGN-98](https://paulovalleriotedev.atlassian.net/browse/LGN-98) · [LGN-91](https://paulovalleriotedev.atlassian.net/browse/LGN-91) (épico de fase, sob [LGN-88](https://paulovalleriotedev.atlassian.net/browse/LGN-88)) · `docs/TDD-desktop-mvp.md` (aprovado — fonte da verdade) · `desktop/AGENTS.md` (regras de construção) · wireframes Tela 1/Tela 2 (`* online`) · contrato real: `GET /health` → `{"data":{"status":"healthy"}}` (client da Fase 1, método `health()`).
> Relação entre specs desta fase: depende de `front-end-phase-2-catalog` (header e painel onde o indicador é montado); entrega em paralelo com `front-end-phase-2-template-detail-dialog`.
> Status: **Approved** — aprovada pelo mantenedor em 2026-09-30 (premissas marcadas `n` aceitas como default).

## Problem Statement

O TDD exige um indicador de conexão visível (health check periódico leve → estado online/offline visível), mas hoje o app não tem nenhuma forma de saber se a API está alcançável: com a Fase 6 do backend pendente, a instância pode estar fora do ar, no túnel ou trocada no build — e o mantenedor só descobriria ao tentar abrir um template. O indicador precisa refletir a realidade em tempo hábil sem travar a navegação.

## Goals

- [ ] Polling de `GET /health` a cada 15s enquanto o app está aberto, com uma única requisição em voo por vez.
- [ ] Três estados distintos — checking, online, offline — atualizados sem ação do usuário e sem bloquear navegação.
- [ ] Indicador no header (canto superior direito, como nos wireframes Tela 1/2) mais detalhe no painel direito (estado + última checagem).
- [ ] Scheduler injetável: a suíte de testes não espera 15s reais.
- [ ] Toda string de UI pela camada de localização (`en`, única locale da V1).

## Out of Scope

Explicitamente excluído. Documentado para prevenir scope creep.

| Feature | Reason |
| --- | --- |
| Ações degradadas por estado offline (desabilitar Salvar, fila de requisições, banner de reconexão) | TDD coloca offline/conectividade no escopo da Fase 5 (Polish) — aqui só o indicador |
| Retry/backoff próprios do health check | O client da Fase 1 já fixa a política (1 retry de 300ms em falha de transporte, timeout 5s); duplicar seria um segundo mecanismo |
| Pausar o polling com a janela minimizada | V1: custo de 1 GET/15s é desprezível e o estado segue verdadeiro ao restaurar; gating de ciclo de vida é complexidade sem ganho |
| Logs locais de requests, métricas, crash reporting | TDD: observabilidade local é Fase 5 / V2 |
| Indicador persistido entre sessões / estado "última known" | Estado de conexão é tempo-de-execução; persistir criaria estado mentiroso no boot |
| Health check de componentes internos, detalhe de latência/uptime | Sem observabilidade client-side na V1 além do indicador |
| Lista de templates e estados de catálogo | Spec `front-end-phase-2-catalog` |
| Detalhe do template e diálogo de criação | Spec `front-end-phase-2-template-detail-dialog` |

---

## Assumptions & Open Questions

Toda ambiguidade está resolvida ou registrada aqui — nada fica silenciosamente incerto.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --- | --- | --- | --- |
| Intervalo do polling | 15 segundos entre inícios de checagem | Decisão do mantenedor ("Polling a cada 15s") — queda refletida em até ~15s + tempo da requisição, com 4 req/min | y |
| Semântica dos estados | checking até a primeira resposta; online em 2xx; offline em falha de transporte (após o retry do client) ou 4xx/5xx | Decisão do mantenedor (checking/online/offline); 4xx/5xx é resposta definitiva de que a API não está servindo — tratar como offline evita indicador mentiroso | y |
| Janela de detecção | Pior caso ~16s até offline (agendamento 15s + timeout 5s + retry 300ms + 5s); retorno a online na próxima checagem bem-sucedida | Derivada da política fail-fast fixada na Fase 1 — sem retry novo no health check | y |
| Posição do indicador | Header, canto superior direito, em qualquer tela | Decisão do mantenedor ("No header") — wireframes Tela 1/Tela 2 mostram `* online` no topo | y |
| Detalhe no painel direito | Seção Connection: estado atual + timestamp da última checagem bem-sucedida ("never" antes da 1ª) | Aproveita a estrutura de painel escolhida para dar profundidade ao indicador compacto do header, sem chamada extra | n |
| Concorrência do scheduler | Single-flight: tick que dispara com requisição em voo é descartado; agendamento por tempo, independente do tráfego de catálogo/detalhe | Evita acúmulo de requisições em rede lenta e evita que refresh de catálogo desloque o agendamento | y |
| Ciclo de vida | Polling roda enquanto o app está aberto (sem gating por minimização/foco) | Simplicidade; custo desprezível; o indicador permanece verdadeiro quando a janela é restaurada | y |
| Fonte do estado | Somente o health check define o estado; resultados de outras requisições (catálogo, detalhe) NÃO alteram o indicador | Uma fonte = um comportamento testável; sem regra "qualquer sucesso = online" escondida | n |
| Efeito na navegação | O indicador nunca bloqueia navegação nem ações do catálogo/diálogo | Critério de aceite do épico ("sem travar a navegação") | y |
| Testabilidade | O agendador é injetável (timer controlável em teste); nenhum teste espera 15s reais | Padrão do repo (transporte injetável da Fase 1); sem isso a suíte fica lenta ou frágil | y |

**Open questions:** none — todas as ambiguidades foram resolvidas na discussão com o mantenedor ou registradas como premissas acima.

---

## User Stories

### P1: Polling de GET /health com estados checking/online/offline ⭐ MVP

**User Story**: Como mantenedor, quero o app sondando a API periodicamente e refletindo o estado online/offline sem ação minha, para que eu saiba em segundos se a instância está de pé antes de tentar usar o catálogo.

**Why P1**: critério de aceite do épico ("indicador de conexão: health check periódico leve → estado online/offline visível") e mitigação direta do risco "API inacessível (Fase 6 pendente, túnel)".

**Acceptance Criteria**:

1. WHEN the app starts THEN it SHALL begin polling `GET /health` through the Phase-1 API client, starting a new check every 15 seconds while the app runs. (CONN-01)
2. WHILE no health response has been received yet THEN the indicator SHALL show the checking state. (CONN-02)
3. WHEN a health check succeeds (2xx) THEN the indicator SHALL show the online state. (CONN-03)
4. WHEN a health check fails — transport failure after the client's retry, or a 4xx/5xx response — THEN the indicator SHALL show the offline state. (CONN-04)
5. WHILE a health check is in flight, a scheduled tick SHALL NOT start a second concurrent health request. (CONN-05)
6. WHEN the state changes THEN the header indicator SHALL update without user action and SHALL hold that state until a new check resolves. (CONN-06)
7. The polling schedule SHALL be time-based and independent of other API traffic — catalog or detail requests SHALL NOT start, stall or reset it. (CONN-07)

**Independent Test**: agendador de teste dispara ticks → 1º sucesso = online, falha de transporte = offline, 500 = offline; tick durante o voo não gera 2ª requisição; chamadas de catálogo em paralelo não mexem no cronômetro.

---

### P1: Indicador no header, visível em qualquer tela ⭐ MVP

**User Story**: Como mantenedor, quero o indicador no canto superior direito do header e o detalhe no painel direito, para que o estado da conexão seja consultável a qualquer momento sem tapetar a tela.

**Why P1**: o wireframe mostra `* online` em todas as telas; sem o slot pronto, a Fase 5 teria que redesenhar o shell.

**Acceptance Criteria**:

1. WHEN the shell renders THEN the header SHALL show the connection indicator in its top-right slot on every screen, with localized text for checking, online and offline. (CONN-08)
2. WHEN the right panel renders THEN it SHALL show a Connection section with the current state and the timestamp of the last successful check, or "never" before the first success. (CONN-09)
3. WHILE the indicator shows offline, the app SHALL NOT block navigation, catalog actions or dialog actions. (CONN-10)
4. The indicator SHALL resolve every visible string through the localization layer (`en`, the only locale of V1) — no UI literals in widgets. (CONN-11)

**Independent Test**: três estados no header em qualquer tela; painel mostra "never" antes do 1º sucesso e a marca depois; com offline, navegar e abrir o diálogo continuam funcionando.

---

### P2: Testabilidade do agendador

**User Story**: Como mantenedor, quero o polling dirigido por um agendador injetável, para que a suíte prove o comportamento sem esperar tempo real e sem deixar teste pendurado.

**Why P2**: não muda o comportamento visto pelo usuário, mas sem isso os testes da fase ficam lentos ou frágeis — e a regra "15s" fica inverificável.

**Acceptance Criteria**:

1. WHEN the test suite runs THEN it SHALL drive health ticks through an injectable scheduler — no test SHALL wait real 15-second intervals. (CONN-12)
2. WHEN the health endpoint responds 2xx with a status payload other than `healthy` THEN the indicator SHALL still show online — any decoded 2xx payload counts as reachable. (CONN-13)

**Independent Test**: injetar scheduler manual → ticks sob comando do teste e asserts de estado em milissegundos; resposta 2xx com outro status → online.

---

## Edge Cases

- IF the app starts with the API unreachable THEN the indicator SHALL settle on offline after the client's fail-fast policy (~10.3s) plus the current tick, and SHALL recover to online on the next successful check — no restart required.
- WHEN a check succeeds while the state is offline THEN the indicator SHALL transition to online immediately (the reverse of CONN-04, no manual refresh).
- IF the health response is 2xx but its body lacks `data.status` THEN the client SHALL raise the defensive parse failure and the indicator SHALL show offline — a body the app cannot decode is not a healthy signal.
- WHEN two checks would overlap (slow network, tick during flight) THEN only the in-flight check SHALL count; no duplicate request SHALL be issued.
- Varredura de dimensões implícitas: input validation & bounds → nenhum input do usuário; failure/partial-failure → CONN-03/04 + casos de borda acima (parse defensivo → offline); idempotência/retry → herdada do client (1 retry GET), single-flight no scheduler (CONN-05); concorrência/ordenação → CONN-05/07 (single-flight, agendamento independente); observabilidade → timestamp da última checagem no painel (CONN-09); external-dependency failure → o próprio offline é o fallback; state-transition integrity → transições checking→online, offline→online (CONN-02/06). N/A nesta spec: auth/rate limits (V1 sem auth; `/health` é público), data lifecycle/expiry (nenhum estado persistido).

---

## Requirement Traceability

Cada requisito tem ID único para rastreamento em design, tasks e validação.

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| CONN-01 | P1: Polling de GET /health com estados | Design | Implementing |
| CONN-02 | P1: Polling de GET /health com estados | Design | Implementing |
| CONN-03 | P1: Polling de GET /health com estados | Design | Implementing |
| CONN-04 | P1: Polling de GET /health com estados | Design | Implementing |
| CONN-05 | P1: Polling de GET /health com estados | Design | Implementing |
| CONN-06 | P1: Polling de GET /health com estados | Design | Implementing |
| CONN-07 | P1: Polling de GET /health com estados | Design | Implementing |
| CONN-08 | P1: Indicador no header, visível em qualquer tela | Design | Implementing |
| CONN-09 | P1: Indicador no header, visível em qualquer tela | Design | Implementing |
| CONN-10 | P1: Indicador no header, visível em qualquer tela | Design | Implementing |
| CONN-11 | P1: Indicador no header, visível em qualquer tela | Design | Implementing |
| CONN-12 | P2: Testabilidade do agendador | Design | Implementing |
| CONN-13 | P2: Testabilidade do agendador | Design | Implementing |

**Coverage:** 13 requisitos no total, 13 mapeados 1:1 a critérios de aceite; 0 não mapeados.

---

## Success Criteria

Como sabemos que a fase foi bem-sucedida:

- [ ] Indicador online/offline reflete o health check sem travar a navegação — critério de aceite do épico.
- [ ] Estado offline visível em até ~16s após a API cair (agendamento + política fail-fast da Fase 1); retorno a online na próxima checagem bem-sucedida.
- [ ] Um único health check em voo mesmo com rede lenta (ticks descartados durante a requisição).
- [ ] Zero espera real de 15s na suíte de testes e zero literal de UI no indicador.
- [ ] `flutter test` verde sem API de pé e `flutter analyze` limpo.
