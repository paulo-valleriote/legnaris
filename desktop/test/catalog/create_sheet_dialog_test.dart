import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/gestures.dart' show PointerDeviceKind;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/catalog/catalog_screen.dart';
import 'package:legnaris/catalog/create_sheet_dialog.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/library/sheet_library_store.dart';
import 'package:legnaris/theme/app_theme.dart';

/// Widget tests for the creation surface content
/// (TPL-01, TPL-02, TPL-04, TPL-11, TPL-12, TPL-13). Every asserted value comes
/// from the spec rule, never from the implementation.
void main() {
  const listBody =
      '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}';
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "baseArmorClass": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}},
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 0}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "is_editable": false, "default": 0}}}},
  "modifiers": {"children": {"dexterityMod": {"schema": {"type": "number", "label": "", "is_editable": false, "default": 0}}}},
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}, "constitution": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}
}}}
''';
  const emptyFieldsBody =
      '{"data": {"id": "fantasy_v1", "name": "Blank Template", "default": false, "fields": {}}}';
  const emptyNameBody =
      '{"data": {"id": "fantasy_v1", "name": "", "default": false, "fields": {"stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}}}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';
  const notFoundBody =
      '{"error": {"code": "NOT_FOUND", "message": "template not found"}}';
  const noDataKeyBody = '{"unexpected": true}';
  const malformedBody = '{"data": [1, 2, 3]}';

  String parseCopy() => lookupAppLocalizations(const Locale('en')).errorParse;

  Future<http.Response> Function(http.Request) routing({
    required String detail,
  }) => (request) async => http.Response(
    request.url.path == '/templates' ? listBody : detail,
    200,
  );

  Future<List<http.Request>> pumpCatalog(
    WidgetTester tester, {
    Future<http.Response> Function(http.Request request)? handler,
  }) async {
    final recorded = <http.Request>[];
    final supportDir = Directory.systemTemp.createTempSync('lgn108-dialog');
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
                return http.Response(
                  request.url.path == '/templates' ? listBody : detailBody,
                  200,
                );
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
    await tester.pumpAndSettle();
    return recorded;
  }

  Finder inDialog(Finder matching) =>
      find.descendant(of: find.byType(CreateSheetDialog), matching: matching);

  Future<void> openDialog(WidgetTester tester) async {
    await tester.tap(find.text('Fantasy Template'));
    await tester.pumpAndSettle();
  }

  int detailRequestCount(List<http.Request> recorded, String id) => recorded
      .where((request) => request.url.path == '/templates/$id')
      .length;

  group('rendering the detail (TPL-01, TPL-02)', () {
    testWidgets(
      'Happy path: opening shows the template name and the read-only preview (PRV-01, PRV-05)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        expect(
          inDialog(find.text('Fantasy Template')),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('create-sheet-preview'))),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('sheet-section-stats'))),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('sheet-section-config'))),
          findsOneWidget,
        );
        expect(inDialog(find.textContaining('Stats: ')), findsNothing);
      },
    );

    testWidgets(
      'Happy path: opening requests GET /templates/:id exactly once (TPL-01)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        await openDialog(tester);

        final detailRequests = recorded
            .where((request) => request.url.path == '/templates/fantasy_v1')
            .toList();
        expect(detailRequests, hasLength(1));
        expect(detailRequests.single.method, 'GET');
      },
    );

    testWidgets(
      'Edge: reserved groups (config, derived, modifiers) render in the preview, read-only (PRV-01, PRV-06)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        expect(
          inDialog(find.byKey(const Key('sheet-section-config'))),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('sheet-section-derived'))),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('sheet-section-modifiers'))),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('sheet-section-stats'))),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: an empty fields tree shows the localized no-fields line in the preview (PRV-09)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: emptyFieldsBody));
        await openDialog(tester);

        expect(
          inDialog(find.text('This template has no fields.')),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: a template with an empty name still renders the layout (spec edge case)',
      (tester) async {
        await pumpCatalog(tester, handler: routing(detail: emptyNameBody));
        await openDialog(tester);

        expect(
          inDialog(find.byKey(const Key('create-sheet-template-id'))),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('create-sheet-template-id-divider'))),
          findsNothing,
        );
        expect(
          inDialog(find.byKey(const Key('sheet-section-stats'))),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Edge: while the detail request is in flight the dialog shows loading and no stale sections (PRV-04)',
      (tester) async {
        final gate = Completer<http.Response>();
        await pumpCatalog(
          tester,
          handler: (request) async =>
              request.url.path == '/templates'
              ? http.Response(listBody, 200)
              : gate.future,
        );

        await tester.tap(find.text('Fantasy Template'));
        await tester.pump();

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(inDialog(find.text('Loading template...')), findsOneWidget);
        expect(
          inDialog(find.byKey(const Key('sheet-section-stats'))),
          findsNothing,
        );

        gate.complete(http.Response(detailBody, 200));
        await tester.pumpAndSettle();
        expect(inDialog(find.text('Loading template...')), findsNothing);
      },
    );
  });

  group('sheet-name input and primary action (TPL-11, TPL-12, TPL-13)', () {
    testWidgets(
      'Happy path: the sheet-name input is empty with a localized label and placeholder (TPL-11)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        expect(inDialog(find.text('Sheet name')), findsOneWidget);
        expect(inDialog(find.text('e.g. My Character')), findsOneWidget);
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('create-sheet-name-input')),
              )
              .controller
              ?.text,
          '',
        );
      },
    );

    testWidgets(
      'Edge: typing a sheet name leaves the preview unchanged and issues no request (PRV-13)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        await openDialog(tester);
        final before = recorded.length;

        await tester.enterText(
          find.byKey(const Key('create-sheet-name-input')),
          'My Character',
        );
        await tester.pump();

        expect(recorded.length, before);
        expect(
          inDialog(find.byKey(const Key('sheet-section-stats'))),
          findsOneWidget,
        );
        expect(
          tester
              .widget<TextField>(
                find.byKey(const Key('sheet-input-stats.dexterity')),
              )
              .controller
              ?.text,
          '10',
        );
      },
    );

    testWidgets(
      'Edge: an empty name keeps the primary action inert with the name-required tooltip; typing enables it (CRT-05)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        final button = tester.widget<TextButton>(
          inDialog(find.widgetWithText(TextButton, 'Create sheet')),
        );
        expect(button.onPressed, isNull);

        final tooltip = tester.widget<Tooltip>(
          find.ancestor(
            of: inDialog(find.text('Create sheet')),
            matching: find.byType(Tooltip),
          ),
        );
        expect(tooltip.message, 'Enter a sheet name.');

        await tester.enterText(
          find.byKey(const Key('create-sheet-name-input')),
          'My Character',
        );
        await tester.pump();

        expect(
          tester
              .widget<TextButton>(
                inDialog(find.widgetWithText(TextButton, 'Create sheet')),
              )
              .onPressed,
          isNotNull,
        );
      },
    );

    testWidgets(
      'Happy path: hovering the disabled action reveals the name-required tooltip (CRT-05)',
      (tester) async {
        await pumpCatalog(tester);
        await openDialog(tester);

        final gesture = await tester.createGesture(kind: PointerDeviceKind.mouse);
        await gesture.addPointer(location: Offset.zero);
        addTearDown(gesture.removePointer);
        await tester.pump();
        await gesture.moveTo(
          tester.getCenter(
            inDialog(find.widgetWithText(TextButton, 'Create sheet')),
          ),
        );
        await tester.pump(const Duration(milliseconds: 100));

        expect(find.text('Enter a sheet name.'), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: opening the dialog issues no mutation — only the read GETs (TPL-13)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        await openDialog(tester);

        expect(recorded.every((request) => request.method == 'GET'), isTrue);
        expect(recorded.any((request) => request.method == 'POST'), isFalse);
      },
    );
  });

  group('inline error states (TPL-07, TPL-09, TPL-10)', () {
    Future<http.Response> Function(http.Request) failing(
      String body,
      int status,
    ) => (request) async => http.Response(
      request.url.path == '/templates' ? listBody : body,
      request.url.path == '/templates' ? 200 : status,
    );

    testWidgets(
      'Error: a server failure shows the mapped copy inline and keeps the dialog open (TPL-07)',
      (tester) async {
        await pumpCatalog(tester, handler: failing(serverErrorBody, 500));
        await openDialog(tester);

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(
          inDialog(find.text(errorCopy(ApiErrorKind.serverError))),
          findsOneWidget,
        );
        expect(
          inDialog(find.widgetWithText(OutlinedButton, 'Retry')),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Error: NOT_FOUND shows the not-found copy inline with Retry (TPL-09)',
      (tester) async {
        await pumpCatalog(tester, handler: failing(notFoundBody, 404));
        await openDialog(tester);

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(
          inDialog(find.text(errorCopy(ApiErrorKind.notFound))),
          findsOneWidget,
        );
        expect(
          inDialog(find.widgetWithText(OutlinedButton, 'Retry')),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'Error: a 2xx without the data key shows the parse copy with Retry, never a preview (PRV-07)',
      (tester) async {
        await pumpCatalog(tester, handler: failing(noDataKeyBody, 200));
        await openDialog(tester);

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(inDialog(find.text(parseCopy())), findsOneWidget);
        expect(
          inDialog(find.widgetWithText(OutlinedButton, 'Retry')),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('sheet-section-stats'))),
          findsNothing,
        );
      },
    );

    testWidgets(
      'Error: a malformed payload shows the parse copy with Retry, never a half preview (PRV-07)',
      (tester) async {
        await pumpCatalog(tester, handler: failing(malformedBody, 200));
        await openDialog(tester);

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(inDialog(find.text(parseCopy())), findsOneWidget);
        expect(
          inDialog(find.widgetWithText(OutlinedButton, 'Retry')),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('sheet-section-stats'))),
          findsNothing,
        );
      },
    );

    testWidgets(
      'Error: an offline transport failure shows the mapped copy with Retry and keeps the dialog open (PRV-07)',
      (tester) async {
        await pumpCatalog(
          tester,
          handler: (request) async {
            if (request.url.path == '/templates') {
              return http.Response(listBody, 200);
            }
            throw http.ClientException('connection refused');
          },
        );
        await openDialog(tester);
        await tester.pump(const Duration(milliseconds: 350));
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(
          inDialog(find.text(errorCopy(ApiErrorKind.offline))),
          findsOneWidget,
        );
        expect(
          inDialog(find.widgetWithText(OutlinedButton, 'Retry')),
          findsOneWidget,
        );
        expect(
          inDialog(find.byKey(const Key('sheet-section-stats'))),
          findsNothing,
        );
      },
    );
  });

  group('retry and lifecycle (TPL-08, TPL-13, TPL-14, TPL-15)', () {
    testWidgets(
      'Happy path: Retry re-requests exactly once and shows the preview (PRV-08)',
      (tester) async {
        var fail = true;
        final recorded = await pumpCatalog(
          tester,
          handler: (request) async {
            if (request.url.path == '/templates') {
              return http.Response(listBody, 200);
            }
            return http.Response(
              fail ? serverErrorBody : detailBody,
              fail ? 500 : 200,
            );
          },
        );
        await openDialog(tester);
        expect(
          inDialog(find.text(errorCopy(ApiErrorKind.serverError))),
          findsOneWidget,
        );
        expect(detailRequestCount(recorded, 'fantasy_v1'), 1);

        fail = false;
        await tester.tap(inDialog(find.widgetWithText(OutlinedButton, 'Retry')));
        await tester.pumpAndSettle();

        expect(detailRequestCount(recorded, 'fantasy_v1'), 2);
        expect(
          inDialog(find.byKey(const Key('sheet-section-stats'))),
          findsOneWidget,
        );
        expect(
          inDialog(find.widgetWithText(OutlinedButton, 'Retry')),
          findsNothing,
        );
      },
    );

    testWidgets(
      'Edge: reopening the same template fetches the detail again (TPL-15)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        await openDialog(tester);
        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(find.byType(CreateSheetDialog), findsNothing);

        await openDialog(tester);

        expect(detailRequestCount(recorded, 'fantasy_v1'), 2);
      },
    );

    testWidgets(
      'Edge: dismissing with Esc neither refetches nor mutates (TPL-13, TPL-14)',
      (tester) async {
        final recorded = await pumpCatalog(tester);
        await openDialog(tester);
        final detailBefore = detailRequestCount(recorded, 'fantasy_v1');

        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(detailRequestCount(recorded, 'fantasy_v1'), detailBefore);
        expect(recorded.any((request) => request.method == 'POST'), isFalse);
      },
    );
  });

  group('creating a sheet (CRT-01 … CRT-07, CRT-11)', () {
    const createdBody =
        '{"data": {"id": "12345678-1234-7000-8000-123456789abc", "template_id": "fantasy_v1", "name": "Aragorn", "data": {"stats": {"dexterity": 16}}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';
    const createServerErrorBody =
        '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

    ProviderContainer containerOf(WidgetTester tester) =>
        ProviderScope.containerOf(tester.element(find.byType(MaterialApp)));

    List<http.Request> postsOf(List<http.Request> recorded) => recorded
        .where(
          (request) =>
              request.method == 'POST' && request.url.path == '/sheets',
        )
        .toList();

    /// Mounts the dialog over a minimal host that captures the popped sheet,
    /// with an isolated temp support directory so the real library store is
    /// exercised without the `path_provider` channel.
    Future<({List<http.Request> recorded, Sheet? Function() popped})> pumpDialog(
      WidgetTester tester, {
      required Future<http.Response> Function(http.Request request) handler,
      SheetLibraryStore? store,
    }) async {
      final recorded = <http.Request>[];
      final supportDir = Directory.systemTemp.createTempSync('lgn109-dialog');
      addTearDown(() => supportDir.deleteSync(recursive: true));
      Sheet? popped;
      tester.view.physicalSize = const Size(1280, 800);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            appSupportDirectoryProvider.overrideWithValue(supportDir),
            if (store != null)
              sheetLibraryStoreProvider.overrideWithValue(store),
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
            home: Builder(
              builder: (context) => Scaffold(
                body: Center(
                  child: TextButton(
                    key: const Key('open-dialog'),
                    onPressed: () async {
                      popped = await showDialog<Sheet>(
                        context: context,
                        builder: (_) =>
                            const CreateSheetDialog(templateId: 'fantasy_v1'),
                      );
                    },
                    child: const Text('open'),
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('open-dialog')));
      await tester.pumpAndSettle();
      return (recorded: recorded, popped: () => popped);
    }

    Future<void> typeName(WidgetTester tester, String name) async {
      await tester.enterText(
        find.byKey(const Key('create-sheet-name-input')),
        name,
      );
      await tester.pump();
    }

    testWidgets(
      'Happy path: one activation posts the trimmed name, registers the entry, and pops with the created sheet (CRT-01, CRT-03)',
      (tester) async {
        final harness = await pumpDialog(
          tester,
          handler: (request) async => http.Response(
            request.url.path == '/sheets' ? createdBody : detailBody,
            request.url.path == '/sheets' ? 201 : 200,
          ),
        );
        await typeName(tester, '  Aragorn  ');

        await tester.tap(find.byKey(const Key('create-sheet-submit')));
        await tester.pumpAndSettle();

        final posts = postsOf(harness.recorded);
        expect(posts, hasLength(1));
        expect(jsonDecode(posts.single.body), {
          'template_id': 'fantasy_v1',
          'sheet_name': 'Aragorn',
        });
        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(harness.popped(), isNotNull);
        expect(harness.popped()!.id, '12345678-1234-7000-8000-123456789abc');

        final entry = containerOf(
          tester,
        ).read(sheetLibraryProvider).entries.single;
        expect(entry.id, '12345678-1234-7000-8000-123456789abc');
        expect(entry.name, 'Aragorn');
        expect(entry.templateId, 'fantasy_v1');
        expect(entry.createdAt, DateTime.utc(2026, 9, 20, 12));
      },
    );

    testWidgets(
      'Edge: a second activation while in flight issues no additional POST (CRT-02)',
      (tester) async {
        final gate = Completer<http.Response>();
        final harness = await pumpDialog(
          tester,
          handler: (request) async => request.url.path == '/sheets'
              ? gate.future
              : http.Response(detailBody, 200),
        );
        await typeName(tester, 'Aragorn');

        await tester.tap(find.byKey(const Key('create-sheet-submit')));
        await tester.pump();

        final submit = find.byKey(const Key('create-sheet-submit'));
        expect(tester.widget<TextButton>(submit).onPressed, isNull);
        await tester.tap(submit, warnIfMissed: false);
        await tester.pump();
        expect(postsOf(harness.recorded), hasLength(1));

        gate.complete(http.Response(createdBody, 201));
        await tester.pumpAndSettle();
        expect(find.byType(CreateSheetDialog), findsNothing);
      },
    );

    testWidgets(
      'Edge: dismissing the dialog while the create request is in flight still registers the created sheet (CRT-03)',
      (tester) async {
        final gate = Completer<http.Response>();
        final harness = await pumpDialog(
          tester,
          handler: (request) async => request.url.path == '/sheets'
              ? gate.future
              : http.Response(detailBody, 200),
        );
        await typeName(tester, 'Aragorn');

        await tester.tap(find.byKey(const Key('create-sheet-submit')));
        await tester.pump();

        await tester.sendKeyEvent(LogicalKeyboardKey.escape);
        await tester.pumpAndSettle();
        expect(find.byType(CreateSheetDialog), findsNothing);

        gate.complete(http.Response(createdBody, 201));
        await tester.pumpAndSettle();

        final entries = containerOf(tester).read(sheetLibraryProvider).entries;
        expect(entries, hasLength(1));
        expect(entries.single.id, '12345678-1234-7000-8000-123456789abc');
        expect(entries.single.name, 'Aragorn');
        expect(postsOf(harness.recorded), hasLength(1));
      },
    );

    testWidgets(
      'Error: a failed create stays open with the mapped copy above the action row, registers nothing, and a retry posts once more (CRT-06, CRT-07)',
      (tester) async {
        var fail = true;
        final harness = await pumpDialog(
          tester,
          handler: (request) async {
            if (request.url.path != '/sheets') {
              return http.Response(detailBody, 200);
            }
            return http.Response(
              fail ? createServerErrorBody : createdBody,
              fail ? 500 : 201,
            );
          },
        );
        await typeName(tester, 'Aragorn');

        await tester.tap(find.byKey(const Key('create-sheet-submit')));
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsOneWidget);
        expect(find.text(errorCopy(ApiErrorKind.serverError)), findsOneWidget);
        final error = find.byKey(const Key('create-sheet-error'));
        expect(
          tester.getTopLeft(error).dy,
          lessThan(
            tester.getTopLeft(find.byKey(const Key('create-sheet-submit'))).dy,
          ),
        );
        expect(containerOf(tester).read(sheetLibraryProvider).entries, isEmpty);

        fail = false;
        await tester.tap(find.byKey(const Key('create-sheet-submit')));
        await tester.pumpAndSettle();

        expect(postsOf(harness.recorded), hasLength(2));
        expect(find.byType(CreateSheetDialog), findsNothing);
      },
    );

    testWidgets(
      'Edge: a local library write failure after a 201 still pops with the sheet and posts once (CRT-11)',
      (tester) async {
        final harness = await pumpDialog(
          tester,
          store: _ThrowingStore(),
          handler: (request) async => http.Response(
            request.url.path == '/sheets' ? createdBody : detailBody,
            request.url.path == '/sheets' ? 201 : 200,
          ),
        );
        await typeName(tester, 'Aragorn');

        await tester.tap(find.byKey(const Key('create-sheet-submit')));
        await tester.pumpAndSettle();

        expect(find.byType(CreateSheetDialog), findsNothing);
        expect(harness.popped(), isNotNull);
        expect(postsOf(harness.recorded), hasLength(1));
      },
    );

    testWidgets(
      'Edge: a whitespace-only name keeps the action inert and posts nothing (CRT-05)',
      (tester) async {
        final harness = await pumpDialog(
          tester,
          handler: (request) async => http.Response(detailBody, 200),
        );
        await typeName(tester, '   ');

        expect(
          tester
              .widget<TextButton>(find.byKey(const Key('create-sheet-submit')))
              .onPressed,
          isNull,
        );
        expect(postsOf(harness.recorded), isEmpty);
        expect(find.byType(CreateSheetDialog), findsOneWidget);
      },
    );

    testWidgets(
      'Edge: the response name wins over the submitted name when they differ (CRT-03)',
      (tester) async {
        const renamedBody =
            '{"data": {"id": "12345678-1234-7000-8000-123456789abc", "template_id": "fantasy_v1", "name": "Strider", "data": {"stats": {"dexterity": 16}}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';
        final harness = await pumpDialog(
          tester,
          handler: (request) async => http.Response(
            request.url.path == '/sheets' ? renamedBody : detailBody,
            request.url.path == '/sheets' ? 201 : 200,
          ),
        );
        await typeName(tester, 'Aragorn');

        await tester.tap(find.byKey(const Key('create-sheet-submit')));
        await tester.pumpAndSettle();

        expect(harness.popped()!.name, 'Strider');
        expect(
          containerOf(tester).read(sheetLibraryProvider).entries.single.name,
          'Strider',
        );
      },
    );
  });
}

/// A sheet library store whose save always fails, to exercise the local-write
/// failure path after a successful `POST /sheets` (CRT-11).
class _ThrowingStore extends SheetLibraryStore {
  _ThrowingStore() : super(directory: Directory.systemTemp);

  @override
  void save(List<SheetLibraryEntry> entries) {
    throw const SheetLibraryException(
      SheetLibraryErrorKind.writeFailed,
      'forced write failure',
    );
  }
}
