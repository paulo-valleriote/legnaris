import 'dart:async';

import 'package:flutter/widgets.dart' show Locale;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:legnaris/catalog/catalog_provider.dart';
import 'package:legnaris/l10n/app_localizations.dart';

void main() {
  const twoTemplatesBody = '''
{"data": [
  {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}},
  {"id": "sci_fi_v1", "name": "Sci Fi Template", "default": false, "fields": {}}
]}
''';
  const oneTemplateBody =
      '{"data": [{"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}}]}';
  const emptyListBody = '{"data": []}';
  const noDataKeyBody = '{"unexpected": true}';
  const malformedEntryBody = '{"data": [{"id": "fantasy_v1"}]}';
  const emptyNameBody =
      '{"data": [{"id": "blank", "name": "", "default": false, "fields": {}}]}';
  const serverErrorBody =
      '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}';

  String parseCopy() =>
      lookupAppLocalizations(const Locale('en')).errorParse;

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

  group('CatalogState model (design)', () {
    test('Happy path: hasLoaded and isEmpty mirror the templates field', () {
      const cases = <(String, CatalogState, bool, bool)>[
        ('never loaded', CatalogState(), false, false),
        ('loaded empty', CatalogState(templates: []), true, true),
        (
          'loaded with entries',
          CatalogState(templates: [
            TemplateDetail(
              id: 'fantasy_v1',
              name: 'Fantasy Template',
              isDefault: true,
              fields: {},
            ),
          ]),
          true,
          false,
        ),
      ];

      for (final (label, state, hasLoaded, isEmpty) in cases) {
        expect(state.hasLoaded, hasLoaded, reason: label);
        expect(state.isEmpty, isEmpty, reason: label);
      }
    });

    test('Happy path: copyWith overrides given fields and preserves the rest', () {
      const base = CatalogState(isLoading: true, lastUpdated: null);
      final derived = base.copyWith(errorMessage: 'boom');

      expect(derived.errorMessage, 'boom');
      expect(derived.isLoading, isTrue);
      expect(derived.templates, isNull);
    });

    test('Edge: the stored template list is immutable once loaded', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver((request) async => http.Response(twoTemplatesBody, 200), recorded),
      );

      container.read(catalogProvider);
      await pumpEventQueue();

      final state = container.read(catalogProvider);
      expect(
        () => state.templates!.add(
          const TemplateDetail(
            id: 'extra',
            name: 'Extra',
            isDefault: false,
            fields: {},
          ),
        ),
        throwsUnsupportedError,
      );
    });
  });

  group('first load (CAT-07 … CAT-14)', () {
    test('Edge: first read fires exactly one GET /templates and stays loading until it resolves (CAT-07)', () async {
      final gate = Completer<http.Response>();
      final recorded = <http.Request>[];
      final container = containerOver(clientOver((request) => gate.future, recorded));

      final initial = container.read(catalogProvider);
      expect(initial.isLoading, isTrue);
      expect(initial.templates, isNull);
      expect(initial.errorMessage, isNull);

      await pumpEventQueue();
      expect(recorded, hasLength(1));
      expect(recorded.single.method, 'GET');
      expect(recorded.single.url.path, '/templates');
      expect(container.read(catalogProvider).isLoading, isTrue);

      gate.complete(http.Response(twoTemplatesBody, 200));
      await pumpEventQueue();

      final settled = container.read(catalogProvider);
      expect(settled.isLoading, isFalse);
      expect(settled.templates, hasLength(2));
      expect(recorded, hasLength(1));
    });

    test('Happy path: success stores the list in API order with lastUpdated (CAT-09)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver((request) async => http.Response(twoTemplatesBody, 200), recorded),
      );

      container.read(catalogProvider);
      await pumpEventQueue();

      final state = container.read(catalogProvider);
      expect(
        state.templates!.map((template) => template.id).toList(),
        ['fantasy_v1', 'sci_fi_v1'],
      );
      expect(state.lastUpdated, isNotNull);
      expect(state.errorMessage, isNull);
      expect(state.isLoading, isFalse);
    });

    test('Edge: an empty successful list becomes the empty state (CAT-10)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver((request) async => http.Response(emptyListBody, 200), recorded),
      );

      container.read(catalogProvider);
      await pumpEventQueue();

      final state = container.read(catalogProvider);
      expect(state.hasLoaded, isTrue);
      expect(state.isEmpty, isTrue);
      expect(state.errorMessage, isNull);
    });

    test('Edge: an entry with an empty name decodes into the state (model edge)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver((request) async => http.Response(emptyNameBody, 200), recorded),
      );

      container.read(catalogProvider);
      await pumpEventQueue();

      final state = container.read(catalogProvider);
      expect(state.templates, hasLength(1));
      expect(state.templates!.single.name, '');
      expect(state.templates!.single.id, 'blank');
    });

    test('Error: domain failure on the first load maps to the kind copy (CAT-11)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver((request) async => http.Response(serverErrorBody, 500), recorded),
      );

      container.read(catalogProvider);
      await pumpEventQueue();

      final state = container.read(catalogProvider);
      expect(state.errorMessage, errorCopy(ApiErrorKind.serverError));
      expect(state.templates, isNull);
      expect(state.hasLoaded, isFalse);
      expect(state.isLoading, isFalse);
      expect(recorded, hasLength(1));
    });

    test('Error: a transport failure maps to the offline copy (kind offline)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver(
          (request) async => throw http.ClientException('connection refused'),
          recorded,
        ),
      );

      container.read(catalogProvider);
      // The client retries a transport failure once after a fixed backoff,
      // so the offline verdict only lands after that delay.
      await Future<void>.delayed(const Duration(milliseconds: 600));
      await pumpEventQueue();

      final state = container.read(catalogProvider);
      expect(state.errorMessage, errorCopy(ApiErrorKind.offline));
      expect(state.templates, isNull);
      expect(recorded, hasLength(2));
    });

    test('Error: a 2xx without the data key takes the parse-failure path (CAT-14 edge)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver((request) async => http.Response(noDataKeyBody, 200), recorded),
      );

      container.read(catalogProvider);
      await pumpEventQueue();

      final state = container.read(catalogProvider);
      expect(state.errorMessage, parseCopy());
      expect(state.hasLoaded, isFalse);
      expect(state.templates, isNull);
    });

    test('Error: a malformed entry maps to the parse copy, never a partial list (CAT-14)', () async {
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver((request) async => http.Response(malformedEntryBody, 200), recorded),
      );

      container.read(catalogProvider);
      await pumpEventQueue();

      final state = container.read(catalogProvider);
      expect(state.errorMessage, parseCopy());
      expect(state.templates, isNull);
      expect(state.hasLoaded, isFalse);
    });
  });

  group('refresh policy (CAT-12, CAT-13, CAT-18, CAT-19)', () {
    test('Edge: a trigger while a request is in flight issues no second request (CAT-13)', () async {
      final gate = Completer<http.Response>();
      final recorded = <http.Request>[];
      final container = containerOver(clientOver((request) => gate.future, recorded));

      container.read(catalogProvider);
      await pumpEventQueue();
      expect(recorded, hasLength(1));

      await container.read(catalogProvider.notifier).refresh();
      expect(recorded, hasLength(1));
      expect(container.read(catalogProvider).isLoading, isTrue);

      gate.complete(http.Response(twoTemplatesBody, 200));
      await pumpEventQueue();

      final state = container.read(catalogProvider);
      expect(state.isLoading, isFalse);
      expect(state.templates, hasLength(2));
      expect(recorded, hasLength(1));
    });

    test('Happy path: retry after a failure re-requests exactly once (CAT-12)', () async {
      var fail = true;
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver(
          (request) async =>
              http.Response(fail ? serverErrorBody : twoTemplatesBody, fail ? 500 : 200),
          recorded,
        ),
      );

      container.read(catalogProvider);
      await pumpEventQueue();
      expect(recorded, hasLength(1));
      expect(container.read(catalogProvider).errorMessage, isNotNull);

      fail = false;
      await container.read(catalogProvider.notifier).refresh();

      expect(recorded, hasLength(2));
      final state = container.read(catalogProvider);
      expect(state.errorMessage, isNull);
      expect(state.templates, hasLength(2));
    });

    test('Happy path: a successful refresh replaces the cached list (CAT-18)', () async {
      var call = 0;
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver(
          (request) async => http.Response(call++ == 0 ? oneTemplateBody : twoTemplatesBody, 200),
          recorded,
        ),
      );

      container.read(catalogProvider);
      await pumpEventQueue();
      expect(
        container.read(catalogProvider).templates!.map((template) => template.id),
        ['fantasy_v1'],
      );

      await container.read(catalogProvider.notifier).refresh();

      expect(recorded, hasLength(2));
      final state = container.read(catalogProvider);
      expect(
        state.templates!.map((template) => template.id).toList(),
        ['fantasy_v1', 'sci_fi_v1'],
      );
      expect(state.errorMessage, isNull);
      expect(state.lastUpdated, isNotNull);
    });

    test('Error: a failed refresh keeps the cached list and sets the error (CAT-19)', () async {
      var fail = false;
      final recorded = <http.Request>[];
      final container = containerOver(
        clientOver(
          (request) async => http.Response(fail ? serverErrorBody : oneTemplateBody, fail ? 500 : 200),
          recorded,
        ),
      );

      container.read(catalogProvider);
      await pumpEventQueue();
      final firstLoad = container.read(catalogProvider);

      fail = true;
      await container.read(catalogProvider.notifier).refresh();

      expect(recorded, hasLength(2));
      final state = container.read(catalogProvider);
      expect(
        state.templates!.map((template) => template.id).toList(),
        ['fantasy_v1'],
      );
      expect(state.errorMessage, errorCopy(ApiErrorKind.serverError));
      expect(state.hasLoaded, isTrue);
      expect(state.isLoading, isFalse);
      expect(state.lastUpdated, firstLoad.lastUpdated);
    });
  });

  group('failureCopy (CAT-11, CAT-14)', () {
    test('Happy path: domain failures resolve to the Phase-1 kind copy', () {
      for (final kind in ApiErrorKind.values) {
        final error = ApiDomainException(message: 'server said no', kind: kind);
        expect(failureCopy(error), errorCopy(kind), reason: kind.name);
      }
    });

    test('Error: any non-domain failure resolves to the parse copy', () {
      final failures = <Object>[
        const ApiParseException('missing data key'),
        const FormatException('broken json'),
        StateError('unexpected shape'),
      ];

      for (final failure in failures) {
        expect(failureCopy(failure), parseCopy(), reason: '$failure');
      }
    });
  });
}
