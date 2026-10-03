import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/sheet/sheet_screen.dart';
import 'package:legnaris/theme/app_theme.dart';

import '../library/library_fixtures.dart';

/// Screen 1 → Screen 2 integration for the library open (OPN-04, OPN-09).
void main() {
  const twoTemplatesBody = '''
{"data": [
  {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}
]}
''';
  const detailBody =
      '{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';
  const sheetBody =
      '{"data": {"id": "sheet-1", "template_id": "fantasy_v1", "name": "Aria", "data": {"stats": {"dexterity": 16}}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';

  Future<List<http.Request>> pumpCatalog(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? handler,
  }) async {
    final recorded = <http.Request>[];
    final supportDir = Directory.systemTemp.createTempSync('lgn108-open');
    addTearDown(() => supportDir.deleteSync(recursive: true));
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(supportDir),
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
                if (handler != null) return handler(request);
                if (request.url.path == '/templates') {
                  return http.Response(twoTemplatesBody, 200);
                }
                if (request.url.path == '/templates/fantasy_v1') {
                  return http.Response(detailBody, 200);
                }
                if (request.url.path == '/sheets/sheet-1') {
                  return http.Response(sheetBody, 200);
                }
                return http.Response('{}', 404);
              }),
            ),
          ),
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const CatalogScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return recorded;
  }

  testWidgets(
    'Happy path: Open on a row mounts Screen 2 and loads the sheet with exactly one GET (OPN-04)',
    (tester) async {
      final recorded = await pumpCatalog(tester);
      final container = ProviderScope.containerOf(
        tester.element(find.byType(CatalogScreen)),
      );
      container.read(sheetLibraryProvider.notifier).upsert(entry('sheet-1'));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('library-name-sheet-1')), findsOneWidget);

      await tester.tap(find.byKey(const Key('library-open-sheet-1')));
      await tester.pumpAndSettle();

      expect(find.byType(SheetScreen), findsOneWidget);
      expect(
        recorded.where((request) => request.url.path == '/sheets/sheet-1'),
        hasLength(1),
      );
      expect(
        tester.widget<Text>(find.byKey(const Key('sheet-title'))).data,
        'Aria',
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
    },
  );

  testWidgets(
    'Edge: an empty library renders the section with no rows (OPN-02)',
    (tester) async {
      await pumpCatalog(tester);

      expect(find.text('My sheets'), findsOneWidget);
      expect(find.text('No sheets created yet'), findsOneWidget);
      expect(find.byType(SheetScreen), findsNothing);
    },
  );

  testWidgets(
    'Edge: a template-detail failure after the sheet loads keeps the library entry and offers Retry (OPN-07 edge)',
    (tester) async {
      await pumpCatalog(
        tester,
        handler: (request) async {
          if (request.url.path == '/templates') {
            return http.Response(twoTemplatesBody, 200);
          }
          if (request.url.path == '/sheets/sheet-1') {
            return http.Response(sheetBody, 200);
          }
          if (request.url.path == '/templates/fantasy_v1') {
            return http.Response(
              '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}',
              500,
            );
          }
          return http.Response('{}', 404);
        },
      );
      final container = ProviderScope.containerOf(
        tester.element(find.byType(CatalogScreen)),
      );
      container.read(sheetLibraryProvider.notifier).upsert(entry('sheet-1'));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('library-open-sheet-1')));
      await tester.pumpAndSettle();

      expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget);
      expect(find.widgetWithText(OutlinedButton, 'Retry'), findsOneWidget);
      expect(
        container
            .read(sheetLibraryProvider)
            .entries
            .map((stored) => stored.id),
        contains('sheet-1'),
        reason: 'a template failure must not evict the local entry',
      );
    },
  );
}
