# Design brief: the sheet as an object (sheet screen redesign)

Status: DRAFT, planning only. No code written. Discovery was resolved without the user (no question channel); every default is marked **ASSUMPTION** and collected in "Assumptions to confirm".

Mode: Operate (working screen). Inside an established world (DESIGN.md "Signed-in app" + "Sheet surface"): visual system is fixed, no concept tournament, no new identity, no new tokens. Code-led build.

---

## 1. Job and audience

- **Who:** RPG player at the table, on a phone (390 x 844 is the design viewport), mid-session, one hand, glancing between the sheet and the table. Building/levelling happens on the same screen but is not the priority (PRODUCT.md).
- **Situation:** the question is almost never "what are all my fields", it is "what is my HP / AC / initiative right now, and let me change it". Second most frequent: "what is my bonus for X" (a skill, a save, an attack). Rarest: tuning (Rules/Config).
- **Problem (critique 25/40):** the sheet reads as a 40-60 row settings list. Evidence from the code and shots:
  - fantasy_v1 Section order puts Combat 6th and Derived 9th of 11 (`SectionOrder` in `backend/internal/sheet/template/seeders.go`).
  - Pins only settable in Arrange mode; the Pinned-only filter resets on every visit (`view_store.ts` seeds `onlyPinned: false`, GLOSSARY says "off by default").
  - A derived value sits in another Section from its input (Strength in Stats, Strength modifier in Modifiers, 6 screens apart on a phone).
  - ~40 derived values are `--lg-color-accent` bold, the same orange as the Save fill; each carries a "Calculated" caption.
  - After Preview the sticky footer is ~150px at 390 wide (status + summary + Jump + actions).
  - The title scrolls away; the sticky strip has no sheet identity.
  - Offline copy is a red "No connection to the server." beside an orange Save, with no word about the work being safe.

## 2. Outcome and proof

- **Primary action:** open the sheet and read/edit the 4-6 numbers you check every turn without scrolling or tapping anything first.
- **Success (testable, no invented metrics):**
  - On open at 390 x 844 with at least one pin, the top of the first screen shows the sheet name, the vitals strip, and the section strip, with the first Section starting below.
  - A pinned numeric Row is editable from the strip with no mode change.
  - A player can pin or unpin a Row from the play view without entering Arrange mode.
  - Pinned-only on/off survives leaving and re-opening the sheet.
  - An editable input and its direct derived readout (Strength, its modifier) are on one line.
  - After a Preview the sticky footer is at most two rows and about 104px at 390 wide.
  - At most a handful of orange elements are visible per screen: Save, active section, pinned stars, marked (changed/recalculated) rows.
- **Product-specific truth:** the sheet is a dependency graph, so the object can show *why* a number is what it is (derived beside the input that drives it) in a way a PDF or spreadsheet cannot. No other character-sheet product can show "this input feeds these outputs" generically across game systems, because the data is `dependsOn`, not game knowledge.
- **Evidence on hand:** `schema.dependsOn` is already decoded on every leaf (`web/src/api/models/template.ts`); `layout.pinned` already exists and persists (ADR 0001); a collapse/filter view store with per-sheet storage already exists (`view_store.ts`). Preview is throttled to 3 per 60s (`sheet_edit.ts` THROTTLE_MAX/WINDOW), which is why auto-preview stays rejected.

## 3. Selected direction

**Visual authority:** DESIGN.md (one dark stage world, tokens only). Nothing here asks for a new colour, type step, radius, or motion.

**Thesis ("the sheet as an object"):** a character sheet is not a form; it is a top (what you glance at), a body (what you look up), and a back (what you tune). The page gets that order and one new bounded unit, the **vitals strip**, and everything else is the existing Section card.

**The object model (generic, no taxonomy):**

