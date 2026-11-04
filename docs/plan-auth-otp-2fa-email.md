# Plano — Auth: 2FA por email, verificação e entrega via RabbitMQ

| Campo          | Valor                                                                                     |
| -------------- | ----------------------------------------------------------------------------------------- |
| Tech Lead      | @pmv                                                                                      |
| Time           | @pmv (projeto solo)                                                                       |
| Status         | Proposto — decisões fechadas, aguardando spec                                            |
| Criado         | 2026-10-20                                                                                |
| Última Edição  | 2026-10-20                                                                                |
| Escopo         | 2FA por email em todo login, verificação de email, códigos de 6 dígitos, Resend, RabbitMQ |
| Referências    | `docs/TDD.md`, `.specs/features/006-backend-browser-auth-cookies/`, `.specs/features/007-web-auth-session/`, `AGENTS.md` |
| Fora de escopo | Spec, implementação, reset de senha, TOTP/backup codes, troca de email, marketing         |

> **Como ler este documento.** Ele registra decisões, descobertas e o desenho acordado
> para orientar a spec. Não é uma spec: não define requisitos numerados, histórias de
> usuário, critérios de aceite nem tarefas. Nada aqui foi implementado.

---

## 1. Resumo executivo

O objetivo é endurecer o fluxo de autenticação do Legnaris com:

1. **2FA obrigatório** em todo login: senha + código de 6 dígitos enviado por email.
2. **Verificação de email** no cadastro, com sessão emitida somente depois do código.
3. **Usuários existentes** verificam o email no primeiro login pós-deploy — o próprio
   código de 2FA cumpre esse papel, sem um fluxo separado.
4. **Resend** como provedor de email transacional.
5. **RabbitMQ** como transporte do envio, com padrão de outbox para não perder emails.

A feature é viável na arquitetura atual, mas **depende de correções prévias** em pontos
que hoje passariam despercebidos em testes e quebrariam a operação em produção. Os
achados estão na seção 7; os principais são o rate limit por IP efetivamente global
(atrás do Traefik) e o `Refresh` que não consulta o usuário.

### 1.1 Decisões de produto

| ID  | Decisão                                                                 | Consequência principal |
| --- | ----------------------------------------------------------------------- | ---------------------- |
| D1  | 2FA por email em **todo** login (senha + código)                        | Login vira duas etapas; nenhuma sessão no primeiro passo |
| D2  | Sessão só é emitida **após verificar** o email                          | Cadastro não devolve mais `SessionBody`; contrato muda |
| D3  | Código numérico de **6 dígitos**                                        | Gerado com `crypto/rand`, hasheado com HMAC, TTL curto, 5 tentativas |
| D4  | Usuários existentes **verificam no primeiro login**                     | Sem backfill; o código de 2FA marca `email_verified_at` |
| D5  | Provedor de email: **Resend**                                           | SDK Go oficial, API HTTPS, free tier com teto diário de 100 |
| D6  | Envio assíncrono via **RabbitMQ**, com **outbox durável no Mongo**      | API não fala com o broker; relay publica; worker envia |

### 1.2 Decisões operacionais

| ID  | Decisão                                                                 | Consequência principal |
| --- | ----------------------------------------------------------------------- | ---------------------- |
| O1  | Retry no broker: **TTL + DLX por tier**                                 | Filas `email.retry.30s/.2m/.5m`; sem delayed retry nativo nem plugin |
| O2  | Relay em **processo separado** (`cmd/relay`)                            | Três binários: `api`, `relay`, `worker`; lease atômico no outbox |
| O3  | Superfície HTTP **unificada**                                           | `POST /auth/challenge/verify` e `POST /auth/challenge/resend` |
| O4  | Resend **free tier no MVP**                                             | Teto de 100/dia; guarda de cota e gatilho de upgrade documentados |
| O5  | Verificação checada **no `Refresh`**                                    | Sessões antigas deixam de renovar sem `email_verified_at` |
| O6  | Management UI **exposta via `ops.` + oauth2-proxy**                     | Rota protegida no Traefik; nunca pública, payloads de OTP visíveis |

---

## 2. Contexto atual verificado

O backend já tem autenticação de navegador consolidada. Fatos relevantes para esta
entrega, todos verificados no código:

- **Rotas**: `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`, montadas em
  `internal/app/router.go:59-61`. O router é criado com `echo.New()` em
  `internal/app/router.go:19`, sem `IPExtractor`.
- **Serviço**: `internal/auth/service.go` concentra `Register` (linha 130), `Login` (189),
  `Refresh` (216) e o `signIn` compartilhado. `Register` e `Login` emitem sessão
  imediatamente; `Refresh` emite access token apenas com o `user_id` do token persistido
  (linha 240), sem carregar o usuário.
- **Sessão**: access token JWT HS256 de 15 min, refresh opaco de 30 dias em cookie
  `__Host-` `HttpOnly` + `SameSite=Strict`, com rotação e detecção de reuso
  (`internal/auth/token.go`, `cookie.go`, `refresh.go`).
- **Usuário**: `internal/user/user.go` tem `ID`, `Email`, `Username`, `PasswordHash`,
  timestamps. Não existe `email_verified_at` nem status de conta.
- **Repositório**: `internal/user/repository.go:13-17` expõe apenas `Create`,
  `FetchByEmail` e `FetchByID`. Não há update.
- **Validação de email**: `Register` chama `mail.ParseAddress(email)` e **descarta o
  resultado** (`internal/auth/service.go:134`), persistindo a string original.
- **Rate limit**: `internal/platform/ratelimit/limiter.go` é um sliding window em memória,
  por processo. `internal/app/ratelimit.go:78-79` usa `ip:<c.RealIP()>`.
