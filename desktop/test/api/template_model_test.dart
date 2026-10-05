import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/template.dart';

void main() {
  const detailJson = '''
{
  "id": "fantasy_v1",
  "name": "Fantasy Template",
  "default": true,
  "fields": {
    "stats": {
      "children": {
        "dexterity": {
          "schema": {
            "type": "number",
            "label": "Dexterity",
            "is_editable": true,
            "default": 10
          }
        }
      }
    },
    "modifiers": {
      "children": {
        "dexterityMod": {
          "schema": {
            "type": "number",
            "label": "Dexterity modifier",
            "formula": "floor((stats_dexterity - 10) / 2)",
            "depends_on": [["stats", "dexterity"]],
            "is_editable": false,
            "default": 0
          }
        }
      }
    },
    "baseArmorClass": {
      "schema": {
        "type": "number",
        "label": "Base armor class",
        "is_editable": true,
        "default": 10
      }
    }
  }
}
''';

  const catalogJson = '''
[
  {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {"baseArmorClass": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}},
  {"id": "sci_fi_v1", "name": "Sci Fi Template", "default": false, "fields": {}}
]
''';

  group('catalog decode (FE1-08)', () {
    test('Happy path: catalog list decodes every item with id, name, default and fields', () {
      final items = TemplateCatalog.listFromJson(jsonDecode(catalogJson));
      expect(items, hasLength(2));

      expect(items[0].id, 'fantasy_v1');
      expect(items[0].name, 'Fantasy Template');
      expect(items[0].isDefault, isTrue);
      expect(items[0].fields, contains('baseArmorClass'));

      expect(items[1].id, 'sci_fi_v1');
      expect(items[1].name, 'Sci Fi Template');
      expect(items[1].isDefault, isFalse);
      expect(items[1].fields, isEmpty);
    });

    test('Happy path: detail decodes id, name, default and fields', () {
      // FE1-08: the JSON key `default` is a reserved word in Dart, so it must
      // be mapped explicitly by the fromJson, not by case convention.
      final template = TemplateDetail.fromJson(
        jsonDecode(detailJson) as Map<String, Object?>,
      );
      expect(template.id, 'fantasy_v1');
      expect(template.name, 'Fantasy Template');
      expect(template.isDefault, isTrue);
      expect(template.fields.keys, containsAll(['stats', 'modifiers', 'baseArmorClass']));
    });

    test('Happy path: leaf and group nodes keep the tree shape', () {
      // FE1-08: leaf = schema only, group = children only.
      final template = TemplateDetail.fromJson(
        jsonDecode(detailJson) as Map<String, Object?>,
      );

      final group = template.fields['stats']!;
      expect(group.schema, isNull);
      expect(group.children, isNotNull);

      final leaf = group.children!['dexterity']!;
      expect(leaf.schema, isNotNull);
      expect(leaf.children, isNull);
      expect(leaf.schema!.type, 'number');
      expect(leaf.schema!.label, 'Dexterity');
      expect(leaf.schema!.isEditable, isTrue);

      final rootLeaf = template.fields['baseArmorClass']!;
      expect(rootLeaf.schema, isNotNull);
      expect(rootLeaf.children, isNull);
    });
  });

  group('field schema decode (FE1-09)', () {
    test('Happy path: complete leaf schema decodes every field', () {
      final template = TemplateDetail.fromJson(
        jsonDecode(detailJson) as Map<String, Object?>,
      );
      final schema = template.fields['modifiers']!.children!['dexterityMod']!.schema!;

      expect(schema.type, 'number');
      expect(schema.label, 'Dexterity modifier');
      expect(schema.formula, 'floor((stats_dexterity - 10) / 2)');
      expect(schema.dependsOn, [
        ['stats', 'dexterity'],
      ]);
      expect(schema.isEditable, isFalse);
      expect(schema.defaultValue, 0);
    });

    test('Edge: schema without formula and depends_on decodes them as absent', () {
      // FE1-09: formula and depends_on are optional in the contract.
      final template = TemplateDetail.fromJson(
        jsonDecode(detailJson) as Map<String, Object?>,
      );
      final schema = template.fields['stats']!.children!['dexterity']!.schema!;

      expect(schema.formula, isNull);
      expect(schema.dependsOn, isNull);
      expect(schema.defaultValue, 10);
    });

    test('Edge: default key decodes numeric values exactly', () {
      final schema = FieldSchema.fromJson({
        'type': 'boolean',
        'label': 'Toggle',
        'is_editable': true,
        'default': 16.5,
      });
      final intSchema = FieldSchema.fromJson({
        'type': 'number',
        'label': 'Count',
        'is_editable': true,
        'default': 16,
      });

      expect(schema.defaultValue, 16.5);
      expect(intSchema.defaultValue, 16);
    });
  });

  group('exclusive group decode (EXC-17)', () {
    test('Happy path: a schema with exclusive_group decodes the group id', () {
      // EXC-17: the contract advertises the membership of a leaf.
      final schema = FieldSchema.fromJson({
        'type': 'number',
        'label': 'Armor class uses dexterity',
        'is_editable': true,
        'default': 1,
        'exclusive_group': 'armorClassAbility',
      });

      expect(schema.exclusiveGroup, 'armorClassAbility');
    });

    test('Edge: a schema without exclusive_group decodes it as null', () {
      // EXC-17 edge: backward compatible with the pre-LGN-105 contract.
      final schema = FieldSchema.fromJson({
        'type': 'number',
        'label': 'Dexterity',
        'is_editable': true,
        'default': 10,
      });

      expect(schema.exclusiveGroup, isNull);
    });

    test('Edge: an explicit null exclusive_group decodes as null', () {
      final schema = FieldSchema.fromJson({
        'type': 'number',
        'label': 'Dexterity',
        'is_editable': true,
        'default': 10,
        'exclusive_group': null,
      });

      expect(schema.exclusiveGroup, isNull);
    });

    test('Edge: an untagged leaf of the rich fixture decodes as null', () {
      final template = TemplateDetail.fromJson(
        jsonDecode(detailJson) as Map<String, Object?>,
      );
      final schema = template.fields['modifiers']!.children!['dexterityMod']!.schema!;

      expect(schema.exclusiveGroup, isNull);
    });
  });

  group('option label decode (EXC-26)', () {
    test('Happy path: a schema with option_label decodes the short label', () {
      // EXC-26: the contract advertises the dropdown option label.
      final schema = FieldSchema.fromJson({
        'type': 'number',
        'label': 'Armor class uses dexterity',
        'is_editable': true,
        'default': 1,
        'exclusive_group': 'armorClassAbility',
        'option_label': 'Dexterity',
      });

      expect(schema.optionLabel, 'Dexterity');
    });

    test('Edge: a schema without option_label decodes it as null', () {
      // EXC-26 edge: backward compatible with the pre-increment contract.
      final schema = FieldSchema.fromJson({
        'type': 'number',
        'label': 'Dexterity',
        'is_editable': true,
        'default': 10,
      });

      expect(schema.optionLabel, isNull);
    });

    test('Edge: an explicit null option_label decodes as null', () {
      final schema = FieldSchema.fromJson({
        'type': 'number',
        'label': 'Dexterity',
        'is_editable': true,
        'default': 10,
        'option_label': null,
      });

      expect(schema.optionLabel, isNull);
    });

    test('Edge: an untagged leaf of the rich fixture decodes as null', () {
      final template = TemplateDetail.fromJson(
        jsonDecode(detailJson) as Map<String, Object?>,
      );
      final schema = template.fields['modifiers']!.children!['dexterityMod']!.schema!;

      expect(schema.optionLabel, isNull);
    });
  });

  group('table metadata decode (RND-21)', () {
    test('Happy path: a schema with column_label decodes the short label', () {
      final schema = FieldSchema.fromJson({
        'type': 'number',
        'label': 'Acrobatics bonus',
        'is_editable': true,
        'default': 0,
        'column_label': 'Bonus',
      });

      expect(schema.columnLabel, 'Bonus');
    });

    test('Edge: a schema without column_label decodes it as null', () {
      final schema = FieldSchema.fromJson({
        'type': 'number',
        'label': 'Dexterity',
        'is_editable': true,
        'default': 10,
      });

      expect(schema.columnLabel, isNull);
    });

    test('Happy path: a group node decodes its label and column order', () {
      final node = FieldNode.fromJson({
        'children': {
          'name': {
            'schema': {
              'type': 'text',
              'label': 'Attack 1 name',
              'is_editable': true,
              'default': 0,
            },
          },
        },
        'label': 'Attack 1',
        'columns': ['name', 'damage', 'bonus'],
      });

      expect(node.label, 'Attack 1');
      expect(node.columns, ['name', 'damage', 'bonus']);
    });

    test('Edge: a group node without table metadata decodes both as null', () {
      final node = FieldNode.fromJson(<String, Object?>{'children': <String, Object?>{}});

      expect(node.label, isNull);
      expect(node.columns, isNull);
    });

    test('Edge: an explicit null columns decodes as null', () {
      final node = FieldNode.fromJson(<String, Object?>{
        'children': <String, Object?>{},
        'columns': null,
      });

      expect(node.columns, isNull);
    });
  });
}
