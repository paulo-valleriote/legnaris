# Phase 0 Research: Web Catalog

**Feature**: `.specs/features/003-web-catalog/` | **Date**: 2026-10-16

Every decision below answers an open question in the feature's Technical Context or resolves a choice
the program plan's F3 task list leaves implicit. No `NEEDS CLARIFICATION` remains.

---

## R1. What owns the catalog and detail server state

**Decision**: TanStack Query (`@tanstack/react-query`) owns both requests. The catalog screen calls a
`useCatalog()` hook and the creation surface calls a `useTemplateDetail(id)` hook; each is a thin
wrapper over the F2 `ApiClient` operation (`listTemplates`, `getTemplate`). The query functions never
build a URL, unwrap an envelope or branch on a status — the F2 client already does all three.

**Rationale**: server state is not component state, and the plan made the choice already (Parte 3 —
"server state: TanStack Query — cache + invalidation; replaces the Riverpod network providers" — and
the F3 task, "TanStack queries … cache `staleTime`"). Using the plan's library means the loading /
error / data branches, the in-flight deduplication (WCT-08) and the "one request per app load"
guarantee come from a tested implementation rather than a hand-rolled `useEffect` machine that would
have to re-derive them. It is also the seam the refresh shortcut (F5-T005) will invalidate.

**Alternatives considered**:

- *A hand-rolled `useEffect` + `useState` hook per request* — rejected: it re-implements deduplication,
  staleness and error branching, and WCT-08 (no second concurrent request) becomes a manual guard that
  drifts from the query identity. The plan already chose the library.
- *SWR* — rejected: equivalent role, but the plan names TanStack Query and the later features
  (invalidation on F5) are described against its API.
- *A Redux Toolkit Query slice* — rejected: it drags a store the feature does not need for two reads
  and contradicts the plan's layered client.

## R2. The one new dependency, and why it clears the "no new package" rule

**Decision**: add `@tanstack/react-query` 5.104.3 as a runtime dependency. Verified at planning time:
the latest 5.x release is 5.104.3 and its peer range is `react: ^18 || ^19`, so it accepts the pinned
React 19.3.1.

**Rationale**: `web/AGENTS.md` forbids a new package "without strict necessity" and bans
*design-system* dependencies specifically. This is neither: it is the server-state library the program
plan selects for the client, and F3-T001 names it. The alternative — hand-rolling the state machine —
is the non-obvious risk the rule is meant to avoid, so the dependency is the lower-risk choice. It is
recorded as an inherited plan decision, not a new one, in `web/AGENTS.md` when the feature lands.

**Alternatives considered**:

- *No dependency, hand-rolled hooks* — see R1.
- *Defer the library and hand-roll now, adopt later* — rejected: it builds code the very next phase
  deletes, and the cache invalidation contract is owned by the library.

## R3. How the client and the query client reach the screens

**Decision**: a new `web/src/app/providers.tsx` exports `AppProviders`, which builds the F2 client once
(`createApiClient({ baseUrl: resolveApiConfigFromBuild().baseUrl, transport: createFetchTransport(fetch) })`),
creates one `QueryClient`, and wraps the app in `QueryClientProvider`. The client is also exposed
through a small React context (`ApiClientProvider`/`useApiClient`) so the query hooks read it from
context instead of importing a module-level singleton. `main.tsx` wraps `<AppRoutes />` in
`<AppProviders>`. In tests, the render helper mounts the same providers with an injected client and a
fresh query client, so nothing about the screen assumes a global.

**Rationale**: F1 deliberately left the provider layer out and `web/AGENTS.md` says it "lands with the
first feature that genuinely needs shared context" — this is that feature. Putting the client in
context (rather than exporting one instance) is what keeps the screens testable with a fake transport
and keeps the module a seam, exactly as the F2 transport is. A single `AppProviders` means the app and
the tests construct the same tree.

**Alternatives considered**:

- *A module-level singleton client built at import time* — rejected: it runs `resolveApiConfigFromBuild`
  and reads `fetch` at import, which is untestable without module mocking and would make a
  configuration failure crash the test process rather than a screen.
- *Passing the client down as props from the router* — rejected: the surface is opened from inside the
  catalog, so the client would thread through components that do not use it.
- *A DI container / service locator* — rejected: one value in one context is the whole need.

## R4. Cache and refresh policy per query

**Decision**:

| Query | Config | Effect |
|---|---|---|
| catalog | `staleTime: Infinity`, `retry: false`, `refetchOnWindowFocus: false`, `refetchOnMount: false`, `refetchOnReconnect: false` | loaded exactly once per app load (WCT-02); no background, focus or reconnect request; no retry beyond the F2 client's own |
| detail | key `['template', id]`, `staleTime: 0`, `gcTime: 0`, `retry: false`, `refetchOnWindowFocus: false` | a fresh request every time the surface opens (WCT-21); the entry is dropped on unmount so a reopen starts in the loading state and never shows the previous template (WCT-13) |

