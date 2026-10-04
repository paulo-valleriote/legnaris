import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'draft_backup_scheduler.g.dart';

/// Injectable 60-second clock for the draft auto-backup (DRB-01).
///
/// The schedule is time-based and independent of API traffic, and tests drive
/// it through the widget tester's fake clock, so no test waits a real minute.
abstract interface class DraftBackupScheduler {
  /// Begins firing [onTick] every interval until [cancel] is called.
  void start(void Function() onTick);

  /// Stops the schedule; calling it when nothing runs is a no-op.
  void cancel();
}

/// [DraftBackupScheduler] over a single [Timer.periodic]. [start] cancels any
/// existing timer first, so a second start never doubles the tick rate, and
/// [cancel] is idempotent.
class PeriodicDraftBackupScheduler implements DraftBackupScheduler {
  PeriodicDraftBackupScheduler({this.interval = defaultInterval});

  /// Spec backup interval: one draft write every 60 seconds (DRB-01).
  static const Duration defaultInterval = Duration(seconds: 60);

  /// Interval between ticks.
  final Duration interval;

  Timer? _timer;

  @override
  void start(void Function() onTick) {
    cancel();
    _timer = Timer.periodic(interval, (_) => onTick());
  }

  @override
  void cancel() {
    _timer?.cancel();
    _timer = null;
  }
}

/// App-wide draft backup clock. The provider owns the timer:
/// [Ref.onDispose] cancels it, so tearing down the container never leaves a
/// pending timer.
@Riverpod(keepAlive: true)
DraftBackupScheduler draftBackupScheduler(Ref ref) {
  final scheduler = PeriodicDraftBackupScheduler();
  ref.onDispose(scheduler.cancel);
  return scheduler;
}
