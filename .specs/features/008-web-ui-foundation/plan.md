# Implementation Plan: Web UI Foundation

**Branch**: `008-web-ui-foundation` | **Date**: 2026-10-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/008-web-ui-foundation/spec.md`

**Source plan**: `~/.opencode/plan/ui-refactor-guidelines.md` (phase F0 + F1). This feature is the
first phase of a four-phase UI redesign; the later phases (component primitives under a new
`src/ui/` layer, the screen-by-screen refactor, the visual-regression suite) are out of scope and
are named in the spec's Assumptions.

## Summary

Replace the web client's single-accent, 17px-capped, system-font token file with a three-layer
design-token system that declares a warm parchment light theme and a neutral charcoal plus amber
dark theme, both asserted at AA by the existing contrast suite. Add a persisted two-state
light/dark preference resolved by a tiny same-origin classic script in the document head, so the
first paint already uses the chosen theme and no inline script or Content-Security-Policy hash is
introduced. Remove the dead contextual panel from the shell — module, styles, tokens and collapse
breakpoint — leaving navigation plus content, and give the navigation footer the theme control.
Vendor the Geist variable font (OFL-1.1) into `public/fonts/`, preload it, and swap the system
stack for it with a system fallback. Finally, write the visual rules down: `web/DESIGN.md` becomes
the source of truth and `web/AGENTS.md` keeps only development, security and architecture rules.

## Technical Context

**Language/Version**: TypeScript 5.9.3 with the F1 strictness settings (`strict`, `noUnusedLocals`,
`noUnusedParameters`, `noFallthroughCasesInSwitch`); Node 24.20.0 / npm 11.19.0 (the pinned
`engines` range `>=24 <25`, verified in the environment).

**Primary Dependencies**: **no new runtime and no new dev dependency.** React 19.3.1, Vite 8.3.4,
`react-router-dom` 7.18.4, Vitest 5.0.5, Testing Library 16.3.3 and Playwright 1.64.1 are already
installed. The font is **vendored** (a committed `woff2` artifact plus its license), not a package,
so the "no design-system dependency" rule holds.

**Storage**: `localStorage` under the key `lg.theme`, holding exactly `light` or `dark`. This is a
display preference, not a credential: the constitution's ban covers credentials, and the refresh
cookie and in-memory access token are untouched (`UIF-19`, `UIF-21`).

**Testing**: Vitest 5.0.5 with Testing Library 16.3.3 and `user-event` 14.6.8, flat suites in
`web/src/test/` (the F1 layout); Playwright 1.64.1 for the browser-level shell spec and the axe
sweep. The existing token scans (`tokens_used.test.tsx`, `l10n_strings.test.ts`) and the contrast
suite carry the new assertions (`UIF-28` … `UIF-32`).

**Target Platform**: browsers — the latest two versions of Chrome, Edge, Firefox and Safari, served
from the web origin over HTTPS; both the light and the dark color scheme are first-class (native
controls follow `color-scheme`).

**Project Type**: web application (client) inside an existing Go monorepo; no backend module is
read or changed.

**Performance Goals**: no flash — the resolved theme is on the document before the first paint, so
the theme is never corrected after content shows (`UIF-20`). The head script is a tiny same-origin
blocking request, and the font is preloaded and `font-display: swap` so text never blocks on it
(`UIF-16`).

**Constraints**: no inline script (the head script is same-origin, so `script-src 'self'` already
permits it — no policy hash); no third-party font/style/script origin (`font-src 'self'`); no
literal color or pixel value in any scanned component directory, with `theme/` the deliberate
exception where the literals live (`UIF-29`); no new package.

**Scale/Scope**: 1 rewritten token file (three layers × two themes), 1 extended token mirror with
per-theme contrast pairs, 1 reworked test harness parser, 4 new theme modules plus the vendored
font pair and the head script, the shell slimmed by one module and its rules, `index.html` extended,
2 documents written/rewritten, 4 user stories (3 P1, 1 P2) and 34 requirements.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The feature adds no backend boundary and raises no `apperr`; the only failure paths are storage reads/writes, which are swallowed by design (`UIF-21`). | PASS (not exercised) |
| II. Canonical Identifiers | No identifier is generated, parsed or validated here. | PASS (not exercised) |
| III. Behavior Ships With Tests | Every requirement lands with a test in the same commit: both-theme token resolution and contrast, the theme resolution order (system, explicit, corrupt, throwing storage), the control's accessible state, the two-region shell and the narrow-viewport behaviour, the font declaration/preload, and the documentation's sections. Edge cases first (no stored value, unknown value, storage throws, no system signal, token declared in one theme only). | PASS |
| IV. English-Only Source | All identifiers, comments, test names, copy keys and both documents are English (the plan document stays pt-BR, as it is a local working plan, not a committed artifact). | PASS |
| V. Layered Architecture | Only `web/` gains code: `theme/` is a leaf consumed by `main.tsx`, the shell and the nav; the shell depends on `l10n/` and `theme/` and imports nothing upward. The Go layering is untouched. | PASS |
| VI. Security Gates Are Blocking | No gate is changed or weakened, and no dependency is added, so the SCA/SAST surface is unchanged. No credential is persisted (`UIF-19` is a display preference); the no-flash resolution runs in a **same-origin** head script, not an inline one, so `script-src 'self'` holds and no policy hash is introduced (`UIF-20`); the font is same-origin, so `font-src 'self'` holds (`UIF-14`); no DOM-injection sink is added, and the F1 ESLint anti-sink rules cover the new `.tsx` sources. | PASS |
| VII. Installable, Offline-Capable Web Client | The shell keeps its routes and deep links, and `theme-color` follows the color scheme so the installed window matches the theme (`UIF-03`). The interface outcome is verified by rendering tests plus the browser-level shell spec, which this feature retargets rather than deletes (`UIF-32`). Service-worker caching and offline read remain F8. | PASS |
| Additional Constraints — Stack | React with Vite and TypeScript under `web/`; no framework, state library or design-system package is added, and the theme preference uses the same small external-store pattern the client already uses (`useSyncExternalStore`). | PASS |
| Additional Constraints — Traceability | `UIF-*` is defined in `spec.md`, and the F1 lint already scans `*.ts`/`*.tsx`, so citations in the delivered sources are gated without further work. | PASS |
| Additional Constraints — Jira | This is pre-work planning; the implementing PR must reference its `LGN-\d+` card. Recorded for the PR, not a gate failure here. | PASS (noted) |
| Development Workflow | The documented client sequence (`make lint-web && make test-web && make sec-scan`) covers the new modules, and the Playwright shell spec runs in the existing e2e suite. No Makefile or workflow change is required. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps the visual contract in one
place: the token file declares the values, the token mirror describes them, and every component
consumes `--lg-*` only, so the literal scan keeps its teeth with `theme/` as the single documented
exception (`R1`). The two decisions most likely to drift were resolved in research: the test
harness parses the token file **per theme block** instead of flattening it, so a token declared in
one theme only fails (`R2`); and the no-flash path is a blocking same-origin script, which keeps
constitution VI's "no inline script" intact rather than amending it (`R3`). The font is vendored
with its license instead of pulled from a package or a CDN, so "no new dependency" and
"same-origin only" both hold (`R4`). The panel removal deletes its module, rules and tokens
together and retargets the suites that pinned it, so no assertion is weakened (`R5`, `UIF-26`,
`UIF-32`).

## Project Structure

### Documentation (this feature)

```text
.specs/features/008-web-ui-foundation/
├── spec.md                  # Requirements (UIF-*)
├── plan.md                  # This file
├── research.md              # Phase 0 — decisions and rejected alternatives
├── data-model.md            # Phase 1 — tokens, themes, preference, shell regions, documents
├── quickstart.md            # Phase 1 — runnable validation guide
├── contracts/               # Phase 1 — one contract per feature interface
│   ├── token-contract.md
│   ├── theme-preference.contract.md
│   └── shell-two-region.contract.md
├── checklists/
│   └── requirements.md      # Spec quality checklist
└── tasks.md                 # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
web/
├── index.html                       # extended: color-scheme theme-color, blocking head script, font preload (UIF-03, UIF-16, UIF-20)
├── public/
│   ├── theme-init.js                # NEW: resolves data-theme before first paint, same-origin (UIF-20)
│   └── fonts/geist/
│       ├── Geist-Variable.woff2     # NEW: vendored variable font (UIF-14, UIF-15)
│       └── Geist-LICENSE.txt        # NEW: OFL-1.1 license text (UIF-15)
└── src/
    ├── theme/
    │   ├── tokens.css               # REWRITTEN: primitives + semantics; :root light, [data-theme='dark'] dark (UIF-01 … UIF-13)
    │   ├── tokens.ts                # extended: per-theme contrastPairs, expanded designTokens (UIF-28 … UIF-31)
    │   ├── fonts.css                # NEW: self-hosted @font-face with swap (UIF-14, UIF-16)
    │   ├── base.css                 # extended: color-scheme, token-driven base, reduced motion (UIF-03, UIF-12, UIF-13)
    │   ├── theme.ts                 # NEW: resolve / read / write / apply the theme preference (UIF-17 … UIF-23)
    │   └── use_theme.ts             # NEW: subscription hook over the theme store (UIF-18)
    ├── shell/
    │   ├── app_shell.tsx            # reworked: navigation + content only (UIF-24, UIF-25)
    │   ├── shell.css                # reworked: two-column grid, nav footer + theme control, no .lg-panel* (UIF-25 … UIF-27)
    │   ├── theme_toggle.tsx         # NEW: the nav-footer control (UIF-18, UIF-27)
    │   ├── left_nav.tsx             # extended: footer hosts the theme control (UIF-27)
    │   └── right_panel.tsx          # DELETED (UIF-26)
    ├── l10n/
    │   └── strings.ts               # extended: theme control copy keys (UIF-18)
    └── test/
        ├── tokens_files.ts          # reworked: per-theme block parser (UIF-02, UIF-28)
        ├── tokens_contrast.test.ts  # extended: both themes (UIF-28, UIF-31)
        ├── theme_toggle.test.tsx    # NEW: resolution order, persistence, control state (UIF-17 … UIF-23, UIF-27)
        ├── tokens_used.test.tsx     # reworked: theme/ documented exception, shell/nav coverage (UIF-29, UIF-30)
        ├── self_hosted_assets.test.ts # NEW: font declaration, preload, no third-party origin (UIF-14 … UIF-16)
        ├── shell.test.tsx           # retargeted: navigation + content, no third region (UIF-24, UIF-32)
        ├── shell_responsive.test.tsx # retargeted: nav persistent, content minimum (UIF-25, UIF-32)
        └── shell_keyboard.test.tsx  # extended: the theme control is reachable (UIF-18)
