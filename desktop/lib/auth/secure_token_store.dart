import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'secure_token_store.g.dart';

/// Persists the single refresh token in OS secure storage, never in the local
/// JSON store and never on disk in plaintext (FE6-02).
abstract class SecureTokenStore {
  /// The persisted refresh token, or null when none is stored.
  Future<String?> read();

  /// Persists [token] as the refresh token.
  Future<void> write(String token);

  /// Removes the persisted refresh token.
  Future<void> clear();
}

/// Low-level credential storage keyed by string; the platform seam tests
/// replace with an in-memory implementation so no channel is hit.
abstract interface class SecureKeyValueStore {
  Future<String?> read(String key);

  Future<void> write(String key, String value);

  Future<void> delete(String key);
}

/// [SecureKeyValueStore] over `flutter_secure_storage`.
class FlutterSecureKeyValueStore implements SecureKeyValueStore {
  FlutterSecureKeyValueStore({FlutterSecureStorage? storage})
    : _storage = storage ?? const FlutterSecureStorage();

  final FlutterSecureStorage _storage;

  @override
  Future<String?> read(String key) => _storage.read(key: key);

  @override
  Future<void> write(String key, String value) =>
      _storage.write(key: key, value: value);

  @override
  Future<void> delete(String key) => _storage.delete(key: key);
}

/// [SecureTokenStore] persisting only the refresh token under one key.
///
/// A storage failure never throws: the token stays in memory for the run and
/// the next launch simply has none to restore, so login still succeeds and
/// only persistence is lost (FE6-17).
class FlutterSecureTokenStore implements SecureTokenStore {
  FlutterSecureTokenStore({SecureKeyValueStore? storage, this.key = defaultKey})
    : _storage = storage ?? FlutterSecureKeyValueStore();

  /// The single secure-storage key the refresh token lives under.
  static const String defaultKey = 'refresh_token';

  final SecureKeyValueStore _storage;

  /// The key [write] and [clear] operate on.
  final String key;

  String? _memoryToken;

  /// True once this instance wrote or cleared, so the in-memory value is the
  /// authority for the run even when the storage call failed.
  bool _hasMemoryToken = false;

  @override
  Future<String?> read() async {
    if (_hasMemoryToken) {
      return _memoryToken;
    }
    try {
      return await _storage.read(key);
    } catch (_) {
      return null;
    }
  }

  @override
  Future<void> write(String token) async {
    _memoryToken = token;
    _hasMemoryToken = true;
    try {
      await _storage.write(key, token);
    } catch (_) {
      return;
    }
  }

  @override
  Future<void> clear() async {
    _memoryToken = null;
    _hasMemoryToken = true;
    try {
      await _storage.delete(key);
    } catch (_) {
      return;
    }
  }
}

/// The app-wide refresh-token store. Tests override it with an in-memory fake
/// so the secure-storage platform channel is never touched.
@Riverpod(keepAlive: true)
SecureTokenStore secureTokenStore(Ref ref) => FlutterSecureTokenStore();
