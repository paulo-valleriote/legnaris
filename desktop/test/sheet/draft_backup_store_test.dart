import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';

/// Unit tests for the atomic draft backup store (DRB-04, DRB-05, DRB-12,
/// DRB-14). Real temp-directory I/O, no platform channel.
void main() {
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

    store.write('sheet-1', delta);

    expect(store.read('sheet-1'), delta);
  });

  test('Happy path: writes the versioned envelope under <appSupport>/drafts/<sheetId>.json (DRB-14)', () {
    final frozen = DateTime.utc(2026, 10, 4, 13);
    newStore(now: () => frozen).write('sheet-1', {'stats': {'dexterity': 16}});

    expect(draftFile('sheet-1').existsSync(), isTrue);
    final envelope =
        jsonDecode(draftFile('sheet-1').readAsStringSync())
            as Map<String, Object?>;
    expect(envelope['schema_version'], 1);
    expect(envelope['sheet_id'], 'sheet-1');
    expect(envelope['delta'], {'stats': {'dexterity': 16}});
    expect(envelope['saved_at'], '2026-10-04T13:00:00.000Z');
  });

  test('Edge: read returns null when the file is missing (DRB-12)', () {
    expect(newStore().read('sheet-1'), isNull);
  });

  test('Edge: a corrupt file reads as null (DRB-12)', () {
    final file = draftFile('sheet-1');
    file.parent.createSync(recursive: true);
    file.writeAsStringSync('{ not json');

    expect(newStore().read('sheet-1'), isNull);
  });

  test('Edge: a non-object root or a delta-less envelope reads as null (DRB-12)', () {
    final file = draftFile('sheet-1');
    file.parent.createSync(recursive: true);

    file.writeAsStringSync('[1, 2, 3]');
    expect(newStore().read('sheet-1'), isNull);

    file.writeAsStringSync('{"schema_version": 1, "sheet_id": "sheet-1"}');
    expect(newStore().read('sheet-1'), isNull);
  });

  test('Edge: a newer schema version reads as null (DRB-12)', () {
    final file = draftFile('sheet-1');
    file.parent.createSync(recursive: true);
    file.writeAsStringSync(
      '{"schema_version": 2, "sheet_id": "sheet-1", '
      '"delta": {"stats": {"dexterity": 16}}, '
      '"saved_at": "2026-10-04T13:00:00.000Z"}',
    );

    expect(newStore().read('sheet-1'), isNull);
  });

  test('Error: a failed write throws and leaves the prior file intact (DRB-04)', () {
    final store = newStore();
    store.write('sheet-1', {'stats': {'dexterity': 16}});
    final before = draftFile('sheet-1').readAsStringSync();
    Directory(tempFile('sheet-1').path).createSync(recursive: true);

    expect(
      () => store.write('sheet-1', {'stats': {'dexterity': 18}}),
      throwsA(isA<DraftBackupException>()),
    );
    expect(draftFile('sheet-1').readAsStringSync(), before);
    expect(store.read('sheet-1'), {'stats': {'dexterity': 16}});
  });

  test('Edge: delete removes the file and is a no-op when absent (DRB-14)', () {
    final store = newStore();
    store.write('sheet-1', {'stats': {'dexterity': 16}});
    expect(draftFile('sheet-1').existsSync(), isTrue);

    store.delete('sheet-1');
    expect(draftFile('sheet-1').existsSync(), isFalse);
    expect(store.read('sheet-1'), isNull);

    expect(() => store.delete('sheet-1'), returnsNormally);
  });

  test('Edge: accepts a whole-delta write when no drafts directory exists yet', () {
    expect(Directory('${root.path}/drafts').existsSync(), isFalse);

    newStore().write('sheet-1', {'stats': {'dexterity': 16}});

    expect(Directory('${root.path}/drafts').existsSync(), isTrue);
    expect(newStore().read('sheet-1'), {'stats': {'dexterity': 16}});
  });
}
