# Create Sheet Design

Source: [LGN-109](https://paulovalleriotedev.atlassian.net/browse/LGN-109). Spec: `.specs/features/lgn-109-create-sheet/spec.md`.

## Context

Three pieces already exist and are reused unchanged: `ApiClient.createSheet` (`desktop/lib/api/api_client.dart`, FE1-05), the local library `SheetLibrary.upsert` (`desktop/lib/library/sheet_library_provider.dart`, LGN-107), and the error mapper `failureCopy` (`desktop/lib/api/api_error.dart`). The creation dialog (`desktop/lib/catalog/create_sheet_dialog.dart`) already renders the template preview and the name input; only its primary action and submit path are missing. Screen 2 (`desktop/lib/sheet/sheet_screen.dart`) renders schema-driven fields from the template detail, seeded from schema defaults by `SheetEditController` (`desktop/lib/sheet/sheet_edit_provider.dart`).

The card's "abrir a ficha criada" touches Screen 2, which is LGN-108's territory (`GET /sheets/:id` + the Screen 1 library list, still `To Do`). Per the confirmed user decision (2026-10-03), this feature does **not** add the `GET /sheets/:id` open path. It injects the `201` payload directly into Screen 2, so LGN-108 can add the `GET`-based open later on top of the same seam.

## Decision: the opened-sheet seam (AD-004 candidate)

Add one overridable provider to `sheet_edit_provider.dart`:

```dart
@Riverpod(keepAlive: true)
Sheet? openedSheet(Ref ref) => null;
```

- **Default (null)** — Screen 2 seeds from the template schema defaults, exactly as today. The catalog "Preview sheet" action keeps this path.
- **Overridden per route** — the creation flow wraps the pushed Screen 2 in a `ProviderScope` with `openedSheetProvider.overrideWithValue(createdSheet)`. `SheetEditController.build` seeds its baseline and working data from `openedSheet.data`; `SheetScreen` renders `openedSheet.name` as the title. A Riverpod nested scope shares non-overridden providers with its parent, so a controller created in the parent would not see the child's `openedSheet` override; the same scope therefore also overrides the controller family (`sheetEditControllerProvider.overrideWith2((_) => SheetEditController())`) so the notifier is created in the child scope and resolves `openedSheetProvider` there.

The seeded `defaults` and `data` are equal at open (`isPending == false`, CRT-10); the first edit copies the tree (existing `_copyTree`) and marks pending relative to the created sheet's values, so the dirty/leave guards in GRN-11 … GRN-19 keep working without change.

### Why a provider, not a constructor field

`SheetEditController` is a family notifier; Riverpod constructs it, so a family argument carrying a mutable `Map` is not safe (map identity/equality). Passing the sheet to `SheetScreen` as a widget field would work for rendering, but the controller (and the back/F5 handlers that read it) must resolve the same seed, and a field is not visible to the provider graph. The route-scoped `ProviderScope` override is the seam that makes the screen, the controller and every field `Consumer` observably consistent.

### Alternatives rejected

- **Family key carries the sheet data** — mutable map keys break equality/hash and leak payload into the family cache.
- **`SheetScreen` self-overrides the edit provider inside its own build** — the screen's top-level `ref` and the nested scope diverge; back/F5 reads miss the seed.
- **Add `GET /sheets/:id` now** — duplicates LGN-108 and rewrites Screen 2's open contract; deferred by the user decision.

## Components

### `desktop/lib/catalog/create_sheet_dialog.dart` (change)

Dialog-local submit state (`_isSubmitting`, `_errorMessage`), mirroring the existing local UI state (`_nameController`, `_confirming`). No new provider: creation is a one-shot action owned by this surface.

- Primary action is disabled while `_isSubmitting` or the trimmed name is empty; a tooltip states a name is required (CRT-05). A `ValueListenableBuilder<TextEditingValue>` around the action row makes the button react to typing.
- `_submit()` — single-flight guard (`_isSubmitting`), then `createSheet(templateId, sheetName: trimmed)`. On `201`: register the entry, then `Navigator.pop(sheet)`. On failure: set the mapped `failureCopy(error)` and re-enable (CRT-01, 02, 06, 07).
- Local registration: `ref.read(sheetLibraryProvider.notifier).upsert(SheetLibraryEntry.fromSheet)`; a `SheetLibraryException` is caught and swallowed so the `201` still opens the sheet and no second POST is issued (CRT-11).
- While submitting, a `LinearProgressIndicator` at the top of the content shows progress (desktop/AGENTS.md async feedback). The mapped error renders inline as the last content child, directly above the action row (CRT-06).

The obsolete `createSheetCreateTooltip` string ("Sheet creation arrives in a later phase.") is replaced by `createSheetNameRequiredTooltip`; the ARB and generated `app_localizations*.dart` are regenerated.

### `desktop/lib/sheet/sheet_edit_provider.dart` (change)

`openedSheetProvider` (above) and `SheetEditController.build` seeding:

```dart
final opened = ref.watch(openedSheetProvider);
final seed = opened == null ? seedDefaults(template.fields) : _copyTree(opened.data);
return SheetEditState(defaults: seed, data: seed);
```

### `desktop/lib/sheet/sheet_screen.dart` (change)

Header title prefers the opened sheet name: `opened?.name ?? template?.name ?? ''` (CRT-09).

### `desktop/lib/catalog/catalog_screen.dart` (change)

`_openCreationDialog` returns `Future<void>` and now captures the dialog result via `showDialog<Sheet>`; after the `finally` restores the catalog surface, a non-null result pushes Screen 2 with the sheet. `_openSheetScreen` gains an optional `Sheet? sheet` and wraps the route in a `ProviderScope` that overrides both `openedSheetProvider` and the `sheetEditControllerProvider` family when present (the family override forces the notifier into the child scope). The "Preview sheet" action passes no sheet.

## Create flow

```
Catalog card tap
  → showDialog<Sheet>(CreateSheetDialog)
      → primary action (name non-empty, not submitting)
      → POST /sheets {template_id, sheet_name}
          → 201: upsert library entry {id,name,template_id,created_at}
                 → pop(created Sheet)
          → error: inline failureCopy, stay open
  → catalog pushes Screen 2 wrapped in ProviderScope(openedSheet = created)
      → SheetEditController seeds defaults+data from created.data
      → SheetScreen title = created.name
      → one GET /templates/:id renders the fields
```

## Failure handling

| Failure | Behavior |
| ------- | -------- |
| `INVALID_REQUEST` / `NOT_FOUND` / `INTERNAL_ERROR` / offline | `failureCopy(error)` inline above the action row; dialog stays; index unchanged; reactivation issues one further POST (CRT-06, 07) |
| Local `SheetLibraryException` after a `201` | Swallowed; the created sheet still opens; no second POST (CRT-11); index stays at the last persisted state (LIB-11) |
| Double activation while in flight | Second activation is inert (CRT-02) |

## Testing strategy

| Layer | Test | What it proves |
| ----- | ---- | -------------- |
| Edit controller | `desktop/test/sheet/sheet_edit_provider_test.dart` | Opened-sheet seeding from injected `Sheet.data`; `isPending == false`; schema-default path unchanged when no override (CRT-08, 10) |
| Dialog | `desktop/test/catalog/create_sheet_dialog_test.dart` | Empty-name inert + tooltip; trimmed exactly-one POST; progress; mapped inline error with no library entry; retry one further POST; double-click one POST (CRT-01, 02, 05, 06, 07, 11) |
| Screen 2 | `desktop/test/sheet/sheet_screen_test.dart` | Title from the opened sheet name (CRT-09) |
| Navigation (integration) | `desktop/test/catalog/create_sheet_navigation_test.dart` | Full flow: POST → library entry persisted → dialog closed → Screen 2 mounted showing the created name and value (CRT-03, 04, 08) |

Library assertions override `appSupportDirectoryProvider` with a fresh `Directory.systemTemp` so no platform channel is touched (LGN-107 pattern).

## SPEC_DEVIATION

None. The user decision to inject the `201` payload instead of adding `GET /sheets/:id` is recorded in the spec's Assumptions table, not a deviation from the card.
