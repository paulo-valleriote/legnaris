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
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/main.dart';
import 'package:legnaris/shell/app_shell.dart';
import 'package:legnaris/theme/app_theme.dart';

import '../connection/manual_health_scheduler.dart';
import 'fakes.dart';

void main() {
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const healthyBody = '{"data": {"status": "healthy"}}';
  const notFoundBody =
      '{"error": {"code": "NOT_FOUND", "message": "none"}}';

  Future<http.Response> defaultDomain(http.Request request) async {
    if (request.url.path == '/health') {
      return http.Response(healthyBody, 200);
    }
    if (request.url.path == '/sheets') {
      return http.Response(notFoundBody, 404);
    }
    return http.Response('{"data": []}', 200);
  }

  Future<({List<http.Request> domain, List<http.Request> auth})> pumpGate(
    WidgetTester tester, {
    String? storedToken,
    Future<http.Response> Function(http.Request request)? domainHandler,
    Future<http.Response> Function(http.Request request)? authHandler,
    ManualHealthScheduler? scheduler,
    bool settle = true,
  }) async {
    final domain = <http.Request>[];
    final auth = <http.Request>[];
    final supportDir = Directory.systemTemp.createTempSync('lgn108-gate');
    addTearDown(() => supportDir.deleteSync(recursive: true));
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(supportDir),
          secureTokenStoreProvider.overrideWithValue(
            InMemorySecureTokenStore(token: storedToken),
          ),
          authApiClientProvider.overrideWithValue(
            AuthApiClient(
              httpClient: MockClient((request) async {
                auth.add(request);
                return authHandler?.call(request) ??
                    http.Response(pairBody, 200);
              }),
            ),
          ),
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                domain.add(request);
                return domainHandler?.call(request) ?? defaultDomain(request);
              }),
            ),
          ),
          healthSchedulerProvider.overrideWithValue(
            scheduler ?? ManualHealthScheduler(),
          ),
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const AuthGate(),
        ),
      ),
    );
    await tester.pump();
    if (settle) await tester.pumpAndSettle();
    return (domain: domain, auth: auth);
  }

  int countPath(List<http.Request> requests, String path) =>
      requests.where((request) => request.url.path == path).length;

  group('gate render (FE6-12, FE6-16)', () {
    testWidgets(
      'Edge: with no stored session only the auth screen renders and no domain request is made',
      (tester) async {
        final requests = await pumpGate(tester);

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byType(AppShell), findsNothing);
        expect(requests.auth, isEmpty);
        expect(countPath(requests.domain, '/sheets'), 0);
      },
    );

    testWidgets(
      'Happy path: a stored session restores without prompting and renders the shell',
      (tester) async {
        final requests = await pumpGate(tester, storedToken: 'stored-refresh');

        expect(find.byType(AppShell), findsOneWidget);
        expect(find.byType(AuthScreen), findsNothing);
        expect(requests.auth.single.url.path, '/auth/refresh');
        expect(
          requests.auth.single.headers[AuthApiClient.refreshTokenHeader],
          'stored-refresh',
        );
      },
    );

    testWidgets(
      'Happy path: main.dart mounts AuthGate as the home surface',
      (tester) async {
        final supportDir = Directory.systemTemp.createTempSync('lgn108-home');
        addTearDown(() => supportDir.deleteSync(recursive: true));
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              appSupportDirectoryProvider.overrideWithValue(supportDir),
              secureTokenStoreProvider.overrideWithValue(
                InMemorySecureTokenStore(),
              ),
              authApiClientProvider.overrideWithValue(
                AuthApiClient(
                  httpClient: MockClient(
                    (request) async => http.Response(pairBody, 200),
                  ),
                ),
              ),
              apiClientProvider.overrideWithValue(
                ApiClient(httpClient: MockClient(defaultDomain)),
              ),
              healthSchedulerProvider.overrideWithValue(
                ManualHealthScheduler(),
              ),
            ],
            child: const LegnarisApp(),
          ),
        );
        await tester.pump();

        final app = tester.widget<MaterialApp>(find.byType(MaterialApp));
        expect(app.home, isA<AuthGate>());
      },
    );

    testWidgets(
      'Edge: the initial restore shows the loading state until the refresh settles',
      (tester) async {
        final gate = Completer<http.Response>();
        final requests = await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          settle: false,
          authHandler: (request) => gate.future,
        );

        expect(find.byKey(const Key('auth-restoring')), findsOneWidget);
        expect(find.byType(AuthScreen), findsNothing);
        expect(countPath(requests.domain, '/sheets'), 0);

        gate.complete(http.Response(pairBody, 200));
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('auth-restoring')), findsNothing);
        expect(find.byType(AppShell), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: a login in flight keeps the auth screen mounted, not the restoring state',
      (tester) async {
        final loginGate = Completer<http.Response>();
        await pumpGate(
          tester,
          authHandler: (request) => request.url.path == '/auth/login'
              ? loginGate.future
              : Future<http.Response>.value(http.Response(pairBody, 200)),
        );

        expect(find.byType(AuthScreen), findsOneWidget);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        final login = container
            .read(authSessionProvider.notifier)
            .login(email: 'player@example.com', password: 'sup3r-secret');
        await tester.pump();

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byKey(const Key('auth-restoring')), findsNothing);

        loginGate.complete(http.Response(pairBody, 200));
        await login;
        await tester.pumpAndSettle();

        expect(find.byType(AppShell), findsOneWidget);
      },
    );
  });

  group('public health while unauthenticated (FE6-16)', () {
    testWidgets(
      'Edge: GET /health still polls and updates the indicator while unauthenticated',
      (tester) async {
        final scheduler = ManualHealthScheduler();
        final requests = await pumpGate(tester, scheduler: scheduler);

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(
          countPath(requests.domain, '/health'),
          1,
          reason: 'the boot health check runs while unauthenticated',
        );

        scheduler.fire();
        await tester.pumpAndSettle();

        expect(
          countPath(requests.domain, '/health'),
          2,
          reason: 'the public poll keeps running while unauthenticated',
        );
      },
    );
  });
}
