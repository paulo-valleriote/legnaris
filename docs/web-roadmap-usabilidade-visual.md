# Roadmap de Melhoria — Usabilidade e Visual (@web/)

> **Status**: Fase 3 concluída (interação / responsividade). Itens implementados: token `--lg-size-control` ajustado para 44 px (touch safe area); engine (`sheet_form`) recebeu `collapsedSectionKeys` + `onToggleSection` para progressive disclosure; testes adicionados (`engine_form.test.tsx`). F4 (validação) pendente.  
> **Programa**: melhorar usabilidade e refinamento visual do PWA React/TS (`web/`) com base no que já está pronto (F1–F7, DESIGN.md, tokens `--lg-*`).  
> **Referências de qualidade**: skill `frontend-design` (aesthetic, typography, layout, motion, interaction, anti-AI-slop) e skill `web-design-guidelines` (a11y, focus, forms, animation, content, performance, navigation, dark mode, anti-patterns).  
> **Exceção de idioma**: este documento fica em pt-BR (exceção prevista para `docs/`).  
> **Fonte visual única**: `web/DESIGN.md` (supersede qualquer outra declaração; ver `web/AGENTS.md`).

---

## Parte A — Resumo

Objetivo: estabelecer um roadmap de melhoria para usabilidade e visual do app `@web/` a partir do estado entregado. Não é reconstrução do design system (já rigoroso: token-only, Geist vendored, dois temas parchment/charcoal+amber, primitives `src/ui/`, engine F4, auth F7, shell duas regiões). O foco é auditoria de acessibilidade/formulários, refinamento anti-AI-slop, interação/responsividade e validação por fase.

**Estado atual verificado:**
- Token architecture completo (`src/theme/tokens.css` espelhado em `tokens.ts`); contraste AA assertado para cada par; nenhum literal de cor/pixel nos diretórios de feature (`tokens_used.test.tsx`).
- Tipografia própria (Geist, `font-display: swap`, escala 12–30 px, tabular figures, line-height 1.2/1.5).
- Shell fixo (nav 200–240 px + content min 320 / max 960 px); sem hamburger; sem colapso por baixo do nav.
- Cinco primitivas (`button`/`field`/`checkbox`/`dialog`/`toast`) + focus trap; extração só com ≥3 usos (`UIP-01`/`UIP-03`).
- Engine F4 (`field_grouping`, `field_control`, `exclusive_control`, `sheet_form`) sem estado próprio; marcação modificados/recalculados via `rowMarkBuilder`; guarda numérica (`NaN`/`Infinity`).
- Interação centralizada (`shortcut_registry`, `leave_guard`, modal único para duas guardas, focus trap, `Esc`, destructive button real — não confirm-by-Enter).
- Segurança/design: sem `dangerouslySetInnerHTML`, sem terceiros, token memória-only, textos via `l10n/strings.ts`, identificadores em inglês.
- Anti-AI-slop estrutural: sem purple gradients, cyan-on-dark, hero metric, glassmorphism decorativo, cards-abril repetidos, centrado automático, rounded rectangles com borda espessa de um lado.

**O que ainda pode ser refinado:** fases de a11y/formulários, ritmo de espaço/fluidez, container queries, progressive disclosure, motion intencional, validação axe/Lighthouse por nova tela.

---

## Parte B — Fases autoritativas do roadmap

### Fase 1 — Usabilidade e acessibilidade (`web-design-guidelines`)

