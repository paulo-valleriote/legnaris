import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/catalog/template_summary.dart';

/// Unit tests for the prefix summary derivation (TPL-02, TPL-03, TPL-05).
/// Every asserted value comes from the spec rule, never from the implementation.
void main() {
  FieldNode leaf() => const FieldNode(
    schema: FieldSchema(
      type: 'number',
      label: '',
      isEditable: true,
      defaultValue: 0,
    ),
  );

  FieldNode fieldGroup(Map<String, FieldNode> children) =>
      FieldNode(children: children);

  group('summarizeFields (TPL-02, TPL-03, TPL-05)', () {
    test(
      'Happy path: one line per top-level key in API order with recursive leaf counts (TPL-02, TPL-03)',
      () {
        final fields = <String, FieldNode>{
          'baseArmorClass': leaf(),
          'config': fieldGroup({'acUsesDex': leaf(), 'acUsesCon': leaf()}),
          'stats': fieldGroup({'dexterity': leaf(), 'constitution': leaf()}),
        };

        final summary = summarizeFields(fields);

        expect(summary, [
          (prefix: 'baseArmorClass', leafCount: 1),
          (prefix: 'config', leafCount: 2),
          (prefix: 'stats', leafCount: 2),
        ]);
      },
    );

    test(
      'Happy path: the pinned fantasy_v1 fixture summary matches its real leaf counts (TPL-02, TPL-03)',
      () {
        final decoded =
            jsonDecode(
                  File('test/fixtures/template_detail.json').readAsStringSync(),
                )
                as Map<String, Object?>;
        final template = TemplateDetail.fromJson(
          (decoded['data'] as Map).cast<String, Object?>(),
        );

        expect(summarizeFields(template.fields), [
          (prefix: 'baseArmorClass', leafCount: 1),
          (prefix: 'config', leafCount: 5),
          (prefix: 'derived', leafCount: 2),
          (prefix: 'modifiers', leafCount: 2),
          (prefix: 'others', leafCount: 1),
          (prefix: 'stats', leafCount: 2),
        ]);
      },
    );

    test(
      'Happy path: a nested group counts every leaf beneath it recursively (TPL-03)',
      () {
        final fields = <String, FieldNode>{
          'outer': fieldGroup({
            'inner': fieldGroup({'a': leaf(), 'b': leaf()}),
            'c': leaf(),
          }),
        };

        expect(summarizeFields(fields), [(prefix: 'outer', leafCount: 3)]);
      },
    );

    test('Edge: an empty fields tree yields no summary lines', () {
      expect(summarizeFields(<String, FieldNode>{}), isEmpty);
    });

    test('Edge: a top-level leaf counts as 1 under its own key', () {
      final fields = <String, FieldNode>{'proficiencyBonus': leaf()};

      expect(summarizeFields(fields), [(prefix: 'proficiencyBonus', leafCount: 1)]);
    });

    test('Edge: a group holding a single leaf still renders its own line', () {
      final fields = <String, FieldNode>{
        'others': fieldGroup({'proficiencyBonus': leaf()}),
      };

      expect(summarizeFields(fields), [(prefix: 'others', leafCount: 1)]);
    });

    test('Edge: an unexpected prefix renders without code changes (TPL-05)', () {
      final fields = <String, FieldNode>{
        'attributeSet': fieldGroup({'might': leaf()}),
        'zzz': leaf(),
      };

      expect(summarizeFields(fields), [
        (prefix: 'attributeSet', leafCount: 1),
        (prefix: 'zzz', leafCount: 1),
      ]);
    });

    test('Edge: a node with neither schema nor children counts zero leaves', () {
      final fields = <String, FieldNode>{'ghost': const FieldNode()};

      expect(summarizeFields(fields), [(prefix: 'ghost', leafCount: 0)]);
    });

    test('Edge: two top-level keys sharing a leaf name count independently', () {
      final fields = <String, FieldNode>{
        'a': fieldGroup({'value': leaf()}),
        'b': fieldGroup({'value': leaf()}),
      };

      expect(summarizeFields(fields), [
        (prefix: 'a', leafCount: 1),
        (prefix: 'b', leafCount: 1),
      ]);
    });
  });
}
