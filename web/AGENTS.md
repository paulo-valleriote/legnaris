# AGENTS.md — web/ (React PWA client)

Visual and interaction rules that every agent must follow when building screens in
`web/`. Ported from `desktop/AGENTS.md` to the web stack: the design decisions are
made **once, here**, not per screen. Design values live in `src/theme/tokens.css`
(mirrored in `src/theme/tokens.ts`) and are the only sanctioned source of visual
values (FND-02).

## Design Principles

- **Functional before decorative.** Every pixel must carry data, hierarchy, or an
  affordance. Decoration that helps scanning (dividers, section headers) is allowed;
  decoration that only looks "designed" is not.
- **Dense yet legible.** Prefer tables, aligned rows, and compact controls over
  whitespace-driven layouts. Never go below the token sizes to gain density.
- **Lowest cognitive load possible.** Plain labels first; captions only when the
  interface would otherwise be ambiguous.
- **Desktop-first.** Mouse + keyboard, hover states, wide windows, persistent
  navigation. No mobile patterns (bottom bars, swipe-only actions, FABs).
- **Schema-driven UI.** Screens render whatever `FieldSchema` the template returns —
  never hardcode layout, labels, or field order for a specific RPG system.

## Avoid

| Avoid                                            | Instead                                                                                                  |
| ------------------------------------------------ | -------------------------------------------------------------------------------------------------------- |
| Cards for everything                             | Dividers + section headers; cards only for genuinely bounded objects                                     |
| Gradients                                        | Flat surfaces                                                                                            |
| Glassmorphism / backdrop blur                    | Opaque surfaces with hairline borders                                                                    |
| Corner radius > 4px                              | Radius 2px or 4px (`--lg-radius-2` / `--lg-radius-4`)                                                    |
| Large headings                                   | Type scale caps at 17px (`--lg-type-17`)                                                                 |
| Floating action buttons                          | Primary action in a fixed footer bar or toolbar                                                          |
| Decorative illustrations (incl. empty-state art) | One line of plain text                                                                                   |
| Icon overload                                    | Text labels; icons only for universally recognized actions                                               |
| "AI SaaS" aesthetic                              | Purple-blue gradients, glass panels, hero text, pill buttons, emoji icons, glow effects — all prohibited |

## Layout

- **Left navigation is persistent** (never collapses into a hamburger/overlay on
  desktop); fixed width `--lg-size-nav-min`–`--lg-size-nav-max` (200–240px).
- **Main content is width-constrained** to `--lg-size-content-max` (~960px); do not
  stretch fields across a 2560px window.
- **Secondary information goes to the right panel** (`--lg-size-panel-min`–`--lg-size-panel-max`,
  280–320px): connection status, field details, derived/cascade preview, template summary.
- Shell order: `navigation | content | panel` (FND-01).
- **Resize behavior**: when the viewport narrows, the panel hides first
  (`SHELL_NARROW_QUERY`), then content reaches its minimum; content never overlaps the
  nav (FND-03).

## Tokens (concrete values)

Fixed. Components must not contain magic numbers — every size, radius, and color comes
from a `--lg-*` custom property. A rendering test enforces this (no literal hex or px
in `src/shell/`).

| Token group           | Values                                                                                      |
| --------------------- | ------------------------------------------------------------------------------------------- |
| Type scale            | `--lg-type-12` · `--lg-type-13` · `--lg-type-15` · `--lg-type-17` — **never above 17px**    |
| Font family           | `--lg-font-family` (system stack) — no custom font in V1                                    |
| Spacing               | `--lg-space-4/8/12/16/24` only (multiples of 4)                                             |
| Radius                | `--lg-radius-2` (inputs, buttons) · `--lg-radius-4` (panels, dialogs)                       |
| Row height            | `--lg-size-row` (32px)                                                                      |
| Control height        | `--lg-size-control` (32–36px)                                                               |
| Nav / content / panel | `--lg-size-nav-*` / `--lg-size-content-max` / `--lg-size-panel-*`                           |
| Borders               | `--lg-size-hairline` (1px)                                                                  |
| Colors                | `--lg-color-*` only — **no hardcoded hex in components**; destructive = `--lg-color-danger` |

Every `color` foreground token declares its pair in `src/theme/tokens.ts`
(`contrastPairs`); the contrast test asserts AA (FND-05).

## Interaction

- **Keyboard-first**: shortcuts are centralized in one registry
  (`src/shell/shortcut_registry.ts`) as a plain action → key map. V1 defaults: `Ctrl+S`
  save, `Ctrl+N` new sheet, `Ctrl+1`/`Ctrl+2` navigate, `Esc` close panel/dialog, `F5`
  refresh.
- **Visible focus**: focus ring from `--lg-focus-*` on every focusable control; tab
  order follows visual order; no keyboard traps (FND-04).
- **Hover states are mandatory** on anything clickable: hover changes background/tint
  only — never layout or size.
- **Destructive actions require explicit confirmation** — a dialog stating the exact
  consequence. Never a one-click destroy, never confirm-by-Enter alone.
- **Avoid dialog overuse.** Prefer inline surfaces: footer bar for dirty state and
  Save, right panel for details, inline messages for errors. Dialogs are for creation
  flows, destructive confirmation, and blocking errors only.
- **Feedback**: every async action shows progress and an inline result (success or
  mapped error).

## Security rules (non-negotiable)

- **No DOM injection sinks**: `dangerouslySetInnerHTML`, `innerHTML`, `eval`,
  `new Function`, `document.write` are forbidden — ESLint fails the build on them
  (FND-10, SEC-02). React escapes text automatically; keep data as text.
- **No third-party scripts, styles, or fonts**: everything is self-hosted and
  same-origin, so a strict Content-Security-Policy can be enforced later.
- **No credentials in JavaScript-readable storage**: never put a token in
  `localStorage`, `sessionStorage`, or IndexedDB. The refresh credential is an
  `HttpOnly` cookie; the access token lives in memory only.
- All identifiers, comments, and user-facing strings in **English** (UI strings go
  through `src/l10n/`).

## Componentization

- Extract a shared component only when all hold: **≥ 3 real usages**, same shape and
  purpose, and the extraction does not leak schema-specific logic into it.
- Never abstract the schema-driven UI Engine internals, screen-specific layouts,
  one-off dialogs, or wireframe structures.
- No "base component" layers, no wrapper-for-wrapper's-sake.

## Dependencies

- **No new package without strict necessity.** Design-system dependencies (font
  packages, theme frameworks, component libraries) are prohibited in V1 — the tokens
  plus plain CSS are sufficient.
- If a dependency is ever unavoidable, justify it in the PR description.

## Delivered deviations from the F1 task list

Recorded here instead of weakening an assertion (constitution III) or silently
dropping a task.

- **No `src/app/providers.tsx`** (T019). V1 has no provider layer: the shell wraps the
  route table directly in `src/app/router.tsx`. The module lands with the first feature
  that genuinely needs shared context (`web-api-client`).
- **Tests are flat in `src/test/`** with a `*.test.ts(x)` suffix rather than co-located
  next to each source module, because the Vitest setup, the `matchMedia` mock, the
  contrast helper, and the render helper are shared by every suite.
- **Toolchain pins differ from the plan's aspirational ones** — TypeScript 5.9.3 and
  ESLint 9.39.5, because `typescript-eslint`, `eslint-plugin-react`, and
  `eslint-plugin-jsx-a11y` do not yet support TypeScript 6 / ESLint 10. See the
  "Toolchain version note" in `web/README.md`.
