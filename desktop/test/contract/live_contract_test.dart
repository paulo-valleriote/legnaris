/// Live contract suite: runs the four TDD scenarios against the real API when
/// a base is configured (`--dart-define=API_BASE_URL` or the `API_BASE_URL`
/// environment variable) and self-skips otherwise (FE1-35). A configured base
/// always runs the scenarios — an unexpected status or `apperr` code fails the
/// suite with the observed values, never a silent skip (FE1-37).
library;

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:legnaris/api/api_client.dart';
import 'package:legnaris/api/api_config.dart';
import 'package:legnaris/api/api_error.dart';

/// Resolves the live API base for the contract run: the compiled
/// `--dart-define=API_BASE_URL` first, then the `API_BASE_URL` environment
/// variable; null when neither is set (FE1-35).
String? resolveLiveApiBase({
  String rawDefine = kApiBaseUrlDefine,
  Map<String, String>? environment,
}) {
  final define = rawDefine.trim();
  if (define.isNotEmpty) {
    return define;
  }
  final fromEnv = (environment ?? Platform.environment)['API_BASE_URL']?.trim() ?? '';
  return fromEnv.isEmpty ? null : fromEnv;
}

/// Self-skip decision for a scenario: a String skips with that reason when no
/// base is configured (FE1-35); `false` runs the scenario — a configured base
/// never skips (FE1-37).
Object contractSkip(String? base) =>
    base == null
        ? 'No API base configured — set --dart-define=API_BASE_URL or the API_BASE_URL env var to run the live contract scenarios (FE1-35).'
        : false;

/// Records the status of the last response so scenarios can assert the exact
/// contract statuses (201/200) that the typed client abstracts away on success.
class RecordingTransport extends http.BaseClient {
  RecordingTransport(this._inner);

  final http.Client _inner;

  int? lastStatus;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final response = await _inner.send(request);
    lastStatus = response.statusCode;
    return response;
  }

  @override
  void close() => _inner.close();
}

/// Client over a real transport pointed at [base], recording response statuses.
(ApiClient, RecordingTransport) liveClient(String base) {
  final transport = RecordingTransport(http.Client());
  final client = ApiClient(
    httpClient: transport,
    config: ApiConfig(rawOverride: base),
  );
  return (client, transport);
}

/// Fails the test with the observed status and `apperr` code when the
/// scenario outcome diverges from the contract (FE1-37 — report what was
/// observed instead of skipping or passing silently).
void expectContractOutcome({
  required String scenario,
  required int? observedStatus,
  required String? observedCode,
  required int expectedStatus,
  Set<String>? acceptedCodes,
}) {
  final statusMatches = observedStatus == expectedStatus;
  final codeMatches =
      acceptedCodes == null ||
      (observedCode != null && acceptedCodes.contains(observedCode));
  if (statusMatches && codeMatches) {
    return;
  }
  fail(
    '$scenario diverged from the contract — observed status: $observedStatus, '
    'observed apperr code: ${observedCode ?? '<none>'} '
    '(expected status: $expectedStatus'
    '${acceptedCodes == null ? '' : ', accepted codes: ${acceptedCodes.join(' | ')}'})',
  );
}

