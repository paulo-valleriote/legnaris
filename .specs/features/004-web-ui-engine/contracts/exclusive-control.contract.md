# Contract: Exclusive control

**Requirements**: ENG-14, ENG-15, ENG-16, ENG-17, ENG-18 | **Backing document**:
[data-model.md `SheetExclusiveGroup` / `SheetField`](../data-model.md) and
[research.md `R5`](../research.md)

**Module**: `web/src/engine/exclusive_control.tsx` — one controlled React component
(`SheetExclusiveControl`) plus two pure helpers. It renders **one** single-choice control for one
mutually exclusive group; the members never render as independent controls (ENG-14). It owns no domain
state.

## Interface

| Symbol | Signature | Obligation |
|---|---|---|
| `SheetExclusiveControl` | component: `{ group: SheetExclusiveGroup; selectedPath?: string; mark?: SheetMark; onSelect?: (signal: string) => void }` | ENG-14…ENG-18 |
| `exclusiveValueIsOn` | `(value: unknown) => boolean` | ENG-16, ENG-18 |
| `EXCLUSIVE_NONE_VALUE` | `const string = '__none__'` | ENG-16, ENG-18 |

`group` carries the id and the member leaves in API order (from the derivation). `selectedPath` is the
dotted path of the active member, or `undefined` when no member is active. `onSelect` reports either an
active member's dotted path or the `EXCLUSIVE_NONE_VALUE` sentinel meaning "clear the group"; a real
dotted path can never collide with the sentinel. An absent `onSelect` renders the control inert.

## Rules

1. **Single control** (ENG-14). A group renders as one native `<select>` (the platform single-choice
   control; `R5`), whose options are the group's members. The members are **not** drawn as independent
   toggles, and a member's own control is never rendered separately.
2. **Option label** (ENG-15). An option is labeled `schema.optionLabel` when it trims non-empty, and
   `fieldDisplayName(lastSegment(path), schema)` (the display name) otherwise. The fallback reuses the
   derivation's name helper, so labels never diverge.
3. **"None" entry rule** (ENG-16). The control offers a `message('engine.none')` entry **only when** no
   member carries a non-zero default (`group.options.every(o => o.schema.defaultValue === 0)`). A group
   with a default-active member does not offer it. When a default-active group has **no active member**
   (seeded before the group existed, or every member explicitly zeroed), the control shows the
   `message('engine.none')` label as its hint rather than rendering blank (ENG-16).
4. **Active member** (ENG-16, ENG-18). `exclusiveValueIsOn` is the one activity test: a value is active
   iff it is a non-zero `number`; a `boolean` is off (the backend contract is numeric 0/1 proxies). The
   control's value is the active member's path when one exists, the `EXCLUSIVE_NONE_VALUE` sentinel when
   nothing is active and the group allows none, and an empty/blank state only when nothing can be shown.
5. **Selection** (ENG-18). With an `onSelect`, selecting a member reports that member's dotted path;
   selecting the "None" entry reports `EXCLUSIVE_NONE_VALUE` (the clear-group signal). The control
   reports the signal verbatim — it does not itself mutate any draft.
6. **Inert without a handler** (ENG-17). With no `onSelect`, the `<select>` is `disabled`: the active
   member (or the "none" hint) is shown and nothing is selectable.
7. **Mark tint** (ENG-22, `R4`). When `mark` is set, the control carries the class `lg-field--marked`
   (the same `--lg-color-surface` tint the field control uses). The tint alters no selection.
8. **Accessible name** (SC-007). The `<select>` exposes the `combobox`/`listbox` role and an accessible
   name tied to its row label, so the rendering suite addresses it by role and name.
9. **No copy, no ad-hoc visual value** (ENG-27, ENG-28). The only strings are `engine.none` and the
   option labels (derived from the schema, never authored); every visual value is a `--lg-*` token.

## Obligations

1. **ENG-14** — members sharing a non-empty exclusive group render as one single-choice control; never
   as independent toggles.
2. **ENG-15** — an option is labeled by its non-empty `optionLabel`, else its display name.
3. **ENG-16** — the "none" entry exists only when no member has a non-zero default; a default-active
   group with no active member shows the "none" hint, not a blank.
4. **ENG-17** — no handler ⇒ inert (active member or hint shown, nothing selectable).
5. **ENG-18** — selecting a member reports its path; selecting "none" reports the clear-group signal.

## Test obligations

- **Happy path**: a three-member group renders one `select` (by role and accessible name) with three
  options, never three toggles.
- **Happy path**: selecting a member reports that member's dotted path; selecting the "none" entry
  reports `EXCLUSIVE_NONE_VALUE`.
- **Happy path**: an option with a non-empty `optionLabel` shows the option label; one with an empty
  `optionLabel` shows its display name.
- **Edge**: a group with every member's default `0` offers the "none" entry; a group with one member
  whose default is non-zero does not.
- **Edge**: a default-active group whose members are all inactive shows the "none" hint (not blank).
- **Edge**: a group with a single member is not collapsed (asserted with the derivation, re-asserted
  here as a stacked row, not this control).
- **Edge**: with `onSelect` absent, the `select` is disabled; selecting nothing reports nothing.
- **Edge**: `exclusiveValueIsOn` — a non-zero number is on, `0` is off, `true`/`null`/`"1"` are off.
- **Edge**: a marked group tints the control (class `lg-field--marked`).
