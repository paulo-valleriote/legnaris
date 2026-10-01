# LGN-105 Exclusive Options Validation

**Date**: 2026-10-01
**Spec**: `.specs/features/lgn-105-exclusive-options/spec.md`
**Diff range**: `6ab54f9..HEAD` (commits `f86d5bb` … `fc775ea`, `010895a`)
**Verifier**: independent sub-agent (author ≠ verifier)

## Validation

**Result**: PASS

The implementation matches the spec-defined outcome for every acceptance criterion (EXC-01…EXC-17). The earlier negative verdict came from one surviving discrimination-sensor mutant (the spec's contract clause *"no `exclusive_group` on any other leaf"* had no whole-tree assertion). Fix commit `010895a` (task T11) extends `TestFantasyV1_LeafContracts` to assert the whole-tree grouped set is exactly the armor-class pair; the re-run kills the mutant. No gaps remain.

---

## Task Completion

| Task | Status  | Notes |
| ---- | ------- | ----- |
| T1   | ✅ Done | contract field + DTO mirror |
| T2   | ✅ Done | ac pair tagged |
| T3   | ✅ Done | DAG index + normalization |
| T4   | ✅ Done | service wiring + 400 mapping |
| T5   | ✅ Done | Dart decode |
| T6   | ✅ Done | grouping `rows` |
| T7   | ✅ Done | segmented/radio control |
| T8   | ✅ Done | form rendering (provisional screen wiring) |
| T9   | ✅ Done | batch `setExclusive` |
| T10  | ✅ Done | screen wiring (replaced T8 provisional wiring), fixtures, layout fix |
| T11  | ✅ Done | whole-tree `exclusive_group` assertion (fix for Verifier gap #1) |

All task boxes in `tasks.md` are checked; commits are one-per-task and Conventional-Commits shaped.

---

## Spec-Anchored Acceptance Criteria

| Criterion | Spec-defined outcome | `file:line` + assertion | Result |
| --------- | -------------------- | ----------------------- | ------ |
| EXC-01 declared group is included | response carries `exclusive_group:"<id>"` verbatim | `internal/sheet/template/dto_test.go:130` - `assert.JSONEq(want, '…"exclusive_group":"armorClassAbility"')`; copy at `internal/sheet/template/dto_test.go:165` | ✅ |
| EXC-02 no group omits key | `exclusive_group` absent from payload | `internal/sheet/template/dto_test.go:135` - expected JSON has no `exclusive_group` key | ✅ |
| EXC-03 fantasy_v1 pair declares `armorClassAbility` | both ac leaves == `armorClassAbility`; all other config leaves == "" | `internal/sheet/template/seeders_test.go:221` and `:228` - `assert.Equal(exclusiveGroupArmorClassAbility, …)` / `assert.Empty(…)` | ✅ |
| EXC-04 one member on zeroes siblings | absent sibling written `0` | `internal/sheet/template/dag_test.go:1473` - `want: {acUsesDex:1, acUsesCon:0}`; `internal/sheet/service_test.go:462` - stored pair `(1,0)`; `internal/app/integration_test.go:391` - stored `acUsesDex == 0` | ✅ |
| EXC-05 two members on → 400 INVALID_REQUEST | typed conflict, HTTP 400 | `internal/sheet/template/dag_test.go:1518` - group+keys; `internal/sheet/service_test.go:487-488` - `CodeInvalidRequest`, `HttpStatus()==400`; `internal/app/integration_test.go:396,400` - `StatusBadRequest`, `INVALID_REQUEST` | ✅ |
| EXC-06 member off leaves siblings | input returned unchanged, no sibling added | `internal/sheet/template/dag_test.go:1490` - `wantSameMap` true; `internal/sheet/service_test.go:424-430` - stored `(0,0)` from stored `(0,1)`, cascade `armorClass 10` | ✅ |
| EXC-07 recompute dependent derived | derived reflects only the active member | `internal/sheet/service_test.go:416` - cascade `derived.armorClass == 12.0` (10+2·dex); `internal/app/integration_test.go:382` - `armorClass == 11.0` (10+1·con) | ✅ |
| EXC-08 <2 editable members not exclusive | 1-member group dropped; derived never grouped | `internal/sheet/template/dag_test.go:1439` - `NotContains(groups,"soloGroup")`; `:1438,1440` derived excluded; `:1500-1504` solo not enforced | ✅ |
| EXC-09 ≤3 options + labels ≤30 → one segmented | `SegmentedButton` keyed `sheet-exclusive-armorClassAbility` | `desktop/lib/engine/exclusive_control.dart:46-50`; `desktop/test/engine/exclusive_control_test.dart:62` findsOneWidget, `:85` 30-char stays segmented | ✅ |
| EXC-10 >3 options or long label → radio | `Radio` group instead | `desktop/test/engine/exclusive_control_test.dart:103-104` (4 opts → 4 radios), `:127-128` (31 chars → radios) | ✅ |
| EXC-11 read-only disabled, active selected | `onSelectionChanged == null`, active selected | `desktop/test/engine/exclusive_control_test.dart:215-216`; `desktop/test/engine/sheet_form_test.dart:406-407` (data mode); `desktop/test/catalog/create_sheet_preview_test.dart:317-318` | ✅ |
| EXC-12 single-member group → normal control | no exclusive row; per-field control | `desktop/lib/engine/field_grouping.dart:85`; `desktop/test/engine/field_grouping_test.dart:162-166`; `desktop/test/engine/sheet_form_test.dart:455-459` | ✅ |
| EXC-13 select sets member 1 + siblings 0, one emission | one state emission, both paths edited | `desktop/lib/sheet/sheet_edit_provider.dart:90-98`; `desktop/test/sheet/sheet_edit_provider_test.dart:273-283` (values + `emissions hasLength(1)`); `desktop/test/sheet/sheet_screen_test.dart:1064-1070` | ✅ |
| EXC-14 re-select active → unchanged | same state instance, no emission | `desktop/lib/sheet/sheet_edit_provider.dart:85-89`; `desktop/test/sheet/sheet_edit_provider_test.dart:312-313` (`identical`, `emissions 0`); `desktop/test/sheet/sheet_screen_test.dart:1104-1110` | ✅ |
| EXC-15 default selection from schema | seeded default shown active | `desktop/test/sheet/sheet_screen_test.dart:1024` - `selected == {'config.acUsesDex'}`; `desktop/test/catalog/create_sheet_preview_test.dart:317`; `desktop/test/sheet/sheet_edit_provider_test.dart:80` | ✅ |
| EXC-16 editing issues no network request | request log GET-only | `desktop/test/sheet/sheet_edit_provider_test.dart:285-286`; `desktop/test/sheet/sheet_screen_test.dart:1079-1080` (`hasLength(1)`, single `GET`) | ✅ |
| EXC-17 desktop parses `exclusive_group` | present → id; absent/null → null | `desktop/lib/api/models/template.dart:58,83`; `desktop/test/api/template_model_test.dart:168` (id), `:180,192` (absent/explicit null), `:201` (untagged leaf null) | ✅ |
| **Contract: "nowhere else"** | only `config.acUsesDex`/`config.acUsesCon` carry a non-empty group across the whole template | T11: `internal/sheet/template/seeders_test.go:354-357` - `assert.Equal(map{"config.acUsesDex": …, "config.acUsesCon": …}, grouped, "exclusive_group must cover exactly the armor-class pair")` over all 131 leaves (`:316`); config-section emptiness also at `:225-230`; Dart fixtures at `desktop/test/fixtures/contract_fixtures_test.dart:106-112` | ✅ |

**Status**: ✅ All 17 ACs match the spec outcome; the Independent-Test sub-clause ("nowhere else", whole tree) is now covered by T11.

**Note on EXC-17 traceability**: the User Stories enumerate 16 EARS criteria (3+5+4+4); EXC-17 exists only in the Requirement Traceability table and T5, and its outcome is defined by the edge case `spec.md:133` (absent group → null). Outcome verified; minor documentation inconsistency, not a precision gap.

---

## Discrimination Sensor

Isolated scratch: `git worktree add --detach /tmp/opencode/lgn-105-verifier HEAD` (never `git stash`). Real-tree porcelain baseline captured before injecting: `?? .claude/skills/tlc-spec-driven/`, `?? .cursor/skills/tlc-spec-driven/`, `?? .windsurf/skills/tlc-spec-driven/`.

Test commands used per mutant:
- Go: `go test ./internal/sheet/template/...` or `go test ./internal/sheet/...`
- Dart: `flutter test <file>` from `desktop/`

| #    | File:line | Injected change | Test run | Killed? |
| ---- | --------- | --------------- | -------- | ------- |
| M1 | `internal/sheet/template/dag.go:610-622` | Remove sibling-zeroing in `NormalizeExclusiveUpdates` (leave both members on) | template pkg | ✅ Killed |
| M2 | `internal/sheet/template/dag.go:623-624` | Accept two non-zero members instead of returning the conflict error | template pkg | ✅ Killed |
| M3 | `internal/sheet/template/dag.go:244-249` | Remove the `<2` group drop (enforce a 1-member group) | template pkg | ✅ Killed |
| M4 | `internal/sheet/service.go:171-174` | Skip `dag.NormalizeExclusiveUpdates` in `PartialUpdate` | sheet pkg | ✅ Killed |
| M5 | `internal/sheet/template/seeders.go:348-349` | Untag `config.acUsesCon` | template pkg | ✅ Killed |
| M6 | `internal/sheet/template/dto.go:38` | Drop `omitempty` from the `exclusive_group` JSON tag | template pkg | ✅ Killed |
| M7 | `desktop/lib/engine/exclusive_control.dart:41` | Segmented option threshold `3 → 2` | `exclusive_control_test.dart` | ✅ Killed |
| M8 | `desktop/lib/engine/exclusive_control.dart:44` | Segmented label limit `30 → 29` (drop inclusive boundary) | `exclusive_control_test.dart` | ✅ Killed |
| M9 | `desktop/lib/engine/exclusive_control.dart:86-92` | Read-only `onSelectionChanged` made non-null | `exclusive_control_test.dart` | ✅ Killed |
| M10 | `desktop/lib/sheet/sheet_edit_provider.dart:90-93` | `setExclusive` sets only the selected member (no sibling zeroing) | `sheet_edit_provider_test.dart` | ✅ Killed |
| M11 | `desktop/lib/sheet/sheet_edit_provider.dart:89` | `setExclusive` always emits even when already active | `sheet_edit_provider_test.dart` | ✅ Killed |
| M12 | `desktop/lib/engine/field_grouping.dart:85` | Collapse a single-member group (`<2 → <1`) | `field_grouping_test.dart` | ✅ Killed |
| P1 | `internal/sheet/template/seeders.go:190-196` | Tag `stats.strength` + `stats.dexterity` with a second group `bogusGroup` | `go test ./internal/sheet/template/...` (re-run after `010895a`), initially `go test ./...` **with `DATABASE_URI` exported** | ✅ **Killed by T11** (was survived) |

**Sensor depth**: lightweight-to-full (13 manual behavior-level mutations across Go + Dart).
**Feature-behavior result**: 12/12 killed.
**Contract-coverage re-check (P1)**: 1/1 killed by T11 — 13/13 total.

P1 creates a real, observable fault: the fantasy_v1 API would advertise `exclusive_group:"bogusGroup"` on two `stats` leaves, and `buildExclusiveGroups` (`internal/sheet/template/dag.go:236`) would form a real exclusive group that zeroes `stats.strength`/`stats.dexterity` siblings on PATCH. Originally the entire Go suite stayed green; after T11 it dies at `internal/sheet/template/seeders_test.go:354` — `expected {"config.acUsesCon","config.acUsesDex"}`, `actual {…,"stats.dexterity":"bogusGroup","stats.strength":"bogusGroup"}`. Re-checked in a fresh detached worktree at `010895a`; the Dart fixture test also scopes to `config`, but the Go whole-tree assertion now closes the contract.

**Isolation**: scratch reverted per mutation, worktrees removed with `git worktree remove --force` (original sensor: `/tmp/opencode/lgn-105-verifier`; P1 re-check: `/tmp/opencode/lgn-105-reverify`, detached at `010895a`). The real tree's `git status --porcelain` was re-checked after each removal and matched its pre-sensor baseline (skill dirs + this Verifier's own `.specs` artifacts). No `git stash` was used.

---

## Code Quality

| Principle | Status |
| --------- | ------ |
| Minimum code / no scope creep | ✅ (layout `Expanded` fix in `desktop/lib/engine/sheet_form.dart:113-128` is a justified, tokens-only overflow fix for the preview) |
| Surgical changes | ✅ |
| Matches existing patterns/style | ✅ |
| Single control source invariant preserved | ✅ `desktop/lib/engine/sheet_form.dart:48-52` assert; `desktop/test/engine/sheet_form_test.dart:300-345` |
| Spec-anchored outcome check (asserted values match spec) | ✅ all 17 ACs |
| Per-layer coverage met (domain 1:1 ACs; routes happy+edge+error) | ✅ EXC-04…08 have unit + service + HTTP e2e (e2e verified with MongoDB up) |
| Every test maps to a spec AC / edge / done-when | ✅ |
| `make fmt && make lint && make vet` | ✅ 0 issues; `go vet` clean; `flutter analyze` "No issues found!" |

---

## Edge Cases

- [x] Derived/non-editable leaf in a group is excluded — `dag_test.go:1438,1440`.
- [x] Single editable member not enforced — `dag_test.go:1500-1504`.
- [x] Sibling absent from payload written `0` only when a sibling is turned on — `dag_test.go:1470-1481`.
- [x] Member set to zero may leave the group with no active member — `dag_test.go:1487-1491`, `sheet_form_test.dart:419-436`.
- [x] Absent `exclusive_group` parses null and renders normal controls — `template_model_test.dart:171-181`, `field_grouping_test.dart:227-239`.
- [x] Read-only with no active member renders none selected + disabled — `exclusive_control_test.dart:243-259`.
- [x] Label exactly at the 30-char limit stays segmented — `exclusive_control_test.dart:78-88`.
- [x] Editing a group rebuilds only the group/consumers, not the whole form — `sheet_screen_test.dart:329-447` (granular rebuild) + `:1038-1082` (group emission).

---

## Gate Check

- **Gate commands** (from `tasks.md`): `go test ./...` and `cd desktop && flutter analyze && flutter test`
- **Build**: `make fmt && make lint && make vet` — lint "0 issues", vet exit 0
- **Go**: 117 passed, 0 failed, 0 skipped when `DATABASE_URI` is exported (the 9 `TestIntegration_*` self-skip only without the repo `.env`; with the running `legnaris-mongo` container they run, incl. `TestIntegration_Sheets_ExclusiveOptions`, 0.06s).
- **Flutter**: 360 passed, 0 failed, 4 skipped (pre-existing `api_client_retry_test.dart` timing skips, unrelated to this feature).
- **Delta**: +17 new/changed test groups across the feature (Go: `dto_test.go`, `seeders_test.go` incl. T11 whole-tree assertion, `dag_test.go`, `service_test.go`, `integration_test.go`; Dart: 6 test files + 2 fixtures). No test deletions.

---

## Shortcuts and Deviations Found

1. **T8 provisional screen wiring corrected in T10** — T8 (`6c6650d`) wired `_exclusiveControl` with a `setPath` loop (two emissions); T10 (`fc775ea`) replaced it with the batch `setExclusive`. Final state satisfies EXC-13's single-emission rule. No violation, but T8 shipped a transiently non-conformant wiring (`desktop/lib/sheet/sheet_screen.dart:349`).
2. **`SheetSection` gained `rows` while keeping `leaves`** — both are live consumers: `desktop/lib/sheet/sheet_defaults.dart:18` uses `leaves`; `desktop/lib/engine/sheet_form.dart:92` uses `rows`. Not a dead field; no violation.
3. **T10 layout fix (`Expanded`) in `sheet_form.dart`** — presentational only, uses `AppSpacing`/`flex` tokens, preserves the single-control-source invariant and read-only behaviour (EXC-09/11). No spec violation.
4. **T10 edited `spec.md` traceability** (EXC-15/16 `Pending → Implementing`). Acceptable; this Verifier now flips all rows to Verified.
5. **T11 added after the first Verifier pass** — a fix for gap #1; it changes only the test file (`internal/sheet/template/seeders_test.go`) and `tasks.md`, no product code. Verified: P1 killed, whole-tree contract covered.

No `// SPEC_DEVIATION` marker was found in the diff.

---

## Ranked Gaps

None. The single earlier gap (whole-tree `exclusive_group` assertion) was fixed by `010895a` (T11) and the P1 mutant is killed on re-check.

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status |
| ----------- | --------------- | ---------- |
| EXC-01 … EXC-17 | Implementing | ✅ Verified |

Traceability rows in `spec.md` flipped to Verified.

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 17/17 ACs matched the spec outcome; whole-tree contract clause now covered (T11).
**Sensor**: 13/13 mutants killed (12 feature-behavior + P1 re-check).
**Gate**: Go 117 passed / 0 failed (with Mongo env); Flutter 360 passed / 4 skipped; analyzer + lint + vet clean.

**What works**: contract metadata (include/omit/copy), the seeded pair (whole-tree exactly), DAG index rules (editable-only, ≥2, sorted), normalization (zero siblings, conflict, off-leaves-siblings, single-member/derived exclusions), service 400 mapping + cascade recompute (unit + HTTP e2e), segmented/radio threshold incl. the inclusive 30-char boundary, read-only disabled rendering in form + preview, single-member degrade, batch/single-emission `setExclusive`, already-active no-op, default selection, GET-only edits, and the Dart decode.

**Issues found**: none outstanding. The earlier whole-tree coverage gap was fixed by `010895a` and is verified killed.

**Next steps**: none — feature is verified. (Lesson L-012 remains recorded from the first pass; no new lesson.)
