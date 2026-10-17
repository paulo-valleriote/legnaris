# Quickstart: Motion, Polish and Visual Regression (F4)

## Prerequisites

- F1, F2, and F3 delivered (tokens, theme, motion system, two-region shell, core screens). `make lint-web && make test-web && make test-race` all green.
- Playwright installed and configured (`playwright.config.ts` builds the client with `VITE_API_BASE_URL` pointing at the preview origin).
- Motion tokens (`--lg-motion-*`, `--lg-ease-*`) exist in `src/theme/tokens.css` and are mirrored in `tokens.ts`.

## Run the Baseline

```bash
cd web && npm test          # unit tests, lint, typecheck
make test-web              # Playwright screenshot diff gate
```

All must pass before changes. Record the green result.

## Extend Token Scans

Edit `src/test/tokens_used.test.tsx` to ensure any new `transition:` or `animation:` property lists are scanned for literal ms. This is enforced by the existing scan; no new test needed.

## Verify Motion Rules

- All transitions use only the six permitted properties.
- Reduced-motion is honored (if not enabled, the gate uses standard motion).
- Dark theme shadows and borders separate layers (asserted via snapshot diff).

## Generate Baselines

Once screens render correctly (after any polish changes), generate the Playwright baselines:

```bash
cd web
npx playwright exec --save-snapshots
```

These snapshots become the approved baselines under `web/e2e/baselines/<theme>/<screen>.png`.

## Run the Gate

```bash
make test-web
```

If diffs appear:

- If unintended (code bug), fix and regenerate baselines.
- If intentional (token change), update baselines and commit.

## Commit

```bash
git add .specs/features/011-web-motion-visual-regression/spec.md \
        .specs/features/011-web-motion-visual-regression/plan.md \
        .specs/features/011-web-motion-visual-regression/research.md \
        .specs/features/011-web-motion-visual-regression/data-model.md \
        .specs/features/011-web-motion-visual-regression/quickstart.md \
        .specs/features/011-web-motion-visual-regression/contracts/ \
        .specs/features/011-web-motion-visual-regression/checklists/requirements.md \
        .specify/feature.json
git commit -q -m "docs(specs): polish motion system + visual regression gate (F4)"
```
