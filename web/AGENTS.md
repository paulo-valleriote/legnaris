# AGENTS.md — web/ (React PWA client)

Development, security and architecture rules for the client under `web/`. The **visual** rules —
principles, token architecture, colour, typography, spacing/layout, radius/border/elevation,
motion, component specifications, accessibility, security posture, per-screen patterns, enforcement
and anti-patterns — live in **[DESIGN.md](./DESIGN.md)**, which is the sole written source of the
visual values and supersedes any visual statement elsewhere (UIF-33). Design values are made
**once**, there, not per screen.

## Visual rules

Do not encode or restate visual values here or in a component. Read `DESIGN.md` for the token
architecture and the token-addition workflow, and `src/theme/tokens.css` (mirrored in
`src/theme/tokens.ts`) for the concrete values. Every component consumes `--lg-*` tokens only;
`src/theme/` is the sanctioned exception where the literals are declared, and a source scan keeps
the other directories literal-free.

## Interaction wiring

The keyboard model is centralised, not spread across screens: shortcuts live in one registry
(`src/shell/shortcut_registry.ts`) as a plain action → key map, and the shell-level leave registry
(`src/shell/leave_guard.ts`) is what a navigation asks before leaving a surface with unsaved work.
The visual side of interaction — focus ring, hover, dialog avoidance, feedback — is specified in
`DESIGN.md`.

## Security rules (non-negotiable)

- **No DOM injection sinks**: `dangerouslySetInnerHTML`, `innerHTML`, `eval`,
  `new Function`, `document.write` are forbidden — ESLint fails the build on them
  (FND-10, SEC-02). React escapes text automatically; keep data as text.
- **No third-party scripts, styles, or fonts**: everything is self-hosted and
  same-origin, so a strict Content-Security-Policy can be enforced later.
- **No credentials in JavaScript-readable storage**: never put a token in
  `localStorage`, `sessionStorage`, or IndexedDB. The refresh credential is an
  `HttpOnly` cookie; the access token lives in memory only. (A display preference such as the
  theme is not a credential and may persist.)
- All identifiers, comments, and user-facing strings in **English** (UI strings go
  through `src/l10n/`).

## API client (`src/api/`)

The typed network layer every screen mounts on. Its rules are non-negotiable.

- **One transport seam.** Every request goes through the transport injected at
  construction; no operation builds its own, and the transport is the single place a
  credential will ever be attached. Requests carry `credentials: 'omit'` — no ambient
  credential is attached implicitly, in any environment.
- **No logging.** The module contains no `console.*` call and imports no logger; a request
  body, a credential and an identifier-bearing URL never reach a log. A source scan
  (`src/test/api_no_logging.test.ts`) enforces this.
- **No user-facing literal.** Every message a person sees resolves through
  `src/l10n/strings.ts`; the API module emits codes and failure kinds, never copy.
- **Failures are classified, drift is distinct.** A refusal becomes a `DomainFailure` with
  a `FailureKind`; an unreadable response stays a `DefensiveParseFailure`; a malformed
  dotted path raises a `PathFailure`; an unusable base address raises a
  `ConfigurationFailure` at construction. A new backend code must be given a kind in
  `error_mapper.ts` — the mapping table is total over the codes the backend emits.
- **Captures are never hand-edited.** `src/api/fixtures/` holds real responses written
  verbatim by `capture.sh`; a capture that stops matching the models is a backend contract
  change and the pinned suite is expected to go red on it.

## UI engine (`src/engine/`)

The schema-driven rendering core (feature `web-ui-engine`): it turns a decoded
`FieldSchema` tree into labeled sections and controls and owns **no state** — no value,
no request, no mark. It mounts no screen; every case renders its modules directly.

- `field_grouping.ts` — pure derivation of ordered, labeled sections and render rows
  (stacked leaf, declared-column table, exclusive group).
- `field_control.tsx` / `exclusive_control.tsx` — one control per leaf schema, one
  single-choice control per exclusive group.
- `sheet_form.tsx` — the composition; `sheet_defaults.ts` — the draft seed/overlay/merge;
  `sheet_mark.ts` — the preview-mark contract; `engine.css` — token-only styles.
- Every string resolves through `src/l10n/strings.ts`; every visual value is a `--lg-*`
  token. Data is rendered as text; markup in a value or label stays literal (no DOM sink).
