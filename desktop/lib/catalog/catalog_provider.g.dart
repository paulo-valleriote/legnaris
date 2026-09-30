// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'catalog_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// Catalog cache and request policy (CAT-07 … CAT-14, CAT-18, CAT-19).
///
/// The first load is scheduled from [build] so exactly one `GET /templates`
/// runs no matter which widget watches first (CAT-07). [refresh] is
/// single-flight: a trigger while a request is in flight starts nothing new
/// (CAT-13). Success replaces the list and [CatalogState.lastUpdated]
/// (CAT-18); failure keeps the cached list and carries the mapped copy
/// (CAT-19). Nothing is ever persisted — the cache lives in memory only.

@ProviderFor(Catalog)
final catalogProvider = CatalogProvider._();

/// Catalog cache and request policy (CAT-07 … CAT-14, CAT-18, CAT-19).
///
/// The first load is scheduled from [build] so exactly one `GET /templates`
/// runs no matter which widget watches first (CAT-07). [refresh] is
/// single-flight: a trigger while a request is in flight starts nothing new
/// (CAT-13). Success replaces the list and [CatalogState.lastUpdated]
/// (CAT-18); failure keeps the cached list and carries the mapped copy
/// (CAT-19). Nothing is ever persisted — the cache lives in memory only.
final class CatalogProvider extends $NotifierProvider<Catalog, CatalogState> {
  /// Catalog cache and request policy (CAT-07 … CAT-14, CAT-18, CAT-19).
  ///
  /// The first load is scheduled from [build] so exactly one `GET /templates`
  /// runs no matter which widget watches first (CAT-07). [refresh] is
  /// single-flight: a trigger while a request is in flight starts nothing new
  /// (CAT-13). Success replaces the list and [CatalogState.lastUpdated]
  /// (CAT-18); failure keeps the cached list and carries the mapped copy
  /// (CAT-19). Nothing is ever persisted — the cache lives in memory only.
  CatalogProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'catalogProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$catalogHash();

  @$internal
  @override
  Catalog create() => Catalog();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CatalogState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CatalogState>(value),
    );
  }
}

String _$catalogHash() => r'148b14fd1acfe2fcffd79bf92c0f4e17f9944802';

/// Catalog cache and request policy (CAT-07 … CAT-14, CAT-18, CAT-19).
///
/// The first load is scheduled from [build] so exactly one `GET /templates`
/// runs no matter which widget watches first (CAT-07). [refresh] is
/// single-flight: a trigger while a request is in flight starts nothing new
/// (CAT-13). Success replaces the list and [CatalogState.lastUpdated]
/// (CAT-18); failure keeps the cached list and carries the mapped copy
/// (CAT-19). Nothing is ever persisted — the cache lives in memory only.

abstract class _$Catalog extends $Notifier<CatalogState> {
  CatalogState build();
  @$mustCallSuper
  @override
  WhenComplete runBuild() {
    final ref = this.ref as $Ref<CatalogState, CatalogState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<CatalogState, CatalogState>,
              CatalogState,
              Object?,
              Object?
            >;
    return element.handleCreate(ref, build);
  }
}
