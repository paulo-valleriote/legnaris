# front-end-phase-2-catalog Validation

**Date**: 2026-09-30
**Spec**: `.specs/features/front-end-phase-2-catalog/spec.md`
**Diff range**: `main..HEAD` (spec/design/tasks + 6 feat commits `ece20ed`…`eaa575f` + fix `4929b46` + post-UAT fix `1c886c3`)
**Verifier**: independent sub-agent (re-verification 1/3, author ≠ verifier); post-UAT fix cycle by the orchestrator

**Verdict**: PASS ✅ — the round-1 gap is closed. The previously surviving mutant (`>=` → `>` at the panel-collapse threshold) is now killed by the exact-boundary test added in fix `4929b46`; all 3 sensor mutations die, the Build gate passes 154/154, and all 19 ACs carry evidence that meets the spec-defined outcome (2 spec-precision flags carried over — non-failing). Interactive UAT then ran (human reviewer): functional behavior confirmed correct, **visual rendering rejected (Major)** → fix cycle T8 applied in `1c886c3` and re-verified (analyze 0, 162/162, after-goldens reviewed), then **accepted on re-review** ("dentro do que eu queria para as primeiras fases") — see Interactive UAT Results and Fix 2 below.

---

## Task Completion

| Task | Status     | Notes   |
| ---- | ---------- | ------- |
| T1   | ✅ Done    | 19 new tests (tokens 10 + theme 9), matches Done-when count |
| T2   | ✅ Done    | 17 new tests; `.g.dart` committed, codegen regenerated with no diff |
| T3   | ✅ Done    | 11 new tests; Build gate run per task |
| T4   | ✅ Done    | 10 new tests; four render states distinguished |
| T5   | ✅ Done    | 6 new tests (5 declarations, CAT-15 table-driven over 2 cases); table-driven CAT-15 cases |
| T6   | ✅ Done    | 10 new tests (boot 6 + registry 4). Note: Done-when says `widget_test.dart` "reescrito"; actual = file deleted and its two behaviors superseded in `desktop/test/app_boot_test.dart` (see Test Integrity) — intent ("nenhum teste órfão") met |
| T7   | ✅ Done    | Fix task (`4929b46`): `shellMinWidthWithPanel` = 1002 = 220 + 480 + 300 + 2 × hairline (`desktop/lib/theme/tokens.dart:83-84`); formula + literal 1002 asserted in `desktop/test/theme/tokens_test.dart:60-68`; viewports tokenized (`app_shell_test.dart:73,97`); new exact-threshold boundary test (`app_shell_test.dart:81-90`); `design.md`/`tasks.md` threshold annotations updated (no stale 1000 outside this report); gate run, analyze 0 issues; suite 154 passed / 4 skipped — matches Done-when |
| T8   | ✅ Done    | Post-UAT fix task (`1c886c3`): `Scaffold` root in `AppShell` (`app_shell.dart:26`) + documented why; `ColorScheme.fromSeed(0xFF546E7A)` passed at `ThemeData` construction (`app_theme.dart:11-17`); `Align(topLeft)` in `LeftNav` (32px row); button themes pin `minimumSize` = `controlHeight` + `shrinkWrap`; dialog label/id hierarchy + muted loading/empty + explicit panel stats styles; 8 new + 2 extended regression tests (Scaffold surface, nav = `rowHeight`, stats ≠ 48px/`0xD0FF0000`, palette hue outside violet band, button sizing, dialog hierarchy); gate: analyze 0 issues, 162 passed / 4 skipped — matches Done-when |

All 53 Done-when checkboxes across T1–T8 are checked (`tasks.md:99-103,126-133,156-162,185-191,214-218,241-246,269-274` + T7/T8 blocks). No task is blocked or partial. Gate command present for every task.

**Consistency sweep**: grep of `.specs/features/front-end-phase-2-catalog/` for the old `1000` threshold returns only this report's history; `design.md:69,97,216` and `tasks.md:157,256,269,272` all state 1002 with hairlines included — design/tasks/threshold derivation are mutually consistent.

---

