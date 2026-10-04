/// Rolling 60-second action windows and the one-shot cooldown that re-enables
/// a blocked Preview or Save (SVE-02 … SVE-05).
///
/// Both the clock and the cooldown timer are injected: a test drives a
/// controllable clock and the widget tester's fake timer, so no test waits a
/// real minute.
library;

import 'dart:async';

import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'action_rate_limiter.g.dart';

/// The injectable clock behind the rolling windows. Production uses
/// [DateTime.now]; tests override it to advance time deterministically.
@Riverpod(keepAlive: true)
DateTime Function() clock(Ref ref) => DateTime.now;

/// Counts the requests issued in the trailing [window] and blocks once
/// [maxRequests] are recorded (SVE-02, SVE-03, SVE-05).
///
/// A record exactly [window] old is already outside the window, so a slot
/// frees the instant the oldest request ages out (SVE-04).
class RollingWindowLimiter {
  RollingWindowLimiter({
    required this.maxRequests,
    required this.window,
    DateTime Function()? now,
  }) : _now = now ?? DateTime.now;

  /// Number of requests allowed inside [window] before the action blocks.
  final int maxRequests;

  /// Length of the rolling window.
  final Duration window;

  final DateTime Function() _now;

  final List<DateTime> _records = [];

  void _prune() {
    final cutoff = _now().subtract(window);
    _records.removeWhere((at) => !at.isAfter(cutoff));
  }

  /// True while [maxRequests] requests were recorded inside the window, so the
  /// action must stay disabled.
  bool get isBlocked {
    _prune();
    return _records.length >= maxRequests;
  }

  /// Records one issued request at the current time.
  void record() {
    _prune();
    _records.add(_now());
  }

  /// The earliest instant a slot frees, when currently blocked; null when the
  /// action is not blocked. Derived from the oldest record plus [window].
  DateTime? nextFreeAt() {
    _prune();
    if (_records.length < maxRequests) {
      return null;
    }
    return _records.first.add(window);
  }
}

/// Injectable one-shot timer used to re-check a blocked action when its window
/// frees (SVE-04). [schedule] replaces any pending fire and [cancel] is a
/// no-op when nothing runs.
abstract interface class CooldownScheduler {
  /// Fires [onFire] once after [delay], replacing any pending schedule.
  void schedule(Duration delay, void Function() onFire);

  /// Cancels the pending fire, if any.
  void cancel();
}

/// [CooldownScheduler] over a single [Timer].
class TimerCooldownScheduler implements CooldownScheduler {
  Timer? _timer;

  @override
  void schedule(Duration delay, void Function() onFire) {
    cancel();
    _timer = Timer(delay, () {
      _timer = null;
      onFire();
    });
  }

  @override
  void cancel() {
    _timer?.cancel();
    _timer = null;
  }
}

/// App-wide cooldown clock. The provider owns the timer: [Ref.onDispose]
/// cancels it, so tearing down the container never leaves a pending timer.
@Riverpod(keepAlive: true)
CooldownScheduler cooldownScheduler(Ref ref) {
  final scheduler = TimerCooldownScheduler();
  ref.onDispose(scheduler.cancel);
  return scheduler;
}
