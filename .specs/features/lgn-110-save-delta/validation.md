# LGN-110 — Independent Verification Report

**Verdict: FAIL**

Two spec requirements are not verified by any test asserting the spec-defined
outcome (SAV-02 tooltip half, SAV-15 entirely), and the discrimination sensor
found one surviving mutant (the `Ctrl+S` active-sheet guard is not discriminated
by its test). Everything else passed.

- **Diff range reviewed:** `origin/main..HEAD` (`HEAD = 71d42989b2bf0120b6fed11247ca46532e35c5c6`).
- **Real tree status after verification:** clean (`git status --porcelain` empty).
- **Verifier:** independent subagent; did not author the code. All mutations were
  applied in an isolated scratch copy (`/tmp/opencode/lgn110-verify`, since removed),
  never in the real tree.

## Step 2 — Gate results (real tree, `desktop/`)

| Gate | Command | Result |
| ---- | ------- | ------ |
| Analyze | `flutter analyze` | `No issues found!` (ran in 3.0s) |
| Unit | `flutter test test/engine/sheet_defaults_test.dart` | **18 passed**, 0 failed |
| Unit | `flutter test test/sheet/sheet_edit_provider_test.dart` | **23 passed**, 0 failed |
| Widget | `flutter test test/sheet/sheet_screen_test.dart` | **46 passed**, 0 failed |
| Widget | `flutter test test/shell/shortcut_registry_test.dart` | **9 passed**, 0 failed |
| Widget | `flutter test test/catalog/create_sheet_layout_test.dart` | **3 passed**, 0 failed |

Targeted total: **99 passed, 0 failed**. Full suite not re-run (author reported 486 passed / 4 skipped).

## Step 1 — Per-requirement table

| ID | Verdict | Evidence (`file:line`) | Note |
| -- | ------- | ---------------------- | ---- |
| SAV-01 | PASS | `test/sheet/sheet_screen_test.dart:1689`, `:1803` | Footer renders when loaded; absent on template failure. |
| SAV-02 | **FAIL** | `test/sheet/sheet_screen_test.dart:1690` | Disabled half asserted (`onPressed isNull`); **no test asserts the `There are no changes to save.` tooltip** (grep over `desktop/test` for `sheetSaveNoChangesTooltip` / `no changes to save` → 0 matches). Spec requires both. |
| SAV-03 | PASS | `test/sheet/sheet_screen_test.dart:1700` | Enabled once a field is edited (`onPressed isNotNull`). |
| SAV-04 | PASS | `test/sheet/sheet_screen_test.dart:1702-1705` | Tap Save → exactly one PATCH; body asserted at `:1706`. |
| SAV-05 | PASS | `test/sheet/sheet_screen_test.dart:1832-1835`; `test/shell/shortcut_registry_test.dart:186-205` | Ctrl+S tick → one PATCH; registry binds/maps Ctrl+S. |
| SAV-06 | PASS | `test/sheet/sheet_screen_test.dart:1758-1789`, `:1841-1886`, `:1888-1925`; `test/sheet/sheet_edit_provider_test.dart:729-758` | Second activation, back and F5 in flight issue no second request; controller single-flight. |
| SAV-07 | PASS (gap) | `test/shell/shortcut_registry_test.dart:208-222` | Asserts tick `0` and no PATCH with no active sheet. **But the guard is not discriminated** — see survivor #6; the autoDispose tick resets before the read, so the assertion is vacuous. |
| SAV-08 | PASS | `test/sheet/sheet_edit_provider_test.dart:563-568`; `test/sheet/sheet_screen_test.dart:1705-1710` | Exact nested delta body. |
| SAV-09 | PASS | `test/sheet/sheet_edit_provider_test.dart:600-604` | Reverted field omitted. |
| SAV-10 | PASS | `test/sheet/sheet_edit_provider_test.dart:605`; `test/sheet/sheet_screen_test.dart:1706-1710` | No `derived` key in the payload. |
| SAV-11 | PASS | `test/sheet/sheet_edit_provider_test.dart:627-631` | Exclusive selection ships selected member + zeroed sibling. |
| SAV-12 | PASS | `test/sheet/sheet_edit_provider_test.dart:648-649` | No change → no PATCH, `isSaving` false. |
| SAV-13 | PASS | `test/engine/sheet_defaults_test.dart:156-213`; `test/sheet/sheet_edit_provider_test.dart:570-572` | Deep merge: new key, overwrite, recursion, empty, input immutability. |
| SAV-14 | PASS | `test/sheet/sheet_edit_provider_test.dart:573`, `:760-792` | Baseline recomputed; in-flight edit stays pending; sent field clears. |
| SAV-15 | **FAIL** | — | **No test.** No control-identity assertion after a save merge; grep for `SAV-15`/rebuild in `desktop/test` → 0 matches. The granular-rebuild tests (`sheet_screen_test.dart:559-677`) cover *editing*, not the merge completion. |
| SAV-16 | PASS | `test/sheet/sheet_edit_provider_test.dart:673-681` | Empty `{}` cascade clears pending, values unchanged. |
| SAV-17 | PASS | `test/sheet/sheet_edit_provider_test.dart:724`; `test/sheet/sheet_screen_test.dart:1753` | Inline error cleared on success. |
| SAV-18 | PASS | `test/sheet/sheet_edit_provider_test.dart:707-710`; `test/sheet/sheet_screen_test.dart:1742-1745` | Edits kept, dirty indicator stays. |
| SAV-19 | PASS (note) | `test/sheet/sheet_screen_test.dart:1742-1743` | Mapped copy rendered inline (`sheet-save-error` + `errorCopy`). The "above the action row" ordering is structural, not geometrically asserted. |
| SAV-20 | PASS | `test/sheet/sheet_edit_provider_test.dart:710` | `derived.armorClass` stays `0.0` — no cascade on failure. |
| SAV-21 | PASS | `test/sheet/sheet_edit_provider_test.dart:717-722`; `test/sheet/sheet_screen_test.dart:1748-1752` | Retry issues exactly one further PATCH. |
| SAV-22 | PASS | `test/sheet/sheet_edit_provider_test.dart:714-715` | No auto-retry. |
| SAV-23 | PASS | `test/sheet/sheet_edit_provider_test.dart:724`; `test/sheet/sheet_screen_test.dart:1753` | Error cleared on later success. |
| DIA-01 | PASS | `test/catalog/create_sheet_layout_test.dart:93-97` | 1280×800: no overflow, submit rect does not overlap preview rect. |
| DIA-02 | PASS | `test/catalog/create_sheet_layout_test.dart:99-103`, `:105-122` | 1280×720 and 640×480: no exception, scroll exists, height ≤ cap, `maxScrollExtent > 0`. |
| DIA-03 | PASS | `test/catalog/create_sheet_layout_test.dart:105-108` | 640×480: rects do not intersect. |
| DIA-04 | PASS (note) | `test/catalog/create_sheet_layout_test.dart:105-122` | Submit present and non-overlapped; "reachable" asserted via `findsOneWidget`, not an actual tap. |

