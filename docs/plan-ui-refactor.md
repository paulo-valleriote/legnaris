# Plano — Guidelines de UI + Refactor da `web/`

- **Status:** plano aprovado. F0 (docs) e F1 (fundação) **entregues**; F2–F4 pendentes.
  - F1 = `.specs/features/008-web-ui-foundation` (prefixo `UIF`), implementada e commitada.
  - `web/DESIGN.md` e o `web/AGENTS.md` enxugado já existem (F0).
- **Escopo:** estabelecer guidelines de desenvolvimento/estilização para a UI de `web/` e um
  roadmap de refactor do design system.
- **Nota de idioma:** este arquivo é um plano de trabalho local (pt-BR). Os artefatos que forem
  **commitados** no repo (`web/DESIGN.md`, `web/AGENTS.md`) devem ser escritos em **inglês**,
  conforme a convenção do repositório. Nomes de token, specs de componente e regras técnicas já
  estão em inglês abaixo para reuso direto.

---

## 1. Estado atual (diagnóstico que motiva o refactor)

Arquivos-chave: `web/src/theme/tokens.css`, `web/src/theme/tokens.ts`, `web/src/shell/right_panel.tsx`,
`web/src/catalog/catalog.css`, `web/src/shell/shell.css`.

- **Paleta "dull":** 1 único accent (`#1a4fa3`), superfícies `#ffffff`/`#f4f4f6`, borda `#d8d8de`.
  Sem escala de tint/shade, sem hierarquia de superfície, sem elevação/sombra, sem estados
  intermediários, sem modo escuro.
- **Tipografia:** sistema do SO, escala `12/13/15/17` com teto rígido de 17px — sem hierarquia real.
- **Forma:** raio máx 4px; proibidos gradiente/glass/sombra/heading grande/pill/ilustração (tabela
  "Avoid" em `web/AGENTS.md`, portada do `desktop/AGENTS.md`).
- **"Aba de contexto" morta:** `right_panel.tsx` é um `<aside aria-label="Context">` fixo com
  placeholder *"Details for the open workspace will appear here."*; `app_shell.tsx` (FND-01)
  **sempre** renderiza 3 colunas `nav | content | panel` e nenhuma tela preenche o painel → UI morta
  em 100% das rotas.
- **Botões:** um `.lg-button` genérico (+`--primary`/`--danger`) em `catalog.css`; sem variantes
  ghost/outline, sem tamanhos, sem ícones.
- **Sem biblioteca de componentes:** CSS por feature (`shell.css`, `catalog.css`, `sheet.css`,
  `engine.css`, `auth.css`, `your_sheet.css`, `update_prompt.css`).

### Gates que qualquer mudança precisa respeitar (aliados do refactor)

- `web/src/test/tokens_used.test.tsx` — proíbe hex literal (`#...`) e `\d+px` em
  `shell/ catalog/ engine/ sheet/ library/ auth/`; todo `var(--lg-*)` precisa existir em
  `tokens.css`. **`theme/` não é escaneado → `tokens.css` pode conter os valores literais.**
- `web/src/test/tokens_contrast.test.ts` + `web/src/theme/tokens.ts` — `contrastPairs` afirmam AA a
  partir dos valores do CSS; todo `designTokens` precisa resolver.
- `web/src/test/l10n_strings.test.ts` — nenhum literal de texto no código; só `message('key')`
  com key conhecida; keys declaradas precisam ser referenciadas.

**Consequência:** como a checagem é "tokens só", todo o trabalho de beleza se concentra em
`tokens.css`/`tokens.ts` e os componentes permanecem limpos.

---

## 2. Decisões travadas (com o usuário)

| Tema | Decisão |
|---|---|
| Personalidade | Warm editorial (pergaminho quente + profundidade sutil); **sem serifa** |
| Ousadia | **Redesign completo** do design system (tokens do zero, revisão de telas/testes) |
| Modo de cor | **Claro + escuro + toggle** (persistido) |
| Painel direito | **Remover do shell** (nav + content) |
| Tipografia | **1 fonte variável self-hosted** |
| Fonte | **Geist** (variable sans, OFL-1.1) |
| Textura | **Só cor, sem textura** de papel |
| Dark | **Charcoal neutro + âmbar** |
| Dependências | **Tokens + CSS puro, zero dependências** |
| Entrega | **Guidelines + roadmap faseado + Fase 1 detalhada** |

---

## 3. A) Guidelines — conteúdo a materializar em `web/DESIGN.md` (inglês)