| Critério | Referência | Ação |
|---|---|---|
| Label permanente (nunca placeholder como label) | Forms | Revisar `field_control` / `exclusive_control`; confirmar `label` sempre renderizado. |
| Erro inline + validação visual clara | Forms / Content | Confirmar `--lg-color-danger` + `-text`; contraste no tema escuro. |
| Foco visível e keyboard-first | Accessibility / Focus | Auditar `focus_trap`, `focus-visible`, ordem de tab em `sheet_screen`, overlays de criação (F3) e guard modal (F5). Nenhum foco removido sem substituto. |
| `prefers-reduced-motion` | Animation | Revisar `motion-design`; clamping já existe, confirmar transições de `sheet_screen` / `engine`. |
| States vazios / erro / loading ensinam, não só dizem | Content / Empty states | `catalog_screen.tsx`, `not_found.tsx`, `auth` — ensinar ação (criar, voltar, tentar de novo). |
| ARIA / role / aria-modal / aria-pressed | Navigation & state | Confirmar `role="dialog"`, `aria-modal`, `aria-label`, `aria-pressed` no theme toggle. |
| Autocomplete / locale | Locale | Strings via `src/l10n/strings.ts`; confirmar teclado/data respeita locale. |
| Teste de contraste real (AA) | Accessibility | Axe / Lighthouse sobre `catalog`, `auth`, `sheet/edit`; corrigir se algum par falhar 4.5:1 (corpo) ou 3:1 (foco). |

**Arquivos de referência:** `web/src/engine/field_control.tsx`, `web/src/engine/exclusive_control.tsx`, `web/src/sheet/sheet_screen.tsx`, `web/src/catalog/catalog_screen.tsx`, `web/src/auth/`, `web/src/shell/`.

---

### Fase 2 — Refinamento visual anti-AI-slop (`frontend-design`)

| Critério | Referência | Ação |
|---|---|---|
| Compromisso de direção estética (refinado / editorial / utilitário) | Design Direction | Manter; não convergir para "dark with glow" ou "gradient text metrics". |
| Tipografia modular + fluidez | Typography | Verificar headings com `clamp()` (não só passos fixos); pairing só Geist. |
| Cor — um accent por tema, tintas nos neutros | Color & Theme | Nenhum cinza puro (`#808080`) sobre superfície; confirmar `-muted`/`-subtle` tintados. |
| Espaçamento com ritmo (variação, não uniforme) | Layout & Space | Evitar `padding: 16` em todo lugar; usar `tight grouping` vs `generous separation`; `clamp()` para gaps onde faz sentido. |
| Layout — container queries | Responsive / Layout | Se componente adaptar independentemente do shell (ex. engine em telas estreitas), usar `@container`; não amputar funcionalidade mobile. |
| Detalhes visuais intencionais (sem decoração inútil) | Visual Details | Nenhum glass card decorativo, divider arredondado com borda espessa de um lado, sparkline sem dados. Se marcação de "Modified", manter `surface` + `accent-text`. |
| Motion — impacto, não ruído | Motion | Um momento orquestrado (staggered reveal de lista/templates/engine) com `ease-out-quart/expo`. Nenhum `bounce`; nenhuma animação de `width`/`height`/`padding`. |

**Arquivos de referência:** `web/DESIGN.md` (princípios, token architecture, componente specs), `web/src/theme/tokens.css`, `web/src/theme/tokens.ts`, `web/src/engine/engine.css`, `web/src/shell/shell.css`.

---

### Fase 3 — Interação e responsividade (`frontend-design` + `web-design-guidelines`)

| Critério | Referência | Ação |
|---|---|---|
| Progressive disclosure | Interaction | Engine já agrupa seções; confirmar grupos menos usados colapsados por padrão, avançado sob interação. |
| Hierarquia de botões (não todos `primary`) | Interaction | Confirmar `sheet_screen`: cancel/preview não competem com save; usar `ghost`/`outline`/`danger` conforme contrato. |
| Feedback otimista / inline | Interaction / Feedback | Salvar/preview pode atualizar footer imediatamente; inline success/error sem modal. Confirmar progresso async (loading). |
| Touch / safe areas | Touch & interaction | Áreas de toque respeitam `min 44 px`; nav fixo não barra conteúdo estreito. |
| Performance / DOM | Performance | Se catálogo crescer, considerar virtualização; garantir `useSyncExternalStore` sem re-render excessivo no engine. |

