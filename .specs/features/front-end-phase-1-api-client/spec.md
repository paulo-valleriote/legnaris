# [Front End] Fase 1 — API Client: cliente tipado, envelope e mapeamento apperr

> Fonte: [LGN-90](https://paulovalleriotedev.atlassian.net/browse/LGN-90) (épico de fase, sob [LGN-88](https://paulovalleriotedev.atlassian.net/browse/LGN-88)) · `docs/TDD-desktop-mvp.md` (aprovado — fonte da verdade) · contrato real: `shared/response`, `shared/apperr`, `internal/app/*` · Planejamento: Fase 1 (2 dias úteis), depende da Fase 0 (LGN-89, entregue), bloqueia as Fases 2–5.
> Status: **Approved** — 2026-09-29 · premissas confirmadas pelo mantenedor.

## Problem Statement

O frontend desktop tem toolchain pronta (Fase 0) mas nenhuma forma de falar com a API: os seis endpoints consumidos (`/health`, `/templates`, `/templates/:id`, `POST/GET/PATCH /sheets`) só seriam acoplados às telas, espalhando decode de envelope, código de erro e URL base por toda a UI. O TDD aponta o API Client como camada isolada — o ponto exato onde a V2 vai adicionar auth sem reescrever features — e o Marco M1 exige contrato validado: client tipado testado contra fixtures fixadas do contrato real antes de qualquer tela existir.

## Goals

- [ ] Seis endpoints cobertos por métodos tipados num client isolado (transporte injetável — ponto de extensão único para auth da V2).
- [ ] Envelope `{"data"}`/`{"error":{code,message}}` decodificado num único lugar, com parse defensivo nos dois lados.
- [ ] Todo código `apperr` mapeado para um kind de erro de domínio da UI; código desconhecido → fallback genérico; copy resolvida pela camada de localização (en, única locale da V1).
- [ ] Models com `data` dinâmico e acesso por caminho pontilhado (`stats.dexterity`) prontos para o UI Engine (Fase 3) e o merge de cascata (Fase 4).
- [ ] URL base hardcoded (https da API pública) + override de dev via `--dart-define`; timeout fail-fast de 5s e retry idempotente só para GET.
- [ ] **Marco M1**: `flutter test` verde no CI com fixtures reais + suite de contrato self-skip.

## Out of Scope

Explicitamente excluído. Documentado para prevenir scope creep.

| Feature | Reason |
| ------- | ------ |
| Merge incremental do delta de cascata e dirty state | Fase 4 (ciclo da ficha) — esta fase só decodifica o delta |
| Estado Riverpod por feature (providers, cache do catálogo, ficha aberta) | Fases 2–4 — o client desta fase é classe pura com transporte injetável |
| Telas, navegação, indicador online, polling periódico de `/health` | Fases 2/3/5 — aqui só existe o método `health()` |
| Auth, headers, tokens, cookie jar | V2 (Fases 1/2 do backend) — a camada isolada é o ponto de extensão, não a implementação |
| Local Store / biblioteca local de fichas | Fase 4 |
| Localização multi-idioma (pt-BR, etc.) | V2+ — esta fase ergue a camada de localização com `en` como única locale |
| Mapeamento de `FieldSchema.type` para widgets (number/string/boolean → input/toggle) | Fase 3 (UI Engine) — `type` é string opaca aqui |
| Logging local de requests, crash reporting | TDD: observabilidade local é Fase 5 / V2; o domínio de erro já carrega status/code/message |
| Retry configurável, circuit breaker, cache offline, backoff exponencial | TDD pede "retry simples" — política fixa decidida na spec |
| `GET /sheets` (lista), `DELETE`, import/export | Não existem no backend (Fases 2/4 do backend) |
| Testes widget/golden e build MSIX | Fases 3 e 5 |

---

## Assumptions & Open Questions

Toda ambiguidade está resolvida ou registrada aqui — nada fica silenciosamente incerto.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | -------------- | --------- | ---------- |
| Stack HTTP do client | `package:http` com `http.Client` injetável; timeout/retry manuais (sem `dio`) | Pacote oficial, superfície mínima e `MockClient` de fábrica para as fixtures; a política tem 1 retry — interceptors/interceptors de `dio` não pagam a dependência; transporte injetável já é o ponto de extensão da V2 | y |
| Idioma / localização da copy de erro | Camada de localização desde a Fase 1 com `en` como única locale; nenhuma string de UI literal no client | Decisão do mantenedor: "precisamos ter localization, no início apenas inglês" — coerente com o AGENTS.md (English only) e prepara V2 sem reescrever copy | y |
| Política de timeout/retry (fail-fast) | 5s por tentativa; GET: 1 retry (2 tentativas) com backoff fixo de 300ms, **apenas** em falha de transporte; POST/PATCH: 1 tentativa; 4xx/5xx nunca retryable | Decisão do mantenedor (fail-fast); pior caso ~10,3s até `offline` — UI cai 2× mais rápido para estado offline; contraponto consciente: um túnel particularmente lento pode estourar os 5s (o retry de 300ms absorve a queda/reerguida do túnel); 4xx/5xx são resposta definitiva → "retry manual" (TDD) | y |
| Acesso por caminho pontilhado anômalo | Segmento ausente / intermediate que não é mapa → `null` (nunca lança); path vazio ou segmento vazio (`stats..dexterity`) → exceção tipada do client | Decisão do mantenedor: separar "dado ausente" (legítimo) de "path malformado" (bug de renderização vs schema, sinalizado) | y |
| URL base default | `https://api.legnaris.com.br` | Hostname já reservado em `deploy/cloudflared/config.yml` (`api.legnaris.com.br` → API pública); TLS obrigatório (TDD Segurança); Fase 6 do backend pendente não muda o default compilado | y |
| Mecânica do override de dev | `--dart-define=API_BASE_URL=...` lido em build, **sem** gate de `kDebugMode`; `http://` aceito apenas quando o override foi passado explicitamente | Decisão do mantenedor; permite que qualquer build (debug local ou artifact da CI) aponte para túnel/local — hoje a API pública ainda não existe; "exclusivo de dev" = não existe setting de usuário, só quem compila com a flag (TDD Questão #2) | y |
| Teste de contrato | Fixtures fixadas no CI (exigência do épico) **+** suite de contrato ao vivo self-skip nos 4 cenários do TDD | Decisão do mantenedor; padrão do repo (self-skip sem serviço externo) e valida o M1 de verdade sem quebrar CI | y |
| Envelope do `/health` | `{"data": {"status": "healthy"}}` (envelope padrão) | O TDD marca `/health` como envelope "próprio", mas `internal/platform/health` usa `response.OK` — o código é a fonte do contrato; divergência registrada para feedback ao TDD | y |
| Forma do catálogo | `GET /templates` devolve o mesmo `TemplateDTO` do detalhe (id, name, fields, default); o model de catálogo decodifica tudo e a UI da Fase 2 ignora `fields` na lista | Fato do código (`TemplateController.List` → `ToDTO()` por item); evita um segundo model desnecessário agora | y |
| Estado, providers e cache | Client é classe pura (sem provider Riverpod nesta fase); providers por feature e cache em memória do catálogo vêm nas Fases 2–4 | O épico entrega a camada de rede; estado é assunto das fases de tela (TDD: "State (Riverpod) — providers por feature") | y |
| Logs locais de requests | Fora desta fase (Fase 5); o domínio de erro carrega status/code/message como premissão de observabilidade | TDD Monitoramento pede logs leves, mas não é critério do épico; capturar os dados agora custa pouco e evita retrabalho | y |
| Códigos sem kind dedicado (`CONFLICT`, `CYCLE_DETECTED`, `INVALID_DEPENDENCY`, `INVALID_PATH`, `UNAUTHORIZED`, `FORBIDDEN`) | Caem no fallback genérico `unknown` | `UNAUTHORIZED`/`FORBIDDEN` são V2 (sem auth na V1); os 400 de engine/DAG não têm tratamento de UI definido no TDD — fallback cobre sem inventar comportamento | y |

**Open questions:** none — todas as ambiguidades foram resolvidas na discussão com o mantenedor ou registradas como premissas acima.

---

## User Stories

### P1: Client tipado por endpoint + decode de envelope ⭐ MVP

**User Story**: Como mantenedor, quero uma camada de rede isolada que fale os 6 endpoints consumidos com types e decode de envelope num único lugar, para que as Fases 2–5 consumam a API sem reimplementar contrato e a V2 adicione auth num ponto só.

**Why P1**: o TDD define o API Client como a base de todas as fases; sem ele, decode, erros e URL base vazam para as telas.

**Acceptance Criteria**:

1. The client SHALL expose exactly one typed method per consumed endpoint — `GET /health`, `GET /templates`, `GET /templates/:id`, `POST /sheets`, `GET /sheets/:id`, `PATCH /sheets/:id` — each returning the payload decoded for that endpoint. (FE1-01)
2. WHEN uma resposta 2xx chega com envelope `{"data": ...}` THEN o client SHALL retornar o payload decodificado no tipo daquele endpoint. (FE1-02)
3. WHEN uma resposta 4xx/5xx chega com envelope `{"error": {"code", "message"}}` THEN o client SHALL falhar com um erro de domínio carregando `code` e `message`. (FE1-03)
4. IF um corpo 2xx não é JSON válido ou não tem a chave `data` THEN o client SHALL falhar com erro de parse defensivo, nunca retornar sucesso vazio. (FE1-04)
5. WHEN o client chama `POST /sheets` THEN o corpo da requisição SHALL ser `{"template_id": "<id>", "sheet_name": "<nome>"}`. (FE1-05)
6. WHEN o client chama `PATCH /sheets/:id` THEN o corpo da requisição SHALL ser `{"data": {...}}` contendo apenas o delta informado pelo chamador. (FE1-06)
7. The client SHALL executar toda requisição através de um transporte HTTP injetado na construção — nenhum método de endpoint cria o próprio transporte. (FE1-07)

**Independent Test**: com transporte fake injetado, chamar os 6 métodos e conferir método/rota/corpo; devolver envelope de sucesso e de erro; devolver JSON quebrado e esperar o erro de parse.

---

### P1: Models com `data` dinâmico e acesso por caminho pontilhado ⭐ MVP

**User Story**: Como mantenedor, quero models fiéis ao contrato (`TemplateCatalog`/`TemplateDetail`/`FieldSchema`/`Sheet`) com `data` dinâmico e accessor pontilhado, para que o UI Engine (Fase 3) e o merge de cascata (Fase 4) operem sobre tipos corretos desde já.

**Why P1**: `data` é intencionalmente dinâmico — o valor do produto é renderizar qualquer schema; decodificar errado aqui contaminaria todas as fases.

**Acceptance Criteria**:

1. WHEN o catálogo ou o detalhe de template é decodificado THEN o model SHALL expor `id`, `name`, `default` e `fields`, preservando a árvore folha/grupo (folha = `schema`, grupo = `children`). (FE1-08)
2. WHEN um schema folha é decodificado THEN o model SHALL expor `type`, `label`, `formula`, `depends_on`, `is_editable` e `default`, tratando `formula` e `depends_on` como ausentes quando omitidos no JSON. (FE1-09)
3. WHEN uma ficha é decodificada THEN o model SHALL expor `id`, `template_id`, `name`, `data`, `created_at` e `updated_at`, com `data` preservando sua estrutura aninhada dinâmica. (FE1-10)
4. WHEN um número é decodificado de `data` THEN inteiros e doubles SHALL ser preservados como estão (16 continua `int`, 16.5 continua `double`), sem truncamento nem cast forçado. (FE1-11)
5. WHEN um caminho pontilhado resolve para um valor (ex.: `stats.dexterity`) THEN o accessor SHALL retornar esse valor. (FE1-12)
6. IF um segmento do caminho não existe ou um nó intermediário não é mapa THEN o accessor SHALL retornar null e SHALL NÃO lançar exceção. (FE1-13)
7. IF o caminho é vazio ou contém segmento vazio (ex.: `stats..dexterity`) THEN o accessor SHALL lançar um erro tipado de client-side. (FE1-14)

**Independent Test**: decodificar fixtures reais de template e ficha; acessar path válido, ausente, com intermediate não-mapa e com segmento vazio; decodificar `16` e `16.5` e conferir os tipos Dart.

---

### P1: Mapeamento `apperr` → erro de domínio da UI ⭐ MVP

**User Story**: Como mantenedor, quero que cada código `apperr` da API vire um kind de erro de domínio da UI com copy localizada, para que as fases de tela tratem falha por estado e não por parsing de JSON espalhado.

**Why P1**: critério de aceite central do épico — é o contrato que transforma resposta HTTP em estado que a UI sabe exibir.

**Mapeamento** (códigos reais de `shared/apperr/err.go`):

| `apperr` code | HTTP | Kind de erro de domínio | Tratamento esperado nas Fases 2–5 |
| ------------- | ---- | ----------------------- | --------------------------------- |
| `INVALID_JSON` | 400 | `clientFault` | bug do client: log + mensagem técnica |
| `INVALID_REQUEST` | 400 | `invalidEdit` | edição inválida: reverter campo, informar read-only |
| `UNKNOWN_FIELD` | 400 | `renderMismatch` | bug renderização vs schema: log + reverter |
| `NOT_FOUND` | 404 | `notFound` | recurso não existe: remover da biblioteca local |
| `INTERNAL_ERROR` | 500 | `serverError` | erro no servidor + retry manual |
| rede / timeout (após retries) | — | `offline` | estado offline claro, edições mantidas |
| qualquer outro código | — | `unknown` | fallback genérico (V2: `UNAUTHORIZED`/`FORBIDDEN`) |
| corpo sem `error.code` | — | *(não é um kind)* | erro de parse defensivo |

**Acceptance Criteria**:

1. WHEN a API retorna `INVALID_JSON` THEN o client SHALL mapeá-lo para o kind `clientFault`. (FE1-15)
2. WHEN a API retorna `INVALID_REQUEST` THEN o client SHALL mapeá-lo para o kind `invalidEdit`. (FE1-16)
3. WHEN a API retorna `UNKNOWN_FIELD` THEN o client SHALL mapeá-lo para o kind `renderMismatch`. (FE1-17)
4. WHEN a API retorna `NOT_FOUND` THEN o client SHALL mapeá-lo para o kind `notFound`. (FE1-18)
5. WHEN a API retorna `INTERNAL_ERROR` THEN o client SHALL mapeá-lo para o kind `serverError`. (FE1-19)
6. WHEN a requisição falha no transporte após esgotar os retries THEN o client SHALL mapeá-la para o kind `offline`. (FE1-20)
7. IF a API retorna um `error.code` fora do mapeamento (ex.: `CONFLICT`, `CYCLE_DETECTED`) THEN o client SHALL mapeá-lo para o kind genérico `unknown`, preservando o código cru e a message do servidor no objeto de erro. (FE1-21)
8. IF um corpo 4xx/5xx não é JSON válido ou não tem `error.code` THEN o client SHALL falhar com erro de parse defensivo em vez de um erro de domínio mapeado. (FE1-22)
9. The client SHALL resolver toda mensagem de erro visível ao usuário pela camada de localização, com inglês como única locale da V1 — sem literais de UI hardcoded no client. (FE1-23)
10. WHEN um erro de domínio é produzido THEN ele SHALL carregar o status HTTP (quando recebido), o código `apperr` (quando presente) e a message do servidor, para logging futuro não precisar refazer a chamada. (FE1-24)

**Independent Test**: tabela-driven sobre fixtures — cada código → kind esperado; código desconhecido → `unknown` com código preservado; corpo sem `error.code` → parse defensivo; erro de transporte → `offline`; copiar a string exibida e conferir que vem da camada de localização.

---

### P1: URL base hardcoded com override de dev ⭐ MVP

**User Story**: Como mantenedor (dev em Linux, uso em Windows), quero a URL base da API compilada no build com override exclusivo de dev, para que o app aponte para a API pública por padrão e para túnel/local quando eu compilar com a flag.

**Why P1**: Questão #2 do TDD resolvida como "hardcode + override dev" — sem ela o app não alcança nenhuma instância (Fase 6 do backend pendente).

**Acceptance Criteria**:

1. The client SHALL usar `https://api.legnaris.com.br` como base URL quando o build não define `API_BASE_URL`. (FE1-25)
2. WHEN o build define `API_BASE_URL` via `--dart-define` THEN o client SHALL usá-la como base URL em qualquer modo de build (debug e release). (FE1-26)
3. IF a base URL resolvida não é uma URL absoluta `http://` ou `https://` THEN o client SHALL falhar com erro de configuração antes de emitir qualquer requisição. (FE1-27)

**Independent Test**: build sem flag → base default https; build com flag http de túnel → base do override; flag malformada (`api:8080`, string vazia) → erro de configuração sem nenhuma requisição emitida.

---

### P1: Timeout e retry idempotente ⭐ MVP

**User Story**: Como mantenedor, quero timeout previsível e retry simples restrito a GET, para que a UI caia para `offline` em tempo previsível e nenhuma mutação seja reenviada automaticamente.

**Why P1**: critério de aceite do épico ("retry idempotente apenas para GET") e pré-condição para o estado offline das fases de tela.

**Acceptance Criteria**:

1. The client SHALL aplicar timeout fail-fast de 5 segundos a cada tentativa de requisição. (FE1-28)
2. WHEN uma tentativa de GET falha no transporte (timeout, conexão recusada, DNS) THEN o client SHALL repetir o GET exatamente uma vez após delay fixo de 300ms. (FE1-29)
3. WHEN um GET recebe resposta 4xx ou 5xx THEN o client SHALL NÃO repetir a requisição e SHALL expor imediatamente o erro mapeado. (FE1-30)
4. WHEN uma tentativa de `POST` ou `PATCH` falha por qualquer motivo THEN o client SHALL NÃO repetir a requisição. (FE1-31)

**Independent Test**: transporte fake que falha no 1º GET e sucesso no 2º → 2 chamadas; falha duas vezes → erro `offline`; 500 no GET → 1 chamada; POST/PATCH falhando → 1 chamada; medir timeout nos 5s e o intervalo de 300ms entre tentativas.

---

### P1: Fixtures do contrato real no CI ⭐ MVP

**User Story**: Como mantenedor, quero a suite do client rodando contra fixtures fixadas de respostas reais, para que o CI valide o contrato sem depender de API alcançável e qualquer drift quebre o build.

**Why P1**: critério de aceite do épico e definição do Marco M1 ("fixtures do contrato real passando no CI — contrato validado").

**Acceptance Criteria**:

1. WHEN `flutter test` roda (local e CI) THEN a suite do client SHALL executar contra fixtures fixadas, sem exigir API alcançável, e SHALL terminar com exit 0. (FE1-32)
2. The fixture set SHALL conter ao menos uma resposta real capturada para cada um dos 6 endpoints consumidos, além do envelope de erro e do par delta do PATCH (request `{"data": {"stats": {"dexterity": 16}}}` → resposta só com cascatas). (FE1-33)
3. IF o envelope ou a forma de campos de uma fixture divergir do que os models esperam (drift de contrato) THEN a suite SHALL falhar. (FE1-34)

**Independent Test**: rodar `flutter test` sem nenhuma API de pé (CI) → verde; alterar uma chave de uma fixture → suite vermelha.

---

### P2: Contrato ao vivo self-skip

**User Story**: Como mantenedor, quero uma suite de contrato que bata na API real quando ela estiver acessível e se auto-pule quando não, para que o contrato seja validado de verdade sem poluir o CI.

**Why P2**: não bloqueia o M1 (as fixtures já o cobrem), mas é a evidência direta de que o client consome o backend real — vem depois do P1.

**Acceptance Criteria**:

1. WHEN nenhuma base de API alcançável está configurada para testes THEN a suite de contrato ao vivo SHALL se auto-pular e a execução geral SHALL terminar com exit 0. (FE1-35)
2. WHEN uma base de API alcançável está configurada (flag/env) THEN a suite SHALL executar os 4 cenários do TDD: `POST /sheets` → 201 com valores derivados; `PATCH` em campo editável → 200 com delta só de cascatas; `PATCH` em campo derivado/desconhecido → 400 `INVALID_REQUEST`/`UNKNOWN_FIELD`; `GET /sheets/:id` inexistente → 404 `NOT_FOUND`. (FE1-36)
3. IF uma base alcançável está configurada e um cenário retorna status ou código `apperr` inesperado THEN a suite SHALL falhar com o status e código observados — nunca skip silencioso. (FE1-37)

**Independent Test**: sem API → pula e sai 0; com API local (`make run`) → 4 cenários verdes; apontar para endpoint errado → falha explícita.

---

## Edge Cases

- IF um corpo 2xx chega vazio (sem JSON) THEN o client SHALL falhar com erro de parse defensivo (FE1-04 cobre o caso; nenhum endpoint do backend devolve 2xx vazio hoje).
- IF o override `API_BASE_URL` é string vazia THEN o client SHALL cair no default https, nunca em base vazia.
- WHEN a base é `http://` via override THEN ela SHALL ser aceita apenas por explícito comando de build; sem override, a base SHALL ser a default https (TLS obrigatório — TDD Segurança).
- WHEN a fixture do PATCH delta é lida THEN ela SHALL refletir que a resposta contém **apenas** os efeitos em cascata — o merge incremental dela é Fase 4, não desta fase.
- The key JSON `default` do `TemplateDTO` SHALL ser mapeada explicitamente no model Dart (palavra reservada da linguagem), sem depender de case convention.
- Varredura de dimensões implícitas: **input validation & bounds** → parse defensivo de envelope + validação da base URL + validação de path (FE1-04, FE1-14, FE1-22, FE1-27); **failure/partial-failure** → `offline` sem estado parcial (client sem estado); **idempotência/retry** → FE1-28–31; **observabilidade** → FE1-24 (status/code/message no erro); **external-dependency failure** → retry + `offline` (FE1-20, FE1-29). N/A nesta fase: **auth/rate limits** (V1 sem auth — camada isolada é o ponto de extensão da V2), **concorrência/ordenação** (client sem estado compartilhado; ordem e estado de UI são Riverpod, Fases 2–4), **data lifecycle/expiry** (client não cacheia — cache do catálogo é Fase 2 e `data` nunca é cacheado localmente), **state-transition integrity** (sem transição de estado — dirty state é Fase 4).

---

## Requirement Traceability

Cada requisito tem ID único para rastreamento em design, tasks e validação.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------ |
| FE1-01 | P1: Client tipado por endpoint + decode de envelope | Execute | Verified |
| FE1-02 | P1: Client tipado por endpoint + decode de envelope | Execute | Verified |
| FE1-03 | P1: Client tipado por endpoint + decode de envelope | Execute | Verified |
| FE1-04 | P1: Client tipado por endpoint + decode de envelope | Execute | Verified |
| FE1-05 | P1: Client tipado por endpoint + decode de envelope | Execute | Verified |
| FE1-06 | P1: Client tipado por endpoint + decode de envelope | Execute | Verified |
| FE1-07 | P1: Client tipado por endpoint + decode de envelope | Execute | Verified |
| FE1-08 | P1: Models com `data` dinâmico e acesso por caminho pontilhado | Execute | Verified |
| FE1-09 | P1: Models com `data` dinâmico e acesso por caminho pontilhado | Execute | Verified |
| FE1-10 | P1: Models com `data` dinâmico e acesso por caminho pontilhado | Execute | Verified |
| FE1-11 | P1: Models com `data` dinâmico e acesso por caminho pontilhado | Execute | Verified |
| FE1-12 | P1: Models com `data` dinâmico e acesso por caminho pontilhado | Execute | Verified |
| FE1-13 | P1: Models com `data` dinâmico e acesso por caminho pontilhado | Execute | Verified |
| FE1-14 | P1: Models com `data` dinâmico e acesso por caminho pontilhado | Execute | Verified |
| FE1-15 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-16 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-17 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-18 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-19 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-20 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-21 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-22 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-23 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-24 | P1: Mapeamento `apperr` → erro de domínio da UI | Execute | Verified |
| FE1-25 | P1: URL base hardcoded com override de dev | Execute | Verified |
| FE1-26 | P1: URL base hardcoded com override de dev | Execute | Verified |
| FE1-27 | P1: URL base hardcoded com override de dev | Execute | Verified |
| FE1-28 | P1: Timeout e retry idempotente | Execute | Verified |
| FE1-29 | P1: Timeout e retry idempotente | Execute | Verified |
| FE1-30 | P1: Timeout e retry idempotente | Execute | Verified |
| FE1-31 | P1: Timeout e retry idempotente | Execute | Verified |
| FE1-32 | P1: Fixtures do contrato real no CI | Execute | Verified |
| FE1-33 | P1: Fixtures do contrato real no CI | Execute | Verified |
| FE1-34 | P1: Fixtures do contrato real no CI | Execute | Verified |
| FE1-35 | P2: Contrato ao vivo self-skip | Execute | Verified |
| FE1-36 | P2: Contrato ao vivo self-skip | Execute | Verified |
| FE1-37 | P2: Contrato ao vivo self-skip | Execute | Verified |

**Coverage:** 37 requisitos no total, 37 mapeados 1:1 a critérios de aceite; 0 não mapeados. Tasks ainda não criadas (fase Tasks roda após a confirmação desta spec).

---

## Success Criteria

Como sabemos que a fase foi bem-sucedida:

- [ ] `flutter test` verde no CI com fixtures reais — **Marco M1: client tipado testado, contrato validado**.
- [ ] Os 6 endpoints cobertos por métodos tipados, com decode de envelope e mapeamento `apperr` completos (tabela inteira + fallback + parse defensivo).
- [ ] Uma mudança de contrato no backend (envelope ou campo) quebra a suite de fixtures — drift detectado antes das telas.
- [ ] `POST`/`PATCH` nunca disparam retry automático; GET repete exatamente uma vez em falha de transporte; timeout fail-fast em 5s (~10,3s no pior caso até `offline`).
- [ ] Build sem flag usa `https://api.legnaris.com.br`; build com `--dart-define` usa a URL informada; flag malformada falha antes de qualquer requisição.
- [ ] Entrega dentro da estimativa do épico (2 dias úteis).