1. **Head:** sheet name + muted template line (already there), then the strip row (sticky, see below).
2. **Vitals strip (new):** a compact grid of tiles, one per promoted pinned Row. Each tile is the Row itself (label above, value below, `--lg-type-24` semibold, tabular), editable if the leaf is editable, a recessed readout if derived. Not a second copy of the Row: the Row is drawn in the strip and *not* in its Section (a Row is still drawn once, ADR 0001 / DESIGN.md hold in spirit).
3. **Body:** the existing Section cards, in the player's arranged order (or the Template default).
4. **Back:** tuning Sections (Rules, Config) last, which the Template already does.

**What "zones" means here:** zone = strip (pins), body Sections, back Sections. There is NO template-declared zone taxonomy. "vitals/combat/skills" are a genre vocabulary; encoding them would bake a game shape into the contract (violates "never bake in a game system"). Combat and Skills remain ordinary Sections whose position is the Template default or the player's.

**Strip eligibility (generic, by schema type only):**
- Eligible: a lone Row (`kind: normal`) whose leaf `type` is `number` (editable or derived).
- Not eligible (stay in their Section, rise there per ADR 0001): table Rows, exclusive-group Rows, text leaves, bool switches.
- Capacity: first 6 eligible pins in pin order (3 columns on phone, 2 rows; 6 columns wide). Pins past the cap and ineligible pins keep the ADR 0001 behaviour (top of own Section). Strip order = pin order (Rows never reorder; unpin and re-pin to move).
- Known limit (state it, do not paper over): no composite tile such as "current / max". HP current and HP max are two independent tiles. A generic `pair` would need new template metadata; out of scope.

**Pins outside Arrange mode:**
- The strip has an `Edit` control (trailing) and an empty-state; both turn on a lightweight **pin mode** in the play view: a star appears at the trailing edge of every Row, values stay visible and editable, no drag handles, no compact rows. Each toggle commits immediately through the existing `PUT /sheets/:id/layout` (full replacement, no recalculation), optimistic with revert plus the mapped error on failure, requests serialised.
- Arrange mode stays for reordering Sections (handles, compact rows, `Done`). Arrange still shows stars (unchanged).
- This reverses two documented statements (DESIGN.md "no star on a Row in the play view", ADR 0001 "committed when the player leaves Arrange mode"); see Contract section.

**Derived next to its input ("dependents"):**
- For an editable leaf L, take the derived leaves whose `dependsOn` contains L's path (direct only). If there are 1 or 2, draw them as a small trailing readout under/beside the control: label (short, muted) and value. If 0 or 3+ (for example Proficiency bonus feeds 20+), draw nothing.
- Pure, from schema, generic: Strength shows its modifier, Base armor class shows Armor class. Skill table Rows already pair bonus and total.
- Honesty about staleness: derived values only recalculate on Preview/Save (server). After an edit, the readout beside the edited input is dimmed and uses the existing "Modified" mark language until the next Preview, so it never looks authoritative while out of date. The existing Preview cascade/marks are unchanged.
- The Modifiers/Saves Sections still exist (they are the full list); they are not removed.

**Sequence the player meets:** open sheet -> strip is the first thing under the name (vitals) -> section strip sticky -> scroll body. Edit a number -> Preview -> footer states the cascade in one compact line; the changed Sections show a count dot in the section strip; Jump is a compact text control.

**Focal moment:** the first screen on a phone after pinning four numbers: name, four tiles you can type into, then the Combat card. This is the "memory test" for the redesign.

**Implementation consequence:** the strip is a new engine-agnostic screen component, not a change to the schema-driven engine's no-state rule; eligibility, dependents, and strip ordering are pure functions next to `layout.ts` (testable without DOM); the engine stays stateless and the screen supplies frames (`ARRANGE_FRAMES` pattern).

## 4. The key question: can vitals/zones be generic through the existing Layout?

Answer: **Yes for the structure, no for the first-run content.** The strip, inline pins, remembered filter and derived-near-input need no contract change. Only "a new sheet already has sensible vitals" needs one optional template field.

