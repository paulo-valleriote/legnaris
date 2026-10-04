import 'package:flutter/widgets.dart' show SizedBox;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/sheet/draft_backup_scheduler.dart';

/// Unit tests for the periodic draft backup clock (DRB-01). The widget
/// tester's fake clock drives the real timer, so no test waits 60 seconds.
void main() {
  const tickInterval = Duration(milliseconds: 20);

  test('Happy path: the default interval is the 60s spec constant (DRB-01)', () {
    expect(
      PeriodicDraftBackupScheduler.defaultInterval,
      const Duration(seconds: 60),
    );
    expect(
      PeriodicDraftBackupScheduler().interval,
      PeriodicDraftBackupScheduler.defaultInterval,
    );
  });

  group('periodic ticking (DRB-01)', () {
    testWidgets('Happy path: a tick fires once per interval and never before it', (
      tester,
    ) async {
      await tester.pumpWidget(const SizedBox.shrink());
      final scheduler = PeriodicDraftBackupScheduler(interval: tickInterval);
      var ticks = 0;
      scheduler.start(() => ticks++);

      await tester.pump(const Duration(milliseconds: 19));
      expect(ticks, 0, reason: 'no tick before the first interval elapses');
      await tester.pump(const Duration(milliseconds: 1));
      expect(ticks, 1, reason: 'one tick at the interval boundary');
      await tester.pump(const Duration(milliseconds: 20));
      expect(ticks, 2, reason: 'one more tick per following interval');

      scheduler.cancel();
    });

    testWidgets('Happy path: cancel() stops the ticks and is idempotent', (
      tester,
    ) async {
      await tester.pumpWidget(const SizedBox.shrink());
      final scheduler = PeriodicDraftBackupScheduler(interval: tickInterval);
      var ticks = 0;
      scheduler.start(() => ticks++);
      await tester.pump(const Duration(milliseconds: 20));
      expect(ticks, 1);

      scheduler.cancel();
      scheduler.cancel();
      await tester.pump(const Duration(milliseconds: 100));
      expect(ticks, 1, reason: 'no tick after cancel');
    });

    testWidgets('Edge: a second start() never doubles the ticks', (tester) async {
      await tester.pumpWidget(const SizedBox.shrink());
      final scheduler = PeriodicDraftBackupScheduler(interval: tickInterval);
      var ticks = 0;
      scheduler.start(() => ticks++);
      scheduler.start(() => ticks++);

      await tester.pump(const Duration(milliseconds: 20));
      expect(ticks, 1, reason: 'one timer only; a doubled start would tick twice');
      await tester.pump(const Duration(milliseconds: 20));
      expect(ticks, 2, reason: 'the single timer keeps its cadence');

      scheduler.cancel();
      await tester.pump(const Duration(milliseconds: 60));
      expect(ticks, 2, reason: 'no leaked timer keeps ticking after cancel');
    });
  });

  testWidgets('Edge: disposing the provider cancels the running timer', (
    tester,
  ) async {
    await tester.pumpWidget(const SizedBox.shrink());
    final container = ProviderContainer();
    final scheduler = container.read(draftBackupSchedulerProvider);
    var ticks = 0;
    scheduler.start(() => ticks++);
    await tester.pump(const Duration(seconds: 60));
    expect(ticks, 1);

    container.dispose();
    await tester.pump(const Duration(seconds: 60));
    expect(ticks, 1, reason: 'the provider disposal cancelled the timer');
  });
}