- **Stable units for the Layout.** Every Section renders one `<section data-section-key>`
  (the top-level field key) holding its title and one `.lg-section__rows` container; every Row
  renders one wrapper carrying `data-row-key` and `data-row-kind` (`normal` | `exclusive` |
  `table`). The row key comes from `rowKey(row)` in `field_grouping.ts`
  (`<path>`, `<parent path>#<groupId>`, `<group path>`; the same keys the backend validates a
  layout against, ADR 0001): it depends only on the template, never on position or value, and is
  unique across the sheet. A table Row is self-sufficient: it draws its own column header inside
  its wrapper, shared with the next Row only when that Row is the adjacent table Row with the same
  columns (`needsTableHeader`), judged on the Rows as drawn, so a pin that changes who neighbours
  whom still leaves exactly one header per run of same-column table Rows.
- `layout.ts` — pure resolution of a `Layout` (`{section_order, pinned}`) against the Sections
  (`resolveArrangement`: partial lists valid, missing Sections appended in Template order, stale and
  duplicate keys ignored, first wins), `arrangementView` (every Section in order, each holding all
  its Rows: its pinned Rows first in pin order, the rest in Template order; a Row never leaves its
  Section) and `arrangementToLayout` (only the lists that deviate from the Template; `null` when
  none does). `arrange_ops.ts` — the staging operations (move a Section to an index, pin, unpin,
  reset); each returns the same reference when it changes nothing. Both are stateless.
  `isNumberRow` names the lone `number` leaves (editable or derived, never a `config.*` switch).
  The vitals strip of ADR 0002 was removed: a pinned Row never leaves its Section.
- `dependents.ts` / `dependent_readout.tsx` (sheet-object brief A12) — a derived value beside the
  input that drives it. `buildDependentsIndex` inverts `schema.dependsOn` over the Sections' leaves
  (direct edges only; a missing, external, empty, repeated or self dependency is dropped, so a cycle
  cannot recurse), `dependentsOf(index, source)` returns the dependents of an editable
  number input that is not a `config.*` switch and only when it feeds 1 or 2 (the count is of the
  schema), and
  `dependentsAreStale(edited, previewed)` says whether the input was edited and no Preview has
  settled it. No formula is evaluated and nothing previews on its own (Preview stays throttled).
  `SheetForm` draws a `DependentList` on a second line of a lone Row (never in a table cell, never
  while arranging, whose compact frame drops its children) and gives `controlBuilder` a third
  argument, the list's id, which the numeric input uses as `aria-describedby`; `dependentBuilder` is
  the seam through which the screen supplies each readout (`ConnectedDependent`: its own value
  subscription, stale from the edit store's `editedPaths` and `previewMarks`). Data mode draws the
  stored values, never stale. The list is a description, not a live region: the footer still owns the one
  announcement of a cascade.
- `SheetForm` takes an optional resolved `arrangement`, replaceable structural `frames`
  (`Root`, `Section`, `Row`), so a caller adds chrome without the engine knowing it. The engine
  imports no drag library.

**Row label ids.** `SheetForm` gives each lone Row's label an id (from `useId`, so two forms never
collide) and passes it to the control source: `controlBuilder(field, labelId)` and
`SheetFieldControl`'s `labelId`. A derived readout uses it as `aria-labelledby`; a table cell
receives none and keeps its `aria-label`.

**One control source.** `SheetForm` takes exactly one: `data` + `onChanged`, or both
`controlBuilder` and `exclusiveControlBuilder`. Any other combination throws rather than
half-render (ENG-25). In builder mode the composition reads no value, so the caller owns
the per-field subscription and edit reporting through the seams (`controlBuilder`,
`exclusiveControlBuilder`, optional `rowMarkBuilder`) — the web expression of `AD-001`.

**Numeric guard.** The numeric input seeds from its stored value, holds its display text
locally, and reports an edit only when the whole text resolves to a finite number: an
empty field clears the value, and a text that can never resolve (`abc`, `--3`, `3-`,
`NaN`/`Infinity`/`1e999`) is rejected, leaving the stored value and the shown text unchanged.
A sign or point typed so far (`-`, `.`, `-.`) is kept as display text and reports nothing, so
`-3` can be typed key by key; `-0` reports `0`, never a negative zero. Leaving the field with
only such a pending text restores the text of the last reported value. The iOS decimal pad has
no minus key, so a focus-only sign toggle (`Toggle sign of <label>`, not a tab stop) flips the
leading minus through the same path (ENG-12).

