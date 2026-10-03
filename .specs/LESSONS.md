# LESSONS - auto-maintained by scripts/lessons.py

> Machine-owned. Do NOT hand-edit. Changes are overwritten on the next `lessons.py` write.
> Canonical state lives in `.specs/lessons.json`. Edit lessons only via the script.
> promote_threshold=2 distinct features · window_days=45 · quarantine_threshold=2

## Confirmed (load these at Specify/Design)

Corroborated across multiple features. Safe to apply as guidance.

_none_

## Candidates (under observation - do NOT load as guidance yet)

Seen once or not yet corroborated. Tracked, not trusted.

### L-001 - Assert provider-driven widget values against a provider override with a distinct value, never a literal the widget could hardcode.
- signal: `surviving_mutant` · recurrence: 1 feature(s) · scope: `desktop` · harmful: 0
- features: front-end-phase-0-setup
- evidence: desktop/test/widget_test.dart:11 (desktop)
- last seen: 2026-09-29T23:53:10Z

### L-002 - Pin layout-conditional behavior at the exact derived threshold width in addition to above and below it, or off-by-one mutants survive
- signal: `surviving_mutant` · recurrence: 1 feature(s) · scope: `desktop/shell` · harmful: 0
- features: front-end-phase-2-catalog
- evidence: M2 (desktop/shell)
- last seen: 2026-09-30T17:57:30Z

### L-003 - When an AC requires loading and empty states to be distinct, pin the copy source (arb key) in the spec so the rendered-text assertion is anchored
- signal: `spec_precision_gap` · recurrence: 1 feature(s) · scope: `desktop/catalog` · harmful: 0
- features: front-end-phase-2-catalog
- evidence: CAT-08 (desktop/catalog)
- last seen: 2026-09-30T17:57:30Z

### L-004 - When a spec requires an error message to show inline, pin its placement relative to the data it annotates
- signal: `spec_precision_gap` · recurrence: 1 feature(s) · scope: `desktop/catalog` · harmful: 0
- features: front-end-phase-2-catalog
- evidence: CAT-19 (desktop/catalog)
- last seen: 2026-09-30T17:57:30Z

### L-005 - Seed the app ColorScheme explicitly at ThemeData construction — Material 3's default scheme is a purple baseline that tints every neutral and reads as the banned AI-SaaS aesthetic
- signal: `spec_deviation` · recurrence: 1 feature(s) · scope: `desktop/theme` · harmful: 0
- features: front-end-phase-2-catalog
- evidence: UAT T8 (1c886c3) (desktop/theme)
- last seen: 2026-09-30T19:10:10Z

### L-006 - Mount the app shell in a Scaffold — a bare Column leaves the window unpainted and drops unstyled Text into MaterialApp's 48px red _errorTextStyle fallback
- signal: `spec_deviation` · recurrence: 1 feature(s) · scope: `desktop/shell` · harmful: 0
- features: front-end-phase-2-catalog
- evidence: UAT T8 (1c886c3) (desktop/shell)
- last seen: 2026-09-30T19:10:10Z

### L-007 - Lay out fixed-height children under a stretch Row inside an Align that loosens the incoming tight height, or the child inflates past its row token
- signal: `spec_deviation` · recurrence: 1 feature(s) · scope: `desktop/shell` · harmful: 0
- features: front-end-phase-2-catalog
- evidence: UAT T8 (1c886c3) (desktop/shell)
- last seen: 2026-09-30T19:10:10Z

### L-008 - Pin the exact trigger condition in the spec when an edge-case bullet and the assumptions table describe the same condition at different precision.
- signal: `spec_precision_gap` · recurrence: 1 feature(s) · scope: `desktop/catalog` · harmful: 0
- features: front-end-phase-2-template-detail-dialog
- evidence: spec.md:132 (desktop/catalog)
- last seen: 2026-09-30T20:29:35Z

