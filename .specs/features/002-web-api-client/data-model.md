# Phase 1 — Data Model: Typed Web API Client

The feature persists nothing. The structures below are the client-side shapes the network layer
introduces and the backend payloads it decodes; every field traces to a functional requirement in
[spec.md](./spec.md) and to the real contract in `shared/response`, `shared/apperr`,
`internal/sheet/template/dto.go` and `internal/sheet/dto.go`.

Naming: TypeScript types use `PascalCase`, values and functions `camelCase`; the backend's
`snake_case` keys appear only inside the decoders.

---

## Envelope

The wrapper every backend response uses (`shared/response.Success[T]` / `Response.ErrorResponse`).

| Field | Type | Rules |
|---|---|---|
| `data` | `unknown`, present on success | required on a 2xx; absence is a defensive parse failure (API-04) |
| `error.code` | `string`, present on failure | required and non-empty on a non-2xx; absence is a defensive parse failure (API-22) |
| `error.message` | `string`, present on failure | carried onto the domain error; absent becomes an empty string (API-24) |

**Invariants**: exactly one of the two branches is decoded per response; a 2xx never yields an empty
success; a non-2xx never yields a payload (API-02, API-03, API-04, API-22).

**Validation rules**: the body must be non-empty valid JSON; the parsed value must be an object; the
required key of the branch must exist with the required type. `decodeEnvelope` is the only place these
checks live.

---

## DomainFailure

The client's representation of a failed call (API-03, API-21, API-24).

| Field | Type | Rules |
|---|---|---|
| `kind` | `FailureKind` | assigned by the mapper; the field a screen branches on |
| `status` | `number \| undefined` | the HTTP status when a response was received; `undefined` for a transport failure |
| `code` | `string \| undefined` | the raw backend code when the envelope carried one (never translated or normalized) |
| `message` | `string` | the server's message, or a client-authored transport description that contains no credential and no identifier-bearing URL (API-29) |

**Invariants**: `kind` is always one of the seven; `code` is preserved verbatim even when the kind is
the generic fallback; `status` is present if and only if a response was received.

---

## FailureKind

A closed set of seven string literals — the interface-facing classification (API-15…API-21).

| Kind | Assigned when | Backend code |
|---|---|---|
| `clientFault` | the client sent a body the backend could not read | `INVALID_JSON` |
| `invalidEdit` | the edit itself is invalid and must be reverted | `INVALID_REQUEST` |
| `renderMismatch` | the rendered form drifted from the template schema | `UNKNOWN_FIELD` |
| `notFound` | the requested resource does not exist | `NOT_FOUND` |
| `serverError` | the backend failed; manual retry expected | `INTERNAL_ERROR` |
| `offline` | the transport failed with its attempts exhausted | — (no response) |
| `unknown` | any code without a dedicated kind | `CONFLICT`, `CYCLE_DETECTED`, `INVALID_DEPENDENCY`, `INVALID_PATH`, `SHEET_LIMIT_REACHED`, `UNAUTHORIZED`, `FORBIDDEN` |

**Invariants**: the table is total over the twelve codes in `shared/apperr/err.go`: five map to a
dedicated kind, seven fall back to `unknown` with the raw code preserved. Adding a code to the backend
without deciding its kind is a contract change the mapper's test must fail on.

---

## DefensiveParseFailure

The representation of a response that does not match the envelope, kept distinct from a mapped failure
so contract drift is distinguishable from a refusal (API-04, API-22).

| Field | Type | Rules |
|---|---|---|
| `reason` | `string` | names what failed to match — empty body, invalid JSON, missing `data`, missing `error.code`, or the field that did not match the model |
| `status` | `number \| undefined` | the HTTP status when a response was received |

**Invariants**: this failure never carries a `kind` and never becomes a `DomainFailure`; a caller can
always tell "the server said no" from "we cannot read the server".

---

## PathFailure

Raised by the dotted-path accessor for a malformed path (API-14).

| Field | Type | Rules |
|---|---|---|
| `path` | `string` | the offending path, as supplied |

**Invariants**: raised only for an empty path or a path with an empty segment; a missing segment or a
non-object intermediate yields "no value" instead (API-13). The two cases are deliberately different
signals: absent data is legitimate, a malformed path is a bug.

---

## Transport (contract)

The injected seam (API-07, API-29). Documented fully in
[contracts/transport.contract.md](./contracts/transport.contract.md).

| Request field | Type | Rules |
|---|---|---|
| `url` | `string` | absolute; the resolved base address joined with the operation path |
| `method` | `'GET' \| 'POST' \| 'PATCH'` | only the methods the six operations use |
| `headers` | `Readonly<Record<string, string>>` | `content-type: application/json` when a body is present; where a later feature adds authorization |
| `body` | `string \| undefined` | JSON text; never logged |
| `credentials` | `'omit'` | fixed — no ambient credential is ever attached implicitly |
| `signal` | `AbortSignal` | aborted when the attempt exceeds the timeout or the caller cancels |

| Response field | Type | Rules |
|---|---|---|
| `status` | `number` | the HTTP status |
| `body` | `string` | the raw text, undecoded |

**Invariants**: the client constructs exactly one transport per instance and never calls the platform
`fetch` itself; a transport rejection is a transport failure, a returned response is definitive
(API-31, API-32).

---

## ConsumedOperation

One of the six backend operations the product calls (API-01, API-05, API-06).