> `web/DESIGN.md` é a fonte da verdade visual. `web/AGENTS.md` mantém só regras de dev/segurança e
> aponta para cá. Toda cor/espaço/raio/sombra/movimento vem de `src/theme/tokens.css` — nunca
> literal no componente.

### 3.1 Princípios
- **Usability first; hierarchy guides the eye.** Tamanho, peso, cor e elevação dizem "leia isto
  primeiro".
- **Comfortable density.** Agrupa e escaneia; não "espalha".
- **Accessible by default.** AA nos dois temas; foco sempre visível; teclado primeiro.
- **Token-only, zero dependencies.** Nada de lib de UI; design system = tokens + CSS + convenção.
- **Restraint.** 1 accent, profundidade sutil, sem excesso decorativo.

### 3.2 Arquitetura de tokens (3 camadas)
1. **Primitives** — escalas cruas: `--lg-ink-*`, `--lg-amber-*`, `--lg-danger-*`, `--lg-space-*`,
   `--lg-radius-*`, `--lg-font-*`, `--lg-motion-*`.
2. **Semantics** — o que os componentes usam: `--lg-color-bg|surface|raised|inset|overlay`,
   `--lg-color-text|text-muted|text-subtle`, `--lg-color-border|border-strong`,
   `--lg-color-accent|accent-hover|accent-text`, `--lg-color-danger*`, `--lg-shadow-1..3`,
   `--lg-focus-*`.
3. **Component** — só quando um semantic não cobre (ex.: `--lg-color-nav-bg`).

**Fluxo obrigatório ao adicionar token:** (1) declarar em `tokens.css` nos **dois temas**;
(2) espelhar em `tokens.ts` (`designTokens` + `contrastPairs` quando for par de contraste);
(3) rodar `npm test` (o teste de contraste valida os dois temas). Naming: `--lg-kebab-case`.

### 3.3 Cor
- **Light — parchment:** `bg #EFE6D4` / `surface #FBF6EA` / `inset #E4D8BF`; tinta `text #2A2118`,
  `text-muted #6B5B47`; `border #D6C9AF` / `border-strong #B9A886`; accent âmbar queimado
  `#B45309` (hover `#92400E`), `accent-text #FFFFFF`; danger tijolo `#A83232`.
- **Dark — charcoal + amber:** `bg #17181B` / `surface #212327` / `inset #141518`; `text #ECE9E4`,
  `text-muted #A8A29B`; `border #33363B` / `border-strong #4A4E54`; accent `#F59E0B` com
  `accent-text #201A10`; danger `#F87171`.
- **Regras:** accent só em ação/estado ativo/seleção — nunca decoração. `accent` como *fill* sempre
  pareado com `accent-text`; texto/ícone âmbar sobre papel usa tom mais escuro. Estados de sistema:
  info/success/warning/danger.
- **Os valores acima são ponto de partida** — quem manda é `tokens_contrast.test.ts`
  (AA ≥ 4.5; foco ≥ 3) nos **dois** temas.

### 3.4 Tipografia
- 1 família: **Geist** (variable, OFL) self-hosted em `public/fonts/`, `--lg-font-family` com
  fallback de sistema.
- Escala ampliada (mata o teto de 17px): `12 / 13 / 15 / 16 / 18 / 20 / 24 / 30`.
- Pesos 400 / 500 / 600 / 700 (eixo `wght` 100–900). `line-height` tight 1.2 / normal 1.5.
- **`font-variant-numeric: tabular-nums`** em toda coluna numérica/derivada (Geist Sans suporta
  `tnum`) — sem necessidade de Geist Mono.
- Carregamento: `@font-face` self-hosted, `font-display: swap`, `<link rel="preload">`; nunca
  fonte de terceiros (CSP same-origin).

### 3.5 Espaçamento, layout, densidade
- Escala de espaço: `2 / 4 / 8 / 12 / 16 / 24 / 32 / 48`.
- Conteúdo com medida legível (mantém `--lg-size-content-max`).
- **Shell = `nav | content`** (painel removido). Nav persistente; foco no conteúdo.

### 3.6 Raio, borda, elevação
- Raio: `sm 4 / md 8 / lg 12 / pill 999` (fim do teto de 4px).
- Borda: hairline `1px` + `border` / `border-strong`.
- Elevação em camadas: `shadow-1` (linhas/cards), `shadow-2` (popover), `shadow-3` (modal). Em dark,
  sombras mais profundas + borda para separar.