The query client created in `AppProviders` sets `retry: false` as a default so no query can fall back
to the library's built-in retry; the API client already performs the single read retry F2 owns.

**Rationale**: the plan's F3-T001 says "cache `staleTime`", which resolves the catalog to a
long-lived cache; and the spec is explicit that the catalog loads once per app load (clarification,
2026-10-16) while the detail is requested once per opening (WCT-21). `retry: false` is essential: the
library's default is three client-side retries, which would multiply every failure into four requests
and contradict WCT-07/WCT-16 ("re-request exactly once") and WCT-02. `refetchOnWindowFocus` is
essential too — its default is `true`, which would re-request the catalog every time the window
regains focus and break "once per app load". `gcTime: 0` on the detail is what makes WCT-13
("must not show a previously opened template's detail") structural rather than a guard in the render.

**Alternatives considered**:

- *A finite `staleTime` (e.g. 5 minutes) for the catalog* — rejected: it re-requests without a user
  action, which the clarification excluded and which F5's refresh owns.
- *`staleTime: 0` for the catalog with deduplication only* — rejected: remounting the route would
  re-request, violating "once per app load".
- *Keeping the detail cached between openings and rendering it while revalidating* — rejected: it
  would show the previous template's name during the new request, exactly what WCT-13 forbids.
- *Leaving the library defaults and disabling retry only* — rejected: default `refetchOnWindowFocus`
  and default `retry` are both wrong for this feature, and a default that happens to be overridden
  elsewhere is not a contract.

## R5. The creation surface: accessible overlay, not a native `<dialog>`

**Decision**: the creation surface is a React-rendered overlay with `role="dialog"` and
`aria-modal="true"`, labelled by its title, rendered conditionally while a template is open. Esc and
its close action dismiss it; focus moves to the surface's first control on open and returns to the
triggering catalog entry on close. It is **not** the platform `HTMLDialogElement` and it is not a
route.

**Rationale**: the platform dialog would be the natural element, but the test environment cannot drive
it — verified at planning time, `jsdom` 30.1.3 does not implement `HTMLDialogElement.showModal` or
`.close` (both read as `undefined`), so opening it in a rendering test throws and the accessibility
contract could not be asserted. The constitution requires interface requirements to be verified by
rendering tests (III, VII), so an element the required test environment cannot open is not usable. The
overlay keeps the same ARIA/keyboard contract (`role="dialog"`, `aria-modal`, labelled-by, Esc, focus
in/out) with an implementation the suite can assert, and it matches the desktop phase's modal dialog
and the plan's "Navegação rasa" decision (no deep link for the surface — spec Assumptions).

**Alternatives considered**:

- *Native `<dialog>` with a jsdom shim in the test setup* — rejected: a shim is test-only behaviour that
  proves nothing about production, and it would still not assert the focus and Esc contract.
- *A route `/templates/:id/create`* — rejected: the spec and the plan keep navigation shallow; the
  surface must return to the catalog unchanged (WCT-19, WCT-20) without a navigation event.
- *A third-party dialog library* — rejected: it adds a dependency and design-system surface the plan
  forbids, for one dialog.

## R6. How the render states are modeled and copied

**Decision**: each screen resolves a discriminated render phase from the query result —
`loading`, `loaded`, `empty` (catalog only), `error`, `parseError` — and renders exactly one branch.
A `DomainFailure` renders `failureMessage(kind)` from the F2 mapper; a `DefensiveParseFailure` renders
`parseFailureMessage()`; a `notFound` kind renders the same mapper copy inline in the surface (WCT-17).
Copy is never authored in the component: the phase resolves a `MessageKey` and the component calls
`message(key)`.

**Rationale**: "distinct loading/empty/error states" is the plan's F3-T002, and the mapper is the F2
contract for turning a failure into copy. Modeling the phase as a union keeps the branches total and
makes the five situations separately assertable (SC-001) rather than inferred from a single render.

**Alternatives considered**:

- *Boolean flags (`isLoading`, `isError`) read ad hoc in JSX* — rejected: it permits a half-rendered
  state where two flags are true, which WCT-09 and SC-007 explicitly forbid (no partial list, no blank
  screen).
- *Rendering the server's raw message* — rejected: WCT-06/WCT-15 require the *mapped* copy, and the F2
  module already centralizes it.

## R7. The sheet-name validation

**Decision**: the surface holds the raw input value in local state; `trimmed = value.trim()`; the
primary action's `disabled` attribute is `trimmed === ''` and its tooltip (a `title` plus an
`aria-describedby` note carrying the same key) states that a sheet name is required. The action's
handler issues no request in this feature.

