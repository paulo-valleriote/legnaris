/// Failures surfaced by the API client while decoding responses.
library;

/// Domain error produced when a 4xx/5xx response carries a valid error
/// envelope `{"error": {"code", "message"}}` (FE1-03).
///
/// Carries the HTTP status, the raw `apperr` code and the server message so
/// logging never has to redo the call (FE1-24). The UI-facing [kind] is
/// attached by the apperr mapper.
class ApiDomainException implements Exception {
  const ApiDomainException({this.statusCode, this.code, required this.message});

  /// HTTP status of the response, when one was received.
  final int? statusCode;

  /// Raw `apperr` code (e.g. `NOT_FOUND`), when present.
  final String? code;

  /// Message reported by the server.
  final String message;

  @override
  String toString() => 'ApiDomainException($statusCode, $code): $message';
}

/// Defensive parse failure: the body does not match the expected envelope —
/// invalid JSON, a 2xx without `data`, or a 4xx/5xx without `error.code`
/// (FE1-04, FE1-22). Distinct from [ApiDomainException] so callers can tell a
/// contract drift from a mapped failure.
class ApiParseException implements Exception {
  const ApiParseException(this.message, {this.statusCode});

  /// Description of what failed to parse.
  final String message;

  /// HTTP status of the response, when one was received.
  final int? statusCode;

  @override
  String toString() => 'ApiParseException($statusCode): $message';
}
