# Local Sheet Library (Local Store) Specification

Source: [LGN-107](https://paulovalleriotedev.atlassian.net/browse/LGN-107) — "[Front End] Spec Fase 4 — Local Store: biblioteca local de fichas (índice + schema_version)".

## Problem Statement

The desktop app persists nothing today: the local sheet library ("Minhas fichas") described in `docs/TDD-desktop-mvp.md` §Persistência Local does not exist. Without it, Screen 1 has no list of sheets to show and the open flow has no source of sheet IDs, because the V1 API exposes no `GET /sheets` (list) endpoint. This feature adds a versioned local index of created sheets as the single source of the "Minhas fichas" list — the sheet `data` itself is never cached locally.

## Goals

- [ ] Persist an index of entries `{id, name, template_id, created_at}` with a store `schema_version`, on disk, and load it at startup.
- [ ] Support add/update (upsert), list, and remove over the index.
- [ ] Never persist the sheet `data` payload or any cell/derived value.
- [ ] Expose the index through Riverpod read and write providers.
- [ ] Recover from a corrupt or unsupported store without crashing, preserving the bad file for manual recovery.
- [ ] Surface persistence failures to the caller while keeping the in-memory index consistent with what is persisted.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Library UI (list, open, remove on Screen 1) | Separate spec "Abrir ficha" (LGN-93 epic) |
| Create / open / save flows (`POST`/`GET`/`PATCH`), cascade merge, dirty state | Other Phase 4 cards under epic LGN-93; this card is the store only |
| Caching the sheet `data` or template content | TDD: the API is the single source of truth; `GET` reloads on open |
| Dev base-URL override and UI preferences in the Local Store | TDD lists them in the store, but this card scopes the store to the sheet index; revisit in a settings card |
| Persisting across the sheet-open lifecycle (dirty state, last open sheet) | Owned by the open/save specs |
| API `DELETE /sheets/:id` | Backend Phase 2; V1 removal is local-only and documented in the UI |
| Auth, token storage, secure storage, migration to the authenticated list | V2 (TDD §Segurança) |
| Multiple simultaneous active sheets / tabs | V2 per TDD |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | --------------- | --------- | ---------- |
| Storage backend | Design chose `path_provider` (`getApplicationSupportDirectory`) + a `dart:io` JSON file behind an injectable persistence boundary; tests inject a temporary directory and never hit the platform channel | The card defers the storage decision to Design; `path_provider` is the strictly necessary first-party package for a portable writable app directory (`desktop/AGENTS.md`) | y |
| Store file location and name | `sheet_library.json` in the OS application-support directory | Design decision; platform-specific path resolution stays inside the store | y |
| Current `schema_version` value | `1` | First versioned format; the migration registry is keyed by the version being migrated from | n |
| Add on an existing `id` | Upsert: replace `name`, `template_id`, and `created_at` of the existing entry, never append a second row | The card lists "adicionar/atualizar" as one operation and requires "id já existente → não duplica" | n |
| List ordering | `created_at` descending; ties broken by reverse insertion order | "Minhas fichas" reads newest-first; the store must still return a deterministic order | n |
| `created_at` representation | ISO-8601 UTC string, taken from the API sheet's `created_at` when it is registered | The API returns that value; storing it verbatim avoids a second timestamp format in the app | n |
| Write failure behavior | Write-through: persist first; on failure leave the in-memory index at its pre-mutation value and report the failure to the caller | User decision (discuss, 2026-10-02) | y |
| Corrupt or unknown store recovery | Quarantine the file under a distinct name (keep the original bytes), then start with an empty index | User decision (discuss, 2026-10-02): preserve the bad file for manual recovery | y |
| Newer `schema_version` than the build supports | Same as corrupt: quarantine and start empty, without attempting to parse | User decision extends to unsupported versions; guessing a newer format risks silent data loss | y |
| Logging / surfacing recovery | The store reports a recovery outcome (`loaded` / `quarantined` / `empty`) and the app decides where to log or show it; the store never logs sheet content | No desktop logging mechanism exists yet; reporting the outcome keeps the AC testable without inventing infrastructure | n |
| Atomicity mechanism | Temporary file `sheet_library.json.tmp` + `rename` over the target | `dart:io` `File.rename` removes an existing destination first, so the replace is atomic on the same filesystem (verified) | y |
| Concrete quarantine name pattern | `sheet_library.json.corrupt-<UTC compact timestamp>`, same directory | Design decision; distinct per event, preserves the original bytes | y |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Versioned local sheet index (add / list / remove / get) ⭐ MVP

**User Story**: As the desktop app, I want a local index of created sheets so that "Minhas fichas" has a list and opening a sheet has a source of IDs without a `GET /sheets` endpoint.

**Why P1**: The API has no list endpoint in V1, so without this index the library surface cannot exist.

**Acceptance Criteria** (each line is one EARS pattern):

1. WHEN the app loads a store that has never been written (absent or empty), the system SHALL expose an empty index without error  <!-- event-driven -->
2. WHEN a new entry with an `id` absent from the index is added, the system SHALL store `{id, name, template_id, created_at}` and persist the full index  <!-- event-driven -->
3. WHEN an entry is added whose `id` is already present, the system SHALL replace the existing entry's `name`, `template_id`, and `created_at` instead of creating a second entry  <!-- event-driven -->
4. WHEN the index is listed, the system SHALL return every stored entry ordered by `created_at` descending, breaking ties by reverse insertion order  <!-- event-driven -->
5. WHEN an entry `id` is removed and the `id` exists, the system SHALL remove that entry and persist the full index  <!-- event-driven -->
6. IF a removal targets an `id` absent from the index, THEN the system SHALL complete as a no-op without changing the persisted revision or raising an error  <!-- unwanted-behavior -->
7. WHEN a single `id` is looked up, the system SHALL return the matching entry or null  <!-- event-driven -->
8. The system SHALL NOT persist any sheet `data` payload, cell value, or derived value in the store  <!-- ubiquitous -->

**Independent Test**: With an in-memory persistence fake, add two entries, add a duplicate id, list (assert order), get one id, remove one id, remove a missing id, and assert the persisted document contains only the four index fields and no `data`.

---

### P1: Persisted schema version and durable writes

**User Story**: As a maintainer, I want the store versioned and written atomically so that a future format change can migrate without losing the index and a failed write never corrupts it.

**Why P1**: The card requires `schema_version` in the persisted file and safe recovery; durability is the foundation for any future migration.

**Acceptance Criteria**:

1. The persisted store SHALL carry a top-level integer `schema_version` equal to the current version on every write  <!-- ubiquitous -->
2. WHEN a mutation (add/update/remove) is applied, the system SHALL persist the complete index before reporting success to the caller  <!-- event-driven -->
3. IF persisting a mutation fails, THEN the system SHALL leave the in-memory index equal to its pre-mutation value and report the failure to the caller  <!-- unwanted-behavior -->
4. The system SHALL persist the store through an atomic replace (write to a temporary file, then rename) so a partial write never leaves a truncated store  <!-- ubiquitous -->
5. The system SHALL serialize concurrent mutations so each persisted store reflects a complete index  <!-- ubiquitous -->

**Independent Test**: Force the persistence fake to fail a write; assert the in-memory index is unchanged and the error is reported. Assert the write path uses a temp-then-rename sequence and that two interleaved mutations leave a parseable store.

---

### P1: Safe recovery across schema versions

**User Story**: As a user, I want a corrupt or future-version store to never crash the app, preserving the bad file so I can recover manually if needed.

**Why P1**: The card's acceptance criteria and the TDD rollback plan both require versioned recovery without data loss on crash.

**Acceptance Criteria**:

1. WHEN the persisted store is valid and its `schema_version` equals the current version, the system SHALL load its entries unchanged  <!-- event-driven -->
2. WHEN the persisted store's `schema_version` is older than the current version, the system SHALL apply the registered migrations up to the current version  <!-- event-driven -->
3. IF no migration exists for an older `schema_version`, THEN the system SHALL quarantine the store and start with an empty index  <!-- unwanted-behavior -->
4. IF the persisted store is unreadable or corrupt (invalid JSON or malformed shape), THEN the system SHALL quarantine the file under a distinct name, preserving its original bytes, and start with an empty index without crashing  <!-- unwanted-behavior -->
5. WHILE the persisted store declares a `schema_version` newer than the build supports, the system SHALL quarantine it and start with an empty index, without attempting to parse it  <!-- state-driven -->
6. WHEN the store finishes loading, the system SHALL report a recovery outcome of `loaded`, `quarantined`, or `empty` and SHALL NOT include any sheet data in that outcome  <!-- event-driven -->
7. IF an entry supplied for a write has an empty `id`, `name`, or `template_id`, THEN the system SHALL reject the mutation without persisting  <!-- unwanted-behavior -->

**Independent Test**: Feed the store valid same-version JSON, older-version JSON with and without a migration, corrupt JSON, and newer-version JSON; assert the outcome, the empty/quarantined result, that the quarantine file holds the original bytes, and that no mutation was written for an invalid entry.

---

### P1: Riverpod read/write surface

**User Story**: As the library and open-flow surfaces, I want Riverpod providers to read and mutate the index so that the store is the single state source.

**Why P1**: The card explicitly requires providers for reading and writing the index; the library/open specs consume them.

**Acceptance Criteria**:

1. WHEN the store is loaded, the system SHALL expose the index through a Riverpod read provider consumed by the library surface  <!-- event-driven -->
2. WHEN a mutation succeeds, the system SHALL publish the updated index through the read provider so consumers rebuild with the new entries  <!-- event-driven -->
3. WHEN a mutation fails during persistence, the system SHALL keep the published index equal to the last successfully persisted index  <!-- event-driven -->

**Independent Test**: Watch the read provider, add an entry, assert the published list changes; force a write failure and assert the published list matches the last successful state.

---

## Edge Cases

Edge cases are usually unwanted-behavior (IF/THEN) or boundary (WHEN) criteria:

- WHEN the index is empty, listing returns an empty list (never null or an error).
- IF the store file exists but is zero bytes, THEN it is treated as corrupt and quarantined.
- IF the OS store directory does not exist on first write, THEN the store creates it before persisting.
- WHEN two entries share a `created_at`, THEN their listing order is deterministic (reverse insertion order).
- WHEN an update supplies an unchanged `created_at`, THEN the stored value is kept as supplied.
- IF a mutation is rejected for an invalid entry, THEN no file write occurs.

---

## Requirement Traceability

Each requirement gets a unique ID for tracking across design, tasks, and validation.

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------- |
| LIB-01 | P1: Versioned local sheet index | Design | Pending |
| LIB-02 | P1: Versioned local sheet index | Design | Pending |
| LIB-03 | P1: Versioned local sheet index | Design | Pending |
| LIB-04 | P1: Versioned local sheet index | Design | Pending |
| LIB-05 | P1: Versioned local sheet index | Design | Pending |
| LIB-06 | P1: Versioned local sheet index | Design | Pending |
| LIB-07 | P1: Versioned local sheet index | Design | Pending |
| LIB-08 | P1: Versioned local sheet index | Design | Pending |
| LIB-09 | P1: Persisted schema version and durable writes | Design | Pending |
| LIB-10 | P1: Persisted schema version and durable writes | Design | Pending |
| LIB-11 | P1: Persisted schema version and durable writes | Design | Pending |
| LIB-12 | P1: Persisted schema version and durable writes | Design | Pending |
| LIB-13 | P1: Persisted schema version and durable writes | Design | Pending |
| LIB-14 | P1: Safe recovery across schema versions | Design | Pending |
| LIB-15 | P1: Safe recovery across schema versions | Design | Pending |
| LIB-16 | P1: Safe recovery across schema versions | Design | Pending |
| LIB-17 | P1: Safe recovery across schema versions | Design | Pending |
| LIB-18 | P1: Safe recovery across schema versions | Design | Pending |
| LIB-19 | P1: Safe recovery across schema versions | Design | Pending |
| LIB-20 | P1: Safe recovery across schema versions | Design | Pending |
| LIB-21 | P1: Riverpod read/write surface | Design | Pending |
| LIB-22 | P1: Riverpod read/write surface | Design | Pending |
| LIB-23 | P1: Riverpod read/write surface | Design | Pending |

**ID format:** `LIB-[NUMBER]`

**Status values:** Pending → In Design → In Tasks → Implementing → Verified

**Coverage:** 23 total, 0 mapped to tasks yet (Tasks phase pending), 23 pending mapping.

---

## Success Criteria

How we know the feature is successful:

- [ ] `flutter test` green, including: upsert dedupe, remove-missing no-op, list ordering, write-failure consistency, corrupt/old/newer-version recovery with quarantine, and the recovered store containing no `data`.
- [ ] The persisted document parses to `{schema_version: 1, sheets: [...]}` and each entry carries exactly `id`, `name`, `template_id`, `created_at`.
- [ ] Adding an existing id leaves the entry count unchanged.
- [ ] Removing a missing id changes nothing.
- [ ] Corrupt, old-without-migration, and newer-version stores all leave the app running with an empty index and a quarantine file holding the original bytes.
- [ ] A forced write failure leaves the in-memory and published index at the last persisted state.
