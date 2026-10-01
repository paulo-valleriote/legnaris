import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/sheet/sheet_refresh.dart';

/// Unit tests for the F5 dispatch seam (LGN-102: GRN-17 … GRN-19). The
/// registration mirrors the mounted sheet screen, which is the provider's
/// only listener while it is on screen.
void main() {
  ProviderSubscription<String?> keepAliveId(ProviderContainer container) {
    final subscription = container.listen(activeSheetIdProvider, (_, _) {});
    addTearDown(subscription.close);
    return subscription;
  }

  ProviderSubscription<int> keepAliveTick(ProviderContainer container) {
    final subscription = container.listen(sheetRefreshTickProvider, (_, _) {});
    addTearDown(subscription.close);
    return subscription;
  }

  group('active sheet registration (GRN-17 … GRN-19)', () {
    test('Happy path: registering an id sets the active sheet', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      keepAliveId(container);
      expect(container.read(activeSheetIdProvider), isNull);

      container.read(activeSheetIdProvider.notifier).set('fantasy_v1');

      expect(container.read(activeSheetIdProvider), 'fantasy_v1');
    });

    test('Happy path: a second registration replaces the first', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      keepAliveId(container);

      container.read(activeSheetIdProvider.notifier).set('fantasy_v1');
      container.read(activeSheetIdProvider.notifier).set('other_v1');

      expect(container.read(activeSheetIdProvider), 'other_v1');
    });

    test(
      'Edge: once the last listener goes, a fresh read is null again',
      () async {
        final container = ProviderContainer();
        addTearDown(container.dispose);
        final subscription = keepAliveId(container);
        container.read(activeSheetIdProvider.notifier).set('fantasy_v1');

        subscription.close();
        await pumpEventQueue();

        expect(container.read(activeSheetIdProvider), isNull);
      },
    );
  });

  group('refresh tick (GRN-17 … GRN-19)', () {
    test('Happy path: the tick starts at 0 and increments monotonically', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      keepAliveTick(container);

      expect(container.read(sheetRefreshTickProvider), 0);

      container.read(sheetRefreshTickProvider.notifier).bump();
      expect(container.read(sheetRefreshTickProvider), 1);

      container.read(sheetRefreshTickProvider.notifier).bump();
      expect(container.read(sheetRefreshTickProvider), 2);
    });
  });
}
