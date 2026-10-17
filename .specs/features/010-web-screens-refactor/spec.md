# Feature Specification: Web Screens Refactor

**Feature Branch**: `010-web-screens-refactor`

**Created**: 2026-10-17

**Status**: Draft

**Input**: User description: Refactor web screens onto the F1 tokens and F2 primitives: catalog, sheet, auth, your-sheet, not-found/update

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Refactor the catalog screen to use F1 tokens and F2 component primitives (Priority: P1)

A person opens the catalog and sees a scannable list of templates with a clear primary action (Create Sheet) and minimal visual noise. Every color, spacing, typography, and elevation value comes from the F1 tokens, and every interactive control (button, input, etc.) is built from the F2 primitives. The refactored screen maintains the same functional behavior: list templates, show loading/error states, open the create-sheet dialog, and navigate to a template detail.

**Why this priority**: The catalog is a high-traffic screen used by every user to start work. Refactoring it validates the F1/F2 contract and provides immediate visual and usability improvements while keeping the feature surface small enough to spec and test quickly.

**Independent Test**: Render the catalog screen in isolation with mocked data and assert that:
- All visual values (colors, spacing, typography, elevation) resolve to `--lg-*` tokens from `src/theme/tokens.css`
- No literal color (`#...`), pixel (`\d+px`), or font-value appears in the rendered DOM (except in `src/theme/`)
- The screen mounts, shows loading state, then displays a list of template cards with a primary button
- The create-sheet dialog opens and closes correctly, using the F2 dialog primitive
- The contrast assertion passes for both light and dark themes
- Keyboard navigation follows visual order and focus rings are visible

**Acceptance Scenarios**:

1. **Given** mocked template data, **When** the catalog screen mounts, **Then** it shows a loading indicator until data arrives.
2. **Given** successful template data, **When** the catalog screen renders, **Then** it displays a scannable list of template cards (name + metadata) with a clear primary "Create Sheet" button.
3. **Given** an error loading templates, **When** the catalog screen renders, **Then** it shows an inline error message with a retry button.
4. **Given** the catalog screen is displayed, **When** the user clicks the "Create Sheet" button, **Then** the create-sheet dialog opens centered over the content.
5. **Given** the create-sheet dialog is open, **When** the user clicks cancel or presses Esc, **Then** the dialog closes and focus returns to the trigger button.

### User Story 2 - Refactor the sheet (ficha) screen to use F1 tokens and F2 component primitives (Priority: P1)

A person opens a sheet and sees a detail/form layout with a header showing the sheet's identity, labelled fields using the F2 field primitive, a sticky footer showing dirty state and the primary Save action, and inline validation errors. Every visual value comes from the F1 tokens and every interactive control is an F2 primitive. The refactored screen maintains the same functional behavior: load sheet data, edit fields, show dirty state, validate input, save changes, and handle loading/error states.

**Why this priority**: The sheet screen is the core authoring experience. Refactoring it demonstrates that the F1/F2 contract works for a complex form with validation, dirty tracking, and sticky footer, while delivering immediate usability and accessibility improvements.

**Independent Test**: Render the sheet screen in isolation with mocked data and assert that:
- All visual values resolve to `--lg-*` tokens from `src/theme/tokens.css`
- No literal color, pixel, or font-value appears in the rendered DOM (except in `src/theme/`)
- The header shows the sheet's identity (name/template)
- Fields are rendered as labelled controls using the F2 field primitive (input/select/textarea/checkbox/radio/switch)
- The sticky footer shows dirty state when fields are edited and hides it when clean
- The primary Save button is disabled when the form is invalid or clean
- Inline validation errors appear under fields with invalid data
- The contrast assertion passes for both light and dark themes
- Keyboard navigation follows visual order (header → fields → footer actions) and focus rings are visible

**Acceptance Scenarios**:

