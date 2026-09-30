import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/shell/shell_state.dart';
import 'package:legnaris/theme/app_theme.dart';
import 'package:legnaris/theme/tokens.dart';

/// Records route pushes so CAT-17 can count what sits above the catalog.
class _PushRecorder extends NavigatorObserver {
  final List<Route<dynamic>> pushed = <Route<dynamic>>[];

  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) {
    pushed.add(route);
  }
}

void main() {
  const twoTemplatesBody = '''
{"data": [
  {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}},
  {"id": "sci_fi_v1", "name": "Sci Fi Template", "default": false, "fields": {}}
]}
''';

  Future<List<http.Request>> pumpCatalog(
    WidgetTester tester,
    _PushRecorder recorder, {
    Future<http.Response> Function(http.Request request)? handler,
  }) async {
    final recorded = <http.Request>[];
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
                return handler == null
                    ? http.Response(twoTemplatesBody, 200)
                    : handler(request);
              }),
            ),
          ),
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          navigatorObservers: [recorder],
          home: const CatalogScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return recorded;
  }

  ShellSurface currentSurface(WidgetTester tester) =>
      ProviderScope.containerOf(tester.element(find.byType(MaterialApp)))
          .read(shellStateProvider);

  Future<void> openDialog(WidgetTester tester, String cardName) async {
    await tester.tap(find.text(cardName));
    await tester.pumpAndSettle();
  }

  group('dismissing the creation dialog (CAT-16)', () {
    testWidgets(
      'Edge: Esc returns to the cached catalog with no new list request (CAT-16)',
      (tester) async {
        final recorder = _PushRecorder();
        final recorded = await pumpCatalog(tester, recorder);
        await openDialog(tester, 'Fantasy Template');

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(currentSurface(tester), ShellSurface.creationDialog);
        expect(recorded, hasLength(1));

        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(find.byType(Card), findsNWidgets(2));
        expect(currentSurface(tester), ShellSurface.catalog);
        expect(recorded, hasLength(1));
      },
    );

    testWidgets(
      'Edge: the Close action returns to the cached catalog with no new list request (CAT-16)',
      (tester) async {
        final recorder = _PushRecorder();
        final recorded = await pumpCatalog(tester, recorder);
        await openDialog(tester, 'Fantasy Template');

        final close = find.widgetWithText(TextButton, 'Close');
        expect(close, findsOneWidget);
        await tester.tap(close);
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(find.byType(Card), findsNWidgets(2));
        expect(currentSurface(tester), ShellSurface.catalog);
        expect(recorded, hasLength(1));
      },
    );

    testWidgets(
      'Edge: tapping the barrier dismisses the dialog and restores the catalog surface (CAT-16)',
      (tester) async {
        final recorder = _PushRecorder();
        final recorded = await pumpCatalog(tester, recorder);
        await openDialog(tester, 'Fantasy Template');
        expect(currentSurface(tester), ShellSurface.creationDialog);

        await tester.tapAt(const Offset(4, 4));
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(find.byType(Card), findsNWidgets(2));
        expect(currentSurface(tester), ShellSurface.catalog);
        expect(recorded, hasLength(1));
      },
    );
  });

  group('shallow navigation (CAT-17)', () {
    testWidgets(
      'Happy path: opening the dialog pushes exactly one route above the catalog (CAT-17)',
      (tester) async {
        final recorder = _PushRecorder();
        await pumpCatalog(tester, recorder);
        final pushesBeforeDialog = recorder.pushed.length;

        await openDialog(tester, 'Fantasy Template');
        expect(recorder.pushed.length, pushesBeforeDialog + 1);

        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();

        expect(recorder.pushed.length, pushesBeforeDialog + 1);
        expect(find.byType(Card), findsNWidgets(2));
      },
    );
  });

  group('opening the creation dialog (CAT-15)', () {
    final cases =
        <String, ({String cardName, bool viaAction, String expectedId})>{
          'the card itself': (
            cardName: 'Fantasy Template',
            viaAction: false,
            expectedId: 'fantasy_v1',
          ),
          'the Create sheet action': (
            cardName: 'Sci Fi Template',
            viaAction: true,
            expectedId: 'sci_fi_v1',
          ),
        };

    for (final entry in cases.entries) {
      final tc = entry.value;
      testWidgets(
        'Happy path: activating ${entry.key} opens the dialog for ${tc.expectedId} (CAT-15)',
        (tester) async {
          final recorder = _PushRecorder();
          final recorded = await pumpCatalog(tester, recorder);

          final card = find.widgetWithText(Card, tc.cardName);
          await tester.tap(
            tc.viaAction
                ? find.descendant(
                    of: card,
                    matching: find.widgetWithText(TextButton, 'Create sheet'),
                  )
                : find.descendant(of: card, matching: find.text(tc.cardName)),
          );
          await tester.pumpAndSettle();

          final dialog = find.byType(CreateSheetDialog);
          expect(dialog, findsOneWidget);
          final shownId = find.descendant(
            of: dialog,
            matching: find.byKey(const Key('create-sheet-template-id')),
          );
          expect(shownId, findsOneWidget);
          expect(tester.widget<Text>(shownId).data, tc.expectedId);
          expect(
            find.descendant(of: dialog, matching: find.text('Create sheet')),
            findsOneWidget,
          );
          expect(
            find.descendant(of: dialog, matching: find.text('Template')),
            findsOneWidget,
          );
          expect(currentSurface(tester), ShellSurface.creationDialog);
          expect(recorded, hasLength(1));
        },
      );
    }
  });

  group('dialog typography and control sizing (UAT T8)', () {
    testWidgets(
      'Edge: the label demotes to caption while the id stays on the body token (T8)',
      (tester) async {
        final recorder = _PushRecorder();
        await pumpCatalog(tester, recorder);
        await openDialog(tester, 'Fantasy Template');

        final dialog = find.byType(CreateSheetDialog);
        final label = tester.widget<Text>(
          find.descendant(of: dialog, matching: find.text('Template')),
        );
        expect(label.style?.fontSize, AppSizes.caption);
        expect(
          label.style?.color,
          Theme.of(tester.element(dialog)).colorScheme.onSurfaceVariant,
        );

        final id = tester.widget<Text>(
          find.descendant(
            of: dialog,
            matching: find.byKey(const Key('create-sheet-template-id')),
          ),
        );
        expect(id.style?.fontSize, AppSizes.body);
      },
    );

    testWidgets(
      'Edge: the Close action measures the control height token (T8)',
      (tester) async {
        final recorder = _PushRecorder();
        await pumpCatalog(tester, recorder);
        await openDialog(tester, 'Fantasy Template');

        expect(
          tester.getSize(find.widgetWithText(TextButton, 'Close')).height,
          AppSizes.controlHeight,
        );
      },
    );
  });
}
