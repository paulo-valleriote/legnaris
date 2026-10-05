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
    String? columnLabel,
  }) => FieldNode(
    schema: FieldSchema(
      type: type,
      label: label,
      formula: formula,
      isEditable: editable,
      defaultValue: defaultValue,
      exclusiveGroup: exclusiveGroup,
      columnLabel: columnLabel,
    ),
  );

  FieldNode groupOf(Map<String, FieldNode> children) =>
      FieldNode(children: children);

  FieldNode tableGroup(
    Map<String, FieldNode> children, {
    String? label,
    List<String>? columns,
  }) => FieldNode(children: children, label: label, columns: columns);

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

  group('table rows (RND-21)', () {
    test(
      'Happy path: a group with columns becomes one row in declared order',
      () {
        final fields = {
          'skills': groupOf({
            'acrobatics': tableGroup(
              {
                'bonus': leaf('number', label: 'Acrobatics bonus'),
                'total': leaf('number', editable: false),
              },
              columns: ['bonus', 'total'],
            ),
          }),
        };

        final section = buildSections(fields).single;

        expect(section.rows, hasLength(1));
        final row = section.rows.single;
        expect(row.exclusive, isNull);
        expect(row.group, isNotNull);
        expect(row.group!.label, 'Acrobatics');
        expect(
          row.fields.map((field) => field.path).toList(),
          ['skills.acrobatics.bonus', 'skills.acrobatics.total'],
        );
      },
    );

    test(
      'Happy path: declared column order wins over the children order',
      () {
        final fields = {
          'attacks': groupOf({
            'attack1': tableGroup(
              {
                'name': leaf('text'),
                'damage': leaf('text'),
                'bonus': leaf('number'),
              },
              label: 'Attack 1',
              columns: ['name', 'damage', 'bonus'],
            ),
          }),
        };

        final row = buildSections(fields).single.rows.single;

        expect(row.group!.label, 'Attack 1');
        expect(
          row.fields.map((field) => field.path).toList(),
          ['attacks.attack1.name', 'attacks.attack1.damage', 'attacks.attack1.bonus'],
        );
      },
    );

    test('Happy path: table rows carry one leaf per declared column', () {
      final fields = {
        'spells': groupOf({
          'slot1': tableGroup(
            {
              'total': leaf('number'),
              'expended': leaf('number'),
              'remaining': leaf('number', editable: false),
            },
            label: 'Level 1',
            columns: ['total', 'expended', 'remaining'],
          ),
        }),
      };

      final row = buildSections(fields).single.rows.single;

      expect(row.group!.label, 'Level 1');
      expect(row.fields, hasLength(3));
    });

    test('Edge: a group without columns stays stacked rows', () {
      final fields = {
        'stats': groupOf({
          'identity': tableGroup(
            {
              'dexterity': leaf('number'),
              'strength': leaf('number'),
            },
          ),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(2));
      expect(section.rows.every((row) => row.group == null), isTrue);
      expect(
        section.rows.map((row) => row.fields.single.path).toList(),
        ['stats.identity.dexterity', 'stats.identity.strength'],
      );
    });

    test('Edge: an empty column list stays stacked rows', () {
      final fields = {
        'stats': groupOf({
          'identity': tableGroup(
            {
              'dexterity': leaf('number'),
              'strength': leaf('number'),
            },
            label: 'Identity',
            columns: const [],
          ),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(2));
      expect(section.rows.every((row) => row.group == null), isTrue);
    });

    test('Edge: a column key missing from the children is ignored', () {
      final fields = {
        'skills': groupOf({
          'acrobatics': tableGroup(
            {'bonus': leaf('number'), 'total': leaf('number')},
            columns: ['bonus', 'ghost', 'total'],
          ),
        }),
      };

      final row = buildSections(fields).single.rows.single;

      expect(
        row.fields.map((field) => field.path).toList(),
        ['skills.acrobatics.bonus', 'skills.acrobatics.total'],
      );
      expect(row.group, isNotNull);
    });

    test('Edge: a column naming a group child is ignored', () {
      final fields = {
        'skills': groupOf({
          'acrobatics': tableGroup(
            {
              'bonus': leaf('number'),
              'nested': groupOf({'total': leaf('number')}),
            },
            columns: ['bonus', 'nested'],
          ),
        }),
      };

      final row = buildSections(fields).single.rows.single;

      expect(row.fields.map((field) => field.path).toList(), [
        'skills.acrobatics.bonus',
      ]);
    });

    test('Edge: a table whose columns all resolve to nothing falls back to stacked rows', () {
      final fields = {
        'skills': groupOf({
          'acrobatics': tableGroup(
            {'bonus': leaf('number')},
            columns: ['ghost', 'phantom'],
          ),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(1));
      expect(section.rows.single.group, isNull);
      expect(
        section.rows.single.fields.single.path,
        'skills.acrobatics.bonus',
      );
    });

    test('Edge: a leaf alongside a table keeps its own normal row', () {
      final fields = {
        'spells': groupOf({
          'attackBonus': leaf('number', editable: false),
          'atWill': leaf('text'),
          'slot1': tableGroup(
            {
              'total': leaf('number'),
              'remaining': leaf('number', editable: false),
            },
            label: 'Level 1',
            columns: ['total', 'remaining'],
          ),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(3));
      expect(section.rows[0].group, isNull);
      expect(section.rows[0].fields.single.path, 'spells.attackBonus');
      expect(section.rows[1].group, isNull);
      expect(section.rows[1].fields.single.path, 'spells.atWill');
      expect(section.rows[2].group, isNotNull);
    });

    test('Edge: table rows leave the flattened section leaves intact', () {
      final fields = {
        'skills': groupOf({
          'acrobatics': tableGroup(
            {'bonus': leaf('number'), 'total': leaf('number')},
            columns: ['bonus', 'total'],
          ),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.leaves.map((field) => field.path).toList(), [
        'skills.acrobatics.bonus',
        'skills.acrobatics.total',
      ]);
    });

    test('Edge: an empty declared label humanizes the group key', () {
      final fields = {
        'skills': groupOf({
          'acrobatics': tableGroup(
            {'bonus': leaf('number')},
            label: '   ',
            columns: ['bonus'],
          ),
        }),
      };

      expect(
        buildSections(fields).single.rows.single.group!.label,
        'Acrobatics',
      );
    });

    test('Edge: a table row coexists with an exclusive group row', () {
      final fields = {
        'config': groupOf({
          'acUsesDex': leaf('number', exclusiveGroup: 'armorClassAbility'),
          'acUsesCon': leaf('number', exclusiveGroup: 'armorClassAbility'),
          'slot1': tableGroup(
            {'total': leaf('number')},
            label: 'Level 1',
            columns: ['total'],
          ),
        }),
      };

      final section = buildSections(fields).single;

      expect(section.rows, hasLength(2));
      expect(section.rows[0].exclusive?.id, 'armorClassAbility');
      expect(section.rows[1].group!.label, 'Level 1');
    });
  });

  group('column labels (RND-21)', () {
    test('Happy path: column_label wins over the schema label', () {
      final schema = FieldSchema(
        type: 'number',
        label: 'Acrobatics bonus',
        isEditable: true,
        defaultValue: 0,
        columnLabel: 'Bonus',
      );

      expect(fieldColumnName('bonus', schema), 'Bonus');
    });

    test('Edge: an empty column_label falls back to the schema label', () {
      final schema = FieldSchema(
        type: 'number',
        label: 'Acrobatics bonus',
        isEditable: true,
        defaultValue: 0,
      );

      expect(fieldColumnName('bonus', schema), 'Acrobatics bonus');
    });

    test('Edge: a blank label falls back to the humanized key', () {
      final schema = FieldSchema(
        type: 'number',
        label: '  ',
        isEditable: true,
        defaultValue: 0,
      );

      expect(fieldColumnName('baseArmorClass', schema), 'Base armor class');
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

    test(
      'Edge: equal group ids separated by an untagged leaf merge into one exclusive row at the first member (EXC-09)',
      () {
        final fields = {
          'config': groupOf({
            'a': leaf('number', exclusiveGroup: 'groupA'),
            'b': leaf('number'),
            'c': leaf('number', exclusiveGroup: 'groupA'),
          }),
        };

        final section = buildSections(fields).single;

        expect(section.rows, hasLength(2));
        final exclusive = section.rows.first.exclusive;
        expect(exclusive, isNotNull);
        expect(exclusive!.id, 'groupA');
        expect(
          exclusive.options.map((option) => option.path).toList(),
          ['config.a', 'config.c'],
        );
        expect(section.rows.last.exclusive, isNull);
        expect(section.rows.last.fields.single.path, 'config.b');
      },
    );

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
