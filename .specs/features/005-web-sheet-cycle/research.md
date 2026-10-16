# Phase 0 Research: Web Sheet Cycle

**Feature**: `.specs/features/005-web-sheet-cycle/` | **Date**: 2026-10-16

Every decision below answers an open question in the feature's Technical Context or resolves a choice
the program plan's F5 task list (`docs/plan-web-pwa.md`, Parte B F5) leaves implicit. No
`NEEDS CLARIFICATION` remains. The desktop originals — `desktop/lib/sheet/` and
`desktop/lib/shell/shortcut_registry.dart` — are the rule source; this feature re-expresses them on the
web stack, it does not redesign them.

---

## R1. The module layout and the persistence port

**Decision**: the feature lives in three client directories:

- `web/src/sheet/` — `sheet_open.ts` (the open-by-id state), `sheet_edit.ts` (the in-memory edit
  store: pending delta, save, preview, marks, throttle, guards), `sheet_screen.tsx` (the mounted
  screen: header, footer, dialog wiring) and `sheet.css` (token-only styles).
- `web/src/library/` — `sheet_store.ts` (the `SheetStore` port, the versioned envelopes and the
  in-memory double), `idb_sheet_store.ts` (the only file that imports `idb`), `sheet_library.ts` (the
  offline cache read/write/evict on top of the port) and `draft_backup.ts` (the draft read/write/delete
  and the injectable cadence coordinator).
- `web/src/shell/shortcut_registry.ts` — the one action → key map the shell dispatches through.

The persistence boundary is a typed `SheetStore` interface, not a concrete database: the app builds the
`idb`-backed adapter, and every suite injects an in-memory double through the same `AppProviders` seam
the `ApiClient` already uses (`WCT-02`, `app_providers.tsx`).

**Rationale**: the desktop split state (`sheet_edit_provider`), persistence (`draft_backup_store`,
`sheet_library_store`) and the shell seam (`shortcut_registry`) into separate units; keeping that split
makes each rule independently testable and lets the `idb` dependency sit behind one file. jsdom has no
IndexedDB, so the versioned logic must be exercisable without a real database — a port with a memory
double is the repo's established shape (`Transport` for the API client, `Directory` for the desktop
stores) rather than a new pattern.

**Alternatives considered**:

- *A single `sheet/` module tree calling `idb` directly* — rejected: it forces every screen test to
  stand up a database, and it spreads the one new runtime dependency across the feature.
- *Mirror the desktop's single-file-per-concern exactly (`sheet_library_store` + `draft_backup_store`)*
  — adopted in spirit: the versioned-envelope logic is shared in `sheet_store.ts` because both records
  carry the same `{ schemaVersion, … }` shape and the same corrupt/foreign/unsupported-is-ignored rule
  (`WSC-30`), so shipping it twice would be duplication (`dupl`).

## R2. The edit state: a per-path external store, not Zustand or Jotai

**Decision**: the edit state is a small mutable `SheetEditStore` (plain TypeScript) that emits
immutable snapshots, consumed through React's `useSyncExternalStore`. A per-path subscription
(`store.subscribePath(path, listener)`) lets one field's control re-render on its own path's change, and
the screen subscribes only to the coarse signals it renders (pending, in-flight, errors, marks). The
store is wired into the F4 `SheetForm` **builder seams** (`controlBuilder`, `exclusiveControlBuilder`,
`rowMarkBuilder`), which F4 shipped precisely for this (F4 assumptions, `AD-001`).

**Rationale**: `AD-001` is "one edit rebuilds one control, not the whole form". F4 already expressed the
web form of it as injected builders that own the value subscription, and the plan's two suggestions —
`Zustand + selectors` or `Jotai per-path` — are ways to obtain a per-path subscription. React's
`useSyncExternalStore` gives the same subscription with no new package, which is what the feature's spec
records: `idb` is *the* one new runtime dependency (`spec.md` Assumptions). Pulling a state library in
addition would contradict `web/AGENTS.md` ("no new package without strict necessity") for a mechanism
the platform already provides. This is a deliberate, recorded deviation from the plan's stack table line
("Estado de edição | Zustand + selectors (ou Jotai por-path)").

**Alternatives considered**:

- *Zustand* / *Jotai* — rejected: both add a runtime dependency to obtain a selector subscription the
  standard `useSyncExternalStore` already provides, and the feature already carries `idb`.
- *React Context + `useReducer`* — rejected: a context value change re-renders every consumer, which
  breaks `AD-001` (one edit would rebuild the whole form); the desktop's `select` granularity is the
  requirement, not incidental.

## R3. The pending delta and the baseline advance

**Decision**: a port of the desktop `SheetEditState`/`save()` rules (`AD-006`, `WSC-12`…`WSC-16`):