## Spec-Anchored Acceptance Criteria

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| CAT-01: WHEN the app boots THEN the shell renders left nav, header, content, right panel | All four shell regions render on boot | `desktop/test/app_boot_test.dart:65` - `expect(find.byType(AppShell), findsOneWidget)`; `desktop/test/shell/app_shell_test.dart:110-113` - `expect(find.byType(LeftNav/RightPanel/Header), findsOneWidget)` + `find.byKey(const Key('shell-content'))` | ✅ PASS |
| CAT-02: WHEN the shell renders THEN nav 200–240px, panel 280–320px, sizes read from `lib/theme/`, no widget-level magic numbers | nav ∈ [200,240], panel ∈ [280,320], both equal the theme tokens | `desktop/test/shell/app_shell_test.dart:121-124` - `expect(navWidth, AppSizes.leftNavWidth); expect(navWidth, inInclusiveRange(200, 240)); expect(panelWidth, AppSizes.rightPanelWidth); expect(panelWidth, inInclusiveRange(280, 320))`; no-magic-number clause: `desktop/test/theme/tokens_test.dart:61-68` - `expect(AppSizes.shellMinWidthWithPanel, AppSizes.leftNavWidth + AppSizes.contentMinWidth + AppSizes.rightPanelWidth + 2 * AppSizes.hairline)` + literal `expect(…, 1002)`; verifier grep over `desktop/lib` (excluding generated + `tokens.dart`) found zero numeric size/radius/spacing literals and zero hardcoded colors | ✅ PASS |
| CAT-03: WHEN the window is too narrow THEN hide the right panel first, never overlap the left nav | Below threshold: panel hidden, nav intact at token width; exactly at threshold (1002): panel visible; restore re-shows panel without reload | Below: `desktop/test/shell/app_shell_test.dart:75-78` - `expect(find.byType(RightPanel), findsNothing); expect(tester.getSize(find.byType(LeftNav)).width, AppSizes.leftNavWidth)`; **boundary (new in `4929b46`)**: `:84-89` - `expect(find.byType(RightPanel), findsOneWidget)` + nav = token + `expect(content.width, greaterThanOrEqualTo(AppSizes.contentMinWidth - 2 * AppSpacing.lg))` at `Size(AppSizes.shellMinWidthWithPanel, 800)`; restore: `:103-104` - `findsOneWidget` with `expect(recorded, hasLength(1))` | ✅ PASS (round-1 boundary gap closed) |
| CAT-04: WHEN the shell renders THEN exactly one nav entry ("Catalog"), active while catalog on screen | 1 entry, `active == true`, label "Catalog" | `desktop/test/shell/app_shell_test.dart:150-153` - `expect(entryFinder, findsOneWidget); … expect(entry.active, isTrue); expect(entry.label, 'Catalog')`; derivation edge `:175` - `expect(entry.active, isTrue)` while creation-dialog surface active | ✅ PASS |
| CAT-05: THE header shows the app title and reserves the connection-indicator slot | Title rendered + slot key exists with reserved width | `desktop/test/shell/app_shell_test.dart:184-190` - `find.text('Legnaris')` + `find.byKey(const Key('connection-indicator-slot'))` + `expect(tester.getSize(slot).width, AppSizes.connectionSlotWidth)` | ✅ PASS |
| CAT-06: THE right panel renders Connection and Catalog sections as separate blocks separated by the theme hairline | Both blocks + hairline divider + Catalog stats | `desktop/test/shell/app_shell_test.dart:199-213` - `find.text('Connection')` / `find.byType(Divider), findsOneWidget` / `find.text('Catalog')` / `find.text('2 templates')` / `find.textContaining(RegExp(r'Last updated \d{1,2}:\d{2}'))` | ✅ PASS |
| CAT-07: WHEN first displayed THEN request `GET /templates` exactly once through the Phase-1 client and stay loading until settled | Exactly 1 GET /templates; isLoading true while gated | `desktop/test/catalog/catalog_provider_test.dart:125-128` - `expect(recorded, hasLength(1))` + method/path + `expect(…isLoading, isTrue)`; boot: `desktop/test/app_boot_test.dart:68` - `expect(recorded, hasLength(1))` | ✅ PASS |
| CAT-08: WHILE first request in flight THEN show loading state and NOT the empty state | Loading visible, empty absent while in flight | `desktop/test/catalog/catalog_screen_test.dart:77-80` - `expect(find.byType(LinearProgressIndicator), findsOneWidget); expect(find.text('Loading templates...'), findsOneWidget); expect(find.text('No templates available yet'), findsNothing); expect(find.byType(Card), findsNothing)` | ⚠️ Spec-precision gap (evidence complete; exact loading copy/composition not spec-defined — see notes) |
| CAT-09: WHEN success with N entries THEN render N entries in API order, each showing the name | N cards, API order preserved, name rendered | `desktop/test/catalog/catalog_screen_test.dart:125,131-132` - `expect(find.byType(Card), findsNWidgets(2))` + `expect(…first.dx, lessThan(…second.dx))`; order: `desktop/test/catalog/catalog_provider_test.dart:150-151` - `expect(state.templates!.map((t) => t.id).toList(), ['fantasy_v1', 'sci_fi_v1'])` | ✅ PASS |
| CAT-10: IF success with empty list THEN single line "No templates available yet", no decorative art | Exactly that copy, one line, no art | `desktop/test/catalog/catalog_screen_test.dart:159-161` - `expect(find.text('No templates available yet'), findsOneWidget); expect(find.byType(Card), findsNothing); expect(find.byType(Icon), findsNothing)` (copy matches spec literal and `app_en.arb`) | ✅ PASS |
| CAT-11: IF first request fails THEN Phase-1 mapped error message + Retry action | Message equals `errorCopy(kind)`; Retry button present | `desktop/test/catalog/catalog_screen_test.dart:111-112` - `expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget); expect(find.widgetWithText(OutlinedButton, 'Retry'), findsOneWidget)`; mapping: `desktop/test/catalog/catalog_provider_test.dart:198` - `expect(state.errorMessage, errorCopy(ApiErrorKind.serverError))` | ✅ PASS |
| CAT-12: WHEN user activates Retry THEN re-request exactly once | Call count 1 → 2 after one activation | `desktop/test/catalog/catalog_screen_test.dart:278,283` - `expect(recorded, hasLength(1))` before tap, `expect(recorded, hasLength(2))` after one tap; provider: `desktop/test/catalog/catalog_provider_test.dart:293,299` | ✅ PASS |
| CAT-13: WHILE in flight, a further trigger SHALL NOT start a second concurrent request | Extra trigger during gated flight → still 1 request | `desktop/test/catalog/catalog_provider_test.dart:265,268,277` - `expect(recorded, hasLength(1))` (before/after extra `refresh()`, after settle) + `:269 isLoading isTrue`; UI double-Retry: `desktop/test/catalog/catalog_screen_test.dart:234` - `expect(recorded, hasLength(2))` (initial + exactly one new) | ✅ PASS |
| CAT-14: IF entry missing a required field THEN defensive parse failure as error state with Retry — never blank, never partial | Parse-failure copy; no list; error+Retry rendered | `desktop/test/catalog/catalog_provider_test.dart:251-253` - `expect(state.errorMessage, parseCopy()); expect(state.templates, isNull); expect(state.hasLoaded, isFalse)`; rendered error+Retry: `desktop/test/catalog/catalog_screen_test.dart:111-112` (same `!hasLoaded && errorMessage != null` branch, `catalog_screen.dart` error state) | ✅ PASS |
| CAT-15: WHEN user activates entry or "Create sheet" THEN open creation surface for that template id | Dialog opens showing the clicked id (both entry points) | `desktop/test/catalog/create_sheet_navigation_test.dart:184-185` - `expect(shownId, findsOneWidget); expect(tester.widget<Text>(shownId).data, tc.expectedId)` over cases `:148-158` (card tap → `fantasy_v1`, button → `sci_fi_v1`) | ✅ PASS |
| CAT-16: WHEN creation surface dismissed THEN back to catalog, cached list intact, no re-request | Dialog gone, cached cards still there, call count unchanged | `desktop/test/catalog/create_sheet_navigation_test.dart:92-95` (Esc) - `expect(find.byType(CreateSheetDialog), findsNothing); expect(find.byType(Card), findsNWidgets(2)); expect(currentSurface(tester), ShellSurface.catalog); expect(recorded, hasLength(1))`; Close path `:108-111` | ✅ PASS |
| CAT-17: THE app keeps navigation shallow — catalog ↔ dialog only, no deep routing stack | Exactly one route push above the catalog | `desktop/test/catalog/create_sheet_navigation_test.dart:137` - `expect(recorder.pushed.length, pushesBeforeDialog + 1)` (unchanged after Esc, `:142`) | ✅ PASS |
| CAT-18: WHEN refresh (F5) on catalog THEN re-request and replace the rendered list | Second request issued; new entries appear | `desktop/test/app_boot_test.dart:160-162` - `expect(recorded, hasLength(2)); expect(find.byType(Card), findsNWidgets(2)); expect(find.text('Sci Fi Template'), findsOneWidget)`; provider: `desktop/test/catalog/catalog_provider_test.dart:324-331` | ✅ PASS |
| CAT-19: IF refresh fails while cache exists THEN keep cached entries + mapped error inline | Old list still rendered; mapped error visible inline (not replacing list) | `desktop/test/catalog/catalog_provider_test.dart:354-360` - `expect(state.templates!.map((t) => t.id).toList(), ['fantasy_v1']); expect(state.errorMessage, errorCopy(ApiErrorKind.serverError)); expect(state.lastUpdated, firstLoad.lastUpdated)`; render: `desktop/test/catalog/catalog_screen_test.dart:206-215` - `expect(find.byType(Card), findsNWidgets(2))` + mapped error + Retry + inline placement `expect(tester.getTopLeft(error).dy, lessThan(tester.getTopLeft(find.text('Fantasy Template')).dy))` | ⚠️ Spec-precision gap (evidence complete; spec says "inline" without placement — see notes) |

