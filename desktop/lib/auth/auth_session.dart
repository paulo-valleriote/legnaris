import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/auth/secure_token_store.dart';
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
  /// it, or null when the refresh was rejected.
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

  @override
  AuthState build() => const AuthUnauthenticated();

  @override
  String? currentAccessToken() => _accessToken;

  /// Restores a stored session by exchanging the refresh token without
  /// prompting (FE6-01). With no stored token it stays unauthenticated and
  /// issues no request.
  Future<void> restore() async {
    final stored = await _store.read();
    if (stored == null || stored.isEmpty) {
      _accessToken = null;
      state = const AuthUnauthenticated();
      return;
    }
    state = const AuthAuthenticating();
    final token = await refresh();
    if (token == null) {
      await onUnauthorized();
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
    } catch (_) {
      return null;
    }
  }

  @override
  Future<void> onUnauthorized() async {
    _version++;
    _accessToken = null;
    await _store.clear();
    state = const AuthUnauthenticated();
  }

  /// Ends the session: best-effort `/auth/logout`, clear storage, reset state
  /// (FE6-07). A failed logout call never blocks the local reset.
  Future<void> logout() async {
    final stored = await _store.read();
    _version++;
    _accessToken = null;
    await _store.clear();
    state = const AuthUnauthenticated();
    if (stored == null || stored.isEmpty) {
      return;
    }
    try {
      await _authClient.logout(stored);
    } catch (_) {
      return;
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
