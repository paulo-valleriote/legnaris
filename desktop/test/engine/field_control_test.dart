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
        expect(find.byType(Checkbox), findsNothing);
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
      'Happy path: a config.* field renders a checkbox emitting 1 and 0 (RND-08, EXC-28)',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'config.acUsesDex',
          schema: schemaFor('number'),
          value: 1.0,
          onChanged: emitted.add,
        );

        final checkbox = find.byKey(const Key('sheet-checkbox-config.acUsesDex'));
        expect(checkbox, findsOneWidget);
        expect(find.byType(Checkbox), findsOneWidget);
        expect(tester.widget<Checkbox>(checkbox).value, isTrue);
        expect(find.byType(TextField), findsNothing);

        await tester.tap(checkbox);

        expect(emitted, [0]);
      },
    );

    testWidgets(
      'Edge: a config.* field holding 0 renders unchecked and emits 1 when toggled (RND-08, EXC-29)',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'config.stealthProfWeight',
          schema: schemaFor('number'),
          value: 0.0,
          onChanged: emitted.add,
        );

        final checkbox = find.byKey(
          const Key('sheet-checkbox-config.stealthProfWeight'),
        );
        expect(tester.widget<Checkbox>(checkbox).value, isFalse);

        await tester.tap(checkbox);

        expect(emitted, [1]);
      },
    );

    testWidgets(
      'Happy path: a bool field renders a checkbox emitting true and false (RND-09, EXC-29)',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'flags.visible',
          schema: schemaFor('bool'),
          value: true,
          onChanged: emitted.add,
        );

        final checkbox = find.byKey(const Key('sheet-checkbox-flags.visible'));
        expect(tester.widget<Checkbox>(checkbox).value, isTrue);

        await tester.tap(checkbox);

        expect(emitted, [false]);
      },
    );

    testWidgets(
      'Edge: tapping an already-on numeric proxy checkbox emits 0, never a bool (EXC-29)',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'config.acUsesCon',
          schema: schemaFor('number'),
          value: 1.0,
          onChanged: emitted.add,
        );

        await tester.tap(
          find.byKey(const Key('sheet-checkbox-config.acUsesCon')),
        );

        expect(emitted, [0]);
        expect(emitted, isNot(contains(false)));
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
        expect(find.byType(Checkbox), findsNothing);
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
        expect(find.byType(Checkbox), findsNothing);
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
        expect(tester.widget<Checkbox>(find.byType(Checkbox)).value, isFalse);
      },
    );

    testWidgets(
      'Edge: a read-only flag renders its checkbox disabled (EXC-30)',
      (tester) async {
        await pump(
          tester,
          path: 'config.acUsesDex',
          schema: schemaFor('number'),
          value: 1.0,
        );

        final checkbox = tester.widget<Checkbox>(find.byType(Checkbox));
        expect(checkbox.value, isTrue);
        expect(checkbox.onChanged, isNull);
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

  group('numeric input hardening (no garbage in, no stale display out)', () {
    testWidgets(
      'Edge: non-numeric text is rejected — the field keeps its stored value (RND-06)',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 16.0,
          onChanged: emitted.add,
        );
        final input = find.byKey(const Key('sheet-input-stats.dexterity'));

        await tester.enterText(input, 'abc');

        expect(
          tester.widget<TextField>(input).controller?.text,
          '16',
          reason: 'garbage must not replace the displayed value',
        );
        expect(emitted, isEmpty, reason: 'garbage must not reach the draft');
      },
    );

    testWidgets(
      'Edge: NaN, Infinity, hex and exponent forms never reach the draft (RND-06)',
      (tester) async {
        final emitted = <Object?>[];
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 16.0,
          onChanged: emitted.add,
        );
        final input = find.byKey(const Key('sheet-input-stats.dexterity'));

        for (final garbage in const ['NaN', 'Infinity', '0x10', '1e999']) {
          await tester.enterText(input, garbage);
          expect(
            tester.widget<TextField>(input).controller?.text,
            isNot(garbage),
            reason: '"$garbage" must not be displayed as a number',
          );
        }

        expect(emitted, isEmpty, reason: 'no garbage value was ever emitted');
        expect(
          emitted.whereType<num>().where((v) => v.isNaN || v.isInfinite),
          isEmpty,
        );
      },
    );

    testWidgets(
      'Edge: a huge run of digits never emits a non-finite value (RND-06)',
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
          '9' * 400,
        );

        expect(
          emitted.whereType<num>().where((v) => v.isInfinite || v.isNaN),
          isEmpty,
          reason: 'an overflowing digit run must not become Infinity',
        );
      },
    );

    testWidgets(
      'Edge: an external change equal to the last emitted value still resyncs the input',
      (tester) async {
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 16.0,
          onChanged: (_) {},
        );
        final input = find.byKey(const Key('sheet-input-stats.dexterity'));

        await tester.enterText(input, '42');
        expect(tester.widget<TextField>(input).controller?.text, '42');

        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 7.0,
          onChanged: (_) {},
        );
        expect(tester.widget<TextField>(input).controller?.text, '7');

        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 42.0,
          onChanged: (_) {},
        );
        expect(
          tester.widget<TextField>(input).controller?.text,
          '42',
          reason:
              'the echo marker must not go stale, or a later external change '
              'equal to it is silently skipped',
        );
      },
    );

    testWidgets(
      'Edge: an abandoned intermediate restores the stored value when focus leaves',
      (tester) async {
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 16.0,
          onChanged: (_) {},
        );
        final input = find.byKey(const Key('sheet-input-stats.dexterity'));

        await tester.enterText(input, '-');
        expect(tester.widget<TextField>(input).controller?.text, '-');

        FocusManager.instance.primaryFocus?.unfocus();
        await tester.pump();

        expect(
          tester.widget<TextField>(input).controller?.text,
          '16',
          reason:
              'at rest the display must show the stored value, not a '
              'half-typed intermediate',
        );
      },
    );

    testWidgets(
      'Edge: a huge integer-valued double is not clamped by toInt in the display',
      (tester) async {
        await pump(
          tester,
          path: 'stats.dexterity',
          schema: schemaFor('number'),
          value: 1e300,
        );

        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '1e+300',
          reason: 'toInt clamps beyond int64 and would display a different '
              'number than the stored one',
        );
      },
    );
  });
}
