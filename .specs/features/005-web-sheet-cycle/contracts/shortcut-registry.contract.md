# Contract: Shortcut registry

**Requirements**: WSC-20, WSC-36, WSC-37 | **Backing document**:
[data-model.md `ShortcutAction` / registry](../data-model.md) and [research.md `R8`](../research.md)

**Module**: `web/src/shell/shortcut_registry.ts` — the one action → key map the shell dispatches
through (`web/AGENTS.md`: shortcuts are centralized in one registry as a plain action → key map). The
registry holds no domain state; the mounted sheet screen registers the active handlers.

## Interface

| Export | Signature | Obligation |
|---|---|---|
| `SHORTCUTS` | `{ saveSheet: ShortcutSpec; refreshSheet: ShortcutSpec }` | the plain action → key map (`Ctrl+S`, `F5`) |
| `matchShortcut(event)` | `(e: KeyboardEvent) => ShortcutAction \| null` | resolves a keydown to an action, or `null` |
| `useSheetShortcuts({ onSave, onRefresh })` | `(handlers) => void` | registers the mounted sheet's handlers; unregisters on unmount |
| `dispatchShortcut(action)` | `(a: ShortcutAction) => void` | invokes the registered handler, or a no-op when none is registered |

## Rules

1. **One registry** (R8). The action → key map is plain data; the screen never binds a `keydown`
   listener of its own. A later feature can rebind a key by editing the map.
2. **`Ctrl+S` saves** (WSC-20). `Ctrl+S` (and `Cmd+S` on macOS) dispatches `saveSheet`, which runs the
   same save as the footer action; it is a no-op when no sheet screen is mounted. The browser's default
   is prevented while a sheet screen is mounted so it does not open "Save page".
3. **`F5` refreshes the template behind a guard** (WSC-36). While a sheet screen is mounted, `F5`
   re-fetches the sheet's template detail exactly once; with pending edits it first confirms a discard
   (rule 4) and only a confirm discards and re-fetches; a cancel issues no request. `F5` is a no-op
   while a preview or save is in flight, and a no-op when no sheet screen is mounted (the browser's
   default reload is not prevented in that case).
4. **Discard guard for refresh** (WSC-36). The confirmation states the exact consequence; confirm
   discards the in-memory edits, deletes the draft and re-fetches; cancel, `Esc`, or the barrier keep
   the edits and issue no request. Never confirm-by-Enter alone (`web/AGENTS.md`).
5. **Discard guard for leaving** (WSC-37). Leaving the sheet screen with pending edits (a link, the
   nav, the browser back) confirms a discard first; confirm discards and deletes the draft; cancel
   stays. A clean sheet leaves without a dialog, and an in-flight preview or save makes leaving a no-op.
6. **Single in-flight confirmation** (WSC-36, WSC-37). Two guards never stack; while a confirmation is
   open a further refresh/leave activation is ignored.
7. **No copy; accessible** (WSC-32, WSC-39). Every guard string resolves through `message()`; the
   confirmation is an accessible dialog (role, modal, labelled, focus-trapped in the F3 overlay
   pattern) and its safe action is a real button.

## Obligations

1. **WSC-20** — `Ctrl+S` runs the footer save; a no-op with no mounted sheet.
2. **WSC-36** — `F5` re-fetches the template detail behind a discard confirmation; no-op in flight and
   with no mounted sheet.
3. **WSC-37** — leaving with pending edits confirms a discard first; a clean sheet leaves silently.

## Test obligations

- **Happy path**: `Ctrl+S` on a mounted sheet runs the same save as the footer action; with no sheet
  mounted it runs nothing.
- **Happy path**: `F5` on a clean sheet re-fetches the template detail exactly once.
- **Edge**: `F5` with pending edits opens the confirm; cancel issues zero requests and keeps the edits;
  confirm discards the edits, deletes the draft and re-fetches once.
- **Edge**: `F5` while a preview or save is in flight issues no request and shows no confirm.
- **Edge**: leaving with pending edits confirms; cancel stays; confirm discards and deletes the draft;
  a clean sheet leaves with no dialog.
- **Edge**: a second refresh/leave activation while a confirmation is open is ignored.
- **Source scan**: the registry and the screen author no user-facing literal.
