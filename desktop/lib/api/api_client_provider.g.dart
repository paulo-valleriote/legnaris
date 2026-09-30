// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'api_client_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// App-wide [ApiClient] over a single [http.Client] transport, closed when
/// the provider is disposed. This is the one override point tests use to
/// swap in a fake transport — no screen or provider builds its own client.
/// The `apiClient` function generates the `apiClientProvider` identifier.

@ProviderFor(apiClient)
final apiClientProvider = ApiClientProvider._();

/// App-wide [ApiClient] over a single [http.Client] transport, closed when
/// the provider is disposed. This is the one override point tests use to
/// swap in a fake transport — no screen or provider builds its own client.
/// The `apiClient` function generates the `apiClientProvider` identifier.

final class ApiClientProvider
    extends $FunctionalProvider<ApiClient, ApiClient, ApiClient>
    with $Provider<ApiClient> {
  /// App-wide [ApiClient] over a single [http.Client] transport, closed when
  /// the provider is disposed. This is the one override point tests use to
  /// swap in a fake transport — no screen or provider builds its own client.
  /// The `apiClient` function generates the `apiClientProvider` identifier.
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

String _$apiClientHash() => r'b1a14bb7b8f3ebf99705063739581b2e222dda9f';