**Status**: ⚠️ Spec-precision gaps flagged — 19/19 ACs carry `file:line` evidence and their assertions meet the spec-defined outcome; 0 ❌ GAPs; 2 ⚠️ precision flags (re-assessed this round, not inherited blindly).

### Spec-precision gap notes (flagged, not failing)

1. **CAT-08 (loading-state copy)** — re-assessed against the current spec text: the AC fixes the loading/empty *distinction* (asserted: loading present, empty absent — spec-defined portion passes) but never defines the loading copy or composition. The test additionally pins `LinearProgressIndicator` + arb copy `catalogLoading` ("Loading templates..."). Solid anchoring in practice; the spec itself stays silent. Flag for spec hygiene: name the arb key as source of truth in the AC. (Grounded as L-003.)
2. **CAT-19 (inline refresh-error position)** — re-assessed: the spec says "inline" without a position. The test asserts the error renders *above* the list. That placement is pinned by tasks.md T4 ("erro inline de refresh acima da lista") and design.md, so the assertion is anchored — one level below spec. Flag: spec should state the placement. (Grounded as L-004.)

Neither flag blocks: both assertions meet every outcome the spec text actually defines.

---

## Discrimination Sensor

Scratch = temporary worktree `/tmp/opencode/lgn96-sensor` at `HEAD` (`4929b46`), created with `git worktree add`, removed with `--force` after the runs. Real-tree `git status --porcelain` captured before the round and confirmed byte-identical after gate + sensor (`M .specs/LESSONS.md`, `M .specs/lessons.json`, 4 untracked skill/validation entries — `PORCELAIN_MATCHES_BASELINE`); `git stash list` empty — no stash used at any point.

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| 1 | `desktop/lib/catalog/catalog_provider.dart:71` | Removed the single-flight guard `if (_inFlight) return;` | ✅ Killed — 2 failures: `catalog_provider_test.dart` "…issues no second request (CAT-13)" + `catalog_screen_test.dart` "two rapid Retry activations…" (152 passed / 2 failed) |
| 2 | `desktop/lib/shell/app_shell.dart:29` | Panel-collapse condition `>=` → `>` (panel hidden at exactly `shellMinWidthWithPanel` = 1002px) | ✅ Killed — exactly 1 failure: **`app_shell_test.dart` "Edge: at exactly the threshold the panel still shows and the content keeps its minimum measure (CAT-03 boundary)"** (test at `:81-90`, added by `4929b46`) — 153 passed / 1 failed |
| 3 | `desktop/lib/catalog/catalog_provider.dart:82` | Refresh failure discards the cached list (`templates: state.templates` → `templates: null`) | ✅ Killed — 3 failures: CAT-19 at provider, screen and boot level (151 passed / 3 failed) |

