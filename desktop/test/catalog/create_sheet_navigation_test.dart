import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/sheet/sheet_screen.dart';
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
  const detailBody =
      '{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';

  Future<List<http.Request>> pumpCatalog(
    WidgetTester tester,
    _PushRecorder recorder, {
    Future<http.Response> Function(http.Request request)? handler,
  }) async {
    final recorded = <http.Request>[];
    final supportDir = Directory.systemTemp.createTempSync('lgn109-nav');
    addTearDown(() => supportDir.deleteSync(recursive: true));
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(supportDir),
          // The created sheet only registers inside a live session
          // (DATA-01 writer guard), matching the authenticated surface this
          // flow runs on.
          authSessionProvider.overrideWithValue(
            const AuthAuthenticated('access-test'),
          ),
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
                if (handler != null) return handler(request);
                return http.Response(
                  request.url.path == '/templates'
                      ? twoTemplatesBody
                      : detailBody,
                  200,
                );
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

  int listRequestCount(List<http.Request> recorded) => recorded
      .where((request) => request.url.path == '/templates')
      .length;

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
        expect(listRequestCount(recorded), 1);

        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(find.byType(Card), findsNWidgets(2));
        expect(currentSurface(tester), ShellSurface.catalog);
        expect(listRequestCount(recorded), 1);
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
        expect(listRequestCount(recorded), 1);
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
        expect(listRequestCount(recorded), 1);
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
          expect(listRequestCount(recorded), 1);
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

  group('creating a sheet from the dialog (CRT-03, CRT-04, CRT-08)', () {
    const createdBody =
        '{"data": {"id": "12345678-1234-7000-8000-123456789abc", "template_id": "fantasy_v1", "name": "Aragorn", "data": {"stats": {"dexterity": 16}}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';

    ProviderContainer containerOf(WidgetTester tester) =>
        ProviderScope.containerOf(tester.element(find.byType(MaterialApp)));

    testWidgets(
      'Happy path: submitting creates the sheet, registers it, and opens Screen 2 with the created values (CRT-03, CRT-04, CRT-08)',
      (tester) async {
        final recorder = _PushRecorder();
        final recorded = await pumpCatalog(
          tester,
          recorder,
          handler: (request) async {
            if (request.url.path == '/templates') {
              return http.Response(twoTemplatesBody, 200);
            }
            if (request.url.path == '/templates/fantasy_v1') {
              return http.Response(detailBody, 200);
            }
            if (request.url.path == '/sheets') {
              return http.Response(createdBody, 201);
            }
            return http.Response('{}', 404);
          },
        );

        await openDialog(tester, 'Fantasy Template');
        await tester.enterText(
          find.byKey(const Key('create-sheet-name-input')),
          'Aragorn',
        );
        await tester.pump();
        await tester.tap(find.byKey(const Key('create-sheet-submit')));
        await tester.pumpAndSettle();

        final posts = recorded
            .where(
              (request) =>
                  request.method == 'POST' && request.url.path == '/sheets',
            )
            .toList();
        expect(posts, hasLength(1));
        expect(jsonDecode(posts.single.body), {
          'template_id': 'fantasy_v1',
          'sheet_name': 'Aragorn',
        });

        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(find.byType(SheetScreen), findsOneWidget);
        expect(
          tester.widget<Text>(find.byKey(const Key('sheet-title'))).data,
          'Aragorn',
        );
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '16',
        );

        final entry = containerOf(
          tester,
        ).read(sheetLibraryProvider).entries.single;
        expect(entry.id, '12345678-1234-7000-8000-123456789abc');
        expect(entry.name, 'Aragorn');
        expect(entry.templateId, 'fantasy_v1');
      },
    );

    testWidgets(
      'Edge: the catalog offers no Preview sheet action; every card only creates (OPN-20, OPN-21)',
      (tester) async {
        final recorder = _PushRecorder();
        final recorded = await pumpCatalog(tester, recorder);

        expect(find.text('Preview sheet'), findsNothing);
        expect(find.byKey(const Key('catalog-preview-fantasy_v1')), findsNothing);
        expect(
          find.widgetWithText(TextButton, 'Create sheet'),
          findsNWidgets(2),
        );
        expect(recorded.where((request) => request.method != 'GET'), isEmpty);
        expect(containerOf(tester).read(sheetLibraryProvider).entries, isEmpty);
      },
    );

    testWidgets(
      'Happy path: back from the created sheet returns to the cached catalog with no new list request (RND-18)',
      (tester) async {
        final recorder = _PushRecorder();
        final recorded = await pumpCatalog(
          tester,
          recorder,
          handler: (request) async {
            if (request.url.path == '/templates') {
              return http.Response(twoTemplatesBody, 200);
            }
            if (request.url.path == '/templates/fantasy_v1') {
              return http.Response(detailBody, 200);
            }
            if (request.url.path == '/sheets') {
              return http.Response(createdBody, 201);
            }
            return http.Response('{}', 404);
          },
        );

        await openDialog(tester, 'Fantasy Template');
        await tester.enterText(
          find.byKey(const Key('create-sheet-name-input')),
          'Aragorn',
        );
        await tester.pump();
        await tester.tap(find.byKey(const Key('create-sheet-submit')));
        await tester.pumpAndSettle();
        expect(find.byType(SheetScreen), findsOneWidget);

        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();

        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(Card), findsNWidgets(2));
        expect(listRequestCount(recorded), 1);
      },
    );
  });
}
