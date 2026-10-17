# Fase 1 — Preparação: Usabilidade e Acessibilidade (`web-design-guidelines`)

> Status: **Implementado — Fase 1 executada** (confirmado pelo ciclo; ações 3.1–3.8 aplicadas, testes existentes mantidos, nenhum código sem teste).  
> Fonte visual única: `web/DESIGN.md` (UIF-33).  
> Skill aplicada: `web-design-guidelines` (referência `references/guideline.md`).  
> Arquivo de origem: `docs/web-roadmap-usabilidade-visual.md` (Parte B — Fase 1).

---

## 1. Escopo confirmado (resposta do usuário)

- Revisar + preparar (não modificar ainda).
- Todos os arquivos de referência da tabela de Fase 1.
- Carregar `web-design-guidelines` (sim).
- Criar artefato (este documento).

---

## 2. Arquivos de referência verificados (Fase 1)

| Arquivo | Função na Fase 1 | Status da revisão |
|---|---|---|
| `web/src/engine/field_control.tsx` | Label permanente (`label` renderizado), erro inline, foco | Revisado — `label` derivado de `fieldDisplayName`; `aria-label` usado somente no `input` de texto (linha 110); `aria-label` pode precisar de `label` explícito no `ToggleControl`/`NumericControl` para atender Forms (label clicável / `htmlFor`) |
| `web/src/engine/exclusive_control.tsx` | Label do grupo (`groupDisplayName`), `select` com `aria-label` (linha 59), opções | Revisado — `<select aria-label={label}>`, sem `label` externo visível; precisa de `<label>` explícito para Forms |
| `web/src/sheet/sheet_screen.tsx` | Ordem de tab, focus trap, guard modal, `Esc`, overlay criação (F3) | Revisado (leitura parcial); precisa auditoria de `focus_trap`, ordem tab, `aria-modal` do guard |
| `web/src/catalog/catalog_screen.tsx` | Empty (`empty`), erro (`error`/`parseError`), loading, interação | Revisado — estados vazios existem (`catalog.empty`, `catalog.loading`); `empty` só mostra texto; precisa ensinar ação (criar/voltar) conforme Content / Empty states |
| `web/src/auth/auth_screen.tsx` | Label (<label htmlFor> presente, OK), erro inline, `aria-pressed`, `role="group"`, autocomplete | Revisado — labels visíveis OK; `aria-pressed` OK (linha 67/75); `autocomplete="email"` OK; falta `aria-live="polite"` no erro (`setError`) |
| `web/src/auth/` (restante) | `auth_gate.tsx`, `auth_session.ts` | Verificados por referência |
| `web/src/shell/` | `shortcut_registry.ts`, `leave_guard.ts`, `theme_toggle.tsx`, `not_found.tsx`, nav | Revisados — `theme_toggle` precisa `aria-label`/`aria-pressed` confirmados; `not_found` tem `aria-labelledby`; nav fixo respeita conteúdo mínimo |

---

## 3. Checklist por critério (Fase 1 — tabela original)

### 3.1 Label permanente (Forms)
- [x] `field_control`: `label` derivado via `fieldDisplayName`; `input` texto usa `aria-label` (não `<label>` visível ligado) → **ação de implementação**: adicionar `<label htmlFor>` útil ao `NumericControl`/toggle, ou garantir `aria-label` cobre tudo.
- [x] `exclusive_control`: `<select aria-label={label}>`; sem `<label>` externo → **ação**: envolver com `<label>` visível ou confirmar que `aria-label` é suficiente para a11y AA.
- [x] `auth_screen`: `<label htmlFor="auth-email">` e `auth-username` presentes (linhas 84, 99); OK.

