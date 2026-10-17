# Contract: Your-sheet, Not-found, and Update-prompt Screens

**Applies to**: `web/src/library/` (your_sheet.tsx, not_found.tsx, update_prompt.tsx)
**Requirements**: UIP-16, UIP-17, UIP-18, UIP-19, UIP-20, UIP-21, UIP-22, SC-001, SC-002, SC-003
**Supersedes**: the screens' previous visual contract (`.lg-empty*` from `shell.css` and the inline `.lg-button` from `auth.css`)

## The screens

| Field | Value |
|---|---|
| Paths | `web/src/library/your_sheet.tsx`, `web/src/library/not_found.tsx`, `web/src/library/update_prompt.tsx` |
| Region | `main` in the two-region shell (`nav \| content`) |
| Primitives consumed | `Button` (primary, optional) |
| Tokens consumed | `--lg-color-bg`, `--lg-color-surface`, `--lg-color-text`, `--lg-color-text-muted`, `--lg-color-accent`, `--lg-color-accent-text`, `--lg-color-focus`, `--lg-space-*`, `--lg-type-*`, `--lg-radius-*`, `--lg-size-*` |

## Visual contract

- Each screen displays one line of plain text (the message) and, where an action exists, a single F2 `Button` primitive.
- Every visual value is a `--lg-*` token. No literal color, pixel, or font-value in the source or rendered DOM (except `src/theme/`).
- The primary button uses `--lg-color-accent` fill with `--lg-color-accent-text` text.
- Where no action exists (loading, not-found with no alternative), only the message is shown.
- Keyboard focus is correctly managed: if a button exists, it receives initial focus; focus rings are visible.
- The screens honor `prefers-reduced-motion: reduce` by clamping non-essential transitions to near-zero duration.

## Functional contract

- **Your-sheet**: shows "No sheet created yet" with a "New sheet" button when no sheet exists; shows "Loading your sheet..." with no action button when a sheet is opening.
- **Not-found**: shows "Sheet not found" with a "Go to catalog" button when a sheet ID is not found.
- **Update-prompt**: shows "A new version is available" with an "Update now" button when an update exists.
- The action button triggers the corresponding navigation or action.

## Enforcement

| Check | Method |
|---|---|
| No literal color/pixel in source or DOM | `tokens_used.test.tsx` (extended to `src/library/`) |
| Contrast passes in both themes | `tokens_contrast.test.ts` |
| Message renders correctly | `your_sheet.test.tsx`, `not_found.test.tsx`, `update_prompt.test.tsx` |
| Action button renders when applicable | `your_sheet.test.tsx`, `not_found.test.tsx`, `update_prompt.test.tsx` |
| No action button when not applicable | `your_sheet.test.tsx`, `not_found.test.tsx`, `update_prompt.test.tsx` |
| Keyboard focus on button when applicable | `your_sheet.test.tsx`, `not_found.test.tsx`, `update_prompt.test.tsx` |
| Copy resolves through `l10n` | `l10n_strings.test.ts` (extended to `src/library/`) |

## Failure modes

- If the literal scan fails, the developer must replace the offending value with a token.
- If the contrast assertion fails, the developer must adjust the token until it passes.
- If the functional test fails, the behavior change must be recorded in `web/AGENTS.md` as a deviation.
- If the button primitive is missing a variant, the developer must add a component token (`--lg-color-*`) rather than inventing a literal.
- If the focus trap is broken, the developer must fix it without weakening the assertion.