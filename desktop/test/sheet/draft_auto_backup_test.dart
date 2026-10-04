import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/sheet/draft_auto_backup.dart';
import 'package:legnaris/sheet/draft_backup_scheduler.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';

/// Unit tests for the draft auto-backup coordinator (DRB-01, DRB-02, DRB-03,
/// DRB-13). A fake scheduler and a temp directory keep the tests deterministic.
void main() {
  const sheetId = 'sheet-1';
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}, "constitution": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 12}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "formula": "x + 1", "is_editable": false, "default": 0}}}}
}}}
''';

  late Directory supportDir;
  late _FakeScheduler scheduler;

  setUp(() {
    supportDir = Directory.systemTemp.createTempSync('draft_auto_backup_');
    scheduler = _FakeScheduler();
    addTearDown(() => supportDir.deleteSync(recursive: true));
  });

  Sheet openedSheet() => Sheet(
    id: sheetId,
    templateId: 'fantasy_v1',
    name: 'Aragorn',
    data: const {
      'stats': {'dexterity': 16},
    },
    createdAt: DateTime.utc(2026, 10, 2),
    updatedAt: DateTime.utc(2026, 10, 2),
  );

  ProviderContainer buildContainer({
    DraftBackupStore? store,
    bool autoDispose = true,
  }) {
    final container = ProviderContainer(
      overrides: [
        appSupportDirectoryProvider.overrideWithValue(supportDir),
        draftBackupSchedulerProvider.overrideWithValue(scheduler),
        if (store != null) draftBackupStoreProvider.overrideWithValue(store),
        apiClientProvider.overrideWithValue(
          ApiClient(
            httpClient: MockClient(
              (request) async => http.Response(detailBody, 200),
            ),
          ),
        ),
        openedSheetProvider.overrideWithValue(openedSheet()),
      ],
    );
    if (autoDispose) addTearDown(container.dispose);
    return container;
  }

  DraftBackupStore storeOf(ProviderContainer container) =>
      container.read(draftBackupStoreProvider);

  Future<void> activate(
    ProviderContainer container,
    List<ProviderSubscription<bool>> subscriptions,
  ) async {
    subscriptions.add(
      container.listen(draftAutoBackupProvider(sheetId), (_, _) {}),
    );
    await pumpEventQueue();
  }

  test(
    'Happy path: a pending sheet writes the current delta on every tick (DRB-01, DRB-02)',
    () async {
      final container = buildContainer();
      final subscriptions = <ProviderSubscription<bool>>[];
      addTearDown(() {
        for (final subscription in subscriptions) {
          subscription.close();
        }
      });
      await activate(container, subscriptions);

      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );
      notifier.setPath('stats.dexterity', 18);
      await pumpEventQueue();
      expect(scheduler.running, isTrue);
      expect(scheduler.startCalls, 1);

      scheduler.tick();
      expect(storeOf(container).read(sheetId), {
        'stats': {'dexterity': 18},
      });

      notifier.setPath('stats.constitution', 20);
      await pumpEventQueue();
      scheduler.tick();
      expect(storeOf(container).read(sheetId), {
        'stats': {'dexterity': 18, 'constitution': 20},
      });
    },
  );

  test(
    'Edge: a clean sheet never starts the scheduler and writes nothing (DRB-03)',
    () async {
      final container = buildContainer();
      final subscriptions = <ProviderSubscription<bool>>[];
      addTearDown(() {
        for (final subscription in subscriptions) {
          subscription.close();
        }
      });
      await activate(container, subscriptions);

      expect(scheduler.running, isFalse);
      expect(scheduler.cancelCalls, greaterThanOrEqualTo(1));

      scheduler.tick();
      expect(storeOf(container).read(sheetId), isNull);
    },
  );

  test('Edge: a sheet returning to clean cancels the scheduler (DRB-03)', () async {
    final container = buildContainer();
    final subscriptions = <ProviderSubscription<bool>>[];
    addTearDown(() {
      for (final subscription in subscriptions) {
        subscription.close();
      }
    });
    await activate(container, subscriptions);
    final notifier = container.read(
      sheetEditControllerProvider(sheetId).notifier,
    );

    notifier.setPath('stats.dexterity', 18);
    await pumpEventQueue();
    expect(scheduler.running, isTrue);

    notifier.setPath('stats.dexterity', 16);
    await pumpEventQueue();
    expect(scheduler.running, isFalse);
  });

  test('Edge: exactly one scheduler start per dirty transition (DRB-02)', () async {
    final container = buildContainer();
    final subscriptions = <ProviderSubscription<bool>>[];
    addTearDown(() {
      for (final subscription in subscriptions) {
        subscription.close();
      }
    });
    await activate(container, subscriptions);
    final notifier = container.read(
      sheetEditControllerProvider(sheetId).notifier,
    );

    notifier.setPath('stats.dexterity', 18);
    await pumpEventQueue();
    notifier.setPath('stats.constitution', 20);
    await pumpEventQueue();
    notifier.setPath('stats.constitution', 21);
    await pumpEventQueue();

    expect(scheduler.startCalls, 1);
    expect(scheduler.running, isTrue);
  });

  test('Edge: a write failure is swallowed and editing continues (DRB-13)', () async {
    final container = buildContainer(
      store: _ThrowingStore(directory: supportDir),
    );
    final subscriptions = <ProviderSubscription<bool>>[];
    addTearDown(() {
      for (final subscription in subscriptions) {
        subscription.close();
      }
    });
    await activate(container, subscriptions);
    final notifier = container.read(
      sheetEditControllerProvider(sheetId).notifier,
    );
    notifier.setPath('stats.dexterity', 18);
    await pumpEventQueue();

    expect(scheduler.tick, returnsNormally);

    notifier.setPath('stats.constitution', 20);
    await pumpEventQueue();
    expect(
      container.read(sheetEditControllerProvider(sheetId)).isPending,
      isTrue,
    );
  });

  test('Edge: disposing the provider cancels the scheduler (DRB-01)', () async {
    final container = buildContainer(autoDispose: false);
    container.listen(draftAutoBackupProvider(sheetId), (_, _) {});
    await pumpEventQueue();
    container
        .read(sheetEditControllerProvider(sheetId).notifier)
        .setPath('stats.dexterity', 18);
    await pumpEventQueue();
    expect(scheduler.running, isTrue);

    container.dispose();

    expect(scheduler.running, isFalse);
    expect(scheduler.cancelCalls, greaterThanOrEqualTo(1));
  });
}

/// Test double for the periodic clock: it records start/cancel calls and lets
/// the test fire the registered tick manually.
class _FakeScheduler implements DraftBackupScheduler {
  int startCalls = 0;
  int cancelCalls = 0;
  bool running = false;
  void Function()? _onTick;

  @override
  void start(void Function() onTick) {
    startCalls++;
    _onTick = onTick;
    running = true;
  }

  @override
  void cancel() {
    cancelCalls++;
    running = false;
    _onTick = null;
  }

  void tick() => _onTick?.call();
}

/// A store whose writes always fail, proving the coordinator swallows them.
class _ThrowingStore extends DraftBackupStore {
  _ThrowingStore({required super.directory});

  @override
  void write(String sheetId, Map<String, Object?> delta) {
    throw const DraftBackupException('disk full');
  }
}
