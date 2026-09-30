import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/catalog/template_summary.dart';

/// Unit tests for the template summary derivation (TPL-02, TPL-03, TPL-05).
/// Every asserted value comes from the spec rule, never from the implementation.
void main() {
  FieldNode leaf({String label = ''}) => FieldNode(
    schema: FieldSchema(
      type: 'number',
      label: label,
      isEditable: true,
      defaultValue: 0,
    ),
  );

  FieldNode fieldGroup(Map<String, FieldNode> children) =>
      FieldNode(children: children);

  group('summarizeFields (TPL-02, TPL-03, TPL-05)', () {
    test(
      'Happy path: the pinned fantasy_v1 fixture lists its visible leaves with normalized names (TPL-02, TPL-03)',
      () {
        final decoded =
            jsonDecode(
                  File('test/fixtures/template_detail.json').readAsStringSync(),
                )
                as Map<String, Object?>;
        final template = TemplateDetail.fromJson(
          (decoded['data'] as Map).cast<String, Object?>(),
        );

        final summary = summarizeFields(template.fields);

        expect(summary.map((line) => line.name), [
          'Base armor class',
          'Others',
          'Stats',
        ]);
        expect(summary[0].members, isEmpty);
        expect(summary[1].members, ['Proficiency bonus']);
        expect(summary[2].members, ['Constitution', 'Dexterity']);
      },
    );

    test(
      'Happy path: a group lists every leaf beneath it recursively with normalized names (TPL-03)',
      () {
        final fields = <String, FieldNode>{
          'baseArmorClass': leaf(),
          'stats': fieldGroup({'dexterity': leaf(), 'constitution': leaf()}),
        };

        final summary = summarizeFields(fields);

        expect(summary, hasLength(2));
        expect(summary[0].name, 'Base armor class');
        expect(summary[0].members, isEmpty);
        expect(summary[1].name, 'Stats');
        expect(summary[1].members, ['Dexterity', 'Constitution']);
      },
    );

    test('Edge: a nested group enumerates its leaves recursively', () {
      final fields = <String, FieldNode>{
        'outer': fieldGroup({
          'inner': fieldGroup({'aValue': leaf(), 'bValue': leaf()}),
          'cValue': leaf(),
        }),
      };

      final summary = summarizeFields(fields);

      expect(summary.single.name, 'Outer');
      expect(summary.single.members, ['A value', 'B value', 'C value']);
    });

    test('Edge: an empty fields tree yields no summary lines', () {
      expect(summarizeFields(<String, FieldNode>{}), isEmpty);
    });

    test('Edge: reserved groups are omitted from the summary (TPL-02, TPL-05)', () {
      final fields = <String, FieldNode>{
        'config': fieldGroup({'acUsesDex': leaf()}),
        'derived': fieldGroup({'armorClass': leaf()}),
        'modifiers': fieldGroup({'dexterityMod': leaf()}),
        'stats': fieldGroup({'dexterity': leaf()}),
      };

      expect(summarizeFields(fields).map((line) => line.name), ['Stats']);
    });

    test('Edge: an unknown prefix still renders without code changes (TPL-05)', () {
      final fields = <String, FieldNode>{
        'attributeSet': fieldGroup({'might': leaf()}),
        'zzz': leaf(),
      };

      final summary = summarizeFields(fields);

      expect(summary.map((line) => line.name), ['Attribute set', 'Zzz']);
      expect(summary[0].members, ['Might']);
      expect(summary[1].members, isEmpty);
    });

    test('Edge: a schema label takes precedence over the humanized key (TPL-02)', () {
      final fields = <String, FieldNode>{
        'stats': fieldGroup({'dexterity': leaf(label: 'Destreza')}),
      };

      expect(summarizeFields(fields).single.members, ['Destreza']);
    });

    test('Edge: a blank schema label falls back to the humanized key (TPL-02)', () {
      final fields = <String, FieldNode>{
        'stats': fieldGroup({'dexterity': leaf(label: '   ')}),
      };

      expect(summarizeFields(fields).single.members, ['Dexterity']);
    });

    test('Edge: a snake_case key is humanized (TPL-02)', () {
      final fields = <String, FieldNode>{
        'spell_group': fieldGroup({'spell_uses_int': leaf()}),
      };

      final summary = summarizeFields(fields);

      expect(summary.single.name, 'Spell group');
      expect(summary.single.members, ['Spell uses int']);
    });

    test('Edge: a group with no leaves renders its name with no members', () {
      final fields = <String, FieldNode>{'ghost': fieldGroup({})};

      final summary = summarizeFields(fields);

      expect(summary.single.name, 'Ghost');
      expect(summary.single.members, isEmpty);
    });

    test('Edge: two top-level keys sharing a leaf name list independently', () {
      final fields = <String, FieldNode>{
        'a': fieldGroup({'value': leaf()}),
        'b': fieldGroup({'value': leaf()}),
      };

      final summary = summarizeFields(fields);

      expect(summary.map((line) => line.name), ['A', 'B']);
      expect(summary[0].members, ['Value']);
      expect(summary[1].members, ['Value']);
    });
  });
}