**Iniciação Fase 3 (confirmada) — auditoria inicial:**

- `Progressive disclosure`: engine (`sheet_form`) agrupa seções; ainda não colapsa grupos menos usados por padrão. Ação: avaliar `field_grouping` para adicionar estado colapsado por seção opcional, sem quebrar `builderMode`.
- `Button hierarchy`: `sheet_screen` usa `primary` (Save) e `secondary` (Preview); cancel/preview não competem. Confirmado conforme contrato; não há `outline`/`danger` usado fora do guard modal (`lg-button--danger` real, não confirm-by-Enter). Nenhuma alteração necessária.
- `Feedback inline`: footer (`SheetFooter`) já mostra `pending`/`error`/`saved` inline sem modal; preview error usa `role="alert"`. Confirmado.
- `Touch / safe areas`: nav fixo 200–240 px; conteúdo `min 320 / max 960`. Verificar se `sheet_screen` respeita `min 44 px` de toque nos botões de footer — `Button` primitiva precisa confirmar `min-height`/padding token.
- `Performance / DOM`: `useSyncExternalStore` já usado em `ConnectedField`/`ConnectedExclusive`; não há re-render excessivo observado. Catálogo ainda pequeno; virtualização não urgente.

**Auditoria completa Fase 3 (todas as telas) — confirmado / ação:**

- `catalog_screen`: estados `loading`/`empty`/`loaded`/`error`/`parseError` ensinam ação (retry via `Button`; empty mostra retry, não apenas mensagem). Progressive disclosure: overlay `CreateSheetDialog` só abre sob `openTemplateId`; foco retorna ao trigger (`handleClose` chama `triggerRef.current?.focus()`). Button hierarchy: `primary` para retry, entries usam `secondary` (`lg-button--secondary`). Touch: entries são `<button>` com `title`; tamanho depende de `lg-button--md` — precisa confirmar `min-height` token. Performance: `useCatalog()` (TanStack) — sem re-render excessivo observado; lista não virtualiza mas catálogo pequeno.
- `auth_screen`: progressive disclosure não aplicável (form único). Hierarquia: um único `primary` submit; modo switch usa `aria-pressed` (botões sem `button` primitiva, mas com `role` implícito). Feedback inline: erro em `p role="alert" aria-live="polite"`; `busy` seta `aria-busy` no submit + `disabled`. Touch: campos `label` + `input` com `autoComplete`; sem verificação explícita de `min 44 px` nos botões de modo — verificar no CSS token.
- `create_sheet_dialog`: overlay acessível (`role="dialog"`, `aria-modal`, `aria-labelledby`, `Escape` cancela, foco inicial no input, overlay click cancela). Progressive disclosure: detalhe do template carrega em `loading`/`loaded`/`error` antes de mostrar campo. Hierarquia: apenas `primary` para criar; cancel pelo botão no header (`lg-button` sem variante, mas não compete). Feedback: `submitError` como `role="alert"`; `isSubmitting` com `aria-busy`. Touch: botão de fechar + input precisam de tap target ≥ 44 px — confirmar token.
- `button` primitiva (`ui/button`): não declara `min-height`; usa apenas classes CSS (`lg-button--sm/md`). **Ação pendente**: confirmar/suplementar token de tamanho mínimo para touch (`min-height` via `--lg-*` ou `padding` token) para atender `min 44 px`.
- `left_nav` / shell: nav fixo 200–240 px; não colapsa; conteúdo respeita `min 320`; `leave_guard` evita navegação sem confirmação; `theme_toggle` usa `aria-pressed`. Performance: `useSyncExternalStore` usado para sessão; nenhum re-render excessivo.

