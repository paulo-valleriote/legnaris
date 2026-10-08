import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/auth/secure_token_store.dart';

import 'fakes.dart';

void main() {
  group('secure token store (FE6-02, FE6-17)', () {
    test('Happy path: write then read returns the token under the single refresh key', () async {
      final storage = FakeSecureKeyValueStore();
      final store = FlutterSecureTokenStore(storage: storage);

      await store.write('refresh-1');

      expect(await store.read(), 'refresh-1');
      expect(storage.writtenKeys, [FlutterSecureTokenStore.defaultKey]);
    });

    test('Happy path: clear deletes the key and read returns null', () async {
      final storage = FakeSecureKeyValueStore();
      final store = FlutterSecureTokenStore(storage: storage);
      await store.write('refresh-1');

      await store.clear();

      expect(await store.read(), isNull);
      expect(storage.deletedKeys, [FlutterSecureTokenStore.defaultKey]);
    });

    test('Edge: reading with nothing stored returns null', () async {
      final store = FlutterSecureTokenStore(storage: FakeSecureKeyValueStore());

      expect(await store.read(), isNull);
    });

    test('Edge: clearing with nothing stored is a no-op', () async {
      final store = FlutterSecureTokenStore(storage: FakeSecureKeyValueStore());

      await store.clear();

      expect(await store.read(), isNull);
    });

    test('Error: a storage write failure is swallowed and read falls back to memory', () async {
      final storage = FakeSecureKeyValueStore(throwOnWrite: true);
      final store = FlutterSecureTokenStore(storage: storage);

      await store.write('refresh-1');

      expect(await store.read(), 'refresh-1');
      expect(storage.value, isNull, reason: 'the token was never persisted');
    });

    test('Error: a storage read failure falls back to the in-memory token', () async {
      final storage = FakeSecureKeyValueStore();
      final store = FlutterSecureTokenStore(storage: storage);
      await store.write('refresh-1');

      storage.throwOnRead = true;

      expect(await store.read(), 'refresh-1');
    });

    test('Error: a storage read failure with no in-memory token returns null', () async {
      final storage = FakeSecureKeyValueStore(throwOnRead: true);
      final store = FlutterSecureTokenStore(storage: storage);

      expect(await store.read(), isNull);
    });

    test('Error: a storage clear failure is swallowed and read returns null', () async {
      final storage = FakeSecureKeyValueStore();
      final store = FlutterSecureTokenStore(storage: storage);
      await store.write('refresh-1');

      storage.throwOnDelete = true;
      await store.clear();

      expect(await store.read(), isNull);
    });
  });
}
