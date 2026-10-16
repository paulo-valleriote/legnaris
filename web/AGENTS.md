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
in `src/shell/`, `src/catalog/` or `src/engine/`).

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

## API client (`src/api/`)

The typed network layer every screen mounts on. Its rules are non-negotiable.

- **One transport seam.** Every request goes through the transport injected at
  construction; no operation builds its own, and the transport is the single place a
  credential will ever be attached. Requests carry `credentials: 'omit'` — no ambient
  credential is attached implicitly, in any environment.
- **No logging.** The module contains no `console.*` call and imports no logger; a request
  body, a credential and an identifier-bearing URL never reach a log. A source scan
  (`src/test/api_no_logging.test.ts`) enforces this.
- **No user-facing literal.** Every message a person sees resolves through
  `src/l10n/strings.ts`; the API module emits codes and failure kinds, never copy.
- **Failures are classified, drift is distinct.** A refusal becomes a `DomainFailure` with
  a `FailureKind`; an unreadable response stays a `DefensiveParseFailure`; a malformed
  dotted path raises a `PathFailure`; an unusable base address raises a
  `ConfigurationFailure` at construction. A new backend code must be given a kind in
  `error_mapper.ts` — the mapping table is total over the codes the backend emits.
- **Captures are never hand-edited.** `src/api/fixtures/` holds real responses written
  verbatim by `capture.sh`; a capture that stops matching the models is a backend contract
  change and the pinned suite is expected to go red on it.

## UI engine (`src/engine/`)

The schema-driven rendering core (feature `web-ui-engine`): it turns a decoded
`FieldSchema` tree into labeled sections and controls and owns **no state** — no value,
no request, no mark. It mounts no screen; every case renders its modules directly.

- `field_grouping.ts` — pure derivation of ordered, labeled sections and render rows
  (stacked leaf, declared-column table, exclusive group).
- `field_control.tsx` / `exclusive_control.tsx` — one control per leaf schema, one
  single-choice control per exclusive group.
- `sheet_form.tsx` — the composition; `sheet_defaults.ts` — the draft seed/overlay/merge;
  `sheet_mark.ts` — the preview-mark contract; `engine.css` — token-only styles.
- Every string resolves through `src/l10n/strings.ts`; every visual value is a `--lg-*`
  token. Data is rendered as text; markup in a value or label stays literal (no DOM sink).

**One control source.** `SheetForm` takes exactly one: `data` + `onChanged`, or both
`controlBuilder` and `exclusiveControlBuilder`. Any other combination throws rather than
half-render (ENG-25). In builder mode the composition reads no value, so the caller owns
the per-field subscription and edit reporting through the seams (`controlBuilder`,
`exclusiveControlBuilder`, optional `rowMarkBuilder`) — the web expression of `AD-001`.

**Numeric guard.** The numeric input seeds from its stored value, holds its display text
locally, and reports an edit only when the whole text resolves to a finite number: an
empty field clears the value, `NaN`/`Infinity`/`1e999` leave the stored value unchanged.
The desktop's partial-token parsing and restore-on-blur are deliberately dropped (ENG-12).

**Mark tint.** A marked control reuses `--lg-color-surface` from `tokens.css`; no new
token was added for the preview tint.

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

## Delivered deviations from the feature task lists

Recorded here instead of weakening an assertion (constitution III) or silently
dropping a task.

