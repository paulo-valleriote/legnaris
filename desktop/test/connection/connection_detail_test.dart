import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:intl/intl.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/connection/connection_detail.dart';
import 'package:legnaris/connection/connection_provider.dart';
import 'package:legnaris/connection/health_scheduler.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/shell/right_panel.dart';
import 'package:legnaris/theme/app_theme.dart';
import 'package:legnaris/theme/tokens.dart';

import 'manual_health_scheduler.dart';

void main() {
  const healthyBody = '{"data": {"status": "healthy"}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';
  const emptyListBody = '{"data": []}';

  final en = lookupAppLocalizations(const Locale('en'));

  Future<ManualHealthScheduler> pumpPanel(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? healthHandler,
  }) async {
    final scheduler = ManualHealthScheduler();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                if (request.url.path == '/health') {
                  return healthHandler == null
                      ? http.Response(healthyBody, 200)
                      : healthHandler(request);
                }
                return http.Response(emptyListBody, 200);
              }),
            ),
          ),
          healthSchedulerProvider.overrideWithValue(scheduler),
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: RightPanel()),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return scheduler;
  }

  String lastCheckText(WidgetTester tester) {
    final container = ProviderScope.containerOf(
      tester.element(find.byType(ConnectionDetail)),
    );
    final lastSuccess = container.read(connectionProvider).lastSuccess;
    expect(lastSuccess, isNotNull);
    return en.connectionLastCheck(DateFormat.Hm().format(lastSuccess!));
  }

  group('last check line (CONN-09)', () {
    testWidgets(
      'Edge: before the first success the detail shows the current state and never (CONN-09)',
      (tester) async {
        final gate = Completer<http.Response>();
        await pumpPanel(
          tester,
          healthHandler: (request) => gate.future,
        );

        final slot = find.byKey(const Key('connection-panel-slot'));
        final detail = find.byType(ConnectionDetail);
        expect(slot, findsOneWidget);
        expect(find.descendant(of: slot, matching: detail), findsOneWidget);
        expect(find.descendant(of: slot, matching: find.text(en.connectionChecking)), findsOneWidget);
        expect(
          find.descendant(of: slot, matching: find.text(en.connectionLastCheckNever)),
          findsOneWidget,
        );

        // Resolve the boot check so no request timeout outlives the test.
        gate.complete(http.Response(healthyBody, 200));
        await tester.pumpAndSettle();
      },
    );

    testWidgets(
      'Happy path: after a success the detail shows the localized last check timestamp (CONN-09)',
      (tester) async {
        await pumpPanel(tester);

        expect(find.text(en.connectionOnline), findsOneWidget);
        expect(find.text(lastCheckText(tester)), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: a failure after a success keeps the timestamp and shows offline (CONN-09)',
      (tester) async {
        var healthy = true;
        final scheduler = await pumpPanel(
          tester,
          healthHandler:
              (request) async =>
                  healthy
                      ? http.Response(healthyBody, 200)
                      : http.Response(serverErrorBody, 500),
        );
        final timestamp = lastCheckText(tester);
        expect(find.text(timestamp), findsOneWidget);

        healthy = false;
        scheduler.fire();
        await tester.pumpAndSettle();

        expect(find.text(en.connectionOffline), findsOneWidget);
        expect(find.text(timestamp), findsOneWidget);
        expect(find.text(en.connectionLastCheckNever), findsNothing);
      },
    );
  });

  group('state line (CONN-11)', () {
    testWidgets(
      'Happy path: the state line follows the provider through all three states with explicit styles (CONN-11)',
      (tester) async {
        Completer<http.Response>? gate = Completer<http.Response>();
        var healthy = true;
        final scheduler = await pumpPanel(
          tester,
          healthHandler: (request) async {
            final pending = gate;
            if (pending != null) return pending.future;
            return healthy
                ? http.Response(healthyBody, 200)
                : http.Response(serverErrorBody, 500);
          },
        );

        final checking = find.text(en.connectionChecking);
        expect(checking, findsOneWidget);
        final checkingStyle = tester.widget<Text>(checking).style;
        expect(
          checkingStyle?.fontSize,
          AppSizes.body,
          reason: 'explicit bodyMedium — never the 48px error fallback (T8)',
        );

        gate.complete(http.Response(healthyBody, 200));
        gate = null;
        await tester.pumpAndSettle();
        expect(find.text(en.connectionOnline), findsOneWidget);

        healthy = false;
        scheduler.fire();
        await tester.pumpAndSettle();
        expect(find.text(en.connectionOffline), findsOneWidget);
        expect(find.text(en.connectionChecking), findsNothing);
      },
    );
  });
}
