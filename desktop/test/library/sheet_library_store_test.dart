import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_store.dart';

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

  SheetLibraryEntry entry(
    String id, {
    String name = 'Aria',
    String templateId = 'fantasy_v1',
    String createdAt = '2026-10-02T15:31:28.401Z',
  }) => SheetLibraryEntry(
    id: id,
    name: name,
    templateId: templateId,
    createdAt: DateTime.parse(createdAt),
  );

  SheetLibraryStore newStore() => SheetLibraryStore(directory: storeDir);

  File storeFile() => File('${storeDir.path}/sheet_library.json');

  File tempFile() => File('${storeDir.path}/sheet_library.json.tmp');

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
}
