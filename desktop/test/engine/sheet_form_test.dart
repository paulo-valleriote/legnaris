import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/exclusive_control.dart';
import 'package:legnaris/engine/field_control.dart';
import 'package:legnaris/engine/sheet_form.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/app_theme.dart';

void main() {
  FieldNode leaf(
    String type, {
    String label = '',
    bool editable = true,
    String? formula,
    String? exclusiveGroup,
  }) => FieldNode(
    schema: FieldSchema(
      type: type,
      label: label,
      formula: formula,
      isEditable: editable,
      defaultValue: 0,
      exclusiveGroup: exclusiveGroup,
    ),
  );

  FieldNode groupOf(Map<String, FieldNode> children) =>
      FieldNode(children: children);

  Future<void> pumpForm(
    WidgetTester tester, {
    required FieldGroup fields,
    required Map<String, Object?>? data,
    void Function(String path, Object? value)? onChanged,
    SheetControlBuilder? controlBuilder,
    SheetExclusiveControlBuilder? exclusiveControlBuilder,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: SingleChildScrollView(
            child: SheetForm(
              fields: fields,
              data: data,
              onChanged: onChanged,
              controlBuilder: controlBuilder,
              exclusiveControlBuilder: exclusiveControlBuilder,
            ),
          ),
        ),
      ),
    );
  }

  group('sections and rows (RND-01, RND-05)', () {
    testWidgets(
      'Happy path: sections render in API order with derived labels (RND-01)',
      (tester) async {
        final fields = {
          'stats': groupOf({'dexterity': leaf('number')}),
          'others': groupOf({'proficiencyBonus': leaf('number')}),
        };

        await pumpForm(tester, fields: fields, data: const {});

        final stats = find.byKey(const Key('sheet-section-stats'));
        final others = find.byKey(const Key('sheet-section-others'));
        expect(tester.widget<Text>(stats).data, 'Stats');
        expect(tester.widget<Text>(others).data, 'Others');
        expect(
          tester.getTopLeft(stats).dy,
          lessThan(tester.getTopLeft(others).dy),
        );
      },
    );

    testWidgets(
      'Happy path: each row prefers the schema label over the key (RND-05)',
      (tester) async {
        final fields = {
          'stats': groupOf({
            'dexterity': leaf('number'),
            'baseArmorClass': leaf('number', label: 'Base armor class'),
          }),
        };

        await pumpForm(tester, fields: fields, data: const {});

        expect(find.text('Dexterity'), findsOneWidget);
        expect(find.text('Base armor class'), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: two top-level keys sharing a leaf name render independently',
      (tester) async {
        final fields = {
          'modifiers': groupOf({'value': leaf('number')}),
          'derived': groupOf({'value': leaf('number', editable: false)}),
        };

        await pumpForm(
          tester,
          fields: fields,
          data: const {
            'modifiers': {'value': 2},
            'derived': {'value': 3},
          },
        );

        expect(
          find.byKey(const Key('sheet-input-modifiers.value')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('sheet-value-derived.value')),
          findsOneWidget,
        );
        expect(find.text('2'), findsOneWidget);
        expect(find.text('3'), findsOneWidget);
      },
    );

    testWidgets('Edge: an empty field tree renders no sections', (tester) async {
      await pumpForm(tester, fields: const {}, data: const {});

      expect(find.byType(Divider), findsNothing);
      expect(find.text('Calculated'), findsNothing);
    });
  });

  group('values and edits (RND-12, RND-16)', () {
    testWidgets(
      'Happy path: a value resolves from the map by dotted path (RND-16)',
      (tester) async {
        final fields = {
          'stats': groupOf({'dexterity': leaf('number')}),
        };

        await pumpForm(
          tester,
          fields: fields,
          data: const {
            'stats': {'dexterity': 16},
          },
        );

        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '16',
        );
      },
    );

    testWidgets(
      'Happy path: editing a row reports its path and value (RND-12)',
      (tester) async {
        final emitted = <(String, Object?)>[];
        final fields = {
          'stats': groupOf({'dexterity': leaf('number')}),
        };

        await pumpForm(
          tester,
          fields: fields,
          data: const {
            'stats': {'dexterity': 16},
          },
          onChanged: (path, value) => emitted.add((path, value)),
        );

        await tester.enterText(
          find.byKey(const Key('sheet-input-stats.dexterity')),
          '42',
        );

        expect(emitted, [('stats.dexterity', 42)]);
      },
    );

    testWidgets(
      'Happy path: config renders a toggle and a derived field renders Calculated (RND-08, RND-10)',
      (tester) async {
        final fields = {
          'config': groupOf({'acUsesDex': leaf('number')}),
          'derived': groupOf({
            'armorClass': leaf('number', editable: false, formula: 'x + 1'),
          }),
        };

        await pumpForm(
          tester,
          fields: fields,
          data: const {
            'config': {'acUsesDex': 1},
            'derived': {'armorClass': 13},
          },
        );

        expect(
          find.byKey(const Key('sheet-toggle-config.acUsesDex')),
          findsOneWidget,
        );
        expect(find.text('Calculated'), findsOneWidget);
        expect(
          find.byKey(const Key('sheet-value-derived.armorClass')),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: a value missing from the map renders empty without throwing',
      (tester) async {
        final fields = {
          'stats': groupOf({'dexterity': leaf('number')}),
        };

        await pumpForm(tester, fields: fields, data: const {});

        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '',
        );
      },
    );
  });

  group('control builder (LGN-102, GRN-07, GRN-08)', () {
    testWidgets(
      'Happy path: controlBuilder renders its own control for every row instead of the default one',
      (tester) async {
        final fields = {
          'stats': groupOf({'dexterity': leaf('number')}),
          'derived': groupOf({
            'armorClass': leaf('number', editable: false, formula: 'x + 1'),
          }),
        };

        await pumpForm(
          tester,
          fields: fields,
          data: null,
          controlBuilder: (context, field) => Text('built ${field.path}'),
          exclusiveControlBuilder:
              (context, group, selectedPath, onSelect) =>
                  const SizedBox.shrink(),
        );

        expect(find.text('built stats.dexterity'), findsOneWidget);
        expect(find.text('built derived.armorClass'), findsOneWidget);
        expect(find.byType(SheetFieldControl), findsNothing);
        expect(find.byKey(const Key('sheet-section-stats')), findsOneWidget);
        expect(find.text('Dexterity'), findsOneWidget);
      },
    );

    testWidgets(
      'Happy path: without a builder the row keeps the data-driven control',
      (tester) async {
        final fields = {
          'stats': groupOf({'dexterity': leaf('number')}),
        };

        await pumpForm(
          tester,
          fields: fields,
          data: const {
            'stats': {'dexterity': 16},
          },
        );

        expect(find.byType(SheetFieldControl), findsOneWidget);
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '16',
        );
      },
    );

    test(
      'Error: both control sources, or neither, trip the constructor assert',
      () {
        const fields = <String, FieldNode>{};

        expect(
          () => SheetForm(
            fields: fields,
            data: const {},
            controlBuilder: (context, field) => const SizedBox(),
          ),
          throwsA(isA<AssertionError>()),
        );
        expect(
          () => SheetForm(fields: fields),
          throwsA(isA<AssertionError>()),
        );
        expect(
          () => SheetForm(
            fields: fields,
            controlBuilder: (context, field) => const SizedBox(),
          ),
          throwsA(isA<AssertionError>()),
          reason: 'the field builder without the exclusive builder is rejected',
        );
        expect(
          () => SheetForm(
            fields: fields,
            exclusiveControlBuilder:
                (context, group, selectedPath, onSelect) => const SizedBox(),
          ),
          throwsA(isA<AssertionError>()),
          reason: 'the exclusive builder without the field builder is rejected',
        );
        expect(
          () => SheetForm(
            fields: fields,
            data: const {},
            exclusiveControlBuilder:
                (context, group, selectedPath, onSelect) => const SizedBox(),
          ),
          throwsA(isA<AssertionError>()),
          reason: 'data mode with an extra exclusive builder is rejected',
        );
      },
    );
  });

  group('exclusive rows (EXC-09, EXC-11, EXC-12)', () {
    FieldGroup pairFields() => {
      'config': groupOf({
        'acUsesDex': leaf(
          'number',
          label: 'Dexterity',
          exclusiveGroup: 'armorClassAbility',
        ),
        'acUsesCon': leaf(
          'number',
          label: 'Constitution',
          exclusiveGroup: 'armorClassAbility',
        ),
      }),
    };

    testWidgets(
      'Happy path: edit mode renders the injected exclusive builder once and never the members as fields (EXC-09)',
      (tester) async {
        final built = <String>[];
        await pumpForm(
          tester,
          fields: pairFields(),
          data: null,
          controlBuilder: (context, field) => Text('field ${field.path}'),
          exclusiveControlBuilder: (context, group, selectedPath, onSelect) {
            built.add(group.id);
            return Text('exclusive ${group.id}');
          },
        );

        expect(built, ['armorClassAbility']);
        expect(find.text('exclusive armorClassAbility'), findsOneWidget);
        expect(find.text('Armor class ability'), findsOneWidget);
        expect(find.text('field config.acUsesDex'), findsNothing);
        expect(find.text('field config.acUsesCon'), findsNothing);
        expect(
          find.byKey(const Key('sheet-toggle-config.acUsesDex')),
          findsNothing,
        );
        expect(find.byType(SheetFieldControl), findsNothing);
      },
    );

    testWidgets(
      'Happy path: data mode renders the group disabled with the active option selected (EXC-11)',
      (tester) async {
        await pumpForm(
          tester,
          fields: pairFields(),
          data: const {
            'config': {'acUsesDex': 1, 'acUsesCon': 0},
          },
        );

        final segmented = tester.widget<SegmentedButton<String>>(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
        );
        expect(segmented.selected, {'config.acUsesDex'});
        expect(segmented.onSelectionChanged, isNull);
        expect(
          find.byKey(const Key('sheet-toggle-config.acUsesDex')),
          findsNothing,
        );
        expect(
          find.byKey(const Key('sheet-toggle-config.acUsesCon')),
          findsNothing,
        );
      },
    );

    testWidgets(
      'Edge: data mode with no active member renders the group with none selected (EXC-11)',
      (tester) async {
        await pumpForm(
          tester,
          fields: pairFields(),
          data: const {
            'config': {'acUsesDex': 0, 'acUsesCon': 0},
          },
        );

        final segmented = tester.widget<SegmentedButton<String>>(
          find.byKey(const Key('sheet-exclusive-armorClassAbility')),
        );
        expect(segmented.selected, isEmpty);
        expect(segmented.onSelectionChanged, isNull);
      },
    );

    testWidgets(
      'Edge: a single-member group renders the member with its normal per-field control (EXC-12)',
      (tester) async {
        final fields = {
          'config': groupOf({
            'acUsesDex': leaf('number', exclusiveGroup: 'armorClassAbility'),
          }),
        };

        await pumpForm(
          tester,
          fields: fields,
          data: const {
            'config': {'acUsesDex': 1},
          },
        );

        expect(find.byType(SheetExclusiveControl), findsNothing);
        expect(
          find.byKey(const Key('sheet-toggle-config.acUsesDex')),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: an untagged config leaf keeps its independent toggle next to the group',
      (tester) async {
        final fields = {
          'config': groupOf({
            'acUsesDex': leaf('number', exclusiveGroup: 'armorClassAbility'),
            'acUsesCon': leaf('number', exclusiveGroup: 'armorClassAbility'),
            'stealthProfWeight': leaf('number'),
          }),
        };

        await pumpForm(
          tester,
          fields: fields,
          data: const {
            'config': {'acUsesDex': 1, 'acUsesCon': 0, 'stealthProfWeight': 0},
          },
        );

        expect(find.byType(SheetExclusiveControl), findsOneWidget);
        expect(
          find.byKey(const Key('sheet-toggle-config.stealthProfWeight')),
          findsOneWidget,
        );
        expect(
          find.byKey(const Key('sheet-toggle-config.acUsesDex')),
          findsNothing,
        );
        expect(
          find.byKey(const Key('sheet-toggle-config.acUsesCon')),
          findsNothing,
        );
      },
    );
  });
}
