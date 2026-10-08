/// Dirty-logout confirmation flow (DATA-05 … DATA-07): pending edits at tap
/// time open the confirmation on the session navigator, only a confirm ends
/// the session, and cancel via button, Esc or barrier keeps everything.
///
/// The session boots straight into a pushed Screen 2 route, so two
/// `AppShell`s are mounted and every tap targets the topmost (hit-testable)
/// `shell-logout`.
library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/auth/auth_gate.dart';
import 'package:legnaris/auth/auth_screen.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/auth/secure_token_store.dart';
import 'package:legnaris/connection/health_scheduler.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/library/sheet_library_store.dart';
import 'package:legnaris/main.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:legnaris/sheet/sheet_refresh.dart';
import 'package:legnaris/sheet/sheet_screen.dart';
import 'package:legnaris/shell/app_shell.dart';

import '../auth/fakes.dart';
import '../connection/manual_health_scheduler.dart';
import '../library/library_fixtures.dart';
import '../sheet/draft_fixtures.dart';

/// One booted session: the recorded auth requests (to count
/// `/auth/logout`), the support directory seeded with session-local files,
/// the in-memory token store, and the setter arming the forced logout.
typedef _Session = ({
  List<http.Request> auth,
  Directory support,
  InMemorySecureTokenStore store,
  void Function() rejectSession,
});

