# Phase 1 — Data Model: Web Sheet Cycle

The feature persists two local records and holds one in-memory edit state; it re-declares none of the
wire models. The payload shapes it reads and writes are the F2 models
(`web/src/api/models/sheet.ts` `Sheet` + `valueAtPath`, `web/src/api/models/template.ts`) and the F4
engine derivation (`seedFromSheet`, `mergeCascade`, `setPathValue`, `buildSections`), which are imported,
not redefined (`WSC-34`). Every field traces to a functional requirement in [spec.md](./spec.md).

Naming: TypeScript types use `PascalCase`, values and functions `camelCase`; the backend's `snake_case`
keys never appear outside the F2 decoders. No local record holds a credential (`WSC-31`).

---

## SheetEditState

The immutable snapshot of one open sheet's edit state, held while the sheet screen is mounted and
discarded when it unmounts (Key Entities, `WSC-12`…`WSC-25`).

| Field | Type | Rules |
|---|---|---|
| `defaults` | `Record<string, unknown>` | the reference values seeded from the template and the stored sheet; the baseline `isPending` compares against and that a save advances (`WSC-16`) |
| `data` | `Record<string, unknown>` | the current values; every write produces a fresh tree (copy-on-write), never an in-place mutation |
| `editedPaths` | `Set<string>` | every dotted path written at least once since the seed (`WSC-12`); a path back at its reference value no longer counts (`WSC-13`) |
| `isSaving` | `boolean` | a `PATCH /sheets/:id` is in flight; Save is disabled and the discard guards stand down (`WSC-18`) |
| `saveError` | `string \| undefined` | mapped copy of the last save failure, rendered inline; cleared on success (`WSC-17`) |
| `revision` | `number` | increments on every data-changing edit; the preview idempotency key |
| `previewedRevision` | `number \| undefined` | revision of the last successful preview; a repeat with no new edit is blocked (`WSC-21`, `WSC-24`) |
| `previewMarks` | `PreviewMarks \| undefined` | the last preview's path sets; absent before a preview, after a save and after a new edit (`WSC-23`, `WSC-24`) |
| `isPreviewing` | `boolean` | a `POST /sheets/:id/preview` is in flight |
| `previewError` | `string \| undefined` | mapped copy of the last preview failure; cleared on success (`WSC-25`) |
| `saveWindow` | `RollingWindow` | the save throttle window (`WSC-38`) |
| `previewWindow` | `RollingWindow` | the preview throttle window (`WSC-38`) |

**Derived invariants**

| Derivation | Rule |
|---|---|
| `isPending` | true when any `editedPaths` entry's `valueAtPath(data, path)` differs from `valueAtPath(defaults, path)` (`WSC-13`) |
| `isBusy` | `isSaving \|\| isPreviewing`; both actions block together so an older cascade never merges over a newer baseline (`WSC-18`, `WSC-21`) |
| `canSave` | `isPending && !isBusy && !saveWindow.isBlocked` (`WSC-19`, `WSC-38`) |
| `canPreview` | `isPending && previewedRevision !== revision && !isBusy && !previewWindow.isBlocked` (`WSC-21`, `WSC-24`, `WSC-38`) |

**Transitions**: seed (on load/draft restore) → edit (pending) → save success (baseline advances,
pending clears, draft deletes) | save failure (pending kept) → preview success (marks set, baseline
unchanged) | preview failure (pending kept, re-previewable) → new edit (marks clear) → unmount (state
discarded).

---

## PendingDelta

The nested object rebuilt from the pending paths — the exact body a save and a preview send (`WSC-14`).

| Operation | Input | Output | Rules |
|---|---|---|---|
| `pendingDelta(state)` | the edit state | a fresh nested object | includes an edited path only while its current value differs from the reference, nested by dotted path via the engine's `setPathValue`; a derived (non-editable) leaf is never written, so it never appears (`WSC-14`) |

**Invariants**: an empty delta means Save and Preview issue no request (`WSC-19`, `WSC-21`). The delta
contains no unchanged field and no derived field (`SC-002`).

---

## PreviewMarks

The path sets the last successful preview published (`WSC-23`).

| Field | Type | Rules |
|---|---|---|
| `modified` | `Set<string>` | the flattened paths of the previewed delta — the rows the person changed |
| `recalculated` | `Set<string>` | the flattened paths of the cascade the preview returned — the derived rows the server recomputed |

**Invariants**: absent before a preview, after a save and after a new edit (`WSC-24`); a mark tints and
labels even when the marked value is empty; the marks come from the F4 `rowMarkBuilder`/`markLabel`
seam, never a per-screen literal (`WSC-23`). The marks never change a value and never issue a request.

---

## SheetOpenState

The load situation of one sheet addressed by URL identifier (`WSC-05`…`WSC-10`).

| Kind | Fields | Rules |
|---|---|---|
| loading | — | shown until the response settles (`WSC-05`) |
| loaded | `sheet: Sheet`, `template: TemplateDetail` | the sheet and its template; the screen renders the name and the field tree (`WSC-08`) |
| notFound | `message: string` | the mapped missing-sheet message, **no Retry**, and the sheet plus its draft are evicted from the local store (`WSC-06`) |
| failure | `message: string` | the mapped message with a Retry that re-requests exactly once (`WSC-07`); an undecodable payload takes this same path |

**Invariants**: one request per open and per Retry (`WSC-05`, `WSC-07`); a failing local eviction never
hides the `notFound` message (`WSC-06`); the template detail has its own loading/failure states
(`WSC-09`).

---

## CachedSheet (envelope)

One locally stored record of a loaded or created sheet, so a later offline read can render it
(`WSC-11`). Stored in the `sheets` object store, keyed by the sheet id.

