# [Front End] Fase 1 — API Client Context

**Gathered:** 2026-09-29
**Spec:** `.specs/features/front-end-phase-1-api-client/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Entrega a camada de rede isolada do app desktop: 6 endpoints tipados (`/health`, `/templates`, `/templates/:id`, `POST/GET/PATCH /sheets`), decode do envelope `{"data"}`/`{"error"}`, models com `data` dinâmico + acesso pontilhado, mapeamento `apperr` → kind de erro de domínio da UI, URL base compilada com override de dev, timeout/retry idempotente e testes com fixtures reais (Marco M1). Não entrega estado, telas, merge de cascata nem auth.

---

## Implementation Decisions

### Idioma / localização da copy

- Camada de localização desde a Fase 1, com `en` como **única locale** — "precisamos ter localization, no início apenas inglês".
- Nenhum literal de UI no client: toda mensagem de erro passa pela camada. Multi-idioma é V2+.

### Timeout e retry (fail-fast)

- 5s por tentativa (fail-fast — decidido pelo mantenedor, troca do original 10s).
- GET: exatamente **1 retry** (2 tentativas), backoff fixo 300ms, **apenas** falha de transporte (timeout/conexão/DNS).
- POST/PATCH: 1 tentativa, nunca retry.
- 4xx/5xx nunca retryable → resposta definitiva, "retry manual" (TDD).
- Pior caso ~10,3s até o kind `offline` (2× mais rápido que a política anterior de 10s/500ms).

### Acesso por caminho pontilhado (`Sheet.data`)

- Segmento ausente ou intermediate que não é mapa → `null`, nunca lança (dado ausente é legítimo).
- Path vazio ou segmento vazio (`stats..dexterity`) → **exceção tipada do client** (bug de renderização vs schema, distinto de dado ausente).

### URL base / override de dev (TDD Questão #2)

- Default compilado: `https://api.legnaris.com.br` (hostname já em `deploy/cloudflared/config.yml`).
- Override: `--dart-define=API_BASE_URL=...`, **sem** gate de `kDebugMode` — debug local e artifact da CI podem apontar para túnel/local.
- "Exclusivo de dev" = não existe setting de usuário; `http://` só entra por override explícito (TLS obrigatório).
- Base não-absolute (`api:8080`, vazia) → erro de configuração antes de qualquer requisição.

### Teste de contrato

- Fixtures fixadas no CI (capturadas de respostas reais, 6 endpoints + envelope de erro + par delta do PATCH).
- Suite de contrato ao vivo nos 4 cenários do TDD, **self-skip** sem API alcançável (padrão do repo) — P2, não bloqueia M1.

### Agent's Discretion

Decisões técnicas registradas como premissas na spec (Confirm = n), livres para o Design refinar forma, não conteúdo:

- Stack HTTP: `package:http` + `http.Client` injetável (sem `dio`); timeout/retry manuais.
- Client como classe pura, sem provider Riverpod nesta fase (providers por feature vêm nas Fases 2–4).
- Model de catálogo decodifica o mesmo `TemplateDTO` do detalhe (a API devolve `fields` na lista).
- `/health` segue o envelope padrão `{"data": ...}` — o TDD marca como "próprio", o código manda (`internal/platform/health`); divergência a feedback ao TDD.
- Logs locais de requests fora daqui (Fase 5); o erro de domínio já carrega status/code/message como premissão.
- Códigos sem kind dedicado (`CONFLICT`, `CYCLE_DETECTED`, `UNAUTHORIZED`...) caem no fallback `unknown`.

---

## Specific References

- Tabela `apperr` → kind de erro de domínio copia o tratamento da coluna "Tratamento na UI" do TDD (`docs/TDD-desktop-mvp.md`), com `rede/timeout` → `offline`.
- Contrato extraído do código, não do TDD: `shared/response/response.go` (envelope), `shared/apperr/err.go` (códigos + status), `internal/app/{router,sheet,template}.go` (rotas), `internal/sheet/dto.go` e `internal/sheet/template/dto.go` (shapes), `internal/platform/health` (envelope do health).
- Exemplo de delta do PATCH citado no épico: request `{"data": {"stats": {"dexterity": 16}}}` → 200 com só cascatas `{"data": {"modifiers": {...}, "derived": {...}}}`.

---

## Deferred Ideas

- Polling de `/health` e indicador "online" na UI → Fase 5.
- Cache em memória do catálogo → Fase 2.
- Multi-idioma (pt-BR) → V2+.
- Retry configurável/circuit breaker → fora (TDD manda retry simples).
