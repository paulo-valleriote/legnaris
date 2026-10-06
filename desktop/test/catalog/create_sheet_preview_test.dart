import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/engine/exclusive_control.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
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
  const exclusivePairBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "Armor class uses dexterity", "is_editable": true, "default": 1, "exclusive_group": "armorClassAbility"}}, "acUsesCon": {"schema": {"type": "number", "label": "Armor class uses constitution", "is_editable": true, "default": 0, "exclusive_group": "armorClassAbility"}}, "spellUsesCharisma": {"schema": {"type": "number", "label": "Spellcasting uses charisma", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesConstitution": {"schema": {"type": "number", "label": "Spellcasting uses constitution", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesDexterity": {"schema": {"type": "number", "label": "Spellcasting uses dexterity", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesIntelligence": {"schema": {"type": "number", "label": "Spellcasting uses intelligence", "is_editable": true, "default": 1, "exclusive_group": "spellcastingAbility"}}, "spellUsesStrength": {"schema": {"type": "number", "label": "Spellcasting uses strength", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesWisdom": {"schema": {"type": "number", "label": "Spellcasting uses wisdom", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}}}
}}}
''';
  const groupsOnlyBody =
      '{"data": {"id": "fantasy_v1", "name": "Empty Groups", "default": false, "fields": {"empty": {"children": {}}, "alsoEmpty": {"children": {"nested": {"children": {}}}}}}}';
  // An exclusive group whose members all default to 0: the dropdown must offer
  // a disabled `None` entry in the read-only preview (EXC-10, EXC-27).
  const exclusiveNoneBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "config": {"children": {"spellUsesCharisma": {"schema": {"type": "number", "label": "Spellcasting uses charisma", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesStrength": {"schema": {"type": "number", "label": "Spellcasting uses strength", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}}}
}}}
''';

  // A template whose skills group declares a table row (RND-21): the preview
  // must inherit the same header + aligned cells as the editable screen.
  const tableBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "skills": {"children": {"acrobatics": {"label": "", "columns": ["bonus", "total"], "children": {"bonus": {"schema": {"type": "number", "label": "Acrobatics bonus", "is_editable": true, "default": 0, "column_label": "Bonus"}}, "total": {"schema": {"type": "number", "label": "Acrobatics total", "is_editable": false, "default": 0, "column_label": "Total"}}}}}}
}}}
''';

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
    final supportDir = Directory.systemTemp.createTempSync('lgn108-preview');
    addTearDown(() => supportDir.deleteSync(recursive: true));
    tester.view.physicalSize = viewSize;
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
        // Unknown types keep the schema default (seedValue falls back to it).
        expect(
          inPreview(
            find.byKey(const Key('sheet-value-weird_prefix-z.customThing')),
          ),
          findsOneWidget,
        );
        expect(
          tester
              .widget<Text>(
                find.byKey(const Key('sheet-value-weird_prefix-z.customThing')),
              )
              .data,
          '7',
        );
        expect(inPreview(find.text('Another one')), findsWidgets);
        // String/text leaves seed empty, mirroring the backend's SeedValue —
        // the numeric default never reaches the sheet value.
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-anotherOne')),
              )
              .controller
              ?.text,
          '',
        );
      },
    );
    testWidgets(
      'Happy path: a table group renders its header and row in the preview (RND-21)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: tableBody));
        await openDialog(tester);

        expect(
          inPreview(find.byKey(const Key('sheet-table-header-skills-0'))),
          findsOneWidget,
        );
        expect(inPreview(find.text('Bonus')), findsOneWidget);
        expect(inPreview(find.text('Total')), findsOneWidget);
        expect(inPreview(find.text('Acrobatics')), findsOneWidget);
        expect(
          inPreview(
            find.byKey(const Key('sheet-input-skills.acrobatics.bonus')),
          ),
          findsOneWidget,
        );
        expect(
          inPreview(
            find.byKey(const Key('sheet-value-skills.acrobatics.total')),
          ),
          findsOneWidget,
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

        final checkboxes = tester.widgetList<Checkbox>(
          inPreview(find.byType(Checkbox)),
        );
        expect(checkboxes, isNotEmpty);
        expect(checkboxes.every((toggle) => toggle.onChanged == null), isTrue);

        expect(recorded.every((request) => request.method == 'GET'), isTrue);
        expect(recorded.any((request) => request.method == 'POST'), isFalse);
      },
    );

    testWidgets(
      'Happy path: the preview renders the exclusive group disabled with the default option selected (EXC-11, EXC-15)',
      (tester) async {
        final recorded = await pumpCatalog(
          tester,
          handler: routing(detail: exclusivePairBody),
        );
        await openDialog(tester);

        final dropdown = tester.widget<DropdownButton<String>>(
          inPreview(
            find.byKey(const Key('sheet-exclusive-armorClassAbility')),
          ),
        );
        expect(dropdown.value, 'config.acUsesDex');
        expect(dropdown.onChanged, isNull);
        expect(
          dropdown.items!.map((item) => item.value),
          isNot(contains(exclusiveNoneValue)),
          reason: 'the defaulted pair offers no None',
        );
        expect(
          inPreview(find.byKey(const Key('sheet-checkbox-config.acUsesDex'))),
          findsNothing,
        );
        expect(
          inPreview(find.byKey(const Key('sheet-checkbox-config.acUsesCon'))),
          findsNothing,
        );
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
      },
    );

    testWidgets(
      'Happy path: the preview renders the spellcasting dropdown disabled with the active option from data (EXC-22)',
      (tester) async {
        final recorded = await pumpCatalog(
          tester,
          handler: routing(detail: exclusivePairBody),
        );
        await openDialog(tester);

        final dropdown = tester.widget<DropdownButton<String>>(
          inPreview(
            find.byKey(const Key('sheet-exclusive-spellcastingAbility')),
          ),
        );
        expect(dropdown.value, 'config.spellUsesIntelligence');
        expect(dropdown.onChanged, isNull);
        expect(
          inPreview(find.byType(Radio<String>)),
          findsNothing,
          reason: 'the spellcasting group is the shared dropdown, not a radio',
        );
        expect(
          inPreview(
            find.byKey(const Key('sheet-checkbox-config.spellUsesIntelligence')),
          ),
          findsNothing,
        );
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
      },
    );

    testWidgets(
      'Edge: an exclusive group with no active default shows None disabled in the preview (EXC-10, EXC-27)',
      (tester) async {
        final recorded = await pumpCatalog(
          tester,
          handler: routing(detail: exclusiveNoneBody),
        );
        await openDialog(tester);

        final dropdown = tester.widget<DropdownButton<String>>(
          inPreview(
            find.byKey(const Key('sheet-exclusive-spellcastingAbility')),
          ),
        );
        expect(dropdown.value, exclusiveNoneValue);
        expect(dropdown.onChanged, isNull);
        expect(
          dropdown.items!.map((item) => item.value),
          contains(exclusiveNoneValue),
        );
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
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

  group('scroll affordance (PRV-10)', () {
    testWidgets(
      'Happy path: many leaves scroll inside a bounded card with an always-visible scrollbar (PRV-10)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: manyLeavesBody(30)));
        await openDialog(tester);

        final card = inDialog(find.byKey(const Key('create-sheet-preview')));
        expect(
          tester.getSize(card).width,
          AppSizes.createSheetDialogWidth,
        );

        final scroll = inPreview(find.byType(SingleChildScrollView));
        expect(scroll, findsOneWidget);
        expect(
          tester.getSize(scroll).height,
          lessThanOrEqualTo(AppSizes.previewCardMaxHeight),
        );

        final bar = tester.widget<Scrollbar>(inPreview(find.byType(Scrollbar)));
        expect(bar.thumbVisibility, isTrue);

        final controller =
            tester.widget<SingleChildScrollView>(scroll).controller!;
        expect(controller.position.maxScrollExtent, greaterThan(0));

        await tester.drag(scroll, const Offset(0, -200));
        await tester.pumpAndSettle();
        expect(controller.position.pixels, greaterThan(0));

        expect(
          inDialog(find.byKey(const Key('create-sheet-name-input'))),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: below the 640px minimum the card is clamped to the window, stays below the name and scrolls (PRV-10)',
      (tester) async {
        await pumpCatalog(
          tester,
          handler: routing(detail: manyLeavesBody(30)),
          viewSize: const Size(600, 800),
        );
        await openDialog(tester);

        final card = inDialog(find.byKey(const Key('create-sheet-preview')));
        expect(card, findsOneWidget);
        final content = tester.getSize(
          inDialog(find.byKey(const Key('create-sheet-content'))),
        );
        expect(content.width, lessThan(AppSizes.createSheetDialogWidth));
        expect(tester.getSize(card).width, content.width);

        final cardTop = tester.getTopLeft(card).dy;
        final nameTop = tester
            .getTopLeft(find.byKey(const Key('create-sheet-name-input')))
            .dy;
        expect(cardTop, greaterThan(nameTop));
        expect(
          tester.getSize(inPreview(find.byType(SingleChildScrollView))).height,
          lessThanOrEqualTo(AppSizes.previewCardMaxHeight),
        );
        expect(
          tester
              .widget<Scrollbar>(inPreview(find.byType(Scrollbar)))
              .thumbVisibility,
          isTrue,
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
