# AGENTS.md — backend/internal/dice (Dice expression grammar)

## Responsibility

- `Parse` reads a Dice expression (syntax and limits only); `Expression.FitsRow`, `ValidateRow` and `ValidateSection` decide whether it can be rolled on a Row / Section against a `Schema`; `Expression.Evaluate` computes it with a caller-supplied `Roller` (the Go side uses it to assert the shared vectors; the client is the one that rolls for real)
- Pure and stateless: `Expression` is immutable, so everything is safe for concurrent use

## Conventions

- Declare own typed errors (`*dice.Error`, one `Class` per reason, `errors.Is` against the `Err*` sentinels) — **do NOT import `apperr`**; the later boundary ticket translates classes (strict `PUT` → 400 `INVALID_REQUEST`)
- No dependency on `template`: the Field schema comes in through the `Schema` interface, and a Section's Rows as plain `[]string` keys (from `template.SectionRowKeys`)

## Gotchas

- **`testdata/dice/vectors.json` is shared with the web client** — every grammar change starts in the vectors (and `testdata/dice/README.md`), then both implementations follow. The `Class` strings and the `Limits` constants are part of that contract (`TestParse_LimitsMatchVectors`)
- **Parse is syntax only** — a well-formed path that is not a Field is accepted by `Parse` and refused by `FitsRow`. Bare `d` / `D` and bare keep tokens (`kh`, `kl`, `kh1`, `kl2`, any case) are refused by `Parse` as `unreadable_term`, so `1d20 + kh1` never reaches the Field check; look-alike paths (`khazad`, `stats.kl`, `@.kh1`) still parse. Always validate through `ValidateRow` / `ValidateSection` before storing
- **Whitespace is space and tab only**, length is counted before anything else, `D` is read like `d`, field paths are case-sensitive ASCII
- **`@` resolves against the Row key** (`@` → `<row>`, `@.total` → `<row>.total`); a table Row key such as `skills.acrobatics` is not itself a Field, so a bare `@` does not fit it, while `@.total` does
- **Section fit is "at least one Row"**; the error returned when none fits carries the class of the first Row's problem
- **Evaluate has two number classes**: `no_value` means a Field is absent, NaN or infinite; `overflow` means finite Fields sum past the float64 range (the total becomes infinite). Do not reuse `no_value` for the total