| Operation | Method + path | Request body | Decoded payload |
|---|---|---|---|
| health | `GET /health` | — | `{ status: string }` |
| template catalog | `GET /templates` | — | `TemplateSummary[]` |
| template detail | `GET /templates/:id` | — | `TemplateDetail` |
| sheet creation | `POST /sheets` | `{ template_id, sheet_name }` | `Sheet` |
| sheet retrieval | `GET /sheets/:id` | — | `Sheet` |
| sheet update | `PATCH /sheets/:id` | `{ data: <change set> }` | `Record<string, unknown>` (recalculated effects only) |

**Invariants**: the path identifier is percent-encoded and never re-formatted (the backend owns
identifier shape); a body carries exactly the declared keys and nothing else (API-05, API-06); the
update payload is returned as decoded — merging it is a later feature.

---

## TemplateSummary / TemplateDetail

The decoded template (`internal/sheet/template/dto.go`). The catalog returns the same shape as the
detail (the backend reuses `TemplateDTO`), so one model decodes both; the catalog-only view is a
projection, not a second parser (API-08).

| Field | Type | Rules |
|---|---|---|
| `id` | `string` | template slug, code-defined |
| `name` | `string` | display name |
| `fields` | `Record<string, FieldNode>` | top-level groups, in the payload's order |
| `default` | `boolean` | read explicitly by key (API-08) |

**Invariants**: key order is preserved as received (the backend emits sections in declared order and the
interface renders them in that order); no field is invented when the payload omits it.

---

## FieldNode

A node of the field tree; exactly one of the two shapes (API-08).

| Variant | Fields | Rules |
|---|---|---|
| leaf | `schema: FieldSchema` | present when the payload carries `schema` |
| group | `children: Record<string, FieldNode>` | present when the payload carries `children`; may also carry `label`, `columns`, `column_label` metadata |

**Invariants**: a node is classified by which key it carries; a node with neither is a defensive parse
failure rather than an empty node.

---

## FieldSchema

A leaf's declared contract (API-09).

| Field | Type | Rules |
|---|---|---|
| `type` | `string` | opaque here; the engine feature maps it to a control |
| `label` | `string` | display label |
| `formula` | `string \| undefined` | absent when the payload omits it — never `''` |
| `dependsOn` | `readonly string[][] \| undefined` | absent when omitted — never `[]` |
| `isEditable` | `boolean` | drives editability in the engine feature |
| `default` | `number` | the declared default value |
| `exclusiveGroup` | `string \| undefined` | optional metadata the payload may carry |
| `optionLabel` / `columnLabel` | `string \| undefined` | optional metadata the payload may carry |

**Invariants**: `formula` and `dependsOn` distinguish "absent" from "empty" (API-09); the optional
metadata is carried through so the engine feature needs no second decode.

---

## Sheet

A stored sheet (`internal/sheet/dto.go`) (API-10, API-11).

| Field | Type | Rules |
|---|---|---|
| `id` | `string` | server-generated identifier, opaque |
| `templateId` | `string` | the template this sheet renders |
| `name` | `string` | user-supplied name; rendered as text, never as markup |
| `data` | `Record<string, unknown>` | dynamic nested structure, preserved verbatim |
| `createdAt` / `updatedAt` | `string` | ISO-8601 timestamps as received, not re-formatted |

**Invariants**: `data` is never coerced, never flattened and never mutated; whole and fractional numbers
are preserved as decoded, without truncation or forced coercion (API-11); nothing in `data` is
interpreted as code (R7).

---

## Dotted path

The schema-declared address of a value inside sheet data (API-12, API-13, API-14).

| Input | Result |
|---|---|
| `"stats.dexterity"` on data containing that path | the value at that path |
| a path whose segment is missing | "no value" (`null`), no exception |
| a path whose intermediate node is not an object | "no value" (`null`), no exception |
| `""` | `PathFailure` |
| `"stats..dexterity"` | `PathFailure` |

**Invariants**: traversal is segment-by-segment over `Record<string, unknown>`; the accessor never
mutates `data`; "no value" and a malformed path are never conflated (R6, and the spec's edge cases).

---

## ApiConfiguration

The resolved base address (API-25…API-28).

| Field | Type | Rules |
|---|---|---|
| `baseUrl` | `string` | an absolute `http(s)` URL, no trailing slash |
| `source` | `'default' \| 'override'` | where the value came from; asserted by the configuration test |
| `secure` | `boolean` | `true` only for `https:` — the connection-indicator feature consumes this to warn (API-28) |
| `isDevOverride` | `boolean` | `true` when the value came from the build override; distinguishes a deliberate local target from the production default |

**Invariants**: an empty override resolves to `source: 'default'` and the HTTPS address (API-26); a
value that is not an absolute HTTP/HTTPS URL raises a configuration failure before any request is
issued (API-27) — the resolver is pure, so this is decided at construction, not at first use.

---

## PinnedCapture

A recorded real response used as a contract fixture (API-34…API-36).

| Field | Type | Rules |
|---|---|---|
| `file` | path | one JSON file per captured case under `web/src/api/fixtures/` |
| `operation` | the captured operation | one capture per consumed operation, plus the failure envelopes and the update pair |
| `request` | `{ method, path, body? }` | recorded alongside the response, so the capture is a contract pair, not just a payload |
| `response` | `{ status, body }` | the raw status and body, byte-faithful |
| `provenance` | note | which local stack produced it and the command that reproduced it |

**Invariants**: a capture is never edited by hand to make a test pass; the update pair records a
single-change request and a response containing only recalculated effects (API-35); the suite fails if
a capture's envelope or field shape stops matching the models (API-36).
