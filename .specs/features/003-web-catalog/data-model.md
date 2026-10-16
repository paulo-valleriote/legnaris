# Phase 1 — Data Model: Web Catalog

The feature persists nothing. The structures below are the client-side shapes the catalog screen and
the creation surface introduce; the payload they decode is the F2 template model
(`web/src/api/models/template.ts`) and is not re-declared here. Every field traces to a functional
requirement in [spec.md](./spec.md).

Naming: TypeScript types use `PascalCase`, values and functions `camelCase`; the backend's
`snake_case` keys never appear outside the F2 decoders.

---

## QueryKey

The identity of a server-state read; it is what makes two requests the same request (WCT-02, WCT-08,
WCT-21).

| Key | Shape | Owns |
|---|---|---|
| catalog | `['templates']` | the template catalog |
| detail | `['template', id]` | one template's detail, keyed by its identifier |

**Invariants**: the catalog key carries no parameter, so every mount resolves to the one cache entry
(WCT-02); the detail key is per identifier, so two templates never share an entry and reopening the
same identifier after its entry is dropped is a new read (WCT-21). A trigger while the query for a key
is in flight joins the in-flight request rather than starting a second one (WCT-08).

---

## CatalogRenderPhase

The single branch the catalog screen renders; exactly one is active (WCT-03, WCT-04, WCT-05, WCT-06,
WCT-09).

| Phase | Entered when | Renders |
|---|---|---|
| `loading` | the catalog query has no data and is fetching (WCT-03) | the loading copy; never the empty copy |
| `loaded` | the query resolved with one or more entries (WCT-04) | one row per entry, in the response order |
| `empty` | the query resolved with zero entries (WCT-05) | one line of plain localized text; no decorative art |
| `error` | the query rejected with a `DomainFailure` (WCT-06) | the mapped copy for the kind + a Retry action |
| `parseError` | the query rejected with a `DefensiveParseFailure` (WCT-09) | the parse-failure copy + a Retry action; never a blank screen or a partial list |

**Invariants**: the phase is total — a query result always resolves to exactly one of the five, so no
"loading and error at once" state is expressible (SC-001, SC-007). A `DomainFailure` with kind
`notFound` is not reachable here (the catalog has no identifier), and any other kind still renders the
`error` phase with its mapped copy.

**Transitions**: `loading → loaded | empty | error | parseError`; `error | parseError → loading` when
Retry is activated (WCT-07); no other transition exists in this feature because the catalog has no
refresh (clarification, 2026-10-16).

---

## DetailRenderPhase

The single branch the creation surface's detail area renders; exactly one is active (WCT-12, WCT-13,
WCT-14, WCT-15, WCT-17, WCT-18).

| Phase | Entered when | Renders |
|---|---|---|
| `loading` | the detail query has no data and is fetching (WCT-12, WCT-13) | the loading copy; never a previously opened template's data |
| `loaded` | the query resolved (WCT-14) | the template's display name; an empty name renders as empty text without throwing |
| `error` | the query rejected with a `DomainFailure` (WCT-15) | the mapped copy inline in the surface; the surface stays open |
| `notFound` | the query rejected with a `DomainFailure` of kind `notFound` (WCT-17) | the mapped missing-template copy inline; the surface stays open, no close, no crash |
| `parseError` | the query rejected with a `DefensiveParseFailure` (WCT-18) | the parse-failure copy inline; never a half-rendered detail |

**Invariants**: `notFound` is a specialization of `error` kept distinct because the copy differs
(WCT-17 versus WCT-15) while the treatment — inline, surface open, Retry available — is the same. The
phase is derived from the query state only; the surface never holds a copy of the detail, so a new
opening cannot display a stale template (WCT-13, R4 in [research.md](./research.md)).

**Transitions**: `loading → loaded | error | notFound | parseError`; `error | notFound | parseError →
loading` when Retry is activated (WCT-16). Dismissing the surface ends the phase by unmounting the
query, which drops its entry (`gcTime: 0`); reopening starts at `loading` again (WCT-21).

