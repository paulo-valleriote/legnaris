---
status: accepted, amended in part by ADR 0002
---

# Layout lives on the Sheet, not the Template

The order of Sections and which Rows are pinned is user data: it is stored on the Sheet as a `layout`, written by its own endpoint (`PUT /sheets/:id/layout`, full replacement, no recalculation) and committed when the player leaves Arrange mode (amended by [ADR 0002](0002-vitals-strip-and-pins-outside-arrange.md): a Pin toggle outside Arrange mode commits immediately; only the Section order waits for leaving Arrange mode). The Template stays code-defined and read-only and only supplies the default order. The layout is `{section_order, pinned}`: Rows are never reordered, only pinned, and a pinned Row rises to the top of its own Section in pin order. Rows are referenced by keys derived from the Template (field path, table-group path, `<parent>#<exclusive_group>`); lists may be partial, missing keys fall back to Template order and stale keys are ignored on read.

## Considered Options

- **Layout in the Template (user variants):** contradicts "templates are code, reviewed" and the anti-copyright line of defense.
- **Local-only layout (IndexedDB):** loses the arrangement across devices and breaks "the user owns their data" (export/import).
- **Layout inside the `PATCH` of `data`:** would route a non-value through the flatten/editability/DAG path and the delta/preview contract.

## Consequences

- Columns of a table Row stay Template-defined; Rows only pin, they do not move.
- A first design also allowed reordering Rows inside a Section (`row_order`); it was dropped to keep the interface clean.
- Layout is part of the self-contained export and validated strictly on import.
- Reordering never touches the recalculation DAG, and never marks the Sheet as having unsaved values.
