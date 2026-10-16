# Implementation Plan: Web Sheet Cycle

**Branch**: `005-web-sheet-cycle` | **Date**: 2026-10-16 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `.specs/features/005-web-sheet-cycle/spec.md`

**Program plan**: `docs/plan-web-pwa.md` (Parte B, feature F5) — the umbrella plan this feature
implements.

## Summary

Port the desktop sheet cycle (`desktop/lib/sheet/`, `desktop/lib/shell/shortcut_registry.dart`) to the
web stack as the first mounted editing surface: make the catalog's creation surface functional (one
`POST /sheets`, then open the created sheet **without** a second `GET`), open a sheet by identifier
through a one-request state machine with loading/missing/retryable situations, edit the sheet through an
in-memory store that records exactly the changed editable leaves and **saves only that delta** with one
`PATCH`, dry-run the pending edits through a stateless preview that marks the `modified` and
`recalculated` rows F4 can already render, back the pending delta up to IndexedDB on a cadence and
restore it on reopen, and guard unsaved edits behind a discard confirmation on `F5` and on leaving while
throttling Save and Preview to three requests per rolling minute. The feature consumes the delivered F2
API client (extended with the preview operation) and the F4 UI engine (mounted through its builder
seams, preserving `AD-001`). The backend is not modified; no credential is attached or stored; nothing
is rendered as markup; every string resolves through the catalog and every visual value through the
shared tokens. `idb` is the one new runtime dependency (already resolved in the tree transitively);
a small per-path external store built on `useSyncExternalStore` supplies the `AD-001` granularity, so no
state library is added (recorded deviation from the plan's stack table, [research.md R2](./research.md)).

## Technical Context

**Language/Version**: TypeScript 5.9.3 with the F1 strictness settings (`strict`, `noUnusedLocals`,
`noUnusedParameters`, `noFallthroughCasesInSwitch`, `isolatedModules`, `resolveJsonModule`,
`@typescript-eslint/consistent-type-imports`) on React 19.3.1 / Vite 8.3.4, Node.js 24 LTS, with
react-router-dom 7.18.4 and TanStack Query 5.104.3 (the F1/F3 pins). Reading `web/package.json` and
running `node --version` confirms 24.x and the versions above; no toolchain change is introduced.

**Primary Dependencies**: one new **runtime** dependency, **`idb` 7.1.1** — the program plan's chosen
IndexedDB wrapper (Parte 3), already resolved in the installed tree through `workbox-expiration`, so the
declaration adds no new resolved package. One new **dev-only** dependency, **`fake-indexeddb`**, needed
because jsdom ships no IndexedDB and constitution III requires the `idb` adapter to be covered; it never
enters the runtime bundle. No editing-state library is added: the per-path subscription the plan lists
as "Zustand + selectors (or Jotai por-path)" is provided by React's `useSyncExternalStore` over a small
external store, wired into the F4 `SheetForm` **builder seams** ([research.md R2](./research.md)). The
feature reuses the F2 `ApiClient` (extended with `previewSheet`), the F2 sheet/template models and
`valueAtPath`, the F4 engine derivation (`seedFromSheet`, `mergeCascade`, `setPathValue`,
`buildSections`) and its mark rendering, and the F1 tokens, copy catalog, router and error mapper.

**Storage**: the browser's **IndexedDB** through `idb`, behind a typed `SheetStore` port
(`web/src/library/sheet_store.ts`). Two object stores — `sheets` (the offline cache, a versioned
`{ schemaVersion, id, sheet }` envelope) and `drafts` (the unsaved pending delta, a versioned
`{ schemaVersion, id, delta }` envelope). A record that is corrupt, of an unsupported version or names
another id reads as `null`; **no credential is ever written** (WSC-30, WSC-31). The app builds the
`idb` adapter; tests inject an in-memory double through the same `AppProviders` seam the `ApiClient`
uses, and the adapter itself is exercised under `fake-indexeddb`.

**Testing**: Vitest 5.0.5 with Testing Library 16.3.3 and `user-event` 14.6.8 (the F1 pins), flat suites
under `web/src/test/` with a `sheet_*` / `library_*` / `shell_*` prefix, edge cases first (constitution
III). Every open/failure/missing situation, the create flow, the delta rules, the preview and its marks,
the draft cadence and restore, the guards and the throttle are proven by unit and rendering tests
driving `fake_transport.ts` and the in-memory store double; the `idb` adapter is covered under
`fake-indexeddb`. The mounted-surface accessibility outcome (SC-010) is measured by a Playwright case
against the real build with `@axe-core/playwright`, following `AD-014` — no axe dependency is added to
the Vitest tree. The F1 token scan (`tokens_used.test.tsx`) and the F3 copy scan (`l10n_strings.test.ts`)
are extended to `web/src/sheet/` and `web/src/library/`, so the token and copy rules keep one gate.

**Target Platform**: browsers — the latest two versions of Chrome, Edge, Firefox and Safari, served
over HTTPS. The screen renders React DOM components; it uses the browser's fetch, history routing and
IndexedDB, and no browser global beyond that.

**Project Type**: web application (client) inside an existing Go monorepo; the backend is not read and
not modified — the feature consumes the F2 client as built, extended with one operation.

**Performance Goals**: not throughput; correctness, granularity and politeness. One edit rebuilds one
control rather than the whole form (the per-path subscription, the web expression of `AD-001`), and Save
and Preview are each capped at three requests per rolling 60-second window (WSC-38). The open issues one
request per sheet and one per template; a created sheet is not re-fetched (WSC-02).

**Constraints**: no credential in `localStorage`/`sessionStorage`/IndexedDB (WSC-31, constitution VI);
no DOM-injection sink, data rendered as text (WSC-35); no user-facing literal outside `src/l10n` (WSC-32,
SC-006); every visual value from the shared tokens, no ad-hoc hex/px (WSC-33); identifiers, comments and
copy in English only; every `WSC-*` citation resolves through the spec lint already covering `*.ts`/
`*.tsx`. No new CI gate is introduced (`idb` is already vetted; `fake-indexeddb` is dev-only).

**Scale/Scope**: 8 new client modules (3 under `web/src/sheet/`, 4 under `web/src/library/`, 1 under
`web/src/shell/`) plus 1 stylesheet, one extended F2 operation (`previewSheet`), one wired creation
surface and one new route, ~10 test suites plus one extended scan pair and one Playwright case, a group
of l10n keys, 5 user stories (2 P1, 2 P2, 1 P3) and 39 requirements.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|---|---|---|
| I. Typed Errors at Every Boundary | The feature raises no `apperr` and adds no backend boundary; it consumes the F2 client, whose `DomainFailure`/`DefensiveParseFailure` it maps to copy through the existing `error_mapper`. No new error surface and no string-matched error. | PASS (not exercised) |
| II. Canonical Identifiers | No identifier is generated here; the feature reads the opaque sheet id from the create response and the route, never parses or re-formats it. | PASS (not exercised) |
| III. Behavior Ships With Tests | Every requirement lands with a test in the same commit: the open situations, the create flow, the delta/exclusive/baseline rules, the preview and marks, the draft cadence/restore/eviction, the guards and the throttle are proven by unit and rendering tests, edge cases first (nil/empty delta, reverted edit, derived path, missing sheet, corrupt draft, empty window, in-flight no-op, `<script>` data). | PASS |
| IV. English-Only Source | All identifiers, comments, test names and user-facing strings are English; every new string is a key in the English catalog. | PASS |
| V. Layered Architecture | Only `web/` gains code, and only inside the client: `sheet/` and `library/` sit above `engine/` and the F2 models; `sheet/` imports the engine and the library, and nothing imports `sheet/` except the router. The Go layering is untouched. | PASS |
| VI. Security Gates Are Blocking | No gate changes and no gate weakened. No credential is attached or stored (WSC-31, SC-004); the screen introduces no DOM-injection sink and renders data as text (WSC-35, SC-007) — the ESLint anti-sink rules (F1) cover the new `.tsx` sources. The one new runtime package (`idb`) is already resolved and vetted in the tree; the dev-only double does not ship. | PASS (maintained) |
| VII. Installable, Offline-Capable Web Client | The screen is deep-linkable (`/sheets/:id`) and writes the offline cache and draft the PWA feature later reads; interface requirements are verified by rendering tests and the browser-level accessibility sweep this feature owns (SC-010), because it is the feature that mounts the engine (F4's hand-off). | PASS |
| Additional Constraints — Stack | The feature uses the stack the constitution names (React/Vite/TypeScript under `web/`), adds the plan-chosen `idb` and one dev-only test double, and adds no framework or state library. | PASS |
| Additional Constraints — Traceability | `WSC-*` is defined in `spec.md` and the F1 lint already scans `*.ts`/`*.tsx`, so citations in the delivered sources are gated without further work. | PASS |
| Additional Constraints — Jira | This is pre-work planning; the implementing PR must reference its `LGN-\d+` card. Recorded for the PR, not a gate failure here. | PASS (noted) |
| Development Workflow | The documented PR sequence (`make lint-web && make test-web && make sec-scan`) covers the new modules; the Playwright a11y case runs in the existing e2e suite. No Makefile change is required. | PASS |

No principle is waived; **Complexity Tracking** stays empty.

**Post-design re-check (after Phase 1)**: still PASS. The design keeps the network layer the F2 client's,
the persistence behind a port, and the edit state in one store with no library: the open state owns its
requests, the edit store owns the delta/preview lifecycle, the library owns the versioned envelopes, and
the shell owns the shortcut map. The two decisions that could have drifted were resolved in research: no
state library is added (`R2`), so the "no new package without strict necessity" rule is honored and only
the plan-chosen `idb` is declared; and `F5` refreshes the sheet's template behind a discard
confirmation rather than the browser default (`R8`), so the shortcut stays sheet-scoped and no surface
is reached into. The accessibility measurement is owned here (SC-010) and reuses F1's axe dependency, so
no gate is added.

## Project Structure

### Documentation (this feature)

```text
.specs/features/005-web-sheet-cycle/
├── spec.md              # Requirements (WSC-*)
├── plan.md              # This file
├── research.md          # Phase 0 — decisions and rejected alternatives
├── data-model.md        # Phase 1 — edit/open state, envelopes, store port, throttle
├── quickstart.md        # Phase 1 — runnable validation guide
├── contracts/           # Phase 1 — one contract per feature interface
│   ├── sheet-open.contract.md
│   ├── sheet-edit.contract.md
│   ├── sheet-store.contract.md
│   └── shortcut-registry.contract.md
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 (/speckit.tasks — not created by /speckit.plan)
```

### Source Code (repository root)

```text
web/src/
├── api/
│   └── client.ts                   # extended: previewSheet(id, delta) (WSC-21)
├── sheet/                          # the open/edit surface (NEW)
│   ├── sheet_open.ts               # useSheetOpen: open-by-id state machine (NEW)
│   ├── sheet_edit.ts               # the edit store: delta, save, preview, marks, throttle (NEW)
│   ├── sheet_screen.tsx            # header, engine mount, footer, guards (NEW)
│   └── sheet.css                   # token-only styles (NEW)
├── library/                        # local persistence (NEW)
│   ├── sheet_store.ts              # SheetStore port + versioned envelopes + memory double (NEW)
│   ├── idb_sheet_store.ts          # the idb adapter — the only importer of `idb` (NEW)
│   ├── sheet_library.ts            # cache read/write/evict on top of the port (NEW)
│   └── draft_backup.ts             # draft read/write/delete + injectable cadence (NEW)
├── shell/
│   └── shortcut_registry.ts        # action → key map + dispatcher (Ctrl+S, F5) (NEW)
├── catalog/
│   ├── create_sheet_dialog.tsx     # wired: create + navigate (WSC-01…WSC-04)
│   └── queries.ts                  # sheet query keys + cache seed on create (extended)
├── app/
│   ├── routes.ts                   # add /sheets/:id (deep-linkable) (extended)
│   ├── router.tsx                  # map /sheets/:id → SheetScreen (extended)
│   └── providers.tsx               # provide the SheetStore (extended)
├── l10n/
│   └── strings.ts                  # sheet-screen copy keys added (WSC-32)
└── test/
    ├── sheet_open.test.tsx             # open situations + cache write (NEW)
    ├── create_sheet_flow.test.tsx      # create → navigate without a second GET (NEW)
    ├── sheet_edit.test.ts              # delta/exclusive/baseline rules (NEW)
    ├── sheet_preview.test.tsx          # preview, marks, no baseline advance (NEW)
    ├── draft_backup.test.ts            # cadence, restore, eviction, corrupt (NEW)
    ├── library_store.test.ts           # versioned envelopes + idb adapter (fake-indexeddb) (NEW)
    ├── sheet_guards.test.tsx           # Ctrl+S, F5, leave guard, throttle (NEW)
    ├── sheet_screen_states.test.tsx    # mounted screen states: loading/loaded/footer (NEW)
    ├── render.tsx                      # extended: mounts an injected SheetStore double
    ├── tokens_used.test.tsx            # extended: scans src/sheet/ + src/library/
    └── l10n_strings.test.ts            # extended: sheet-* keys + source scan
web/e2e/
    └── sheet.spec.ts                   # mounted-surface axe sweep (SC-010) (NEW)
```

**Structure Decision**: the surface lives under `web/src/sheet/` and the persistence under
`web/src/library/`, the paths the program plan already reserves (Parte 6), so the ported units map onto
their desktop counterparts (`sheet_open_provider`, `sheet_edit_provider`, `sheet_screen`,
`sheet_library_store`, `draft_backup_store`) and the shell seam keeps its name
(`web/src/shell/shortcut_registry.ts`). The split is deliberate: `sheet/` owns the open request and the
in-memory edit lifecycle, `library/` owns the versioned local records behind the `SheetStore` port, and
`shell/` owns the one action → key map. The screen first renders the field tree read-only (the engine's inert data mode) and switches to the builder seams when the edit store lands (`WSC-08`, US1 → US2), so the open story owns no edit state. `api/client.ts` gains only the preview operation; `catalog/`
gains only the create wiring; `app/` gains only the route and the store provider. Styles stay in one
`sheet.css` expressed only in `--lg-*` tokens, so the F1 token-usage scan can inspect the new
directories the same way it inspects `shell/`, `catalog/` and `engine/`. Tests stay flat in
`web/src/test/` with the feature prefixes, and the two cross-cutting scans plus the Playwright axe case
are extended rather than duplicated so the token, copy and accessibility rules keep one gate each.

## Complexity Tracking

> No constitution violations; nothing to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| — | — | — |
