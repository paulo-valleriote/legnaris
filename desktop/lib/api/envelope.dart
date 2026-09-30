import 'dart:convert';

import 'package:legnaris/api/api_error.dart';

/// Decodes the API response envelope in a single place (FE1-02, FE1-03,
/// FE1-04, FE1-22).
///
/// On a 2xx, the body must be valid JSON with a `data` key; the payload is
/// handed to [decode] so each endpoint gets its own type. On a non-2xx, the
/// body must be `{"error": {"code", "message"}}` or an [ApiParseException]
/// is raised. Any deviation from those shapes is an [ApiParseException] —
/// never an empty success and never a domain error built from a broken body.
T decodeEnvelope<T>({
  required int statusCode,
  required String body,
  required T Function(Object? data) decode,
}) {
  Object? parsed;
  try {
    parsed = body.isEmpty ? throw const FormatException('empty body') : jsonDecode(body);
  } on FormatException catch (err) {
    throw ApiParseException(
      'Response body is not valid JSON: ${err.message}',
      statusCode: statusCode,
    );
  }

  if (statusCode >= 200 && statusCode < 300) {
    if (parsed is! Map<String, Object?> || !parsed.containsKey('data')) {
      throw ApiParseException(
        'Success response is missing the data envelope',
        statusCode: statusCode,
      );
    }
    return decode(parsed['data']);
  }

  final error = parsed is Map<String, Object?> ? parsed['error'] : null;
  if (error is! Map<String, Object?> ||
      error['code'] is! String ||
      (error['code'] as String).isEmpty) {
    throw ApiParseException(
      'Error response is missing error.code',
      statusCode: statusCode,
    );
  }
  final message = error['message'];
  throw ApiDomainException(
    statusCode: statusCode,
    code: error['code'] as String,
    message: message is String ? message : '',
  );
}
