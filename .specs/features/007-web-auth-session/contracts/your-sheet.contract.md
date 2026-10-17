# Contract: "Your sheet" destination (resolve, open, confirmed delete)

**Requirements**: WAS-25, WAS-26, WAS-27, WAS-28, WAS-29 | **Backing document**:
[data-model.md `YourSheet`](../data-model.md) and [research.md `R10`](../research.md). The consumed
backend operations are `GET /sheets` (owner-scoped, `404` when none) and `DELETE /sheets/:id` (`204`).

**Modules**: `web/src/sheet/your_sheet.ts` (the resolve state machine and the delete),
`web/src/sheet/your_sheet_screen.tsx` (the surface and the confirmation),
`web/src/api/client.ts` (the extended `ApiClient`: `getMySheet`, `deleteSheet`),
`web/src/shell/left_nav.tsx` (the session-only navigation entry).

## Interface

| Operation | Request | Success | Failure |
|---|---|---|---|
| `resolve()` | `GET /sheets` (Bearer) | `200` + `Sheet` ⇒ `sheet`, or `404 NOT_FOUND` ⇒ `empty` | any other outcome ⇒ `failure` with a Retry |
| `delete(id)` | `DELETE /sheets/:id` (Bearer) | `204` ⇒ evict locally, show `empty` | mapped copy; the sheet is kept |

```ts
export type YourSheetState =
  | { kind: 'loading' }
  | { kind: 'empty' }
  | { kind: 'sheet'; sheet: Sheet }
  | { kind: 'failure'; message: string };
```

## Rules

1. **Server is the source of truth** (WAS-25). The destination resolves the caller's sheet with one
   `GET /sheets`; the local cache never decides the resolve kind. A sheet that is not the caller's
   resolves as missing and is never rendered as the user's sheet.
2. **Empty is not failure** (WAS-26). A `404`/`NOT_FOUND` renders the designed empty line, distinct
   from the loading state and from a failure.
3. **Failure retries once** (WAS-27). Any other outcome — including an undecodable payload — renders
   the mapped copy with a Retry that re-requests exactly once.
4. **Confirm before destroy** (WAS-28). Activating Delete opens a dialog stating the exact
   consequence; cancel issues zero requests and keeps the sheet; confirm issues exactly one
   `DELETE /sheets/:id`.
5. **Success evicts, failure keeps** (WAS-29). On success the sheet and its draft are removed from the
   local store and the empty state is shown; on failure the sheet is kept and the mapped copy is
   shown.
6. **Session-only entry** (WAS-25). The left-navigation entry renders only while a session is held and
   navigates to `/your-sheet`, a session-only route (see [auth-gate.contract.md](./auth-gate.contract.md)).

## Obligations

1. **WAS-25** — one owner-scoped `GET /sheets`; the server decides, not the cache.
2. **WAS-26** — a designed empty state, distinct from loading and failure.
3. **WAS-27** — a mapped failure with a single-request Retry.
4. **WAS-28** — cancel ⇒ 0 requests; confirm ⇒ 1 `DELETE`.
5. **WAS-29** — evict the sheet and its draft on success; keep the sheet on failure.

## Test obligations

- **Happy path**: resolve a person with a sheet ⇒ the sheet renders from one `GET`; Open navigates to
  `/sheets/:id`; confirm delete ⇒ one `DELETE`, the local record gone, the empty state shown.
- **Edge**: resolve with no sheet ⇒ the empty state and no error; a `404` is not a failure; Retry
  issues exactly one request; cancel issues zero requests and keeps the sheet.
- **Error**: a `5xx` resolves to the mapped failure with a Retry; a delete failure keeps the sheet and
  shows the mapped copy; an undecodable `GET` payload is a failure, not a crash.
- **Interface**: the dialog states the exact consequence and its safe action carries focus; the delete
  control and the confirmation are reachable by keyboard with accessible names.

## Traceability

- Resolve: WAS-25, WAS-26, WAS-27.
- Delete: WAS-28, WAS-29.
