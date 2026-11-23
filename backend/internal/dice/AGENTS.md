# AGENTS.md — backend/internal/dice (Dice expression grammar)

## Responsibility

- `Parse` reads syntax and limits only; `Expression.FitsRow`, `ValidateRow` and `ValidateSection` check a Row or Section against a `Schema`; `Expression.Evaluate` computes with a caller-supplied `Roller`.
- `Expression` is immutable, so everything is safe for concurrent use.

## Rules

- Declare `*dice.Error` with one `Class` per reason and the `Err*` sentinels; use `errors.Is`/`ClassOf` as the seam. Never import `apperr` or `template`; the sheet service translates classes at its boundary.
- The Field schema arrives through `Schema`; a Section's Rows arrive as plain `[]string` keys (from `template.SectionRowKeys`).
- `testdata/dice/vectors.json` is shared with the web client. A grammar change starts there; the `Class` strings and `Limits` are contract (`TestParse_LimitsMatchVectors`).
- Parse is syntax only: a path outside the schema passes `Parse` and fails `FitsRow`. Validate through `ValidateRow`/`ValidateSection` before storing.
- Bare `d` / `D` and bare keep tokens (`kh`, `kl`, `kh1`, `KL2`, any case) are `unreadable_term`; look-alikes (`khazad`, `stats.kl`, `@.kh1`) parse.
- A die's count and faces are each a literal or a Field path in braces (`{hp.count}d{hp.size}`, `{@.count}d8`, `1d{hitDie.size}`); the omitted count is 1. Fit follows the same path, `@` and numeric rules for a braced reference as for a term.
- A trailing ` min N` stores a minimum on the expression; it is the keyword only with surrounding whitespace and a following ASCII whole number. A bare `min` is a Field, and a malformed trailing one is `unreadable_term` (past 9999 is `number_range`).
- Whitespace is space and tab only; length is checked first; `D` is read like `d`; Field paths are case-sensitive ASCII; `String` emits braces, a lower-case `d` and the ` min N` suffix.
- `@` is the Row key and `@.<column>` is `<row>.<column>`; a table Row key is not a Field, so bare `@` does not fit it.
- Section fit needs at least one Row; the no-fit error carries the first Row's class.
- `Parse` never range-checks a Field-sized count or faces: `Evaluate` floors the value and answers `dice_count` / `dice_faces` outside 1–20 / 1–1000, or `no_value` when absent. A dice part's label at Evaluate is the resolved `NdM`; a minimum is reported only when it raised the total.
- `no_value` is an absent, NaN or infinite Field; `overflow` is a finite sum past float64; never reuse `no_value` for the total.
