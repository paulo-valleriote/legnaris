# Quickstart: Web Screens Refactor

## Prerequisites

- F1 delivered: `.specs/features/008-web-ui-foundation/` with tokens, theme toggle, shell, Geist font.
- F2 delivered: `.specs/features/009-web-ui-primitives/` with `src/ui/` components (button, field, dialog, toast).
- `make lint-web && make test-web && make test-race` all green on `main`.
- `npm install` has been run; `node_modules` available.

## Setup

1. **Theme & Font**: Ensure the Geist font is in `web/public/fonts/geist/Geist[wght].woff2` and the license is committed. The theme boot script (`public/theme-init.js`) loads same-origin before first paint.
2. **Tokens**: Verify `src/theme/tokens.css` has both light (`:root`) and dark (`[data-theme='dark']`) blocks. Run `npm test` to confirm the contrast assertion passes.
3. **Shell**: The shell is in two regions: `nav | main` (no panel). Verify `app_shell.tsx` has no `<RightPanel>` or `<aside className="lg-panel">`.
4. **F2 primitives**: Ensure `src/ui/` exists with these minimal exports:
   - `ui_button.tsx` (primary, secondary, ghost, outline, danger; sm/md; states hover/active/focus-visible/disabled/loading)
   - `ui_field.tsx` (input, select, textarea, checkbox, radio, switch; permanent label; inline error)
   - `ui_dialog.tsx` (role=dialog, aria-modal, focus trap, Esc, shadow-3, overlay)
   - `ui_toast.tsx` (inline notice; empty/loading/error; 1 line + optional 1 action)

## Run the Baseline

```bash
cd web && npm test
npm run lint
npm run typecheck
```

All must pass before any changes. Record the green result (this is T001 from F2 checkpoint).

## Extend the Source Scans

Edit `src/test/tokens_used.test.tsx` to add `src/ui/`, `src/catalog/`, `src/sheet/`, `src/auth/`, `src/library/` to the literal-scan assertions. Document that `src/theme/` is the allowed exception (as in F1).

Edit `src/test/l10n_strings.test.ts` to add the same directories and check for unresolved literals.

Run `npm test` again to ensure the scans pass with the new directories included.

## Refactor Each Screen

### Catalog

1. Replace any literal hex/pixel values with `--lg-*` tokens.
2. Import F2 `Button` primitives instead of the generic `.lg-button` from `catalog.css`.
3. Import F2 `Dialog` primitive for the create-sheet overlay instead of custom `.lg-panel`.
4. Ensure all colors/spacing/typography use `--lg-*` values.
5. Run `npm test`; fix any token/resolution failures.

### Sheet (Sheet Detail / Form)

1. Replace all form field controls with F2 `Field` primitive (input/select/textarea/checkbox/radio/switch).
2. Replace footer action buttons with F2 `Button` primitives.
3. Inline validation errors appear under fields using F2 styling with `--lg-color-border-strong` on the field border and `--lg-color-focus` on focus.
4. Sticky footer shows `--lg-color-text-muted` when clean, `--lg-color-text` when dirty; Save button `--lg-color-bg` enabled state and `--lg-color-text` text.
5. Run `npm test`; fix any token/resolution failures.

### Auth

1. Replace mode switch with F2 `Switch` primitive.
2. Replace all form fields with F2 `Field` primitive.
3. Replace submit button with F2 `Button` primary.
4. Center the panel with same positioning as before (no visual change).
5. Run `npm test`; fix any token/resolution failures.

### Your-sheet / Not-found / Update-prompt

1. Replace message text styling with `--lg-color-text` / `--lg-color-text-muted`.
2. Add F2 `Button` primitive for any action (New sheet, Go to catalog, Update now).
3. Ensure the message appears on one line; if a button exists, focus it on mount.
4. Run `npm test`; fix any token/resolution failures.

## Verify

After each screen refactor:

```bash
npm test          # all unit tests green
npm run lint      # lint clean
npm run typecheck # TypeScript clean
make lint-web     # web lint green
make test-web     # web tests green
make test-race    # race detector clean
```

If `make lint-specs` passes, you're ready to commit.

## Commit

When all screens are refactored and all tests pass:

```bash
git add .specs/features/010-web-screens-refactor/spec.md \
        .specs/features/010-web-screens-refactor/plan.md \
        .specs/features/010-web-screens-refactor/research.md \
        .specs/features/010-web-screens-refactor/data-model.md \
        .specs/features/010-web-screens-refactor/quickstart.md \
        .specify/feature.json
git commit -q -m "docs(specs): spec the web screens refactor (catalog, sheet, auth, your-sheet, not-found, update-prompt) with tokens and F2 primitives"
echo "Local commit complete. Nothing was pushed."
```

## Rollback

If anything fails after a change:

```bash
git reset --hard HEAD~1
# Re-record green baseline and re-apply changes
```