# Legnaris Web (React PWA)

The product client: an installable React + Vite + TypeScript Progressive Web App that
replaces the deprecated Flutter desktop app. It renders a token-driven application
shell, the template catalog and its creation surface, the sheet cycle (open, edit,
save, preview, guards and crash-safe drafts), and the browser cookie session
(sign-in, registration, single-flight silent refresh and sign-out).

Program plan: [`docs/plan-web-pwa.md`](../docs/plan-web-pwa.md). Feature specs, design and
validation guides: [`.specs/features/001-web-pwa-foundation/`](../.specs/features/001-web-pwa-foundation/)
(the shell), [`.specs/features/002-web-api-client/`](../.specs/features/002-web-api-client/)
(the typed API client), [`.specs/features/005-web-sheet-cycle/`](../.specs/features/005-web-sheet-cycle/)
(the sheet cycle) and [`.specs/features/007-web-auth-session/`](../.specs/features/007-web-auth-session/)
(the auth session) and [`.specs/features/008-web-ui-foundation/`](../.specs/features/008-web-ui-foundation/)
(the visual foundation) — each `quickstart.md` holds the runnable validation scenarios.
Visual rules: [`DESIGN.md`](./DESIGN.md); development, security and architecture rules:
[`AGENTS.md`](./AGENTS.md). Shared primitives layer: `web/src/ui/` (`button`, `field`,
`checkbox`, `dialog`, `toast`, `focus_trap`), extended literal/copy scans (`tokens_used`,
`l10n_strings`) and requirement-ID lint over `.css`.

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

## Design foundation (`src/theme/`, `src/shell/`, `public/`)

The visual system is three-layer CSS custom properties plus plain CSS — **no UI library, theme
framework or font package**. The full rules live in [`DESIGN.md`](./DESIGN.md).

- **Tokens.** `src/theme/tokens.css` is the one file that declares literal values (primitives →
  semantic aliases → the occasional component token), mirrored in `src/theme/tokens.ts`. The light
  theme is on `:root` and the dark theme under `[data-theme='dark']`, each setting `color-scheme`;
  every component consumes `--lg-*` only. Contrast is asserted per theme in
  `src/test/tokens_contrast.test.ts`.
- **Theme preference.** The choice is exactly `light` | `dark` under `localStorage['lg.theme']`.
  `public/theme-init.js` is a same-origin **classic** head script that resolves it (falling back to
  `prefers-color-scheme` when nothing is stored) before the first paint, so there is no load flash
  and no inline script to hash; `src/theme/theme.ts` owns the runtime resolution and the
  `src/shell/theme_toggle.tsx` control writes the choice back.
- **Shell.** Two regions only — navigation then content (`src/shell/app_shell.tsx`), a two-column
  grid in `src/shell/shell.css`. The old contextual right panel is gone.
- **Font.** Geist is **vendored**, not installed: `public/fonts/geist/Geist-Variable.woff2`
  (69,492 bytes, sha256 `9ce2d97863cab59a0c7522534112d6c42a4e25c0e6b2ab37766761b0f82cfbd6`) with its
  OFL-1.1 license beside it as `Geist-LICENSE.txt`, both extracted from the official `geist@1.8.1`
  release on the npm registry (`dist/fonts/geist-sans/`, published by Vercel). One `@font-face` in
  `src/theme/fonts.css`, preloaded from `index.html`; nothing is fetched from a third-party origin.

```bash
cd web
npm test                                          # tokens_contrast, self_hosted_assets, theme_toggle, shell
npx vitest run src/test/tokens_contrast.test.ts   # AA in both themes
npx vitest run src/test/theme_toggle.test.tsx     # resolution, persistence, no third state
npm run test:e2e -- theme.spec.ts                 # explicit choice survives a reload, no flash
```

## API client (`src/api/`)

The typed network layer every screen mounts on: the named operations (health, template
catalog, template detail, sheet creation, retrieval, own-sheet read and delete, sheet
update and preview), one envelope decoder, contract-faithful template/sheet models, a
failure-kind mapper, and a pure base address resolver. It holds no state and touches no
browser global outside the transport adapter.

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
  go run ./backend/cmd/api.go                    # the API on :8080 (needs DATABASE_URI, JWT_SECRET)
  bash web/src/api/fixtures/capture.sh   # writes src/api/fixtures/*.json verbatim
  ```

  A capture is never hand-edited to make a test pass; a shape change is a backend contract
  change and the suite is expected to go red on it.

- **Base address.** `resolveApiConfig` reads the Vite build variable `VITE_API_BASE_URL`
  (default `https://api.legnaris.com.br`); an empty value falls back to the default, and a
  non-`http(s)` value fails at construction. `http://` is accepted only as an explicit
  build choice and is reported as `secure: false` on the resolved configuration.

## Auth session (`src/auth/`, `src/api/auth_api.ts`)

The browser cookie session that gates every surface. `src/api/auth_api.ts` is the four
`/auth/*` operations (register, login, refresh, logout) over the **raw** transport with
`credentials: 'include'` and the `X-CSRF-Token` echo on refresh/logout; `src/api/csrf.ts`
reads the script-readable `__Host-legnaris_csrf` cookie. `src/auth/auth_session.ts` holds
the short-lived access token **in memory only** — nothing is persisted — with a session
version and a `restoring | unauthenticated | authenticating | authenticated` state, plus
single-flight `refresh()` and `logout()`. `src/api/authenticated_transport.ts` decorates the
transport: it attaches `Authorization: Bearer` to a protected path, refreshes once on a
`401` and replays once, while `/health` and `/auth/*` are never Bearer-authenticated.
`src/auth/auth_gate.tsx` runs the one cold-start restore, renders the sign-in surface while
no session is held and otherwise mounts the shell. A definitive boundary (sign-out or forced
logout) sweeps the session-local data through `src/library/session_data.ts`
(`store.clearAll()`); a transient refresh failure keeps it. `src/sheet/your_sheet_screen.tsx`
renders the signed-in person's own sheet (`GET /sheets`) with a confirmed delete.

```bash
cd web
npm test                                                 # auth_*, session_* and your_sheet suites
npx vitest run src/test/auth_gate.test.tsx               # restoring, rejected cookie, deep-link reset
npx vitest run src/test/authenticated_transport.test.ts  # Bearer, single-flight refresh, replay
npx vitest run src/test/session_boundary.test.ts         # sign-out sweep and single-flight
npx vitest run src/test/your_sheet.test.tsx              # resolve, open and confirmed delete
npm run test:e2e -- auth.spec.ts                         # axe: zero critical violations
```

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