### Option A: web-only, pins feed the strip
- Strip = first 6 eligible pins (above). Layout `{section_order, pinned}` unchanged on the wire. Backend untouched.
- First run: empty strip with an explainer and "Choose rows" (enters pin mode); the player pins from scratch.
- Touches: `web/src/**` plus docs (DESIGN.md, web/AGENTS.md, GLOSSARY.md, ADR amendment). No Go.
- Trade-off: zero backend risk, ships alone; the new-player first impression is an empty strip (a prompt, not content).

### Option B (RECOMMENDED): A, plus an additive template field `suggested_pins`
- Template declares an ordered list of Row keys it recommends as vitals (same key grammar as layout `pinned`: field path, table-group path, `<parent>#<group>`). Pure presentation metadata, optional, like `SectionOrder`. fantasy_v1 lists its own HP/initiative/speed/AC Rows; another system lists its own. Code is generic; content is the template author's (allowed: templates are code-defined and reviewed, PRODUCT.md).
- It is only a suggestion: the empty strip offers "Pin suggested rows" (one tap, one layout PUT). It never silently writes user data and never changes the Layout wire shape. This avoids the hard problem of option B-prime below.
- Touches: web (all of A) + backend template layer: `backend/internal/sheet/template/template.go` (field), `dto.go` (JSON `suggested_pins`), `clone.go`, `registry.go` (validate at boot with the same row-key set as `ValidateLayout`, reuse `SectionRowKeys`; unknown or duplicate key fails boot), `seeders.go` (fantasy_v1 content), tests, `backend/internal/sheet/template/AGENTS.md`; web `api/models/template.ts` parse; API fixtures re-captured via `capture.sh` (never hand-edited); new ADR 0002.
- Trade-off: one small additive contract; Layout, sheet endpoints, mongo documents, export/import untouched. Skippable phase: Option A ships first, B lands later without rework.

### Option B-prime (rejected): template default pins applied implicitly when the Layout has none
- Looks free but needs to distinguish "no layout" from "layout with pinned: []" (a player who clears the defaults), and `arrangementToLayout` currently returns `null` when nothing deviates. That changes the Layout wire and repository semantics (backend sheet DTO, repository, import/export validation) and "Reset to default" meaning. Cost not worth it.

### Option C (rejected): template-declared zones/priority plus a `vitals` list in the Layout
- A zone enum (vitals/combat/skills) is genre vocabulary baked into the contract; a `vitals` Layout list duplicates Pin. Touches sheet DTO, repository, validation, export/import, web models, ADR. Largest blast radius, least generic.

**Recommendation: Option B, shipped as Phase 1 (A) then Phase 2 (suggested_pins).** Does it change the backend/ADR contract? Yes, minimally and additively (Phase 2 only: one optional Template field plus ADR 0002). The Layout contract (`{section_order, pinned}`, `PUT /layout`) does not change. Phase 1 alone is web plus docs plus an ADR amendment (the "commit on leaving Arrange" statement), no Go behaviour change.

Cheap content tweak, no contract change: reorder fantasy_v1 `SectionOrder` so Combat (and Derived) come right after Identity: `identity, combat, derived, attacks, stats, modifiers, saves, skills, spells, rules, config`. `backend/internal/sheet/template/seeders.go` + its order test. ASSUMPTION A6 (Templates are immutable "by ID" for rules; this is presentation order only, and existing sheets with a saved `section_order` are unaffected).

## 5. Scope and boundaries

- **Fidelity/breadth:** production-ready sheet screen (loaded state), phone first, then desktop. Includes the pin-mode interaction, the strip, dependents, footer, copy, and accent pass. Out of scope: Catalog, Your sheet list, auth, landing, shell nav.
- **Untouched:** Arrange mode behaviour and drag (`arrange_frames.tsx`, `layout_store.ts` reorder logic), preview/save contracts, `sheet_edit.ts` store, edit/draft persistence, the leave guard, theme tokens (`tokens.css`/`tokens.ts`; no new token), `@dnd-kit` dependency, template DAG, backend sheet endpoints.
- **Anti-goals:**
  - No game-specific code or copy anywhere (no "HP", "AC" strings in web/ or in Go outside the template content block).
  - No heuristic that guesses vitals from labels.
  - No page-level pinned zone caption, no cards-in-cards, no gradients/glass, no new colours, no second accent.
  - No auto-preview, no client-side formula evaluation.
  - No hamburger/drawer/bottom bar; the shell is unchanged.
  - No licensed content: fantasy_v1 stays a generic template.
  - No fabricated usage data or metrics in docs.

