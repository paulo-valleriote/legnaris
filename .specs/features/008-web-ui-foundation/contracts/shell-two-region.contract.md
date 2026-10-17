# Contract: Application Shell (two regions)

**Applies to**: every client screen rendered under `web/src/`.
**Requirements**: UIF-24 … UIF-27, UIF-32.
**Supersedes**: `001-web-pwa-foundation/contracts/shell-layout.contract.md` (its three-region table
and its panel-collapse rule). Its token and interaction clauses are restated by
`token-contract.md`.

## Regions

| Order | Region | Landmark | Width |
|---|---|---|---|
| 1 | Navigation | `nav` with an accessible name | `--lg-size-nav-min`–`--lg-size-nav-max`, fixed |
| 2 | Content | `main` | grows, readable measure `--lg-size-content-max` |

Rules:

- Exactly these two regions render on **every** declared route; there is no third region in the
  markup, and no route renders a region without real content.
- The navigation is persistent at every width: it never collapses into a hamburger or an overlay,
  and it remains visible as the viewport narrows.
- The content keeps a minimum width and MUST NOT overlap the navigation; the readable measure is
  preserved rather than stretched across a wide window.
- Region order (navigation before content) is asserted by test.

## Removed

The contextual panel is removed, not hidden:

- `web/src/shell/right_panel.tsx` no longer exists.
- The `.lg-panel*` rules no longer exist in `web/src/shell/shell.css`.
- `--lg-size-panel-min` / `--lg-size-panel-max` no longer exist in `tokens.css` or `tokens.ts`.
- `SHELL_NARROW_QUERY` (the `max-width: 900px` panel-collapse breakpoint) no longer exists, and
  neither does the shell's narrow variant.

## Navigation footer

- The navigation ends in a footer block that hosts the theme control beside the session control
  (`UIF-27`).
- The footer's controls keep the keyboard and focus rules of the shell: visible focus ring, tab
  order following visual order, hover that changes only background/tint.

## Verification

| Check | Method |
|---|---|
| Two regions, in order, on every route | `shell.test.tsx` role-order walk; `web/e2e/shell.spec.ts` |
| No third region anywhere | rendering assertion (`queryByRole('complementary')` is absent) |
| Navigation persists and content keeps its minimum | `shell_responsive.test.tsx` at wide and narrow widths; browser spec at a narrow viewport |
| Panel module, rules, tokens and breakpoint gone | source scan asserting no reference to the removed names |
| Footer hosts the theme control | `shell.test.tsx` within-the-nav assertion |
| Deep links and the not-found route still render the shell | `web/e2e/shell.spec.ts` |