**Boundary probe (mutant 2, honest)**: the mutant differs from the original only at viewport == `shellMinWidthWithPanel` (1001 → both hide; 1280/1600 → both show). The suite's only exact-1002 viewport is the new boundary test at `app_shell_test.dart:82` (`Size(AppSizes.shellMinWidthWithPanel, 800)`), and its first assertion `expect(find.byType(RightPanel), findsOneWidget)` (`:84`) is precisely what the mutant breaks — the failure log names that test and no other. The kill is attributable to the fix's new test, not to incidental coverage.

**Sensor depth**: lightweight (3 mutations, highest-risk new code: single-flight, collapse threshold, cache retention)
**Result**: 3/3 killed — PASS ✅ (round-1 survivor M2 now closed)

---

## Interactive UAT Results (if performed)

Performed by the human maintainer on the running desktop app (PR #46). Verbatim feedback:

> Está maquiavelamente feito. COm cores que não combinam nem produzem legibilidade. Fontes desproporcionais e tamanhos que não condizem com o que conversamos até aqui. Mas funcionalmente está correto

**Severity**: Major (inference table: "wrong" vs the normative `desktop/AGENTS.md` tokens). The reviewer confirmed the functional behavior correct; the rendering violated three token rules — type scale ≤ 17 (48px error fallback on screen), row height 32 (full-height nav block), colors derived from a legible `ColorScheme` (M3 purple baseline over an unpainted black surface).

**Diagnosis** (empirical: golden render + pixel sampling + Flutter SDK source):

1. `AppShell` had no `Scaffold` → nothing painted the surface (black window) and, with no `Material` ancestor, unstyled `Text` fell into MaterialApp's `_errorTextStyle` (`material/app.dart:45-50`: `Color(0xD0FF0000)`, `fontSize: 48.0`, yellow double underline) — the red blocks in the right panel ("3 templates", "Last updated").
2. `ThemeData(useMaterial3: true)` with no `colorScheme` → `_colorSchemeLightM3` purple baseline (`theme_data.dart:3380`: `primary: 0xFF6750A4`) — purple-tinted neutrals across the shell (violates the "AI SaaS" prohibition).
3. The shell `Row(crossAxisAlignment: stretch)` forced a tight height the nav entry's `SizedBox(height: 32)` could not shrink → active tint rendered ~750px tall instead of the 32px row token.
4. M3 button defaults are 40px (outside the 32–36 control band) and the dialog rendered label/id at the same size (flat hierarchy).

| # | Check | Result | Details |
| - | ----- | ------ | ------- |
| 1 | Colors combine and produce legibility | ✅ Pass (post-fix) | Blue-gray `fromSeed` scheme, `Scaffold` surface painted; after-golden review shows no purple, neutral hairlines, readable contrast |
| 2 | Fonts/sizes match the agreed table | ✅ Pass (post-fix) | All rendered text ∈ {12, 13, 15, 17}; panel stats 13px (was 48px red); nav entry = 32px row; buttons = 36px |
| 3 | Functional correctness | ✅ Confirmed by reviewer | All 19 ACs green before the fix; the fix changed rendering only (suite 162 green, no assertion weakened) |

Fix applied in `1c886c3` (T8); re-verified per the fix→re-verify protocol (iteration 1 of max 3): gates green, regression sensors added, before/after goldens reviewed against the `desktop/AGENTS.md` Critical Layout Review checklist.

**Re-review (post-fix)**: ✅ **accepted** by the maintainer —

> Agora sim, ainda está um pouco cru mas dentro do que eu queria para as primeiras fases

Visual pass granted for the first phases; residual roughness noted as non-blocking and within V1 scope. No further fix cycle required.

---

## Code Quality

Checked against `.claude/skills/tlc-spec-driven/references/coding-principles.md`, with `desktop/AGENTS.md` and root `AGENTS.md` as the documented project guidelines.

| Principle        | Status |
| ---------------- | ------ |
| No features beyond what was asked | ✅ |
| No abstractions for single-use code  | ✅ |
| No unnecessary "flexibility" added   | ✅ |
| Only touched files required for task | ✅ (fix commit = 5 files: token, 2 tests, design, tasks) |
| Didn't "improve" unrelated code      | ✅ |
| Matches existing patterns/style      | ✅ |
| Would senior engineer approve?       | ✅ (round-1 gap closed; boundary now pinned) |
| Tests map to acceptance criteria and are non-shallow (spot-check one story) | ✅ |
| Spec-anchored outcome check: each test's asserted value matches the spec-defined outcome (or gap flagged) | ✅ (2 precision gaps flagged above) |
| Per-layer Coverage Expectation met: domain logic 1:1 AC mapping; widget layers cover happy + edge + error paths for every surface in scope | ✅ |
| Every test in scope maps to a spec AC, listed edge case, or Done-when criterion (no unclaimed tests) | ✅ |
| Documented project quality/testing guidelines followed | ✅ — root `AGENTS.md` (tests mandatory, edge cases first, `Happy path:`/`Edge:`/`Error:` prefixes, table-driven, English, tests co-located), `desktop/AGENTS.md` (tokens, layout, l10n, rebindable shortcut registry), tasks.md Test Coverage Matrix |

Spot-check (P1 shell / CAT-03): the collapse story is asserted non-shallowness — below-threshold (`:75-78`), **exact boundary** (`:84-89`), shrink→restore with a stable call counter (`:94-104`), plus the symbolic derivation in `tokens_test.dart:61-67` — four independent seams, not a shallow "renders" assertion.

Additional mandated checks:

- **`pubspec.yaml` unchanged**: `git diff main..HEAD -- desktop/pubspec.yaml` is empty ✅
- **English-only**: grep over the changed `desktop/lib`/`desktop/test` diff found no Portuguese identifiers, comments or strings ✅ (spec/design/tasks docs are pt-BR by skill convention, as in the spec phase)
- **No UI string literals outside `app_en.arb`**: grep for quoted `Text(...)`/label/title literals in `desktop/lib` (excluding generated) → zero hits; every widget reads `localizations.*` ✅
- **No widget magic numbers / hardcoded colors**: grep for numeric `width/height/padding/radius/fontSize/spacing` and `Color(0x…)/0xFF…/Colors.*` literals in `desktop/lib` outside `tokens.dart` → zero hits; `tokens_test.dart:61-68` pins the derived threshold ✅
- **Generated artifacts current**: Build gate ran `flutter pub get` + `dart run build_runner build` + `flutter analyze` (clean) + `flutter test` (154); `git status --porcelain` after the run is byte-identical to baseline → `.g.dart` / `app_localizations*.dart` are committed and in sync with sources ✅ (note: current build_runner warns `--delete-conflicting-outputs` was removed and ignored — harmless CLI deprecation; the gate command is verbatim from tasks.md)
- **Test integrity in the fix commit**: `4929b46` strengthens, never weakens — viewports `999` → `shellMinWidthWithPanel - 1` (same semantics, now derived), formula assertion extended with `+ 2 * AppSizes.hairline`, literal `1000` → `1002` follows the intentionally changed derivation, one boundary test added, zero tests removed ✅

---

## Edge Cases

- [x] **Missing `data` key → parse-failure error state**: `desktop/test/catalog/catalog_provider_test.dart:226` test "a 2xx without the data key takes the parse-failure path (CAT-14 edge)", assertions `:236-238` - `expect(state.errorMessage, parseCopy()); expect(state.hasLoaded, isFalse); expect(state.templates, isNull)` — never an empty success; rendered as error + Retry via the branch exercised in `catalog_screen_test.dart:111-112`
- [x] **Empty entry name renders**: `desktop/test/catalog/catalog_screen_test.dart:97-100` - `expect(find.byType(Card), findsOneWidget); expect(find.text(''), findsOneWidget); expect(find.text('blank'), findsOneWidget); expect(find.widgetWithText(TextButton, 'Create sheet'), findsOneWidget)` — layout intact, no throw (model side: `catalog_provider_test.dart:183-185`)
- [x] **F5 on a non-catalog surface → no request**: `desktop/test/app_boot_test.dart:128` - `expect(recorded, hasLength(1))` after F5 with the creation dialog open (guard: `shortcut_registry.dart:45` `if (ref.read(shellStateProvider) != ShellSurface.catalog) return null;`)
- [x] **Narrow→wide resize → panel returns without catalog reload**: `desktop/test/shell/app_shell_test.dart:97-104` - resize to `shellMinWidthWithPanel - 1` → `RightPanel findsNothing` (`:99`); restore to 1280 → `findsOneWidget` (`:103`); `expect(recorded, hasLength(1))` throughout (`:95,104`)
- [x] **Exactly at threshold vs threshold−1 (fix boundary)**: `app_shell_test.dart:72-79` (threshold−1 → panel hidden, nav intact) and `:81-90` (threshold → panel visible, nav = token, content ≥ `contentMinWidth - 2 * AppSpacing.lg`)
- [x] Spec edge sweep (spec.md:139): idempotency/retry → CAT-12/13 ✅; ordering → CAT-09 ✅; external-dependency failure → offline copy `catalog_provider_test.dart:221` ✅; state transitions → CAT-15/16/17 ✅; auth/rate limits, data lifecycle, observability explicitly N/A in spec

---

## Gate Check

- **Gate command**: Build level (tasks.md → Gate Check Commands): `export PATH="$HOME/flutter/bin:$PATH" && cd desktop && flutter pub get && dart run build_runner build --delete-conflicting-outputs && flutter analyze && flutter test`
- **Result**: 154 passed, 0 failed, 4 skipped (exit 0; `flutter analyze`: "No issues found!"; build_runner completed, no tracked output changed)
- **Test count before feature**: 82 passed + 4 skipped (Phase-1 baseline; includes the 2 obsolete `widget_test.dart` placeholder tests)
- **Test count after feature**: 154 passed + 4 skipped
- **Delta**: +72 = +74 new tests (T1 19 + T2 17 + T3 11 + T4 10 + T5 6 + T6 10 + T7 1) − 2 placeholder tests removed — arithmetic reconciles exactly
- **Skipped tests** (each justified, all pre-existing Phase-1 self-skipping contract tests):
  1. `live_contract_test.dart` "POST /sheets answers 201 with computed derived values" — self-skip: "No API base configured — set --dart-define=API_BASE_URL … (FE1-35)"
  2. `live_contract_test.dart` "PATCH an editable field answers 200 with a cascade-only delta" — same FE1-35 self-skip
  3. `live_contract_test.dart` "PATCH a derived field answers 400 INVALID_REQUEST/UNKNOWN_FIELD" — same FE1-35 self-skip
  4. `live_contract_test.dart` "GET a missing sheet answers 404 NOT_FOUND" — same FE1-35 self-skip

  All four require a live API; unit/widget suites cover the same contracts over `MockClient` (per tasks.md matrix: "contrato self-skip sem API"). None is a feature test.
- **Failures**: none
- **Post-UAT gate (T8, commit `1c886c3`)**: same command → `flutter analyze` "No issues found"; `flutter test` **162 passed, 0 failed, 4 skipped** (+8 net new tests: 3 shell + 2 dialog + 3 theme; 2 catalog tests extended with style assertions). `git diff main..HEAD -- desktop/pubspec.yaml` still empty; no generated output changed (no string/codegen change); edited files pass `dart format`.
- **Test Integrity Check**: no Phase-1 test file was modified or deleted except `desktop/test/widget_test.dart` (diff stat: only deletions, file removed). T6's Done-when called it a "rewrite"; the actual change is delete-and-supersede: placeholder test 1 ("renders the default app title") → `app_boot_test.dart:71` `expect(find.text('Legnaris'), findsOneWidget)` in full boot context; placeholder test 2 ("provider value, not hardcoded") → `app_boot_test.dart:86` `expect(tester.widget<MaterialApp>(find.byType(MaterialApp)).title, …)` — a stronger property-level assertion of the same contract. No meaningful test lost; no assertion weakened anywhere in the diff (the fix commits' changes are strengthening — see Code Quality). The T8 diff additionally reflows edited test files through `dart format` and braces three T6-era `if`s — both behavior-identical, mandated by the analyze gate.

