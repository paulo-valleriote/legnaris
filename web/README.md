# Legnaris Web (React PWA)

The product client: an installable React + Vite + TypeScript Progressive Web App that
replaces the deprecated Flutter desktop app. It renders a token-driven application
shell and, in later features, the catalog, the sheet cycle, and authentication.

Program plan: [`docs/plan-web-pwa.md`](../docs/plan-web-pwa.md). Feature specs, design and
validation guides: [`.specs/features/001-web-pwa-foundation/`](../.specs/features/001-web-pwa-foundation/)
(the shell) and [`.specs/features/002-web-api-client/`](../.specs/features/002-web-api-client/)
(the typed API client) — each `quickstart.md` holds the runnable validation scenarios. Visual
rules: [`AGENTS.md`](./AGENTS.md).

## Prerequisites

- Node.js 24 LTS (pinned in [`.nvmrc`](./.nvmrc)) and npm.

## Setup

```bash
cd web
npm ci            # frozen lockfile
npm run dev       # development server
```

## Container image

`web/Dockerfile` builds the client into an nginx image, which the deploy stack
runs as the `web` service on `WEB_DOMAIN` (see `deploy/README.md`):

```bash
cd deploy
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build web
```

The browser-facing API origin is a **build argument**, because Vite inlines
every `VITE_*` value into the bundle: `WEB_API_BASE_URL` in `deploy/.env` becomes
`VITE_API_BASE_URL` here. Changing it (or `WEB_DOMAIN`) therefore needs
`--build web`, not a restart. The web origin must also be listed in
`CORS_ALLOWED_ORIGINS`, which the same file configures.

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

## API client (`src/api/`)

The typed network layer every screen mounts on: six named operations (health, template
catalog, template detail, sheet creation, sheet retrieval, sheet update), one envelope
decoder, contract-faithful template/sheet models, a failure-kind mapper, and a pure base
address resolver. It holds no state and touches no browser global outside the transport
adapter.

```bash
cd web
npm test                                  # unit + pinned captures; no backend needed
API_BASE_URL=http://localhost:8080 npm test # also runs the live contract suite
```

- **Suites.** `api_*.test.ts` cover the transport seam, the envelope, the models, the
  client, the failure states, retry/timeout, the pinned captures and credential hygiene.
  The pinned suite (`api_contract_captures.test.ts`) runs against real captured responses
  and needs no backend; the live suite (`api_live_contract.test.ts`) runs only when
  `API_BASE_URL` is set and otherwise reports as skipped.
- **Pinned captures.** `src/api/fixtures/` holds real responses from the local stack; see
  its [`README.md`](./src/api/fixtures/README.md) for the inventory and the reproduction
  commands. Regenerate them with the running stack:

  ```bash
  make dev-db                            # MongoDB
  go run ./cmd/api.go                    # the API on :8080 (needs DATABASE_URI, JWT_SECRET)
  bash web/src/api/fixtures/capture.sh   # writes src/api/fixtures/*.json verbatim
  ```

  A capture is never hand-edited to make a test pass; a shape change is a backend contract
  change and the suite is expected to go red on it.

- **Base address.** `resolveApiConfig` reads the Vite build variable `VITE_API_BASE_URL`
  (default `https://api.legnaris.com.br`); an empty value falls back to the default, and a
  non-`http(s)` value fails at construction. `http://` is accepted only as an explicit
  build choice and is reported as `secure: false` on the resolved configuration.

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
