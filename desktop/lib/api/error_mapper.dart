import 'package:flutter/widgets.dart' show Locale;
import 'package:legnaris/l10n/app_localizations.dart';

/// UI-facing failure classes of the API client.
///
/// Each `apperr` code from the backend maps to one of these kinds so screen
/// phases can react by state instead of parsing JSON (spec §Mapeamento).
enum ApiErrorKind {
  /// Bug in the client request (apperr `INVALID_JSON`).
  clientFault,

  /// The edit itself is invalid and must be reverted (apperr
  /// `INVALID_REQUEST`).
  invalidEdit,

  /// Rendered form drifted from the template schema (apperr `UNKNOWN_FIELD`).
  renderMismatch,

  /// The requested resource does not exist (apperr `NOT_FOUND`).
  notFound,

  /// Server-side failure with manual retry expected (apperr `INTERNAL_ERROR`).
  serverError,

  /// Transport failure after retries were exhausted.
  offline,

  /// Fallback for codes without a dedicated kind (e.g. `CONFLICT`,
  /// `CYCLE_DETECTED`, `UNAUTHORIZED`).
  unknown,
}

/// Maps a raw `apperr` code to its [ApiErrorKind] (FE1-15 … FE1-19).
///
/// Codes outside the table fall back to [ApiErrorKind.unknown] (FE1-21); the
/// caller keeps the raw code and server message on the error object.
ApiErrorKind apperrKind(String code) {
  switch (code) {
    case 'INVALID_JSON':
      return ApiErrorKind.clientFault;
    case 'INVALID_REQUEST':
      return ApiErrorKind.invalidEdit;
    case 'UNKNOWN_FIELD':
      return ApiErrorKind.renderMismatch;
    case 'NOT_FOUND':
      return ApiErrorKind.notFound;
    case 'INTERNAL_ERROR':
      return ApiErrorKind.serverError;
    default:
      return ApiErrorKind.unknown;
  }
}

/// Resolves the user-facing copy for [kind] through the localization layer,
/// with English as the only V1 locale (FE1-23). No UI literal lives in the
/// client: every string comes from the generated `AppLocalizations`.
String errorCopy(ApiErrorKind kind) {
  final localizations = lookupAppLocalizations(const Locale('en'));
  switch (kind) {
    case ApiErrorKind.clientFault:
      return localizations.errorClientFault;
    case ApiErrorKind.invalidEdit:
      return localizations.errorInvalidEdit;
    case ApiErrorKind.renderMismatch:
      return localizations.errorRenderMismatch;
    case ApiErrorKind.notFound:
      return localizations.errorNotFound;
    case ApiErrorKind.serverError:
      return localizations.errorServerError;
    case ApiErrorKind.offline:
      return localizations.errorOffline;
    case ApiErrorKind.unknown:
      return localizations.errorUnknown;
  }
}
