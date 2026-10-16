# Contract: Section and row derivation

**Requirements**: ENG-01, ENG-02, ENG-03, ENG-04, ENG-05, ENG-06 | **Backing document**:
[data-model.md `SheetSection` / `SheetRow` / `SheetField` / `SheetExclusiveGroup`](../data-model.md)

**Module**: `web/src/engine/field_grouping.ts` — pure derivation, no React, no state, no I/O. It reads
the F2 field tree (`Record<string, FieldNode>` from `web/src/api/models/template.ts`) and returns the
sections the composition renders.

## Interface

| Symbol | Signature | Obligation |
|---|---|---|
| `buildSections` | `(fields: Record<string, FieldNode>) => SheetSection[]` | ENG-01, ENG-02, ENG-03, ENG-04, ENG-05 |
| `fieldDisplayName` | `(key: string, schema?: FieldSchema) => string` | ENG-03 |
| `groupDisplayName` | `(key: string, label?: string) => string` | ENG-03 |
| `fieldColumnName` | `(key: string, schema: FieldSchema) => string` | ENG-06 |
| `lastSegment` | `(path: string) => string` | ENG-03, ENG-06 |

The three names are pure and shared: the composition reuses `fieldDisplayName` for a row label and
`fieldColumnName` for a table header, so a name is derived the same way everywhere. All of them trim
their source label and fall back to `humanize(key)` only when the trimmed label is empty.

## Rules

1. **One section per top-level key, in arrival order** (ENG-01). The iteration order of `fields` is the
   order the service returned the keys; the derivation never sorts, never filters by name and never
   merges two keys into one section.
2. **Flattening** (ENG-02). Every leaf beneath a key becomes a `SheetField` whose `path` is the node
   keys joined by `.` in tree order, and whose `schema` is the leaf's own `FieldSchema`. A group is
   flattened recursively; `keys` in the path are the tree's keys, not the labels.
3. **Empty key is omitted** (ENG-02). A key whose subtree holds no leaf produces no section. A group
   whose only leaves sit under an empty sub-group is likewise dropped; the derivation never emits a
   section with an empty `leaves`.
4. **Top-level leaf is its own section** (ENG-02). A leaf at the root becomes a one-leaf section, keyed
   by its own key.
5. **Labels are schema-first, else humanized** (ENG-03). A field label is `schema.label` when it trims
   non-empty, otherwise `humanize(key)`; a group/section label is `label` when it trims non-empty,
   otherwise `humanize(key)`. No per-system literal, no hardcoded key→label map. `humanize` splits
   `_`/`-` and camel-case boundaries, lowercases, and capitalizes the first letter; an all-empty result
   returns the raw key.
6. **Declared-column group becomes one table row** (ENG-04). A group that declares a non-empty
   `columns` list and has at least one resolvable column is one `SheetRow` of kind `table`: the group's
   label plus one `SheetField` per declared column, **in declared order**. A column naming an absent or
   leaf-less child is skipped; when **no** declared column resolves, the group falls back to stacked
   rows instead of drawing an empty table.
7. **Column label and alignment** (ENG-06). A column is labeled `schema.columnLabel` when non-empty and
   `fieldDisplayName(lastSegment(path), schema)` otherwise. A `number`-typed column is right-aligned;
   every other column is left-aligned. Consecutive table rows with the same ordered column labels share
   one header (the composition, not the derivation, draws it).
8. **Exclusive grouping by identity, across the whole parent** (ENG-05). Every leaf sharing a non-empty
   `schema.exclusiveGroup` joins one `exclusive` row placed at the **first** member's position,
   regardless of adjacency. Grouping is by the group id across the whole section, not by adjacency or
   by depth. A group with a **single** member is **not** collapsed — it stays a normal row.
9. **Provenance of `leaves`** (ENG-02, ENG-04). `SheetSection.leaves` always carries the full flattened
   set (tables included), while `SheetSection.rows` may collapse a declared group into one table row or
   an exclusive group into one exclusive row. `rows` preserves the source order of first appearance.
10. **Rows are derived, never edited** (data-model invariants). No exported function mutates the
    `fields` input, a `FieldSchema`, or a returned section; each call builds fresh objects.

## Obligations

1. **ENG-01** — rendered sections are one-per-key in arrival order.
2. **ENG-02** — flattened dotted paths in tree order; empty group omitted; top-level leaf becomes a
   one-field section.
3. **ENG-03** — a label is the non-empty schema/group label, else a mechanical humanization of the key.
4. **ENG-04** — a declared-column group is one table row (group label + one cell per declared column,
   declared order); unknown/absent columns skipped; no resolvable column falls back to stacked rows.
5. **ENG-05** — same non-empty exclusive id ⇒ one exclusive row at the first member's position; a
   single-member group is not collapsed.
6. **ENG-06** — column label from `columnLabel` else the display name; numeric column right-aligned,
   any other left-aligned.

## Test obligations

- One case per row kind: a synthetic tree with a stacked leaf, a declared-column table and a
  multi-member exclusive group asserts the ordered row kinds and the first-member position.
- **Edge**: an empty field tree yields `[]` and does not throw.
- **Edge**: a group with no leaf (and a group whose only leaves sit under an empty sub-group) yields no
  section.
- **Edge**: a group declaring columns none of which resolve falls back to stacked rows, never an empty
  table; a group declaring an unknown column alongside a known one skips only the unknown.
- **Edge**: a single-member exclusive group stays a normal row (not collapsed).
- **Edge**: non-adjacent members of one exclusive id collapse into one row at the first member's
  position.
- **Happy path**: labels — a non-empty `label`/`columnLabel` wins; an empty one humanizes the key
  (`armorClassAbility` → `Armor class ability`).
- **Happy path**: `SheetSection.leaves` carries every flattened leaf even when a table collapses the
  rows, in tree order.
- **Happy path**: alignment — a numeric column is right-aligned, a text column left-aligned.