- Modelo de camadas: `bg → surface → raised → overlay`; nunca mais de 2 níveis visíveis.

### 3.7 Movimento
- Durações `--lg-motion-100/180/240` (`120ms/180ms/240ms`), easing `standard` / `emphasized`.
- Transicionar só `background, color, border-color, box-shadow, transform, opacity`.
- **`@media (prefers-reduced-motion: reduce)`** zera animações não essenciais.

### 3.8 Componentes (specs)
- **Button** — variantes `primary / secondary / ghost / outline / danger`; tamanhos `sm / md`;
  estados `hover / active / focus-visible / disabled / loading`; altura mín. 36–40px; radius `sm/md`.
- **Fields** — input/select/textarea/checkbox/radio/switch; label permanente, erro inline,
  `invalid`, `disabled`.
- **Data** — linhas/tabela com zebra sutil opcional, `tabular-nums`, header sticky.
- **Nav** — item com hover/active/current; tema + sessão no rodapé.
- **Dialog/overlay** — `shadow-3`, `role=dialog`, `aria-modal`, Esc/focus-trap (mantém contrato).
- **Feedback** — inline para resultado de ação; `toast` só para eventos não bloqueantes; estados
  vazio/loading/erro com 1 linha + 1 ação.
- **CSS convention:** `lg-<block>__<element>--<modifier>`; CSS por feature, primitivas em `src/ui/`.

### 3.9 Acessibilidade
- AA nos dois temas; `:focus-visible` tokenizado; tab order = ordem visual; alvos ≥ 24px; axe (e2e)
  sem violação crítica; nada depende só de cor.

### 3.10 Segurança (mantém)
- Sem sinks DOM (`dangerouslySetInnerHTML` etc.), ESLint bloqueia. Fontes self-hosted/same-origin.
- **Preferência de tema em `localStorage` é permitida** (não é credencial); token/credencial
  continua em memória/cookie.

### 3.11 Padrões por tela
- Catálogo: grid de cards escaneável (nome + metadados), ação primária clara.
- Ficha: cabeçalho + formulário do engine + footer sticky (Salvar/Preview).
- Auth: painel centrado com foco na ação única.
- Your-sheet / not-found / update-prompt: estado com 1 linha + ação.

### 3.12 Enforcement & workflow
- Manter `tokens_used` (sem hex/px), `tokens_contrast` (agora 2 temas), `l10n_strings`.
- Estender `tokens_used` para `src/ui/` (e tratar `theme/` como exceção consciente).
- DoD de mudança de UI: tokens nos 2 temas, contraste AA verde, foco visível, reduced-motion,
  `make lint-web && make test-web`.
- Checklist de PR: variante/estado coberto? token-only? contraste? teclado? axe?

### 3.13 Anti-padrões (lista revisada)
- Gradiente/chrome decorativo → profundidade por sombra/camada.
- Cor literal ou `px` em componente → token.
- Segundo accent → 1 só.
- Sombra pesada/glow → `shadow-1..3`.
- Texto ≤ 13px em dado crítico → base 15.
- Animação > 240ms ou ignorando reduced-motion.
- Região/painel sem conteúdo real (a lição do "Context").

---

## 4. B) Roadmap faseado

| Fase | Entrega | Toca |
|---|---|---|
| **F0** | `web/DESIGN.md` + `web/AGENTS.md` enxugado + decisão em log | docs |
| **F1** | Fundação: tokens 3 camadas (2 temas), toggle, remover painel, Geist self-host | `theme/`, `shell/`, `index.html`, `base.css`, testes de tema/shell |
| **F2** | Primitivas `src/ui/` (button, field, checkbox, dialog, toast) + testes | novo `src/ui/`, scans |
| **F3** | Refactor das telas: catálogo → ficha → auth → your-sheet → not-found/update | `catalog/`, `sheet/`, `auth/`, `shell/` |
| **F4** | Movimento + polish + regressão visual Playwright (T001/F10-T001) | e2e/screenshots |

Cada fase: tokens nos 2 temas → contraste AA → testes no mesmo commit → gates verdes.

**Mapeamento para features do Spec Kit** (slug do diretório em `.specs/features/` e prefixo de
requirement-ID — um prefixo único por feature):