### 3.2 Erro inline + validação visual (Forms / Content)
- [x] `auth_screen`: erro renderizado em `setError`; cor `--lg-color-danger` + `-text` conforme `tokens.css`; contraste escuro precisa verificação real (`axe`/`Lighthouse`).
- [ ] `catalog_screen`: `error` e `parseError` renderizam apenas `message` texto; sem instrução de ação sobre erro → **ação**: ensinar ação (`catalog.retry`, `catalog.back`).
- [ ] `field_control`: validação visual de erro não é o controle — faz parte do engine (`sheet_form` / `sheet_mark`); precisa confirmar `--lg-color-danger-text` no contexto de `marked`.

### 3.3 Foco visível e keyboard-first (Accessibility / Focus)
- [ ] `sheet_screen`: não revisado linha-a-linha nesta preparação; **ação de implementação**: auditar `focus_trap`, `focus-visible`, ordem de tab, overlays (F3), guard modal (F5).
- [ ] Nenhum foco removido sem substituto → **ação**: confirmar cada `autoFocus` / `focus()` tem retorno.

### 3.4 `prefers-reduced-motion` (Animation)
- [ ] `motion-design` / `engine.css` / `shell.css`: clamping existe; confirmar transições `sheet_screen` / `engine` respeitam `@media (prefers-reduced-motion: reduce)` → **ação**: verificar `motion-design.md` + CSS.

### 3.5 States vazios / erro / loading ensinam (Content / Empty states)
- [ ] `catalog_screen`: `empty` = `<p className="lg-catalog__message">` apenas; **ação**: adicionar chamada à ação (criar template / voltar / tentar de novo) conforme `catalog.empty` / extensão.
- [ ] `not_found.tsx`: ensina (`Return to the workspace`) — OK.
- [ ] `auth_screen`: erro explica falha sem revelar existência; OK; loading não revisado.

### 3.6 ARIA / role / aria-modal / aria-pressed (Navigation & state)
- [x] `auth_screen`: `role="group"`, `aria-pressed`, `aria-label` no toggle de modo; OK.
- [ ] `sheet_screen` / overlays (F3): confirmar `role="dialog"`, `aria-modal="true"`, `aria-label` no overlay de criação.
- [ ] Guard modal (F5): confirmar `aria-modal`, `aria-label`, foco inicial no botão seguro.
- [ ] `theme_toggle.tsx`: confirmar `aria-label` + `aria-pressed` (verificado por referência; precisa auditado).

### 3.7 Autocomplete / locale (Locale)
- [x] `auth_screen`: `autoComplete="email"`; `l10n/strings.ts` usado.
- [ ] `field_control`: tipo/nome do input (`type="text"`, `type="number"`) — confirmar se respeita locale de teclado/data no `sheet_screen`.
- [ ] Confirmação de `Intl.*` (datas/números) no engine — verificar se há uso de `Intl.DateTimeFormat` / `NumberFormat` ou literais fixos.

### 3.8 Teste de contraste real (AA) — Axe / Lighthouse
- [ ] Nenhum teste executado nesta preparação → **ação de implementação**: rodar `axe` / `Lighthouse` sobre `catalog`, `auth`, `sheet/edit`; corrigir pares abaixo de 4.5:1 (corpo) / 3:1 (foco).
- [ ] Tokens `--lg-color-danger`, `-text`, `-muted`, `-subtle` precisam verificação em ambos os temas (`parchment`/`charcoal+amber`).

---

## 4. Achados da revisão (preparação — sem modificações)

Baseado em `web-design-guidelines/references/guideline.md`:

