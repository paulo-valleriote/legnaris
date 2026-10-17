# Contract: Catalog Screen Refactor

**Applies to**: `web/src/catalog/` (catalog_screen.tsx, catalog.css, catalog.ts, catalog_queries.ts)
**Requirements**: UIP-01, UIP-02, UIP-03, UIP-04, UIP-05, SC-001, SC-002, SC-003, SC-004
**Supersedes**: the catalog's previous visual contract (the catalog's `.lg-button` from `catalog.css` and the create-sheet dialog's `.lg-panel` region)

## The screen

| Field | Value |
|---|---|
| Path | `web/src/catalog/catalog_screen.tsx` |
| Region | `main` in the two-region shell (`nav \| content`) |
| Primitives consumed | `Button` (primary), `Dialog` (create-sheet) |
| Tokens consumed | `--lg-color-bg`, `--lg-color-surface`, `--lg-color-text`, `--lg-color-text-muted`, `--lg-color-border`, `--lg-color-accent`, `--lg-color-accent-text`, `--lg-color-focus`, `--lg-shadow-1`, `--lg-space-*`, `--lg-type-*`, `--lg-radius-*`, `--lg-size-*` |

## Visual contract

- The screen renders a list of template cards (name + metadata) and a primary "Create Sheet" button.
- Every visual value is a `--lg-*` token. No literal color, pixel, or font-value in the source or rendered DOM (except `src/theme/`).
- The primary button uses `--lg-color-accent` fill with `--lg-color-accent-text` text.
- The dialog uses `--lg-shadow-3` and `--lg-color-overlay` scrim, with `role="dialog"`, `aria-modal`, focus trap, and `Esc` to cancel.

## Functional contract

- Loading state: shows a loading indicator until data arrives.
- Error state: shows an inline error message with a retry button.
- Empty state: shows a designed empty state (one line of plain text).
- Create-sheet dialog: opens on button click, closes on cancel or Esc, returns focus to the trigger button.

## Enforcement

| Check | Method |
|---|---|
| No literal color/pixel in source or DOM | `tokens_used.test.tsx` (extended to `src/catalog/`) |
| Contrast passes in both themes | `tokens_contrast.test.ts` |
| Functional behavior preserved | `catalog_screen.test.tsx` |
| Dialog opens/closes correctly | `catalog_screen.test.tsx` |
| Keyboard navigation follows visual order | `catalog_screen.test.tsx` |
| Copy resolves through `l10n` | `l10n_strings.test.ts` (extended to `src/catalog/`) |

## Failure modes

- If the literal scan fails, the developer must replace the offending value with a token.
- If the contrast assertion fails, the developer must adjust the token until it passes.
- If the functional test fails, the behavior change must be recorded in `web/AGENTS.md` as a deviation.
- If the dialog primitive is missing a variant, the developer must add a component token (`--lg-color-*`) rather than inventing a literal.