| Fase | Feature | Prefixo | Entrega |
|---|---|---|---|
| F0 | — (docs) | — | `web/DESIGN.md` + `web/AGENTS.md` enxugado |
| F1 | `008-web-ui-foundation` | `UIF` | tokens 2 temas, toggle, shell de 2 regiões, Geist |
| F2 | `009-web-ui-primitives` | `UIP` | `src/ui/` (button, field, checkbox, dialog, toast) + scans |
| F3 | `010-web-screens-refactor` | `UIS` | catálogo, ficha, auth, your-sheet, not-found/update |
| F4 | `011-web-motion-visual-regression` | `UIM` | movimento + polish + regressão visual Playwright |

---

## 5. C) Fase 1 detalhada (fundação)

### Entregáveis
1. `src/theme/tokens.css` reescrito: camadas primitives+semantics; blocos `:root` (light) e
   `[data-theme='dark']`; `color-scheme`.
2. `src/theme/tokens.ts`: `designTokens` expandido; `contrastPairs` **por tema**.
3. `src/test/tokens_files.ts`: parser passa a ler **por bloco de tema** (hoje o regex pega 1 valor e
   colide entre temas).
4. `src/test/tokens_contrast.test.ts`: itera os dois temas.
5. Tema: `src/theme/theme.ts` (resolve `matchMedia` + `localStorage['lg.theme']` + `set`),
   `use_theme.ts`, `ThemeToggle` no rodapé da nav.
   - **Sem FOUC:** pequeno script inline no `<head>` de `index.html` (documentar hash de CSP) **ou**
     aplicar em `main.tsx` aceitando 1 frame — decisão a registrar no log de decisões.
6. **Remover painel:** `app_shell.tsx` (sem `RightPanel`/`narrow`), apagar `right_panel.tsx`,
   `shell.css` → grid de 2 colunas e remover `.lg-panel*`; remover `SHELL_NARROW_QUERY` e
   `--lg-size-panel-*` (CSS + `tokens.ts`).
7. **Fonte Geist:**
   - `public/fonts/geist/Geist[wght].woff2` (variable) + `Geist-LICENSE.txt` (OFL) commitados.
   - Origem do artefato fixada na implementação (release oficial do Geist ou Fontsource); nunca
     `@import` de terceiros.
   - `@font-face { font-family: 'Geist'; src: url('/fonts/geist/Geist[wght].woff2') format('woff2-variations'); font-weight: 100 900; font-display: swap; }`
     em `src/theme/fonts.css`, importado pelo `base.css`.
   - `<link rel="preload" as="font" type="font/woff2" crossorigin>` no `index.html`.
   - Atualizar `--lg-font-family`. Sem Geist Mono por ora.
8. `index.html`: `theme-color` condicionado a `prefers-color-scheme`; `lang="en"` já ok.
9. `base.css`: `color-scheme`, cor base via tokens, focus ring tokenizado.

### Testes a reescrever / adicionar
- `shell.test.tsx` e `shell_responsive.test.tsx`: hoje afirmam 3 regiões (`complementary`) — passam
  a afirmar `nav | content`; nova responsividade = conteúdo com min-width e nav persistente.
- Novos: `theme_toggle.test.tsx` (persistência + default por `prefers-color-scheme`);
  `tokens_contrast` cobrindo dark.
- Revisar e2e que contêm regiões: `auth.spec.ts` (axe sweep), demais specs.
- `tokens_used.test.tsx`: manter; ao criar `src/ui/`, adicionar ao scan.

### Definition of Done
- Ambos os temas passam AA; toggle persiste e não pisca; painel fora do DOM em todas as rotas;
  Geist carregando self-hosted; `make lint-web && make test-web` e e2e verdes;
  `web/AGENTS.md` / `web/DESIGN.md` atualizados.

---

## 6. Riscos e pontos abertos

- **Parser de tokens por tema:** `tokens_files.ts` hoje achata tudo num único mapa; precisa separar
  light/dark para o teste de contraste.
- **FOUC vs CSP:** script inline evita flash mas exige hash de CSP; alternativa aceita 1 frame.
- **Largura sem o painel:** revisar `--lg-size-content-max` / layout de 2 colunas.
- **Geist variable no `<link preload>`:** conferir `as="font"` + `crossorigin` e a sintaxe
  `format('woff2-variations')` no build.
- **Escopo F3:** remover a regra antiga de "não criar camada base" (`src/ui/`) exige ajuste no
  `web/AGENTS.md` (a regra `≥ 3 usos` continua válida para o resto).

## 7. Próximo passo
Trocar de agente para: (F0) escrever `web/DESIGN.md` + enxugar `web/AGENTS.md`; (F1) iniciar a
fundação (tokens 2 temas, toggle, remoção do painel, Geist).
