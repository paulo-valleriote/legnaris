import 'dart:async';

import 'package:flutter/gestures.dart' show PointerDeviceKind;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/app_theme.dart';

/// Widget tests for the creation surface content
/// (TPL-01, TPL-02, TPL-04, TPL-11, TPL-12, TPL-13). Every asserted value comes
/// from the spec rule, never from the implementation.
void main() {
  const listBody =
      '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}';
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "baseArmorClass": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}},
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 0}}, "acUsesCon": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 0}}}},
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}, "constitution": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}
}}}
''';
  const emptyFieldsBody =
      '{"data": {"id": "fantasy_v1", "name": "Blank Template", "default": false, "fields": {}}}';
  const emptyNameBody =
      '{"data": {"id": "fantasy_v1", "name": "", "default": false, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';

  Future<http.Response> Function(http.Request) routing({
    required String detail,
  }) => (request) async => http.Response(
    request.url.path == '/templates' ? listBody : detail,
    200,
  );

  Future<List<http.Request>> pumpCatalog(
    WidgetTester tester, {
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
                if (handler != null) return handler(request);
                return http.Response(
                  request.url.path == '/templates' ? listBody : detailBody,
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

  Finder inDialog(Finder matching) =>
      find.descendant(of: find.byType(CreateSheetDialog), matching: matching);

  Future<void> openDialog(WidgetTester tester) async {
    await tester.tap(find.text('Fantasy Template'));
    await tester.pumpAndSettle();
  }

  group('rendering the detail (TPL-01, TPL-02)', () {
    testWidgets(
      'Happy path: opening shows the template name and one summary line per top-level key (TPL-02, TPL-03)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        expect(
          inDialog(find.text('Fantasy Template')),
          findsOneWidget,
        );
        expect(inDialog(find.text('baseArmorClass: 1')), findsOneWidget);
        expect(inDialog(find.text('config: 2')), findsOneWidget);
        expect(inDialog(find.text('stats: 2')), findsOneWidget);
      },
    );

    testWidgets(
      'Happy path: opening requests GET /templates/:id exactly once (TPL-01)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        await openDialog(tester);

        final detailRequests = recorded
            .where((request) => request.url.path == '/templates/fantasy_v1')
            .toList();
        expect(detailRequests, hasLength(1));
        expect(detailRequests.single.method, 'GET');
      },
    );

    testWidgets(
      'Edge: an empty fields tree shows the localized no-fields line in place of the summary (TPL-02)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: emptyFieldsBody));
        await openDialog(tester);

        expect(
          inDialog(find.text('This template has no fields.')),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: a template with an empty name still renders the layout (spec edge case)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: emptyNameBody));
        await openDialog(tester);

        expect(
          inDialog(find.byKey(const Key('create-sheet-template-id'))),
          findsOneWidget,
        );
        expect(inDialog(find.text('stats: 1')), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: while the detail request is in flight the dialog shows loading and no stale summary (TPL-04)',
      (tester) async {
        final gate = Completer<http.Response>();
        await pumpCatalog(
          tester,
          handler: (request) async =>
              request.url.path == '/templates'
              ? http.Response(listBody, 200)
              : gate.future,
        );

        await tester.tap(find.text('Fantasy Template'));
        await tester.pump();

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(inDialog(find.text('Loading template...')), findsOneWidget);
        expect(inDialog(find.textContaining('config:')), findsNothing);

        gate.complete(http.Response(detailBody, 200));
        await tester.pumpAndSettle();
        expect(inDialog(find.text('Loading template...')), findsNothing);
      },
    );
  });

  group('sheet-name input and primary action (TPL-11, TPL-12, TPL-13)', () {
    testWidgets(
      'Happy path: the sheet-name input is empty with a localized label and placeholder (TPL-11)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        expect(inDialog(find.text('Sheet name')), findsOneWidget);
        expect(inDialog(find.text('e.g. My Character')), findsOneWidget);
        expect(
          tester.widget<TextField>(find.byType(TextField)).controller?.text,
          '',
        );
      },
    );

    testWidgets(
      'Edge: typing a sheet name issues no request (TPL-11, TPL-13)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        await openDialog(tester);
        final before = recorded.length;

        await tester.enterText(find.byType(TextField), 'My Character');
        await tester.pump();

        expect(recorded.length, before);
      },
    );

    testWidgets(
      'Happy path: the primary action is disabled and exposes the tooltip copy (TPL-12)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        final button = tester.widget<TextButton>(
          inDialog(find.widgetWithText(TextButton, 'Create sheet')),
        );
        expect(button.onPressed, isNull);

        final tooltip = tester.widget<Tooltip>(
          find.ancestor(
            of: inDialog(find.text('Create sheet')),
            matching: find.byType(Tooltip),
          ),
        );
        expect(tooltip.message, 'Sheet creation arrives in a later phase.');
      },
    );

    testWidgets(
      'Happy path: hovering the disabled action reveals the tooltip (TPL-12)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        final gesture = await tester.createGesture(kind: PointerDeviceKind.mouse);
        await gesture.addPointer(location: Offset.zero);
        addTearDown(gesture.removePointer);
        await tester.pump();
        await gesture.moveTo(
          tester.getCenter(
            inDialog(find.widgetWithText(TextButton, 'Create sheet')),
          ),
        );
        await tester.pump(const Duration(milliseconds: 100));

        expect(
          find.text('Sheet creation arrives in a later phase.'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: opening the dialog issues no mutation — only the read GETs (TPL-13)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        await openDialog(tester);

        expect(recorded.every((request) => request.method == 'GET'), isTrue);
        expect(recorded.any((request) => request.method == 'POST'), isFalse);
      },
    );
  });
}