void main() {
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const healthyBody = '{"data": {"status": "healthy"}}';
  const unauthorizedBody =
      '{"error": {"code": "UNAUTHORIZED", "message": "revoked"}}';
  const sheetBody =
      '{"data": {"id": "12345678-1234-7000-8000-123456789abc", "template_id": "fantasy_v1", "name": "Aragorn", "data": {}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';
  const oneTemplateBody =
      '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}';
  const detailBody =
      '{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';

  /// Domain responses for a session whose post-login lookup resolves the
  /// fixture sheet: Screen 2 mounts as a route above the catalog, then
  /// serves its own fetch and the template detail.
  Future<http.Response> sessionDomain(http.Request request) async {
    if (request.url.path == '/health') {
      return http.Response(healthyBody, 200);
    }
    if (request.url.path == '/sheets') {
      return http.Response(sheetBody, 200);
    }
    if (request.url.path == '/sheets/$draftSheetId') {
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

  /// Boots the app with a stored session and arms the transport-driven
  /// forced logout through the returned setter: with it flipped, the domain
  /// answers `401` and the refresh is rejected, so any protected call ends
  /// the session through the real transport path.
  Future<_Session> pumpSession(WidgetTester tester) async {
    final auth = <http.Request>[];
    var reject = false;
    final support = Directory.systemTemp.createTempSync('lgn-dsb-logout');
    addTearDown(() => support.deleteSync(recursive: true));
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    final store = InMemorySecureTokenStore(token: 'stored-refresh');
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(support),
          secureTokenStoreProvider.overrideWithValue(store),
          authApiClientProvider.overrideWithValue(
            AuthApiClient(
              httpClient: MockClient((request) async {
                auth.add(request);
                if (reject && request.url.path == '/auth/refresh') {
                  return http.Response(unauthorizedBody, 401);
                }
                if (request.url.path == '/auth/logout') {
                  return http.Response('', 204);
                }
                return http.Response(pairBody, 200);
              }),
            ),
          ),
          // Overriding the raw transport keeps the AuthenticatedClient
          // decorator in the path, so a protected 401 drives the rejected
          // refresh / forced-logout flow exactly as in production.
          apiTransportProvider.overrideWithValue(
            MockClient(
              (request) async => reject
                  ? http.Response(unauthorizedBody, 401)
                  : sessionDomain(request),
            ),
          ),
          healthSchedulerProvider.overrideWithValue(ManualHealthScheduler()),
        ],
        child: const LegnarisApp(),
      ),
    );
    await tester.pump();
    await tester.pumpAndSettle();
    expect(find.byType(SheetScreen), findsOneWidget);
    expect(
      find.byType(AppShell, skipOffstage: false),
      findsNWidgets(2),
      reason: 'precondition: the pushed route mounts a second AppShell',
    );
    return (
      auth: auth,
      support: support,
      store: store,
      rejectSession: () => reject = true,
    );
  }

  /// Brings the session into the dirty state: a pending edit on the mounted
  /// sheet plus the session-local files a cancel must keep (DATA-06).
  Future<({_Session app, ProviderContainer container})> pumpDirtySession(
    WidgetTester tester,
  ) async {
    final app = await pumpSession(tester);
    final container = ProviderScope.containerOf(
      tester.element(find.byType(AuthGate)),
    );
    container
        .read(sheetEditControllerProvider(draftSheetId).notifier)
        .setPath('stats.dexterity', 99);
    container.read(sheetLibraryProvider.notifier).upsert(entry(draftSheetId));
    seedDraft(app.support);
    await tester.pumpAndSettle();
    expect(
      container.read(sheetEditControllerProvider(draftSheetId)).isPending,
      isTrue,
      reason: 'precondition: the sheet has pending edits',
    );
    return (app: app, container: container);
  }

  /// Taps the topmost shell's logout action: the pushed route's own
  /// `AppShell` covers the base one, so exactly one button is hit-testable.
  Future<void> tapTopmostLogout(WidgetTester tester) async {
    final logoutButton = find.byKey(const Key('shell-logout')).hitTestable();
    expect(logoutButton, findsOneWidget);
    await tester.tap(logoutButton);
    await tester.pumpAndSettle();
  }

  /// Asserts the session-local files a cancel must leave untouched
  /// (DATA-06): the library still lists the seeded entry on disk and the
  /// seeded draft still exists.
  void expectLocalFilesKept(Directory support) {
    expect(
      SheetLibraryStore(
        directory: support,
      ).load().entries.map((stored) => stored.id),
      contains(draftSheetId),
      reason: 'the persisted library survives the cancel',
    );
    expect(draftFiles(support), hasLength(1), reason: 'the drafts survive');
  }

  void expectNoLogoutRequest(List<http.Request> auth) {
    expect(
      auth.where((request) => request.url.path == '/auth/logout'),
      isEmpty,
      reason: 'a cancelled confirmation never sends /auth/logout (DATA-06)',
    );
  }

  group('dirty logout confirmation (DATA-05 … DATA-07)', () {
    testWidgets(
      'Happy path: pending edits open the confirmation and only a confirm ends the session (DATA-05)',
      (tester) async {
        final (:app, :container) = await pumpDirtySession(tester);

        await tapTopmostLogout(tester);

        expect(find.byType(AlertDialog), findsOneWidget, reason: 'DATA-05');
        expect(find.text('Log out with unsaved edits?'), findsOneWidget);
        expect(
          find.text('Your unsaved edits will be discarded when you log out.'),
          findsOneWidget,
        );

        await tester.tap(find.widgetWithText(TextButton, 'Discard and log out'));
        await tester.pumpAndSettle();

        expect(find.byType(AuthScreen), findsOneWidget);
        expect(
          container.read(authSessionProvider),
          isA<AuthUnauthenticated>(),
        );
        expect(
          app.auth.where((request) => request.url.path == '/auth/logout'),
          hasLength(1),
        );
        expect(
          SheetLibraryStore(directory: app.support).load().entries,
          isEmpty,
          reason: 'confirm proceeds with the session-end sweep (DATA-01)',
        );
        expect(draftFiles(app.support), isEmpty);
      },
    );

    testWidgets(
      'Edge: cancel via the action button keeps the session, the edits and the files (DATA-06)',
      (tester) async {
        final (:app, :container) = await pumpDirtySession(tester);
        await tapTopmostLogout(tester);
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tap(find.widgetWithText(TextButton, 'Cancel'));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(SheetScreen), findsOneWidget);
        expect(
          container.read(authSessionProvider),
          isA<AuthAuthenticated>(),
          reason: 'the session survives the cancel',
        );
        expect(
          app.store.token,
          'refresh-1',
          reason: 'the rotated token from the restore survives the cancel',
        );
        expectNoLogoutRequest(app.auth);
        expect(
          container.read(sheetEditControllerProvider(draftSheetId)).isPending,
          isTrue,
          reason: 'the pending edits survive the cancel',
        );
        expectLocalFilesKept(app.support);
      },
    );

    testWidgets(
      'Edge: Esc dismisses the confirmation without logging out (DATA-06)',
      (tester) async {
        final (:app, :container) = await pumpDirtySession(tester);
        await tapTopmostLogout(tester);
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(
          container.read(authSessionProvider),
          isA<AuthAuthenticated>(),
        );
        expectNoLogoutRequest(app.auth);
        expect(
          container.read(sheetEditControllerProvider(draftSheetId)).isPending,
          isTrue,
        );
        expectLocalFilesKept(app.support);
      },
    );

    testWidgets(
      'Edge: a barrier tap dismisses the confirmation without logging out (DATA-06)',
      (tester) async {
        final (:app, :container) = await pumpDirtySession(tester);
        await tapTopmostLogout(tester);
        expect(find.byType(AlertDialog), findsOneWidget);

        await tester.tapAt(const Offset(4, 4));
        await tester.pumpAndSettle();

        expect(find.byType(AlertDialog), findsNothing);
        expect(
          container.read(authSessionProvider),
          isA<AuthAuthenticated>(),
        );
        expectNoLogoutRequest(app.auth);
        expect(
          container.read(sheetEditControllerProvider(draftSheetId)).isPending,
          isTrue,
        );
        expectLocalFilesKept(app.support);
      },
    );

    testWidgets(
      'Happy path: a clean active sheet logs out with no confirmation (DATA-07)',
      (tester) async {
        final app = await pumpSession(tester);
        final container = ProviderScope.containerOf(
          tester.element(find.byType(AuthGate)),
        );
        expect(
          container.read(activeSheetIdProvider),
          draftSheetId,
          reason: 'precondition: an active sheet without pending edits',
        );

        await tapTopmostLogout(tester);

        expect(
          find.byType(AlertDialog),
          findsNothing,
          reason: 'DATA-07: no confirmation without pending edits',
        );
        expect(find.byType(AuthScreen), findsOneWidget);
        expect(
          container.read(authSessionProvider),
          isA<AuthUnauthenticated>(),
        );
        expect(
          app.auth.where((request) => request.url.path == '/auth/logout'),
          hasLength(1),
        );
        expect(app.store.token, isNull);
      },
    );

    testWidgets(
      'Error: a forced logout during the confirmation ends the flow without a logout request (DATA-06)',
      (tester) async {
        final (:app, :container) = await pumpDirtySession(tester);
        await tapTopmostLogout(tester);
        expect(find.byType(AlertDialog), findsOneWidget);

        app.rejectSession();
        await expectLater(
          container.read(apiClientProvider).getMySheet(),
          throwsA(isA<ApiDomainException>()),
        );
        await tester.pumpAndSettle();

        expect(
          find.byType(AlertDialog),
          findsNothing,
          reason: 'the confirmation dies with the session',
        );
        expect(find.byType(AuthScreen), findsOneWidget);
        expect(
          container.read(authSessionProvider),
          isA<AuthUnauthenticated>(),
          reason: 'the rejected refresh forced the session end',
        );
        expectNoLogoutRequest(app.auth);
      },
    );
  });
}
