import 'dart:async';

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
import 'package:legnaris/theme/tokens.dart';

/// Widget tests for the read-only creation preview card (LGN-100,
/// PRV-01 … PRV-06, PRV-09, PRV-10). Every asserted value comes from the spec
/// rule, never from the implementation.
void main() {
  const listBody =
      '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}';
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "baseArmorClass": {"schema": {"type": "number", "label": "Armor class base", "is_editable": true, "default": 10}},
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 0}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "is_editable": false, "default": 0}}}},
  "modifiers": {"children": {"dexterityMod": {"schema": {"type": "number", "label": "", "is_editable": false, "default": 0}}}},
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "Dexterity score", "is_editable": true, "default": 10}}, "constitution": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 12}}}}
}}}
''';
  const groupWithoutLeavesBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": false, "fields": {
  "empty": {"children": {}},
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}
}}}
''';
  const exoticBody = '''
{"data": {"id": "exotic_v1", "name": "Exotic Template", "default": false, "fields": {
  "weird_prefix-z": {"children": {"customThing": {"schema": {"type": "gizmo", "label": "", "is_editable": true, "default": 7}}}},
  "anotherOne": {"schema": {"type": "string", "label": "", "is_editable": true, "default": 3}}
}}}
''';
  const emptyFieldsBody =
      '{"data": {"id": "fantasy_v1", "name": "Blank Template", "default": false, "fields": {}}}';
  const groupsOnlyBody =
      '{"data": {"id": "fantasy_v1", "name": "Empty Groups", "default": false, "fields": {"empty": {"children": {}}, "alsoEmpty": {"children": {"nested": {"children": {}}}}}}}';

  String manyLeavesBody(int count) {
    final leaves = [
      for (var i = 0; i < count; i++)
        '"field$i": {"schema": {"type": "number", "label": "Field $i", "is_editable": true, "default": $i}}',
    ].join(',');
    return '{"data": {"id": "big_v1", "name": "Big Template", "default": false, "fields": {"stats": {"children": {$leaves}}}}}';
  }

  Future<List<http.Request>> pumpCatalog(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? handler,
    Size viewSize = const Size(1280, 800),
  }) async {
    final recorded = <http.Request>[];
    tester.view.physicalSize = viewSize;
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
  Finder inPreview(Finder matching) => find.descendant(
    of: find.byKey(const Key('create-sheet-preview')),
    matching: matching,
  );

  Future<void> openDialog(WidgetTester tester, {String name = 'Fantasy Template'}) async {
    await tester.tap(find.text(name));
    await tester.pumpAndSettle();
  }

  double sectionTop(WidgetTester tester, String key) =>
      tester.getTopLeft(find.byKey(Key('sheet-section-$key'))).dy;

  group('preview card structure (PRV-01, PRV-02, PRV-03)', () {
    testWidgets(
      'Happy path: one labeled section per top-level key in API order, reserved groups included (PRV-01)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        for (final key in [
          'baseArmorClass',
          'config',
          'derived',
          'modifiers',
          'stats',
        ]) {
          expect(
            inPreview(find.byKey(Key('sheet-section-$key'))),
            findsOneWidget,
            reason: 'section $key must render in the preview',
          );
        }

        final order = [
          for (final key in [
            'baseArmorClass',
            'config',
            'derived',
            'modifiers',
            'stats',
          ])
            sectionTop(tester, key),
        ];
        expect(order, orderedEquals([...order]..sort()));
      },
    );

    testWidgets(
      'Happy path: each leaf shows its display name and base value from the schema default (PRV-02)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        expect(inPreview(find.text('Dexterity score')), findsOneWidget);
        expect(inPreview(find.text('Constitution')), findsOneWidget);
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
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.constitution')),
              )
              .controller
              ?.text,
          '12',
        );
      },
    );

    testWidgets('Edge: a group with no leaves is omitted (PRV-01)', (tester) async {
      await pumpCatalog(tester, handler: routing(detail: groupWithoutLeavesBody));
      await openDialog(tester);

      expect(
        inPreview(find.byKey(const Key('sheet-section-empty'))),
        findsNothing,
      );
      expect(
        inPreview(find.byKey(const Key('sheet-section-stats'))),
        findsOneWidget,
      );
    });

    testWidgets(
      'Edge: a synthetic template with unknown prefixes and types renders without code changes (PRV-03)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: exoticBody));
        await openDialog(tester, name: 'Fantasy Template');

        expect(
          inPreview(find.text('Weird prefix z')),
          findsOneWidget,
        );
        expect(inPreview(find.text('Custom thing')), findsOneWidget);
        expect(inPreview(find.text('Another one')), findsWidgets);
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-anotherOne')),
              )
              .controller
              ?.text,
          '3',
        );
      },
    );
  });

  group('stacked layout (PRV-01, PRV-06)', () {
    testWidgets(
      'Happy path: the card header states the preview is read-only (PRV-06)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        expect(inPreview(find.text('Read-only preview')), findsOneWidget);
      },
    );

    testWidgets(
      'Happy path: the preview card renders below the sheet-name input (PRV-01)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        final cardTop =
            tester.getTopLeft(find.byKey(const Key('create-sheet-preview'))).dy;
        final nameTop = tester
            .getTopLeft(find.byKey(const Key('create-sheet-name-input')))
            .dy;

        expect(cardTop, greaterThan(nameTop));
      },
    );

    testWidgets(
      'Happy path: the template name and id share one row, separated by a hairline (PRV-01)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        final name = inDialog(find.text('Fantasy Template'));
        final id = inDialog(find.byKey(const Key('create-sheet-template-id')));
        expect(name, findsOneWidget);
        expect(id, findsOneWidget);
        expect(tester.getCenter(id).dy, closeTo(tester.getCenter(name).dy, 1));
        expect(tester.getCenter(name).dx, lessThan(tester.getCenter(id).dx));
        expect(
          inDialog(find.byKey(const Key('create-sheet-template-id-divider'))),
          findsOneWidget,
        );
      },
    );
  });

  group('read-only and no mutation (PRV-06)', () {
    testWidgets(
      'Edge: no preview control is enabled and the transport carries only GETs (PRV-06)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        await openDialog(tester);

        final textFields = tester.widgetList<TextField>(
          inPreview(find.byType(TextField)),
        );
        expect(textFields, isNotEmpty);
        expect(textFields.every((field) => field.enabled == false), isTrue);

        final switches = tester.widgetList<Switch>(
          inPreview(find.byType(Switch)),
        );
        expect(switches.every((toggle) => toggle.onChanged == null), isTrue);

        expect(recorded.every((request) => request.method == 'GET'), isTrue);
        expect(recorded.any((request) => request.method == 'POST'), isFalse);
      },
    );
  });

  group('preview states (PRV-04, PRV-09)', () {
    testWidgets(
      'Edge: while the detail request is in flight the preview shows loading and no stale sections (PRV-04)',
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

        expect(inDialog(find.text('Loading template...')), findsOneWidget);
        expect(inDialog(find.byKey(const Key('sheet-section-stats'))), findsNothing);

        gate.complete(http.Response(detailBody, 200));
        await tester.pumpAndSettle();
        expect(inDialog(find.text('Loading template...')), findsNothing);
      },
    );

    testWidgets(
      'Edge: an empty fields tree shows the localized no-fields line in the preview (PRV-09)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: emptyFieldsBody));
        await openDialog(tester);

        expect(
          inPreview(find.text('This template has no fields.')),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: a tree of only groups without leaves shows the localized no-fields line (PRV-09)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: groupsOnlyBody));
        await openDialog(tester);

        expect(
          inPreview(find.text('This template has no fields.')),
          findsOneWidget,
        );
        expect(inPreview(find.byType(TextField)), findsNothing);
      },
    );
  });

  group('bounded height (PRV-10)', () {
    testWidgets(
      'Happy path: many leaves scroll inside a bounded card and the name field stays reachable (PRV-10)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: manyLeavesBody(30)));
        await openDialog(tester);

        final scroll = inPreview(find.byType(SingleChildScrollView));
        expect(scroll, findsOneWidget);
        expect(
          tester.getSize(scroll).height,
          lessThanOrEqualTo(AppSizes.previewCardMaxHeight),
        );
        expect(inDialog(find.byKey(const Key('create-sheet-name-input'))), findsOneWidget);
        expect(
          tester.getCenter(find.byKey(const Key('create-sheet-name-input'))).dy,
          lessThan(tester.view.physicalSize.height),
        );
      },
    );

    testWidgets(
      'Edge: at a narrow window the two columns stay side by side and the card scrolls internally (PRV-10)',
      (tester) async {
        await pumpCatalog(
          tester,
          handler: routing(detail: manyLeavesBody(30)),
          viewSize: const Size(700, 800),
        );
        await openDialog(tester);

        expect(
          tester.getSize(find.byType(AlertDialog)).width,
          lessThanOrEqualTo(700),
        );
        expect(
          inDialog(find.byKey(const Key('create-sheet-preview'))),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('create-sheet-name-input'))),
          findsOneWidget,
        );
        expect(
          tester.getSize(inPreview(find.byType(SingleChildScrollView))).height,
          lessThanOrEqualTo(AppSizes.previewCardMaxHeight),
        );
      },
    );
  });
}

Future<http.Response> Function(http.Request) routing({required String detail}) =>
    (request) async => http.Response(
      request.url.path == '/templates'
          ? '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}'
          : detail,
      200,
    );
