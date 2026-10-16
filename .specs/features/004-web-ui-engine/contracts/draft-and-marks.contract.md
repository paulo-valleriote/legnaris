# Contract: Draft derivation and preview marks

**Requirements**: ENG-19, ENG-20, ENG-21, ENG-22, ENG-23, ENG-24, ENG-25, ENG-26 | **Backing
document**: [data-model.md `Draft` / `SheetMark` / `ControlSource`](../data-model.md) and
[research.md `R2` / `R6`](../research.md)

**Modules**: `web/src/engine/sheet_defaults.ts` (pure derivation and merge),
`web/src/engine/sheet_mark.ts` (the mark contract), `web/src/engine/sheet_form.tsx` (the composition and
its seams). Nothing persists; nothing requests; no state lives in the engine.

## Draft derivation

| Operation | Signature | Rules |
|---|---|---|
| `seedValue` | `(schema: FieldSchema) => unknown` | `''` for `text`/`string`, `false` for `bool`, else `schema.defaultValue` (ENG-19) |
| `seedDefaults` | `(fields: Record<string, FieldNode>) => Record<string, unknown>` | every leaf seeded under its dotted path, in **section order** (ENG-19) |
| `seedFromSheet` | `(fields, data: Record<string, unknown>) => Record<string, unknown>` | `mergeCascade(seedDefaults(fields), data)` (ENG-20) |
| `mergeCascade` | `(base, overrides) => Record<string, unknown>` | recurse where both sides hold a plain object at a key, replace the leaf otherwise; fresh result, inputs untouched; empty `overrides` returns a copy equal to `base` (ENG-21) |
| `setPathValue` | `(data, path: string, value: unknown) => void` | create intermediate objects as needed, overwrite the leaf (used only inside `seedDefaults`, never on a caller's object) |

1. **Seeded draft** (ENG-19). `seedDefaults` walks `buildSections(fields)` and, for every leaf in
   `section.leaves`, sets its seed under its dotted path. A text leaf starts empty, a boolean starts
   `false`, every other type takes its (numeric) `defaultValue` — a text leaf is **not** seeded with the
   JSON `default`, which stays numeric for every type.
2. **Overlay precedence** (ENG-20). For `seedFromSheet`, the merge is **key-driven, not
   schema-driven**: a stored value wins at its dotted path; a schema leaf the stored object omits keeps
   its schema default; a stored key absent from the schema is carried over unchanged (so a sheet
   survives a template that gains and loses leaves).
3. **Incremental cascade** (ENG-21). `mergeCascade` is the same function the sheet-cycle feature applies
   after `PATCH` (`AD-006`): where both sides hold a plain object at a key it recurses, so a cascade
   that changes one leaf preserves its siblings; anywhere else it replaces the leaf. It returns a fresh
   tree, mutates **neither** input, and returns a copy equal to the base for an empty `overrides`.

## Marks

| Value | Meaning | Row label |
|---|---|---|
| `'modified'` | the person's pending change touched this editable leaf | `message('engine.modified')` |
| `'recalculated'` | the last successful dry-run computed this derived leaf | `message('engine.recalculated')` |
| `undefined` | no preview touched the row | a derived leaf shows the base "Calculated" label |

4. **Mark tints, labels a row, changes nothing** (ENG-22). A `SheetMark` (`'modified'` / `'recalculated'`)
   is passed to a control as `mark`, which tints it (`lg-field--marked`) and is passed to the row, which
   labels it with the matching copy. A row carrying a mark is labeled **even when its value is empty**.
   The base "Calculated" indicator is dropped exactly while a mark is present. A mark never changes a
   value and never issues a request.

## Composition and its sources

`SheetForm` (`web/src/engine/sheet_form.tsx`) renders the derived sections and rows and takes **exactly
one** control source (ENG-25, `R2`):

| Mode | Active when | The composition… | The caller… |
|---|---|---|---|
| data | `data` and `onChanged` given, builders not | resolves each value with `valueAtPath(data, path)` and reports `(path, value)` (ENG-24) | owns nothing |
| builder | `controlBuilder` **and** `exclusiveControlBuilder` given, `data` not | hands each row's control to the builder and reads **no** value (ENG-26) | owns the per-field subscription and edit reporting (`AD-001`) |

| Seam | Signature | Obligation |
|---|---|---|
| `controlBuilder` | `(field: SheetField) => ReactNode` | replaces one leaf's control (ENG-25, ENG-26) |
| `exclusiveControlBuilder` | `(group: SheetExclusiveGroup, selectedPath?: string, onSelect?) => ReactNode` | replaces one exclusive row's control (ENG-25, ENG-26) |
| `rowMarkBuilder` | `(paths: readonly string[]) => ReactNode` | optional, orthogonal in either mode (ENG-23) |

5. **Exactly one source** (ENG-25). Passing both sources, or neither, is a **construction error**
   (a thrown Error or an asserted invariant) — never a silently half-rendered form.
6. **Data mode** (ENG-24). The composition renders the derived sections/rows with each row labeled,
   resolves every control value by its dotted path, and reports an edit as that path with its new value.
7. **Builder mode** (ENG-26). With builders injected, the composition hands each row's control to the
   builder and **subscribes to no value itself**, so one edit can rebuild one control rather than the
   whole form — the web expression of `AD-001`.
8. **Mark seam** (ENG-23). When `rowMarkBuilder` is given, the composition calls it with **every dotted
   path the row covers** (a single leaf, or all members of an exclusive group) and renders its result;
   the composition owns no mark state, and the seam never changes the control source (it is orthogonal
   to data vs builder mode).
9. **Copy and tokens** (ENG-27, ENG-28). Every string the composition renders resolves through
   `message()`; every visual value comes from a `--lg-*` token via `engine.css`.

## Obligations

1. **ENG-19** — the empty draft holds one seed per leaf under its dotted path, in section order:
   empty text, `false`, else the numeric default.
2. **ENG-20** — per-leaf precedence: stored value wins, omitted schema leaf keeps its default, unknown
   stored key carried over.
3. **ENG-21** — recursive where both sides object, replace otherwise, fresh result, no input mutated,
   empty cascade returns the base unchanged.
4. **ENG-22** — a `mark` tints the control and labels the row (even when the value is empty); the base
   "Calculated" label yields.
5. **ENG-23** — `rowMarkBuilder` receives every path the row covers; the composition owns no mark state.
6. **ENG-24** — data mode resolves values by path and reports `(path, value)`.
7. **ENG-25** — exactly one control source; any other combination is a construction error.
8. **ENG-26** — builder mode hands each row's control to the builder and reads no value.

## Test obligations

- **Happy path**: `seedDefaults` over a synthetic tree yields exactly one value per leaf, in section
  order, with the type-correct seed (empty text, `false` bool, numeric default).
- **Happy path**: `seedFromSheet` — a stored value wins at its path, an omitted schema leaf keeps its
  default, an unknown stored key is carried over.
- **Edge**: `mergeCascade` with an empty override returns a value equal to the base and mutates neither
  input; a nested override merges only its leaf and preserves its siblings.
- **Edge**: `setPathValue` creates missing intermediate objects and overwrites an existing leaf.
- **Happy path**: the composition in data mode resolves each value by path and reports an edit as
  `(path, value)`; the row label is derived by the section contract.
- **Edge**: passing both `data` and the builders (and passing neither) is a construction error, not a
  half-rendered form.
- **Happy path**: in builder mode the injected `controlBuilder` is consulted once per leaf, the default
  data-driven control is not rendered, and the composition subscribes to no value.
- **Happy path**: `rowMarkBuilder` receives the single leaf path for a normal row and **all** member
  paths for an exclusive row.
- **Edge**: a row marked `modified` / `recalculated` whose value is empty still shows the tint and the
  matching label; a derived marked row drops the "Calculated" label.
- **Source scan**: `web/src/engine/` holds no literal hex/px and no user-facing literal string; every
  referenced catalog key (`engine.calculated`, `engine.none`, `engine.modified`,
  `engine.recalculated`) exists, so removing one turns the suite red (SC-004).
