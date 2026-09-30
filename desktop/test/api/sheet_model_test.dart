import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/models/sheet.dart';

void main() {
  const sheetJson = '''
{
  "id": "sheet-1",
  "template_id": "fantasy_v1",
  "name": "Aragorn",
  "data": {
    "stats": {"dexterity": 16, "constitution": 16.5},
    "modifiers": {"dexterityMod": 3},
    "tags": ["hero", "ranger"],
    "config": {"acUsesDex": 1}
  },
  "created_at": "2026-09-20T12:00:00Z",
  "updated_at": "2026-09-21T08:30:00Z"
}
''';

  Sheet decodeSheet() =>
      Sheet.fromJson(jsonDecode(sheetJson) as Map<String, Object?>);

  group('sheet decode (FE1-10, FE1-11)', () {
    test('Happy path: sheet decodes every contract field', () {
      // FE1-10.
      final sheet = decodeSheet();
      expect(sheet.id, 'sheet-1');
      expect(sheet.templateId, 'fantasy_v1');
      expect(sheet.name, 'Aragorn');
      expect(sheet.createdAt, DateTime.parse('2026-09-20T12:00:00Z'));
      expect(sheet.updatedAt, DateTime.parse('2026-09-21T08:30:00Z'));
    });

    test('Happy path: data keeps its nested structure', () {
      // FE1-10: dynamic nesting is preserved for the UI Engine.
      final data = decodeSheet().data;
      expect(data['stats'], isA<Map<String, Object?>>());
      expect((data['stats'] as Map<String, Object?>)['dexterity'], 16);
      expect(data['tags'], isA<List<Object?>>());
      expect(data['tags'], ['hero', 'ranger']);
      expect(data['config'], isA<Map<String, Object?>>());
    });

    test('Edge: integers stay int and doubles stay double', () {
      // FE1-11: 16 remains int, 16.5 remains double — no truncation, no
      // forced cast.
      final data = decodeSheet().data;
      final stats = data['stats'] as Map<String, Object?>;

      final intValue = stats['dexterity'];
      expect(intValue, isA<int>());
      expect(intValue, 16);

      final doubleValue = stats['constitution'];
      expect(doubleValue, isA<double>());
      expect(doubleValue, 16.5);
    });
  });

  group('dotted path accessor (FE1-12, FE1-13, FE1-14)', () {
    test('Happy path: a resolvable path returns its value', () {
      // FE1-12, spec example: stats.dexterity.
      final sheet = decodeSheet();
      expect(sheet.valueAt('stats.dexterity'), 16);
      expect(sheet.valueAt('stats'), isA<Map<String, Object?>>());
      expect(sheet.valueAt('tags'), ['hero', 'ranger']);
    });

    test('Edge: a missing segment returns null without throwing', () {
      // FE1-13.
      final sheet = decodeSheet();
      expect(sheet.valueAt('stats.strength'), isNull);
      expect(sheet.valueAt('missing.top'), isNull);
    });

    test('Edge: an intermediate node that is not a map returns null', () {
      // FE1-13: descend past a scalar → null, never throws.
      final sheet = decodeSheet();
      expect(sheet.valueAt('stats.dexterity.x'), isNull);
      expect(sheet.valueAt('tags.first.value'), isNull);
    });

    test('Error: an empty path segment throws a typed client error', () {
      // FE1-14: stats..dexterity.
      final sheet = decodeSheet();
      expect(
        () => sheet.valueAt('stats..dexterity'),
        throwsA(isA<InvalidPathException>()),
      );
      expect(
        () => sheet.valueAt('.stats'),
        throwsA(isA<InvalidPathException>()),
      );
    });

    test('Error: an empty path throws a typed client error', () {
      // FE1-14: "".
      final sheet = decodeSheet();
      expect(
        () => sheet.valueAt(''),
        throwsA(isA<InvalidPathException>()),
      );
    });
  });
}
