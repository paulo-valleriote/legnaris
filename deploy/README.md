# Legnaris — Stack de produção (Docker Compose + Traefik)

Stack autocontido para VPS única: aplicação Go (Echo v5) + MongoDB +
observabilidade (Prometheus, Grafana, Loki, Alloy) + interface de banco (Mongo
Express), com acesso administrativo protegido por **OAuth do GitHub**
(oauth2-proxy) e **Traefik** na borda.

---

## 1. Arquitetura e domínios

```
legnaris.com.br          → Vercel (frontend)          [intocado]
api.legnaris.com.br      → Docker/Traefik             [público, sem OAuth]
   └─ /                  → API do app (health, templates, sheets, /metrics)
ops.legnaris.com.br      → Docker/Traefik             [OAuth do GitHub]
   ├─ /oauth2/callback   → callback do GitHub OAuth
   ├─ /grafana           → Grafana
   ├─ /prometheus        → Prometheus
   └─ /mongo             → Mongo Express
```

Por que separar em dois subdomínios? O frontend (Vercel) e a API/observabilidade
ficam em hosts diferentes, cada um com sua regra de segurança. O `api.` é
público; o `ops.` exige login via GitHub.

---

## 2. Entendendo cada camada (sem jargão)

### DNS — o "catálogo de endereços"
O DNS traduz nomes (`api.legnaris.com.br`) em IPs. Dentro do seu domínio você
cria **registros** que apontam subdomínios para destinos:

| Registro | Significado |
|---|---|
| `A` | "este nome → este IP" (ex.: `api` → `203.0.113.10`). |
| `CNAME` | "este nome → outro nome" (ex.: `api` → `tunel.cfargotunnel.com`). |
| `@` ou vazio | o domínio raiz (`legnaris.com.br`). |

O apex `legnaris.com.br` já aponta para a Vercel. Você só **adiciona** dois
registros novos (`api` e `ops`) — não mexe no que já existe.

### Traefik — o "porteiro" (reverse proxy)
O Traefik fica na frente de tudo, escutando as portas 80/443. Ele **não** é o seu
servidor web — é quem decide, para cada requisição, qual serviço deve atendê-la,
baseado no **Host** (domínio pedido) e no **path** (caminho):

- Pediu `api.legnaris.com.br/...` → encaminha para o app.
- Pediu `ops.legnaris.com.br/grafana` → encaminha para o Grafana, mas antes
  consulta o oauth2-proxy ("essa pessoa está logada?").

Ele também faz o **TLS/HTTPS**: emite e renova certificados via Let's Encrypt.

### Roteamento — Host vs path
- **Por Host** (`api.` × `ops.`): domínios diferentes → serviços diferentes.
- **Por path** (`/grafana` × `/prometheus` no mesmo `ops.`): mesmo domínio,
  caminhos diferentes.

Aqui usamos os dois: **Host** para separar público/administrativo, e **path**
para dividir as ferramentas dentro do `ops.`.

### OAuth (oauth2-proxy) — o login
Quando alguém acessa uma rota do `ops.`, o Traefik pergunta ao oauth2-proxy se há
um cookie de sessão válido. Se não, redireciona para o GitHub; após o login, o
GitHub devolve para `ops.legnaris.com.br/oauth2/callback` e o oauth2-proxy
verifica se o usuário está na allowlist (`GITHUB_USER`) antes de liberar.

### Cloudflare Tunnel (só no ambiente local)
Para testar localmente sem abrir porta nem ter VPS, o `cloudflared` cria um
"túnel" que publica seu Docker local numa URL HTTPS pública. O Cloudflare
termina o TLS; o Traefik local fala HTTP simples atrás do túnel.

---

## 3. Pré-requisitos

1. **Domínio** `legnaris.com.br` (registrado no Registro.br, DNS gerenciado na Cloudflare) — já existe.
2. **OAuth App do GitHub** (ver §4).
3. **DNS dos subdomínios** `api.` e `ops.` (ver §5).
4. **Cookie secret** do oauth2-proxy (ver §6).

