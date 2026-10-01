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
  }) => FieldNode(
    schema: FieldSchema(
      type: type,
      label: label,
      formula: formula,
      isEditable: editable,
      defaultValue: defaultValue,
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
