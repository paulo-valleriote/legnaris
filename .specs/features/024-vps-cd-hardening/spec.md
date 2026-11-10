# 024 — VPS deploy cleanup and hardened CD

Status: draft, ready for an agent. Triage label to apply once a tracker exists: `ready-for-agent`.

## Problem Statement

Production moved from a local stack exposed through a Cloudflare tunnel to an automated deploy on a Hostinger VPS driven by CI/CD. The migration left debris and weak spots:

- Dead artifacts from the old model (tunnel docs, "local override" comments, an unused entrypoint indirection, a parallel Kubernetes/GitOps stack that nothing applies, one-off setup wizards).
- A CD path that gives the CI key an unrestricted shell on a host where the deploy user is effectively root, deploys any 40-hex SHA without proving it is on `main` or that its images exist, and declares success without checking the public URLs.
- Production configuration (`.env`) that can only be changed by hand-running a wizard, with secrets sitting in plaintext in the home directory and in world-readable files.
- A Docker socket mounted into the edge proxy that does not use it.
- Workflow actions pinned by mutable tags and a binary downloaded without a checksum.

## Solution

Two sequential pull requests.

1. **Cleanup**: remove everything that only made sense for the tunnel, Kubernetes/GitOps or wizard workflows, fix documentation and comments, drop the unused proxy indirection and socket mount, and lock the ops access restriction with a test.
2. **Hardened CD**: restrict what the CI key can do, validate the commit and its images before touching the VPS, verify the public endpoints after deploy, pin actions by commit, manage production configuration as a single GitHub environment secret delivered over the same restricted channel, and provide an idempotent bootstrap that brings the already-running VPS to the hardened baseline without recreating anything.

## User Stories

1. As the maintainer, I want dead tunnel-era files and docs removed, so that the repo describes only how production really runs.
2. As the maintainer, I want the Kubernetes/GitOps stack and its automation removed, so that a push to `main` has one deploy path instead of two.
3. As the maintainer, I want the superseded architecture decision on GitOps marked as superseded rather than deleted, so that the history of the choice stays readable.
4. As the maintainer, I want the one-off setup wizards removed, so that no script embeds a hard-coded PR number or writes secrets to the home directory.
5. As a new contributor, I want the deploy README to describe only the VPS flow, so that I am not sent to Cloudflare or Kubernetes steps that no longer apply.
6. As a new contributor, I want the web and CI READMEs to list the workflows and compose commands that exist, so that I can trust them.
7. As the maintainer, I want the edge proxy to stop receiving a single-valued entrypoint variable, so that routing config has no indirection that cannot vary.
8. As the maintainer, I want the edge proxy to run without the Docker socket mounted, so that a compromise of the proxy cannot drive the Docker API.
9. As the maintainer, I want a test that fails if the proxy ever mounts the Docker socket again, so that the hardening cannot silently regress.
10. As the maintainer, I want a test that fails if the ops OAuth proxy loses its GitHub user allowlist while the wildcard e-mail domain is set, so that Grafana, Prometheus, Mongo Express and the broker console never open to any GitHub account.
11. As the maintainer, I want shell scripts in the deploy and scripts areas linted in CI, so that quoting and safety bugs are caught before they reach the VPS.
12. As the maintainer, I want the Makefile free of dead targets and stray formatting, so that `make help`-style discovery lists only working commands.
13. As the maintainer, I want the CI SSH key limited to a forced command with `restrict`, so that a leaked key can only request a deploy, not open a shell.
14. As the maintainer, I want the forced command to refuse anything that is not a 40-hex commit SHA, so that arguments cannot inject commands.
15. As the maintainer, I want a manual redeploy to be rejected unless the SHA is an ancestor of `main`, so that unreviewed commits cannot reach production.
16. As the maintainer, I want the deploy to be rejected unless the images for that SHA exist in the registry, so that the VPS is never asked to pull something that was not built and scanned.
17. As the maintainer, I want the pipeline to probe the public API health endpoint and the web client after the stack reports healthy, so that a green run means users can actually reach it.
18. As the maintainer, I want the probe to retry a few times with a bounded timeout, so that a slow start does not cause a false failure and a real failure does not hang the run.
19. As the maintainer, I want a failed deploy to still roll back to the previous commit on the VPS, so that availability does not depend on me reacting.
20. As the maintainer, I want every third-party action pinned to a commit SHA with Dependabot proposing bumps, so that a retagged action cannot execute in my pipeline.
21. As the maintainer, I want the vulnerability scanner binary verified against a published checksum, so that a tampered download cannot run in CI.
22. As the maintainer, I want production configuration stored as one environment secret, so that rotating a value is editing a secret and running the deploy.
23. As the maintainer, I want the secret delivered over the restricted SSH channel on standard input, so that it never appears in process arguments or logs.
24. As the maintainer, I want the VPS to write the config file atomically with owner-only permissions, so that a failed transfer never leaves a partial or world-readable file.
25. As the maintainer, I want the deploy to fail when the secret's keys do not match the documented example, so that a missing or misspelled variable is caught before containers restart.
26. As the maintainer, I want `main` protected by required pull requests and no force-push, so that the only gate before production is a reviewed merge.
27. As the operator, I want a bootstrap that is safe to run on the already-running VPS, so that hardening does not recreate users, volumes or containers.
28. As the operator, I want a dry-run mode that prints what the bootstrap would change, so that I can review before applying.
29. As the operator, I want the firewall to allow only SSH, HTTP and HTTPS, and SSH to be allowed before the firewall is enabled, so that I cannot lock myself out.
30. As the operator, I want password and root SSH login disabled only after key login is proven from a second session, so that hardening never costs access.
31. As the operator, I want automatic security updates enabled, so that the host stays patched without manual work.
32. As the operator, I want the old registry PAT revoked and the VPS logged out of the registry, so that no unused credential remains (images are public).
33. As the operator, I want the leftover local wizard environment file deleted once its values live in the secret, so that no plaintext copy of production secrets remains in the home directory.
34. As the operator, I want local `.env` files set to owner-only permissions, so that other local users cannot read secrets.
35. As the maintainer, I want the README to document the Docker socket exposure on the log collector, so that the accepted risk is explicit.

