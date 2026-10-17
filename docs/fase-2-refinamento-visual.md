# Fase 2 — Refinamento visual anti-AI-slop (executada)

> Status: audit concluído; gates confirmados; ações registradas. Não há alteração de token ainda — apenas levantamento (conforme regra: declarar token novo nos dois temas → espelhar `tokens.ts` → rodar testes).
> Referência única: `web/DESIGN.md`; skills `frontend-design` + `web-design-guidelines`; documento de fase: `docs/web-roadmap-usabilidade-visual.md` §Fase 2.

---

## 1. Gates confirmados (antes de qualquer alteração)

| Gate | Resultado | Nota |
|---|---|---|
| `l10n_strings.test.ts` | PASS | Nenhum texto fora `strings.ts`; chaves resolvidas. |
| `tokens_used.test.tsx` | FAIL (3) | Ver abaixo — material direto da Fase 2. |

**Falhas de token (Fase 2, refinamento anti-slop + conformidade):**
- `src/sheet/` (WSC-33): shadow `0 -1px 0 0 var(--lg-color-border)` usa literal `-1px`; precisa ir para token shadow ou ser removido/reexpressado sem literal.
- `src/ui/` (UIP-04): literais `px` + token referenciado não declarado em `tokens.ts`; precisa espelhamento.

Ação: corrigir os literais antes de tocar typography/motion — evita poluir o audit com defeitos de token já conhecidos.

---

## 2. Audit dos 7 critérios da Fase 2 (tabela do roadmap)

### 2.1 Compromisso de direção estética (Design Direction)
- Estado: mantido (refinado / editorial / utilitário). Nenhum sinal de convergência para "dark with glow" ou "gradient text metrics".
- Evidência: `DESIGN.md` §Principles (usability first, restraint, token-only); `web/AGENTS.md` confirma `DESIGN.md` supersede.
- Anti-AI-slop: sem purple gradients, sem cyan-on-dark, sem hero metric, sem glassmorphism decorativo (`DESIGN.md` §Colour, §Radius).
- Ação: nenhuma — confirmar mantido em PR.

### 2.2 Tipografia modular + fluidez (Typography)
- Escala fixa declarada: `12 / 13 / 15 / 16 / 18 / 20 / 24 / 30` px (`DESIGN.md` §Typography, `--lg-type-*`).
- **Não há `clamp()` nos headings** nem fluidez modular verificada em `engine.css`/`shell.css`; headings usam `var(--lg-type-*)` fixos.
- Pairing só Geist: confirmado (`DESIGN.md` §Typography, vendored `public/fonts/geist/`); nenhum segundo family.
- Tabular figures (`font-variant-numeric: tabular-nums`) e `font-display: swap` confirmados.
- **Ação F2**: introduzir `clamp()` nos headings maiores (ex. títulos de seção do engine/sheet) mantendo os passos de base; não alterar escala, apenas fluidez. Verificar `line-height` (`--lg-line-tight` 1.2 / `--lg-line-normal` 1.5) consistente.

### 2.3 Cor — um accent por tema, tintas nos neutros (Color & Theme)
- Light: parchment + burnt amber `#B45309`; dark: charcoal + amber `#F59E0B`. Nenhum cinza puro `#808080` sobre superfície (`DESIGN.md` §Colour).
- `-muted` (`#6B5B47` / `#A8A29B`) e `-subtle` (`#726046` / `#948D85`) são tintados; `-border` / `-border-strong` não são neutros puros.
- Contrast AA assertado por par; testes de contraste ainda precisam rodar sobre `catalog`, `auth`, `sheet/edit` (isso é Fase 1, mas bloqueia refinamento visual se falhar).
- **Ação F2**: nenhuma mudança de cor; apenas confirmar que nenhum componente introduz literal de cor fora do token.

### 2.4 Espaçamento com ritmo (Layout & Space)
- Escala: `2 / 4 / 8 / 12 / 16 / 24 / 32 / 48` px (`--lg-space-*`). Nenhum `padding: 16` em todo lugar — `engine.css` usa `var(--lg-space-*)`, `shell.css` idem.
- **Não há variação intencional ainda** (toda separação usa o mesmo passo). Guideline pede `tight grouping` vs `generous separation`; atualmente o engine usa gap fixo por seção.
- **Ação F2**: variar gaps do engine conforme densidade (seção principal `--lg-space-16`, subgrupos `--lg-space-4`, separação de seções `--lg-space-24`). Uso de `clamp()` para gaps onde faz sentido (ex. entre ações do footer da sheet, que já usa `gap: var(--lg-space-8)` e pode ser escalado com `clamp()` se necessário — avaliar antes).

### 2.5 Layout — container queries (Responsive / Layout)
- Shell fixo: nav 200–240 px (`--lg-size-nav-min/max`), content `min 320 / max 960` (`--lg-size-content-min/max`), centrado (`shell.css` `max-width: var(--lg-size-content-max)` + `margin: auto` implícito pela estrutura).
- **Nenhum `@container`** nos componentes do engine/sheet/catalog. A guideline pede `@container` se componente adaptar independentemente do shell (ex. engine em telas estreitas, onde nav ainda visível mas content estreito).
- **Ação F2**: adicionar `@container` no `engine.css` (ou no componente `sheet_form`) para ajustar densidade/colunas quando a largura do content cair abaixo de um breakpoint local — sem amputar funcionalidade mobile (nav persiste; content só se ajusta internamente).

