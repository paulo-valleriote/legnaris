import 'dart:async';
import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/api/authenticated_client.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/auth/secure_token_store.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/library/sheet_library_store.dart';

import '../library/library_fixtures.dart';
import '../sheet/draft_fixtures.dart';
import 'fakes.dart';

void main() {
  const userBody =
      '{"data": {"id": "user-1", "email": "player@example.com", "username": "player", "created_at": "2026-09-20T12:00:00Z"}}';
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const rotatedBody =
      '{"data": {"access_token": "access-2", "refresh_token": "refresh-2", "token_type": "Bearer", "expires_in": 900}}';
  const unauthorizedBody =
      '{"error": {"code": "UNAUTHORIZED", "message": "invalid refresh token"}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

  ProviderContainer containerWith({
    required http.Client authTransport,
    SecureTokenStore? store,
    Directory? supportDir,
  }) {
    final container = ProviderContainer(
      overrides: [
        secureTokenStoreProvider.overrideWithValue(
          store ?? InMemorySecureTokenStore(),
        ),
        authApiClientProvider.overrideWithValue(
          AuthApiClient(httpClient: authTransport),
        ),
        if (supportDir != null)
          appSupportDirectoryProvider.overrideWithValue(supportDir),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  group('session lifecycle (FE6-01, FE6-03, FE6-04)', () {
    test('Happy path: login authenticates, persists the refresh token and holds the access token in memory', () async {
      final recorded = <http.Request>[];
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          recorded.add(request);
          return http.Response(pairBody, 200);
        }),
      );
      final session = container.read(authSessionProvider.notifier);

      await session.login(email: 'player@example.com', password: 'sup3r-secret');

      final state = container.read(authSessionProvider);
      expect(state, isA<AuthAuthenticated>());
      expect((state as AuthAuthenticated).accessToken, 'access-1');
      expect(session.currentAccessToken(), 'access-1');
      expect(store.token, 'refresh-1');
      expect(recorded.single.url.path, '/auth/login');
    });

    test('Happy path: register then logs in to reach the authenticated app', () async {
      final recorded = <http.Request>[];
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          recorded.add(request);
          return http.Response(
            request.url.path == '/auth/register' ? userBody : pairBody,
            request.url.path == '/auth/register' ? 201 : 200,
          );
        }),
      );
      final session = container.read(authSessionProvider.notifier);

      await session.register(
        email: 'player@example.com',
        username: 'player',
        password: 'sup3r-secret',
      );

      expect(container.read(authSessionProvider), isA<AuthAuthenticated>());
      expect(
        recorded.map((request) => request.url.path).toList(),
        ['/auth/register', '/auth/login'],
      );
      expect(store.token, 'refresh-1');
    });

    test('Happy path: restore exchanges the stored token without prompting and rotates it', () async {
      final recorded = <http.Request>[];
      final store = InMemorySecureTokenStore(token: 'stored-refresh');
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          recorded.add(request);
          return http.Response(rotatedBody, 200);
        }),
      );
      final session = container.read(authSessionProvider.notifier);

      await session.restore();

      final state = container.read(authSessionProvider);
      expect(state, isA<AuthAuthenticated>());
      expect((state as AuthAuthenticated).accessToken, 'access-2');
      expect(store.token, 'refresh-2', reason: 'the rotated token is persisted');
      expect(recorded.single.url.path, '/auth/refresh');
      expect(
        recorded.single.headers[AuthApiClient.refreshTokenHeader],
        'stored-refresh',
      );
    });

    test('Edge: restore with no stored token stays unauthenticated and issues no request', () async {
      final recorded = <http.Request>[];
      final container = containerWith(
        authTransport: MockClient((request) async {
          recorded.add(request);
          return http.Response(pairBody, 200);
        }),
      );
      final session = container.read(authSessionProvider.notifier);

      await session.restore();

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(recorded, isEmpty);
    });
  });

  group('session failures (FE6-05, FE6-06, FE6-17)', () {
    test('Error: a rejected restore refresh clears storage and returns to unauthenticated', () async {
      final store = InMemorySecureTokenStore(token: 'revoked');
      final container = containerWith(
        store: store,
        authTransport: MockClient(
          (request) async => http.Response(unauthorizedBody, 401),
        ),
      );
      final session = container.read(authSessionProvider.notifier);

      await session.restore();

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(session.currentAccessToken(), isNull);
      expect(store.token, isNull, reason: 'the revoked token is cleared');
    });

    test('Error: a failed login rethrows the mapped error and stays unauthenticated', () async {
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient(
          (request) async => http.Response(unauthorizedBody, 401),
        ),
      );
      final session = container.read(authSessionProvider.notifier);

      await expectLater(
        session.login(email: 'player@example.com', password: 'wrong'),
        throwsA(
          isA<ApiDomainException>().having(
            (error) => error.kind,
            'kind',
            ApiErrorKind.unknown,
          ),
        ),
      );

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(store.token, isNull);
    });

    test('Error: a rejected registration rethrows the mapped error and never logs in', () async {
      final recorded = <http.Request>[];
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          recorded.add(request);
          return http.Response(
            '{"error": {"code": "INVALID_REQUEST", "message": "password must be at least 8 bytes"}}',
            400,
          );
        }),
      );
      final session = container.read(authSessionProvider.notifier);

      await expectLater(
        session.register(
          email: 'a@b.com',
          username: 'ab',
          password: 'short',
        ),
        throwsA(
          isA<ApiDomainException>().having(
            (error) => error.kind,
            'kind',
            ApiErrorKind.invalidEdit,
          ),
        ),
      );

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(recorded.map((request) => request.url.path), ['/auth/register']);
      expect(store.token, isNull);
    });

    test('Error: unavailable secure storage keeps the session for the run', () async {
      final raw = FakeSecureKeyValueStore(throwOnRead: true, throwOnWrite: true);
      final container = containerWith(
        store: FlutterSecureTokenStore(storage: raw),
        authTransport: MockClient((request) async => http.Response(pairBody, 200)),
      );
      final session = container.read(authSessionProvider.notifier);

      await session.login(email: 'player@example.com', password: 'sup3r-secret');

      expect(container.read(authSessionProvider), isA<AuthAuthenticated>());
      expect(session.currentAccessToken(), 'access-1');
      expect(raw.value, isNull, reason: 'the token was not persisted');
    });
  });

  group('logout and forced logout (FE6-07, FE6-19)', () {
    test('Happy path: logout sends the stored token, clears storage and resets state', () async {
      final recorded = <http.Request>[];
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          recorded.add(request);
          if (request.url.path == '/auth/logout') {
            return http.Response('', 204);
          }
          return http.Response(pairBody, 200);
        }),
      );
      final session = container.read(authSessionProvider.notifier);
      await session.login(email: 'player@example.com', password: 'sup3r-secret');

      await session.logout();

      final logout = recorded.singleWhere(
        (request) => request.url.path == '/auth/logout',
      );
      expect(logout.headers[AuthApiClient.refreshTokenHeader], 'refresh-1');
      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(session.currentAccessToken(), isNull);
      expect(store.token, isNull);
    });

    test('Happy path: onUnauthorized clears the stored token and resets state', () async {
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async => http.Response(pairBody, 200)),
      );
      final session = container.read(authSessionProvider.notifier);
      await session.login(email: 'player@example.com', password: 'sup3r-secret');

      await session.onUnauthorized();

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(session.currentAccessToken(), isNull);
      expect(store.token, isNull);
    });

    test('Edge: a refresh completing after logout does not restore the session', () async {
      final refreshCompleter = Completer<http.Response>();
      final store = InMemorySecureTokenStore(token: 'stored-refresh');
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          if (request.url.path == '/auth/refresh') {
            return refreshCompleter.future;
          }
          return http.Response('', 204);
        }),
      );
      final session = container.read(authSessionProvider.notifier);

      final restoring = session.restore();
      await Future<void>.delayed(Duration.zero);

      await session.logout();
      refreshCompleter.complete(http.Response(rotatedBody, 200));
      await restoring;

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(session.currentAccessToken(), isNull);
      expect(store.token, isNull);
    });

    test('Edge: refresh with no stored token returns null', () async {
      final container = containerWith(
        authTransport: MockClient((request) async => http.Response(rotatedBody, 200)),
      );
      final session = container.read(authSessionProvider.notifier);

      expect(await session.refresh(), isNull);
    });
  });

  group('single-flight logout (DATA-08)', () {
    test('Edge: two logout calls in the same frame issue one request and share the teardown (DATA-08)', () async {
      final recorded = <http.Request>[];
      final logoutCompleter = Completer<http.Response>();
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          recorded.add(request);
          if (request.url.path == '/auth/logout') {
            return logoutCompleter.future;
          }
          return http.Response(pairBody, 200);
        }),
      );
      final session = container.read(authSessionProvider.notifier);
      await session.login(email: 'player@example.com', password: 'sup3r-secret');

      final first = session.logout();
      final second = session.logout();
      await Future<void>.delayed(Duration.zero);
      expect(
        recorded.where((request) => request.url.path == '/auth/logout').length,
        1,
        reason: 'the second activation shares the in-flight teardown',
      );
      logoutCompleter.complete(http.Response('', 204));
      await Future.wait([first, second]);

      expect(
        recorded.where((request) => request.url.path == '/auth/logout').length,
        1,
      );
      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(session.currentAccessToken(), isNull);
      expect(store.token, isNull, reason: 'both callers completed after the same teardown');
    });

    test('Happy path: a logout after the first completes issues a new request (DATA-08)', () async {
      final recorded = <http.Request>[];
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          recorded.add(request);
          if (request.url.path == '/auth/logout') {
            return http.Response('', 204);
          }
          return http.Response(pairBody, 200);
        }),
      );
      final session = container.read(authSessionProvider.notifier);
      await session.login(email: 'player@example.com', password: 'sup3r-secret');

      await session.logout();
      await session.login(email: 'player@example.com', password: 'sup3r-secret');
      await session.logout();

      expect(
        recorded.where((request) => request.url.path == '/auth/logout').length,
        2,
        reason: 'the guard clears once the first logout completes',
      );
      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(store.token, isNull);
    });
  });

  group('transient refresh failure keeps the session (FE6-06, FE6-09)', () {
    test('Error: a 5xx refresh rethrows and keeps the stored token', () async {
      final store = InMemorySecureTokenStore(token: 'stored-refresh');
      final container = containerWith(
        store: store,
        authTransport: MockClient(
          (request) async => http.Response(serverErrorBody, 500),
        ),
      );
      final session = container.read(authSessionProvider.notifier);

      await expectLater(
        session.refresh(),
        throwsA(
          isA<ApiDomainException>().having(
            (error) => error.kind,
            'kind',
            ApiErrorKind.serverError,
          ),
        ),
      );
      expect(
        store.token,
        'stored-refresh',
        reason: 'a transient failure does not clear the stored token',
      );
      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
    });

    test('Error: a transport refresh failure rethrows as offline and keeps the stored token', () async {
      final store = InMemorySecureTokenStore(token: 'stored-refresh');
      final container = containerWith(
        store: store,
        authTransport: MockClient(
          (request) async => throw const SocketException('unreachable'),
        ),
      );
      final session = container.read(authSessionProvider.notifier);

      await expectLater(
        session.refresh(),
        throwsA(
          isA<ApiDomainException>().having(
            (error) => error.kind,
            'kind',
            ApiErrorKind.offline,
          ),
        ),
      );
      expect(store.token, 'stored-refresh');
    });

    test('Edge: restore keeps the stored token when the refresh fails transiently', () async {
      final store = InMemorySecureTokenStore(token: 'stored-refresh');
      final container = containerWith(
        store: store,
        authTransport: MockClient(
          (request) async => http.Response(serverErrorBody, 500),
        ),
      );
      final session = container.read(authSessionProvider.notifier);

      await session.restore();

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(
        store.token,
        'stored-refresh',
        reason: 'the token survives for the next launch',
      );
    });

    test('Error: a 5xx refresh during a protected call keeps the session and surfaces offline', () async {
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          if (request.url.path == '/auth/refresh') {
            return http.Response(serverErrorBody, 500);
          }
          return http.Response(pairBody, 200);
        }),
      );
      final session = container.read(authSessionProvider.notifier);
      await session.login(
        email: 'player@example.com',
        password: 'sup3r-secret',
      );

      final client = ApiClient(
        httpClient: AuthenticatedClient(
          session: session,
          inner: MockClient(
            (request) async => http.Response(unauthorizedBody, 401),
          ),
        ),
      );

      await expectLater(
        client.getMySheet(),
        throwsA(
          isA<ApiDomainException>().having(
            (error) => error.kind,
            'kind',
            ApiErrorKind.offline,
          ),
        ),
      );
      expect(container.read(authSessionProvider), isA<AuthAuthenticated>());
      expect(
        store.token,
        'refresh-1',
        reason: 'the session and the stored token are kept',
      );
    });
  });

  group('session-end local sweep (DATA-01 … DATA-04)', () {
    late Directory supportDir;

    setUp(() {
      supportDir = Directory.systemTemp.createTempSync('auth_session_sweep_');
      addTearDown(() => supportDir.deleteSync(recursive: true));
      SheetLibraryStore(directory: supportDir).save([entry('sheet-1')]);
      seedDraft(supportDir);
    });

    List<String> storedLibraryIds() => SheetLibraryStore(
      directory: supportDir,
    ).load().entries.map((stored) => stored.id).toList();

    test('Happy path: logout sweeps the library index and every draft from disk (DATA-01)', () async {
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        supportDir: supportDir,
        authTransport: MockClient((request) async {
          if (request.url.path == '/auth/logout') {
            return http.Response('', 204);
          }
          return http.Response(pairBody, 200);
        }),
      );
      final session = container.read(authSessionProvider.notifier);
      await session.login(email: 'player@example.com', password: 'sup3r-secret');
      expect(container.read(sheetLibraryProvider).entries, hasLength(1));

      await session.logout();

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(container.read(sheetLibraryProvider).entries, isEmpty);
      expect(
        storedLibraryIds(),
        isEmpty,
        reason: 'the empty index lands on disk',
      );
      expect(draftFiles(supportDir), isEmpty, reason: 'every draft is gone');
    });

    test('Happy path: onUnauthorized sweeps the library index and every draft from disk (DATA-02)', () async {
      final container = containerWith(
        supportDir: supportDir,
        authTransport: MockClient(
          (request) async => http.Response(pairBody, 200),
        ),
      );
      final session = container.read(authSessionProvider.notifier);
      await session.login(email: 'player@example.com', password: 'sup3r-secret');
      expect(container.read(sheetLibraryProvider).entries, hasLength(1));

      await session.onUnauthorized();

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(container.read(sheetLibraryProvider).entries, isEmpty);
      expect(storedLibraryIds(), isEmpty);
      expect(draftFiles(supportDir), isEmpty);
    });

    test('Edge: logout completes even when the local stores cannot be reached (DATA-03)', () async {
      // No appSupportDirectoryProvider override: both sweeps hit the throwing
      // directory provider and must be swallowed.
      final store = InMemorySecureTokenStore();
      final container = containerWith(
        store: store,
        authTransport: MockClient((request) async {
          if (request.url.path == '/auth/logout') {
            return http.Response('', 204);
          }
          return http.Response(pairBody, 200);
        }),
      );
      final session = container.read(authSessionProvider.notifier);
      await session.login(email: 'player@example.com', password: 'sup3r-secret');

      await session.logout();

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(session.currentAccessToken(), isNull);
      expect(store.token, isNull);
    });

    test('Edge: a transient restore failure keeps the token, the library and the drafts (DATA-04)', () async {
      final store = InMemorySecureTokenStore(token: 'stored-refresh');
      final container = containerWith(
        store: store,
        supportDir: supportDir,
        authTransport: MockClient(
          (request) async => http.Response(serverErrorBody, 500),
        ),
      );
      final session = container.read(authSessionProvider.notifier);

      await session.restore();

      expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
      expect(
        store.token,
        'stored-refresh',
        reason: 'the token survives for the next launch',
      );
      expect(
        storedLibraryIds(),
        ['sheet-1'],
        reason: 'the transient path never sweeps the library',
      );
      expect(draftFiles(supportDir), hasLength(1), reason: 'nor the drafts');
    });
  });
}
