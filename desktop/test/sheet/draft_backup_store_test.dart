import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';

/// Unit tests for the atomic draft backup store (DRB-04, DRB-05, DRB-12,
/// DRB-14). Real temp-directory I/O, no platform channel.
void main() {
  const sheetId = '12345678-1234-7000-8000-123456789abc';
  late Directory root;

  setUp(() {
    root = Directory.systemTemp.createTempSync('draft_backup_store_');
    addTearDown(() => root.deleteSync(recursive: true));
  });

  DraftBackupStore newStore({DateTime Function()? now}) =>
      DraftBackupStore(directory: root, now: now);

  File draftFile(String sheetId) => File('${root.path}/drafts/$sheetId.json');

  File tempFile(String sheetId) => File('${draftFile(sheetId).path}.$pid.tmp');

  test('Happy path: write then read returns the same nested delta (DRB-04, DRB-05)', () {
    final store = newStore();
    final delta = {
      'stats': {'dexterity': 16},
      'config': {'acUsesDex': 0},
    };

    store.write(sheetId, delta);

    expect(store.read(sheetId), delta);
  });

  test('Happy path: writes the versioned envelope under <appSupport>/drafts/<sheetId>.json (DRB-14)', () {
    final frozen = DateTime.utc(2026, 10, 4, 13);
    newStore(now: () => frozen).write(sheetId, {'stats': {'dexterity': 16}});

    expect(draftFile(sheetId).existsSync(), isTrue);
    final envelope =
        jsonDecode(draftFile(sheetId).readAsStringSync())
            as Map<String, Object?>;
    expect(envelope['schema_version'], 1);
    expect(envelope['sheet_id'], sheetId);
    expect(envelope['delta'], {'stats': {'dexterity': 16}});
    expect(envelope['saved_at'], '2026-10-04T13:00:00.000Z');
  });

  test('Edge: read returns null when the file is missing (DRB-12)', () {
    expect(newStore().read(sheetId), isNull);
  });

  test('Edge: a corrupt file reads as null (DRB-12)', () {
    final file = draftFile(sheetId);
    file.parent.createSync(recursive: true);
    file.writeAsStringSync('{ not json');

    expect(newStore().read(sheetId), isNull);
  });

  test('Edge: a non-object root or a delta-less envelope reads as null (DRB-12)', () {
    final file = draftFile(sheetId);
    file.parent.createSync(recursive: true);

    file.writeAsStringSync('[1, 2, 3]');
    expect(newStore().read(sheetId), isNull);

    file.writeAsStringSync('{"schema_version": 1, "sheet_id": "$sheetId"}');
    expect(newStore().read(sheetId), isNull);
  });

  test('Edge: a newer schema version reads as null (DRB-12)', () {
    final file = draftFile(sheetId);
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(
      '{"schema_version": 2, "sheet_id": "$sheetId", '
      '"delta": {"stats": {"dexterity": 16}}, '
      '"saved_at": "2026-10-04T13:00:00.000Z"}',
    );

    expect(newStore().read(sheetId), isNull);
  });

  test('Error: a failed write throws and leaves the prior file intact (DRB-04)', () {
    final store = newStore();
    store.write(sheetId, {'stats': {'dexterity': 16}});
    final before = draftFile(sheetId).readAsStringSync();
    Directory(tempFile(sheetId).path).createSync(recursive: true);

    expect(
      () => store.write(sheetId, {'stats': {'dexterity': 18}}),
      throwsA(isA<DraftBackupException>()),
    );
    expect(draftFile(sheetId).readAsStringSync(), before);
    expect(store.read(sheetId), {'stats': {'dexterity': 16}});
  });

  test('Edge: delete removes the file and is a no-op when absent (DRB-14)', () {
    final store = newStore();
    store.write(sheetId, {'stats': {'dexterity': 16}});
    expect(draftFile(sheetId).existsSync(), isTrue);

    store.delete(sheetId);
    expect(draftFile(sheetId).existsSync(), isFalse);
    expect(store.read(sheetId), isNull);

    expect(() => store.delete(sheetId), returnsNormally);
  });

  test('Edge: accepts a whole-delta write when no drafts directory exists yet', () {
    expect(Directory('${root.path}/drafts').existsSync(), isFalse);

    newStore().write(sheetId, {'stats': {'dexterity': 16}});

    expect(Directory('${root.path}/drafts').existsSync(), isTrue);
    expect(newStore().read(sheetId), {'stats': {'dexterity': 16}});
  });

  test('Edge: read of an invalid sheet id resolves null without touching the filesystem', () {
    final store = newStore();

    expect(store.read('../escape'), isNull);
    expect(store.read(''), isNull);
    expect(store.read('NOT-A-UUID'), isNull);
    expect(Directory('${root.path}/drafts').existsSync(), isFalse);
  });

  test('Edge: delete of an invalid sheet id is a no-op', () {
    expect(() => newStore().delete('../escape'), returnsNormally);
    expect(() => newStore().delete(''), returnsNormally);
    expect(Directory('${root.path}/drafts').existsSync(), isFalse);
  });

  test('Error: write of an invalid sheet id throws DraftBackupException, not an Error', () {
    final store = newStore();

    expect(
      () => store.write('../escape', {'stats': {'dexterity': 16}}),
      throwsA(isA<DraftBackupException>()),
    );
    expect(Directory('${root.path}/drafts').existsSync(), isFalse);
  });

  test('Happy path: clearAll deletes every stored backup and read returns null for each (DATA-01)', () {
    const otherId = 'abcdefab-cdef-7000-8000-abcdefabcdef';
    final store = newStore();
    store.write(sheetId, {'stats': {'dexterity': 16}});
    store.write(otherId, {'config': {'acUsesDex': 1}});
    expect(store.read(sheetId), isNotNull);
    expect(store.read(otherId), isNotNull);

    store.clearAll();

    expect(store.read(sheetId), isNull);
    expect(store.read(otherId), isNull);
    final files = Directory('${root.path}/drafts')
        .listSync()
        .whereType<File>()
        .toList();
    expect(files, isEmpty);
  });

  test('Edge: clearAll on a missing drafts directory is a no-op (DATA-03)', () {
    final store = newStore();

    expect(store.clearAll, returnsNormally);
    expect(Directory('${root.path}/drafts').existsSync(), isFalse);
    expect(store.read(sheetId), isNull);
  });

  test('Edge: clearAll skips a subdirectory entry and still deletes the files (DATA-03)', () {
    final store = newStore();
    store.write(sheetId, {'stats': {'dexterity': 16}});
    final nested = Directory('${root.path}/drafts/keep-me')
      ..createSync(recursive: true);
    final nestedFile = File('${nested.path}/nested.json')
      ..writeAsStringSync('{}');

    store.clearAll();

    expect(store.read(sheetId), isNull);
    expect(nested.existsSync(), isTrue, reason: 'non-file entries are skipped');
    expect(nestedFile.existsSync(), isTrue);
  });

  test('Edge: a drafts path that exists as a file never throws (DATA-03)', () {
    final draftsPath = '${root.path}/drafts';
    final blocker = File(draftsPath)..writeAsStringSync('not a directory');

    expect(newStore().clearAll, returnsNormally);
    expect(blocker.existsSync(), isTrue);
  });
}
