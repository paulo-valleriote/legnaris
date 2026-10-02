import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';

/// Unit tests for the in-memory sheet edit state (LGN-102: GRN-01 … GRN-06).
/// Every asserted value comes from the spec rule, never from the
/// implementation.
void main() {
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}},
  "config": {"children": {"acUsesDex": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 1}}, "acUsesCon": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 0}}}},
  "derived": {"children": {"armorClass": {"schema": {"type": "number", "label": "", "formula": "x + 1", "is_editable": false, "default": 0}}}}
}}}
''';
  const otherBody = '''
{"data": {"id": "other_v1", "name": "Other Template", "default": false, "fields": {
  "stats": {"children": {"strength": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 8}}}}
}}}
''';

  ProviderContainer containerOver(
    Future<http.Response> Function(http.Request request) handler,
    List<http.Request> recorded,
  ) {
    final container = ProviderContainer(
      overrides: [
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

  /// Listens to keep the autoDispose edit state alive for one test.
  ProviderSubscription<SheetEditState> keepAlive(
    ProviderContainer container,
    String templateId,
  ) {
    final subscription = container.listen(
      sheetEditControllerProvider(templateId),
      (_, _) {},
    );
    addTearDown(subscription.close);
    return subscription;
  }

  SheetEditState stateOf(ProviderContainer container, String templateId) =>
      container.read(sheetEditControllerProvider(templateId));

  group('seeding (GRN-01)', () {
    test('Happy path: empty while the detail loads, then every leaf is seeded from its schema default (GRN-01)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        (request) async => http.Response(detailBody, 200),
        recorded,
      );

      keepAlive(container, 'fantasy_v1');
      expect(stateOf(container, 'fantasy_v1').data, isEmpty);

      await pumpEventQueue();
      final seeded = stateOf(container, 'fantasy_v1');
      expect(valueAtPath(seeded.data, 'stats.dexterity'), 10);
      expect(valueAtPath(seeded.data, 'config.acUsesDex'), 1);
      expect(valueAtPath(seeded.data, 'derived.armorClass'), 0);
      expect(seeded.isPending, isFalse);
      expect(recorded, hasLength(1));
      expect(recorded.single.method, 'GET');
    });
  });

  group('editing (GRN-02, GRN-03, GRN-04)', () {
    test('Happy path: setPath stores at the dotted path without mutating the previous state (GRN-02)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        (request) async => http.Response(detailBody, 200),
        recorded,
      );
      keepAlive(container, 'fantasy_v1');
      await pumpEventQueue();
      final before = stateOf(container, 'fantasy_v1');

      container
          .read(sheetEditControllerProvider('fantasy_v1').notifier)
          .setPath('stats.dexterity', 16);

      final after = stateOf(container, 'fantasy_v1');
      expect(valueAtPath(after.data, 'stats.dexterity'), 16);
      expect(valueAtPath(before.data, 'stats.dexterity'), 10);
      expect(identical(after.data, before.data), isFalse);
      expect(after.editedPaths, contains('stats.dexterity'));
    });

    test('Happy path: seeding and editing issue no request beyond the detail GET (GRN-03, GRN-04)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        (request) async => http.Response(detailBody, 200),
        recorded,
      );
      keepAlive(container, 'fantasy_v1');
      await pumpEventQueue();

      final notifier = container.read(
        sheetEditControllerProvider('fantasy_v1').notifier,
      );
      notifier.setPath('stats.dexterity', 16);
      notifier.setPath('config.acUsesDex', 0);
      notifier.setPath('derived.armorClass', 99);
      await pumpEventQueue();

      expect(recorded, hasLength(1));
      expect(recorded.every((request) => request.method == 'GET'), isTrue);
      expect(
        valueAtPath(
          stateOf(container, 'fantasy_v1').data,
          'derived.armorClass',
        ),
        99,
      );
    });

    test('Edge: the pending flag flips only while the state differs from the seeded defaults', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        (request) async => http.Response(detailBody, 200),
        recorded,
      );
      keepAlive(container, 'fantasy_v1');
      await pumpEventQueue();
      final notifier = container.read(
        sheetEditControllerProvider('fantasy_v1').notifier,
      );

      expect(stateOf(container, 'fantasy_v1').isPending, isFalse);

      notifier.setPath('stats.dexterity', 16);
      expect(stateOf(container, 'fantasy_v1').isPending, isTrue);

      notifier.setPath('stats.dexterity', 10);
      expect(stateOf(container, 'fantasy_v1').isPending, isFalse);
    });
  });

  group('lifetime (GRN-06)', () {
    test('Edge: different template ids hold independent edit state (family isolation)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        (request) async => http.Response(
          request.url.path.endsWith('other_v1') ? otherBody : detailBody,
          200,
        ),
        recorded,
      );
      keepAlive(container, 'fantasy_v1');
      keepAlive(container, 'other_v1');
      await pumpEventQueue();

      container
          .read(sheetEditControllerProvider('fantasy_v1').notifier)
          .setPath('stats.dexterity', 16);

      expect(
        valueAtPath(stateOf(container, 'fantasy_v1').data, 'stats.dexterity'),
        16,
      );
      expect(
        valueAtPath(stateOf(container, 'other_v1').data, 'stats.strength'),
        8,
      );
      expect(stateOf(container, 'other_v1').isPending, isFalse);
    });

    test('Edge: a detail reload discards edits as it starts and reseeds from the fresh schema (GRN-06)', () async {
      final gate = Completer<http.Response>();
      var calls = 0;
      final recorded = <http.Request>[];
      final container = containerOver(
        (request) async =>
            calls++ == 0 ? http.Response(detailBody, 200) : gate.future,
        recorded,
      );
      keepAlive(container, 'fantasy_v1');
      await pumpEventQueue();
      container
          .read(sheetEditControllerProvider('fantasy_v1').notifier)
          .setPath('stats.dexterity', 16);
      expect(stateOf(container, 'fantasy_v1').isPending, isTrue);

      final loadFuture = container
          .read(templateDetailControllerProvider('fantasy_v1').notifier)
          .load();
      await pumpEventQueue();

      final duringReload = stateOf(container, 'fantasy_v1');
      expect(duringReload.data, isEmpty);
      expect(duringReload.isPending, isFalse);

      gate.complete(http.Response(detailBody, 200));
      await loadFuture;
      await pumpEventQueue();

      final reseeded = stateOf(container, 'fantasy_v1');
      expect(valueAtPath(reseeded.data, 'stats.dexterity'), 10);
      expect(reseeded.isPending, isFalse);
      expect(recorded, hasLength(2));
      expect(recorded.every((request) => request.method == 'GET'), isTrue);
    });

    test('Edge: disposing the edit state discards it — a fresh subscription reseeds from defaults (GRN-06)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        (request) async => http.Response(detailBody, 200),
        recorded,
      );
      final first = keepAlive(container, 'fantasy_v1');
      await pumpEventQueue();
      container
          .read(sheetEditControllerProvider('fantasy_v1').notifier)
          .setPath('stats.dexterity', 16);

      first.close();
      await pumpEventQueue();

      keepAlive(container, 'fantasy_v1');
      await pumpEventQueue();
      final fresh = stateOf(container, 'fantasy_v1');
      expect(valueAtPath(fresh.data, 'stats.dexterity'), 10);
      expect(fresh.isPending, isFalse);
      expect(recorded.every((request) => request.method == 'GET'), isTrue);
    });
  });

  group('exclusive selection (EXC-13, EXC-14, EXC-16)', () {
    test(
      'Happy path: setExclusive sets the selected member to 1 and every sibling to 0 in one emission (EXC-13)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          (request) async => http.Response(detailBody, 200),
          recorded,
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();
        final before = stateOf(container, 'fantasy_v1');
        final emissions = <SheetEditState>[];
        final subscription = container.listen(
          sheetEditControllerProvider('fantasy_v1'),
          (_, next) => emissions.add(next),
        );
        addTearDown(subscription.close);

        container
            .read(sheetEditControllerProvider('fantasy_v1').notifier)
            .setExclusive('config.acUsesCon', const ['config.acUsesDex']);

        final after = stateOf(container, 'fantasy_v1');
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
        expect(recorded, hasLength(1));
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
      },
    );

    test(
      'Edge: selecting the already-active option leaves the state instance identical and emits nothing (EXC-14)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          (request) async => http.Response(detailBody, 200),
          recorded,
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();
        final before = stateOf(container, 'fantasy_v1');
        var emissions = 0;
        final subscription = container.listen(
          sheetEditControllerProvider('fantasy_v1'),
          (_, _) => emissions++,
        );
        addTearDown(subscription.close);

        container
            .read(sheetEditControllerProvider('fantasy_v1').notifier)
            .setExclusive('config.acUsesDex', const ['config.acUsesCon']);

        expect(identical(stateOf(container, 'fantasy_v1'), before), isTrue);
        expect(emissions, 0);
        expect(recorded, hasLength(1));
      },
    );

    test(
      'Edge: a selected member already on still zeroes a lingering sibling (EXC-13)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          (request) async => http.Response(detailBody, 200),
          recorded,
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider('fantasy_v1').notifier,
        );
        notifier.setPath('config.acUsesCon', 1);
        final before = stateOf(container, 'fantasy_v1');

        notifier.setExclusive('config.acUsesDex', const ['config.acUsesCon']);

        final after = stateOf(container, 'fantasy_v1');
        expect(valueAtPath(after.data, 'config.acUsesDex'), 1);
        expect(valueAtPath(after.data, 'config.acUsesCon'), 0);
        expect(identical(after, before), isFalse);
      },
    );

    test(
      'Happy path: setExclusive(null) clears every member in one emission and issues no request (EXC-13, EXC-27)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          (request) async => http.Response(detailBody, 200),
          recorded,
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();
        final before = stateOf(container, 'fantasy_v1');
        final emissions = <SheetEditState>[];
        final subscription = container.listen(
          sheetEditControllerProvider('fantasy_v1'),
          (_, next) => emissions.add(next),
        );
        addTearDown(subscription.close);

        container
            .read(sheetEditControllerProvider('fantasy_v1').notifier)
            .setExclusive(null, const ['config.acUsesDex', 'config.acUsesCon']);

        final after = stateOf(container, 'fantasy_v1');
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
        expect(recorded, hasLength(1));
        expect(recorded.every((request) => request.method == 'GET'), isTrue);
      },
    );

    test(
      'Edge: clearing an already-empty group leaves the state instance identical (EXC-14)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          (request) async => http.Response(detailBody, 200),
          recorded,
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();
        final notifier = container.read(
          sheetEditControllerProvider('fantasy_v1').notifier,
        );
        notifier.setExclusive(null, const ['config.acUsesDex', 'config.acUsesCon']);
        final cleared = stateOf(container, 'fantasy_v1');
        var emissions = 0;
        final subscription = container.listen(
          sheetEditControllerProvider('fantasy_v1'),
          (_, _) => emissions++,
        );
        addTearDown(subscription.close);

        notifier.setExclusive(null, const ['config.acUsesDex', 'config.acUsesCon']);

        expect(identical(stateOf(container, 'fantasy_v1'), cleared), isTrue);
        expect(emissions, 0);
        expect(recorded, hasLength(1));
      },
    );
  });
}