**Rationale**: the desktop phase and its create-sheet spec both fix the rule — trim leading/trailing
whitespace, a whitespace-only name counts as empty, the action stays disabled with a tooltip — and
WCT-22…WCT-25 restate it. Deriving `disabled` from the trimmed value makes "no mutation on an empty
name" structural: there is no code path from a disabled control to a request.

**Alternatives considered**:

- *Validating on submit (enable always, refuse on activate)* — rejected: WCT-23 requires the action to
  be disabled with a tooltip while empty.
- *Trimming the stored value on every keystroke* — rejected: it fights the user's typing; validation
  reads the trimmed value, the raw value is what the person sees.

## R8. Keyboard and focus management

**Decision**: on open, focus moves to the sheet-name input (the surface's first focusable control); on
close, focus returns to the catalog entry that opened the surface, whose element reference is captured
when it is activated. Esc dismisses the surface from anywhere inside it. Tab order follows the visual
order, and every control uses the shared focus token.

**Rationale**: WCT-27 requires keyboard-only operability, focus moved in on open and returned to the
trigger on close — the standard dialog focus contract, which the desktop phase also kept. Capturing the
trigger reference is what makes "returns to the triggering entry" true rather than "returns to the
document".

**Alternatives considered**:

- *Relying on the browser to place focus* — rejected: focus would stay on the (now background) catalog
  entry, so a keyboard user would tab into the page behind the surface.
- *A focus-trap library* — rejected: a dependency for one surface; the surface's controls are few and
  the contract (move in, Esc out, no trap) is small.

## R9. How the screens are tested

**Decision**: the F1 render helper (`web/src/test/render.tsx`) is extended to accept an injected
`ApiClient` (built over a fake transport from `web/src/test/fake_transport.ts`) and to wrap the app in
`AppProviders` with a fresh `QueryClient` per test (`retry: false`, `gcTime: 0`). Suites assert by
role, accessible name and `data-testid`, and assert request counts through the fake transport rather
than through the DOM. No MSW, no network. The F1 suites that pinned the empty workspace on `/`
(`shell.test.tsx`, `routing.test.tsx`) and the e2e shell spec are updated to assert the catalog
instead; `shell_keyboard.test.tsx` asserts the nav link and the not-found route, neither of which this
feature changes, so it is left untouched.

**Rationale**: the F2 feature already established the fake transport as the assertion seam and MSW was
deliberately not added; reusing it keeps one way to script a response and one way to count requests.
A fresh query client per test is required because the catalog's `staleTime: Infinity` would otherwise
leak a previous test's data across cases. Updating rather than deleting the F1 assertions respects
constitution III — the assertion moves to the new, truthful subject.

**Alternatives considered**:

- *MSW* — rejected for the same reason F2 rejected it (R-plan, `web/AGENTS.md`): the injected transport
  is the contract, and network-level interception is less direct for call counting.
- *A shared query client across tests* — rejected: it makes the "one request per app load" assertion
  order-dependent and flaky.
- *Deleting the F1 empty-state assertions* — rejected: it would drop coverage of a designed shell state
  (which still exists for the not-found route) rather than retarget it.

## R10. Where the new copy lives

**Decision**: new keys are added to the existing typed English catalog (`web/src/l10n/strings.ts`)
under a `catalog.*` and `sheet.*` group — for example the catalog title, the loading, empty and Retry
strings, the surface title, the sheet-name label, placeholder and the "required" tooltip. The F2
`api.error.*` keys are reused unchanged for failure and parse-failure copy. No literal string is
authored in `web/src/catalog/`.

**Rationale**: WCT-26 requires every visible string to resolve through the catalog, and the F2 mapper
already resolves failure copy from it. Adding keys to the one catalog keeps the later localization
feature a matter of replacing this module.

**Alternatives considered**:

- *A second catalog module for the catalog feature* — rejected: two catalogs is two places to look for
  a string; the F2 module is already the single catalog.
- *Reusing the F2 error keys for the loading/empty strings* — rejected: those strings are not errors;
  distinct keys keep the copy honest.

## R11. Captures and fixtures

**Decision**: no new contract capture is added. The screens are tested against the fake transport with
the same payload shapes the F2 fixtures already pin (`templates.json`, `template_detail.json`), so the
catalog screen and the surface consume the F2 contract rather than re-declaring it.

**Rationale**: the F2 feature pinned real captures of `GET /templates` and `GET /templates/:id` and its
suite fails if those captures drift from the models. Re-deriving captures here would duplicate that
gate; the screens add no new field or operation (spec Assumptions). The one screen-level shape that is
new — the render phase — is asserted directly, not via a capture.

**Alternatives considered**:

- *A screen-level capture set* (a recorded response per render situation) — rejected: the fake
  transport already scripts those responses, and a second capture set would need its own provenance
  upkeep for no new contract.