Phone editing is the same guard made faster. Focus selects the whole text (in a frame after focus,
and only while the text is unchanged, so an early keystroke is never selected over; a tap on an
already focused field places the caret as usual; a disabled field selects nothing). In a lone
Row (the control receives a `labelId`) an editable numeric leaf also offers `Decrease <label>` /
`Increase <label>` steppers while focused, each a full `--lg-size-control` target, not tab stops,
reporting through the same path as typing; ArrowDown/ArrowUp step in every editable numeric field.
The rule reads only the schema and the layout slot: editable `number` leaf, not derived, not inert,
not a table cell (a cell is a fraction of a row; three full controls cannot fit there, so a cell
keeps the sign toggle only). A step is 1 because the field schema carries no step, minimum or
maximum; the sum is rounded to the decimals of its start (`0.7 - 1` is `-0.3`), a pending text
(`-`, `.`, `-.`) and an empty value step from zero, `-0` never results, and a result that is not
finite or that a large value cannot tell from its start is refused, never reported. The sign
toggle, the digits and the steppers are plain flex items of one wrapper, so no button covers a
digit; with steppers the focused wrapper lifts out of its slot and grows toward the label (the
row never changes height), and stays exactly the field's width when the row is roomy.

**Mark tint.** A marked control reuses `--lg-color-surface` from `tokens.css`; no new
token was added for the preview tint.

## Sheet cycle (`src/sheet/`, `src/library/`)

The open/edit/preview/draft feature over one sheet (feature `web-sheet-cycle`). It reuses
the F2 client and the F4 engine and owns no wire model.

- `src/sheet/sheet_open.ts` — the open situation for the `:id` route: one `GET /sheets/:id`
  then one `GET /templates/:id`, a seeded fresh `['sheet', id]` rendered with no second
  fetch, and a `NOT_FOUND` that evicts the sheet and its draft.
- `src/sheet/sheet_edit.ts` — the in-memory edit store: `editedPaths`, the copy-on-write
  `data`, the pending delta, the save/preview lifecycle, the preview marks, two rolling
  throttle windows and the draft restore. It emits immutable snapshots for
  `useSyncExternalStore` and binds no listener itself.
- `src/sheet/sheet_screen.tsx` — the mounted screen: header, the F4 form in builder mode,
  the sticky footer and the one guard modal.
- `src/library/sheet_store.ts` — the `SheetStore` port, the versioned envelopes and the
  in-memory double every suite injects; `idb_sheet_store.ts` is the only module importing
  `idb`; `sheet_library.ts` and `draft_backup.ts` hold the cache helpers and the injectable
  draft cadence.
- `src/sheet/layout_store.ts` — the layout store (Arrange mode), **separate from the
  edit store**: the committed arrangement, the staged one (defined only while arranging), the commit
  lifecycle and the best-effort backup of a pending arrangement. `src/sheet/arrange_frames.tsx` — the Arrange
  chrome (Section handle on `@dnd-kit`, Row star) injected through the engine's frames.
- `src/sheet/view_store.ts` — the per-viewer, per-sheet **view state** (collapsed Sections and the
  pinned filter), **separate from the edit store, the layout store and the IndexedDB `SheetStore`**:
  the collapsed keys and the pinned filter persist (`{v: 1, collapsed, onlyPinned}`), in `localStorage` under `lg.sheet-view.<sheetId>` behind the
  injectable `ViewStorage` seam (`useViewStorage`, like the theme); every read or write failure is
  swallowed and a corrupt, wrong-version or stale record falls back to defaults. It never touches the
  network, never counts as unsaved work and never reaches the leave guard. `sheet_view.tsx` resolves
  it against the modes (the filter reads off while arranging or pinning and Sections draw open while
  arranging, the state untouched); `section_index.tsx` / `use_active_section.ts` are the sticky strip (roving `nav`, not
  tabs) and its scrollspy, which announces through the sheet's one polite live region.
- `src/shell/shortcut_registry.ts` — the one action → key map (`Ctrl+S`/`Cmd+S` save, `F5`
  refresh), with the mounted screen registering and unregistering its handlers;
  `src/shell/leave_guard.ts` — the shell-level leave registry the navigation asks before it
  leaves a surface with unsaved work.

**One edit rebuilds one control.** A field control subscribes to its own dotted path
(`store.subscribePath`) and reports edits back through the engine's `controlBuilder`/
`exclusiveControlBuilder` seams; the screen subscribes only to the coarse signals it renders
(footer, marks). This is the web expression of `AD-001`, obtained with
`useSyncExternalStore` — no state library is added.

**Delta, never a snapshot.** A save and a preview send only the pending editable delta,
rebuilt through the engine's `setPathValue`; a save merges the returned cascade and advances
the baseline, a preview merges it **without** advancing it and marks the changed and
recalculated rows through the F4 `rowMarkBuilder`/`markLabel` seam.

