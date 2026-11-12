---
status: accepted, amends ADR 0005
---

# Dice choices are pending work saved with the Sheet

A player's Dice choice is edited like a value: it stays on the client as pending work and reaches the server only when the whole Sheet is saved. The save is one `PATCH /sheets/:id` carrying the data delta and, when a choice is pending, the full `dice` block; both are validated before a single compare-and-swap write, so a refused block writes no data and the other way round. `PUT /sheets/:id/dice` is removed. This replaces only the clause of [ADR 0005](0005-dice-expressions-own-grammar-and-live-on-the-sheet.md) that saved a choice "the moment it is changed, never through the `PATCH` of `data`"; the grammar, the precedence and where the block lives are unchanged.

## Considered Options

- **Keep the immediate `PUT` (ADR 0005):** every edit was a request of its own and could not be reviewed, discarded or saved with the values the player was changing at the same time; the sheet showed no unsaved state for it.
- **Footer Save sends `PATCH` then `PUT`:** two requests with no atomicity, each with its own 409 handling, so one could land and the other fail.
- **A new "save everything" endpoint:** a second write path for the same data next to a `PATCH` that already carries the version and the conflict contract.

## Consequences

- A pending choice is unsaved work: it lights the unsaved status and Save (and `Ctrl+S`), counts for the leave guard and `F5`, is reverted by discard, is backed up in the local draft and marks its Row or Section "Modified". Setting it back to the saved value clears it.
- Preview recalculates values only and never carries `dice`; a choice alone never enables it.
- A Roll uses the pending expression, as it already used pending values.
- A failed save keeps the choice pending; there is no rollback to the last stored block.
- `dice` in the `PATCH` is a full replacement; absent or `null` leaves the stored block alone, and an empty block clears it. A body with neither a delta nor a block is 400. A dice-only save skips the DAG and answers an empty cascade.
- A save carrying a block advances `version` and `updated_at`, like any other save.
