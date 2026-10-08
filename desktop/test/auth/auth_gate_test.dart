import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/auth/auth_gate.dart';
import 'package:legnaris/auth/auth_screen.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/auth/authenticated_home.dart';
import 'package:legnaris/auth/secure_token_store.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/connection/health_scheduler.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/library/sheet_library_store.dart';
import 'package:legnaris/main.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:legnaris/sheet/sheet_screen.dart';
import 'package:legnaris/shell/app_shell.dart';
import 'package:legnaris/shell/shortcut_registry.dart';
import 'package:legnaris/shell/shell_state.dart';
import 'package:legnaris/theme/app_theme.dart';

import '../connection/manual_health_scheduler.dart';
import '../library/library_fixtures.dart';
import '../sheet/draft_fixtures.dart';
import 'fakes.dart';

/// Records every `didPush` it receives, so tests can count the routes a
/// session pushes through the `AuthGate.navigatorObservers` seam (NAV-07).
/// Removals are deliberately not tracked: disposing the session navigator
/// unmounts its routes without observer notifications.
class RecordingNavigatorObserver extends NavigatorObserver {
  final List<Route<dynamic>> pushed = [];

  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) {
    pushed.add(route);
  }
}

void main() {
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const healthyBody = '{"data": {"status": "healthy"}}';
  const notFoundBody =
      '{"error": {"code": "NOT_FOUND", "message": "none"}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';
  const sheetBody =
      '{"data": {"id": "sheet-1", "template_id": "fantasy_v1", "name": "Aragorn", "data": {}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';
  const cascadeBody = '{"data": {"derived": {"armorClass": 13}}}';

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
    List<NavigatorObserver> navigatorObservers = const [],
    Directory? supportDir,
    bool settle = true,
  }) async {
    final domain = <http.Request>[];
    final auth = <http.Request>[];
    // A caller-passed directory is torn down by its own test; the default is
    // owned here.
    final ownsSupportDir = supportDir == null;
    final support =
        supportDir ?? Directory.systemTemp.createTempSync('lgn108-gate');
    if (ownsSupportDir) {
      addTearDown(() => support.deleteSync(recursive: true));
    }
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(support),
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
          // Overriding the raw transport keeps the AuthenticatedClient
          // decorator in the path, so a protected 401 drives the refresh /
          // forced-logout flow exactly as in production (NAV-03).
          apiTransportProvider.overrideWithValue(
            MockClient((request) async {
              domain.add(request);
              return domainHandler?.call(request) ?? defaultDomain(request);
            }),
          ),
          healthSchedulerProvider.overrideWithValue(
            scheduler ?? ManualHealthScheduler(),
          ),
        ],
        // The production app wires the shortcut/action registry at the root
        // MaterialApp (main.dart); the harness mirrors it so keyboard
        // dispatch (NAV-08) travels the same path.
        child: Consumer(
          builder: (context, ref, _) => MaterialApp(
            theme: buildAppTheme(),
            shortcuts: appShortcuts(),
            actions: appActions(ref),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: AuthGate(navigatorObservers: navigatorObservers),
          ),
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

  group('sheet resolution after login (FE6-13 … FE6-15, FE6-20)', () {
    Future<http.Response> withHealth(
      http.Request request,
      Future<http.Response> Function() sheets,
    ) async {
      if (request.url.path == '/health') {
        return http.Response(healthyBody, 200);
      }
      if (request.url.path == '/sheets') return sheets();
      if (request.url.path == '/sheets/sheet-1') {
        return http.Response(sheetBody, 200);
      }
      return http.Response('{"data": []}', 200);
    }

    testWidgets(
      'Happy path: 200 opens Screen 2 as a route above the mounted catalog (NAV-05)',
      (tester) async {
        final requests = await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: (request) =>
              withHealth(request, () async => http.Response(sheetBody, 200)),
        );

        expect(find.byType(SheetScreen), findsOneWidget);
        expect(
          tester.widget<SheetScreen>(find.byType(SheetScreen)).sheetId,
          'sheet-1',
        );
        expect(
          find.byType(CatalogScreen, skipOffstage: false),
          findsOneWidget,
          reason: 'the catalog stays mounted beneath the pushed Screen 2',
        );
        expect(find.byType(AppShell, skipOffstage: false), findsNWidgets(2));
        expect(countPath(requests.domain, '/sheets'), 1);
      },
    );

    testWidgets(
      'Happy path: 404 presents the create flow (the catalog) for the authenticated user',
      (tester) async {
        final requests = await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: (request) =>
              withHealth(request, () async => http.Response(notFoundBody, 404)),
        );

        expect(find.byType(CatalogScreen), findsOneWidget);
        expect(find.byType(SheetScreen), findsNothing);
        expect(countPath(requests.domain, '/sheets'), 1);
      },
    );

    testWidgets(
      'Error: a non-404 failure renders the mapped message with Retry and keeps the session',
      (tester) async {
        await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: (request) => withHealth(
            request,
            () async => http.Response(serverErrorBody, 500),
          ),
        );

        expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget);
        expect(find.byKey(const Key('home-sheet-retry')), findsOneWidget);
        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(CatalogScreen), findsNothing);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        expect(
          container.read(authSessionProvider),
          isA<AuthAuthenticated>(),
          reason: 'a lookup failure never ends the session',
        );
      },
    );

    testWidgets(
      'Happy path: Retry re-issues GET /sheets once and resolves to the create flow',
      (tester) async {
        var call = 0;
        final requests = await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: (request) => withHealth(
            request,
            () async => call++ == 0
                ? http.Response(serverErrorBody, 500)
                : http.Response(notFoundBody, 404),
          ),
        );
        expect(find.byKey(const Key('home-sheet-retry')), findsOneWidget);

        await tester.tap(find.byKey(const Key('home-sheet-retry')));
        await tester.pumpAndSettle();

        expect(countPath(requests.domain, '/sheets'), 2);
        expect(find.byType(CatalogScreen), findsOneWidget);
        expect(find.text(errorCopy(ApiErrorKind.serverError)), findsNothing);
      },
    );

    testWidgets(
      'Edge: exactly one GET /sheets per authenticated transition',
      (tester) async {
        final requests = await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: (request) =>
              withHealth(request, () async => http.Response(notFoundBody, 404)),
        );
        expect(countPath(requests.domain, '/sheets'), 1);

        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        await container.read(authSessionProvider.notifier).logout();
        await tester.pumpAndSettle();
        expect(find.byType(AuthScreen), findsOneWidget);

        await container
            .read(authSessionProvider.notifier)
            .login(email: 'player@example.com', password: 'sup3r-secret');
        await tester.pumpAndSettle();

        expect(
          countPath(requests.domain, '/sheets'),
          2,
          reason: 'one lookup per authenticated transition, never a duplicate',
        );
      },
    );
  });

  const detailBody =
      '{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';
  const unauthorizedBody =
      '{"error": {"code": "UNAUTHORIZED", "message": "revoked"}}';
  const oneTemplateBody =
      '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}';

  /// Domain responses for a session that owns `sheet-1`: the post-login
  /// lookup 404s so the catalog presents itself, one template gives the
  /// catalog a create entry, and the library open resolves the sheet plus its
  /// template detail.
  Future<http.Response> sessionDomain(http.Request request) async {
    if (request.url.path == '/health') {
      return http.Response(healthyBody, 200);
    }
    if (request.url.path == '/sheets') {
      return http.Response(notFoundBody, 404);
    }
    if (request.url.path == '/sheets/sheet-1') {
      return http.Response(sheetBody, 200);
    }
    if (request.url.path == '/templates') {
      return http.Response(oneTemplateBody, 200);
    }
    if (request.url.path == '/templates/fantasy_v1') {
      return http.Response(detailBody, 200);
    }
    return http.Response('{"data": []}', 200);
  }

  /// Domain responses for a session whose post-login lookup returns the sheet
  /// (NAV-05): Screen 2 mounts above the catalog, then serves its own fetch,
  /// the template detail, the F5 refresh and a `Ctrl+S` commit (NAV-08).
  Future<http.Response> postLoginDomain(http.Request request) async {
    if (request.url.path == '/health') {
      return http.Response(healthyBody, 200);
    }
    if (request.url.path == '/sheets') {
      return http.Response(sheetBody, 200);
    }
    if (request.url.path == '/sheets/sheet-1') {
      return http.Response(
        request.method == 'PATCH' ? cascadeBody : sheetBody,
        200,
      );
    }
    if (request.url.path == '/templates') {
      return http.Response(oneTemplateBody, 200);
    }
    if (request.url.path == '/templates/fantasy_v1') {
      return http.Response(detailBody, 200);
    }
    return http.Response('{"data": []}', 200);
  }

  /// Restores the stored session (404 lookup → catalog), opens `sheet-1`
  /// from the local library as a route above the catalog, and asserts the
  /// pre-condition: two mounted `AppShell`s (base + pushed route).
  Future<void> openPushedSheetRoute(WidgetTester tester) async {
    final container = ProviderScope.containerOf(
      tester.element(find.byType(AuthGate)),
    );
    container.read(sheetLibraryProvider.notifier).upsert(entry('sheet-1'));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('library-open-sheet-1')));
    await tester.pumpAndSettle();

    // The covered base shell reports as offstage, so count both shells
    // explicitly to pin the two-navigator layout (base + pushed route).
    expect(find.byType(SheetScreen), findsOneWidget);
    expect(find.byType(AppShell, skipOffstage: false), findsNWidgets(2));
  }

  /// Taps the topmost shell's logout action: the pushed route's own
  /// `AppShell` covers the base one, so exactly one button is hit-testable.
  Future<void> logoutTopmostShell(WidgetTester tester) async {
    final logoutButton = find.byKey(const Key('shell-logout')).hitTestable();
    expect(logoutButton, findsOneWidget);
    await tester.tap(logoutButton);
    await tester.pumpAndSettle();
  }

  /// Boots the gate with a stored session whose domain and refresh answers
  /// can be rejected on demand through the returned setter, arming the
  /// transport-driven forced logout (NAV-03).
  Future<void Function()> pumpGateWithRejectableSession(
    WidgetTester tester, {
    Directory? supportDir,
  }) async {
    var reject = false;
    await pumpGate(
      tester,
      storedToken: 'stored-refresh',
      supportDir: supportDir,
      authHandler: (request) async =>
          reject && request.url.path == '/auth/refresh'
          ? http.Response(unauthorizedBody, 401)
          : http.Response(pairBody, 200),
      domainHandler: (request) async =>
          reject ? http.Response(unauthorizedBody, 401) : sessionDomain(request),
    );
    return () => reject = true;
  }

  /// Runs a protected request after [pumpGateWithRejectableSession]'s session
  /// was armed: the domain `401` plus the rejected refresh forces the session
  /// end through the real transport path.
  Future<void> forceLogoutViaTransport(WidgetTester tester) async {
    final container = ProviderScope.containerOf(
      tester.element(find.byType(AuthGate)),
    );
    await expectLater(
      container.read(apiClientProvider).getMySheet(),
      throwsA(isA<ApiDomainException>()),
    );
    await tester.pumpAndSettle();
  }

  group('session navigator boundary (NAV-01 … NAV-04)', () {
    testWidgets(
      'Edge: while unauthenticated no catalog or sheet surface renders (NAV-04)',
      (tester) async {
        await pumpGate(tester);

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byType(CatalogScreen), findsNothing);
        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(AppShell), findsNothing);
      },
    );

    testWidgets(
      'Edge: navigatorObservers defaults to empty and receives the session navigator pushes',
      (tester) async {
        expect(const AuthGate().navigatorObservers, isEmpty);

        final observer = RecordingNavigatorObserver();
        await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: sessionDomain,
          navigatorObservers: [observer],
        );
        expect(
          observer.pushed,
          hasLength(1),
          reason: 'the session navigator pushes its initial route (NAV-02)',
        );

        await openPushedSheetRoute(tester);
        expect(
          observer.pushed,
          hasLength(2),
          reason: 'one observer push per mounted route (NAV-07 seam)',
        );
      },
    );

    testWidgets(
      'Error: a transport-driven forced logout unmounts every route pushed in the session (NAV-03)',
      (tester) async {
        var rejectDomain = false;
        final requests = await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          authHandler: (request) async =>
              rejectDomain && request.url.path == '/auth/refresh'
              ? http.Response(unauthorizedBody, 401)
              : http.Response(pairBody, 200),
          domainHandler: (request) async => rejectDomain
              ? http.Response(unauthorizedBody, 401)
              : sessionDomain(request),
        );
        await openPushedSheetRoute(tester);

        final refreshesBefore = requests.auth
            .where((request) => request.url.path == '/auth/refresh')
            .length;
        rejectDomain = true;
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        await expectLater(
          container.read(apiClientProvider).getMySheet(),
          throwsA(isA<ApiDomainException>()),
        );
        await tester.pumpAndSettle();

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(CatalogScreen), findsNothing);
        expect(find.byType(AppShell), findsNothing);
        expect(
          container.read(authSessionProvider),
          isA<AuthUnauthenticated>(),
          reason: 'the rejected refresh forced the session end',
        );
        expect(
          requests.auth
              .where((request) => request.url.path == '/auth/refresh'),
          hasLength(refreshesBefore + 1),
          reason: 'exactly one transport refresh drove the forced logout',
        );
      },
    );

    testWidgets(
      'Happy path: logout from a pushed SheetScreen route leaves only AuthScreen mounted (NAV-01)',
      (tester) async {
        await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: sessionDomain,
        );
        await openPushedSheetRoute(tester);

        await logoutTopmostShell(tester);

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(CatalogScreen), findsNothing);
        expect(find.byType(AppShell), findsNothing);
      },
    );

    testWidgets(
      'Happy path: a second login starts from an empty stack with no route from the previous session (NAV-02)',
      (tester) async {
        await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: sessionDomain,
        );
        await openPushedSheetRoute(tester);
        await logoutTopmostShell(tester);
        expect(find.byType(AuthScreen), findsOneWidget);

        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        await container.read(authSessionProvider.notifier).login(
          email: 'player@example.com',
          password: 'sup3r-secret',
        );
        await tester.pumpAndSettle();

        expect(find.byType(CatalogScreen), findsOneWidget);
        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(AppShell), findsNWidgets(1));
      },
    );
  });

  group('session-hosted dialogs (NAV-04)', () {
    testWidgets(
      'Error: a forced logout while the creation dialog is open tears it down with the session (NAV-04)',
      (tester) async {
        final rejectSession = await pumpGateWithRejectableSession(tester);

        await tester.tap(find.byKey(const Key('catalog-create-fantasy_v1')));
        await tester.pumpAndSettle();
        expect(find.byType(CreateSheetDialog), findsOneWidget);

        rejectSession();
        await forceLogoutViaTransport(tester);

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(find.byType(CatalogScreen), findsNothing);
        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(AppShell), findsNothing);
      },
    );

    testWidgets(
      'Error: a forced logout with the sheet discard confirmation open ends the session without it',
      (tester) async {
        final rejectSession = await pumpGateWithRejectableSession(tester);
        await openPushedSheetRoute(tester);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        container
            .read(sheetEditControllerProvider('sheet-1').notifier)
            .setPath('stats.dexterity', 99);
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();
        expect(
          find.byType(AlertDialog),
          findsOneWidget,
          reason: 'the discard confirmation is up before the forced logout',
        );

        rejectSession();
        await forceLogoutViaTransport(tester);

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(AppShell), findsNothing);
        expect(
          container.read(authSessionProvider),
          isA<AuthUnauthenticated>(),
          reason: 'the session ends even with the confirmation open',
        );
      },
    );

    testWidgets(
      'Error: a forced logout while the library-removal confirmation is open tears it down (NAV-04)',
      (tester) async {
        final rejectSession = await pumpGateWithRejectableSession(tester);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        container.read(sheetLibraryProvider.notifier).upsert(entry('sheet-1'));
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('library-remove-sheet-1')));
        await tester.pumpAndSettle();
        expect(
          find.byType(AlertDialog),
          findsOneWidget,
          reason: 'the removal confirmation is up before the forced logout',
        );

        rejectSession();
        await forceLogoutViaTransport(tester);

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(AuthScreen), findsOneWidget);
        expect(find.byType(CatalogScreen), findsNothing);
        expect(find.byType(AppShell), findsNothing);
        expect(
          container.read(authSessionProvider),
          isA<AuthUnauthenticated>(),
          reason: 'the session ends even with the confirmation open',
        );
      },
    );
  });

  group('post-login sheet route (NAV-02, NAV-05 … NAV-08)', () {
    testWidgets(
      'Happy path: back on the post-login Screen 2 returns to the catalog (NAV-06)',
      (tester) async {
        await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: postLoginDomain,
        );
        expect(find.byType(SheetScreen), findsOneWidget);
        expect(
          find.byType(CatalogScreen, skipOffstage: false),
          findsOneWidget,
        );

        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();

        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(CatalogScreen), findsOneWidget);
        expect(find.byType(AppShell), findsNWidgets(1));
      },
    );

    testWidgets(
      'Happy path: a dirty post-login Screen 2 confirms the discard and then returns to the catalog (NAV-06)',
      (tester) async {
        await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: postLoginDomain,
        );
        expect(find.byType(SheetScreen), findsOneWidget);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        container
            .read(sheetEditControllerProvider('sheet-1').notifier)
            .setPath('stats.dexterity', 99);
        await tester.pumpAndSettle();

        await tester.tap(find.byKey(const Key('sheet-back')));
        await tester.pumpAndSettle();
        expect(
          find.widgetWithText(TextButton, 'Discard and leave'),
          findsOneWidget,
          reason: 'pending edits must be confirmed before leaving (NAV-06)',
        );

        await tester.tap(find.widgetWithText(TextButton, 'Discard and leave'));
        await tester.pumpAndSettle();

        expect(find.byType(SheetScreen), findsNothing);
        expect(find.byType(CatalogScreen), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: a Retry after a failure plus a rebuild while in flight push exactly one Screen 2 route (NAV-07)',
      (tester) async {
        final retryResponse = Completer<http.Response>();
        var sheetLookups = 0;
        final observer = RecordingNavigatorObserver();
        final requests = await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          navigatorObservers: [observer],
          domainHandler: (request) async {
            if (request.url.path == '/sheets' && sheetLookups++ == 0) {
              return http.Response(serverErrorBody, 500);
            }
            if (request.url.path == '/sheets') return retryResponse.future;
            return sessionDomain(request);
          },
        );
        expect(find.byKey(const Key('home-sheet-retry')), findsOneWidget);
        expect(
          observer.pushed,
          hasLength(1),
          reason: 'only the initial session route is mounted so far',
        );

        await tester.tap(find.byKey(const Key('home-sheet-retry')));
        await tester.pump();
        tester.element(find.byType(AuthenticatedHome)).markNeedsBuild();
        await tester.pump();
        expect(
          countPath(requests.domain, '/sheets'),
          2,
          reason: 'the rebuild while in flight re-issues no lookup',
        );

        retryResponse.complete(http.Response(sheetBody, 200));
        await tester.pumpAndSettle();

        expect(
          observer.pushed,
          hasLength(2),
          reason: 'exactly one Screen 2 route for the whole transition (NAV-07)',
        );
        expect(find.byType(SheetScreen), findsOneWidget);
        expect(
          find.byType(CatalogScreen, skipOffstage: false),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: a 404 lookup presents the catalog without pushing Screen 2 (NAV-07)',
      (tester) async {
        final observer = RecordingNavigatorObserver();
        final requests = await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: sessionDomain,
          navigatorObservers: [observer],
        );

        expect(find.byType(CatalogScreen), findsOneWidget);
        expect(find.byType(SheetScreen), findsNothing);
        expect(
          observer.pushed,
          hasLength(1),
          reason: 'no Screen 2 route for a 404 lookup (NAV-07)',
        );
        expect(countPath(requests.domain, '/sheets'), 1);
      },
    );

    testWidgets(
      'Happy path: F5 and Ctrl+S dispatch to the post-login-mounted SheetScreen (NAV-08)',
      (tester) async {
        final requests = await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          domainHandler: postLoginDomain,
        );
        expect(find.byType(SheetScreen), findsOneWidget);
        expect(
          find.byType(CatalogScreen, skipOffstage: false),
          findsOneWidget,
        );

        final detailLoads = countPath(requests.domain, '/templates/fantasy_v1');
        final catalogLoads = countPath(requests.domain, '/templates');
        await tester.sendKeyEvent(LogicalKeyboardKey.f5);
        await tester.pumpAndSettle();

        expect(
          countPath(requests.domain, '/templates/fantasy_v1'),
          detailLoads + 1,
          reason: 'F5 refreshes the mounted SheetScreen',
        );
        expect(
          countPath(requests.domain, '/templates'),
          catalogLoads,
          reason: 'F5 never touches the catalog while a sheet is mounted',
        );

        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        container
            .read(sheetEditControllerProvider('sheet-1').notifier)
            .setPath('stats.dexterity', 99);
        await tester.pumpAndSettle();

        await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
        await tester.sendKeyEvent(LogicalKeyboardKey.keyS);
        await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
        await tester.pumpAndSettle();

        final patches = requests.domain
            .where((request) => request.method == 'PATCH')
            .toList();
        expect(
          patches,
          hasLength(1),
          reason: 'Ctrl+S commits the pending edit exactly once (NAV-08)',
        );
        expect(patches.single.url.path, '/sheets/sheet-1');
      },
    );

    testWidgets(
      'Happy path: a session starting from a stuck shell surface resets it to the catalog (NAV-02)',
      (tester) async {
        await pumpGate(tester);
        expect(find.byType(AuthScreen), findsOneWidget);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        container
            .read(shellStateProvider.notifier)
            .show(ShellSurface.creationDialog);
        expect(container.read(shellStateProvider), ShellSurface.creationDialog);

        await container.read(authSessionProvider.notifier).login(
          email: 'player@example.com',
          password: 'sup3r-secret',
        );
        await tester.pumpAndSettle();

        expect(
          container.read(shellStateProvider),
          ShellSurface.catalog,
          reason: 'the branch mount clears the surface the session inherited',
        );
        expect(find.byType(CatalogScreen), findsOneWidget);
      },
    );
  });

  group('session-local data at session end (DATA-02, DATA-04)', () {
    late Directory support;

    setUp(() {
      support = Directory.systemTemp.createTempSync('lgn108-sweep');
      addTearDown(() => support.deleteSync(recursive: true));
    });

    testWidgets(
      'Error: a transport-driven forced logout sweeps the library and every draft (DATA-02)',
      (tester) async {
        final rejectSession = await pumpGateWithRejectableSession(
          tester,
          supportDir: support,
        );
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        container.read(sheetLibraryProvider.notifier).upsert(entry('sheet-1'));
        seedDraft(support);
        await tester.pumpAndSettle();
        expect(draftFiles(support), hasLength(1));

        rejectSession();
        await forceLogoutViaTransport(tester);

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(
          SheetLibraryStore(directory: support).load().entries,
          isEmpty,
          reason: 'the forced logout sweeps the local index (DATA-02)',
        );
        expect(draftFiles(support), isEmpty);
      },
    );

    testWidgets(
      'Edge: the no-stored-token first-launch path sweeps nothing',
      (tester) async {
        SheetLibraryStore(directory: support).save([entry('sheet-1')]);
        seedDraft(support);

        final requests = await pumpGate(tester, supportDir: support);

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(
          requests.auth,
          isEmpty,
          reason: 'no restore runs without a stored token',
        );
        expect(
          SheetLibraryStore(directory: support).load().entries,
          hasLength(1),
          reason: 'the first-launch path never sweeps local data',
        );
        expect(draftFiles(support), hasLength(1));
      },
    );

    testWidgets(
      'Edge: a transient restore failure falls back to the auth screen keeping the token, the library and the drafts (DATA-04)',
      (tester) async {
        SheetLibraryStore(directory: support).save([entry('sheet-1')]);
        seedDraft(support);

        await pumpGate(
          tester,
          storedToken: 'stored-refresh',
          supportDir: support,
          authHandler: (request) async => http.Response(serverErrorBody, 500),
        );

        expect(find.byType(AuthScreen), findsOneWidget);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        expect(
          await container.read(secureTokenStoreProvider).read(),
          'stored-refresh',
          reason: 'the token survives for the next launch (DATA-04)',
        );
        expect(
          SheetLibraryStore(directory: support).load().entries,
          hasLength(1),
          reason: 'the transient path never sweeps the library',
        );
        expect(draftFiles(support), hasLength(1), reason: 'nor the drafts');
      },
    );
  });
}
