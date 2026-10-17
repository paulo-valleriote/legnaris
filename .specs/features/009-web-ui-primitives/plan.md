# Implementation Plan: Web UI Primitives

**Branch**: `009-web-ui-primitives` | **Date**: 2026-10-17 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/009-web-ui-primitives/spec.md`

**Source plan**: `docs/plan-ui-refactor.md` (phase F2; §3.8 component specifications, §3.12
enforcement, §3.13 anti-patterns, §4 feature map, §6 risks). It is the second phase of a four-phase
UI redesign: the previous phase delivered the tokens, the two themes, the theme control and the
two-region shell (`008-web-ui-foundation`); the screen-by-screen refactor and the visual-regression
suite are later phases and are out of scope.

## Summary

Create the client's first shared component layer, `web/src/ui/`, holding the five primitives the
program plan names for this phase — the action (button), the field, the checkbox, the dialog and the
notice (toast) — each in its own module with one token-only stylesheet. The layer is a leaf: it
imports no feature directory, resolves its chrome copy through the existing catalog and takes every
visual value from the tokens the previous phase declared, so it adds no token and no dependency. Each
primitive's variants, sizes, states and accessibility contract are asserted by rendering tests, with
the dialog's focus confinement extracted into a module the suite drives directly. The literal scan
and the copy scan grow a `web/src/ui/` block, the requirement-ID lint grows `.css` so the layer's
stylesheet citations are gated, and `web/AGENTS.md` replaces the "no base component layer"
prohibition with the sanction of this layer while keeping the "three real usages" rule for every
other extraction — recorded in the design document too. No screen is migrated: the existing feature
styles stay until the next phase switches the surfaces over, exactly as the schema-driven engine
shipped before any screen mounted it.

## Technical Context

**Language/Version**: TypeScript 5.9.3 with the client's strictness settings (`strict`,
`noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`); Node 24.20.0 / npm 11.19.0
(the pinned `engines` range `>=24 <25`, verified in the environment).

**Primary Dependencies**: **no new runtime and no new dev dependency.** React 19.3.1, Vite 8.3.4,
Vitest 5.0.5, Testing Library 16.3.3 and `user-event` 14.6.8 are already installed. The primitives
are plain React plus plain CSS; no component library, headless-UI package or styling runtime is
added, and the dialog uses the overlay pattern rather than the platform's native modal element.

**Storage**: none. The layer owns no persisted state; the only copy it owns is generic chrome
(`ui.dismiss`) in the existing catalog module.

**Testing**: Vitest 5.0.5 with Testing Library 16.3.3 and `user-event` 14.6.8, flat suites in
`web/src/test/` (the client layout) — one suite per primitive, one for the focus-confinement module,
one for the layer's governance, plus the two existing scans (`tokens_used.test.tsx`,
`l10n_strings.test.ts`) extended with a `web/src/ui/` block and the document suite
(`design_guidelines.test.ts`) extended with the layer rule. No Playwright case: the layer mounts no
route, so there is nothing for a browser spec to drive; the browser-level visual regression is the
last phase of the redesign.

**Target Platform**: browsers — the latest two versions of Chrome, Edge, Firefox and Safari, in both
themes, with keyboard-only operation first-class (tab order, visible focus, Escape, focus return).

**Project Type**: web application (client) inside an existing Go monorepo; no backend module is read
or changed.

**Performance Goals**: none specific to the layer. The primitives add no timer, no listener beyond the
open dialog's key handler, and no request; the stylesheet is a single token-only file.

**Constraints**: no literal colour or pixel value in any scanned component directory, `web/src/ui/`
included; no unresolved token reference; no user-facing literal; no dependency; no change to the
tokens the previous phase declared; the requirement-ID lint must walk stylesheets so the layer's
citations have a gate.

**Scale/Scope**: 1 new source directory with 6 modules (5 primitives plus the focus-confinement
helper) and 1 stylesheet, 1 catalog key added, 1 lint script extended (suffix list) with its
self-test, 2 documents updated (`web/AGENTS.md`, `web/DESIGN.md`), 7 new suites plus 3 extended ones,
4 user stories (3 P1, 1 P2) and 26 requirements.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The layer raises no `apperr` and touches no backend boundary; a primitive has no failure mode to classify (an empty notice is not rendered, per `UIP-24`). | PASS (not exercised) |
| II. Canonical Identifiers | No identifier is generated, parsed or validated here. | PASS (not exercised) |
| III. Behavior Ships With Tests | Every requirement lands with a test in the same commit: one suite per primitive asserting its variants, sizes, states and accessibility contract by rendering it, the focus-confinement suite driving the standalone module with zero, one and many controls, and the extended scans. Edge cases first (disabled/busy activation does not run, no error association without an error, zero focusable controls, activation inside the panel, empty message, a value that is not a boolean). | PASS |
| IV. English-Only Source | All identifiers, comments, test names, class names, copy keys and both documents are English. | PASS |
| V. Layered Architecture | The Go layering is untouched. In the client the new layer is a leaf: it imports no feature directory, resolves its chrome copy through the existing catalog and consumes the design tokens through CSS custom properties, so it sits *below* every screen rather than beside one (`UIP-02`). | PASS |
| VI. Security Gates Are Blocking | No gate is changed or weakened and no dependency is added, so the SCA/SAST surface is unchanged. No DOM-injection sink is added: the notice and every primitive render their content as text children, and the existing ESLint anti-sink rules cover the new `.tsx` sources. Nothing is persisted, and no credential is touched. | PASS |
| VII. Installable, Offline-Capable Web Client | The layer adds no route, no service-worker interaction and no network call; the shell's routes and deep links are untouched. The interface outcome is verified by rendering tests, which is what the constitution requires for interface behaviour; the browser-level accessibility sweep of the assembled surfaces is the catalog/auth phase's (`AD-014`) and the visual regression is the last phase's. | PASS |
| Additional Constraints — Stack | React with Vite and TypeScript; no framework, state library, component library or styling runtime is added, and the primitives are plain function components over CSS classes, the shape the client already uses. | PASS |
| Additional Constraints — Traceability | `UIP-*` is defined in `spec.md` and cited from the layer's sources; the requirement-ID lint already walks `.ts`/`.tsx` and this feature extends it to `.css` so the stylesheet citations resolve too (`UIP-06`). A citation that does not resolve stays a defect. | PASS |
| Additional Constraints — Jira | This is pre-work planning; the implementing PR must reference its `LGN-\d+` card. Recorded for the PR, not a gate failure here. | PASS (noted) |
| Development Workflow | The documented client sequence (`make lint-web && make test-web && make sec-scan`) covers the new modules, and `make lint-specs` covers the extended lint script. No Makefile or workflow change is required. | PASS |

No principle is waived; **Complexity Tracking** stays empty. The one rule that changes is a
`web/AGENTS.md` development rule (not a constitution principle): the "no base component layer"
prohibition is replaced by the sanction of `web/src/ui/`, recorded as `AD-019` and asserted by a
document test (`UIP-03`, `SC-007`). No constitution amendment is needed — constitution V's
directionality is a backend import order, and this layer respects the client's own directionality
rather than changing it.

**Post-design re-check (after Phase 1)**: still PASS. Three decisions were settled in research so the
gate stays honest: the layer is a leaf with exactly one allowed upward-ish import, the copy catalog
(`R2`), so a primitive cannot drift into a screen's logic; the dialog's focus confinement is a
standalone module (`R5`), so the keyboard contract is proven without a mounted screen and without
`jsdom`'s missing native modal method; and the requirement-ID lint's stylesheet extension is
**safe on the current corpus** — every ID cited in an existing client stylesheet already resolves
(`UIF-01/02/03/14`, `WCT-28`, `ENG-28`, `WSC-33`, `WAS-31`), verified before the change (`R7`), so no
assertion is weakened and no existing comment has to be removed. The notice's missing timer (`R6`)
and the field family's stop at the text input, the select and the multi-line text area (`R4`) are the
two scope boundaries the spec writes down in Assumptions rather than hiding.

## Project Structure

### Documentation (this feature)

```text
.specs/features/009-web-ui-primitives/
├── spec.md                  # Requirements (UIP-*)
├── plan.md                  # This file
├── research.md              # Phase 0 — decisions and rejected alternatives
├── data-model.md            # Phase 1 — primitives, states, layer, focus confinement, notice
├── quickstart.md            # Phase 1 — runnable validation guide
├── contracts/               # Phase 1 — one contract per feature interface
│   ├── ui-layer.contract.md
│   ├── form-controls.contract.md
│   └── overlay-and-feedback.contract.md
├── checklists/
│   └── requirements.md      # Spec quality checklist
└── tasks.md                 # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
web/
├── src/
│   ├── ui/                          # NEW: the shared base component layer (UIP-01, UIP-02)
│   │   ├── ui.css                   # NEW: token-only styles for every primitive (UIP-04, UIP-06)
│   │   ├── button.tsx               # NEW: the action primitive (UIP-07 … UIP-10)
│   │   ├── field.tsx                # NEW: the field primitive — input, select, text area (UIP-11 … UIP-15)
│   │   ├── checkbox.tsx             # NEW: the labelled boolean primitive (UIP-16, UIP-17)
│   │   ├── dialog.tsx               # NEW: the overlay + panel primitive (UIP-18 … UIP-21)
│   │   ├── focus_trap.ts            # NEW: the standalone focus-confinement module (UIP-22)
│   │   └── toast.tsx                # NEW: the non-blocking notice primitive (UIP-23 … UIP-26)
│   ├── l10n/
│   │   └── strings.ts               # extended: `ui.dismiss` (UIP-24)
│   └── test/
│       ├── ui_button.test.tsx       # NEW: variants, sizes, states, activation (UIP-07 … UIP-10)
│       ├── ui_field.test.tsx        # NEW: label, error, disabled, numeric, three controls (UIP-11 … UIP-15)
│       ├── ui_checkbox.test.tsx     # NEW: label, states, boolean contract (UIP-16, UIP-17)
│       ├── ui_dialog.test.tsx       # NEW: role, name, Escape, scrim, focus move/return (UIP-18 … UIP-21)
│       ├── ui_focus_trap.test.ts    # NEW: zero/one/many controls, wrap, never escapes (UIP-22)
│       ├── ui_toast.test.tsx        # NEW: live region, dismiss, variants, no timer (UIP-23 … UIP-26)
│       ├── ui_layer.test.ts         # NEW: layer contents + no feature-directory import (UIP-01, UIP-02)
│       ├── tokens_used.test.tsx     # extended: a `web/src/ui/` block (UIP-04)
│       ├── l10n_strings.test.ts     # extended: a `web/src/ui/` block + the `ui.*` keys (UIP-05)
│       └── design_guidelines.test.ts# extended: the layer rule in both documents (UIP-03)
├── DESIGN.md                        # updated: the layer, its primitive set, its class convention (UIP-03)
└── AGENTS.md                        # updated: the base-layer sanction, the ≥3 rule retained (UIP-03)