- `editedPaths` records every path written at least once; `isPending` is true when *any* edited path
  currently differs from the reference (`defaults`). A field reverted to its seeded value therefore
  un-pends (`WSC-13`).
- `pendingDelta` rebuilds the nested object from only the pending paths, using the engine's
  `setPathValue`; a derived (non-editable) leaf is never written, so it never enters the delta
  (`WSC-14`).
- `setExclusive(selected, siblings)` writes the selected member on and every sibling off in one
  emission, or every member off for "None" (`WSC-15`).
- `save()` sends one `PATCH /sheets/:id` with `pendingDelta`, and on success merges the returned cascade
  into `data` and advances `defaults` to `mergeCascade(mergeCascade(defaults, delta), cascade)` — the
  exact server state — so the sent edits stop being pending while an edit made *during* the request
  stays pending (`WSC-16`). The engine's `mergeCascade`/`setPathValue` are reused verbatim (`WSC-34`).

**Rationale**: the desktop rules are the contract; recomputing the baseline from the sent delta rather
than setting `defaults = data` is what keeps a concurrent edit pending — a subtle rule that must not
drift (`AD-006`). Reusing the engine's derivation is required by the spec (`WSC-34`) and keeps the
screen and the creation preview from disagreeing.

**Alternatives considered**:

- *`defaults = data` after a save* — rejected: an edit typed while the `PATCH` was in flight would be
  marked saved without ever reaching the server.
- *Diff the whole `data` against the sheet on save* — rejected: it cannot tell a person's change from a
  derived recomputation, so a derived field could be sent (`WSC-14` forbids it).

## R4. The dry-run preview and its marks

**Decision**: `preview()` sends one `POST /sheets/:id/preview` with the same `pendingDelta`, and on
success merges the returned cascade into `data` **without** advancing `defaults` (`WSC-22`), records the
attempted revision so a repeat with no new edit is blocked, and publishes `previewMarks` — the delta's
flattened paths as `modified` and the cascade's flattened paths as `recalculated` (`WSC-21`, `WSC-23`).
The screen renders the marks through the F4 `rowMarkBuilder` seam and `markLabel`, never a per-screen
literal (`WSC-23`). Any new edit clears the marks and re-enables Preview (`WSC-24`); a failure keeps the
edits and the message and leaves Preview available (`WSC-25`).

**Rationale**: the F4 spec states in its assumptions that the sheet-cycle feature "owns the dry-run and
the draft" and that it produces the marks the engine already knows how to render — so this feature only
publishes path sets through an existing seam (`AD-009`). Because the preview returns the same cascade a
save would (`AD-007`), the marks and values can never disagree with the save.

**Alternatives considered**:

- *A second preview surface* — rejected: the desktop deliberately made the preview additive on Screen 2
  (`AD-009`); a second surface would duplicate the form and re-open the drift risk.
- *Compute the marks by comparing `data` before/after the merge* — rejected: it cannot distinguish a
  user-modified path from a recalculated one; the delta and the cascade are the two authoritative sets.

## R5. Local persistence: `idb` behind the `SheetStore` port, versioned envelopes

**Decision**: add **`idb` 7.1.1** as a direct runtime dependency (it is already resolved in the tree
transitively through `workbox-expiration`, so no new package enters the lockfile — only a direct
declaration). `idb_sheet_store.ts` opens one database with two object stores, `sheets` and `drafts`,
each record a versioned envelope `{ schemaVersion: 1, id, … }`. `sheet_library.ts` caches a loaded or
created sheet by id under the envelope (`WSC-11`); `draft_backup.ts` writes the pending editable delta
under its own envelope (`WSC-26`). A record that is corrupt, carries an unsupported `schemaVersion`, or
names a different id resolves to `null` so the sheet seeds cleanly (`WSC-30`, `WSC-31`). **No credential
is ever written**: the envelopes hold sheet values and a delta only (`WSC-31`, `SC-004`).

For tests, the versioned logic runs against `createMemorySheetStore()` (a `Map`-backed double); the thin
`idb` adapter is covered by **`fake-indexeddb`** as a dev-only dependency (jsdom ships no IndexedDB),
which does not enter the runtime bundle.

**Rationale**: the program plan chooses IndexedDB via `idb` (`docs/plan-web-pwa.md`, Parte 3) and the
feature's spec records it as the one new runtime dependency. The port keeps the versioning and
corruption rules testable without a database and keeps the runtime dependency in one file — the same
shape F2 used for `Transport`. `fake-indexeddb` is the only way to honor constitution III for the
adapter in a jsdom suite, and it is a dev dependency, so the "no credential / no runtime package
creep" posture is unchanged.