## 4. Criar o OAuth App no GitHub

1. GitHub → Settings → Developer settings → OAuth Apps → **New OAuth App**.
2. Preencha:
   - **Application name**: `Legnaris`.
   - **Homepage URL**: `https://api.legnaris.com.br`.
   - **Authorization callback URL**: `https://ops.legnaris.com.br/oauth2/callback` ⚠️ exato.
3. **Register application**.
4. Copie o **Client ID** e gere o **Client secret**.

> O callback exige HTTPS (GitHub só aceita `http://` para `localhost`).

## 5. Migrar o DNS para a Cloudflare

O DNS passa a ser gerenciado na **Cloudflare** (que também entrega analytics de
borda e integração nativa com o Cloudflare Tunnel). O domínio continua
**registrado** no Registro.br — só muda *onde* a zona é gerenciada.

### 5.1 Migrar a zona (uma vez)

1. Crie uma conta em **https://dash.cloudflare.com** e clique em **Add a site**,
   digitando `legnaris.com.br` (plano Free basta).
2. O Cloudflare **escaneia e importa** os registros atuais (o apex da Vercel).
   Confira se vieram corretos.
3. O Cloudflare mostra **dois nameservers** (ex.: `xxx.ns.cloudflare.com`,
   `yyy.ns.cloudflare.com`). Anote-os.
4. No **Registro.br**, troque o **NS** do domínio por esses dois nameservers:
   acesse https://registro.br → Meus domínios → `legnaris.com.br` →
   **Servidores DNS / Alterar NS**.
5. Aguarde a propagação (minutos a algumas horas). No Cloudflare, a zona muda
   para **Active**.

### 5.2 Criar os subdomínios (no Cloudflare)

**Produção (VPS) — tipo A, deixe "DNS only" (sem proxy):**
| Nome | Tipo | Valor | Proxy |
|---|---|---|---|
| `api` | A | IP da VPS | DNS only |
| `ops` | A | IP da VPS | DNS only |

**Local (Cloudflare Tunnel):** não crie à mão — use `cloudflared tunnel route dns`
(ver §8), que cria os CNAME automaticamente.

### 5.3 Notas

- **Apex da Vercel**: mantenha `legnaris.com.br` em **DNS only** (não ative o
  proxy/orange). Pôr o Cloudflare na frente da Vercel gera double-CDN e conflito
  de cache; a Vercel já tem analytics próprio.
- **Analytics de borda (opcional)**: se quiser analytics do Cloudflare para o
  backend, ative o proxy (orange) em `api.`/`ops.`. Nesse caso desative o cache
  em `api.` e configure o Traefik para confiar nos headers do Cloudflare
  (forwardedHeaders). O origin já usa Let's Encrypt, que funciona no modo
  "Full (strict)".

## 6. Configurar o `.env`

```bash
cd deploy
cp .env.example .env
# preencher: API_DOMAIN, OPS_DOMAIN, ACME_EMAIL, GITHUB_CLIENT_ID,
# GITHUB_CLIENT_SECRET, GITHUB_USER, CORS_ALLOWED_ORIGINS, MONGO_ROOT_PASSWORD,
# GRAFANA_ADMIN_USER/PASSWORD
openssl rand -base64 32 | tr '+/' '-_'   # usar como OAUTH2_PROXY_COOKIE_SECRET
```

## 7. Subir (produção)

```bash
cd deploy
docker compose up -d --build
docker compose ps        # mongodb "healthy" e demais "Up"
```

## 8. Subir (local, via Cloudflare Tunnel)

```bash
# 1. uma vez, no host:
cloudflared tunnel login
cloudflared tunnel create legnaris
cloudflared tunnel route dns legnaris api.legnaris.com.br   # cria o CNAME automaticamente
cloudflared tunnel route dns legnaris ops.legnaris.com.br

# 2. configurar o túnel:
cp cloudflared/config.yml.example cloudflared/config.yml
# edite e troque <TUNNEL_UUID> pelo valor de `cloudflared tunnel list`

# 3. subir com o override local:
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build
```

