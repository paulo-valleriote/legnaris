import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:intl/intl.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/connection/connection_provider.dart';
import 'package:legnaris/connection/health_scheduler.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/main.dart';
import 'package:legnaris/shell/shell_state.dart';

import 'manual_health_scheduler.dart';

void main() {
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

  final en = lookupAppLocalizations(const Locale('en'));

  /// Boots the wired app (ProviderScope + LegnarisApp) at the desktop
  /// viewport. By default `/health` fails with a 500, so every test starts
  /// from the offline state in situ; pass [handler] to drive any path and
  /// [scheduler] when the test needs to fire health ticks on command.
  Future<List<http.Request>> pumpApp(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? handler,
    ManualHealthScheduler? scheduler,
  }) async {
    final recorded = <http.Request>[];
    final manual = scheduler ?? ManualHealthScheduler();
    final supportDir = Directory.systemTemp.createTempSync('lgn108-conn');
    addTearDown(() => supportDir.deleteSync(recursive: true));
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(supportDir),
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
                if (handler != null) return handler(request);
                if (request.url.path == '/health') {
                  return http.Response(serverErrorBody, 500);
                }
                return http.Response(
                  request.url.path == '/templates'
                      ? twoTemplatesBody
                      : detailBody,
                  200,
                );
              }),
            ),
          ),
          healthSchedulerProvider.overrideWithValue(manual),
        ],
        child: const LegnarisApp(),
      ),
    );
    await tester.pumpAndSettle();
    return recorded;
  }

  int listRequestCount(List<http.Request> recorded) => recorded
      .where((request) => request.url.path == '/templates')
      .length;

  int healthRequestCount(List<http.Request> recorded) => recorded
      .where((request) => request.url.path == '/health')
      .length;

  Finder indicatorLabel(String label) => find.descendant(
    of: find.byKey(const Key('connection-indicator')),
    matching: find.text(label),
  );

  Finder panelText(String label) => find.descendant(
    of: find.byKey(const Key('connection-panel-slot')),
    matching: find.text(label),
  );

  ShellSurface currentSurface(WidgetTester tester) =>
      ProviderScope.containerOf(
        tester.element(find.byType(MaterialApp)),
      ).read(shellStateProvider);

  String lastCheckText(WidgetTester tester) {
    final container = ProviderScope.containerOf(
      tester.element(find.byType(MaterialApp)),
    );
    final lastSuccess = container.read(connectionProvider).lastSuccess;
    expect(lastSuccess, isNotNull);
    return en.connectionLastCheck(DateFormat.Hm().format(lastSuccess!));
  }

  group('offline does not block navigation (CONN-10, CONN-08, CONN-06)', () {
    testWidgets(
      'Error: a failing boot health check settles both surfaces on offline (CONN-04, CONN-09)',
      (tester) async {
        final recorded = await pumpApp(tester);

        expect(
          healthRequestCount(recorded),
          1,
          reason: 'exactly one boot health check',
        );
        expect(indicatorLabel(en.connectionOffline), findsOneWidget);
        expect(panelText(en.connectionOffline), findsOneWidget);
        expect(panelText(en.connectionLastCheckNever), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: a boot failure recovers to online on the next successful check without a restart (CONN-06)',
      (tester) async {
        var healthy = false;
        final scheduler = ManualHealthScheduler();
        final recorded = await pumpApp(
          tester,
          scheduler: scheduler,
          handler: (request) async {
            if (request.url.path == '/health') {
              return healthy
                  ? http.Response(healthyBody, 200)
                  : http.Response(serverErrorBody, 500);
            }
            if (request.url.path == '/templates') {
              return http.Response(twoTemplatesBody, 200);
            }
            return http.Response(detailBody, 200);
          },
        );
        expect(healthRequestCount(recorded), 1);
        expect(indicatorLabel(en.connectionOffline), findsOneWidget);
        expect(panelText(en.connectionOffline), findsOneWidget);
        expect(panelText(en.connectionLastCheckNever), findsOneWidget);

        healthy = true;
        scheduler.fire();
        await tester.pumpAndSettle();

        expect(
          healthRequestCount(recorded),
          2,
          reason: 'only the scheduled check, no restart',
        );
        expect(find.text(en.connectionOffline), findsNothing);
        expect(indicatorLabel(en.connectionOnline), findsOneWidget);
        expect(panelText(en.connectionOnline), findsOneWidget);
        expect(panelText(en.connectionLastCheckNever), findsNothing);
        expect(panelText(lastCheckText(tester)), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: the indicator and the connection detail stay rendered with the creation dialog open (CONN-08)',
      (tester) async {
        await pumpApp(tester);
        expect(indicatorLabel(en.connectionOffline), findsOneWidget);

        await tester.tap(find.text('Fantasy Template'));
        await tester.pumpAndSettle();
        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(currentSurface(tester), ShellSurface.creationDialog);

        expect(find.byKey(const Key('connection-indicator')), findsOneWidget);
        expect(indicatorLabel(en.connectionOffline), findsOneWidget);
        expect(panelText(en.connectionOffline), findsOneWidget);
        expect(panelText(en.connectionLastCheckNever), findsOneWidget);
      },
    );

    testWidgets(
      'Happy path: while offline, tapping a catalog card opens the creation dialog (CONN-10)',
      (tester) async {
        await pumpApp(tester);
        expect(
          indicatorLabel(en.connectionOffline),
          findsOneWidget,
          reason: 'precondition: the indicator shows offline',
        );

        await tester.tap(find.text('Fantasy Template'));
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(currentSurface(tester), ShellSurface.creationDialog);
      },
    );

    testWidgets(
      'Happy path: while offline, Esc closes the creation dialog (CONN-10)',
      (tester) async {
        await pumpApp(tester);
        expect(indicatorLabel(en.connectionOffline), findsOneWidget);

        await tester.tap(find.text('Fantasy Template'));
        await tester.pumpAndSettle();
        expect(find.byType(CreateSheetDialog), findsOneWidget);

        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(currentSurface(tester), ShellSurface.catalog);
        expect(find.byType(Card), findsNWidgets(2));
      },
    );

    testWidgets(
      'Happy path: while offline, F5 re-requests /templates exactly once and refreshes the list (CONN-10)',
      (tester) async {
        var call = 0;
        final recorded = await pumpApp(
          tester,
          handler: (request) async {
            if (request.url.path == '/health') {
              return http.Response(serverErrorBody, 500);
            }
            if (request.url.path == '/templates') {
              return http.Response(
                call++ == 0 ? oneTemplateBody : twoTemplatesBody,
                200,
              );
            }
            return http.Response(detailBody, 200);
          },
        );
        expect(indicatorLabel(en.connectionOffline), findsOneWidget);
        expect(listRequestCount(recorded), 1);
        expect(find.byType(Card), findsOneWidget);

        await tester.sendKeyEvent(LogicalKeyboardKey.f5);
        await tester.pumpAndSettle();

        expect(
          listRequestCount(recorded),
          2,
          reason: 'F5 re-requests the catalog exactly once',
        );
        expect(
          healthRequestCount(recorded),
          1,
          reason: 'F5 issues no extra health check',
        );
        expect(find.byType(Card), findsNWidgets(2));
        expect(find.text('Sci Fi Template'), findsOneWidget);
      },
    );
  });
}
