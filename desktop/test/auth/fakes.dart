/// Shared in-memory fakes for the auth test suites.
///
/// They keep the secure-storage platform channel and any real transport out of
/// the tests (design §Risks: injectable `SecureTokenStore`).
library;

import 'package:legnaris/auth/secure_token_store.dart';

/// In-memory [SecureTokenStore] used wherever the session needs a store.
class InMemorySecureTokenStore implements SecureTokenStore {
  InMemorySecureTokenStore({this.token});

  String? token;

  @override
  Future<String?> read() async => token;

  @override
  Future<void> write(String token) async => this.token = token;

  @override
  Future<void> clear() async => token = null;
}

/// In-memory [SecureKeyValueStore] that can be told to fail per operation, so
/// the unavailable-storage path (FE6-17) is exercised without a platform
/// channel.
class FakeSecureKeyValueStore implements SecureKeyValueStore {
  FakeSecureKeyValueStore({
    this.value,
    this.throwOnRead = false,
    this.throwOnWrite = false,
    this.throwOnDelete = false,
  });

  String? value;

  bool throwOnRead;
  bool throwOnWrite;
  bool throwOnDelete;

  /// Keys passed to a successful or failed [write], in call order.
  final List<String> writtenKeys = [];

  /// Keys passed to a successful or failed [delete], in call order.
  final List<String> deletedKeys = [];

  @override
  Future<String?> read(String key) async {
    if (throwOnRead) {
      throw StateError('secure storage unavailable');
    }
    return value;
  }

  @override
  Future<void> write(String key, String value) async {
    writtenKeys.add(key);
    if (throwOnWrite) {
      throw StateError('secure storage unavailable');
    }
    this.value = value;
  }

  @override
  Future<void> delete(String key) async {
    deletedKeys.add(key);
    if (throwOnDelete) {
      throw StateError('secure storage unavailable');
    }
    value = null;
  }
}
