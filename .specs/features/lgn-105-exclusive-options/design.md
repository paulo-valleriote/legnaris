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
    // OptionLabel is the short label used when this leaf is shown as an option
    // of its exclusive group (e.g. "Dexterity"). Empty falls back to Label.
    OptionLabel string `bson:"option_label,omitempty"`
}

// internal/sheet/template/dto.go
type FieldSchemaDTO struct {
    // ... existing fields ...
    ExclusiveGroup string `json:"exclusive_group,omitempty"`
    OptionLabel    string `json:"option_label,omitempty"`
}
```

Serialized for the pair:

```json
"acUsesDex": { "schema": { "type": "number", "label": "Armor class uses dexterity",
                "is_editable": true, "default": 1, "exclusive_group": "armorClassAbility",
                "option_label": "Dexterity" } },
"acUsesCon": { "schema": { "type": "number", "label": "Armor class uses constitution",
                "is_editable": true, "default": 0, "exclusive_group": "armorClassAbility",
                "option_label": "Constitution" } }
```

`omitempty` keeps every other leaf byte-identical to today, so the change is backward compatible:
old clients ignore the key; a client that never sees it renders the two independent toggles as before.

The second exclusive group is the spellcasting ability: the six `config.spellUses*` leaves carry
`exclusive_group: "spellcastingAbility"` (defaults all `0`). Every exclusive group renders through the
same dropdown control (increment 3). The free-text `spells.ability` leaf is removed — its label becomes
the group row label.

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

### Control (increment 3 — unified dropdown)

`desktop/lib/engine/exclusive_control.dart`:

```dart
class SheetExclusiveControl extends StatelessWidget {
  final SheetExclusiveGroup group;
  final String? selectedPath;      // null = none active
  final ValueChanged<String>? onSelect; // null = read-only
  // None is offered only when the group has no active default:
  //   _allowsNone = group.options.every((o) => o.schema.defaultValue == 0)
}
```

- Always a single dense `DropdownButton<String>` keyed `sheet-exclusive-<id>`; one
  `DropdownMenuItem` per option keyed `sheet-exclusive-option-<path>`.
- Items = `[None?] + options`; an option label is `optionLabel` when non-empty, else the member label.
- `None` (a sentinel value) is included iff `_allowsNone`; it clears the group. The dropdown `value`
  is the active path, or the `None` sentinel when nothing is active, or null when nothing can be.
- Read-only (`onSelect == null`) renders the same dropdown disabled.
- Height/radius/type scale come from `lib/theme/tokens.dart`; the `None` text is an l10n string.

`desktop/lib/engine/field_control.dart` (flags):

- The flag branch renders a compact `Checkbox` (key `sheet-checkbox-<path>`) instead of a `Switch`.
- Emission keeps the field type: `bool` → `true`/`false`, numeric `config.*` proxy → `1`/`0`.
- A null/absent value renders unchecked; read-only renders the checkbox disabled.

### Form

`desktop/lib/engine/sheet_form.dart`:

- The optional `exclusiveControlBuilder` signature is unchanged; edit mode uses it for exclusive rows,
  read-only/data mode builds `SheetExclusiveControl` directly, resolving the active option (or `None`)
  from `data` and disabling selection.
- Constructor assertion stays "both builders, or neither".

### Edit state

`desktop/lib/sheet/sheet_edit_provider.dart` — `setExclusive(String? selectedPath, Iterable<String> siblings)`:

- `selectedPath == null` (None) sets every member to `0`; otherwise sets `selectedPath = 1` and each
  sibling `= 0`, all in one copy-on-write state.
- If no affected path actually changes, returns without emitting (state instance stays identical, EXC-14).
- Records every changed path in `editedPaths`; one emission, so each control's `select` listener
  rebuilds at most once.

### Screen wiring

`desktop/lib/sheet/sheet_screen.dart` — passes `exclusiveControlBuilder` that:

- watches each option path through `provider.select(valueAtPath(...))` inside one `Consumer`,
- renders `SheetExclusiveControl(group, selectedPath, onSelect: (path) => notifier.setExclusive(
  path == noneSentinel ? null : path, siblings))`.

### Fixtures and preview

- `desktop/test/fixtures/{template_detail,templates}.json` gain `option_label` on the ac pair and the
  six `spellUses*` (and keep the existing `exclusive_group` tags).
- `create_sheet_preview` / `SheetForm` data mode renders the dropdown disabled with the current value
  (or `None`).

## Component map

| Layer | File | Change |
| ----- | ---- | ------ |
| Contract (Go) | `internal/sheet/template/template.go` | `ExclusiveGroup` + `OptionLabel` fields |
| Contract (Go) | `internal/sheet/template/dto.go` | `exclusive_group` / `option_label` JSON + `toDTO` copy |
| Seed (Go) | `internal/sheet/template/seeders.go` | tag ac pair + six `spellUses*`, set their `option_label`; remove `spells.ability` |
| DAG (Go) | `internal/sheet/template/dag.go` | build index + `NormalizeExclusiveUpdates` |
| Errors (Go) | `internal/sheet/template/errors.go` | `ExclusiveGroupConflictError` |
| Service (Go) | `internal/sheet/service.go` | call normalization + `translateDAGErr` case |
| Model (Dart) | `desktop/lib/api/models/template.dart` | parse `exclusive_group` + `option_label` |
| Grouping (Dart) | `desktop/lib/engine/field_grouping.dart` | collapse exclusive members |
| Control (Dart) | `desktop/lib/engine/exclusive_control.dart` | single dropdown control (+ None) |
| Control (Dart) | `desktop/lib/engine/field_control.dart` | flags render a checkbox |
| Form (Dart) | `desktop/lib/engine/sheet_form.dart` | exclusive rows + builder injection |
| State (Dart) | `desktop/lib/sheet/sheet_edit_provider.dart` | `setExclusive` (member or None) |
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
