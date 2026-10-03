import 'dart:io';

import 'package:legnaris/library/sheet_library_entry.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'sheet_library_provider.g.dart';

/// The OS application-support directory local persistence is rooted at.
///
/// The body throws until overridden: `main()` resolves it through
/// `path_provider` at bootstrap and every test overrides it with a temp
/// directory, so the platform channel is never hit by a test.
@Riverpod(keepAlive: true)
Directory appSupportDirectory(Ref ref) {
  throw StateError(
    'appSupportDirectoryProvider must be overridden at bootstrap',
  );
}

/// The sheet library persistence boundary over the injected support directory.
@Riverpod(keepAlive: true)
SheetLibraryStore sheetLibraryStore(Ref ref) =>
    SheetLibraryStore(directory: ref.watch(appSupportDirectoryProvider));

/// The published state of the local sheet index: the sorted list view and how
/// the store loaded.
class SheetLibraryState {
  const SheetLibraryState({required this.entries, required this.outcome});

  /// "Minhas fichas" list view: `created_at` descending, ties broken by
  /// reverse insertion order (LIB-04).
  final List<SheetLibraryEntry> entries;

  /// How the store loaded at build time (LIB-19).
  final LibraryLoadOutcome outcome;
}

/// The in-memory sheet index and the only state the surfaces read (LIB-21 …
/// LIB-23).
///
/// Write-through: [upsert] and [remove] persist the complete index before
/// publishing, so a failed save leaves the published entries equal to the last
/// persisted index.
@Riverpod(keepAlive: true)
class SheetLibrary extends _$SheetLibrary {
  late List<SheetLibraryEntry> _insertionOrder;

  @override
  SheetLibraryState build() {
    final result = ref.watch(sheetLibraryStoreProvider).load();
    _insertionOrder = List.of(result.entries);
    return _publish(result.outcome);
  }

  /// Replaces the entry with a matching id, or appends a new one, then
  /// persists the full index (LIB-02, LIB-03, LIB-10).
  void upsert(SheetLibraryEntry entry) {
    final next = List.of(_insertionOrder);
    final index = next.indexWhere((stored) => stored.id == entry.id);
    if (index == -1) {
      next.add(entry);
    } else {
      next[index] = entry;
    }
    _persist(next);
    _insertionOrder = next;
    state = _publish(state.outcome);
  }

  /// Removes the entry with [id] and persists; an absent id is a no-op
  /// (LIB-05, LIB-06).
  void remove(String id) {
    final next = List.of(_insertionOrder);
    final index = next.indexWhere((stored) => stored.id == id);
    if (index == -1) {
      return;
    }
    next.removeAt(index);
    _persist(next);
    _insertionOrder = next;
    state = _publish(state.outcome);
  }

  /// The entry with [id], or null (LIB-07).
  SheetLibraryEntry? entryFor(String id) {
    for (final entry in _insertionOrder) {
      if (entry.id == id) {
        return entry;
      }
    }
    return null;
  }

  SheetLibraryState _publish(LibraryLoadOutcome outcome) => SheetLibraryState(
    entries: List.unmodifiable(_sorted()),
    outcome: outcome,
  );

  List<SheetLibraryEntry> _sorted() {
    final indexed = _insertionOrder.indexed.toList();
    indexed.sort((left, right) {
      final byCreatedAt = right.$2.createdAt.compareTo(left.$2.createdAt);
      if (byCreatedAt != 0) {
        return byCreatedAt;
      }
      return right.$1.compareTo(left.$1);
    });
    return [for (final (_, entry) in indexed) entry];
  }

  /// Persists [next] first; a typed store failure propagates untouched and an
  /// unexpected one is reported as [SheetLibraryErrorKind.writeFailed].
  void _persist(List<SheetLibraryEntry> next) {
    try {
      ref.read(sheetLibraryStoreProvider).save(next);
    } on SheetLibraryException {
      rethrow;
    } catch (error) {
      throw SheetLibraryException(
        SheetLibraryErrorKind.writeFailed,
        'failed to persist the sheet library: $error',
      );
    }
  }
}
