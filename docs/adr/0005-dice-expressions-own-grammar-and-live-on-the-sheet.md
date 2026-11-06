---
status: accepted
---

# Dice expressions use their own grammar and the player's choices live on the Sheet

Rolling is a client action: the web evaluates a Dice expression with a local random source against the values on screen, and nothing about a Roll is stored. A Dice expression is a small grammar of its own (`NdM`, whole numbers and numeric Field paths joined by `+`/`-`, plus `@` for "this Row"), not an expr-lang formula, because a player can type it and the formula engine must never evaluate user-supplied text. The Template supplies defaults as `SectionDice` and `RowDice` maps, validated at boot like `SuggestedPins`. The player's replacements, additions and explicit "no dice" (`null`) are user data stored on the Sheet as a `dice` block, written by `PUT /sheets/:id/dice` with full replacement, no recalculation, saved the moment it is changed, never through the `PATCH` of `data` (same reasoning as [ADR 0001](0001-layout-lives-on-the-sheet.md)).

## Considered Options

- **expr-lang for the bonus:** reuses the engine and `DependsOn`, but evaluates user-controlled text and cannot express the dice part.
- **Server-side Rolls with a persisted history:** gives auditability, which the V1 scope (no game master, no shared table) does not need, and costs latency and offline resilience at the table.
- **Dice on each Field (not Row or Section):** misses table Rows (`skills.acrobatics`) where the Row, not a column, is what the player rolls.

## Consequences

- Resolution is specificity-first, with "no dice" as the one exception: the player's Row choice, then the player's "no dice" for the Section (a kill switch that also silences Template Row defaults), then the Template's Row default, then the player's Section expression, then the Template's Section default. A player's Section expression therefore replaces only the Section default, never a Row the Template configured on purpose; the client says so under the Section heading.
- `d20` is read as `1d20`; whole numbers stop at 9999 so a typed total cannot overflow a Roll.

- The grammar has two implementations (Go validates, TypeScript evaluates); one shared file of test vectors keeps them in step.
- Reads are tolerant (an override pointing at a Row or Field the Template no longer has is ignored); the `PUT` is strict (unknown key or bad grammar → 400 `INVALID_REQUEST`).
- No Roll history and no keep-highest/lowest in the first version, so advantage and disadvantage are not expressible yet.
- The `dice` block is part of the Sheet payload, so any future export/import carries and re-validates it.
