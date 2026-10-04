// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft_backup_store.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The draft backup boundary over the injected app-support directory.

@ProviderFor(draftBackupStore)
final draftBackupStoreProvider = DraftBackupStoreProvider._();

/// The draft backup boundary over the injected app-support directory.

final class DraftBackupStoreProvider
    extends
        $FunctionalProvider<
          DraftBackupStore,
          DraftBackupStore,
          DraftBackupStore
        >
    with $Provider<DraftBackupStore> {
  /// The draft backup boundary over the injected app-support directory.
  DraftBackupStoreProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'draftBackupStoreProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$draftBackupStoreHash();

  @$internal
  @override
  $ProviderElement<DraftBackupStore> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  DraftBackupStore create(Ref ref) {
    return draftBackupStore(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DraftBackupStore value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DraftBackupStore>(value),
    );
  }
}

String _$draftBackupStoreHash() => r'8775d51a63869c1d7b6f53cd252ceb5895e0e01e';
