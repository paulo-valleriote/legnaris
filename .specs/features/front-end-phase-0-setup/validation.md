# front-end-phase-0-setup Validation

**Date**: 2026-09-29
**Spec**: `.specs/features/front-end-phase-0-setup/spec.md`
**Diff range**: `b959b39..ac9f5f8` (5 commits: `7bd534a` docs, `8d9a70e` bootstrap+CI, `db21efe` riverpod skeleton, `79a1c8c` go path filter, `ac9f5f8` test strengthening fix)
**Verifier**: independent sub-agent (iteration 2, author ≠ verifier)

**Result**: PASS ✅
**Verdict**: PASS — the iteration-1 gap is closed. The surviving mutant (`ref.watch(appTitleProvider)` → hardcoded `'Legnaris'`) was re-injected and is now KILLED by the new override test. All 16 ACs carry spec-anchored `file:line` evidence; 9 workflow ACs (FE0-08…FE0-16) are structural evidence only — runtime confirmation happens on the first PR run, which requires maintainer go-ahead to push (registered in `tasks.md` "Validação remota", not a code gap).

---

## Iteration History

| Iteration | Verdict | Gap | Status now |
| --------- | ------- | --- | ---------- |
| 1 | FAIL | Surviving mutant: `desktop/lib/main.dart:25` `ref.watch(appTitleProvider)` → hardcoded `'Legnaris'` stayed green — `widget_test.dart` asserted only the literal (Fix 1 in that report; task T6) | ✅ Closed — re-injected in this iteration, killed by `desktop/test/widget_test.dart:26` |
| 2 | PASS | none | — |

Fix applied in `ac9f5f8` (`test(LGN-89): assert widget renders overridden provider value`): `widget_test.dart` gained a second scenario pumping `appTitleProvider.overrideWithValue('Legnaris (overridden)')`. Verified as strengthening, not weakening: `git show ac9f5f8` keeps the original assertion `expect(find.text('Legnaris'), findsOneWidget);` verbatim (only the test name changed) and adds two new assertions.

---

## Task Completion

| Task | Status     | Notes   |
| ---- | ---------- | ------- |
| T1   | ✅ Done    | All 3 boxes checked; `git show --stat 7bd534a` = exactly `spec.md`, `tasks.md`, `docs/TDD-desktop-mvp.md`; gates `validate_spec.py`/`validate_tasks.py` exit 0 (this run) |
| T2   | ✅ Done    | All 6 boxes checked; platforms windows+linux only (`git ls-files desktop | grep -E '^desktop/(android\|ios\|macos\|web)/'` → empty), `environment.flutter: 3.47.5`, analyze/test green |
| T3   | ✅ Done    | All 7 boxes checked; FE0-07 verified — `git show --stat 8d9a70e` ships `desktop/**` **and** `.github/workflows/ci-flutter.yml` (55 lines) in one commit; YAML parse exit 0 |
| T4   | ⚠️ Partial | 6/7 boxes checked; unchecked box = manual `flutter run -d linux` demo (`tasks.md:184`) — needs system apt toolchain (user-installed), explicitly non-blocking; local FE0-02 evidence is the widget test, demo tracked in Success Criteria |
| T5   | ⚠️ Partial | 2/3 boxes checked; unchecked box = remote behavior validation (`tasks.md:210`) — needs the first PR run (push requires maintainer go-ahead) |
| T6   | ✅ Done    | All 3 boxes checked; 2 widget cases present, gate 3/3 green, kill-confirm re-derived independently this iteration (sensor mutation 1) |

---

## Spec-Anchored Acceptance Criteria