**Footer status and `revealRow`.** The footer has one status line (`FooterStatus`): `footerStatus(pending, count)` in `preview_summary.ts` merges the unsaved state and the cascade ("Unsaved, 3 recalculated"), derived from the store's `previewMarks`, so it clears whenever the marks clear. Jump (a ghost button) sits on the same line; the one polite live region announces only the cascade and the visible phrase is `aria-hidden` while a Preview shows. The derived readout draws no "Calculated" caption: `ReadOnlyValue` keeps it as a visually hidden `aria-describedby` target. `reveal_row.ts` owns
bringing a Row or a Section into view by its `data-row-key` / `data-section-key` (`revealRow`,
`revealSection`, `revealFirstRecalculated`); handed the view controls (`RevealView`), it first turns
the pinned filter off and expands a collapsed Section, synchronously, so no caller does either. Auto-preview on
blur was evaluated and not built: Preview is throttled per window (`THROTTLE_MAX`), so passive calls
would spend the player's budget, and a merged cascade would rewrite values under a field being edited.

**Two guards, one modal.** `F5` refreshes the sheet's template detail and leaving the screen
with pending edits asks to confirm first; both open the screen's single confirmation modal,
so two guards never stack. The safe action carries focus, Escape and the overlay barrier
cancel, and the destructive action is a real button — never a confirm-by-Enter default.

**Best-effort local records.** A cache write, a draft write and every delete are swallowed on
failure; no local record ever holds a credential, and the draft holds only the pending
editable delta, restored over the stored values and filtered to the template's editable
leaves.
The one exception is an offline save or preview failure: the edit store writes the pending delta
to the draft immediately (not on the 60 s cadence) and the footer says "kept on this device" only
when that write landed; otherwise it says the changes could not be kept (`sheet.error.*OfflineUnkept`).

## Layout and Arrange mode (ADR 0001)

The player's order of Sections and the pinned Rows form the sheet's `layout`
(`{"section_order": [...], "pinned": [...]}`; `GET /sheets/:id` carries it only when set;
`PUT /sheets/:id/layout` replaces it in full with `{"layout": {...}}`, `{"layout": null}` resets it,
no recalculation). Only the lists that deviate from the Template are sent.

- **Explicit mode, one commit.** Arrange mode is a toggle in the sheet screen. Nothing is sent
  while arranging: `Done` issues exactly one `PUT` (and none when nothing changed), `Cancel`
  discards everything, `Reset to default` stages the Template default and is committed by `Done`.
  A failed `PUT` keeps the staging, shows the mapped copy and `Done` retries.
- **A layout is not a value.** The layout store never touches `sheet_edit.ts`: arranging never marks
  the sheet as having unsaved values, `Ctrl+S` saves nothing while arranging, and no value save or
  preview carries a layout. `LoadedSheet` keys the edit store on the sheet's id and values only, so
  committing a layout (which updates the cached sheet) never rebuilds it.
- **One guard, one modal.** A pending arrangement (and an in-flight commit) counts as unsaved work
  for the leave guard and for `F5`; both open the screen's single confirmation modal, and
  confirming discards the staging, its backup and any value edits.
- **Rows never move, they pin.** A pinned Row rises to the top of its own Section, ordered by pin
  time (the order of `pinned`); unpinning returns it to its Template position, and a Section whose
  Rows are all pinned renders as any other. There is no page-level pinned zone, no Row reordering and no Move up / Move
  down control. A `pinned` key naming no Row is ignored on read; the layout
  store resolves every staged result, so one never stays staged or reaches the wire.
  An optional **pinned filter** in the play view (per viewer and per sheet, remembered in the view
  store, never part of the Layout, never sent to the server and never unsaved work; off until first
  used) draws only the pinned Rows, each Section that has any as its normal card; it is a filter
  over the same Sections, not a zone, and a Row is still drawn once. A reveal that must show a
  filtered-out Row turns the filter off and that is remembered too.
- **Pinning lives in Arrange mode.** The instant-save pin mode of ADR 0002 was removed: a Row is
  pinned by its star while arranging, staged with the Section order and saved by the one `PUT` on
  Done. `Edit pins` (section strip, pinned-filter empty state) enters Arrange mode and focuses its
  hint. The live region announces `arrange.pinned` / `arrange.unpinned`. There is no cap on pins.
  Known limit: the layout store is seeded from the layout at open, so a Template refresh that
  rebuilds Sections within a session reseeds it from that first layout.
