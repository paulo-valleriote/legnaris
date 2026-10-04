import 'dart:async';
import 'dart:io';

import 'package:flutter/widgets.dart' show Locale;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/library/sheet_library_store.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:legnaris/sheet/sheet_open_provider.dart';

import '../library/library_fixtures.dart';

/// Unit tests for the sheet-open load state (OPN-05 … OPN-09). Every asserted
/// value comes from the spec rule, never from the implementation.
void main() {
  late Directory supportDir;

  setUp(() {
    supportDir = Directory.systemTemp.createTempSync('sheet_open_');
    addTearDown(() => supportDir.deleteSync(recursive: true));
  });

  const sheetBody =
      '{"data": {"id": "sheet-1", "template_id": "fantasy_v1", "name": "Aragorn", "data": {"stats": {"dexterity": 16}}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';
  const notFoundBody =
      '{"error": {"code": "NOT_FOUND", "message": "no such sheet"}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

  ProviderContainer containerWith({
    required Future<http.Response> Function(http.Request request) handler,
    List<http.Request>? recorded,
    SheetLibraryStore? store,
    Sheet? injected,
    DraftBackupStore? draftStore,
  }) {
    final container = ProviderContainer(
      overrides: [
        appSupportDirectoryProvider.overrideWithValue(supportDir),
        apiClientProvider.overrideWithValue(
          ApiClient(
            httpClient: MockClient((request) async {
              recorded?.add(request);
              return handler(request);
            }),
          ),
        ),
        if (store != null) sheetLibraryStoreProvider.overrideWithValue(store),
        if (draftStore != null)
          draftBackupStoreProvider.overrideWithValue(draftStore),
        if (injected != null) openedSheetProvider.overrideWithValue(injected),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  File draftFile(String id) => File('${supportDir.path}/drafts/$id.json');

  /// Listens to keep the autoDispose open state alive for one test.
  ProviderSubscription<SheetOpenState> keepAlive(
    ProviderContainer container,
    String sheetId,
  ) {
    final subscription = container.listen(
      sheetOpenControllerProvider(sheetId),
      (_, _) {},
    );
    addTearDown(subscription.close);
    return subscription;
  }

  SheetOpenState stateOf(ProviderContainer container, String sheetId) =>
      container.read(sheetOpenControllerProvider(sheetId));

  _FakeStore storeWithEntry(String id) => _FakeStore()
    ..onLoad = () => LibraryLoadResult(
      outcome: LibraryLoadOutcome.loaded,
      entries: [entry(id)],
    );

  Sheet injectedSheet(String id) => Sheet(
    id: id,
    templateId: 'fantasy_v1',
    name: 'Aragorn',
    data: const {
      'stats': {'dexterity': 16},
    },
    createdAt: DateTime.utc(2026, 9, 20, 12),
    updatedAt: DateTime.utc(2026, 9, 20, 12),
  );

  group('loading (OPN-05)', () {
    test(
      'Happy path: starts loading and issues exactly one GET that publishes the sheet',
      () async {
        final recorded = <http.Request>[];
        final gate = Completer<http.Response>();
        final container = containerWith(
          handler: (_) => gate.future,
          recorded: recorded,
        );

        keepAlive(container, 'sheet-1');
        expect(stateOf(container, 'sheet-1').isLoading, isTrue);

        await pumpEventQueue();
        expect(recorded, hasLength(1));
        expect(recorded.single.method, 'GET');
        expect(recorded.single.url.path, '/sheets/sheet-1');

        gate.complete(http.Response(sheetBody, 200));
        await pumpEventQueue();

        final state = stateOf(container, 'sheet-1');
        expect(state.isLoading, isFalse);
        expect(state.sheet?.id, 'sheet-1');
        expect(state.sheet?.name, 'Aragorn');
        expect(state.errorMessage, isNull);
      },
    );

    test(
      'Edge: re-listening after disposal issues a fresh GET — no cache',
      () async {
        final recorded = <http.Request>[];
        final container = containerWith(
          handler: (_) async => http.Response(sheetBody, 200),
          recorded: recorded,
        );

        final first = keepAlive(container, 'sheet-1');
        await pumpEventQueue();
        expect(recorded, hasLength(1));

        first.close();
        await pumpEventQueue();

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();
        expect(recorded, hasLength(2));
        expect(stateOf(container, 'sheet-1').sheet?.id, 'sheet-1');
      },
    );
  });

  group('injected sheet (OPN-09)', () {
    test(
      'Happy path: an injected sheet with the matching id is used without a request',
      () async {
        final recorded = <http.Request>[];
        final injected = injectedSheet('sheet-1');
        final container = containerWith(
          handler: (_) async => http.Response('{}', 500),
          recorded: recorded,
          injected: injected,
        );

        keepAlive(container, 'sheet-1');

        expect(stateOf(container, 'sheet-1').sheet, same(injected));
        await pumpEventQueue();
        expect(stateOf(container, 'sheet-1').sheet, same(injected));
        expect(recorded, isEmpty);
      },
    );

    test(
      'Edge: an injected sheet whose id does not match the key is ignored and fetched',
      () async {
        final recorded = <http.Request>[];
        final container = containerWith(
          handler: (_) async => http.Response(sheetBody, 200),
          recorded: recorded,
          injected: injectedSheet('other-sheet'),
        );

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();

        expect(recorded, hasLength(1));
        expect(stateOf(container, 'sheet-1').sheet?.id, 'sheet-1');
      },
    );
  });

  group('not found (OPN-06)', () {
    test(
      'Error: NOT_FOUND removes the entry and publishes isNotFound with no Retry copy',
      () async {
        final store = storeWithEntry('sheet-1');
        final container = containerWith(
          handler: (_) async => http.Response(notFoundBody, 404),
          store: store,
        );
        expect(container.read(sheetLibraryProvider).entries, hasLength(1));

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();

        final state = stateOf(container, 'sheet-1');
        expect(state.isNotFound, isTrue);
        expect(state.sheet, isNull);
        expect(
          state.errorMessage,
          lookupAppLocalizations(const Locale('en')).sheetMissingMessage,
        );
        expect(container.read(sheetLibraryProvider).entries, isEmpty);
        expect(store.saveCalls, 1);
      },
    );

    test(
      'Edge: a failed eviction write still publishes isNotFound and keeps the entry',
      () async {
        final store = storeWithEntry('sheet-1')
          ..onSave = (_) => throw const SheetLibraryException(
            SheetLibraryErrorKind.writeFailed,
            'disk full',
          );
        final container = containerWith(
          handler: (_) async => http.Response(notFoundBody, 404),
          store: store,
        );

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();

        expect(stateOf(container, 'sheet-1').isNotFound, isTrue);
        expect(container.read(sheetLibraryProvider).entries, hasLength(1));
      },
    );

    test(
      'Happy path: NOT_FOUND removes the draft file and the library entry (DRB-10)',
      () async {
        DraftBackupStore(directory: supportDir).write('sheet-1', {
          'stats': {'dexterity': 18},
        });
        expect(draftFile('sheet-1').existsSync(), isTrue);
        final store = storeWithEntry('sheet-1');
        final container = containerWith(
          handler: (_) async => http.Response(notFoundBody, 404),
          store: store,
        );

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();

        expect(stateOf(container, 'sheet-1').isNotFound, isTrue);
        expect(container.read(sheetLibraryProvider).entries, isEmpty);
        expect(draftFile('sheet-1').existsSync(), isFalse);
      },
    );

    test(
      'Edge: a failed draft delete still publishes the missing-sheet copy (DRB-10)',
      () async {
        final container = containerWith(
          handler: (_) async => http.Response(notFoundBody, 404),
          store: storeWithEntry('sheet-1'),
          draftStore: _ThrowingDeleteStore(directory: supportDir),
        );

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();

        final state = stateOf(container, 'sheet-1');
        expect(state.isNotFound, isTrue);
        expect(
          state.errorMessage,
          lookupAppLocalizations(const Locale('en')).sheetMissingMessage,
        );
      },
    );

    test(
      'Edge: a successful load leaves any draft untouched (DRB-10)',
      () async {
        DraftBackupStore(directory: supportDir).write('sheet-1', {
          'stats': {'dexterity': 18},
        });
        final container = containerWith(
          handler: (_) async => http.Response(sheetBody, 200),
          store: storeWithEntry('sheet-1'),
        );

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();

        expect(stateOf(container, 'sheet-1').sheet?.id, 'sheet-1');
        expect(draftFile('sheet-1').existsSync(), isTrue);
      },
    );
  });

  group('retryable failure (OPN-07, OPN-08)', () {
    test(
      'Error: a server failure keeps the entry and publishes the mapped copy',
      () async {
        final store = storeWithEntry('sheet-1');
        final container = containerWith(
          handler: (_) async => http.Response(serverErrorBody, 500),
          store: store,
        );

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();

        final state = stateOf(container, 'sheet-1');
        expect(state.isNotFound, isFalse);
        expect(state.sheet, isNull);
        expect(state.errorMessage, errorCopy(ApiErrorKind.serverError));
        expect(container.read(sheetLibraryProvider).entries, hasLength(1));
        expect(store.saveCalls, 0);
      },
    );

    test(
      'Edge: a parse failure keeps the entry and publishes the defensive copy',
      () async {
        final store = storeWithEntry('sheet-1');
        final container = containerWith(
          handler: (_) async => http.Response('not json', 200),
          store: store,
        );

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();

        expect(stateOf(container, 'sheet-1').errorMessage,
            lookupAppLocalizations(const Locale('en')).errorParse);
        expect(container.read(sheetLibraryProvider).entries, hasLength(1));
      },
    );

    test(
      'Edge: two rapid loads issue one request and a later Retry issues one more',
      () async {
        final recorded = <http.Request>[];
        final first = Completer<http.Response>();
        var call = 0;
        final container = containerWith(
          handler: (_) {
            call++;
            return call == 1
                ? first.future
                : Future.value(http.Response(sheetBody, 200));
          },
          recorded: recorded,
        );

        keepAlive(container, 'sheet-1');
        await pumpEventQueue();
        final notifier = container.read(
          sheetOpenControllerProvider('sheet-1').notifier,
        );

        final a = notifier.load();
        final b = notifier.load();
        await pumpEventQueue();
        expect(recorded, hasLength(1));

        first.complete(http.Response(serverErrorBody, 500));
        await a;
        await b;
        await pumpEventQueue();
        expect(stateOf(container, 'sheet-1').errorMessage,
            errorCopy(ApiErrorKind.serverError));

        await notifier.load();
        await pumpEventQueue();
        expect(recorded, hasLength(2));
        expect(stateOf(container, 'sheet-1').sheet?.id, 'sheet-1');
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

/// A draft store whose delete always fails, proving the eviction is best-effort.
class _ThrowingDeleteStore extends DraftBackupStore {
  _ThrowingDeleteStore({required super.directory});

  @override
  void delete(String sheetId) => throw const DraftBackupException('disk full');
}
