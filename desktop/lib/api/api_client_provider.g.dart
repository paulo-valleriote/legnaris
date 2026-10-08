// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'api_client_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// The raw transport the authenticated domain client decorates. This is the
/// single override point tests use to swap in a fake transport.

@ProviderFor(apiTransport)
final apiTransportProvider = ApiTransportProvider._();

/// The raw transport the authenticated domain client decorates. This is the
/// single override point tests use to swap in a fake transport.

final class ApiTransportProvider
    extends $FunctionalProvider<http.Client, http.Client, http.Client>
    with $Provider<http.Client> {
  /// The raw transport the authenticated domain client decorates. This is the
  /// single override point tests use to swap in a fake transport.
  ApiTransportProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'apiTransportProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$apiTransportHash();

  @$internal
  @override
  $ProviderElement<http.Client> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  http.Client create(Ref ref) {
    return apiTransport(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(http.Client value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<http.Client>(value),
    );
  }
}

String _$apiTransportHash() => r'934868756c108f218cee256f85f252b2bc47fd63';

/// App-wide [ApiClient] over the [AuthenticatedClient] decorator, which adds
/// the bearer header and refreshes on a protected `401` through the session
/// delegate. No screen or provider builds its own client.

@ProviderFor(apiClient)
final apiClientProvider = ApiClientProvider._();

/// App-wide [ApiClient] over the [AuthenticatedClient] decorator, which adds
/// the bearer header and refreshes on a protected `401` through the session
/// delegate. No screen or provider builds its own client.

final class ApiClientProvider
    extends $FunctionalProvider<ApiClient, ApiClient, ApiClient>
    with $Provider<ApiClient> {
  /// App-wide [ApiClient] over the [AuthenticatedClient] decorator, which adds
  /// the bearer header and refreshes on a protected `401` through the session
  /// delegate. No screen or provider builds its own client.
  ApiClientProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'apiClientProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$apiClientHash();

  @$internal
  @override
  $ProviderElement<ApiClient> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  ApiClient create(Ref ref) {
    return apiClient(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ApiClient value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ApiClient>(value),
    );
  }
}

String _$apiClientHash() => r'18ec9b2e456d940dc78412e9f2d8feda53b152a5';