---

## Fix Plans (if issues found)

### Fix 1 (round 1, round-2 verification): Pin the panel-collapse threshold at its exact boundary — VERIFIED CLOSED ✅

- **Root cause (round 1)**: `app_shell_test.dart` never exercised exactly `AppSizes.shellMinWidthWithPanel`, so the off-by-one mutant `>=` → `>` (`app_shell.dart:29`) survived; the derivation also excluded the two 1px `VerticalDivider`s (1000 vs 1002 ambiguity).
- **Delivered by `4929b46`**: derivation now `leftNavWidth + contentMinWidth + rightPanelWidth + 2 * hairline` = 1002 (`tokens.dart:83-84`, asserted symbolically + literally in `tokens_test.dart:61-68`); viewports tokenized (`app_shell_test.dart:73,97`); boundary test added (`:81-90`); design/tasks annotations updated.
- **Verify (executed this round)**: Build gate green (154/0/4); mutant M2 re-injected in a scratch worktree → suite FAILS with exactly the boundary test failing; real-tree porcelain unchanged. Done-when of T7 fully satisfied.

No new issues found this round.

### Fix 2 (post-UAT, T8): paint the shell surface, replace the purple baseline, pin control sizing — APPLIED ✅

- **Root cause (UAT round)**: three compounding render defects plus a token overrun: (a) `AppShell` built a bare `Column`, so nothing painted the surface (black window) and unstyled `Text` sat outside any `Material` — MaterialApp's `_errorTextStyle` (48px red, yellow underline) rendered as the panel stats; (b) no `colorScheme` meant M3's purple baseline (`#6750A4`) tinted every neutral; (c) the stretch-fed tight height inflated the nav entry from the 32px row token to a full-height block; (d) M3 buttons default to 40px (band: 32–36) and the dialog had label = id sizing.
- **Delivered by `1c886c3`** (6 lib files + 4 test files): `Scaffold` root with a load-bearing doc comment (`app_shell.dart:26`); `ColorScheme.fromSeed(0xFF546E7A)` passed at construction (`app_theme.dart:11-17`); `Align(topLeft)` in `LeftNav`; button themes pin `minimumSize = Size(64, controlHeight)` + `shrinkWrap`; dialog label → `bodySmall`/`onSurfaceVariant`, id → `bodyMedium`; loading/empty muted; panel stats explicit `bodyMedium`. Palette direction ("azul-acinzentado") chosen by the maintainer and recorded in `design.md`.
- **Verify (executed)**: `flutter analyze` → "No issues found"; `flutter test` → **162 passed, 0 failed, 4 skipped** (8 new regression tests + 2 extended; none removed or weakened — the only pre-existing edits are `dart format` reflow and braces for three T6-era `if`s); regression sensors: Scaffold presence + `surface` background, nav = `AppSizes.rowHeight`, panel stats `fontSize == body` and `color != 0xD0FF0000`, primary hue outside the violet band and ≠ `0xFF6750A4`, button `minimumSize` at theme level + measured `Close` height, dialog hierarchy; before/after goldens rendered from the real app and reviewed (Critical Layout Review checklist: no template-like grid, no unjustified modernism, no decoration). Lessons recorded: L-005 (seed the scheme), L-006 (Scaffold/Material root), L-007 (loosen stretch constraints).

