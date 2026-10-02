// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'sheet_library_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The OS application-support directory local persistence is rooted at.
///
/// The body throws until overridden: `main()` resolves it through
/// `path_provider` at bootstrap and every test overrides it with a temp
/// directory, so the platform channel is never hit by a test.

@ProviderFor(appSupportDirectory)
final appSupportDirectoryProvider = AppSupportDirectoryProvider._();

/// The OS application-support directory local persistence is rooted at.
///
/// The body throws until overridden: `main()` resolves it through
/// `path_provider` at bootstrap and every test overrides it with a temp
/// directory, so the platform channel is never hit by a test.

final class AppSupportDirectoryProvider
    extends $FunctionalProvider<Directory, Directory, Directory>
    with $Provider<Directory> {
  /// The OS application-support directory local persistence is rooted at.
  ///
  /// The body throws until overridden: `main()` resolves it through
  /// `path_provider` at bootstrap and every test overrides it with a temp
  /// directory, so the platform channel is never hit by a test.
  AppSupportDirectoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'appSupportDirectoryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$appSupportDirectoryHash();

  @$internal
  @override
  $ProviderElement<Directory> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  Directory create(Ref ref) {
    return appSupportDirectory(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(Directory value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<Directory>(value),
    );
  }
}

String _$appSupportDirectoryHash() =>
    r'e141ff2f505ac7e879c8891cb6eb5c33fa5410d2';

/// The sheet library persistence boundary over the injected support directory.

@ProviderFor(sheetLibraryStore)
final sheetLibraryStoreProvider = SheetLibraryStoreProvider._();

/// The sheet library persistence boundary over the injected support directory.

final class SheetLibraryStoreProvider
    extends
        $FunctionalProvider<
          SheetLibraryStore,
          SheetLibraryStore,
          SheetLibraryStore
        >
    with $Provider<SheetLibraryStore> {
  /// The sheet library persistence boundary over the injected support directory.
  SheetLibraryStoreProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sheetLibraryStoreProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sheetLibraryStoreHash();

  @$internal
  @override
  $ProviderElement<SheetLibraryStore> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SheetLibraryStore create(Ref ref) {
    return sheetLibraryStore(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SheetLibraryStore value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SheetLibraryStore>(value),
    );
  }
}

String _$sheetLibraryStoreHash() => r'35fe824a649e29814ab66c5afcb119b5429e0d93';

/// The in-memory sheet index and the only state the surfaces read (LIB-21 …
/// LIB-23).
///
/// Write-through: [upsert] and [remove] persist the complete index before
/// publishing, so a failed save leaves the published entries equal to the last
/// persisted index.

@ProviderFor(SheetLibrary)
final sheetLibraryProvider = SheetLibraryProvider._();

/// The in-memory sheet index and the only state the surfaces read (LIB-21 …
/// LIB-23).
///
/// Write-through: [upsert] and [remove] persist the complete index before
/// publishing, so a failed save leaves the published entries equal to the last
/// persisted index.
final class SheetLibraryProvider
    extends $NotifierProvider<SheetLibrary, SheetLibraryState> {
  /// The in-memory sheet index and the only state the surfaces read (LIB-21 …
  /// LIB-23).
  ///
  /// Write-through: [upsert] and [remove] persist the complete index before
  /// publishing, so a failed save leaves the published entries equal to the last
  /// persisted index.
  SheetLibraryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sheetLibraryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sheetLibraryHash();

  @$internal
  @override
  SheetLibrary create() => SheetLibrary();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SheetLibraryState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SheetLibraryState>(value),
    );
  }
}

String _$sheetLibraryHash() => r'32031eca4f669f8c029536d1cbc1130b231e8b50';

/// The in-memory sheet index and the only state the surfaces read (LIB-21 …
/// LIB-23).
///
/// Write-through: [upsert] and [remove] persist the complete index before
/// publishing, so a failed save leaves the published entries equal to the last
/// persisted index.

abstract class _$SheetLibrary extends $Notifier<SheetLibraryState> {
  SheetLibraryState build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<SheetLibraryState, SheetLibraryState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<SheetLibraryState, SheetLibraryState>,
              SheetLibraryState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}
