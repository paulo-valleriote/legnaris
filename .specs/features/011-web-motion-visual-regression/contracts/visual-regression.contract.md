# Contract: Visual Regression Gate (Playwright)

**Applies to**: `web/e2e/*.spec.ts`, `playwright.config.ts`, screenshot baselines under `web/e2e/baselines/`.
**Requirements**: UIM-09, UIM-10, UIM-11, UIM-12, SC-004, SC-005
**Supersedes**: the prior e2e suites (no change to behavior; this adds the screen-rendering gate).

## The gate

- Screenshots are captured for **each core screen** (`catalog`, `sheet`, `auth`, `your-sheet`, `not-found`, `update-prompt`) in both light and dark themes, with reduced-motion off and on.
- The diff is against approved baselines under `web/e2e/baselines/<theme>/<screen>.png` (baselines committed).
- Tolerance is set conservatively so a deliberate pixel change passes but an unintended drift fails.
- The gate runs as part of `make test-web` (Playwright launch); anything that drifts fails the build.

## Screenshot policy

| Screen | Theme | Motion | What is baselined |
|---|---|---|---|
| catalog | light | off | initial render + 1 card |
| catalog | dark | off | |
| catalog | light | on | reduced-motion snapshot |
| catalog | dark | on | |
| sheet | light | off | |
| sheet | dark | off | |
| sheet | light | on | |
| sheet | dark | on | |
| auth | light | off | |
| auth | dark | off | |
| auth | light | on | |
| auth | dark | on | |
| your-sheet / not-found / update-prompt | light/dark | off | |
| your-sheet / not-found / update-prompt | light/dark | on | |

## Enforcement

| Check | Method |
|---|---|
| Baseline exists per screen/theme | `find web/e2e/baselines -name '*.png'` |
| No unapproved drift | `make test-web` (failures) |
| Intentional change requires approval | baseline update is reviewed and committed |