**Alternatives considered**:

- *Raw IndexedDB with a hand-rolled promise wrapper* — rejected: it re-implements `idb`'s small,
  well-tested surface for no gain, against the plan's explicit choice.
- *`localStorage`* — rejected: the plan names IndexedDB, and `localStorage` is synchronous string
  storage with a hard size cap — wrong for a sheet cache and a draft store.
- *No port; `idb` used in place* — rejected: every screen test would need a database.

## R6. The draft cadence

**Decision**: `draft_backup.ts` exposes an injectable scheduler and clock (`start(onTick)`,
`stop()`), ticking once every 60 seconds while the sheet has pending edits (`AD-008`), and it is a
no-op while the sheet is clean (`WSC-27`). Each tick writes the current `pendingDelta` through the port,
skipping an empty delta and swallowing a write failure (a failed backup never blocks editing). Tests
drive the cadence with a fake scheduler, so no suite waits a real minute.

**Rationale**: the desktop fixed 60 seconds with an injected scheduler (`AD-008`) so the cadence is
testable without waiting and without a new package; the same shape ports directly. The feature owns the
cadence value, injected rather than read from a global, so the test is deterministic (`WSC-26`).

**Alternatives considered**:

- *`setInterval` inline in the screen* — rejected: the screen would own an untestable timer and would
  keep ticking after unmount unless carefully cleared; an injected coordinator is testable and
  cancellable.
- *Back up on every keystroke* — rejected: it multiplies writes and defeats the point of a periodic,
  best-effort backup.

## R7. The request throttle

**Decision**: Save and Preview each keep a rolling 60-second window that records at most three
attempts; a fourth activation inside the window is a no-op, and the action re-enables when the window
frees (`WSC-38`, `SC-011`). The window is a pure, injectable-clock helper (a port of the desktop
`RollingWindowLimiter`, `SVE-02`), evaluated alongside the in-flight no-op rules (`WSC-18`, `WSC-21`) —
the throttle never replaces them.

**Rationale**: the desktop throttles both actions to three requests per rolling minute to protect the
server; the user decision (2026-10-16) ports it, and it is distinct from the server-side rate limiting a
later backend feature owns (`spec.md` Clarifications). An injected clock keeps the rolling window
assertable without waiting.

**Alternatives considered**:

- *A fixed counter reset every minute* — rejected: it allows a burst of six across a window boundary;
  the desktop used a true rolling window and the spec says "rolling".
- *Rely on the in-flight guard alone* — rejected: it does not bound rapid completed requests; the user
  chose to port the limiter.

## R8. The shortcut registry

**Decision**: introduce `web/src/shell/shortcut_registry.ts` — a plain action → key map
(`Ctrl+S` → save the mounted sheet, `F5` → refresh the mounted sheet's template behind a discard
confirmation) — plus one document-level dispatcher the shell mounts. `Ctrl+S` is a no-op with no sheet
screen mounted (`WSC-20`); `F5` is a no-op with no sheet screen mounted (`WSC-36`). The registry holds no
state: the mounted screen registers the active handlers and unregisters on unmount, mirroring the
desktop's `activeSheetIdProvider`/tick seam.

**Rationale**: the desktop centralized every binding in one rebindable registry
(`desktop/lib/shell/shortcut_registry.dart`) and the web AGENTS.md rules put shortcuts in one registry
too. `F5` refreshing the sheet's template (not the browser default, and not the catalog) is the user
decision for this feature; the catalog refresh binding is deliberately out of scope here because this
feature owns the sheet screen's refresh, and reaching into F3's surface would widen the contract.

**Alternatives considered**:

- *A `keydown` listener per screen* — rejected: the desktop centralized it so two surfaces cannot both
  claim a key; one registry keeps the map reviewable.
- *Mounting the registry with the F1 shell and binding the catalog* — rejected: F1 documented `F5` as a
  V1 default but built no handler, and this feature's `F5` is sheet-scoped; the catalog binding belongs
  with the catalog surface.

## R9. Routing and the creation flow

**Decision**: declare `/sheets/:id` in `web/src/app/routes.ts` (deep-linkable, `deepLink: true`) and
map it in `router.tsx` to `SheetScreen`. The creation surface's primary action calls
`client.createSheet({ templateId, sheetName })`, and on success **seeds the sheet query cache** with the
returned `Sheet` for `['sheet', id]` before navigating to `/sheets/:id`, so the screen renders the
created sheet without a second `GET` (`WSC-02`). A failed creation keeps the dialog open with the typed
name and the mapped message (`WSC-04`); the primary action is disabled while in flight (`WSC-03`).