- **Config**: `shared/env/env.go:66-68` exige apenas `DATABASE_URI` e `JWT_SECRET`.
  `deploy/compose_test.go` valida essa lista contra o compose.
- **Mongo**: `deploy/docker-compose.yml:149-150` usa `mongo:7.0` standalone — sem
  transações multi-documento e sem change streams.
- **Índices**: criados no boot por `shared/db/indexes.go` (`EnsureIndexes`), de forma
  idempotente. Não há framework de migração de dados.
- **Front-end**: `web/src/api/auth_api.ts` expõe exatamente quatro operações e
  `web/src/auth/auth_session.ts` conhece quatro estados (`restoring`,
  `unauthenticated`, `authenticating`, `authenticated`). O gate renderiza `AuthScreen`
  para qualquer estado não autenticado (`web/src/auth/auth_gate.tsx`).
- **Erros**: `web/src/api/error_mapper.ts` mapeia apenas os códigos atuais; qualquer código
  novo cai em `unknown` e mostra cópia genérica.
- **Contrato**: a feature 006 fixa que o cadastro autentica (`AUTH-02` em
  `.specs/features/006-backend-browser-auth-cookies/spec.md`). A feature 007 repete essa
  premissa no cliente (`WAS-02`).
- **Build/deploy**: `cmd/api.go:66` sobe o servidor com `router.Start`, sem graceful
  shutdown. O `Dockerfile` produz um único binário; o compose não tem broker nem worker.

---

## 3. Desenho alvo

### 3.1 Fluxos

#### Cadastro (D2 + D3)

1. `POST /auth/register` valida email, username e senha; canonicaliza o email.
2. Cria o usuário com `email_verified_at = nil`.
3. Cria um challenge `verify_email` com código de 6 dígitos.
4. Grava a intenção de envio no outbox.
5. Responde `201` com `challenge_id`, `expires_in` e `resend_after` — **sem cookie e
   sem access token**.
6. O usuário informa o código em `POST /auth/challenge/verify`, com o `challenge_id`
   devolvido no cadastro (propósito `verify_email`).
7. Consumo atômico do challenge, marcação de `email_verified_at` e emissão da sessão.

O código do cadastro é o segundo fator da primeira sessão: a senha foi definida no
cadastro e o código prova a posse do email. Não há um terceiro código.

#### Login (D1)

1. `POST /auth/login` recebe email e senha.
2. Email inexistente ou senha errada → `401` genérico, com bcrypt “dummy” no caminho de
   email inexistente para não criar oráculo de timing.
3. Credenciais válidas → cria challenge `login_otp`, grava o outbox e responde `202`
   com `challenge_id` e TTL. **Nenhuma sessão é emitida aqui.**
4. `POST /auth/challenge/verify` consome o challenge `login_otp`:
   - se `email_verified_at` era `nil`, marca agora (cumpre D4);
   - emite a sessão (cookies + access token).

O primeiro passo nunca revela se o email existe nem se está verificado: a resposta é
`202` para qualquer credencial válida, e o código só é enviado se a conta existir.

#### Usuário existente (D4)

Não há backfill e não há tela de “verificar email”. No primeiro login após o deploy:

1. senha correta → challenge `login_otp`;
2. código correto → `email_verified_at = now` + sessão.

Sessões antigas precisam ser contidas: `Refresh` deve carregar o usuário e recusar quem
está sem verificação. Sem isso, uma sessão criada antes do deploy continua renovando por
até 30 dias, e “verificar no login” deixa de ser verdade.

### 3.2 Endpoints propostos (desenho, não contrato)

| Endpoint                       | Entrada                | Saída                                              |
| ------------------------------ | ---------------------- | -------------------------------------------------- |
| `POST /auth/register`          | email, username, senha | `201 {challenge_id, expires_in, resend_after}`     |
| `POST /auth/challenge/verify`  | challenge_id, código   | `200` sessão                                       |
| `POST /auth/challenge/resend`  | challenge_id           | `202` (ou `503` se a cota global esgotar)          |
| `POST /auth/login`             | email, senha           | `202 {challenge_id, ...}` ou `401`                 |
| `GET /auth/me`                 | Bearer                 | `{id, email, username, email_verified}`            |

Na superfície unificada (O3), o `purpose` do challenge (`verify_email` | `login_otp`) fica
no servidor e determina o efeito do `verify`: ambos emitem sessão; `verify_email` marca a
verificação;
`login_otp` marca também se `email_verified_at` ainda for `nil` (D4). O cliente não
escolhe o propósito nem o envia no corpo.

O `resend` é idempotente do ponto de vista do cliente: `202`, sujeito a cooldown, com o
mesmo código até o teto de expiração; só a indisponibilidade global de cota vira `503`.
Um `challenge_id` inexistente, consumido ou expirado responde igual, para não revelar
estado.

`GET /auth/me` é necessário porque hoje o front não tem como saber o estado de
verificação após um reload. Sem ele, o único momento em que o cliente sabe que a conta
está verificada é a resposta do login.

### 3.3 Modelo de dados

`users`:
- `email_verified_at *time.Time` — `nil` para todos os usuários existentes; **sem
  backfill** (D4).

`auth_challenges`:
- `_id` (UUID v7 ou 128 bits aleatórios), `user_id`, `purpose`
  (`verify_email` | `login_otp`), `code_hash`, `expires_at`, `consumed_at`, `attempts`,
  `created_at`, `last_sent_at`.
- Um challenge ativo por `{user_id, purpose}`; um novo login invalida o anterior.
- Consumo atômico em um único documento, porque o Mongo é standalone:
  1. `findOneAndUpdate` com filtro `consumed_at: null`, `expires_at > now`,
     `attempts < 5`, aplicando `$inc: {attempts: 1}`;
  2. comparação do hash em tempo constante;
  3. segundo `findOneAndUpdate` com o `code_hash` no filtro para “claimar” o consumo.

