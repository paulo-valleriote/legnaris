---
description: "Task list for the Web Catalog feature"
---

# Tasks: Web Catalog

**Input**: Design documents from `.specs/features/003-web-catalog/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED here. The constitution's "behavior ships with tests" principle, the feature's own
WCT-01…WCT-28, and the repo's edge-case-first rule all demand test tasks; they are written first and
must fail before the implementation they cover.

**Organization**: tasks are grouped by user story so each story is independently implementable,
testable, and demonstrable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks)
- **[Story]**: US1 … US3, per spec.md
- Every task names its exact file path

## Path Conventions

The screens live under `web/src/catalog/`, the shared context under `web/src/app/`, the copy catalog
under `web/src/l10n/`, and every unit/rendering test under `web/src/test/` with a `catalog_*` prefix,
matching the flat layout F1 established. The single browser-level accessibility case lives under
`web/e2e/`, next to the F1 specs it extends.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the dependency and copy keys every story consumes.

- [ ] T001 [P] Add `@tanstack/react-query` 5.104.3 to `web/package.json` and install it — the server-state
      library the program plan selects (Parte 3) and F3-T001 names; it is the only runtime dependency
      this feature adds, recorded as an inherited plan decision in `web/AGENTS.md` (T022)
- [ ] T002 [P] Extend the translation catalog `web/src/l10n/strings.ts` with the `catalog.*` and `sheet.*`
      keys listed in `data-model.md` (title, loading, empty, retry, open hint, dialog title, name label,
      placeholder, required tooltip, create, close, detail loading/retry) and extend
      `web/src/test/l10n_strings.test.ts` so every new key is asserted to resolve to a non-empty string
      and distinct from its neighbours (WCT-26)

**Checkpoint**: `cd web && npm test` passes with the dependency and the new keys in place.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the shared context that builds the F2 client and the query client, and the render helper
that injects them.

**⚠️ CRITICAL**: US1, US2 and US3 all consume this module and helper.

### Tests for the foundation ⚠️ (write first; they must fail before the implementation)

- [ ] T003 [P] Provider test `web/src/test/app_providers.test.tsx` (WCT-02, WCT-12): `AppProviders` builds
      one API client over the injected transport and exposes it through the client context; consuming
      `useApiClient` outside the provider raises the documented error; the query client it creates has
      query retries disabled, so the retry policy is the F2 client's alone

### Implementation for the foundation

- [ ] T004 Implement `web/src/app/providers.tsx` per `contracts/server-state.contract.md`: `AppProviders`
      resolves the base address with `resolveApiConfigFromBuild`, builds the client with
      `createApiClient({ baseUrl, transport: createFetchTransport(fetch) })`, creates one `QueryClient`
      with `retry: false`, and provides both through `ApiClientProvider` and `QueryClientProvider`, with
      `useApiClient` reading the client from context (never a module singleton)
- [ ] T005 Extend the render helper `web/src/test/render.tsx` to accept an injected `ApiClient` and mount
      `AppProviders` with a fresh `QueryClient` per test (`retry: false`, `gcTime: 0`); when no client is
      given, default to a fake transport whose catalog succeeds with an empty list, so the existing
      suites stay meaningful (test infrastructure — no behavior of its own)

**Checkpoint**: a screen can be rendered with a scripted client, and no screen reaches a global.

---

## Phase 3: User Story 1 - Browse the template catalog (Priority: P1) 🎯 MVP

**Goal**: the root route lists the templates the service exposes, with loading, list, empty, error and
parse-error states that are each distinct.

**Independent Test**: with a scripted transport, drive the catalog through a slow response, two
entries, an empty list, a failure, and a malformed payload, and confirm five distinct render
situations plus a call counter proving exactly one request per app load.

### Tests for User Story 1 ⚠️

- [ ] T006 [US1] Catalog render test `web/src/test/catalog_screen.test.tsx` (WCT-03, WCT-04, WCT-05,
      WCT-06, WCT-09, WCT-10), edge cases first: while the request is in flight the loading copy shows
      and the empty copy does not; a two-entry response renders two rows in the response order with
      their names; an empty response renders exactly one line and no decorative art; a failure response
      renders the copy from `failureMessage(kind)` plus a Retry action; a malformed payload renders
      `parseFailureMessage()` plus Retry and neither a blank screen nor a partial list; an entry whose
      name is `""` renders without throwing and keeps the layout intact
- [ ] T007 [US1] Query-policy test `web/src/test/catalog_queries.test.tsx` (WCT-02, WCT-07, WCT-08): the
      first mount issues exactly one request through the fake transport; remounting the route and firing
      a window focus event issue none; Retry from the error state issues exactly one further request;
      a second activation while that request is in flight issues none (the in-flight request is joined,
      not duplicated)
- [ ] T008 [US1] Update the F1 suites `web/src/test/shell.test.tsx`, `web/src/test/routing.test.tsx` and
      `web/e2e/shell.spec.ts` to assert the catalog on the root route: the shell order and the deep-link
      assertions keep their shape with the catalog as the subject, and the "no placeholder rows" edge
      renders the catalog's empty state and still asserts no row/listitem/table is fabricated (no
      assertion is weakened — the subject moves to the feature that owns it)

### Implementation for User Story 1

- [ ] T009 [US1] Implement the catalog query in `web/src/catalog/queries.ts`: the query-key module
      (`['templates']`, `['template', id]`) and `useCatalog()` calling `listTemplates` through
      `useApiClient`, configured `staleTime: Infinity`, `retry: false`, `refetchOnWindowFocus: false`,
      `refetchOnMount: false`, `refetchOnReconnect: false` per `contracts/server-state.contract.md`
      (WCT-02, WCT-08)
- [ ] T010 [US1] Implement `web/src/catalog/catalog_screen.tsx` per
      `contracts/catalog-render-state.contract.md`: resolve exactly one `CatalogRenderPhase` from the
      query result and render it — the loading copy, N rows in the response order (name, with the
      identifier as a secondary line), the single empty line, or the mapped/parse-failure copy with a
      Retry action — with every string from `message(key)` and every visual value from the shared tokens
      (WCT-03…WCT-06, WCT-09, WCT-10, WCT-26, WCT-28)
- [ ] T011 [US1] Wire the root route: render `CatalogScreen` instead of `EmptyWorkspace` in
      `web/src/app/router.tsx`, delete `web/src/shell/empty_workspace.tsx` (its doc comment declares it
      temporary until the catalog lands), and remove the now-unused `.lg-empty*` rules from
      `web/src/shell/shell.css` (WCT-01)
- [ ] T012 [P] [US1] Extend `web/src/test/tokens_used.test.tsx` to scan `web/src/catalog/` for literal hex
      and px values, so every visual value in the new screens is proven to come from the shared tokens
      (WCT-28)

**Checkpoint**: the catalog is demonstrable with a fake transport and the five situations are each
asserted; the creation surface does not exist yet.

---

## Phase 4: User Story 2 - Open a template's creation surface (Priority: P1)

**Goal**: activating a catalog entry opens an overlay that loads that template's detail and shows it,
with inline states for loading, failure, a missing template and drift, and a dismiss that returns to
the catalog unchanged.

**Independent Test**: activate two different entries in turn and confirm each surface loads its own
identifier's detail; drive a slow response, a failure, a missing template and a malformed payload and
confirm the inline states; dismiss with Esc and confirm the list is unchanged and no catalog request
was repeated.

### Tests for User Story 2 ⚠️

- [ ] T013 [US2] Creation-surface detail test `web/src/test/create_sheet_dialog.test.tsx` (WCT-11…WCT-18,
      WCT-21), edge cases first: activating an entry opens the surface and issues exactly one detail
      request for that identifier; while it is in flight the detail area shows the loading copy and no
      previously opened template's detail; success shows the template's display name (an empty name
      rendering as empty text without throwing); a failure shows the mapped copy inline and keeps the
      surface open; `NOT_FOUND` shows the missing-template copy inline without closing or crashing; a
      malformed payload shows the parse-failure copy inline and never a half-filled detail; Retry
      re-requests exactly once; opening the same template again issues exactly one further request and
      starts in the loading state; opening two different entries loads each one's own detail
- [ ] T014 [US2] Keyboard/navigation test `web/src/test/catalog_keyboard.test.tsx` (WCT-19, WCT-20,
      WCT-27): activating an entry moves focus into the surface; Esc and the close action dismiss it and
      return focus to the triggering entry; the catalog list is unchanged and no catalog request is
      issued; the URL is unchanged (the surface is never a route), and no control traps focus inside the
      surface

### Implementation for User Story 2

- [ ] T015 [US2] Implement the detail query in `web/src/catalog/queries.ts`: `useTemplateDetail(id)`
      calling `getTemplate` through `useApiClient`, keyed `['template', id]` with `staleTime: 0`,
      `gcTime: 0`, `retry: false`, `refetchOnWindowFocus: false`, so each opening starts fresh and no
      previous template's detail can be shown (WCT-12, WCT-13, WCT-21)
- [ ] T016 [US2] Implement `web/src/catalog/create_sheet_dialog.tsx` per
      `contracts/creation-surface.contract.md`: an accessible overlay (`role="dialog"`,
      `aria-modal="true"`, labelled by its title, no native `<dialog>` — research R5) whose detail area
      resolves exactly one `DetailRenderPhase` (loading, the display name, the mapped failure copy
      inline, the missing-template copy inline, or the parse-failure copy inline) with a close action
      and a Retry action that re-requests once (WCT-11…WCT-18, WCT-27)
- [ ] T017 [US2] Hook the surface to the catalog in `web/src/catalog/catalog_screen.tsx`: activating an
      entry records the open `templateId` and the triggering element, renders the surface over the
      catalog, returns focus to the recorded element on close, and leaves the catalog list and its cache
      untouched (WCT-11, WCT-19, WCT-20)

**Checkpoint**: a template can be chosen and its detail shown, with the surface opening and dismissing
without reloading the catalog; the sheet-name field does not exist yet.

---

## Phase 5: User Story 3 - Name the sheet in the creation surface (Priority: P1)

**Goal**: the surface ships the sheet-name field and its validation, gating a primary action that
issues no mutation.

**Independent Test**: open the surface, confirm the field starts empty with a localized label and
placeholder and the action is disabled with a tooltip; type spaces and confirm it stays disabled; type
a name and confirm it enables; activate the action and confirm zero requests were made.

### Tests for User Story 3 ⚠️

- [ ] T018 [US3] Name-validation test additions in `web/src/test/create_sheet_dialog.test.tsx` (WCT-22,
      WCT-23, WCT-24, WCT-25), edge cases first: the field starts empty with a localized label and
      example placeholder; the primary action is disabled for `""` and `"   "` and exposes a tooltip
      stating a sheet name is required; it is enabled for `"  Aragorn  "` and `"Aragorn"` (validation
      reads the trimmed value); activating it with a valid name issues zero transport attempts, and
      typing issues none either

### Implementation for User Story 3

- [ ] T019 [US3] Implement the sheet-name field and validation in
      `web/src/catalog/create_sheet_dialog.tsx`: hold the raw value in local state, derive
      `trimmedName = value.trim()`, set the primary action's `disabled` to `trimmedName === ''`, expose
      the required tooltip and an `aria-describedby` note from `sheet.nameRequired`, and make the
      action's handler issue no request — creation belongs to F5 (WCT-22…WCT-25)

**Checkpoint**: the surface's one input is validated and the action cannot mutate anything.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T020 [P] Add the accessibility case `web/e2e/catalog.spec.ts` (SC-005, WCT-27): load the real build
      with Playwright, open the catalog and the creation surface, and assert zero critical `axe-core`
      violations on each using the `@axe-core/playwright` dependency F1 already installs
- [ ] T021 [P] Add the catalog copy source scan (a case in `web/src/test/l10n_strings.test.ts`): no
      user-facing literal and no `api.error.` string literal exists in `web/src/catalog/`, every
      `message('…')` key referenced there resolves to a catalog entry (so removing a referenced key turns
      the suite red — SC-004), and every string is therefore proven to resolve through the catalog
      (WCT-26, SC-003, SC-004)
- [ ] T022 [P] Update `web/AGENTS.md` (the `@tanstack/react-query` dependency as an inherited plan
      decision, the deleted empty workspace, the accessible-overlay-instead-of-`<dialog>` decision) and
      `web/README.md` (how to run the catalog suites and the accessibility case)
- [ ] T023 Run the full gate set from `web/` (`npm run format:check`, `npm run lint`, `npm run typecheck`,
      `npm test`, `npm run build`, `npm run test:e2e`) and `make lint-specs` from the repo root, and
      confirm every `WCT-*` citation in the delivered sources resolves to `spec.md`
- [ ] T024 Run the nine `quickstart.md` scenarios and record the outcome with the feature
- [ ] T025 Record any plan-versus-delivered deviation (dependency, pin or path that changed) in
      `web/AGENTS.md` under the deviations section rather than weakening an assertion (constitution III)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — starts immediately.
- **Foundational (Phase 2)**: depends on T001 (the query library) and T002 (the copy keys); blocks every
  story.
- **US1 (Phase 3)**: depends on Foundational; delivers the MVP.
- **US2 (Phase 4)**: depends on Foundational and on US1's `queries.ts` (T015 extends T009) and
  `catalog_screen.tsx` (T017 extends T010).
- **US3 (Phase 5)**: depends on US2's `create_sheet_dialog.tsx` (T019 extends T016).
- **Polish (Phase 6)**: depends on all desired stories.

### User Story Dependencies

- **US1 (P1)**: no dependency on other stories — the MVP.
- **US2 (P1)**: builds on US1's query module and screen, but its surface is independently testable once
  the catalog renders.
- **US3 (P1)**: builds on US2's surface; the validation is independently testable once the surface
  opens.

### Within Each User Story

- Tests are written first and must fail before the implementation lands (constitution III).
- The query module before the screen; the screen before the surface is hooked to it; the surface before
  the field is added to it.
- Story complete before moving to the next priority.

### Parallel Opportunities

- T001 and T002 (setup) — different files.
- T003 (provider test) and T005 (render helper) touch different files; T004's implementation follows
  T003.
- T006 and T007 — independent test files; T008 and T012 are independent of both.
- T013 and T014 — independent test files.
- T020, T021 and T022 — independent files.

---

## Parallel Example: User Story 1

```bash
# Tests first (independent files, must fail before T009/T010):
Task: "Catalog render test in web/src/test/catalog_screen.test.tsx"
Task: "Query-policy test in web/src/test/catalog_queries.test.tsx"

