# CI

| Workflow | Runs on | Purpose |
|----------|---------|---------|
| `ci-lint.yml` | PR (Go/deploy paths) | golangci-lint, `go mod tidy`, race tests, compose config, shellcheck on deploy and scripts shell files |
| `ci-spec-lint.yml` | PR only | Spec/test lint rules; rule 3 needs the PR base to scope its diff |
| `ci-web.yml` | PR + push to `main` (`web/**`) | Client quality, SCA/SAST, e2e, DAST, Lighthouse, SBOM |
| `ci-images.yml` | PR (build + scan), push to `main`, `v*` tags (publish) | Builds, Trivy-scans and publishes images to GHCR |
| `cd-vps.yml` | After `Images` completes on `main`, or manual dispatch with a SHA | Deploys one commit to the VPS over SSH (`deploy/vps/deploy.sh`) |

`opencode-review.yml` stays in `workflows-disabled/`.

## Images

Published as `ghcr.io/<owner>/legnaris-app`, `legnaris-web` and `legnaris-rabbitmq` (broker with the Prometheus plugin and provisioning).

- **Immutable tag:** `sha-<full commit SHA>` — the deploy pins this tag.
- **Version tag:** `X.Y.Z` on `vX.Y.Z` tags.
- A CRITICAL/HIGH fixable vulnerability fails the job before anything is pushed.

### One image, four binaries (decision)

`legnaris-app` carries `/app` (API), `/relay`, `/worker` and
`/canonicalize-emails`; the API is the default entrypoint. Relay and worker run
from the same image by overriding the command (`entrypoint: ["/relay"]` in
compose). This keeps one build, one scan and one
version for components that share code and must stay in lockstep. Split them
only if their dependencies or release cadence diverge.

## Secrets

None are stored in the repository. The pipeline uses only the automatic
`GITHUB_TOKEN` (`packages: write` on the image job, push events only). Other
jobs use the secrets documented in their own workflow headers; no extra
repository secret is required for `ci-images.yml`.
