---
description: "Task list for the Web Sheet Cycle feature"
---

# Tasks: Web Sheet Cycle

**Input**: Design documents from `.specs/features/005-web-sheet-cycle/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED here. The constitution's "behavior ships with tests" principle, the feature's own
`WSC-01…WSC-39`, and the repo's edge-case-first rule all demand test tasks; they are written first and
must fail before the implementation they cover.

**Organization**: tasks are grouped by user story so each story is independently implementable,
testable, and demonstrable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no dependency on incomplete tasks)
- **[Story]**: US1 … US5, per spec.md
- Every task names its exact file path

## Path Conventions

The open/edit surface lives under `web/src/sheet/`, the local persistence under `web/src/library/`, the
shell seam under `web/src/shell/`, the copy catalog under `web/src/l10n/`, and every unit/rendering test
under `web/src/test/` with a `sheet_*` / `library_*` / `shell_*` prefix, matching the flat layout F1
established. The single browser-level accessibility case lives under `web/e2e/`, next to the F1/F3 specs
it extends. The backend is not touched; `api/client.ts` gains only the preview operation.

## Traceability

| Story | Priority | Requirements |
|---|---|---|
| US1 Create a sheet and open it | P1 | WSC-01…WSC-11 |
| US2 Edit fields and save only the changed delta | P1 | WSC-12…WSC-20 |
| US3 Dry-run a preview that marks what changed | P2 | WSC-21…WSC-25 |
| US4 Guard unsaved edits and throttle requests | P2 | WSC-36, WSC-37, WSC-38 |
| US5 Unsaved drafts survive a crash and are restored | P3 | WSC-26…WSC-31 |
| Cross-cutting | — | WSC-32, WSC-33, WSC-34, WSC-35, WSC-39 |

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: the two dependencies and the copy keys every story consumes.

- [ ] T001 [P] Add the two dependencies to `web/package.json`: `idb` 7.1.1 as a direct **runtime**
      dependency (the program plan's IndexedDB wrapper, research R5 — already resolved transitively
      through `workbox-expiration`, so only a direct declaration is added) and `fake-indexeddb` as a
      **dev-only** dependency (jsdom ships no IndexedDB; it never enters the runtime bundle). No state
      library is added (research R2)
- [ ] T002 [P] Extend the translation catalog `web/src/l10n/strings.ts` with the sheet-cycle keys from
      `data-model.md` `MessageKey additions` — `sheet.headerTitle`, `sheet.openLoading`,
      `sheet.templateLoading`, `sheet.emptyFields`, `sheet.missing`, `sheet.retryOpen`,
      `sheet.pendingEdits`, `sheet.save`, `sheet.saving`, `sheet.saved`, `sheet.preview`,
      `sheet.previewing`, `sheet.refreshTitle`, `sheet.refreshMessage`, `sheet.refreshConfirm`,
      `sheet.leaveTitle`, `sheet.leaveMessage`, `sheet.leaveConfirm`, `sheet.cancel` — and extend
      `web/src/test/l10n_strings.test.ts` so each new key resolves to non-empty copy, distinct from its
      neighbours (WSC-32, SC-006)

**Checkpoint**: `cd web && npm install && npm test` passes with the dependency, the dev double and the
new keys in place.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the persistence port every story writes through, its `idb` adapter, and the provider/render
seams that inject it.

**⚠️ CRITICAL**: US1's cache write, US2's draft delete on save and US5's draft all import this port; no
story can be proven without it.

### Tests for the foundation ⚠️ (write first; they must fail before the implementation)

- [ ] T003 [P] Store test `web/src/test/library_store.test.ts` (WSC-11, WSC-26, WSC-27, WSC-29, WSC-30,
      WSC-31), edge cases first: `readSheet`/`readDraft` return `null` for a missing key, a malformed
      record, an unsupported `schemaVersion` and a foreign `id`, and none throws; `deleteSheet`/
      `deleteDraft` on a missing key resolve without throwing; a `putSheet`→`readSheet` round-trip yields
      the `Sheet` under `schemaVersion 1` and `writeDraft`→`readDraft` round-trips the delta; then the
      adapter cases under `fake-indexeddb` — `createIdbSheetStore(factory)` round-trips a sheet and a
      draft and reads a corrupt record as `null`; a stored record contains no credential

### Implementation for the foundation

- [ ] T004 Implement `web/src/library/sheet_store.ts` per `contracts/sheet-store.contract.md`: the
      `SheetStore` port (`readSheet`/`putSheet`/`deleteSheet`/`readDraft`/`writeDraft`/`deleteDraft`), the
      shared `SCHEMA_VERSION = 1` versioned envelopes (`{ schemaVersion, id, sheet }` and
      `{ schemaVersion, id, delta }`), the total rule that a missing/corrupt/unsupported-version/foreign
      record reads as `null` without raising, and `createMemorySheetStore()` (a `Map`-backed double every
      logic and screen suite injects); no operation writes a credential (WSC-11, WSC-26, WSC-27, WSC-29,
      WSC-30, WSC-31)
- [ ] T005 Implement `web/src/library/idb_sheet_store.ts` per `contracts/sheet-store.contract.md`:
      `createIdbSheetStore(factory?)` opening one database with the `sheets` and `drafts` object stores
      and the same versioned-envelope rules — the only module in the feature that imports `idb` (research
      R5, WSC-11, WSC-30)
- [ ] T006 Extend `web/src/app/providers.tsx` to build a `SheetStore` (the `idb` adapter by default,
      injectable) and expose it through a `useSheetStore()` context, and extend
      `web/src/test/render.tsx` to mount a fresh in-memory `SheetStore` double per test (WSC-11).
      *Depends on T004, T005.*

**Checkpoint**: the port, the adapter and the provider seam exist; every later screen test can inject the
double through `AppProviders`.

---

## Phase 3: User Story 1 - Create a sheet and open it (Priority: P1) 🎯 MVP

**Goal**: the creation surface issues one `POST /sheets` and opens the created sheet without a second
`GET`; the sheet screen is deep-linkable, issues one `GET /sheets/:id` and one `GET /templates/:id`, and
renders loading, loaded, missing and retryable situations plus the sheet's name and its template's field
tree.

**Independent Test**: drive the creation surface through a successful creation and a failed one and
confirm one request each and the transition onto the sheet screen with no second fetch; then load an
existing sheet through a slow response, a failure, a missing sheet and a decodable-but-wrong payload and
confirm the four distinct screen states and the local eviction on the missing case.

### Tests for User Story 1 ⚠️ (write first; they must fail before the implementation)

- [ ] T007 [P] [US1] Open-situation test `web/src/test/sheet_open.test.tsx` (WSC-05…WSC-11), edge cases
      first: a slow response shows `sheet.openLoading` until it settles and the open issues exactly one
      `GET /sheets/:id`; a `NOT_FOUND` shows `sheet.missing` with **no** Retry, calls the store's
      `deleteSheet` and `deleteDraft`, and a throwing `deleteSheet` still shows the message; any other
      failure (including a malformed payload) shows the mapped message with a Retry that re-fetches
      exactly once; a template failure shows `sheet.retryOpen` with a Retry; a template with no fields
      shows `sheet.emptyFields`; a direct navigation of `/sheets/:id` resolves the same screen as a client
      navigation (WSC-10); after a successful open the store holds the sheet under `schemaVersion 1` and
      no credential (WSC-11)
- [ ] T008 [P] [US1] Creation-flow test `web/src/test/create_sheet_flow.test.tsx` (WSC-01…WSC-04), edge
      cases first: a whitespace-only name keeps the primary action disabled and issues zero requests;
      while the request is in flight the action is disabled, the surface shows progress, and a repeat
      activation issues no second request; a failure keeps the surface open with the typed name intact and
      the mapped message inline and navigates nowhere; then the happy path — a non-empty trimmed name
      issues exactly one `POST /sheets` carrying the template id and the name, and on success the flow
      navigates to `/sheets/:id` and the screen renders the created sheet with **zero** `GET /sheets/:id`
      because the cache was seeded
- [ ] T009 [P] [US1] Screen-state test `web/src/test/sheet_screen_states.test.tsx` (WSC-08, WSC-10, SC-001):
      the loaded screen shows the sheet's name (`sheet.headerTitle`) and renders the template's field tree
      through the F4 `SheetForm` (a synthetic template exercising several field types), the loading and
      template-loading lines resolve through the catalog (no literal), and a deep link renders the same
      screen

### Implementation for User Story 1

- [ ] T010 [US1] Implement `web/src/sheet/sheet_open.ts` per `contracts/sheet-open.contract.md`:
      `useSheetOpen(sheetId)` returning the `SheetOpenResult` union (`loading` / `loaded` / `notFound` /
      `failure` / `templateLoading` / `templateFailure`), issuing exactly one `GET /sheets/:id` then one
      `GET /templates/:id`, returning `loaded` from a fresh seeded `['sheet', id]` without a second `GET`
      (WSC-02), evicting the sheet **and** its draft best-effort on `NOT_FOUND` (a failing eviction never
      hides the message, WSC-06), and taking the retryable path for an undecodable payload (WSC-07); export
      `SHEET_QUERY_KEY` (WSC-05…WSC-11)
- [ ] T011 [US1] Implement `web/src/library/sheet_library.ts` per `contracts/sheet-store.contract.md`: the
      cache read/write/evict helpers on top of the `SheetStore` port — `cacheSheet(store, sheet)` writing
      the loaded/created sheet by id under the versioned envelope (WSC-11), `readCachedSheet(store, id)`,
      and `evictSheet(store, id)` deleting the sheet and its draft best-effort (WSC-06, WSC-29); no
      credential is ever stored (WSC-31). *Depends on T004.*
- [ ] T012 [US1] Extend `web/src/catalog/queries.ts` with the sheet query key and the create-seed seam:
      `sheetKey(id)` = `['sheet', id]` and a helper that seeds that key with the `POST /sheets` `201`
      payload and marks it fresh, so the screen renders the created sheet without a second fetch (WSC-02,
      WSC-34)
- [ ] T013 [US1] Wire the creation surface `web/src/catalog/create_sheet_dialog.tsx` per
      `contracts/sheet-open.contract.md` and research R9: the primary action calls
      `client.createSheet({ templateId, sheetName })`, is disabled and shows progress while in flight
      (WSC-03), keeps the typed name and shows the mapped message inline on failure (WSC-04), and on
      success seeds `['sheet', id]` (T012) then navigates to `/sheets/:id` via `useNavigate` (WSC-01,
      WSC-02)
- [ ] T014 [US1] Declare the sheet route in `web/src/app/routes.ts` (`/sheets/:id`, `audience: 'public'`,
      `deepLink: true`) and map it to `SheetScreen` in `web/src/app/router.tsx`, so the screen is
      enumerable and reachable by direct navigation (WSC-10, research R9/R10)
- [ ] T015 [US1] Implement `web/src/sheet/sheet_screen.tsx` and `web/src/sheet/sheet.css` per
      `contracts/sheet-open.contract.md`: read the `:id` route parameter, call `useSheetOpen`, and render
      the six situations (loading `sheet.openLoading`, loaded, `notFound` `sheet.missing` with no Retry
      action, retryable `failure` with a Retry, `templateLoading`, `templateFailure` with a Retry) — the
      loaded case shows the sheet's name in the header and mounts the F4 `SheetForm` over `template.fields`
      and `sheet.data`; every visual value is a `--lg-*` token and every string resolves through
      `message()` (WSC-05…WSC-11, WSC-32, WSC-33). *Depends on T010, T011, T014.*

**Checkpoint**: a sheet can be created from the catalog and opened with exactly one request each, and the
open renders its four plus two states; the sheet is not editable yet (the screen renders the field tree in
the engine's inert data mode until US2 wires the builder seams).

---

## Phase 4: User Story 2 - Edit fields and save only the changed delta (Priority: P1)

**Goal**: the in-memory edit store records exactly the changed editable leaves, reports pending state in a
footer, and saves only that nested delta with one `PATCH`, merging the returned cascade and advancing the
baseline; a reverted edit un-pends; a failed save keeps every edit; a clean or in-flight Save issues no
request; `Ctrl+S` runs the same save.

**Independent Test**: load a sheet, change an editable field, revert it, change two fields (one an
exclusive-group member) and assert the pending state and the exact nested payload sent; then drive a
successful save and assert the derived values merge and the baseline advances, and a failed save and assert
the edits and the mapped message both survive.

### Tests for User Story 2 ⚠️ (write first; they must fail before the implementation)

- [ ] T016 [P] [US2] Edit-store test `web/src/test/sheet_edit.test.ts` (WSC-12…WSC-19, WSC-34), edge cases
      first: a redundant write (same value) advances no `revision`; an edit back to its seeded value
      un-pends and leaves `pendingDelta` empty; a derived (non-editable) path never enters the delta; an
      unchanged sibling of a changed nested group never enters the delta; a high-precision decimal is sent
      unchanged; an exclusive selection writes one member on and its siblings off in one emission,
      `None` writes all off, and a redundant exclusive write emits nothing; Save with an empty delta issues
      no request; a second Save while one is in flight issues none; a failed save keeps every edit and sets
      the mapped `saveError` with no automatic retry; an edit made during a save stays pending after it
      succeeds; then the happy paths — `setPath` records the path and produces a fresh `data` tree, the save
      sends the exact nested delta, and on success merges the returned cascade and advances `defaults` to
      `mergeCascade(mergeCascade(defaults, delta), cascade)` so the sent edits un-pend
- [ ] T017 [US2] Footer/Save rendering cases added to `web/src/test/sheet_screen_states.test.tsx`
      (WSC-12, WSC-14, WSC-16, WSC-17): the footer shows `sheet.pendingEdits` only while pending, Save is
      disabled while clean or in flight and shows `sheet.saving`/`sheet.saved` feedback, the mapped save
      failure renders inline, and no user-facing literal is authored in the screen

### Implementation for User Story 2

- [ ] T018 [US2] Implement `web/src/sheet/sheet_edit.ts` per `contracts/sheet-edit.contract.md`:
      `createSheetEditStore(seed, deps)` with `getState`, `subscribe`, `subscribePath`, `setPath`,
      `setExclusive` and `save()` — the `editedPaths` set with `isPending` true only while a path differs
      from the reference (WSC-12, WSC-13), copy-on-write `data` (never an in-place mutation), `pendingDelta`
      rebuilt through the engine's `setPathValue` and never carrying a derived leaf (WSC-14), one `PATCH`
      with the delta on `canSave` only (WSC-18, WSC-19), the success merge + baseline advance (WSC-16) and
      the failure keeping edits + mapped `saveError` with no auto-retry (WSC-17); reuse `seedFromSheet`,
      `mergeCascade` and `setPathValue` from F4 (WSC-34)
- [ ] T019 [US2] Switch `web/src/sheet/sheet_screen.tsx` to the F4 builder seams per
      `contracts/sheet-edit.contract.md`: build the edit store from `seedFromSheet(template, sheet)`, wire
      `controlBuilder`/`exclusiveControlBuilder` to the per-path subscription + `setPath`/`setExclusive`
      (research R2, the web expression of `AD-001`), and render the footer (dirty indicator + Save action
      with its in-flight/success feedback and the inline mapped save error) (WSC-12, WSC-14, WSC-16,
      WSC-17). *Extends T015, T018.*
- [ ] T020 [US2] Create `web/src/shell/shortcut_registry.ts` per `contracts/shortcut-registry.contract.md`:
      the `SHORTCUTS` action → key map, `matchShortcut(event)`, `dispatchShortcut(action)` and the
      `useSheetShortcuts({ onSave, onRefresh })` registration seam (handlers register on mount, unregister
      on unmount), and wire `Ctrl+S`/`Cmd+S` to the screen's save with the browser default prevented while a
      sheet is mounted; `Ctrl+S` is a no-op with no sheet screen mounted (WSC-20, research R8)

**Checkpoint**: editing records exactly the changed editable leaves and a save sends only that delta,
merging the cascade and advancing the baseline; the footer, Save and `Ctrl+S` all run the same save.

---

## Phase 5: User Story 3 - Dry-run a preview that marks what changed (Priority: P2)

**Goal**: Preview sends the same delta to a stateless dry-run, merges the returned cascade **without**
advancing the baseline, and marks the changed editable rows "Modified" and the recalculated derived rows
"Recalculated" through F4's mark seam; a new edit clears the marks and re-enables Preview; a failure keeps
edits and shows the mapped reason.

**Independent Test**: with pending edits, run the preview and assert one request carrying the delta, the
derived values merged without the baseline advancing, and the marks on the changed and recalculated rows;
then edit again and assert the marks clear, and drive a failing preview and assert the edits and message
survive.

### Tests for User Story 3 ⚠️ (write first; they must fail before the implementation)

- [ ] T021 [P] [US3] Preview test `web/src/test/sheet_preview.test.tsx` (WSC-21…WSC-25, SC-009), edge
      cases first: Preview with an empty delta issues no request; a repeat preview with no new edit issues
      none; a failed preview keeps every edit, sets the mapped `previewError`, and leaves Preview
      available; an edit after a preview clears the marks and re-enables Preview; a marked row whose value
      is empty still shows its label and tint; then the happy paths — Preview sends one
      `POST /sheets/:id/preview` with the same delta, merges the cascade into the display **without**
      advancing the baseline (every edit still pending), and publishes `previewMarks` (`modified` = the
      delta's flattened paths, `recalculated` = the cascade's flattened paths), rendered through the F4
      `rowMarkBuilder`/`markLabel` seam (never a per-screen literal)

### Implementation for User Story 3

- [ ] T022 [US3] Extend the API client `web/src/api/client.ts` with `previewSheet(id, changeSet)`: one
      `POST /sheets/:id/preview` carrying `{ data: changeSet }`, decoding the returned cascade as a
      `Record<string, unknown>` exactly as `patchSheet` does, and add the operation to the `ApiClient`
      interface and its F2 contract test `web/src/test/api_client.test.ts` (WSC-21, WSC-34)
- [ ] T023 [US3] Add `preview()` to `web/src/sheet/sheet_edit.ts` per `contracts/sheet-edit.contract.md`:
      the `canPreview` gate (pending, a new `revision` since `previewedRevision`, not busy, window free),
      one preview request, the cascade merged into `data` without advancing `defaults`, `previewedRevision`
      recorded and `previewMarks` set on success, the marks cleared on any data-changing edit, and the
      failure keeping edits + mapped `previewError` with Preview still available (WSC-21…WSC-25). *Extends
      T018.*
- [ ] T024 [US3] Wire Preview into `web/src/sheet/sheet_screen.tsx`: a Preview action beside Save
      (`sheet.preview`/`sheet.previewing`, disabled while clean, without a new edit, or busy), the inline
      mapped `previewError`, and a `rowMarkBuilder` feeding `previewMarks` into the F4 engine so the marks
      tint and label their rows (WSC-21, WSC-23, WSC-25, SC-009). *Extends T019.*

**Checkpoint**: a dry-run previews the exact save payload, marks the changed and recalculated rows through
the engine seam, and never advances the save baseline.

---

## Phase 6: User Story 4 - Guard unsaved edits and throttle requests (Priority: P2)

**Goal**: `F5` re-fetches the sheet's template detail behind a discard confirmation; leaving the screen with
pending edits confirms first; both are inert in flight; and Save and Preview are each capped at three
requests per rolling 60-second window.

**Independent Test**: with pending edits press `F5` and assert the confirmation gates the template re-fetch
and that cancel issues zero requests, then confirm and assert the edits are discarded and the draft deleted;
leave with pending edits and assert the same; press `F5` on a clean sheet and assert one template request;
then drive four rapid saves and assert at most three requests inside the window and the action re-enabling
once it frees.

### Tests for User Story 4 ⚠️ (write first; they must fail before the implementation)

- [ ] T025 [P] [US4] Guards/throttle test `web/src/test/sheet_guards.test.tsx` (WSC-20, WSC-36, WSC-37,
      WSC-38, SC-011), edge cases first: `Ctrl+S` with no sheet mounted runs nothing; `F5` while a preview
      or save is in flight shows no confirmation and issues no request; a second refresh or leave
      activation while a confirmation is open is ignored; a fourth Save (and Preview) inside the rolling
      minute issues no request and the action re-enables once the window frees; then the happy paths —
      `Ctrl+S` runs the same save as the footer action; `F5` on a clean sheet re-fetches the template detail
      exactly once; `F5` with pending edits opens the confirmation, cancel issues zero requests and keeps
      the edits, and confirm discards the edits, deletes the draft and re-fetches once; leaving with
      pending edits confirms (cancel stays, confirm discards and deletes the draft) and a clean sheet leaves
      with no dialog

### Implementation for User Story 4

- [ ] T026 [US4] Add the `RollingWindow` throttle to `web/src/sheet/sheet_edit.ts` per
      `contracts/sheet-edit.contract.md` and `data-model.md` `RollingWindow`: a pure, injectable-clock helper
      (`record(now)`, `isBlocked(now)`, `nextFreeAt(now)`; `max 3`, `windowMs 60000`) held as the store's
      `saveWindow`/`previewWindow`, and wire `!isBlocked` into `canSave`/`canPreview` without replacing the
      in-flight no-op rules (WSC-38, SC-011, research R7). *Extends T018, T023.*
- [ ] T027 [US4] Extend `web/src/shell/shortcut_registry.ts` and `web/src/sheet/sheet_screen.tsx` with the
      `F5` refresh per `contracts/shortcut-registry.contract.md`: while a sheet screen is mounted, `F5`
      re-fetches that sheet's template detail exactly once; with pending edits it confirms a discard first
      (cancel, `Esc` and the barrier keep the edits and issue no request; confirm discards the in-memory
      edits, deletes the draft and re-fetches) and it is a no-op while a preview or save is in flight and
      with no sheet screen mounted (the browser default is not prevented then) (WSC-36)
- [ ] T028 [US4] Implement the leave guard in `web/src/sheet/sheet_screen.tsx`: leaving the screen with
      pending edits (a link, the nav, the browser back) confirms a discard first (confirm discards the
      in-memory edits and deletes the draft, cancel stays), a clean sheet leaves without a dialog, and an
      in-flight preview or save makes leaving a no-op; both guards are accessible dialogs (role, modal,
      labelled, focus-trapped in the F3 overlay pattern, safe action a real button, never confirm-by-Enter
      alone) resolving every string through `message()` and never stacking (WSC-37, WSC-32, WSC-39).
      *Extends T019, T027. Cross-reference: WSC-29 is a US5 requirement, but its "confirmed discard
      deletes the draft" branch lands here; US5's T031 wires the save-success branch and T010 the
      missing-sheet branch.*

**Checkpoint**: no unsaved edit is discarded silently and Save/Preview cannot exceed three requests per
rolling minute; the throttle does not replace the in-flight no-op rules.

---

## Phase 7: User Story 5 - Unsaved drafts survive a crash and are restored (Priority: P3)

**Goal**: while edits are pending the client periodically backs up only the editable delta to IndexedDB;
reopening the sheet restores it over the stored values and marks the restored fields pending; the draft is
deleted on a successful save, a confirmed discard or a missing sheet; a corrupt/unsupported/foreign draft is
ignored without raising; nothing stored is a credential.

**Independent Test**: make the cadence controllable, edit a sheet, advance the cadence and assert one backup
holding only the editable delta; reopen the sheet and assert the draft is restored over the stored values and
the restored fields are pending; then save and assert the backup is gone, and write a corrupt backup and
assert the sheet seeds cleanly instead of throwing.

### Tests for User Story 5 ⚠️ (write first; they must fail before the implementation)

- [ ] T029 [P] [US5] Draft test `web/src/test/draft_backup.test.ts` (WSC-26…WSC-31, SC-004, SC-005), edge
      cases first: a clean sheet's tick writes nothing; a tick with an empty delta writes nothing; a write
      failure is swallowed and the cadence keeps running; stopping the cadence stops the writes; a corrupt,
      unsupported-version or foreign draft is ignored so the sheet seeds cleanly and nothing throws; a draft
      key that is not an editable leaf of the loaded template is ignored on restore; then the happy paths —
      while pending a scheduler-driven tick writes the current `pendingDelta` (only the editable delta, never
      a derived value), reopening overlays only the draft's editable-leaf keys and marks them pending, a save
      success and a confirmed discard and `NOT_FOUND` eviction each delete the draft, and an inspection of
      every stored record finds no credential

### Implementation for User Story 5

- [ ] T030 [US5] Implement `web/src/library/draft_backup.ts` per `contracts/sheet-store.contract.md`:
      `startDraftCadence(store, getDelta, scheduler)` — an injectable scheduler ticking once per 60 seconds
      while the sheet is pending and the delta is non-empty, writing the delta through the port, swallowing a
      write failure, and returning a stop function that clears the schedule; a no-op while the sheet is clean
      (WSC-26, WSC-27, research R6, `AD-008`)
- [ ] T031 [US5] Extend `web/src/sheet/sheet_edit.ts` seed and lifecycle per `contracts/sheet-edit.contract.md`:
      overlay the stored draft on `seedFromSheet(template, sheet)`, applying **only** keys that are editable
      leaves of the loaded template and marking those restored paths pending (clearing `previewMarks` and
      leaving `defaults` the stored baseline), and delete the sheet's draft on save success (WSC-28, WSC-29,
      WSC-34). *Extends T018.*
- [ ] T032 [US5] Wire the cadence into `web/src/sheet/sheet_screen.tsx`: start `startDraftCadence` on mount
      with the store's `pendingDelta`, stop it on unmount, and delete the draft on a confirmed discard
      (WSC-26, WSC-29). *Extends T019, T028.*

**Checkpoint**: an unsaved edit survives a crash and is restored as pending; the draft holds only the
editable delta, never a derived value or credential.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [ ] T033 [P] Extend `web/src/test/tokens_used.test.tsx` to scan `web/src/sheet/` and `web/src/library/`
      for literal hex and px values and to assert every `var(--lg-*)` they reference is defined, so every
      visual value in the feature is proven to come from the shared tokens (WSC-33)
- [ ] T034 [P] Extend `web/src/test/l10n_strings.test.ts` with the sheet-cycle copy scan: no user-facing
      literal and no `api.error.` string exists in `web/src/sheet/`, `web/src/library/` and
      `web/src/shell/shortcut_registry.ts`, every `message('…')` key referenced there resolves to a catalog
      entry (so removing a referenced key turns the suite red — SC-006), and the `sheet.*` cycle keys are
      asserted present (WSC-32, SC-006)
- [ ] T035 [P] Add the mounted-surface accessibility case `web/e2e/sheet.spec.ts` (WSC-39, SC-010, AD-014):
      build the real client, open the creation surface and the mounted sheet screen with Playwright, and
      assert zero critical `@axe-core/playwright` violations on each and an accessible role and name on every
      control (no new axe dependency is added)
- [ ] T036 [P] Add the markup-safety case to `web/src/test/sheet_screen_states.test.tsx` (WSC-35, SC-007): a
      sheet name and a field value of `"<script>"` render as literal text and produce 0 `script` nodes, and
      the feature's sources contain no DOM-injection sink
- [ ] T037 [P] Update `web/AGENTS.md` (the new `sheet/` and `library/` modules, the `idb` direct runtime
      dependency and the `fake-indexeddb` dev-only double, the no-state-library deviation R2, the `F5`
      template-refresh behavior, and the single shortcut registry) and `web/README.md` (how to run the sheet
      suites and the accessibility case)
- [ ] T038 Run the full gate set from `web/` (`npm run format:check`, `npm run lint`, `npm run typecheck`,
      `npm test`, `npm run build`, `npm run test:e2e`) and `make lint-specs` from the repo root, and confirm
      every `WSC-*` citation in the delivered sources resolves to `spec.md`
- [ ] T039 Run the ten `quickstart.md` scenarios and record the per-scenario outcome with the feature
- [ ] T040 Record any plan-versus-delivered deviation in `web/AGENTS.md` under the deviations section — a
      dependency, pin or path that changed — rather than weakening an assertion (constitution III)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies — starts immediately (T001 and T002 are independent files).
- **Foundational (Phase 2)**: T003 is the test for T004/T005; T004 (port) and T005 (adapter) are
  independent files; T006 depends on both. Blocks every story.
- **US1 (Phase 3)**: depends on Foundational. The open state (T010), the cache helper (T011) and the route
  (T014) precede the screen (T015).
- **US2 (Phase 4)**: depends on US1's screen (T015, extended by T019) and on the store port; `sheet_edit.ts`
  (T018) precedes the screen rewire (T019) and the registry (T020).
- **US3 (Phase 5)**: depends on US2's `sheet_edit.ts` and screen (extends both); the client operation
  (T022) precedes the store's `preview()` (T023).
- **US4 (Phase 6)**: depends on US2's store and screen and on US3's preview; the throttle (T026) extends the
  store, the `F5` guard (T027) extends the registry and screen, the leave guard (T028) extends the screen.
- **US5 (Phase 7)**: depends on US2's store and screen; the cadence (T030) is standalone, the seed/lifecycle
  (T031) extends the store, the wiring (T032) extends the screen.
- **Polish (Phase 8)**: depends on all desired stories.

### User Story Dependencies

- **US1 (P1)**: needs only Foundational — the MVP.
- **US2 (P1)**: builds on US1's loaded screen; its store (T018) is independently testable against the fake
  transport and the memory store before the screen rewire lands.
- **US3 (P2)**: builds on US2's store (the preview shares the delta and the busy gate).
- **US4 (P2)**: builds on US2's store and US3's preview (the in-flight guard spans both).
- **US5 (P3)**: builds on US2's store (the draft holds the pending delta).

### Within Each User Story

- Tests are written first and must fail before the implementation lands (constitution III).
- The port before its consumers; the store method before the screen action that triggers it; the client
  operation before the store method that calls it; the store before the screen that renders it.
- Story complete before moving to the next priority.

### Parallel Opportunities

- T001 and T002 (setup) — different files.
- T003 (store test) is independent of T002 and of the foundation implementation.
- T004 and T005 — different files.
- T007, T008 and T009 (US1 tests) — independent test files.
- T016 (US2 test) is independent of T010–T015.
- T021 (US3 test) and T022 (client) — different files.
- T025 (US4 test) and T029 (US5 test) — independent test files.
- T033, T034, T035, T036 and T037 (Polish) — independent files.

---

## Parallel Example: User Story 1

```bash
# Tests first (independent files, must fail before T010–T015):
Task: "Open-situation test in web/src/test/sheet_open.test.tsx"
Task: "Creation-flow test in web/src/test/create_sheet_flow.test.tsx"
Task: "Screen-state test in web/src/test/sheet_screen_states.test.tsx"