- **Sheet header.** The sheet name is the heading, with the template name as a muted line under it
  (a kicker above a heading is banned). The actions are one hairline-divided group of ghost text
  buttons (`Help`, `Arrange`); `Collapse all` lives in the section strip's `trailing`
  slot. The vitals strip, `Pin suggested rows` and `suggestionsUsed` of ADR 0002 were removed:
  pinned Rows rise to the top of their own Section. `src/sheet/sheet_help.tsx`: a `Help` disclosure in the
  header actions (`aria-expanded`, `aria-controls`) opens a quiet bordered note (not a modal, never
  moves focus; an icon close button named `Got it`) explaining the calculated readout and Preview versus Save; it opens by itself until
  dismissed once (`explainerSeen`, view store) and only when the Template has a calculated leaf.
  The sticky section strip's leading slot is the filter star
  (name "Pinned only" read always, drawn from `40rem`) plus a chip with the sheet name (scrolls to
  top, `revealTop`), and a section link whose Section holds preview-marked Rows carries a count dot
  (`markedRowCounts`).
- **Two controls.** In Arrange mode a Section opens with a focusable drag handle (pointer, touch
  and keyboard: `Space` lifts, arrows move, `Space` drops, `Esc` cancels; a live region announces
  every drag) and a Row is its label plus a star toggle (`aria-pressed`, always named `Pin <label>`,
  outline = unpinned, filled = pinned, inline SVG on `currentColor`). Only Sections
  are reorderable. Outside Arrange mode no handle, star or drag listener is mounted.
- **Local records.** The cached sheet carries the committed `layout`; a pending arrangement is
  backed up on every staged change (`writeLayoutDraft`, best-effort, swallowed failures, no
  credential) and restored into Arrange mode on open. `SCHEMA_VERSION` is 3: older local records
  read as missing and are rebuilt, not migrated.
- **`@dnd-kit` (core, sortable, utilities)** is the one drag dependency (exact pins, bundled by
  Vite, self-hosted). It touches the DOM only through React props and CSSOM (`style.setProperty`),
  never `innerHTML` or a `<style>` element, so the production CSP (`style-src 'self'`, Trusted
  Types) needs no relaxing; it is mounted only while arranging, and only Sections are sortable.

## Theme (`src/theme/`)

- `theme.ts` — the theme store over injected seams (storage, media-query reader, root). The
  resolution order, the live system follow while nothing is stored, and every swallowed
  storage failure are specified in `DESIGN.md`; the accepted values are exactly `light` and
  `dark` under `lg.theme`.
- `use_theme.ts` — the `useSyncExternalStore` hook the control and any surface read.
- `public/theme-init.js` — the same-origin classic head script that resolves the theme before
  the first paint; no inline script and no policy hash (`UIF-20`).

## Componentization

- Extract a shared component only when all hold: **≥ 3 real usages**, same shape and
  purpose, and the extraction does not leak schema-specific logic into it.
- Never abstract the schema-driven UI Engine internals, screen-specific layouts,
  one-off dialogs, or wireframe structures.
- One sanctioned base layer at `web/src/ui/` holds the five primitives (`button`, `field`,
  `checkbox`, `dialog`, `toast`) plus the focus trap; it imports no feature directory,
  uses `lg-<block>__<element>--<modifier>` and keeps the `≥ 3 real usages` rule for every
  other extraction (UIP-01, UIP-03, SC-007). No wrapper-for-wrapper's-sake elsewhere.

## Dependencies

- **No new package without strict necessity.** `@dnd-kit/{core,sortable,utilities}` is the
  justified exception for Arrange mode (accessible keyboard, pointer and touch reordering of
  Sections with screen-reader announcements is not worth hand-rolling). Design-system dependencies (font
  packages, theme frameworks, component libraries) are prohibited — the tokens plus plain
  CSS are sufficient, and the font is vendored, not depended on.
- If a dependency is ever unavoidable, justify it in the PR description.

## Delivered deviations from the feature task lists

Recorded here instead of weakening an assertion (constitution III) or silently
dropping a task.

### F1 — `web-pwa-foundation`

- **No `src/app/providers.tsx`** (T019). V1 has no provider layer: the shell wraps the
  route table directly in `src/app/router.tsx`. The module lands with the first feature
  that genuinely needs shared context (`web-api-client`).
- **Tests are flat in `src/test/`** with a `*.test.ts(x)` suffix rather than co-located
  next to each source module, because the Vitest setup, the `matchMedia` mock, the
  contrast helper, and the render helper are shared by every suite.
- **Toolchain pins differ from the plan's aspirational ones** — TypeScript 5.9.3 and
  ESLint 9.39.5, because `typescript-eslint`, `eslint-plugin-react`, and
  `eslint-plugin-jsx-a11y` do not yet support TypeScript 6 / ESLint 10. See the
  "Toolchain version note" in `web/README.md`.
