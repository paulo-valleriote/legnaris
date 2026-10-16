# Legnaris Web (React PWA)

The product client: an installable React + Vite + TypeScript Progressive Web App that
replaces the deprecated Flutter desktop app. It renders a token-driven application
shell, the template catalog and its creation surface, and the sheet cycle (open, edit,
save, preview, guards and crash-safe drafts); authentication arrives in a later feature.

Program plan: [`docs/plan-web-pwa.md`](../docs/plan-web-pwa.md). Feature specs, design and
validation guides: [`.specs/features/001-web-pwa-foundation/`](../.specs/features/001-web-pwa-foundation/)
(the shell), [`.specs/features/002-web-api-client/`](../.specs/features/002-web-api-client/)
(the typed API client) and [`.specs/features/005-web-sheet-cycle/`](../.specs/features/005-web-sheet-cycle/)
(the sheet cycle) — each `quickstart.md` holds the runnable validation scenarios. Visual
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

## Catalog (`src/catalog/`)

The root route renders the template catalog (`catalog_screen.tsx`), and activating an
entry opens the creation surface (`create_sheet_dialog.tsx`) as an overlay. Both read
server state through TanStack Query hooks (`queries.ts`); `src/app/providers.tsx` builds
the F2 client and the query client and provides them to the tree. The catalog loads once
per app load and is cached in memory; the detail loads once per opening. The creation
surface issues one `POST /sheets` and seeds the created sheet before navigating to it.

```bash
cd web
npm test                                              # catalog, surface and query-policy suites
npx vitest run src/test/catalog_screen.test.tsx       # the five render situations
npx vitest run src/test/create_sheet_dialog.test.tsx  # surface states + name validation
npm run test:e2e -- catalog.spec.ts                   # axe: zero critical violations
```

## Sheet cycle (`src/sheet/`, `src/library/`)

The open/edit/preview/draft feature over one sheet, mounted at `/sheets/:id`.
`sheet_open.ts` resolves the route id (one `GET /sheets/:id`, then one
`GET /templates/:id`, reusing a seeded created sheet with no second fetch); `sheet_edit.ts`
holds the in-memory edit store — pending delta, copy-on-write values, the save and preview
lifecycle, the preview marks, two rolling throttle windows and the draft restore — exposed
through `useSyncExternalStore`. `sheet_screen.tsx` mounts the F4 engine in builder mode with
the sticky footer and the one guard modal. `src/library/` holds the `SheetStore` port, the
`idb` adapter (the only module importing `idb`), the cache read/write/evict helpers and the
injectable draft cadence. `src/shell/shortcut_registry.ts` binds `Ctrl+S`/`Cmd+S` (save) and
`F5` (refresh the sheet's template behind a discard confirmation), and
`src/shell/leave_guard.ts` asks before a navigation leaves unsaved work. No credential is
ever written to local storage; the draft holds only the pending editable delta.

```bash
cd web
npm test                                           # all sheet/library suites (sheet_*, library_*, draft_*)
npx vitest run src/test/sheet_open.test.tsx        # the six open situations
npx vitest run src/test/sheet_edit.test.ts         # delta, save, exclusive and baseline
npx vitest run src/test/sheet_preview.test.tsx     # the dry-run and its marks
npx vitest run src/test/sheet_guards.test.tsx      # F5, the leave confirmation and the throttle
npx vitest run src/test/draft_backup.test.ts       # cadence, restore and lifecycle
npm run test:e2e -- sheet.spec.ts                  # axe: zero critical violations on the surfaces
```

## UI engine (`src/engine/`)

The stateless rendering core, ported from the desktop UI engine: it turns a decoded
`FieldSchema` tree into labeled sections and controls, owns no state, issues no request,
and is wired to **no screen** (F4) — every case exercises the engine's own modules
directly. `field_grouping.ts` derives sections and rows; `field_control.tsx` and
`exclusive_control.tsx` render one leaf and one exclusive group; `sheet_form.tsx`
composes them; `sheet_defaults.ts` derives the draft; `sheet_mark.ts` holds the preview
mark contract; `engine.css` styles it all from `--lg-*` tokens. Copy resolves through
`src/l10n/strings.ts`.

`SheetForm` takes **exactly one** control source — `data` plus `onChanged`, or both
`controlBuilder` and `exclusiveControlBuilder`; any other combination throws (ENG-25).
The caller owns the per-field value subscription, the edit reporting and the preview
marks through the seams; the engine reads no value in builder mode.

```bash
cd web
npm test                                              # all engine suites (engine_* files)
npx vitest run src/test/engine_form.test.tsx          # composition and its single source
npx vitest run src/test/engine_field_control.test.tsx # one control per schema
npx vitest run src/test/engine_defaults.test.ts       # draft seed, overlay and cascade
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