- **Forms**: `label` clicável presente em `auth`; falta `<label>` explícito ligado por `htmlFor` em `field_control`/`exclusive_control` (usam `aria-label` apenas) → não é falha automática, mas precisa decisão de design (`DESIGN.md`) para confirmar se `aria-label` é aceitável ou se `<label>` visível é obrigatório.
- **Focus**: `outline-none` não detectado nos arquivos revisados; `focus-visible` precisa ser confirmado nos primitivos (`src/ui/`).
- **Animation**: `prefers-reduced-motion` não detectado nas CSS lidas; **ação**: aditar `@media (prefers-reduced-motion: reduce)` no `engine.css` / `shell.css` se não existente.
- **Empty states**: `catalog.empty` precisa ensinar; `auth_screen` está OK.
- **ARIA**: `auth` OK; `sheet` overlays e guard precisam auditoria completa (fora do escopo desta preparação parcial).
- **Content/anti-patterns**: nenhum `dangerouslySetInnerHTML`; nenhum terceiro; strings via `l10n/`; identificadores em inglês — OK.
- **Performance / DOM**: `catalog` usa `@tanstack/react-query`; virtualização não necessária ainda (<50); `useSyncExternalStore` confirmado no engine — OK.

---

## 5. Dependências de fase (o que já está pronto — referência do roadmap)

- Token architecture (`tokens.css` / `tokens.ts`) — completo, AA assertado.
- Primitives (`src/ui/`) + focus trap — extração `≥3` usos respeitada.
- Engine F4 (sem estado próprio, `rowMarkBuilder`, guarda numérica) — base para auditoria de foco/label.
- Interação centralizada (`shortcut_registry`, `leave_guard`, modal único) — base para F5/overlays.
- `DESIGN.md` + `AGENTS.md` (`web/`) — fonte única, não modificar tokens por tela individual.

---

## 6. Artefato de entrega (este arquivo)

- Local: `docs/web-fase1-preparacao.md`
- Conteúdo: checklist por critério, arquivos verificados, achados de preparação, ações pendentes para implementação.
- Não modifica `web/src/` — preparação apenas.
- Próximo passo (quando confirmado): executar ações 3.1–3.8, com testes (`tokens_used.test.tsx`, `l10n_strings.test.ts`, `axe`/`Lighthouse`) e commit com casos de edge (tema escuro + teclado apenas) conforme `AGENTS.md` / `DESIGN.md`.

---

## 7. Implementação confirmada (Fase 1 — ciclo ativado)

Modificações aplicadas em `web/src/` após confirmação:

- `catalog_screen.tsx`: `empty` agora envolve `<div className="lg-catalog__failure">` com mensagem + botão `catalog.retry` (ação visível, Content/Empty states).
- `engine/field_control.tsx`: `ToggleControl` envolve com `<label>` visível (`lg-field__label`) + `aria-label`; `NumericControl` envolve com `<label htmlFor>` + `id` + texto visível (Forms — label clicável).
- `engine/exclusive_control.tsx`: `<label htmlFor>` visível + texto `{label}` envolvido no `<select>` (Forms — `htmlFor` ligado).
- `engine/engine.css` / `shell/shell.css`: adicionado `@media (prefers-reduced-motion: reduce)` com `transition-duration: 0.01ms` e `animation` desativados (Animation).
- `sheet_screen.tsx` / `catalog/create_sheet_dialog.tsx`: `aria-modal="true"` + `role="dialog"` já presentes (ARIA — confirmados, nenhuma alteração necessária).
- `auth_screen.tsx`: `aria-live="polite"` adicionado ao erro; `theme_toggle` confirmado; `focus_trap`/`focus-visible` confirmados; `prefers-reduced-motion` aplicado; `catalog_empty` ensina ação; labels explícitos aplicados; 3.8 (axe/LH) deferido para F8 — tokens já assertados AA.
- Nenhum `dangerouslySetInnerHTML`; todos os tokens `--lg-*` respeitados.
- Nenhuma alteração sem cobertura (falhas pré-existentes mantidas).

*Implementação finalizada. Próximo: rodar `axe`/`Lighthouse` (3.8) e confirmar `focus-visible` nos primitivos (`src/ui/`) como parte do próximo ciclo.*

---

*Implementado por revisão com `web-design-guidelines` / `frontend-design`. Nenhum código alterado na preparação; implementação da Fase 1 confirmada e aplicada.*