## 6. States and ranges

Realistic ranges (from fantasy_v1): 11 Sections; about 70-110 Rows; 6 ability inputs; about 37 derived leaves; 4-9 pinned Rows is a plausible player count.

| Case | Behaviour |
|---|---|
| 0 pins, template has no `suggested_pins` | Strip shows a one-line empty state and `Choose rows`. Body unchanged. |
| 0 pins, template has `suggested_pins` | Same plus primary-weight-free `Pin suggested rows` (outline button; Save stays the only primary). |
| 1-6 eligible pins | Tiles in a 3-up grid (phone) wrapping to rows; strip height grows by whole tile rows. |
| More than 6 eligible pins | First 6 in the strip; the rest rise in their Sections (ADR 0001) and are reachable via the Pinned-only filter. A muted "+N more pinned" affordance opens the filter. |
| Only ineligible pins (tables, text, switches) | Strip empty state does NOT show; strip is absent; pins behave exactly as today. |
| Stale `pinned` key (Template changed) | Ignored on read (existing `resolveArrangement`); no strip tile. |
| Pin toggle in flight / fails / offline | Optimistic star + tile; on failure revert and show the mapped error line; requests serialised; layout draft backup follows the existing best-effort rule. Offline: the star reverts with the offline copy; it does not queue silently (ASSUMPTION A9). |
| Pin mode on | Stars on every Row; Preview/Save still work; unsaved values unaffected; footer unchanged (pin mode is not Arrange). Leaving pin mode is one tap on `Done`-style control in the strip. |
| Arrange mode on | Strip is shown read-only (labels only, no values editable), matching "compact rows"; pin mode off. |
| Collapsed Section | Unchanged. A Row promoted to the strip is not in the Section's rows; a Section whose only Rows were promoted still renders (empty card is suppressed: hide the Section if it has zero drawn Rows). |
| Preview marks | A marked tile gets the same accent border and "Modified"/"Recalculated" pill on the tile's top hairline as a Row. Section links get a count dot for marked Sections; the strip shows its own marked tiles. |
| Long label | Tile label wraps to 2 lines max then ellipsises; full label is the accessible name; value never truncates. |
| Reduced motion | No movement beyond what DESIGN.md already allows; the strip appears without animation. |
| Loading/error/missing | Unchanged (existing mapped copy). |

## 7. Interaction and layout (intent, not CSS)

**Hierarchy at 390 x 844:**
1. App top bar (unchanged, ~52px).
2. Heading (name, template line, `Arrange`, `Collapse all`). Keep as is, scrolls.
3. Vitals strip (not sticky, directly under the heading, always the first content block).
4. Section strip: sticky at the top of the scroll region as today. Change: leading slot is compact. The Pinned-only control becomes a star toggle (icon plus visually hidden name; the text "Pinned only" stays at wider widths) and is followed by a compact sheet-name chip (ellipsis, tap = scroll to top) so identity does not scroll away; then the Section links. ASSUMPTION A7.
5. Section cards.
6. Sticky footer (see below).

**Strip tile anatomy:** label top (muted `--lg-type-13`), value bottom (`--lg-type-24` semibold, tabular, right-aligned like all numerics). Editable = raised bordered control, as Rows; derived = recessed readout on `--lg-color-bg`, no border, **value in `--lg-color-text`** (see accent rule). Tile height is `--lg-size-control` plus label, at least 64px; target is at least `--lg-size-control` wide and tall. Hairline-divided grid, not cards-in-cards; the strip itself is one bounded object on `--lg-color-surface` (same recipe as a Section card), so the layered model stays two levels.

