# Phase 1 — Data Model: Web UI Engine

The feature persists nothing and issues no request. The structures below are the client-side shapes the
engine derives and renders; the payload they read is the F2 template model
(`web/src/api/models/template.ts`) and the F2 sheet model (`web/src/api/models/sheet.ts`, `valueAtPath`)
and is not re-declared here. Every field traces to a functional requirement in [spec.md](./spec.md).

Naming: TypeScript types use `PascalCase`, values and functions `camelCase`; the backend's `snake_case`
keys never appear outside the F2 decoders.

---

## SheetField

One leaf of a section: its dotted path and the schema that decides how it renders (ENG-02, ENG-24).

| Field | Type | Rules |
|---|---|---|
| `path` | `string` | the dotted path from the top-level key to the leaf, e.g. `stats.dexterity`; unique within a template |
| `schema` | `FieldSchema` | the F2 leaf contract: `type`, `label`, `isEditable`, `defaultValue`, optional `formula`, `exclusiveGroup`, `optionLabel`, `columnLabel` |

**Invariants**: the path is built by joining the node keys in tree order and is what `valueAtPath`
reads and `setPathValue` writes (ENG-24). The schema is never mutated.

---

## SheetExclusiveGroup

One mutually exclusive set inside a section (ENG-05, ENG-14, ENG-15).

| Field | Type | Rules |
|---|---|---|
| `id` | `string` | the group identity advertised by the leaf contract (`exclusiveGroup`), non-empty |
| `options` | `SheetField[]` | the member leaves, in tree/API order, placed at the first member's position |

**Invariants**: members are grouped by `id` across the whole parent, not by adjacency (ENG-05). A group
with one member is not collapsed (ENG-05). Two leaves with the same non-empty `id` under one parent are
the same group.

---

## SheetRow

One render line of a section; exactly one kind is active (ENG-02, ENG-04, ENG-05).

| Kind | Fields | Rules |
|---|---|---|
| normal | `fields: [SheetField]` | one untagged leaf, no group metadata |
| exclusive | `fields: SheetField[]`, `exclusive: SheetExclusiveGroup` | every member of one group, replacing the members' independent controls |
| table | `fields: SheetField[]`, `group: { label: string }` | the group's label and one leaf per declared column, in declared order |

**Invariants**: a normal row's `fields` holds exactly one leaf; an exclusive row's `fields` equals its
group's `options`; a table row's `fields` preserves the declared column order and never includes an
unresolved column (ENG-04).

**Transitions**: none — rows are derived, never edited.

---

## SheetSection

One top-level key rendered as a block (ENG-01, ENG-02).

| Field | Type | Rules |
|---|---|---|
| `key` | `string` | the top-level field key |
| `label` | `string` | the schema/group label when non-empty, else the humanized key (ENG-03) |
| `leaves` | `SheetField[]` | every leaf beneath the key, flattened, in tree order |
| `rows` | `SheetRow[]` | the render rows derived from those leaves |

**Invariants**: one section per top-level key, in the order the service returned the keys (ENG-01); a
key with no leaf produces no section (ENG-02); `leaves` always carries the full flattened set even when
`rows` collapses a table group.

---

## SheetMark

The preview marking of one control or row (ENG-10, ENG-22).

| Value | Meaning | Base label |
|---|---|---|
| `modified` | the person's pending change touched this editable leaf | the row shows "Modified"; the "Calculated" label is absent |
| `recalculated` | the last successful dry-run recalculated this derived leaf | the row shows "Recalculated"; the "Calculated" label yields |
| `undefined` | no preview touched the row | a derived leaf shows its base "Calculated" label |

**Invariants**: a mark only tints — it never changes a value and never issues a request. The base
"Calculated" indicator is dropped exactly while a mark is present, including when the marked value is
empty (ENG-10, ENG-22). Read-only surfaces that never preview carry no mark.

---

## Draft

The values a sheet surface holds before anything is stored, addressed by dotted path (ENG-19, ENG-20,
ENG-21).

| Operation | Input | Output | Rules |
|---|---|---|---|
| `seedValue(schema)` | one leaf schema | `unknown` | `''` for text/string, `false` for bool, `schema.defaultValue` otherwise (ENG-19) |
| `seedDefaults(fields)` | the field tree | a fresh nested object | every leaf seeded under its dotted path, in section order (ENG-19) |
| `seedFromSheet(fields, data)` | tree + stored sheet object | a fresh nested object | `mergeCascade(seedDefaults(fields), data)` (ENG-20) |
| `mergeCascade(base, overrides)` | two nested objects | a fresh nested object | recurse where both hold a plain object, replace the leaf otherwise; never mutate; empty override returns a copy of the base (ENG-21) |
| `setPathValue(data, path, value)` | object + dotted path | (mutates the object it is given) | create intermediate objects as needed, overwrite the leaf (ENG-19) |

**Invariants**: precedence is per leaf — a stored value wins at its path, a schema leaf the stored
object omits keeps its default, a stored key absent from the schema is carried over (ENG-20). No
operation mutates an input except `setPathValue`, which is the builder `seedDefaults` owns and never
exposes to a caller's object (ENG-21).

---

## ControlSource

Whether the composition reads and reports values itself or delegates to injected builders; exactly one
is active (ENG-25, ENG-26).

| Mode | Active when | The composition… | The caller… |
|---|---|---|---|
| data | `data` and `onChanged` are given and the builders are not | reads each value with `valueAtPath` and reports `(path, value)` | owns nothing |
| builder | both `controlBuilder` and `exclusiveControlBuilder` are given and `data` is not | hands each row's control to the builder and reads no value | owns the per-field value subscription and the edit reporting |
| (mark seam) | `rowMarkBuilder` may be given in either mode | calls it with the row's paths and renders its result | owns the mark state |

**Invariants**: mixing the modes, or passing neither, is a construction error (ENG-25). The mark seam
is orthogonal — it never changes the control source (ENG-23). In builder mode the composition
subscribes to no value, so one edit can rebuild one control rather than the whole form (ENG-26,
`AD-001`).

---

## MessageKey additions

The engine's copy resolves through the F2/F3 catalog (`web/src/l10n/strings.ts`); no key is removed
(ENG-27).

| Key | Used by |
|---|---|
| `engine.calculated` | the derived leaf's base "Calculated" indicator (ENG-10) |
| `engine.none` | the exclusive control's "no choice" entry and hint (ENG-16) |
| `engine.modified` | the row label for a `modified` mark (ENG-22) |
| `engine.recalculated` | the row label for a `recalculated` mark (ENG-22) |

**Invariants**: no user-facing literal exists in `web/src/engine/`; every branch resolves a key through
`message()`, which is what SC-003 and SC-004 assert.
