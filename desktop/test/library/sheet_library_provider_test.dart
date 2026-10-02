import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show ProviderException;
import 'package:flutter_test/flutter_test.dart';
import 'package:legnaris/library/sheet_library_provider.dart';

/// Unit tests for the store providers and the directory seam (LIB-21).
void main() {
  late Directory root;
  late Directory supportDir;

  setUp(() {
    root = Directory.systemTemp.createTempSync('sheet_library_provider_');
    supportDir = Directory('${root.path}/app_support');
    addTearDown(() => root.deleteSync(recursive: true));
  });

  test('Happy path: the directory provider returns the overridden temp dir', () {
    final container = ProviderContainer(
      overrides: [appSupportDirectoryProvider.overrideWithValue(supportDir)],
    );
    addTearDown(container.dispose);

    expect(container.read(appSupportDirectoryProvider).path, supportDir.path);
  });

  test('Happy path: the store provider is rooted at the overridden directory', () {
    final container = ProviderContainer(
      overrides: [appSupportDirectoryProvider.overrideWithValue(supportDir)],
    );
    addTearDown(container.dispose);

    final store = container.read(sheetLibraryStoreProvider);

    expect(store.directory.path, supportDir.path);
    expect(store.fileName, 'sheet_library.json');
  });

  test('Error: reading the directory provider without an override throws StateError', () {
    final container = ProviderContainer();
    addTearDown(container.dispose);

    expect(
      () => container.read(appSupportDirectoryProvider),
      throwsA(
        isA<ProviderException>().having(
          (error) => error.exception,
          'exception',
          isA<StateError>(),
        ),
      ),
    );
  });
}
