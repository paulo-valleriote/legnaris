import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/catalog/catalog_provider.dart';
import 'package:legnaris/sheet/sheet_refresh.dart';
import 'package:legnaris/shell/shell_state.dart';

/// The actions the shortcut registry can invoke (desktop/AGENTS.md: one
/// central, rebindable registry — plain action → activator map).
enum AppShortcut { refreshCatalog, saveSheet, closeOverlay }

/// Catalog refresh, fired by the F5 binding of [defaultShortcuts].
class RefreshCatalogIntent extends Intent {
  const RefreshCatalogIntent();
}

/// Sheet save, fired by the `Ctrl+S` binding of [defaultShortcuts].
class SaveSheetIntent extends Intent {
  const SaveSheetIntent();
}

/// V1 default bindings: F5 refreshes the catalog, `Ctrl+S` saves the open
/// sheet, Esc closes the top overlay.
const Map<AppShortcut, ShortcutActivator> defaultShortcuts = {
  AppShortcut.refreshCatalog: SingleActivator(LogicalKeyboardKey.f5),
  AppShortcut.saveSheet: SingleActivator(
    LogicalKeyboardKey.keyS,
    control: true,
  ),
  AppShortcut.closeOverlay: SingleActivator(LogicalKeyboardKey.escape),
};

/// The registry consumed by `MaterialApp.shortcuts`: [defaultShortcuts]
/// translated to intents, on top of the framework's own bindings. Passing a
/// custom map to `MaterialApp` *replaces* the defaults (widgets/app.dart), so
/// [WidgetsApp.defaultShortcuts] is spread in — Esc resolves to the framework
/// [DismissIntent] either way, which `ModalRoute` gates on
/// `barrierDismissible` (routes.dart:981).
Map<ShortcutActivator, Intent> appShortcuts() => <ShortcutActivator, Intent>{
  ...WidgetsApp.defaultShortcuts,
  defaultShortcuts[AppShortcut.refreshCatalog]!: const RefreshCatalogIntent(),
  defaultShortcuts[AppShortcut.saveSheet]!: const SaveSheetIntent(),
  defaultShortcuts[AppShortcut.closeOverlay]!: const DismissIntent(),
};

/// The registry consumed by `MaterialApp.actions`: the catalog refresh action
/// on top of the framework defaults (same replacement rule as [appShortcuts]).
/// Dispatch is by surface: with a sheet screen mounted, F5 bumps its refresh
/// tick and never touches the catalog (the sheet owns its refresh flow,
/// GRN-17 … GRN-19); otherwise the refresh only runs while the catalog
/// surface is active — the binding sits above the Navigator, so it also
/// fires with the creation dialog open, where the spec forbids a catalog
/// request (F5 edge case). `Ctrl+S` bumps the active sheet's save tick and is
/// a no-op with no sheet mounted (SAV-05, SAV-07).
Map<Type, Action<Intent>> appActions(WidgetRef ref) => <Type, Action<Intent>>{
  ...WidgetsApp.defaultActions,
  RefreshCatalogIntent: CallbackAction<RefreshCatalogIntent>(
    onInvoke: (intent) {
      if (ref.read(activeSheetIdProvider) != null) {
        ref.read(sheetRefreshTickProvider.notifier).bump();
        return null;
      }
      if (ref.read(shellStateProvider) != ShellSurface.catalog) return null;
      unawaited(ref.read(catalogProvider.notifier).refresh());
      return null;
    },
  ),
  SaveSheetIntent: CallbackAction<SaveSheetIntent>(
    onInvoke: (intent) {
      if (ref.read(activeSheetIdProvider) != null) {
        ref.read(sheetSaveTickProvider.notifier).bump();
      }
      return null;
    },
  ),
};
