# Implementation Plan: Web Catalog

**Branch**: `003-web-catalog` | **Date**: 2026-10-16 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/003-web-catalog/spec.md`

**Program plan**: `docs/plan-web-pwa.md` (Parte B, feature F3) — the umbrella plan this feature
implements.

## Summary

Give the shell its first real content: the root route renders the template catalog, and activating an
entry opens a creation surface that loads that template's detail. The catalog is server state, so it
is owned by TanStack Query — the server-state choice the program plan already made — through one
provider that builds the F2 API client over the F2 fetch transport and the resolved base address, and
hands a query client to the screens. The catalog loads **once per app load** and is cached in memory:
query retries and window-focus refetching are disabled, because the F2 client already bounds its own
reads with a single retry and the plan gives the refresh shortcut to F5. The detail is the opposite
policy — one request per opening, no reuse — so opening a template never shows a previous template's
data. A catalog or detail request that the server refuses, or a response the client cannot read, each
become a distinct inline state resolved through the F2 error mapper and the translation catalog; a
missing template stays inside the open surface. The creation surface is an accessible overlay
(`role="dialog"`, focus moved in on open and returned to the triggering entry on close) carrying the
sheet-name field and its trim/required validation, which gates a primary action that issues **no**
mutation — creation is F5. Nothing is persisted, no credential is attached, and no visual value or
user-facing string is authored outside the shared tokens and the translation catalog.

## Technical Context

**Language/Version**: TypeScript 5.9.3 with the F1 strictness settings (`strict`, `noUnusedLocals`,
`noUnusedParameters`, `noFallthroughCasesInSwitch`, `isolatedModules`, `resolveJsonModule`,
`@typescript-eslint/consistent-type-imports`) on React 19.3.1 / Vite 8.3.4, Node.js 24 LTS. The version
is the one the delivered F1 toolchain pins — no toolchain change is introduced here.

**Primary Dependencies**: one runtime dependency added — `@tanstack/react-query` 5.104.3 (peer
`react: ^18 || ^19`, satisfies 19.3.1). It is the program plan's server-state choice (Parte 3, "Estado
servidor: TanStack Query"), and F3-T001 names it ("Queries TanStack"). React Router 7.18.4
(already installed), the F2 API client and its fetch transport are consumed unchanged. No new test
dependency: the suite uses Vitest 5.0.5, Testing Library 16.3.3 and `user-event` 14.6.8, already in F1.
MSW stays out for the same reason F2 kept it out — the transport is the injectable seam and call
counts are asserted through it directly (R9).

**Storage**: none. The catalog cache lives in memory in the query client and is never persisted; the
sheet library and drafts belong to F5. No credential is stored (the surface attaches none) and no
token reaches `localStorage`/`sessionStorage`/IndexedDB.

**Testing**: Vitest 5.0.5 with the F1 render helper extended to inject the API client and a fresh
`QueryClient` per test (retries off, `gcTime: 0`), a fake transport for every screen test, and
Testing Library queries by role/name/test id. The catalog states, the surface states, the name
validation, the keyboard flow and the "zero mutating requests" rule are all proven by rendering
tests, per constitution III and VII. Accessibility is verified by a Playwright case over the real
build using `@axe-core/playwright` 4.14.0 (already installed by F1) asserting zero critical
violations on the catalog and the creation surface (SC-005), on top of the ARIA-structure assertions
in the rendering suites; the broader axe sweep of every main flow is F9-T007. The F1
shell/routing/e2e assertions that pinned the empty-workspace state are updated to the catalog (no
assertion is weakened).

**Target Platform**: browsers — the latest two versions of Chrome, Edge, Firefox and Safari, served
over HTTPS; the screens use React DOM, the F2 client and no browser global beyond the transport
adapter's `fetch`.

**Project Type**: web application (client) inside an existing Go monorepo; the backend is read, never
modified — the feature only consumes the two operations F2 already exposes.

**Performance Goals**: one catalog request per app load and one detail request per opening, not
throughput. The catalog is not re-requested on remount, on window focus, or by any background
refetch; a second trigger while a request is in flight is deduplicated by the query client rather
than issued (WCT-02, WCT-08, WCT-21).

**Constraints**: no mutating request anywhere in the feature (WCT-25); no user-facing literal outside
`src/l10n/strings.ts` (WCT-26); every visual value from the shared tokens, no ad-hoc hex/px (WCT-28);
keyboard-only operability with focus moved in and returned (WCT-27); identifiers, comments and copy in
English only; every `WCT-*` citation resolves through the spec lint already covering `*.ts`/`*.tsx`.

**Scale/Scope**: 1 query module, 2 queries (catalog + per-template detail), 1 provider, 1 catalog
screen, 1 creation surface, 1 render helper change, 1 l10n key group, 3 user stories (all P1), 28
requirements.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The screens branch on the F2 failure taxonomy, never on strings: a refusal is a `DomainFailure` (`kind`) and an unreadable response stays a `DefensiveParseFailure`, both rendered through `failureMessage`/`parseFailureMessage` (WCT-06, WCT-09, WCT-15, WCT-17, WCT-18). No new backend error surface and no string-matched error. | PASS |
| II. Canonical Identifiers | No identifier is generated here. The template identifier is opaque, arrives from the catalog response and is passed to `getTemplate`, which percent-encodes it; it is never parsed or re-formatted. | PASS (not exercised) |
| III. Behavior Ships With Tests | Every requirement lands with a test in the same commit: the five catalog render situations, the five surface situations, the name validation and the zero-mutation rule are proven by rendering tests; edge cases (empty list, empty entry name, whitespace-only name, malformed payload, missing template, reopen) come first. | PASS |
| IV. English-Only Source | All identifiers, comments, test names and user-facing strings are English; every new string is a key in the English catalog. | PASS |
| V. Layered Architecture | Only `web/` gains code; the Go layering (`app → sheet → template → engine → shared`) is untouched. Inside the client the new code sits above the F2 client (`catalog` and `app/providers` consume `api` and `l10n`; `api` imports neither), so the network layer keeps its direction. | PASS |
| VI. Security Gates Are Blocking | No gate changes: F1's `ci-web` already runs the suite, `sca` sees the one new dependency, and this feature must not weaken any gate. No credential is attached or stored, and the screens introduce no DOM injection sink — data (template names, sheet name) renders as text only. | PASS (maintained) |
| VII. Installable, Offline-Capable Web Client | The catalog is the app shell's primary reading surface and the states this feature renders are the ones the offline-capable client degrades into (the `offline` kind is already one of them). Accessibility is part of "done": no critical automated violations on either surface. Deep-linking the creation surface is explicitly out of scope (spec Assumptions), so the shallow-navigation contract is preserved, not broken. | PASS |
| Additional Constraints — Stack | The feature uses the stack the constitution names (React/Vite/TypeScript under `web/`), adds the one dependency the program plan already selected, and adds no framework. | PASS |
| Additional Constraints — Traceability | `WCT-*` is defined in `spec.md` and the F1 lint already scans `*.ts`/`*.tsx`, so citations in the delivered sources are gated without further work. | PASS |
| Additional Constraints — Jira | This is pre-work planning; the implementing PR must reference its `LGN-\d+` card. Recorded for the PR, not a gate failure here. | PASS (noted) |
| Development Workflow | The documented PR sequence (`make lint-web && make test-web && make sec-scan`) already covers the new modules; no Makefile change is required. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design kept the network layer untouched and
put every decision the feature owns above it: one provider constructs the client and the query client,
the screens render server state through the query client, and every branch resolves copy through the
existing mapper and catalog. The one design decision that could have drifted — owning a native
`<dialog>` element — was resolved in favour of an accessible overlay, because the platform dialog
cannot be driven by the test environment the constitution requires tests to run in (R4), and the
overlay delivers the same Esc/focus/ARIA contract with an assertable implementation. The one
dependency added is the program plan's own server-state choice, recorded in `web/AGENTS.md` as an
inherited decision rather than a new one.

## Project Structure

### Documentation (this feature)

```text
.specs/features/003-web-catalog/
├── spec.md              # Requirements (WCT-*)
├── plan.md              # This file
├── research.md          # Phase 0 — decisions and rejected alternatives
├── data-model.md        # Phase 1 — query state, render states, surface state
├── quickstart.md        # Phase 1 — runnable validation guide
├── contracts/           # Phase 1 — server state, catalog render state, creation surface
│   ├── server-state.contract.md
│   ├── catalog-render-state.contract.md
│   └── creation-surface.contract.md
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
web/src/
├── app/
│   ├── providers.tsx        # builds the API client + query client; the app's shared context (NEW)
│   └── router.tsx           # root route renders the catalog screen instead of the empty workspace
├── catalog/
│   ├── queries.ts           # query keys + the catalog and detail query hooks (NEW)
│   ├── catalog_screen.tsx   # loading / list / empty / error / parse-error states (NEW)
│   └── create_sheet_dialog.tsx  # the creation surface + sheet-name validation (NEW)
├── shell/
│   ├── shell.css            # the temporary .lg-empty* rules are removed with the component
│   └── empty_workspace.tsx  # DELETED: its own doc comment declares it temporary until the catalog lands
├── l10n/
│   └── strings.ts           # catalog.* and sheet.* copy keys added
└── test/
    ├── render.tsx           # render helper extended to inject the client + a fresh query client
    ├── catalog_screen.test.tsx       # the five catalog render situations (NEW)
    ├── create_sheet_dialog.test.tsx  # the surface states, validation and zero-mutation rule (NEW)
    ├── catalog_queries.test.tsx      # one request per load/opening; no focus refetch; dedupe (NEW)
    ├── catalog_keyboard.test.tsx     # Esc, focus in/out, no keyboard trap (NEW)
    ├── shell.test.tsx                # updated: root route asserts the catalog, not the empty state
    ├── routing.test.tsx              # updated: root route asserts the catalog
    └── shell_keyboard.test.tsx       # updated: nav label/focus expectation

web/e2e/
└── catalog.spec.ts                 # real-build axe pass: zero critical violations (NEW)
```

**Structure Decision**: the screens live under `web/src/catalog/`, the path the program plan already
reserves for them (Parte 6), with the query module as their sibling so all three files that describe
one screen's server state sit together. The shared context lands in `web/src/app/providers.tsx` — the
file F1 deferred ("the module lands with the first feature that genuinely needs shared context",
`web/AGENTS.md`) — because this is the first feature whose screens need the client and the query
client. Tests stay flat in `web/src/test/` with a `catalog_*` prefix, matching the layout F1
established; the render helper is extended rather than duplicated so every suite injects its own
client. `empty_workspace.tsx` is deleted with the `.lg-empty*` rules it owned, because its own doc
comment declares it "until the catalog feature lands" — it is a temporary stand-in, not a designed
shell state the feature preserves, so keeping it would leave dead code. The F1 assertions that pinned
the empty state on `/` are rewritten to pin the catalog instead — the same assertions (shell order, no
placeholder rows, deep link), a truthful subject. The shell suites' "no placeholder rows" edge stays
meaningful by rendering the catalog's empty state, where no row may be fabricated.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