### P1: Bootstrap do app Flutter em `desktop/`

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| FE0-01: WHEN repo inspected THEN `desktop/` complete (pubspec, lib, test) with `windows`+`linux`, Go root untouched | Complete project, exactly those 2 platforms, no Go changes | `desktop/pubspec.yaml:1`, `desktop/lib/main.dart:1`, `desktop/test/widget_test.dart:1`; platforms `desktop/windows/runner/main.cpp:1`, `desktop/linux/runner/main.cc:1`; `git ls-files desktop \| grep -E '^desktop/(android\|ios\|macos\|web)/'` → empty; `git diff b959b39..ac9f5f8 -- '*.go' go.mod go.sum` → empty | ✅ PASS |
| FE0-02: WHEN `flutter run` in `desktop/` THEN placeholder opens without crash, app mounted inside `ProviderScope` | Boot without crash under `ProviderScope` | `desktop/test/widget_test.dart:9` - `await tester.pumpWidget(const ProviderScope(child: LegnarisApp()));` (pump throws on build crash) + `:11` - `expect(find.text('Legnaris'), findsOneWidget);`; wiring `desktop/lib/main.dart:14` - `const ProviderScope(child: LegnarisApp())` | ✅ PASS — widget-test evidence; manual `flutter run -d linux` demo pending (`tasks.md:184`, Success Criteria; non-blocking) |
| FE0-03: WHEN `flutter analyze` runs THEN exit 0 under `flutter_lints` | exit 0 | Gate run: "No issues found!", exit 0; baseline `desktop/analysis_options.yaml:10` - `include: package:flutter_lints/flutter.yaml` | ✅ PASS |
| FE0-04: WHEN `flutter test` runs THEN skeleton widget test passes (exit 0) | exit 0 | Gate run: 3/3 passed, exit 0; `desktop/test/widget_test.dart:11` - `expect(find.text('Legnaris'), findsOneWidget);` | ✅ PASS |
| FE0-05: WHEN build produces artifacts THEN `desktop/` `.gitignore` ignores them; nothing generated committed | `build/`, `.dart_tool/` ignored; no generated files tracked | `desktop/.gitignore:33` - `/build/`, `desktop/.gitignore:29` - `.dart_tool/`; `git check-ignore -v` matches both; `git ls-files desktop/build desktop/.dart_tool` → empty (`.g.dart` is spec-mandated committed, `pubspec.lock` is app lockfile — not build output) | ✅ PASS |
| FE0-06: WHEN a provider is defined THEN Riverpod 3 codegen (`@riverpod`, `.g.dart` generated+committed) | Generated provider returns `'Legnaris'`; UI renders the provider value | `desktop/lib/main.dart:9` - `@riverpod`, `:5` - `part 'main.g.dart';`, `desktop/lib/main.g.dart:14` - `@ProviderFor(appTitle)`; unit `desktop/test/app_title_provider_test.dart:10` - `expect(container.read(appTitleProvider), 'Legnaris')`; widget linkage `desktop/test/widget_test.dart:20` - `appTitleProvider.overrideWithValue('Legnaris (overridden)')` → `:26` - `expect(find.text('Legnaris (overridden)'), findsOneWidget)` + `:27` - `expect(find.text('Legnaris'), findsNothing)` | ✅ PASS (iteration-1 gap closed: override case kills the hardcode mutant — sensor mutation 1) |

