import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/sheet/sheet_edit_provider.dart';

/// Unit tests for the in-memory sheet edit state (LGN-102: GRN-01 … GRN-06,
/// LGN-108: OPN-10, OPN-11). Every asserted value comes from the spec rule,
/// never from the implementation.
void main() {
  const sheetId = 'sheet-1';
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
              sheetBody(id, templateId: id == 'sheet-other' ? 'other_v1' : 'fantasy_v1'),
              200,
            );
          }
          return http.Response(
            path.endsWith('other_v1') ? otherBody : detailBody,
            200,
          );
        }, recorded);

        keepAlive(container, sheetId);
        keepAlive(container, 'sheet-other');
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
            stateOf(container, 'sheet-other').data,
            'stats.strength',
          ),
          8.0,
        );
        expect(stateOf(container, 'sheet-other').isPending, isFalse);
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
}
