import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/catalog/catalog_provider.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/theme/app_theme.dart';
import 'package:legnaris/theme/tokens.dart';

void main() {
  const twoTemplatesBody = '''
{"data": [
  {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}},
  {"id": "sci_fi_v1", "name": "Sci Fi Template", "default": false, "fields": {}}
]}
''';
  const emptyListBody = '{"data": []}';
  const emptyNameBody =
      '{"data": [{"id": "blank", "name": "", "default": false, "fields": {}}]}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

  Future<List<http.Request>> pumpScreen(
    WidgetTester tester, {
    required Future<http.Response> Function(http.Request request) handler,
    Size viewport = const Size(1280, 800),
  }) async {
    final recorded = <http.Request>[];
    tester.view.physicalSize = viewport;
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(
            ApiClient(
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
          home: const CatalogScreen(),
        ),
      ),
    );
    await tester.pump();
    return recorded;
  }

  Future<void> triggerRefresh(WidgetTester tester) async {
    final container = ProviderScope.containerOf(
      tester.element(find.byType(CatalogScreen)),
    );
    unawaited(container.read(catalogProvider.notifier).refresh());
    await tester.pump();
  }

  group('first load (CAT-08 … CAT-11)', () {
    testWidgets('Edge: while the first request is gated the loading state shows and the empty state does not (CAT-08)', (tester) async {
      final gate = Completer<http.Response>();
      final recorded = await pumpScreen(tester, handler: (request) => gate.future);

      expect(recorded, hasLength(1));
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.text('Loading templates...'), findsOneWidget);
      expect(find.text('No templates available yet'), findsNothing);
      expect(find.byType(Card), findsNothing);

      gate.complete(http.Response(twoTemplatesBody, 200));
      await tester.pumpAndSettle();

      expect(find.byType(LinearProgressIndicator), findsNothing);
      expect(find.text('Loading templates...'), findsNothing);
      expect(find.byType(Card), findsNWidgets(2));
    });

    testWidgets('Edge: an entry with an empty name keeps the card layout intact', (tester) async {
      await pumpScreen(
        tester,
        handler: (request) async => http.Response(emptyNameBody, 200),
      );
      await tester.pumpAndSettle();

      expect(find.byType(Card), findsOneWidget);
      expect(find.text(''), findsOneWidget);
      expect(find.text('blank'), findsOneWidget);
      expect(find.widgetWithText(TextButton, 'Create sheet'), findsOneWidget);
    });

    testWidgets('Error: a failed first load renders the mapped message with a Retry action (CAT-11)', (tester) async {
      final recorded = await pumpScreen(
        tester,
        handler: (request) async => http.Response(serverErrorBody, 500),
      );
      await tester.pumpAndSettle();

      expect(recorded, hasLength(1));
      expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget);
      expect(find.widgetWithText(OutlinedButton, 'Retry'), findsOneWidget);
      expect(find.byType(Card), findsNothing);
      expect(find.text('No templates available yet'), findsNothing);
      expect(find.text('Loading templates...'), findsNothing);
    });

    testWidgets('Happy path: cards render in API order with name and id on the type scale (CAT-09)', (tester) async {
      await pumpScreen(
        tester,
        handler: (request) async => http.Response(twoTemplatesBody, 200),
      );
      await tester.pumpAndSettle();

      expect(find.byType(Card), findsNWidgets(2));
      final first = find.text('Fantasy Template');
      final second = find.text('Sci Fi Template');
      expect(first, findsOneWidget);
      expect(second, findsOneWidget);
      expect(
        tester.getTopLeft(first).dx,
        lessThan(tester.getTopLeft(second).dx),
      );

      final firstName = tester.widget<Text>(first);
      expect(firstName.style?.fontSize, AppSizes.emphasis);
      final firstId = tester.widget<Text>(find.text('fantasy_v1'));
      expect(firstId.style?.fontSize, AppSizes.caption);
      final scheme = Theme.of(tester.element(first)).colorScheme;
      expect(firstId.style?.color, scheme.onSurfaceVariant);

      expect(
        find.widgetWithText(TextButton, 'Create sheet'),
        findsNWidgets(2),
      );
      final action = tester.widget<TextButton>(
        find.widgetWithText(TextButton, 'Create sheet').first,
      );
      expect(action.onPressed, isNotNull);
    });

    testWidgets('Happy path: an empty catalog renders one localized line without art (CAT-10)', (tester) async {
      await pumpScreen(
        tester,
        handler: (request) async => http.Response(emptyListBody, 200),
      );
      await tester.pumpAndSettle();

      expect(find.text('No templates available yet'), findsOneWidget);
      expect(find.byType(Card), findsNothing);
      expect(find.byType(Icon), findsNothing);
      expect(find.byType(LinearProgressIndicator), findsNothing);
    });
  });

  group('refresh and retry (CAT-12, CAT-19)', () {
    testWidgets('Edge: a refresh in flight keeps the list on screen with the progress indicator', (tester) async {
      var call = 0;
      final gate = Completer<http.Response>();
      final recorded = await pumpScreen(tester, handler: (request) {
        if (call++ == 0) return Future.value(http.Response(twoTemplatesBody, 200));
        return gate.future;
      });
      await tester.pumpAndSettle();
      expect(find.byType(Card), findsNWidgets(2));

      await triggerRefresh(tester);

      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(Card), findsNWidgets(2));
      expect(recorded, hasLength(2));

      gate.complete(http.Response(twoTemplatesBody, 200));
      await tester.pumpAndSettle();

      expect(find.byType(LinearProgressIndicator), findsNothing);
      expect(find.byType(Card), findsNWidgets(2));
      expect(recorded, hasLength(2));
    });

    testWidgets('Edge: a failed refresh keeps the cached list and shows the inline error with Retry (CAT-19)', (tester) async {
      var call = 0;
      final recorded = await pumpScreen(tester, handler: (request) async {
        final first = call++ == 0;
        return http.Response(
          first ? twoTemplatesBody : serverErrorBody,
          first ? 200 : 500,
        );
      });
      await tester.pumpAndSettle();
      expect(find.byType(Card), findsNWidgets(2));

      await triggerRefresh(tester);
      await tester.pumpAndSettle();

      expect(recorded, hasLength(2));
      expect(find.byType(Card), findsNWidgets(2));
      expect(find.byType(LinearProgressIndicator), findsNothing);
      expect(find.text('No templates available yet'), findsNothing);
      final error = find.text(errorCopy(ApiErrorKind.serverError));
      expect(error, findsOneWidget);
      expect(find.widgetWithText(OutlinedButton, 'Retry'), findsOneWidget);
      expect(
        tester.getTopLeft(error).dy,
        lessThan(tester.getTopLeft(find.text('Fantasy Template')).dy),
      );
    });

    testWidgets('Edge: two rapid Retry activations start exactly one new request', (tester) async {
      var call = 0;
      final gate = Completer<http.Response>();
      final recorded = await pumpScreen(tester, handler: (request) {
        if (call++ == 0) return Future.value(http.Response(serverErrorBody, 500));
        return gate.future;
      });
      await tester.pumpAndSettle();
      expect(recorded, hasLength(1));

      final retry = find.widgetWithText(OutlinedButton, 'Retry');
      await tester.tap(retry);
      await tester.tap(retry);
      await tester.pump();

      expect(recorded, hasLength(2));

      gate.complete(http.Response(twoTemplatesBody, 200));
      await tester.pumpAndSettle();

      expect(recorded, hasLength(2));
      expect(find.byType(Card), findsNWidgets(2));
    });

    testWidgets('Edge: retrying from the error state shows loading until the response lands', (tester) async {
      var call = 0;
      final gate = Completer<http.Response>();
      await pumpScreen(tester, handler: (request) {
        if (call++ == 0) return Future.value(http.Response(serverErrorBody, 500));
        return gate.future;
      });
      await tester.pumpAndSettle();

      await tester.tap(find.widgetWithText(OutlinedButton, 'Retry'));
      await tester.pump();

      expect(find.text(errorCopy(ApiErrorKind.serverError)), findsNothing);
      expect(find.text('Loading templates...'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(Card), findsNothing);

      gate.complete(http.Response(twoTemplatesBody, 200));
      await tester.pumpAndSettle();

      expect(find.byType(Card), findsNWidgets(2));
      expect(find.text('Loading templates...'), findsNothing);
    });

    testWidgets('Happy path: activating Retry re-requests exactly once and renders the list (CAT-12)', (tester) async {
      var call = 0;
      final recorded = await pumpScreen(tester, handler: (request) async {
        final first = call++ == 0;
        return http.Response(
          first ? serverErrorBody : twoTemplatesBody,
          first ? 500 : 200,
        );
      });
      await tester.pumpAndSettle();
      expect(find.widgetWithText(OutlinedButton, 'Retry'), findsOneWidget);
      expect(recorded, hasLength(1));

      await tester.tap(find.widgetWithText(OutlinedButton, 'Retry'));
      await tester.pumpAndSettle();

      expect(recorded, hasLength(2));
      expect(find.byType(Card), findsNWidgets(2));
      expect(find.text(errorCopy(ApiErrorKind.serverError)), findsNothing);
    });
  });
}
