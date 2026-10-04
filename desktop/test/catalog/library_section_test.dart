import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/catalog/library_section.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/library/sheet_library_store.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:legnaris/theme/app_theme.dart';

import '../library/library_fixtures.dart';

/// Widget tests for the "My sheets" section (OPN-01 … OPN-03, OPN-12 … OPN-15).
void main() {
  Future<List<http.Request>> pumpSection(
    WidgetTester tester, {
    required SheetLibraryStore store,
    ValueChanged<SheetLibraryEntry>? onOpen,
    Directory? supportDir,
  }) async {
    final recorded = <http.Request>[];
    final directory =
        supportDir ?? Directory.systemTemp.createTempSync('library_section_');
    if (supportDir == null) {
      addTearDown(() => directory.deleteSync(recursive: true));
    }
    tester.view.physicalSize = const Size(1280, 800);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appSupportDirectoryProvider.overrideWithValue(directory),
          sheetLibraryStoreProvider.overrideWithValue(store),
          apiClientProvider.overrideWithValue(
            ApiClient(
              httpClient: MockClient((request) async {
                recorded.add(request);
                return http.Response('{}', 200);
              }),
            ),
          ),
        ],
        child: MaterialApp(
          theme: buildAppTheme(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: LibrarySection(onOpen: onOpen ?? (_) {}),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return recorded;
  }

  File draftFile(Directory supportDir, String id) =>
      File('${supportDir.path}/drafts/$id.json');

  group('listing (OPN-01 … OPN-03)', () {
    testWidgets(
      'Happy path: lists every entry newest-first with name, template id and both actions (OPN-01, OPN-03)',
      (tester) async {
        final store = _FakeStore()
          ..onLoad = () => LibraryLoadResult(
            outcome: LibraryLoadOutcome.loaded,
            entries: [
              entry('old', name: 'Old', createdAt: '2026-10-01T10:00:00.000Z'),
              entry('new', name: 'New', createdAt: '2026-10-03T10:00:00.000Z'),
            ],
          );

        await pumpSection(tester, store: store);

        expect(find.text('My sheets'), findsOneWidget);
        final newer = find.byKey(const Key('library-name-new'));
        final older = find.byKey(const Key('library-name-old'));
        expect(newer, findsOneWidget);
        expect(older, findsOneWidget);
        expect(
          tester.getTopLeft(newer).dy,
          lessThan(tester.getTopLeft(older).dy),
          reason: 'entries must read newest-first',
        );
        expect(find.text('fantasy_v1'), findsNWidgets(2));
        expect(find.byKey(const Key('library-open-new')), findsOneWidget);
        expect(find.byKey(const Key('library-remove-new')), findsOneWidget);
        expect(find.byKey(const Key('library-open-old')), findsOneWidget);
        expect(find.byKey(const Key('library-remove-old')), findsOneWidget);
      },
    );

    testWidgets(
      'Happy path: the Open action reports the tapped entry to the caller (OPN-04)',
      (tester) async {
        final opened = <String>[];
        final store = _FakeStore()
          ..onLoad = () => LibraryLoadResult(
            outcome: LibraryLoadOutcome.loaded,
            entries: [entry('id-1')],
          );

        await pumpSection(tester, store: store, onOpen: (e) => opened.add(e.id));

        await tester.tap(find.byKey(const Key('library-open-id-1')));
        await tester.pumpAndSettle();

        expect(opened, ['id-1']);
      },
    );

    testWidgets(
      'Edge: an empty library renders one localized line and no rows (OPN-02)',
      (tester) async {
        await pumpSection(tester, store: _FakeStore());

        expect(find.text('My sheets'), findsOneWidget);
        expect(find.text('No sheets created yet'), findsOneWidget);
        expect(find.byType(TextButton), findsNothing);
      },
    );
  });

  group('removing an entry (OPN-12 … OPN-15)', () {
    _FakeStore storeWithOne() => _FakeStore()
      ..onLoad = () => LibraryLoadResult(
        outcome: LibraryLoadOutcome.loaded,
        entries: [entry('id-1', name: 'Aria')],
      );

    testWidgets(
      'Happy path: Remove confirms, drops the entry and issues zero API requests (OPN-12, OPN-13)',
      (tester) async {
        final store = storeWithOne();
        final recorded = await pumpSection(tester, store: store);

        await tester.tap(find.byKey(const Key('library-remove-id-1')));
        await tester.pumpAndSettle();

        final dialog = find.byType(AlertDialog);
        expect(dialog, findsOneWidget);
        expect(find.text('Remove from local library?'), findsOneWidget);
        expect(
          find.textContaining('stays in the API'),
          findsOneWidget,
        );

        await tester.tap(
          find.descendant(
            of: dialog,
            matching: find.widgetWithText(TextButton, 'Remove'),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('library-name-id-1')), findsNothing);
        expect(find.text('No sheets created yet'), findsOneWidget);
        expect(store.saveCalls, 1);
        expect(recorded, isEmpty, reason: 'removal is local-only (OPN-13)');
      },
    );

    testWidgets(
      'Edge: cancelling the dialog keeps the entry with zero requests (OPN-14)',
      (tester) async {
        final store = storeWithOne();
        final recorded = await pumpSection(tester, store: store);

        await tester.tap(find.byKey(const Key('library-remove-id-1')));
        await tester.pumpAndSettle();
        await tester.tap(
          find.descendant(
            of: find.byType(AlertDialog),
            matching: find.widgetWithText(TextButton, 'Cancel'),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('library-name-id-1')), findsOneWidget);
        expect(store.saveCalls, 0);
        expect(recorded, isEmpty);
      },
    );

    testWidgets(
      'Error: a failed removal keeps the entry and renders the inline error (OPN-15)',
      (tester) async {
        final store = storeWithOne()
          ..onSave = (_) => throw const SheetLibraryException(
            SheetLibraryErrorKind.writeFailed,
            'disk full',
          );
        final recorded = await pumpSection(tester, store: store);

        await tester.tap(find.byKey(const Key('library-remove-id-1')));
        await tester.pumpAndSettle();
        await tester.tap(
          find.descendant(
            of: find.byType(AlertDialog),
            matching: find.widgetWithText(TextButton, 'Remove'),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('library-name-id-1')), findsOneWidget);
        expect(find.byKey(const Key('library-remove-error')), findsOneWidget);
        expect(recorded, isEmpty);
      },
    );

    testWidgets(
      'Happy path: confirming the removal deletes the sheet draft file (DRB-11)',
      (tester) async {
        final supportDir = Directory.systemTemp.createTempSync('library_draft_');
        addTearDown(() => supportDir.deleteSync(recursive: true));
        DraftBackupStore(directory: supportDir).write('id-1', {
          'stats': {'dexterity': 18},
        });
        final store = storeWithOne();
        final recorded = await pumpSection(
          tester,
          store: store,
          supportDir: supportDir,
        );

        await tester.tap(find.byKey(const Key('library-remove-id-1')));
        await tester.pumpAndSettle();
        await tester.tap(
          find.descendant(
            of: find.byType(AlertDialog),
            matching: find.widgetWithText(TextButton, 'Remove'),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('library-name-id-1')), findsNothing);
        expect(draftFile(supportDir, 'id-1').existsSync(), isFalse);
        expect(recorded, isEmpty);
      },
    );

    testWidgets(
      'Edge: a failed removal leaves the draft in place (DRB-11)',
      (tester) async {
        final supportDir = Directory.systemTemp.createTempSync('library_draft_');
        addTearDown(() => supportDir.deleteSync(recursive: true));
        DraftBackupStore(directory: supportDir).write('id-1', {
          'stats': {'dexterity': 18},
        });
        final store = storeWithOne()
          ..onSave = (_) => throw const SheetLibraryException(
            SheetLibraryErrorKind.writeFailed,
            'disk full',
          );
        await pumpSection(tester, store: store, supportDir: supportDir);

        await tester.tap(find.byKey(const Key('library-remove-id-1')));
        await tester.pumpAndSettle();
        await tester.tap(
          find.descendant(
            of: find.byType(AlertDialog),
            matching: find.widgetWithText(TextButton, 'Remove'),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('library-name-id-1')), findsOneWidget);
        expect(find.byKey(const Key('library-remove-error')), findsOneWidget);
        expect(draftFile(supportDir, 'id-1').existsSync(), isTrue);
      },
    );

    testWidgets(
      'Edge: cancelling the removal deletes nothing (DRB-11)',
      (tester) async {
        final supportDir = Directory.systemTemp.createTempSync('library_draft_');
        addTearDown(() => supportDir.deleteSync(recursive: true));
        DraftBackupStore(directory: supportDir).write('id-1', {
          'stats': {'dexterity': 18},
        });
        final store = storeWithOne();
        await pumpSection(tester, store: store, supportDir: supportDir);

        await tester.tap(find.byKey(const Key('library-remove-id-1')));
        await tester.pumpAndSettle();
        await tester.tap(
          find.descendant(
            of: find.byType(AlertDialog),
            matching: find.widgetWithText(TextButton, 'Cancel'),
          ),
        );
        await tester.pumpAndSettle();

        expect(find.byKey(const Key('library-name-id-1')), findsOneWidget);
        expect(draftFile(supportDir, 'id-1').existsSync(), isTrue);
        expect(store.saveCalls, 0);
      },
    );
  });
}

/// A store double: overrides [load]/[save] and never touches the filesystem.
class _FakeStore extends SheetLibraryStore {
  _FakeStore() : super(directory: Directory('/legnaris-fake'));

  LibraryLoadResult Function()? onLoad;

  void Function(List<SheetLibraryEntry> entries)? onSave;

  int saveCalls = 0;

  @override
  LibraryLoadResult load() =>
      onLoad?.call() ??
      const LibraryLoadResult(
        outcome: LibraryLoadOutcome.empty,
        entries: [],
      );

  @override
  void save(List<SheetLibraryEntry> entries) {
    saveCalls++;
    onSave?.call(entries);
  }
}
