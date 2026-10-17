# Implementation Plan: Motion, Polish and Visual Regression (F4 / F10)

**Branch**: `011-web-motion-visual-regression` | **Date**: 2026-10-17 | **Spec**: `.specs/features/011-web-motion-visual-regression/spec.md` (prefix `UIM`)

**Summary**: Add a tokenized motion system (`--lg-motion-*`/`--lg-ease-*`), enforce reduced-motion (`prefers-reduced-motion`), polish dark theme with deeper shadows and borders, and establish a Playwright visual regression gate that compares screenshots of core screens in both light/dark themes (reduced-motion off/on) against approved baselines. No new runtime dependencies.

**Primary Dependency**: F1 (`UIF-*`), F2 (`UIP-*`), F3 (`UIS-*`) must be fully delivered, with tokens, primitives, screens, and all scan extensions in place.

**Testing**:
- `tokens_used.test.tsx`: no literal ms; motion tokens resolve.
- `tokens_contrast.test.ts`: dark theme shadows do not break contrast.
- Playwright (`make test-web`): screenshot diff gate passes (no unapproved drift).
- `l10n_strings.test.ts`: unchanged; no new user-facing strings in F4.
- `design_guidelines.test.ts`: unchanged; F4 does not change design rules.
- `self_hosted_assets.test.ts`: unchanged.

## Technical Context

- **Language/Version**: TypeScript 5.9.3 / React 19 / Playwright (dev dependency)
- **Storage**: No storage involved (presentation only)
- **Testing**: Vitest + Playwright (`playwright.config.ts` builds with `VITE_API_BASE_URL` = preview origin)
- **Target Platform**: Web PWA (browser + service worker)
- **Project Type**: Web client (`web/`)
- **Performance Goals**: No performance regression; reduced-motion improves accessibility
- **Constraints**: AA contrast in both themes; motion tokenized; no new runtime dependencies
- **Scale/Scope**: 3 surfaces (motion + dark polish + visual regression gate) + baseline snapshots; no backend contract change

## Constitution Check

- I: Typed errors — unchanged.
- II: Canonical IDs — unchanged.
- III: Behavior ships with tests — all 3 user stories have independent tests (token scan, reduced-motion measurement, baseline diff).
- IV: English-only — unchanged.
- V: Layered architecture — F4 is presentation-layer only (`theme/`, `e2e/`); no `internal/` or `api/` change.
- VI: Security gates blocking — no DOM sinks; no new scripts; Playwright runs locally against preview origin; no production data exposed.
- VII: Installable, offline-capable PWA — unchanged.

## Design Decisions

- **No new runtime dependency**: Playwright is already installed; motion uses plain CSS (`transition`, `animation`) with token durations. Confirmed by `package.json` inspection.
- **Reduced-motion enforcement**: Uses the native `prefers-reduced-motion: media query`, not a runtime flag. The CSS layer handles it.
- **Screenshot baselines**: Committed locally (not pushed) and approved by the maintainer. The gate compares against these baselines; any token change requires updating the baselines intentionally.
- **No screen-level code change for motion**: Motion is declared in the component CSS (`src/ui/*.css` or `base.css`); it does not require a new component.
- **Dark theme shadows deeper per layer**: Confirmed by design doc (§3.6) and implemented via token values (`--lg-shadow-1` / `--lg-shadow-2` / `--lg-shadow-3`).
- **Visual regression covers reduced-motion on and off**: Confirmed by spec (`SC-002` and `SC-005`).

## Implementation Strategy

1. **Motion tokens**: Declare `--lg-motion-*` and `--lg-ease-*` in `tokens.css` / `tokens.ts`.
2. **Reduced-motion media query**: Add `@media (prefers-reduced-motion: reduce)` in `base.css` that clamps durations to near-zero.
3. **Component motion rules**: Update `src/ui/*.css` (if exists) or component styles to use `transition` with the permitted properties and token durations; no literal ms.
4. **Dark theme polish**: Ensure `--lg-shadow-*` values are deeper in dark theme than in light theme; add borders where needed (`--lg-color-border` / `--lg-color-border-strong`).
5. **Visual regression gate**: Confirm Playwright config (`playwright.config.ts`) uses `VITE_API_BASE_URL` preview origin; generate baselines; run `make test-web` and verify no drift.
6. **Commit baselines**: Once baselines are approved, commit them locally.

## Risk and Open Points (from plan §6)

- **Baseline updates**: Any token change requires updating baselines; this must be a deliberate review step.
- **Performance**: Deep shadows and borders add rendering overhead; ensure no visual regression failure is due to performance-related flicker.
- **Playwright version**: Must be compatible with the current Vite build and `node_modules`.
