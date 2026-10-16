# Contract: Sheet edit

**Requirements**: WSC-12, WSC-13, WSC-14, WSC-15, WSC-16, WSC-17, WSC-18, WSC-19, WSC-20, WSC-21,
WSC-22, WSC-23, WSC-24, WSC-25, WSC-26, WSC-27, WSC-28, WSC-29, WSC-34, WSC-38 | **Backing document**:
[data-model.md `SheetEditState` / `PendingDelta` / `PreviewMarks` / `RollingWindow`](../data-model.md) and
[research.md `R2`…`R7`](../research.md)

**Module**: `web/src/sheet/sheet_edit.ts` — the in-memory edit store for one open sheet. It holds the
values, the pending set, the save/preview lifecycle, the preview marks and the two throttle windows. It
performs exactly the `PATCH` and the preview request, through the F2 client.

## Interface

| Member | Signature | Obligation |
|---|---|---|
| `createSheetEditStore(seed, deps)` | `(seed: Seed, deps: Deps) => SheetEditStore` | builds a store seeded from the template and the stored sheet, over the restored draft |
| `store.getState()` | `() => SheetEditState` | the current immutable snapshot |
| `store.subscribe(listener)` | `(l: () => void) => () => void` | coarse subscription, for the screen's own signals |
| `store.subscribePath(path, listener)` | `(p: string, l: () => void) => () => void` | the per-path subscription a field control uses (`AD-001`, `WSC-34`) |
| `store.setPath(path, value)` | `(p: string, v: unknown) => void` | records the edit (`WSC-12`) |
| `store.setExclusive(selected, siblings)` | `(s: string \| null, sib: readonly string[]) => void` | one exclusive-group write (`WSC-15`) |
| `store.save()` | `() => Promise<void>` | one delta `PATCH` (`WSC-14`, `WSC-16`, `WSC-17`) |
| `store.preview()` | `() => Promise<void>` | one delta dry-run (`WSC-21`…`WSC-25`) |

`deps` injects the `ApiClient`, the `SheetStore`, the clock, and the two `RollingWindow`s, so a suite
drives the store with a fake transport, a memory store and a fake clock.

## Rules

1. **Pending set** (WSC-12, WSC-13). `setPath` records the dotted path and produces a fresh `data` tree
   (copy-on-write). `isPending` is true only while some edited path's current value differs from the
   reference; a reverted edit therefore un-pends. A redundant write (same value) does not advance
   `revision` and does not clear the marks.
2. **Delta** (WSC-14). `pendingDelta` rebuilds the nested object from the pending paths only, via the
   engine's `setPathValue`; it carries no unchanged field, no derived field and no value the person did
   not change.
3. **Exclusive write** (WSC-15). `setExclusive(path, siblings)` sets the chosen member on (`1`) and every
   sibling off (`0`) in one emission; `setExclusive(null, siblings)` sets every member off. A write that
   changes nothing emits nothing.
4. **Save** (WSC-14, WSC-16, WSC-17, WSC-18, WSC-19). `save()` is a no-op unless `canSave` (pending, not
   busy, window free) and the delta is non-empty; it records the attempt in the save window, sends one
   `PATCH` with `pendingDelta`, and on success merges the returned cascade into `data` and advances
   `defaults` to `mergeCascade(mergeCascade(defaults, delta), cascade)`, so the sent edits stop being
   pending. On failure every edit stays, `saveError` is the mapped copy, and no retry is issued. The
   `PATCH` is never repeated automatically.
5. **Preview** (WSC-21, WSC-22, WSC-23, WSC-24, WSC-25). `preview()` is a no-op unless `canPreview`
   (pending, a new revision since the last preview, not busy, window free) and the delta is non-empty;
   it records the attempt in the preview window and sends one `POST .../preview` with the same delta. On
   success the cascade is merged into `data` **without** advancing `defaults`, `previewedRevision` is
   recorded, and `previewMarks` is set to `{ modified: flatten(delta), recalculated: flatten(cascade) }`.
   On failure the edits stay, `previewError` is the mapped copy, `previewedRevision` is not recorded,
   and Preview stays available.
