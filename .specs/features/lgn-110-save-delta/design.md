# Save with Delta and Incremental Cascade Merge — Design

Source: [LGN-110](https://paulovalleriotedev.atlassian.net/browse/LGN-110). Spec: `.specs/features/lgn-110-save-delta/spec.md`.

## Context

Backend `PATCH /sheets/:id` already validates editability, collapses exclusive groups and returns **only the changed derived delta** (`internal/sheet/service.go:148` → `nestedCascadingEffects`). `ApiClient.patchSheet` already wraps the payload as `{"data": delta}` and never retries PATCH (GET-only retry). Screen 2 owns an in-memory `SheetEditState` (`data`, `defaults` baseline, `editedPaths`) keyed by sheet id; the creation flow overrides the family in a route-scoped `ProviderScope` (AD-004), while the library path resolves it in the root container.

## Decisions

### AD-110-1 — Save lives on `SheetEditController`, not a second provider family
The controller is guaranteed to be the exact scoped instance the screen reads. A separate `SheetSaveController` family would, in the creation path, resolve in the parent container and watch an **unseeded** edit state (the edit family is overridden in the child scope only, AD-004). Keeping `save()` on the edit controller makes scope consistency structural and mirrors `SheetOpenController`, which already reads `apiClientProvider` itself.

### AD-110-2 — Baseline is recomputed, not reset to `data`
On success the controller needs `defaults'` to represent what the server now stores, while `data` may have newer in-flight edits (the form stays editable during a save). So:
- `data'    = mergeCascade(data, cascade)`
- `defaults' = mergeCascade(mergeCascade(defaults, delta), cascade)`
- `editedPaths` is preserved; `isPending` filters by value.

Plain case: `data' == defaults'` → dirty clears. In-flight case: the newer edits still differ → dirty stays. Resetting `defaults = data` would silently mark unsent edits as saved.

### AD-110-3 — `mergeCascade` is the existing `_overlay`, promoted
`seedFromSheet` already deep-merges stored data over schema defaults with exactly the cascade semantics (recurse where both sides are maps, replace the leaf). Promote `_overlay` to public `mergeCascade` and reuse it; no second merge implementation.

### AD-110-4 — `Ctrl+S` uses the same tick seam as F5
The central `appShortcuts`/`appActions` registry sits above the Navigator. Mirror `sheetRefreshTickProvider`: add `sheetSaveTickProvider` (non-family, autoDispose) and `AppShortcut.saveSheet → Ctrl+S`; `appActions` bumps the tick only when `activeSheetIdProvider != null`; the mounted screen listens and calls `save()`. The screen keeps the tick alive; no sheet → no-op (also covers the creation dialog).

### AD-110-5 — Creation dialog: bound the content, not the window height
Reproduced: at 1280×720 the content `RenderFlex` overflows 39px and the submit rect intersects the preview (159px at 600px). Fix by making the dialog content flex: wrap the preview in `Flexible` in the AlertDialog content column, and make the preview's scroll area `Flexible` inside its card. The preview then shrinks to the available height and scrolls internally; the action row stays separate. No magic height, no viewport probing.

## Components

| Unit | File | Responsibility |
| ---- | ---- | -------------- |
| `mergeCascade` | `desktop/lib/engine/sheet_defaults.dart` | Public deep-merge (cascade/overlay) |
| `SheetEditState` | `desktop/lib/sheet/sheet_edit_provider.dart` | Adds `isSaving`, `saveError`, `pendingDelta` |
| `SheetEditController` | `desktop/lib/sheet/sheet_edit_provider.dart` | Adds `save()`; recomputes baseline on success, keeps edits + mapped copy on failure |
| `sheetSaveTickProvider` | `desktop/lib/sheet/sheet_refresh.dart` | Dispatch counter for `Ctrl+S` |
| Screen 2 footer | `desktop/lib/sheet/sheet_screen.dart` | Dirty indicator + Save + inline error; listens to the save tick; guards back/F5 while saving |
| Shortcut registry | `desktop/lib/shell/shortcut_registry.dart` | `Ctrl+S` binding + dispatch |
| l10n | `desktop/lib/l10n/app_en.arb` | `sheetSaveAction`, `sheetSaveNoChangesTooltip` |
| Creation dialog | `desktop/lib/catalog/create_sheet_dialog.dart` | Bounded flex content |

## Data Flow (save)

```
Save / Ctrl+S
  → SheetEditController.save()
      delta = state.pendingDelta            // editedPaths where value != baseline
      if delta.isEmpty || isSaving: return
      state = copyWith(isSaving: true, saveError: null)
      cascade = ApiClient.patchSheet(sheetId, delta)
      success: _applySaved(delta, cascade)
                 data'     = mergeCascade(data, cascade)
                 defaults' = mergeCascade(mergeCascade(defaults, delta), cascade)
                 editedPaths preserved  → dirty only if newer edits remain
      failure: state = copyWith(isSaving: false, saveError: failureCopy(error))
```

## Risks / mitigations

| Risk | Mitigation |
| ---- | ---------- |
| Riverpod scope mismatch (creation vs library path) | Save on the scoped edit controller (AD-110-1) |
| Whole-form rebuild on save status change | Footer is an isolated `Consumer` selecting `isPending`/`isSaving`/`saveError`; field controls select scalars (AD-001) |
| Edits made during the in-flight save | Baseline recomputation keeps them pending (AD-110-2) |
| `Ctrl+S` stealing keystrokes in the creation dialog | Tick only dispatches when `activeSheetIdProvider != null`; creation flow has no active sheet |
| Preview regression from the flex fix | Existing PRV-10 assertions (card width, scroll height ≤ cap, always-visible bar) stay green |

## Out of Scope

See the spec. No backend change, no auto-save, no per-field revert, no offline queue.
