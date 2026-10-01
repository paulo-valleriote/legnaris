import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/field_control.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/app_theme.dart';

void main() {
  FieldSchema schemaFor(
    String type, {
    bool editable = true,
    String label = '',
    String? formula,
  }) => FieldSchema(
    type: type,
    label: label,
    formula: formula,
    isEditable: editable,
    defaultValue: 0,
  );

  Future<void> pump(
    WidgetTester tester, {
    required String path,
    required FieldSchema schema,
    required Object? value,
    ValueChanged<Object?>? onChanged,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: Center(
            child: SheetFieldControl(
              path: path,
              schema: schema,
              value: value,
              onChanged: onChanged,
            ),
          ),
        ),
      ),
    );
  }

  group('editable controls (RND-06 … RND-09, RND-12)', () {
    testWidgets(
      'Happy path: a number renders a numeric input holding its value (RND-06)',
      (tester) async {
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 16.0,
        );

        final input = find.byKey(const Key('sheet-input-stats.dexterity'));
        expect(input, findsOneWidget);
        expect(tester.widget<TextField>(input).controller?.text, '16');
        expect(find.byType(Switch), findsNothing);
      },
    );

    testWidgets(
      'Happy path: editing a number reports the parsed value (RND-12)',
      (tester) async {
        Object? emitted;
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 16.0,
          onChanged: (value) => emitted = value,
        );

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '42',
        );

        expect(emitted, 42);
      },
    );

    testWidgets(
      'Happy path: text renders a text input and reports the text (RND-07, RND-12)',
      (tester) async {
        Object? emitted;
        await pump(
          tester,
          path: 'meta.characterName',
          schema: schemaFor('text'),
          value: 'Aria',
          onChanged: (value) => emitted = value,
        );

        final input = find.byKey(const Key('sheet-input-meta.characterName'));
        expect(tester.widget<TextField>(input).controller?.text, 'Aria');

        await tester.enterText(input, 'Borin');

        expect(emitted, 'Borin');
      },
    );

    testWidgets(
      'Happy path: a config.* field renders a toggle emitting 1 and 0 (RND-08)',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'config.acUsesDex',
          schema: schemaFor('number'),
          value: 1.0,
          onChanged: emitted.add,
        );

        final toggle = find.byType(Switch);
        expect(toggle, findsOneWidget);
        expect(tester.widget<Switch>(toggle).value, isTrue);
        expect(find.byType(TextField), findsNothing);

        await tester.tap(toggle);

        expect(emitted, [0]);
      },
    );

    testWidgets(
      'Edge: a config.* field holding 0 renders off and emits 1 when toggled (RND-08)',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'config.stealthProfWeight',
          schema: schemaFor('number'),
          value: 0.0,
          onChanged: emitted.add,
        );

        final toggle = find.byType(Switch);
        expect(tester.widget<Switch>(toggle).value, isFalse);

        await tester.tap(toggle);

        expect(emitted, [1]);
      },
    );

    testWidgets(
      'Happy path: a bool field renders a toggle emitting true and false (RND-09)',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'flags.visible',
          schema: schemaFor('bool'),
          value: true,
          onChanged: emitted.add,
        );

        final toggle = find.byType(Switch);
        expect(tester.widget<Switch>(toggle).value, isTrue);

        await tester.tap(toggle);

        expect(emitted, [false]);
      },
    );
  });

  group('read-only controls (RND-10, RND-11, RND-13)', () {
    testWidgets(
      'Happy path: a non-editable field is read-only with a Calculated highlight (RND-10)',
      (tester) async {
        await pump(
          tester,
          path: 'derived.armorClass',
          schema: schemaFor('number', editable: false),
          value: 13.0,
        );

        expect(find.byType(TextField), findsNothing);
        expect(find.byType(Switch), findsNothing);
        expect(
          find.byKey(const Key('sheet-value-derived.armorClass')),
          findsOneWidget,
        );
        expect(find.text('13'), findsOneWidget);
        expect(find.text('Calculated'), findsOneWidget);
      },
    );

    testWidgets(
      'Happy path: a formula is exposed as the derived indicator (RND-11)',
      (tester) async {
        await pump(
          tester,
          path: 'modifiers.dexterityMod',
          schema: schemaFor(
            'number',
            editable: false,
            formula: 'floor((stats_dexterity - 10) / 2)',
          ),
          value: 3.0,
        );

        expect(find.byTooltip('floor((stats_dexterity - 10) / 2)'), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: an unknown type falls back to a read-only value, not a highlight or a crash (RND-13)',
      (tester) async {
        await pump(
          tester,
          path: 'meta.colour',
          schema: schemaFor('colour'),
          value: 'red',
        );

        expect(find.byType(TextField), findsNothing);
        expect(find.byType(Switch), findsNothing);
        expect(find.text('red'), findsOneWidget);
        expect(find.text('Calculated'), findsNothing);
      },
    );

    testWidgets(
      'Edge: a missing value renders empty and off without throwing',
      (tester) async {
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: null,
        );
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '',
        );

        await pump(
          tester,
          path: 'config.acUsesDex',
          schema: schemaFor('number'),
          value: null,
        );
        expect(tester.widget<Switch>(find.byType(Switch)).value, isFalse);
      },
    );
  });

  group('controller sync (RND-06, RND-12)', () {
    testWidgets(
      'Edge: clearing a numeric input reports null',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 16.0,
          onChanged: emitted.add,
        );

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '',
        );

        expect(emitted, [null]);
      },
    );

    testWidgets(
      'Edge: an external value change resyncs the input text',
      (tester) async {
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 1.0,
        );
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '1',
        );

        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 5.0,
        );

        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '5',
        );
      },
    );
  });
}
