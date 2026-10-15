# Contract: Client Quality and Security Gates

**Applies to**: `.github/workflows/ci-web.yml`, `.github/workflows/ci-lint.yml`,
`.github/dependabot.yml`, `web/package.json`, `Makefile`.
**Requirements**: SEC-01, SEC-02, SEC-03, SEC-04, SEC-05, SEC-06.

## Workflow

- Name: `ci-web`. Trigger: pull requests (and pushes to the default branch) touching `web/**`,
  `.github/workflows/ci-web.yml`, or the client lockfile.
- `permissions` are minimal per job (`contents: read`; `security-events: write` only where a
  scanner uploads results).
- `concurrency` cancels superseded runs on the same ref.

## Jobs

| Job | Checks | Blocking | Fail-closed |
|---|---|---|---|
| `quality` | format check, lint (ESLint incl. anti-XSS rules), type-check, unit tests, production build | yes | yes |
| `sca` | `osv-scanner` on the lockfile, `Trivy fs`, `npm audit` (with signature verification) | yes | yes |
| `sast` | CodeQL (JS/TS, security-extended) and Semgrep (`p/owasp-top-ten`, `p/typescript`, `p/react`) | yes | yes |
| `secrets` | `gitleaks` scan of the diff and the tree | yes | yes |
| `lockfile` | `npm ci` on the frozen lockfile, lockfile-lint integrity | yes | yes |

Rules:

- No job sets `continue-on-error`, and no job is conditionally skipped on the trigger paths
  (SEC-04). A scanner that fails to execute fails the job.
- New findings fail the pipeline even when `quality` is green (SEC-02, SEC-03).
- Scanner versions/actions are pinned; dependabot covers the workflow's own action versions where
  the ecosystem allows it.
- Reports that the tools can emit (SARIF/JSON) are uploaded as job artifacts for the security
  review.

## ESLint anti-XSS rules (part of `quality`, SEC-02/FND-10)

`react/no-danger`, `no-eval`, `no-new-func`, `no-script-url`, `no-unsanitized/property`, and
`no-unsanitized/method` are errors. Any future use of a DOM injection sink requires a plan
amendment plus DOMPurify; the lint rule is what makes that visible in review.

## Path scoping (SEC-06)

- `ci-web` runs on the paths above and is a required check.
- `ci-lint` (Go) and `ci-flutter` keep their own paths and MUST NOT be triggered by client-only
  changes; `ci-web` MUST NOT be triggered by backend-only changes.
- `ci-spec-lint` additionally triggers on `web/**/*.ts` and `web/**/*.tsx` so client requirement
  citations are linted (SEC-07).

## Dependency updates (SEC-05)

- Dependabot: `npm` ecosystem, directory `/web`, weekly schedule.
- Security updates are enabled and grouped separately from routine version updates, so a security
  release is distinguishable in the update queue.

## Local parity

`make lint-web`, `make test-web`, and `make sec-scan` run the same checks locally that `ci-web`
runs, so a contributor can reproduce a red pipeline before pushing.

## Verification

| Check | Method |
|---|---|
| Each gate fails on a seeded negative | acceptance test: a known-vulnerable dependency fails `sca`; a secret-like string fails `secrets` |
| No gate can silently pass | review + automated check that no job in `ci-web.yml` sets `continue-on-error` or a `if:` that skips it on the trigger paths |
| Path scoping both ways | contract test asserting the `paths:` filters of the three workflows |
| Local parity | run `make lint-web`, `make test-web`, `make sec-scan` |
