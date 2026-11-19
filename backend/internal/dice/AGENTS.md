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
- Whitespace is space and tab only; length is checked first; `D` is read like `d`; Field paths are case-sensitive ASCII.
- `@` is the Row key and `@.<column>` is `<row>.<column>`; a table Row key is not a Field, so bare `@` does not fit it.
- Section fit needs at least one Row; the no-fit error carries the first Row's class.
- `no_value` is an absent, NaN or infinite Field; `overflow` is a finite sum past float64; never reuse `no_value` for the total.
