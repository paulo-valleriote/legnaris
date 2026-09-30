import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:legnaris/api/api_config.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/envelope.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/api/models/sheet.dart';
import 'package:legnaris/api/models/template.dart';

/// Typed client for the six endpoints consumed by the desktop app (FE1-01).
///
/// The [http.Client] transport is injected at construction (FE1-07): it is
/// the single extension point for auth in V2, and no endpoint method ever
/// creates its own transport.
class ApiClient {
  ApiClient({required this._httpClient, ApiConfig? config})
    : _config = config ?? ApiConfig();

  static const Map<String, String> _jsonHeaders = {
    'content-type': 'application/json; charset=utf-8',
  };

  /// Fail-fast timeout applied to every request attempt (FE1-28).
  static const Duration requestTimeout = Duration(seconds: 5);

  /// Fixed backoff before the single GET retry, transport failure only
  /// (FE1-29). The policy is fixed by spec: POST/PATCH never retry.
  static const Duration retryDelay = Duration(milliseconds: 300);

  final http.Client _httpClient;
  final ApiConfig _config;

  /// `GET /health` → the reported status string.
  Future<String> health() async => _decode(
    await _get('/health'),
    (data) => (data as Map<String, Object?>)['status'] as String,
  );

  /// `GET /templates` → every catalog entry, decoded with the full template
  /// shape (the catalog UI ignores [TemplateDetail.fields]).
  Future<List<TemplateCatalog>> listTemplates() async =>
      _decode(await _get('/templates'), TemplateCatalog.listFromJson);

  /// `GET /templates/:id` → the template detail.
  Future<TemplateDetail> getTemplate(String templateId) async => _decode(
    await _get('/templates/${Uri.encodeComponent(templateId)}'),
    (data) => TemplateDetail.fromJson(data as Map<String, Object?>),
  );

  /// `POST /sheets` → the created sheet, sent as
  /// `{"template_id": ..., "sheet_name": ...}` (FE1-05).
  Future<Sheet> createSheet({
    required String templateId,
    required String sheetName,
  }) async => _decode(
    await _post('/sheets', {
      'template_id': templateId,
      'sheet_name': sheetName,
    }),
    (data) => Sheet.fromJson(data as Map<String, Object?>),
  );

  /// `GET /sheets/:id` → the stored sheet.
  Future<Sheet> getSheet(String sheetId) async => _decode(
    await _get('/sheets/${Uri.encodeComponent(sheetId)}'),
    (data) => Sheet.fromJson(data as Map<String, Object?>),
  );

  /// `PATCH /sheets/:id` → the cascading effects of the update, sent as
  /// `{"data": <delta>}` with only the caller's changes (FE1-06).
  Future<Map<String, Object?>> patchSheet(
    String sheetId,
    Map<String, Object?> delta,
  ) async => _decode(
    await _patch('/sheets/${Uri.encodeComponent(sheetId)}', {'data': delta}),
    (data) => data as Map<String, Object?>,
  );

  Uri _uri(String path) => _config.baseUrl.resolve(path);

  /// GETs [path], retrying exactly once after [retryDelay] when the transport
  /// itself fails (timeout, refused connection, DNS). A received response —
  /// 4xx or 5xx included — ends the attempt and is mapped by the envelope
  /// decoder (FE1-30). Once attempts are exhausted the failure becomes an
  /// `offline` domain error (FE1-20).
  Future<http.Response> _get(String path) async {
    final uri = _uri(path);
    try {
      return await _attempt(uri);
    } catch (_) {
      await Future<void>.delayed(retryDelay);
      try {
        return await _attempt(uri);
      } catch (error) {
        throw _offline(error);
      }
    }
  }

  Future<http.Response> _attempt(Uri uri) =>
      _httpClient.get(uri).timeout(requestTimeout);

  Future<http.Response> _post(String path, Map<String, Object?> body) async {
    final uri = _uri(path);
    final encoded = jsonEncode(body);
    try {
      return await _httpClient
          .post(uri, headers: _jsonHeaders, body: encoded)
          .timeout(requestTimeout);
    } catch (error) {
      throw _offline(error);
    }
  }

  Future<http.Response> _patch(String path, Map<String, Object?> body) async {
    final uri = _uri(path);
    final encoded = jsonEncode(body);
    try {
      return await _httpClient
          .patch(uri, headers: _jsonHeaders, body: encoded)
          .timeout(requestTimeout);
    } catch (error) {
      throw _offline(error);
    }
  }

  ApiDomainException _offline(Object cause) => ApiDomainException(
    message: 'transport failure: $cause',
    kind: ApiErrorKind.offline,
  );

  /// Decodes [response] through the single envelope decoder and wires the
  /// apperr mapper, so every domain error leaving the client carries its UI
  /// kind (FE1-15 … FE1-21).
  T _decode<T>(http.Response response, T Function(Object? data) decode) {
    try {
      return decodeEnvelope(
        statusCode: response.statusCode,
        body: response.body,
        decode: decode,
      );
    } on ApiDomainException catch (error) {
      throw ApiDomainException(
        statusCode: error.statusCode,
        code: error.code,
        message: error.message,
        kind: apperrKind(error.code!),
      );
    }
  }
}