---

## CatalogEntry

The view model of one catalog row (WCT-04, WCT-10).

| Field | Type | Rules |
|---|---|---|
| `id` | `string` | the template identifier from the payload, opaque; passed verbatim to the detail request |
| `name` | `string` | the template's display name; an empty string renders as empty text with the layout intact (WCT-10) |

**Invariants**: the entry is a projection of the decoded `TemplateSummary` — the same decoded template
the detail returns (F2 `TemplateSummary = TemplateDetail`) — so the row needs no second decode (spec
Assumptions). The list preserves the response order and is never sorted or filtered (WCT-04).

---

## CreationSurfaceState

The state the open overlay owns (WCT-11, WCT-19, WCT-22, WCT-23, WCT-24).

| Field | Type | Rules |
|---|---|---|
| `templateId` | `string \| undefined` | the identifier the surface was opened for; `undefined` means no surface is open |
| `sheetName` | `string` | the raw text the person typed; starts empty (WCT-22) |
| `trimmedName` | `string` | derived: `sheetName.trim()`; the validation input only |
| `detailPhase` | `DetailRenderPhase` | the detail area's branch, owned by the detail query (above) |
| `canSubmit` | `boolean` | derived: `trimmedName !== ''` (WCT-23, WCT-24) |

**Invariants**: the surface is open if and only if `templateId` is defined, and closing it sets
`templateId` back to `undefined` without touching the catalog's cache or `sheetName`'s siblings in the
list (WCT-19). `canSubmit` gates the primary action's `disabled` attribute, and the action's handler
issues no request in this feature (WCT-25) — so there is no code path from the surface to a mutation.

**Transitions**: closed → open(identifier) on activating a catalog entry (WCT-11); open → closed on
Esc or the close action (WCT-19); `sheetName` changes on typing and moves `canSubmit` with it
(WCT-23, WCT-24). Opening the same identifier again re-runs the detail query from `loading`
(WCT-21), because closing dropped the previous entry.

---

## SheetNameValue

The validation input (WCT-23, WCT-24).

| Input | `trimmedName` | `canSubmit` |
|---|---|---|
| `""` | `""` | `false` |
| `"   "` (spaces/tabs) | `""` | `false` |
| `"  Aragorn  "` | `"Aragorn"` | `true` |
| `"Aragorn"` | `"Aragorn"` | `true` |

**Invariants**: emptiness is decided on the trimmed value, never the raw one, so a whitespace-only
name is empty (WCT-23); the raw value is what the field displays, so typing is never fought.

---

## MessageKey additions

The catalog's copy resolves through the F2 catalog (`web/src/l10n/strings.ts`); the failure and
parse-failure keys are reused unchanged (WCT-26).

| Key | Used by |
|---|---|
| `catalog.title` | the catalog screen's heading |
| `catalog.loading` | `CatalogRenderPhase.loading` |
| `catalog.empty` | `CatalogRenderPhase.empty` |
| `catalog.retry` | the Retry action in `error`/`parseError` (WCT-06, WCT-09) |
| `catalog.openHint` | the accessible name/hint for activating an entry (WCT-11) |
| `sheet.dialogTitle` | the creation surface's accessible label (WCT-27) |
| `sheet.nameLabel` | the sheet-name input's label (WCT-22) |
| `sheet.namePlaceholder` | the sheet-name input's example placeholder (WCT-22) |
| `sheet.nameRequired` | the disabled primary action's tooltip and description (WCT-23) |
| `sheet.create` | the primary action's label (WCT-23, WCT-24) |
| `sheet.close` | the close action's label (WCT-19) |
| `sheet.loading` / `sheet.retry` | the detail area's loading copy and Retry action (WCT-12, WCT-16) |
| `api.error.*` (existing) | every failure and parse-failure branch (WCT-06, WCT-09, WCT-15, WCT-17, WCT-18) |

**Invariants**: no user-facing literal exists in `web/src/catalog/`; every branch resolves a key
through `message()`, which is what SC-003 asserts.
