# Dice expression test vectors

`vectors.json` is the shared source of truth for the **Dice expression** grammar
(see `GLOSSARY.md` and `docs/adr/0005-dice-expressions-own-grammar-and-live-on-the-sheet.md`).
Two implementations read it: the Go package `backend/internal/dice` (validates) and
the web client (evaluates and validates). Every vector must be asserted by both;
a behaviour change starts here.

## Sections

| Key | Meaning |
|-----|---------|
| `version` | Format version (`1`). Bump on an incompatible change. |
| `limits` | Grammar limits: 64 characters, 8 terms, 1–20 dice, 1–1000 faces, whole numbers 0–9999. |
| `fields` | Field schema used by `fit`, `sections` and `evaluate`: dotted path → `number` \| `text` \| `bool`. Anything absent is "not a Field" (a group, a Section, a table Row key). |
| `valid` | `expr` parses; `canonical` is its normalised text (single spaces around `+`/`-`, lower-case `d`, explicit dice count, no leading zeros, no leading `+`). |
| `invalid` | `expr` is refused at parse time with the reason `class`. |
| `fit` | `expr` against Row key `row`: `fits` true, or false with the `class` of the first problem. A Row fits when every Field it names exists and is numeric; `@` is the Row's own Field and `@.<column>` is `<row>.<column>`. |
| `sections` | `expr` against the Row keys `rows` of a Section: it fits when it fits at least one Row; no Rows fits nothing. |
| `evaluate` | `expr` on Row `row`, Field `values`, with the fixed die results `dice` consumed in order. Expect `total` and `parts` (`sign`, `label`, `value`, and `rolls` for dice terms), or an `error` class. |

## Grammar rules the vectors pin

- Terms are dice (`NdM` or `dM`, `D` read like `d`), whole numbers, Field paths and `@` / `@.<column>`, joined by `+` / `-`, with an optional leading sign (a sign alone is `missing_term`).
- Whitespace is **space and tab only**, and only around terms and operators. A non-breaking space, a newline or a space inside a term is `unreadable_term`.
- Length is counted in Unicode characters (code points, **not** UTF-16 units: use `[...s].length` in JS) before anything else is checked: an over-long string is `too_long` even if it is also garbage or blank. A blank string of at most 64 characters is `empty`.
- Field paths are case-sensitive, ASCII, dot-separated; each segment is `[A-Za-z][A-Za-z0-9]*`. `@` takes at most one column.
- A whole term that is a bare `d` / `D`, or a bare keep token `kh` / `kl` (any case) optionally followed by digits (`kh1`, `KL2`, `kh01`), is `unreadable_term`, never a Field. The rule applies to the single-segment token only: `stats.kl`, `kh.total`, `@.kh1`, `khazad`, `kl2x`, `dexterity` and `d1` (a die) keep their meaning.
- Numbers are read as ASCII digits; leading zeros are allowed (`007` is 7); arbitrarily long digit strings are range errors, not overflows.
- Parse errors are checked term by term left to right; the term-count limit is checked after every term has been read.
- A Roll floors each Field value (toward minus infinity) before adding it and may total below zero. Dice are thrown in expression order.

## Reason classes

`empty`, `too_long`, `too_many_terms`, `dice_count`, `dice_faces`, `number_range`,
`missing_term`, `unreadable_term` (parse); `unknown_field`, `non_numeric_field`,
`no_rows` (fit); `no_value` (evaluate). The Go package also has `no_roller`, a
programming error that no vector covers.
