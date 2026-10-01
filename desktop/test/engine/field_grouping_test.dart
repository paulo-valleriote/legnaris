import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/field_grouping.dart';

void main() {
  FieldNode leaf(
    String type, {
    String label = '',
    bool editable = true,
    String? formula,
    double defaultValue = 0,
    String? exclusiveGroup,
  }) => FieldNode(
    schema: FieldSchema(
      type: type,
      label: label,
      formula: formula,
      isEditable: editable,
      defaultValue: defaultValue,
      exclusiveGroup: exclusiveGroup,
    ),
  );

  FieldNode groupOf(Map<String, FieldNode> children) =>
      FieldNode(children: children);

  group('buildSections (RND-01, RND-02, RND-03)', () {
    test(
      'Happy path: one section per top-level key in API order (RND-01)',
      () {
        final fields = {
          'stats': groupOf({'dexterity': leaf('number')}),
          'others': groupOf({'proficiencyBonus': leaf('number')}),
        };

        final sections = buildSections(fields);

        expect(sections.map((section) => section.key), ['stats', 'others']);
        expect(sections.map((section) => section.label), ['Stats', 'Others']);
      },
    );

    test(
      'Happy path: a nested group flattens into dotted leaf paths in tree order (RND-02)',
      () {
        final fields = {
          'a': groupOf({
            'b': groupOf({'c': leaf('number')}),
            'd': leaf('number'),
          }),
        };

        final leaves = buildSections(fields).single.leaves;

        expect(leaves.map((field) => field.path), ['a.b.c', 'a.d']);
      },
    );

    test(
      'Happy path: a top-level leaf becomes its own single-field section (RND-03)',
      () {
        final fields = {'baseArmorClass': leaf('number')};

        final section = buildSections(fields).single;

        expect(section.key, 'baseArmorClass');
        expect(section.label, 'Base armor class');
        expect(section.leaves, hasLength(1));
        expect(section.leaves.single.path, 'baseArmorClass');
      },
    );

    test(
      'Happy path: a top-level leaf section prefers its schema label (RND-04)',
      () {
        final fields = {
          'baseArmorClass': leaf('number', label: 'Armor class base'),
        };

        final section = buildSections(fields).single;

        expect(section.label, 'Armor class base');
      },
    );

    test('Edge: empty fields tree yields no sections', () {
      expect(buildSections(const {}), isEmpty);
    });

    test('Edge: a group with no leaves is omitted', () {
      final fields = {
        'empty': groupOf({}),
        'stats': groupOf({'dexterity': leaf('number')}),
      };

      final sections = buildSections(fields);

      expect(sections.map((section) => section.key), ['stats']);
    });

    test('Edge: two top-level keys sharing a leaf name stay independent', () {
      final fields = {
        'modifiers': groupOf({'value': leaf('number')}),
        'derived': groupOf({'value': leaf('number', editable: false)}),
      };

      final sections = buildSections(fields);

      expect(sections.map((section) => section.key), ['modifiers', 'derived']);
      expect(
        sections.map((section) => section.leaves.single.path),
        ['modifiers.value', 'derived.value'],
      );
    });
  });

  group('exclusive grouping (EXC-09, EXC-12)', () {
    test(
      "Happy path: a two-member group becomes one exclusive row at its first member's position, options in API order (EXC-09)",
      () {
        final fields = {
          'config': groupOf({
            'first': leaf('number'),
            'acUsesDex': leaf('number', exclusiveGroup: 'armorClassAbility'),
            'acUsesCon': leaf('number', exclusiveGroup: 'armorClassAbility'),
            'last': leaf('number'),
          }),
        };

        final section = buildSections(fields).single;

        expect(
          section.rows
              .map((row) => row.fields.map((field) => field.path).toList())
              .toList(),
          [
            ['config.first'],
            ['config.acUsesDex', 'config.acUsesCon'],
            ['config.last'],
          ],
        );
        final exclusive = section.rows[1].exclusive;
        expect(exclusive, isNotNull);
        expect(exclusive!.id, 'armorClassAbility');
        expect(
          exclusive.options.map((option) => option.path).toList(),
          ['config.acUsesDex', 'config.acUsesCon'],
        );
      },
    );

    test('Edge: a single-member group stays a normal row with no group (EXC-12)', () {
      final fields = {
        'config': groupOf({
          'acUsesDex': leaf('number', exclusiveGroup: 'armorClassAbility'),
          'other': leaf('number'),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(2));
      expect(section.rows.first.exclusive, isNull);
      expect(section.rows.first.fields.single.path, 'config.acUsesDex');
      expect(section.rows.last.exclusive, isNull);
      expect(section.rows.last.fields.single.path, 'config.other');
    });

    test('Edge: untagged leaves keep one normal row each', () {
      final fields = {
        'stats': groupOf({
          'dexterity': leaf('number'),
          'strength': leaf('number'),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(2));
      expect(section.rows.every((row) => row.exclusive == null), isTrue);
      expect(
        section.rows.map((row) => row.fields.single.path).toList(),
        ['stats.dexterity', 'stats.strength'],
      );
    });

    test('Edge: adjacent distinct groups stay separate rows', () {
      final fields = {
        'config': groupOf({
          'a': leaf('number', exclusiveGroup: 'groupA'),
          'b': leaf('number', exclusiveGroup: 'groupA'),
          'c': leaf('number', exclusiveGroup: 'groupB'),
          'd': leaf('number', exclusiveGroup: 'groupB'),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(2));
      expect(section.rows[0].exclusive!.id, 'groupA');
      expect(
        section.rows[0].exclusive!.options.map((option) => option.path).toList(),
        ['config.a', 'config.b'],
      );
      expect(section.rows[1].exclusive!.id, 'groupB');
      expect(
        section.rows[1].exclusive!.options.map((option) => option.path).toList(),
        ['config.c', 'config.d'],
      );
    });

    test('Edge: equal group ids separated by an untagged leaf do not merge', () {
      final fields = {
        'config': groupOf({
          'a': leaf('number', exclusiveGroup: 'groupA'),
          'b': leaf('number'),
          'c': leaf('number', exclusiveGroup: 'groupA'),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(3));
      expect(section.rows.every((row) => row.exclusive == null), isTrue);
    });

    test('Edge: an empty group id counts as untagged', () {
      final fields = {
        'config': groupOf({
          'a': leaf('number', exclusiveGroup: ''),
          'b': leaf('number', exclusiveGroup: ''),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(2));
      expect(section.rows.every((row) => row.exclusive == null), isTrue);
    });
  });

  group('fieldDisplayName (RND-04, RND-05)', () {
    test('Happy path: a non-empty schema label wins over the key', () {
      final schema = FieldSchema(
        type: 'number',
        label: 'Dexterity score',
        isEditable: true,
        defaultValue: 10,
      );

      expect(fieldDisplayName('dexterity', schema), 'Dexterity score');
    });

    final cases = <String, String>{
      '': 'Base armor class',
      '   ': 'Base armor class',
    };

    for (final entry in cases.entries) {
      test(
        'Edge: a blank label (${entry.key.isEmpty ? 'empty' : 'whitespace'}) humanizes the key (RND-05)',
        () {
          final schema = FieldSchema(
            type: 'number',
            label: entry.key,
            isEditable: true,
            defaultValue: 10,
          );

          expect(
            fieldDisplayName('baseArmorClass', schema),
            entry.value,
          );
        },
      );
    }

    test('Edge: an unknown prefix renders through the same derivation (RND-04)', () {
      final fields = {
        'weird_prefix-x': groupOf({'customThing': leaf('number')}),
      };

      final section = buildSections(fields).single;

      expect(section.label, 'Weird prefix x');
      expect(section.leaves.single.path, 'weird_prefix-x.customThing');
    });
  });
}
