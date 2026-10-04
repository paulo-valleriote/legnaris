/// Auto-backup coordinator for one open sheet's unsaved draft (DRB-01 …
/// DRB-03, DRB-13).
///
/// While the edit controller reports pending edits, the injectable scheduler
/// ticks once a minute and writes the current pending delta through the draft
/// store. When the sheet is clean the scheduler stops, and disposing the
/// provider cancels it. Every write is best-effort: a failed write never blocks
/// editing or surfaces an error.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:legnaris/sheet/draft_backup_scheduler.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'draft_auto_backup.g.dart';

/// Runs the periodic draft backup for [sheetId] while its edit state has
/// pending edits, and returns whether the backup clock is running.
///
/// One scheduler runs per dirty transition: a rebuild that leaves `isPending`
/// true does not restart the timer. The write is best-effort so a full disk
/// never interrupts editing (DRB-13).
@riverpod
bool draftAutoBackup(Ref ref, String sheetId) {
  final scheduler = ref.watch(draftBackupSchedulerProvider);
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
    } catch (_) {
      // Best-effort: a failed backup never blocks editing (DRB-13).
    }
  });
  return true;
}
