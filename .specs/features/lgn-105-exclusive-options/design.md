# LGN-105 — Design: Exclusive Groups

## Contract shape

`FieldSchema` gains one optional scalar; `FieldSchemaDTO` mirrors it.

```go
// internal/sheet/template/template.go
type FieldSchema struct {
    Type       string     `bson:"type"`
    Label      string     `bson:"label"`
    Formula    string     `bson:"formula"`
    DependsOn  [][]string `bson:"depends_on"`
    IsEditable bool       `bson:"is_editable"`
    Default    float64    `bson:"default"`
    // ExclusiveGroup names the mutually exclusive set this leaf belongs to.
    // Leaves sharing a non-empty group under the same parent may have at most
    // one non-zero value. Empty means "independent".
    ExclusiveGroup string `bson:"exclusive_group,omitempty"`
}

// internal/sheet/template/dto.go
type FieldSchemaDTO struct {
    // ... existing fields ...
    ExclusiveGroup string `json:"exclusive_group,omitempty"`
}
```

Serialized for the pair:

```json
"acUsesDex": { "schema": { "type": "number", "label": "Armor class uses dexterity",
                "is_editable": true, "default": 1, "exclusive_group": "armorClassAbility" } },
"acUsesCon": { "schema": { "type": "number", "label": "Armor class uses constitution",
                "is_editable": true, "default": 0, "exclusive_group": "armorClassAbility" } }
```

`omitempty` keeps every other leaf byte-identical to today, so the change is backward compatible:
old clients ignore the key; a client that never sees it renders the two independent toggles as before.

The second exclusive group is the spellcasting ability: the six `config.spellUses*` leaves carry
`exclusive_group: "spellcastingAbility"` (defaults all `0`). With six options the desktop uses its
radio fallback. The free-text `spells.ability` leaf is removed — its label becomes the group row label.

**Why metadata, not a select field:** see `context.md` Area 1. The formula
`combat_baseArmorClass + (modifiers_dexterityMod * config_acUsesDex) + (modifiers_constitutionMod * config_acUsesCon)`
is unchanged; exclusivity is enforced on the inputs so the multiplicands can never both be 1.

## Backend

### DAG index (build time)

`BuildDAG` already collects leaf schemas + paths. After `compileNodeFormulas`, build:

```
exclusiveGroups map[string][]string   // group id -> sorted editable leaf keys
```

- Only leaves with `IsEditable == true` and a non-empty `ExclusiveGroup` join a group.
- A group with fewer than two members is dropped (nothing to exclude / EXC-08).
- Members are sorted for deterministic errors and tests.

Stored on `DAG` and read only under `mu` (the graph is immutable after boot, same discipline as
`nodes`/`adjacencyList`).

### Normalization (update time)

```go
// NormalizeExclusiveUpdates returns a copy of updates with each touched exclusive
// group collapsed to at most one non-zero member. It returns an
// ExclusiveGroupConflictError when the payload itself sets two members on, and the
// input map unchanged when no exclusive group is touched.
func (d *DAG) NormalizeExclusiveUpdates(updates map[string]any) (map[string]any, error)
```

Algorithm, per group:

1. Collect the explicit updates for members of the group.
2. A member is **on** iff its value is numeric and non-zero. (A non-numeric value — string, bool,
   null, map — is rejected with `ExclusiveValueTypeError`: treating it as "off" would let a payload
   bypass the at-most-one rule. Numeric detection goes through the shared `maputils.ToFloat64`.)
3. If ≥2 members are on → return `ExclusiveGroupConflictError{Group, Keys}`.
4. If exactly one member is on → for every other member absent from `updates`, set it to `0`.
5. If none is on → leave the group untouched (turning a member off is allowed; EXC-06).

The service reaches the DAG through `NormalizeAndRecalculate`, which validates editability,
normalizes and recalculates under a **single** read-lock acquisition — `ValidateUpdateKeys` +
`NormalizeExclusiveUpdates` + `Recalculate` would otherwise take the lock three times per PATCH.
`NormalizeExclusiveUpdates`, `ValidateUpdateKeys` and `Recalculate` stay public (Create and the
unit tests use them) and delegate to lock-free cores.

### Service wiring

`ServiceImpl.PartialUpdate` (`internal/sheet/service.go`):

```
updates, err := maputils.FlattenPathKeys(cmd.Data)   // rejects depth > MaxNestingDepth
normalized, cascading, err := dag.NormalizeAndRecalculate(storedSheet.Data, updates)
```

`translateDAGErr` gains cases for `*ExclusiveGroupConflictError` and `*ExclusiveValueTypeError` →
`apperr.NewError(apperr.CodeInvalidRequest, ..., err)` (HTTP 400).

### Nesting depth cap (DoS guard)

`maputils.FlattenPathKeys`/`ToEngineEnv` (and their shared `flattenRecursive`) reject a map deeper
than `maputils.MaxNestingDepth` (5) with `maputils.ErrMaxDepthExceeded`; the service maps that to 400
`INVALID_REQUEST` ("sheet data is nested too deeply"). The walk is recursive over user-controlled
PATCH data, so without the cap a deeply nested body could exhaust the goroutine stack. Templates nest
at most three levels, so five leaves headroom. `Create` flattens template-defined data, which is
already shallow; `DeepCopy` still copies trusted stored data.

Change detection is unaffected: adding `sibling: 0` only cascades when the sibling actually
changed, and the member that was turned on always cascades (EXC-07).

## Desktop

### Model