**Reenvio não rotaciona o código.** Ele reenvia o mesmo código, estende `expires_at` até
um teto absoluto (ex.: 30 min) e respeita cooldown de 60s. Isso elimina a corrida em que
um email antigo chega depois do reenvio e o código exibido já não vale. O teto evita que
sucessivos reenvios mantenham um código válido indefinidamente.

### 3.4 Invariantes de segurança e limites

- Código gerado com `crypto/rand` (nunca `math/rand`), 6 dígitos com zero à esquerda.
- Hash HMAC-SHA256 com `EMAIL_CODE_SECRET` **separado** do `JWT_SECRET`; a mensagem do
  HMAC inclui `user_id` e `purpose` para impedir reuso cruzado.
- Comparação com `hmac.Equal`/`subtle.ConstantTimeCompare`.
- 5 tentativas por challenge; ao esgotar, invalida e exige novo challenge.
- Limites por challenge, por usuário, por email, por IP e um teto global diário de envios.
- Código nunca vai para log, resposta HTTP ou assunto do email.
- Challenge só é criado depois de credencial válida (senha no login, cadastro válido na
  verificação). Sem isso, alguém poderia pedir código para o email de outra pessoa.
- `time.Now()` no auth service deve virar clock injetado antes dos testes de TTL e
  cooldown.

### 3.5 Front-end

- `AuthApi` ganha as operações de challenge; `register` deixa de devolver `SessionBody`.
- `AuthSession` ganha um estado de “pendente de código” e o gate não pode tratar `202`
  como sessão.
- Tela de código com `autocomplete="one-time-code"`, `inputmode="numeric"`, suporte a
  colar e contador de reenvio.
- `error_mapper.ts` precisa mapear os novos códigos; hoje eles cairiam em `unknown`.
- O catálogo `l10n/strings.ts` precisa das cópias correspondentes.
- Testes existentes de register/login/gate/session mudam em bloco.

---

## 4. Envio de email — Resend (D5)

### 4.1 Escolha

Resend via API HTTPS, com o SDK oficial `github.com/resend/resend-go/v4`:

- free tier de 3.000 emails/mês, **100/dia**; Pro ~US$ 20/50k;
- `SendWithContext` + idempotency key;
- setup de domínio guiado (SPF/DKIM), webhooks de bounce/complaint;
- região EU disponível.

O MVP usa o **free tier** (O4). O upgrade para Pro é uma troca de configuração, não de
código, porque o provedor fica atrás da interface `EmailSender`.

Alternativas consideradas:

| Provedor     | Quando faz sentido                                                |
| ------------ | ----------------------------------------------------------------- |
| **Postmark** | Entrega crítica, melhor inbox placement, logs de 45 dias          |
| **Amazon SES** | Já está na AWS, quer custo mínimo e aceita operar reputação      |
| **Mailtrap/Mailpit** | Dev/staging e QA manual                                    |

Não recomendado: SMTP direto da VPS. Além da reputação do IP e das portas 25/587, o
`net/smtp` do Go está congelado (bug fixes apenas). Se SMTP for requisito, usar
`github.com/wneessen/go-mail` contra um relay.

### 4.2 Capacidade e cota no MVP

O free tier tem **100 emails/dia**, não apenas 3.000/mês. Com 2FA em todo login, isso é
aproximadamente 100 logins por dia no total, ou ~50 cadastros se cada um consumir dois
emails. O MVP aceita esse teto (O4), mas precisa de guardas explícitas:

- **Teto global diário configurável**, abaixo do limite do provedor (ex.: 80), com alerta
  em 70%. O contador é compartilhado entre API e worker; no MVP, uma coleção
  `email_quota` (ou contagem de linhas do outbox por dia) resolve sem infraestrutura
  extra.
- Ao atingir o teto, **não enfileirar** novos envios e responder `503` com `Retry-After`
  e mensagem genérica (sem revelar conta), além de métrica e alerta. O `202` uniforme
  vale para o caminho normal; esconder a indisponibilidade de cota deixaria o usuário
  esperando um email que não vai chegar.
- O `resend` também consome cota; o cooldown de 60s e o limite por usuário são a primeira
  defesa.
- Sessões de 30 dias reduzem a frequência de 2FA e, portanto, o consumo diário.
- **Gatilho de upgrade**: mais de 60 emails/dia por 3 dias seguidos, lançamento público
  ou qualquer campanha. Nesse ponto, mudar para Pro (50k/mês) ou SES.
- **Não usar o free tier em load test.** Testes e QA usam o sender `log`/fake; o provedor
  só é exercitado em smoke test manual.

### 4.3 Configuração de domínio

- Subdomínio dedicado, ex.: `auth.legnaris.com.br`, para isolar reputação.
- SPF, DKIM e DMARC (`p=none` → `p=quarantine` após validar).
- Return-Path alinhado; `Reply-To` monitorado; corpo texto + HTML.
- Webhook de bounce/complaint, ou a base acumula endereços inválidos e queima reputação.
- Não colocar o código no assunto (aparece em notificação/lock screen).

### 4.4 Abstração

Uma interface `EmailSender` no domínio, com adapters `resend`, `smtp` (se necessário) e
`log` (dev/test). Templates embutidos (`embed` + `text/template`/`html/template`) no
pacote de email. Testes usam fake sender; nenhum teste unitário chama o provedor.

---

## 5. RabbitMQ e entrega assíncrona (D6)

### 5.1 O problema: dual-write

A API precisa gravar o challenge no Mongo e publicar o email no RabbitMQ. Sem
coordenação, há três janelas de falha:

| Falha                   | Consequência                                             |
| ----------------------- | -------------------------------------------------------- |
| Mongo OK, publish falha | Usuário sem código; depende de resend                    |
| Publish OK, Mongo falha | Email com código de um challenge que não existe          |
| Crash entre os dois     | Um dos lados perdido                                     |