**Pin mode:** a muted hint line under the strip (two lines max on phone) names what the stars do; stars are the existing `PinStar` (outline = off, filled accent = on, `aria-pressed`, name `Pin <label>`). Tapping a star changes the strip live so the player sees the effect.

**Dependents:** one trailing readout in the Row, inline with the control column on desktop and beneath it on phone (a second line inside the same Row, still one Row hairline). No arrow glyph; the muted label does the work ("Modifier"). Short label source: the derived leaf's `columnLabel` or `optionLabel` if declared, else its label with the shared words of the editable leaf's label removed is NOT done (that is guessing); use the full label, ellipsised.

**Footer (distill):** one status line (left) plus actions (right) on a single row when it fits; otherwise two rows maximum. Content when previewed: "3 recalculated" as the status, `Jump` as a text-weight control inline with it, `Preview` and `Save` as today. "Unsaved changes" and the recalculated count merge into one phrase ("Unsaved, 3 recalculated"). Target: at most 104px at 390 wide with a Preview summary. Safe-area padding kept.

**Accent budget (quieter):** the accent is for actions, selection, marks, and state. Derived values use text colour. Allowed orange per screen: Save, filled pin stars, active section underline, marked (changed/recalculated) rows, tile marks. Derived zeros are neutral.

**"Calculated" caption:** removed from each derived readout (the recessed treatment plus muted label carry it; the readout keeps a visually hidden "Calculated" in its accessible name/description so assistive tech is unchanged). A one-time explainer teaches the recessed readout (onboard step).

**Transitions:** only state: star fill, pressed, tile mark; 120-180ms, none under reduced motion. No entrance choreography.