---

## Requirement Traceability Update

Verifier is read-only on the real worktree (mandate: only `validation.md` may be written), so `spec.md` was not modified; statuses below are the verification outcome of record for the orchestrator to apply.

| Requirement | Previous Status (round 1) | New Status |
| ----------- | ------------------------- | ---------- |
| CAT-01, CAT-02, CAT-04…CAT-07 | Verified | ✅ Verified (independently re-confirmed) |
| CAT-03 | ⚠️ Verified — boundary-test gap open | ✅ Verified (gap closed by `4929b46`; sensor M2 killed) |
| CAT-08 | Verified — ⚠️ precision flag | ✅ Verified — ⚠️ spec-precision gap flagged (copy not spec-pinned) |
| CAT-09…CAT-18 | Verified | ✅ Verified (independently re-confirmed) |
| CAT-19 | Verified — ⚠️ precision flag | ✅ Verified — ⚠️ spec-precision gap flagged (placement not spec-pinned) |

---

## Summary

**Overall**: ✅ Ready (UAT fix cycle closed)

**Spec-anchored check**: 19/19 ACs have `file:line` evidence and meet the spec-defined outcome | 2 spec-precision gaps flagged (CAT-08, CAT-19 — non-failing, re-assessed this round)
**Sensor**: 3/3 mutations killed (including the round-1 survivor)
**Gate**: 154 passed, 0 failed, 4 skipped (Build level, exit 0) → **162 passed after the post-UAT T8 fix**
**UAT**: human reviewer confirmed functional correctness and rejected the visuals (Major) → fixed in `1c886c3`, re-verified (gates green, regression sensors, before/after golden review)

