import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/auth/secure_token_store.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'auth_session.g.dart';

/// The session state the gate and screens read.
sealed class AuthState {
  const AuthState();
}

/// No session: the auth screen is shown.
class AuthUnauthenticated extends AuthState {
  const AuthUnauthenticated();
}

/// A login, register or restore is in flight.
class AuthAuthenticating extends AuthState {
  const AuthAuthenticating();
}

/// A session is held; [accessToken] lives in memory only.
class AuthAuthenticated extends AuthState {
  const AuthAuthenticated(this.accessToken);

  final String accessToken;
}

/// The token surface the authenticated transport decorator drives (FE6-08 …
/// FE6-12).
abstract interface class SessionDelegate {
  /// The in-memory access token, or null when unauthenticated.
  String? currentAccessToken();

  /// Exchanges the stored refresh token for a fresh access token and returns
  /// it, or null when the refresh was rejected with a 401. A transient failure
  /// (offline, 5xx, contract drift) is rethrown so the caller keeps the session
  /// and can retry instead of forcing a logout.
  Future<String?> refresh();

  /// Forces the session back to unauthenticated and clears the stored token.
  Future<void> onUnauthorized();
}

/// Owns the in-memory access token and drives the refresh-token lifecycle
/// against [SecureTokenStore] and the public [AuthApiClient] (FE6-01 … FE6-07).
@Riverpod(keepAlive: true)
class AuthSession extends _$AuthSession implements SessionDelegate {
  String? _accessToken;

  /// Bumped on every logout/forced logout so a refresh that was in flight
  /// cannot restore the session afterwards (FE6-19).
  int _version = 0;

  /// Shared in-flight logout: concurrent activations return this future
  /// instead of starting a second teardown (DATA-08).
  Future<void>? _logoutInFlight;

  @override
  AuthState build() => const AuthUnauthenticated();

  @override
  String? currentAccessToken() => _accessToken;

  /// Restores a stored session by exchanging the refresh token without
  /// prompting (FE6-01). With no stored token it stays unauthenticated and
  /// issues no request. A transient refresh failure keeps the stored token for
  /// the next launch and falls back to the auth screen for this run, instead of
  /// forcing a logout (FE6-06, FE6-09).
  Future<void> restore() async {
    final stored = await _store.read();
    if (stored == null || stored.isEmpty) {
      _accessToken = null;
      state = const AuthUnauthenticated();
      return;
    }
    state = const AuthAuthenticating();
    try {
      final token = await refresh();
      if (token == null) {
        await onUnauthorized();
      }
    } catch (_) {
      _accessToken = null;
      state = const AuthUnauthenticated();
    }
  }

  /// Logs in and holds the access token in memory, persisting the refresh
  /// token (FE6-03). A failure keeps the unauthenticated state and rethrows
  /// the mapped error for the screen (FE6-05).
  Future<void> login({
    required String email,
    required String password,
  }) async {
    state = const AuthAuthenticating();
    try {
      final pair = await _authClient.login(email: email, password: password);
      await _apply(pair);
    } catch (_) {
      _accessToken = null;
      state = const AuthUnauthenticated();
      rethrow;
    }
  }

  /// Registers, then immediately logs in to reach the authenticated app
  /// (FE6-04). A register failure keeps the unauthenticated state and
  /// rethrows the mapped error (FE6-21).
  Future<void> register({
    required String email,
    required String username,
    required String password,
  }) async {
    state = const AuthAuthenticating();
    try {
      await _authClient.register(
        email: email,
        username: username,
        password: password,
      );
      final pair = await _authClient.login(email: email, password: password);
      await _apply(pair);
    } catch (_) {
      _accessToken = null;
      state = const AuthUnauthenticated();
      rethrow;
    }
  }

  @override
  Future<String?> refresh() async {
    final stored = await _store.read();
    if (stored == null || stored.isEmpty) {
      return null;
    }
    final version = _version;
    try {
      final pair = await _authClient.refresh(stored);
      if (_version != version) {
        return null;
      }
      await _apply(pair);
      return pair.accessToken;
    } catch (error) {
      if (_isRejected(error)) {
        return null;
      }
      rethrow;
    }
  }

  /// True when [error] is a definitive rejection of the refresh token, so the
  /// session must end. The backend answers a revoked, reused or expired token
  /// with a 401; an offline failure, a 5xx or contract drift is transient and
  /// must not destroy the stored token (FE6-06, FE6-09).
  bool _isRejected(Object error) {
    if (error is ApiDomainException) {
      return error.statusCode == 401;
    }
    if (error is ApiParseException) {
      return error.statusCode == 401;
    }
    return false;
  }

  @override
  Future<void> onUnauthorized() async {
    _version++;
    _accessToken = null;
    await _store.clear();
    state = const AuthUnauthenticated();
    _clearSessionLocalData();
  }

  /// Ends the session: best-effort `/auth/logout`, clear storage, reset state
  /// (FE6-07). A failed logout call never blocks the local reset.
  ///
  /// Single-flight (DATA-08): the in-flight guard is set synchronously, before
  /// the body's first `await` can yield to a second activation in the same
  /// frame, so at most one `/auth/logout` is issued per logout and every
  /// caller waits on the same teardown. The guard clears on completion, so a
  /// later logout runs a fresh teardown.
  Future<void> logout() {
    final existing = _logoutInFlight;
    if (existing != null) {
      return existing;
    }
    final future = _doLogout();
    _logoutInFlight = future;
    return future.whenComplete(() {
      if (identical(_logoutInFlight, future)) {
        _logoutInFlight = null;
      }
    });
  }

  Future<void> _doLogout() async {
    final stored = await _store.read();
    _version++;
    _accessToken = null;
    await _store.clear();
    state = const AuthUnauthenticated();
    _clearSessionLocalData();
    if (stored == null || stored.isEmpty) {
      return;
    }
    try {
      await _authClient.logout(stored);
    } catch (_) {
      return;
    }
  }

  /// Sweeps the session-local stores at a definitive session end (DATA-01,
  /// DATA-02): the local library index and every draft backup belong to this
  /// session only. Each store clears in its own `try/catch`, so a failing
  /// library never skips the drafts sweep nor the logout itself (DATA-03).
  /// Never called from the transient restore fallback, which keeps the files
  /// (DATA-04). Synchronous right after the state flip — no `await` between —
  /// so no response continuation or timer tick can interleave the pair
  /// (design: sweep ordering).
  void _clearSessionLocalData() {
    try {
      ref.read(sheetLibraryProvider.notifier).clear();
    } catch (_) {
      // Best-effort: a failing library never skips the drafts sweep.
    }
    try {
      ref.read(draftBackupStoreProvider).clearAll();
    } catch (_) {
      // Best-effort: a failing drafts store never blocks the session end.
    }
  }

  Future<void> _apply(TokenPair pair) async {
    _accessToken = pair.accessToken;
    await _store.write(pair.refreshToken);
    state = AuthAuthenticated(pair.accessToken);
  }

  SecureTokenStore get _store => ref.read(secureTokenStoreProvider);

  AuthApiClient get _authClient => ref.read(authApiClientProvider);
}