- **No `@lhci/cli`** (the plan's tooling line named Lighthouse CI). Nothing invoked it:
  no npm script and no CI step consume it, because the Lighthouse gate belongs to F8
  (`F8-T005`). Its dev-only tree carried the only advisories the `sca` gate could see
  (17 findings, 11 high, two of them — `extract-zip` and `sprintf-js` — with no patched
  release), so the gate could not pass while it was installed. `web/lighthouserc.json`
  (T025) stays; the runner arrives with the F8 gate. Decision: `AD-011`.

### F2 — `web-api-client`

- **No MSW** (the program plan's stack table named it). The injected transport satisfies
  API-07 and makes the retry count and the `credentials: 'omit'` posture directly
  assertable, so no network-interception dependency was added. Recorded in the plan as a
  deliberate deferral.
- **No app-level client wiring yet** (T021, reworded). No screen constructs the client in
  this feature, so `resolveApiConfig` is documented in `web/README.md` as the single
  source of the base address rather than wired into an app entry point; the first feature
  that constructs the client consumes it. The insecure state is reported on the resolved
  configuration, not rendered.
- **A shared decoding helper, `src/api/parse.ts`**, is not named in the task list. It
  holds the payload-narrowing helpers (`isPlainObject`, `drift`, `requireString`, …) that
  the envelope and both models share, instead of duplicating them per module.
- **The INTERNAL_ERROR capture comes from an unmatched route.** No consumed endpoint
  deterministically returns 500, so `error_internal.json` is captured from `GET
/no-such-route`, which surfaces through the global handler as
  `{"code":"INTERNAL_ERROR"}` — a real response, never a hand-written stand-in.
- **The live suite's two mechanism cases assert the gate predicate**, not a real skip;
  the API-38 flow cases run only against a configured backend (`API_BASE_URL`).

### F3 — `web-catalog`

- **`@tanstack/react-query` 5.104.3 is added** (`AD-016`). It is the program plan's
  server-state choice (Parte 3, the F3 task "Queries TanStack"), recorded here as an
  inherited plan decision rather than a new one; it is the feature's only new runtime
  dependency, and the query client disables the library's own retry so the retry policy
  stays the F2 client's.
- **`src/shell/empty_workspace.tsx` is deleted** and the root route renders
  `src/catalog/catalog_screen.tsx` (`AD-013`). The F1 shell/routing/e2e assertions that
  pinned the empty workspace are retargeted to the catalog, not deleted.
- **The left navigation's root item is relabelled "Catalog"** through
  `nav.catalog` (`AD-015`, WCT-29), and `src/app/routes.ts` follows.
- **The creation surface is an accessible overlay, not a native `<dialog>`** (R5):
  `jsdom` does not implement `HTMLDialogElement.showModal`, so the platform element could
  not be driven by the rendering tests the constitution requires; the overlay keeps the
  same `role="dialog"` / `aria-modal` / Esc / focus contract.
- **The `.lg-empty*` rules stay in `src/shell/shell.css`** (deviation from T011). T011
  called them "now-unused" and asked for their removal with `empty_workspace.tsx`, but
  `src/shell/not_found.tsx` renders the same `.lg-empty` / `.lg-empty__title` /
  `.lg-empty__text` classes; deleting the rules would strip the not-found screen's
  styling. They are live CSS, so they are kept.
- **The dialog's accessible title is `sheet.dialogTitle`; the template's display name is
  rendered in the detail area.** The contract's title row and the "detail area resolves
  the display name" obligation are both satisfied this way, and the dialog always has a
  non-empty accessible name (WCT-14, WCT-27).
- **`playwright.config.ts` builds the e2e client with `VITE_API_BASE_URL` set to the
  preview origin**, so the browser specs intercept the catalog request locally instead of
  the built client reaching the production API; `e2e/catalog.spec.ts` blocks the service
  worker and scripts the two responses.

### F4 — `web-ui-engine`

- **The engine mounts no surface** (spec Clarifications, 2026-10-16). No screen, no
  `web/e2e/` case and no test-only route were added; every case renders the engine's own
  modules directly, so the feature proves `ENG-01…ENG-29` without touching the shell.
- **The `rowMarkBuilder` seam landed with the composition (T012), not T016.** The task list
  ordered the seam into US3, but the composition's label row is where the seam plugs in, so
  it shipped with `sheet_form.tsx` and US3 only added the mark tests and the shared
  `markLabel` copy helper.
- **`markLabel(mark)` is exported from `sheet_form.tsx`.** The desktop screen held the
  "Modified" / "Recalculated" copy in its own localization; here the engine owns the two
  catalog keys (`engine.modified`, `engine.recalculated`) so a caller's `rowMarkBuilder`
  names no literal. `sheet_mark.ts` stays behavior-free (a type and a class constant).
- **A section uses the group's declared label when non-empty, else a humanized key.** The
  contract (`R7` / data-model) requires the label; the desktop quirk that ignores a
  group's label is not carried over, because the contract is authoritative here.
- **The numeric input holds its display text locally plus a forced re-render** (a
  `useReducer` tick) instead of the desktop's `TextEditingController`: React would otherwise
  refuse to visually reject a text that can never be a number, so it re-renders the controlled
  input back to the last accepted text (ENG-12). A pending `-`/`.` is kept (see "Numeric guard");
  its blur restore is limited to that pending text.
- **Data mode renders exclusive controls inert** (no `onSelect`), matching the desktop's
  read-only preview; exclusive editing flows through `exclusiveControlBuilder`.
- **The engine tests are flat in `src/test/`** with an `engine_*` prefix, matching the F1
  layout, and the token/copy scans (`tokens_used.test.tsx`, `l10n_strings.test.ts`) now
  cover `src/engine/`.

### F5 — `web-sheet-cycle`

- **No state library** (R2, `AD-001`). The plan's stack table named "Zustand + selectors (or
  Jotai per-path)"; the per-path subscription is React's `useSyncExternalStore`, so no state
  package was added. `idb` 7.1.1 is the feature's one new runtime dependency (already resolved
  transitively through `workbox-expiration`), declared directly; `fake-indexeddb` 6.2.6 is
  dev-only, because jsdom ships no IndexedDB.
