import 'package:legnaris/api/api_client_provider.dart';
import 'package:legnaris/api/api_error.dart';
import 'package:legnaris/api/models/template.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'catalog_provider.g.dart';

/// Immutable render state of the catalog, mapping the five render situations
/// the screen distinguishes: loading, error, empty, list, list + inline error.
class CatalogState {
  const CatalogState({
    this.templates,
    this.isLoading = false,
    this.errorMessage,
    this.lastUpdated,
  });

  /// Loaded entries; null while the catalog has never been loaded.
  final List<TemplateCatalog>? templates;

  /// A catalog request is in flight (first load or refresh).
  final bool isLoading;

  /// Resolved UI copy of the last failure; null after a success.
  final String? errorMessage;

  /// Timestamp of the last successful load.
  final DateTime? lastUpdated;

  /// Whether a load ever succeeded.
  bool get hasLoaded => templates != null;

  /// Whether the catalog was loaded and holds zero entries (CAT-10).
  bool get isEmpty => templates != null && templates!.isEmpty;

  /// Copy with the given fields replaced; omitted fields keep their value.
  CatalogState copyWith({
    List<TemplateCatalog>? templates,
    bool? isLoading,
    String? errorMessage,
    DateTime? lastUpdated,
  }) => CatalogState(
    templates: templates ?? this.templates,
    isLoading: isLoading ?? this.isLoading,
    errorMessage: errorMessage ?? this.errorMessage,
    lastUpdated: lastUpdated ?? this.lastUpdated,
  );
}

/// Catalog cache and request policy (CAT-07 … CAT-14, CAT-18, CAT-19).
///
/// The first load is scheduled from [build] so exactly one `GET /templates`
/// runs no matter which widget watches first (CAT-07). [refresh] is
/// single-flight: a trigger while a request is in flight starts nothing new
/// (CAT-13). Success replaces the list and [CatalogState.lastUpdated]
/// (CAT-18); failure keeps the cached list and carries the mapped copy
/// (CAT-19). Nothing is ever persisted — the cache lives in memory only.
@Riverpod(keepAlive: true)
class Catalog extends _$Catalog {
  bool _inFlight = false;

  @override
  CatalogState build() {
    ref.watch(apiClientProvider);
    Future.microtask(refresh);
    return const CatalogState(isLoading: true);
  }

  /// Loads the catalog unless a request is already in flight.
  Future<void> refresh() async {
    if (_inFlight) return;
    _inFlight = true;
    state = state.copyWith(isLoading: true);
    try {
      final templates = await ref.read(apiClientProvider).listTemplates();
      state = CatalogState(
        templates: List<TemplateCatalog>.unmodifiable(templates),
        lastUpdated: DateTime.now(),
      );
    } catch (error) {
      state = CatalogState(
        templates: state.templates,
        errorMessage: failureCopy(error),
        lastUpdated: state.lastUpdated,
      );
    } finally {
      _inFlight = false;
    }
  }
}
