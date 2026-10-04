import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/error_mapper.dart';

void main() {
  const sheetBody =
      '{"data": {"id": "12345678-1234-7000-8000-123456789abc", "template_id": "fantasy_v1", "name": "Aragorn", "data": {}, "created_at": "2026-09-20T12:00:00Z", "updated_at": "2026-09-21T08:30:00Z"}}';

  Future<ApiDomainException> domainErrorFrom(Future<Object?> call) async {
    try {
      await call;
      fail('expected ApiDomainException');
    } on ApiDomainException catch (error) {
      return error;
    }
  }

  group('timeout and retry policy (FE1-28 … FE1-31)', () {
    test('Happy path: the fail-fast timeout default is the 5 second constant', () {
      // FE1-28: 5s per attempt is the compiled default constant.
      expect(ApiClient.requestTimeout, const Duration(seconds: 5));
      expect(ApiClient.retryDelay, const Duration(milliseconds: 300));
    });

    test('Happy path: a GET retried after the fixed 300ms delay succeeds (FE1-29)', () async {
      final callTimes = <DateTime>[];
      final client = ApiClient(
        httpClient: MockClient((request) async {
          callTimes.add(DateTime.now());
          if (callTimes.length == 1) {
            throw http.ClientException('connection refused');
          }
          return http.Response(sheetBody, 200);
        }),
      );

      final sheet = await client.getSheet('12345678-1234-7000-8000-123456789abc');
      expect(sheet.id, '12345678-1234-7000-8000-123456789abc');
      expect(callTimes, hasLength(2));
      expect(
        callTimes[1].difference(callTimes[0]),
        greaterThanOrEqualTo(const Duration(milliseconds: 300)),
      );
    });

    test('Error: exhausted GET transport failures map to offline after exactly two attempts (FE1-29, FE1-20)', () async {
      var calls = 0;
      final client = ApiClient(
        httpClient: MockClient((request) async {
          calls += 1;
          throw http.ClientException('connection refused');
        }),
      );

      final error = await domainErrorFrom(client.getSheet('12345678-1234-7000-8000-123456789abc'));
      expect(error.kind, ApiErrorKind.offline);
      expect(calls, 2);
    });

    test('Error: every GET attempt is cut by the 5s timeout before going offline (FE1-28)', () async {
      var calls = 0;
      final client = ApiClient(
        httpClient: MockClient((request) async {
          calls += 1;
          return Completer<http.Response>().future;
        }),
      );

      final started = DateTime.now();
      final error = await domainErrorFrom(client.getSheet('12345678-1234-7000-8000-123456789abc'));
      final elapsed = DateTime.now().difference(started);

      expect(error.kind, ApiErrorKind.offline);
      expect(calls, 2);
      expect(elapsed, greaterThanOrEqualTo(const Duration(seconds: 10)));
    });

    test('Error: a GET answered with 4xx is not retried and exposes the mapped error (FE1-30)', () async {
      var calls = 0;
      final client = ApiClient(
        httpClient: MockClient((request) async {
          calls += 1;
          return http.Response(
            '{"error": {"code": "NOT_FOUND", "message": "sheet not found"}}',
            404,
          );
        }),
      );

      final error = await domainErrorFrom(client.getSheet('missing'));
      expect(calls, 1);
      expect(error.kind, ApiErrorKind.notFound);
      expect(error.statusCode, 404);
    });

    test('Error: a GET answered with 5xx is not retried and exposes the mapped error (FE1-30)', () async {
      var calls = 0;
      final client = ApiClient(
        httpClient: MockClient((request) async {
          calls += 1;
          return http.Response(
            '{"error": {"code": "INTERNAL_ERROR", "message": "boom"}}',
            500,
          );
        }),
      );

      final error = await domainErrorFrom(client.getSheet('12345678-1234-7000-8000-123456789abc'));
      expect(calls, 1);
      expect(error.kind, ApiErrorKind.serverError);
    });

    test('Error: a POST transport failure is not retried and maps to offline (FE1-31, FE1-20)', () async {
      var calls = 0;
      final client = ApiClient(
        httpClient: MockClient((request) async {
          calls += 1;
          throw http.ClientException('connection refused');
        }),
      );

      final error = await domainErrorFrom(
        client.createSheet(templateId: 'fantasy_v1', sheetName: 'Aragorn'),
      );
      expect(calls, 1);
      expect(error.kind, ApiErrorKind.offline);
    });

    test('Error: a PATCH transport failure is not retried and maps to offline (FE1-31, FE1-20)', () async {
      var calls = 0;
      final client = ApiClient(
        httpClient: MockClient((request) async {
          calls += 1;
          throw http.ClientException('connection refused');
        }),
      );

      final error = await domainErrorFrom(
        client.patchSheet('12345678-1234-7000-8000-123456789abc', {'stats': {'dexterity': 16}}),
      );
      expect(calls, 1);
      expect(error.kind, ApiErrorKind.offline);
    });

    test('Error: a POST answered with 4xx is not retried either (FE1-31)', () async {
      var calls = 0;
      final client = ApiClient(
        httpClient: MockClient((request) async {
          calls += 1;
          return http.Response(
            '{"error": {"code": "INVALID_REQUEST", "message": "bad name"}}',
            400,
          );
        }),
      );

      final error = await domainErrorFrom(
        client.createSheet(templateId: 'fantasy_v1', sheetName: 'Aragorn'),
      );
      expect(calls, 1);
      expect(error.kind, ApiErrorKind.invalidEdit);
    });
  });
}
