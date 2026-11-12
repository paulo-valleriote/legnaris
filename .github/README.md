# CI

| Workflow | Runs on | Purpose |
|----------|---------|---------|
| `ci.yml` | Every PR | A `changes` job picks the suites the diff needs: Go (golangci-lint, `go mod tidy`, race tests, compose config, shellcheck), web (format, lint, typecheck, tests, build, Playwright e2e) and an image build without push. The final `gate` job is the only required check |
| `ci-images.yml` | push to `main`, `v*` tags | Builds and publishes images to GHCR |
| `cd-vps.yml` | After `Images` completes on `main`, or manual dispatch with a SHA | Deploys one commit to the VPS over SSH (`deploy/vps/deploy.sh`) |

## Required check

Branch protection on `main` requires only `gate`. It always runs and fails when
any suite that ran failed or was cancelled; suites skipped by the path filter
count as passed. Requiring the individual suites instead would leave a PR that
skips one stuck on "pending".

## Visual regression (removed)

The Playwright screenshot comparisons (`toHaveScreenshot`, POL-01 parity gate)
were removed: the UI changes in most pull requests, so the committed baselines
failed on every visual change and on anti-aliasing noise between machines,
without ever catching a real regression. Accessibility, responsive-layout and
behavior specs remain; visual review happens by hand.

## Local hook

`make setup` points `core.hooksPath` at `.githooks/`, whose `pre-commit`
formats the staged Go (`gofmt`) and web (Prettier) files. CI keeps the format
checks as the safety net.

The spec lint scripts under `scripts/` (`check_*.py`) no longer run in CI. `opencode-review.yml` stays in `workflows-disabled/`.

## Images

Published as `ghcr.io/<owner>/legnaris-app`, `legnaris-web` and `legnaris-rabbitmq` (broker with the Prometheus plugin and provisioning).

- **Immutable tag:** `sha-<full commit SHA>` — the deploy pins this tag.
- **Version tag:** `X.Y.Z` on `vX.Y.Z` tags.

### One image, four binaries (decision)

`legnaris-app` carries `/app` (API), `/relay`, `/worker` and
`/canonicalize-emails`; the API is the default entrypoint. Relay and worker run
from the same image by overriding the command (`entrypoint: ["/relay"]` in
compose). This keeps one build and one
version for components that share code and must stay in lockstep. Split them
only if their dependencies or release cadence diverge.

## Secrets

None are stored in the repository. The pipeline uses only the automatic
`GITHUB_TOKEN` (`packages: write` on the image job, push events only). Other
jobs use the secrets documented in their own workflow headers; no extra
repository secret is required for `ci-images.yml`.