### L-009 - Assert the 'value when present' branch of a fallback rule, not only its empty/default path.
- signal: `spec_precision_gap` · recurrence: 1 feature(s) · scope: `desktop` · harmful: 0
- features: front-end-phase-3-sheet-rendering
- evidence: RND-04; spec.md:77; desktop/lib/engine/field_grouping.dart:31 (desktop)
- last seen: 2026-10-01T02:11:51Z

### L-010 - Give every layout boundary (min window width, max panel/control size) an explicit numeric value in the spec so its edge case is testable.
- signal: `spec_precision_gap` · recurrence: 1 feature(s) · scope: `desktop` · harmful: 0
- features: front-end-phase-3-sheet-preview
- evidence: spec.md:125 (min-width edge; validation.md spec-precision note) (desktop)
- last seen: 2026-10-01T14:22:57Z

### L-011 - Assert the exact rendered layout dimension, not a one-sided bound any in-window layout satisfies; pin the literal value in a token test too.
- signal: `surviving_mutant` · recurrence: 1 feature(s) · scope: `desktop` · harmful: 0
- features: front-end-phase-3-sheet-preview
- evidence: create_sheet_preview_test.dart narrow-window width assertion (iteration-2 survivor) (desktop)
- last seen: 2026-10-01T15:43:50Z

### L-012 - Assert a metadata key is absent from every leaf of a template, not only the section that sets it.
- signal: `surviving_mutant` · recurrence: 1 feature(s) · scope: `internal/sheet/template` · harmful: 0
- features: lgn-105-exclusive-options
- evidence: internal/sheet/template/seeders.go:190 (internal/sheet/template)
- last seen: 2026-10-01T23:19:44Z

### L-013 - Assert every field an update replaces, not just the entry count and one field.
- signal: `surviving_mutant` · recurrence: 1 feature(s) · scope: `desktop/library` · harmful: 0
- features: lgn-107-local-sheet-library
- evidence: desktop/lib/library/sheet_library_provider.dart:64-66 (mutant 7) (desktop/library)
- last seen: 2026-10-02T18:40:51Z

### L-014 - Feed a non-UTC timestamp to a serializer test so dropping the UTC conversion fails the suite.
- signal: `surviving_mutant` · recurrence: 1 feature(s) · scope: `desktop/library` · harmful: 0
- features: lgn-107-local-sheet-library
- evidence: desktop/lib/library/sheet_library_entry.dart:58 (mutant 8) (desktop/library)
- last seen: 2026-10-02T18:40:51Z

### L-015 - Prove durability by reloading a fresh store from a real temp directory, not by asserting a fake recorded the call.
- signal: `spec_precision_gap` · recurrence: 1 feature(s) · scope: `desktop/library` · harmful: 0
- features: lgn-107-local-sheet-library
- evidence: LIB-13 (desktop/library)
- last seen: 2026-10-02T18:40:51Z

### L-016 - Inject versioned-migration maps per store instance so a migration path is testable without mutating global state.
- signal: `spec_deviation` · recurrence: 1 feature(s) · scope: `desktop/library` · harmful: 0
- features: lgn-107-local-sheet-library
- evidence: desktop/lib/library/sheet_library_store.dart:53 (SPEC_DEVIATION) (desktop/library)
- last seen: 2026-10-02T18:40:51Z

### L-017 - Assert boundary inputs that normalize to empty (whitespace-only strings) separately from the empty case, so the no-op guard is proven for both.
- signal: `ac_gap` · recurrence: 1 feature(s) · scope: `desktop/widgets` · harmful: 0
- features: lgn-109-create-sheet
- evidence: desktop/test/catalog/create_sheet_dialog_test.dart:288 (desktop/widgets)
- last seen: 2026-10-03T15:45:57Z

## Quarantined (failed when applied - ignore)

A confirmed lesson that recurred alongside failure. Kept for the maintainer to review.

_none_