**What works**: shell (CAT-01–06) with token-only dimensions and an exact, hairline-inclusive collapse boundary at 1002px, painted surface and Material context (post-UAT), 32px nav row, 36px buttons; catalog provider with in-memory cache, single-flight and mapped failures (CAT-07, 09–14, 18, 19); four distinct render states with exact spec copy on empty (CAT-08–11); Retry semantics (CAT-12/13); shallow dialog navigation with zero refetch (CAT-15–17); F5 refresh wired through the rebindable registry with the surface guard; V1 palette = maintainer-approved blue-gray `fromSeed` scheme; codegen and l10n artifacts committed and current; analyze clean; test integrity reconciled (+80 over baseline, no weakened assertions).

**Issues found**: 2 spec-precision flags (CAT-08 copy, CAT-19 placement) carried as ⚠️ — spec hygiene, no code change required; already grounded as L-003/L-004. Post-UAT visual defects (Major) fixed and re-verified; grounded as L-005/L-006/L-007.

**Next steps**: UAT accepted post-fix (see Interactive UAT Results) — feature done from the agent's side. Merging PR #46 and transitioning LGN-96 remain with the maintainer (outside this agent's allowed scope).

**Closing gate**: `python3 .claude/skills/tlc-spec-driven/scripts/validate_state.py .specs/features/front-end-phase-2-catalog` exits 0 against this report.