### P1: CI de build Windows desde o primeiro commit

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| FE0-07: WHEN first commit introducing `desktop/` arrives THEN it includes the full active Flutter workflow | Same commit ships `desktop/` + `ci-flutter.yml` | `git show --stat 8d9a70e` → `desktop/**` + `.github/workflows/ci-flutter.yml` (55 lines) in one commit; `git log b959b39..ac9f5f8 -- desktop/` → first entry `8d9a70e` | ✅ PASS |
| FE0-08: WHEN PR touches `desktop/**` THEN job runs on `windows-latest` with analyze, test, build steps | `windows-latest`; steps `flutter analyze`, `flutter test`, `flutter build windows` | `.github/workflows/ci-flutter.yml:7-10` - `pull_request: paths: ['desktop/**', …]`; `:23` - `runs-on: windows-latest`; `:41-42` analyze; `:44-45` test; `:47-48` - `flutter build windows --release` | ✅ PASS (structural; runtime pending first PR run) |
| FE0-09: WHEN job succeeds THEN artifact = whole `build/windows/x64/runner/Release` (exe + DLLs + data) | Whole Release folder published | `.github/workflows/ci-flutter.yml:50-55` - `path: desktop/build/windows/x64/runner/Release`, `if-no-files-found: error`, upload step after build | ✅ PASS (structural; runtime pending first PR run) |
| FE0-10: IF any step fails THEN workflow fails, PR red, no artifact published | Failure stops job; upload never runs | Steps sequential with no `continue-on-error` (`ci-flutter.yml:41-48`; `grep -c 'continue-on-error\|matrix:'` → 0); upload at `:50` has no `if: always()` → GitHub skips it on failure | ✅ PASS (structural; runtime pending — spec Independent Test: break analyze in a test PR) |
| FE0-11: WHEN SDK version changes in `desktop/pubspec.yaml` THEN CI uses it automatically; `environment.flutter` single source | `flutter-version-file: desktop/pubspec.yaml`; no duplicated version in workflow | `.github/workflows/ci-flutter.yml:35` - `flutter-version-file: desktop/pubspec.yaml`; `desktop/pubspec.yaml:25` - `flutter: 3.47.5`; workflow holds only `channel: stable` (`:34`), no hardcoded Flutter version | ✅ PASS (structural) |
| FE0-12: WHEN push to `main` touches `desktop/**` THEN same jobs run and artifact published | `push` + `branches: [main]` + same job incl. upload | `.github/workflows/ci-flutter.yml:11-15` - `push: branches: [main]`, same `paths`; single `jobs.windows` with upload `:50-55` | ✅ PASS (structural; runtime pending first PR run) |

### P1: Isolamento de pipelines por path

| Criterion (WHEN X THEN Y) | Spec-defined outcome | `file:line` + assertion | Result |
| ------------------------- | -------------------- | ----------------------- | ------ |
| FE0-13: WHEN PR changes only `desktop/**` THEN Go workflow skipped, Flutter runs | Go `paths-ignore: desktop/**`; Flutter `paths` includes `desktop/**` | `.github/workflows/ci-lint.yml:7-8` - `paths-ignore: ['desktop/**']`; `.github/workflows/ci-flutter.yml:9` - `- 'desktop/**'` | ✅ PASS (structural; runtime pending — T5 unchecked box `tasks.md:210`) |
| FE0-14: WHEN PR changes only Go root THEN Flutter skipped, Go runs | Flutter paths exclude Go root; Go ignores only `desktop/**` | `ci-flutter.yml:8-10` - paths only `desktop/**` + own file; `ci-lint.yml:7-8` - ignores only `desktop/**` → Go-root change fires Go, not Flutter | ✅ PASS (structural; runtime pending first PR run) |
| FE0-15: WHEN PR changes both THEN both run | Filters non-exclusive; jobs untouched | `ci-lint.yml:7-8` + `ci-flutter.yml:8-10` (non-exclusive filters); `79a1c8c` diff adds only the 4 trigger lines — jobs at `ci-lint.yml:13-38` unchanged | ✅ PASS (structural; runtime pending first PR run) |
| FE0-16: WHEN PR changes only `.github/workflows/ci-flutter.yml` THEN Flutter runs (self-validation) | Own workflow file in `paths` | `ci-flutter.yml:10` - `'.github/workflows/ci-flutter.yml'` (both `pull_request` `:10` and `push` `:15`) | ✅ PASS (structural; runtime pending first PR run) |

**Status**: ✅ All 16 ACs covered with `file:line` evidence targeting the spec-defined outcomes — 7 with test/commit/runtime-gate evidence, 9 structural with runtime confirmation deferred to the first PR run (no push without maintainer go-ahead; deferral registered in `tasks.md:16`). · 0 spec-precision gaps — every AC defines a checkable outcome.

---

## Discrimination Sensor

Scratch: `git worktree add --detach /tmp/opencode/sensor-fe0-it2 HEAD` (HEAD = `ac9f5f8`); per mutation the full gate `flutter analyze && flutter test` ran in the scratch; removed with `--force` + `git worktree prune`. Real-tree `git status --porcelain` diffed against the pre-sensor baseline → byte-identical (`PORCELAIN_IDENTICAL`); `git worktree list` shows only the real tree.