# Then the open state, the cache helper, the query seam and the route:
Task: "Implement web/src/sheet/sheet_open.ts"
Task: "Implement web/src/library/sheet_library.ts"
Task: "Extend web/src/catalog/queries.ts with the sheet key and the create seed"
Task: "Declare the route in web/src/app/routes.ts and map it in web/src/app/router.tsx"
```

---

## Parallel Example: the two P2/P3 stories

```bash
# Once US2's store and screen exist, US3 and US5 share no file at the start:
Task: "Preview test in web/src/test/sheet_preview.test.tsx"
Task: "Draft test in web/src/test/draft_backup.test.ts"

Task: "Extend web/src/api/client.ts with previewSheet"
Task: "Implement web/src/library/draft_backup.ts"
```

---

## Implementation Strategy

### MVP First (US1 + US2, both P1)

1. Phase 1 Setup → Phase 2 Foundational.
2. Phase 3 US1: create from the catalog and open the sheet through every situation.
3. Phase 4 US2: the delta discipline, the footer/Save and `Ctrl+S`.
4. **STOP and VALIDATE** with `quickstart.md` scenarios 2, 3 and 4 (the open situations, the creation flow
   and the delta discipline).
5. The screen now edits and saves; the preview, the guards and the drafts are the next increments.

### Incremental Delivery

1. Setup + Foundational → the dependencies, the copy keys and the persistence port exist.
2. US1 → the sheet is created and opened (MVP).
3. US2 → editing records exactly the changed delta and a save advances the baseline.
4. US3 → the dry-run previews and marks what changed.
5. US4 → the guards protect the work and the throttle protects the server.
6. US5 → an unsaved edit survives a crash and is restored.
7. Polish → the two scans, the accessibility and markup cases, documentation and the gate sweep.

### Parallel Team Strategy

With two contributors: one takes the persistence (Foundational, then US5) while the other takes the
open/edit core (US1 then US2); US3 extends US2's store and screen, and US4 extends both. The two scans
(T033, T034), the accessibility case (T035) and the markup case (T036) share no file with the feature
modules and can be done at any point in Polish.

---

## Notes

- [P] tasks touch different files and have no dependency on incomplete work.
- Requirement IDs cited in task descriptions (`WSC-*`) are defined in `spec.md`; the delivered source must
  cite the same IDs so `make lint-specs` can trace them.
- Write tests first (T003, T007, T008, T009, T016, T021, T025, T029) and confirm they fail before
  implementing.
- Commit per task or per logical group, with the tests in the same commit (constitution III).
- Do not weaken an assertion to make a gate pass; retarget it to the feature that owns the subject, or amend
  the spec and record the decision instead.
- The feature must issue no request the fake transport cannot prove, author no user-facing literal outside
  `web/src/l10n/strings.ts`, derive every visual value from the shared tokens, render data as text, and
  store no credential — check those four rules on every task that touches `web/src/sheet/` or
  `web/src/library/`.
- US1 mounts the engine in inert data mode (WSC-08); the editable builder seams arrive with US2's T019,
  which extends T015 rather than replacing it.