Como a sessão só existe após o código, perder o email trava o usuário. Por isso a
recomendação é **outbox no Mongo + relay para o RabbitMQ**, e não publish direto do
handler. A atomicidade do outbox no Mongo standalone atual tem uma ressalva importante,
descrita na seção 5.3.

### 5.2 Arquitetura recomendada

```
API ──(1) user + challenge + outbox──▶ MongoDB
                                         │
                            (2) relay poll │ lease + publish confirm
                                         ▼
                                    RabbitMQ
                              exchange legnaris.email (direct)
                                         │
                         ┌───────────────┴───────────────┐
                         ▼                               ▼
                 email.otp (quorum)              email.retry.30s/2m/5m
                         │                               │
                 (3) worker consome ──▶ Resend          │
                         │                               │
                         ├── transient ──▶ retry ────────┘
                         ├── expirado ───▶ ack + métrica
                         └── permanente ─▶ email.dead (DLQ)
```

Componentes:

1. **API** (`cmd/api`): grava usuário + challenge + linha no `email_outbox`. Não fala com
   o RabbitMQ. Responde `201`/`202` imediatamente.
2. **Relay** (`cmd/relay`, processo separado — O2): busca linhas pendentes, publica com
   publisher confirm, marca `published_at`. Lease atômico permite mais de um relay.
3. **RabbitMQ**: exchange, fila principal, filas de retry e DLQ.
4. **Worker** (`cmd/worker`): consome, validade de expiração, renderiza o template, chama
   o Resend, faz ack/nack.

Separar o worker do processo HTTP evita que uma falha de envio derrube a API e permite
escalar/restartar o consumo de forma independente.

### 5.3 Outbox

```
email_outbox
  _id             uuidv7 (vira message_id e idempotency key)
  to              string
  template        string
  payload         json (código, expiração, propósito)
  status          pending | published | failed
  attempts        int
  next_attempt_at time
  locked_until    time | null
  locked_by       string
  created_at      time
  published_at    time | null
  last_error      string
```

Índices: `{status, next_attempt_at}` e TTL para linhas publicadas antigas (ex.: 7 dias).

**Atomicidade no MVP.** O Mongo do compose é standalone e não tem transações
multi-documento. A ordem de escrita é: usuário + challenge e, por último, a linha do
outbox. Se o processo cair entre as duas, a conta/challenge existe sem email; o usuário
pede reenvio, que cria uma nova linha. Não há perda silenciosa do código, mas também não
há atomicidade real entre challenge e outbox. Com replica set (opção futura), as três
escritas viram uma transação única.

Relay:
- `findOneAndUpdate` com `status=pending`, `next_attempt_at <= now`, lease expirado →
  `locked_until = now + 30s`; ordenação por `next_attempt_at`; lote de 10–50.
- Publica com `PublishWithDeferredConfirm` + `WaitContext` (timeout de 5s).
- Confirmado → `published`; nack/timeout → `attempts++`, backoff (5s, 15s, 1m, 5m) e
  libera o lease.
- Se o relay cair depois de publicar e antes de marcar, republica: duplicata. O `_id`
  como idempotency key do Resend resolve.
- Poll de 250ms–1s. Change streams seriam mais elegantes, mas o Mongo do compose é
  standalone e não suporta change streams nem transações.

### 5.4 Topologia

| Entidade                                  | Tipo/args                                      | Função                    |
| ----------------------------------------- | ---------------------------------------------- | ------------------------- |
| `legnaris.email`                          | direct, durable                                | exchange principal        |
| `email.otp`                               | quorum, durable, `x-dead-letter-exchange`     | trabalho real; DLX → retry 30s |
| `legnaris.email.retry`                    | direct, durable                                | exchange de retry         |
| `email.retry.30s`, `.2m`, `.5m`           | quorum/classic, `x-message-ttl`, DLX de volta | backoff                   |
| `email.dead`                              | quorum, durable                                | DLQ, inspeção/replay      |

- **Quorum queue** na principal: durável, poison message handling, at-least-once
  dead-lettering. Em nó único não há HA, mas há durabilidade em disco.
- **Filas de retry separadas por delay** evitam head-of-line blocking. Uma fila única com
  `Expiration` por mensagem faz uma mensagem de 5 min bloquear as de 30s atrás dela.
- **Decisão O1: TTL + DLX por tier.** O delayed retry nativo do RabbitMQ 4.3+ existe, mas
  não é usado: evita acoplar o deploy a uma versão específica e mantém o backoff
  explícito na topologia. O plugin `x-delayed-message` também não é usado.
- `email.otp` separada de futuros `email.notification`: OTP é latência-crítica e não pode
  ficar atrás de volume.
- Fila com `x-max-length` e alerta; sem limite, um broker ou worker parado acumula tudo.

### 5.5 Contrato da mensagem

```json
{
  "id": "uuid-v7-do-outbox",
  "type": "email.otp",
  "to": "user@example.com",
  "template": "otp_login",
  "data": {
    "code": "123456",
    "expires_in_minutes": 10,
    "purpose": "login_otp"
  },
  "created_at": "2026-10-20T12:00:00Z",
  "attempt": 0
}
```

- `id` = idempotency key no Resend.
- `attempt` explícito; o worker incrementa ao reagendar. Não depender só de `x-death`.
- O código vai na mensagem porque é gerado e hasheado na API. A alternativa (worker gera
  e atualiza o hash) cria corrida: o usuário pode tentar verificar antes de o worker
  gravar o hash.
- Renderização no worker mantém a mensagem pequena e permite corrigir template sem
  republicar. API e worker devem ser deployados juntos por causa da versão do template.
