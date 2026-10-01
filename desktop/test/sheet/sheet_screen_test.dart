import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/engine/field_control.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
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

        final toggles = tester.widgetList<Switch>(find.byType(Switch));
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
            'config.acUsesDex',
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
          find.byKey(const Key('sheet-toggle-config.acUsesDex')),
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
            afterSecond['config.acUsesDex'],
            afterFirst['config.acUsesDex'],
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
          find.byKey(const Key('sheet-toggle-config.acUsesDex')),
        );
        await tester.pump();

        expect(
          tester
              .widget<Switch>(
                find.byKey(const Key('sheet-toggle-config.acUsesDex')),
              )
              .value,
          isFalse,
        );
        final container = ProviderScope.containerOf(
          tester.element(find.byKey(const Key('sheet-title'))),
        );
        final state = container.read(sheetEditControllerProvider('fantasy_v1'));
        expect(valueAtPath(state.data, 'config.acUsesDex'), 0);
        expect(state.isPending, isTrue);

        final after = {
          for (final control in tester.widgetList<SheetFieldControl>(
            find.byType(SheetFieldControl),
          ))
            control.path: control,
        };
        for (final path in before.keys.where(
          (path) => path != 'config.acUsesDex',
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
}
