import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/engine/exclusive_control.dart';
import 'package:legnaris/engine/field_control.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:legnaris/sheet/sheet_refresh.dart';
import 'package:legnaris/sheet/sheet_screen.dart';
import 'package:legnaris/theme/app_theme.dart';

void main() {
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}},
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "Armor class uses dexterity", "is_editable": true, "default": 1, "exclusive_group": "armorClassAbility"}}, "acUsesCon": {"schema": {"type": "number", "label": "Armor class uses constitution", "is_editable": true, "default": 0, "exclusive_group": "armorClassAbility"}}, "spellUsesCharisma": {"schema": {"type": "number", "label": "Spellcasting uses charisma", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesConstitution": {"schema": {"type": "number", "label": "Spellcasting uses constitution", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesDexterity": {"schema": {"type": "number", "label": "Spellcasting uses dexterity", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesIntelligence": {"schema": {"type": "number", "label": "Spellcasting uses intelligence", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesStrength": {"schema": {"type": "number", "label": "Spellcasting uses strength", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "spellUsesWisdom": {"schema": {"type": "number", "label": "Spellcasting uses wisdom", "is_editable": true, "default": 0, "exclusive_group": "spellcastingAbility"}}, "stealthProfWeight": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 0}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "formula": "x + 1", "is_editable": false, "default": 0}}}}
}}}
''';
  const emptyFieldsBody =
      '{"data": {"id": "blank", "name": "Blank", "default": false, "fields": {}}}';
  const emptyNameBody =
      '{"data": {"id": "blank", "name": "", "default": false, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';
  const freshBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 16}}}},
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "Armor class uses dexterity", "is_editable": true, "default": 1, "exclusive_group": "armorClassAbility"}}, "acUsesCon": {"schema": {"type": "number", "label": "Armor class uses constitution", "is_editable": true, "default": 0, "exclusive_group": "armorClassAbility"}}, "stealthProfWeight": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 0}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "formula": "x + 1", "is_editable": false, "default": 0}}}}
}}}
''';
  // Same template with only one member tagged: the exclusive group degrades to
  // a single-member group, so a refresh must drop the exclusive row (M1).
  const singleMemberBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}},
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "Armor class uses dexterity", "is_editable": true, "default": 1, "exclusive_group": "armorClassAbility"}}, "acUsesCon": {"schema": {"type": "number", "label": "Armor class uses constitution", "is_editable": true, "default": 0}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "formula": "x + 1", "is_editable": false, "default": 0}}}}
}}}
''';

  /// A full-size template: 20 groups × 10 fields = 200 editable controls,
  /// the upper bound of the GRN-10 render budget.
  String bigTemplateBody() {
    final fields = <String, Object?>{};
    for (var group = 0; group < 20; group++) {
      final children = <String, Object?>{};
      for (var field = 0; field < 10; field++) {
        children['field$group$field'] = {
          'schema': {
            'type': 'number',
            'label': '',
            'is_editable': true,
            'default': 0,
          },
        };
      }
      fields['group$group'] = {'children': children};
    }
    return jsonEncode({
      'data': {
        'id': 'fantasy_v1',
        'name': 'Big Template',
        'default': false,
        'fields': fields,
      },
    });
  }

  Future<List<http.Request>> pumpScreen(
    WidgetTester tester, {
    required Future<http.Response> Function(http.Request request) handler,
    bool stacked = false,
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
          home: stacked
              ? const Scaffold(
                  body: Text('sheet-landing', key: Key('sheet-landing')),
                )
              : const Scaffold(body: SheetScreen(templateId: 'fantasy_v1')),
        ),
      ),
    );
    await tester.pump();
    if (stacked) {
      // A real back stack, so the leave guard has a route to pop back to.
      final navigator = tester.state<NavigatorState>(find.byType(Navigator));
      navigator.push(
        MaterialPageRoute<void>(
          builder: (_) =>
              const Scaffold(body: SheetScreen(templateId: 'fantasy_v1')),
        ),
      );
      await tester.pumpAndSettle();
    }
    return recorded;
  }

  /// The sheet screen's side of the F5 dispatch (T6 bumps the same tick
  /// through the shortcut registry), followed by a settle of the flow it
  /// triggers.
  Future<void> pressF5(WidgetTester tester) async {
    final container = ProviderScope.containerOf(
      tester.element(find.byKey(const Key('sheet-title'))),
    );
    container.read(sheetRefreshTickProvider.notifier).bump();
    await tester.pumpAndSettle();
  }

  /// Opens an exclusive-group dropdown and taps one entry. The entry key also
  /// exists in the closed button's `IndexedStack`, so `.last` picks the menu.
  Future<void> selectExclusive(
    WidgetTester tester,
    String groupId,
    String optionPath,
  ) async {
    await tester.tap(find.byKey(Key('sheet-exclusive-$groupId')));
    await tester.pumpAndSettle();
    await tester.tap(
      find.byKey(Key('sheet-exclusive-option-$optionPath')).last,
    );
    await tester.pumpAndSettle();
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
        final exclusive = tester.widget<DropdownButton<String>>(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
        );
        expect(exclusive.value, 'config.acUsesDex');
        expect(exclusive.onChanged, isNotNull);
        expect(
          tester
              .widget<DropdownButton<String>>(
                find.byKey(const Key('sheet-exclusive-spellcastingAbility')),
              )
              .value,
          exclusiveNoneValue,
          reason: 'a group with no active default shows None',
        );
        expect(
          find.byKey(const Key('sheet-checkbox-config.acUsesDex')),
          findsNothing,
        );
        expect(
          find.byKey(const Key('sheet-checkbox-config.acUsesCon')),
          findsNothing,
        );
        expect(
          tester
              .widget<Checkbox>(
                find.byKey(const Key('sheet-checkbox-config.stealthProfWeight')),
              )
              .value,
          isFalse,
        );
        expect(find.text('Calculated'), findsOneWidget);
        expect(recorded.where((request) => request.method != 'GET'), isEmpty);
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

  group('edges (RND-16, RND-19, RND-20)', () {
    testWidgets('Edge: an empty field tree renders the single localized line', (
      tester,
    ) async {
      await pumpScreen(
        tester,
        handler: (_) async => http.Response(emptyFieldsBody, 200),
      );
      await tester.pumpAndSettle();

      expect(find.text('This template has no fields.'), findsOneWidget);
    });

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
      'Edge: editable controls accept in-memory edits and no mutation is ever issued (GRN-03, GRN-05)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        final inputs = tester.widgetList<TextField>(find.byType(TextField));
        expect(inputs, isNotEmpty);
        expect(inputs.every((field) => field.enabled == true), isTrue);

        final toggles = tester.widgetList<Checkbox>(find.byType(Checkbox));
        expect(toggles, isNotEmpty);
        expect(toggles.every((toggle) => toggle.onChanged != null), isTrue);

        expect(
          find.byKey(const Key('sheet-value-derived.armorClass')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('sheet-input-derived.armorClass')),
          findsNothing,
        );
        expect(find.text('Calculated'), findsOneWidget);

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '42',
        );
        await tester.pump();

        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '42',
        );
        expect(recorded, hasLength(1));
        expect(recorded.single.method, 'GET');
      },
    );
  });

  group('in-memory editing and granular rebuild (LGN-102)', () {
    testWidgets(
      'Happy path: an edit stores the value in the edit state and shows it (GRN-02)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();

        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        final state = container.read(sheetEditControllerProvider('fantasy_v1'));
        expect(valueAtPath(state.data, 'stats.dexterity'), 16);
        expect(state.isPending, isTrue);
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '16',
        );
        expect(recorded, hasLength(1));
        expect(recorded.single.method, 'GET');
      },
    );

    testWidgets(
      'Happy path: editing one field rebuilds only that field control, labels included untouched (GRN-07, GRN-08)',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        Map<String, SheetFieldControl> snapshot() => {
          for (final control in tester.widgetList<SheetFieldControl>(
            find.byType(SheetFieldControl),
          ))
            control.path: control,
        };

        final before = snapshot();
        expect(
          before.keys,
          containsAll([
            'stats.dexterity',
            'config.stealthProfWeight',
            'derived.armorClass',
          ]),
        );
        final label = tester.widget<Text>(find.text('Dexterity'));
        final sectionHeader = tester.widget(
          find.byKey(const Key('sheet-section-stats')),
        );

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();

        final after = snapshot();
        expect(
          identical(after['stats.dexterity'], before['stats.dexterity']),
          isFalse,
        );
        for (final path in before.keys.where(
          (path) => path != 'stats.dexterity',
        )) {
          expect(
            identical(after[path], before[path]),
            isTrue,
            reason: '$path must not rebuild when another field changes',
          );
        }
        expect(
          identical(tester.widget<Text>(find.text('Dexterity')), label),
          isTrue,
          reason: 'row labels must not rebuild when a field changes',
        );
        expect(
          identical(
            tester.widget(find.byKey(const Key('sheet-section-stats'))),
            sectionHeader,
          ),
          isTrue,
          reason: 'section headers must not rebuild when a field changes',
        );
      },
    );

    testWidgets(
      'Happy path: a second edit rebuilds only the second field (GRN-07, GRN-08)',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        Map<String, SheetFieldControl> snapshot() => {
          for (final control in tester.widgetList<SheetFieldControl>(
            find.byType(SheetFieldControl),
          ))
            control.path: control,
        };

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();
        final afterFirst = snapshot();

        await tester.tap(
          find.byKey(const Key('sheet-checkbox-config.stealthProfWeight')),
        );
        await tester.pump();
        final afterSecond = snapshot();

        expect(
          identical(
            afterSecond['stats.dexterity'],
            afterFirst['stats.dexterity'],
          ),
          isTrue,
          reason: 'the first field must not rebuild on the second edit',
        );
        expect(
          identical(
            afterSecond['config.stealthProfWeight'],
            afterFirst['config.stealthProfWeight'],
          ),
          isFalse,
        );
        expect(
          identical(
            afterSecond['derived.armorClass'],
            afterFirst['derived.armorClass'],
          ),
          isTrue,
        );
      },
    );

    testWidgets(
      'Edge: a toggle flip displays the new state and stores it without any other control rebuilding (GRN-02, GRN-08)',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();
        final before = {
          for (final control in tester.widgetList<SheetFieldControl>(
            find.byType(SheetFieldControl),
          ))
            control.path: control,
        };

        await tester.tap(
          find.byKey(const Key('sheet-checkbox-config.stealthProfWeight')),
        );
        await tester.pump();

        expect(
          tester
              .widget<Checkbox>(
                find.byKey(const Key('sheet-checkbox-config.stealthProfWeight')),
              )
              .value,
          isTrue,
        );
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        final state = container.read(sheetEditControllerProvider('fantasy_v1'));
        expect(valueAtPath(state.data, 'config.stealthProfWeight'), 1);
        expect(state.isPending, isTrue);

        final after = {
          for (final control in tester.widgetList<SheetFieldControl>(
            find.byType(SheetFieldControl),
          ))
            control.path: control,
        };
        for (final path in before.keys.where(
          (path) => path != 'config.stealthProfWeight',
        )) {
          expect(
            identical(after[path], before[path]),
            isTrue,
            reason: '$path must not rebuild when the toggle flips',
          );
        }
      },
    );
  });

  group('local-edits marker and leave guard (LGN-102)', () {
    testWidgets(
      'Happy path: the first differing edit shows the local-edits marker (GRN-11)',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('sheet-local-edits')), findsNothing);

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();

        expect(find.text('Local edits'), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: reverting the field to its default hides the marker again (GRN-12)',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();
        expect(find.text('Local edits'), findsOneWidget);

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '10',
        );
        await tester.pump();

        expect(find.byKey(const Key('sheet-local-edits')), findsNothing);
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        expect(
          container.read(sheetEditControllerProvider('fantasy_v1')).isPending,
          isFalse,
        );
      },
    );

    testWidgets(
      'Happy path: back with pending edits opens the discard confirmation (GRN-13)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
          stacked: true,
        );

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();

        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsOneWidget);
        expect(
          find.text(
            'Your local edits on this sheet were never saved and will be discarded.',
          ),
          findsOneWidget,
        );
        expect(find.byType(SheetScreen), findsOneWidget);
        expect(recorded, hasLength(1));
      },
    );

    testWidgets(
      'Happy path: confirming the leave dialog discards the state and navigates back (GRN-14)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
          stacked: true,
        );

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();
        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();

        await tester.tap(find.widgetWithText(TextButton, 'Discard and leave'));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byKey(const Key('sheet-landing')), findsOneWidget);
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-landing'))),
        );
        expect(
          container.exists(sheetEditControllerProvider('fantasy_v1')),
          isFalse,
          reason: 'popping must discard the autoDispose edit state',
        );
        expect(recorded, hasLength(1));
      },
    );

    testWidgets(
      'Edge: cancelling the leave dialog keeps the screen and the edit state (GRN-15)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
          stacked: true,
        );

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();
        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();

        await tester.tap(find.widgetWithText(TextButton, 'Keep editing'));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(SheetScreen), findsOneWidget);
        expect(find.text('Local edits'), findsOneWidget);
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        expect(
          container.read(sheetEditControllerProvider('fantasy_v1')).isPending,
          isTrue,
        );
        expect(recorded, hasLength(1));
      },
    );

    testWidgets(
      'Happy path: back with no pending edits navigates without a dialog (GRN-16)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
          stacked: true,
        );

        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byKey(const Key('sheet-landing')), findsOneWidget);
        expect(recorded, hasLength(1));
      },
    );

    testWidgets(
      'Edge: the open dialog blocks the back action behind its modal barrier (GRN-13)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
          stacked: true,
        );

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();
        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();

        expect(find.byType(SheetScreen), findsOneWidget);
        expect(find.byKey(const Key('sheet-landing')), findsNothing);
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        expect(
          container.read(sheetEditControllerProvider('fantasy_v1')).isPending,
          isTrue,
        );
        expect(recorded, hasLength(1));
      },
    );
  });

  group('active sheet registration (GRN-17)', () {
    testWidgets(
      'Happy path: the screen registers itself as the active sheet while mounted',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        expect(container.exists(activeSheetIdProvider), isTrue);
        expect(container.read(activeSheetIdProvider), 'fantasy_v1');
      },
    );

    testWidgets(
      'Edge: unmounting the screen clears the active-sheet registration',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
          stacked: true,
        );
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        expect(container.read(activeSheetIdProvider), 'fantasy_v1');

        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();
        // The autoDispose task runs on a scheduled vsync after the unmount.
        await tester.pump();

        expect(container.exists(activeSheetIdProvider), isFalse);
      },
    );
  });

  group('refresh behind confirmation (GRN-17 … GRN-19)', () {
    testWidgets(
      'Happy path: F5 with no pending edits issues one detail GET and reseeds from the fresh schema (GRN-17)',
      (tester) async {
        var detailGets = 0;
        final recorded = await pumpScreen(
          tester,
          handler: (_) async =>
              http.Response(detailGets++ == 0 ? detailBody : freshBody, 200),
        );
        await tester.pumpAndSettle();
        expect(detailGets, 1);

        await pressF5(tester);

        expect(detailGets, 2);
        expect(
          recorded.where(
            (request) => request.url.path == '/templates/fantasy_v1',
          ),
          hasLength(2),
        );
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '16',
          reason: 'the form must show the fresh schema default',
        );
        expect(find.byKey(const Key('sheet-local-edits')), findsNothing);
      },
    );

    testWidgets(
      'Happy path: F5 with pending edits opens the confirmation before any request (GRN-18)',
      (tester) async {
        var detailGets = 0;
        final recorded = await pumpScreen(
          tester,
          handler: (_) async {
            detailGets++;
            return http.Response(detailBody, 200);
          },
        );
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();

        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        container.read(sheetRefreshTickProvider.notifier).bump();
        container.read(sheetRefreshTickProvider.notifier).bump();
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsOneWidget);
        expect(
          find.text(
            'Your local edits will be discarded and the latest template will be loaded.',
          ),
          findsOneWidget,
        );
        expect(
          detailGets,
          1,
          reason: 'no new detail request until the user confirms',
        );
        expect(recorded, hasLength(1));
        expect(
          container.read(sheetEditControllerProvider('fantasy_v1')).isPending,
          isTrue,
        );
      },
    );

    testWidgets(
      'Edge: cancelling the refresh dialog issues zero requests (GRN-19)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();

        await pressF5(tester);
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tap(find.widgetWithText(TextButton, 'Keep editing'));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(SheetScreen), findsOneWidget);
        expect(find.text('Local edits'), findsOneWidget);
        expect(
          recorded,
          hasLength(1),
          reason: 'cancel must issue no template detail request',
        );
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        expect(
          container.read(sheetEditControllerProvider('fantasy_v1')).isPending,
          isTrue,
        );
      },
    );

    testWidgets(
      'Happy path: confirming the refresh discards the edits and reseeds from defaults',
      (tester) async {
        var detailGets = 0;
        final recorded = await pumpScreen(
          tester,
          handler: (_) async {
            detailGets++;
            return http.Response(detailBody, 200);
          },
        );
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();

        await pressF5(tester);
        expect(find.byType(AlertDialog), findsOneWidget);
        expect(detailGets, 1);

        await tester.tap(find.widgetWithText(TextButton, 'Discard and reload'));
        await tester.pumpAndSettle();

        expect(detailGets, 2);
        expect(
          recorded.where(
            (request) => request.url.path == '/templates/fantasy_v1',
          ),
          hasLength(2),
        );
        expect(find.byKey(const Key('sheet-local-edits')), findsNothing);
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '10',
          reason: 'the reseeded form shows the schema default again',
        );
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        expect(
          container.read(sheetEditControllerProvider('fantasy_v1')).isPending,
          isFalse,
        );
      },
    );

    testWidgets(
      'Edge: a confirmed refresh that fails shows the mapped error and Retry, and the state reseeds after a successful retry',
      (tester) async {
        var call = 0;
        final recorded = await pumpScreen(
          tester,
          handler: (_) async {
            final failing = call++ == 1;
            return http.Response(
              failing ? serverErrorBody : detailBody,
              failing ? 500 : 200,
            );
          },
        );
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '16',
        );
        await tester.pump();

        await pressF5(tester);
        await tester.tap(find.widgetWithText(TextButton, 'Discard and reload'));
        await tester.pumpAndSettle();

        expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget);
        expect(find.widgetWithText(OutlinedButton, 'Retry'), findsOneWidget);
        expect(find.byType(SheetFieldControl), findsNothing);
        expect(find.byKey(const Key('sheet-local-edits')), findsNothing);
        expect(recorded, hasLength(2));
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        expect(
          container.read(sheetEditControllerProvider('fantasy_v1')).isPending,
          isFalse,
          reason: 'a failed confirmed refresh keeps the edits discarded',
        );

        await tester.tap(find.widgetWithText(OutlinedButton, 'Retry'));
        await tester.pumpAndSettle();

        expect(recorded, hasLength(3));
        expect(find.byKey(const Key('sheet-section-stats')), findsOneWidget);
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '10',
        );
        expect(find.byKey(const Key('sheet-local-edits')), findsNothing);
      },
    );
  });

  group('render budget (GRN-10)', () {
    testWidgets(
      'Happy path: the response for a 200-field template mounts all 200 controls within the render budget (GRN-10)',
      (tester) async {
        final gate = Completer<http.Response>();
        await pumpScreen(tester, handler: (_) => gate.future);
        expect(find.byType(SheetFieldControl), findsNothing);

        final stopwatch = Stopwatch()..start();
        gate.complete(http.Response(bigTemplateBody(), 200));
        await tester.pump();
        stopwatch.stop();

        expect(find.byType(SheetFieldControl), findsNWidgets(200));
        expect(
          stopwatch.elapsedMilliseconds,
          // Debug-mode regression budget: `flutter test` measures ~4.4 ms
          // per control (200 ≈ 1000 ms for this file alone; 1800-2700 ms
          // when the full suite runs files in parallel on a loaded
          // machine). The <100 ms AC is enforced in profile mode per the
          // spec's amended gating decision.
          lessThan(4000),
          reason: 'GRN-10: 200 field controls must mount within the budget',
        );
      },
    );
  });

  group('exclusive groups (EXC-13, EXC-15, EXC-16)', () {
    testWidgets(
      'Happy path: the edit screen renders one dropdown control for the pair, default option selected, and no per-field checkboxes (EXC-15)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        final dropdown = tester.widget<DropdownButton<String>>(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
        );
        expect(dropdown.value, 'config.acUsesDex');
        expect(dropdown.onChanged, isNotNull);
        expect(
          find.byKey(const Key('sheet-checkbox-config.acUsesDex')),
          findsNothing,
        );
        expect(
          find.byKey(const Key('sheet-checkbox-config.acUsesCon')),
          findsNothing,
        );
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
      },
    );

    testWidgets(
      'Happy path: selecting the constitution option flips the stored pair to (0,1) in one emission and issues no request (EXC-13, EXC-16)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        var emissions = 0;
        final subscription = container.listen(
          sheetEditControllerProvider('fantasy_v1'),
          (_, _) => emissions++,
        );
        addTearDown(subscription.close);

        await selectExclusive(
          tester,
          'armorClassAbility',
          'config.acUsesCon',
        );

        final state = container.read(
          sheetEditControllerProvider('fantasy_v1'),
        );
        expect(valueAtPath(state.data, 'config.acUsesCon'), 1);
        expect(valueAtPath(state.data, 'config.acUsesDex'), 0);
        expect(
          state.editedPaths,
          containsAll(['config.acUsesCon', 'config.acUsesDex']),
        );
        expect(emissions, 1, reason: 'the group must change in one emission');
        expect(
          tester
              .widget<DropdownButton<String>>(
                find.byKey(const Key('sheet-exclusive-armorClassAbility')),
              )
              .value,
          'config.acUsesCon',
        );
        expect(recorded, hasLength(1));
        expect(recorded.single.method, 'GET');
      },
    );

    testWidgets(
      'Edge: re-selecting the active option leaves the state instance identical (EXC-14)',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        final before = container.read(
          sheetEditControllerProvider('fantasy_v1'),
        );

        await selectExclusive(
          tester,
          'armorClassAbility',
          'config.acUsesDex',
        );

        expect(
          identical(
            container.read(sheetEditControllerProvider('fantasy_v1')),
            before,
          ),
          isTrue,
        );
      },
    );

    testWidgets(
      'Edge: a refresh that changes the group composition rebuilds without error (M1)',
      (tester) async {
        var request = 0;
        await pumpScreen(
          tester,
          handler: (_) async {
            request += 1;
            return http.Response(
              request == 1 ? detailBody : singleMemberBody,
              200,
            );
          },
        );
        await tester.pumpAndSettle();

        expect(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
          findsOneWidget,
        );

        await pressF5(tester);

        expect(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
          findsNothing,
        );
        expect(
          find.byKey(const Key('sheet-checkbox-config.acUsesDex')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('sheet-checkbox-config.acUsesCon')),
          findsOneWidget,
        );
        expect(tester.takeException(), isNull);
      },
    );
  });

  group('exclusive spellcasting group (EXC-20, EXC-21, EXC-22)', () {
    const abilities = [
      'Charisma',
      'Constitution',
      'Dexterity',
      'Intelligence',
      'Strength',
      'Wisdom',
    ];

    testWidgets(
      'Happy path: the six-option spellcasting group is the same single dropdown as the pair, with a None entry and one item per ability (EXC-20, EXC-27)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        final spellcasting = tester.widget<DropdownButton<String>>(
          find.byKey(const Key('sheet-exclusive-spellcastingAbility')),
        );
        final armorClass = tester.widget<DropdownButton<String>>(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
        );
        expect(
          find.byType(DropdownButton<String>),
          findsNWidgets(2),
          reason: 'both groups use the one dropdown control',
        );
        expect(
          find.byType(Radio<String>),
          findsNothing,
          reason: 'the spellcasting group is the shared dropdown, not a radio',
        );
        expect(spellcasting.value, exclusiveNoneValue);
        expect(
          spellcasting.items!.map((item) => item.value),
          [exclusiveNoneValue, for (final ability in abilities) 'config.spellUses$ability'],
        );
        expect(
          armorClass.items!.map((item) => item.value),
          isNot(contains(exclusiveNoneValue)),
          reason: 'the defaulted pair offers no None',
        );
        for (final ability in abilities) {
          expect(
            find.byKey(Key('sheet-checkbox-config.spellUses$ability')),
            findsNothing,
          );
        }
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
      },
    );

    testWidgets(
      'Edge: with all six abilities off the spellcasting dropdown shows None and stays selectable (EXC-20)',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();

        final dropdown = tester.widget<DropdownButton<String>>(
          find.byKey(const Key('sheet-exclusive-spellcastingAbility')),
        );
        expect(dropdown.value, exclusiveNoneValue);
        expect(dropdown.onChanged, isNotNull);
        expect(
          dropdown.items!.map((item) => item.value),
          contains(exclusiveNoneValue),
        );
      },
    );

    testWidgets(
      'Happy path: selecting a spellcasting ability sets it to 1, zeroes the other five in one emission and issues no request (EXC-21)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        var emissions = 0;
        final subscription = container.listen(
          sheetEditControllerProvider('fantasy_v1'),
          (_, _) => emissions++,
        );
        addTearDown(subscription.close);
        final before = {
          for (final control in tester.widgetList<SheetFieldControl>(
            find.byType(SheetFieldControl),
          ))
            control.path: control,
        };

        await selectExclusive(
          tester,
          'spellcastingAbility',
          'config.spellUsesConstitution',
        );

        final state = container.read(
          sheetEditControllerProvider('fantasy_v1'),
        );
        expect(valueAtPath(state.data, 'config.spellUsesConstitution'), 1);
        for (final ability in abilities.where((a) => a != 'Constitution')) {
          expect(
            valueAtPath(state.data, 'config.spellUses$ability'),
            0,
            reason: 'every sibling ability must be zeroed by the selection',
          );
        }
        expect(emissions, 1, reason: 'the group must change in one emission');
        expect(state.editedPaths, contains('config.spellUsesConstitution'));
        expect(state.editedPaths, contains('config.spellUsesCharisma'));
        expect(recorded, hasLength(1));
        expect(recorded.single.method, 'GET');

        expect(
          tester
              .widget<DropdownButton<String>>(
                find.byKey(const Key('sheet-exclusive-spellcastingAbility')),
              )
              .value,
          'config.spellUsesConstitution',
        );

        final after = {
          for (final control in tester.widgetList<SheetFieldControl>(
            find.byType(SheetFieldControl),
          ))
            control.path: control,
        };
        for (final path in before.keys) {
          expect(
            identical(after[path], before[path]),
            isTrue,
            reason: '$path must not rebuild when the exclusive group changes',
          );
        }
      },
    );

    testWidgets(
      'Happy path: selecting None clears the whole spellcasting group in one emission with GET-only requests (EXC-27)',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (_) async => http.Response(detailBody, 200),
        );
        await tester.pumpAndSettle();
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        await selectExclusive(
          tester,
          'spellcastingAbility',
          'config.spellUsesConstitution',
        );
        expect(
          valueAtPath(
            container.read(sheetEditControllerProvider('fantasy_v1')).data,
            'config.spellUsesConstitution',
          ),
          1,
        );
        var emissions = 0;
        final subscription = container.listen(
          sheetEditControllerProvider('fantasy_v1'),
          (_, _) => emissions++,
        );
        addTearDown(subscription.close);

        await selectExclusive(tester, 'spellcastingAbility', 'none');

        final state = container.read(
          sheetEditControllerProvider('fantasy_v1'),
        );
        for (final ability in abilities) {
          expect(
            valueAtPath(state.data, 'config.spellUses$ability'),
            0,
            reason: 'None must clear every member of the group',
          );
        }
        expect(emissions, 1, reason: 'the group must clear in one emission');
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
        expect(
          tester
              .widget<DropdownButton<String>>(
                find.byKey(const Key('sheet-exclusive-spellcastingAbility')),
              )
              .value,
          exclusiveNoneValue,
        );
      },
    );
  });
}
