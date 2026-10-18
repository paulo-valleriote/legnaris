# Contract: Parity Gate (F10 / POL-01 / POL-07)

- Playwright golden comparison runs before any desktop file removal.
- Criteria: screenshot comparison of catalog/sheet/template/shell screens; perceptual threshold
  must not exceed the configured value; no missing UI elements against the reference.
- If comparison fails: removal blocked; fixes applied; comparison re-run; only after pass may
  `desktop/` and `ci-flutter.yml` be removed.
- **References are committed, not CI artifacts** (delivered decision, superseding the original
  "stored as CI artifacts, not committed to repo" line). The baselines live under
  `web/e2e/baselines/` and are resolved by `snapshotPathTemplate` in `web/playwright.config.ts`
  (`{testDir}/baselines/{arg}-{projectName}-{platform}{ext}`). Committing a small, stable set is
  what makes `toHaveScreenshot` a real gate: an artifact-only baseline that is missing makes the
  assertion pass vacuously, so a layout change would go undetected.
- Updating a baseline requires a deliberate `--update-snapshots` run reviewed in the diff, never a
  silent overwrite. The reference set is kept small on purpose (catalog, sheet, template, shell).
- **Status**: `desktop/` and `ci-flutter.yml` are removed; the committed baseline set is the
  evidence the gate passed.
