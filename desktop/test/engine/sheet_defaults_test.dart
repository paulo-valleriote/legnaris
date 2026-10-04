import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/engine/sheet_defaults.dart';

void main() {
  FieldNode leaf(
    double defaultValue, {
    String label = '',
    bool editable = true,
    String type = 'number',
  }) => FieldNode(
    schema: FieldSchema(
      type: type,
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

    test(
      'Happy path: text and bool leaves seed by type, not by float default',
      () {
        final fields = {
          'identity': groupOf({
            'characterName': leaf(0, type: 'text'),
            'ready': leaf(1, type: 'bool', editable: false),
          }),
        };

        expect(seedDefaults(fields), {
          'identity': {'characterName': '', 'ready': false},
        });
      },
    );

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

  group('seedFromSheet (OPN-10)', () {
    test(
      'Happy path: a stored value overrides its schema default and a missing key keeps the default',
      () {
        final fields = {
          'stats': groupOf({'dexterity': leaf(10), 'constitution': leaf(12)}),
        };

        expect(seedFromSheet(fields, const {
          'stats': {'dexterity': 16},
        }), {
          'stats': {'dexterity': 16, 'constitution': 12.0},
        });
      },
    );

    test('Edge: a stored key absent from the schema is carried over', () {
      final fields = {
        'stats': groupOf({'dexterity': leaf(10)}),
      };

      expect(seedFromSheet(fields, const {
        'stats': {'dexterity': 16, 'extra': 'kept'},
        'notInSchema': {'nested': 1},
      }), {
        'stats': {'dexterity': 16, 'extra': 'kept'},
        'notInSchema': {'nested': 1},
      });
    });

    test('Edge: a nested group merges deeply instead of replacing the group', () {
      final fields = {
        'outer': groupOf({
          'inner': groupOf({'a': leaf(1), 'b': leaf(2)}),
        }),
      };

      expect(seedFromSheet(fields, const {
        'outer': {
          'inner': {'a': 9},
        },
      }), {
        'outer': {
          'inner': {'a': 9, 'b': 2.0},
        },
      });
    });

    test('Edge: empty stored data yields the schema defaults unchanged', () {
      final fields = {
        'stats': groupOf({'dexterity': leaf(10)}),
      };

      expect(seedFromSheet(fields, const {}), seedDefaults(fields));
    });

    test('Edge: the returned tree is fresh, so mutating it leaves data intact', () {
      final data = <String, Object?>{
        'stats': <String, Object?>{'dexterity': 16},
      };
      final fields = {
        'stats': groupOf({'dexterity': leaf(10)}),
      };

      final seeded = seedFromSheet(fields, data);
      (seeded['stats']! as Map<String, Object?>)['dexterity'] = 99;

      expect((data['stats']! as Map<String, Object?>)['dexterity'], 16);
    });
  });

  group('mergeCascade (SAV-13)', () {
    test(
      'Happy path: adds a new nested key and overwrites an existing leaf',
      () {
        final base = <String, Object?>{
          'stats': <String, Object?>{'dexterity': 16},
          'derived': <String, Object?>{'armorClass': 10.0},
        };

        expect(mergeCascade(base, const {
          'derived': {'armorClass': 13.0},
          'modifiers': {'dexterityMod': 3},
        }), {
          'stats': {'dexterity': 16},
          'derived': {'armorClass': 13.0},
          'modifiers': {'dexterityMod': 3},
        });
      },
    );

    test('Edge: a deep nested tree merges recursively instead of replacing', () {
      final base = <String, Object?>{
        'a': <String, Object?>{
          'b': <String, Object?>{'keep': 1, 'change': 2},
        },
      };

      expect(mergeCascade(base, const {
        'a': {
          'b': {'change': 9},
        },
      }), {
        'a': {
          'b': {'keep': 1, 'change': 9},
        },
      });
    });

    test('Edge: an empty delta yields an equal tree', () {
      final base = <String, Object?>{
        'stats': <String, Object?>{'dexterity': 16},
      };

      expect(mergeCascade(base, const {}), base);
    });

    test('Edge: the inputs are not mutated', () {
      final base = <String, Object?>{
        'derived': <String, Object?>{'armorClass': 10.0},
      };
      final delta = <String, Object?>{
        'derived': <String, Object?>{'armorClass': 13.0},
      };

      mergeCascade(base, delta);

      expect((base['derived']! as Map<String, Object?>)['armorClass'], 10.0);
      expect((delta['derived']! as Map<String, Object?>)['armorClass'], 13.0);
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
