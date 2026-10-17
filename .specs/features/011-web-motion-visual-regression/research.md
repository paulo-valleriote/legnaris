# Research & Analysis for Motion, Polish and Visual Regression

## Existing State (F1/F2/F3)

- F1 delivered: two-theme token system, theme toggle (`localStorage` persistence), two-region shell (`nav | content`), self-hosted Geist font.
- F2 delivered: `src/ui/` primitives (button, field, checkbox, dialog, toast) with token-only styles; scan extensions (`tokens_used`, `l10n_strings`) covering primitive directory.
- F3 delivered: catalog, sheet, auth, your-sheet/not-found/update screens refactored onto F1 tokens + F2 primitives; literal scan extended to all screen directories.

## Motion System

The motion system in `web/DESIGN.md` §3.7 defines:
- Durations: `--lg-motion-120` (120 ms), `--lg-motion-180` (180 ms), `--lg-motion-240` (240 ms).
- Easings: `--lg-ease-standard`, `--lg-ease-emphasized`.
- Only these properties may transition: `background`, `color`, `border-color`, `box-shadow`, `transform`, `opacity`.
- `prefers-reduced-motion: reduce` zeros non-essential animations.

## Dark Theme Polish

Plan §6 notes dark polish: deeper shadows + borders separate layers. The layered model (`bg → surface → raised → overlay`) ensures no more than two levels visible. Shadow tokens will be adjusted per theme to pass the contrast assertion.

## Visual Regression Gate

Playwright is already installed (from F3/F7). `playwright.config.ts` builds the client with `VITE_API_BASE_URL` set to the preview origin, so e2e runs locally with stubbed responses. Screenshots are compared against baselines per screen/theme/motion state.

## Integration Points

- `src/theme/`: motion tokens live here; reduced-motion media query will go into `base.css`.
- `src/ui/*`: primitives may use motion tokens in their CSS (if they declare transitions). No primitive introduces its own literal ms.
- `web/e2e/`: visual regression baselines live under `web/e2e/baselines/<theme>/<screen>.png`; `make test-web` runs the diff gate.
- `src/test/`: existing test suites (`tokens_used`, `tokens_contrast`, `l10n_strings`, `design_guidelines`) must remain green; no new test files needed unless a new scan is added (motion tokens are already scanned).

## Assumptions

- The motion-token names and values are the starting point; the `tokens_contrast.test.ts` assertion will adjust them as needed to pass AA.
- The dark theme will need adjusted shadow values and borders to pass contrast; these are declared in `tokens.css` per theme.
- Visual regression baselines are generated after screens render correctly (post-polish); they are committed and updated only on deliberate visual change.
- No new runtime dependencies: Playwright already present; motion uses CSS transitions; no extra library needed.
- The reduced-motion media query will not affect essential layout transitions (none exist); it only targets animated properties.