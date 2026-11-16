---
status: accepted, amends ADR 0002
---

# Templates do not suggest Pins

A Template carries no `suggested_pins`. The field, its boot-time validation (`ValidateSuggestedPins`, `MaxSuggestedPins` and the three suggested-pin errors) and the template DTO key are removed, and the web decoder no longer reads the key. This replaces only decision 3 of [ADR 0002](0002-vitals-strip-and-pins-outside-arrange.md); the Layout contract and how Pins are stored are unchanged.

The suggestion existed to feed the vitals strip and its "Pin suggested rows" action. Both were removed from the web client: a pinned Row rises to the top of its own Section and Pins are chosen in Arrange mode. No surface read the list, while the backend still validated it at boot and shipped it on every template response, and its cap of 6 was the capacity of a strip that no longer exists.

## Considered Options

- **Keep the field with a new reason for the cap:** documents a contract no client consumes.
- **Keep the field without a cap:** same dead payload, minus the one check that had a reason.
- **Reuse the suggestion elsewhere (first Arrange):** a product change with no current demand; a new ADR can add a field when a surface needs it.

## Consequences

- `GET /templates` and `GET /templates/:id` no longer emit `suggested_pins`. A client that still sends or reads the key is unaffected: the web decoder ignores unknown keys.
- A Template author has one less boot-time check; Layout pins keep their own strict-on-write validation (`ValidateLayout`).
- fantasy_v1 loses its five suggested Rows; stored Sheets and Layouts are untouched because the server never wrote the suggestion into either.
