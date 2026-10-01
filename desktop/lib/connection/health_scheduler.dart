import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'health_scheduler.g.dart';

/// Injectable polling clock of the connection health check. The schedule is
/// time-based and independent of API traffic (CONN-07), and tests replace it
/// with a scheduler they drive, so no test waits a real 15-second interval
/// (CONN-12).
abstract interface class HealthScheduler {
  /// Begins firing [onTick] every interval until [cancel] is called.
  void start(void Function() onTick);

  /// Stops the schedule; calling it when nothing runs is a no-op.
  void cancel();
}

/// [HealthScheduler] over a single [Timer.periodic]. [start] cancels any
/// existing timer first, so a second start never doubles the tick rate, and
/// [cancel] is idempotent.
class PeriodicHealthScheduler implements HealthScheduler {
  PeriodicHealthScheduler({this.interval = defaultInterval});

  /// Spec polling interval: one health check every 15 seconds (CONN-01).
  static const Duration defaultInterval = Duration(seconds: 15);

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

/// App-wide polling clock. The provider owns the timer: [Ref.onDispose]
/// cancels it, so tearing down the container never leaves a pending timer.
@Riverpod(keepAlive: true)
HealthScheduler healthScheduler(Ref ref) {
  final scheduler = PeriodicHealthScheduler();
  ref.onDispose(scheduler.cancel);
  return scheduler;
}
