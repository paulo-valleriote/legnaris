import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/api/error_mapper.dart';

void main() {
  const userBody =
      '{"data": {"id": "user-1", "email": "player@example.com", "username": "player", "created_at": "2026-09-20T12:00:00Z"}}';
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const rotatedBody =
      '{"data": {"access_token": "access-2", "refresh_token": "refresh-2", "token_type": "Bearer", "expires_in": 900}}';

  MockClient mockClient(
    Future<http.Response> Function(http.Request request) handler,
    List<http.Request> recorded,
  ) => MockClient((request) async {
    recorded.add(request);
    return handler(request);
  });

  Future<ApiDomainException> domainErrorFrom(Future<Object?> call) async {
    try {
      await call;
      fail('expected ApiDomainException');
    } on ApiDomainException catch (error) {
      return error;
    }
  }

  group('auth client (FE6-03, FE6-07, FE6-10)', () {
    test('Happy path: register posts the credentials and returns the created user', () async {
      final recorded = <http.Request>[];
      final client = AuthApiClient(
        httpClient: mockClient(
          (request) async => http.Response(userBody, 201),
          recorded,
        ),
      );

      final user = await client.register(
        email: 'player@example.com',
        username: 'player',
        password: 'sup3r-secret',
      );

      expect(user.id, 'user-1');
      expect(user.email, 'player@example.com');
      expect(user.username, 'player');

      final request = recorded.single;
      expect(request.method, 'POST');
      expect(request.url.path, '/auth/register');
      expect(jsonDecode(request.body), {
        'email': 'player@example.com',
        'username': 'player',
        'password': 'sup3r-secret',
      });
      expect(request.headers['content-type'], contains('application/json'));
    });

    test('Happy path: login posts the credentials and returns the token pair', () async {
      final recorded = <http.Request>[];
      final client = AuthApiClient(
        httpClient: mockClient(
          (request) async => http.Response(pairBody, 200),
          recorded,
        ),
      );

      final pair = await client.login(
        email: 'player@example.com',
        password: 'sup3r-secret',
      );

      expect(pair.accessToken, 'access-1');
      expect(pair.refreshToken, 'refresh-1');

      final request = recorded.single;
      expect(request.method, 'POST');
      expect(request.url.path, '/auth/login');
      expect(jsonDecode(request.body), {
        'email': 'player@example.com',
        'password': 'sup3r-secret',
      });
    });

    test('Happy path: refresh sends X-Refresh-Token with no body and returns the rotated pair', () async {
      final recorded = <http.Request>[];
      final client = AuthApiClient(
        httpClient: mockClient(
          (request) async => http.Response(rotatedBody, 200),
          recorded,
        ),
      );

      final pair = await client.refresh('refresh-1');

      expect(pair.accessToken, 'access-2');
      expect(pair.refreshToken, 'refresh-2');

      final request = recorded.single;
      expect(request.method, 'POST');
      expect(request.url.path, '/auth/refresh');
      expect(request.headers[AuthApiClient.refreshTokenHeader], 'refresh-1');
      expect(request.bodyBytes, isEmpty, reason: 'refresh has no body');
    });

    test('Happy path: logout sends X-Refresh-Token and treats 204 as success', () async {
      final recorded = <http.Request>[];
      final client = AuthApiClient(
        httpClient: mockClient(
          (request) async => http.Response('', 204),
          recorded,
        ),
      );

      await client.logout('refresh-1');

      final request = recorded.single;
      expect(request.method, 'POST');
      expect(request.url.path, '/auth/logout');
      expect(request.headers[AuthApiClient.refreshTokenHeader], 'refresh-1');
      expect(request.bodyBytes, isEmpty, reason: 'logout has no body');
    });

    test('Edge: refresh and logout never send a cookie', () async {
      final recorded = <http.Request>[];
      final client = AuthApiClient(
        httpClient: mockClient((request) async {
          return http.Response(
            request.url.path == '/auth/logout' ? '' : pairBody,
            request.url.path == '/auth/logout' ? 204 : 200,
          );
        }, recorded),
      );

      await client.refresh('refresh-1');
      await client.logout('refresh-1');

      for (final request in recorded) {
        expect(
          request.headers.keys.map((key) => key.toLowerCase()),
          isNot(contains('cookie')),
        );
      }
    });
  });

  group('auth client mapped errors (FE6-05, FE6-21)', () {
    test('Error: 401 UNAUTHORIZED on login keeps the raw code as unknown', () async {
      final client = AuthApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "UNAUTHORIZED", "message": "invalid credentials"}}',
            401,
          ),
        ),
      );

      final error = await domainErrorFrom(
        client.login(email: 'player@example.com', password: 'wrong'),
      );

      expect(error.kind, ApiErrorKind.unknown);
      expect(error.code, 'UNAUTHORIZED');
      expect(error.message, 'invalid credentials');
      expect(error.statusCode, 401);
    });

    test('Error: 400 INVALID_REQUEST on register maps to invalidEdit', () async {
      final client = AuthApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "INVALID_REQUEST", "message": "password must be at least 8 bytes"}}',
            400,
          ),
        ),
      );

      final error = await domainErrorFrom(
        client.register(email: 'a@b.com', username: 'ab', password: 'short'),
      );

      expect(error.kind, ApiErrorKind.invalidEdit);
      expect(error.code, 'INVALID_REQUEST');
      expect(error.statusCode, 400);
    });

    test('Error: 409 CONFLICT on register keeps the raw code as unknown', () async {
      final client = AuthApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "CONFLICT", "message": "email already registered"}}',
            409,
          ),
        ),
      );

      final error = await domainErrorFrom(
        client.register(
          email: 'player@example.com',
          username: 'player',
          password: 'sup3r-secret',
        ),
      );

      expect(error.kind, ApiErrorKind.unknown);
      expect(error.code, 'CONFLICT');
    });

    test('Error: 401 UNAUTHORIZED on refresh maps to unknown', () async {
      final client = AuthApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "UNAUTHORIZED", "message": "invalid refresh token"}}',
            401,
          ),
        ),
      );

      final error = await domainErrorFrom(client.refresh('revoked'));

      expect(error.kind, ApiErrorKind.unknown);
      expect(error.statusCode, 401);
    });

    test('Error: 500 INTERNAL_ERROR on logout maps to serverError', () async {
      final client = AuthApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}',
            500,
          ),
        ),
      );

      final error = await domainErrorFrom(client.logout('refresh-1'));

      expect(error.kind, ApiErrorKind.serverError);
      expect(error.statusCode, 500);
    });

    test('Error: a transport failure maps to offline', () async {
      final client = AuthApiClient(
        httpClient: MockClient(
          (request) async => throw http.ClientException('connection refused'),
        ),
      );

      final error = await domainErrorFrom(
        client.login(email: 'player@example.com', password: 'sup3r-secret'),
      );

      expect(error.kind, ApiErrorKind.offline);
    });

    test('Error: a 200 without the data envelope fails as a parse error', () async {
      final client = AuthApiClient(
        httpClient: MockClient(
          (request) async => http.Response('{"unexpected": true}', 200),
        ),
      );

      expect(
        () => client.login(email: 'player@example.com', password: 'sup3r-secret'),
        throwsA(isA<ApiParseException>()),
      );
    });
  });
}