O que o override local muda (vs produção):
- Traefik usa `traefik.local.yml` (HTTP puro, **sem** Let's Encrypt — o Cloudflare
  termina o TLS).
- oauth2-proxy usa `--cookie-secure=false` (a origem é HTTP atrás do túnel).
- Adiciona o serviço `cloudflared` que publica `api.` e `ops.`.

**Alternativa rápida (URLs aleatórias, só para checar se sobe):**
```bash
docker run --rm --network legnaris-prod_edge cloudflare/cloudflared:latest \
  tunnel --no-autoupdate --url http://traefik:80
```

### Segurança extra para o túnel local: Cloudflare Access (Zero Trust)

O túnel torna `api.`/`ops.` alcançáveis da internet. Para testes locais, adicione
uma camada de autenticação **antes** do seu PC — assim nada chega na sua máquina
sem autenticar primeiro no Cloudflare:

1. Acesse **https://one.dash.cloudflare.com** (Zero Trust) → **Access** →
   **Applications** → **Add an application** → **Self-hosted**.
2. Em *Application domain*, informe `ops.legnaris.com.br` (e `api.legnaris.com.br`
   se quiser proteger a API também durante os testes).
3. Adicione uma **policy** *Allow* para o seu e-mail (login por PIN/OTP) ou via
   GitHub.
4. Salve. Agora o Cloudflare pede seu login **antes** de chegar ao oauth2-proxy —
   são duas barreiras em sequência.

> Isso protege o acesso *ao seu serviço local*. Não substitui o OAuth do `ops.`
> (que autoriza por allowlist) — é uma camada extra, específica do ambiente
> local. Em produção (VPS), o Access é opcional. Quando não estiver testando,
> desligue o túnel (`docker compose down`).

## 9. Acesso

| URL | O que é | Proteção |
|---|---|---|
| `https://api.legnaris.com.br/` | API pública | pública |
| `https://ops.legnaris.com.br/grafana` | Grafana | GitHub OAuth |
| `https://ops.legnaris.com.br/prometheus` | Prometheus UI | GitHub OAuth |
| `https://ops.legnaris.com.br/mongo` | Mongo Express | GitHub OAuth |

## 10. Observabilidade

- **P50/P95/P99 por rota**: Grafana → `legnaris_http_request_duration_seconds`
  (labels method/path/status).
- **Logs estruturados**: Grafana → Explore (Loki), `{service="app"} | json`.
  Correlação via `X-Request-Id`.
- **Métricas de borda/DB**: Prometheus raspa Traefik e MongoDB Exporter.
- **Consultar o banco**: Mongo Express em `/mongo`.

## 11. CORS (frontend → API)

O app permite origens de `CORS_ALLOWED_ORIGINS` (padrão: `https://legnaris.com.br`
no `.env`). Sem isso, o navegador bloqueia chamadas da Vercel para a API.

## 12. Operação do dia a dia

```bash
docker compose ps
docker compose logs -f app traefik oauth2-proxy
docker compose restart app
docker compose down          # para sem apagar dados
docker compose down -v       # para E apaga volumes (destrói dados)
```

Backup: volumes `legnaris-prod_mongo_data`, `..._prometheus_data`, `..._loki_data`.

## 13. Gotchas (aprendidos em teste)

1. **oauth2-proxy `cookie_secret`** exige base64 **URL-safe** (`tr '+/' '-_'`).
2. **Project name** do deploy é `legnaris-prod` (escopa volumes/rede desta
   stack, evitando colisão com outros projetos).
3. **Traefik `/metrics`** precisa de `metrics.prometheus.entryPoint: metrics`.
4. **Callback do OAuth** fica no `ops.` (não no `api.`), pois é onde o login
   acontece.
