import 'package:flutter/widgets.dart' show SizedBox;
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/sheet/action_rate_limiter.dart';

/// Unit tests for the rolling action window and the one-shot cooldown
/// (SVE-02 … SVE-05). A controllable clock and the widget tester's fake timer
/// keep the tests deterministic.
void main() {
  const window = Duration(seconds: 60);
  late DateTime now;

  setUp(() => now = DateTime.utc(2026, 10, 4, 12));

  RollingWindowLimiter newLimiter() =>
      RollingWindowLimiter(maxRequests: 3, window: window, now: () => now);

  group('rolling window (SVE-02, SVE-03, SVE-05)', () {
    test('Happy path: three requests are allowed and the fourth is blocked (SVE-02, SVE-03)', () {
      final limiter = newLimiter();

      expect(limiter.isBlocked, isFalse);
      limiter.record();
      expect(limiter.isBlocked, isFalse);
      limiter.record();
      expect(limiter.isBlocked, isFalse);
      limiter.record();
      expect(
        limiter.isBlocked,
        isTrue,
        reason: 'the fourth activation is blocked',
      );
    });

    test('Edge: a request exactly at the window edge frees one slot (SVE-04)', () {
      final limiter = newLimiter();
      limiter.record();
      limiter.record();
      limiter.record();
      expect(limiter.isBlocked, isTrue);

      now = now.add(window);

      expect(limiter.isBlocked, isFalse);
    });

    test('Edge: nextFreeAt returns the oldest record plus the window (SVE-04)', () {
      final limiter = newLimiter();
      final first = now;
      limiter.record();
      now = first.add(const Duration(seconds: 10));
      limiter.record();
      now = first.add(const Duration(seconds: 20));
      limiter.record();

      expect(limiter.nextFreeAt(), first.add(window));
    });

    test('Edge: entries age out as the window rolls, freeing slots in order (SVE-04)', () {
      final limiter = newLimiter();
      final t0 = now;
      limiter.record();
      now = t0.add(const Duration(seconds: 30));
      limiter.record();
      now = t0.add(const Duration(seconds: 50));
      limiter.record();
      expect(limiter.isBlocked, isTrue);
      expect(limiter.nextFreeAt(), t0.add(window));

      now = t0.add(window);
      expect(
        limiter.isBlocked,
        isFalse,
        reason: 'the t0 record aged out exactly at the edge',
      );
      expect(limiter.nextFreeAt(), isNull);

      limiter.record();
      expect(limiter.isBlocked, isTrue);
      expect(limiter.nextFreeAt(), t0.add(const Duration(seconds: 90)));
    });

    test('Edge: two limiters keep independent windows (SVE-05)', () {
      final preview = newLimiter();
      final save = newLimiter();
      preview.record();
      preview.record();
      preview.record();

      expect(preview.isBlocked, isTrue);
      expect(save.isBlocked, isFalse);
      expect(save.nextFreeAt(), isNull);
    });
  });

  group('cooldown (SVE-04)', () {
    testWidgets('Happy path: the cooldown fires once after the delay and never repeats', (
      tester,
    ) async {
      await tester.pumpWidget(const SizedBox.shrink());
      final scheduler = TimerCooldownScheduler();
      var fires = 0;
      scheduler.schedule(const Duration(seconds: 60), () => fires++);

      await tester.pump(const Duration(seconds: 59));
      expect(fires, 0, reason: 'no fire before the delay elapses');
      await tester.pump(const Duration(seconds: 1));
      expect(fires, 1, reason: 'one fire at the delay boundary');
      await tester.pump(const Duration(seconds: 120));
      expect(fires, 1, reason: 'one-shot, it never repeats');
    });

    testWidgets('Happy path: cancel() prevents the fire', (tester) async {
      await tester.pumpWidget(const SizedBox.shrink());
      final scheduler = TimerCooldownScheduler();
      var fires = 0;
      scheduler.schedule(const Duration(seconds: 60), () => fires++);

      scheduler.cancel();
      await tester.pump(const Duration(seconds: 120));

      expect(fires, 0);
    });
  });
}