1. **Given** mocked sheet data, **When** the sheet screen mounts, **Then** it shows a loading indicator until data arrives.
2. **Given** successful sheet data, **When** the sheet screen renders, **Then** it displays a header with the sheet name and template identity, followed by labelled fields for each editable schema leaf.
3. **Given** the user edits a field to an invalid value, **When** the field loses focus, **Then** an inline error appears under that field.
4. **Given** the user edits one or more fields, **When** the changes are validated, **Then** the dirty state appears in the sticky footer and the Save button enables.
5. **Given** the form is valid and dirty, **When** the user clicks Save, **Then** a save request is sent with the editable delta and the button shows a loading state.
6. **Given** the save succeeds, **When** the response is processed, **Then** the dirty state clears and the Save button disables.
7. **Given** the save fails, **When** the error is received, **Then** an inline error appears at the form level with a retry option.

### User Story 3 - Refactor the auth screen to use F1 tokens and F2 component primitives (Priority: P1)

A person opens the auth screen and sees a centered panel focused on the single active mode (sign in or sign up), with labelled fields using the F2 field primitive, clear inline errors, and the mode switch using the F2 switch primitive. Every visual value comes from the F1 tokens and every interactive control is an F2 primitive. The refactored screen maintains the same functional behavior: toggle between sign in/sign up, handle form submission, show loading/error states, and process successful authentication.

**Why this priority**: The auth screen is the gateway to the application. Refactoring it ensures the entry point uses the new design system contract while improving clarity, accessibility, and visual consistency with the rest of the app.

**Independent Test**: Render the auth screen in isolation and assert that:
- All visual values resolve to `--lg-*` tokens from `src/theme/tokens.css`
- No literal color, pixel, or font-value appears in the rendered DOM (except in `src/theme/`)
- The screen is centered in the viewport with appropriate padding
- The mode switch toggles between sign in and sign up, updating the header and button text accordingly
- Fields are rendered as labelled controls using the F2 field primitive
- Inline validation errors appear under fields with invalid data
- The submit button is disabled when the form is invalid
- The contrast assertion passes for both light and dark themes
- Keyboard navigation follows visual order and focus rings are visible

**Acceptance Scenarios**:

1. **Given** the auth screen loads in sign-in mode, **When** it renders, **Then** it shows a centered panel with "Sign in" header, email/password fields, and a "Sign in" button.
2. **Given** the user switches to sign-up mode, **When** the mode switch toggles, **Then** the header changes to "Sign up", the password field label updates to "Choose password", and the button text becomes "Create account".
3. **Given** the user enters an invalid email, **When** the email field loses focus, **Then** an inline error appears under the email field.
4. **Given** the user enters a valid email but short password, **When** the password field loses focus, **Then** an inline error appears under the password field.
5. **Given** the form is valid, **When** the user clicks the submit button, **Then** the button shows a loading state and disables.
6. **Given** the authentication succeeds, **When** the response is processed, **Then** the user is redirected to the sheet screen or shown a success state.
7. **Given** the authentication fails, **When** the error is received, **Then** an inline error appears at the form level with the mapped message.

### User Story 4 - Refactor the your-sheet, not-found, and update-prompt screens to use F1 tokens and F2 component primitives (Priority: P2)

A person encounters transient states (your-sheet empty, not-found, or update available) and sees a designed empty state: one line of plain text and, where there is an action, one control. Every visual value comes from the F1 tokens and the action control is an F2 primitive. The refactored screens maintain the same functional behavior: show the appropriate message and provide the available action.

**Why this priority**: These screens are low-complexity but high-visibility edge cases. Refactoring them completes the visual consistency sweep and ensures even temporary states follow the design system, improving perceived quality and accessibility.

**Independent Test**: Render each screen in isolation and assert that:
- All visual values resolve to `--lg-*` tokens from `src/theme/tokens.css`
- No literal color, pixel, or font-value appears in the rendered DOM (except in `src/theme/`)
- The screen displays one line of plain text (the message)
- Where an action exists (your-sheet: "New sheet"; not-found: "Go to catalog"; update-prompt: "Update now"), a single F2 primitive button is presented
- Where no action exists (your-sheet loading, not-found with no alternative), only the message is shown
- The contrast assertion passes for both light and dark themes
- Keyboard focus is correctly managed (if a button exists, it receives initial focus; focus rings are visible)

