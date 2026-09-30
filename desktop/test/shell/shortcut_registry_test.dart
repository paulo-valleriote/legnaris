import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/shell/shortcut_registry.dart';

void main() {
  group('shortcut registry (F5 → refreshCatalog, Esc → closeOverlay)', () {
    test('Edge: appShortcuts spreads the framework defaults instead of replacing them', () {
      final shortcuts = appShortcuts();

      expect(
        shortcuts[const SingleActivator(LogicalKeyboardKey.tab)],
        isA<NextFocusIntent>(),
      );
      expect(
        shortcuts[const SingleActivator(LogicalKeyboardKey.enter)],
        isA<ActivateIntent>(),
      );
      expect(
        shortcuts.keys.length,
        greaterThan(WidgetsApp.defaultShortcuts.length),
      );
    });

    test('Happy path: defaultShortcuts binds refreshCatalog to F5 and closeOverlay to Esc', () {
      expect(
        defaultShortcuts[AppShortcut.refreshCatalog],
        const SingleActivator(LogicalKeyboardKey.f5),
      );
      expect(
        defaultShortcuts[AppShortcut.closeOverlay],
        const SingleActivator(LogicalKeyboardKey.escape),
      );
    });

    test('Happy path: appShortcuts maps F5 to RefreshCatalogIntent and Esc to DismissIntent', () {
      final shortcuts = appShortcuts();

      expect(
        shortcuts[const SingleActivator(LogicalKeyboardKey.f5)],
        isA<RefreshCatalogIntent>(),
      );
      expect(
        shortcuts[const SingleActivator(LogicalKeyboardKey.escape)],
        isA<DismissIntent>(),
      );
    });

    testWidgets('Happy path: appActions registers RefreshCatalogIntent and keeps the default actions', (tester) async {
      late Map<Type, Action<Intent>> actions;
      await tester.pumpWidget(
        ProviderScope(
          child: Consumer(
            builder: (context, ref, child) {
              actions = appActions(ref);
              return const SizedBox();
            },
          ),
        ),
      );

      expect(
        actions[RefreshCatalogIntent],
        isA<CallbackAction<RefreshCatalogIntent>>(),
      );
      expect(actions[NextFocusIntent], isA<NextFocusAction>());
      expect(actions[ScrollIntent], isA<ScrollAction>());
    });
  });
}
