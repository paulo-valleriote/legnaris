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

## Quarantined (failed when applied - ignore)

A confirmed lesson that recurred alongside failure. Kept for the maintainer to review.

_none_
