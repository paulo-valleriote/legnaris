# Create Sheet (POST /sheets) Specification

Source: [LGN-109](https://paulovalleriotedev.atlassian.net/browse/LGN-109) — "[Front End] Spec Fase 4 — Criar ficha (POST /sheets) a partir do diálogo da Tela 3", under epic [LGN-93](https://paulovalleriotedev.atlassian.net/browse/LGN-93).

## Problem Statement

The Screen 3 creation dialog (LGN-97/LGN-100) already renders the template preview and a sheet-name input, but its primary action is hard-disabled ("Sheet creation arrives in a later phase."). No sheet can actually be created from the UI, so the "Minhas fichas" library (LGN-107) never fills and Screen 2 can never show a stored sheet. This feature enables the real creation flow: `POST /sheets`, register the returned sheet in the local library, and open it on Screen 2.

## Goals

- [ ] Enable the creation dialog's primary action and validate the sheet name (required, trimmed).
- [ ] Issue exactly one `POST /sheets {template_id, sheet_name}` per activation, with no duplicate POST on double-click or retry.
- [ ] On `201`, register `{id, name, template_id, created_at}` from the response in the local sheet library (LGN-107).
- [ ] On `201`, open the created sheet on Screen 2 showing the created name and stored values, closing the dialog.
- [ ] On failure, keep the dialog open with the mapped error copy inline and add no library entry.
- [ ] Never persist the sheet `data` payload in the library.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Editing / saving the created sheet (dirty state, Save, PATCH delta, cascade merge) | Separate Fase 4 card ("Salvar com delta e merge incremental da cascata") |
| Loading a sheet from the library via `GET /sheets/:id` on Screen 2, and the "Minhas fichas" list on Screen 1 | Owned by LGN-108 ("Abrir ficha"), still `To Do`; this card passes the `201` payload directly into Screen 2 instead |
| Sheet-name duplication checks and sheet-count limits | V1 has neither (card §Fora do escopo) |
| Import / export | Out of the epic scope |
| Removing a library entry / `DELETE /sheets/:id` | LGN-107 removal is local-only; not part of this flow |
| Caching the sheet `data` or template content in the local store | `docs/TDD-desktop-mvp.md`: the API is the single source of truth; the library stores identity only (LIB-08) |
| Auth, multi-user, secure storage | V2 |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | -------------- | --------- | ---------- |
| How the created sheet reaches Screen 2 | The `201` `Sheet` object is injected into Screen 2 through an overridable `openedSheetProvider`; Screen 2 makes **no** `GET /sheets/:id` | User decision (2026-10-03): the `201` already carries the full sheet, and LGN-108 (the `GET`-based open) is still `To Do`; duplicating its mechanism here would collide with it | y |
| Primary action when the name is empty | Disabled, with a tooltip stating a sheet name is required | `desktop/AGENTS.md`: "disabled primary actions explain themselves via tooltip"; also makes "no POST on empty name" structural | n |
| Name normalization | Trim leading/trailing whitespace; a whitespace-only name counts as empty | Prevents a persisted name of spaces and a meaningless POST | n |
| Inline error placement | Inside the dialog content, directly above the action row | Pins the placement so the rendered-copy assertion is anchored (lesson L-004) | n |
| Retry affordance after a failure | The same primary action; each activation issues exactly one POST | Card AC: "permanece aberto + Retry"; avoids a second competing button | n |
| Local library write fails after a `201` | Still open the created sheet; never issue a second POST | The `201` is the irreversible operation; re-POSTing would create a duplicate sheet. The local index is best-effort | n |
| Does opening Screen 2 still fetch the template schema? | Yes: one `GET /templates/:id` to render the fields; only the sheet `data` comes from the `201` | Screen 2 renders schema-driven fields; only one sheet id exists at a time, so the template detail family is reused unchanged | n |
| Which values the library and Screen 2 display | The `201` response's `name`, `id`, `template_id`, `created_at` and `data` (server-authoritative) | The API computes derived/seed values; the client must not substitute its own input | n |
| The created sheet's `data` is not persisted | The library upsert stores identity only (four fields) | LIB-08 / TDD §Persistência Local | y |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Create a sheet from the creation dialog ⭐ MVP

**User Story**: As the app user, I want the creation dialog's primary action to actually create the sheet so that a template selection turns into a stored sheet.

**Why P1**: This is the vertical slice the card asks for; without it the create flow is dead and the library never fills.

**Acceptance Criteria** (each line is one EARS pattern):

1. WHEN the primary action is activated while the trimmed sheet name is non-empty, the system SHALL issue exactly one `POST /sheets` with `{template_id, sheet_name}` where `sheet_name` is the trimmed name  <!-- event-driven -->
2. WHILE a create request is in flight, the system SHALL keep the primary action disabled so a second activation issues no POST  <!-- state-driven -->
3. WHEN `POST /sheets` responds `201`, the system SHALL upsert `{id, name, template_id, created_at}` from the response into the local sheet library  <!-- event-driven -->
4. WHEN a sheet is registered from a `201`, the system SHALL close the dialog and open Screen 2 for the created sheet  <!-- event-driven -->
5. WHILE the trimmed sheet name is empty, the system SHALL keep the primary action disabled and SHALL expose a tooltip stating a sheet name is required  <!-- state-driven -->
6. IF `POST /sheets` fails, THEN the system SHALL keep the dialog open, render the mapped error copy inline above the action row, and add no entry to the local sheet library  <!-- unwanted-behavior -->
7. WHEN the primary action is activated again after a failed create, the system SHALL issue exactly one further `POST /sheets`  <!-- event-driven -->
8. IF the local library write fails after a `201`, THEN the system SHALL still open the created sheet and SHALL NOT issue a second `POST /sheets`  <!-- unwanted-behavior -->

**Independent Test**: With a mocked `http.Client`, open the dialog, prove the action is inert with an empty name, type a name, activate once, and assert exactly one `POST /sheets` whose body carries the trimmed name; assert the library index gained the four fields from the response; assert the dialog closed and Screen 2 is mounted. Repeat with a failing response and assert the dialog stays, the mapped copy renders, and the index stayed empty; then retry and assert exactly one further POST.

---

### P1: Open the created sheet on Screen 2 with its stored values

**User Story**: As the app user, I want the sheet I just created to open showing the values the server stored so that I can start from the real sheet, not from schema defaults.

**Why P1**: "Ficha aberta na Tela 2" is part of the card's acceptance; opening the wrong values would be worse than not opening at all.

**Acceptance Criteria**:

1. WHEN Screen 2 is opened for a created sheet, the system SHALL seed the edit-state baseline and working data from the sheet's `data`, not from the template schema defaults  <!-- event-driven -->
2. WHILE Screen 2 shows a created sheet, the system SHALL render the sheet's `name` as the screen title  <!-- state-driven -->
3. WHEN Screen 2 is opened for a created sheet, the system SHALL report the edit state as not pending  <!-- event-driven -->

**Independent Test**: Mount Screen 2 with an injected `Sheet` whose `data.stats.dexterity` differs from the template schema default; assert the rendered control shows the sheet's value and the edit state is not pending; assert the header shows the sheet name.

---

## Edge Cases

Edge cases are usually unwanted-behavior (IF/THEN) or boundary (WHEN) criteria:

- WHEN the supplied name has leading or trailing whitespace, the system SHALL POST the trimmed value.
- WHEN the primary action is activated twice while a request is in flight, the system SHALL issue exactly one `POST /sheets`.
- IF the name is whitespace-only, THEN the system SHALL treat it as empty and issue no POST.
- WHEN the `201` response's `name` differs from the submitted name, the system SHALL register and display the response's values (server-authoritative).
- IF the local library write fails after a `201`, THEN the local index is left unchanged and no second POST is issued.
- WHEN the created sheet's `data` holds a value at a leaf path, Screen 2 SHALL render that value over the template schema default.

---

## Requirement Traceability

Each requirement gets a unique ID for tracking across design, tasks, and validation.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| CRT-01 | P1: Create a sheet from the creation dialog | Execute | Implementing |
| CRT-02 | P1: Create a sheet from the creation dialog | Execute | Implementing |
| CRT-03 | P1: Create a sheet from the creation dialog | Execute | Implementing |
| CRT-04 | P1: Create a sheet from the creation dialog | Execute | Pending |
| CRT-05 | P1: Create a sheet from the creation dialog | Execute | Implementing |
| CRT-06 | P1: Create a sheet from the creation dialog | Execute | Implementing |
| CRT-07 | P1: Create a sheet from the creation dialog | Execute | Implementing |
| CRT-08 | P1: Open the created sheet on Screen 2 with its stored values | Execute | Implementing |
| CRT-09 | P1: Open the created sheet on Screen 2 with its stored values | Execute | Implementing |
| CRT-10 | P1: Open the created sheet on Screen 2 with its stored values | Execute | Implementing |
| CRT-11 | P1: Create a sheet from the creation dialog | Execute | Implementing |

**ID format:** `CRT-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 11 total, 11 mapped to tasks, 0 unmapped ⚠️ (mapped at Tasks).

---

## Success Criteria

How we know the feature is successful:

- [ ] `flutter test` green, including: empty-name inert action, exactly-one-POST, library upsert on `201`, dialog closes and Screen 2 opens, mapped inline error with no library entry, retry issues one further POST, and injected-data seeding on Screen 2.
- [ ] The persisted library document holds only `{id, name, template_id, created_at}` and no `data`.
- [ ] A failed create leaves the dialog open and the library index unchanged.