`desktop/lib/api/models/template.dart` — `FieldSchema` gains `String? exclusiveGroup`, decoded from
`exclusive_group`. Absent/`null` → `null` (backward compatible).

### Grouping

`desktop/lib/engine/field_grouping.dart` — after flattening a section's leaves, group every leaf
sharing a non-empty `exclusiveGroup` by id into one logical row (not by adjacency), so the desktop
matches the backend, which groups by id across the whole parent:

```dart
typedef SheetExclusiveGroup = ({String id, List<SheetField> options});
typedef SheetRow = ({List<SheetField> fields, SheetExclusiveGroup? exclusive});
```

- A group with ≥2 members becomes one exclusive row, rendered at the position of its first member,
  even when an untagged leaf sits between two members.
- A single-member group is not collapsed (EXC-12): the member stays its own normal row.
- Order within `options` follows tree/API order.

### Control

New `desktop/lib/engine/exclusive_control.dart`:

```dart
class SheetExclusiveControl extends StatelessWidget {
  final SheetExclusiveGroup group;
  final String? selectedPath;      // null = none active
  final ValueChanged<String>? onSelect; // null = read-only
  // constants:
  //   _segmentedMaxOptions = 3
  //   _segmentedMaxLabelLength = 30   (inclusive)
}
```

- `_useSegmented` = `group.options.length <= 3 && every label.length <= 30`.
- Segmented branch: Material `SegmentedButton<String>` keyed `sheet-exclusive-<id>`, one segment per
  option keyed `sheet-exclusive-option-<path>`, `selected` = the active path (empty when none),
  `onSelectionChanged` null when `onSelect == null` (read-only).
- Radio fallback: a vertical group of `Radio` rows with the same keys, disabled when `onSelect == null`.
- Height/type scale come from `lib/theme/tokens.dart`; labels come from the member `label`.

### Form

`desktop/lib/engine/sheet_form.dart`:

- New optional `exclusiveControlBuilder`: `(BuildContext, SheetExclusiveGroup, String?, ValueChanged<String>?) -> Widget`.
- Edit mode (injected builders) uses it for exclusive rows; read-only/data mode builds
  `SheetExclusiveControl` directly, selecting the active option from `data` and disabling selection.
- Constructor assertion extends to "both builders, or neither" so the control source stays singular
  (the existing `controlBuilder`/`data` invariant).

### Edit state

`desktop/lib/sheet/sheet_edit_provider.dart` — new `setExclusive(String selectedPath, Iterable<String> siblings)`:

- Builds the next state once; sets `selectedPath = 1` and each sibling `= 0`.
- If no affected path actually changes (the option was already active), returns without emitting, so
  the state instance stays identical (EXC-14).
- Records every changed path in `editedPaths`; one emission, so each control's `select` listener
  rebuilds at most once.

### Screen wiring

`desktop/lib/sheet/sheet_screen.dart` — passes `exclusiveControlBuilder` that:

- watches each option path through `provider.select(valueAtPath(...))` inside one `Consumer`
  (rebuilds only when a member value changes),
- renders `SheetExclusiveControl(group, selectedPath, onSelect: (path) => notifier.setExclusive(path, siblings))`.

### Fixtures and preview

- `desktop/test/fixtures/{template_detail,templates}.json` gain
  `"exclusive_group":"armorClassAbility"` on the two ac schemas (frozen shape).
- `create_sheet_preview` / `SheetForm` data mode renders the group disabled with the active option.

## Component map

| Layer | File | Change |
| ----- | ---- | ------ |
| Contract (Go) | `internal/sheet/template/template.go` | `ExclusiveGroup` field |
| Contract (Go) | `internal/sheet/template/dto.go` | `exclusive_group` JSON + `toDTO` copy |
| Seed (Go) | `internal/sheet/template/seeders.go` | tag the ac pair and the six `spellUses*`; remove `spells.ability` |
| DAG (Go) | `internal/sheet/template/dag.go` | build index + `NormalizeExclusiveUpdates` |
| Errors (Go) | `internal/sheet/template/errors.go` | `ExclusiveGroupConflictError` |
| Service (Go) | `internal/sheet/service.go` | call normalization + `translateDAGErr` case |
| Model (Dart) | `desktop/lib/api/models/template.dart` | parse `exclusive_group` |
| Grouping (Dart) | `desktop/lib/engine/field_grouping.dart` | collapse exclusive members |
| Control (Dart) | `desktop/lib/engine/exclusive_control.dart` | segmented/radio control |
| Form (Dart) | `desktop/lib/engine/sheet_form.dart` | exclusive rows + builder injection |
| State (Dart) | `desktop/lib/sheet/sheet_edit_provider.dart` | `setExclusive` |
| Screen (Dart) | `desktop/lib/sheet/sheet_screen.dart` | wire the builder |

## Risks and mitigations

| Risk | Mitigation |
| ---- | ---------- |
| Non-numeric value on an exclusive member misread as "on" | Numeric-only "on" helper; non-numeric is treated as off; DAG groups only numeric editable leaves |
| Nested DAG lock (deadlock) | `NormalizeExclusiveUpdates` takes `mu` once and is called sequentially, never while another lock is held |
| Double-count regression in `armorClass` | EXC-07 service test asserts the recomputed derived value after normalization |
| Over-eager grouping of independent flags | Only leaves explicitly tagged join a group; `spellUses*`/`save*Prof`/`skill*Prof` are untagged |
| Desktop rebuild regression | Batch emission + per-path `select`; a granularity identity test guards it |
| Fixture drift | Fixtures updated to the frozen shape in the same task as the contract tests |
