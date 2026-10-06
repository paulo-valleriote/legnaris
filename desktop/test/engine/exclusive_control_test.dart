import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/exclusive_control.dart';
import 'package:legnaris/engine/field_grouping.dart';
import 'package:legnaris/engine/sheet_mark.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/app_theme.dart';

void main() {
  SheetField field(
    String path, {
    String label = '',
    String? optionLabel,
    double defaultValue = 0,
  }) => (
    path: path,
    schema: FieldSchema(
      type: 'number',
      label: label,
      isEditable: true,
      defaultValue: defaultValue,
      optionLabel: optionLabel,
    ),
  );

  SheetExclusiveGroup groupOf(List<SheetField> options) =>
      (id: 'armorClassAbility', options: options);

  Future<void> pump(
    WidgetTester tester, {
    required SheetExclusiveGroup group,
    String? selectedPath,
    SheetMark? mark,
    ValueChanged<String>? onSelect,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: Center(
            child: SheetExclusiveControl(
              group: group,
              selectedPath: selectedPath,
              mark: mark,
              onSelect: onSelect,
            ),
          ),
        ),
      ),
    );
  }

  /// Opens the dropdown and taps one menu entry — the entry key also exists in
  /// the button's `IndexedStack`, so `.last` picks the overlay menu item.
  Future<void> selectOption(WidgetTester tester, String optionKey) async {
    await tester.tap(
      find.byKey(const Key('sheet-exclusive-armorClassAbility')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(Key(optionKey)).last);
    await tester.pumpAndSettle();
  }

  DropdownButton<String> dropdownOf(WidgetTester tester) =>
      tester.widget<DropdownButton<String>>(
        find.byKey(const Key('sheet-exclusive-armorClassAbility')),
      );

  /// The declared menu entries — inspecting `items` reaches every option even
  /// while the menu is closed (only the selected entry is built then).
  Map<String?, String?> labelsOf(DropdownButton<String> dropdown) => {
    for (final item in dropdown.items!) item.value: (item.child as Text).data,
  };

  group('unified dropdown (EXC-09, EXC-10, EXC-27)', () {
    testWidgets(
      'Happy path: a group with an active default renders one dropdown showing it and no None',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field(
              'config.acUsesDex',
              label: 'Armor class uses dexterity',
              optionLabel: 'Dexterity',
              defaultValue: 1,
            ),
            field(
              'config.acUsesCon',
              label: 'Armor class uses constitution',
              optionLabel: 'Constitution',
            ),
          ]),
          selectedPath: 'config.acUsesDex',
          onSelect: (_) {},
        );

        expect(find.byType(SegmentedButton<String>), findsNothing);
        expect(find.byType(Radio<String>), findsNothing);
        final dropdown = dropdownOf(tester);
        expect(dropdown.value, 'config.acUsesDex');
        expect(dropdown.onChanged, isNotNull);
        expect(dropdown.hint, isNull, reason: 'a selected value shows no hint');
        final values = dropdown.items!.map((item) => item.value).toList();
        expect(values, ['config.acUsesDex', 'config.acUsesCon']);
        expect(
          values,
          isNot(contains(exclusiveNoneValue)),
          reason: 'a group with an active default offers no None (EXC-10)',
        );
        expect(labelsOf(dropdown), {
          'config.acUsesDex': 'Dexterity',
          'config.acUsesCon': 'Constitution',
        });
      },
    );

    testWidgets(
      'Happy path: a six-option group with no active default is the same dropdown and offers None',
      (tester) async {
        const abilities = [
          'Strength',
          'Dexterity',
          'Constitution',
          'Intelligence',
          'Wisdom',
          'Charisma',
        ];
        await pump(
          tester,
          group: groupOf([
            for (final ability in abilities)
              field(
                'config.spellUses$ability',
                label: 'Spellcasting uses ${ability.toLowerCase()}',
                optionLabel: ability,
              ),
          ]),
          onSelect: (_) {},
        );

        expect(find.byType(DropdownButton<String>), findsOneWidget);
        expect(find.byType(Radio<String>), findsNothing);
        final dropdown = dropdownOf(tester);
        expect(dropdown.value, exclusiveNoneValue);
        final values = dropdown.items!.map((item) => item.value).toList();
        expect(values, [
          exclusiveNoneValue,
          for (final ability in abilities) 'config.spellUses$ability',
        ]);
        final labels = labelsOf(dropdown);
        expect(labels[exclusiveNoneValue], 'None');
        for (final ability in abilities) {
          expect(
            labels['config.spellUses$ability'],
            ability,
            reason: 'the option text comes from option_label (EXC-27)',
          );
        }
      },
    );

    testWidgets(
      'Edge: a missing or empty option_label falls back to the member schema label (EXC-27)',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.a', label: 'Alpha', optionLabel: 'Short A'),
            field('config.b', label: 'Bravo', optionLabel: ''),
            field('config.c', label: 'Charlie'),
          ]),
          onSelect: (_) {},
        );

        final labels = labelsOf(dropdownOf(tester));
        expect(labels['config.a'], 'Short A');
        expect(labels['config.b'], 'Bravo');
        expect(labels['config.c'], 'Charlie');
      },
    );

    testWidgets(
      'Edge: an active default with nothing selected shows no value and no None',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.a', label: 'Alpha', defaultValue: 1),
            field('config.b', label: 'Bravo'),
          ]),
          onSelect: (_) {},
        );

        expect(dropdownOf(tester).value, isNull);
        expect(
          find.byKey(const Key('sheet-exclusive-option-none')),
          findsNothing,
        );
        expect(
          find.byKey(const Key('sheet-exclusive-hint-none')),
          findsOneWidget,
          reason: 'a blank default-active group shows the None hint',
        );
        expect(
          find.text('None'),
          findsOneWidget,
          reason: 'the hint reads None',
        );
      },
    );
  });

  group('selection (EXC-09, EXC-13)', () {
    testWidgets('Happy path: selecting a member reports its path', (
      tester,
    ) async {
      final selected = <String>[];
      await pump(
        tester,
        group: groupOf([
          field('config.acUsesDex', label: 'Dexterity', defaultValue: 1),
          field('config.acUsesCon', label: 'Constitution'),
        ]),
        selectedPath: 'config.acUsesDex',
        onSelect: selected.add,
      );

      await selectOption(tester, 'sheet-exclusive-option-config.acUsesCon');

      expect(selected, ['config.acUsesCon']);
    });

    testWidgets('Happy path: selecting None reports the sentinel', (
      tester,
    ) async {
      final selected = <String>[];
      await pump(
        tester,
        group: groupOf([
          field('config.a', label: 'Alpha'),
          field('config.b', label: 'Bravo'),
        ]),
        onSelect: selected.add,
      );

      await selectOption(tester, 'sheet-exclusive-option-none');

      expect(selected, [exclusiveNoneValue]);
    });
  });

  group('read-only (EXC-11)', () {
    testWidgets(
      'Happy path: read-only renders the dropdown disabled with the active option shown',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.acUsesDex', label: 'Dexterity', defaultValue: 1),
            field('config.acUsesCon', label: 'Constitution'),
          ]),
          selectedPath: 'config.acUsesDex',
        );

        expect(dropdownOf(tester).onChanged, isNull);
        expect(dropdownOf(tester).value, 'config.acUsesDex');
      },
    );

    testWidgets(
      'Edge: read-only with no active member shows None and stays disabled',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.a', label: 'Alpha'),
            field('config.b', label: 'Bravo'),
          ]),
        );

        expect(dropdownOf(tester).onChanged, isNull);
        expect(dropdownOf(tester).value, exclusiveNoneValue);
        expect(
          find.byKey(const Key('sheet-exclusive-option-none')),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: read-only with an active default and no value shows the None hint and stays disabled',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.a', label: 'Alpha', defaultValue: 1),
            field('config.b', label: 'Bravo'),
          ]),
        );

        final dropdown = dropdownOf(tester);
        expect(dropdown.onChanged, isNull);
        expect(dropdown.value, isNull);
        expect(
          dropdown.hint,
          isNotNull,
          reason: 'a blank read-only row is not empty',
        );
        expect(
          find.byKey(const Key('sheet-exclusive-hint-none')),
          findsOneWidget,
        );
      },
    );
  });

  group('preview marks (PRV-12)', () {
    testWidgets(
      'Happy path: a modified group tints the dropdown (PRV-12)',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.acUsesDex', defaultValue: 1),
            field('config.acUsesCon'),
          ]),
          selectedPath: 'config.acUsesDex',
          mark: SheetMark.modified,
          onSelect: (_) {},
        );

        expect(
          find.byKey(const Key('sheet-mark-tint-armorClassAbility')),
          findsOneWidget,
        );
        expect(
          tester
              .widget<DecoratedBox>(
                find.byKey(const Key('sheet-mark-tint-armorClassAbility')),
              )
              .decoration,
          isA<BoxDecoration>(),
        );
      },
    );

    testWidgets(
      'Edge: an unmarked group renders no tint wrapper (PRV-12)',
      (tester) async {
        await pump(
          tester,
          group: groupOf([
            field('config.acUsesDex', defaultValue: 1),
            field('config.acUsesCon'),
          ]),
          selectedPath: 'config.acUsesDex',
          onSelect: (_) {},
        );

        expect(
          find.byKey(const Key('sheet-mark-tint-armorClassAbility')),
          findsNothing,
        );
      },
    );
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