| Field | Type | Rules |
|---|---|---|
| `schemaVersion` | `1` | the version this build writes and understands; another version is ignored (`WSC-30`) |
| `id` | `string` | the sheet id; a record whose `id` differs from the one opened is ignored (`WSC-30`) |
| `sheet` | `Sheet` | the F2 sheet payload verbatim; **never a credential** (`WSC-31`) |

**Invariants**: eviction on `notFound` removes the record best-effort; a corrupt record reads as `null`
rather than throwing (`WSC-06`, `WSC-30`).

---

## DraftBackup (envelope)

One locally stored record of a sheet's unsaved pending delta (`WSC-26`). Stored in the `drafts` object
store, keyed by the sheet id.

| Field | Type | Rules |
|---|---|---|
| `schemaVersion` | `1` | an unsupported version is ignored (`WSC-30`) |
| `id` | `string` | a draft naming another sheet is ignored on open (`WSC-30`) |
| `delta` | `Record<string, unknown>` | only the pending **editable** delta; **never a derived value** (`WSC-26`), **never a credential** (`WSC-31`) |

**Invariants**: written periodically only while the sheet is pending (`WSC-26`, `WSC-27`); overlaid on
the stored sheet on open, filtered to the template's editable leaves, with the restored paths marked
pending (`WSC-28`); deleted on save success, confirmed discard, and `notFound` eviction (`WSC-29`).

---

## SheetStore (port)

The persistence boundary every sheet/library module depends on; the app builds the `idb` adapter and
tests build the in-memory double (R1, R5).

| Operation | Input | Output | Rules |
|---|---|---|---|
| `readSheet(id)` | sheet id | `Sheet \| null` | `null` for missing, corrupt, unsupported-version or foreign record (`WSC-30`) |
| `putSheet(sheet)` | a `Sheet` | `Promise<void>` | writes the `CachedSheet` envelope (`WSC-11`) |
| `deleteSheet(id)` | sheet id | `Promise<void>` | best-effort; never throws (`WSC-06`) |
| `readDraft(id)` | sheet id | `Record<string, unknown> \| null` | `null` for missing/corrupt/unsupported/foreign (`WSC-30`) |
| `writeDraft(id, delta)` | sheet id + editable delta | `Promise<void>` | writes the `DraftBackup` envelope; a failure never blocks editing |
| `deleteDraft(id)` | sheet id | `Promise<void>` | best-effort (`WSC-29`) |

**Invariants**: no operation stores a credential (`WSC-31`); reads never throw on bad input
(`WSC-30`); `idb` is imported only by the adapter (R5).

---

## RollingWindow

The per-action request throttle (`WSC-38`, `SC-011`).

| Field | Type | Rules |
|---|---|---|
| `attempts` | `number[]` | the timestamps of the last ≤ 3 attempts inside the window |
| `max` | `3` | the cap per window |
| `windowMs` | `60000` | the rolling window length |

| Operation | Rule |
|---|---|
| `record(now)` | appends an attempt and drops timestamps older than `windowMs` |
| `isBlocked(now)` | true while three attempts remain inside the window |
| `nextFreeAt(now)` | the earliest instant the oldest attempt leaves the window, so the action re-enables without waiting |

**Invariants**: the window is injectable-clock driven so a suite never waits; the throttle is orthogonal
to `isBusy` and never replaces the in-flight no-op rules (`WSC-18`, `WSC-21`).

---

## ShortcutAction / registry

The one action → key map the shell dispatches through (`WSC-20`, `WSC-36`; `web/AGENTS.md`).

| Action | Default key | Dispatched when |
|---|---|---|
| `saveSheet` | `Ctrl+S` | a sheet screen is mounted; a no-op otherwise (`WSC-20`) |
| `refreshSheet` | `F5` | a sheet screen is mounted; a no-op otherwise (`WSC-36`) |

**Invariants**: the registry holds no state; the mounted screen registers its handlers and unregisters
on unmount, so a stale action can never outlive its screen. The map is plain data, so a later feature
can rebind a key without touching the screens.

---

## MessageKey additions

The feature's copy resolves through the single catalog (`web/src/l10n/strings.ts`); no key is removed
(`WSC-32`). Adding these keys keeps every string out of the components (`SC-006`).

| Key | Used by |
|---|---|
| `sheet.headerTitle` | the sheet screen's header (`WSC-08`) |
| `sheet.openLoading` | the open loading state (`WSC-05`) |
| `sheet.templateLoading` | the template loading state (`WSC-09`) |
| `sheet.emptyFields` | a template with no fields |
| `sheet.missing` | the missing-sheet message, no Retry (`WSC-06`) |
| `sheet.retryOpen` | the retryable-failure Retry (`WSC-07`, `WSC-09`) |
| `sheet.pendingEdits` | the footer's dirty indicator (`WSC-12`) |
| `sheet.save` / `sheet.saving` / `sheet.saved` | the Save action and its in-flight/success feedback (`WSC-14`, `WSC-16`) |
| `sheet.preview` / `sheet.previewing` | the Preview action and its in-flight feedback (`WSC-21`) |
| `sheet.refreshTitle` / `sheet.refreshMessage` / `sheet.refreshConfirm` | the `F5` discard guard (`WSC-36`) |
| `sheet.leaveTitle` / `sheet.leaveMessage` / `sheet.leaveConfirm` | the leave discard guard (`WSC-37`) |
| `sheet.cancel` | the guards' safe action (`WSC-36`, `WSC-37`) |

**Invariants**: no user-facing literal exists in `web/src/sheet/` or `web/src/library/`; every branch
resolves a key through `message()`, which is what `WSC-32` and `SC-006` assert.