- **PII/credencial no broker**: a mensagem contém email e código. Restringir vhost e
  usuário, manter a rede interna, TTL curto e sem retenção desnecessária. O management só é
  acessível via `ops.` autenticado (O6), nunca publicamente. Se o broker sair da rede
  interna, `amqps` + TLS.

### 5.6 Publisher e consumer

Publisher (`amqp091-go`, client oficial):

- `amqp.DialConfig` com `Recovery` e `NotifyStateChange` (a lib atual tem recuperação
  embutida; a antiga regra “não reconecta” não vale mais).
- Canal exclusivo para publicar; canais não são seguros para uso concorrente.
- `Confirm(false)` + `PublishWithDeferredConfirm` + `WaitContext`.
- `mandatory=true` + `NotifyReturn`: confirm só diz que o broker aceitou, não que roteou.
- `NotifyBlocked` para memory/disk alarm: logar e parar de publicar até desbloquear.

Consumer:

- `Qos(prefetch=10, 0, false)`, `autoAck=false`.
- Sucesso do Resend → `Ack`.
- 429/5xx/timeout → `Nack(requeue=false)` para o retry (ou publica no retry e ack).
- 4xx permanente (endereço inválido) → publica no DLQ, ack, métrica.
- Código expirado (`expires_at` no payload) → ack sem enviar; não adianta entregar OTP
  vencido.
- Timeout por envio (ex.: 10s) com `context`.
- Shutdown: cancelar o consumer, drenar in-flight, fechar conexão. Hoje `cmd/api.go` não
  tem graceful shutdown; o worker precisa disso desde o início.
- Reconexão: `Recovery` + supervisor que re-registra o consumer após `StateOpen` ou
  fechamento de canal.
- Deduplicação: o Resend tem `IdempotencyKey`; usar `id`. A garantia real é at-least-once,
  não exactly-once.

### 5.7 Retry — decisão e alternativas

| Abordagem                          | Como                                              | Prós                                   | Contras                                      | Status |
| ---------------------------------- | ------------------------------------------------- | -------------------------------------- | -------------------------------------------- | ------ |
| **A. TTL + DLX por tier**          | 3–4 filas com TTL e DLX de volta                  | Funciona em qualquer 4.x; controle total; sem plugin | Mais filas para declarar/operar; `attempt` manual | **Escolhida (O1)** |
| **B. Delayed retry nativo (4.3+)** | Política de quorum queue                          | Menos entidades; backoff linear        | Exige 4.3+; menos flexível para backoff custom | Não escolhida |
| **C. App/outbox agenda o retry**   | Worker grava nova linha no outbox                 | Backoff exato, auditável, DLQ simples  | Worker escreve no Mongo; mistura responsabilidades | Alternativa |

A topologia de A é a da seção 5.4: em falha transitória, o worker publica no exchange de
retry com a routing key do tier (`30s`, `2m`, `5m`) conforme o `attempt`, e cada fila de
retry tem TTL e DLX de volta para `legnaris.email`. `Nack(requeue=false)` fica como
fallback para a DLX principal. O `attempt` é incrementado a cada reagendamento e o teto é
5; ao estourar, a mensagem vai para `email.dead` e o alerta dispara. O outbox **não**
participa do retry de envio — ele só garante a publicação inicial.

### 5.8 Deploy no stack atual

- Serviço `rabbitmq` (`rabbitmq:4-management`, com o minor pinado no deploy), redes
  `internal` e `edge`, volume `rabbitmq_data`, **sem portas publicadas**, healthcheck
  `rabbitmq-diagnostics -q ping`. A rede `edge` existe só para o Traefik alcançar a
  management UI (O6).
- Três binários na mesma imagem: `cmd/api`, `cmd/relay` e `cmd/worker`. O relay é um
  **processo separado** (O2), com lease atômico no outbox; a API não executa relay.
  `depends_on` de RabbitMQ/Mongo apenas para `relay` e `worker`.
- A **API não depende do RabbitMQ** para subir: ela só grava no Mongo. Se o broker cair,
  o outbox acumula e o site continua funcionando.
- `RABBITMQ_URL`, `RESEND_API_KEY`, `EMAIL_FROM` em `deploy/.env.example`. Cuidado com
  `env.RequiredKeys()` e `deploy/compose_test.go`: não tornar RabbitMQ obrigatório para a
  API; validar no worker/relay.
- **Management UI via `ops.` (O6)**: router no Traefik com
  `Host(OPS_DOMAIN) && PathPrefix(/rabbitmq)`, middlewares `oauth` (oauth2-proxy/GitHub) e
  `security-headers`, apontando para `rabbitmq:15672`, com
  `management.path_prefix=/rabbitmq` no broker. A UI mostra payloads de OTP, então:
  - usuário dedicado com tag `management`, separado do usuário da aplicação;
  - usuário da aplicação sem tag `management` e com permissões só no vhost `legnaris`;
  - nenhuma porta 15672 publicada no host;
  - acesso restrito ao mantenedor pelo GitHub OAuth já existente.
- Prometheus: habilitar `rabbitmq_prometheus` (porta 15692, rede interna) e scrape no
  `prometheus.yml`.
- Recursos: `vm_memory_high_watermark` e disk free limit. Nó único é SPOF; com outbox os
  emails não se perdem, mas atrasam.
- RabbitMQ 4.x: classic mirroring removido; quorum é o default recomendado para
  durabilidade.

### 5.9 Observabilidade

Métricas de aplicação:
- `email_outbox_backlog`, `email_publish_total{result}`, `email_send_total{result}`,
  `email_send_duration_seconds`, `email_dead_total`.
- `auth_otp_issued_total`, `auth_otp_verify_total{result}`, `auth_otp_expired_total`.

Métricas do broker:
- `rabbitmq_queue_messages_ready`, `messages_unacked`, `consumer_count`,
  `message_ack_total`, `publish_total`.