| Mutation | File:line | Description | Killed? |
| -------- | --------- | ----------- | ------- |
| 1 (iteration-1 survivor, re-injected) | `desktop/lib/main.dart:25` | Removed provider linkage: `final title = ref.watch(appTitleProvider);` → `const title = 'Legnaris';` | ✅ Killed — `flutter analyze` exit 0, but `widget_test.dart:26` failed: `Expected: exactly one matching candidate / Actual: Found 0 widgets with text "Legnaris (overridden)"`; `flutter test` exit 1 |
| 2 | `desktop/lib/main.dart:10` | Wrong return value: `=> 'Legnaris'` → `=> 'Legnar'` | ✅ Killed — `app_title_provider_test` + `widget_test` default case failed (2 failing), exit 1 |
| 3 | `desktop/lib/main.dart:29` | Removed required render side effect: `body: Center(child: Text(title)),` → `body: const SizedBox(),` | ✅ Killed — both `widget_test` cases failed, exit 1 |

**Sensor depth**: lightweight (3 mutations)
**Result**: 3/3 killed — PASS ✅

Isolation: after `git worktree remove --force` + `prune`, `git status --porcelain` byte-identical to the 8-entry pre-sensor baseline (`.agents/.skill-lock.json*` modified; untracked `.claude/`, `.cursor/`, `.windsurf/`, `docs/plans/`, `.specs/LESSONS.md`, `.specs/lessons.json`, `validation.md` — pre-existing tree noise, untouched).

---

## Interactive UAT Results

Not performed — toolchain/CI infrastructure feature; automated checks suffice (validate.md §7: backend/infrastructure work needs no interactive UAT). The manual `flutter run -d linux` demo stays a Success Criterion pending the user's apt toolchain.

---

## Code Quality

| Principle        | Status |
| ---------------- | ------ |
| Minimum code     | ✅ — skeleton only (1 provider, 1 screen), zero features beyond spec Out of Scope |
| Surgical changes | ✅ — `79a1c8c` touches only `ci-lint.yml` trigger block (`git show 79a1c8c`: 4 added lines); `ac9f5f8` touches only `widget_test.dart` + `tasks.md` |
| No scope creep   | ✅ — no changes outside `desktop/`, workflows, `.specs/`, `docs/TDD-desktop-mvp.md`; Go code diff empty |
| Matches patterns | ✅ — workflow reuses `ci-lint.yml` idioms (`actions/checkout@v7`, `permissions: contents: read`); tests co-located, English everywhere (`AGENTS.md`) |
| Spec-anchored outcome check (asserted values match spec) | ✅ — `'Legnaris'` (spec default title), `'Legnaris (overridden)'` distinct override, `'Legnar'` mutant killed |
| Per-layer Coverage Expectation met | ✅ — widget layer for FE0-02/FE0-04 + FE0-06 linkage, unit layer for FE0-06; workflow layer = none per matrix (`tasks.md:28`) |
| Every test maps to a spec requirement - no unclaimed tests | ✅ — `widget_test.dart:6` → FE0-02/FE0-04; `widget_test.dart:14` → FE0-06 linkage (matrix row `tasks.md:26`); `app_title_provider_test.dart:6` → FE0-06 |
| Tests non-shallow (spot-check) | ✅ — story FE0-06: override test proves provider **provenance** of the rendered string, not just the literal; empirical: mutation 1 killed |
| Test integrity | ✅ — `git show ac9f5f8`: original assertion kept verbatim, one test renamed, two assertions added; no skip/disable, nothing weakened |
| Documented guidelines followed | ✅ — `AGENTS.md` (English, tests co-located, behavior changes ship tests), `docs/TDD-desktop-mvp.md` §Estratégia de Testes (`:245`), `references/coding-principles.md` |

---

## Edge Cases