**Rationale**: `WSC-02` forbids re-fetching the created sheet — the desktop achieved this by injecting
the `201` payload through a route-scoped provider (`AD-004`/`AD-005`). On the web the equivalent seam is
the query cache: seeding `['sheet', id]` with the create response and marking it fresh is exactly "the
`201` payload moves into the screen" with TanStack Query as the store (`AD-016`). Declaring the route in
`routes.ts` is what makes it enumerable and deep-linkable (`WSC-10`, `FND-08`).

**Alternatives considered**:

- *Navigate then `GET /sheets/:id`* — rejected: `WSC-02` explicitly forbids the second fetch; it would
  also show a loading flash the desktop avoided.
- *Pass the created sheet through router location state* — rejected: it is not shareable or
  deep-linkable, and a refresh would drop it; the query cache is the durable seam.

## R10. Deep links and the serve fallback

**Decision**: `/sheets/:id` renders from the URL parameter alone, so a direct navigation resolves the
same screen a client navigation would (`WSC-10`). The route is `audience: 'public'` until the
browser-auth feature (F6) and the session feature (F7) land; the session gate is a later feature, so
`SheetScreen` renders unauthenticated here, against the F2 client as built (no credential attached).

**Rationale**: F1 made the router the deep-link mechanism and requires every declared route to resolve
identically on direct navigation (`FND-08`); the server-side `index.html` fallback for history routes
already exists from F1. This feature adds the route, not a new serving mechanism.

**Alternatives considered**:

- *A hash route (`#/sheets/:id`)* — rejected: F1 chose history routing, and mixing schemes would break
  the existing deep-link assertions.

## R11. The copy keys

**Decision**: extend the single English catalog (`web/src/l10n/strings.ts`) with the feature's keys:
screen states (`sheet.openLoading`, `sheet.templateLoading`, `sheet.missing`, `sheet.retryOpen`,
`sheet.emptyFields`), the footer (`sheet.save`, `sheet.saving`, `sheet.saved`, `sheet.preview`,
`sheet.previewing`), the dirty indicators (`sheet.pendingEdits`), the discard guards
(`sheet.refreshTitle`, `sheet.refreshMessage`, `sheet.refreshConfirm`, `sheet.leaveTitle`,
`sheet.leaveMessage`, `sheet.leaveConfirm`, `sheet.cancel`) and the header (`sheet.headerTitle`). No
literal string is authored in `web/src/sheet/` (`WSC-32`, `SC-006`). The existing `sheet.*` keys from F3
are untouched.

**Rationale**: the constitution and `web/AGENTS.md` require every user-facing string to resolve through
the catalog so a later localization feature replaces one module (`WSC-32`). The F3 copy scan already
guards the catalog and engine directories; extending it to `src/sheet/`/`src/library/` keeps one gate.

**Alternatives considered**:

- *Inline literals in the screen* — rejected: `WSC-32` forbids them and the copy scan would fail.
- *Reusing F3's `sheet.*` keys* — rejected: those name the creation dialog ("New sheet", "Create
  sheet"), not the open sheet's states, and a duplicate string would fail the distinct-copy assertion.

## R12. The testing strategy and the accessibility measurement

**Decision**: suites stay flat in `web/src/test/` with a `sheet_*`/`library_*`/`shell_*` prefix, edge
cases first, driving a **fake transport** (existing `fake_transport.ts`) and the **in-memory
`SheetStore`** double. The pure rules (delta, reverted edit, exclusive write, baseline advance, merge,
marks, versioned-envelope corruption, rolling window) are unit-tested; the screen's situations
(loading, loaded, missing, retryable failure, creation in flight/failed, save ok/failed, preview
ok/failed, guards, throttle) are rendering tests; the `idb` adapter is exercised under
`fake-indexeddb`. The mounted-surface accessibility outcome (`SC-010`) is measured by a Playwright case
against the real build with `@axe-core/playwright`, following the `AD-014` pattern — no axe dependency is
added to the Vitest tree. The F1 token scan (`tokens_used.test.tsx`) and the F3 copy scan
(`l10n_strings.test.ts`) are extended to `src/sheet/` and `src/library/`.

**Rationale**: constitution III and VII require edge-case-first tests for every behavior and a
rendering/e2e measurement for interface requirements. F4 deferred the mounted-surface sweep to the
feature that mounts the engine; this feature mounts it, so it owns the measurement (`SC-010`) and
reuses F1's existing axe dependency (`AD-014`), adding no gate.

**Alternatives considered**:

- *MSW* — rejected (as in F2): the injected transport already proves request counts and payloads
  without a network-interception package.
- *A Vitest axe sweep* — rejected: `AD-014` fixed the browser-level measurement as the Playwright case,
  and the Vitest suites assert ARIA structure instead.
