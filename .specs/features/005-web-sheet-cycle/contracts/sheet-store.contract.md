# Contract: Sheet store (local persistence)

**Requirements**: WSC-11, WSC-26, WSC-27, WSC-29, WSC-30, WSC-31 | **Backing document**:
[data-model.md `SheetStore` / `CachedSheet` / `DraftBackup`](../data-model.md) and
[research.md `R1` / `R5` / `R6`](../research.md)

**Modules**: `web/src/library/sheet_store.ts` (the `SheetStore` port, the versioned envelopes and the
in-memory double), `web/src/library/idb_sheet_store.ts` (the `idb` adapter — the only file importing
`idb`), `web/src/library/draft_backup.ts` (the draft cadence coordinator). `sheet_library.ts` reads and
writes the cache through the port.

## Interface

| Export | Signature | Obligation |
|---|---|---|
| `createMemorySheetStore()` | `() => SheetStore` | the in-memory double every logic/screen suite injects |
| `createIdbSheetStore(factory?)` | `(factory?: IDBFactory) => Promise<SheetStore>` | the `idb` adapter over one database, two object stores |
| `readDraft` / `writeDraft` / `deleteDraft` | see data-model | the draft envelope operations |
| `readSheet` / `putSheet` / `deleteSheet` | see data-model | the cache envelope operations |
| `startDraftCadence(store, getDelta, scheduler)` | `(...) => () => void` | the periodic backup while pending; returns a stop function |

`SCHEMA_VERSION = 1` is shared by both envelopes.

## Rules

1. **Cache by id** (WSC-11). `putSheet(sheet)` writes a `{ schemaVersion: 1, id, sheet }` record keyed by
   `sheet.id`; `readSheet(id)` returns the stored `Sheet` or `null`.
2. **Versioned, corrupt is ignored** (WSC-30). A record that is missing, is not a well-formed envelope,
   carries a `schemaVersion` other than `1`, or names a different `id` reads as `null` — never a throw.
3. **No credential** (WSC-31). No operation writes a token, cookie or any credential; the cache holds a
   `Sheet` and the draft holds a delta of sheet values only.
4. **Draft holds the editable delta** (WSC-26, WSC-31). `writeDraft(id, delta)` stores the pending
   editable delta the caller passes; the store never adds derived values and never validates the
   template (the restore filter is the edit store's, `WSC-28`).
5. **Periodic, pending-only** (WSC-26, WSC-27). `startDraftCadence` writes once per injected tick while
   the sheet is pending and the delta is non-empty; it is a no-op while the sheet is clean and it stops
   on unmount. A write failure is swallowed (a failed backup never blocks editing), and the cadence
   default is 60 seconds with the scheduler injected (`AD-008`).
6. **Delete is best-effort** (WSC-29). `deleteSheet(id)` and `deleteDraft(id)` never throw; a failing
   delete leaves the message/flow intact. `notFound` eviction calls both; a save success calls
   `deleteDraft`.
7. **One dependency, one file** (R5). `idb` is imported only by `idb_sheet_store.ts`; the port and every
   consumer are database-agnostic.

## Obligations

1. **WSC-11** — a loaded/created sheet is cached by id under `schemaVersion 1`.
2. **WSC-26** — the cadence writes the pending editable delta and stores no derived value.
3. **WSC-27** — a clean sheet writes no draft.
4. **WSC-29** — save success, confirmed discard and `notFound` eviction each delete the draft.
5. **WSC-30** — a corrupt/unsupported/foreign record is ignored without raising.
6. **WSC-31** — no operation stores a credential.

## Test obligations

- **Happy path**: `putSheet` then `readSheet` round-trips a `Sheet`; `writeDraft` then `readDraft`
  round-trips the delta.
- **Edge**: `readSheet`/`readDraft` return `null` for a missing key, a malformed record, an unsupported
  `schemaVersion` and a foreign `id` — none throws.
- **Edge**: `deleteSheet`/`deleteDraft` on a missing key resolve without throwing.
- **Edge**: the cadence writes nothing while clean; a tick with an empty delta writes nothing; a
  scheduler-driven tick while pending writes the current delta; a write failure is swallowed and the
  cadence keeps running; stopping the cadence stops the writes.
- **Security**: an inspection of every stored record finds no credential-shaped value.
- **Happy path (adapter)**: under `fake-indexeddb`, `createIdbSheetStore` round-trips a sheet and a
  draft, and reads a corrupt record as `null`.
