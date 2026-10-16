# Implementation Plan: Web UI Engine

**Branch**: `004-web-ui-engine` | **Date**: 2026-10-16 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/004-web-ui-engine/spec.md`

**Program plan**: `docs/plan-web-pwa.md` (Parte B, feature F4) — the umbrella plan this feature
implements.

## Summary

Port the desktop UI engine (`desktop/lib/engine/`) to the web stack as a self-contained, stateless
rendering core: a pure derivation that turns a decoded template field tree into labeled sections and
render rows (stacked leaves, declared-column table rows, mutually exclusive rows), a control per leaf
chosen from the leaf's own schema (numeric/text input, `config.*`/boolean toggle, derived read-only
"Calculated"), a single-choice control for an exclusive group, a seeded draft plus an incremental
cascade merge, and an additive preview mark (Modified/Recalculated) that tints a control and yields to
the base "Calculated". The composition exposes two seams — a per-row control builder and a per-row mark
builder — so the editing screen the sheet-cycle feature owns can subscribe to one field at a time
(preserving the granularity decision `AD-001`) without the engine owning any state, any request or any
copy. This feature wires **no surface**: every requirement is proven by rendering tests of the engine's
own components (Clarifications, 2026-10-16). Nothing is persisted, no credential is attached, no new
dependency is added, and no visual value or user-facing string is authored outside the shared tokens
and the translation catalog.

## Technical Context

**Language/Version**: TypeScript 5.9.3 with the F1 strictness settings (`strict`, `noUnusedLocals`,
`noUnusedParameters`, `noFallthroughCasesInSwitch`, `isolatedModules`, `resolveJsonModule`,
`@typescript-eslint/consistent-type-imports`) on React 19.3.1 / Vite 8.3.4, Node.js 24 LTS. These are
the pins the delivered F1 toolchain holds — running `node --version` and reading `web/package.json`
confirms 24.x and the versions above; no toolchain change is introduced here.

**Primary Dependencies**: **none added.** The engine consumes only what F1 and F2 already ship: the F2
template model (`FieldSchema`, `LeafNode`/`GroupNode`, `TemplateDetail`) and the F2 dotted-path
accessor (`valueAtPath`) from `web/src/api/models/`, the F1 design tokens
(`web/src/theme/tokens.css` + `tokens.ts`) and the F1 copy catalog (`web/src/l10n/strings.ts`). The
editing-state library the program plan selects is the sheet-cycle feature's, not this one's
(Clarifications, 2026-10-16); the engine stays state-agnostic and exposes its seams instead.

**Storage**: none. The engine derives values in memory and persists nothing; it never touches
`localStorage`/`sessionStorage`/IndexedDB and holds no credential.

**Testing**: Vitest 5.0.5 with Testing Library 16.3.3 and `user-event` 14.6.8 (the F1 pins), flat
suites under `web/src/test/` with an `engine_*` prefix (the layout F1 established). Every control
branch, the grouping rules, the exclusive rules, the derivation and the marks are proven by rendering
and unit tests — per constitution III and VII — with edge cases first. The interface requirements are
verified through rendering tests of the components, because this feature mounts no surface
(Clarifications, 2026-10-16); the browser-level accessibility sweep and the golden/visual coverage of a
mounted surface belong to the features that mount the engine (the accessibility sweep is `F9-T007`,
`AD-014`). The F1 token-usage scan (`tokens_used.test.tsx`) and the F3 copy scan
(`l10n_strings.test.ts`) are extended to `web/src/engine/`, so the token and copy rules have a gate
here too.

**Target Platform**: browsers — the latest two versions of Chrome, Edge, Firefox and Safari, served
over HTTPS. The engine renders React DOM components and uses no browser global.

**Project Type**: web application (client) inside an existing Go monorepo; the backend is not read and
not modified — the feature is pure client rendering over the F2 models.

**Performance Goals**: not throughput; correctness and granularity. The derivation is pure and runs
once per render input, and the composition surfaces per-field values through the injected builder so a
single edit can rebuild one control rather than the whole form (the web expression of `AD-001`). The
engine issues 0 requests, so no latency or caching goal applies.

**Constraints**: the engine is stateless — no control owns domain state (ENG-13, ENG-25, ENG-26); no
user-facing literal outside `src/l10n/strings.ts` (ENG-27); every visual value from the shared tokens,
no ad-hoc hex/px (ENG-28); no DOM-injection sink and data rendered as text (ENG-29); identifiers,
comments and copy in English only; every `ENG-*` citation resolves through the spec lint already
covering `*.ts`/`*.tsx`.

**Scale/Scope**: 6 engine modules plus 1 stylesheet, ~6 engine test suites, 1 token-scan extension, 1
copy-scan extension, 1 l10n key group, 3 user stories (2 P1, 1 P2), 29 requirements.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The engine raises no `apperr` and performs no I/O; it consumes the F2 models and the dotted-path accessor, whose `PathFailure`/`DefensiveParseFailure` are the network layer's. No new backend error surface and no string-matched error. | PASS (not exercised) |
| II. Canonical Identifiers | No identifier is generated here; the engine handles dotted field paths and opaque template ids it never parses or re-formats. | PASS (not exercised) |
| III. Behavior Ships With Tests | Every requirement lands with a test in the same commit: the grouping/row rules, each control branch, the numeric guard, the exclusive rules, the seed/merge derivation and the marks are proven by rendering/unit tests, edge cases first (empty tree, group without leaves, single-member group, default-active group with no member, empty/whitespace/non-finite numeric input, empty cascade, `<script>` data, marked row with an empty value). | PASS |
| IV. English-Only Source | All identifiers, comments, test names and user-facing strings are English; every new string is a key in the English catalog. | PASS |
| V. Layered Architecture | Only `web/` gains code, and only inside the client: the new `engine` modules sit above the F2 models and the F1 `theme`/`l10n`, importing neither the network layer nor `catalog`/`sheet`. The Go layering is untouched. | PASS |
| VI. Security Gates Are Blocking | No gate changes and no gate weakened. No credential is attached or stored; the engine introduces no DOM-injection sink and renders data as text (ENG-29, SC-005) — the ESLint anti-sink rules (F1) cover the new `.tsx` sources. | PASS (maintained) |
| VII. Installable, Offline-Capable Web Client | The engine is the offline-capable client's core reading/rendering surface: it derives and renders everything from data already in memory, so an offline read has nothing to fetch. Interface requirements are verified by component rendering tests (III, VII). This feature mounts no surface, so the browser-level accessibility sweep is the mounting feature's (`F9-T007`, `AD-014`); the engine contributes the accessible role/name assertions the sweep relies on (SC-007). | PASS |
| Additional Constraints — Stack | The feature uses the stack the constitution names (React/Vite/TypeScript under `web/`), adds no dependency and adds no framework. | PASS |
| Additional Constraints — Traceability | `ENG-*` is defined in `spec.md` and the F1 lint already scans `*.ts`/`*.tsx`, so citations in the delivered sources are gated without further work. | PASS |
| Additional Constraints — Jira | This is pre-work planning; the implementing PR must reference its `LGN-\d+` card. Recorded for the PR, not a gate failure here. | PASS (noted) |
| Development Workflow | The documented PR sequence (`make lint-web && make test-web && make sec-scan`) already covers the new modules; no Makefile change is required. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps the engine free of the network
layer, of state and of copy: the grouping and the derivation are pure functions, the controls are
controlled components that report edits through callbacks, and the composition owns exactly one control
source. The one decision that could have drifted — mounting the field tree on a live surface and
measuring accessibility end-to-end — was resolved in favour of the clarified scope (engine plus
rendering tests, no surface), so the browser-level sweep stays with the feature that mounts the engine
(`F9-T007`) and no test-only route is added to production. No dependency is added, so the "no new
package" rule is not exercised.

## Project Structure

### Documentation (this feature)

```text
.specs/features/004-web-ui-engine/
├── spec.md              # Requirements (ENG-*)
├── plan.md              # This file
├── research.md          # Phase 0 — decisions and rejected alternatives
├── data-model.md        # Phase 1 — sections, rows, marks, draft, seams
├── quickstart.md        # Phase 1 — runnable validation guide
├── contracts/           # Phase 1 — one contract per engine interface
│   ├── section-derivation.contract.md
│   ├── field-control.contract.md
│   ├── exclusive-control.contract.md
│   └── draft-and-marks.contract.md
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
web/src/
├── engine/                         # the UI engine (NEW) — the port of desktop/lib/engine/
│   ├── field_grouping.ts           # buildSections + row derivation + label helpers (NEW)
│   ├── field_control.tsx           # SheetFieldControl: number/text/toggle/read-only (NEW)
│   ├── exclusive_control.tsx       # SheetExclusiveControl: one single-choice control (NEW)
│   ├── sheet_defaults.ts           # seedDefaults, seedFromSheet, mergeCascade, setPathValue (NEW)
│   ├── sheet_mark.ts               # SheetMark + the mark tint class (NEW)
│   ├── sheet_form.tsx              # SheetForm + control/rowMark seams (NEW)
│   └── engine.css                  # token-based row/section/control styling (NEW)
├── l10n/
│   └── strings.ts                  # engine.* copy keys added (calculated, none, modified, recalculated)
└── test/
    ├── engine_grouping.test.ts         # sections, rows, labels, table fallback (NEW)
    ├── engine_field_control.test.tsx   # one control per type + numeric guard (NEW)
    ├── engine_exclusive_control.test.tsx # single control, None rules, selection (NEW)
    ├── engine_defaults.test.ts         # seed + merge derivation, no mutation (NEW)
    ├── engine_form.test.tsx            # composition, seams, granularity, inert mode (NEW)
    ├── engine_marks.test.tsx           # Modified/Recalculated tint + Calculated yield (NEW)
    ├── engine_xss.test.tsx             # <script> data renders as text (NEW)
    ├── tokens_used.test.tsx            # extended: scans web/src/engine/ too
    └── l10n_strings.test.ts            # extended: engine.* keys + engine copy source scan
```

**Structure Decision**: the engine lives under `web/src/engine/`, the path the program plan already
reserves for it (Parte 6), so the ported modules map one-to-one onto their desktop counterparts
(`field_grouping`, `field_control`, `exclusive_control`, `sheet_defaults`, `sheet_mark`, `sheet_form`).
It is a sibling of `catalog/` and, later, `sheet/`: it imports the F2 models and the F1
`theme`/`l10n`, and nothing imports it yet — the sheet-cycle feature mounts `SheetForm`, and whichever
surface wants the read-only preview mounts it too. Styles stay in one `engine.css` owned by the engine
and expressed only in `--lg-*` tokens, so the F1 token-usage scan can inspect the directory the same
way it inspects `shell/` and `catalog/`. Tests stay flat in `web/src/test/` with an `engine_*` prefix,
matching the layout F1 established, and the two cross-cutting scans are extended rather than duplicated
so the token and copy rules keep one gate. No test-only route and no surface are added: the components
are rendered directly by the suites, which is the honest way to prove an engine that mounts nowhere
yet.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