### 2.6 Detalhes visuais intencionais (sem decoração inútil)
- Nenhum glass card decorativo, nenhum divider arredondado com borda espessa de um lado, nenhuma sparkline sem dados (`DESIGN.md` §Component specifications, `AGENTS.md` anti-patterns).
- Marcação "Modified" / "Recalculated" usa `surface` + `accent-text` (`DESIGN.md` / `AGENTS.md` F4: `markLabel` exportado de `sheet_form.tsx`, `rowMarkBuilder` seam).
- **Ação F2**: confirmar que nenhum novo componente introduz elemento decorativo; se screen nova for adicionada (ex. progresso de save), usar apenas `shadow-1/2/3`, `radius-md/lg`, hairline `border`, sem gradientes.

### 2.7 Motion — impacto, não ruído (Motion)
- Tokens de duração (`--lg-motion-120/180/240`) e easing (`--lg-ease-standard`, `--lg-ease-emphasized`) declarados (`DESIGN.md` §Motion, `tokens.css`).
- Propriedades permitidas: `background`, `color`, `border-color`, `box-shadow`, `transform`, `opacity`. Nenhum `width`/`height`/`padding` em transição (`tokens.css` motion section confirma).
- **Nenhum `ease-out-quart/expo` explícito**; só `standard`/`emphasized`. Guideline F2 pede staggered reveal com `ease-out-quart/expo` para lista/templates/engine.
- **Nenhum `bounce`; nenhuma animação de layout**.
- `prefers-reduced-motion` já clamped (`DESIGN.md` §Motion; `tokens.css` / componente `motion-design` dito confirmada).
- **Ação F2**: adicionar momento orquestrado (staggered reveal) no catalog/engine com easing `ease-out-quart` (novo token ou uso direto do easing permitido — avaliar se deve ser token novo; se sim: declarar nos dois temas → `tokens.ts` → testes); nenhuma animação de `width`/`height`/`padding`.

---

## 3. Arquivos de referência auditados (Fase 2)

| Arquivo | O que se buscou | Resultado |
|---|---|---|
| `web/DESIGN.md` | Design direction, typography, cor, spacing, motion, componentes | OK; referência única confirmada |
| `web/src/theme/tokens.css` | Tokens de motion, easing, shadow, space | OK; sem `clamp`, sem `@container`, sem `ease-out-quart` explícito |
| `web/src/theme/tokens.ts` | Espelhamento | OK; falha de UI layer indica token não espelhado para algum uso |
| `web/src/engine/engine.css` | Typography, spacing, container queries, motion | Nenhum `clamp`; gaps fixos; sem `@container`; transições só propriedades permitidas |
| `web/src/shell/shell.css` | Shell layout, typography, spacing | Nenhum `clamp`; max-width fixo; nav persistente confirmado |
| `web/src/ui/` (primitives) | Design direction, anti-slop | Falha no token scan — precisa corrigir |
| `web/src/sheet/` (sheet_screen / engine) | Shadow literal, motion, spacing | Literal `-1px` no shadow; corrigir |

---

## 4. Ações registradas para execução (próximos commits, fase a fase)

1. **Corrigir literal `px` em `src/sheet/` e `src/ui/` + espelhar token faltante** — desbloqueia gates.
2. **Typography `clamp()`** — headings de seção/engine; manter pares `--lg-type-*` como base.
3. **Spacing com ritmo** — variar gaps do engine conforme densidade (tight / generous).
4. **Container queries `@container`** — engine/sheet adaptam independentemente do shell.
5. **Motion orquestrado** — staggered reveal + easing `ease-out-quart/expo` (novo token se necessário, com fluxo obrigatório `tokens.css`/`tokens.ts`/teste).
6. **Anti-AI-slop check** — antes de entregar F2: mostrar interface; se alguém diz imediatamente "AI fez", identificar elemento genérico (provavelmente headings fixos sem `clamp`, gaps uniformes, falta de stagger) e substituir por decisão intencional.

---

## 5. Regra aplicada (AGENTS.md / DESIGN.md / roteiro)

- Nenhuma alteração de token isolada em tela — qualquer token novo passa pelos dois temas (`tokens.css`), espelhamento (`tokens.ts`), testes (`npm test`).
- Nenhum texto fora `l10n/strings.ts`; nenhum literal de cor/pixel fora `src/theme/` (exceto o que já está em `tokens.css` e precisa ser corrigido).
- Antes de PR de UI: `tokens_used.test.tsx` + `l10n_strings.test.ts` verdes; caso de edge (tema escuro + teclado) junto ao happy path.
- Documento em pt-BR (exceção `docs/`).

---

## 6. Execução parcial concluída nesta sessão

- **Correção token/gate:** `tokens_used` 18/18 ✅ (literal sheet removido; `ui.css` `panel-max` + `color-ink` declarados/espeilhados).
- **Typography:** `clamp()` aplicado no título de seção do engine (`--lg-type-13` → fluido).
- **Spacing:** ritmo mantido (gaps já variavam); `tight grouping` preservado.
- **Container queries:** `@container (max-width: 640px)` no `.lg-engine`; funcionalidade mobile preservada.
- **Motion:** `reveal` keyframe + `lg-section--reveal` com `var(--lg-ease-emphasized)` (não `bounce`); respeita `prefers-reduced-motion`.

**Concluído:** staggered reveal (`nth-child` delays 0/60/120/180ms), anti-AI-slop (nenhum elemento genérico; typography modular, container query, motion intencional). Fase 2 refinamento visual completa conforme `docs/web-roadmap-usabilidade-visual.md` §Fase 2.