Alertas:
- DLQ > 0; consumer count = 0; mensagem mais antiga na fila > 2 min; outbox backlog > 100;
  falha de publish por 5 min.
- Cota do Resend: monitorar 429 e o teto diário.

Logs: `outbox_id`/`message_id`, propósito e resultado; nunca código, nunca email completo
(mascarar/hashear).

### 5.10 Testes

- `go test ./...` não pode exigir RabbitMQ: `Publisher`, `Consumer` e `EmailSender` atrás
  de interfaces; fakes nos testes. O CI de race roda sem serviços.
- Unidade: relay com publisher que nacka/derruba; worker com 200/429/400/timeout; código
  expirado; duplicata; dois relays com lease; dois consumers.
- Integração opcional com `RABBITMQ_URL`, self-skip como o Mongo.
- Golden file do JSON da mensagem; template renderizado (texto e HTML).
- Asserção de que o código não aparece em log nem em resposta HTTP.

### 5.11 Abordagens ponta a ponta consideradas

| #   | Publicação                    | Retry                  | Worker          | Prós                                              | Contras                                        | Veredito       |
| --- | ----------------------------- | ---------------------- | --------------- | ------------------------------------------------- | ---------------------------------------------- | -------------- |
| A   | API publica direto            | broker                 | separado        | Menos peças, menor latência                       | Dual-write; perde email; email órfão           | Não recomendo  |
| B   | Outbox + relay                | TTL/DLX (O1)           | separado        | Durável, auditável, replay, API não depende do broker | Mais serviços/coleções                     | **Recomendado**|
| C   | Outbox + relay                | app grava novo outbox  | separado        | Backoff exato; DLQ simples                        | Worker escreve no Mongo; mais acoplamento      | Alternativa    |
| D   | Outbox only, sem broker       | app                    | mesmo processo  | Mínimo de infra                                   | Sem desacoplamento; sem broker para futuros canais | Baseline  |
| E   | Transação Mongo + publish     | broker                 | separado        | Consistência DB                                   | Não resolve o publish; exige replica set; ainda precisa de outbox | Não resolve sozinho |

Decisões e variantes dentro de B:
- **Relay separado (`cmd/relay`) — decidido (O2).** A goroutine na API foi descartada: o
  relay precisa de lease e shutdown próprios e não deve competir com o processo HTTP.
- **Polling — decidido.** Change streams exigem replica set; polling funciona no
  standalone atual.
- **Binários separados — decidido (O2).** `ROLE` no mesmo binário foi descartado para
  isolar o ciclo de vida de API, relay e worker.
- **Mongo replica set de nó único**: opção futura, fora do MVP. Habilitaria transações e
  change streams, permitindo escrever user + challenge + outbox atomicamente e reduzir a
  latência do relay. Custo: mudar `make dev-db`, compose, connection string
  (`?replicaSet=rs0`) e init.
- **Uma fila vs filas por propósito**: separar `email.otp` agora e `email.notification`
  depois; não usar prioridade.

---

## 6. Impacto no contrato e no cliente

| Área                        | Impacto                                                                 |
| --------------------------- | ----------------------------------------------------------------------- |
| `AUTH-02` / `WAS-02`        | Cadastro deixa de autenticar; spec 006 e 007 precisam refletir o novo fluxo |
| `internal/auth/service.go`  | `Register` e `Login` separam autenticação de emissão de sessão          |
| `internal/auth/service.go`  | `Refresh` passa a carregar o usuário e checar `email_verified_at`       |
| `internal/user`             | `email_verified_at`, `MarkEmailVerified`, DTO e mocks                   |
| `internal/app`              | Novas rotas, novos códigos de erro, rate limit por challenge            |
| `shared/apperr`             | Novos códigos + status + teste de mapeamento                            |
| `shared/env`                | `EMAIL_*`, `RABBITMQ_URL`, TTLs, limites, `EMAIL_CODE_SECRET`           |
| `shared/db/indexes.go`      | Índices de `auth_challenges` e `email_outbox`                           |
| `web/src/api`               | `AuthApi`, `AuthSession`, `error_mapper`, `l10n`                        |
| `deploy/`                   | RabbitMQ, worker, relay, env, compose test, Prometheus                  |
| Specs/CI                    | IDs novos precisam existir em `.specs/**/spec.md`; ver `scripts/check_requirement_ids.py` |

---

## 7. Achados pré-existentes (bloqueadores e riscos)

Ordenados por severidade. Todos foram verificados no código; nenhum foi corrigido nesta
entrega.

### F-01 — Rate limit por IP é global atrás do Traefik (P0)

`internal/app/router.go:19` cria o Echo com `echo.New()` e nunca define `IPExtractor`.
No Echo v5, sem `IPExtractor`, `c.RealIP()` retorna `RemoteAddr`
(`echo/context.go`). O app não publica porta; só o Traefik o alcança, então
`RemoteAddr` é o IP do container do Traefik — **igual para todos os clientes**.

Consequência: `AUTH_RATE_LIMIT_ATTEMPTS=10/min` é um teto global. Um atacante com 10
requisições por minuto bloqueia login e cadastro de todo mundo. Com 2FA, o volume de
chamadas cresce e o problema piora. Os testes não pegam porque injetam `RemoteAddr`
diferente por request, sem proxy.

Correção: configurar `router.IPExtractor = echo.ExtractIPFromXFFHeader(...)` com trust
ranges do Traefik (ou `X-Real-IP`, se configurado), e testar com XFF forjado.

### F-02 — `Register` emite sessão (P0)

`internal/auth/service.go:180` chama `signIn` após criar o usuário. É contrato explícito
(`AUTH-02`). A verificação de email exige quebrar isso em backend, front e specs ao mesmo
tempo. Não é ajuste local.

