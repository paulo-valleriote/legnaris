# Contract: Auth Screen Refactor

**Applies to**: `web/src/auth/` (auth_screen.tsx, auth_session.ts)
**Requirements**: UIP-11, UIP-12, UIP-13, UIP-14, UIP-15, SC-001, SC-002, SC-003, SC-005
**Supersedes**: the auth's previous visual contract (`.lg-button` from `auth.css` and the `.lg-panel` region for mode switch)

## The screen

| Field | Value |
|---|---|
| Path | `web/src/auth/auth_screen.tsx` |
| Region | `main` in the two-region shell (`nav \| content`) |
| Primitives consumed | `Field` (input), `Button` (primary), `Switch` (mode switch: sign in / sign up) |
| Tokens consumed | `--lg-color-bg`, `--lg-color-surface`, `--lg-color-text`, `--lg-color-text-muted`, `--lg-color-border`, `--lg-color-border-strong`, `--lg-color-accent`, `--lg-color-accent-text`, `--lg-color-focus`, `--lg-space-*`, `--lg-type-*`, `--lg-radius-*`, `--lg-size-*` |

## Visual contract

- The screen renders a centered panel with the active mode header ("Sign in" or "Sign up").
- The mode switch toggles between sign in and sign up, updating the header and button text accordingly.
- Every visual value is a `--lg-*` token. No literal color, pixel, or font-value in the source or rendered DOM (except `src/theme/`).
- The primary button uses `--lg-color-accent` fill with `--lg-color-accent-text` text.
- The submit button is disabled when the form is invalid.
- The panel is centered with appropriate padding; no new region or layout changes.

## Functional contract

- Mode switch: toggles between sign in and sign up.
- Fields: email and password fields with permanent labels and inline validation errors.
- Submit: button shows loading state and disables when the form is valid and submitted.
- Success: redirects to the sheet screen or shows a success state.
- Error: inline error appears at the form level with the mapped message.

## Enforcement

| Check | Method |
|---|---|
| No literal color/pixel in source or DOM | `tokens_used.test.tsx` (extended to `src/auth/`) |
| Contrast passes in both themes | `tokens_contrast.test.ts` |
| Mode switch toggles correctly | `auth_screen.test.tsx` |
| Fields render as labelled controls | `auth_screen.test.tsx` |
| Submit button disabled when invalid | `auth_screen.test.tsx` |
| Inline validation errors appear | `auth_screen.test.tsx` |
| Copy resolves through `l10n` | `l10n_strings.test.ts` (extended to `src/auth/`) |

## Failure modes

- If the literal scan fails, the developer must replace the offending value with a token.
- If the contrast assertion fails, the developer must adjust the token until it passes.
- If the functional test fails, the behavior change must be recorded in `web/AGENTS.md` as a deviation.
- If the switch primitive is missing a variant, the developer must add a component token (`--lg-color-*`) rather than inventing a literal.