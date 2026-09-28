# AGENTS.md — internal/sheet/template (DAG, Registry, Templates)

## Key Concepts

- **Template** — code-defined structure (fields, formulas, deps). Immutable by ID (`fantasy_v1`, `fantasy_v2`...).
- **DAG** — compiled dependency graph with topological order. Built once at boot per template and immutable afterwards:
  - Private (never embedded) mutex
  - Single read-lock acquisition per `Recalculate`
  - `GetNode` returns value copies
- **Sheet** — `map[string]any` document linked to a template. Base fields editable; derived fields computed.
- **Recalculation** — `DAG.Recalculate(original, updates)` returns only changed derived fields (cascading).

## Gotchas

- **Templates are code, not DB** — no `template` collection in Mongo; templates live in the in-memory registry
- **DAG keys are flat path keys** (`stats.dexterity`) while engine env uses underscores (`stats_dexterity`) — field names validated at `BuildDAG` (LGN-52): every segment must match `^[A-Za-z][A-Za-z0-9]*$`, so neither `.` nor `_` can appear in a name and the two key encodings can never collide (regression tests: `TestBuildDAG_FieldNameValidation` and LGN-41 case in `TestBuildDag_EdgeCases`)
- **PartialUpdate validates editability** against DAG before recalc — non-editable fields in payload = 400
- **Registry and DAGs are frozen after boot** — `Registry.List()`/`FetchByID()` return deep copies (registry clones `DefaultTemplates` on ingest), DAGs resolved per ID via `Registry.DAG(id)` through `sheet.DAGProvider` interface (no `DAGs()` map accessor), and `DAG` guards state with unexported mutex: never acquire twice — read through `HasNode`/`GetNode`/`GetChildren`, or stay under single lock `Recalculate` takes (recursive RLock deadlocks with queued writer, regression test `TestRecalculate_ValidationRunsUnderSingleReadLock`)
- **PATCH body** uses nested map (`{"data":{"stats":{"dexterity":12}}}`), flattened internally to DAG dot-keys
- **Change detection (LGN-59)** — `Recalculate` only cascades the keys `changedUpdates` says really changed, compared with `maputils.Equal`'s *relative* epsilon (1e-9): rounding/type coercion cannot start a cascade, but the absolute slack grows with magnitude (up to 1.0 at 1e9) and integers above 2⁵³ collapse in `float64`, so `Equal` is unsuitable for fields needing exact large values (currency, XP). A PATCH whose values are unchanged returns `{}` and recomputes nothing: stale derived values do not self-heal on a no-op update.

## Template Errors

- Declare own typed errors (sentinels or custom types) — **do NOT import `apperr`**
- Wrap with `fmt.Errorf("...: %w", err)`
- Service layer translates to `apperr` at boundary