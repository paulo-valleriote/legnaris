# Quickstart: Validating the Web PWA Foundation

Runnable scenarios that prove the feature works end to end. This guide validates; it does not
prescribe implementation — see [plan.md](./plan.md), [data-model.md](./data-model.md), and
[contracts/](./contracts/) for the design.

## Prerequisites

- Node.js 24 LTS (`.nvmrc` pins it) and npm.
- A Chromium-based browser for the interactive checks.
- `docker` only if you also want the backend running (not required by this feature).

## Setup

```bash
cd web
npm ci                     # frozen lockfile — also the `lockfile` gate's local equivalent
npm run dev                # development server
```

## Deployed preview (required for the final pass)

Scenarios 2 (real install affordance) and the "Definition of done" line below need HTTPS on a
public hostname, not the local preview server. Bring the stack up and run the scenarios against it:

```bash
make deploy-up             # production stack (Traefik terminates TLS on the web domain)
# or, for a local tunnel that keeps the same hostnames:
make deploy-local
make deploy-down           # tear the stack down when done
```

**Expect**: `https://<WEB_DOMAIN>` serves the built client with the `web-security-headers`
middleware applied (CSP in `enforce` plus HSTS / `nosniff` / `Referrer-Policy`). This is the
surface T044's seven scenarios must run against.

## Scenario 1 — Shell renders (FND-01, FND-03, FND-04, FND-05)

1. Open the development URL.
2. **Expect**: three regions in order — navigation, content, contextual panel — with the content
   region showing the designed empty-workspace message and no demo rows.
3. Narrow the window until layout changes. **Expect**: the panel disappears first; navigation and
   content remain; content never overlaps navigation.
4. `Tab` through the shell. **Expect**: every control shows a visible focus ring, in visual order,
   with no trap.

Automated equivalent:

```bash
npm run test -- shell
```

## Scenario 2 — Installable (FND-06, FND-10)

```bash
npm run build
npm run preview            # serves the production build over http://localhost
```

1. Open DevTools → Application → Manifest. **Expect**: name, short name, `start_url`, `scope`,
   `display: standalone`, and 192/512 px icons (standard + maskable).
2. Application → Service Workers. **Expect**: a worker registered from an external module, active,
   controlling the page; View Source shows no inline `<script>` in the served HTML.
3. Run Lighthouse (Application/Installability). **Expect**: installability passes, no audit failure
   for manifest fields, icons, or worker.

HTTPS is required for the real install affordance; use the deployed preview for the final check.
Firefox and Safari are verified manually per release and recorded in the release notes.

## Scenario 3 — Works offline (FND-07)

1. With the built app served, load it once and wait for the worker to activate.
2. DevTools → Network → Offline (or stop the preview server).
3. Reload the page. **Expect**: the shell renders from cache and the app states that data is
   unavailable; no browser error page, no blank screen.
4. Navigate to a declared route while offline. **Expect**: the same shell/screen renders.

## Scenario 4 — Deep links (FND-08)

1. Cold-load `/` directly in a new tab. **Expect**: the shell and the root screen.
2. Cold-load an unknown path (e.g. `/does-not-exist`). **Expect**: the designed not-found state —
   not a blank page and not the host's 404 page.

## Scenario 5 — Update handover (FND-09)

1. Serve the built app and leave the tab open.
2. Rebuild (`npm run build`) and reload the server so a new version exists.
3. **Expect**: the tab shows a non-blocking update notice and keeps running the old version.
4. Accept the update. **Expect**: the new version activates and the page reloads once.
5. Reload and decline instead. **Expect**: the old version keeps working; no mixed assets.

## Scenario 6 — Gates are real (SEC-01…SEC-05)

Locally reproduce the pipeline:

```bash
make lint-web
make test-web
make sec-scan
```

Prove the gates fail closed (each check must produce a red result, then be reverted):

1. Add a dependency with a known published advisory, then run `make sec-scan`.
   **Expect**: the SCA gate fails with a non-zero exit.
2. Add a file containing a secret-shaped string, then run `make sec-scan`.
   **Expect**: the secret gate fails with a non-zero exit.
3. Introduce an unused variable, then run `make lint-web`. **Expect**: lint fails.
4. Break a type, then run `make test-web`. **Expect**: type-check/tests fail.

In CI, open a client-only pull request and confirm `ci-web` runs (with all five jobs) while the Go
pipelines do not. Then change only a Go file and confirm the opposite (SEC-06).

## Scenario 7 — Traceability (SEC-07)

```bash
make lint-specs
```

1. **Expect**: `check_requirement_ids: clean`.
2. Add a comment in a `.tsx` file citing an ID that no spec defines (e.g. `ZZZ-99`), then rerun.
   **Expect**: the lint reports the citation and exits non-zero. Revert it.

## Definition of done for this feature

- [ ] Scenarios 1–7 pass on the deployed preview and in CI.
- [ ] `ci-web` jobs all green with no `continue-on-error`, and the three seeded negatives fail.
- [ ] Lighthouse: installability + performance budgets met; zero critical accessibility violations.
- [ ] `make lint-specs` clean with the requirement-ID rule covering `*.ts`/`*.tsx`.
- [ ] `web/AGENTS.md` documents the visual rules the shell implements.
