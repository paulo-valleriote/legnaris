import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/authenticated_client.dart';
import 'package:legnaris/auth/auth_session.dart';

/// In-memory [SessionDelegate] with an optional gate so a refresh can be held
/// in flight while concurrent `401`s arrive.
class FakeSessionDelegate implements SessionDelegate {
  FakeSessionDelegate({this.accessToken, this.refreshedToken});

  String? accessToken;
  String? refreshedToken;

  int refreshCalls = 0;
  int unauthorizedCalls = 0;

  Completer<String?>? refreshGate;

  @override
  String? currentAccessToken() => accessToken;

  @override
  Future<String?> refresh() async {
    refreshCalls++;
    final gate = refreshGate;
    if (gate != null) {
      return gate.future;
    }
    return refreshedToken;
  }

  @override
  Future<void> onUnauthorized() async {
    unauthorizedCalls++;
    accessToken = null;
  }
}

void main() {
  final protectedUri = Uri.parse('https://api.legnaris.com.br/sheets');
  final healthUri = Uri.parse('https://api.legnaris.com.br/health');
  final loginUri = Uri.parse('https://api.legnaris.com.br/auth/login');

  AuthenticatedClient clientWith(
    FakeSessionDelegate session,
    Future<http.Response> Function(http.Request request) handler, [
    List<http.Request>? recorded,
  ]) => AuthenticatedClient(
    session: session,
    inner: MockClient((request) async {
      recorded?.add(request);
      return handler(request);
    }),
  );

  group('bearer scoping (FE6-08, FE6-12)', () {
    test('Happy path: a protected request carries the bearer header', () async {
      final recorded = <http.Request>[];
      final client = clientWith(
        FakeSessionDelegate(accessToken: 'access-1'),
        (request) async => http.Response('{"data": {}}', 200),
        recorded,
      );

      await client.get(protectedUri);

      expect(recorded.single.headers['Authorization'], 'Bearer access-1');
    });

    test('Edge: /auth/* and /health never carry an authorization header', () async {
      final recorded = <http.Request>[];
      final client = clientWith(
        FakeSessionDelegate(accessToken: 'access-1'),
        (request) async => http.Response('{"data": {}}', 200),
        recorded,
      );

      await client.get(healthUri);
      await client.post(loginUri);

      for (final request in recorded) {
        expect(request.headers.containsKey('Authorization'), isFalse);
      }
    });

    test('Edge: an unauthenticated protected request omits the header', () async {
      final recorded = <http.Request>[];
      final client = clientWith(
        FakeSessionDelegate(),
        (request) async => http.Response('{"data": {}}', 200),
        recorded,
      );

      await client.get(protectedUri);

      expect(recorded.single.headers.containsKey('Authorization'), isFalse);
    });
  });

  group('401 refresh and replay (FE6-09, FE6-11)', () {
    test('Happy path: a protected 401 refreshes once and replays once', () async {
      final recorded = <http.Request>[];
      final session = FakeSessionDelegate(
        accessToken: 'access-1',
        refreshedToken: 'access-2',
      );
      final client = clientWith(session, (request) async {
        return request.headers['Authorization'] == 'Bearer access-2'
            ? http.Response('{"data": {}}', 200)
            : http.Response(
                '{"error": {"code": "UNAUTHORIZED", "message": "expired"}}',
                401,
              );
      }, recorded);

      final response = await client.get(protectedUri);

      expect(response.statusCode, 200);
      expect(session.refreshCalls, 1);
      expect(recorded, hasLength(2));
      expect(recorded.first.headers['Authorization'], 'Bearer access-1');
      expect(recorded.last.headers['Authorization'], 'Bearer access-2');
    });

    test('Happy path: the replay preserves the method and body', () async {
      final recorded = <http.Request>[];
      final session = FakeSessionDelegate(
        accessToken: 'access-1',
        refreshedToken: 'access-2',
      );
      final client = clientWith(session, (request) async {
        return request.headers['Authorization'] == 'Bearer access-2'
            ? http.Response('{"data": {}}', 200)
            : http.Response(
                '{"error": {"code": "UNAUTHORIZED", "message": "expired"}}',
                401,
              );
      }, recorded);

      await client.post(
        protectedUri,
        headers: {'content-type': 'application/json'},
        body: '{"data":{"stats":{"dexterity":16}}}',
      );

      expect(recorded.last.method, 'POST');
      expect(
        recorded.last.body,
        '{"data":{"stats":{"dexterity":16}}}',
      );
    });

    test('Error: a failed refresh forces logout and never replays', () async {
      final recorded = <http.Request>[];
      final session = FakeSessionDelegate(accessToken: 'access-1');
      final client = clientWith(
        session,
        (request) async => http.Response(
          '{"error": {"code": "UNAUTHORIZED", "message": "revoked"}}',
          401,
        ),
        recorded,
      );

      final response = await client.get(protectedUri);

      expect(response.statusCode, 401);
      expect(session.refreshCalls, 1);
      expect(session.unauthorizedCalls, 1);
      expect(recorded, hasLength(1), reason: 'no replay after a failed refresh');
    });

    test('Edge: two concurrent 401s share a single refresh', () async {
      final recorded = <http.Request>[];
      final session = FakeSessionDelegate(
        accessToken: 'access-1',
        refreshedToken: 'access-2',
      )..refreshGate = Completer<String?>();
      final client = clientWith(session, (request) async {
        return request.headers['Authorization'] == 'Bearer access-2'
            ? http.Response('{"data": {}}', 200)
            : http.Response(
                '{"error": {"code": "UNAUTHORIZED", "message": "expired"}}',
                401,
              );
      }, recorded);

      final first = client.get(protectedUri);
      final second = client.get(protectedUri);
      await Future<void>.delayed(Duration.zero);
      expect(session.refreshCalls, 1);

      session.refreshGate!.complete('access-2');
      final responses = await Future.wait([first, second]);

      expect(responses.map((response) => response.statusCode), [200, 200]);
      expect(session.refreshCalls, 1, reason: 'single-flight refresh');
    });

    test('Edge: a non-401 response is returned without refreshing', () async {
      final session = FakeSessionDelegate(accessToken: 'access-1');
      final client = clientWith(
        session,
        (request) async => http.Response(
          '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}',
          500,
        ),
      );

      final response = await client.get(protectedUri);

      expect(response.statusCode, 500);
      expect(session.refreshCalls, 0);
      expect(session.unauthorizedCalls, 0);
    });

    test('Edge: a 401 on a public path is not refreshed', () async {
      final session = FakeSessionDelegate(accessToken: 'access-1');
      final client = clientWith(
        session,
        (request) async => http.Response(
          '{"error": {"code": "UNAUTHORIZED", "message": "invalid refresh token"}}',
          401,
        ),
      );

      final response = await client.post(loginUri);

      expect(response.statusCode, 401);
      expect(session.refreshCalls, 0);
      expect(session.unauthorizedCalls, 0);
    });
  });
}
