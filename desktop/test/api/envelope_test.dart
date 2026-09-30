import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/envelope.dart';

void main() {
  group('decodeEnvelope success side', () {
    test('Happy path: 200 with {"data"} returns the decoded payload', () {
      // FE1-02: 2xx + data envelope → payload decoded in the endpoint type.
      final status = decodeEnvelope<Object?>(
        statusCode: 200,
        body: '{"data": {"status": "healthy"}}',
        decode: (data) => (data as Map)['status'],
      );
      expect(status, 'healthy');
    });

    test('Happy path: 201 with {"data"} returns the decoded payload', () {
      final created = decodeEnvelope<List<Object?>>(
        statusCode: 201,
        body: '{"data": [1, 2, 3]}',
        decode: (data) => data as List<Object?>,
      );
      expect(created, [1, 2, 3]);
    });
  });

  group('decodeEnvelope error envelope', () {
    test('Happy path: 4xx error envelope becomes a domain error', () {
      // FE1-03 + FE1-24: domain error carries status, apperr code and
      // server message.
      expect(
        () => decodeEnvelope<Object?>(
          statusCode: 404,
          body: '{"error": {"code": "NOT_FOUND", "message": "sheet not found"}}',
          decode: (data) => data,
        ),
        throwsA(
          isA<ApiDomainException>()
              .having((e) => e.statusCode, 'statusCode', 404)
              .having((e) => e.code, 'code', 'NOT_FOUND')
              .having((e) => e.message, 'message', 'sheet not found'),
        ),
      );
    });

    test('Happy path: 5xx error envelope becomes a domain error', () {
      expect(
        () => decodeEnvelope<Object?>(
          statusCode: 500,
          body:
              '{"error": {"code": "INTERNAL_ERROR", "message": "internal server error"}}',
          decode: (data) => data,
        ),
        throwsA(
          isA<ApiDomainException>()
              .having((e) => e.statusCode, 'statusCode', 500)
              .having((e) => e.code, 'code', 'INTERNAL_ERROR')
              .having(
                (e) => e.message,
                'message',
                'internal server error',
              ),
        ),
      );
    });
  });

  group('decodeEnvelope defensive parse (success side)', () {
    test('Error: 200 with invalid JSON is a parse error, never success', () {
      // FE1-04.
      expect(
        () => decodeEnvelope<Object?>(
          statusCode: 200,
          body: 'not-json',
          decode: (data) => data,
        ),
        throwsA(isA<ApiParseException>()),
      );
    });

    test('Error: 200 without the data key is a parse error', () {
      // FE1-04: no empty success when the envelope shape drifts.
      expect(
        () => decodeEnvelope<Object?>(
          statusCode: 200,
          body: '{"status": "ok"}',
          decode: (data) => data,
        ),
        throwsA(isA<ApiParseException>()),
      );
    });

    test('Error: empty 2xx body is a parse error', () {
      // Spec edge: empty 2xx body → defensive parse error.
      expect(
        () => decodeEnvelope<Object?>(
          statusCode: 200,
          body: '',
          decode: (data) => data,
        ),
        throwsA(isA<ApiParseException>()),
      );
    });
  });

  group('decodeEnvelope defensive parse (error side)', () {
    test('Error: 4xx without error.code is a parse error, not a domain error', () {
      // FE1-22: must NOT surface as a mapped domain error.
      expect(
        () => decodeEnvelope<Object?>(
          statusCode: 500,
          body: '{"details": "boom"}',
          decode: (data) => data,
        ),
        throwsA(isA<ApiParseException>()),
      );
      expect(
        () => decodeEnvelope<Object?>(
          statusCode: 400,
          body: '{"error": {"message": "boom"}}',
          decode: (data) => data,
        ),
        throwsA(isA<ApiParseException>()),
      );
    });

    test('Error: 4xx with invalid JSON is a parse error, not a domain error', () {
      // FE1-22.
      expect(
        () => decodeEnvelope<Object?>(
          statusCode: 400,
          body: '<html>502 Bad Gateway</html>',
          decode: (data) => data,
        ),
        throwsA(
          isA<ApiParseException>().having(
            (e) => e.statusCode,
            'statusCode',
            400,
          ),
        ),
      );
    });

    test('Error: error field that is not an envelope is a parse error', () {
      // FE1-22: only {"error": {code, message}} counts as the error envelope.
      expect(
        () => decodeEnvelope<Object?>(
          statusCode: 500,
          body: '{"error": "boom"}',
          decode: (data) => data,
        ),
        throwsA(isA<ApiParseException>()),
      );
    });
  });
}
