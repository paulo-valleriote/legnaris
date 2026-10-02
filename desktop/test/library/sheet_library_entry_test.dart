import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/library/sheet_library_entry.dart';

/// Unit tests for the persisted sheet index entry (LIB-02, LIB-03, LIB-08,
/// LIB-20). Every asserted value comes from the spec, never from the
/// implementation.
void main() {
  const validJson = <String, Object?>{
    'id': '01920f3a-0000-7000-8000-000000000000',
    'name': 'Aria',
    'template_id': 'fantasy_v1',
    'created_at': '2026-10-02T15:31:28.401Z',
  };

  group('fromJson', () {
    test('Happy path: reads id, name, template_id and created_at (LIB-02)', () {
      final entry = SheetLibraryEntry.fromJson(validJson);

      expect(entry.id, '01920f3a-0000-7000-8000-000000000000');
      expect(entry.name, 'Aria');
      expect(entry.templateId, 'fantasy_v1');
      expect(entry.createdAt, DateTime.parse('2026-10-02T15:31:28.401Z'));
    });

    test('Error: a missing required field is a FormatException', () {
      for (final key in validJson.keys) {
        final partial = Map<String, Object?>.from(validJson)..remove(key);

        expect(
          () => SheetLibraryEntry.fromJson(partial),
          throwsFormatException,
          reason: 'missing $key must be rejected',
        );
      }
    });

    test('Error: a wrong-typed required field is a FormatException', () {
      for (final key in validJson.keys) {
        final wrongTyped = Map<String, Object?>.from(validJson)..[key] = 42;

        expect(
          () => SheetLibraryEntry.fromJson(wrongTyped),
          throwsFormatException,
          reason: '$key must be a string',
        );
      }
    });

    test('Error: a non-ISO created_at is a FormatException', () {
      expect(
        () => SheetLibraryEntry.fromJson({
          ...validJson,
          'created_at': 'not-a-timestamp',
        }),
        throwsFormatException,
      );
    });
  });

  group('toJson', () {
    test('Happy path: emits exactly the four persisted keys and no data (LIB-08)', () {
      final entry = SheetLibraryEntry.fromJson(validJson);

      final json = entry.toJson();

      expect(json.keys.toSet(), {'id', 'name', 'template_id', 'created_at'});
      expect(json.containsKey('data'), isFalse);
    });

    test('Happy path: writes created_at as ISO-8601 UTC (LIB-03)', () {
      final utcEntry = SheetLibraryEntry(
        id: 'id-1',
        name: 'Aria',
        templateId: 'fantasy_v1',
        createdAt: DateTime.utc(2026, 10, 2, 15, 31, 28, 401),
      );
      expect(utcEntry.toJson()['created_at'], '2026-10-02T15:31:28.401Z');

      final localCreatedAt = DateTime(2026, 10, 2, 15, 31, 28, 401);
      final stored = SheetLibraryEntry(
        id: 'id-1',
        name: 'Aria',
        templateId: 'fantasy_v1',
        createdAt: localCreatedAt,
      ).toJson()['created_at'] as String;

      expect(stored.endsWith('Z'), isTrue);
      expect(DateTime.parse(stored).isAtSameMomentAs(localCreatedAt), isTrue);
    });
  });

  group('isValid (LIB-20)', () {
    test('Edge: false when id, name or template_id is empty, true otherwise', () {
      final valid = SheetLibraryEntry.fromJson(validJson);
      expect(valid.isValid, isTrue);

      for (final (id, name, templateId) in [
        ('', 'Aria', 'fantasy_v1'),
        ('id-1', '', 'fantasy_v1'),
        ('id-1', 'Aria', ''),
      ]) {
        final entry = SheetLibraryEntry(
          id: id,
          name: name,
          templateId: templateId,
          createdAt: DateTime.parse('2026-10-02T15:31:28.401Z'),
        );
        expect(entry.isValid, isFalse, reason: '($id, $name, $templateId)');
      }
    });
  });

  test('Happy path: JSON round-trip preserves every field', () {
    final original = SheetLibraryEntry.fromJson(validJson);

    final restored = SheetLibraryEntry.fromJson(original.toJson());

    expect(restored.id, original.id);
    expect(restored.name, original.name);
    expect(restored.templateId, original.templateId);
    expect(restored.createdAt, original.createdAt);
  });
}
