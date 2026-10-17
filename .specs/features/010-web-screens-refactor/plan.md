# Implementation Plan: Web Screens Refactor (F3)

**Branch**: `010-web-screens-refactor` | **Date**: 2026-10-17 | **Spec**: `.specs/features/010-web-screens-refactor/spec.md` (prefix `UIS`)

**Summary**: Refactor the catalog, sheet, auth, your-sheet, not-found, and update-prompt screens onto the F1 token system (`src/theme/tokens.css`, `tokens.ts`, `base.css`, `theme/init.js`) and the F2 component primitives (`src/ui/`: button, field, checkbox, dialog, toast). Every visual value resolves through `--lg-*` tokens; no literal colors or pixels in component code. The two-region shell (nav | content) stays intact; the self-hosted Geist font continues to load; the theme toggle and boot script remain unchanged.

**Primary Dependency**: The F1 feature (`008-web-ui-foundation`, `UIF-*`) must be fully delivered before F3 starts; the F2 feature (`009-web-ui-primitives`, `UIP-*`) delivers the primitives (`button`, `field`, `dialog`, `toast`). F3 consumes both but does not modify them.

**Testing**: Unit and rendering tests must cover:
- Token resolution (`tokens_used.test.tsx`) for all refactored directories (`src/catalog/`, `src/sheet/`, `src/auth/`, `src/library/`)
- Contrast (`tokens_contrast.test.ts`) for both themes across all refactored screens
- Component-level tests for the F2 primitives, if not covered by F2
- Per-screen functional tests (happy path + edge cases) for loading, error, validation, dirty tracking, and navigation
- `l10n_strings.test.ts` must cover any new or changed user-facing strings
- `design_guidelines.test.ts` must pass (DESIGN.md sections unchanged, AGENTS.md updated separately for the `src/ui/` rule change)
- `self_hosted_assets.test.ts` continues to assert the Geist font is declared, preloaded, and same-origin

## Technical Context

- **Language/Version**: TypeScript 5.9.3 / React 19 (from `package-lock.json`)
- **Primary Dependencies**: React, Vite; no new runtime dependencies (tokens + plain CSS only; F2 primitives are plain React components)
- **Storage**: No persistence layer changed; the F3 refactor touches only presentation
- **Testing**: Existing test suite extended; `test-race` clean; `lint-specs` clean; `lint-web` + `test-web` green
- **Target Platform**: Web PWA (browser + service worker)
- **Project Type**: Web client (`web/`)
- **Performance Goals**: No new runtime dependencies; no performance regression vs. F1/F2; reduced-motion honored
- **Constraints**: AA contrast in both themes; token-only styling; `src/ui/` primitives consumed; `tokens_used` scan covers new directories
- **Scale/Scope**: 5 screen surfaces (catalog, sheet, auth, your-sheet/not-found/update-prompt) + scan extensions; no backend contract changes

## Constitution Check

- I (Typed errors): Not changed; F3 uses the existing typed error contracts (`apperr`) for API failures shown in error states.
- II (Canonical IDs): Unchanged; no new server-generated entities.
- III (Behavior ships with tests): Every screen change ships with its rendering and behavior tests; edge cases first then happy path.
- IV (English-only): All user-facing strings resolve through `src/l10n/`.
- V (Layered architecture): F3 is a presentation-layer change only (`catalog/`, `sheet/`, `auth/`, `library/`, `shell/`); does not touch `internal/` backend or `src/api/` contract layers.
- VI (Security gates blocking): No DOM sinks (`dangerouslySetInnerHTML` forbidden); self-hosted font continues to pass CSP; no third-party scripts; `localStorage` theme preference unchanged.
- VII (Installable, offline-capable PWA): Shell unaltered; no service-worker contract change; theme boot script preserved.

## Design Decisions

- **No new runtime dependencies**: Confirmed by plan (§6, F3 scope) and the F3 spec (`UIP-29`). The F2 primitives are plain React components using token CSS; no UI library is added.
- **`src/ui/` primitives consumption**: The F2 primitives are consumed directly by each screen's JSX. There is no wrapper layer; the existing screen components import from `src/ui/`.
- **Scan extensions**: `tokens_used` and `l10n_strings` must cover `src/catalog/`, `src/sheet/`, `src/auth/`, `src/library/`, and `src/ui/`. This aligns with the F1 decision (`UIF-29`) to extend the scan.
- **No base-layer prohibition removal in AGENTS.md (implementation task)**: The spec (`UIP-30` related, `AD-*` in `.specs/STATE.md`) records the rule change as a decision (`AD-19`), but the actual text edit to `web/AGENTS.md` is an implementation task, not part of this artifact cycle.
- **Two-region shell preserved**: Confirmed by F1 contract (`UIF-01`) and design guidelines (§3.15 Anti-pattern #193: no dead region); no contextual panel is reintroduced.
- **Theme boot script preserved**: Confirmed by F1 (`UIF-20`, `public/theme-init.js`); F3 does not alter theme resolution.

## Implementation Strategy

1. Catalog screen: Replace all component-level styling with `--lg-*` tokens; consume F2 `button` (primary) and `dialog` (create-sheet); extend token/copy scans.
2. Sheet screen: Replace form fields with F2 `field` primitive (input/select/textarea); replace footer button with F2 `button`; add inline validation styling with tokens; extend scans.
3. Auth screen: Replace mode switch with F2 `switch`; replace fields with F2 `field`; replace submit button with F2 `button`; extend scans.
4. Your-sheet / not-found / update-prompt: Replace message styling with tokens; add F2 `button` for actions; extend scans.
5. Scan updates: Extend `tokens_used.test.tsx` and `l10n_strings.test.ts` to cover new directories.
6. Final gates: `make lint-web && make test-web && make test-race`; `make lint-specs`.

## Risk and Open Points (from plan §6)

- **Source scan coverage**: If the literal scan misses a new directory, a hidden literal could enter a component.
- **Theme boot script interaction**: Changing the shell or theme module could break the F1 no-FOUC guarantee; F3 avoids touching theme logic.
- **F2 primitive maturity**: If an F2 primitive is missing a variant the screen needs, the screen must either extend the primitive (not permitted — primitives own their contract) or add a component token (`--lg-color-*`). The design document allows component tokens only when semantics do not cover a need (§3.2).
