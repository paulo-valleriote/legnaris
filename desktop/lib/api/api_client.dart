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

  Future<http.Response> _get(String path) => _httpClient.get(_uri(path));

  Future<http.Response> _post(String path, Map<String, Object?> body) =>
      _httpClient.post(_uri(path), headers: _jsonHeaders, body: jsonEncode(body));

  Future<http.Response> _patch(String path, Map<String, Object?> body) =>
      _httpClient.patch(_uri(path), headers: _jsonHeaders, body: jsonEncode(body));

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
