# Quickstart: validate the web UI foundation

**Feature**: `.specs/features/008-web-ui-foundation/` | **Date**: 2026-10-17

Runnable scenarios that prove the feature works end to end. Each lists what to run and the outcome
that means it passed. Prerequisites: the F1 client toolchain (`web/` with `npm install` already
done, Node 24). The Vitest suites need no backend, no MongoDB and no network: the theme preference
uses an injected storage double and the font assertions read committed files. Only the browser
scenarios (8, 9) need a built client.

Paths referenced here are defined in [plan.md](./plan.md) (source layout), [data-model.md](./data-model.md)
(shapes) and the three documents under [contracts/](./contracts/).

---

## 1. The suite runs with no backend at all

```bash
cd web && npm test
```

**Outcome**: the whole suite passes with the API and MongoDB stopped. The theme, shell and token
suites read the committed token file and use injected storage, so nothing touches the network.

---

## 2. The token file resolves in both themes

```bash
cd web && npx vitest run src/test/tokens_contrast.test.ts
```

**Outcome**: the per-theme parser finds a light block and a dark block; every documented design
token resolves in `tokens.css` (UIF-31); every semantic token resolves in **both** themes (UIF-02);
and every declared pair meets its minimum ratio in its own theme — 4.5:1 for text pairs, 3:1 for
the focus ring (UIF-28, SC-001). Removing a token from the dark block turns the suite red.

---

## 3. No component authors a visual value

```bash
cd web && npx vitest run src/test/tokens_used.test.tsx
```

**Outcome**: the scan finds no literal hex colour and no `NNpx` value in `src/shell/`,
`src/catalog/`, `src/engine/`, `src/sheet/`, `src/library/` or `src/auth/`, and every `var(--lg-*)`
those sources reference resolves in `tokens.css` (UIF-29, UIF-30, SC-002). `src/theme/` is the
documented exception where the literals live.

---

## 4. The theme resolves, persists and never flashes

```bash
cd web && npx vitest run src/test/theme_toggle.test.tsx
```

**Outcome**: with nothing stored the applied theme follows the simulated `prefers-color-scheme`
(UIF-17) and a change to that preference is followed live (UIF-23); activating the nav-footer control
switches the theme, writes `light`/`dark` to `lg.theme`, and exposes the control's accessible name
and current state (UIF-18, UIF-19, UIF-27); a stored value wins over a differing system preference
and survives a re-render (UIF-19); a value outside the set (`purple`) falls back to the system
preference (UIF-22); a storage that throws on read and on write shows no error and keeps the
system-derived theme (UIF-21).

---

## 5. The first paint already uses the chosen theme

```bash
cd web && npm run build && npx playwright test e2e/theme.spec.ts
```

**Outcome**: seeding `lg.theme = 'dark'` and loading the app paints the dark palette on the first
frame — no light frame precedes it — because `public/theme-init.js` is a blocking same-origin script
in the head (UIF-20, SC-004). The script is served from the app's own origin and the document
contains no inline script.

---

## 6. The shell has two regions and no dead one

```bash
cd web && npx vitest run src/test/shell.test.tsx src/test/shell_responsive.test.tsx src/test/shell_keyboard.test.tsx
```

**Outcome**: every route exposes `nav` (named) then `main`, in order, and no `complementary` region
(UIF-24, SC-003); at a narrow width both regions remain, the nav stays visible and the content keeps
its minimum (UIF-25); the nav footer hosts the theme control, reachable by keyboard (UIF-18, UIF-27);
and no source references `right_panel`, `.lg-panel`, `--lg-size-panel-*` or `SHELL_NARROW_QUERY`
(UIF-26).

---

## 7. The font is self-hosted and preloaded

```bash
cd web && npx vitest run src/test/self_hosted_assets.test.ts
```

**Outcome**: `Geist-Variable.woff2` and its OFL license exist under `public/fonts/geist/` and are
committed; `fonts.css` declares one `@font-face` with a same-origin `url('/fonts/geist/…')`, a
`100 900` weight range and `font-display: swap`; `index.html` preloads it with `crossorigin`; and no
third-party font/style/script origin appears anywhere in the client (UIF-14, UIF-15, UIF-16, SC-005).

---

## 8. The documented rules are the source of truth

```bash
grep -c '^## ' DESIGN.md && grep -n 'DESIGN.md' AGENTS.md
```

**Outcome**: `web/DESIGN.md` carries its mandatory sections — principles, token architecture,
colour, typography, spacing/layout, radius/border/elevation, motion, component specifications,
accessibility, security posture, per-screen patterns, enforcement, anti-patterns (UIF-33) — and
`web/AGENTS.md` defers visual values to it (UIF-33). The decision record in `.specs/STATE.md`
captures the three-layer two-theme tokens, the `localStorage` preference, the same-origin no-flash
script and the panel removal (UIF-34).

---

## 9. The interface has no critical accessibility violation

```bash
cd web && npm run build && npx playwright test e2e/shell.spec.ts e2e/auth.spec.ts
```

**Outcome**: the shell, the catalog surface and the auth surface report zero critical automated
violations from `@axe-core/playwright` in both themes, and the retargeted shell spec still proves
region order, deep links and the designed not-found state (UIF-32). No new axe dependency is added.

---

## 10. The gates stay green

```bash
cd web && npm run format:check && npm run lint && npm run typecheck && npm test && npm run build
cd .. && make lint-specs
```

**Outcome**: all client gates pass and the spec lint reports clean, so every `UIF-*` citation in the
delivered sources resolves to `spec.md` and no suite assertion is vacuous. No dependency is added, so
the secret, SCA, SAST and lockfile gates see nothing new to vet.

---

## Traceability

| Scenario | Requirements |
|---|---|
| 1 | all `UIF-*` (no backend) |
| 2 | UIF-01…UIF-13, UIF-28, UIF-31 |
| 3 | UIF-29, UIF-30 |
| 4 | UIF-17…UIF-23, UIF-27 |
| 5 | UIF-20 |
| 6 | UIF-18, UIF-24…UIF-27, UIF-32 |
| 7 | UIF-14…UIF-16 |
| 8 | UIF-33, UIF-34 |
| 9 | UIF-32 |
| 10 | all `UIF-*` (lint) |
