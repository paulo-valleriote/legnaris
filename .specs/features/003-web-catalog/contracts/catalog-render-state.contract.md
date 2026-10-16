# Contract: Catalog render state

**Requirements**: WCT-01, WCT-03, WCT-04, WCT-05, WCT-06, WCT-07, WCT-09, WCT-10, WCT-26 | **Backing
document**: [data-model.md `CatalogRenderPhase`](../data-model.md)

The catalog screen (`web/src/catalog/catalog_screen.tsx`) renders exactly one of five situations and
resolves every string through the translation catalog.

## The five situations

| Phase | Trigger | Required rendering | Forbidden |
|---|---|---|---|
| `loading` | the query has no data and is fetching | the loading copy (WCT-03) | the empty copy; a blank content region |
| `loaded` | the query resolved with N ≥ 1 entries | N entries in the response order, each with the template name (WCT-04); an entry whose name is `""` still renders with the layout intact (WCT-10) | reordering, filtering, or dropping an entry |
| `empty` | the query resolved with 0 entries | one line of plain localized text (WCT-05) | decorative art; a Retry action |
| `error` | the query rejected with a `DomainFailure` | the copy from `failureMessage(kind)` + a Retry action (WCT-06) | the server's raw message; a partial list |
| `parseError` | the query rejected with a `DefensiveParseFailure` | the copy from `parseFailureMessage()` + a Retry action (WCT-09) | a blank screen; a partial list |

## Obligations

1. The catalog replaces the designed empty-workspace state on the root route (WCT-01): the root route
   mounts the catalog screen and `web/src/shell/empty_workspace.tsx` is deleted with its `.lg-empty*`
   rules (`AD-013`).
2. The left navigation's item for the root route is labelled "Catalog" through the copy catalog
   (WCT-29, `AD-015`).
3. Retry is the only way to leave `error`/`parseError`; activating it re-enters `loading` and issues
   exactly one request (WCT-07).
4. Every string comes from `message(key)`; no literal is authored in `web/src/catalog/` (WCT-26).
5. Every visual value (spacing, type, radius, color, focus, hover) comes from the shared tokens; no
   literal hex or px appears in `web/src/catalog/` (WCT-28).
6. Activating an entry opens the creation surface for that entry's identifier (WCT-11) and does not
   navigate.

## Test obligations

- One case per phase proves the rendering: the loading copy present and the empty copy absent while in
  flight; N rows in order; the single empty line; the mapped error copy with Retry; the parse-failure
  copy with Retry.
- A case with an entry whose name is `""` asserts the row renders without throwing.
- A case activates Retry from each error phase and asserts one further request.
- A case asserts the left navigation's root item is named "Catalog" (WCT-29).
- A source scan asserts no literal hex/px and no `api.error.` string literal outside the l10n catalog
  in `web/src/catalog/`.
