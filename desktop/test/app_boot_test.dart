import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/main.dart';
import 'package:legnaris/shell/app_shell.dart';
import 'package:legnaris/shell/shell_state.dart';

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
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

  Future<List<http.Request>> pumpApp(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? handler,
  }) async {
    final recorded = <http.Request>[];
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
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

  group('boot (CAT-01, CAT-05, CAT-07)', () {
    testWidgets('Happy path: booting renders the shell with the header slot and issues exactly one GET /templates (CAT-01/05/07)', (tester) async {
      final recorded = await pumpApp(tester);

      expect(find.byType(AppShell), findsOneWidget);
      expect(find.byType(Header), findsOneWidget);
      expect(find.byKey(const Key('connection-indicator-slot')), findsOneWidget);
      expect(recorded, hasLength(1));
      expect(recorded.single.method, 'GET');
      expect(recorded.single.url.path, '/templates');
      expect(find.text('Legnaris'), findsOneWidget);
      expect(find.byType(Card), findsNWidgets(2));
    });

    testWidgets('Happy path: MaterialApp.title comes from appTitleProvider, never a hardcoded string', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            appTitleProvider.overrideWithValue('Legnaris (overridden)'),
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

      expect(recorded, hasLength(2));
      expect(find.byType(Card), findsNWidgets(2));
      expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget);
      expect(find.widgetWithText(OutlinedButton, 'Retry'), findsOneWidget);
    });

    testWidgets('Edge: F5 with the creation dialog open issues no catalog request', (tester) async {
      final recorded = await pumpApp(tester);
      expect(recorded, hasLength(1));

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
        handler: (request) async =>
            http.Response(call++ == 0 ? oneTemplateBody : twoTemplatesBody, 200),
      );
      expect(find.byType(Card), findsOneWidget);
      expect(find.text('Sci Fi Template'), findsNothing);

      await tester.sendKeyEvent(LogicalKeyboardKey.f5);
      await tester.pumpAndSettle();

      expect(recorded, hasLength(2));
      expect(find.byType(Card), findsNWidgets(2));
      expect(find.text('Sci Fi Template'), findsOneWidget);
    });
  });
}
