# Contract: Sheet open

**Requirements**: WSC-02, WSC-05, WSC-06, WSC-07, WSC-08, WSC-09, WSC-10, WSC-11 | **Backing
document**: [data-model.md `SheetOpenState`](../data-model.md) and [research.md `R9` / `R10`](../research.md)

**Module**: `web/src/sheet/sheet_open.ts` — the open-by-id state for one sheet, addressed by URL
identifier. It issues the one `GET /sheets/:id` (through the F2 client) and, once the sheet is known,
the one `GET /templates/:id` for its template detail; it owns no edit state.

## Interface

| Export | Signature | Obligation |
|---|---|---|
| `useSheetOpen(sheetId)` | `(id: string) => SheetOpenResult` | resolves the sheet's situation and, on success, its template's detail |
| `SHEET_QUERY_KEY` | `(id: string) => readonly ['sheet', string]` | the cache key a creation seeds (`WSC-02`) |

`SheetOpenResult` is one of: `{ kind: 'loading' }`, `{ kind: 'loaded', sheet, template }`,
`{ kind: 'notFound', message }`, `{ kind: 'failure', message, retry() }`,
`{ kind: 'templateLoading' }`, `{ kind: 'templateFailure', message, retry() }`.

## Rules

1. **One request per open** (WSC-05). Mounting the screen for an identifier issues exactly one
   `GET /sheets/:id`; a Retry issues exactly one more. No other mount re-requests.
2. **Created sheet is not re-fetched** (WSC-02). When `['sheet', id]` was seeded by the creation flow
   with the `201` payload and is still fresh, the screen returns `loaded` without issuing a request.
3. **Missing sheet** (WSC-06). A `NOT_FOUND` domain failure yields `notFound` with the mapped
   `message('sheet.missing')`, **no Retry**, and evicts the sheet (and its draft) from the local store
   best-effort; a failing eviction does not change the rendered state.
4. **Other failure** (WSC-07). Any other failure (including an undecodable payload) yields `failure`
   with `failureMessage(kind)` / `parseFailureMessage()` and a Retry that re-requests exactly once.
5. **Template detail** (WSC-09). After the sheet is known, its `templateId` is fetched once;
   `templateLoading` is rendered until it settles and `templateFailure` (mapped message + Retry) on
   error. A template `NOT_FOUND` is a failure with Retry, not a missing sheet.
6. **Rendered load** (WSC-08). `loaded` carries the sheet and the template; the screen shows
   `sheet.name` and renders `sheet.data` seeded through the F4 `SheetForm` over `template.fields`.
7. **Deep link** (WSC-10). The identifier comes from the route parameter alone, so a direct navigation
   resolves the same state a client navigation produces.
8. **Local cache write** (WSC-11). On `loaded` (whether fetched or seeded), the sheet is written to the
   local store through the `SheetStore` port; the write is best-effort and never blocks rendering.
9. **No credential** (WSC-31). The fetched sheet holds sheet values only; nothing is attached to the
   request (the F2 client sends `credentials: 'omit'`).

## Obligations

1. **WSC-02** — a seeded created sheet is returned without a second fetch.
2. **WSC-05** — one `GET /sheets/:id` per open and per Retry.
3. **WSC-06** — `NOT_FOUND` ⇒ missing message, no Retry, best-effort eviction of sheet + draft.
4. **WSC-07** — any other failure ⇒ mapped message + Retry; undecodable ⇒ same path.
5. **WSC-08** — `loaded` exposes the name and feeds the field tree.
6. **WSC-09** — template loading/failure states with a Retry.
7. **WSC-10** — route-parameter-driven, deep-linkable.
8. **WSC-11** — cache by id under the versioned envelope.

## Test obligations

- **Happy path**: opening an existing sheet issues exactly one `GET /sheets/:id` and one
  `GET /templates/:id`, then renders the name and the field tree.
- **Happy path**: a created sheet seeded into the cache renders without a `GET /sheets/:id`.
- **Edge**: a slow response shows the loading state until it settles.
- **Error**: `NOT_FOUND` shows the missing message, offers no Retry, and the store's `deleteSheet` +
  `deleteDraft` are called; a throwing `deleteSheet` still shows the message.
- **Error**: a server failure shows the mapped message and one Retry re-requests exactly once; a
  malformed sheet payload takes the same path and renders no blank screen.
- **Error**: a template failure shows the mapped message and a Retry; a template `NOT_FOUND` is still a
  Retry case.
- **Edge**: a template with no fields renders the empty-fields line.
- **Edge**: after a successful open the store holds the sheet under `schemaVersion 1`, and the record
  contains no credential.
