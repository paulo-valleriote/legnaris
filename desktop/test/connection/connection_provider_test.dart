import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/catalog/catalog_provider.dart';
import 'package:legnaris/connection/connection_provider.dart';
import 'package:legnaris/connection/health_scheduler.dart';

import 'manual_health_scheduler.dart';

void main() {
  const healthyBody = '{"data": {"status": "healthy"}}';
  const degradedBody = '{"data": {"status": "degraded"}}';
  const missingStatusBody = '{"data": {"missing": true}}';
  const notFoundBody =
      '{"error": {"code": "NOT_FOUND", "message": "no such thing"}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';
  const oneTemplateBody =
      '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}';

  int healthCount(List<http.Request> recorded) =>
      recorded.where((request) => request.url.path == '/health').length;

  int listCount(List<http.Request> recorded) =>
      recorded.where((request) => request.url.path == '/templates').length;

  ApiClient clientOver(
    Future<http.Response> Function(http.Request request) handler,
    List<http.Request> recorded,
  ) => ApiClient(
    httpClient: MockClient((request) async {
      recorded.add(request);
      return handler(request);
    }),
  );

  ProviderContainer containerOver(
    ManualHealthScheduler scheduler,
    ApiClient client,
  ) {
    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(client),
        healthSchedulerProvider.overrideWithValue(scheduler),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  group('state model and boot check (CONN-01, CONN-02, CONN-06)', () {
    test('Happy path: the state model defaults to checking with no last success (CONN-02)', () {
      const initial = ConnectionState();

      expect(initial.status, ConnectionStatus.checking);
      expect(initial.lastSuccess, isNull);
    });

    test('Edge: the boot state holds checking with a null last success until the first response lands (CONN-02, CONN-06)', () async {
      final gate = Completer<http.Response>();
      final recorded = <http.Request>[];
      final container = containerOver(
        ManualHealthScheduler(),
        clientOver((request) => gate.future, recorded),
      );

      final initial = container.read(connectionProvider);
      expect(initial.status, ConnectionStatus.checking);
      expect(initial.lastSuccess, isNull);

      await pumpEventQueue();
      expect(
        healthCount(recorded),
        1,
        reason: 'the boot check runs without any scheduler tick',
      );
      expect(
        container.read(connectionProvider).status,
        ConnectionStatus.checking,
        reason: 'the state holds until the in-flight check resolves',
      );

      gate.complete(http.Response(healthyBody, 200));
      await pumpEventQueue();

      final settled = container.read(connectionProvider);
      expect(settled.status, ConnectionStatus.online);
      expect(settled.lastSuccess, isNotNull);
    });

    test('Happy path: boot issues exactly one GET /health with no tick and starts the schedule once (CONN-01, CONN-12)', () async {
      final recorded = <http.Request>[];
      final scheduler = ManualHealthScheduler();
      final container = containerOver(
        scheduler,
        clientOver((request) async => http.Response(healthyBody, 200), recorded),
      );

      container.read(connectionProvider);
      await pumpEventQueue();

      expect(scheduler.startCount, 1, reason: 'the schedule starts once at build');
      expect(
        healthCount(recorded),
        1,
        reason: 'the boot check needs no tick',
      );
      final healthRequest = recorded.singleWhere(
        (request) => request.url.path == '/health',
      );
      expect(healthRequest.method, 'GET');
      expect(container.read(connectionProvider).status, ConnectionStatus.online);
    });

    test('Edge: a second reader shares the boot check — build runs once (CONN-01)', () async {
      final recorded = <http.Request>[];
      final scheduler = ManualHealthScheduler();
      final container = containerOver(
        scheduler,
        clientOver((request) async => http.Response(healthyBody, 200), recorded),
      );

      container.read(connectionProvider);
      await pumpEventQueue();
      container.read(connectionProvider);
      await pumpEventQueue();

      expect(healthCount(recorded), 1);
      expect(scheduler.startCount, 1);
    });

    test('Edge: a scheduler tick after boot issues the next health check (CONN-01)', () async {
      final recorded = <http.Request>[];
      final scheduler = ManualHealthScheduler();
      final container = containerOver(
        scheduler,
        clientOver((request) async => http.Response(healthyBody, 200), recorded),
      );

      container.read(connectionProvider);
      await pumpEventQueue();
      expect(healthCount(recorded), 1);

      scheduler.fire();
      await pumpEventQueue();

      expect(healthCount(recorded), 2);
      expect(container.read(connectionProvider).status, ConnectionStatus.online);
    });
  });

  group('health response mapping (CONN-03, CONN-04, CONN-13)', () {
    test('Happy path: any decoded 2xx payload lands on online (CONN-03, CONN-13)', () async {
      const cases = <(String, String)>[
        ('200 healthy', healthyBody),
        ('200 degraded', degradedBody),
      ];

      for (final (label, body) in cases) {
        final recorded = <http.Request>[];
        final container = containerOver(
          ManualHealthScheduler(),
          clientOver((request) async => http.Response(body, 200), recorded),
        );

        container.read(connectionProvider);
        await pumpEventQueue();

        final state = container.read(connectionProvider);
        expect(state.status, ConnectionStatus.online, reason: label);
        expect(state.lastSuccess, isNotNull, reason: label);
      }
    });

    test('Error: a 4xx or 5xx response lands on offline (CONN-04)', () async {
      const cases = <(String, int, String)>[
        ('404', 404, notFoundBody),
        ('500', 500, serverErrorBody),
      ];

      for (final (label, statusCode, body) in cases) {
        final recorded = <http.Request>[];
        final container = containerOver(
          ManualHealthScheduler(),
          clientOver(
            (request) async => http.Response(body, statusCode),
            recorded,
          ),
        );

        container.read(connectionProvider);
        await pumpEventQueue();

        final state = container.read(connectionProvider);
        expect(state.status, ConnectionStatus.offline, reason: label);
        expect(state.lastSuccess, isNull, reason: label);
      }
    });

    test('Error: a 2xx without data.status lands on offline (CONN-04 parse edge)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        ManualHealthScheduler(),
        clientOver((request) async => http.Response(missingStatusBody, 200), recorded),
      );

      container.read(connectionProvider);
      await pumpEventQueue();

      final state = container.read(connectionProvider);
      expect(state.status, ConnectionStatus.offline);
      expect(state.lastSuccess, isNull);
    });

    test('Error: a transport failure lands on offline after the client retry (CONN-04)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        ManualHealthScheduler(),
        clientOver(
          (request) async => throw http.ClientException('connection refused'),
          recorded,
        ),
      );

      container.read(connectionProvider);
      // The Phase-1 client retries once after a fixed backoff, so the
      // offline verdict only lands after that delay.
      await Future<void>.delayed(const Duration(milliseconds: 600));
      await pumpEventQueue();

      final state = container.read(connectionProvider);
      expect(state.status, ConnectionStatus.offline);
      expect(state.lastSuccess, isNull);
      expect(healthCount(recorded), 2, reason: 'one retry per the client policy');
    });
  });

  group('single-flight and state holds (CONN-05, CONN-06)', () {
    test('Edge: a tick during an in-flight check issues no second request and re-arms after it resolves (CONN-05)', () async {
      final gate = Completer<http.Response>();
      final recorded = <http.Request>[];
      final scheduler = ManualHealthScheduler();
      final container = containerOver(
        scheduler,
        clientOver((request) => gate.future, recorded),
      );

      container.read(connectionProvider);
      await pumpEventQueue();
      expect(healthCount(recorded), 1);

      scheduler.fire();
      scheduler.fire();
      await pumpEventQueue();

      expect(
        healthCount(recorded),
        1,
        reason: 'ticks during the flight are discarded (CONN-05)',
      );
      expect(
        container.read(connectionProvider).status,
        ConnectionStatus.checking,
        reason: 'the state still holds while the check is in flight',
      );

      gate.complete(http.Response(healthyBody, 200));
      await pumpEventQueue();
      expect(container.read(connectionProvider).status, ConnectionStatus.online);

      scheduler.fire();
      await pumpEventQueue();
      expect(
        healthCount(recorded),
        2,
        reason: 'the guard releases once the check resolves',
      );
    });

    test('Edge: the previous state and timestamp hold until the in-flight check resolves (CONN-06)', () async {
      Completer<http.Response>? gate;
      final recorded = <http.Request>[];
      final scheduler = ManualHealthScheduler();
      final container = containerOver(
        scheduler,
        clientOver(
          (request) async => gate?.future ?? http.Response(healthyBody, 200),
          recorded,
        ),
      );

      container.read(connectionProvider);
      await pumpEventQueue();
      final online = container.read(connectionProvider);
      expect(online.status, ConnectionStatus.online);
      final timestamp = online.lastSuccess;
      expect(timestamp, isNotNull);

      gate = Completer<http.Response>();
      scheduler.fire();
      await pumpEventQueue();

      final inflight = container.read(connectionProvider);
      expect(
        inflight.status,
        ConnectionStatus.online,
        reason: 'no flicker back to checking while a check flies',
      );
      expect(inflight.lastSuccess, timestamp);

      gate.complete(http.Response(serverErrorBody, 500));
      await pumpEventQueue();

      final failed = container.read(connectionProvider);
      expect(failed.status, ConnectionStatus.offline);
      expect(
        failed.lastSuccess,
        timestamp,
        reason: 'the failure carries the previous timestamp',
      );
    });
  });

  group('last success timestamp (CONN-09)', () {
    test('Edge: lastSuccess is null before the first success, set on it, and preserved by later failures (CONN-09)', () async {
      final gate = Completer<http.Response>();
      var failNext = false;
      final recorded = <http.Request>[];
      final scheduler = ManualHealthScheduler();
      final container = containerOver(
        scheduler,
        clientOver(
          (request) async =>
              failNext
                  ? http.Response(serverErrorBody, 500)
                  : gate.future,
          recorded,
        ),
      );

      container.read(connectionProvider);
      await pumpEventQueue();
      expect(container.read(connectionProvider).lastSuccess, isNull);

      gate.complete(http.Response(healthyBody, 200));
      await pumpEventQueue();
      final timestamp = container.read(connectionProvider).lastSuccess;
      expect(timestamp, isNotNull);

      failNext = true;
      scheduler.fire();
      await pumpEventQueue();

      final failed = container.read(connectionProvider);
      expect(failed.status, ConnectionStatus.offline);
      expect(failed.lastSuccess, timestamp, reason: 'kept across the failure');
    });
  });

  group('schedule independence from API traffic (CONN-07)', () {
    test('Happy path: catalog traffic issues no health request and never restarts the schedule (CONN-07)', () async {
      final recorded = <http.Request>[];
      final scheduler = ManualHealthScheduler();
      final container = containerOver(
        scheduler,
        clientOver(
          (request) async =>
              request.url.path == '/templates'
                  ? http.Response(oneTemplateBody, 200)
                  : http.Response(healthyBody, 200),
          recorded,
        ),
      );

      container.read(connectionProvider);
      await pumpEventQueue();
      expect(healthCount(recorded), 1);
      expect(scheduler.startCount, 1);

      container.read(catalogProvider);
      await pumpEventQueue();
      expect(listCount(recorded), 1);
      expect(healthCount(recorded), 1, reason: 'catalog load adds no health request');
      expect(scheduler.startCount, 1, reason: 'catalog load does not restart the schedule');

      await container.read(catalogProvider.notifier).refresh();
      expect(listCount(recorded), 2);
      expect(healthCount(recorded), 1);
      expect(scheduler.startCount, 1);

      scheduler.fire();
      await pumpEventQueue();
      expect(
        healthCount(recorded),
        2,
        reason: 'the schedule still drives the connection check',
      );
    });
  });

  group('recovery (CONN-06 edge)', () {
    test('Edge: a successful check while offline recovers to online (CONN-06)', () async {
      var healthy = false;
      final recorded = <http.Request>[];
      final scheduler = ManualHealthScheduler();
      final container = containerOver(
        scheduler,
        clientOver(
          (request) async =>
              healthy
                  ? http.Response(healthyBody, 200)
                  : http.Response(serverErrorBody, 500),
          recorded,
        ),
      );

      container.read(connectionProvider);
      await pumpEventQueue();
      expect(container.read(connectionProvider).status, ConnectionStatus.offline);

      healthy = true;
      scheduler.fire();
      await pumpEventQueue();

      final recovered = container.read(connectionProvider);
      expect(recovered.status, ConnectionStatus.online);
      expect(recovered.lastSuccess, isNotNull);
    });
  });
}