- **The store reads its draft through `restore()`, not a seed field.** The draft read is
  asynchronous, so `createSheetEditStore` stays synchronous and `store.restore()` overlays the
  stored draft after construction; this keeps `SheetOpenResult.loaded` unchanged (a seed-time
  draft would have forced the draft into the open result).
- **The leave guard is a shell-level registry, not a router blocker.** `useBlocker` requires a
  data router, which the F1 shell does not mount, so the guard is a small module-level registry
  (`src/shell/leave_guard.ts`) that `LeftNav` asks before it navigates. **Limitation:** a
  browser-back gesture is not interceptable this way; only the in-app navigation is guarded.
- **The guard modal is a screen-local component.** The refresh and leave confirmations are two
  uses of one component inside `sheet_screen.tsx`, not a shared `ConfirmDialog`: two usages do
  not meet the `≥ 3` extraction bar, and the single instance is what keeps the two guards from
  stacking.
- **The F3 dialog also renders `sheet.creating`.** T002 added the "Creating sheet" key; the F3
  `CreateSheetDialog` is retargeted from the idle label to that key so the in-flight state has
  its own copy rather than showing "Create sheet".
- **`.lg-button--danger` is an outlined variant.** No `--lg-color-danger-hover` token exists,
  so the destructive button keeps the surface background with a `--lg-color-danger` border and
  text and tints to `--lg-color-surface-hover` on hover — a real hover change with no new
  token (`--lg-color-danger-text` stays declared for a future filled variant).
- **The token and copy scans now cover the feature.** `tokens_used.test.tsx` scans
  `src/sheet/` and `src/library/` for literal colors and pixels, and `l10n_strings.test.ts`
  scans `src/sheet/`, `src/library/` and `src/shell/shortcut_registry.ts` for user-facing
  literals and unresolved keys (WSC-32, WSC-33).

### F7 — `web-auth-session`

- **`session_store.ts` shipped as `auth_session.ts`.** The plan named the session module
  `src/auth/session_store.ts`; it landed as `src/auth/auth_session.ts`, because the module
  is the session itself (state + the four operations + the in-memory token), not a store
  any surface subscribes to through a separate seam.
- **`authenticated_client.ts` shipped as `authenticated_transport.ts`.** The plan named the
  decorator `src/api/authenticated_client.ts`; it landed as
  `src/api/authenticated_transport.ts` and decorates the `Transport`, not the `ApiClient`,
  so the Bearer attach, the single-flight refresh and the one replay sit at the wire seam
  every request already crosses.
- **No token store — the access token is memory-only** (R9, WAS-32). There is no
  `localStorage`/`sessionStorage`/cookie write anywhere in `src/auth/`; the token lives in a
  module-private field and is exposed only to the authenticated transport and the session's
  own snapshot. A reload therefore holds no session until the silent refresh re-establishes
  it, which is the intended "nothing a script can persist" posture.
