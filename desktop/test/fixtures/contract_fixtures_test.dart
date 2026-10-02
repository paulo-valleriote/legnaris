import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/api/models/template.dart';

/// Suite that runs the typed client against the real responses pinned in
/// `test/fixtures/` — no reachable API required (FE1-32). Every fixture was
/// captured from the local backend; a fixture whose envelope or field shape
/// drifts from what the models expect fails this suite (FE1-34).
void main() {
  /// Reads a pinned fixture; `flutter test` runs from the package root.
  String fixtureBody(String name) {
    final file = File('test/fixtures/$name');
    expect(file.existsSync(), isTrue, reason: 'missing pinned fixture: $name');
    return file.readAsStringSync();
  }

  /// Client whose injected transport answers with the fixture at [status] —
  /// the suite never touches the network (FE1-32).
  ApiClient fixtureClient(String body, {int status = 200}) => ApiClient(
    httpClient: MockClient((_) async => http.Response(body, status)),
  );

  /// Every leaf of the tree carrying a non-empty `option_label`, keyed by its
  /// dotted path, so a new tag anywhere fails the exact-set assertion.
  Map<String, String> optionLabels(FieldGroup fields) {
    final labels = <String, String>{};
    void walk(String path, FieldNode node) {
      final schema = node.schema;
      if (schema != null) {
        final label = schema.optionLabel;
        if (label != null && label.isNotEmpty) labels[path] = label;
        return;
      }
      final children = node.children;
      if (children == null) return;
      for (final entry in children.entries) {
        walk(path.isEmpty ? entry.key : '$path.${entry.key}', entry.value);
      }
    }
    for (final entry in fields.entries) {
      walk(entry.key, entry.value);
    }
    return labels;
  }

  group('pinned contract fixtures (FE1-32, FE1-33, FE1-34)', () {
    test('Fixture set: the 6 endpoints, the error envelope and the PATCH delta pair are all pinned (FE1-33)', () {
      const required = [
        'health.json', // GET /health
        'templates.json', // GET /templates
        'template_detail.json', // GET /templates/:id
        'sheet_create.json', // POST /sheets
        'sheet_get.json', // GET /sheets/:id
        'patch_delta_request.json', // PATCH /sheets/:id — captured request
        'patch_delta_response.json', // PATCH /sheets/:id — captured response
        'error_not_found.json', // error envelope
      ];

      for (final name in required) {
        final file = File('test/fixtures/$name');
        expect(file.existsSync(), isTrue, reason: 'missing pinned fixture: $name');
        expect(
          () => jsonDecode(file.readAsStringSync()),
          returnsNormally,
          reason: '$name must stay valid JSON — it is a real captured body',
        );
      }
    });

    test('Happy path: GET /health fixture decodes through the client (FE1-32)', () async {
      final client = fixtureClient(fixtureBody('health.json'));

      expect(await client.health(), 'healthy');
    });

    test('Happy path: GET /templates fixture decodes the real catalog', () async {
      final client = fixtureClient(fixtureBody('templates.json'));

      final items = await client.listTemplates();
      expect(items, hasLength(1));

      final fantasy = items.single;
      expect(fantasy.id, 'fantasy_v1');
      expect(fantasy.name, 'Fantasy Template');
      expect(fantasy.isDefault, isTrue, reason: 'the reserved JSON key "default" must map explicitly');
      expect(fantasy.fields, hasLength(11), reason: 'the sheet serves 11 sections');
      expect(fantasy.fields['stats']?.schema, isNull, reason: 'a group exposes children, not a schema');
      expect(fantasy.fields['stats']?.children?.keys, containsAll(['dexterity', 'constitution']));
    });

    test('Happy path: GET /templates/:id fixture preserves the leaf/group tree', () async {
      final client = fixtureClient(fixtureBody('template_detail.json'));

      final template = await client.getTemplate('fantasy_v1');
      expect(template.id, 'fantasy_v1');
      expect(template.isDefault, isTrue);

      final dexterity = template.fields['stats']?.children?['dexterity']?.schema;
      expect(dexterity, isNotNull, reason: 'a leaf exposes its schema');
      expect(dexterity?.type, 'number');
      expect(dexterity?.isEditable, isTrue);
      expect(dexterity?.defaultValue, 10);
      expect(dexterity?.formula, isNull, reason: 'formula is omitted for editable fields');
      expect(dexterity?.dependsOn, isNull, reason: 'depends_on is omitted for editable fields');

      final armorClass = template.fields['derived']?.children?['armorClass']?.schema;
      expect(armorClass, isNotNull);
      expect(armorClass?.isEditable, isFalse, reason: 'derived fields are read-only');
      expect(armorClass?.formula, contains('baseArmorClass'));
      expect(armorClass?.dependsOn?.first, ['combat', 'baseArmorClass']);
    });

    test(
      'Happy path: the frozen fixtures tag exactly the two exclusive groups and expose no spells.ability (EXC-17, EXC-18, EXC-19)',
      () async {
        final detailClient = fixtureClient(fixtureBody('template_detail.json'));
        final detail = await detailClient.getTemplate('fantasy_v1');
        final catalogClient = fixtureClient(fixtureBody('templates.json'));
        final catalog = (await catalogClient.listTemplates()).single;

        for (final template in [detail, catalog]) {
          final config = template.fields['config']!.children!;
          final groups = <String, Set<String>>{};
          for (final entry in config.entries) {
            final id = entry.value.schema?.exclusiveGroup;
            if (id == null || id.isEmpty) continue;
            groups.putIfAbsent(id, () => <String>{}).add(entry.key);
          }

          expect(groups, {
            'armorClassAbility': {'acUsesDex', 'acUsesCon'},
            'spellcastingAbility': {
              'spellUsesCharisma',
              'spellUsesConstitution',
              'spellUsesDexterity',
              'spellUsesIntelligence',
              'spellUsesStrength',
              'spellUsesWisdom',
            },
          });
          expect(
            template.fields['spells']?.children?.containsKey('ability'),
            isNot(true),
            reason: 'the redundant spells.ability leaf is removed (EXC-19)',
          );
        }
      },
    );

    test(
      'Happy path: option_label is present on exactly the ac pair + six spellUses and nowhere else (EXC-25, EXC-26)',
      () async {
        final detail = await fixtureClient(
          fixtureBody('template_detail.json'),
        ).getTemplate('fantasy_v1');
        final catalog = (await fixtureClient(
          fixtureBody('templates.json'),
        ).listTemplates()).single;

        const expected = {
          'config.acUsesDex': 'Dexterity',
          'config.acUsesCon': 'Constitution',
          'config.spellUsesStrength': 'Strength',
          'config.spellUsesDexterity': 'Dexterity',
          'config.spellUsesConstitution': 'Constitution',
          'config.spellUsesIntelligence': 'Intelligence',
          'config.spellUsesWisdom': 'Wisdom',
          'config.spellUsesCharisma': 'Charisma',
        };

        for (final template in [detail, catalog]) {
          expect(optionLabels(template.fields), expected);
        }
      },
    );

    test('Happy path: POST /sheets fixture decodes the created sheet with computed values', () async {      final client = fixtureClient(fixtureBody('sheet_create.json'), status: 201);

      final sheet = await client.createSheet(
        templateId: 'fantasy_v1',
        sheetName: 'Fixture Capture',
      );
      expect(sheet.id, isNotEmpty);
      expect(sheet.templateId, 'fantasy_v1');
      expect(sheet.name, 'Fixture Capture');
      expect(sheet.valueAt('stats.dexterity'), 10, reason: 'editable default seeded as int');
      expect(
        sheet.valueAt('identity.characterName'),
        '',
        reason: 'text leaves seed as an empty string, never at the numeric default (SeedValue)',
      );
      expect(
        sheet.valueAt('derived.armorClass'),
        10,
        reason: 'derived value computed at creation: combat.baseArmorClass 10 + dexMod 0',
      );
    });

    test('Happy path: GET /sheets/:id fixture decodes the stored sheet', () async {
      final client = fixtureClient(fixtureBody('sheet_get.json'));

      final sheet = await client.getSheet('7a6fe174-a2b2-4248-9662-3d441d404bef');
      expect(sheet.templateId, 'fantasy_v1');
      expect(sheet.createdAt, isNotNull);
      expect(sheet.updatedAt, isNotNull);
      expect(sheet.valueAt('stats.dexterity'), 10);
      expect(sheet.valueAt('modifiers.dexterityMod'), 0, reason: 'derived value stored with the sheet');
      expect(sheet.valueAt('missing.path'), isNull, reason: 'missing data resolves to null');
    });

    test('Happy path: PATCH delta pair — documented request, cascade-only response', () async {
      final request = jsonDecode(fixtureBody('patch_delta_request.json'));
      expect(request, {
        'data': {
          'stats': {'dexterity': 16},
        },
      });

      final client = fixtureClient(fixtureBody('patch_delta_response.json'));
      final cascades = await client.patchSheet('sheet-id', {
        'stats': {'dexterity': 16},
      });

      for (final key in ['id', 'template_id', 'name', 'created_at', 'stats']) {
        expect(
          cascades.containsKey(key),
          isFalse,
          reason: 'the PATCH response is cascade-only — "$key" must not appear',
        );
      }
      expect((cascades['modifiers'] as Map<String, Object?>)['dexterityMod'], 3);
      expect((cascades['derived'] as Map<String, Object?>)['armorClass'], 13);
      expect(
        ((cascades['skills'] as Map<String, Object?>)['stealth'] as Map<String, Object?>)['total'],
        3,
        reason: 'the skill total cascades from the new dexterity modifier',
      );
    });

    test('Error: 404 fixture carries status, code, server message and the notFound kind', () async {
      final client = fixtureClient(fixtureBody('error_not_found.json'), status: 404);

      try {
        await client.getSheet('00000000-0000-4000-8000-000000000000');
        fail('expected ApiDomainException for the 404 fixture');
      } on ApiDomainException catch (error) {
        expect(error.statusCode, 404);
        expect(error.code, 'NOT_FOUND');
        expect(error.message, 'sheet not found');
        expect(error.kind, ApiErrorKind.notFound);
      }
    });
  });
}
