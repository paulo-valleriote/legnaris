import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/sheet_form.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/app_theme.dart';

void main() {
  FieldNode leaf(
    String type, {
    String label = '',
    bool editable = true,
    String? formula,
  }) => FieldNode(
    schema: FieldSchema(
      type: type,
      label: label,
      formula: formula,
      isEditable: editable,
      defaultValue: 0,
    ),
  );

  FieldNode groupOf(Map<String, FieldNode> children) =>
      FieldNode(children: children);

  Future<void> pumpForm(
    WidgetTester tester, {
    required FieldGroup fields,
    required Map<String, Object?> data,
    void Function(String path, Object? value)? onChanged,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        theme: buildAppTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: SingleChildScrollView(
            child: SheetForm(fields: fields, data: data, onChanged: onChanged),
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
}
