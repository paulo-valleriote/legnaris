# Research & Analysis for Web Screens Refactor

## Technical Comparisons

### Existing Screens (current state)

- **Catalog** (`src/catalog/`): Uses token CSS primitives, a generic `.lg-button` variant (primary/danger) from `catalog.css`, and a `.lg-panel` region for the create-sheet dialog. The button is not componentized; styling is in `catalog.css`. No self-hosting: fonts from Google Fonts.
- **Sheet** (`src/sheet/`): Uses token CSS primitives, `.lg-button` variants (primary/danger), and the `.lg-panel` region for the delete/confirm dialogs. No component primitives for inputs; field styling is in `shell.css` and `sheet.css`.
- **Auth** (`src/auth/`): Uses token CSS primitives, `.lg-button` variants (primary), and `.lg-panel` region for the mode switch.
- **Your-sheet / not-found / update-prompt** (`src/library/`): Uses token CSS primitives (CSS only) and `.lg-button` for actions. The messages are styled via classnames `.lg-empty__title`, `.lg-empty__text` (shell.css).

### Component Library Recommendations

1. **Button** (F2): Needs to cover `primary`, `secondary`, `ghost`, `outline`, `danger` variants; sizes `sm`, `md`, and states (`hover`, `active`, `focus-visible`, `disabled`, `loading`). Height 36–40px, radius `sm`/`md`. Should not reuse the `.lg-button` from existing screens because:
   - The existing `.lg-button` has only one color token (`--lg-color-bg`, `--lg-color-text`)
   - Missing ghost and outline variants (use `--lg-color-surface` and `--lg-color-border` for outline)
   - No size handling; needs consistent height and min-width for loading states
   - Missing focus ring token (`--lg-color-focus`)
   - No `:hover`/`disabled` tokens; colors not AA checked
2. **Field** (F2): Should unify input, select, textarea, checkbox, radio, switch rendering with:
   - Permanent label (no placeholder-as-label)
   - Inline error positioning (`top: -1rem`, `z-index: 1`)
   - Consistent token-based border (`--lg-color-border` / `--lg-color-border-strong`) and radius (`--lg-radius-sm`)
   - Focus ring (`--lg-color-focus`) and disabled coloring (`--lg-color-text-muted`)
   - Validation on blur or touch-end; errors styled with `.lg-field__error`
   - Numeric inputs right-aligned (`font-variant-numeric: tabular-nums`)
3. **Dialog/Overlay** (F2): Central dialog component for create-sheet, confirm, error modals; should avoid mixing in screen-specific layout (e.g., `shell.css`). Needs to:
   - Render under `role="dialog"`, `aria-modal` true, focus trap; `Esc` cancels.
   - Use `--lg-shadow-3`, `--lg-color-overlay` scrim.
   - Be placed on a dedicated `.lg-dialog` layer, not `catalog.css` or `shell.css`.
   - Provide a header slot (title) and content slot; always have an accessible name.
4. **Toast/Notice** (F2): Inline for action results; should replace any `.lg-toast` or similar usage (none currently). State: empty/loading/error, one-line message plus optional action. Consistent tokens and minimal markup.

### Tokens & Constraints

- **F1 tokens** (`src/theme/tokens.css`): Must be consumed; no literal colors or pixels allowed in component code. The existing screens already obey this (except `catalog.css` with `.lg-button` color tokens, which are still tokens — fine).
- **Contrast**: Both themes must pass AA (`tokens_contrast.test.ts`).
- **Font**: Geist variable font is vendored (`Geist[wght].woff2` in `public/fonts/`) and loaded via `@font-face`. No external fonts for the refactor.
- **Security**: No DOM injection sinks (`dangerouslySetInnerHTML` etc.). The plan forbids adding new risky dependencies.

## Integration Points

- **`src/shell/`**: The theme toggle lives there (`theme_toggle.tsx`), the shortcut registry, leave guard, and baseline shell styling. The F3 refactor must not change the shell’s role; only the content under the `main` region changes.
- **`src/theme/`**: Already provides `--lg-*` token CSS and the `theme.ts` store. The theme-init script (`public/theme-init.js`) boots before first paint.
- **`src/ui/`** (to be created): Will house the F2 primitives. The refactor screens will import from there. The plan has T014 (`WEB-DESIGN-016`) to create `src/ui/` and extract the primitive components.
- **`src/l10n/`**: All user-facing strings go through `src/l10n/strings.ts`. F3 must update any string changes here (e.g., button copy) and ensure keys are referenced.
- **`web/e2e/`**: The e2e tests (`.spec.ts`) reference the entire flow; when the screens refactor, the tests should still pass because behavior is unchanged, but the selectors may need to update (field controls, button text, dialog title). The task list will update the e2e specs after implementation (T012, T018, T027, T034).
- **`src/test/`**: Existing tests (`shell.test.tsx`, `shell_responsive.test.tsx`, `shell_keyboard.test.tsx`, `auth.spec.ts`, `catalog.spec.ts`, `sheet.spec.ts`) must be updated to reflect the new component structure. The existing test files should be edited directly (e.g., `catalog_screen.test.tsx`), not re-written.

