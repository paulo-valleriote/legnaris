import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/auth/auth_screen.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/auth/secure_token_store.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/app_theme.dart';

import 'fakes.dart';

void main() {
  const userBody =
      '{"data": {"id": "user-1", "email": "player@example.com", "username": "player", "created_at": "2026-09-20T12:00:00Z"}}';
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const unauthorizedBody =
      '{"error": {"code": "UNAUTHORIZED", "message": "invalid credentials"}}';
  const invalidBody =
      '{"error": {"code": "INVALID_REQUEST", "message": "password must be at least 8 bytes"}}';

  Future<List<http.Request>> pumpScreen(
    WidgetTester tester, {
    required Future<http.Response> Function(http.Request request) handler,
    InMemorySecureTokenStore? store,
  }) async {
    final recorded = <http.Request>[];
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          secureTokenStoreProvider.overrideWithValue(
            store ?? InMemorySecureTokenStore(),
          ),
          authApiClientProvider.overrideWithValue(
            AuthApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
                return handler(request);
              }),
            ),
          ),
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const AuthScreen(),
        ),
      ),
    );
    await tester.pump();
    return recorded;
  }

  AuthState sessionState(WidgetTester tester) => ProviderScope.containerOf(
    tester.element(find.byType(AuthScreen)),
  ).read(authSessionProvider);

  String fieldText(WidgetTester tester, String key) =>
      tester.widget<TextField>(find.byKey(Key(key))).controller!.text;

  Future<void> fillLogin(
    WidgetTester tester, {
    String email = 'player@example.com',
    String password = 'sup3r-secret',
  }) async {
    await tester.enterText(find.byKey(const Key('auth-email')), email);
    await tester.enterText(find.byKey(const Key('auth-password')), password);
  }

  group('login mode (FE6-03, FE6-05)', () {
    testWidgets(
      'Happy path: submits credentials through the session and persists the refresh token',
      (tester) async {
        final store = InMemorySecureTokenStore();
        final recorded = await pumpScreen(
          tester,
          store: store,
          handler: (request) async => http.Response(pairBody, 200),
        );

        await fillLogin(tester);
        await tester.tap(find.byKey(const Key('auth-submit')));
        await tester.pumpAndSettle();

        final login = recorded.single;
        expect(login.method, 'POST');
        expect(login.url.path, '/auth/login');
        expect(jsonDecode(login.body), {
          'email': 'player@example.com',
          'password': 'sup3r-secret',
        });
        expect(sessionState(tester), isA<AuthAuthenticated>());
        expect(store.token, 'refresh-1');
      },
    );

    testWidgets(
      'Edge: login mode shows email and password but never the username field',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (request) async => http.Response(pairBody, 200),
        );

        expect(find.byKey(const Key('auth-email')), findsOneWidget);
        expect(find.byKey(const Key('auth-password')), findsOneWidget);
        expect(find.byKey(const Key('auth-username')), findsNothing);
        expect(find.widgetWithText(TextButton, 'Log in'), findsWidgets);
      },
    );
  });

  group('register mode (FE6-04, FE6-21)', () {
    testWidgets(
      'Happy path: register mode submits the three fields, registers then logs in',
      (tester) async {
        final store = InMemorySecureTokenStore();
        final recorded = await pumpScreen(
          tester,
          store: store,
          handler: (request) async => http.Response(
            request.url.path == '/auth/register' ? userBody : pairBody,
            request.url.path == '/auth/register' ? 201 : 200,
          ),
        );

        await tester.tap(find.byKey(const Key('auth-mode-register')));
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('auth-username')), findsOneWidget);
        expect(find.widgetWithText(TextButton, 'Create account'), findsOneWidget);

        await tester.enterText(
          find.byKey(const Key('auth-email')),
          'player@example.com',
        );
        await tester.enterText(find.byKey(const Key('auth-username')), 'player');
        await tester.enterText(
          find.byKey(const Key('auth-password')),
          'sup3r-secret',
        );
        await tester.tap(find.byKey(const Key('auth-submit')));
        await tester.pumpAndSettle();

        expect(
          recorded.map((request) => request.url.path).toList(),
          ['/auth/register', '/auth/login'],
        );
        expect(jsonDecode(recorded.first.body), {
          'email': 'player@example.com',
          'username': 'player',
          'password': 'sup3r-secret',
        });
        expect(sessionState(tester), isA<AuthAuthenticated>());
        expect(store.token, 'refresh-1');
      },
    );

    testWidgets(
      'Edge: switching back to login hides the username field',
      (tester) async {
        await pumpScreen(
          tester,
          handler: (request) async => http.Response(pairBody, 200),
        );

        await tester.tap(find.byKey(const Key('auth-mode-register')));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('auth-username')), findsOneWidget);

        await tester.tap(find.byKey(const Key('auth-mode-login')));
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('auth-username')), findsNothing);
      },
    );
  });

  group('mapped errors keep the entered fields (FE6-05, FE6-21)', () {
    testWidgets(
      'Error: a failed login renders the mapped message and keeps the entered fields',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (request) async => http.Response(unauthorizedBody, 401),
        );

        await fillLogin(tester, password: 'wrong-secret');
        await tester.tap(find.byKey(const Key('auth-submit')));
        await tester.pumpAndSettle();

        expect(recorded, hasLength(1));
        expect(find.text(errorCopy(ApiErrorKind.unknown)), findsOneWidget);
        expect(fieldText(tester, 'auth-email'), 'player@example.com');
        expect(fieldText(tester, 'auth-password'), 'wrong-secret');
        expect(sessionState(tester), isA<AuthUnauthenticated>());
      },
    );

    testWidgets(
      'Error: a rejected registration renders the mapped message and keeps the entered fields',
      (tester) async {
        final recorded = await pumpScreen(
          tester,
          handler: (request) async => http.Response(invalidBody, 400),
        );

        await tester.tap(find.byKey(const Key('auth-mode-register')));
        await tester.pumpAndSettle();
        await tester.enterText(
          find.byKey(const Key('auth-email')),
          'player@example.com',
        );
        await tester.enterText(find.byKey(const Key('auth-username')), 'player');
        await tester.enterText(find.byKey(const Key('auth-password')), 'short');
        await tester.tap(find.byKey(const Key('auth-submit')));
        await tester.pumpAndSettle();

        expect(recorded.map((request) => request.url.path), ['/auth/register']);
        expect(find.text(errorCopy(ApiErrorKind.invalidEdit)), findsOneWidget);
        expect(fieldText(tester, 'auth-email'), 'player@example.com');
        expect(fieldText(tester, 'auth-username'), 'player');
        expect(fieldText(tester, 'auth-password'), 'short');
        expect(sessionState(tester), isA<AuthUnauthenticated>());
      },
    );
  });

  group('pending state (FE6-05)', () {
    testWidgets(
      'Edge: the primary action disables and progress shows while the request is in flight',
      (tester) async {
        final gate = Completer<http.Response>();
        await pumpScreen(tester, handler: (request) => gate.future);

        await fillLogin(tester);
        await tester.tap(find.byKey(const Key('auth-submit')));
        await tester.pump();

        expect(
          tester
              .widget<TextButton>(find.byKey(const Key('auth-submit')))
              .onPressed,
          isNull,
        );
        expect(find.byKey(const Key('auth-progress')), findsOneWidget);

        gate.complete(http.Response(pairBody, 200));
        await tester.pumpAndSettle();

        expect(
          tester
              .widget<TextButton>(find.byKey(const Key('auth-submit')))
              .onPressed,
          isNotNull,
        );
        expect(find.byKey(const Key('auth-progress')), findsNothing);
      },
    );
  });
}
