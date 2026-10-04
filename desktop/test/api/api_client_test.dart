import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_config.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/error_mapper.dart';

void main() {
  const healthBody = '{"data": {"status": "healthy"}}';
  const templateListBody = '''
{"data": [
  {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {}},
  {"id": "sci_fi_v1", "name": "Sci Fi Template", "default": false, "fields": {}}
]}
''';
  const templateBody =
      '{"data": {"id": "fantasy_v1", "name": "Fantasy Template", "default": true, "fields": {"baseArmorClass": {"schema": {"type": "number", "label": "Base AC", "is_editable": true, "default": 10}}}}}';
  const sheetBody =
      '{"data": {"id": "12345678-1234-7000-8000-123456789abc", "template_id": "fantasy_v1", "name": "Aragorn", "data": {"stats": {"dexterity": 16}}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';
  const cascadesBody =
      '{"data": {"derived": {"armorClass": 15}, "stats": {"constitution": 16.5}}}';

  MockClient mockClient(
    Future<http.Response> Function(http.Request request) handler,
    List<http.Request> recorded,
  ) => MockClient((request) async {
    recorded.add(request);
    return handler(request);
  });

  group('typed endpoints (FE1-01, FE1-05, FE1-06)', () {
    test('Happy path: health calls GET /health and returns the status', () async {
      final recorded = <http.Request>[];
      final client = ApiClient(
        httpClient: mockClient(
          (request) async => http.Response(healthBody, 200),
          recorded,
        ),
      );

      expect(await client.health(), 'healthy');
      expect(recorded, hasLength(1));
      expect(recorded.single.method, 'GET');
      expect(recorded.single.url.toString(), 'https://api.legnaris.com.br/health');
    });

    test('Happy path: listTemplates calls GET /templates and decodes the catalog', () async {
      final recorded = <http.Request>[];
      final client = ApiClient(
        httpClient: mockClient(
          (request) async => http.Response(templateListBody, 200),
          recorded,
        ),
      );

      final items = await client.listTemplates();
      expect(items, hasLength(2));
      expect(items.first.id, 'fantasy_v1');
      expect(items.first.isDefault, isTrue);
      expect(items.last.id, 'sci_fi_v1');
      expect(recorded.single.method, 'GET');
      expect(recorded.single.url.path, '/templates');
    });

    test('Happy path: getTemplate calls GET /templates/:id and decodes the detail', () async {
      final recorded = <http.Request>[];
      final client = ApiClient(
        httpClient: mockClient(
          (request) async => http.Response(templateBody, 200),
          recorded,
        ),
      );

      final template = await client.getTemplate('fantasy_v1');
      expect(template.id, 'fantasy_v1');
      expect(template.name, 'Fantasy Template');
      expect(template.isDefault, isTrue);
      expect(template.fields, contains('baseArmorClass'));
      expect(recorded.single.method, 'GET');
      expect(recorded.single.url.path, '/templates/fantasy_v1');
    });

    test('Happy path: createSheet posts POST /sheets with template_id and sheet_name (FE1-05)', () async {
      final recorded = <http.Request>[];
      final client = ApiClient(
        httpClient: mockClient(
          (request) async => http.Response(sheetBody, 201),
          recorded,
        ),
      );

      final sheet = await client.createSheet(
        templateId: 'fantasy_v1',
        sheetName: 'Aragorn',
      );
      expect(sheet.id, '12345678-1234-7000-8000-123456789abc');
      expect(sheet.name, 'Aragorn');
      expect(sheet.templateId, 'fantasy_v1');

      final request = recorded.single;
      expect(request.method, 'POST');
      expect(request.url.path, '/sheets');
      expect(
        jsonDecode(request.body),
        {'template_id': 'fantasy_v1', 'sheet_name': 'Aragorn'},
      );
      expect(request.headers['content-type'], contains('application/json'));
    });

    test('Happy path: getSheet calls GET /sheets/:id and decodes the sheet', () async {
      final recorded = <http.Request>[];
      final client = ApiClient(
        httpClient: mockClient(
          (request) async => http.Response(sheetBody, 200),
          recorded,
        ),
      );

      final sheet = await client.getSheet('12345678-1234-7000-8000-123456789abc');
      expect(sheet.id, '12345678-1234-7000-8000-123456789abc');
      expect(sheet.data['stats'], isA<Map<String, Object?>>());
      expect(recorded.single.method, 'GET');
      expect(recorded.single.url.path, '/sheets/12345678-1234-7000-8000-123456789abc');
    });

    test('Happy path: patchSheet sends PATCH /sheets/:id with only the delta (FE1-06)', () async {
      final recorded = <http.Request>[];
      final client = ApiClient(
        httpClient: mockClient(
          (request) async => http.Response(cascadesBody, 200),
          recorded,
        ),
      );

      final cascades = await client.patchSheet('12345678-1234-7000-8000-123456789abc', {
        'stats': {'dexterity': 16},
      });
      expect(cascades, {
        'derived': {'armorClass': 15},
        'stats': {'constitution': 16.5},
      });

      final request = recorded.single;
      expect(request.method, 'PATCH');
      expect(request.url.path, '/sheets/12345678-1234-7000-8000-123456789abc');
      expect(
        jsonDecode(request.body),
        {
          'data': {
            'stats': {'dexterity': 16},
          },
        },
      );
    });
  });

  group('sheet preview (PRV-03)', () {
    test('Happy path: previewSheet posts POST /sheets/:id/preview with only the delta (PRV-03)', () async {
      final recorded = <http.Request>[];
      final client = ApiClient(
        httpClient: mockClient(
          (request) async => http.Response(cascadesBody, 200),
          recorded,
        ),
      );

      final cascades = await client.previewSheet('12345678-1234-7000-8000-123456789abc', {
        'stats': {'dexterity': 16},
      });
      expect(cascades, {
        'derived': {'armorClass': 15},
        'stats': {'constitution': 16.5},
      });

      final request = recorded.single;
      expect(request.method, 'POST');
      expect(request.url.path, '/sheets/12345678-1234-7000-8000-123456789abc/preview');
      expect(
        jsonDecode(request.body),
        {
          'data': {
            'stats': {'dexterity': 16},
          },
        },
      );
      expect(request.headers['content-type'], contains('application/json'));
    });

    test('Edge: a transport failure issues exactly one preview attempt (no retry)', () async {
      var attempts = 0;
      final client = ApiClient(
        httpClient: MockClient((request) async {
          attempts++;
          throw http.ClientException('connection refused');
        }),
      );

      await expectLater(
        client.previewSheet('12345678-1234-7000-8000-123456789abc', {'stats': {'dexterity': 16}}),
        throwsA(
          isA<ApiDomainException>().having(
            (error) => error.kind,
            'kind',
            ApiErrorKind.offline,
          ),
        ),
      );
      expect(attempts, 1);
    });
  });

  group('injected transport (FE1-07)', () {
    test('Happy path: all six endpoints run through the injected transport', () async {
      final recorded = <http.Request>[];
      final client = ApiClient(
        httpClient: mockClient((request) async {
          if (request.url.path == '/health') {
            return http.Response(healthBody, 200);
          }
          if (request.url.path == '/templates') {
            return http.Response(templateListBody, 200);
          }
          if (request.url.path.startsWith('/templates/')) {
            return http.Response(templateBody, 200);
          }
          if (request.method == 'POST') {
            return http.Response(sheetBody, 201);
          }
          return http.Response(sheetBody, 200);
        }, recorded),
      );

      await client.health();
      await client.listTemplates();
      await client.getTemplate('fantasy_v1');
      await client.createSheet(templateId: 'fantasy_v1', sheetName: 'Aragorn');
      await client.getSheet('12345678-1234-7000-8000-123456789abc');
      await client.patchSheet('12345678-1234-7000-8000-123456789abc', {'stats': {'dexterity': 16}});

      expect(
        recorded.map((r) => '${r.method} ${r.url.path}').toList(),
        [
          'GET /health',
          'GET /templates',
          'GET /templates/fantasy_v1',
          'POST /sheets',
          'GET /sheets/12345678-1234-7000-8000-123456789abc',
          'PATCH /sheets/12345678-1234-7000-8000-123456789abc',
        ],
      );
    });

    test('Happy path: request URLs use the configured base URL', () async {
      final recorded = <http.Request>[];
      final client = ApiClient(
        httpClient: mockClient(
          (request) async => http.Response(healthBody, 200),
          recorded,
        ),
        config: ApiConfig(rawOverride: 'http://localhost:8080'),
      );

      await client.health();
      expect(recorded.single.url.toString(), 'http://localhost:8080/health');
    });
  });

  group('error wiring (FE1-02, FE1-03, FE1-15 … FE1-22)', () {
    Future<ApiDomainException> domainErrorFrom(
      ApiClient client,
      Future<Object?> call,
    ) async {
      try {
        await call;
        fail('expected ApiDomainException');
      } on ApiDomainException catch (error) {
        return error;
      }
    }

    test('Error: 404 NOT_FOUND on getSheet maps to notFound', () async {
      // FE1-18 wiring + FE1-03/FE1-24: status, code and message preserved.
      final client = ApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "NOT_FOUND", "message": "sheet not found"}}',
            404,
          ),
        ),
      );

      final error = await domainErrorFrom(client, client.getSheet('missing'));
      expect(error.kind, ApiErrorKind.notFound);
      expect(error.code, 'NOT_FOUND');
      expect(error.message, 'sheet not found');
      expect(error.statusCode, 404);
    });

    test('Error: 400 INVALID_REQUEST on patchSheet maps to invalidEdit', () async {
      // FE1-16 wiring.
      final client = ApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "INVALID_REQUEST", "message": "field is read-only"}}',
            400,
          ),
        ),
      );

      final error = await domainErrorFrom(
        client,
        client.patchSheet('12345678-1234-7000-8000-123456789abc', {'derived': {'armorClass': 99}}),
      );
      expect(error.kind, ApiErrorKind.invalidEdit);
      expect(error.code, 'INVALID_REQUEST');
      expect(error.statusCode, 400);
    });

    test('Error: 400 UNKNOWN_FIELD on patchSheet maps to renderMismatch', () async {
      // FE1-17 wiring.
      final client = ApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "UNKNOWN_FIELD", "message": "unknown field"}}',
            400,
          ),
        ),
      );

      final error = await domainErrorFrom(
        client,
        client.patchSheet('12345678-1234-7000-8000-123456789abc', {'nope': 1}),
      );
      expect(error.kind, ApiErrorKind.renderMismatch);
      expect(error.code, 'UNKNOWN_FIELD');
    });

    test('Error: 400 UNKNOWN_FIELD on previewSheet maps to renderMismatch (PRV-03)', () async {
      final client = ApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "UNKNOWN_FIELD", "message": "unknown field"}}',
            400,
          ),
        ),
      );

      final error = await domainErrorFrom(
        client,
        client.previewSheet('12345678-1234-7000-8000-123456789abc', {'nope': 1}),
      );
      expect(error.kind, ApiErrorKind.renderMismatch);
      expect(error.code, 'UNKNOWN_FIELD');
      expect(error.statusCode, 400);
    });

    test('Error: 400 INVALID_JSON on createSheet maps to clientFault', () async {
      // FE1-15 wiring.
      final client = ApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "INVALID_JSON", "message": "malformed body"}}',
            400,
          ),
        ),
      );

      final error = await domainErrorFrom(
        client,
        client.createSheet(templateId: 'fantasy_v1', sheetName: 'Aragorn'),
      );
      expect(error.kind, ApiErrorKind.clientFault);
      expect(error.code, 'INVALID_JSON');
    });

    test('Error: 500 INTERNAL_ERROR on health maps to serverError', () async {
      // FE1-19 wiring.
      final client = ApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}',
            500,
          ),
        ),
      );

      final error = await domainErrorFrom(client, client.health());
      expect(error.kind, ApiErrorKind.serverError);
      expect(error.code, 'INTERNAL_ERROR');
      expect(error.statusCode, 500);
    });

    test('Error: unmapped code maps to unknown keeping raw code and message', () async {
      // FE1-21 wiring.
      final client = ApiClient(
        httpClient: MockClient(
          (request) async => http.Response(
            '{"error": {"code": "CONFLICT", "message": "name taken"}}',
            409,
          ),
        ),
      );

      final error = await domainErrorFrom(
        client,
        client.createSheet(templateId: 'fantasy_v1', sheetName: 'Aragorn'),
      );
      expect(error.kind, ApiErrorKind.unknown);
      expect(error.code, 'CONFLICT');
      expect(error.message, 'name taken');
    });

    test('Error: 200 response without the data envelope fails as parse error', () async {
      // FE1-04 wiring: never an empty success.
      final client = ApiClient(
        httpClient: MockClient(
          (request) async => http.Response('{"unexpected": true}', 200),
        ),
      );

      expect(
        () => client.getTemplate('fantasy_v1'),
        throwsA(isA<ApiParseException>()),
      );
    });

    test('Error: 4xx without error.code fails as parse error, not a domain error', () async {
      // FE1-22 wiring.
      final client = ApiClient(
        httpClient: MockClient(
          (request) async => http.Response('{"oops": true}', 500),
        ),
      );

      expect(
        () => client.getSheet('12345678-1234-7000-8000-123456789abc'),
        throwsA(isA<ApiParseException>()),
      );
    });
  });
}