void main() {
  final base = resolveLiveApiBase();

  group('live scenarios against the real API (FE1-36)', () {
    test('Happy path: POST /sheets answers 201 with computed derived values (FE1-36)', () async {
      final (client, transport) = liveClient(base!);
      final name = 'Live Contract ${DateTime.now().microsecondsSinceEpoch}';

      final sheet = await client.createSheet(
        templateId: 'fantasy_v1',
        sheetName: name,
      );

      expectContractOutcome(
        scenario: 'POST /sheets',
        observedStatus: transport.lastStatus,
        observedCode: null,
        expectedStatus: 201,
      );
      expect(sheet.templateId, 'fantasy_v1');
      expect(sheet.name, name, reason: 'the API echoes the created sheet name');
      expect(
        sheet.valueAt('derived.armorClass'),
        10,
        reason: 'derived value computed at creation: baseArmorClass 10 + dexMod 0 × acUsesDex 1',
      );
    }, skip: contractSkip(base));

    test('Happy path: PATCH an editable field answers 200 with a cascade-only delta (FE1-36)', () async {
      final (client, transport) = liveClient(base!);
      final sheet = await client.createSheet(
        templateId: 'fantasy_v1',
        sheetName: 'Live Contract Editable',
      );
      transport.lastStatus = null;

      final cascades = await client.patchSheet(sheet.id, {
        'stats': {'dexterity': 16},
      });

      expectContractOutcome(
        scenario: 'PATCH /sheets/:id (editable field)',
        observedStatus: transport.lastStatus,
        observedCode: null,
        expectedStatus: 200,
      );
      for (final key in ['id', 'template_id', 'name', 'stats']) {
        expect(
          cascades.containsKey(key),
          isFalse,
          reason: 'the delta is cascade-only — "$key" must not be echoed back',
        );
      }
      expect((cascades['modifiers'] as Map<String, Object?>)['dexterityMod'], 3);
      expect((cascades['derived'] as Map<String, Object?>)['armorClass'], 13);
    }, skip: contractSkip(base));

    test('Error: PATCH a derived field answers 400 INVALID_REQUEST/UNKNOWN_FIELD (FE1-36)', () async {
      final (client, transport) = liveClient(base!);
      final sheet = await client.createSheet(
        templateId: 'fantasy_v1',
        sheetName: 'Live Contract Derived',
      );
      transport.lastStatus = null;

      String? code;
      try {
        await client.patchSheet(sheet.id, {
          'modifiers': {'dexterityMod': 5},
        });
      } on ApiDomainException catch (error) {
        code = error.code;
      }

      expectContractOutcome(
        scenario: 'PATCH /sheets/:id (derived field)',
        observedStatus: transport.lastStatus,
        observedCode: code,
        expectedStatus: 400,
        acceptedCodes: const {'INVALID_REQUEST', 'UNKNOWN_FIELD'},
      );
    }, skip: contractSkip(base));

    test('Error: GET a missing sheet answers 404 NOT_FOUND (FE1-36)', () async {
      final (client, transport) = liveClient(base!);

      String? code;
      try {
        await client.getSheet('00000000-0000-4000-8000-000000000000');
      } on ApiDomainException catch (error) {
        code = error.code;
      }

      expectContractOutcome(
        scenario: 'GET /sheets/:id (missing)',
        observedStatus: transport.lastStatus,
        observedCode: code,
        expectedStatus: 404,
        acceptedCodes: const {'NOT_FOUND'},
      );
    }, skip: contractSkip(base));
  });

  group('skip and failure mechanisms (FE1-35, FE1-37)', () {
    test('Edge: no configured base self-skips the scenarios; a configured base runs them (FE1-35)', () {
      expect(resolveLiveApiBase(rawDefine: '', environment: const {}), isNull);
      expect(
        contractSkip(null),
        isA<String>(),
        reason: 'without a base the scenarios skip with a visible reason',
      );

      expect(
        resolveLiveApiBase(rawDefine: 'http://localhost:8080', environment: const {}),
        'http://localhost:8080',
      );
      expect(
        resolveLiveApiBase(rawDefine: '', environment: const {'API_BASE_URL': 'http://localhost:8080'}),
        'http://localhost:8080',
      );
      expect(
        contractSkip('http://localhost:8080'),
        isFalse,
        reason: 'a configured base must run the scenarios — never a silent skip (FE1-37)',
      );
    });

    test('Error: a divergent outcome fails with the observed status and apperr code (FE1-37)', () {
      expectContractOutcome(
        scenario: 'GET /sheets/:id (missing)',
        observedStatus: 404,
        observedCode: 'NOT_FOUND',
        expectedStatus: 404,
        acceptedCodes: const {'NOT_FOUND'},
      );

      final reported = <String>[];
      try {
        expectContractOutcome(
          scenario: 'status mismatch',
          observedStatus: 500,
          observedCode: 'INTERNAL_ERROR',
          expectedStatus: 404,
          acceptedCodes: const {'NOT_FOUND'},
        );
      } catch (error) {
        reported.add('$error');
      }
      try {
        expectContractOutcome(
          scenario: 'code mismatch',
          observedStatus: 404,
          observedCode: 'FORBIDDEN',
          expectedStatus: 404,
          acceptedCodes: const {'NOT_FOUND'},
        );
      } catch (error) {
        reported.add('$error');
      }

      expect(reported, hasLength(2), reason: 'both divergences must fail, never pass silently');
      expect(reported.first, contains('500'));
      expect(reported.first, contains('INTERNAL_ERROR'));
      expect(reported.last, contains('404'));
      expect(reported.last, contains('FORBIDDEN'));
    });
  });
}
