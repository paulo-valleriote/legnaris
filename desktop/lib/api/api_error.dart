/// Failures surfaced by the API client while decoding responses.
library;

import 'package:flutter/widgets.dart' show Locale;
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/l10n/app_localizations.dart';

/// Domain error produced when a 4xx/5xx response carries a valid error
/// envelope `{"error": {"code", "message"}}` (FE1-03).
///
/// Carries the HTTP status, the raw `apperr` code and the server message so
/// logging never has to redo the call (FE1-24), plus the UI-facing [kind]
/// assigned by the apperr mapper (FE1-15 … FE1-21).
class ApiDomainException implements Exception {
  const ApiDomainException({
    this.statusCode,
    this.code,
    required this.message,
    this.kind = ApiErrorKind.unknown,
  });

  /// HTTP status of the response, when one was received.
  final int? statusCode;

  /// Raw `apperr` code (e.g. `NOT_FOUND`), when present.
  final String? code;

  /// Message reported by the server.
  final String message;

  /// UI failure class this error maps to; defaults to
  /// [ApiErrorKind.unknown] until the apperr mapper assigns one.
  final ApiErrorKind kind;

  @override
  String toString() =>
      'ApiDomainException($statusCode, $code, ${kind.name}): $message';
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

/// Resolves the user-facing copy for any [error] raised while loading data.
///
/// Domain failures keep the Phase-1 kind copy (CAT-11/CAT-19); everything
/// else — broken JSON, a missing `data` key, a payload that drifted from the
/// model — gets the defensive parse copy (CAT-14), so contract drift always
/// renders as a mapped message instead of a blank screen. Lives beside the
/// exception types to avoid an import cycle with `error_mapper`.
String failureCopy(Object error) {
  if (error is ApiDomainException) {
    return errorCopy(error.kind);
  }
  return lookupAppLocalizations(const Locale('en')).errorParse;
}
