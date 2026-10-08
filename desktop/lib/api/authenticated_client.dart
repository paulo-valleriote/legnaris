import 'package:http/http.dart' as http;
import 'package:legnaris/auth/auth_session.dart' show SessionDelegate;

/// `http.BaseClient` decorator that authenticates the protected domain calls.
///
/// It attaches `Authorization: Bearer <access>`, refreshes exactly once on a
/// protected `401` and replays that request once. Concurrent `401`s share a
/// single refresh. A rejected refresh (401) forces a logout; a transient
/// refresh failure propagates so the session is kept. `/auth/*` and `/health`
/// are never authenticated or retried (FE6-08 … FE6-12).
class AuthenticatedClient extends http.BaseClient {
  AuthenticatedClient({
    required this._inner,
    required this._session,
  });

  /// Paths that are public and must never carry a token or trigger a refresh.
  static const String healthPath = '/health';
  static const String authPrefix = '/auth/';

  final http.Client _inner;
  final SessionDelegate _session;

  Future<String?>? _refreshInFlight;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final isPublic = _isPublic(request.url.path);
    if (!isPublic) {
      final token = _session.currentAccessToken();
      if (token != null) {
        request.headers['Authorization'] = 'Bearer $token';
      }
    }

    final response = await _inner.send(request);
    if (isPublic || response.statusCode != 401) {
      return response;
    }

    final token = await _refreshOnce();
    if (token == null) {
      await _session.onUnauthorized();
      return response;
    }

    final replay = _replay(request);
    if (replay == null) {
      return response;
    }
    replay.headers['Authorization'] = 'Bearer $token';
    return _inner.send(replay);
  }

  /// Runs one refresh and shares the in-flight future with every concurrent
  /// `401` so the token family is rotated only once (FE6-11).
  Future<String?> _refreshOnce() {
    final existing = _refreshInFlight;
    if (existing != null) {
      return existing;
    }
    final future = _session.refresh();
    _refreshInFlight = future;
    return future.whenComplete(() {
      if (identical(_refreshInFlight, future)) {
        _refreshInFlight = null;
      }
    });
  }

  /// Rebuilds a finalized [http.Request] so it can be replayed once. Returns
  /// null for streamed requests, which cannot be replayed.
  http.BaseRequest? _replay(http.BaseRequest request) {
    if (request is! http.Request) {
      return null;
    }
    return http.Request(request.method, request.url)
      ..headers.addAll(request.headers)
      ..bodyBytes = request.bodyBytes
      ..followRedirects = request.followRedirects
      ..maxRedirects = request.maxRedirects
      ..persistentConnection = request.persistentConnection;
  }

  bool _isPublic(String path) =>
      path == healthPath || path.startsWith(authPrefix);
}
