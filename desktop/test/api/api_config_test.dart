import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/api_config.dart';

void main() {
  group('resolveApiBaseUrl', () {
    test('Happy path: defaults to the compiled https base without a flag', () {
      // FE1-25: no API_BASE_URL define → https://api.legnaris.com.br
      expect(resolveApiBaseUrl(), 'https://api.legnaris.com.br');
    });

    test('Happy path: explicit https override prevails over the default', () {
      // FE1-26: the define wins over the compiled default.
      expect(
        resolveApiBaseUrl(rawOverride: 'https://tunnel.example.dev'),
        'https://tunnel.example.dev',
      );
    });

    test('Happy path: explicit http override is accepted (dev tunnel)', () {
      // Spec edge: http:// enters only through an explicit build override;
      // the resolution itself must accept it.
      expect(
        resolveApiBaseUrl(rawOverride: 'http://localhost:8080'),
        'http://localhost:8080',
      );
    });

    test('Edge: empty override falls back to the default base', () {
      // Spec edge: "IF o override API_BASE_URL é string vazia THEN o client
      // SHALL cair no default https, nunca em base vazia."
      expect(resolveApiBaseUrl(rawOverride: ''), 'https://api.legnaris.com.br');
      expect(
        resolveApiBaseUrl(rawOverride: '   '),
        'https://api.legnaris.com.br',
      );
    });

    test('Error: host:port without a scheme is a config error', () {
      // FE1-27: non-absolute URL → config error.
      expect(
        () => resolveApiBaseUrl(rawOverride: 'api:8080'),
        throwsA(isA<ApiConfigException>()),
      );
      expect(
        () => resolveApiBaseUrl(rawOverride: 'localhost:8080'),
        throwsA(isA<ApiConfigException>()),
      );
    });

    test('Error: non-http scheme is a config error', () {
      // FE1-27: only http:// or https:// count as absolute here.
      expect(
        () => resolveApiBaseUrl(rawOverride: 'ftp://files.example'),
        throwsA(isA<ApiConfigException>()),
      );
    });

    test('Error: scheme-less base is a config error', () {
      expect(
        () => resolveApiBaseUrl(rawOverride: 'api.legnaris.com.br'),
        throwsA(isA<ApiConfigException>()),
      );
      expect(
        () => resolveApiBaseUrl(rawOverride: 'https://'),
        throwsA(isA<ApiConfigException>()),
      );
    });
  });

  group('ApiConfig', () {
    test('Happy path: constructor resolves the base before any request', () {
      // FE1-27: validation happens at construction time, so a bad base can
      // never be used to emit a request.
      expect(ApiConfig().baseUrl.toString(), 'https://api.legnaris.com.br');
      expect(
        ApiConfig(rawOverride: 'http://127.0.0.1:3000').baseUrl.toString(),
        'http://127.0.0.1:3000',
      );
    });

    test('Error: constructor rejects a malformed base', () {
      expect(
        () => ApiConfig(rawOverride: 'api:8080'),
        throwsA(isA<ApiConfigException>()),
      );
    });
  });
}