6. **Marks clear** (WSC-24). Any data-changing edit clears `previewMarks` (through `setPath`/
   `setExclusive`); a save also clears them (`WSC-16`).
7. **Marks rendering** (WSC-23). The store publishes path sets; the screen renders them through the F4
   `rowMarkBuilder` seam and `markLabel`, never a per-screen literal. A mark tints and labels even when
   the marked value is empty.
8. **Throttle** (WSC-38, `R7`). Each of save and preview records at most three attempts in a rolling
   60-second window; while three remain inside the window the action is a no-op, and it re-enables when
   the window frees. The throttle never replaces the in-flight no-op rules (rules 4 and 5).
9. **Draft restore** (WSC-28). The seed overlays the stored draft on `seedFromSheet(template, sheet)` and
   applies only keys that are editable leaves of the loaded template, marking those paths pending; a
   draft key that is not an editable leaf is ignored.
10. **Draft lifecycle** (WSC-26, WSC-27, WSC-29). The store exposes `pendingDelta` for the cadence
    coordinator (`draft_backup.ts`, rule R6); it deletes the sheet's draft on save success. Discard
    (rules in the shortcut/screen contract) also deletes it.
11. **Reuse, no re-implementation** (WSC-34). `seedFromSheet`, `mergeCascade`, `setPathValue` come from
    the F4 engine; the store defines no second merge or seed.

## Obligations

1. **WSC-12** — an edit records its dotted path.
2. **WSC-13** — a reverted field is not pending.
3. **WSC-14** — the payload is the pending editable delta, nested, with no unchanged/derived value.
4. **WSC-15** — an exclusive write sets one on and the rest off in one change; "None" sets all off.
5. **WSC-16** — a save merges derived values and advances the baseline; the edits un-pend.
6. **WSC-17** — a failed save keeps every edit, shows the mapped message, issues no retry.
7. **WSC-18** — a second Save while one is in flight (including `Ctrl+S`) issues no request.
8. **WSC-19** — a clean sheet's Save issues no request.
9. **WSC-21** — Preview issues one request, none when clean or in flight.
10. **WSC-22** — the preview cascade merges without advancing the baseline.
11. **WSC-23** — `modified`/`recalculated` marks come from the engine seam; empty marked rows still mark.
12. **WSC-24** — a new edit clears the marks and re-enables Preview.
13. **WSC-25** — a failed preview keeps the edits and the message; Preview stays available.
14. **WSC-28** — the draft overlay applies only editable leaves of the loaded template.
15. **WSC-38** — Save and Preview are each capped at three requests per rolling 60-second window.

## Test obligations

- **Edge**: an edit back to its seeded value un-pends and leaves `pendingDelta` empty; a derived path
  never enters the delta; an unchanged sibling of a changed nested group never enters the delta; a
  high-precision decimal is sent unchanged.
- **Edge**: an exclusive selection writes one on and its siblings off in one emission; selecting "None"
  writes all off; a redundant exclusive write emits nothing.
- **Edge**: Save with an empty delta issues no request; a second Save while one is in flight issues no
  request; a fourth Save inside the rolling window issues no request and re-enables after the window.
- **Edge**: a failed save keeps every edit and issues no automatic retry.
- **Happy path**: a save sends the exact nested delta and, on success, merges the cascade, advances the
  baseline and clears pending.
- **Happy path**: a preview sends the same delta, merges the cascade, publishes the two mark sets and
  does not advance the baseline.
- **Edge**: an edit made during a save stays pending after the save succeeds.
- **Edge**: a repeat preview with no new edit issues no request; an edit after a preview clears the
  marks and re-enables Preview.
- **Edge**: a failed preview keeps every edit and the mapped message and leaves Preview available.
- **Edge**: the draft overlay applies only editable leaves; a derived or foreign key is ignored.
- **Security**: no payload or stored draft contains a credential; two concurrent saves cannot both
  issue a request.
