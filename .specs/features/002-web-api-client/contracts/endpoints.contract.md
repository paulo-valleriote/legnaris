# Contract: Consumed endpoints

**Requirements**: API-01, API-02, API-05, API-06 | **Backing documents**: `spec.md` (US1),
[data-model.md](../data-model.md), `shared/response/response.go`, `internal/app/{template,sheet}.go`

Exactly six operations are consumed. Each has one typed client method, one request shape and one
decoded payload type. The backend exposes more routes (listing the caller's own sheet, deletion,
preview); those are out of scope and are not wrapped here.

## Operations

| # | Method + path | Client method | Request body | Success status | Decoded payload |
|---|---|---|---|---|---|
| 1 | `GET /health` | `health()` | — | 200 | `{ status: string }` |
| 2 | `GET /templates` | `listTemplates()` | — | 200 | `TemplateSummary[]` |
| 3 | `GET /templates/:id` | `getTemplate(id)` | — | 200 | `TemplateDetail` |
| 4 | `POST /sheets` | `createSheet({ templateId, sheetName })` | `{"template_id": "<id>", "sheet_name": "<name>"}` | 201 | `Sheet` |
| 5 | `GET /sheets/:id` | `getSheet(id)` | — | 200 | `Sheet` |
| 6 | `PATCH /sheets/:id` | `patchSheet(id, changeSet)` | `{"data": <change set>}` | 200 | `Record<string, unknown>` — recalculated effects only |

## Rules

1. **Envelope**: every one of the six returns the standard envelope; the payload is the value of `data`
   (API-02). The `/health` route is inside the envelope too — the backend uses the same
   `response.OK` helper, so the client does not special-case it.
2. **Body exactness**: the creation body carries exactly `template_id` and `sheet_name`; the update body
   carries exactly `data`. Extra keys are a defect, not a convenience (API-05, API-06).
3. **Change set**: `patchSheet` sends the caller's change set unmodified — no merging, no enrichment and
   no client-side recalculation. The response contains only the cascading effects the backend computed,
   and the client returns them as decoded; merging them into a sheet is the sheet-cycle feature.
4. **Path identifiers**: the identifier is percent-encoded (`encodeURIComponent`) and otherwise passed
   through verbatim — the backend defines identifier shape, the client never validates or re-formats it.
5. **No query strings**: none of the six takes a parameter, so a URL carrying a query string is a defect.
6. **No request-specific retry policy**: all reads use the read policy, all writes the no-retry policy
   (transport contract, obligation 4).

## Test obligations

- One table-driven case per operation asserting method, path, headers, body and decoded payload.
- A case per write asserting the exact JSON body keys.
- A case asserting the update call never mutates the caller's change set.
- A structural case asserting every operation's request went through the injected transport (API-07).
