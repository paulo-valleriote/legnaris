import 'package:legnaris/connection/health_scheduler.dart';

/// Test double for [HealthScheduler]: stores the registered tick so a test
/// fires it on command instead of waiting a real interval (CONN-12), and
/// records how many times the schedule was started and cancelled.
class ManualHealthScheduler implements HealthScheduler {
  int startCount = 0;
  int cancelCount = 0;

  void Function()? _onTick;

  @override
  void start(void Function() onTick) {
    startCount += 1;
    _onTick = onTick;
  }

  @override
  void cancel() {
    cancelCount += 1;
    _onTick = null;
  }

  /// Invokes the registered tick, exactly as a scheduled interval would.
  void fire() => _onTick?.call();
}
