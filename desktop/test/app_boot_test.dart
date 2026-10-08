import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/auth/secure_token_store.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/connection/health_scheduler.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/main.dart';
import 'package:legnaris/shell/app_shell.dart';
import 'package:legnaris/shell/shell_state.dart';

import 'auth/fakes.dart';
import 'connection/manual_health_scheduler.dart';

void main() {
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const notFoundBody =
      '{"error": {"code": "NOT_FOUND", "message": "none"}}';
  const oneTemplateBody =
      '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}';
  const twoTemplatesBody = '''
{"data": [
  {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}},
  {"id": "sci_fi_v1", "name": "Sci Fi Template", "default": false, "fields": {}}
]}
''';
  const detailBody =
      '{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';
  const healthyBody = '{"data": {"status": "healthy"}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

  Future<List<http.Request>> pumpApp(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? handler,
  }) async {
    final recorded = <http.Request>[];
    final supportDir = Directory.systemTemp.createTempSync('lgn108-boot');
    addTearDown(() => supportDir.deleteSync(recursive: true));
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(supportDir),
          secureTokenStoreProvider.overrideWithValue(
            InMemorySecureTokenStore(token: 'stored-refresh'),
          ),
          authApiClientProvider.overrideWithValue(
            AuthApiClient(
              httpClient: MockClient((request) async {
                if (request.url.path == '/auth/refresh') {
                  return http.Response(pairBody, 200);
                }
                return http.Response(
                  '{"error": {"code": "UNAUTHORIZED", "message": "no"}}',
                  401,
                );
              }),
            ),
          ),
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
                if (request.url.path == '/sheets') {
                  return http.Response(notFoundBody, 404);
                }
                if (handler != null) return handler(request);
                return http.Response(
                  request.url.path == '/templates'
                      ? twoTemplatesBody
                      : detailBody,
                  200,
                );
              }),
            ),
          ),
          healthSchedulerProvider.overrideWithValue(ManualHealthScheduler()),
        ],
        child: const LegnarisApp(),
      ),
    );
    await tester.pumpAndSettle();
    return recorded;
  }

  ShellSurface currentSurface(WidgetTester tester) =>
      ProviderScope.containerOf(
        tester.element(find.byType(MaterialApp)),
      ).read(shellStateProvider);

  int listRequestCount(List<http.Request> recorded) => recorded
      .where((request) => request.url.path == '/templates')
      .length;

  int healthRequestCount(List<http.Request> recorded) => recorded
      .where((request) => request.url.path == '/health')
      .length;

  group('boot (CAT-01, CAT-05, CAT-07)', () {
    testWidgets('Happy path: booting renders the shell with the header slot and issues exactly one GET /templates (CAT-01/05/07)', (tester) async {
      final recorded = await pumpApp(tester);

      expect(find.byType(AppShell), findsOneWidget);
      expect(find.byType(Header), findsOneWidget);
      expect(find.byKey(const Key('connection-indicator-slot')), findsOneWidget);
      expect(listRequestCount(recorded), 1);
      final listRequest = recorded.singleWhere(
        (request) => request.url.path == '/templates',
      );
      expect(listRequest.method, 'GET');
      expect(healthRequestCount(recorded), 1, reason: 'exactly one boot health check');
      expect(find.text('Legnaris'), findsOneWidget);
      expect(find.byType(Card), findsNWidgets(2));
    });

    testWidgets('Happy path: MaterialApp.title comes from appTitleProvider, never a hardcoded string', (tester) async {
      final supportDir = Directory.systemTemp.createTempSync('lgn108-boot-title');
      addTearDown(() => supportDir.deleteSync(recursive: true));
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            appSupportDirectoryProvider.overrideWithValue(supportDir),
            appTitleProvider.overrideWithValue('Legnaris (overridden)'),
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
              ApiClient(
                httpClient: MockClient(
                  (request) async => http.Response(
                    request.url.path == '/health'
                        ? '{"data": {"status": "healthy"}}'
                        : '{"data": []}',
                    200,
                  ),
                ),
              ),
            ),
            healthSchedulerProvider.overrideWithValue(ManualHealthScheduler()),
          ],
          child: const LegnarisApp(),
        ),
      );

      expect(
        tester.widget<MaterialApp>(find.byType(MaterialApp)).title,
        'Legnaris (overridden)',
      );
    });
  });

  group('shortcuts in the wired app (CAT-18, CAT-19, edges)', () {
    testWidgets('Edge: a failed F5 keeps the cached list and shows the mapped inline error (CAT-19 in situ)', (tester) async {
      var call = 0;
      final recorded = await pumpApp(
        tester,
        handler: (request) async {
          if (request.url.path == '/health') {
            return http.Response(healthyBody, 200);
          }
          final first = call++ == 0;
          return http.Response(
            first ? twoTemplatesBody : serverErrorBody,
            first ? 200 : 500,
          );
        },
      );
      expect(find.byType(Card), findsNWidgets(2));

      await tester.sendKeyEvent(LogicalKeyboardKey.f5);
      await tester.pumpAndSettle();

      expect(listRequestCount(recorded), 2);
      expect(healthRequestCount(recorded), 1, reason: 'F5 issues no extra health check');
      expect(find.byType(Card), findsNWidgets(2));
      expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget);
      expect(find.widgetWithText(OutlinedButton, 'Retry'), findsOneWidget);
    });

    testWidgets('Edge: F5 with the creation dialog open issues no catalog request', (tester) async {
      final recorded = await pumpApp(tester);
      expect(listRequestCount(recorded), 1);

      await tester.tap(find.text('Fantasy Template'));
      await tester.pumpAndSettle();
      expect(find.byType(CreateSheetDialog), findsOneWidget);
      expect(currentSurface(tester), ShellSurface.creationDialog);

      await tester.sendKeyEvent(LogicalKeyboardKey.f5);
      await tester.pumpAndSettle();

      expect(listRequestCount(recorded), 1);
      expect(find.byType(CreateSheetDialog), findsOneWidget);
    });

    testWidgets('Happy path: Esc closes the dialog through the registry with no refetch (closeOverlay)', (tester) async {
      final recorded = await pumpApp(tester);
      await tester.tap(find.text('Fantasy Template'));
      await tester.pumpAndSettle();
      expect(find.byType(CreateSheetDialog), findsOneWidget);

      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pumpAndSettle();

      expect(find.byType(CreateSheetDialog), findsNothing);
      expect(find.byType(Card), findsNWidgets(2));
      expect(currentSurface(tester), ShellSurface.catalog);
      expect(listRequestCount(recorded), 1);
    });

    testWidgets('Happy path: F5 on the catalog re-requests and replaces the list (CAT-18)', (tester) async {
      var call = 0;
      final recorded = await pumpApp(
        tester,
        handler: (request) async {
          if (request.url.path == '/health') {
            return http.Response(healthyBody, 200);
          }
          return http.Response(
            call++ == 0 ? oneTemplateBody : twoTemplatesBody,
            200,
          );
        },
      );
      expect(find.byType(Card), findsOneWidget);
      expect(find.text('Sci Fi Template'), findsNothing);

      await tester.sendKeyEvent(LogicalKeyboardKey.f5);
      await tester.pumpAndSettle();

      expect(listRequestCount(recorded), 2);
      expect(find.byType(Card), findsNWidgets(2));
      expect(find.text('Sci Fi Template'), findsOneWidget);
    });
  });
}