- [x] `desktop/pubspec.yaml` declares `environment.flutter` (pin obrigatório) — `desktop/pubspec.yaml:21` `environment:` → `:25` `flutter: 3.47.5`
- [x] Committed `.g.dart` drift fails CI without a `build_runner` step — empirically verified in scratch: source `String appTitle` → `int appTitle`, `flutter analyze` exit 1 with `error • A value of type 'int' can't be returned … lib/main.g.dart:45:12 • return_of_invalid_type`; workflow has no `build_runner` step (`ci-flutter.yml:38-48`: pub get → analyze → test → build)
- [x] Docs-only PR keeps Go workflow firing — `.github/workflows/ci-lint.yml:7-8` excludes only `desktop/**`; `docs/` unfiltered (structural; GitHub path-filter semantics)
- [x] No silent runner fallback when `windows-latest` unavailable — `.github/workflows/ci-flutter.yml:23` single `runs-on: windows-latest`; no matrix, no `continue-on-error`, no fallback OS (grep count 0)
- [x] N/A dims (auth, data lifecycle, runtime observability, app concurrency) — out of scope per spec; no product behavior in this diff

---

## Gate Check

- **Gate command**: `cd desktop && flutter analyze && flutter test` · `cd desktop && dart run build_runner build` (T4) · YAML parse of `ci-lint.yml` + `ci-flutter.yml` (T3/T5)
- **Result**: 4 passed, 0 failed, 0 skipped — `flutter analyze`: "No issues found!", exit 0 · `flutter test`: 3/3 passed (app_title_provider ×1, widget ×2), exit 0 · `build_runner`: exit 0, "wrote 0 outputs", `git diff -- desktop/lib/main.g.dart` empty (committed `.g.dart` in sync) · YAML parse: exit 0, `YAML OK`
- **Test count before feature**: 0 (`desktop/` absent at `b959b39` — `git ls-tree b959b39 --name-only` has no `desktop`; diff contains no Go files, Go suite untouched)
- **Test count after feature**: 3
- **Delta**: +3 new tests
- **Skipped tests**: none
- **Failures**: none

---

## Fix Plans (if issues found)

None in iteration 2. Iteration 1's Fix 1 (strengthen `widget_test.dart` with a provider-override case) landed in `ac9f5f8` and is verified closed: its mutant re-injection is sensor mutation 1 → killed.

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status   |
| ----------- | --------------- | ------------ |
| FE0-01…FE0-05, FE0-07 | Verified | ✅ Verified |
| FE0-06 | Verified (iter-1 note) | ✅ Verified (widget linkage discriminating — sensor mutation 1 killed) |
| FE0-08…FE0-12 | Implementing | Implementing — structural evidence complete; flip to Verified after first PR run confirms runtime behavior |
| FE0-13…FE0-16 | Implementing | Implementing — structural evidence complete; flip to Verified after first PR run (T5 remote box `tasks.md:210`) |

Feature status: **code-level PASS** — no gaps in this iteration; remaining step is external: push on maintainer go-ahead, then first PR run confirms FE0-08…FE0-16 and the Success Criteria.

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 16/16 ACs matched spec outcome (7 runtime/test/commit evidence, 9 structural — runtime confirmation on first PR run) | 0 spec-precision gaps
**Sensor**: 3/3 mutations killed (incl. iteration-1 survivor re-injected)
**Gate**: 4 passed, 0 failed

**What works**: `desktop/` complete (windows+linux, no Go impact); Riverpod 3 codegen wired, analyze clean, 3 tests green, build_runner no-op; `ci-flutter.yml` matches FE0-07…FE0-12 + FE0-16 declaratively (`windows-latest`, version-file pin, Release artifact, no fallback); `ci-lint.yml` path isolation matches FE0-13…FE0-15 and keeps docs-only PRs firing; all spec edge cases verified with `file:line`, `.g.dart` drift failure verified empirically; widget test now discriminates provider provenance.

**Issues found**: none in iteration 2. Iteration 1's surviving mutant is killed (sensor mutation 1).

**Next steps**: push the branch and open the first PR on maintainer go-ahead — first run confirms FE0-08…FE0-16 (and the T5 remote box), then flip those requirements to Verified in spec.md.