web/
├── DESIGN.md                        # NEW: the visual source of truth (UIF-33)
└── AGENTS.md                        # rewritten: development, security and architecture rules only (UIF-33)
```

**Structure Decision**: the foundation stays where the client already keeps its design values —
`web/src/theme/` — and grows by four modules instead of a new directory, because the plan reserves
the shared component layer (`src/ui/`) for the next phase and this phase ships no component
primitives. `tokens.css` remains the only file holding literal values, which is exactly why the
token-usage scan treats `theme/` as its documented exception and keeps forbidding literals in
`shell/`, `catalog/`, `engine/`, `sheet/`, `library/` and `auth/` (`UIF-29`). The theme preference
lives in `theme.ts` as a small external store with a `use_theme.ts` hook, matching the client's
existing `useSyncExternalStore` pattern (F5/F7) rather than adding a state library. The no-flash
script is a **`public/`** asset, not a source module, because it must be a same-origin classic
script that runs before the bundle; `index.html` loads it in the head. The vendored font lives under
`public/fonts/geist/` beside its license so the artifact and its terms travel together. The panel
module is deleted outright: it was a placeholder whose own doc comment declared it temporary, and
keeping it would leave dead code and dead CSS. `web/DESIGN.md` is created and `web/AGENTS.md`
rewritten to defer all visual values to it, which is the one place the plan asked to remove the old
"no base component layer" rule — that edit belongs to the next phase, when `src/ui/` actually
lands, so this phase only records the pointer.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
