import 'dart:async';

import 'package:flutter/widgets.dart' show Locale;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/catalog/template_detail_provider.dart';
import 'package:legnaris/l10n/app_localizations.dart';

/// Unit tests for the on-demand template detail provider
/// (TPL-01, TPL-04, TPL-06 … TPL-10). Every asserted value comes from the
/// spec rule, never from the implementation.
void main() {
  const detailBody = '''
{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {
  "baseArmorClass": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}},
  "stats": {"children": {"dexterity": {"schema": {"type": "number", "label": "", "is_editable": true, "default": 10}}}}
}}}
''';
  const noDataKeyBody = '{"unexpected": true}';
  const malformedBody = '{"data": [1, 2, 3]}';
  const notFoundBody =
      '{"error": {"code": "NOT_FOUND", "message": "template not found"}}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

  String parseCopy() => lookupAppLocalizations(const Locale('en')).errorParse;

  ApiClient clientOver(
    Future<http.Response> Function(http.Request request) handler,
    List<http.Request> recorded,
  ) => ApiClient(
    httpClient: MockClient((request) async {
      recorded.add(request);
      return handler(request);
    }),
  );

  ProviderContainer containerOver(ApiClient client) {
    final container = ProviderContainer(
      overrides: [apiClientProvider.overrideWithValue(client)],
    );
    addTearDown(container.dispose);
    return container;
  }

  /// Listens to keep the autoDispose family alive for the duration of a test.
  void keepAlive(ProviderContainer container, String templateId) {
    final subscription = container.listen(
      templateDetailControllerProvider(templateId),
      (_, _) {},
    );
    addTearDown(subscription.close);
  }

  TemplateDetailState stateOf(ProviderContainer container, String id) =>
      container.read(templateDetailControllerProvider(id));

  group('on-demand load (TPL-01, TPL-06)', () {
    test(
      'Edge: the first watch fires exactly one GET /templates/:id and stays loading until it resolves (TPL-01)',
      () async {
        final gate = Completer<http.Response>();
        final recorded = <http.Request>[];
        final container = containerOver(
          clientOver((request) => gate.future, recorded),
        );
        keepAlive(container, 'fantasy_v1');

        final initial = stateOf(container, 'fantasy_v1');
        expect(initial.isLoading, isTrue);
        expect(initial.template, isNull);
        expect(initial.errorMessage, isNull);

        await pumpEventQueue();
        expect(recorded, hasLength(1));
        expect(recorded.single.method, 'GET');
        expect(recorded.single.url.path, '/templates/fantasy_v1');
        expect(stateOf(container, 'fantasy_v1').isLoading, isTrue);

        gate.complete(http.Response(detailBody, 200));
        await pumpEventQueue();

        final settled = stateOf(container, 'fantasy_v1');
        expect(settled.isLoading, isFalse);
        expect(settled.template, isNotNull);
        expect(recorded, hasLength(1));
      },
    );

    test(
      'Happy path: success exposes the decoded detail with the fields tree intact (TPL-06)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          clientOver((request) async => http.Response(detailBody, 200), recorded),
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();

        final template = stateOf(container, 'fantasy_v1').template;
        expect(template, isNotNull);
        expect(template!.id, 'fantasy_v1');
        expect(template.name, 'Fantasy Template');
        expect(template.fields['stats']?.schema, isNull);
        expect(
          template.fields['stats']?.children?['dexterity']?.schema?.type,
          'number',
        );
      },
    );

    test(
      'Edge: a new template id starts loading with no template, never a stale summary (TPL-04)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          clientOver((request) async => http.Response(detailBody, 200), recorded),
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();
        expect(stateOf(container, 'fantasy_v1').template, isNotNull);

        keepAlive(container, 'other_v1');
        final fresh = stateOf(container, 'other_v1');
        expect(fresh.isLoading, isTrue);
        expect(fresh.template, isNull);
        expect(fresh.errorMessage, isNull);
      },
    );
  });

  group('failure copies (TPL-07, TPL-09, TPL-10)', () {
    test('Error: a domain failure maps to the kind copy (TPL-07)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver((request) async => http.Response(serverErrorBody, 500), recorded),
      );
      keepAlive(container, 'fantasy_v1');
      await pumpEventQueue();

      final state = stateOf(container, 'fantasy_v1');
      expect(state.errorMessage, errorCopy(ApiErrorKind.serverError));
      expect(state.template, isNull);
      expect(state.isLoading, isFalse);
      expect(recorded, hasLength(1));
    });

    test('Error: NOT_FOUND shows the not-found copy (TPL-09)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver((request) async => http.Response(notFoundBody, 404), recorded),
      );
      keepAlive(container, 'missing_v1');
      await pumpEventQueue();

      final state = stateOf(container, 'missing_v1');
      expect(state.errorMessage, errorCopy(ApiErrorKind.notFound));
      expect(state.template, isNull);
    });

    test(
      'Error: a 2xx without the data key takes the parse-failure path (TPL-10)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          clientOver((request) async => http.Response(noDataKeyBody, 200), recorded),
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();

        final state = stateOf(container, 'fantasy_v1');
        expect(state.errorMessage, parseCopy());
        expect(state.template, isNull);
      },
    );

    test(
      'Error: a malformed payload shape takes the parse-failure path, never a half detail (TPL-10)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          clientOver((request) async => http.Response(malformedBody, 200), recorded),
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();

        final state = stateOf(container, 'fantasy_v1');
        expect(state.errorMessage, parseCopy());
        expect(state.template, isNull);
      },
    );

    test(
      'Error: a transport failure maps to the offline copy (kind offline)',
      () async {
        final recorded = <http.Request>[];
        final container = containerOver(
          clientOver(
            (request) async => throw http.ClientException('connection refused'),
            recorded,
          ),
        );
        keepAlive(container, 'fantasy_v1');
        await Future<void>.delayed(const Duration(milliseconds: 600));
        await pumpEventQueue();

        expect(
          stateOf(container, 'fantasy_v1').errorMessage,
          errorCopy(ApiErrorKind.offline),
        );
        expect(recorded, hasLength(2));
      },
    );
  });

  group('retry policy (TPL-08)', () {
    test(
      'Happy path: load() after a failure re-requests exactly once and recovers (TPL-08)',
      () async {
        var fail = true;
        final recorded = <http.Request>[];
        final container = containerOver(
          clientOver(
            (request) async => http.Response(
              fail ? serverErrorBody : detailBody,
              fail ? 500 : 200,
            ),
            recorded,
          ),
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();
        expect(recorded, hasLength(1));
        expect(stateOf(container, 'fantasy_v1').errorMessage, isNotNull);

        fail = false;
        await container
            .read(templateDetailControllerProvider('fantasy_v1').notifier)
            .load();

        expect(recorded, hasLength(2));
        final state = stateOf(container, 'fantasy_v1');
        expect(state.errorMessage, isNull);
        expect(state.template, isNotNull);
      },
    );

    test(
      'Edge: a load() while a request is in flight issues no second request (single-flight)',
      () async {
        final gate = Completer<http.Response>();
        final recorded = <http.Request>[];
        final container = containerOver(
          clientOver((request) => gate.future, recorded),
        );
        keepAlive(container, 'fantasy_v1');
        await pumpEventQueue();
        expect(recorded, hasLength(1));

        await container
            .read(templateDetailControllerProvider('fantasy_v1').notifier)
            .load();
        expect(recorded, hasLength(1));

        gate.complete(http.Response(detailBody, 200));
        await pumpEventQueue();
        expect(stateOf(container, 'fantasy_v1').template, isNotNull);
      },
    );
  });

  group('dispose safety (TPL-14)', () {
    test(
      'Edge: disposing the surface during an in-flight load does not throw',
      () async {
        final gate = Completer<http.Response>();
        final recorded = <http.Request>[];
        final container = ProviderContainer(
          overrides: [
            apiClientProvider.overrideWithValue(
              clientOver((request) => gate.future, recorded),
            ),
          ],
        );
        final subscription = container.listen(
          templateDetailControllerProvider('fantasy_v1'),
          (_, _) {},
        );
        await pumpEventQueue();
        expect(recorded, hasLength(1));

        subscription.close();
        container.dispose();

        gate.complete(http.Response(detailBody, 200));
        await pumpEventQueue();
      },
    );
  });
}
