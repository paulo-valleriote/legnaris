import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/connection/connection_indicator.dart';
import 'package:legnaris/connection/health_scheduler.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/shell/app_shell.dart';
import 'package:legnaris/theme/app_theme.dart';
import 'package:legnaris/theme/tokens.dart';

import 'manual_health_scheduler.dart';

void main() {
  const healthyBody = '{"data": {"status": "healthy"}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

  final en = lookupAppLocalizations(const Locale('en'));

  Future<ManualHealthScheduler> pumpHeader(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? handler,
  }) async {
    final scheduler = ManualHealthScheduler();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient(
                (request) async =>
                    handler == null
                        ? http.Response(healthyBody, 200)
                        : handler(request),
              ),
            ),
          ),
          healthSchedulerProvider.overrideWithValue(scheduler),
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(body: Header()),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return scheduler;
  }

  group('status labels from l10n (CONN-02, CONN-03, CONN-04, CONN-11)', () {
    testWidgets(
      'Happy path: before the first response the indicator renders the checking label (CONN-02)',
      (tester) async {
        final gate = Completer<http.Response>();
        await pumpHeader(tester, handler: (request) => gate.future);

        expect(find.byKey(const Key('connection-indicator')), findsOneWidget);
        expect(find.text(en.connectionChecking), findsOneWidget);

        // Resolve the boot check so no request timeout outlives the test.
        gate.complete(http.Response(healthyBody, 200));
        await tester.pumpAndSettle();
      },
    );

    testWidgets(
      'Happy path: a successful health check renders the online label (CONN-03, CONN-08)',
      (tester) async {
        await pumpHeader(tester);

        expect(find.text(en.connectionOnline), findsOneWidget);
      },
    );

    testWidgets(
      'Error: a failed health check renders the offline label in the scheme error color (CONN-04, CONN-08)',
      (tester) async {
        await pumpHeader(
          tester,
          handler: (request) async => http.Response(serverErrorBody, 500),
        );

        final indicator = find.byKey(const Key('connection-indicator'));
        expect(find.descendant(of: indicator, matching: find.text(en.connectionOffline)), findsOneWidget);

        final scheme = Theme.of(tester.element(indicator)).colorScheme;
        final label = tester.widget<Text>(find.text(en.connectionOffline));
        expect(label.style?.color, scheme.error);
        final dotFinder = find.descendant(
          of: indicator,
          matching: find.byType(Container),
        );
        final dot = tester.widget<Container>(dotFinder);
        final decoration = dot.decoration! as BoxDecoration;
        expect(decoration.shape, BoxShape.circle);
        expect(
          tester.getSize(dotFinder).width,
          AppSpacing.sm,
          reason: 'the dot is the 8px token circle from the design',
        );
        expect(
          decoration.color,
          scheme.error,
          reason: 'the dot carries the same scheme color as the label',
        );
      },
    );
  });

  group('header slot (CONN-08)', () {
    testWidgets(
      'Happy path: the indicator sits right-aligned inside the keyed header slot (CONN-08)',
      (tester) async {
        await pumpHeader(tester);

        final slot = find.byKey(const Key('connection-indicator-slot'));
        final indicator = find.byType(ConnectionIndicator);
        expect(slot, findsOneWidget);
        expect(find.descendant(of: slot, matching: indicator), findsOneWidget);
        expect(
          find.descendant(
            of: slot,
            matching: find.byKey(const Key('connection-indicator')),
          ),
          findsOneWidget,
        );
        expect(tester.getSize(slot).width, AppSizes.connectionSlotWidth);

        final slotRect = tester.getRect(slot);
        final indicatorRect = tester.getRect(indicator);
        expect(indicatorRect.right, closeTo(slotRect.right, 0.01));
        expect(
          indicatorRect.left,
          greaterThan(slotRect.left),
          reason: 'the row hugs the right edge instead of filling the slot',
        );
      },
    );
  });

  group('update without user action (CONN-06)', () {
    testWidgets(
      'Happy path: the label flips when a scheduled check resolves, with no user action (CONN-06)',
      (tester) async {
        var healthy = true;
        final scheduler = await pumpHeader(
          tester,
          handler:
              (request) async =>
                  healthy
                      ? http.Response(healthyBody, 200)
                      : http.Response(serverErrorBody, 500),
        );
        expect(find.text(en.connectionOnline), findsOneWidget);

        healthy = false;
        scheduler.fire();
        await tester.pumpAndSettle();

        expect(find.text(en.connectionOffline), findsOneWidget);
        expect(find.text(en.connectionOnline), findsNothing);
      },
    );
  });
}
