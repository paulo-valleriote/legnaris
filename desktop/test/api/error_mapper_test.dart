import 'dart:convert';
import 'dart:io';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/error_mapper.dart';
import 'package:legnaris/l10n/app_localizations.dart';

void main() {
  Map<String, String> arbCopy() {
    final raw = jsonDecode(File('lib/l10n/app_en.arb').readAsStringSync());
    return {
      for (final entry in (raw as Map<String, Object?>).entries)
        if (entry.key.startsWith('error') && entry.value is String)
          entry.key: entry.value as String,
    };
  }

  group('apperr code to error kind mapping', () {
    test('Happy path: INVALID_JSON maps to clientFault', () {
      // FE1-15.
      expect(apperrKind('INVALID_JSON'), ApiErrorKind.clientFault);
    });

    test('Happy path: INVALID_REQUEST maps to invalidEdit', () {
      // FE1-16.
      expect(apperrKind('INVALID_REQUEST'), ApiErrorKind.invalidEdit);
    });

    test('Happy path: UNKNOWN_FIELD maps to renderMismatch', () {
      // FE1-17.
      expect(apperrKind('UNKNOWN_FIELD'), ApiErrorKind.renderMismatch);
    });

    test('Happy path: NOT_FOUND maps to notFound', () {
      // FE1-18.
      expect(apperrKind('NOT_FOUND'), ApiErrorKind.notFound);
    });

    test('Happy path: INTERNAL_ERROR maps to serverError', () {
      // FE1-19.
      expect(apperrKind('INTERNAL_ERROR'), ApiErrorKind.serverError);
    });

    test('Fallback: codes outside the table map to unknown', () {
      // FE1-21: generic fallback for CONFLICT, CYCLE_DETECTED, UNAUTHORIZED…
      for (final code in [
        'CONFLICT',
        'CYCLE_DETECTED',
        'INVALID_DEPENDENCY',
        'INVALID_PATH',
        'UNAUTHORIZED',
        'FORBIDDEN',
      ]) {
        expect(apperrKind(code), ApiErrorKind.unknown, reason: code);
      }
    });
  });

  group('offline kind (FE1-20)', () {
    test('offline kind exists and resolves its localized copy', () {
      // The transport-failure classification itself is wired in T7; here the
      // kind and its copy must exist.
      expect(
        errorCopy(ApiErrorKind.offline),
        arbCopy()['errorOffline'],
      );
    });
  });

  group('unknown fallback preserves the error object (FE1-21)', () {
    test('unmapped code keeps raw code, server message and unknown kind', () {
      const error = ApiDomainException(
        statusCode: 409,
        code: 'CONFLICT',
        message: 'sheet name already exists',
      );
      expect(error.kind, ApiErrorKind.unknown);
      expect(apperrKind(error.code!), ApiErrorKind.unknown);
      expect(error.code, 'CONFLICT');
      expect(error.message, 'sheet name already exists');
      expect(error.statusCode, 409);
    });
  });

  group('localized copy (FE1-23)', () {
    test('every kind resolves the copy defined by the localization layer', () {
      final arb = arbCopy();
      final copyByKind = {
        ApiErrorKind.clientFault: arb['errorClientFault'],
        ApiErrorKind.invalidEdit: arb['errorInvalidEdit'],
        ApiErrorKind.renderMismatch: arb['errorRenderMismatch'],
        ApiErrorKind.notFound: arb['errorNotFound'],
        ApiErrorKind.serverError: arb['errorServerError'],
        ApiErrorKind.offline: arb['errorOffline'],
        ApiErrorKind.unknown: arb['errorUnknown'],
      };
      expect(copyByKind.values, isNot(contains(null)));
      for (final kind in ApiErrorKind.values) {
        expect(errorCopy(kind), copyByKind[kind], reason: kind.name);
      }
    });

    test('English is the only supported locale', () {
      expect(
        AppLocalizations.supportedLocales,
        const <Locale>[Locale('en')],
      );
    });

    test('no API client source hardcodes a copy literal', () {
      // FE1-23: copy must come from the localization layer only.
      final arb = arbCopy();
      expect(arb, isNotEmpty);
      final apiSources =
          Directory('lib/api')
              .listSync()
              .whereType<File>()
              .where((file) => file.path.endsWith('.dart'))
              .toList();
      expect(apiSources, isNotEmpty);
      for (final file in apiSources) {
        final source = file.readAsStringSync();
        for (final copy in arb.values) {
          expect(
            source.contains(copy),
            isFalse,
            reason: '${file.path} hardcodes UI copy "$copy"',
          );
        }
      }
    });
  });
}