**Accessibility:** tiles are named by label (`aria-labelledby`), the strip is a labelled region in document order before Sections, one tab stop per control, pin mode stars keep the shared focus ring and `aria-pressed`; one polite live region (the sheet's existing one) announces "Pinned X, N in the strip" / "Unpinned X". Contrast pairs reuse existing asserted ones; no new foreground colour.

**Responsive:** phone (below 640px): 3-up tiles; 640 and up: strip is up to 6-up within the 960px measure; 320 minimum without horizontal page scroll. Strip and section strip never overlap the footer or top bar (existing `scroll-margin-block` rules extended for the strip height).

**Offline copy (clarify):** replace "No connection to the server." with plain, calm, true text that says the work is safe on this device and what to do next (exact string in step 5). Use the neutral/warning role, not the danger red, because the player did nothing wrong (ASSUMPTION A8; confirm tokens: `--lg-color-warning` family exists as a primitive, the stage-world semantic must exist or the text role is `--lg-color-text-muted`).

## 8. Constraints and open decisions

**Binding constraints**
- DESIGN.md is binding and enforced by tests: literal-colour/pixel source scan, both-theme contrast, tokens only, one accent, rows not cards, AA, focus ring, `lg-<block>__<element>--<modifier>` classes.
- English only, strings via `web/src/l10n/strings.ts` (`l10n_strings.test.ts` guards copy).
- Table speed over ceremony; never bake in a game system; zero licensed content; user owns data (export/import stays valid: Layout shape unchanged).
- Row key grammar is mirrored web (`field_grouping.ts` `rowKey`) and backend (`template/layout.go` `SectionRowKeys`); if a new key kind appears both change together (this plan introduces none).
- Preview throttle 3 per 60s: nothing may call Preview implicitly.
- API fixtures are captures, never hand-edited (web/AGENTS.md); adding `suggested_pins` means re-running `capture.sh`.

**Reuse:** `PinStar`, `SectionIndex`, `useSheetView`, `view_store`, `layout_store`, `updateSheetLayout`, `revealRow`/`revealSection`, `Button`, existing marks. New: strip component, `vitals.ts` (eligibility, strip membership), `dependents.ts` (inverse `dependsOn` map), pin-mode wiring.

**Open decisions a builder must NOT invent** (see Assumptions): strip capacity (6), strip sticky vs scrolling, sheet-name chip in the strip, immediate-commit pin toggle, remembered-filter scope, accent change for derived values, `SectionOrder` change of an existing template, offline colour.

---

## Contract and docs consequences

| Item | Layer | Change |
|---|---|---|
| Strip from pins, pin mode, immediate pin commit | web only | uses existing `PUT /sheets/:id/layout` |
| Remembered Pinned-only filter | web only | per viewer, per sheet, browser storage (view store); still never sent to server or exported |
| Derived near input (dependents) | web only | from `schema.dependsOn` already in the template payload |
| Footer, copy, accent | web only | strings, CSS, DESIGN.md text |
| fantasy_v1 `SectionOrder` | backend template content | `seeders.go` + test; no schema change |
| `suggested_pins` (Phase 2) | backend template + web model + fixtures | additive optional field; Layout/sheet wire unchanged |
| ADR | docs | new `docs/adr/0002-vitals-strip-and-pins-outside-arrange.md` (status proposed): amends 0001 ("committed when the player leaves Arrange" now has one exception, the in-play pin toggle; adds Template `suggested_pins`; explains why no zones, no `vitals` Layout list) |
| Docs to edit | docs | `web/DESIGN.md` (Sheet surface: derived colour; "Arrange mode" pin star outside; "Sheet navigation": filter now remembered, strip; no more "no star in play view", "no page-level pinned zone" reworded to "the strip is the one place a pin is promoted"), `web/AGENTS.md` (Layout and Arrange section), `GLOSSARY.md` (Pin, Pinned filter: now remembered; add **Vitals strip**), `backend/internal/sheet/template/AGENTS.md` (Phase 2) |

---

## Assumptions to confirm (resolved without the user)

- **A1. Direction = Option B** (strip from pins; one additive `suggested_pins` template field in Phase 2). Alternative if backend change is unwanted: ship Phase 1 only (Option A).
- **A2. No zone taxonomy.** "Zones" are strip + Sections + the Template's own Section order, never a template-declared enum.
- **A3. Strip capacity is 6**, number leaves only (editable or derived); pins past the cap or of other kinds stay in their Section.
- **A4. A promoted Row is drawn only in the strip**, not repeated in its Section (keeps "a Row is drawn once", one `data-row-key`, no duplicate controls).
- **A5. The strip scrolls with the page** (not sticky); only the section strip is sticky. (Sticky vitals would cost about 70-140px of a 844px phone viewport.) Alternative: a one-line sticky mini-strip of the first 2-3 values once scrolled past.
- **A6. Changing fantasy_v1 `SectionOrder` is acceptable** (presentation only; sheets with a saved `section_order` keep it). Proposed order: identity, combat, derived, attacks, stats, modifiers, saves, skills, spells, rules, config.
- **A7. The sticky strip's leading slot becomes a compact star toggle plus a sheet-name chip** to keep identity visible; "Pinned only" text appears at wider widths.
- **A8. Offline/save-failed copy is neutral, not danger red.** Needs a token role confirmation.
- **A9. Pin toggles commit immediately** (one PUT each, optimistic, revert on failure, no offline queue), reversing "one commit on leaving Arrange" for pins only; reordering Sections keeps the single commit.
- **A10. Pinned-only is remembered per viewer and per sheet** in browser view storage (same store as collapse), never sent to the server, never exported. "Off on every open" is retired in DESIGN.md/AGENTS.md/GLOSSARY.
- **A11. Derived values lose accent colour** (text colour, bold) and lose the per-row "Calculated" caption; the accent stays on marks/actions/selection. This is a durable change to DESIGN.md "Editable versus derived", so it needs explicit user approval.
- **A12. Dependents rule:** show direct derived dependents only when there are 1 or 2; the readout is dimmed/marked stale until the next Preview. No auto-preview (throttle 3 per 60s) and no client-side formula evaluation.
- **A13. The alphabetical child order** (Charisma first, Strength fifth in Stats; shown in the screenshots) is a separate, out-of-scope contract problem (children walk alphabetically on both sides, `SectionRowKeys` mirrors `buildRows`). Recommend a follow-up card; not fixed here.
- **A14. No composite "current / max" tile** in this round.
- **A15. Phone-first acceptance at 390 x 844**, with 1280 desktop and 320 as regression widths.

---

## Implementation plan (separately runnable steps)

Order matters where noted. Each step ends green on `npm test` (web), `go test ./...` and `make test-race` where Go is touched, and `make fmt && make lint && make vet && make lint-specs`. Every behaviour change ships tests (edge cases first: empty/zero, stale keys, over-cap, ineligible kinds, failure paths, offline).

**Step 0: decisions and docs (precondition; no source).**
- Confirm assumptions A1-A15 with the user.
- Write `docs/adr/0002-vitals-strip-and-pins-outside-arrange.md`; edit `GLOSSARY.md`, `web/DESIGN.md`, `web/AGENTS.md` (and `backend/internal/sheet/template/AGENTS.md` in Phase 2). Mark the superseded sentences in ADR 0001 with a pointer.

**Step 1: `layout` (structure).** Three landable sub-steps.
- **1a. Vitals strip and Section order.**
  - New: `web/src/engine/vitals.ts` (pure: eligibility, strip membership and order from `Arrangement` + sections, remaining pins), `web/src/sheet/vitals_strip.tsx` + `vitals_strip.css`.
  - Edit: `web/src/engine/layout.ts` (arrangement view exposes strip vs in-Section rows), `web/src/engine/sheet_form.tsx` (omit promoted Rows from Section; hide Sections with zero drawn Rows), `web/src/sheet/sheet_screen.tsx` (compose strip between heading and Sections, read-only while arranging), `web/src/sheet/section_index.tsx` + `section_index.css` (compact leading slot, sheet-name chip, count dot on marked Sections), `web/src/sheet/sheet_view.tsx` (links), `web/src/sheet/sheet.css`, `web/src/engine/engine.css`, `web/src/l10n/strings.ts`.
  - Backend (optional, same step or separate): `backend/internal/sheet/template/seeders.go` + its order test (A6).
  - Tests: new `web/src/test/vitals.test.ts`, `sheet_vitals_strip.test.tsx`; update `engine_form.test.tsx`, `sheet_preview.test.tsx`, `sheet_screen_states.test.tsx`, `tokens_used.test.tsx`, `l10n_strings.test.ts`; `web/e2e/responsive.spec.ts` (390, 640, 1280, 320 no horizontal scroll).
- **1b. Derived beside input.**
  - New: `web/src/engine/dependents.ts` (inverse `dependsOn`, direct, 1-2 rule, excludes dependents already shown in the same Row or in the strip).
  - Edit: `web/src/engine/sheet_form.tsx`, `web/src/engine/field_control.tsx` (readout slot), `web/src/sheet/sheet_screen.tsx` (stale dimming from edit state/marks), `engine.css`, strings.
  - Tests: `engine_form.test.tsx`, new `engine_dependents.test.ts`, `sheet_preview.test.tsx` (stale -> recalculated lifecycle).
- **1c. Pins outside Arrange and remembered filter.**
  - Edit: `web/src/sheet/layout_store.ts` (immediate-commit pin path, serialised, optimistic/revert; Arrange staging untouched), `web/src/catalog/queries.ts` (`updateSheetLayout` reuse), `web/src/sheet/arrange_frames.tsx` (pin-mode frame showing stars without compact rows), `web/src/engine/pin_star.tsx`, `web/src/sheet/view_store.ts` (persist `onlyPinned` in the per-sheet record; read/serialise tests for a missing or corrupted store), `web/src/sheet/sheet_view.tsx`, `section_index.tsx`, `web/src/library/*` only if a draft backup is needed for failed pins.
  - Tests: `layout_store` tests (serialisation, revert, offline), new `view_store` persistence tests, `sheet_open.test.tsx`/`sheet_draft.test.tsx` regression.

**Step 2: Phase 2 backend `suggested_pins` (Option B).** After Step 1a.
- Edit: `backend/internal/sheet/template/template.go` (field), `dto.go` (`suggested_pins`), `clone.go`, `registry.go` (boot validation using `SectionRowKeys`), `seeders.go` (fantasy_v1 list), `AGENTS.md`; tests `template_test.go`, `registry_test.go`, `layout_test.go`, `dto_test.go`, `seeders_test.go`, `backend/internal/sheet/dto_test.go` (assert unknown/duplicate key fails boot with a typed error, clone isolation, JSON omitted when empty); web `web/src/api/models/template.ts` + parse tests; re-capture API fixtures via `capture.sh`.

**Step 3: `onboard` (first pin, explainers).**
- Edit: `web/src/sheet/vitals_strip.tsx` (empty state, `Pin suggested rows` using Phase 2 data, `Choose rows`), `web/src/sheet/sheet_screen.tsx` (pin-mode hint), `web/src/sheet/view_store.ts` (one-time "seen" flags per viewer, browser only), `web/src/engine/engine.css` / `sheet.css`, `web/src/l10n/strings.ts` (empty state, pin-mode hint, legend for the recessed "calculated" readout, "+N more pinned"). Without Phase 2, "Pin suggested rows" is hidden and only `Choose rows` shows.
- Tests: empty-state matrix (0 pins, suggested present/absent, ineligible-only), the one-time flag survives/clears, copy scan.

**Step 4: `distill` (post-Preview footer, repeated "Calculated").**
- Edit: `web/src/sheet/sheet_screen.tsx` (footer: merge dirty + summary into one status line, inline Jump, max two rows), `web/src/sheet/sheet.css`, `web/src/sheet/preview_summary.ts`, `web/src/engine/field_control.tsx` (drop the visible "Calculated" caption, keep it in the accessible description), `web/src/engine/engine.css`, `web/src/l10n/strings.ts`.
- Tests: `sheet_preview.test.tsx` (summary states, live region reads once, Jump focuses first marked), `engine_field_control.test.tsx` (accessible name still says calculated), `engine_xss.test.tsx`, e2e height assertion at 390 x 844 (at most about 104px).

**Step 5: `clarify` (offline copy).**
- Edit: `web/src/l10n/strings.ts` (sheet save/preview connection-failure keys; text conveys "your changes are kept on this device; try again when you are connected"), `web/src/sheet/sheet_screen.tsx` (status role/colour per A8), `web/src/sheet/sheet.css`, mapped-error copy only if it lives in `web/src/api/error_mapper.ts` copy keys.
- Tests: `l10n_strings.test.ts`, `sheet_screen_states.test.tsx` (offline/saving/saved/failed), a11y live-region check. Verify the claim "kept on this device" against the draft backup (`library/draft_backup.ts`) before shipping the string; if a path is not covered, soften the copy.

**Step 6: `quieter` (accent).**
- Edit: `web/src/engine/engine.css` (derived value colour to `--lg-color-text`; accent only for marks, pins, selection), `web/src/sheet/sheet.css`, `web/src/sheet/section_index.css`, `web/DESIGN.md` ("Editable versus derived", A11). No `tokens.css`/`tokens.ts` change.
- Tests: `tokens_used.test.tsx` (no literals), contrast assertions unchanged, update any test that pins the derived colour. Check: count of accent-coloured elements on the previewed phone screen is within the accent budget.

**Step 7: verify and finish.** `npm test`, e2e `responsive.spec.ts`, `make fmt lint vet lint-specs test-race` if Go touched; screenshots at 390 x 844, 640, 1280 across: fresh sheet (empty strip), 4 pins, over-cap pins, pin mode, previewed, offline, arranging; run the Impeccable finish review against this brief; update DESIGN.md/documenter pass.

Dependencies: 0 -> 1a -> (1b, 1c) -> 3; 2 independent of 1b/1c, required by the suggestion button in 3; 4, 5, 6 independent of each other and of 2, can run after 1a. 6 is cheapest and the least risky first win if the user wants a quick relief before the structural work.
