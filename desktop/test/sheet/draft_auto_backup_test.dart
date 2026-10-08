import 'dart:convert';
import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/auth_api_client.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/auth/auth_session.dart';
import 'package:legnaris/auth/secure_token_store.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/sheet/draft_auto_backup.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';

import '../auth/fakes.dart';

/// Tests for the draft auto-backup coordinator (DRB-01, DRB-02, DRB-03,
/// DRB-05, DRB-13). Real temp-directory I/O keeps the writes verifiable, and
/// the widget tester's fake clock drives the 60-second scheduler without
/// waiting a real minute.
void main() {
  const sheetId = '12345678-1234-7000-8000-123456789abc';
  const pairBody =
      '{"data": {"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 900}}';
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}, "constitution": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 12}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "formula": "x + 1", "is_editable": false, "default": 0}}}}
}}}
''';

  late Directory supportDir;

  setUp(() {
    supportDir = Directory.systemTemp.createTempSync('draft_auto_backup_');
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
    bool authenticated = true,
  }) {
    final container = ProviderContainer(
      overrides: [
        appSupportDirectoryProvider.overrideWithValue(supportDir),
        if (store != null) draftBackupStoreProvider.overrideWithValue(store),
        apiClientProvider.overrideWithValue(
          ApiClient(
            httpClient: MockClient(
              (request) async => http.Response(detailBody, 200),
            ),
          ),
        ),
        openedSheetProvider.overrideWithValue(openedSheet()),
        // The writer guard skips writes outside a live session (DATA-01), so
        // the suite runs authenticated; `authenticated: false` hands back the
        // real session for the session-end test to drive through login.
        secureTokenStoreProvider.overrideWithValue(InMemorySecureTokenStore()),
        authApiClientProvider.overrideWithValue(
          AuthApiClient(
            httpClient: MockClient(
              (request) async => http.Response(pairBody, 200),
            ),
          ),
        ),
        if (authenticated)
          authSessionProvider.overrideWithValue(
            const AuthAuthenticated('access-test'),
          ),
      ],
    );
    if (autoDispose) addTearDown(container.dispose);
    return container;
  }

  File draftFile() => File('${supportDir.path}/drafts/$sheetId.json');

  Map<String, Object?> draftDelta() =>
      jsonDecode(draftFile().readAsStringSync())['delta']
          as Map<String, Object?>;

  Future<void> activate(
    WidgetTester tester,
    ProviderContainer container,
    List<ProviderSubscription<bool>> subscriptions,
  ) async {
    subscriptions.add(
      container.listen(draftAutoBackupProvider(sheetId), (_, _) {}),
    );
    await tester.pump(Duration.zero);
    await tester.pump(Duration.zero);
  }

  /// Tears the container down inside the test body: the framework checks for
  /// pending timers before `addTearDown` callbacks run, so a still-running
  /// scheduler would fail the test instead of being silently cancelled.
  void release(
    ProviderContainer container,
    List<ProviderSubscription<bool>> subscriptions,
  ) {
    for (final subscription in subscriptions) {
      subscription.close();
    }
    container.dispose();
  }

  testWidgets(
    'Happy path: a pending sheet writes the current delta on every tick (DRB-01, DRB-02)',
    (tester) async {
      final container = buildContainer(autoDispose: false);
      final subscriptions = <ProviderSubscription<bool>>[];
      await activate(tester, container, subscriptions);
      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );

      notifier.setPath('stats.dexterity', 18);
      await tester.pump(Duration.zero);
      expect(
        container.read(sheetEditControllerProvider(sheetId)).isPending,
        isTrue,
      );
      expect(
        draftFile().existsSync(),
        isFalse,
        reason: 'nothing is written before the first tick',
      );

      await tester.pump(const Duration(seconds: 60));
      expect(draftFile().existsSync(), isTrue, reason: 'the first tick writes');
      expect(draftDelta(), {
        'stats': {'dexterity': 18},
      });

      notifier.setPath('stats.constitution', 20);
      await tester.pump(const Duration(seconds: 60));
      expect(draftDelta(), {
        'stats': {'dexterity': 18, 'constitution': 20},
      }, reason: 'the next periodic tick carries the newer delta (DRB-05)');

      release(container, subscriptions);
    },
  );

  testWidgets(
    'Edge: a clean sheet never starts the scheduler and writes nothing (DRB-03)',
    (tester) async {
      final container = buildContainer();
      final subscriptions = <ProviderSubscription<bool>>[];
      addTearDown(() {
        for (final subscription in subscriptions) {
          subscription.close();
        }
      });
      await activate(tester, container, subscriptions);

      // Two minutes of fake time pass; a scheduler started by mistake would
      // leave a pending periodic timer, which fails the run's final check.
      await tester.pump(const Duration(seconds: 120));
      expect(draftFile().existsSync(), isFalse);
    },
  );

  testWidgets(
    'Edge: a sheet returning to clean cancels the scheduler (DRB-03)',
    (tester) async {
      final container = buildContainer(autoDispose: false);
      final subscriptions = <ProviderSubscription<bool>>[];
      await activate(tester, container, subscriptions);
      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );

      notifier.setPath('stats.dexterity', 18);
      await tester.pump(Duration.zero);
      expect(
        container.read(sheetEditControllerProvider(sheetId)).isPending,
        isTrue,
      );

      await tester.pump(const Duration(seconds: 30));
      notifier.setPath('stats.dexterity', 16);
      await tester.pump(Duration.zero);
      expect(
        container.read(sheetEditControllerProvider(sheetId)).isPending,
        isFalse,
      );

      // The container is deliberately not disposed here: the pending-timer
      // check below proves the clean transition itself cancelled the clock.
      await tester.pump(const Duration(seconds: 60));
      expect(draftFile().existsSync(), isFalse);
    },
  );

  testWidgets(
    'Edge: exactly one scheduler start per dirty transition (DRB-02)',
    (tester) async {
      final container = buildContainer(autoDispose: false);
      final subscriptions = <ProviderSubscription<bool>>[];
      await activate(tester, container, subscriptions);
      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );

      notifier.setPath('stats.dexterity', 18);
      await tester.pump(Duration.zero);
      expect(
        container.read(sheetEditControllerProvider(sheetId)).isPending,
        isTrue,
      );

      // A second edit inside the same dirty stretch must not restart the
      // clock: the tick still lands 60 s after the first edit.
      await tester.pump(const Duration(seconds: 30));
      notifier.setPath('stats.constitution', 20);
      await tester.pump(const Duration(seconds: 29));
      expect(
        draftFile().existsSync(),
        isFalse,
        reason: 'the second edit did not restart the timer',
      );
      await tester.pump(const Duration(seconds: 1));
      expect(
        draftFile().existsSync(),
        isTrue,
        reason: 'the tick lands exactly 60 s after the first edit',
      );

      release(container, subscriptions);
    },
  );

  testWidgets(
    'Edge: a write failure is swallowed and editing continues (DRB-13)',
    (tester) async {
      final container = buildContainer(
        store: _ThrowingStore(directory: supportDir),
        autoDispose: false,
      );
      final subscriptions = <ProviderSubscription<bool>>[];
      await activate(tester, container, subscriptions);
      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );

      notifier.setPath('stats.dexterity', 18);
      await tester.pump(Duration.zero);
      expect(
        container.read(sheetEditControllerProvider(sheetId)).isPending,
        isTrue,
      );

      // The tick fires and the store throws; an uncaught error would fail
      // this test, so surviving it is the assertion.
      await tester.pump(const Duration(seconds: 60));
      expect(
        container.read(sheetEditControllerProvider(sheetId)).isPending,
        isTrue,
      );

      notifier.setPath('stats.constitution', 20);
      await tester.pump(Duration.zero);
      expect(
        container.read(sheetEditControllerProvider(sheetId)).isPending,
        isTrue,
        reason: 'editing keeps working after the failed write',
      );

      release(container, subscriptions);
    },
  );

  testWidgets('Edge: disposing the provider cancels the scheduler (DRB-01)', (
    tester,
  ) async {
    final container = buildContainer(autoDispose: false);
    container.listen(draftAutoBackupProvider(sheetId), (_, _) {});
    await tester.pump(Duration.zero);
    container
        .read(sheetEditControllerProvider(sheetId).notifier)
        .setPath('stats.dexterity', 18);
    await tester.pump(Duration.zero);
    expect(
      container.read(sheetEditControllerProvider(sheetId)).isPending,
      isTrue,
    );

    container.dispose();

    await tester.pump(const Duration(seconds: 120));
    expect(
      draftFile().existsSync(),
      isFalse,
      reason: 'no write survives the disposal',
    );
  });

  testWidgets(
    'Error: a scheduler tick after the session ends writes no draft file (DATA-01)',
    (tester) async {
      final container = buildContainer(
        autoDispose: false,
        authenticated: false,
      );
      final subscriptions = <ProviderSubscription<bool>>[];
      await container.read(authSessionProvider.notifier).login(
        email: 'player@example.com',
        password: 'sup3r-secret',
      );
      await activate(tester, container, subscriptions);
      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );
      notifier.setPath('stats.dexterity', 18);
      await tester.pump(Duration.zero);
      expect(
        container.read(sheetEditControllerProvider(sheetId)).isPending,
        isTrue,
      );

      await tester.pump(const Duration(seconds: 60));
      expect(
        draftFile().existsSync(),
        isTrue,
        reason: 'while authenticated the tick writes the delta',
      );

      await container.read(authSessionProvider.notifier).onUnauthorized();
      expect(
        draftFile().existsSync(),
        isFalse,
        reason: 'the session-end sweep deletes the backup',
      );

      // The scheduler is still armed and pending edits survive, so a tick
      // landing now would rewrite the swept file without the writer guard.
      await tester.pump(const Duration(seconds: 60));
      expect(
        draftFile().existsSync(),
        isFalse,
        reason: 'DATA-01: a dead session never re-writes the swept backup',
      );

      release(container, subscriptions);
    },
  );
}

/// A store whose writes always fail, proving the coordinator swallows them.
class _ThrowingStore extends DraftBackupStore {
  _ThrowingStore({required super.directory});

  @override
  void write(String sheetId, Map<String, Object?> delta) {
    throw const DraftBackupException('disk full');
  }
}