## Implementation Decisions

**Cleanup PR**
- Remove the Kubernetes/GitOps stack: manifests, Argo CD config, the bump workflow, the sealed-secrets script, the local-cluster Make targets and their tests. Keep the architecture decision record, marked superseded. If local-cluster testing is still wanted, keep only the local overlay (open point for the implementer to confirm with the maintainer before deleting it).
- Remove both VPS wizards. No replacement script writes secrets outside the repo's controlled flow.
- Edge proxy: drop the Docker socket volume; remove the single-valued entrypoint environment variable and hard-code the secure entrypoint in every router in the dynamic config. The proxy only uses the file provider, so routing is unaffected; validate by bringing the stack up locally before opening the PR.
- Rewrite the deploy README for the VPS only (drop the Cloudflare DNS migration and Kubernetes sections); fix the web README, CI README, project instructions and stale compose/dynamic-config comments.
- Makefile: remove dead targets and fix the `.PHONY` formatting.
- CI lint: add a shell-script lint step covering the deploy and scripts shell files.

**CD PR**
- Forced command entry script on the VPS: accepts one request type, validates a 40-hex SHA, optionally reads the environment file from standard input, then invokes the existing deploy script. Authorized key line carries the forced command and `restrict`.
- Deploy script: remove the duplicate fetch; keep rollback to the previous commit on failure and the compose `--wait` health gate.
- Workflow: validate SHA format; verify ancestry on `main` (checkout with enough history); verify image manifests exist for the three published images; run deploy over the restricted channel; then probe the public API health endpoint and the web root with bounded retries. Ancestry, image-existence and probe logic live in scripts, not inline YAML, so they are testable.
- Production config: one `production` environment secret holding the whole file; the pipeline compares its keys with the example file's keys and fails on drift; the VPS entry script writes it atomically at mode 600 before `up`.
- Pin all actions by commit SHA (comment the human version); keep Dependabot for the actions ecosystem; verify the scanner download checksum.
- Repository setting: branch protection on `main` (PR required, no force-push). Applied through the platform API or documented as a manual step.
- Bootstrap script: idempotent, supports dry-run; checks state before each action (user, key line, firewall, SSH daemon settings, updates, clone). Allows SSH in the firewall before enabling it; validates the SSH daemon config before reload; refuses to disable password login unless told key login was verified.
- Operator one-offs (not code): revoke the registry PAT, log the VPS out of the registry, delete the leftover wizard env file after migrating values into the secret, set owner-only permissions on local env files. Rotate other secrets only if any value was pasted into chat, a ticket or a log.

**Accepted decisions**
- No manual approval gate on the `production` environment.
- No backup service in this scope.
- Registry packages are public, so the VPS needs no credentials to pull.
- The log collector keeps a read-only Docker socket mount; the risk is documented.

## Testing Decisions

A good test asserts external, observable behavior of a contract (what the rendered stack contains, what a script does with given input and state), not how it is implemented.

Seams (confirmed with the maintainer):

1. **Deploy stack contract** (existing compose/traefik tests, which parse the YAML): assert the proxy has no Docker socket volume; no routing entrypoint variable remains; the OAuth proxy sets the GitHub user allowlist from the env var and the wildcard e-mail domain only appears alongside it; example env keys match the variables the compose file requires. Prior art: the existing compose and traefik test files.
2. **VPS scripts executed as processes** (one new Go test seam, using a fake `PATH` for system tools): the entry script refuses non-40-hex input, injection attempts and missing arguments with a non-zero exit, and accepts a valid SHA; the bootstrap in dry-run mutates nothing and a second real run is a no-op; the bootstrap never disables password login without the verification flag; the ancestry, manifest and probe helpers return the right exit codes for pass, fail and retry-then-pass.
3. **Workflows** have no execution test. The shell lint step and the compose config validation in CI are their gate.

Failure paths assert exit codes and messages; edge cases (empty, oversized, uppercase-hex, 39/41-char SHA, shell metacharacters) come first, happy path after, per the repo's testing conventions. Tests must not require external services.

## Out of Scope

- MongoDB backup and restore automation on the VPS.
- Manual approval on the `production` environment.
- Image signing or digest pinning.
- Replacing the Docker socket usage in the log collector (e.g. with a socket proxy).
- Migrating to another host or orchestrator.
- Rotating application secrets that never left the maintainer's machine or the VPS.

## Further Notes

- Delivery order: cleanup PR first, so the hardened CD lands on a clean base. The first commit of the cleanup PR contains the deletions already pending in the working tree on the current branch. No push or PR is created without the maintainer asking.
- Risk to verify during implementation: the OAuth proxy restriction was found correctly applied in the compose file; the new test exists to keep it that way.
- Risk to verify: removing the socket from the proxy must be exercised by bringing the stack up before the PR, not only by static assertions.
- The bootstrap must be safe on the already-running VPS: it may not recreate the deploy user, the clone, volumes or containers, and it must keep an authenticated SSH session open while reloading the daemon.
