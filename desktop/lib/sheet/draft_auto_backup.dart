/// Auto-backup coordinator for one open sheet's unsaved draft (DRB-01 …
/// DRB-03, DRB-13).
///
/// While the edit controller reports pending edits, a per-sheet scheduler
/// ticks once a minute and writes the current pending delta through the draft
/// store. When the sheet is clean the scheduler stops, and disposing the
/// provider cancels it. Every write is best-effort: a failed write never blocks
/// editing or surfaces an error.
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'draft_auto_backup.g.dart';

/// Per-sheet 60-second clock for the draft auto-backup (DRB-01).
///
/// The schedule is time-based and independent of API traffic, and tests drive
/// it through the widget tester's fake clock, so no test waits a real minute.
class _DraftBackupScheduler {
  _DraftBackupScheduler();

  static const Duration _defaultInterval = Duration(seconds: 60);

  Timer? _timer;

  void start(void Function() onTick) {
    cancel();
    _timer = Timer.periodic(_defaultInterval, (_) => onTick());
  }

  void cancel() {
    _timer?.cancel();
    _timer = null;
  }
}

/// Runs the periodic draft backup for [sheetId] while its edit state has
/// pending edits, and returns whether the backup clock is running.
///
/// One scheduler runs per dirty transition: a rebuild that leaves `isPending`
/// true does not restart the timer. The write is best-effort so a full disk
/// never interrupts editing (DRB-13).
@riverpod
bool draftAutoBackup(Ref ref, String sheetId) {
  final scheduler = _DraftBackupScheduler();
  final store = ref.watch(draftBackupStoreProvider);
  final isPending = ref.watch(
    sheetEditControllerProvider(
      sheetId,
    ).select((state) => state.isPending),
  );

  ref.onDispose(scheduler.cancel);

  if (!isPending) {
    scheduler.cancel();
    return false;
  }

  scheduler.start(() {
    final delta = ref.read(sheetEditControllerProvider(sheetId)).pendingDelta;
    if (delta.isEmpty) return;
    try {
      store.write(sheetId, delta);
    } on Exception {
      // Best-effort: a failed backup never blocks editing (DRB-13).
      // Only catch Exception, not Error (e.g., OutOfMemoryError).
    }
  });
  return true;
}
