// Base URL configuration for the API client. The base URL is compiled into
// the build: the default is the public API host, and
// `--dart-define=API_BASE_URL=...` overrides it in any build mode (no debug
// gate — a CI artifact must be able to point at a tunnel too).

/// Compiled-in default base URL (TLS mandatory — see TDD Security).
const String kDefaultApiBaseUrl = 'https://api.legnaris.com.br';

/// Raw `--dart-define=API_BASE_URL` value. Empty string when the flag was
/// not passed at build time.
const String kApiBaseUrlDefine = String.fromEnvironment('API_BASE_URL');

/// Thrown when the resolved base URL is not an absolute `http(s)` URL.
///
/// Raised during config resolution, before any request can be emitted.
class ApiConfigException implements Exception {
  ApiConfigException(this.message);

  final String message;

  @override
  String toString() => 'ApiConfigException: $message';
}

/// Resolves the effective base URL from a raw override value.
///
/// An empty or whitespace-only [rawOverride] falls back to
/// [kDefaultApiBaseUrl]; any non-empty value must be an absolute `http://` or
/// `https://` URL or an [ApiConfigException] is thrown.
String resolveApiBaseUrl({String rawOverride = kApiBaseUrlDefine}) {
  final trimmed = rawOverride.trim();
  final effective = trimmed.isEmpty ? kDefaultApiBaseUrl : trimmed;

  final uri = Uri.tryParse(effective);
  final isHttp = uri != null &&
      (uri.scheme == 'http' || uri.scheme == 'https') &&
      uri.host.isNotEmpty;
  if (!isHttp) {
    throw ApiConfigException(
      'API_BASE_URL must be an absolute http(s) URL, got "$rawOverride"',
    );
  }
  return effective;
}

/// Resolved API configuration handed to the client at construction.
class ApiConfig {
  ApiConfig({String rawOverride = kApiBaseUrlDefine})
    : baseUrl = Uri.parse(resolveApiBaseUrl(rawOverride: rawOverride));

  /// Validated base URL, e.g. `https://api.legnaris.com.br`.
  final Uri baseUrl;
}
