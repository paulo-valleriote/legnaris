---
status: accepted
amends: 0001 (in part)
---

# Pins commit immediately outside Arrange mode, and Templates may suggest Pins

The play view shows a vitals strip built from the player's Pins, so Pins can no longer be an Arrange-only gesture. We decided three things, none of which changes the Layout wire shape (`{section_order, pinned}` via `PUT /sheets/:id/layout`, full replacement, no recalculation):

1. **A Pin toggle commits immediately.** Outside Arrange mode a Pin is saved with its own `PUT /sheets/:id/layout`, not on leaving Arrange mode. Reordering Sections stays a single commit when the player leaves Arrange mode. This is web behavior: the client sends the same full-replacement body, optimistic with revert on failure, one request at a time. The server contract is unchanged, and `PUT` is already idempotent.
2. **The Pinned filter is remembered per viewer and per Sheet** in the browser view store, next to the collapsed Sections. It is still never sent to the server, never part of the Layout and never exported; a missing or unreadable record means "off". "Off on every open" is retired.
3. **A Template may declare `suggested_pins`**: an optional, ordered list of Row keys (the Layout key grammar: field path, table-group path, `<parent path>#<exclusive_group>`), exposed read-only on the Template detail and omitted when empty. It is only a suggestion the client offers ("Pin suggested rows"); the server never writes it into a Sheet or Layout. It is validated when the Template is registered (boot): an unknown or non-Row key, a duplicate, or more entries than the vitals strip holds (`MaxSuggestedPins`, 6) is a typed error that fails boot. The first content is fantasy_v1's, whose Section order also moves Combat and Derived right after Identity (presentation only: a stored `section_order` or `pinned` still resolves).

## Considered Options

- **Template default pins applied implicitly when the Layout has none:** needs to tell "no layout" from "a layout that cleared its pins" and changes the Layout wire, the repository semantics and what "reset to default" means.
- **Template-declared zones (vitals, combat, skills) or a `vitals` list in the Layout:** bakes a genre vocabulary into the contract and duplicates Pin; the widest blast radius (DTO, repository, validation, export/import).
- **Keep committing Pins on leaving Arrange mode:** forces the player into a reorder mode to pin one number, and loses the Pin if the player leaves without finishing.

## Consequences

- ADR 0001 stands except for the sentence "committed when the player leaves Arrange mode", which now applies to Section order only.
- Two Pin requests can race; the client serializes them. Offline, the toggle reverts; it is not queued.
- Layout, `PATCH`, the Mongo documents and export/import are untouched. A new Template author owns its `suggested_pins`, and a wrong key stops the API from booting instead of surfacing in a request.
- The Row key grammar is still defined twice, by the backend `SectionRowKeys` and the web `buildRows`; `suggested_pins` reuses it and adds no new key kind.
