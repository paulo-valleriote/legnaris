import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:legnaris/api/api_config.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/envelope.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'auth_api_client.g.dart';

/// The user returned by `POST /auth/register` (no password field).
class AuthUser {
  const AuthUser({required this.id, required this.email, required this.username});

  factory AuthUser.fromJson(Map<String, Object?> json) => AuthUser(
    id: json['id'] as String,
    email: json['email'] as String,
    username: json['username'] as String,
  );

  final String id;
  final String email;
  final String username;
}

/// Access + refresh token pair returned by `/auth/login` and `/auth/refresh`.
class TokenPair {
  const TokenPair({required this.accessToken, required this.refreshToken});

  factory TokenPair.fromJson(Map<String, Object?> json) => TokenPair(
    accessToken: json['access_token'] as String,
    refreshToken: json['refresh_token'] as String,
  );

  final String accessToken;
  final String refreshToken;
}

/// Plain client for the unauthenticated `/auth/*` routes (FE6-03, FE6-05,
/// FE6-07, FE6-10, FE6-21).
///
/// It is deliberately separate from the decorated domain transport: the
/// refresh token travels in the `X-Refresh-Token` header, never a cookie
/// (LGN-111), so these calls must never be authenticated or retried.
class AuthApiClient {
  AuthApiClient({required this._httpClient, ApiConfig? config})
    : _config = config ?? ApiConfig();

  /// Header carrying the opaque refresh token on `/auth/refresh` and
  /// `/auth/logout`.
  static const String refreshTokenHeader = 'X-Refresh-Token';

  /// Fail-fast timeout applied to every auth request.
  static const Duration requestTimeout = Duration(seconds: 5);

  final http.Client _httpClient;
  final ApiConfig _config;

  /// `POST /auth/register` → the created user (no session).
  Future<AuthUser> register({
    required String email,
    required String username,
    required String password,
  }) async => _decode(
    await _post(
      '/auth/register',
      body: {'email': email, 'username': username, 'password': password},
    ),
    (data) => AuthUser.fromJson(data as Map<String, Object?>),
  );

  /// `POST /auth/login` → the token pair.
  Future<TokenPair> login({
    required String email,
    required String password,
  }) async => _decode(
    await _post('/auth/login', body: {'email': email, 'password': password}),
    (data) => TokenPair.fromJson(data as Map<String, Object?>),
  );

  /// `POST /auth/refresh` → a rotated token pair, token in the header.
  Future<TokenPair> refresh(String refreshToken) async => _decode(
    await _post('/auth/refresh', refreshToken: refreshToken),
    (data) => TokenPair.fromJson(data as Map<String, Object?>),
  );

  /// `POST /auth/logout` → `204` with an empty body, token in the header.
  Future<void> logout(String refreshToken) async {
    final response = await _post('/auth/logout', refreshToken: refreshToken);
    if (response.statusCode == 204) {
      return;
    }
    _decode<void>(response, (_) {});
  }

  Uri _uri(String path) => _config.baseUrl.resolve(path);

  Future<http.Response> _post(
    String path, {
    Map<String, Object?>? body,
    String? refreshToken,
  }) async {
    final uri = _uri(path);
    final headers = <String, String>{
      if (body != null) 'content-type': 'application/json; charset=utf-8',
      refreshTokenHeader: ?refreshToken,
    };
    try {
      return await _httpClient
          .post(
            uri,
            headers: headers,
            body: body == null ? null : jsonEncode(body),
          )
          .timeout(requestTimeout);
    } catch (error) {
      throw ApiDomainException(
        message: 'transport failure: $error',
        kind: ApiErrorKind.offline,
      );
    }
  }

  /// Decodes [response] through the envelope decoder and assigns the UI kind,
  /// so every auth failure carries its mapped error (FE6-05, FE6-21).
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

/// The app-wide public auth client over its own plain transport; the decorated
/// domain transport never calls these routes.
@Riverpod(keepAlive: true)
AuthApiClient authApiClient(Ref ref) {
  final client = http.Client();
  ref.onDispose(client.close);
  return AuthApiClient(httpClient: client);
}
