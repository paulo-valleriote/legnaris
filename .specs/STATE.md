# STATE

## Decisions

### AD-001
- **Decision**: Sheet edit state lives in an autoDispose Riverpod family keyed by template id (`sheetEditProvider`); each field control subscribes to its own dotted path via `select`, and the screen never watches the edit state directly (dirty reads happen at tap time; the marker is its own `Consumer`).
- **Reason**: Riverpod `select` only rebuilds listeners whose selected value changed, so one edit rebuilds exactly one control; a screen-level watch would rebuild the whole form and defeat the LGN-102 granularity requirement.
- **Trade-off**: `SheetForm` grew a `controlBuilder` injection point (the engine stays Riverpod-free but has two control sources), and dirty-aware UI must live in isolated `Consumer` widgets rather than the screen build.
- **Scope**: desktop/ — Screen 2 and any future surface rendering schema-driven fields with local edits (Phase 4's dirty + Save builds on this same state).
- **Date**: 2026-10-01
- **Status**: active

### AD-002
- **Decision**: Mutually exclusive schema options are expressed as an opt-in scalar `exclusive_group` on the leaf `FieldSchema` (json/bson `exclusive_group`, omitted when empty), not as a `select`/enum field. Leaves sharing a non-empty group under the same parent may have at most one non-zero value; the DAG normalizes PATCH updates (turns one member on → zeroes its siblings; two members on in one payload → typed 400).
- **Reason**: Keeping the 0/1 proxy leaves preserves the anti-copyright multiplier strategy and every existing sheet; a select field would rewrite the numeric formula environment, `SeedValue`/`default`, and require a data migration. `one_of` was rejected as a name because it collides with JSON Schema's array-of-schemas keyword and promises "exactly one" while the contract is "at most one".
- **Trade-off**: Exclusivity is a per-leaf tag rather than a structural group node, so it cannot carry its own display label; group/label humanization stays a follow-up. The desktop must collapse tagged siblings into one control, and any client that ignores the tag still renders independent toggles (safe but non-ideal).
- **Scope**: backend `internal/sheet/template` + `internal/sheet` (contract, DAG, PATCH); desktop `lib/engine` + `lib/sheet` (grouping, exclusive control, edit state, screen).
- **Date**: 2026-10-01
- **Status**: active

## Handoff

- **Feature**: `lgn-105-exclusive-options` (.specs/features/lgn-105-exclusive-options)
- **Phase / Task**: Execute complete — T1–T11 done; Verifier PASS
- **Completed**: Specify + Design + Tasks + Execute (T1–T11); AD-002 recorded; Verifier PASS (report in `.specs/features/lgn-105-exclusive-options/validation.md`; gap #1 fixed by T11 `010895a`)
- **In-progress** (file:line): none
- **Next step**: open the PR for `feat/lgn-105-exclusive-options`
- **Blockers**: none
- **Uncommitted files**: this STATE; untracked skill dirs (`.claude/`, `.cursor/`, `.windsurf/` — never commit)
- **Branch**: feat/lgn-105-exclusive-options
- **Prior feature**: `lgn-102-granular-rebuild` — COMPLETE (Verifier PASS, report in `.specs/features/lgn-102-granular-rebuild/validation.md`)