**Implementações Fase 3 concluídas:**
- Token `--lg-size-control` elevado de 36 px → 44 px (toque seguro, AA / `min-height`).
- Engine (`sheet_form`) recebeu `collapsedSectionKeys` + `onToggleSection` (progressive disclosure por seção, sem estado interno).
- Teste `Happy path: collapsed sections...` adicionado; `l10n` atualizado (`engine.expand`, `engine.collapse`).
- Nenhum novo token além do ajuste de valor; nenhum literal `#`/`px` introduzido fora `src/theme/`.

**Referência imediata para implementação:** `web/src/sheet/sheet_screen.tsx`, `web/src/engine/sheet_form.tsx`, `web/src/shell/shortcut_registry.ts`, `web/src/catalog/catalog_screen.tsx`, `web/src/auth/auth_screen.tsx`, `web/src/catalog/create_sheet_dialog.tsx`, `web/src/ui/button.tsx`, `web/src/shell/left_nav.tsx`.

---

### Fase 4 — Validação e entrega (`web-design-guidelines` + testes)

| Critério | Referência | Ação |
|---|---|---|
| Source scan de literais | Enforcement / Tokens | Novos componentes devem passar `tokens_used.test.tsx`; nenhum `#`/`px` literal fora `src/theme/`. |
| Copy scan | Enforcement / L10n | Nenhum texto fora `src/l10n/strings.ts`; chaves resolvidas em `l10n_strings.test.ts`. |
| Contrast assertion | Accessibility | Manter AA em ambos os temas; se token ajustado, rodar suite. |
| Lighthouse / axe / e2e | Performance / Accessibility | Ativar gate F8 (`lighthouserc.json` existe, runner ainda não chegou); adicionar sweep axe nos casos de `catalog`, `auth`, `sheet`. |
| AI Slop Test | frontend-design | Antes de entregar: mostrar interface; se alguém diz imediatamente "AI fez", identificar elemento genérico e substituir por decisão intencional (tipografia, composição assimétrica, detalhe único). |

**Arquivos de referência:** `web/src/test/tokens_used.test.tsx`, `web/src/test/l10n_strings.test.ts`, `web/e2e/auth.spec.ts`, `web/playwright.config.ts`, `web/lighthouserc.json`.

---

## 3. Regra de uso do documento

- Não modificar tokens ou princípios em telas individuais — todo valor visual vem de `DESIGN.md` / `tokens.css`. Se precisar de token novo: declarar nos dois temas → espelhar `tokens.ts` → rodar `npm test` (como especificado no fluxo obrigatório de `DESIGN.md`).
- Antes de qualquer PR de UI: confirmar `tokens_used.test.tsx` e `l10n_strings.test.ts` verdes; se mudança visual, incluir caso de edge (tema escuro, teclado apenas) junto ao happy path (`AGENTS.md` — testing conventions).
- Para cada fase: começar pelo que já está pronto (ex. Fase 1 começa revisando `field_control`/`engine` antes de tocar `catalog`).

---

## 4. Referências diretas no repo (para consulta durante a fase)

- Visual contract único: `web/DESIGN.md` (supersede qualquer outra declaração).
- Regras de desenvolvimento/visual: `web/AGENTS.md` (aponta para DESIGN.md).
- Tokens: `web/src/theme/tokens.css`, `web/src/theme/tokens.ts`.
- Primitives: `web/src/ui/`.
- Engine: `web/src/engine/`.
- Shell/rotas: `web/src/shell/`.
- Catalog/auth/sheet: `web/src/catalog/`, `web/src/auth/`, `web/src/sheet/`, `web/src/library/`.
- E2E/testes: `web/e2e/`, `web/src/test/`.
- Skills: `.claude/skills/frontend-design/` (references: typography, color, spatial, motion, interaction, responsive, ux-writing) e `.claude/skills/web-design-guidelines/references/guideline.md`.

---

*Documentado em modo de execução (Plan já encerrado). Este arquivo é o plano-programa de melhoria visual/usabilidade do `@web/`; a implementação segue fase a fase conforme confirmação do usuário.*