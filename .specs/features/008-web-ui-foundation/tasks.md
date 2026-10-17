# Tasks: Web UI Foundation

**Input**: Design documents from `.specs/features/008-web-ui-foundation/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md), [data-model.md](./data-model.md), [contracts/](./contracts/)

**Tests**: required — constitution III makes behavior ships with tests non-negotiable. Each story
opens with its suites (edge cases first) and they MUST fail before that story's implementation.

**Organization**: tasks are grouped by user story so each story is independently implementable and
testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: can run in parallel (different files, no incomplete dependency)
- **[Story]**: the user story the task belongs to
- Exact file paths are in every description

## Path Conventions

- Client sources: `web/src/`, client tests: `web/src/test/` (flat, the F1 layout)
- Static assets served same-origin: `web/public/`
- Browser specs: `web/e2e/`
- Feature docs: `web/DESIGN.md`, `web/AGENTS.md`; decisions: `.specs/STATE.md`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: pin down the starting point so a later red is attributable to this feature.

- [x] T001 Run the baseline on the feature branch — `cd web && npm test && npm run lint && npm run typecheck` — and record the green result. No file changes; this is the reference the story checkpoints compare against.

**Checkpoint**: the client suites, lint and typecheck are green before any change.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the token harness and the vendored asset every story reads. No user story starts until
this phase completes, and each task here is green on its own.

- [x] T002 [P] Rework `web/src/test/tokens_files.ts` into a per-theme parser: split the token file into its light (`:root`) and dark (`[data-theme='dark']`) blocks and expose `readThemeTokenValues(theme)` plus `readThemeTokenNames(theme)`, keeping `readTokenValues()`/`readTokenNames()` over the union so existing consumers stay green (UIF-02, UIF-28, `R2`).
- [x] T003 [P] Vendor the font: extract `Geist-Variable.woff2` and the OFL-1.1 `LICENSE.txt` from the official `geist@1.8.1` release into `web/public/fonts/geist/Geist-Variable.woff2` and `web/public/fonts/geist/Geist-LICENSE.txt`, and record the provenance in a short header comment in `web/public/fonts/geist/Geist-LICENSE.txt`'s sibling doc (`web/README.md`, T032) (UIF-14, UIF-15, `R4`).
- [x] T004 [P] Rework `web/src/test/tokens_used.test.tsx`: keep the literal scan and the token-resolution check over `src/shell/`, `src/catalog/`, `src/engine/`, `src/sheet/`, `src/library/` and `src/auth/`, and state in the file's doc comment that `src/theme/` is the deliberate exception where literals are declared (UIF-29, UIF-30).

**Checkpoint**: the token file has a per-theme parser, the font artifact and its license are
committed, and the literal scan's exception is documented.

---

## Phase 3: User Story 1 - One foundation that reads the same in light and dark (Priority: P1) 🎯 MVP

**Goal**: a three-layer token file with a parchment light theme and a charcoal-plus-amber dark
theme, both asserted at AA, plus the self-hosted variable font applied through the base styles.

**Independent Test**: run `tokens_contrast.test.ts` and `self_hosted_assets.test.ts` — both themes
resolve every token and meet AA, and the font is declared, preloaded and same-origin.

### Tests for User Story 1

- [x] T005 [P] [US1] `web/src/test/tokens_contrast.test.ts` — edge first: a token declared in one theme only is reported missing in the other; an unknown pair variable is reported; happy path: every declared pair meets its minimum in its own theme (4.5:1 for text, 3:1 for the focus ring) and every `designTokens` entry resolves (UIF-02, UIF-28, UIF-31, SC-001). Fails until T007/T008.
- [x] T006 [P] [US1] `web/src/test/self_hosted_assets.test.ts` — edge first: a `@font-face` with a third-party `url()` fails, a missing license fails; happy path: `Geist-Variable.woff2` and `Geist-LICENSE.txt` exist, `fonts.css` declares one same-origin `@font-face` with `font-weight: 100 900` and `font-display: swap`, `index.html` carries the preload with `crossorigin`, `--lg-font-family` leads with the vendored family, and no third-party font/style/script origin appears in `src/theme/**` or `index.html` (UIF-14, UIF-15, UIF-16, SC-005). Fails until T009/T010/T011.

### Implementation for User Story 1

- [x] T007 [US1] Extend `web/src/theme/tokens.ts`: `ContrastPair` gains `theme: 'light' | 'dark'`; declare the pairs for both themes (text/bg, text/surface, text-muted/bg, text-muted/surface, text-subtle/bg, accent-text/accent, accent-text/accent-hover, danger-text/danger, focus/bg, focus/surface); grow `designTokens` to the new type (`12/13/15/16/18/20/24/30`), weight (400/500/600/700), space (`2/4/8/12/16/24/32/48`), radius (`sm/md/lg/pill`), shadow (`1..3`) and motion groups (UIF-04, UIF-07, UIF-09, UIF-10, UIF-11, UIF-12, UIF-28, UIF-31, `R9`).
- [x] T008 [US1] Rewrite `web/src/theme/tokens.css` in three layers: primitive scales (ink ramp, amber ramp, danger ramp, space, radius, font, motion/easing), then the semantic aliases; declare the light palette under `:root` with `color-scheme: light` and the dark palette under `[data-theme='dark']` with `color-scheme: dark`; every semantic resolves in both themes; one accent per theme; three elevations; the reduced-motion-independent motion tokens (UIF-01…UIF-13, SC-001, SC-002, `R1`, `R7`, `R8`, `R10`).
- [x] T009 [P] [US1] `web/src/theme/fonts.css` — one `@font-face` (`font-family: 'Geist'`, `src: url('/fonts/geist/Geist-Variable.woff2') format('woff2')`, `font-weight: 100 900`, `font-style: normal`, `font-display: swap`) (UIF-14, UIF-16, `R4`).
- [x] T010 [US1] `web/src/theme/base.css` — import `fonts.css`, set `color-scheme` on the root, drive the body background/foreground from the semantic tokens, set `--lg-font-family` to lead with the vendored family, keep the tokenised `:focus-visible` ring, and add the `prefers-reduced-motion: reduce` block that zeroes non-essential animation (UIF-03, UIF-12, UIF-13, UIF-16).
- [x] T011 [P] [US1] `web/index.html` — replace the fixed `theme-color` with one entry per `prefers-color-scheme` and add `<link rel="preload" as="font" type="font/woff2" crossorigin href="/fonts/geist/Geist-Variable.woff2" />` (UIF-03, UIF-16, `R11`).

**Checkpoint**: both themes resolve and pass AA, the font is declared, preloaded and self-hosted, and
the component directories still contain no literal value — the token foundation is the MVP.

---

## Phase 4: User Story 2 - The chosen theme stays chosen, and never flashes (Priority: P1)

**Goal**: the theme follows the system preference until the person chooses, the choice persists, the
first paint already uses it, and a broken storage degrades silently.

**Independent Test**: run `theme_toggle.test.tsx` and the browser `theme.spec.ts` — the resolution
order, the persistence, the control's accessible state and the first painted theme are all proven.

### Tests for User Story 2

- [x] T012 [P] [US2] `web/src/test/theme_toggle.test.tsx` — edge first: no stored value follows the simulated `prefers-color-scheme` and a later change is followed live; `purple` falls back to the system preference; a storage that throws on read and on write surfaces no error; a system change is ignored once a value is stored; happy path: activating the control flips the theme, writes `lg.theme`, and exposes the accessible name and pressed state (UIF-17…UIF-23, UIF-27, SC-004). Fails until T015/T016/T017/T018.
- [x] T013 [P] [US2] `web/e2e/theme.spec.ts` — seed `lg.theme = 'dark'` and assert the first painted theme is the stored one (no pale frame precedes it), that the head script is served from the app's own origin, and that the document contains no inline script (UIF-20, SC-004). Fails until T015.

### Implementation for User Story 2

- [x] T014 [US2] Add the theme control's copy keys to `web/src/l10n/strings.ts` — the accessible label and the per-target action copy — so no literal reaches the component (UIF-18, `R13`).
- [x] T015 [US2] `web/public/theme-init.js` plus its `<script src="/theme-init.js"></script>` in `web/index.html`'s head: read `lg.theme`, accept only `light`/`dark`, fall back to `prefers-color-scheme`, and set `document.documentElement.dataset.theme` before the first paint; same-origin classic script, no inline code and no policy hash (UIF-20, `R3`).
- [x] T016 [US2] `web/src/theme/theme.ts` — the theme store with an injected storage and media-query reader: the documented resolution order, the live system follow while nothing is stored, `set`/`toggle` writing the two accepted values, and every storage failure swallowed (UIF-17, UIF-19, UIF-21, UIF-22, UIF-23, `R6`).
- [x] T017 [US2] `web/src/theme/use_theme.ts` — a `useSyncExternalStore` hook over T016 so the control and any surface read the resolved theme without a state library (UIF-18).
- [x] T018 [US2] `web/src/shell/theme_toggle.tsx` — the two-state light/dark control, copy resolved through `message()`, accessible name and pressed state exposed, styles in `web/src/shell/shell.css` using `--lg-*` tokens only (UIF-18, UIF-27, UIF-29).

**Checkpoint**: the theme can be chosen, persists across reloads, follows the system until then, and
the first paint is never wrong.

---

## Phase 5: User Story 3 - A shell that shows only what has content (Priority: P1)

**Goal**: exactly two shell regions — navigation then content — with the contextual panel, its
styles, its tokens and its collapse breakpoint removed, and the nav footer hosting the theme control.

**Independent Test**: run the shell suites and the browser shell spec — two regions in order on
every route, no third region, navigation persistent at a narrow width, and no reference to the
removed panel anywhere.

### Tests for User Story 3

- [x] T019 [P] [US3] Retarget `web/src/test/shell.test.tsx`: assert the region order of `nav` (named) then `main` on the catalog route, and assert `queryByRole('complementary')` is absent; add the source-scan assertion that no file under `web/src/` references `right_panel`, `.lg-panel`, `--lg-size-panel-` or `SHELL_NARROW_QUERY` (UIF-24, UIF-26, UIF-32, SC-003). Fails until T023/T024/T025/T027.
- [x] T020 [P] [US3] Retarget `web/src/test/shell_responsive.test.tsx`: at a wide and at a narrow viewport both regions are present, the navigation stays visible, and the content keeps its minimum rather than collapsing; no third region appears at either width (UIF-25, UIF-32). Fails until T023/T025.
- [x] T021 [P] [US3] Extend `web/src/test/shell_keyboard.test.tsx`: the theme control is reachable by keyboard in the navigation's tab order and the first control is still the catalog link (UIF-18, UIF-27). Fails until T026.
- [x] T022 [P] [US3] Retarget `web/e2e/shell.spec.ts`: drop the complementary-region assertions, keep the region order, the deep-link and the designed not-found checks, and assert both regions at a narrow viewport (UIF-24, UIF-25, UIF-32). Fails until T023/T025.

### Implementation for User Story 3

- [x] T023 [US3] Rework `web/src/shell/app_shell.tsx`: render `LeftNav` and `main` only — no `RightPanel`, no media query, no narrow variant (UIF-24, UIF-25, `R5`).
- [x] T024 [P] [US3] Delete `web/src/shell/right_panel.tsx` (UIF-26, `R5`).
- [x] T025 [US3] Rework `web/src/shell/shell.css`: a two-column grid, the navigation footer block, the theme control's styles, and the removal of every `.lg-panel*` rule (UIF-25, UIF-26, UIF-27).
- [x] T026 [US3] `web/src/shell/left_nav.tsx` — the footer block hosts `ThemeToggle` beside the session control (UIF-27).
- [x] T027 [US3] Remove the panel's tokens: drop `--lg-size-panel-min`/`--lg-size-panel-max` from `web/src/theme/tokens.css` and `web/src/theme/tokens.ts`, and drop the `SHELL_NARROW_QUERY` export (UIF-26).

**Checkpoint**: every route renders two regions, the panel is gone from the markup and the sources,
the navigation persists as the viewport narrows, and the footer carries the theme control.

---

## Phase 6: User Story 4 - The guidelines are the one place a visual rule is written (Priority: P2)

**Goal**: `web/DESIGN.md` is the visual source of truth with its mandatory sections, `web/AGENTS.md`
keeps only development, security and architecture rules and defers to it, and the decisions are
recorded.

**Independent Test**: run `design_guidelines.test.ts` and read the two documents — the sections
exist, the deferral exists and no visual value is restated in the developer rules document.

### Tests for User Story 4

- [x] T028 [P] [US4] `web/src/test/design_guidelines.test.ts` — edge first: a design document missing a mandatory section is reported, a developer-rules document that restates a `--lg-*` value table is reported; happy path: `web/DESIGN.md` carries principles, token architecture, colour, typography, spacing/layout, radius/border/elevation, motion, component specifications, accessibility, security posture, per-screen patterns, enforcement and anti-patterns, and `web/AGENTS.md` points to it (UIF-33, SC-006). Fails until T029/T030.

### Implementation for User Story 4

- [x] T029 [US4] Write `web/DESIGN.md` in English with the plan's mandatory sections: principles, three-layer token architecture and the token-addition workflow, colour (both themes, the accent rule, system states), typography, spacing/layout/density, radius/border/elevation, motion, component specifications (button, fields, data, nav, dialog/overlay, feedback), accessibility, security posture, per-screen patterns, enforcement and the anti-pattern list (UIF-33, `R11`).
- [x] T030 [US4] Rewrite `web/AGENTS.md`: replace the visual sections with a pointer to `DESIGN.md` (which supersedes them), keep the development, security, architecture, engine, sheet-cycle, componentization and dependency rules, and drop the invalidated values (the 17 px cap, the 4 px radius cap, elevation 0, the panel and `SHELL_NARROW_QUERY`) (UIF-33).
- [x] T031 [P] [US4] Record the feature's decisions as `AD-018` in `.specs/STATE.md`: the three-layer tokens with two themes, the theme preference in `localStorage`, the same-origin no-flash head script (with the inline-plus-hash alternative rejected), the removal of the panel, and the vendored font with its provenance (UIF-34, `R3`, `R4`, `R5`, `R6`).

**Checkpoint**: one document owns the visual rules, the developer rules defer to it, and the
decisions are traceable.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: the recorded deviations, the browser-level measurement and the final validation.

- [x] T032 [P] Update `web/README.md` (the vendored font and its provenance, the theme preference key, the two-region shell) and add this feature's delivered deviations to the `web/AGENTS.md` log (the artifact filename, the toggle component's home under `shell/`, the retargeted suites).
- [x] T033 Run the [quickstart.md](./quickstart.md) scenarios end to end (`cd web && npm test`, the individual suites, `npm run lint`, `npm run build`, the Playwright specs) and `make lint-specs`; confirm no dependency was added and the security gates are unchanged.
- [x] T034 Final gates: `make fmt && make lint && make vet && make lint-specs` from the repo root and `make lint-web && make test-web && make sec-scan` from the client, with the full suite green.

**Total tasks**: 34.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001)**: no dependencies.
- **Foundational (T002–T004)**: depends on nothing but the baseline — **blocks all user stories**.
- **US1 (T005–T011)**: depends on Foundational (`T002`'s parser, `T003`'s artifact).
- **US2 (T012–T018)**: depends on Foundational; its styles read US1's tokens (T008), so T008 precedes T018.
- **US3 (T019–T027)**: depends on Foundational; T026 mounts US2's `ThemeToggle` (T018), and T027 removes tokens US1 declared (T008).
- **US4 (T028–T031)**: depends on Foundational; T029/T030 document the rules US1–US3 implement, so it runs last among the stories.
- **Polish (T032–T034)**: depends on all desired stories.

### User Story Dependencies

- **US1 (P1)**: no dependency on other stories — it is the MVP and the token contract every later story reads.
- **US2 (P1)**: independently testable with the injected storage and media-query reader; its control is styled by US1's tokens but proven by `theme_toggle.test.tsx` without the shell.
- **US3 (P1)**: independently testable through the shell suites; it integrates US2's control into the navigation footer (`T026` after `T018`).
- **US4 (P2)**: independently testable through its document suite; it depends on the earlier stories only for the content it documents.

### Within Each User Story

- Tests are written first and must fail before the story's implementation.
- The shared pieces come before their consumers (`tokens.ts` before `tokens.css`, `theme.ts` before `use_theme.ts` before `theme_toggle.tsx`, `app_shell.tsx` before the module deletion).
- A story is complete before the next priority starts.

### Parallel Opportunities

- Foundational: `T002`, `T003`, `T004` in parallel (different files).
- US1 tests `T005`/`T006` in parallel; then `T007`→`T008` (the mirror before the file it asserts), `T009`/`T011` in parallel, `T010` after `T009`.
- US2 tests `T012`/`T013` in parallel; then `T014`/`T015` in parallel, `T016`→`T017`→`T018`.
- US3 tests `T019`–`T022` in parallel; then `T023`/`T024` in parallel, `T025`→`T026`, `T027` after `T023`.
- US4: `T028` first, then `T029`/`T030`/`T031` in parallel.
- Polish: `T032` in parallel with anything; `T033` after the stories; `T034` last.

---

## Parallel Example: User Story 1

```bash
# Launch both US1 suites together (they fail until T007–T011):
Task: "tokens_contrast.test.ts — both themes resolve and meet AA (UIF-02, UIF-28, UIF-31)"
Task: "self_hosted_assets.test.ts — font declared, preloaded, self-hosted (UIF-14…UIF-16)"

# Then the independent implementation files together:
Task: "fonts.css — the single self-hosted @font-face (UIF-14, UIF-16)"
Task: "index.html — theme-color per scheme + the font preload (UIF-03, UIF-16)"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Setup (T001) and Foundational (T002–T004).
2. Complete US1 (T005–T011): the token file, the mirror, both themes, the contrast suite and the font.
3. **STOP and VALIDATE** independently (quickstart scenarios 2, 3, 7).
4. Demo: the app renders in the parchment theme and flips to charcoal plus amber with the stored or
   system preference.

### Incremental Delivery

1. Setup + Foundational → the harness, the artifact and the documented scan exception.
2. US1 → the token foundation and the font (MVP).
3. US2 → the persisted, flash-free theme.
4. US3 → the two-region shell with the theme control in the navigation footer.
5. US4 → the guidelines and the decision record.
6. Polish → the deviations, the browser measurement and the gate run.

---

## Requirement Coverage

| Story | Requirements | Success criteria |
|---|---|---|
| Foundational | UIF-29, UIF-30 | SC-002 |
| US1 | UIF-01…UIF-16, UIF-28, UIF-31 | SC-001, SC-002, SC-005 |
| US2 | UIF-17…UIF-23, UIF-27 | SC-004 |
| US3 | UIF-24…UIF-27, UIF-32 | SC-003 |
| US4 | UIF-33, UIF-34 | SC-006 |
| Cross-cutting | UIF-28…UIF-32 | SC-001…SC-003 |

Every `UIF-*` requirement and every `SC-*` outcome maps to at least one task above; UIF-27 is
implemented by US2's control (`T018`) and mounted by US3's footer (`T026`), which is why it appears
in both stories.

---

## Notes

- `[P]` tasks target different files with no incomplete dependency.
- `[Story]` labels trace a task to its user story.
- Commit after each task or logical group; every behavior change ships its tests in the same commit.
- No dependency is added, so the SCA/SAST/secret gates see nothing new to vet.
- Stop at any checkpoint to validate a story independently.