- **Single-flight refresh lives on the session, not the decorator.** `AuthSession.refresh()`
  coalesces concurrent callers over one `/auth/refresh` (`refreshInFlight`); the
  authenticated transport calls `session.refresh()` instead of owning its own in-flight
  promise, so a burst of `401`s from any surface shares one rotation (WAS-12, WAS-13).
- **Sign-out has no confirmation dialog.** The plan left the confirmation open; sign-out
  drops the token and sweeps the cache immediately, and the control is disabled while the
  request is in flight, so a double activation issues one `/auth/logout` (WAS-19, WAS-23).
  A confirmation was judged unnecessary because the action is recoverable by signing in
  again and the in-flight guard already covers the double-activation edge.
- **The fake transport needed no extension** (T001). `createFakeTransport` records the whole
  `HttpRequest`, so `method`, `credentials`, `url` and `headers` are already readable off each
  recorded request once `T004` widened the type; the scripted `{status, body}` queue was
  already in place, so the seam is unchanged.
- **The browser specs seed `/auth/refresh`.** `playwright.config.ts` builds the e2e client
  with `VITE_API_BASE_URL` pointing at the preview origin, so the built client's cold-start
  refresh targets that origin; `e2e/auth_seed.ts` routes `/auth/refresh` to a session body so
  the previously public shell, catalog, sheet and PWA specs still render behind the session
  (WAS-08, `AD-013`), and `e2e/auth.spec.ts` adds the axe sweep over the auth surface, the
  signed-in shell and the "Your sheet" surface (SC-010).

### F1 — `web-ui-foundation`

- **The vendored font artifact is `Geist-Variable.woff2`**, not the plan's `Geist[wght].woff2`:
  brackets in a `url()` token and an HTML attribute require percent-encoding, and the official
  file is already named `Geist-Variable.woff2`. The file's role is unchanged (`R4`).
- **The theme control lives at `src/shell/theme_toggle.tsx`**, with its logic in
  `src/theme/theme.ts` and `src/theme/use_theme.ts`. `shell/` stays under the literal scan, so
  the control's styles are token-only and it authors no visual value.
- **`--lg-size-content-min` (320 px) is added** to the size primitives. The two-region grid
  needs a content minimum so the content never collapses under the navigation, and the literal
  scan forbids a bare pixel value in `shell.css`.
- **The motion tokens are named `--lg-motion-120/180/240`**, matching their declared durations;
  the source plan spelled the first `--lg-motion-100` while giving it 120 ms.
- **`src/shell/use_media_query.ts` is deleted** with the panel; nothing else used it once the
  shell stopped reading the narrow breakpoint.
- **The retargeted suites.** `shell.test.tsx`, `shell_responsive.test.tsx`,
  `shell_keyboard.test.tsx` and `web/e2e/shell.spec.ts` now assert the two remaining regions
  and the removed panel, and `no_inline_assets.test.ts` accepts the two external scripts (the
  same-origin theme boot script plus the module entry) while still forbidding inline bodies
  (`UIF-32`).

### F8 — `web-auth-code-step`

- **The code step is a state of the auth surface, not a route** (WCS-02). `AuthGate` renders
  `AuthCodeScreen` in place of the credential form while a step is pending; no entry was added
  to `src/app/routes.ts` and the URL never changes. The challenge handle lives in a
  module-private field beside the access token, so no subscriber can read or persist it.
- **The values the form comes back with live in the session** (WCS-21). The credential form
  unmounts while the step is pending, so the address and the account mode survive the
  abandonment in the session's `restored` projection rather than in a component's local state.
  The snapshot gained that field; the contract and the data model were amended in the same
  change.
- **The credential operations decode a discriminated outcome** (WCS-01). `register` and `login`
  answer either a session or a code step, and the discrimination reads the payload's own keys,
  never the status: registration answers `201` in both gate states. A payload carrying both
  keys or neither is a `DefensiveParseFailure`.
- **The copy that carries a value resolves through the catalog** (WCS-13). `messageWith()` is
  the parameterized resolution next to `message()` in `src/l10n/strings.ts`; no surface
  concatenates a sentence with a value.
- **The pinned capture carries a redacted token** (API-35, WCS-14). The verification capture
  keeps the session body's shape with the `redacted-access-token` placeholder, and the pinned
  suite's credential assertion looks for a credential _value_ instead of the key's name.
- **The capture script reads the code from the send intention** (WCS-14). The code never
  reaches a log line or the sender's output, so `capture.sh` reads it from the `email_outbox`
  collection with `mongosh`, and the run removes the send intentions it recorded so the day's
  allowance is restored. The fixtures' README documents the requirement and the quick local
  configuration.