**Acceptance Scenarios**:

1. **Given** the your-sheet screen loads with no existing sheet, **When** it renders, **Then** it shows the message "No sheet created yet" and a primary button labelled "New sheet".
2. **Given** the not-found screen loads for a deleted sheet ID, **When** it renders, **Then** it shows the message "Sheet not found" and a primary button labelled "Go to catalog".
3. **Given** the update-prompt screen loads when a newer version is available, **When** it renders, **Then** it shows the message "A new version is available" and a primary button labelled "Update now".
4. **Given** the your-sheet screen loads while a sheet is opening, **When** it renders, **Then** it shows only the message "Loading your sheet..." with no action button.
5. **Given** the user clicks the action button on any of these screens, **When** the button processes the click, **Then** the corresponding navigation or action is triggered.

### Edge Cases

- What happens when the theme changes while a screen is mounted? → The screen updates all token-derived values and the contrast assertion continues to pass.
- How does the system handle missing token values? → The build fails (`tokens_used.test.tsx` catches unresolved tokens) and the developer must declare the missing token in both themes.
- What happens when a screen attempts to use a literal color or pixel value? → The literal scan (`tokens_used.test.tsx`) fails and the developer must replace it with a token.
- How does the system handle reduced motion preferences? → Transitions and animations are clamped to near-zero duration, and the screen remains usable.

## Requirements *(mandatory)*

### Functional Requirements

- **UIP-01**: The catalog screen MUST derive all visual values (colors, spacing, typography, elevation) from F1 tokens in `src/theme/tokens.css`.
- **UIP-02**: The catalog screen MUST use F2 component primitives for all interactive controls (button, dialog).
- **UIP-03**: The catalog screen MUST contain no literal color (`#...`), pixel (`\d+px`), or font-value in its source code or rendered DOM (except in `src/theme/`).
- **UIP-04**: The catalog screen MUST pass the contrast assertion for both light and dark themes.
- **UIP-05**: The catalog screen MUST maintain the same functional behavior: loading state, template list, error state, create-sheet dialog invocation.
- **UIP-06**: The sheet screen MUST derive all visual values from F1 tokens in `src/theme/tokens.css`.
- **UIP-07**: The sheet screen MUST use F2 component primitives for all interactive controls (field, button, checkbox, radio, switch).
- **UIP-08**: The sheet screen MUST contain no literal color, pixel, or font-value in its source code or rendered DOM (except in `src/theme/`).
- **UIP-09**: The sheet screen MUST pass the contrast assertion for both light and dark themes.
- **UIP-10**: The sheet screen MUST maintain the same functional behavior: header with identity, labelled fields, sticky footer with dirty state and primary action, inline validation, loading/error states.
- **UIP-11**: The auth screen MUST derive all visual values from F1 tokens in `src/theme/tokens.css`.
- **UIP-12**: The auth screen MUST use F2 component primitives for all interactive controls (field, button, switch).
- **UIP-13**: The auth screen MUST contain no literal color, pixel, or font-value in its source code or rendered DOM (except in `src/theme/`).
- **UIP-14**: The auth screen MUST pass the contrast assertion for both light and dark themes.
- **UIP-15**: The auth screen MUST maintain the same functional behavior: mode switch, labelled fields, inline validation, loading/error states, successful authentication redirect.
- **UIP-16**: The your-sheet, not-found, and update-prompt screens MUST derive all visual values from F1 tokens in `src/theme/tokens.css`.
- **UIP-17**: The your-sheet, not-found, and update-prompt screens MUST use an F2 component primitive for any action button.
- **UIP-18**: The your-sheet, not-found, and update-prompt screens MUST contain no literal color, pixel, or font-value in their source code or rendered DOM (except in `src/theme/`).
- **UIP-19**: The your-sheet, not-found, and update-prompt screens MUST pass the contrast assertion for both light and dark themes.
- **UIP-20**: The your-sheet screen MUST show "No sheet created yet" with a "New sheet" button when no sheet exists.
- **UIP-21**: The not-found screen MUST show "Sheet not found" with a "Go to catalog" button when a sheet ID is not found.
- **UIP-22**: The update-prompt screen MUST show "A new version is available" with an "Update now" button when an update exists.
- **UIP-23**: The source scan in `tokens_used.test.tsx` MUST be extended to include `src/ui/` and the refactored screen directories (`src/catalog/`, `src/sheet/`, `src/auth/`, `src/library/` for your-sheet/not-found/update).
- **UIP-24**: The literal scan in `l10n_strings.test.ts` MUST be extended to include `src/ui/` and the refactored screen directories.
- **UIP-25**: The design guideline assertion in `design_guidelines.test.ts` MUST pass for the refactored screens.
- **UIP-26**: The theme toggle in the shell navigation footer MUST continue to function and persist the theme preference in `localStorage`.
- **UIP-27**: The two-region shell (nav | content) MUST remain intact; no contextual panel is reintroduced.
- **UIP-28**: The self-hosted Geist font MUST continue to be declared, preloaded, and same-origin.
- **UIP-29**: No new runtime dependencies MAY be introduced for the refactor (tokens + plain CSS + React only).
- **UIP-30**: All refactored screens MUST honor `prefers-reduced-motion: reduce` by clamping non-essential transitions to near-zero duration.