### F-03 — `Refresh` não consulta o usuário (P0)

`internal/auth/service.go:216` emite access token a partir do refresh token persistido
(linha 240), sem carregar o usuário. “Só entra quem verificou” não vale para sessões já
emitidas: elas renovam por até 30 dias. **Decisão O5:** `Refresh` carrega o usuário e
recusa quem não tem `email_verified_at`; sem isso, “verificar no login” não vale para
sessões pré-deploy. A limpeza das famílias de refresh no deploy é opcional — a checagem
no `Refresh` é a garantia.

### F-04 — Email validado e armazenado cru (P1)

`internal/auth/service.go:134` chama `mail.ParseAddress(email)` e descarta o resultado.
O valor original é persistido. Isso aceita `"Nome <a@b.com>"`, cria duas contas para a
mesma caixa e produz um `To` inválido no envio. Também não há limite de tamanho de email.
Canonicalizar para `parsed.Address` e tratar colisões antes de migrar.

### F-05 — Enumeração e oráculo de timing (P1)

Cadastro responde `409` para email existente. Login responde `401` genérico, mas **pula o
bcrypt** quando o email não existe, então o tempo denuncia a existência. Com 2FA, os
endpoints de challenge precisam ser uniformes em status, corpo e tempo.

### F-06 — Não há update de usuário (P1)

`internal/user/repository.go:13-17` só tem `Create`, `FetchByEmail` e `FetchByID`. Falta
`MarkEmailVerified`/update. Isso muda a interface, os mocks e os testes que usam
`NewMockRepository`.

### F-07 — Rate limiter em memória e por processo (P1)

`internal/platform/ratelimit/limiter.go` é um map com mutex, reiniciado no deploy e não
compartilhado entre réplicas. Não serve como contador de tentativas de código; esse
contador precisa estar no documento do challenge.

### F-08 — `time.Now()` fixo no auth service (P2)

TTL, cooldown e expiração viram testes frágeis. O `ratelimit.Limiter` já injeta relógio;
o auth service não. Injetar `Clock` antes de escrever os testes de OTP.

### F-09 — Segredo reaproveitado (P2)

`internal/app/bootstrap.go:66` usa `JWT_SECRET` como chave HMAC do CSRF. Não repetir no
OTP: usar `EMAIL_CODE_SECRET` separado.

### F-10 — Sem `me` e sem `email_verified` no DTO (P2)

O front não restaura o estado de verificação após reload; só a resposta do login carrega
isso. Adicionar o campo e `GET /auth/me`.

### F-11 — Códigos novos caem em `unknown` no front (P2)

`web/src/api/error_mapper.ts` mapeia apenas `INVALID_JSON`, `INVALID_REQUEST`,
`UNKNOWN_FIELD`, `NOT_FOUND`, `INTERNAL_ERROR`, `UNAUTHORIZED`, `FORBIDDEN` e
`RATE_LIMITED`. Novos códigos precisam de kind e cópia em `l10n/strings.ts`.

### F-12 — Mongo standalone, sem transações (P2)

`deploy/docker-compose.yml:149-150` usa `mongo:7.0` standalone. Atomicidade só por
documento; sem transações e sem change streams. O desenho do challenge e do outbox
precisa respeitar isso.

### F-13 — TTL não é expiração (P2)

O monitor de TTL do Mongo remove depois de ~60s. Nunca confiar nele para negar um código;
verificar `expires_at` na query.

### F-14 — Sem migração e sem graceful shutdown (P2)

`EnsureIndexes` é idempotente, mas não versiona dados. `cmd/api.go:66` sobe com
`router.Start`, sem drain. Um worker/relay precisa de ciclo de vida e shutdown.

### F-15 — Cookie `__Host-` + `SameSite=Strict` (P3)

Link de verificação clicado no email é navegação cross-site; o refresh cookie não vai no
primeiro hit. Normalmente o SPA carrega e faz refresh depois, mas fluxos que esperem a
sessão já no clique quebram. Mais um motivo para preferir código a magic link.

### F-16 — Nenhuma infraestrutura de email/fila (P3)

Não há dependência, template, worker, broker, bounce handling nem variáveis de ambiente.
Tudo isso é novo.

### F-17 — Acoplamento com specs e CI (P3)

`scripts/check_requirement_ids.py` exige que IDs citados em código existam em
`.specs/**/spec.md`. `deploy/compose_test.go` valida `RequiredKeys()` contra o compose.
`AGENTS.md` exige testes de edge case + happy path no mesmo commit, com race.

### F-18 — Raio de explosão nos testes (P3)

`internal/app/auth_test.go`, `auth_integration_test.go`,
`web/src/test/auth_api.test.ts`, `auth_session.test.ts`, `auth_screen.test.tsx`,
`auth_gate.test.tsx` e os contratos de 006/007 assumem “register → sessão”. A mudança é
grande e deve acompanhar a spec.

---

## 8. Riscos e preocupações

### Segurança e abuso

- 6 dígitos = ~20 bits. Só é seguro com limite de tentativas por challenge (5), cooldown
  de reenvio e limite por email + IP. Sem contador persistente, 1M de combinações é
  viável.
- Reenvio é vetor de email bombing: limite por email, por IP, por janela e teto global
  diário. Captcha só se o abuso aparecer.
- Não revelar existência de conta em `resend`/login. Responder `202` uniforme.
- Evitar lockout de conta: bloqueio duro permite DoS contra a vítima. Preferir throttle
  por IP + email.
- Normalização: lowercase + trim; **não** remover pontos nem `+tag` (quebra outros
  provedores).

### Produto e recuperação

- 2FA por email é o único segundo fator: quem perde o email perde a conta. Backup codes
  ou TOTP são o próximo passo natural.
- Usuário antigo com email digitado errado fica travado no primeiro login 2FA. Precisa de
  troca de email/suporte.
