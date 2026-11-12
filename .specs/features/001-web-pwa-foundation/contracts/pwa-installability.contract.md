# Contract: PWA Installability, Offline Shell, and Updates

**Applies to**: `web/index.html`, `web/vite.config.ts`, `web/public/manifest.webmanifest`,
`web/src/pwa/`.
**Requirements**: FND-06, FND-07, FND-08, FND-09, FND-10.

## Installability (FND-06)

Served over HTTPS, with:

- a web app manifest reachable at a stable path, declaring `name`, `short_name`, `start_url`,
  `scope`, `display: standalone`, `theme_color`, `background_color`, and `orientation`;
- `start_url` inside `scope`, and `scope` equal to the client's own origin path prefix;
- icons at 192 px and 512 px in both standard and maskable form, referenced with `sizes` and
  `purpose`;
- a service worker registered from an external module (no inline registration script, FND-10)
  that controls `start_url` after the first visit.

Definition of installable = all of the above, and the Chromium installability audit passes in CI.
Safari (Add to Home Screen) and Firefox are verified manually each release and the outcome is
recorded with the release.

## Offline shell (FND-07, FND-09)

| Request class                                              | Behaviour                                                                                                     |
| ---------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Navigation (any declared route)                            | serve the cached shell when the network is unavailable; the app then renders the route or the not-found state |
| Build assets (HTML, entry chunks, CSS, icons)              | precached at install time                                                                                     |
| Public GET application data                                | from the next feature onward: runtime cache with an explicit strategy; never cache authenticated responses    |
| `/auth/*` and any request carrying an authorization header | never cached, never served from cache                                                                         |
| Storage                                                    | no credentials and no authenticated response payload stored in Cache Storage or IndexedDB                     |

On `activate`, caches whose name lacks the current version identity are deleted, so a release
cannot leave mixed assets behind.

## Deep links (FND-08)

- Direct navigation to any declared route renders the same screen as in-app navigation.
- Unknown paths render the designed not-found state — never a blank page, never an unhandled
  runtime error, never a server 404 page.
- The host serves the shell for unknown paths (history fallback) so a cold deep link works before
  the worker exists.

## Updates (FND-09)

1. A newly built shell installs the new worker but does **not** activate it while a session is open
   (`skipWaiting: false`).
2. No notice is shown: the waiting worker activates once every window of the old version closes,
   so the next launch runs the new version silently.
3. Runtime caching covers same-origin static assets only; API responses are caller-scoped by
   cookie and are never cached, so a deleted or edited sheet is never served stale.

## Content-policy preconditions (FND-10)

- No inline `<script>` in the served HTML; the entry module is a same-origin file.
- No script, style, or font is loaded from a third-party origin.
- Asset URLs are same-origin and versioned by the build, so a strict policy can be enabled later
  without changing the shell.

## Verification

| Check                          | Method                                                                    |
| ------------------------------ | ------------------------------------------------------------------------- |
| Manifest fields and icon sizes | automated manifest assertion                                              |
| Installability audit           | Lighthouse CI (Chromium)                                                  |
| Offline boot                   | E2E: load once, disable network, reload, assert the shell                 |
| Deep link                      | E2E per declared route, cold load                                         |
| Unknown route                  | E2E asserting the not-found state                                         |
| Cache exclusions               | unit test over the worker's route rules (`/auth/*`, authorization header) |
| Version handover               | E2E asserting the notice appears and the reload happens only on accept    |
| No inline/third-party assets   | HTML/build-output assertion                                               |
