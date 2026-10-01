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

## Quarantined (failed when applied - ignore)

A confirmed lesson that recurred alongside failure. Kept for the maintainer to review.

_none_
