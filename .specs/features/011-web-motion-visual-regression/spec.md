# Feature Specification: Motion, Polish and Visual Regression

**Feature Branch**: `011-web-motion-visual-regression`

**Created**: 2026-10-17

**Status**: Draft

**Input**: User description: Polish, motion and Playwright visual regression for the UI redesign

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Motion system with reduced-motion honors (Priority: P1)

A person navigates the app and every motion (hover tint, state change, dialog open, toast appear) is subtle, bounded to 240 ms, and uses only transition-safe properties. When the person has `prefers-reduced-motion: reduce` enabled, all non-essential animations are clamped to near-zero duration. Every duration and easing comes from `--lg-motion-*` and `--lg-ease-*` tokens; nothing in a component invents its own millisecond value.

**Why this priority**: Motion is one of the three remaining polish surfaces (alongside dark-theme shadows/border and visual regression). A tokenized, reduced-motion-safe motion system is a prerequisite for visual regression screenshots to be deterministic.

**Independent Test**: Read the token source (`tokens.css`, `tokens.ts`) and assert every motion duration resolves to a token; render a screen in light and dark with reduced-motion enabled and assert transitions are near-zero; run the both-theme contrast assertion.

**Acceptance Scenarios**:

1. **Given** the token source, **When** it is inspected, **Then** motion durations are declared as `--lg-motion-120/180/240` and easings as `--lg-ease-standard`/`--lg-ease-emphasized`.
2. **Given** a component with a hover state, **When** reduced-motion is not set, **Then** only `background`, `color`, `border-color`, `box-shadow`, `transform`, `opacity` transition, each over a declared duration.
3. **Given** `prefers-reduced-motion: reduce`, **When** a motion plays, **Then** its duration is clamped to near-zero and scroll behaviour becomes instant.

### User Story 2 - Dark theme polish: deeper shadows and borders separate layers (Priority: P1)

A person switches to the dark theme and sees layers separated by deeper shadows (`shadow-2` popovers, `shadow-3` modals) plus borders between levels; the layered model `bg → surface → raised → overlay` holds with no more than two levels visible at once. Every shadow and border value comes from tokens and passes AA contrast.

**Why this priority**: Dark theme is the second color mode delivered by F1; its polish (deeper shadows, borders) completes the contract and prevents two-level surfaces from looking flat.

**Independent Test**: Render the app in dark theme and assert shadow tokens resolve per-layer; assert border-strong separates layers; assert contrast passes for both themes.

**Acceptance Scenarios**:

1. **Given** the dark theme is applied, **When** a raised card is rendered, **Then** it uses `--lg-shadow-1`.
2. **Given** the dark theme is applied, **When** a popover is rendered, **Then** it uses `--lg-shadow-2` plus a border.
3. **Given** the dark theme is applied, **When** a modal is rendered, **Then** it uses `--lg-shadow-3`.
4. **Given** both themes, **When** contrast is asserted, **Then** every foreground/background pair meets its minimum per theme.

### User Story 3 - Visual regression gate on core screens (Priority: P2)

A maintainer merges a change to `web/` and the Playwright e2e suite renders the catalog, sheet, auth, and state screens in both themes and compares screenshots against approved baselines; any unintended visual drift fails the gate. Screenshots are taken at reduced-motion off (default) and on to cover both motion states.

**Why this priority**: After F1/F2/F3, the visual surface is stable; a regression gate prevents unintended changes from shipping without review. It covers both themes and the motion toggles.

**Independent Test**: Run Playwright against the built client in light and dark and assert screenshot diffs stay within tolerance for every core screen; assert failures are attributable to intentional changes (approved baseline update) or bugs.

**Acceptance Scenarios**:

1. **Given** the built client is running, **When** Playwright renders each core screen in light and dark, **Then** screenshots match approved baselines within tolerance.
2. **Given** reduced-motion is on, **When** screenshots are taken, **Then** animations are near-zero and baselines remain stable.
3. **Given** a maintainer changes a token, **When** visual regression runs, **Then** drift is detected on all screens consuming that token (intentional change requires baseline update).
4. **Given** no drift exists, **When** visual regression runs, **Then** the gate passes (`make test-web` green).

### Edge Cases

- What happens when a component uses a non-transitionable property (e.g. `width` or `height`)? → The transitioning set is restricted; only six properties may transition; anything else fails lint/code-review.
- How does the system handle a token whose duration is invalid? → `tokens_contrast.test.ts` resolves every declared token; an invalid duration breaks the assertion.
- What happens when a screenshot baseline is missing? → Playwright fails on the missing baseline; the maintainer must approve and commit a new baseline.
- How do light and dark baselines interact? → Each screen is baselined per theme; a dark-only token change only affects dark baselines.

