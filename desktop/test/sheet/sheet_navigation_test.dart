import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/sheet_screen.dart';
import 'package:legnaris/theme/app_theme.dart';

void main() {
  const twoTemplatesBody = '''
{"data": [
  {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}},
  {"id": "sci_fi_v1", "name": "Sci Fi Template", "default": false, "fields": {}}
]}
''';
  const detailBody =
      '{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';

  Future<List<http.Request>> pumpCatalog(WidgetTester tester) async {
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
          home: const CatalogScreen(),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return recorded;
  }

  int requestsTo(List<http.Request> recorded, String path) =>
      recorded.where((request) => request.url.path == path).length;

  group('opening Screen 2 from the catalog (RND-14, RND-19)', () {
    testWidgets(
      'Happy path: the card preview action opens Screen 2 for its template (RND-14)',
      (tester) async {
        final recorded = await pumpCatalog(tester);

        final action = find.byKey(const Key('catalog-preview-fantasy_v1'));
        expect(action, findsOneWidget);
        expect(
          find.widgetWithText(TextButton, 'Preview sheet'),
          findsNWidgets(2),
        );

        await tester.tap(action);
        await tester.pumpAndSettle();

        expect(find.byType(SheetScreen), findsOneWidget);
        expect(find.byKey(const Key('sheet-section-stats')), findsOneWidget);
        expect(requestsTo(recorded, '/templates/fantasy_v1'), 1);
        expect(recorded.where((request) => request.method != 'GET'), isEmpty);
      },
    );
  });

  group('returning to the catalog (RND-18)', () {
    testWidgets(
      'Happy path: back returns to the cached catalog without a new list request (RND-18)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        final listRequestsBefore = requestsTo(recorded, '/templates');

        await tester.tap(find.byKey(const Key('catalog-preview-fantasy_v1')));
        await tester.pumpAndSettle();
        expect(find.byType(SheetScreen), findsOneWidget);

        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();

        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(Card), findsNWidgets(2));
        expect(requestsTo(recorded, '/templates'), listRequestsBefore);
        expect(recorded.where((request) => request.method != 'GET'), isEmpty);
      },
    );
  });
}