### Key Entities *(include if feature involves data)*

- **Template**: Represents a schema for a sheet; key attributes: `id`, `name`, `schema`.
- **Sheet**: Represents an instance of a template filled with data; key attributes: `id`, `template_id`, `name`, `data`, `updated_at`.
- **Auth State**: Represents the authentication status of a user; key attributes: `isAuthenticated`, `user`, `accessToken`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of visual values in the catalog, sheet, auth, your-sheet, not-found, and update-prompt screens resolve to `--lg-*` tokens from `src/theme/tokens.css` (measured by `tokens_used.test.tsx`).
- **SC-002**: 0 literal color (`#...`), pixel (`\d+px`), or font-value appears in the source code or rendered DOM of the refactored screens (except in `src/theme/`).
- **SC-003**: The contrast assertion passes for both light and dark themes in all refactored screens (measured by `tokens_contrast.test.ts`).
- **SC-004**: All refactored screens maintain their pre-refactor functional behavior (loading, error states, navigation, form submission) as measured by their respective test suites.
- **SC-005**: The theme toggle persists the user's choice across page reloads and respects the system preference as the initial default.
- **SC-006**: No new runtime dependencies are introduced in `package.json` for the web client beyond those present in F1/F2.
- **SC-007**: All refactored screens honor `prefers-reduced-motion: reduce` by clamping non-essential transitions and animations to near-zero duration.

## Assumptions

- The F1 tokens (`src/theme/tokens.css`) and F2 primitives (`src/ui/` components) are stable and available as the design system contract.
- The existing test infrastructure (`tokens_used.test.tsx`, `tokens_contrast.test.ts`, `l10n_strings.test.ts`, `design_guidelines.test.ts`) can be extended to cover the new directories without changing its fundamental behavior.
- The sheet screen's field set is limited to input, select, and textarea for the F2 field primitive (checkbox, radio, switch are handled by dedicated F2 primitives but still considered part of the field family for visual consistency).
- The notice/toast component (F2) owns no timer; timing is controlled by the calling screen or service.
- The "no flash of the wrong theme" solution from F1 (same-origin classic boot script in `public/theme-init.js`) continues to work and is not altered by the screen refactor.
- Accessibility guidelines (AA contrast, visible focus, keyboard order) from F1 continue to apply and are verified by the existing test suites.
- The refactor does not change any API contracts or data shapes; it only changes the visual presentation and internal component usage.
- The `web/AGENTS.md` file will be updated separately to remove the "no base component layer" prohibition now that `src/ui/` exists as the sanctioned primitive layer.