## Technologies & Libraries

- **Frontend**: Vite + React 19, TypeScript 5.9.3 (as per `package-lock.json`). Build target: `dist/`.
- **Styling**: Plain CSS (no CSS-in-JS, no CSS framework). Token CSS (`src/theme/tokens.css`) -> `tokens.ts` -> `designTokens` + `contrastPairs`.
- **Component architecture**: F2 primitives are plain React functional components with minimal state; they are not framework-agnostic, they are React components that may use hooks like `useSyncExternalStore` for sync reads (mirrors the design from `src/sheet/`).
- **Testing**: Vitest + React testing library (existing). Mocks for API responses (`src/test/fake_transport.ts`).
- **Build**: `npm run lint` runs ESLint; `npm run lint-web` runs the web-specific lint (extends from the root lint but for `web/`). `npm test` runs Vitest; `npm run typecheck` runs tsc.
- **Migration risk**: The screens keep their functional behavior; only the visual rendering changes.

## Assumptions

1. The F2 primitives exist as documented in `web/DESIGN.md` §3.8 with the exact variant/state list.
2. The token scan extension to `src/ui/` and `src/catalog/`, `src/sheet/`, `src/auth/`, `src/library/` is enforced by the same validation (`tokens_used.test.tsx`) and the spec scans (`design_guidelines.test.ts`).
3. The design document is authoritative; AGENTS.md may list new extraction rules (`src/ui/` as a component library) but does not specify its internal contracts; those live in `web/DESIGN.md`.
4. The F1 theme and boot script (same-origin classic script) continue to work and are not altered by the screen refactor.
5. The two-region shell (`nav | main`) is preserved; no contextual panel is re-introduced.
6. Existing tests run against the old screen behavior (e.g., `.lg-button` with a different token, `.lg-panel` region). The new screens will still pass those tests because they still render the same visual design (different internal component composition). The selectors may need to update (e.g., `.lg-button` becomes `button.lg-button`), but the test functions use component queries (by test ID) rather than CSS classnames.
7. The existing browser e2e tests are designed with flexible selectors; they should still pass after the refactor (new components are still accessible via their content or test IDs). If they fail, the tasks list includes updates (T012, T018, T027, T034).

## Trade-offs

- **Component vs. modular CSS**: The plan forces us to extract `.lg-button` into a primitive. This is a refactor that reduces duplication but introduces component composition. The tradeoff is the upfront effort vs. future maintainability.
- **Component size vs. abstraction level**: The primitives need to stay small (one semantic role, e.g., `button` for any button) vs. covering many variants/states (need explicit variants/states in the component). We chose small, explicit variants; the F2 spec defines each (primary, secondary, ghost, outline, danger; sm, md; hover, active, focus-visible, disabled, loading).
- **Test vs. runtime difference**: The existing screens test styles by CSS classes; the new screens test styles by token validation. There is a shift in what is tested; we must ensure that both tests run (and both pass) after the refactor.
- **Library extraction vs. code duplication**: Extracting the same styling (`.lg-button`) into a component reduces duplicate CSS but introduces a React layer. The compensation is a central, maintainable primitive for any future button usage.

## Evidence & References

- F2 plan (UIP): `.specs/features/009-web-ui-primitives/spec.md`, `plan.md`, `data-model.md`, contracts.
- F1 plan (UIF): `.specs/features/008-web-ui-foundation/spec.md`, `plan.md`, `data-model.md`, contracts.
- `web/DESIGN.md` §3.8 Component specs: button, field, dialog, toast definitions.
- `web/AGENTS.md`: interaction wiring, security, componentization, no libraries.
- `.specs/STATE.md`: all `AD-*` decisions, most importantly `AD-001` (sheet edit state), `AD-005` (screen open), `AD-019` (design rule change: base component extraction for ≥ 3 real usages).
- F1/2 test specs: `src/test/tokens_used.test.tsx`, `tokens_contrast.test.ts`, `l10n_strings.test.ts`, `design_guidelines.test.ts`.
- Existing screens: `src/catalog/`, `src/sheet/`, `src/auth/`, `src/library/`, `src/shell/`.
- CI scripts: `Makefile`, `package.json`, `vite.config.ts`.
