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
  same-origin, so the strict production Content-Security-Policy
  (`deploy/traefik/dynamic.yml`) holds without exceptions.
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

The schema-driven rendering core: it turns a decoded `FieldSchema` tree into labeled sections and
controls and owns **no state** — no value, no request, no mark. It mounts no screen.

- `field_grouping.ts` derives ordered Sections and Rows (stacked leaf, declared-column table,
  exclusive group); `field_control.tsx`/`exclusive_control.tsx` render one leaf and one
  single-choice group; `sheet_form.tsx` composes them; `sheet_defaults.ts` seeds and merges values;
  `sheet_mark.ts` is the mark contract; `layout.ts`/`arrange_ops.ts` are pure arrangement helpers.
- **Stable units for the Layout.** Every Section renders one `<section data-section-key>` and every
  Row a wrapper with `data-row-key` and `data-row-kind`. The key comes from `rowKey(row)` and must
  stay identical to `template.SectionRowKeys`; it depends only on the template. A table Row draws
  its own header, shared with the next Row only when `needsTableHeader` says so.
- **One control source.** `SheetForm` takes `data` + `onChanged`, or both `controlBuilder` and
  `exclusiveControlBuilder`; any other combination throws. Builder mode reads no value, so the
  caller owns the per-field subscription and edit reporting through the seams.
- **One field, one subscription.** Controls subscribe to their dotted path with
  `useSyncExternalStore`; the screen subscribes only to the coarse footer and mark signals. Do not
  add a state library.
- **Numeric guard.** A numeric control reports only when the whole text resolves to a finite
  number; an empty field clears, a pending sign/point stays display text, and a rejected text
  changes nothing. Do not report NaN, Infinity or `-0`.
- **Seams, not screens.** `SheetForm` takes an optional resolved `arrangement` and replaceable
  `frames` (`Root`, `Section`, `Row`); the engine imports no drag library. A lone Row's label id
  (from `useId`) goes through `controlBuilder`; a table cell gets none.
- `dependents.ts` is a pure inversion of `dependsOn` (one or two dependents, no formula
  evaluation); it never triggers a preview.
- Every string resolves through `src/l10n/strings.ts`; every visual value is a `--lg-*` token; data
  is rendered as text (no DOM sink).

## Sheet cycle (`src/sheet/`, `src/library/`)

The open/edit/preview/draft feature over one sheet. It reuses the typed client and the engine and
owns no wire model.

- `sheet_open.ts` resolves `/sheets/:id`: one sheet GET, one template GET; a seeded created sheet is
  not fetched again; `NOT_FOUND` evicts the sheet and both drafts.
- `sheet_edit.ts` is the in-memory edit store: `editedPaths`, copy-on-write `data`, pending delta,
  save/preview lifecycle, preview marks, two rolling throttle windows and draft restore. It emits
  immutable snapshots and binds no listener.
- **Delta, never a snapshot.** Only changed editable paths are sent; a save merges the cascade and
  advances the baseline, a preview merges without advancing and marks the changed/recalculated
  rows. A failed save keeps the edits; a 409 is retried by `save_retry.ts`, not by the store.
- `sheet_screen.tsx` mounts the form in builder mode with the footer and one guard modal.
- `src/library/sheet_store.ts` is the port; `idb_sheet_store.ts` is the only `idb` importer;
  `draft_backup.ts` owns the 60-second cadence. Every local write is best-effort, never holds a
  credential, and the draft holds only the pending editable delta and Dice block.
- `Ctrl+S` saves and `F5` refetches the template through the shortcut registry; the leave guard
  asks before discarding. `F5` and the leave guard use the screen's single confirmation modal —
  never stack a second.

## Dice expressions (`src/dice/`, ADR 0005)

Pure TypeScript half of the Dice grammar: no React, no network, no dependency on the API models.
The Go package `backend/internal/dice` validates; this module parses, resolves and rolls.

- `parseDice` / `canonicalForm` (`parse.ts`), `validateRow` / `validateSection` / `fitsRow`
  (`fit.ts`, against a `DiceSchema` path → kind), `resolveDice` (`resolve.ts`, the five-step
  precedence), `rollExpression` / `performRoll` (`roll.ts`). Operations that can be refused return a
  `DiceResult` carrying a `DiceError` whose `class` is the vocabulary of the shared vectors; they do
  not throw (the one exception is a random source that never yields a usable word).
- **`testdata/dice/vectors.json` is shared with Go.** `dice_*.test.ts` assert every vector; a grammar
  change starts in the vectors and both implementations follow. `LIMITS` and the class strings are
  part of that contract.
- **Resolution** reads own keys only (`Object.hasOwn`): an absent key falls through, `null` is "no
  dice", and the chosen expression is judged against the Row (parse, then fit). A Row it does not fit
  is not rollable with it; no later step is tried.
