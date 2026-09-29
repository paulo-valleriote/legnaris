# AGENTS.md — internal/sheet/template (DAG, Registry, Templates)

## Key Concepts

- **Template** — code-defined structure (fields, formulas, deps). Immutable by ID (`fantasy_v1`, `fantasy_v2`...).
- **DAG** — compiled dependency graph with topological order. Built once at boot per template and immutable afterwards:
  - Private (never embedded) mutex
  - Single read-lock acquisition per `Recalculate` and per `ValidateUpdateKeys`
  - `GetNode` returns value copies
- **Sheet** — `map[string]any` document linked to a template. Base fields editable; derived fields computed.
- **Recalculation** — `DAG.Recalculate(original, updates)` returns only changed derived fields (cascading).

## Gotchas

- **Templates are code, not DB** — no `template` collection in Mongo; templates live in the in-memory registry
- **DAG keys are flat path keys** (`stats.dexterity`) while engine env uses underscores (`stats_dexterity`) — field names validated at `BuildDAG` (LGN-52): every segment must match `^[A-Za-z][A-Za-z0-9]*$`, so neither `.` nor `_` can appear in a name and the two key encodings can never collide (regression tests: `TestBuildDAG_FieldNameValidation` and LGN-41 case in `TestBuildDag_EdgeCases`)
- **Updates are validated by the DAG itself** — the sheet service flattens the PATCH payload and calls `DAG.ValidateUpdateKeys` before `Recalculate` (LGN-66): unknown key or group prefix → `UnknownFieldError` (400 `UNKNOWN_FIELD`), derived field → `NotEditableError` (400 `INVALID_REQUEST`). Editability is deliberately *not* enforced inside `Recalculate`, because `Create` seeds a sheet with every field, derived ones included
- **Template IDs are slugs** — `FetchTemplate` rejects anything outside `^[a-z][a-z0-9_]*$` with `ErrTemplateIDInvalid` *before* the registry lookup (LGN-86 follow-up), so junk gets 400 instead of 404 on `GET /templates/:id`; the sheet service calls `FetchTemplate` first in `Create` and translates the typed error itself, so `POST /sheets` behaves the same
- **Registry and DAGs are frozen after boot** — `Registry.List()`/`FetchByID()` return deep copies (registry clones `DefaultTemplates()` on ingest), DAGs resolved per ID via `Registry.DAG(id)` through `sheet.DAGProvider` interface (no `DAGs()` map accessor), and `DAG` guards state with unexported mutex: never acquire twice — read through `HasNode`/`GetNode`/`GetChildren`/`ValidateUpdateKeys` (each takes the lock itself), or stay under single lock `Recalculate` takes (recursive RLock deadlocks with queued writer, regression test `TestRecalculate_ValidationRunsUnderSingleReadLock`)
- **PATCH body** uses nested map (`{"data":{"stats":{"dexterity":12}}}`), flattened internally to DAG dot-keys
- **Change detection (LGN-59)** — `Recalculate` only cascades the keys `changedUpdates` says really changed, compared with `maputils.Equal`'s *relative* epsilon (1e-9): rounding/type coercion cannot start a cascade, but the absolute slack grows with magnitude (up to 1.0 at 1e9) and integers above 2⁵³ collapse in `float64`, so `Equal` is unsuitable for fields needing exact large values (currency, XP). A PATCH whose values are unchanged returns `{}` and recomputes nothing: stale derived values do not self-heal on a no-op update.

## Template Errors

- Declare own typed errors (sentinels or custom types) — **do NOT import `apperr`**
- Wrap with `fmt.Errorf("...: %w", err)`
- Service layer translates to `apperr` at boundary
- `ErrTemplateIDNotProvided` (missing template ID, LGN-66) and `ErrTemplateIDInvalid` (malformed slug, LGN-86 follow-up) are translated to 400s at the boundary: `internal/app.templateError` handles the `/templates` routes and the sheet service's `translateTemplateErr` handles `POST /sheets` — the same boundary translation `shared/json` decoding errors get