# Then the query module and the screen:
Task: "Implement the catalog query in web/src/catalog/queries.ts"
Task: "Implement the catalog screen in web/src/catalog/catalog_screen.tsx"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 US1: the catalog renders with its five states and loads once per app load.
3. **STOP and VALIDATE** with `quickstart.md` scenarios 1, 2 and 3.
4. The shell now has real content; the creation surface and the sheet cycle are still ahead.

### Incremental Delivery

1. Setup + Foundational → the client and query client are provided and injectable.
2. US1 → the catalog replaces the empty workspace (MVP).
3. US2 → a template opens its detail in an accessible overlay.
4. US3 → the sheet name is validated and the action is inert.
5. Polish → the accessibility case, the copy scan, documentation and the gate sweep.

### Parallel Team Strategy

With two contributors: one takes US1 (the catalog and its query policy) while the other prepares the
US2 suite (the surface states) against the query module US1 lands; US3 then extends the surface. The
accessibility case (T020) and the copy scan (T021) share no file with either and can be done at any
point in Polish.

---

## Notes

- [P] tasks touch different files and have no dependency on incomplete work.
- Requirement IDs cited in task descriptions (`WCT-*`) are defined in `spec.md`; the delivered source
  must cite the same IDs so `make lint-specs` can trace them.
- Write tests first (T003, T006, T007, T013, T014, T018) and confirm they fail before implementing.
- Commit per task or per logical group, with the tests in the same commit (constitution III).
- Do not weaken an assertion to make a gate pass; retarget it to the feature that owns the subject, or
  amend the spec and record the decision instead.
- The catalog and the surface must never issue a mutating request, must never author a user-facing
  literal outside `web/src/l10n/strings.ts`, and must derive every visual value from the shared tokens —
  check those three rules on every task that touches `web/src/catalog/`.
