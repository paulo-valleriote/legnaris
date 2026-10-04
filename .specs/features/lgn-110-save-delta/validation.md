# LGN-110 — Independent Verification Report (Round 2)

**Verdict: PASS**

Round 1 flagged SAV-02 (tooltip half), SAV-15 (merge granularity) and a surviving
mutant (#6, `Ctrl+S` active-sheet guard not discriminated). Commit
`cdfb3593ee70003ca43c66562c5acaef960b0c74` (T7) closes all three. Every one of the
27 requirements is now asserted against the spec-defined outcome, and the
discrimination sensor killed 7/7 mutants, including the round-1 survivor and a new
SAV-15 probe.

- **Diff range reviewed:** `origin/main..HEAD`
  (`origin/main = 6eab23312d8a3f44faac5d6e574a186be91a66a8`,
  `HEAD = cdfb3593ee70003ca43c66562c5acaef960b0c74`).
- **Real tree status after verification:** clean, then only the report file is
  modified. `git status --porcelain` is empty before writing this file; this report
  is **tracked** at HEAD (the round-1 report was committed), so overwriting it shows
  ` M .specs/features/lgn-110-save-delta/validation.md` — **not** the `??` the task
  text predicted. No other path is touched (see Step 3 tail).
- **Verifier:** independent subagent; did not author the code. All mutations were
  applied in an isolated scratch copy (`/tmp/opencode/lgn110-verify2/desktop`), never
  in the real tree, and the scratch was removed and parity-checked before writing.

## Step 2 — Gate results (real tree, `desktop/`)

| Gate | Command | Result |
| ---- | ------- | ------ |
| Analyze | `flutter analyze` | `No issues found!` (ran in 3.2s) |
| Unit | `flutter test test/engine/sheet_defaults_test.dart` | **18 passed**, 0 failed |
| Unit | `flutter test test/sheet/sheet_edit_provider_test.dart` | **23 passed**, 0 failed |
| Widget | `flutter test test/sheet/sheet_screen_test.dart` | **47 passed**, 0 failed |
| Widget | `flutter test test/shell/shortcut_registry_test.dart` | **9 passed**, 0 failed |
| Widget | `flutter test test/catalog/create_sheet_layout_test.dart` | **3 passed**, 0 failed |

Targeted total: **100 passed, 0 failed**. Full suite not re-run.

## Step 1 — Per-requirement table (spec-defined outcome)

| ID | Verdict | Evidence (`file:line`) | Note |
| -- | ------- | ---------------------- | ---- |
| SAV-01 | PASS | `desktop/test/sheet/sheet_screen_test.dart:1689`; `:1880` | Footer + Save render when the form is loaded; absent when the template fails. |
| SAV-02 | PASS | `desktop/test/sheet/sheet_screen_test.dart:1690`, `:1692-1703`, `:1713-1724` | Disabled Save asserted `onPressed isNull` **and** wrapped in a `Tooltip` whose message is `sheetSaveNoChangesTooltip`; tooltip asserted absent once enabled (T7 gap closed). |
| SAV-03 | PASS | `desktop/test/sheet/sheet_screen_test.dart:1712` | Enabled (`onPressed isNotNull`) once a field is edited. |
| SAV-04 | PASS | `desktop/test/sheet/sheet_screen_test.dart:1726-1734` | Tap Save → exactly one PATCH; body asserted nested `{'stats': {'dexterity': 16}}`. |
| SAV-05 | PASS | `desktop/test/sheet/sheet_screen_test.dart:1903-1913`; `desktop/test/shell/shortcut_registry_test.dart:174-205` | Ctrl+S bound and mapped; tick → exactly one PATCH with a pending edit. |
| SAV-06 | PASS | `desktop/test/sheet/sheet_screen_test.dart:1809-1815`, `:1918-1963`, `:1965-2002`; `desktop/test/sheet/sheet_edit_provider_test.dart:729-758` | Second activation, back and F5 while in flight issue no second request; controller single-flight. |
| SAV-07 | PASS | `desktop/test/shell/shortcut_registry_test.dart:208-226` | No active sheet → tick stays `0`, no request. The tick is held alive (`:215`) so the guard is now discriminated (mutant #6 killed). |
| SAV-08 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:563-568`; `desktop/test/sheet/sheet_screen_test.dart:1729-1734` | `data` carries exactly the changed editable leaf, nested by dotted path. |
| SAV-09 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:579-607` | A reverted field is omitted (`delta == {'stats': {'dexterity': 18}}`). |
| SAV-10 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:605`; `desktop/test/sheet/sheet_screen_test.dart:1730-1734` | No `derived` key in the payload. |
| SAV-11 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:609-633` | Exclusive selection ships selected member `1` and sibling `0`. |
| SAV-12 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:635-650`; `desktop/test/sheet/sheet_screen_test.dart:1899-1901` | No difference → no PATCH, `isSaving` false. |
| SAV-13 | PASS | `desktop/test/engine/sheet_defaults_test.dart:155-214`; `desktop/test/sheet/sheet_edit_provider_test.dart:544-577` | `mergeCascade`: new key, overwrite, deep recursion, empty delta, input immutability; merged into `data`, untouched derived key preserved. |
| SAV-14 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:569-576`, `:760-792` | Baseline advances to merged data (`defaults.dexterity == 18`) and dirty clears on the plain path; in-flight edit stays pending. |
| SAV-15 | PASS | `desktop/test/sheet/sheet_screen_test.dart:1823-1867` | Snapshots `SheetFieldControl` identities across a save; only `derived.armorClass` (the cascade-changed control) rebuilds, every other control keeps identity (T7 gap closed). |
| SAV-16 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:652-681`; `desktop/test/sheet/sheet_screen_test.dart:1817-1819` | Empty `{}` cascade clears pending and advances the baseline without changing values. |
| SAV-17 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:724`; `desktop/test/sheet/sheet_screen_test.dart:1784` | Previously rendered inline error cleared on success. |
| SAV-18 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:707-710`; `desktop/test/sheet/sheet_screen_test.dart:1768` | Edits kept and dirty indicator stays after a failure. |
| SAV-19 | PASS | `desktop/test/sheet/sheet_screen_test.dart:1766-1775` | Mapped copy rendered inline (`sheet-save-error` + `errorCopy`) **and** geometrically above the action row (`error.bottom <= save.top`) (T7 gap closed). |
| SAV-20 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:710` | `derived.armorClass` stays `0.0` — no cascade added on failure. |
| SAV-21 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:717-722`; `desktop/test/sheet/sheet_screen_test.dart:1779-1783` | Retry issues exactly one further PATCH. |
| SAV-22 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:714-715` | PATCH never auto-retried. |
| SAV-23 | PASS | `desktop/test/sheet/sheet_edit_provider_test.dart:724`; `desktop/test/sheet/sheet_screen_test.dart:1784` | Inline error cleared on the later success. |
| DIA-01 | PASS | `desktop/test/catalog/create_sheet_layout_test.dart:93-97` | 1280×800: no exception; submit rect does not overlap preview rect. |
| DIA-02 | PASS | `desktop/test/catalog/create_sheet_layout_test.dart:99-103`, `:105-137` | 1280×720 and 640×480: no exception, preview scroll exists, height ≤ `previewCardMaxHeight`, `maxScrollExtent > 0`. |
| DIA-03 | PASS | `desktop/test/catalog/create_sheet_layout_test.dart:105-108` | 640×480: action row and preview rects do not intersect. |
| DIA-04 | PASS | `desktop/test/catalog/create_sheet_layout_test.dart:110-123` | At 640×480 a name is entered, Create asserted enabled (`onPressed isNotNull`) and its rect bounded to `[0, 480]` — reachable and unclipped (T7 gap closed). |

**27/27 PASS.**

## Step 3 — Discrimination sensor (isolated scratch, removed after)

Baseline copy: `cp -r desktop /tmp/opencode/lgn110-verify2/desktop`. Each mutation
applied alone, the named test run, then reverted before the next. Scratch `lib/`
and `test/` were diffed against the real tree after reverting (`LIB_PARITY_OK`,
`TEST_PARITY_OK`) and the scratch was `rm -rf`'d.

| # | Mutation | Target test | Result |
| - | -------- | ----------- | ------ |
| 1 | `sheet_edit_provider.dart` `pendingDelta`: drop the `value != valueAtPath(defaults, path)` filter | `sheet_edit_provider_test.dart` "a field reverted to the baseline leaves the delta…" (`:580`) | **KILLED** — `Expected {'stats': {'dexterity': 18}} / Actual {… 'constitution': 12}` at `:602` |
| 2 | `_applySaved`: `defaults: data` instead of the recomputed baseline | `sheet_edit_provider_test.dart` "an edit made while the save is in flight stays pending…" (`:761`) | **KILLED** — `Expected: true / Actual: <false>` at `:786` |
| 3 | `save()` catch: reset `data` to the baseline and clear `editedPaths` (drop edits) | `sheet_edit_provider_test.dart` "a failure keeps the edits…" (`:684`) | **KILLED** — `Expected: true / Actual: <false>` at `:708` |
| 4 | `create_sheet_dialog.dart`: remove both `Flexible` wrappers | `create_sheet_layout_test.dart` (whole file) | **KILLED** — `+1 -2`; 1280×720 (`:99`) and 640×480 (`:105`) fail in `expectNoOverlap` (`:72`) |
| 5 | `sheet_screen.dart` `_leave`: remove the `isSaving` early-return | `sheet_screen_test.dart` "back while a save is in flight…" (`:1920`) | **KILLED** — `AlertDialog` appears; fails at `:1946` |
| 6 | `shortcut_registry.dart`: bump the save tick unconditionally (drop the `activeSheetIdProvider != null` guard) | `shortcut_registry_test.dart` "Ctrl+S with no registered sheet…" (`:209`) | **KILLED** — round-1 survivor now dead; tick reads `1`, fails at `:223` |
| 7 | `_applySaved`: `data: state.data` (no cascade applied) | `sheet_screen_test.dart` SAV-15 granular test (`:1823`) | **KILLED** — `derived.armorClass` fails to rebuild; fails at `:1858` |

Killed: **7 / 7**. Survived: **0**.

## Step 3 tail — real tree untouched

`git status --porcelain` in the real tree is empty except for this report (the only
file created/modified). No scratch path leaked into the real tree.

## Remaining gaps

**None blocking.** All 27 requirements have a spec-anchored assertion and every
probed mutant is killed. Two non-blocking observations:

1. `tasks.md:138` describes a `SheetEditState.copyWith`; the implementation builds
   `SheetEditState(...)` directly (`desktop/lib/sheet/sheet_edit_provider.dart:112`,
   `:164`, `:198`). The spec outcome is unaffected; only the task text diverges.
2. The report path is tracked at HEAD, so this round's overwrite reports as a
   modification rather than a new untracked file (contrary to the task text's `??`).
   This is a bookkeeping note, not a product gap.
