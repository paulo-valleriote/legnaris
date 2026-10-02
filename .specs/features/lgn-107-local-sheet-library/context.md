# Local Sheet Library Context

**Gathered:** 2026-10-02
**Spec:** `.specs/features/lgn-107-local-sheet-library/spec.md`
**Status:** Ready for design

---

## Feature Boundary

Persist and expose a versioned local index of created sheets (`{id, name, template_id, created_at}` + `schema_version`) as the single source of the "Minhas fichas" list. Add/update, list, and remove operations, plus Riverpod read/write providers. The sheet `data` is never persisted. No UI in this feature.

---

## Implementation Decisions

### Persistence failure semantics

- Write-through: a mutation persists the full index before reporting success.
- On a write failure, the in-memory (and published) index stays at its pre-mutation value and the failure is reported to the caller.
- No silent best-effort writes.

### Corrupt / unsupported store recovery

- A corrupt store, or one whose `schema_version` is newer than the build supports, is quarantined: the original file is moved aside under a distinct name and its bytes are preserved.
- After quarantine the app starts with an empty index and continues without crashing.
- An older `schema_version` runs the registered migration chain; if no migration exists for it, it is quarantined like a corrupt store.

---

## Agent's Discretion

- Storage backend and file location/name (the card defers this to Design): choose the mechanism behind an injectable persistence boundary so unit tests use an in-memory fake, without adding a package unless strictly necessary per `desktop/AGENTS.md`.
- Atomicity implementation (temp-file + rename or platform equivalent).
- Quarantine filename pattern and the exact recovery-outcome reporting channel.

---

## Declined / Undiscussed Gray Areas → Assumptions

- Upsert semantics for a duplicate `id` (replace, never duplicate) → assumption in spec.
- List ordering (`created_at` descending, reverse-insertion tie-break) → assumption in spec.
- `created_at` stored as the API's ISO-8601 value verbatim → assumption in spec.
- Current `schema_version` = `1`, migration registry keyed by from-version → assumption in spec.
- Entry validation (non-empty `id`/`name`/`template_id`) → assumption in spec.
- Logging sink for recovery events (no desktop logger exists yet; the store reports an outcome instead) → assumption in spec.

---

## Specific References

- `docs/TDD-desktop-mvp.md` §Persistência Local (Local Store) — index fields, no `data` caching, `schema_version` for future migration.
- `docs/TDD-desktop-mvp.md` §Fluxo de Dados — biblioteca local de fichas.
- `docs/TDD-desktop-mvp.md` §Plano de Rollback — versioned store so a new build migrates without losing the index.
- Epic LGN-93 ("Fase 4 — Ciclo da ficha").
- Existing Riverpod pattern: `desktop/lib/sheet/sheet_edit_provider.dart` and generated providers.

---

## Deferred Ideas

- Dev base-URL override persistence and UI preferences in the Local Store (TDD lists them) — belongs to a settings card, not the sheet index.
- Displaying the quarantine/recovery notice in the UI — belongs to the library ("Abrir ficha") spec.
