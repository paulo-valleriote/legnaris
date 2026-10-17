# Contract: Parity Gate (F10 / POL-01 / POL-07)

- Playwright golden comparison runs before any desktop file removal.
- Criteria: screenshot comparison of sheet/template/catalog/shell screens against desktop reference; perceptual threshold must not exceed configured value; no missing UI elements vs desktop.
- If comparison fails: removal blocked; fixes applied; comparison re-run; only after pass may `desktop/` and `ci-flutter.yml` be removed.
- References stored as CI artifacts, not committed to repo.