## Requirements *(mandatory)*

### Functional Requirements

- **UIM-01**: Motion durations MUST be declared only as `--lg-motion-120`, `--lg-motion-180`, `--lg-motion-240` in `tokens.css` and mirrored in `tokens.ts` (`motionTokens`).
- **UIM-02**: Motion easings MUST be declared only as `--lg-ease-standard` and `--lg-ease-emphasized` in `tokens.css` and mirrored in `tokens.ts`.
- **UIM-03**: Components MAY transition ONLY `background`, `color`, `border-color`, `box-shadow`, `transform`, `opacity`; no other property MAY appear in a `transition` declaration.
- **UIM-04**: Under `prefers-reduced-motion: reduce`, ALL non-essential transitions/animations MUST be clamped to near-zero duration, and scroll behaviour MUST become instant.
- **UIM-05**: Every component declaring a hover state MUST change background/tint only — never layout, size or position.
- **UIM-06**: Dark theme shadows MUST be deeper than light theme shadows per layer (`shadow-1` rows/cards, `shadow-2` popovers, `shadow-3` modals).
- **UIM-07**: Dark theme MUST add borders between layers where light uses shadow alone; no more than two levels visible at once (`bg → surface → raised → overlay`).
- **UIM-08**: A tooltip/component implementing motion MUST derive all durations/easings from tokens; no literal ms value MAY appear outside `src/theme/`.
- **UIM-09**: Visual regression baselines MUST exist for catalog, sheet, auth, your-sheet/not-found/update-prompt screens in both light and dark themes.
- **UIM-10**: Visual regression MUST run under reduced-motion off (default) and reduced-motion on.
- **UIM-11**: Visual regression gate MUST fail on any screenshot drift outside tolerance; an intentional token/visual change requires an approved baseline update.
- **UIM-12**: The e2e client MUST be built with `VITE_API_BASE_URL` set to the preview origin so screenshots run locally against stubbed responses (inherited from F3/F7).
- **UIM-13**: No new runtime dependencies MAY be introduced for motion or visual regression (Playwright already present; tokens + plain CSS only).
- **UIM-14**: Existing `tokens_used.test.tsx`, `tokens_contrast.test.ts`, `l10n_strings.test.ts`, `design_guidelines.test.ts` MUST remain green after motion/polish changes.
- **UIM-15**: `web/AGENTS.md` MUST restate no visual value; motion rules live only in `web/DESIGN.md` and `src/theme/tokens.css` (UIF-33 contract).
- **UIM-16**: Focus rings (`--lg-focus-*`) MUST remain visible in both themes after polish; a focused control MUST be distinguishable from a hovered one.

### Key Entities *(include if feature involves data)*

- **Motion Token**: a duration or easing declared in `tokens.css` and mirrored in `tokens.ts` (`.motionTokens` / `.motion`).
- **Screenshot Baseline**: an approved PNG comparison artifact per screen per theme per motion state, committed under `web/e2e/baselines/`.
- **Visual Regression Gate**: the Playwright suite + `make test-web` step that diffs screenshots against baselines.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of motion durations/easings resolve to `--lg-motion-*`/`--lg-ease-*` tokens (zero literal ms in components).
- **SC-002**: Under reduced-motion, 100% of non-essential transitions/clamped animations are near-zero duration (measured per screen in Playwright).
- **SC-003**: Only the six permitted properties transition in any component (measured by lint/source scan).
- **SC-004**: Dark theme shadows are deeper than light per layer, and borders separate layers (asserted per screen in both themes).
- **SC-005**: Visual regression gate passes on catalog, sheet, auth, states in light+dark with no unapproved drift.
- **SC-006**: No new runtime dependencies added for motion/visual regression (`package.json` unchanged apart from already-resolved deps).
- **SC-007**: All existing token/contrast/copy/design-guidelines suites stay green.

## Assumptions

- Playwright is already installed and configured (`playwright.config.ts` exists; `npm test` runs Vitest unit tests; `make test-web` runs Playwright e2e).
- The F1 tokens and F2 primitives are stable and delivered (dependencies: `008-web-ui-foundation`, `009-web-ui-primitives`, `010-web-screens-refactor`).
- The e2e client builds with `VITE_API_BASE_URL` pointing at the preview origin (inherited from F3/F7); no network reaches production API during screenshots.
- The `--lg-focus-*` tokens already exist from F1 and are consumed by focus-ring styling; F4 does not add new focus tokens.
- Visual regression baselines must be generated once (locally) after F4 screens render, then committed; they are not regenerated automatically on every run (drift detection only).
- No new runtime dependency is needed: Playwright is already a dev dependency of `web/`; motion uses plain CSS transitions.