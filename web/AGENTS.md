# AGENTS.md — web/ (React PWA client)

The client platform — shell, routing, theme, shared UI primitives, copy, typed API client, offline
behaviour and security posture — is documented in
[`../context/web-client.md`](../context/web-client.md). Visual rules — principles, token
architecture, colour, typography, spacing/layout, radius/border/elevation, motion, component
specifications, accessibility, per-screen patterns, enforcement and anti-patterns — live in
**[DESIGN.md](./DESIGN.md)**, the sole written source of the visual values. Design values are made
**once**, there, not per screen.

Shared primitives live in `src/ui/`; screens compose them instead of re-implementing controls.

The sections below hold the working rules for the engine, the sheet cycle, dice and the Layout.

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
  layout against): it depends only on the template, never on position or value, and is
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
  A pinned Row never leaves its Section.
- `dependents.ts` / `dependent_readout.tsx` — a derived value beside the
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
half-render. In builder mode the composition reads no value, so the caller owns
the per-field subscription and edit reporting through the seams (`controlBuilder`,
`exclusiveControlBuilder`, optional `rowMarkBuilder`).

**Numeric guard.** The numeric input seeds from its stored value, holds its display text
locally, and reports an edit only when the whole text resolves to a finite number: an
empty field clears the value, and a text that can never resolve (`abc`, `--3`, `3-`,
`NaN`/`Infinity`/`1e999`) is rejected, leaving the stored value and the shown text unchanged.
A sign or point typed so far (`-`, `.`, `-.`) is kept as display text and reports nothing, so
`-3` can be typed key by key; `-0` reports `0`, never a negative zero. Leaving the field with
only such a pending text restores the text of the last reported value. The iOS decimal pad has
no minus key, so a focus-only sign toggle (`Toggle sign of <label>`, not a tab stop) flips the
leading minus through the same path.

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
  tabs) and its scrollspy, which announces through the sheet's polite live region.
- `src/sheet/preset/` — the template-id registry and the `fantasy_v1` Play preset: a `Header`
  component plus ordered `sections` slots sharing one preset context over the edit store, Dice
  context and decoded template. `fantasy/` holds the hero, the combat strip, the ability medallions
  and the saving-throw/skill cards: the strip container later cards extend, holding the hit-points
  card (`fantasy/hp_card.tsx`) over the pure `fantasy/hit_points.ts` (damage spends temporary points
  first, healing caps at the maximum, non-positive amounts are ignored), sticky on a desktop and
  static on a phone; the card writes `combat.hitPoints` and `combat.hitPointsTemp` to the pending
  delta and draws the unsaved difference as a hatched segment with `was <n>` until Save. The
  medallions pair a large `RollableValue` over the Dice store with an inline-editable score badge
  and the passive-awareness pill; the saving-throw/skill rows carry an `aria-pressed` proficiency
  dot that writes the matching `config.save<A>Prof` / `config.skill<S>Prof` leaf and previews so the
  derived total follows, with inline-editable skill bonuses and totals rolling through the Section
  dice (pure tables, path builders and the bonus clamp in `saves_skills_data.ts`).
  `rollable_value.tsx` with `useRowRoll` (`dice_row.tsx`) is the reusable roll target later gameplay
  sections reuse. One file per Play section means a new panel is one `sections` entry:
  `fantasy/spells.tsx` draws the spellcasting panel (the ability as `segmented_choice.tsx`, a
  reusable exclusive-group control independent of the engine's native select; the rollable spell
  attack through the Dice store with a preset row default; slot pips from the pure
  `fantasy/slot_pips.ts`; totals through the engine's numeric guard) and `fantasy/rules.tsx` the
  table-rules disclosure, collapsed by default. A template without a preset keeps the generic form.
  `mode_toggle.tsx` is the Play/Edit switch, component state for the open screen (local to the
  viewer, never persisted, never sent); Edit keeps the generic form, Arrange and the pinned filter,
  Play ignores the layout and view filters. `src/sheet/inline_edit.tsx` is the reusable one-path
  editor: click/Enter opens an input, Enter or blur commits, Escape cancels, an empty value shows
  `Add <label>`, and the accessible name is the template field label. The Play view also mounts
  `pending_card.tsx`: a `role="status"` polite card listing each edited field's label and old → new
  value with Discard, Preview and Save over the same store, a muted table-sync placeholder and the
  mapped save/preview error; `pending_changes.ts` builds that summary purely from the defaults,
  the current data and the edited paths.
- `src/shell/shortcut_registry.ts` — the one action → key map (`Ctrl+S`/`Cmd+S` save, `F5`
  refresh), with the mounted screen registering and unregistering its handlers;
  `src/shell/leave_guard.ts` — the shell-level leave registry the navigation asks before it
  leaves a surface with unsaved work.

**One edit rebuilds one control.** A field control subscribes to its own dotted path
(`store.subscribePath`) and reports edits back through the engine's `controlBuilder`/
`exclusiveControlBuilder` seams; the screen subscribes only to the coarse signals it renders
(footer, marks). The per-path subscription is obtained with
`useSyncExternalStore` — no state library is added.

**Delta, never a snapshot.** A save and a preview send only the pending editable delta,
rebuilt through the engine's `setPathValue`; a save merges the returned cascade and advances
the baseline, a preview merges it **without** advancing it and marks the changed and
recalculated rows through the F4 `rowMarkBuilder`/`markLabel` seam.

**Footer status and `revealRow`.** The footer has one status line (`FooterStatus`): `footerStatus(pending, count)` in `preview_summary.ts` merges the unsaved state and the cascade ("Unsaved, 3 recalculated"), derived from the store's `previewMarks`, so it clears whenever the marks clear. Jump (a ghost button) sits on the same line; the footer's polite live region announces only the cascade and the visible phrase is `aria-hidden` while a Preview shows (the Play-only pending card owns the value delta). The derived readout draws no "Calculated" caption: `ReadOnlyValue` keeps it as a visually hidden `aria-describedby` target. `reveal_row.ts` owns
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

## Dice expressions (`src/dice/`)

Pure TypeScript parsing, resolution and rolling live in `src/dice/`; the Go package `backend/internal/dice` validates and `src/sheet/dice_*` owns the UI and pending work. The grammar, precedence, save contract and vectors are documented in [../context/dice.md](../context/dice.md); a grammar change starts in `testdata/dice/vectors.json` and moves both implementations together.

## Layout and Arrange mode

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
  preview carries a layout. `LoadedSheet` keys the edit store on the sheet's id, values and Dice block
  only, so committing a layout (which updates the cached sheet and keeps its `dice`) never rebuilds it.
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
- **Pinning lives in Arrange mode.** A Row is
  pinned by its star while arranging, staged with the Section order and saved by the one `PUT` on
  Done. `Edit pins` (section strip, pinned-filter empty state) enters Arrange mode and focuses its
  hint. The live region announces `arrange.pinned` / `arrange.unpinned`. There is no cap on pins.
  Known limit: the layout store is seeded from the layout at open, so a Template refresh that
  rebuilds Sections within a session reseeds it from that first layout.
- **Sheet header.** The sheet name is the heading, with the template name as a muted line under it
  (a kicker above a heading is banned). The actions are one hairline-divided group of ghost text
  buttons (`Help`, `Arrange`); `Collapse all` lives in the section strip's `trailing`
  slot. `src/sheet/sheet_help.tsx`: a `Help` disclosure in the
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
