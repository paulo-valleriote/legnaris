import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/sheet_defaults.dart';

void main() {
  FieldNode leaf(double defaultValue, {String label = '', bool editable = true}) =>
      FieldNode(
        schema: FieldSchema(
          type: 'number',
          label: label,
          isEditable: editable,
          defaultValue: defaultValue,
        ),
      );

  FieldNode groupOf(Map<String, FieldNode> children) =>
      FieldNode(children: children);

  group('seedDefaults', () {
    test('Happy path: seeds every leaf default, nested under its group', () {
      final fields = {
        'baseArmorClass': leaf(10),
        'stats': groupOf({
          'dexterity': leaf(10),
          'nested': groupOf({'constitution': leaf(12)}),
        }),
      };

      expect(seedDefaults(fields), {
        'baseArmorClass': 10.0,
        'stats': {
          'dexterity': 10.0,
          'nested': {'constitution': 12.0},
        },
      });
    });

    test('Edge: a zero default is kept, not dropped', () {
      final fields = {
        'derived': groupOf({'armorClass': leaf(0, editable: false)}),
      };

      expect(seedDefaults(fields), {
        'derived': {'armorClass': 0.0},
      });
    });

    test('Edge: a group with no leaves contributes nothing', () {
      final fields = {
        'empty': groupOf({}),
        'stats': groupOf({'dexterity': leaf(10)}),
      };

      expect(seedDefaults(fields), {
        'stats': {'dexterity': 10.0},
      });
    });

    test('Edge: an empty tree yields an empty map', () {
      expect(seedDefaults(const {}), isEmpty);
    });
  });

  group('setPathValue', () {
    test('Happy path: sets a top-level key', () {
      final data = <String, Object?>{};

      setPathValue(data, 'value', 3.0);

      expect(data, {'value': 3.0});
    });

    test('Edge: creates the missing intermediate maps for a dotted path', () {
      final data = <String, Object?>{};

      setPathValue(data, 'stats.dexterity', 10.0);

      expect(data, {
        'stats': {'dexterity': 10.0},
      });
    });

    test('Edge: reuses an existing intermediate map', () {
      final data = <String, Object?>{
        'stats': <String, Object?>{'dexterity': 1.0},
      };

      setPathValue(data, 'stats.constitution', 2.0);

      expect(data, {
        'stats': {'dexterity': 1.0, 'constitution': 2.0},
      });
    });

    test('Edge: overwrites an existing leaf', () {
      final data = <String, Object?>{'value': 1.0};

      setPathValue(data, 'value', 2.0);

      expect(data, {'value': 2.0});
    });
  });
}
