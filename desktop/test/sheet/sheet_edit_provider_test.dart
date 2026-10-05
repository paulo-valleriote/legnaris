import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/library/sheet_library_provider.dart';
import 'package:legnaris/library/sheet_library_store.dart';
import 'package:legnaris/sheet/action_rate_limiter.dart';
import 'package:legnaris/sheet/draft_backup_store.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';

import '../library/library_fixtures.dart';

/// Unit tests for the in-memory sheet edit state (LGN-102: GRN-01 … GRN-06,
/// LGN-108: OPN-10, OPN-11). Every asserted value comes from the spec rule,
/// never from the implementation.
void main() {
  const sheetId = '12345678-1234-7000-8000-123456789abc';
  late Directory supportDir;
  late DateTime now;
  late _FakeCooldownScheduler cooldown;

  setUp(() {
    supportDir = Directory.systemTemp.createTempSync('sheet_edit_');
    now = DateTime.utc(2026, 10, 4, 12);
    cooldown = _FakeCooldownScheduler();
    addTearDown(() => supportDir.deleteSync(recursive: true));
  });
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}, "constitution": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 12}}}},
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 1}}, "acUsesCon": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 0}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "formula": "x + 1", "is_editable": false, "default": 0}}}}
}}}
''';
  const otherBody = '''
{"data": {"id": "other_v1", "name": "Other Template", "default": false, "fields": {
  "stats": {"children": {"strength": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 8}}}}
}}}
''';

  String sheetBody(
    String id, {
    String templateId = 'fantasy_v1',
    Map<String, Object?> data = const {},
  }) => jsonEncode({
    'data': {
      'id': id,
      'template_id': templateId,
      'name': 'Sheet $id',
      'data': data,
      'created_at': '2026-09-20T12:00:00Z',
      'updated_at': '2026-09-20T12:00:00Z',
    },
  });

  Sheet openedSheet() => Sheet(
    id: sheetId,
    templateId: 'fantasy_v1',
    name: 'Aragorn',
    data: const {
      'stats': {'dexterity': 16},
    },
    createdAt: DateTime.utc(2026, 10, 2, 15, 31, 28),
    updatedAt: DateTime.utc(2026, 10, 2, 15, 31, 28),
  );

  /// A container with no injected sheet: the edit state fetches the sheet by
  /// id and then its template detail.
  ProviderContainer containerFetching(
    Future<http.Response> Function(http.Request request) handler,
    List<http.Request> recorded,
  ) {
    final container = ProviderContainer(
      overrides: [
        appSupportDirectoryProvider.overrideWithValue(supportDir),
        clockProvider.overrideWithValue(() => now),
        cooldownSchedulerProvider.overrideWithValue(cooldown),
        apiClientProvider.overrideWithValue(
          ApiClient(
            httpClient: MockClient((request) async {
              recorded.add(request);
              return handler(request);
            }),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  /// A container whose open sheet is injected, so only the template detail is
  /// requested (the creation-flow path, OPN-09).
  ProviderContainer containerWithOpened(
    Sheet opened, {
    Future<http.Response> Function(http.Request request)? handler,
    List<http.Request>? recorded,
  }) {
    final container = ProviderContainer(
      overrides: [
        appSupportDirectoryProvider.overrideWithValue(supportDir),
        clockProvider.overrideWithValue(() => now),
        cooldownSchedulerProvider.overrideWithValue(cooldown),
        apiClientProvider.overrideWithValue(
          ApiClient(
            httpClient: MockClient((request) async {
              recorded?.add(request);
              return handler?.call(request) ?? http.Response(detailBody, 200);
            }),
          ),
        ),
        openedSheetProvider.overrideWithValue(opened),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  /// Listens to keep the autoDispose edit state alive for one test.
  ProviderSubscription<SheetEditState> keepAlive(
    ProviderContainer container,
    String id,
  ) {
    final subscription = container.listen(
      sheetEditControllerProvider(id),
      (_, _) {},
    );
    addTearDown(subscription.close);
    return subscription;
  }

  SheetEditState stateOf(ProviderContainer container, String id) =>
      container.read(sheetEditControllerProvider(id));

  group('opened-sheet seeding (OPN-10, OPN-11)', () {
    test('Edge: openedSheetProvider defaults to null when not overridden', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      expect(container.read(openedSheetProvider), isNull);
    });

    test(
      'Happy path: an injected sheet seeds the baseline and data from its stored values (OPN-10, OPN-11)',
      () async {
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);

        await pumpEventQueue();

        final seeded = stateOf(container, sheetId);
        expect(valueAtPath(seeded.data, 'stats.dexterity'), 16);
        expect(valueAtPath(seeded.data, 'stats.constitution'), 12.0);
        expect(valueAtPath(seeded.defaults, 'stats.dexterity'), 16);
        expect(seeded.isPending, isFalse);
      },
    );

    test(
      'Edge: the baseline is the opened value, so returning to it clears pending (OPN-11)',
      () async {
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );

        notifier.setPath('stats.dexterity', 18);
        expect(stateOf(container, sheetId).isPending, isTrue);

        notifier.setPath('stats.dexterity', 16);
        expect(stateOf(container, sheetId).isPending, isFalse);
      },
    );
  });

  group('seeding from the fetched sheet (OPN-10)', () {
    test(
      'Happy path: fetches the sheet then its template, overlaying stored values on the schema defaults (OPN-10)',
      () async {
        final recorded = <http.Request>[];
        final container = containerFetching(
          (request) async => request.url.path.startsWith('/sheets/')
              ? http.Response(
                  sheetBody(
                    sheetId,
                    data: const {
                      'stats': {'dexterity': 16},
                    },
                  ),
                  200,
                )
              : http.Response(detailBody, 200),
          recorded,
        );

        keepAlive(container, sheetId);
        expect(stateOf(container, sheetId).data, isEmpty);

        await pumpEventQueue();
        final seeded = stateOf(container, sheetId);
        expect(valueAtPath(seeded.data, 'stats.dexterity'), 16);
        expect(
          valueAtPath(seeded.data, 'stats.constitution'),
          12.0,
          reason: 'a key missing from the sheet keeps the schema default',
        );
        expect(valueAtPath(seeded.data, 'config.acUsesDex'), 1.0);
        expect(valueAtPath(seeded.data, 'derived.armorClass'), 0.0);
        expect(seeded.isPending, isFalse);
        expect(recorded, hasLength(2));
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
      },
    );
  });

  group('editing (GRN-02, GRN-03, GRN-04)', () {
    test(
      'Happy path: setPath stores at the dotted path without mutating the previous state (GRN-02)',
      () async {
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final before = stateOf(container, sheetId);

        container
            .read(sheetEditControllerProvider(sheetId).notifier)
            .setPath('stats.dexterity', 20);

        final after = stateOf(container, sheetId);
        expect(valueAtPath(after.data, 'stats.dexterity'), 20);
        expect(valueAtPath(before.data, 'stats.dexterity'), 16);
        expect(identical(after.data, before.data), isFalse);
        expect(after.editedPaths, contains('stats.dexterity'));
      },
    );

    test(
      'Happy path: seeding and editing issue no request beyond the detail GET (GRN-03, GRN-04)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();

        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 20);
        notifier.setPath('config.acUsesDex', 0);
        notifier.setPath('derived.armorClass', 99);
        await pumpEventQueue();

        expect(recorded, hasLength(1));
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
        expect(
          valueAtPath(stateOf(container, sheetId).data, 'derived.armorClass'),
          99,
        );
      },
    );

    test(
      'Edge: the pending flag flips only while the state differs from the seeded defaults',
      () async {
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );

        expect(stateOf(container, sheetId).isPending, isFalse);

        notifier.setPath('stats.dexterity', 20);
        expect(stateOf(container, sheetId).isPending, isTrue);

        notifier.setPath('stats.dexterity', 16);
        expect(stateOf(container, sheetId).isPending, isFalse);
      },
    );
  });

  group('lifetime (GRN-06)', () {
    test(
      'Edge: different sheet ids hold independent edit state (family isolation)',
      () async {
        final recorded = <http.Request>[];
        final container = containerFetching((request) async {
          final path = request.url.path;
          if (path.startsWith('/sheets/')) {
            final id = path.substring('/sheets/'.length);
            return http.Response(
              sheetBody(id, templateId: id == '87654321-4321-7000-8000-987654321cba' ? 'other_v1' : 'fantasy_v1'),
              200,
            );
          }
          return http.Response(
            path.endsWith('other_v1') ? otherBody : detailBody,
            200,
          );
        }, recorded);

        keepAlive(container, sheetId);
        keepAlive(container, '87654321-4321-7000-8000-987654321cba');
        await pumpEventQueue();

        container
            .read(sheetEditControllerProvider(sheetId).notifier)
            .setPath('stats.dexterity', 20);

        expect(
          valueAtPath(stateOf(container, sheetId).data, 'stats.dexterity'),
          20,
        );
        expect(
          valueAtPath(
            stateOf(container, '87654321-4321-7000-8000-987654321cba').data,
            'stats.strength',
          ),
          8.0,
        );
        expect(stateOf(container, '87654321-4321-7000-8000-987654321cba').isPending, isFalse);
      },
    );

    test(
      'Edge: a detail reload discards edits as it starts and reseeds from the fresh schema (GRN-06)',
      () async {
        final gate = Completer<http.Response>();
        var calls = 0;
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: (request) async =>
              calls++ == 0 ? http.Response(detailBody, 200) : gate.future,
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        container
            .read(sheetEditControllerProvider(sheetId).notifier)
            .setPath('stats.dexterity', 20);
        expect(stateOf(container, sheetId).isPending, isTrue);

        final loadFuture = container
            .read(templateDetailControllerProvider('fantasy_v1').notifier)
            .load();
        await pumpEventQueue();

        final duringReload = stateOf(container, sheetId);
        expect(duringReload.data, isEmpty);
        expect(duringReload.isPending, isFalse);

        gate.complete(http.Response(detailBody, 200));
        await loadFuture;
        await pumpEventQueue();

        final reseeded = stateOf(container, sheetId);
        expect(
          valueAtPath(reseeded.data, 'stats.dexterity'),
          16,
          reason: 'the reload reseeds from the stored sheet, not the schema',
        );
        expect(reseeded.isPending, isFalse);
        expect(recorded, hasLength(2));
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
      },
    );

    test(
      'Edge: disposing the edit state discards it — a fresh subscription reseeds (GRN-06)',
      () async {
        final container = containerWithOpened(openedSheet());
        final first = keepAlive(container, sheetId);
        await pumpEventQueue();
        container
            .read(sheetEditControllerProvider(sheetId).notifier)
            .setPath('stats.dexterity', 20);

        first.close();
        await pumpEventQueue();

        keepAlive(container, sheetId);
        await pumpEventQueue();
        final fresh = stateOf(container, sheetId);
        expect(valueAtPath(fresh.data, 'stats.dexterity'), 16);
        expect(fresh.isPending, isFalse);
      },
    );
  });

  group('exclusive selection (EXC-13, EXC-14, EXC-16)', () {
    test(
      'Happy path: setExclusive sets the selected member to 1 and every sibling to 0 in one emission (EXC-13)',
      () async {
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final before = stateOf(container, sheetId);
        final emissions = <SheetEditState>[];
        final subscription = container.listen(
          sheetEditControllerProvider(sheetId),
          (_, next) => emissions.add(next),
        );
        addTearDown(subscription.close);

        container
            .read(sheetEditControllerProvider(sheetId).notifier)
            .setExclusive('config.acUsesCon', const ['config.acUsesDex']);

        final after = stateOf(container, sheetId);
        expect(valueAtPath(after.data, 'config.acUsesCon'), 1);
        expect(valueAtPath(after.data, 'config.acUsesDex'), 0);
        expect(
          after.editedPaths,
          containsAll(['config.acUsesCon', 'config.acUsesDex']),
        );
        expect(
          emissions,
          hasLength(1),
          reason: 'the whole group must change in a single emission',
        );
        expect(identical(after, before), isFalse);
      },
    );

    test(
      'Edge: selecting the already-active option leaves the state instance identical and emits nothing (EXC-14)',
      () async {
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final before = stateOf(container, sheetId);
        var emissions = 0;
        final subscription = container.listen(
          sheetEditControllerProvider(sheetId),
          (_, _) => emissions++,
        );
        addTearDown(subscription.close);

        container
            .read(sheetEditControllerProvider(sheetId).notifier)
            .setExclusive('config.acUsesDex', const ['config.acUsesCon']);

        expect(identical(stateOf(container, sheetId), before), isTrue);
        expect(emissions, 0);
      },
    );

    test(
      'Edge: a selected member already on still zeroes a lingering sibling (EXC-13)',
      () async {
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('config.acUsesCon', 1);
        final before = stateOf(container, sheetId);

        notifier.setExclusive('config.acUsesDex', const ['config.acUsesCon']);

        final after = stateOf(container, sheetId);
        expect(valueAtPath(after.data, 'config.acUsesDex'), 1);
        expect(valueAtPath(after.data, 'config.acUsesCon'), 0);
        expect(identical(after, before), isFalse);
      },
    );

    test(
      'Happy path: setExclusive(null) clears every member in one emission (EXC-13, EXC-27)',
      () async {
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final before = stateOf(container, sheetId);
        final emissions = <SheetEditState>[];
        final subscription = container.listen(
          sheetEditControllerProvider(sheetId),
          (_, next) => emissions.add(next),
        );
        addTearDown(subscription.close);

        container
            .read(sheetEditControllerProvider(sheetId).notifier)
            .setExclusive(null, const ['config.acUsesDex', 'config.acUsesCon']);

        final after = stateOf(container, sheetId);
        expect(valueAtPath(after.data, 'config.acUsesDex'), 0);
        expect(valueAtPath(after.data, 'config.acUsesCon'), 0);
        expect(
          after.editedPaths,
          containsAll(['config.acUsesDex', 'config.acUsesCon']),
        );
        expect(
          emissions,
          hasLength(1),
          reason: 'the whole group must clear in a single emission',
        );
        expect(identical(after, before), isFalse);
      },
    );

    test(
      'Edge: clearing an already-empty group leaves the state instance identical (EXC-14)',
      () async {
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setExclusive(null, const ['config.acUsesDex', 'config.acUsesCon']);
        final cleared = stateOf(container, sheetId);
        var emissions = 0;
        final subscription = container.listen(
          sheetEditControllerProvider(sheetId),
          (_, _) => emissions++,
        );
        addTearDown(subscription.close);

        notifier.setExclusive(null, const ['config.acUsesDex', 'config.acUsesCon']);

        expect(identical(stateOf(container, sheetId), cleared), isTrue);
        expect(emissions, 0);
      },
    );
  });

  group('save (SAV-08 … SAV-23)', () {
    const cascadeBody =
        '{"data": {"derived": {"armorClass": 13}, "modifiers": {"dexterityMod": 3}}}';
    const emptyCascadeBody = '{"data": {}}';
    const invalidBody =
        '{"error": {"code": "INVALID_REQUEST", "message": "not editable"}}';

    Future<http.Response> Function(http.Request) responder(
      List<http.Request> recorded, {
      Future<http.Response> Function(http.Request)? onPatch,
    }) => (request) async {
      if (request.method == 'PATCH') {
        return onPatch != null
            ? onPatch(request)
            : http.Response(cascadeBody, 200);
      }
      return http.Response(detailBody, 200);
    };

    List<http.Request> patches(List<http.Request> recorded) =>
        recorded.where((request) => request.method == 'PATCH').toList();

    test(
      'Happy path: one PATCH carries the changed editable delta and the cascade merges (SAV-08, SAV-10, SAV-13, SAV-14, SAV-17)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: responder(recorded),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);

        await notifier.save();
        await pumpEventQueue();

        expect(patches(recorded), hasLength(1));
        expect(jsonDecode(patches(recorded).single.body), {
          'data': {
            'stats': {'dexterity': 18},
          },
        });
        final after = stateOf(container, sheetId);
        expect(valueAtPath(after.data, 'stats.dexterity'), 18);
        expect(valueAtPath(after.data, 'derived.armorClass'), 13);
        expect(valueAtPath(after.data, 'modifiers.dexterityMod'), 3);
        expect(after.isPending, isFalse);
        expect(after.isSaving, isFalse);
        expect(after.saveError, isNull);
      },
    );

    test(
      'Edge: a field reverted to the baseline leaves the delta and omits the derived leaf (SAV-09, SAV-10)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: responder(recorded),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);
        notifier.setPath('stats.constitution', 20);
        notifier.setPath('stats.constitution', 12);

        await notifier.save();
        await pumpEventQueue();

        final body = jsonDecode(patches(recorded).single.body) as Map;
        final delta = body['data'] as Map;
        expect(delta, {
          'stats': {'dexterity': 18},
        });
        expect(delta.containsKey('derived'), isFalse);
      },
    );

    test(
      'Edge: an exclusive selection ships the affected members so at most one is non-zero (SAV-11)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: responder(recorded),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();

        container
            .read(sheetEditControllerProvider(sheetId).notifier)
            .setExclusive('config.acUsesCon', const ['config.acUsesDex']);
        await container.read(sheetEditControllerProvider(sheetId).notifier).save();
        await pumpEventQueue();

        expect(jsonDecode(patches(recorded).single.body), {
          'data': {
            'config': {'acUsesDex': 0, 'acUsesCon': 1},
          },
        });
      },
    );

    test('Edge: no change issues no PATCH (SAV-12)', () async {
      final recorded = <http.Request>[];
      final container = containerWithOpened(
        openedSheet(),
        recorded: recorded,
        handler: responder(recorded),
      );
      keepAlive(container, sheetId);
      await pumpEventQueue();

      await container.read(sheetEditControllerProvider(sheetId).notifier).save();
      await pumpEventQueue();

      expect(patches(recorded), isEmpty);
      expect(stateOf(container, sheetId).isSaving, isFalse);
    });

    test('Edge: an empty cascade updates the baseline and clears pending (SAV-16)', () async {
      final recorded = <http.Request>[];
      final container = containerWithOpened(
        openedSheet(),
        recorded: recorded,
        handler: responder(
          recorded,
          onPatch: (_) async => http.Response(emptyCascadeBody, 200),
        ),
      );
      keepAlive(container, sheetId);
      await pumpEventQueue();
      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );
      notifier.setPath('stats.dexterity', 18);
      final before = stateOf(container, sheetId).data;

      await notifier.save();
      await pumpEventQueue();

      final after = stateOf(container, sheetId);
      expect(after.isPending, isFalse);
      expect(valueAtPath(after.data, 'stats.dexterity'), 18);
      expect(valueAtPath(after.data, 'stats.constitution'), 12.0);
      expect(
        valueAtPath(before, 'derived.armorClass'),
        valueAtPath(after.data, 'derived.armorClass'),
      );
    });

    test(
      'Error: a failure keeps the edits and publishes the mapped copy, and a retry saves once more (SAV-18, SAV-19, SAV-20, SAV-21, SAV-22, SAV-23)',
      () async {
        var failing = true;
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: responder(
            recorded,
            onPatch: (_) async =>
                failing ? http.Response(invalidBody, 400) : http.Response(cascadeBody, 200),
          ),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);

        await notifier.save();
        await pumpEventQueue();

        final failed = stateOf(container, sheetId);
        expect(failed.isPending, isTrue);
        expect(valueAtPath(failed.data, 'stats.dexterity'), 18);
        expect(valueAtPath(failed.data, 'derived.armorClass'), 0.0);
        expect(failed.saveError, errorCopy(ApiErrorKind.invalidEdit));
        expect(patches(recorded), hasLength(1));

        await pumpEventQueue();
        expect(patches(recorded), hasLength(1), reason: 'PATCH is never auto-retried');

        failing = false;
        await notifier.save();
        await pumpEventQueue();

        final saved = stateOf(container, sheetId);
        expect(patches(recorded), hasLength(2));
        expect(saved.isPending, isFalse);
        expect(saved.saveError, isNull);
        expect(valueAtPath(saved.data, 'derived.armorClass'), 13);
      },
    );

    test('Edge: two rapid saves issue exactly one PATCH (single-flight)', () async {
      final gate = Completer<http.Response>();
      final recorded = <http.Request>[];
      final container = containerWithOpened(
        openedSheet(),
        recorded: recorded,
        handler: responder(recorded, onPatch: (_) => gate.future),
      );
      keepAlive(container, sheetId);
      await pumpEventQueue();
      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );
      notifier.setPath('stats.dexterity', 18);

      final first = notifier.save();
      final second = notifier.save();
      await pumpEventQueue();

      expect(patches(recorded), hasLength(1));
      expect(stateOf(container, sheetId).isSaving, isTrue);

      gate.complete(http.Response(cascadeBody, 200));
      await first;
      await second;
      await pumpEventQueue();

      expect(stateOf(container, sheetId).isSaving, isFalse);
      expect(patches(recorded), hasLength(1));
    });

    test(
      'Edge: an edit made while the save is in flight stays pending after the success (SAV-14)',
      () async {
        final gate = Completer<http.Response>();
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: responder(recorded, onPatch: (_) => gate.future),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);

        final saving = notifier.save();
        await pumpEventQueue();
        notifier.setPath('stats.constitution', 20);

        gate.complete(http.Response(cascadeBody, 200));
        await saving;
        await pumpEventQueue();

        final after = stateOf(container, sheetId);
        expect(after.isPending, isTrue);
        expect(after.editedPaths, contains('stats.constitution'));
        expect(valueAtPath(after.data, 'stats.constitution'), 20);
        expect(valueAtPath(after.defaults, 'stats.constitution'), 12.0);
        expect(valueAtPath(after.data, 'stats.dexterity'), 18);
        expect(valueAtPath(after.defaults, 'stats.dexterity'), 18);
      },
    );
  });

  const cascadeBody =
      '{"data": {"derived": {"armorClass": 13}, "modifiers": {"dexterityMod": 3}}}';
  const previewFailBody =
      '{"error": {"code": "UNKNOWN_FIELD", "message": "unknown field"}}';
  const notFoundBody =
      '{"error": {"code": "NOT_FOUND", "message": "no such sheet"}}';
  const invalidBody =
      '{"error": {"code": "INVALID_REQUEST", "message": "not editable"}}';

  Future<http.Response> Function(http.Request) editResponder(
    List<http.Request> recorded, {
    Future<http.Response> Function(http.Request)? onPreview,
    Future<http.Response> Function(http.Request)? onPatch,
  }) => (request) async {
    if (request.method == 'PATCH') {
      return onPatch != null
          ? onPatch(request)
          : http.Response(cascadeBody, 200);
    }
    if (request.url.path.endsWith('/preview')) {
      return onPreview != null
          ? onPreview(request)
          : http.Response(cascadeBody, 200);
    }
    return http.Response(detailBody, 200);
  };

  List<http.Request> previews(List<http.Request> recorded) => recorded
      .where((request) => request.url.path.endsWith('/preview'))
      .toList();

  List<http.Request> patches(List<http.Request> recorded) => recorded
      .where((request) => request.method == 'PATCH')
      .toList();

  File draftFile() => File('${supportDir.path}/drafts/$sheetId.json');

  group('preview cycle (PRV-01 … PRV-14)', () {
    test(
      'Happy path: one dry-run merges the cascade, keeps edits pending and blocks a repeat (PRV-01, PRV-03 … PRV-07)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(recorded),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );

        expect(stateOf(container, sheetId).canPreview, isFalse);
        notifier.setPath('stats.dexterity', 18);
        expect(stateOf(container, sheetId).canPreview, isTrue);

        await notifier.preview();
        await pumpEventQueue();

        expect(previews(recorded), hasLength(1));
        expect(patches(recorded), isEmpty);
        expect(jsonDecode(previews(recorded).single.body), {
          'data': {
            'stats': {'dexterity': 18},
          },
        });

        final after = stateOf(container, sheetId);
        expect(valueAtPath(after.data, 'stats.dexterity'), 18);
        expect(valueAtPath(after.data, 'derived.armorClass'), 13);
        expect(
          valueAtPath(after.data, 'modifiers.dexterityMod'),
          3,
          reason: 'a nested cascade key is merged, not flattened',
        );
        expect(after.isPending, isTrue);
        expect(
          valueAtPath(after.defaults, 'stats.dexterity'),
          16,
          reason: 'the baseline is not advanced by a preview',
        );
        expect(valueAtPath(after.defaults, 'derived.armorClass'), 0.0);
        expect(after.editedPaths, contains('stats.dexterity'));
        expect(after.previewedRevision, after.revision);
        expect(after.canPreview, isFalse, reason: 'no new edit to preview');
        expect(after.previewMarks, isNotNull);
        expect(after.previewMarks!.modified, {'stats.dexterity'});
        expect(after.previewMarks!.recalculated, {
          'derived.armorClass',
          'modifiers.dexterityMod',
        });
      },
    );

    test(
      'Edge: a new edit after a preview clears the marks (PRV-14)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(recorded),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );

        notifier.setPath('stats.dexterity', 18);
        await notifier.preview();
        await pumpEventQueue();
        expect(stateOf(container, sheetId).previewMarks, isNotNull);

        notifier.setPath('stats.constitution', 20);

        expect(
          stateOf(container, sheetId).previewMarks,
          isNull,
          reason: 'a further edit leaves the last preview stale',
        );
      },
    );

    test(
      'Edge: an exclusive selection after a preview clears the marks (PRV-14)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(recorded),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );

        notifier.setPath('stats.dexterity', 18);
        await notifier.preview();
        await pumpEventQueue();
        expect(stateOf(container, sheetId).previewMarks, isNotNull);

        notifier.setExclusive('config.acUsesCon', const ['config.acUsesDex']);

        expect(stateOf(container, sheetId).previewMarks, isNull);
      },
    );

    test(
      'Edge: an empty cascade keeps the edits and the rendered values (PRV-05)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(
            recorded,
            onPreview: (_) async => http.Response('{"data": {}}', 200),
          ),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);
        final derivedBefore = valueAtPath(
          stateOf(container, sheetId).data,
          'derived.armorClass',
        );

        await notifier.preview();
        await pumpEventQueue();

        final after = stateOf(container, sheetId);
        expect(valueAtPath(after.data, 'stats.dexterity'), 18);
        expect(valueAtPath(after.data, 'derived.armorClass'), derivedBefore);
        expect(after.isPending, isTrue);
        expect(after.previewedRevision, after.revision);
        expect(after.previewMarks!.modified, {'stats.dexterity'});
        expect(
          after.previewMarks!.recalculated,
          isEmpty,
          reason: 'an empty cascade marks no calculated field',
        );
      },
    );

    test(
      'Error: a preview failure keeps the edits, publishes the copy and leaves Preview available (PRV-08, PRV-09)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(
            recorded,
            onPreview: (_) async => http.Response(previewFailBody, 400),
          ),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);

        await notifier.preview();
        await pumpEventQueue();

        final after = stateOf(container, sheetId);
        expect(previews(recorded), hasLength(1));
        expect(after.isPreviewing, isFalse);
        expect(after.previewError, errorCopy(ApiErrorKind.renderMismatch));
        expect(after.isPending, isTrue);
        expect(valueAtPath(after.data, 'stats.dexterity'), 18);
        expect(after.previewedRevision, isNull);
        expect(after.canPreview, isTrue);
        expect(
          after.previewMarks,
          isNull,
          reason: 'a failed preview publishes no marks',
        );
      },
    );

    test(
      'Edge: a preview NOT_FOUND keeps the edits, publishes the mapped copy and does not evict the local entry',
      () async {
        SheetLibraryStore(directory: supportDir).save([entry(sheetId)]);
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(
            recorded,
            onPreview: (_) async => http.Response(notFoundBody, 404),
          ),
        );
        expect(container.read(sheetLibraryProvider).entries, hasLength(1));

        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);

        await notifier.preview();
        await pumpEventQueue();

        final after = stateOf(container, sheetId);
        expect(after.isPending, isTrue);
        expect(valueAtPath(after.data, 'stats.dexterity'), 18);
        expect(after.previewError, errorCopy(ApiErrorKind.notFound));
        final entries = container.read(sheetLibraryProvider).entries;
        expect(
          entries,
          hasLength(1),
          reason: 'a failed preview must never evict the local entry',
        );
        expect(entries.single.id, sheetId);
      },
    );

    test(
      'Edge: an edit made while a preview is in flight is merged without being discarded (PRV-05, PRV-06)',
      () async {
        final gate = Completer<http.Response>();
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(recorded, onPreview: (_) => gate.future),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);
        final started = notifier.preview();
        await pumpEventQueue();
        expect(stateOf(container, sheetId).isPreviewing, isTrue);

        notifier.setPath('stats.constitution', 20);
        gate.complete(http.Response(cascadeBody, 200));
        await started;
        await pumpEventQueue();

        final after = stateOf(container, sheetId);
        expect(valueAtPath(after.data, 'stats.constitution'), 20);
        expect(valueAtPath(after.data, 'stats.dexterity'), 18);
        expect(valueAtPath(after.data, 'derived.armorClass'), 13);
        expect(after.previewedRevision, 1);
        expect(
          after.canPreview,
          isTrue,
          reason: 'the newer edit is un-previewed',
        );
        expect(
          after.previewMarks!.modified,
          {'stats.dexterity'},
          reason: 'only the previewed delta is marked, not the in-flight edit',
        );
      },
    );

    test(
      'Happy path: a save clears the preview record and deletes the draft backup (PRV-10, DRB-08)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(recorded),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);
        await notifier.preview();
        await pumpEventQueue();
        expect(
          stateOf(container, sheetId).previewMarks,
          isNotNull,
          reason: 'the preview published marks',
        );
        DraftBackupStore(directory: supportDir).write(sheetId, {
          'stats': {'dexterity': 18},
        });
        expect(draftFile().existsSync(), isTrue);

        await notifier.save();
        await pumpEventQueue();

        final after = stateOf(container, sheetId);
        expect(after.previewedRevision, isNull);
        expect(
          after.previewMarks,
          isNull,
          reason: 'a save clears the preview marks',
        );
        expect(draftFile().existsSync(), isFalse);
        expect(valueAtPath(after.data, 'derived.armorClass'), 13);

        notifier.setPath('stats.constitution', 20);
        expect(stateOf(container, sheetId).canPreview, isTrue);
      },
    );
  });

  group('single-flight (SVE-01)', () {
    test('Edge: while a preview is in flight a save issues no PATCH', () async {
      final gate = Completer<http.Response>();
      final recorded = <http.Request>[];
      final container = containerWithOpened(
        openedSheet(),
        recorded: recorded,
        handler: editResponder(recorded, onPreview: (_) => gate.future),
      );
      keepAlive(container, sheetId);
      await pumpEventQueue();
      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );
      notifier.setPath('stats.dexterity', 18);
      final started = notifier.preview();
      await pumpEventQueue();

      await notifier.save();
      await pumpEventQueue();

      expect(patches(recorded), isEmpty);
      gate.complete(http.Response(cascadeBody, 200));
      await started;
      await pumpEventQueue();
      expect(stateOf(container, sheetId).isBusy, isFalse);
    });

    test('Edge: while a save is in flight a preview issues no POST', () async {
      final gate = Completer<http.Response>();
      final recorded = <http.Request>[];
      final container = containerWithOpened(
        openedSheet(),
        recorded: recorded,
        handler: editResponder(recorded, onPatch: (_) => gate.future),
      );
      keepAlive(container, sheetId);
      await pumpEventQueue();
      final notifier = container.read(
        sheetEditControllerProvider(sheetId).notifier,
      );
      notifier.setPath('stats.dexterity', 18);
      final started = notifier.save();
      await pumpEventQueue();

      await notifier.preview();
      await pumpEventQueue();

      expect(previews(recorded), isEmpty);
      gate.complete(http.Response(cascadeBody, 200));
      await started;
      await pumpEventQueue();
      expect(stateOf(container, sheetId).isBusy, isFalse);
    });
  });

  group('rate limit (SVE-02 … SVE-07)', () {
    test(
      'Edge: three previews burn the window; the fourth issues nothing until the cooldown frees a slot (SVE-02, SVE-04)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(
            recorded,
            onPreview: (_) async => http.Response(previewFailBody, 400),
          ),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);

        for (var i = 0; i < 3; i++) {
          await notifier.preview();
          await pumpEventQueue();
        }
        expect(previews(recorded), hasLength(3));
        expect(stateOf(container, sheetId).isPreviewBlocked, isTrue);
        expect(stateOf(container, sheetId).canPreview, isFalse);
        expect(cooldown.isArmed, isTrue);
        expect(cooldown.pendingDelay, const Duration(seconds: 60));

        await notifier.preview();
        await pumpEventQueue();
        expect(
          previews(recorded),
          hasLength(3),
          reason: 'the fourth activation issues no request',
        );

        now = now.add(const Duration(seconds: 60));
        cooldown.fire();
        await pumpEventQueue();

        expect(stateOf(container, sheetId).isPreviewBlocked, isFalse);
        expect(stateOf(container, sheetId).canPreview, isTrue);

        await notifier.preview();
        await pumpEventQueue();
        expect(previews(recorded), hasLength(4));
      },
    );

    test(
      'Edge: three saves burn the save window; the fourth issues nothing until the cooldown (SVE-03, SVE-06)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(
            recorded,
            onPatch: (_) async => http.Response(invalidBody, 400),
          ),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);

        for (var i = 0; i < 3; i++) {
          await notifier.save();
          await pumpEventQueue();
        }
        expect(patches(recorded), hasLength(3));
        expect(stateOf(container, sheetId).isSaveBlocked, isTrue);
        expect(stateOf(container, sheetId).canSave, isFalse);

        await notifier.save();
        await pumpEventQueue();
        expect(
          patches(recorded),
          hasLength(3),
          reason: 'the fourth activation issues no request',
        );

        now = now.add(const Duration(seconds: 60));
        cooldown.fire();
        await pumpEventQueue();

        expect(stateOf(container, sheetId).isSaveBlocked, isFalse);
        expect(stateOf(container, sheetId).canSave, isTrue);
      },
    );

    test(
      'Edge: the preview and save windows are independent (SVE-05, SVE-07)',
      () async {
        final recorded = <http.Request>[];
        final container = containerWithOpened(
          openedSheet(),
          recorded: recorded,
          handler: editResponder(
            recorded,
            onPreview: (_) async => http.Response(previewFailBody, 400),
          ),
        );
        keepAlive(container, sheetId);
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider(sheetId).notifier,
        );
        notifier.setPath('stats.dexterity', 18);
        for (var i = 0; i < 3; i++) {
          await notifier.preview();
          await pumpEventQueue();
        }
        expect(stateOf(container, sheetId).isPreviewBlocked, isTrue);

        expect(stateOf(container, sheetId).canSave, isTrue);
        await notifier.save();
        await pumpEventQueue();

        expect(patches(recorded), hasLength(1));
        expect(jsonDecode(patches(recorded).single.body), {
          'data': {
            'stats': {'dexterity': 18},
          },
        });
      },
    );
  });

  group('draft restore (DRB-05 … DRB-08, DRB-12)', () {
    test(
      'Happy path: opening a sheet with a draft overlays only editable leaves and marks them pending (DRB-05, DRB-06, DRB-07)',
      () async {
        DraftBackupStore(directory: supportDir).write(sheetId, {
          'stats': {'dexterity': 18, 'unknownStat': 99},
          'derived': {'armorClass': 99},
        });
        final container = containerWithOpened(openedSheet());
        keepAlive(container, sheetId);
        await pumpEventQueue();

        final restored = stateOf(container, sheetId);
        expect(valueAtPath(restored.data, 'stats.dexterity'), 18);
        expect(
          valueAtPath(restored.data, 'derived.armorClass'),
          0.0,
          reason: 'a derived draft key is never overlaid',
        );
        expect(
          valueAtPath(restored.data, 'stats.unknownStat'),
          isNull,
          reason: 'a key absent from the template is ignored',
        );
        expect(
          valueAtPath(restored.defaults, 'stats.dexterity'),
          16,
          reason: 'the baseline stays the stored sheet',
        );
        expect(restored.editedPaths, contains('stats.dexterity'));
        expect(restored.editedPaths, isNot(contains('derived.armorClass')));
        expect(restored.editedPaths, isNot(contains('stats.unknownStat')));
        expect(restored.isPending, isTrue);
        expect(restored.pendingDelta, {
          'stats': {'dexterity': 18},
        });
        expect(
          restored.previewMarks,
          isNull,
          reason: 'a restored draft carries no preview marks (PRV-14)',
        );
      },
    );

    test('Edge: a corrupt draft seeds clean from the stored sheet (DRB-12)', () async {
      final file = draftFile();
      file.parent.createSync(recursive: true);
      file.writeAsStringSync('{ not json');
      final container = containerWithOpened(openedSheet());
      keepAlive(container, sheetId);
      await pumpEventQueue();

      final seeded = stateOf(container, sheetId);
      expect(valueAtPath(seeded.data, 'stats.dexterity'), 16);
      expect(seeded.editedPaths, isEmpty);
      expect(seeded.isPending, isFalse);
    });
  });
}

/// Test double for the one-shot cooldown: it records the delay and lets the
/// test fire the callback when the controllable clock reaches the edge.
class _FakeCooldownScheduler implements CooldownScheduler {
  Duration? pendingDelay;
  void Function()? _onFire;

  bool get isArmed => _onFire != null;

  @override
  void schedule(Duration delay, void Function() onFire) {
    pendingDelay = delay;
    _onFire = onFire;
  }

  @override
  void cancel() {
    pendingDelay = null;
    _onFire = null;
  }

  void fire() {
    final onFire = _onFire;
    pendingDelay = null;
    _onFire = null;
    onFire?.call();
  }
}