### F1 — `web-pwa-foundation`

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
- **No `@lhci/cli`** (the plan's tooling line named Lighthouse CI). Nothing invoked it:
  no npm script and no CI step consume it, because the Lighthouse gate belongs to F8
  (`F8-T005`). Its dev-only tree carried the only advisories the `sca` gate could see
  (17 findings, 11 high, two of them — `extract-zip` and `sprintf-js` — with no patched
  release), so the gate could not pass while it was installed. `web/lighthouserc.json`
  (T025) stays; the runner arrives with the F8 gate. Decision: `AD-011`.

### F2 — `web-api-client`

- **No MSW** (the program plan's stack table named it). The injected transport satisfies
  API-07 and makes the retry count and the `credentials: 'omit'` posture directly
  assertable, so no network-interception dependency was added. Recorded in the plan as a
  deliberate deferral.
- **No app-level client wiring yet** (T021, reworded). No screen constructs the client in
  this feature, so `resolveApiConfig` is documented in `web/README.md` as the single
  source of the base address rather than wired into an app entry point; the first feature
  that constructs the client consumes it. The insecure state is reported on the resolved
  configuration, not rendered.
- **A shared decoding helper, `src/api/parse.ts`**, is not named in the task list. It
  holds the payload-narrowing helpers (`isPlainObject`, `drift`, `requireString`, …) that
  the envelope and both models share, instead of duplicating them per module.
- **The INTERNAL_ERROR capture comes from an unmatched route.** No consumed endpoint
  deterministically returns 500, so `error_internal.json` is captured from `GET
/no-such-route`, which surfaces through the global handler as
  `{"code":"INTERNAL_ERROR"}` — a real response, never a hand-written stand-in.
- **The live suite's two mechanism cases assert the gate predicate**, not a real skip;
  the API-38 flow cases run only against a configured backend (`API_BASE_URL`).

### F3 — `web-catalog`

- **`@tanstack/react-query` 5.104.3 is added** (`AD-016`). It is the program plan's
  server-state choice (Parte 3, the F3 task "Queries TanStack"), recorded here as an
  inherited plan decision rather than a new one; it is the feature's only new runtime
  dependency, and the query client disables the library's own retry so the retry policy
  stays the F2 client's.
- **`src/shell/empty_workspace.tsx` is deleted** and the root route renders
  `src/catalog/catalog_screen.tsx` (`AD-013`). The F1 shell/routing/e2e assertions that
  pinned the empty workspace are retargeted to the catalog, not deleted.
- **The left navigation's root item is relabelled "Catalog"** through `nav.catalog`
  (`AD-015`, WCT-29), and `src/app/routes.ts` follows.
- **The creation surface is an accessible overlay, not a native `<dialog>`** (R5):
  `jsdom` does not implement `HTMLDialogElement.showModal`, so the platform element could
  not be driven by the rendering tests the constitution requires; the overlay keeps the
  same `role="dialog"` / `aria-modal` / Esc / focus contract.
- **The `.lg-empty*` rules stay in `src/shell/shell.css`** (deviation from T011). T011
  called them "now-unused" and asked for their removal with `empty_workspace.tsx`, but
  `src/shell/not_found.tsx` renders the same `.lg-empty` / `.lg-empty__title` /
  `.lg-empty__text` classes; deleting the rules would strip the not-found screen's
  styling. They are live CSS, so they are kept.
- **The dialog's accessible title is `sheet.dialogTitle`; the template's display name is
  rendered in the detail area.** The contract's title row and the "detail area resolves
  the display name" obligation are both satisfied this way, and the dialog always has a
  non-empty accessible name (WCT-14, WCT-27).
- **`playwright.config.ts` builds the e2e client with `VITE_API_BASE_URL` set to the
  preview origin**, so the browser specs intercept the catalog request locally instead of
  the built client reaching the production API; `e2e/catalog.spec.ts` blocks the service
  worker and scripts the two responses.

### F4 — `web-ui-engine`

- **The engine mounts no surface** (spec Clarifications, 2026-10-16). No screen, no
  `web/e2e/` case and no test-only route were added; every case renders the engine's own
  modules directly, so the feature proves `ENG-01…ENG-29` without touching the shell.
- **The `rowMarkBuilder` seam landed with the composition (T012), not T016.** The task list
  ordered the seam into US3, but the composition's label row is where the seam plugs in, so
  it shipped with `sheet_form.tsx` and US3 only added the mark tests and the shared
  `markLabel` copy helper.
- **`markLabel(mark)` is exported from `sheet_form.tsx`.** The desktop screen held the
  "Modified" / "Recalculated" copy in its own localization; here the engine owns the two
  catalog keys (`engine.modified`, `engine.recalculated`) so a caller's `rowMarkBuilder`
  names no literal. `sheet_mark.ts` stays behavior-free (a type and a class constant).
- **A section uses the group's declared label when non-empty, else a humanized key.** The
  contract (`R7` / data-model) requires the label; the desktop quirk that ignores a
  group's label is not carried over, because the contract is authoritative here.
- **The numeric input holds its display text locally plus a forced re-render** (a
  `useReducer` tick) instead of the desktop's `TextEditingController` + restore-on-blur:
  React would otherwise refuse to visually reject a whole-value edit, so a rejected
  non-finite text re-renders the controlled input back to the last accepted text (ENG-12).
- **Data mode renders exclusive controls inert** (no `onSelect`), matching the desktop's
  read-only preview; exclusive editing flows through `exclusiveControlBuilder`.
- **The engine tests are flat in `src/test/`** with an `engine_*` prefix, matching the F1
  layout, and the token/copy scans (`tokens_used.test.tsx`, `l10n_strings.test.ts`) now
  cover `src/engine/`.
