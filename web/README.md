# Legnaris Web (React PWA)

The product client: an installable React + Vite + TypeScript Progressive Web App that
replaces the deprecated Flutter desktop app. It renders a token-driven application
shell and, in later features, the catalog, the sheet cycle, and authentication.

Program plan: [`docs/plan-web-pwa.md`](../docs/plan-web-pwa.md). Feature spec, design and
validation guide: [`.specs/features/001-web-pwa-foundation/`](../.specs/features/001-web-pwa-foundation/)
(see `quickstart.md` for the runnable validation scenarios). Visual rules:
[`AGENTS.md`](./AGENTS.md).

## Prerequisites

- Node.js 24 LTS (pinned in [`.nvmrc`](./.nvmrc)) and npm.

## Setup

```bash
cd web
npm ci            # frozen lockfile
npm run dev       # development server
```

## Scripts

| Script                 | What it does                                             |
| ---------------------- | -------------------------------------------------------- |
| `npm run dev`          | Vite dev server (SPA fallback for deep links)            |
| `npm run build`        | Type-check + production build (also emits `dist/sw.js`)  |
| `npm run preview`      | Serve the production build (SPA fallback, offline shell) |
| `npm run lint`         | ESLint, including the anti-XSS rules                     |
| `npm run format:check` | Prettier check                                           |
| `npm run typecheck`    | `tsc --noEmit`                                           |
| `npm test`             | Vitest unit + rendering tests                            |
| `npm run test:e2e`     | Playwright end-to-end specs (`e2e/`)                     |

Local parity with the CI pipeline (`ci-web.yml`):

```bash
make lint-web     # format-check + lint + type-check
make test-web     # Vitest
make sec-scan     # npm audit + signatures + lockfile-lint (+ gitleaks when installed)
```

## Toolchain version note

`package.json` pins **TypeScript 5.9** and **ESLint 9.39**, not the newer major lines the
program plan sketched (TypeScript 7, ESLint 10). As of this feature the security/tooling
ecosystem does not yet support them: `typescript-eslint` requires `typescript < 6.1`, and
`eslint-plugin-react` / `eslint-plugin-jsx-a11y` require `eslint <= 9`. The pinned versions
are the newest that satisfy every required anti-XSS and a11y plugin; bump them together once
the plugins publish compatible majors.

## Seeded-negative acceptance (proving the gates are real)

The CI gates must fail closed. Reproduce each negative locally, then revert it (SEC-02,
SEC-03, SEC-04):

1. **SCA** — add a dependency with a known advisory, then run `make sec-scan`.
   Expect: `npm audit` (and `ci-web`'s `sca` job) exits non-zero.
2. **Secrets** — add a file containing a secret-shaped string (e.g. a fake AWS key), then
   run `make sec-scan` (with `gitleaks` installed) or rely on the `ci-web` `secrets` job.
   Expect: a non-zero exit.
3. **Lint** — introduce an unused variable, then run `make lint-web`. Expect: failure.
4. **Type/tests** — break a type, then run `make test-web`. Expect: failure.

No gate in `.github/workflows/ci-web.yml` sets `continue-on-error`, and none is
conditionally skipped on the trigger paths; a scanner that cannot run fails its job.