- Cadastro com email de outra pessoa: o atacante não verifica, mas bloqueia o dono no
  `register` (409). Sem reset de senha, o dono não tem caminho. Reset de senha deve usar
  a mesma infraestrutura e provavelmente entra no mesmo ciclo.
- Todo login depende do Resend. Sessões de 30 dias reduzem a frequência; “lembrar este
  dispositivo” é uma opção futura.
- Sem “esqueci a senha”, a feature cria um beco sem saída para contas não verificadas.

### Entrega

- SPF/DKIM/DMARC, subdomínio dedicado, bounce/complaint, texto alternativo, `From`
  estável.
- Provedor fora do ar não pode impedir cadastro; o outbox segura e o usuário tem resend.
- Cota diária do free tier pode ser esgotada por abuso; monitorar e colocar teto global.
- Domínio novo tem desconfiança inicial; começar com volume baixo.

### Operação

- RabbitMQ em nó único é SPOF. Com outbox os emails não se perdem, mas atrasam.
- Management UI mostra payloads de OTP. Exposta somente em `ops.` atrás do oauth2-proxy
  (O6), com usuário dedicado e sem porta pública; o acesso é sensível e fica restrito ao
  mantenedor.
- Mensagens com código são credenciais de curta duração: TTL curto, vhost restrito.
- Canais do `amqp091-go` não são concorrentes; um canal por papel/goroutine.

### Legal (LGPD)

- Email e IP são dados pessoais. Definir retenção dos challenges (apagar após
  consumo/expiração) e não logar email completo nem código.
- Escolher região do provedor e ter DPA; para usuários BR, avaliar SES `sa-east-1` ou
  região EU.
- Atualizar política de privacidade e registro de tratamento.

### Qualidade

- Testes: fake sender, relógio fixo, integração com Mongo, concorrência no consumo,
  replay, expiração, cooldown, provedor falhou → nenhuma sessão emitida.
- Emails em inglês? `AGENTS.md` exige inglês em strings de usuário; se o produto quiser
  pt-BR nos emails, isso é uma decisão que contraria a regra atual.

---

## 9. Estratégia de rollout sugerida

1. **Pré-requisitos independentes**: corrigir `IPExtractor`, canonicalização de email,
   clock injetado, `user.Repository.Update`, `Refresh` com checagem de usuário.
2. **Infraestrutura de email**: `EmailSender`, adapter Resend, templates, config, fake,
   outbox e relay.
3. **Challenges**: collection, índices, consumo atômico, endpoints e códigos de erro.
4. **Endurecimento do auth**: `Register`/`Login` sem sessão, verificação, 2FA, `me`.
5. **Front-end**: estados, tela de código, l10n, error mapper.
6. **Deploy**: RabbitMQ, `cmd/relay` e `cmd/worker` como processos separados (O2),
   management UI em `ops.` com oauth (O6), env e Prometheus.
7. **Ativação**: sem flag, o OTP no cadastro é sempre ativo (LGN-138). Contas e sessões
   existentes não sofrem backfill; a verificação ocorre no próximo login (D2).
8. **Observar**: DLQ, backlog, 429 do Resend, falhas de verificação.

A flag permite subir a infraestrutura desligada e evita que um deploy quebre todos os
usuários de uma vez.

---

## 10. Fora de escopo

- Reset de senha e troca de email (recomendados como próximo ciclo, pela recuperação).
- TOTP, backup codes, “lembrar este dispositivo”.
- Emails de marketing/notificação.
- Admin panel, suporte a múltiplos provedores simultâneos.
- Migração para replica set do Mongo (opção futura; fora do MVP).
- Spec, requisitos numerados, tarefas e implementação.

---

## 11. Próximo passo

As decisões de produto (D1–D6) e operacionais (O1–O6) estão fechadas. O próximo artefato
é a **spec** (Spec Kit), que deve detalhar:

- contrato HTTP definitivo com os endpoints unificados de challenge, payloads e códigos de
  erro;
- schema e índices de `auth_challenges`/`email_outbox`, incluindo consumo atômico e cota;
- topologia RabbitMQ com TTL + DLX (O1) e ciclo de vida de `cmd/relay`/`cmd/worker` (O2);
- rollout com a flag, checagem no `Refresh` (O5) e limpeza opcional de sessões;
- configuração do management UI em `ops.` (O6) e das guardas do free tier do Resend (O4);
- matriz de testes, incluindo falhas de broker e provedor.

Nenhum código deve ser escrito antes da spec.

---

## 12. Referências

- `docs/TDD.md` — arquitetura e decisões do backend.
- `.specs/features/006-backend-browser-auth-cookies/spec.md` — contrato de auth atual
  (`AUTH-01`…`AUTH-26`).
- `.specs/features/007-web-auth-session/spec.md` — contrato do cliente
  (`WAS-01`…`WAS-32`).
- `internal/auth/service.go`, `internal/auth/token.go`, `internal/auth/cookie.go`,
  `internal/auth/refresh_repository.go`.
- `internal/app/router.go`, `internal/app/ratelimit.go`, `internal/app/bootstrap.go`.
- `internal/platform/ratelimit/limiter.go`.
- `internal/user/user.go`, `internal/user/repository.go`.
- `shared/env/env.go`, `shared/db/indexes.go`, `shared/apperr/`.
- `web/src/api/auth_api.ts`, `web/src/api/error_mapper.ts`,
  `web/src/auth/auth_session.ts`, `web/src/auth/auth_gate.tsx`.
- `deploy/docker-compose.yml`, `deploy/traefik/dynamic.yml`, `deploy/compose_test.go`.
- `cmd/api.go`, `Dockerfile`, `Makefile`.
- `AGENTS.md` (convenções), `scripts/check_requirement_ids.py` (lint de IDs).
