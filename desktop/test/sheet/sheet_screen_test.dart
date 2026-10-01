import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/sheet_screen.dart';
import 'package:legnaris/theme/app_theme.dart';

void main() {
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}},
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 1}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "formula": "x + 1", "is_editable": false, "default": 0}}}}
}}}
''';
  const emptyFieldsBody =
      '{"data": {"id": "blank", "name": "Blank", "default": false, "fields": {}}}';
  const emptyNameBody =
      '{"data": {"id": "blank", "name": "", "default": false, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

  Future<List<http.Request>> pumpScreen(
    WidgetTester tester, {
    required Future<http.Response> Function(http.Request request) handler,
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
                return handler(request);
              }),
            ),
          ),
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: SheetScreen(templateId: 'fantasy_v1')),
        ),
      ),
    );
    await tester.pump();
    return recorded;
  }

  group('loading, seed and render (RND-15, RND-16)', () {
    testWidgets(
      'Happy path: one detail request, loading first, then sections seeded from defaults (RND-15, RND-16)',
      (tester) async {
        final gate = Completer<http.Response>();
        final recorded = await pumpScreen(tester, handler: (_) => gate.future);

        expect(recorded, hasLength(1));
        expect(recorded.single.url.path, '/templates/fantasy_v1');
        expect(find.byType(LinearProgressIndicator), findsOneWidget);
        expect(find.text('Loading template...'), findsOneWidget);

        gate.complete(http.Response(detailBody, 200));
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('sheet-section-stats')), findsOneWidget);
        expect(find.byKey(const Key('sheet-section-config')), findsOneWidget);
        expect(find.byKey(const Key('sheet-section-derived')), findsOneWidget);
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '10',
        );
        expect(
          tester
              .widget<Switch>(
                find.byKey(const Key('sheet-toggle-config.acUsesDex')),
              )
              .value,
          isTrue,
        );
        expect(find.text('Calculated'), findsOneWidget);
        expect(
          recorded.where((request) => request.method != 'GET'),
          isEmpty,
        );
      },
    );
  });

  group('error handling (RND-17)', () {
    testWidgets(
      'Error: a failed detail shows the mapped message and Retry re-requests once (RND-17)',
      (tester) async {
        var call = 0;
        final recorded = await pumpScreen(
          tester,
          handler: (request) async {
            final first = call++ == 0;
            return http.Response(
              first ? serverErrorBody : detailBody,
              first ? 500 : 200,
            );
          },
        );
        await tester.pumpAndSettle();

        expect(recorded, hasLength(1));
        expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget);
        expect(find.widgetWithText(OutlinedButton, 'Retry'), findsOneWidget);
        expect(find.byKey(const Key('sheet-section-stats')), findsNothing);

        await tester.tap(find.widgetWithText(OutlinedButton, 'Retry'));
        await tester.pumpAndSettle();

        expect(recorded, hasLength(2));
        expect(find.byKey(const Key('sheet-section-stats')), findsOneWidget);
        expect(find.text(errorCopy(ApiErrorKind.serverError)), findsNothing);
      },
    );
  });

  group('edges (RND-16, RND-19)', () {
    testWidgets(
      'Edge: an empty field tree renders the single localized line',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(emptyFieldsBody, 200),
        );
        await tester.pumpAndSettle();

        expect(find.text('This template has no fields.'), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: an empty template name renders an empty title without throwing',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(emptyNameBody, 200),
        );
        await tester.pumpAndSettle();

        expect(
          tester.widget<Text>(find.byKey(const Key('sheet-title'))).data,
          '',
        );
        expect(find.byKey(const Key('sheet-section-stats')), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: editing a field updates the local draft and issues no mutation (RND-19)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        final input = find.byKey(const Key('sheet-input-stats.dexterity'));
        await tester.enterText(input, '42');
        await tester.pump();

        expect(tester.widget<TextField>(input).controller?.text, '42');
        expect(
          tester
              .widget<Switch>(
                find.byKey(const Key('sheet-toggle-config.acUsesDex')),
              )
              .value,
          isTrue,
        );
        expect(recorded, hasLength(1));
        expect(recorded.single.method, 'GET');
      },
    );
  });
}
