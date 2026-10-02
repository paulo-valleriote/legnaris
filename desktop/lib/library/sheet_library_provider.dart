import 'dart:io';

import 'package:legnaris/library/sheet_library_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'sheet_library_provider.g.dart';

/// The OS application-support directory local persistence is rooted at.
///
/// The body throws until overridden: `main()` resolves it through
/// `path_provider` at bootstrap and every test overrides it with a temp
/// directory, so the platform channel is never hit by a test.
@Riverpod(keepAlive: true)
Directory appSupportDirectory(Ref ref) {
  throw StateError(
    'appSupportDirectoryProvider must be overridden at bootstrap',
  );
}

/// The sheet library persistence boundary over the injected support directory.
@Riverpod(keepAlive: true)
SheetLibraryStore sheetLibraryStore(Ref ref) =>
    SheetLibraryStore(directory: ref.watch(appSupportDirectoryProvider));