- **A Roll** reads the on-screen values, `draft` before saved (a path present in the draft wins even
  when emptied), floors with `Math.floor`, and refuses a missing, `null`, non-finite or non-numeric
  Field as `no_value`. Dice come from `crypto.getRandomValues` through `dieFromWords`, which redraws
  words above the largest multiple of the face count (no modulo bias). Nothing is stored.
- User-facing copy for a `DiceError` is the UI's job (LGN-166), by `class`, through `src/l10n/`.
- **Validation mirrors the server.** The client refuses what the `dice` block of `PATCH /sheets/:id` would refuse
  (same limits, grammar, Row fit and 64-entry cap) so a bad expression never reaches the wire; the
  server stays the authority and its 400 `INVALID_REQUEST` is still mapped.
- **A dice choice is pending work (ADR 0006).** The edit store (`sheet_edit.ts`) holds the saved and
  the current block; `createDiceStore` (`src/sheet/dice_store.ts`) is the face the Dice controls use
  over it: it runs the local check and, when accepted, records the choice with `setDice`. Nothing is
  sent until Save (or `Ctrl+S`), which issues one `PATCH` carrying the delta and the whole block
  (`patchSheet(id, delta, dice)`; an empty delta is left out). A choice counts for `isPending`,
  `pendingCount`, the leave guard, `F5`, discard and the draft (`writeDraft(id, delta, dice)`);
  setting it back to its saved value clears it. Both the saved block and a restored draft go through
  `pruneStaleChoices` (`src/sheet/dice_view.ts`), which mirrors the server's save rules (unknown key,
  Row expression that no longer fits, Section expression that fits none of its Rows), so a stale entry
  never reaches the `PATCH`; the Section check while typing uses the same rule. It never
  enables Preview (`canPreview` reads values only) and never resets the preview marks. A failed save
  keeps it pending and shows the footer error. A pending Row is marked "Modified" through
  `rowMarkBuilder(paths, rowKey)` (the preview mark wins when both apply); a pending Section choice
  draws the same mark in the Section's title row.
- **Where it shows.** A dice button (`src/sheet/dice_row.tsx`) appears only on a Row with an
  applicable expression, or one switched off by "no dice" (on the Row or its Section). Tap rolls;
  long-press (`LONG_PRESS_MS`, 500 ms) opens the Row's editor; the Edit button is the keyboard
  alternative. Roll and Edit sit at the trailing edge of the Row's label in every Row kind (lone,
  exclusive, table), right before the value. Editing a Row creates a Row override, which wins over the
  Section. The Roll button names the dice it throws (`diceSummary`), Edit is a pencil icon (a struck die on a Row whose dice are off), and the Section's die icon button sits at the trailing edge of its title row. Everything is hidden
  in Arrange mode. The editor is the shared `Dialog` (one at a time, Escape closes it and focus
  returns to the button that opened it); the Section editor carries the `dice.sectionNote` note. The
  Roll result (`src/sheet/dice_panel.tsx`) is a non-modal polite status that floats at the bottom of
  the viewport, above the element marked `data-floating-floor` (the sheet footer). The provider
  portals both to `document.body`, so neither reflows the sheet nor inherits a Row's container
  query containment. Rendering helpers are in `src/sheet/dice_view.ts`; wire models in
  `src/api/models/dice.ts`. Copy lives in `src/l10n/strings.ts`.

## Layout and Arrange mode

- **Explicit mode, one commit.** Nothing is sent while arranging. `Done` issues one
  `PUT /sheets/:id/layout` (none when unchanged); `Cancel` discards; `Reset to default` stages the
  template default and `Done` commits it. A failed `PUT` keeps the staging.
- **A layout is not a value.** `layout_store.ts` never touches `sheet_edit.ts`; arranging never
  marks unsaved values and `Ctrl+S` saves nothing. A commit updates the cached sheet only.
- **Rows pin, never move.** A pinned Row rises to the top of its own Section in pin order; only
  Sections are sortable; there is no pin cap.
- **View state is local** (`view_store.ts`: collapsed Sections, pinned filter, explainer seen),
  never sent and never unsaved work.
- **@dnd-kit** is the only drag dependency, mounted only while arranging. Keep it on React props
  and CSSOM so the production CSP needs no relaxation.

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

## Standing decisions

The per-feature deviation log lives in `docs/web-delivered-deviations.md`. These still bind:

- Tests are flat in `src/test/` with a `*.test.ts(x)` suffix (feature prefixes such as
  `engine_*`), because the Vitest setup and shared helpers serve every suite.
- No state library: per-path subscriptions use `useSyncExternalStore` (`AD-001`).
- The leave guard is a shell-level registry, not a router blocker; a browser-back gesture is
  not intercepted, only in-app navigation.
- Modal surfaces are accessible overlays (`role="dialog"`, `aria-modal`), not native
  `<dialog>`, because jsdom does not implement `showModal`.
- The access token is memory-only; `AuthSession.refresh()` is single-flight.
- Captures in `src/api/fixtures/` are real responses; a credential in one is redacted
  (`redacted-access-token`).