## Step 3 — Discrimination sensor (isolated scratch, `rm -rf` after)

Baseline copy: `cp -r desktop /tmp/opencode/lgn110-verify/desktop`. Each mutation
applied alone, targeted test run, then reverted.

| # | Mutation | Target test | Result |
| - | -------- | ----------- | ------ |
| 1 | `sheet_edit_provider.dart` `pendingDelta`: drop the `value != valueAtPath(defaults, path)` filter | `sheet_edit_provider_test.dart` "a field reverted to the baseline…" (`:580`) | **KILLED** — `Expected {'stats': {'dexterity': 18}} / Actual {'stats': {'dexterity': 18, 'constitution': 12}}` at `:602` |
| 2 | `_applySaved`: `defaults: data` instead of recomputed `baseline` | `sheet_edit_provider_test.dart` "an edit made while the save is in flight stays pending…" (`:761`) | **KILLED** — `Expected: true / Actual: <false>` at `:786` |
| 3 | `save()` catch: reset `data` to the old baseline (drop edits) | `sheet_edit_provider_test.dart` "a failure keeps the edits…" (`:684`) | **KILLED** — `Expected: true / Actual: <false>` at `:708` |
| 4 | `create_sheet_dialog.dart`: remove both `Flexible` wrappers | `create_sheet_layout_test.dart` (whole file) | **KILLED** — `+1 -2`; 1280×720 (`:99`) and 640×480 (`:105`) both fail in `expectNoOverlap` (`:72`) |
| 5 | `sheet_screen.dart` `_leave`: remove the `isSaving` early-return | `sheet_screen_test.dart` "back while a save is in flight…" (`:1843`) | **KILLED** — fails at `:1869` (`AlertDialog` appears) |
| 6 | `shortcut_registry.dart`: bump the save tick unconditionally (drop `activeSheetIdProvider != null`) | `shortcut_registry_test.dart` "no registered sheet" (`:209`) | **SURVIVED** — full file still `9 passed`. Diagnostic: adding `harness.container.listen(sheetSaveTickProvider, (_, _) {})` before the invoke kills it, proving the mutant is live but the test lets the autoDispose provider reset to `0` before the read. |

Killed: **5 / 6**. Survived: **1** (mutation 6).

## Ranked fix tasks

1. **SAV-15 has no test (highest).** Add a widget test in
   `desktop/test/sheet/sheet_screen_test.dart` that snapshots `SheetFieldControl`
   identities, edits one field, completes the save with a cascade that changes a
   *different* derived control, and asserts only the cascade-affected control
   rebuilds (identity changes) while untouched controls keep identity — the same
   pattern as `:559-677` but driven by the merge.
2. **SAV-02 tooltip untested.** Add an assertion that the disabled Save action is
   wrapped in a `Tooltip` carrying `AppLocalizations.sheetSaveNoChangesTooltip`
   (`desktop/test/sheet/sheet_screen_test.dart`, in the save-footer group near `:1690`).
3. **Surviving mutant #6 — SAV-07 guard not discriminated.** The "no registered
   sheet" test must hold `sheetSaveTickProvider` alive (e.g.
   `harness.container.listen(sheetSaveTickProvider, (_, _) {})`) so removing the
   `activeSheetIdProvider != null` guard is actually caught. Without this the test
   is green even when the guard is deleted.
4. **Minor — SAV-19 / DIA-04 precision.** SAV-19 does not assert the error copy is
   *above* the action row (ordering is structural only); DIA-04 asserts the submit
   button exists but never taps it. Both are low risk but leave the exact spec
   wording partly unverified.

## Notes / non-blocking observations

- The implementation uses direct `SheetEditState(...)` construction rather than the
  `copyWith` named in `tasks.md:138`; the spec outcome is unaffected, but the task
  text and code diverge.
- `SheetEditState.pendingDelta` correctly excludes derived leaves because read-only
  controls never call `setPath` (verified by `sheet_edit_provider_test.dart:226-250`
  and `sheet_screen_test.dart:445-453`).
