import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show ProviderException;
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/library/sheet_library_store.dart';

/// Unit tests for the store providers and the directory seam (LIB-21).
void main() {
  late Directory root;
  late Directory supportDir;

  setUp(() {
    root = Directory.systemTemp.createTempSync('sheet_library_provider_');
    supportDir = Directory('${root.path}/app_support');
    addTearDown(() => root.deleteSync(recursive: true));
  });

  test('Happy path: the directory provider returns the overridden temp dir', () {
    final container = ProviderContainer(
      overrides: [appSupportDirectoryProvider.overrideWithValue(supportDir)],
    );
    addTearDown(container.dispose);

    expect(container.read(appSupportDirectoryProvider).path, supportDir.path);
  });

  test('Happy path: the store provider is rooted at the overridden directory', () {
    final container = ProviderContainer(
      overrides: [appSupportDirectoryProvider.overrideWithValue(supportDir)],
    );
    addTearDown(container.dispose);

    final store = container.read(sheetLibraryStoreProvider);

    expect(store.directory.path, supportDir.path);
    expect(store.fileName, 'sheet_library.json');
  });

  test('Error: reading the directory provider without an override throws StateError', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    expect(
      () => container.read(appSupportDirectoryProvider),
      throwsA(
        isA<ProviderException>().having(
          (error) => error.exception,
          'exception',
          isA<StateError>(),
        ),
      ),
    );
  });

  SheetLibraryEntry entry(
    String id, {
    String name = 'Aria',
    String createdAt = '2026-10-02T15:31:28.401Z',
  }) => SheetLibraryEntry(
    id: id,
    name: name,
    templateId: 'fantasy_v1',
    createdAt: DateTime.parse(createdAt),
  );

  ProviderContainer containerWith(_FakeStore store) {
    final container = ProviderContainer(
      overrides: [sheetLibraryStoreProvider.overrideWithValue(store)],
    );
    addTearDown(container.dispose);
    return container;
  }

  List<String> idsOf(ProviderContainer container) => container
      .read(sheetLibraryProvider)
      .entries
      .map((stored) => stored.id)
      .toList();

  test('Happy path: build loads the store and publishes entries and outcome (LIB-21)', () {
    final store = _FakeStore()
      ..onLoad = () => LibraryLoadResult(
        outcome: LibraryLoadOutcome.loaded,
        entries: [
          entry('id-1', createdAt: '2026-10-02T15:00:00.000Z'),
          entry('id-2', createdAt: '2026-10-02T16:00:00.000Z'),
        ],
      );
    final container = containerWith(store);

    final state = container.read(sheetLibraryProvider);

    expect(state.outcome, LibraryLoadOutcome.loaded);
    expect(idsOf(container), ['id-2', 'id-1']);
  });

  test('Happy path: upsert appends a new id and persists the full index (LIB-02)', () {
    final saved = <List<SheetLibraryEntry>>[];
    final store = _FakeStore()..onSave = saved.add;
    final container = containerWith(store);

    container.read(sheetLibraryProvider.notifier).upsert(entry('id-1'));

    expect(idsOf(container), ['id-1']);
    expect(saved, hasLength(1));
    expect(saved.single.map((stored) => stored.id), ['id-1']);
  });

  test('Happy path: upsert on an existing id replaces it without duplicating (LIB-03)', () {
    final store = _FakeStore()
      ..onLoad = () => LibraryLoadResult(
        outcome: LibraryLoadOutcome.loaded,
        entries: [entry('id-1', name: 'Old')],
      );
    final container = containerWith(store);

    container
        .read(sheetLibraryProvider.notifier)
        .upsert(entry('id-1', name: 'New'));

    final entries = container.read(sheetLibraryProvider).entries;
    expect(entries, hasLength(1));
    expect(entries.single.name, 'New');
  });

  test('Edge: entries are ordered by created_at descending, ties by reverse insertion (LIB-04)', () {
    final container = containerWith(_FakeStore());
    final notifier = container.read(sheetLibraryProvider.notifier);

    notifier.upsert(entry('oldest', createdAt: '2026-10-01T10:00:00.000Z'));
    notifier.upsert(entry('tie-a', createdAt: '2026-10-03T10:00:00.000Z'));
    notifier.upsert(entry('newest', createdAt: '2026-10-04T10:00:00.000Z'));
    notifier.upsert(entry('tie-b', createdAt: '2026-10-03T10:00:00.000Z'));

    expect(idsOf(container), ['newest', 'tie-b', 'tie-a', 'oldest']);
  });

  test('Happy path: remove deletes a present id and persists the reduced index (LIB-05)', () {
    final saved = <List<SheetLibraryEntry>>[];
    final store = _FakeStore();
    store.onLoad = () => LibraryLoadResult(
      outcome: LibraryLoadOutcome.loaded,
      entries: [entry('id-1'), entry('id-2')],
    );
    store.onSave = saved.add;
    final container = containerWith(store);

    container.read(sheetLibraryProvider.notifier).remove('id-1');

    expect(idsOf(container), ['id-2']);
    expect(saved.single.map((stored) => stored.id), ['id-2']);
  });

  test('Edge: removing a missing id is a no-op and calls save zero times (LIB-06)', () {
    final store = _FakeStore()
      ..onLoad = () => LibraryLoadResult(
        outcome: LibraryLoadOutcome.loaded,
        entries: [entry('id-1')],
      );
    final container = containerWith(store);

    container.read(sheetLibraryProvider.notifier).remove('missing');

    expect(idsOf(container), ['id-1']);
    expect(store.saveCalls, 0);
  });

  test('Happy path: entryFor returns the matching entry (LIB-07)', () {
    final store = _FakeStore()
      ..onLoad = () => LibraryLoadResult(
        outcome: LibraryLoadOutcome.loaded,
        entries: [entry('id-1', name: 'Aria')],
      );
    final container = containerWith(store);

    final found = container.read(sheetLibraryProvider.notifier).entryFor('id-1');

    expect(found?.name, 'Aria');
  });

  test('Edge: entryFor returns null for a missing id (LIB-07)', () {
    final container = containerWith(_FakeStore());

    expect(
      container.read(sheetLibraryProvider.notifier).entryFor('missing'),
      isNull,
    );
  });

  test('Error: a failed save leaves the published entries at the last persisted state and rethrows (LIB-11, LIB-23)', () {
    final store = _FakeStore();
    store.onLoad = () => LibraryLoadResult(
      outcome: LibraryLoadOutcome.loaded,
      entries: [entry('id-1', name: 'Persisted')],
    );
    store.onSave = (entries) => throw const SheetLibraryException(
      SheetLibraryErrorKind.writeFailed,
      'disk full',
    );
    final container = containerWith(store);
    final before = idsOf(container);

    expect(
      () => container.read(sheetLibraryProvider.notifier).upsert(entry('id-2')),
      throwsA(
        isA<SheetLibraryException>().having(
          (error) => error.kind,
          'kind',
          SheetLibraryErrorKind.writeFailed,
        ),
      ),
    );
    final after = container.read(sheetLibraryProvider).entries;
    expect(after.map((stored) => stored.id), before);
    expect(after, hasLength(1));
  });

  test('Happy path: a successful mutation publishes so the read provider rebuilds (LIB-22)', () {
    final container = containerWith(_FakeStore());
    final emissions = <List<String>>[];
    final subscription = container.listen(sheetLibraryProvider, (_, next) {
      emissions.add(next.entries.map((stored) => stored.id).toList());
    });
    addTearDown(subscription.close);

    container.read(sheetLibraryProvider.notifier).upsert(entry('id-1'));

    expect(emissions.last, ['id-1']);
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
