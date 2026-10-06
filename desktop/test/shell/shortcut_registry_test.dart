import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/sheet/sheet_refresh.dart';
import 'package:legnaris/shell/shortcut_registry.dart';

void main() {
  const emptyListBody = '{"data": []}';

  /// Pumps the actions registry over a mocked API and hands back the
  /// recorded requests, the container behind the captured [WidgetRef], and
  /// the registered actions.
  Future<
    ({
      List<http.Request> recorded,
      ProviderContainer container,
      Map<Type, Action<Intent>> actions,
    })
  >
  pumpActions(WidgetTester tester) async {
    final recorded = <http.Request>[];
    late Map<Type, Action<Intent>> actions;
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
                return http.Response(emptyListBody, 200);
              }),
            ),
          ),
        ],
        child: Consumer(
          builder: (context, ref, child) {
            actions = appActions(ref);
            return const SizedBox();
          },
        ),
      ),
    );
    final container = ProviderScope.containerOf(
      tester.element(find.byType(Consumer)),
    );
    return (recorded: recorded, container: container, actions: actions);
  }

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

    testWidgets(
      'Happy path: appActions registers RefreshCatalogIntent and keeps the default actions',
      (tester) async {
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
      },
    );
  });

  group('F5 dispatch by surface (LGN-102)', () {
    testWidgets(
      'Happy path: F5 with a registered sheet bumps the refresh tick and issues no catalog request (GRN-17 … GRN-19)',
      (tester) async {
        final harness = await pumpActions(tester);
        // The mounted sheet screen is the seam's only listener.
        harness.container.listen(activeSheetIdProvider, (_, _) {});
        harness.container.listen(sheetRefreshTickProvider, (_, _) {});
        harness.container
            .read(activeSheetIdProvider.notifier)
            .set('fantasy_v1');

        const ActionDispatcher().invokeAction(
          harness.actions[RefreshCatalogIntent]!,
          const RefreshCatalogIntent(),
        );
        await tester.pumpAndSettle();

        expect(harness.container.read(sheetRefreshTickProvider), 1);
        expect(
          harness.recorded,
          isEmpty,
          reason: 'F5 aimed at the sheet must not touch the catalog',
        );
      },
    );

    testWidgets(
      'Happy path: F5 without a registered sheet refreshes the catalog (existing behavior)',
      (tester) async {
        final harness = await pumpActions(tester);
        expect(harness.container.read(activeSheetIdProvider), isNull);

        const ActionDispatcher().invokeAction(
          harness.actions[RefreshCatalogIntent]!,
          const RefreshCatalogIntent(),
        );
        await tester.pumpAndSettle();

        expect(
          harness.recorded.where((request) => request.url.path == '/templates'),
          hasLength(1),
          reason: 'the catalog refresh must run exactly once',
        );
        expect(
          harness.recorded.every((request) => request.method == 'GET'),
          isTrue,
        );
      },
    );
  });

  group('Ctrl+S dispatch by surface (LGN-110)', () {
    test('Happy path: defaultShortcuts binds saveSheet to Ctrl+S and appShortcuts maps it', () {
      expect(
        defaultShortcuts[AppShortcut.saveSheet],
        const SingleActivator(LogicalKeyboardKey.keyS, control: true),
      );
      expect(
        appShortcuts()[
            const SingleActivator(LogicalKeyboardKey.keyS, control: true)],
        isA<SaveSheetIntent>(),
      );
    });

    testWidgets(
      'Happy path: Ctrl+S with a registered sheet bumps the save tick and issues no request (SAV-05)',
      (tester) async {
        final harness = await pumpActions(tester);
        // The mounted sheet screen is the seam's only listener.
        harness.container.listen(activeSheetIdProvider, (_, _) {});
        harness.container.listen(sheetSaveTickProvider, (_, _) {});
        harness.container
            .read(activeSheetIdProvider.notifier)
            .set('12345678-1234-7000-8000-123456789abc');

        const ActionDispatcher().invokeAction(
          harness.actions[SaveSheetIntent]!,
          const SaveSheetIntent(),
        );
        await tester.pumpAndSettle();

        expect(harness.container.read(sheetSaveTickProvider), 1);
        expect(harness.recorded, isEmpty);
      },
    );

    testWidgets(
      'Edge: Ctrl+S with no registered sheet bumps nothing and issues no request (SAV-07)',
      (tester) async {
        final harness = await pumpActions(tester);
        expect(harness.container.read(activeSheetIdProvider), isNull);
        // Keep the autoDispose tick alive so an unconditional bump would be
        // observable and kill the mutant that drops the guard.
        harness.container.listen(sheetSaveTickProvider, (_, _) {});

        const ActionDispatcher().invokeAction(
          harness.actions[SaveSheetIntent]!,
          const SaveSheetIntent(),
        );
        await tester.pumpAndSettle();

        expect(harness.container.read(sheetSaveTickProvider), 0);
        expect(harness.recorded, isEmpty);
      },
    );
  });
}
