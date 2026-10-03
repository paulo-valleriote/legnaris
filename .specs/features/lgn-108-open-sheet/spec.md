# Open Sheet from the Library (Screen 1 list + GET /sheets/:id on Screen 2) Specification

Source: [LGN-108](https://paulovalleriotedev.atlassian.net/browse/LGN-108) — "[Front End] Spec Fase 4 — Abrir ficha: biblioteca na Tela 1 + GET /sheets/:id na Tela 2", under epic [LGN-93](https://paulovalleriotedev.atlassian.net/browse/LGN-93). Depends on LGN-107 (local sheet library) and builds on the opened-sheet seam from LGN-109 (AD-004).

## Problem Statement

The local sheet library (LGN-107) stores the index of created sheets but no surface consumes it: Screen 1 has no "My sheets" list and there is no way to open a stored sheet. Screen 2 can only be reached with the `201` payload injected by the creation flow (LGN-109); the temporary "Preview sheet" action renders schema defaults for a template and is not a real sheet. This feature connects the library to Screen 2: Screen 1 lists the local entries, opening one switches the value source to the sheet loaded via `GET /sheets/:id`, and the edit state is keyed by the sheet id.

## Goals

- [ ] Render a "My sheets" section on Screen 1 listing every stored library entry, newest-first, with an empty state.
- [ ] Open a library entry on Screen 2 with exactly one `GET /sheets/:id`, a loading state, and the real stored values.
- [ ] On `NOT_FOUND`, remove the entry from the local library and render a mapped message with no Retry.
- [ ] On network/5xx, render the mapped message with Retry and never remove the entry.
- [ ] Remove an entry from the local library behind an explicit confirmation, issuing zero API requests.
- [ ] Retarget the edit state from template id to sheet id, seeded from `Sheet.data` with the schema default as per-key fallback.
- [ ] Keep one active sheet at a time and keep the existing discard guards (back / F5) working on the opened sheet.
- [ ] Replace the temporary "Preview sheet" action; Screen 2 always renders a stored sheet.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Save / `PATCH` / cascade merge / dirty footer | Separate Fase 4 card ("Salvar com delta e merge incremental da cascata") |
| Create sheet (`POST /sheets`) | LGN-109, already shipped; this card only reuses its `201` → Screen 2 hand-off |
| Tabs / multiple simultaneous sheets | V2 per TDD Questão #5; V1 keeps one active sheet |
| API `DELETE /sheets/:id` | Backend Phase 2; V1 removal is local-only and the confirmation says so |
| Auth, ownership, multi-user | V2 |
| Caching the sheet `data` or template content locally | TDD: the API is the single source of truth (LIB-08); opening always reloads via `GET` |
| Persisting the last-open sheet / edit state across restarts | Not requested; edit state stays in memory (GRN-06) |
| New catalog features (search, genres) | Phase 2 scope, unchanged |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | --------------- | --------- | ---------- |
| Where the open request runs | Screen 2 runs the single `GET /sheets/:id` and owns the loading / error / Retry states | Card title ("GET /sheets/:id na Tela 2") and AC "Abrir ficha da biblioteca → 1 `GET /sheets/:id`, estado de carregando e depois os valores reais" | y |
| UI on `NOT_FOUND` | Stay on Screen 2: remove the entry from the local library and render the mapped missing-sheet message with no Retry | User decision (2026-10-03): the sheet no longer exists, so a Retry would only re-find the 404 | y |
| UI on network/5xx | Stay on Screen 2 with the mapped message + Retry; the entry stays in the library | Card AC ("mensagem mapeada + Retry, sem remover da biblioteca") | y |
| Fate of the "Preview sheet" action | Removed; Screen 2 always requires a stored sheet id | User decision (2026-10-03): Fase 3 anticipated Phase 4 replacing preview with the real open flow | y |
| Edit-state family key | `sheetEditControllerProvider` keyed by the **sheet id**; the creation flow keeps injecting the `201` payload through `openedSheetProvider` so no second GET is issued | Card scope ("Retarget … de templateId para sheet id"); AD-004 seam reused | y |
| Seeding when a schema key is missing from `Sheet.data` | Seed every schema leaf from the schema default, then overlay the sheet's stored values; a stored value overrides its default, a missing key keeps the default | Card scope ("fallback ao default do schema quando a chave faltar") | y |
| Template detail fetch for a library sheet | Screen 2 issues the existing `GET /templates/:id` once the loaded sheet's `template_id` is known, to render the fields; the card's "1 GET /sheets/:id" constrains only the sheet request | Screen 2 renders schema-driven fields; only the sheet `data` comes from `GET /sheets/:id` | n |
| How many sheets can be open | One: opening is only possible from Screen 1, so a pushed Screen 2 route is the only active sheet; autoDispose discards its state on pop | TDD Questão #5; no tabs in V1 | n |
| Removal confirmation copy | A dialog stating the exact consequence ("removed from this app only; it stays in the API"), cancel is safe | `desktop/AGENTS.md`: destructive actions state the consequence and never confirm by Enter alone | n |
| Library write failure on removal | Keep the entry (the provider leaves state at the last persisted index, LIB-11) and render an inline local-library error | The library is best-effort; the UI must not claim a removal that did not persist | n |
| What the "My sheets" row shows | Name plus template id, with Open and Remove actions | Wireframe Tela 1 ("Meu Personagem (fantasy_v1) [abrir]"); template id disambiguates same-named sheets | n |
| Screen 1 layout when the library is long | The library section and the catalog share one scroll region so the catalog is never pushed off screen | `desktop/AGENTS.md` dense layout; avoids nested scrollables | n |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: "My sheets" list on Screen 1 ⭐ MVP

**User Story**: As the app user, I want to see the sheets I created listed on Screen 1 so that I can reopen one.

**Why P1**: The library index (LGN-107) is invisible without a surface; the wireframe places "Minhas fichas" on Tela 1.

**Acceptance Criteria** (each line is one EARS pattern):

1. WHEN Screen 1 is shown THEN the system SHALL render a "My sheets" section listing every stored library entry, newest first.  <!-- event-driven -->
2. WHEN the local library is empty THEN the system SHALL render the section with a single localized empty-state line and no entry rows.  <!-- event-driven -->
3. WHEN an entry is listed THEN the system SHALL render its `name` and its `template_id`, plus an Open and a Remove action.  <!-- event-driven -->

**Independent Test**: Seed the local store with two entries and mount Screen 1; assert two rows in the section in `created_at` order with their names, template ids and both actions; empty the store and assert the single empty-state line and no rows.

---

### P1: Open a library sheet on Screen 2 ⭐ MVP

**User Story**: As the app user, I want to open a library sheet and edit its real stored values so that the screen shows the sheet, not schema defaults.

**Why P1**: This is the vertical slice the card asks for; it replaces the preview-only Screen 2.

**Acceptance Criteria**:

1. WHEN an entry's Open action is activated THEN the system SHALL navigate to Screen 2 for that entry's sheet id.  <!-- event-driven -->
2. WHEN Screen 2 opens for a library sheet THEN the system SHALL issue exactly one `GET /sheets/:id` and SHALL show a loading state until the response settles.  <!-- event-driven -->
3. WHEN `GET /sheets/:id` responds `200` THEN Screen 2 SHALL render the sheet `name` as the title and the sheet's stored values over the template schema defaults.  <!-- event-driven -->
4. WHEN Screen 2 seeds the opened sheet THEN every schema leaf SHALL take the sheet's stored value when present and the schema default when the key is missing.  <!-- event-driven -->
5. WHEN Screen 2 is opened for a library sheet and the response settles THEN the edit state SHALL report not pending.  <!-- event-driven -->
6. IF `GET /sheets/:id` responds `NOT_FOUND` THEN the system SHALL remove that entry from the local library, render the mapped missing-sheet message, and SHALL NOT render a Retry action.  <!-- unwanted-behavior -->
7. IF `GET /sheets/:id` fails for a reason other than `NOT_FOUND` (network / 5xx / parse) THEN the system SHALL keep the entry in the library, keep Screen 2 open, and render the mapped message with a Retry action.  <!-- unwanted-behavior -->
8. WHEN Retry is activated after a non-`NOT_FOUND` failure THEN the system SHALL issue exactly one further `GET /sheets/:id` (single-flight, never two concurrent attempts).  <!-- event-driven -->
9. WHEN the creation flow opens the created sheet THEN the system SHALL render it from the injected `201` payload and SHALL NOT issue a `GET /sheets/:id` for that sheet.  <!-- event-driven -->

**Independent Test**: With a mocked transport, tap Open on a library entry and assert exactly one `GET /sheets/:id`, a visible loading state, then the sheet's name and a stored value different from the schema default; repeat with a `404` and assert the entry left the library and no Retry renders; repeat with a `500`, assert the entry remains and Retry issues one further GET; create a sheet and assert Screen 2 renders it with zero `/sheets/:id` GETs.

---

### P1: Remove an entry from the local library

**User Story**: As the app user, I want to remove a library entry behind a confirmation so that I do not delete it by accident, knowing it stays in the API.

**Why P1**: Card scope; the library is the only "delete" available in V1 because the API has no `DELETE`.

**Acceptance Criteria**:

1. WHEN an entry's Remove action is activated THEN the system SHALL show a confirmation dialog stating that the removal is local and the sheet stays in the API.  <!-- event-driven -->
2. WHEN the removal confirmation is confirmed THEN the system SHALL remove the entry from the local library and SHALL issue zero API requests.  <!-- event-driven -->
3. IF the removal confirmation is dismissed THEN the system SHALL keep the entry and issue zero API requests.  <!-- unwanted-behavior -->
4. IF the local library write fails during removal THEN the system SHALL keep the entry and render an inline local-library error.  <!-- unwanted-behavior -->

**Independent Test**: Tap Remove, assert the dialog and its exact copy; confirm and assert the row disappeared with zero recorded requests; repeat and cancel asserting the row remains and zero requests; force a store write failure and assert the row remains with the inline error.

---

### P1: One active sheet and unchanged discard guards

**User Story**: As the app user, I want opening a second sheet to replace the first and my unsaved edits to still be protected, so that the desk-focused single-sheet model holds.

**Why P1**: Card scope ("1 ficha ativa por vez"; "Guards existentes de descarte … valem sobre a ficha aberta").

**Acceptance Criteria**:

1. WHILE a sheet is open on Screen 2, the system SHALL keep exactly one active sheet and SHALL discard that sheet's edit state when the screen is popped.  <!-- state-driven -->
2. WHEN the back action is activated with pending edits on an opened library sheet THEN the system SHALL show the existing discard confirmation before leaving.  <!-- event-driven -->
3. WHEN F5 is pressed with pending edits on an opened library sheet THEN the system SHALL show the existing discard confirmation before reloading the template.  <!-- event-driven -->
4. WHILE Screen 2 shows an opened library sheet the system SHALL NOT issue any mutation request (`POST`/`PATCH`).  <!-- state-driven -->

**Independent Test**: Open a library sheet, edit a field, press back and confirm the existing dialog; repeat with F5; pop and assert the edit state is disposed; assert the transport recorded no mutation.

---

### P1: Preview action replaced

**User Story**: As the app user, I want the catalog to open real sheets only so that Screen 2 never shows fabricated data.

**Why P1**: User decision to remove the temporary preview now that the real open flow exists.

**Acceptance Criteria**:

1. WHEN a catalog card renders THEN the system SHALL NOT render a "Preview sheet" action.  <!-- ubiquitous -->
2. WHILE Screen 2 is mounted the system SHALL require a stored sheet identity and SHALL NOT render template-schema-only fake data as a sheet.  <!-- state-driven -->

**Independent Test**: Mount the catalog and assert no "Preview sheet" action exists; assert the only secondary action is Open/Remove inside the library section.

---

## Edge Cases

Edge cases are usually unwanted-behavior (IF/THEN) or boundary (WHEN) criteria:

- WHEN a stored `Sheet.data` contains a key absent from the template schema, Screen 2 SHALL still seed it and render it does not break the known fields.
- WHEN a template leaf is missing from `Sheet.data`, the field SHALL render the schema default.
- IF the same sheet is opened, popped, and opened again, THEN it SHALL re-issue one `GET /sheets/:id` (no cache) and reseed.
- WHEN the library removal write fails, no API request SHALL be issued.
- WHEN the `NOT_FOUND` removal write itself fails, the mapped missing-sheet message SHALL still render and the app SHALL NOT crash.
- WHEN the template detail request fails after the sheet loaded, Screen 2 SHALL render the mapped message with Retry and keep the entry in the library.
- IF opening a sheet is retried while a request is in flight, THEN exactly one `GET /sheets/:id` SHALL be outstanding.

---

## Requirement Traceability

Each requirement gets a unique ID for tracking across design, tasks, and validation.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| OPN-01 | P1: "My sheets" list on Screen 1 | Execute | Verified |
| OPN-02 | P1: "My sheets" list on Screen 1 | Execute | Verified |
| OPN-03 | P1: "My sheets" list on Screen 1 | Execute | Verified |
| OPN-04 | P1: Open a library sheet on Screen 2 | Execute | Verified |
| OPN-05 | P1: Open a library sheet on Screen 2 | Execute | Verified |
| OPN-06 | P1: Open a library sheet on Screen 2 | Execute | Verified |
| OPN-07 | P1: Open a library sheet on Screen 2 | Execute | Verified |
| OPN-08 | P1: Open a library sheet on Screen 2 | Execute | Verified |
| OPN-09 | P1: Open a library sheet on Screen 2 | Execute | Verified |
| OPN-10 | P1: Open a library sheet on Screen 2 | Execute | Verified |
| OPN-11 | P1: Open a library sheet on Screen 2 | Execute | Verified |
| OPN-12 | P1: Remove an entry from the local library | Execute | Verified |
| OPN-13 | P1: Remove an entry from the local library | Execute | Verified |
| OPN-14 | P1: Remove an entry from the local library | Execute | Verified |
| OPN-15 | P1: Remove an entry from the local library | Execute | Verified |
| OPN-16 | P1: One active sheet and unchanged discard guards | Execute | Verified |
| OPN-17 | P1: One active sheet and unchanged discard guards | Execute | Verified |
| OPN-18 | P1: One active sheet and unchanged discard guards | Execute | Verified |
| OPN-19 | P1: One active sheet and unchanged discard guards | Execute | Verified |
| OPN-20 | P1: Preview action replaced | Execute | Verified |
| OPN-21 | P1: Preview action replaced | Execute | Verified |

**ID format:** `OPN-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 21 total, mapped to tasks at Tasks (see tasks.md).

---

## Success Criteria

How we know the feature is successful:

- [ ] `flutter test` green, including: library list order + empty state, one `GET /sheets/:id` with loading then stored values, `404` removes + mapped message without Retry, `500` keeps + Retry issues one further GET, remove dialog + zero API calls, per-key schema fallback, and unchanged back/F5 discard guards.
- [ ] The creation flow still opens the created sheet with zero `GET /sheets/:id`.
- [ ] No "Preview sheet" action exists in the catalog.
- [ ] No mutation request is issued while a library sheet is open.
- [ ] `flutter analyze` clean.

---

## Implicit-Requirement Dimensions Sweep (Large)

| Dimension | Resolution |
| --------- | ---------- |
| Input validation & bounds | N/A because the only identity input is the library id, already validated at the store boundary (LIB-20); no new free-text input. |
| Failure / partial-failure states | OPN-07, OPN-08, OPN-15, and the edge cases: network/5xx keep state; `NOT_FOUND` removes; removal write failure is reported. |
| Idempotency / retry / duplicate handling | OPN-08, OPN-09: single-flight Retry, exactly one outstanding GET; `NOT_FOUND` removal is one-shot. |
| Auth boundaries & rate limits | N/A because V1 has no auth and the app is single-user (TDD §Segurança). |
| Concurrency / ordering | Single-flight load; sheet GET precedes the template detail watch; autoDispose discards on pop; the template watch starts only after the sheet identity is known. |
| Data lifecycle / expiry | OPN-07: a missing sheet is evicted from the local index; no sheet `data` is ever persisted (LIB-08). |
| Observability | N/A because desktop logging is Fase 5; every failure surfaces as a mapped, localized message. |
| External-dependency failure | OPN-07, OPN-08: `NOT_FOUND` removes; network/5xx offers Retry without losing the entry. |
| State-transition integrity | OPN-05, OPN-06: loading → content/error; the removal confirmation gates the only destructive local action. |
