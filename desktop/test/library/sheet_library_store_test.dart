import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_store.dart';

import 'library_fixtures.dart';

/// Unit tests for the atomic save path of the local sheet library store
/// (LIB-09 … LIB-13, LIB-20). Real temp-directory I/O, no platform channel.
void main() {
  late Directory root;
  late Directory storeDir;

  setUp(() {
    root = Directory.systemTemp.createTempSync('sheet_library_store_');
    storeDir = Directory('${root.path}/app_support');
    addTearDown(() => root.deleteSync(recursive: true));
  });

  SheetLibraryStore newStore() => SheetLibraryStore(directory: storeDir);

  File storeFile() => File('${storeDir.path}/sheet_library.json');

  File tempFile() => File('${storeDir.path}/sheet_library.json.$pid.tmp');

  Map<String, Object?> readEnvelope() =>
      jsonDecode(storeFile().readAsStringSync()) as Map<String, Object?>;

  test('Happy path: writes a parseable {schema_version, sheets} with exactly the four entry fields (LIB-09)', () {
    newStore().save([entry('id-1')]);

    final envelope = readEnvelope();
    expect(envelope['schema_version'], 1);
    final sheets = envelope['sheets'] as List<Object?>;
    expect(sheets, hasLength(1));
    final first = sheets.single as Map<String, Object?>;
    expect(first.keys.toSet(), {'id', 'name', 'template_id', 'created_at'});
    expect(first['id'], 'id-1');
    expect(first['name'], 'Aria');
    expect(first['template_id'], 'fantasy_v1');
    expect(first['created_at'], '2026-10-02T15:31:28.401Z');
  });

  test('Edge: creates the OS store directory on the first write', () {
    expect(storeDir.existsSync(), isFalse);

    newStore().save([entry('id-1')]);

    expect(storeDir.existsSync(), isTrue);
    expect(storeFile().existsSync(), isTrue);
  });

  test('Error: an invalid entry throws invalidEntry and writes nothing (LIB-20)', () {
    final store = newStore();

    expect(
      () => store.save([entry('id-1', name: '')]),
      throwsA(
        isA<SheetLibraryException>().having(
          (error) => error.kind,
          'kind',
          SheetLibraryErrorKind.invalidEntry,
        ),
      ),
    );
    expect(storeFile().existsSync(), isFalse);
  });

  test('Error: a filesystem failure throws writeFailed and leaves the previous content intact (LIB-11)', () {
    final store = newStore();
    store.save([entry('id-1', name: 'Aria')]);
    final before = storeFile().readAsStringSync();
    Directory(tempFile().path).createSync(recursive: true);

    expect(
      () => store.save([entry('id-1', name: 'Changed')]),
      throwsA(
        isA<SheetLibraryException>().having(
          (error) => error.kind,
          'kind',
          SheetLibraryErrorKind.writeFailed,
        ),
      ),
    );
    expect(storeFile().readAsStringSync(), before);
  });

  test('Happy path: after a successful save the temp file is gone and the target holds the new content (LIB-12)', () {
    final store = newStore();
    store.save([entry('id-1', name: 'First')]);

    store.save([entry('id-1', name: 'Second')]);

    expect(tempFile().existsSync(), isFalse);
    final sheets = readEnvelope()['sheets'] as List<Object?>;
    expect(sheets, hasLength(1));
    expect((sheets.single as Map<String, Object?>)['name'], 'Second');
  });

  test('Happy path: a second save replaces the whole index with no leftover entries (LIB-12)', () {
    final store = newStore();
    store.save([entry('id-1'), entry('id-2')]);

    store.save([entry('id-2')]);

    final sheets = readEnvelope()['sheets'] as List<Object?>;
    expect(sheets, hasLength(1));
    expect((sheets.single as Map<String, Object?>)['id'], 'id-2');
  });

  test('Edge: saving an empty index persists an empty sheets array', () {
    newStore().save(const []);

    final envelope = readEnvelope();
    expect(envelope['schema_version'], 1);
    expect(envelope['sheets'], isEmpty);
  });

  test('Edge: the persisted sheets keep the supplied order for a deterministic reload (LIB-04)', () {
    newStore().save([entry('id-1'), entry('id-2'), entry('id-3')]);

    final ids = (readEnvelope()['sheets'] as List<Object?>)
        .map((raw) => (raw as Map<String, Object?>)['id'])
        .toList();
    expect(ids, ['id-1', 'id-2', 'id-3']);
  });

  test('Happy path: an absent store loads empty; a saved index round-trips through a fresh store (LIB-01, LIB-14)', () {
    final absent = newStore().load();
    expect(absent.outcome, LibraryLoadOutcome.empty);
    expect(absent.entries, isEmpty);

    newStore().save([entry('id-1', name: 'Aria'), entry('id-2', name: 'Borin')]);

    final loaded = newStore().load();
    expect(loaded.outcome, LibraryLoadOutcome.loaded);
    expect(loaded.entries.map((stored) => stored.id), ['id-1', 'id-2']);
    expect(loaded.entries.first.name, 'Aria');
  });

  test('Error: a corrupt or malformed store is quarantined with its original bytes (LIB-17)', () {
    final variants = <String, String>{
      'zero bytes': '',
      'invalid JSON': '{ not json',
      'non-object root': '[1, 2, 3]',
      'non-integer version': '{"schema_version": "1", "sheets": []}',
      'sheets not a list': '{"schema_version": 1, "sheets": {}}',
      'entry not an object': '{"schema_version": 1, "sheets": [42]}',
      'entry missing a field':
          '{"schema_version": 1, "sheets": [{"id": "a", "name": "b"}]}',
      'entry invalid':
          '{"schema_version": 1, "sheets": [{"id": "", "name": "b", "template_id": "t", "created_at": "2026-10-02T15:31:28.401Z"}]}',
    };

    for (final MapEntry(key: label, value: body) in variants.entries) {
      storeDir.createSync(recursive: true);
      storeFile().writeAsStringSync(body);

      final result = newStore().load();

      expect(result.outcome, LibraryLoadOutcome.quarantined, reason: label);
      expect(result.entries, isEmpty, reason: label);
      final quarantined = storeDir
          .listSync()
          .whereType<File>()
          .where((file) => file.path.contains('.corrupt-'))
          .toList();
      expect(quarantined, hasLength(1), reason: label);
      expect(quarantined.single.readAsStringSync(), body, reason: label);
      storeDir.deleteSync(recursive: true);
    }
  });

  test('Error: when the quarantine rename fails, load reports empty and the file is left for the next save (LIB-17, LIB-19)', () {
    storeDir.createSync(recursive: true);
    storeFile().writeAsStringSync('not json');
    final frozen = DateTime.utc(2026, 10, 3, 12);
    final stamp = frozen
        .toUtc()
        .toIso8601String()
        .replaceAll(RegExp(r'[^0-9]'), '');
    Directory('${storeFile().path}.corrupt-$stamp').createSync();

    final store = SheetLibraryStore(directory: storeDir, now: () => frozen);
    final result = store.load();

    expect(result.outcome, LibraryLoadOutcome.empty);
    expect(result.entries, isEmpty);
    expect(storeFile().readAsStringSync(), 'not json');

    store.save([entry('id-1')]);
    expect(storeFile().readAsStringSync(), contains('"id-1"'));
  });

  test('Happy path: an older version with a registered migration upgrades; without one it is quarantined (LIB-15, LIB-16)', () {
    const older =
        '{"schema_version": 0, "sheets": [{"id": "id-1", "name": "Aria", "template_id": "fantasy_v1", "created_at": "2026-10-02T15:31:28.401Z"}]}';
    storeDir.createSync(recursive: true);

    storeFile().writeAsStringSync(older);
    final upgraded = SheetLibraryStore(
      directory: storeDir,
      migrations: {
        0: (envelope) => {
          'schema_version': 1,
          'sheets': envelope['sheets'],
        },
      },
    ).load();
    expect(upgraded.outcome, LibraryLoadOutcome.loaded);
    expect(upgraded.entries.single.id, 'id-1');

    storeFile().writeAsStringSync(older);
    final noMigration = newStore().load();
    expect(noMigration.outcome, LibraryLoadOutcome.quarantined);
    expect(noMigration.entries, isEmpty);
  });

  test('Edge: a migrator raising FormatException is quarantined like any other bad input (LIB-17)', () {
    storeDir.createSync(recursive: true);
    storeFile().writeAsStringSync('{"schema_version": 0, "sheets": []}');

    final result = SheetLibraryStore(
      directory: storeDir,
      migrations: {
        0: (envelope) => throw const FormatException('unmigratable envelope'),
      },
    ).load();

    expect(result.outcome, LibraryLoadOutcome.quarantined);
    expect(result.entries, isEmpty);
  });

  test('Edge: a newer schema_version is quarantined without parsing its entries (LIB-18)', () {
    storeDir.createSync(recursive: true);
    storeFile().writeAsStringSync('{"schema_version": 2, "sheets": [42]}');

    final result = newStore().load();

    expect(result.outcome, LibraryLoadOutcome.quarantined);
    expect(result.entries, isEmpty);
  });

  test('Edge: a leftover temp file is deleted on load and does not change the outcome (LIB-14)', () {
    newStore().save([entry('id-1')]);
    tempFile().writeAsStringSync('partial garbage');

    final result = newStore().load();

    expect(result.outcome, LibraryLoadOutcome.loaded);
    expect(result.entries.single.id, 'id-1');
    expect(tempFile().existsSync(), isFalse);
  });

  test('Edge: per-process temps — save never shares another instance file and load sweeps stale temps (LIB-12, LIB-14)', () {
    storeDir.createSync(recursive: true);
    final foreignTemp = File(
      '${storeDir.path}/sheet_library.json.999999999.tmp',
    );
    foreignTemp.writeAsStringSync('in-flight write of another process');

    final writer = newStore();
    writer.save([entry('id-1')]);

    expect(foreignTemp.existsSync(), isTrue);
    expect(tempFile().existsSync(), isFalse);

    final reader = newStore();
    reader.save([entry('id-1'), entry('id-2')]);
    final loaded = reader.load();

    expect(loaded.outcome, LibraryLoadOutcome.loaded);
    expect(loaded.entries.map((item) => item.id), ['id-1', 'id-2']);
    expect(foreignTemp.existsSync(), isFalse);
  });
}