scripts/
├── check_requirement_ids.py         # extended: `.css` joins the walked suffixes (UIP-06)
└── test_lints.py                    # extended: the stylesheet-walker case (UIP-06)

.specs/STATE.md                      # AD-019: the base layer's rule change (UIP-03)
```

**Structure Decision**: the primitives live in one new directory, `web/src/ui/`, because the program
plan reserves exactly that home for them (§3.8: "primitives in `src/ui/`"; §3.12: "extend the literal
scan to `src/ui/`") and because the previous phase deliberately shipped no such layer. The directory
keeps the client's established conventions instead of inventing new ones: one stylesheet per
directory (like `engine.css` and `shell.css`), modules named after the thing they render
(`button.tsx`, `dialog.tsx`), copy through the one catalog, and no barrel file — callers import the
module they use, as every other client feature does. The dialog's focus confinement is a separate
module because it is behaviour with its own edge cases (zero controls, wrapping, never escaping) that
the constitution's edge-case-first rule wants tested directly, and because the test DOM does not
implement the platform's native modal method, so the overlay pattern's keyboard contract has to be
owned by the code rather than the platform (the catalog phase recorded the same constraint). The
requirement-ID lint is extended rather than duplicated: `.css` joins the suffix set it already walks
(`.go`, `.dart`, `.ts`, `.tsx`), which is where the layer's stylesheet citations live.

The feature directory follows the program plan's feature map (`009-web-ui-primitives`, prefix `UIP`);
the short name passed to `create-new-feature.sh` was normalized to match that table so the plan and
the spec corpus stay in agreement.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
