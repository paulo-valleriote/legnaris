/// Loads one stored sheet opened on Screen 2 (OPN-05 … OPN-09).
///
/// The family is keyed by the sheet id and autoDispose: the state lives only
/// while Screen 2 is mounted, so a pop discards it and a reopen fetches again
/// with no cache. The creation flow injects the `201` payload through
/// [openedSheetProvider]; when it matches the key this controller returns it
/// without a request, so LGN-109 never issues a second `GET /sheets/:id`.
library;

import 'package:flutter/widgets.dart' show Locale;
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/library/sheet_library_exception.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/l10n/app_localizations.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'sheet_open_provider.g.dart';

/// Render state of the sheet an open Screen 2 is loading (OPN-05 … OPN-09).
class SheetOpenState {
  const SheetOpenState({
    this.sheet,
    this.isLoading = false,
    this.errorMessage,
    this.isNotFound = false,
  });

  /// Decoded sheet after a success or the injected `201` payload; null before.
  final Sheet? sheet;

  /// A sheet request is in flight (first load or Retry).
  final bool isLoading;

  /// Resolved UI copy of the last failure; null after a success.
  final String? errorMessage;

  /// The sheet no longer exists in the API: the local entry was evicted and
  /// the UI must not offer Retry (OPN-06).
  final bool isNotFound;
}

/// Load state of one stored sheet, fetched on demand for Screen 2 (OPN-05).
@riverpod
class SheetOpenController extends _$SheetOpenController {
  bool _inFlight = false;

  @override
  SheetOpenState build(String sheetId) {
    final injected = ref.watch(openedSheetProvider);
    if (injected != null && injected.id == sheetId) {
      return SheetOpenState(sheet: injected);
    }
    ref.watch(apiClientProvider);
    Future.microtask(load);
    return const SheetOpenState(isLoading: true);
  }

  /// Loads the sheet unless a request is already in flight (single-flight, so
  /// Retry never doubles up). `NOT_FOUND` evicts the local entry and publishes
  /// the missing-sheet copy with no Retry (OPN-06); every other failure keeps
  /// the entry and publishes the mapped copy (OPN-07).
  Future<void> load() async {
    if (_inFlight) return;
    _inFlight = true;
    state = const SheetOpenState(isLoading: true);
    try {
      final sheet = await ref.read(apiClientProvider).getSheet(sheetId);
      if (!ref.mounted) return;
      state = SheetOpenState(sheet: sheet);
    } on ApiDomainException catch (error) {
      if (!ref.mounted) return;
      if (error.kind == ApiErrorKind.notFound) {
        _evict();
        state = SheetOpenState(
          errorMessage: lookupAppLocalizations(const Locale('en'))
              .sheetMissingMessage,
          isNotFound: true,
        );
      } else {
        state = SheetOpenState(errorMessage: failureCopy(error));
      }
    } catch (error) {
      if (!ref.mounted) return;
      state = SheetOpenState(errorMessage: failureCopy(error));
    } finally {
      _inFlight = false;
    }
  }

  /// Drops the entry from the local library best-effort and deletes the sheet's
  /// draft backup: neither a library write failure nor a draft delete failure
  /// may hide the missing-sheet message (DRB-10, edge case).
  void _evict() {
    try {
      ref.read(sheetLibraryProvider.notifier).remove(sheetId);
    } on SheetLibraryException {
      // The message still renders; the entry stays at the last persisted state.
    }
    try {
      ref.read(draftBackupStoreProvider).delete(sheetId);
    } catch (_) {
      // Best-effort: a failed draft delete still publishes the missing copy.
    }
  }
}
