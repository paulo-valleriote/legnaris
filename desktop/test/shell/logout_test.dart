import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/auth/auth_gate.dart';
import 'package:legnaris/auth/auth_screen.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/auth/secure_token_store.dart';
import 'package:legnaris/connection/health_scheduler.dart';
import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/library/sheet_library_store.dart';
import 'package:legnaris/main.dart';
import 'package:legnaris/shell/app_shell.dart';

import '../auth/fakes.dart';
import '../connection/manual_health_scheduler.dart';
import '../library/library_fixtures.dart';
import '../sheet/draft_fixtures.dart';

void main() {
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const rotatedBody =
      '{"data": {"access_token": "access-2", "refresh_token": "refresh-2", "token_type": "Bearer", "expires_in": 900}}';
  const healthyBody = '{"data": {"status": "healthy"}}';
  const notFoundBody =
      '{"error": {"code": "NOT_FOUND", "message": "none"}}';

  Future<http.Response> defaultAuth(http.Request request) async {
    if (request.url.path == '/auth/logout') {
      return http.Response('', 204);
    }
    return http.Response(pairBody, 200);
  }

  Future<http.Response> defaultDomain(http.Request request) async {
    if (request.url.path == '/health') {
      return http.Response(healthyBody, 200);
    }
    if (request.url.path == '/sheets') {
      return http.Response(notFoundBody, 404);
    }
    return http.Response('{"data": []}', 200);
  }

  Future<({
    List<http.Request> auth,
    InMemorySecureTokenStore store,
    Directory support,
  })> pumpApp(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? authHandler,
    SheetLibraryStore? libraryStore,
  }) async {
    final auth = <http.Request>[];
    final store = InMemorySecureTokenStore(token: 'stored-refresh');
    final supportDir = Directory.systemTemp.createTempSync('lgn108-logout');
    addTearDown(() => supportDir.deleteSync(recursive: true));
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(supportDir),
          secureTokenStoreProvider.overrideWithValue(store),
          if (libraryStore != null)
            sheetLibraryStoreProvider.overrideWithValue(libraryStore),
          authApiClientProvider.overrideWithValue(
            AuthApiClient(
              httpClient: MockClient((request) async {
                auth.add(request);
                return authHandler?.call(request) ?? defaultAuth(request);
              }),
            ),
          ),
          apiClientProvider.overrideWithValue(
            ApiClient(httpClient: MockClient(defaultDomain)),
          ),
          healthSchedulerProvider.overrideWithValue(ManualHealthScheduler()),
        ],
        child: const LegnarisApp(),
      ),
    );
    await tester.pumpAndSettle();
    return (auth: auth, store: store, support: supportDir);
  }

  group('logout action (FE6-06, FE6-19)', () {
    testWidgets(
      'Happy path: logout calls /auth/logout, clears storage and returns the gate to the auth screen',
      (tester) async {
        final result = await pumpApp(tester);
        expect(find.byType(AppShell), findsOneWidget);

        await tester.tap(find.byKey(const Key('shell-logout')));
        await tester.pumpAndSettle();

        final logout = result.auth.singleWhere(
          (request) => request.url.path == '/auth/logout',
        );
        expect(
          logout.headers[AuthApiClient.refreshTokenHeader],
          'refresh-1',
        );
        expect(result.store.token, isNull);
        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byType(AppShell), findsNothing);
      },
    );

    testWidgets(
      'Edge: a late refresh completing after logout does not restore the session',
      (tester) async {
        final refreshGate = Completer<http.Response>();
        var refreshCalls = 0;
        final result = await pumpApp(
          tester,
          authHandler: (request) {
            if (request.url.path == '/auth/logout') {
              return Future.value(http.Response('', 204));
            }
            if (request.url.path == '/auth/refresh') {
              return refreshCalls++ == 0
                  ? Future.value(http.Response(pairBody, 200))
                  : refreshGate.future;
            }
            return Future.value(http.Response(pairBody, 200));
          },
        );
        expect(find.byType(AppShell), findsOneWidget);

        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        final late = container.read(authSessionProvider.notifier).refresh();
        await tester.pump();

        await tester.tap(find.byKey(const Key('shell-logout')));
        await tester.pumpAndSettle();
        expect(find.byType(AuthScreen), findsOneWidget);

        refreshGate.complete(http.Response(rotatedBody, 200));
        expect(await late, isNull, reason: 'the version guard drops it');
        await tester.pumpAndSettle();

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byType(AppShell), findsNothing);
        expect(
          container.read(authSessionProvider),
          isA<AuthUnauthenticated>(),
        );
        expect(result.store.token, isNull);
      },
    );
  });

  group('session-local data at logout (DATA-01, DATA-03)', () {
    testWidgets(
      'Happy path: logout clears the library file and every draft backup on disk (DATA-01)',
      (tester) async {
        final result = await pumpApp(tester);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        container.read(sheetLibraryProvider.notifier).upsert(entry('sheet-1'));
        seedDraft(result.support);
        expect(
          SheetLibraryStore(directory: result.support).load().entries,
          hasLength(1),
        );
        expect(draftFiles(result.support), hasLength(1));

        await tester.tap(find.byKey(const Key('shell-logout')));
        await tester.pumpAndSettle();

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(
          SheetLibraryStore(directory: result.support).load().entries,
          isEmpty,
          reason: 'the empty index is what landed on disk',
        );
        expect(draftFiles(result.support), isEmpty);
      },
    );

    testWidgets(
      'Error: a failing library store never blocks logout and never skips the drafts sweep (DATA-03)',
      (tester) async {
        final result = await pumpApp(
          tester,
          libraryStore: _FailingSaveStore(),
        );
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        expect(container.read(sheetLibraryProvider).entries, hasLength(1));
        seedDraft(result.support);

        await tester.tap(find.byKey(const Key('shell-logout')));
        await tester.pumpAndSettle();

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(container.read(authSessionProvider), isA<AuthUnauthenticated>());
        expect(result.store.token, isNull);
        expect(
          container.read(sheetLibraryProvider).entries,
          isEmpty,
          reason: 'the in-memory clear lands even though the save failed',
        );
        expect(draftFiles(result.support), isEmpty, reason: 'the drafts sweep still ran');
      },
    );
  });
}

/// A library store that loads one entry but fails every save, so the
/// session-end sweep hits a real write failure (DATA-03).
class _FailingSaveStore extends SheetLibraryStore {
  _FailingSaveStore() : super(directory: Directory('/legnaris-fake'));

  @override
  LibraryLoadResult load() => LibraryLoadResult(
    outcome: LibraryLoadOutcome.loaded,
    entries: [entry('sheet-1')],
  );

  @override
  void save(List<SheetLibraryEntry> entries) {
    throw const SheetLibraryException(
      SheetLibraryErrorKind.writeFailed,
      'disk full',
    );
  }
}
