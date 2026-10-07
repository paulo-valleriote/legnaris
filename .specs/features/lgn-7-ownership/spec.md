# Fase 2 — Ownership: autorização e limite de 1 ficha por usuário Specification

Source: epic [LGN-7](https://paulovalleriotedev.atlassian.net/browse/LGN-7) with children LGN-19, LGN-20, LGN-21, LGN-22. Depends on Fase 1 (LGN-4, Done): the JWT middleware already exists (`internal/auth/middleware.go`) but is not yet applied to any route. Contract source: `docs/TDD.md` §Autenticação, §APIs & Endpoints, §Mudanças no Banco de Dados, §Critérios de Aceite.

## Problem Statement

The sheets API is public and ownerless: `Sheet` has no `owner_id`, every `/sheets` and `/templates` route is reachable without a token, and `GET /sheets` / `DELETE /sheets/:id` do not exist. Fase 1 shipped authentication but nothing enforces it, so any client that knows a sheet UUID can read, edit, or recompute it. Fase 2 makes a sheet belong to exactly one user, caps that user at one sheet at the database level, and closes every sheets route behind JWT.

## Goals

- [ ] Persist `owner_id` on every sheet and enforce one sheet per owner with a unique Mongo index.
- [ ] Second `POST /sheets` by the same user returns `409 SHEET_LIMIT_REACHED`, race-safe against concurrent creation.
- [ ] `GET /sheets` returns the authenticated user's sheet (`404` when the user owns none).
- [ ] `GET`/`PATCH`/`DELETE`/preview on `/sheets/:id` are owner-guarded: another user's sheet is indistinguishable from a nonexistent one (`404`).
- [ ] `DELETE /sheets/:id` removes the owner's sheet and returns `204`.
- [ ] Every `/sheets*` and `/templates*` route requires a valid JWT (`401` otherwise); only `/health`, `/metrics`, `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout` stay public.
- [ ] Marco M1: the backend is usable end to end with `fantasy_v1` under auth + ownership.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Multiple sheets per user | V2 per TDD; V1 is strictly one sheet per user |
| Sheet sharing / "table mode" | V2 |
| Admin profile with unrestricted access | Not in the epic; no role model exists |
| Data migration of existing dev sheets | TDD §Deploy prefers a dev-DB reset; no production rows exist (Fase 6 pending) |
| CORS and rate limiting | Fase 5 |
| Desktop/Flutter client adaptation | Tracked as the companion feature `front-end-phase-6-auth`; this spec is backend-only |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | -------------- | --------- | ---------- |
| Response for another user's sheet | `404 NOT_FOUND`, byte-identical to a nonexistent sheet, on `GET`/`PATCH`/`DELETE`/preview | LGN-22: "sem vazar informação sobre a existência do recurso"; user decision (2026-10-07) | y |
| `DELETE /sheets/:id` in scope | Implemented in this feature (route + repository + service + tests) | LGN-22 and TDD endpoint table list it in Fase 2; user decision (2026-10-07) | y |
| `GET /sheets` when the user owns none | `404 NOT_FOUND` | LGN-20: "404 quando não possui" | y |
| Conflict error code | New `apperr.CodeSheetLimitReached = "SHEET_LIMIT_REACHED"` mapped to `409` | TDD response example uses `SHEET_LIMIT_REACHED` (not the generic `CONFLICT`) | y |
| Duplicate-key detection | Repository maps Mongo duplicate-key (code 11000) on `sheets.owner_id` to typed `ErrSheetOwnerExists`; service maps it to `409 SHEET_LIMIT_REACHED` | Keeps `apperr` out of the repository layer; the unique index is the race guard | y |
| Pre-check before insert | Service also does a best-effort `FetchByOwnerID` for a friendly 409; the unique index remains the authoritative guard | The pre-check improves the common path message; only the index closes the race | n |
| Ownership check placement | Service fetches the sheet by id, then compares `owner_id`; mismatch is returned as the repository's `ErrSheetNotFound` translation (`404`) | Single translation path; no new error code; the check happens before any recalculation or write | y |
| Owner id source | `auth.UserID(c)` populated by the JWT middleware; the controller builds the command's `OwnerID` | Fase 1 already stores the user id at `auth.UserIDContextKey` | y |
| `owner_id` in the API response | Never exposed; `SheetDTO` stays as-is | The client already knows its identity; leaking owner ids adds nothing | n |
| Missing user id at the service boundary | Treated as a `401 UNAUTHORIZED` (defense in depth) even though the middleware guarantees it | A route accidentally mounted without the middleware must fail closed, not operate ownerless | n |
| Concurrency test strategy | Integration test fires two concurrent `POST /sheets` for one user against real Mongo and asserts exactly one `201` and one `409` | Proves the index guard, not the pre-check | y |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Owner-scoped sheet persistence (LGN-19) ⭐ MVP

**User Story**: As the platform, I want every sheet to carry its owner and to enforce a single sheet per owner in the database so that concurrent creation cannot produce two sheets.

**Why P1**: The unique index is the durable guarantee the rest of Fase 2 relies on; the model change is a prerequisite for every guarded route.

**Acceptance Criteria** (each line is one EARS pattern):

1. The `Sheet` model SHALL persist an `owner_id` string field in the `sheets` collection.  <!-- ubiquitous -->
2. WHEN the application boots THEN it SHALL ensure a unique index on `sheets.owner_id` before accepting traffic.  <!-- event-driven -->
3. IF a create targets an owner that already owns a sheet THEN the repository SHALL reject it with the typed duplicate-owner error and the API SHALL respond `409` with code `SHEET_LIMIT_REACHED`.  <!-- unwanted-behavior -->
4. WHEN two creates for the same owner run concurrently THEN exactly one SHALL persist and the other SHALL respond `409` with code `SHEET_LIMIT_REACHED`.  <!-- complex (WHEN+IF) -->

**Independent Test**: Boot the router against Mongo, fire two concurrent `POST /sheets` for one authenticated user, assert one `201` and one `409 SHEET_LIMIT_REACHED` and exactly one persisted document; assert the index exists on `sheets.owner_id`.

---

### P1: Create and fetch the authenticated user's sheet (LGN-20) ⭐ MVP

**User Story**: As an authenticated player, I want to create my sheet from a template and read it back, knowing I can only own one, so that my character persists under my identity.

**Why P1**: This is the vertical slice that turns the public CRUD into an owner-scoped resource.

**Acceptance Criteria**:

1. WHEN an authenticated user POSTs a valid `template_id` THEN the system SHALL persist the sheet with `owner_id` equal to the token's user id and SHALL respond `201`.  <!-- event-driven -->
2. IF an authenticated user that already owns a sheet POSTs `/sheets` THEN the system SHALL respond `409` with code `SHEET_LIMIT_REACHED` and SHALL persist nothing.  <!-- unwanted-behavior -->
3. WHEN an authenticated user GETs `/sheets` THEN the system SHALL return exactly that user's sheet.  <!-- event-driven -->
4. IF the authenticated user owns no sheet THEN the system SHALL respond `404` with code `NOT_FOUND`.  <!-- unwanted-behavior -->
5. The system SHALL never return a sheet owned by another user from `GET /sheets`.  <!-- ubiquitous -->

**Independent Test**: Register user A and B, create a sheet for A, assert `GET /sheets` as A returns it (with A as owner, unexposed) and as B returns `404`; assert A's second `POST` is `409` and no second document exists.

---

### P1: Ownership guard on sheet operations (LGN-22) ⭐ MVP

**User Story**: As a player, I want nobody but me to view, edit, recompute, or delete my sheet so that my character stays mine.

**Why P1**: Ownership is only real if every operation that touches a sheet checks possession.

**Acceptance Criteria**:

1. WHEN the authenticated owner requests `GET`/`PATCH`/`DELETE`/`POST /sheets/:id/preview` THEN the system SHALL perform the operation for that owner.  <!-- event-driven -->
2. IF the sheet exists but is owned by another user THEN the system SHALL respond `404` with code `NOT_FOUND`, identical to a nonexistent sheet.  <!-- unwanted-behavior -->
3. IF the sheet does not exist THEN the system SHALL respond `404` with code `NOT_FOUND`.  <!-- unwanted-behavior -->
4. WHEN the owner DELETEs their sheet THEN the system SHALL remove it, SHALL respond `204`, and a subsequent `GET` SHALL respond `404`.  <!-- event-driven -->
5. IF a request targets another user's sheet THEN the system SHALL NOT recompute or persist any value for that sheet.  <!-- unwanted-behavior -->

**Independent Test**: Create a sheet for A; assert `GET`/`PATCH`/`DELETE`/`preview` as B each return `404` and leave A's stored `data` untouched; assert A's `DELETE` returns `204` and A's following `GET` returns `404`.

---

### P1: Protected routes with JWT (LGN-21) ⭐ MVP

**User Story**: As the platform, I want every sheets and templates route behind the JWT middleware so that unauthenticated callers cannot reach the domain.

**Why P1**: The epic's marco M1 requires the whole API surface to be auth-gated; the middleware exists but is unwired.

**Acceptance Criteria**:

1. The system SHALL require a valid bearer token on every `/sheets*` and `/templates*` route.  <!-- ubiquitous -->
2. IF a protected request carries no token or an invalid/expired token THEN the system SHALL respond `401` with code `UNAUTHORIZED` and SHALL NOT reach the service.  <!-- unwanted-behavior -->
3. The system SHALL keep `/health`, `/metrics`, `/auth/register`, `/auth/login`, `/auth/refresh`, and `/auth/logout` reachable without a token.  <!-- ubiquitous -->
4. WHEN a protected request carries a valid token THEN the system SHALL reach the normal handler with the user id resolved from the token.  <!-- event-driven -->

**Independent Test**: Call `POST /sheets`, `GET /sheets`, `GET /templates`, and `GET /templates/:id` with no token and assert `401` with zero repository writes; assert `/health` and `/metrics` still return `200`; repeat the protected calls with a valid token and assert normal responses.

---

## Edge Cases

Edge cases are usually unwanted-behavior (IF/THEN) or boundary (WHEN) criteria:

- IF the request context carries no authenticated user id THEN the sheet service SHALL reject with a `401 UNAUTHORIZED` and SHALL perform no repository access.  <!-- unwanted-behavior -->
- IF `POST /sheets` carries a malformed or unknown `template_id` while authenticated THEN the system SHALL preserve the existing `400 INVALID_REQUEST` / `404 NOT_FOUND` template behavior.  <!-- unwanted-behavior -->
- WHEN the owner requests `POST /sheets/:id/preview` on their own sheet THEN the system SHALL return the cascade and SHALL persist nothing.  <!-- event-driven -->
- IF a Mongo duplicate-key error is returned for any document marked as a sheet limit conflict THEN the system SHALL map it to `409 SHEET_LIMIT_REACHED`, never to a generic `500`.  <!-- unwanted-behavior -->
- WHEN `DELETE /sheets/:id` is called twice for the same owner THEN the first SHALL return `204` and the second SHALL return `404`.  <!-- event-driven -->
- WHEN `owner_id` is absent from a stored document (legacy row) THEN the system SHALL treat it as owned by nobody and SHALL respond `404` to any owner request.  <!-- event-driven -->

---

## Requirement Traceability

Each requirement gets a unique ID for tracking across design, tasks, and validation.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| OWN-01 | P1: Owner-scoped sheet persistence | Execute | Implementing |
| OWN-02 | P1: Owner-scoped sheet persistence | Design | Pending |
| OWN-03 | P1: Owner-scoped sheet persistence | Design | Pending |
| OWN-04 | P1: Owner-scoped sheet persistence | Design | Pending |
| OWN-05 | P1: Create and fetch the authenticated user's sheet | Design | Pending |
| OWN-06 | P1: Create and fetch the authenticated user's sheet | Design | Pending |
| OWN-07 | P1: Create and fetch the authenticated user's sheet | Design | Pending |
| OWN-08 | P1: Create and fetch the authenticated user's sheet | Design | Pending |
| OWN-09 | P1: Create and fetch the authenticated user's sheet | Design | Pending |
| OWN-10 | P1: Ownership guard on sheet operations | Design | Pending |
| OWN-11 | P1: Ownership guard on sheet operations | Design | Pending |
| OWN-12 | P1: Ownership guard on sheet operations | Design | Pending |
| OWN-13 | P1: Ownership guard on sheet operations | Design | Pending |
| OWN-14 | P1: Ownership guard on sheet operations | Design | Pending |
| OWN-15 | P1: Protected routes with JWT | Design | Pending |
| OWN-16 | P1: Protected routes with JWT | Design | Pending |
| OWN-17 | P1: Protected routes with JWT | Design | Pending |
| OWN-18 | P1: Protected routes with JWT | Design | Pending |
| OWN-19 | Edge: missing user id at the boundary | Design | Unmapped ⚠️ |
| OWN-20 | Edge: template validation preserved | Design | Unmapped ⚠️ |
| OWN-21 | Edge: owner preview persists nothing | Design | Unmapped ⚠️ |
| OWN-22 | Edge: duplicate-key maps to 409 | Design | Unmapped ⚠️ |
| OWN-23 | Edge: delete is idempotent-by-404 | Design | Unmapped ⚠️ |
| OWN-24 | Edge: legacy row without owner | Design | Unmapped ⚠️ |

**ID format:** `OWN-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 24 total, mapped to tasks at Tasks (see tasks.md).

---

## Success Criteria

How we know the feature is successful:

- [ ] Marco M1 works end to end: register → login → create `fantasy_v1` sheet → edit → `409` on a second create → `404` on another user's sheet → `204` delete.
- [ ] Two concurrent creates for one owner produce exactly one sheet (integration test with real Mongo).
- [ ] Every `/sheets*` and `/templates*` request without a token is `401`.
- [ ] `make test-race`, `make lint`, and `make vet` are clean.

---

## Implicit-Requirement Dimensions Sweep (Large)

| Dimension | Resolution |
| --------- | ---------- |
| Input validation & bounds | OWN-20 keeps existing template validation; owner id comes from a validated JWT (Fase 1). |
| Failure / partial-failure states | OWN-03, OWN-06, OWN-08, OWN-12, OWN-13, OWN-19, OWN-24 define every failure response. |
| Idempotency / retry / duplicate handling | OWN-03, OWN-04, OWN-06, OWN-22, OWN-23: the unique index dedupes creation; delete is guarded by the `404`-after-delete rule. |
| Auth boundaries & rate limits | OWN-15…OWN-18: JWT required on `/sheets*` and `/templates*`; rate limiting is Fase 5 (out of scope). |
| Concurrency / ordering | OWN-04: concurrent creates race the unique index; the guard runs before any write. |
| Data lifecycle / expiry | OWN-13, OWN-23: delete removes the document; no TTL on sheets. |
| Observability | Existing metrics middleware records the mapped status per route; creation and delete already log via the repository. |
| External-dependency failure | Mongo errors are translated at the repository/service boundary (OWN-22); no other external dependency. |
| State-transition integrity | OWN-05, OWN-13: create transitions the owner from no-sheet to one-sheet; delete transitions back; both are guarded. |
