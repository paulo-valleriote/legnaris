# Contract: Sheet Screen Refactor

**Applies to**: `web/src/sheet/` (sheet_screen.tsx, sheet_edit.ts, sheet_open.ts, sheet_form.tsx)
**Requirements**: UIP-06, UIP-07, UIP-08, UIP-09, UIP-10, SC-001, SC-002, SC-003, SC-004
**Supersedes**: the sheet's previous visual contract (`.lg-button` from `shell.css`/`sheet.css` and the inline field styling)

## The screen

| Field | Value |
|---|---|
| Path | `web/src/sheet/sheet_screen.tsx` |
| Region | `main` in the two-region shell (`nav \| content`) |
| Primitives consumed | `Field` (input/select/textarea/checkbox/radio/switch), `Button` (primary), `Toast` (inline feedback) |
| Tokens consumed | `--lg-color-bg`, `--lg-color-surface`, `--lg-color-text`, `--lg-color-text-muted`, `--lg-color-border`, `--lg-color-border-strong`, `--lg-color-accent`, `--lg-color-accent-text`, `--lg-color-danger`, `--lg-color-focus`, `--lg-shadow-1`, `--lg-space-*`, `--lg-type-*`, `--lg-radius-*`, `--lg-size-*` |

## Visual contract

- The header shows the sheet's identity (name + template).
- Fields are rendered as labelled controls using the F2 `Field` primitive.
- The sticky footer shows the dirty state and the primary Save action.
- Inline validation errors appear under fields with invalid data.
- Every visual value is a `--lg-*` token. No literal color, pixel, or font-value in the source or rendered DOM (except `src/theme/`).
- The primary button uses `--lg-color-accent` fill with `--lg-color-accent-text` text.
- The Save button is disabled when the form is invalid or clean.
- The toast/notice shows inline feedback for async actions.

## Functional contract

- Loading state: shows a loading indicator until data arrives.
- Header: shows the sheet name and template identity.
- Fields: renders each editable schema leaf as a labelled control.
- Dirty state: appears in the sticky footer when fields are edited; clears when clean.
- Validation: inline errors appear under fields with invalid data.
- Save: sends the editable delta with one PATCH request; button shows loading state.
- Error state: shows an inline error at the form level with a retry option.

## Enforcement

| Check | Method |
|---|---|
| No literal color/pixel in source or DOM | `tokens_used.test.tsx` (extended to `src/sheet/`) |
| Contrast passes in both themes | `tokens_contrast.test.ts` |
| Header shows identity | `sheet_screen_states.test.tsx` |
| Fields render as labelled controls | `sheet_screen_states.test.tsx` |
| Footer shows dirty state | `sheet_screen_states.test.tsx` |
| Save button disabled when invalid/clean | `sheet_screen_states.test.tsx` |
| Inline validation errors appear | `sheet_screen_states.test.tsx` |
| Copy resolves through `l10n` | `l10n_strings.test.ts` (extended to `src/sheet/`) |

## Failure modes

- If the literal scan fails, the developer must replace the offending value with a token.
- If the contrast assertion fails, the developer must adjust the token until it passes.
- If the functional test fails, the behavior change must be recorded in `web/AGENTS.md` as a deviation.
- If the field primitive is missing a variant, the developer must add a component token (`--lg-color-*`) rather than inventing a literal.
- If the toast/notice primitive is missing a state, the developer must add a component token (`--lg-color-*`) rather than inventing a literal.