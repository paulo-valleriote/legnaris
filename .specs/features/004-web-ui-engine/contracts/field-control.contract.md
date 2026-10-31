# Contract: Field control

**Requirements**: ENG-07, ENG-08, ENG-09, ENG-10, ENG-11, ENG-12, ENG-13, ENG-29 | **Backing
document**: [data-model.md `SheetField` / `SheetMark`](../data-model.md) and
[research.md `R3` / `R4` / `R11`](../research.md)

**Module**: `web/src/engine/field_control.tsx` — one controlled React component (`SheetFieldControl`).
It owns no domain state: the value arrives as a prop and every edit leaves through a callback. The only
local state is the numeric input's display text, which is presentational.

## Interface

| Prop | Type | Obligation |
|---|---|---|
| `path` | `string` | the dotted path, e.g. `stats.dexterity`; used for the accessible name key and the `config` proxy test |
| `schema` | `FieldSchema` | decides which control is drawn (ENG-07…ENG-11) |
| `value` | `unknown` | the stored value, as held by the caller |
| `mark` | `SheetMark \| undefined` | the last preview's marking of this leaf, tints the control (ENG-22) |
| `onChanged` | `((value: unknown) => void) \| undefined` | reports an edit; **absent** ⇒ the control renders inert (ENG-13) |

## Control per schema

| Condition | Control (rendered) | Reported value |
|---|---|---|
| `!schema.isEditable` | the formatted value, read-only, marked "Calculated" (`message('engine.calculated')`) with `schema.formula` exposed on hover when declared (ENG-09, ENG-10) | none (read-only) |
| `schema.type === 'bool'` | an on/off toggle (a checkbox) | `boolean` `true`/`false` (ENG-08) |
| first path segment is `config` | an on/off toggle (a checkbox) | `number` `1`/`0` (ENG-08) |
| `schema.isEditable && schema.type === 'number'` | a numeric text input (`inputMode="decimal"`) | a finite `number`, or `null` when cleared (ENG-07, ENG-12) |
| `schema.isEditable && (type === 'text' \|\| 'string')` | a text input | the typed `string` (ENG-07) |
| editable, unrecognized type | the formatted value, read-only, **without** the "Calculated" label (ENG-11) | none |

The ordering is significant: non-editable first, then toggle, then numeric, then text, then the
read-only fallback for an editable leaf whose type is unrecognized. A config proxy is decided by the
**first** path segment alone (`config`), which is the backend's 0/1 proxy rule, not a per-system
literal (ENG-08).

## Rules

1. **Numeric input, finite guard** (ENG-12). The control draws a **text** input with
   `inputMode="decimal"` (never `type="number"`, which would hide the guard inside the browser). The
   display text is held locally. On each change: an empty value reports a cleared value (`null`); a text
   that can never resolve to a finite number (`Number(text)` is `NaN`, an infinity, or overflows) leaves
   the stored value and the shown text unchanged; any text the platform resolves to a finite number
   reports that number (a negative zero reports `0`). A sign or point typed so far (`-`, `.`, `-.`) is
   kept as display text and reports nothing; leaving the field with only such a pending text restores
   the text of the last reported value. A sign toggle, shown while the field has focus, flips the
   leading minus for keyboards without one. The desktop's general partial-token retention is **not**
   ported (Clarifications, 2026-10-16, amended 2026-10-30; [research.md `R3`](../research.md)).
2. **Toggle conversion** (ENG-08). A `bool` leaf reports `true`/`false`; a `config.*` leaf reports
   `1`/`0`. A stored value counts as on when it is `true` or a non-zero number.
3. **Read-only value** (ENG-09). A non-editable leaf renders its value as text with no editor. Empty or
   absent stored value renders an empty display and never throws.
4. **"Calculated" indicator and formula** (ENG-10). A non-editable leaf is marked with
   `message('engine.calculated')` and, when `schema.formula` is declared, exposes it on hover (a
   `title`/tooltip). The indicator is **dropped** while `mark` is set — the mark's row label takes its
   place (ENG-10, ENG-22).
5. **Unknown type degrades** (ENG-11). A leaf of an unrecognized type renders the read-only value, with
   **no** "Calculated" label and no fabricated editor, and never throws. This applies both to an
   unrecognized non-editable type and to an editable leaf whose type matches no editable branch.
6. **Inert without a handler** (ENG-13). With no `onChanged`, every control is non-interactive: the
   inputs are `disabled`, the toggle is `disabled`, the read-only value is already inert. With a
   handler, an edit reports the value from the table above.
7. **Mark tint** (ENG-22, `R4`). When `mark` is set, the control carries the class `lg-field--marked`
   whose tint is `--lg-color-surface` (an existing token with a declared contrast pair). The tint alters
   no value and issues no request.
8. **Text, never markup** (ENG-29, `R11`). Values and labels are rendered as React text children only:
   no `dangerouslySetInnerHTML`, no `innerHTML`, no markup interpolation. A value of `<script>` appears
   as the literal string and creates no element.
9. **Accessible name** (SC-007). Every control exposes an accessible role (a textbox, a checkbox, an
   output/`status` for the read-only value) and an accessible name tied to its row label (rendered by
   the composition) or its column header, so a rendering test can address it by role and name.
10. **No copy, no ad-hoc visual value** (ENG-27, ENG-28). Every string resolves through
    `message()`; every visual value comes from a `--lg-*` token via `engine.css`.

## Obligations

1. **ENG-07** — an editable number leaf is a numeric input, an editable text leaf a text input.
2. **ENG-08** — a `bool` or `config.*` leaf is an on/off toggle reporting `true`/`false` or `1`/`0`.
3. **ENG-09** — a non-editable leaf is its formatted value, read-only.
4. **ENG-10** — a derived leaf is marked "Calculated" with its formula on hover; the mark yields while a
   preview mark is present.
5. **ENG-11** — an unrecognized type falls back to the read-only value, no fabricated editor, no throw.
6. **ENG-12** — text ∈ {empty ⇒ clear, finite ⇒ accepted, non-finite ⇒ unchanged}.
7. **ENG-13** — no change handler ⇒ inert.
8. **ENG-29** — a value/label with markup renders as literal text; no DOM-injection sink.

## Test obligations

- **Happy path**: one case per type — number, text, `bool`, `config.*`, derived with a formula, derived
  without a formula, unknown type — asserts the control, its accessible role/name, and the reported
  value.
- **Happy path**: a derived leaf shows the "Calculated" label and its formula on hover.
- **Edge**: numeric input holding a number, typing `"abc"` / `"Infinity"` / `"1e999"` leaves the value
  unchanged (the callback is not called, or is called with the stored value); typing `""` reports
  `null`.
- **Edge**: with `onChanged` absent, the numeric and text inputs and the toggle are non-interactive and
  a keystroke reports nothing.
- **Edge**: an unknown editable type renders no editor and does not throw; a `null`/absent stored value
  renders an empty display and a subsequent edit still reports a real value.
- **Edge**: a marked derived leaf drops the "Calculated" label and tints the control (class
  `lg-field--marked`).
- **Happy path**: a `config.*` toggle round-trips `1`/`0`; a `bool` toggle round-trips
  `true`/`false`.
- **Security**: a value of `"<script>"` appears as literal text and the DOM holds no `script` element
  (ENG-29, SC-005).
- **Source scan**: `web/src/engine/` holds no literal hex/px and no user-facing literal string (extends
  the F1 token scan and the F3 copy scan).
