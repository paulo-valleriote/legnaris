# Contract: Creation surface

**Requirements**: WCT-11…WCT-25, WCT-27, WCT-28 | **Backing document**:
[research.md R5, R7, R8](../research.md)

The creation surface (`web/src/catalog/create_sheet_dialog.tsx`) is an accessible overlay opened from a
catalog entry. This contract is what its suite asserts and what the F5 creation flow will extend.

## Structure

| Element | Rule |
|---|---|
| container | an overlay with `role="dialog"` and `aria-modal="true"`, labelled by its title (WCT-27) |
| title | the template's display name from the detail, or the loading/error copy while the detail is not loaded |
| detail area | exactly one `DetailRenderPhase` from [data-model.md](../data-model.md) |
| sheet-name field | a text input, empty by default, with a localized label and example placeholder (WCT-22) |
| primary action | disabled while `trimmedName === ''`, with a tooltip stating a sheet name is required (WCT-23, WCT-24) |
| close action | a labelled control that dismisses the surface (WCT-19) |

## Obligations

1. Activating a catalog entry opens the surface for that identifier and issues exactly one detail
   request (WCT-11, WCT-12).
2. While the detail is in flight the detail area shows the loading copy and never a previously opened
   template's detail (WCT-13).
3. On success the surface shows the detail's display name; an empty name renders as empty text without
   throwing (WCT-14).
4. A detail failure shows the mapped copy inline and keeps the surface open (WCT-15); a missing
   template shows the missing-template copy inline and does not close or crash (WCT-17); an unreadable
   response shows the parse-failure copy inline and never a half-filled detail (WCT-18).
5. The surface's Retry action re-requests the detail exactly once (WCT-16).
6. Esc or the close action dismisses the surface, returns to the catalog with the list intact, and
   issues no catalog request (WCT-19); navigation stays shallow — the surface is never a route
   (WCT-20).
7. `trimmedName` is `sheetName.trim()`; the primary action is disabled while it is empty and enabled
   once it is non-empty (WCT-23, WCT-24).
8. The surface is side-effect free: opening, typing and activating the primary action issue no mutating
   request (WCT-25).
9. Focus moves to the sheet-name input on open and returns to the triggering catalog entry on close;
   every control uses the shared focus token; no keyboard trap (WCT-27).
10. Every string resolves through the translation catalog and every visual value through the shared
    tokens (WCT-26, WCT-28).

## Test obligations

- Cases drive the detail request through `loading`, `loaded`, `error`, `notFound` and `parseError` and
  assert the inline rendering and the surface staying open.
- Cases type `""`, `"   "`, `"  Aragorn  "` and `"Aragorn"` and assert `disabled`/`enabled`.
- A case activates the primary action with a valid name and asserts zero transport attempts.
- A case opens two different entries in turn and asserts each surface loads its own identifier's detail.
- A case opens, dismisses with Esc and asserts focus returned to the entry and the list is unchanged.
- An accessibility pass asserts no critical violations on the open surface (SC-005).
