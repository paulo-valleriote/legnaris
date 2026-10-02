import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/exclusive_control.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/app_theme.dart';

void main() {
  SheetField field(String path, {String label = ''}) => (
    path: path,
    schema: FieldSchema(
      type: 'number',
      label: label,
      isEditable: true,
      defaultValue: 0,
    ),
  );

  SheetExclusiveGroup groupOf(List<SheetField> options) =>
      (id: 'armorClassAbility', options: options);

  Future<void> pump(
    WidgetTester tester, {
    required SheetExclusiveGroup group,
    String? selectedPath,
    ValueChanged<String>? onSelect,
    double? width,
  }) async {
    final Widget control = SheetExclusiveControl(
      group: group,
      selectedPath: selectedPath,
      onSelect: onSelect,
    );
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: Center(
            child: width == null
                ? control
                : SizedBox(width: width, child: control),
          ),
        ),
      ),
    );
  }

  String labelOfLength(int length) => List.filled(length, 'x').join();

  group('control choice (EXC-09, EXC-10)', () {
    testWidgets(
      'Happy path: three short options render one segmented control with a segment per option',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.a', label: 'Alpha'),
            field('config.b', label: 'Bravo'),
            field('config.c', label: 'Charlie'),
          ]),
        );

        expect(find.byType(SegmentedButton<String>), findsOneWidget);
        expect(find.byType(Radio<String>), findsNothing);
        expect(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
          findsOneWidget,
        );
        for (final path in ['config.a', 'config.b', 'config.c']) {
          expect(
            find.byKey(Key('sheet-exclusive-option-$path')),
            findsOneWidget,
          );
        }
      },
    );

    testWidgets(
      'Edge: a label of exactly 30 characters stays segmented (limit is inclusive)',
      (tester) async {
        await pump(
          tester,
          group: groupOf([field('config.a', label: labelOfLength(30))]),
        );

        expect(find.byType(SegmentedButton<String>), findsOneWidget);
        expect(find.byType(Radio<String>), findsNothing);
      },
    );

    testWidgets(
      'Edge: more than three options render a radio group instead',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.a', label: 'Alpha'),
            field('config.b', label: 'Bravo'),
            field('config.c', label: 'Charlie'),
            field('config.d', label: 'Delta'),
          ]),
        );

        expect(find.byType(SegmentedButton<String>), findsNothing);
        expect(find.byType(Radio<String>), findsNWidgets(4));
        expect(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('sheet-exclusive-option-config.a')),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: a label longer than 30 characters renders a radio group instead',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.a', label: 'Alpha'),
            field('config.b', label: labelOfLength(31)),
          ]),
        );

        expect(find.byType(SegmentedButton<String>), findsNothing);
        expect(find.byType(Radio<String>), findsNWidgets(2));
      },
    );

    testWidgets(
      'Edge: a long radio label wraps in a narrow row without overflowing (EXC-10)',
      (tester) async {
        await pump(
          tester,
          width: 160,
          group: groupOf([
            field('config.a', label: 'Alpha'),
            field('config.b', label: 'Spellcasting uses constitution'),
            field('config.c', label: 'Charlie'),
            field('config.d', label: 'Delta'),
          ]),
        );

        expect(find.byType(Radio<String>), findsNWidgets(4));
        expect(find.text('Spellcasting uses constitution'), findsOneWidget);
        expect(tester.takeException(), isNull);
      },
    );
  });

  group('selection (EXC-09)', () {
    testWidgets('Happy path: selecting a segment reports its path', (
      tester,
    ) async {
      final selected = <String>[];
      await pump(
        tester,
        group: groupOf([
          field('config.acUsesDex', label: 'Dexterity'),
          field('config.acUsesCon', label: 'Constitution'),
        ]),
        selectedPath: 'config.acUsesDex',
        onSelect: selected.add,
      );

      await tester.tap(
        find.byKey(const Key('sheet-exclusive-option-config.acUsesCon')),
      );
      await tester.pump();

      expect(selected, ['config.acUsesCon']);
    });

    testWidgets('Happy path: selecting a radio reports its path', (
      tester,
    ) async {
      final selected = <String>[];
      await pump(
        tester,
        group: groupOf([
          field('config.a', label: 'Alpha'),
          field('config.b', label: 'Bravo'),
          field('config.c', label: 'Charlie'),
          field('config.d', label: 'Delta'),
        ]),
        selectedPath: 'config.a',
        onSelect: selected.add,
      );

      await tester.tap(
        find.byKey(const Key('sheet-exclusive-option-config.c')),
      );
      await tester.pump();

      expect(selected, ['config.c']);
    });

    testWidgets('Edge: no active option renders nothing selected', (
      tester,
    ) async {
      await pump(
        tester,
        group: groupOf([
          field('config.a', label: 'Alpha'),
          field('config.b', label: 'Bravo'),
        ]),
        onSelect: (_) {},
      );

      final segmented = tester.widget<SegmentedButton<String>>(
        find.byType(SegmentedButton<String>),
      );
      expect(segmented.selected, isEmpty);
    });
  });

  group('read-only (EXC-11)', () {
    testWidgets(
      'Happy path: read-only segmented is disabled and keeps the active option selected',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.acUsesDex', label: 'Dexterity'),
            field('config.acUsesCon', label: 'Constitution'),
          ]),
          selectedPath: 'config.acUsesDex',
        );

        final segmented = tester.widget<SegmentedButton<String>>(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
        );
        expect(segmented.onSelectionChanged, isNull);
        expect(segmented.selected, {'config.acUsesDex'});
      },
    );

    testWidgets(
      'Happy path: read-only radio is disabled and keeps the active option selected',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.a', label: 'Alpha'),
            field('config.b', label: 'Bravo'),
            field('config.c', label: 'Charlie'),
            field('config.d', label: 'Delta'),
          ]),
          selectedPath: 'config.b',
        );

        final radios = tester.widgetList<Radio<String>>(find.byType(Radio<String>));
        expect(radios.every((radio) => radio.enabled == false), isTrue);
        final group = tester.widget<RadioGroup<String>>(
          find.byType(RadioGroup<String>),
        );
        expect(group.groupValue, 'config.b');
      },
    );

    testWidgets('Edge: read-only with no active option renders none selected', (
      tester,
    ) async {
      await pump(
        tester,
        group: groupOf([
          field('config.a', label: 'Alpha'),
          field('config.b', label: 'Bravo'),
        ]),
      );

      final segmented = tester.widget<SegmentedButton<String>>(
        find.byType(SegmentedButton<String>),
      );
      expect(segmented.selected, isEmpty);
      expect(segmented.onSelectionChanged, isNull);
    });
  });

  group('exclusiveValueIsOn (numeric proxy contract)', () {
    test('Happy path: a non-zero num is on', () {
      expect(exclusiveValueIsOn(1), isTrue);
      expect(exclusiveValueIsOn(1.0), isTrue);
      expect(exclusiveValueIsOn(-0.5), isTrue);
    });

    test('Edge: zero is off', () {
      expect(exclusiveValueIsOn(0), isFalse);
      expect(exclusiveValueIsOn(0.0), isFalse);
    });

    test('Edge: a bool is off (the backend rejects non-numeric members)', () {
      expect(exclusiveValueIsOn(true), isFalse);
    });

    test('Edge: a string, null and a map are off', () {
      expect(exclusiveValueIsOn('1'), isFalse);
      expect(exclusiveValueIsOn(null), isFalse);
      expect(exclusiveValueIsOn(const <String, Object?>{}), isFalse);
    });
  });
}
